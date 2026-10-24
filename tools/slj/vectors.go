package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type vCase struct {
	Name    string   `json:"name"`
	QR      string   `json:"qr"`
	Now     int64    `json:"now"`
	Expect  int      `json:"expect"`
	Revoked []string `json:"revoked,omitempty"`
	Accept  []int    `json:"accept,omitempty"`
}

type vFile struct {
	Note    string            `json:"note"`
	Issuers map[string]string `json:"issuers"`
	Cases   []vCase           `json:"cases"`
}

const vNow int64 = 1_800_000_000

func buildVectors() (*vFile, error) {
	devPub := compress(&testDevice.PublicKey)
	exp := uint32(vNow + 86400)
	issue := func(issuer, attr byte, exp uint32) []byte {
		c, err := issueWallet(issuer, attr, exp, testHandle, devPub, []byte{1})
		if err != nil {
			panic(err)
		}
		return c
	}
	pres := func(cred []byte, t int64) string {
		q, err := presentWith(testDevice, cred, uint32(t), cred[2])
		if err != nil {
			panic(err)
		}
		return q
	}
	mutate := func(qr string, f func(b []byte)) string {
		b, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(qr, prefix))
		f(b)
		return prefix + base64.RawURLEncoding.EncodeToString(b)
	}
	good := pres(issue(1, 1, exp), vNow)
	hexHandle := fmt.Sprintf("%016x", testHandle)
	cs := []vCase{
		{Name: "valido", QR: good, Expect: CodeOK},
		{Name: "valido_universidad_carnet", QR: pres(issue(2, 7, exp), vNow), Expect: CodeOK},
		{Name: "valido_limite_ventana", QR: pres(issue(1, 1, exp), vNow-30), Expect: CodeOK},
		{Name: "emisor_alterado_valor", QR: mutate(good, func(b []byte) { b[headerLen+devPubLen] ^= 1 }), Expect: CodeBadSig},
		{Name: "firma_celular_alterada", QR: mutate(good, func(b []byte) { b[len(b)-10] ^= 1 }), Expect: CodeBadSig},
		{Name: "vencido", QR: pres(issue(1, 1, uint32(vNow-1)), vNow), Expect: CodeExpired},
		{Name: "revocado", QR: good, Expect: CodeRevoked, Revoked: []string{hexHandle}},
		{Name: "hora_pasada", QR: pres(issue(1, 1, exp), vNow-40), Expect: CodeStale},
		{Name: "hora_futura", QR: pres(issue(1, 1, exp), vNow+40), Expect: CodeStale},
		{Name: "attr_inconsistente", QR: mutate(good, func(b []byte) { b[len(b)-1] ^= 1 }), Expect: CodeBadFmt},
		{Name: "sin_permiso", QR: good, Expect: CodeNoPerm, Accept: []int{2}},
		{Name: "emisor_desconocido", QR: pres(mustIssueUnknown(devPub, exp), vNow), Expect: CodeBadFmt},
		{Name: "prefijo_malo", QR: "SLJ2:" + strings.TrimPrefix(good, prefix), Expect: CodeBadFmt},
		{Name: "truncado", QR: good[:len(good)-6], Expect: CodeBadFmt},
	}
	for i := range cs {
		cs[i].Now = vNow
	}
	return &vFile{
		Note:    "GENERADO por `go run ./tools/slj vectors`. Claves SOLO DE PRUEBA (TEST ONLY); no son las del emisor real.",
		Issuers: issuerPubHex(),
		Cases:   cs,
	}, nil
}

// mustIssueUnknown firma con el emisor 1 pero declara el emisor 9 (no registrado).
func mustIssueUnknown(devPub []byte, exp uint32) []byte {
	u := encodeUnsigned(verWallet, 9, 1, exp, testHandle, devPub, []byte{1})
	sig, err := signRaw(testIssuer[1], u)
	if err != nil {
		panic(err)
	}
	return append(u, sig...)
}

func writeVectors(path string) error {
	v, err := buildVectors()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
