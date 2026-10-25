// Package repoutil ubica la raíz del repositorio para las herramientas de línea de comandos.
package repoutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Root sube desde el directorio actual hasta encontrar platanus-hack-project.jsonc.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "platanus-hack-project.jsonc")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no se encontró la raíz del repo (platanus-hack-project.jsonc)")
		}
		dir = parent
	}
}

// DatabaseURL devuelve DATABASE_URL del entorno o, si falta, la de .env.local.
// En .env.local acepta la línea DATABASE_URL=... (con o sin comillas) o la URL sola.
func DatabaseURL() (string, error) {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u, nil
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(root, ".env.local"))
	if err != nil {
		return "", errors.New("DATABASE_URL no está definido y no hay .env.local")
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "DATABASE_URL="); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`), nil
		}
		if strings.HasPrefix(line, "postgres://") || strings.HasPrefix(line, "postgresql://") {
			return line, nil
		}
	}
	return "", errors.New("no hay DATABASE_URL en .env.local")
}
