// Package repoutil ubica la raíz del repositorio para las herramientas de línea de comandos.
package repoutil

import (
	"errors"
	"os"
	"path/filepath"
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
