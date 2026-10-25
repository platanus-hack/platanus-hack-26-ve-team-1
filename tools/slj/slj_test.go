package main

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

// Los vectores los genera el propio CLI y los consume la suite de TS (conformidad cruzada).
func TestVectorsRoundTrip(t *testing.T) {
	v, err := buildVectors()
	if err != nil {
		t.Fatal(err)
	}
	issuers, err := parseIssuerMap(v.Issuers)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range v.Cases {
		ctx := VerifyCtx{Issuers: issuers, Revoked: map[uint64]bool{}, Now: c.Now}
		for _, h := range c.Revoked {
			n, _ := strconv.ParseUint(h, 16, 64)
			ctx.Revoked[n] = true
		}
		if len(c.Accept) > 0 {
			ctx.Accept = map[byte]bool{}
			for _, a := range c.Accept {
				ctx.Accept[byte(a)] = true
			}
		}
		if got := verifyPresentation(c.QR, ctx).Code; got != c.Expect {
			t.Errorf("%s: código %d, esperado %d", c.Name, got, c.Expect)
		}
	}
}

// Presentación firmada por WebCrypto (la escribe `node --test web/tests/slj/conformance.test.ts`).
func TestVerifyTSPresentation(t *testing.T) {
	b, err := os.ReadFile("../../web/tests/slj/testdata/ts-presentation.json")
	if err != nil {
		t.Skip("falta ts-presentation.json; corre los tests de TS primero")
	}
	var f struct {
		Now     int64             `json:"now"`
		QR      string            `json:"qr"`
		Issuers map[string]string `json:"issuers"`
		Expect  int               `json:"expect"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	issuers, err := parseIssuerMap(f.Issuers)
	if err != nil {
		t.Fatal(err)
	}
	if got := verifyPresentation(f.QR, VerifyCtx{Issuers: issuers, Now: f.Now}).Code; got != f.Expect {
		t.Fatalf("WebCrypto→Go: código %d, esperado %d", got, f.Expect)
	}
}
