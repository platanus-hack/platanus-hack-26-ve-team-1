package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribir(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCargarTomaLosTokensDeDemoJSON(t *testing.T) {
	repo := t.TempDir()
	escribir(t, filepath.Join(repo, "secrets", "demo.json"),
		`{"password":"x","tokens":{"Bar La Esquina":"tok-bar","Registro":"tok-reg"}}`)
	ruta := filepath.Join(repo, "gateway", "config.local.json")
	escribir(t, ruta, `{"terminal":"Registro","tokens_demo":"../secrets/demo.json","tokens":{"Registro":"tok-propio"}}`)

	c, err := Cargar(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tokens["Bar La Esquina"] != "tok-bar" || c.Tokens["Registro"] != "tok-propio" {
		t.Errorf("tokens: %v", c.Tokens)
	}
	if c.Puerto != "COM10" || c.Baudios != 115200 || c.API != APIPorDefecto {
		t.Errorf("valores por defecto: %+v", c)
	}
	if c.Datos != filepath.Join(repo, "gateway", "data") {
		t.Errorf("datos relativo a la config: %s", c.Datos)
	}
}

func TestCargarRechazaUnTerminalSinToken(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.local.json")
	escribir(t, ruta, `{"terminal":"Clinica","tokens":{"Bar La Esquina":"tok-bar"}}`)
	if _, err := Cargar(ruta); err == nil || !strings.Contains(err.Error(), "Bar La Esquina") {
		t.Errorf("error esperado con la lista de terminales: %v", err)
	}
}
