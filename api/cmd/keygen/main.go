// keygen genera los dos pares P-256 de los emisores simulados.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
)

func main() {
	force := flag.Bool("force", false, "sobrescribe las claves existentes")
	flag.Parse()

	root, err := repoutil.Root()
	if err != nil {
		log.Fatal(err)
	}
	secrets := filepath.Join(root, "secrets")
	keysDir := filepath.Join(root, "keys")
	issuers := []struct {
		name string
		code string
		c    string
	}{
		{"nacional", "0x01", "ISSUER_NACIONAL_PUB"},
		{"universidad", "0x02", "ISSUER_UNIVERSIDAD_PUB"},
	}

	if !*force {
		for _, is := range issuers {
			p := filepath.Join(secrets, is.name+".key.b64")
			if _, err := os.Stat(p); err == nil {
				log.Fatalf("%s ya existe; usa -force para sobrescribir", p)
			}
		}
	}
	for _, d := range []string{secrets, keysDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			log.Fatal(err)
		}
	}

	jwks := map[string]sign.JWK{}
	var h strings.Builder
	h.WriteString("// Generado por cmd/keygen. Claves públicas P-256 como X||Y (64 B, sin prefijo 0x04), formato de micro-ecc.\n#pragma once\n#include <stdint.h>\n\n")
	for _, is := range issuers {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			log.Fatal(err)
		}
		b64, err := sign.EncodePrivateB64(priv)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(secrets, is.name+".key.b64"), []byte(b64+"\n"), 0o600); err != nil {
			log.Fatal(err)
		}
		jwks[is.code] = sign.PublicJWK(&priv.PublicKey)

		xy := sign.RawXY(&priv.PublicKey)
		fmt.Fprintf(&h, "const uint8_t %s[64] = {\n", is.c)
		for i := 0; i < 64; i += 8 {
			h.WriteString("  ")
			for j := i; j < i+8; j++ {
				fmt.Fprintf(&h, "0x%02X,", xy[j])
				if j < i+7 {
					h.WriteString(" ")
				}
			}
			h.WriteString("\n")
		}
		h.WriteString("};\n\n")
		fmt.Printf("%s: clave generada (privada en secrets/%s.key.b64)\n", is.name, is.name)
	}
	js, _ := json.MarshalIndent(jwks, "", "  ")
	if err := os.WriteFile(filepath.Join(keysDir, "issuers.json"), append(js, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keysDir, "issuers.h"), []byte(h.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("keys/issuers.json y keys/issuers.h escritos")
}
