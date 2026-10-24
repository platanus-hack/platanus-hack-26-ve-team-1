// slj: herramienta de P4 para generar y verificar credenciales de billetera y QR SLJ1.
//
//	slj vectors [-out ruta]        genera los vectores de conformidad para los tests de TS
//	slj issue   [flags]            firma una credencial de billetera de prueba (hex)
//	slj present [flags]            imprime un QR SLJ1 fresco del dispositivo de prueba
//	slj verify  [flags] <SLJ1:..>  decodifica y verifica; sale 0 solo si el código es 01
package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "vectors":
		err = cmdVectors(os.Args[2:])
	case "issue":
		err = cmdIssue(os.Args[2:])
	case "present":
		err = cmdPresent(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "uso: slj vectors|issue|present|verify [flags]")
	os.Exit(2)
}

func cmdVectors(args []string) error {
	fs := flag.NewFlagSet("vectors", flag.ExitOnError)
	out := fs.String("out", "web/tests/slj/testdata/vectors.json", "archivo de salida")
	fs.Parse(args)
	if err := writeVectors(*out); err != nil {
		return err
	}
	fmt.Println("escrito", *out)
	return nil
}

type walletFlags struct {
	issuer, attr          *int
	exp                   *int64
	handle, value, devpub *string
}

func addWalletFlags(fs *flag.FlagSet) walletFlags {
	return walletFlags{
		issuer: fs.Int("issuer", 1, "emisor (1 nacional, 2 universidad)"),
		attr:   fs.Int("attr", 1, "atributo (1 mayor18, 2 mayor21, 6 licencia, 7 carnet)"),
		exp:    fs.Int64("exp", time.Now().Add(365*24*time.Hour).Unix(), "vencimiento Unix"),
		handle: fs.String("handle", fmt.Sprintf("%016x", testHandle), "handle hex de 8 B"),
		value:  fs.String("value", "01", "valor hex"),
		devpub: fs.String("devpub", "", "clave pública del celular hex de 33 B (por defecto, la del dispositivo de prueba)"),
	}
}

func (w walletFlags) build() ([]byte, error) {
	if _, ok := testIssuer[byte(*w.issuer)]; !ok {
		return nil, fmt.Errorf("emisor de prueba %d no existe", *w.issuer)
	}
	h, err := hex.DecodeString(*w.handle)
	if err != nil || len(h) != 8 {
		return nil, fmt.Errorf("handle debe ser hex de 8 B")
	}
	val, err := hex.DecodeString(*w.value)
	if err != nil {
		return nil, err
	}
	dp := compress(&testDevice.PublicKey)
	if *w.devpub != "" {
		if dp, err = hex.DecodeString(*w.devpub); err != nil || len(dp) != devPubLen {
			return nil, fmt.Errorf("devpub debe ser hex de 33 B")
		}
	}
	return issueWallet(byte(*w.issuer), byte(*w.attr), uint32(*w.exp), binary.BigEndian.Uint64(h), dp, val)
}

func cmdIssue(args []string) error {
	fs := flag.NewFlagSet("issue", flag.ExitOnError)
	w := addWalletFlags(fs)
	fs.Parse(args)
	c, err := w.build()
	if err != nil {
		return err
	}
	fmt.Println(hex.EncodeToString(c))
	return nil
}

