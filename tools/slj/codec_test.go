package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"testing"
)

var testDevPub = compress(&testDevice.PublicKey)

func wallet(t testing.TB, value []byte) []byte {
	t.Helper()
	c, err := issueWallet(1, 1, 2_000_000_000, testHandle, testDevPub, value)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseCredWalletRoundTrip(t *testing.T) {
	raw := wallet(t, []byte{1})
	c, err := parseCred(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ver != ver || c.Issuer != 1 || c.Attr != 1 || c.Exp != 2_000_000_000 || c.Handle != testHandle {
		t.Fatalf("campos: %+v", c)
	}
	if !bytes.Equal(c.DevPub, testDevPub) || !bytes.Equal(c.Value, []byte{1}) || len(c.Sig) != sigLen {
		t.Fatal("devpub/valor/firma no coinciden")
	}
	if !bytes.Equal(append(append([]byte{}, c.Signed...), c.Sig...), raw) {
		t.Fatal("signed||sig debe reconstruir la credencial")
	}
}

func TestParseCredLengths(t *testing.T) {
	for _, n := range []int{0, 1, 64, 255} {
		raw := append(encodeUnsigned(1, 3, 1, 1, testDevPub, make([]byte, n)), make([]byte, sigLen)...)
		if _, err := parseCred(raw); err != nil {
			t.Errorf("len %d: %v", n, err)
		}
		if _, err := parseCred(append(raw, 0)); err == nil {
			t.Errorf("len %d +1 debió fallar", n)
		}
		if _, err := parseCred(raw[:len(raw)-1]); err == nil {
			t.Errorf("len %d -1 debió fallar", n)
		}
	}
	bad := wallet(t, []byte{1})
	for _, v := range []byte{0, 2, 3} {
		bad[0] = v
		if _, err := parseCred(bad); err == nil {
			t.Errorf("ver %d debió fallar", v)
		}
	}
	if _, err := parseCred(nil); err == nil {
		t.Error("vacío debió fallar")
	}
}

func TestParsePresentationRejects(t *testing.T) {
	qr, err := presentWith(testDevice, wallet(t, []byte{1}), 1_800_000_000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsePresentation(qr); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"", "SLJ1:", "SLJ2:" + qr[5:], qr[:len(qr)-4], qr + "AAAA", "SLJ1:%%%"} {
		if _, err := parsePresentation(s); err == nil {
			t.Errorf("%q debió fallar", s)
		}
	}
}

func TestCompressAndPubFromAny(t *testing.T) {
	raw, _ := testDevice.PublicKey.Bytes()
	for _, in := range [][]byte{raw, compress(&testDevice.PublicKey)} {
		k, err := pubFromAny(in)
		if err != nil || !k.Equal(&testDevice.PublicKey) {
			t.Fatalf("pubFromAny(%d B): %v", len(in), err)
		}
	}
	// ambas paridades de y
	seen := map[byte]bool{}
	for _, k := range testIssuer {
		seen[compress(&k.PublicKey)[0]] = true
	}
	seen[compress(&testDevice.PublicKey)[0]] = true
	for p := range seen {
		if p != 2 && p != 3 {
			t.Errorf("prefijo inválido %d", p)
		}
	}
	// punto que no está en la curva y largos erróneos
	off := make([]byte, 33)
	off[0] = 2
	for x := byte(1); x < 100; x++ {
		off[32] = x
		if x1, _ := elliptic.UnmarshalCompressed(elliptic.P256(), off); x1 == nil {
			break
		}
	}
	for _, in := range [][]byte{off, make([]byte, 10), make([]byte, 65), nil} {
		if _, err := pubFromAny(in); err == nil {
			t.Errorf("pubFromAny(%x) debió fallar", in)
		}
	}
}

func TestSignVerifyRaw(t *testing.T) {
	msg := []byte("hola")
	sig, err := signRaw(testDevice, msg)
	if err != nil || len(sig) != sigLen {
		t.Fatalf("firma: %v len=%d", err, len(sig))
	}
	if !verifyRaw(&testDevice.PublicKey, sig, msg) {
		t.Fatal("firma válida rechazada")
	}
	if verifyRaw(&testDevice.PublicKey, sig, []byte("adiós")) {
		t.Fatal("mensaje distinto aceptado")
	}
	if verifyRaw(&testDevice.PublicKey, sig[:63], msg) || verifyRaw(&testDevice.PublicKey, append(sig, 0), msg) {
		t.Fatal("largo incorrecto aceptado")
	}
	if verifyRaw(&testIssuer[1].PublicKey, sig, msg) {
		t.Fatal("clave distinta aceptada")
	}
}

func TestParseIssuerMapErrors(t *testing.T) {
	good, _ := testIssuer[1].PublicKey.Bytes()
	if m, err := parseIssuerMap(map[string]string{"1": hex.EncodeToString(good)}); err != nil || len(m) != 1 {
		t.Fatalf("válido: %v", err)
	}
	for name, in := range map[string]map[string]string{
		"id no numérico": {"x": hex.EncodeToString(good)},
		"hex malo":       {"1": "zz"},
		"punto inválido": {"1": hex.EncodeToString(make([]byte, 65))},
	} {
		if _, err := parseIssuerMap(in); err == nil {
			t.Errorf("%s debió fallar", name)
		}
	}
}

// Las claves de prueba son deterministas: si cambian, los vectores versionados dejarían de verificar.
func TestTestKeysAreStable(t *testing.T) {
	a, b := testKey("issuer-nacional"), testKey("issuer-nacional")
	if !a.Equal(b) {
		t.Fatal("la misma semilla debe dar la misma clave")
	}
	if a.Equal(testKey("issuer-universidad")) {
		t.Fatal("semillas distintas deben dar claves distintas")
	}
	var _ *ecdsa.PrivateKey = a
	if !testIssuer[1].Equal(a) {
		t.Fatal("testIssuer[1] debe derivarse de issuer-nacional")
	}
}

func BenchmarkVerifyPresentation(b *testing.B) {
	qr, _ := presentWith(testDevice, wallet(b, []byte{1}), 1_800_000_000, 1)
	ctx := VerifyCtx{Issuers: issuerPubs(), Revoked: map[uint64]bool{}, Now: 1_800_000_000}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if verifyPresentation(qr, ctx).Code != CodeOK {
			b.Fatal("debió aprobar")
		}
	}
}
