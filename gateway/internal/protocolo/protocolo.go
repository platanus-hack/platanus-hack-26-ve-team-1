// Package protocolo implementa el enlace 1 de docs/contrato-terminal.md: una línea
// JSON por mensaje entre el lector (ESP32) y el gateway, a 115200 baudios.
package protocolo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"unicode/utf8"
)

// MaxLinea es el largo máximo de una línea del gateway al lector, con el \n.
const MaxLinea = 120

// MaxHandlesRev es la cantidad máxima de handles por mensaje rev.
const MaxHandlesRev = 2

// Las líneas del lector más largas que esto se descartan (basura del bootloader).
const maxEntrada = 4096

// Mensaje es una línea del lector al gateway; solo traen valor los campos de su tipo T.
type Mensaje struct {
	T    string   `json:"t"`
	H    string   `json:"h"`
	A    Atributo `json:"a"`
	Res  string   `json:"res"`
	TS   int64    `json:"ts"`
	Emg  bool     `json:"emg"`
	UID  string   `json:"uid"`
	S    int      `json:"s"`
	B    int      `json:"b"`
	OK   bool     `json:"ok"`
	M    string   `json:"m"`
	Sync *bool    `json:"sync"`
}

// Atributo acepta "A1" (contrato) y también 1, como lo enviaba el firmware antes de
// alinearse con el contrato.
type Atributo string

func (a *Atributo) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*a = Atributo(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*a = Atributo("A" + strconv.Itoa(n))
	return nil
}

// Decodificar interpreta una línea del lector. false = no es un mensaje JSON válido;
// el contrato pide ignorarla.
func Decodificar(linea []byte) (Mensaje, bool) {
	linea = bytes.TrimSpace(linea)
	if len(linea) == 0 || linea[0] != '{' {
		return Mensaje{}, false
	}
	var m Mensaje
	if err := json.Unmarshal(linea, &m); err != nil || m.T == "" {
		return Mensaje{}, false
	}
	return m, true
}

// ─── Gateway → lector ────────────────────────────────────────────────────────

func codificar(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b)+1 > MaxLinea {
		return nil, fmt.Errorf("mensaje de %d B: el máximo es %d B con el \\n", len(b)+1, MaxLinea)
	}
	return b, nil
}

// Time: {"t":"time","ts":1761300000}
func Time(ts int64) ([]byte, error) {
	return codificar(struct {
		T  string `json:"t"`
		TS int64  `json:"ts"`
	}{"time", ts})
}

// Rev arma la lista de revocadas en trozos de MaxHandlesRev, con more:false en el
// último. Una lista vacía también se envía, para que el lector borre la suya.
func Rev(hs []string) ([][]byte, error) {
	var out [][]byte
	for i := 0; i == 0 || i < len(hs); i += MaxHandlesRev {
		trozo := hs[i:min(i+MaxHandlesRev, len(hs))]
		b, err := codificar(struct {
			T    string   `json:"t"`
			HS   []string `json:"hs"`
			More bool     `json:"more"`
		}{"rev", append([]string{}, trozo...), i+MaxHandlesRev < len(hs)})
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// Cfg: {"t":"cfg","rol":"…","a":"A1","local":"…","modo":"verificar"}. Si la línea no
// cabe, se acorta el nombre del local.
func Cfg(rol, a, local, modo string) ([]byte, error) {
	for {
		b, err := codificar(struct {
			T     string `json:"t"`
			Rol   string `json:"rol"`
			A     string `json:"a"`
			Local string `json:"local"`
			Modo  string `json:"modo"`
		}{"cfg", rol, a, local, modo})
		if err == nil || local == "" {
			return b, err
		}
		_, n := utf8.DecodeLastRuneInString(local)
		local = local[:len(local)-n]
	}
}

// Write: {"t":"write","s":1,"b":0,"d":"<32 hex>"}
func Write(s, b int, d string) ([]byte, error) {
	return codificar(struct {
		T string `json:"t"`
		S int    `json:"s"`
		B int    `json:"b"`
		D string `json:"d"`
	}{"write", s, b, d})
}

// Seal pide al lector grabar los trailers; sin bloques escritos solo lo libera
// (responde written con ok:false).
func Seal() ([]byte, error) {
	return codificar(struct {
		T string `json:"t"`
	}{"seal"})
}

// Code: {"t":"code","c":"123456"}
func Code(c string) ([]byte, error) {
	return codificar(struct {
		T string `json:"t"`
		C string `json:"c"`
	}{"code", c})
}

// ─── Conexión ────────────────────────────────────────────────────────────────

// Lector es la conexión con el ESP32 por el puerto serie (o un io.ReadWriter en las pruebas).
type Lector struct {
	r  *bufio.Reader
	mu sync.Mutex
	w  io.Writer
}

func NuevoLector(rw io.ReadWriter) *Lector {
	return &Lector{r: bufio.NewReaderSize(rw, 512), w: rw}
}

// Enviar escribe una línea hacia el lector y le agrega el \n.
func (l *Lector) Enviar(linea []byte) error {
	if len(linea)+1 > MaxLinea {
		return fmt.Errorf("mensaje de %d B: el máximo es %d B con el \\n", len(linea)+1, MaxLinea)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.w.Write(append(append([]byte(nil), linea...), '\n'))
	return err
}

// Leer entrega a fn cada mensaje válido del lector, en orden, hasta que la conexión falle.
func (l *Lector) Leer(fn func(Mensaje)) error {
	var linea []byte
	larga := false
	for {
		trozo, err := l.r.ReadSlice('\n')
		if !larga {
			linea = append(linea, trozo...)
			if len(linea) > maxEntrada {
				larga, linea = true, linea[:0]
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return err
		}
		if !larga {
			if m, ok := Decodificar(linea); ok {
				fn(m)
			}
		}
		larga, linea = false, linea[:0]
	}
}