func cmdPresent(args []string) error {
	fs := flag.NewFlagSet("present", flag.ExitOnError)
	w := addWalletFlags(fs)
	showQR := fs.Bool("qr", false, "dibuja el QR en la terminal")
	fs.Parse(args)
	if *w.devpub != "" {
		return fmt.Errorf("present firma con el dispositivo de prueba; no admite -devpub")
	}
	c, err := w.build()
	if err != nil {
		return err
	}
	q, err := presentWith(testDevice, c, uint32(time.Now().Unix()), byte(*w.attr))
	if err != nil {
		return err
	}
	fmt.Println(q)
	if *showQR {
		return printQR(q)
	}
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	keys := fs.String("keys", "", "JSON de emisores: {\"1\":\"hex\"} o el keys/issuers.json de P1 ({\"0x01\":JWK}); por defecto, las claves de prueba")
	rev := fs.String("rev", "", "revocaciones: binario n × 8 B o JSON {\"hs\":[handle hex,...]} como /api/revocations")
	now := fs.Int64("now", time.Now().Unix(), "hora Unix para la verificación")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("falta el QR SLJ1:...")
	}
	issuers := issuerPubs()
	if *keys != "" {
		var err error
		if issuers, err = loadIssuers(*keys); err != nil {
			return err
		}
	}
	revoked := map[uint64]bool{}
	if *rev != "" {
		b, err := os.ReadFile(*rev)
		if err != nil {
			return err
		}
		if err := loadRevoked(b, revoked); err != nil {
			return err
		}
	}
	res := verifyPresentation(fs.Arg(0), VerifyCtx{Issuers: issuers, Revoked: revoked, Now: *now})
	if c := res.Cred; c != nil {
		fmt.Printf("emisor=%d attr=%d handle=%016x exp=%d t=%d (Δ%+ds)\n", c.Issuer, c.Attr, c.Handle, c.Exp, res.T, int64(res.T)-*now)
	}
	fmt.Printf("código: %02d\n", res.Code)
	if res.Code != CodeOK {
		os.Exit(1)
	}
	return nil
}

func loadIssuers(path string) (map[byte]*ecdsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseIssuersJSON(b)
}

// loadRevoked acepta el blob binario (n × 8 B) o el JSON {"hs":[...]} que entrega GET /api/revocations.
func loadRevoked(b []byte, into map[uint64]bool) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '{' {
		var j struct {
			Hs []string `json:"hs"`
		}
		if err := json.Unmarshal(t, &j); err != nil {
			return err
		}
		for _, h := range j.Hs {
			raw, err := hex.DecodeString(h)
			if err != nil || len(raw) != 8 {
				return fmt.Errorf("handle inválido en la lista: %q", h)
			}
			into[binary.BigEndian.Uint64(raw)] = true
		}
		return nil
	}
	for o := 0; o+8 <= len(b); o += 8 {
		into[binary.BigEndian.Uint64(b[o:])] = true
	}
	return nil
}

// parseIssuersJSON acepta {"1":"hex"} y el keys/issuers.json de P1: {"0x01":{kty,crv,x,y}}.
func parseIssuersJSON(b []byte) (map[byte]*ecdsa.PublicKey, error) {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	m := map[byte]*ecdsa.PublicKey{}
	for id, v := range raw {
		var n int
		if _, err := fmt.Sscan(id, &n); err != nil {
			return nil, err
		}
		var k *ecdsa.PublicKey
		var err error
		var h string
		if json.Unmarshal(v, &h) == nil {
			var pb []byte
			if pb, err = hex.DecodeString(h); err == nil {
				k, err = pubFromAny(pb)
			}
		} else {
			var j struct{ X, Y string }
			if err = json.Unmarshal(v, &j); err == nil {
				k, err = pubFromJWK(j.X, j.Y)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("emisor %s: %w", id, err)
		}
		m[byte(n)] = k
	}
	return m, nil
}

func pubFromJWK(x, y string) (*ecdsa.PublicKey, error) {
	xb, err1 := base64.RawURLEncoding.DecodeString(x)
	yb, err2 := base64.RawURLEncoding.DecodeString(y)
	if err1 != nil || err2 != nil || len(xb) != 32 || len(yb) != 32 {
		return nil, fmt.Errorf("JWK inválida")
	}
	return pubFromAny(append([]byte{4}, append(xb, yb...)...))
}

func parseIssuerMap(raw map[string]string) (map[byte]*ecdsa.PublicKey, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return parseIssuersJSON(b)
}
