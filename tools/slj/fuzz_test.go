package main

import (
	"fmt"
	"testing"
)

// content identifica lo que firman el emisor y el celular (credencial sin firma, t y attr), no los bytes de las
// firmas: ECDSA es aleatorio y cada proceso del fuzzer regenera los vectores con firmas distintas.
func content(qr string) (string, bool) {
	p, err := parsePresentation(qr)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%x|%d|%d", p.Cred.Signed, p.T, p.Attr), true
}

// Ninguna entrada puede hacer entrar en pánico al verificador, y solo se aprueba el contenido de los vectores válidos.
func FuzzVerifyPresentation(f *testing.F) {
	v, _ := buildVectors()
	valid := map[string]bool{}
	for _, c := range v.Cases {
		f.Add(c.QR)
		if k, ok := content(c.QR); ok && c.Expect == CodeOK {
			valid[k] = true
		}
	}
	f.Add("")
	f.Add("SLJ1:")
	ctx := VerifyCtx{Issuers: issuerPubs(), Revoked: map[uint64]bool{}, Now: vNow}
	f.Fuzz(func(t *testing.T, qr string) {
		if verifyPresentation(qr, ctx).Code != CodeOK {
			return
		}
		k, ok := content(qr)
		if !ok || !valid[k] {
			t.Fatalf("aprobó contenido que no es de ningún vector válido: %q", qr)
		}
	})
}
