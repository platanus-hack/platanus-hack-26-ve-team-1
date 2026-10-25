package api

import (
	"strings"
	"testing"
)

func TestUseMessage(t *testing.T) {
	cases := []struct {
		name      string
		res       int
		emg       bool
		want      []string
		blockable bool
	}{
		{"aprobado", 1, false, []string{"Hola Ana", "se autorizó el acceso", "'Mayor de 18'", "Bar La Esquina", "hoy a las 21:40"}, true},
		{"emergencia", 1, true, []string{"terminal de emergencia (Bar La Esquina)", "tipo de sangre y contacto"}, true},
		{"firma inválida", 2, false, []string{"se negó el acceso", "firma de la tarjeta no es válida"}, true},
		{"revocada", 3, false, []string{"se negó el acceso", "bloqueada", "reactivarla"}, false},
		{"vencida", 4, false, []string{"se negó el acceso", "vencida"}, true},
		{"sin permiso", 5, false, []string{"no tiene permiso", "No se compartió nada"}, true},
		{"rechazada", 6, false, []string{"no aceptaste compartir 'Mayor de 18'"}, true},
	}
	for _, c := range cases {
		body, blockable := useMessage("Ana", "Bar La Esquina", 1, c.res, c.emg, "hoy a las 21:40")
		for _, w := range c.want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: falta %q en %q", c.name, w, body)
			}
		}
		if blockable != c.blockable {
			t.Errorf("%s: blockable = %v, se esperaba %v", c.name, blockable, c.blockable)
		}
	}
	if body, _ := useMessage("", "X", 1, 9, false, "hoy"); body != "" {
		t.Errorf("un resultado desconocido no debe generar aviso: %q", body)
	}
}

func TestConsentMessage(t *testing.T) {
	got := consentMessage("Ana", "Bar La Esquina", 1)
	for _, w := range []string{"Hola Ana", "Bar La Esquina pide 'Mayor de 18'", "¿Lo compartes?"} {
		if !strings.Contains(got, w) {
			t.Errorf("falta %q en %q", w, got)
		}
	}
	if got := consentMessage("", "X", 4); !strings.HasPrefix(got, "Hola: X pide 'Sangre y alergias'") {
		t.Errorf("sin nombre: %q", got)
	}
}
