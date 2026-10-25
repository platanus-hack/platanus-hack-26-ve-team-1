// testvectors genera docs/test-vectors.json, docs/tarjetas-prueba.json y
// docs/credenciales-billetera-prueba.json con claves de los emisores en secrets/.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
)

type vector struct {
	Name        string `json:"name"`
	Issuer      string `json:"issuer"`
	MessageHex  string `json:"message_hex"`
	SigHex      string `json:"sig_hex"`
	ExpectValid bool   `json:"expect_valid"`
}

type card struct {
	UID     string              `json:"uid"`
	Issuer  string              `json:"issuer"`
	Sectors map[string][]string `json:"sectors"`
	Expect  map[string]bool     `json:"expect"`
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}

func writeJSON(path string, v any) {
	b := must(json.MarshalIndent(v, "", "  "))
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("escrito", path)
}

func hexs(bs [][]byte) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = hex.EncodeToString(b)
	}
	return out
}

func main() {
	uidHex := flag.String("uid", "A1B2C3D4", "UID de la tarjeta en hex")
	flag.Parse()
	uid := must(hex.DecodeString(strings.ReplaceAll(*uidHex, ":", "")))
	if n := len(uid); n != 4 && n != 7 && n != 10 {
		log.Fatalf("el UID debe tener 4, 7 o 10 bytes, no %d", n)
	}
	otherUID := append([]byte(nil), uid...)
	otherUID[len(otherUID)-1] ^= 0x01

	root := must(repoutil.Root())
	keys := map[string]*ecdsa.PrivateKey{
		"nacional":    must(sign.LoadPrivateFile(filepath.Join(root, "secrets", "nacional.key.b64"))),
		"universidad": must(sign.LoadPrivateFile(filepath.Join(root, "secrets", "universidad.key.b64"))),
	}
	code := map[string]byte{"nacional": sign.IssuerNacional, "universidad": sign.IssuerUniversidad}

	var handle [8]byte
	must(rand.Read(handle[:]))
	future := uint32(time.Now().AddDate(1, 0, 0).Unix())
	past := uint32(time.Now().AddDate(0, 0, -1).Unix())
	mk := func(issuer string, exp uint32) sign.Attribute {
		return sign.Attribute{Issuer: code[issuer], Code: sign.AttrMayor18, Expires: exp, Handle: handle, Value: []byte{0x01}}
	}

	// --- test-vectors.json ---
	var vecs []vector
	add := func(name, issuer string, msg, sig []byte, ok bool) {
		vecs = append(vecs, vector{name, issuer, hex.EncodeToString(msg), hex.EncodeToString(sig), ok})
	}
	a := mk("nacional", future)
	msg := must(a.Message(uid))
	sig := must(a.Sign(keys["nacional"], uid))
	add("valido", "nacional", msg, sig, true)

	bad := append([]byte(nil), msg...)
	bad[len(bad)-1] ^= 0x01
	add("valor_alterado", "nacional", bad, sig, false)

	add("uid_distinto", "nacional", must(a.Message(otherUID)), sig, false)

	u := mk("universidad", future)
	add("emisor_universidad", "universidad", must(u.Message(uid)), must(u.Sign(keys["universidad"], uid)), true)

	add("emisor_equivocado", "nacional", must(u.Message(uid)), must(u.Sign(keys["universidad"], uid)), false)

	id := sign.Attribute{Issuer: sign.IssuerNacional, Code: sign.AttrIdentidad, Expires: future, Handle: handle,
		Value: []byte("ANA PEREZ V-12345678 VIGENTE")}
	add("identidad_valida", "nacional", must(id.Message(uid)), must(id.Sign(keys["nacional"], uid)), true)

	out := map[string]any{"uid": strings.ToUpper(hex.EncodeToString(uid)), "vectors": vecs}
	writeJSON(filepath.Join(root, "docs", "test-vectors.json"), out)

	// --- tarjetas-prueba.json ---
	build := func(attr sign.Attribute, tamper bool, expect map[string]bool) card {
		s := must(attr.Sign(keys["nacional"], uid))
		if tamper {
			attr.Value = append([]byte(nil), attr.Value...)
			attr.Value[0] ^= 0x01 // después de firmar
		}
		blocks := must(attr.Record(s))
		secs := must(sign.SectorBlocks(attr.Code, blocks))
		c := card{UID: strings.ToUpper(hex.EncodeToString(uid)), Issuer: "nacional",
			Sectors: map[string][]string{"1": hexs(sign.IndexBlocks(handle))}, Expect: expect}
		for sec, bs := range secs {
			c.Sectors[fmt.Sprint(sec)] = hexs(bs)
		}
		return c
	}
	cards := map[string]card{
		"ana_valida":   build(mk("nacional", future), false, map[string]bool{"signature_valid": true, "expired": false}),
		"ana_alterada": build(mk("nacional", future), true, map[string]bool{"signature_valid": false, "expired": false}),
		"ana_vencida":  build(mk("nacional", past), false, map[string]bool{"signature_valid": true, "expired": true}),
	}
	writeJSON(filepath.Join(root, "docs", "tarjetas-prueba.json"), cards)

	// --- credenciales-billetera-prueba.json ---
	phone := must(ecdsa.GenerateKey(elliptic.P256(), rand.Reader))
	pub33 := sign.CompressPub(&phone.PublicKey)
	pj := sign.PublicJWK(&phone.PublicKey)
	d := phone.D.FillBytes(make([]byte, 32))
	type cred struct {
		Attribute  int    `json:"attribute"`
		Issuer     string `json:"issuer"`
		BlobHex    string `json:"blob_hex"`
		MessageHex string `json:"message_hex"`
		SigHex     string `json:"sig_hex"`
	}
	var creds []cred
	for _, c := range []byte{sign.AttrMayor18, sign.AttrMayor21} {
		at := sign.Attribute{Issuer: sign.IssuerNacional, Code: c, Expires: future, Handle: handle, Value: []byte{0x01}}
		m := must(at.WalletMessage(pub33))
		s := must(at.SignWallet(keys["nacional"], pub33))
		creds = append(creds, cred{int(c), "nacional", hex.EncodeToString(must(at.WalletBlob(pub33, s))),
			hex.EncodeToString(m), hex.EncodeToString(s)})
	}
	writeJSON(filepath.Join(root, "docs", "credenciales-billetera-prueba.json"), map[string]any{
		"aviso":         "SOLO PRUEBA: esta clave privada es pública en el repo; no usarla fuera de pruebas.",
		"phone_pub_hex": hex.EncodeToString(pub33),
		"phone_private_jwk": map[string]string{
			"SOLO PRUEBA": "true", "kty": pj.Kty, "crv": pj.Crv, "x": pj.X, "y": pj.Y,
			"d": base64URL(d),
		},
		"handle_hex":  hex.EncodeToString(handle[:]),
		"credentials": creds,
	})
}

func base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
