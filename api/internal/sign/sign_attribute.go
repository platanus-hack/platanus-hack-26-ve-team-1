// Package sign implementa el contrato de bytes de SoloLoJusto (docs/contrato.md):
// atributos firmados con ECDSA P-256, firma cruda r‖s (IEEE P1363).
package sign

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

const (
	Version = 0x01

	IssuerNacional    = 0x01
	IssuerUniversidad = 0x02

	AttrMayor18    = 0x01
	AttrMayor21    = 0x02
	AttrIdentidad  = 0x03
	AttrSangre     = 0x04
	AttrEmergencia = 0x05
	AttrDonante    = 0x06

	HeaderLen = 16
	SigLen    = 64
	BlockLen  = 16
	PubLen    = 33
)

// Attribute es un atributo del chip antes de firmar.
type Attribute struct {
	Issuer  byte
	Code    byte
	Expires uint32 // Unix, big-endian en la cabecera
	Handle  [8]byte
	Value   []byte // sin relleno
}

// MaxValueLen devuelve el largo máximo del valor: 64 B en identidad, 16 B en el resto.
func MaxValueLen(code byte) int {
	if code == AttrIdentidad {
		return 64
	}
	return 16
}

// Validate comprueba emisor, código y largo del valor.
func (a Attribute) Validate() error {
	if a.Issuer != IssuerNacional && a.Issuer != IssuerUniversidad {
		return fmt.Errorf("emisor inválido: 0x%02x", a.Issuer)
	}
	if a.Code < AttrMayor18 || a.Code > AttrDonante {
		return fmt.Errorf("atributo inválido: 0x%02x", a.Code)
	}
	if len(a.Value) > MaxValueLen(a.Code) {
		return fmt.Errorf("valor de %d B excede el máximo de %d B", len(a.Value), MaxValueLen(a.Code))
	}
	return nil
}

// Header arma la cabecera de 16 B: version|emisor|atributo|largo|vence(4)|handle(8).
func (a Attribute) Header() []byte {
	h := make([]byte, HeaderLen)
	h[0] = Version
	h[1] = a.Issuer
	h[2] = a.Code
	h[3] = byte(len(a.Value))
	binary.BigEndian.PutUint32(h[4:8], a.Expires)
	copy(h[8:16], a.Handle[:])
	return h
}

// Message es lo que firma el chip: UID ‖ cabecera ‖ valor[0:largo] (sin relleno).
func (a Attribute) Message(uid []byte) ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	m := make([]byte, 0, len(uid)+HeaderLen+len(a.Value))
	m = append(m, uid...)
	m = append(m, a.Header()...)
	m = append(m, a.Value...)
	return m, nil
}

// Sign firma Message(uid) con la clave del emisor. Devuelve r‖s de 64 B.
func (a Attribute) Sign(priv *ecdsa.PrivateKey, uid []byte) ([]byte, error) {
	m, err := a.Message(uid)
	if err != nil {
		return nil, err
	}
	return SignRaw(priv, m)
}

// Verify comprueba la firma del atributo para ese UID.
func (a Attribute) Verify(pub *ecdsa.PublicKey, uid, sig []byte) bool {
	m, err := a.Message(uid)
	if err != nil {
		return false
	}
	return VerifyRaw(pub, m, sig)
}

// Record arma cabecera ‖ valor relleno ‖ firma (96 B o 144 B en identidad) y lo
// parte en bloques de 16 B.
func (a Attribute) Record(sig []byte) ([][]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if len(sig) != SigLen {
		return nil, errors.New("la firma debe ser de 64 B")
	}
	rec := make([]byte, 0, HeaderLen+MaxValueLen(a.Code)+SigLen)
	rec = append(rec, a.Header()...)
	val := make([]byte, MaxValueLen(a.Code))
	copy(val, a.Value)
	rec = append(rec, val...)
	rec = append(rec, sig...)
	return SplitBlocks(rec), nil
}

// ParseRecord invierte Record: devuelve el atributo y la firma.
func ParseRecord(rec []byte) (Attribute, []byte, error) {
	var a Attribute
	if len(rec) < HeaderLen {
		return a, nil, errors.New("registro demasiado corto")
	}
	if rec[0] != Version {
		return a, nil, fmt.Errorf("versión no soportada: 0x%02x", rec[0])
	}
	a.Issuer, a.Code = rec[1], rec[2]
	n := int(rec[3])
	a.Expires = binary.BigEndian.Uint32(rec[4:8])
	copy(a.Handle[:], rec[8:16])
	if a.Code < AttrMayor18 || a.Code > AttrDonante {
		return a, nil, fmt.Errorf("atributo inválido: 0x%02x", a.Code)
	}
	max := MaxValueLen(a.Code)
	if n > max {
		return a, nil, errors.New("largo mayor al permitido")
	}
	if len(rec) < HeaderLen+max+SigLen {
		return a, nil, errors.New("registro demasiado corto")
	}
	a.Value = append([]byte(nil), rec[HeaderLen:HeaderLen+n]...)
	sig := append([]byte(nil), rec[HeaderLen+max:HeaderLen+max+SigLen]...)
	return a, sig, nil
}

// SplitBlocks parte b en bloques de 16 B (b debe ser múltiplo de 16).
func SplitBlocks(b []byte) [][]byte {
	out := make([][]byte, 0, len(b)/BlockLen)
	for i := 0; i+BlockLen <= len(b); i += BlockLen {
		out = append(out, append([]byte(nil), b[i:i+BlockLen]...))
	}
	return out
}

