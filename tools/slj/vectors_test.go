package main

import (
	"encoding/json"
	"os"
	"testing"
)

const vectorsPath = "../../web/tests/slj/testdata/vectors.json"

// El archivo versionado no puede quedar viejo respecto al generador: mismas claves, mismos casos y códigos.
// (Las firmas ECDSA son aleatorias, así que se compara la estructura y no los bytes.)
func TestVectorsFileMatchesGenerator(t *testing.T) {
	b, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Skip("falta vectors.json; corre `go run . vectors`")
	}
	var file vFile
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	gen, err := buildVectors()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) != len(gen.Cases) {
		t.Fatalf("vectors.json tiene %d casos y el generador %d: regenera con `go run . vectors`", len(file.Cases), len(gen.Cases))
	}
	for i, c := range gen.Cases {
		f := file.Cases[i]
		if f.Name != c.Name || f.Expect != c.Expect || f.Now != c.Now {
			t.Errorf("caso %d: archivo %s/%d, generador %s/%d", i, f.Name, f.Expect, c.Name, c.Expect)
		}
	}
	for id, pub := range gen.Issuers {
		if file.Issuers[id] != pub {
			t.Errorf("la clave del emisor %s cambió: regenera los vectores", id)
		}
	}
}

func TestVectorsCoverEveryVerifyCode(t *testing.T) {
	gen, _ := buildVectors()
	seen := map[int]bool{}
	for _, c := range gen.Cases {
		seen[c.Expect] = true
	}
	for _, code := range []int{CodeOK, CodeBadSig, CodeRevoked, CodeExpired, CodeNoPerm, CodeStale, CodeBadFmt} {
		if !seen[code] {
			t.Errorf("ningún vector cubre el código %d", code)
		}
	}
}
