package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
)

// Conformidad con el contrato REAL de P1: sus credenciales de billetera de prueba, sus emisores (JWK) y su
// celular de prueba (clave privada publicada en su repo). Fixtures copiados a app/lib/slj/testdata/p1-*.json.
const fixtures = "../../app/lib/slj/testdata/"

type p1Creds struct {
	Credentials []struct {
		Attribute int    `json:"attribute"`
		BlobHex   string `json:"blob_hex"`
	} `json:"credentials"`
	HandleHex string `json:"handle_hex"`
	Phone     struct {
		X, Y, D string
	} `json:"phone_private_jwk"`
	PhonePubHex string `json:"phone_pub_hex"`
}

func loadP1(t *testing.T) (p1Creds, map[byte]*ecdsa.PublicKey, *ecdsa.PrivateKey) {
	t.Helper()
	var w p1Creds
	b, err := os.ReadFile(fixtures + "p1-credenciales-billetera.json")
	if err != nil {
		t.Skip("faltan los fixtures de P1")
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	kb, _ := os.ReadFile(fixtures + "p1-issuers.json")
	issuers, err := parseIssuersJSON(kb)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := base64.RawURLEncoding.DecodeString(w.Phone.D)
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		t.Fatal(err)
	}
	return w, issuers, priv
}

func TestP1CredentialsVerify(t *testing.T) {
	w, issuers, phone := loadP1(t)
	if hex.EncodeToString(compress(&phone.PublicKey)) != w.PhonePubHex {
		t.Fatal("la clave privada de P1 no corresponde a su clave pública publicada")
	}
	if len(issuers) != 2 {
		t.Fatalf("emisores de P1: %d, esperaba 2 (0x01 y 0x02)", len(issuers))
	}
	const now = 1_800_000_000
	h, _ := hex.DecodeString(w.HandleHex)
	for _, c := range w.Credentials {
		blob, _ := hex.DecodeString(c.BlobHex)
		cred, err := parseCred(blob)
		if err != nil || int(cred.Attr) != c.Attribute || cred.Handle != binary.BigEndian.Uint64(h) {
			t.Fatalf("A%d: %v %+v", c.Attribute, err, cred)
		}
		qr, err := presentWith(phone, blob, now, cred.Attr)
		if err != nil {
			t.Fatal(err)
		}
		ctx := VerifyCtx{Issuers: issuers, Revoked: map[uint64]bool{}, Now: now}
		if got := verifyPresentation(qr, ctx).Code; got != CodeOK {
			t.Errorf("A%d: código %d, esperaba 01", c.Attribute, got)
		}
		ctx.Revoked[cred.Handle] = true
		if got := verifyPresentation(qr, ctx).Code; got != CodeRevoked {
			t.Errorf("A%d revocada: código %d, esperaba 03", c.Attribute, got)
		}
		// valor alterado → firma inválida
		bad := append([]byte{}, blob...)
		bad[headerLen] ^= 1
		qr2, _ := presentWith(phone, bad, now, cred.Attr)
		ctx.Revoked = map[uint64]bool{}
		if got := verifyPresentation(qr2, ctx).Code; got != CodeBadSig {
			t.Errorf("A%d alterada: código %d, esperaba 02", c.Attribute, got)
		}
	}
}

func TestIssuersJSONAcceptsHexAndJWK(t *testing.T) {
	good, _ := testIssuer[1].PublicKey.Bytes()
	m, err := parseIssuersJSON([]byte(`{"1":"` + hex.EncodeToString(good) + `"}`))
	if err != nil || !m[1].Equal(&testIssuer[1].PublicKey) {
		t.Fatalf("hex: %v", err)
	}
	x := base64.RawURLEncoding.EncodeToString(good[1:33])
	y := base64.RawURLEncoding.EncodeToString(good[33:])
	m, err = parseIssuersJSON([]byte(`{"0x01":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}}`))
	if err != nil || !m[1].Equal(&testIssuer[1].PublicKey) {
		t.Fatalf("JWK: %v", err)
	}
	for _, bad := range []string{`{"1":{"x":"AA","y":"AA"}}`, `{"x":"00"}`, `{"1":"zz"}`, `no es json`} {
		if _, err := parseIssuersJSON([]byte(bad)); err == nil {
			t.Errorf("%s debió fallar", bad)
		}
	}
}

func TestLoadRevokedBinaryAndJSON(t *testing.T) {
	bin := binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(nil, 7), 9)
	got := map[uint64]bool{}
	if err := loadRevoked(bin, got); err != nil || !got[7] || !got[9] || len(got) != 2 {
		t.Fatalf("binario: %v %v", err, got)
	}
	got = map[uint64]bool{}
	if err := loadRevoked([]byte(`{"hs":["9e429475799fc8ed","0000000000000001"]}`), got); err != nil || !got[0x9e429475799fc8ed] || !got[1] {
		t.Fatalf("json: %v %v", err, got)
	}
	for _, bad := range []string{`{"hs":["zz"]}`, `{"hs":["00"]}`, `{"hs":`} {
		if err := loadRevoked([]byte(strings.TrimSpace(bad)), map[uint64]bool{}); err == nil {
			t.Errorf("%s debió fallar", bad)
		}
	}
	_ = big.NewInt
}
