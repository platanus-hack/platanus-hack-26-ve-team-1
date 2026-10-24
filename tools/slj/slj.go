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

// Formatos (docs/p4-qr.md). Enteros big-endian, firmas ECDSA P-256 raw r||s de 64 B.

const (
	verChip     = 0x01
	verWallet   = 0x02
	headerLen   = 16
	sigLen      = 64
	devPubLen   = 33
	prefix      = "SLJ1:"
	windowSecs  = 30
	rotateSecs  = 20
	CodeOK      = 1
	CodeBadSig  = 2
	CodeRevoked = 3
	CodeExpired = 4
	CodeNoPerm  = 5
	CodeStale   = 7
	CodeBadFmt  = 8
)

var tag = []byte("SLJ1")

type Cred struct {
	Ver, Issuer, Attr byte
	Exp               uint32
	Handle            uint64
	DevPub            []byte // 33 B, solo ver=0x02
	Value             []byte
	Sig               []byte
	Signed            []byte
	Raw               []byte
}

type Pres struct {
	Cred   *Cred
	T      uint32
	DevSig []byte
	Attr   byte
}

func credLen(valueLen int, ver byte) int {
	n := headerLen + valueLen + sigLen
	if ver == verWallet {
		n += devPubLen
	}
	return n
}

func parseCred(b []byte) (*Cred, error) {
	if len(b) < headerLen+sigLen || (b[0] != verChip && b[0] != verWallet) {
		return nil, errors.New("credencial inválida")
	}
	n := int(b[3])
	if len(b) != credLen(n, b[0]) {
		return nil, errors.New("largo de credencial inválido")
	}
	c := &Cred{Ver: b[0], Issuer: b[1], Attr: b[2], Exp: binary.BigEndian.Uint32(b[4:8]), Handle: binary.BigEndian.Uint64(b[8:16]), Raw: b}
	vo := headerLen
	if c.Ver == verWallet {
		c.DevPub = b[headerLen : headerLen+devPubLen]
		vo += devPubLen
	}
	c.Value = b[vo : vo+n]
	c.Sig = b[vo+n:]
	c.Signed = b[:vo+n]
	return c, nil
}

func encodeUnsigned(ver, issuer, attr byte, exp uint32, handle uint64, devPub, value []byte) []byte {
	b := []byte{ver, issuer, attr, byte(len(value))}
	b = binary.BigEndian.AppendUint32(b, exp)
	b = binary.BigEndian.AppendUint64(b, handle)
	b = append(b, devPub...)
	return append(b, value...)
}

func parsePresentation(qr string) (*Pres, error) {
	if !strings.HasPrefix(qr, prefix) {
		return nil, errors.New("prefijo")
	}
	b, err := base64.RawURLEncoding.DecodeString(qr[len(prefix):])
	if err != nil || len(b) < headerLen+devPubLen+sigLen+4+sigLen+1 || b[0] != verWallet {
		return nil, errors.New("formato")
	}
	cl := credLen(int(b[3]), verWallet)
	if len(b) != cl+4+sigLen+1 {
		return nil, errors.New("largo")
	}
	c, err := parseCred(b[:cl])
	if err != nil {
		return nil, err
	}
	return &Pres{Cred: c, T: binary.BigEndian.Uint32(b[cl:]), DevSig: b[cl+4 : cl+4+sigLen], Attr: b[cl+4+sigLen]}, nil
}

func deviceMessage(t uint32, cred []byte) []byte {
	m := append([]byte{}, tag...)
	m = binary.BigEndian.AppendUint32(m, t)
	return append(m, cred...)
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

// verifyPresentation replica el orden de verify.ts: lo barato primero, firmas al final.
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
	if int64(cr.Exp) < c.Now {
		return r(CodeExpired)
	}
	if len(c.Accept) > 0 && !c.Accept[cr.Attr] {
		return r(CodeNoPerm)
	}
	if c.Revoked[cr.Handle] {
		return r(CodeRevoked)
	}
	ik, ok := c.Issuers[cr.Issuer]
	if !ok {
		return r(CodeBadFmt)
	}
	dk, err := pubFromAny(cr.DevPub)
	if err != nil {
		return r(CodeBadSig)
	}
	if !verifyRaw(ik, cr.Sig, cr.Signed) || !verifyRaw(dk, p.DevSig, deviceMessage(p.T, cr.Raw)) {
		return r(CodeBadSig)
	}
	return r(CodeOK)
}