// Sectores del chip que ocupa cada atributo (el sector 1 es el índice, el 15 reserva).
var sectorsByAttr = map[byte][]int{
	AttrMayor18:    {2, 3},
	AttrMayor21:    {4, 5},
	AttrIdentidad:  {6, 7, 8},
	AttrSangre:     {9, 10},
	AttrEmergencia: {11, 12},
	AttrDonante:    {13, 14},
}

// SectorsFor devuelve los sectores donde se escribe el atributo.
func SectorsFor(code byte) []int { return sectorsByAttr[code] }

// IndexBlocks arma el sector 1: version ‖ handle(8) relleno a 16 B, sin firma.
// Los otros dos bloques de datos van en cero.
func IndexBlocks(handle [8]byte) [][]byte {
	b0 := make([]byte, BlockLen)
	b0[0] = Version
	copy(b0[1:9], handle[:])
	return [][]byte{b0, make([]byte, BlockLen), make([]byte, BlockLen)}
}

// SectorBlocks agrupa los bloques de un registro en sectores de 3 bloques de datos.
func SectorBlocks(code byte, blocks [][]byte) (map[int][][]byte, error) {
	secs := SectorsFor(code)
	if len(secs)*3 != len(blocks) {
		return nil, fmt.Errorf("se esperaban %d bloques y hay %d", len(secs)*3, len(blocks))
	}
	out := map[int][][]byte{}
	for i, s := range secs {
		out[s] = blocks[i*3 : i*3+3]
	}
	return out, nil
}

// --- Credencial de billetera ---

// WalletMessage es cabecera ‖ valor[0:largo] ‖ clave pública del celular (33 B), sin UID.
func (a Attribute) WalletMessage(phonePub []byte) ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if len(phonePub) != PubLen {
		return nil, errors.New("la clave pública del celular debe ser de 33 B comprimida")
	}
	m := append([]byte(nil), a.Header()...)
	m = append(m, a.Value...)
	return append(m, phonePub...), nil
}

func (a Attribute) SignWallet(priv *ecdsa.PrivateKey, phonePub []byte) ([]byte, error) {
	m, err := a.WalletMessage(phonePub)
	if err != nil {
		return nil, err
	}
	return SignRaw(priv, m)
}

func (a Attribute) VerifyWallet(pub *ecdsa.PublicKey, phonePub, sig []byte) bool {
	m, err := a.WalletMessage(phonePub)
	if err != nil {
		return false
	}
	return VerifyRaw(pub, m, sig)
}

// WalletBlob es cabecera ‖ valor[0:largo] ‖ pubkey33 ‖ firma64.
func (a Attribute) WalletBlob(phonePub, sig []byte) ([]byte, error) {
	m, err := a.WalletMessage(phonePub)
	if err != nil {
		return nil, err
	}
	if len(sig) != SigLen {
		return nil, errors.New("la firma debe ser de 64 B")
	}
	return append(m, sig...), nil
}

// ParseWalletBlob invierte WalletBlob.
func ParseWalletBlob(blob []byte) (a Attribute, phonePub, sig []byte, err error) {
	if len(blob) < HeaderLen+PubLen+SigLen {
		return a, nil, nil, errors.New("blob demasiado corto")
	}
	if blob[0] != Version {
		return a, nil, nil, fmt.Errorf("versión no soportada: 0x%02x", blob[0])
	}
	n := int(blob[3])
	if len(blob) != HeaderLen+n+PubLen+SigLen {
		return a, nil, nil, errors.New("largo del blob no coincide con la cabecera")
	}
	a.Issuer, a.Code = blob[1], blob[2]
	a.Expires = binary.BigEndian.Uint32(blob[4:8])
	copy(a.Handle[:], blob[8:16])
	a.Value = append([]byte(nil), blob[HeaderLen:HeaderLen+n]...)
	if err = a.Validate(); err != nil {
		return a, nil, nil, err
	}
	phonePub = blob[HeaderLen+n : HeaderLen+n+PubLen]
	sig = blob[HeaderLen+n+PubLen:]
	return a, phonePub, sig, nil
}

// --- Primitivas ---

// SignRaw firma SHA-256(msg) y devuelve r‖s con cada mitad rellena a 32 B.
func SignRaw(priv *ecdsa.PrivateKey, msg []byte) ([]byte, error) {
	h := sha256.Sum256(msg)
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	if err != nil {
		return nil, err
	}
	sig := make([]byte, SigLen)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig, nil
}

// VerifyRaw verifica una firma r‖s de 64 B sobre SHA-256(msg).
func VerifyRaw(pub *ecdsa.PublicKey, msg, sig []byte) bool {
	if len(sig) != SigLen {
		return false
	}
	h := sha256.Sum256(msg)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	return ecdsa.Verify(pub, h[:], r, s)
}

// CompressPub devuelve (0x02|0x03) ‖ X: 33 B.
func CompressPub(pub *ecdsa.PublicKey) []byte {
	out := make([]byte, PubLen)
	out[0] = 0x02 | byte(pub.Y.Bit(0))
	pub.X.FillBytes(out[1:])
	return out
}

// DecompressPub invierte CompressPub.
func DecompressPub(b []byte) (*ecdsa.PublicKey, error) {
	if len(b) != PubLen || (b[0] != 0x02 && b[0] != 0x03) {
		return nil, errors.New("clave comprimida inválida")
	}
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), b)
	if x == nil {
		return nil, errors.New("punto fuera de la curva")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// RawXY devuelve X‖Y (64 B, sin prefijo 0x04): formato de micro-ecc.
func RawXY(pub *ecdsa.PublicKey) []byte {
	out := make([]byte, 64)
	pub.X.FillBytes(out[:32])
	pub.Y.FillBytes(out[32:])
	return out
}
