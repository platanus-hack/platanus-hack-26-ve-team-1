package cola

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func agregar(t *testing.T, c *Cola, eventos ...string) {
	t.Helper()
	for _, e := range eventos {
		if err := c.Agregar(Item{Terminal: "Bar La Esquina", Evento: json.RawMessage(e)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVaciarEnOrdenYApartaRechazados(t *testing.T) {
	dir := t.TempDir()
	c, _ := Abrir(dir)
	agregar(t, c, `{"n":1}`, `{"n":2}`, `{"n":3}`)

	var vistos []string
	enviados, rechazados, err := c.Vaciar(func(it Item) error {
		vistos = append(vistos, string(it.Evento))
		if string(it.Evento) == `{"n":2}` {
			return Rechazado{Err: errors.New("400")}
		}
		return nil
	})
	if err != nil || enviados != 2 || rechazados != 1 {
		t.Fatalf("enviados=%d rechazados=%d err=%v", enviados, rechazados, err)
	}
	if len(vistos) != 3 || vistos[0] != `{"n":1}` || vistos[2] != `{"n":3}` {
		t.Errorf("orden de envío: %v", vistos)
	}
	if c.Pendientes() != 0 {
		t.Errorf("quedaron %d pendientes", c.Pendientes())
	}
	if aparte, _ := os.ReadDir(filepath.Join(dir, "rechazados")); len(aparte) != 1 {
		t.Errorf("rechazados/ tiene %d archivos", len(aparte))
	}
}

func TestErrorTransitorioConservaLaCola(t *testing.T) {
	c, _ := Abrir(t.TempDir())
	agregar(t, c, `{"n":1}`, `{"n":2}`)

	intentos := 0
	enviados, _, err := c.Vaciar(func(Item) error {
		intentos++
		return errors.New("sin red")
	})
	if err == nil || enviados != 0 || intentos != 1 {
		t.Fatalf("enviados=%d intentos=%d err=%v", enviados, intentos, err)
	}
	if c.Pendientes() != 2 {
		t.Errorf("la cola debía conservar los 2 eventos; tiene %d", c.Pendientes())
	}

	// Al volver la red se envían, y la cola sobrevive a un reinicio del gateway
	c, _ = Abrir(c.dir)
	if enviados, _, err := c.Vaciar(func(Item) error { return nil }); err != nil || enviados != 2 {
		t.Errorf("tras reabrir: enviados=%d err=%v", enviados, err)
	}
}
