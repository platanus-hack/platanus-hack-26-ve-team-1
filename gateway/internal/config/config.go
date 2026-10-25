// Package config carga la configuración del gateway: gateway/config.local.json, fuera
// de git. Los tokens de los terminales pueden venir del secrets/demo.json que genera
// p1-backend (go -C api run ./cmd/admin demo), así no se copian a otro archivo.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// APIPorDefecto es la API desplegada de p1-backend.
const APIPorDefecto = "https://hackathonplatanusvenezuela2026.onrender.com"

type Config struct {
	API        string            `json:"api"`
	Puerto     string            `json:"puerto"`      // COM del ESP32 (COM10 según docs/p2-roadmap.md)
	Baudios    int               `json:"baudios"`     // 115200 (enlace 1)
	Terminal   string            `json:"terminal"`    // terminal activo al arrancar, p. ej. "Bar La Esquina"
	TokensDemo string            `json:"tokens_demo"` // ruta a secrets/demo.json de P1
	Tokens     map[string]string `json:"tokens"`      // nombre del terminal → token; reemplaza a los de tokens_demo
	Datos      string            `json:"datos"`       // cola de eventos y copias de la última config y lista de revocadas
	Audio      string            `json:"audio"`       // audios de voz del roadmap (opcional)
	Logs       string            `json:"logs"`
}

// Cargar lee el archivo; las rutas relativas son relativas a la carpeta del archivo.
func Cargar(ruta string) (Config, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return Config{}, fmt.Errorf("no se pudo leer %s (copia config.example.json a config.local.json): %w", ruta, err)
	}
	c := Config{API: APIPorDefecto, Puerto: "COM10", Baudios: 115200, Datos: "data", Audio: "audio", Logs: "logs"}
	if err := json.Unmarshal(datos, &c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", ruta, err)
	}

	base := filepath.Dir(ruta)
	rel := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	c.Datos, c.Audio, c.Logs = rel(c.Datos), rel(c.Audio), rel(c.Logs)

	tokens := map[string]string{}
	if c.TokensDemo != "" {
		demo, err := os.ReadFile(rel(c.TokensDemo))
		if err != nil {
			return Config{}, fmt.Errorf("tokens_demo: %w", err)
		}
		var d struct {
			Tokens map[string]string `json:"tokens"`
		}
		if err := json.Unmarshal(demo, &d); err != nil {
			return Config{}, fmt.Errorf("tokens_demo: %w", err)
		}
		for nombre, token := range d.Tokens {
			tokens[nombre] = token
		}
	}
	for nombre, token := range c.Tokens {
		tokens[nombre] = token
	}
	c.Tokens = tokens

	if len(c.Tokens) == 0 {
		return Config{}, errors.New("no hay tokens de terminal: define tokens o tokens_demo")
	}
	if c.Terminal == "" && len(c.Tokens) == 1 {
		c.Terminal = c.Nombres()[0]
	}
	if _, ok := c.Tokens[c.Terminal]; !ok {
		return Config{}, fmt.Errorf("el terminal %q no tiene token; hay: %s", c.Terminal, strings.Join(c.Nombres(), ", "))
	}
	return c, nil
}

// Nombres devuelve los terminales con token, en orden alfabético.
func (c Config) Nombres() []string {
	out := make([]string, 0, len(c.Tokens))
	for nombre := range c.Tokens {
		out = append(out, nombre)
	}
	sort.Strings(out)
	return out
}
