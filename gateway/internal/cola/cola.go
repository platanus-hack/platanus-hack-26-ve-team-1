// Package cola guarda en disco cada evento del lector antes de enviarlo a la API y lo
// reintenta hasta que la API lo acepte (docs/contrato-terminal.md, enlace 4). Si el
// gateway se cierra o se cae internet, los eventos siguen ahí al volver.
package cola

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Item es un evento pendiente con el terminal que lo generó: al cambiar de terminal en
// la demo, cada evento se envía con el token de quien lo leyó.
type Item struct {
	Terminal string          `json:"terminal"`
	Evento   json.RawMessage `json:"evento"`
	Creado   time.Time       `json:"creado"`
}

// Rechazado envuelve un error definitivo: el evento se aparta en rechazados/ y no se
// reintenta.
type Rechazado struct{ Err error }

func (r Rechazado) Error() string { return r.Err.Error() }
func (r Rechazado) Unwrap() error { return r.Err }

// Cola es una carpeta con un archivo por evento; el nombre ordena por hora de llegada.
type Cola struct {
	dir   string
	mu    sync.Mutex
	seq   uint64
	nuevo chan struct{}
}

func Abrir(dir string) (*Cola, error) {
	if err := os.MkdirAll(filepath.Join(dir, "rechazados"), 0o755); err != nil {
		return nil, err
	}
	return &Cola{dir: dir, nuevo: make(chan struct{}, 1)}, nil
}

// Agregar guarda el evento en disco (archivo temporal y rename, así nunca queda a medias).
func (c *Cola) Agregar(it Item) error {
	datos, err := json.Marshal(it)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.seq++
	nombre := fmt.Sprintf("%019d-%06d.json", time.Now().UnixNano(), c.seq%1000000)
	c.mu.Unlock()

	tmp := filepath.Join(c.dir, nombre+".tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(datos)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(c.dir, nombre))
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}

	select {
	case c.nuevo <- struct{}{}:
	default:
	}
	return nil
}

// Pendientes devuelve cuántos eventos faltan por enviar.
func (c *Cola) Pendientes() int {
	nombres, _ := c.archivos()
	return len(nombres)
}

func (c *Cola) archivos() ([]string, error) {
	entradas, err := os.ReadDir(c.dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entradas {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Vaciar envía los eventos en orden de llegada. Los rechazados se apartan; el primer
// error transitorio detiene el envío, para reintentar más tarde sin desordenar la cola.
func (c *Cola) Vaciar(enviar func(Item) error) (enviados, rechazados int, err error) {
	nombres, err := c.archivos()
	if err != nil {
		return 0, 0, err
	}
	for _, nombre := range nombres {
		ruta := filepath.Join(c.dir, nombre)
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return enviados, rechazados, err
		}
		var it Item
		if err := json.Unmarshal(datos, &it); err != nil {
			c.apartar(nombre)
			rechazados++
			continue
		}
		if err := enviar(it); err != nil {
			var r Rechazado
			if errors.As(err, &r) {
				c.apartar(nombre)
				rechazados++
				continue
			}
			return enviados, rechazados, err
		}
		if err := os.Remove(ruta); err != nil {
			return enviados, rechazados, err
		}
		enviados++
	}
	return enviados, rechazados, nil
}

func (c *Cola) apartar(nombre string) {
	os.Rename(filepath.Join(c.dir, nombre), filepath.Join(c.dir, "rechazados", nombre))
}

// Correr vacía la cola cada vez que llega un evento y, mientras la API no responda,
// reintenta con espera creciente de 2 s a 1 min.
func (c *Cola) Correr(ctx context.Context, enviar func(Item) error, avisar func(string, ...any)) {
	espera := 2 * time.Second
	for {
		enviados, rechazados, err := c.Vaciar(enviar)
		if enviados > 0 {
			avisar("Cola: %d evento(s) enviado(s) a la API", enviados)
		}
		if rechazados > 0 {
			avisar("Cola: la API rechazó %d evento(s); quedan en %s", rechazados, filepath.Join(c.dir, "rechazados"))
		}

		var reintento <-chan time.Time
		if err != nil {
			avisar("Cola: %d evento(s) sin enviar (%v); reintento en %s", c.Pendientes(), err, espera)
			reintento = time.After(espera)
			espera = min(espera*2, time.Minute)
		} else {
			espera = 2 * time.Second
		}

		select {
		case <-ctx.Done():
			return
		case <-c.nuevo:
		case <-reintento:
		}
	}
}
