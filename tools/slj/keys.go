package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// randReader evita el import de crypto/rand en cada sitio de firma.
type randReader struct{}

func (randReader) Read(p []byte) (int, error) { return rand.Read(p) }

// CLAVES SOLO DE PRUEBA (TEST ONLY): derivadas de semillas públicas y deterministas.
// Nunca son las claves reales de la autoridad; esas viven en el gestor de contraseñas de P1.
func testKey(seed string) *ecdsa.PrivateKey {
	h := sha256.Sum256([]byte("SLJ-TEST-ONLY:" + seed))
	k, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), h[:])
	if err != nil {
		panic(err)
	}
	return k
}

var (
	testIssuer = map[byte]*ecdsa.PrivateKey{1: testKey("issuer-nacional"), 2: testKey("issuer-universidad")}
	testDevice = testKey("device-ana")
)

const testHandle uint64 = 0x1122334455667788

func issuerPubs() map[byte]*ecdsa.PublicKey {
	m := map[byte]*ecdsa.PublicKey{}
	for id, k := range testIssuer {
		m[id] = &k.PublicKey
	}
	return m
}

func issuerPubHex() map[string]string {
	m := map[string]string{}
	for id, k := range testIssuer {
		raw, _ := k.PublicKey.Bytes()
		m[fmt.Sprint(id)] = hex.EncodeToString(raw)
	}
	return m
}

// issueWallet firma una credencial de billetera de prueba para devPub (33 B).
func issueWallet(issuer, attr byte, exp uint32, handle uint64, devPub, value []byte) ([]byte, error) {
	u := encodeUnsigned(verWallet, issuer, attr, exp, handle, devPub, value)
	sig, err := signRaw(testIssuer[issuer], u)
	if err != nil {
		return nil, err
	}
	return append(u, sig...), nil
}

func presentWith(priv *ecdsa.PrivateKey, cred []byte, t uint32, attr byte) (string, error) {
	sig, err := signRaw(priv, deviceMessage(t, cred))
	if err != nil {
		return "", err
	}
	return encodePresentation(cred, t, sig, attr), nil
}
