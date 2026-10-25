// Package api es el cliente del enlace 4 de docs/contrato-terminal.md (gateway ↔ API
// de p1-backend). Cada pedido de un terminal lleva su token en Authorization: Bearer.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Cliente habla con la API. HTTP necesita un Timeout largo: en el plan gratuito de
// Render el servicio se duerme y el primer pedido puede tardar cerca de un minuto.
type Cliente struct {
	Base string
	HTTP *http.Client
}

// Error es una respuesta de error de la API ({"ok":false,"error":"..."}).
type Error struct {
	Estado  int
	Mensaje string
}

func (e *Error) Error() string {
	return fmt.Sprintf("la API respondió %d: %s", e.Estado, e.Mensaje)
}

// Permanente indica que la API rechazó el pedido (4xx): reintentarlo no cambia nada.
func (e *Error) Permanente() bool {
	return e.Estado >= 400 && e.Estado < 500 &&
		e.Estado != http.StatusRequestTimeout && e.Estado != http.StatusTooManyRequests
}

// EsPermanente dice si err es un rechazo definitivo de la API.
func EsPermanente(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Permanente()
}

// Estado devuelve el código HTTP de un error de la API, o 0 si el error es de red.
func Estado(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Estado
	}
	return 0
}

// Config es la configuración del terminal: GET /api/terminal/config.
type Config struct {
	Rol   string `json:"rol"`
	A     string `json:"a"`
	Local string `json:"local"`
	Modo  string `json:"modo"`
}

// Bloque es un bloque de 16 B a escribir en la cédula: sector, bloque del sector y datos en hex.
type Bloque struct {
	S int    `json:"s"`
	B int    `json:"b"`
	D string `json:"d"`
}

// Emision es la respuesta de POST /api/emision/start: el handle nuevo y sus bloques,
// el índice (sector 1) primero.
type Emision struct {
	H      string   `json:"h"`
	Blocks []Bloque `json:"blocks"`
}

func (c *Cliente) Config(ctx context.Context, token string) (Config, error) {
	var out Config
	err := c.hacer(ctx, http.MethodGet, "/api/terminal/config", token, nil, &out)
	return out, err
}

func (c *Cliente) Revocaciones(ctx context.Context) ([]string, error) {
	var out struct {
		HS []string `json:"hs"`
	}
	err := c.hacer(ctx, http.MethodGet, "/api/revocations", "", nil, &out)
	return out.HS, err
}

// Evento envía una lectura o una alarma: POST /api/events.
func (c *Cliente) Evento(ctx context.Context, token string, evento json.RawMessage) error {
	return c.hacer(ctx, http.MethodPost, "/api/events", token, evento, nil)
}

func (c *Cliente) EmisionStart(ctx context.Context, token, uid string) (Emision, error) {
	var out Emision
	err := c.hacer(ctx, http.MethodPost, "/api/emision/start", token, map[string]string{"uid": uid}, &out)
	return out, err
}

// LinkStart devuelve el código de 6 dígitos para vincular la billetera: POST /api/link/start.
func (c *Cliente) LinkStart(ctx context.Context, token, h string) (string, error) {
	var out struct {
		C string `json:"c"`
	}
	err := c.hacer(ctx, http.MethodPost, "/api/link/start", token, map[string]string{"h": h}, &out)
	return out.C, err
}

// ConsentStart pide la autorización del titular, que la API le pregunta por WhatsApp:
// POST /api/consent. Con el titular sin WhatsApp o con los avisos apagados responde 422.
func (c *Cliente) ConsentStart(ctx context.Context, token, h, a string) (int64, error) {
	var out struct {
		ID int64 `json:"id"`
	}
	err := c.hacer(ctx, http.MethodPost, "/api/consent", token, map[string]string{"h": h, "a": a}, &out)
	return out.ID, err
}

// ConsentStatus consulta la solicitud: pendiente, aprobado, rechazado o vencido.
func (c *Cliente) ConsentStatus(ctx context.Context, token string, id int64) (string, error) {
	var out struct {
		Estado string `json:"estado"`
	}
	err := c.hacer(ctx, http.MethodGet, "/api/consent/"+strconv.FormatInt(id, 10), token, nil, &out)
	return out.Estado, err
}

func (c *Cliente) hacer(ctx context.Context, metodo, ruta, token string, cuerpo, salida any) error {
	var datos []byte
	switch v := cuerpo.(type) {
	case nil:
	case json.RawMessage:
		datos = v
	default:
		var err error
		if datos, err = json.Marshal(v); err != nil {
			return err
		}
	}

	var body io.Reader
	if datos != nil {
		body = bytes.NewReader(datos)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, strings.TrimRight(c.Base, "/")+ruta, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if datos != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respuesta, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respuesta, &e)
		if e.Error == "" {
			e.Error = http.StatusText(resp.StatusCode)
		}
		return &Error{Estado: resp.StatusCode, Mensaje: e.Error}
	}
	if salida == nil {
		return nil
	}
	return json.Unmarshal(respuesta, salida)
}
