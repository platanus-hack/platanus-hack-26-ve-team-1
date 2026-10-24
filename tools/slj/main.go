// slj: herramienta de P4 para generar y verificar credenciales de billetera y QR SLJ1.
//
//	slj vectors [-out ruta]        genera los vectores de conformidad para los tests de TS
//	slj issue   [flags]            firma una credencial de billetera de prueba (hex)
//	slj present [flags]            imprime un QR SLJ1 fresco del dispositivo de prueba
//	slj verify  [flags] <SLJ1:..>  decodifica y verifica; sale 0 solo si el código es 01
package main

import (
	"crypto/ecdsa"
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
	out := fs.String("out", "app/lib/slj/testdata/vectors.json", "archivo de salida")
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
	keys := fs.String("keys", "", "JSON {\"1\":\"hex65\",...}; por defecto, las claves de prueba")
	rev := fs.String("rev", "", "archivo de revocaciones (n × 8 B)")
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
		for o := 0; o+8 <= len(b); o += 8 {
			revoked[binary.BigEndian.Uint64(b[o:])] = true
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
	raw := map[string]string{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return parseIssuerMap(raw)
}

func parseIssuerMap(raw map[string]string) (map[byte]*ecdsa.PublicKey, error) {
	m := map[byte]*ecdsa.PublicKey{}
	for id, h := range raw {
		var n int
		if _, err := fmt.Sscan(id, &n); err != nil {
			return nil, err
		}
		pb, err := hex.DecodeString(h)
		if err != nil {
			return nil, err
		}
		k, err := pubFromAny(pb)
		if err != nil {
			return nil, fmt.Errorf("emisor %s: %w", id, err)
		}
		m[byte(n)] = k
	}
	return m, nil
}
