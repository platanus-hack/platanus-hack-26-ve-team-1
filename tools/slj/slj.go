package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/big"
	"strings"
)

// Formatos del contrato de P1 (docs/contrato.md en p1-backend). Big-endian; firmas ECDSA P-256 raw r||s de 64 B.
//
//	blob = cabecera(16) ‖ valor(largo) ‖ clave del celular(33) ‖ firma(64)
//	cabecera = ver(1)=0x01 ‖ emisor(1) ‖ atributo(1) ‖ largo(1) ‖ vence(4) ‖ handle(8)
//	el emisor firma cabecera ‖ valor ‖ clave del celular
//	QR = "SLJ1:" + base64url( blob ‖ hora(4) ‖ firma del celular sobre (hora ‖ atributo) ‖ atributo(1) )

const (
	ver         = 0x01
	headerLen   = 16
	sigLen      = 64
	devPubLen   = 33
	prefix      = "SLJ1:"
	windowSecs  = 30
	CodeOK      = 1
	CodeBadSig  = 2
	CodeRevoked = 3
	CodeExpired = 4
	CodeNoPerm  = 5
	CodeStale   = 7 // interno de la pantalla del verificador; nunca se envía a la API
	CodeBadFmt  = 8 // interno
)

type Cred struct {
	Ver, Issuer, Attr byte
	Exp               uint32
	Handle            uint64
	Value             []byte
	DevPub            []byte // 33 B
	Sig               []byte
	Signed            []byte // lo que cubre la firma del emisor
	Raw               []byte
}

type Pres struct {
	Cred   *Cred
	T      uint32
	DevSig []byte
	Attr   byte
}

func credLen(valueLen int) int { return headerLen + valueLen + devPubLen + sigLen }

func parseCred(b []byte) (*Cred, error) {
	if len(b) < credLen(0) || b[0] != ver {
		return nil, errors.New("credencial inválida")
	}
	n := int(b[3])
	if len(b) != credLen(n) {
		return nil, errors.New("largo de credencial inválido")
	}
	vEnd := headerLen + n
	pEnd := vEnd + devPubLen
	return &Cred{
		Ver: b[0], Issuer: b[1], Attr: b[2],
		Exp: binary.BigEndian.Uint32(b[4:8]), Handle: binary.BigEndian.Uint64(b[8:16]),
		Value: b[headerLen:vEnd], DevPub: b[vEnd:pEnd], Sig: b[pEnd:], Signed: b[:pEnd], Raw: b,
	}, nil
}

// encodeUnsigned devuelve cabecera ‖ valor ‖ clave del celular: lo que firma el emisor.
func encodeUnsigned(issuer, attr byte, exp uint32, handle uint64, devPub, value []byte) []byte {
	b := []byte{ver, issuer, attr, byte(len(value))}
	b = binary.BigEndian.AppendUint32(b, exp)
	b = binary.BigEndian.AppendUint64(b, handle)
	b = append(b, value...)
	return append(b, devPub...)
}

func parsePresentation(qr string) (*Pres, error) {
	if !strings.HasPrefix(qr, prefix) {
		return nil, errors.New("prefijo")
	}
	b, err := base64.RawURLEncoding.DecodeString(qr[len(prefix):])
	if err != nil || len(b) < credLen(0)+4+sigLen+1 {
		return nil, errors.New("formato")
	}
	cl := credLen(int(b[3]))
	if len(b) != cl+4+sigLen+1 {
		return nil, errors.New("largo")
	}
	c, err := parseCred(b[:cl])
	if err != nil {
		return nil, err
	}
	return &Pres{Cred: c, T: binary.BigEndian.Uint32(b[cl:]), DevSig: b[cl+4 : cl+4+sigLen], Attr: b[cl+4+sigLen]}, nil
}

// deviceMessage es lo que firma el celular: hora(4) ‖ atributo(1).
func deviceMessage(t uint32, attr byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, t), attr)
}

func encodePresentation(cred []byte, t uint32, devSig []byte, attr byte) string {
	b := append([]byte{}, cred...)
	b = binary.BigEndian.AppendUint32(b, t)
	b = append(b, devSig...)
	b = append(b, attr)
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}

type rs struct{ R, S *big.Int }

func signRaw(priv *ecdsa.PrivateKey, msg []byte) ([]byte, error) {
	h := sha256.Sum256(msg)
	der, err := ecdsa.SignASN1(randReader{}, priv, h[:])
	if err != nil {
		return nil, err
	}
	var v rs
	if _, err := asn1.Unmarshal(der, &v); err != nil {
		return nil, err
	}
	out := make([]byte, sigLen)
	v.R.FillBytes(out[:32])
	v.S.FillBytes(out[32:])
	return out, nil
}

func verifyRaw(pub *ecdsa.PublicKey, sig, msg []byte) bool {
	if len(sig) != sigLen {
		return false
	}
	der, err := asn1.Marshal(rs{new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])})
	if err != nil {
		return false
	}
	h := sha256.Sum256(msg)
	return ecdsa.VerifyASN1(pub, h[:], der)
}

func compress(pub *ecdsa.PublicKey) []byte {
	raw, _ := pub.Bytes()
	out := make([]byte, 33)
	out[0] = 2 + (raw[64] & 1)
	copy(out[1:], raw[1:33])
	return out
}

// pubFromAny acepta 33 B (comprimida) o 65 B (sin comprimir).
func pubFromAny(b []byte) (*ecdsa.PublicKey, error) {
	if len(b) == 33 {
		x, y := elliptic.UnmarshalCompressed(elliptic.P256(), b)
		if x == nil {
			return nil, errors.New("punto fuera de la curva")
		}
		raw := make([]byte, 65)
		raw[0] = 4
		x.FillBytes(raw[1:33])
		y.FillBytes(raw[33:])
		b = raw
	}
	return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), b)
}

type VerifyCtx struct {
	Issuers map[byte]*ecdsa.PublicKey
	Revoked map[uint64]bool
	Accept  map[byte]bool
	Now     int64
}

type Result struct {
	Code int
	Cred *Cred
	T    uint32
}

// verifyPresentation replica el orden de verify.ts: formato → hora → emisor conocido → firmas → vencimiento →
// permiso → revocación. Las firmas van antes que cualquier resultado que nombre la cédula.
func verifyPresentation(qr string, c VerifyCtx) Result {
	p, err := parsePresentation(qr)
	if err != nil {
		return Result{Code: CodeBadFmt}
	}
	r := func(code int) Result { return Result{Code: code, Cred: p.Cred, T: p.T} }
	cr := p.Cred
	if p.Attr != cr.Attr {
		return r(CodeBadFmt)
	}
	if d := c.Now - int64(p.T); d > windowSecs || d < -windowSecs {
		return r(CodeStale)
	}
	ik, ok := c.Issuers[cr.Issuer]
	if !ok {
		return r(CodeBadFmt)
	}
	dk, err := pubFromAny(cr.DevPub)
	if err != nil {
		return r(CodeBadSig)
	}
	if !verifyRaw(ik, cr.Sig, cr.Signed) || !verifyRaw(dk, p.DevSig, deviceMessage(p.T, cr.Attr)) {
		return r(CodeBadSig)
	}
	if int64(cr.Exp) < c.Now {
		return r(CodeExpired)
	}
	if len(c.Accept) > 0 && !c.Accept[cr.Attr] {
		return r(CodeNoPerm)
	}
	if c.Revoked[cr.Handle] {
		return r(CodeRevoked)
	}
	return r(CodeOK)
}
