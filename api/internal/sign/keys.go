package sign

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// JWK es la forma pública de una clave P-256.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func PublicJWK(pub *ecdsa.PublicKey) JWK {
	xy := RawXY(pub)
	return JWK{
		Kty: "EC", Crv: "P-256",
		X: base64.RawURLEncoding.EncodeToString(xy[:32]),
		Y: base64.RawURLEncoding.EncodeToString(xy[32:]),
	}
}

func (j JWK) PublicKey() (*ecdsa.PublicKey, error) {
	if j.Kty != "EC" || j.Crv != "P-256" {
		return nil, errors.New("JWK no es EC P-256")
	}
	x, err := base64.RawURLEncoding.DecodeString(j.X)
	if err != nil {
		return nil, err
	}
	y, err := base64.RawURLEncoding.DecodeString(j.Y)
	if err != nil {
		return nil, err
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, errors.New("punto fuera de la curva")
	}
	return pub, nil
}

// EncodePrivateB64 serializa la clave privada como PKCS#8 DER en base64 estándar.
func EncodePrivateB64(priv *ecdsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// ParsePrivateB64 lee una clave PKCS#8 (o SEC1) DER en base64.
func ParsePrivateB64(s string) (*ecdsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok || ec.Curve != elliptic.P256() {
			return nil, errors.New("la clave no es ECDSA P-256")
		}
		return ec, nil
	}
	return x509.ParseECPrivateKey(der)
}

func LoadPrivateFile(path string) (*ecdsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePrivateB64(string(b))
}

// LoadIssuersJSON lee keys/issuers.json: {"0x01": JWK, "0x02": JWK}.
func LoadIssuersJSON(path string) (map[byte]*ecdsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]JWK
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := map[byte]*ecdsa.PublicKey{}
	for k, j := range raw {
		var v int
		if _, err := fmt.Sscanf(k, "0x%x", &v); err != nil {
			return nil, err
		}
		pub, err := j.PublicKey()
		if err != nil {
			return nil, err
		}
		out[byte(v)] = pub
	}
	return out, nil
}
