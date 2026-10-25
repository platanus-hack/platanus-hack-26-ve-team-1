package protocolo

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestMensajesCabenEnLaLinea(t *testing.T) {
	hora, _ := Time(4102444800)
	write, _ := Write(14, 2, strings.Repeat("ff", 16))
	seal, _ := Seal()
	code, _ := Code("123456")
	consent, _ := Consent(false)
	revs, _ := Rev([]string{"7f94683ed2fff69d", "0123456789abcdef"})
	cfg, err := Cfg("emergencia", "", strings.Repeat("Ambulancia de guardia ", 10), "emergencia")
	if err != nil {
		t.Fatalf("Cfg con un local largo: %v", err)
	}
	if string(consent) != `{"t":"consent","approved":false}` {
		t.Errorf("consent: %s", consent)
	}

	for _, m := range append([][]byte{hora, write, seal, code, consent, cfg}, revs...) {
		if len(m)+1 > MaxLinea {
			t.Errorf("%s mide %d B con el \\n", m, len(m)+1)
		}
		if !json.Valid(m) {
			t.Errorf("%s no es JSON válido", m)
		}
	}
	if got := len(revs[0]) + 1; got > 71 {
		t.Errorf("rev con 2 handles mide %d B; el contrato reserva 71", got)
	}
}

func TestRevEnTrozos(t *testing.T) {
	vacia, _ := Rev(nil)
	if len(vacia) != 1 || string(vacia[0]) != `{"t":"rev","hs":[],"more":false}` {
		t.Fatalf("lista vacía: %s", vacia)
	}

	trozos, _ := Rev([]string{"a1", "a2", "a3"})
	want := []string{`{"t":"rev","hs":["a1","a2"],"more":true}`, `{"t":"rev","hs":["a3"],"more":false}`}
	if len(trozos) != len(want) {
		t.Fatalf("3 handles: %d trozos", len(trozos))
	}
	for i := range want {
		if string(trozos[i]) != want[i] {
			t.Errorf("trozo %d = %s, se esperaba %s", i, trozos[i], want[i])
		}
	}
}

func TestDecodificar(t *testing.T) {
	for _, basura := range []string{"", "ets Jul 29 2019 rst:0x1", "{no es json", `{"m":"sin tipo"}`} {
		if _, ok := Decodificar([]byte(basura)); ok {
			t.Errorf("%q no debería aceptarse", basura)
		}
	}

	m, ok := Decodificar([]byte(`{"t":"verif","h":"9E429475799FC8ED","a":1,"res":"01","ts":1761300000,"emg":false}` + "\r\n"))
	if !ok || m.A != "A1" || m.Res != "01" || m.TS != 1761300000 {
		t.Errorf("verif con atributo numérico: %+v", m)
	}

	m, _ = Decodificar([]byte(`{"t":"hb","sync":false}`))
	if m.Sync == nil || *m.Sync {
		t.Errorf("hb sin hora: %+v", m)
	}
}

func TestLectorDescartaLineasLargas(t *testing.T) {
	entrada := strings.Repeat("x", 5000) + "\n" + `{"t":"hb"}` + "\n"
	l := NuevoLector(struct {
		io.Reader
		io.Writer
	}{strings.NewReader(entrada), io.Discard})

	var tipos []string
	l.Leer(func(m Mensaje) { tipos = append(tipos, m.T) })
	if len(tipos) != 1 || tipos[0] != "hb" {
		t.Errorf("mensajes leídos: %v", tipos)
	}
}
