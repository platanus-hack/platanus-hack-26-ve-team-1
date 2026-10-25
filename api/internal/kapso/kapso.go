// Package kapso envía mensajes de WhatsApp por la API de Kapso y lee sus webhooks.
package kapso

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBase = "https://api.kapso.ai"

// Button es un botón de respuesta rápida (título de hasta 20 caracteres).
type Button struct{ ID, Title string }

type Client struct {
	APIKey        string
	PhoneNumberID string
	BaseURL       string
	HTTP          *http.Client
}

func New(apiKey, phoneNumberID string) *Client {
	return &Client{APIKey: apiKey, PhoneNumberID: phoneNumberID, BaseURL: defaultBase,
		HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// SendText envía un mensaje de texto. to es el teléfono en dígitos con código de país.
func (c *Client) SendText(ctx context.Context, to, body string) error {
	return c.post(ctx, map[string]any{
		"messaging_product": "whatsapp", "to": to, "type": "text",
		"text": map[string]any{"body": body},
	})
}

// SendButtons envía un mensaje con hasta 3 botones de respuesta.
func (c *Client) SendButtons(ctx context.Context, to, body string, buttons []Button) error {
	if len(buttons) == 0 || len(buttons) > 3 {
		return fmt.Errorf("se necesitan entre 1 y 3 botones")
	}
	bs := make([]map[string]any, len(buttons))
	for i, b := range buttons {
		bs[i] = map[string]any{"type": "reply", "reply": map[string]any{"id": b.ID, "title": b.Title}}
	}
	return c.post(ctx, map[string]any{
		"messaging_product": "whatsapp", "to": to, "type": "interactive",
		"interactive": map[string]any{
			"type":   "button",
			"body":   map[string]any{"text": body},
			"action": map[string]any{"buttons": bs},
		},
	})
}

func (c *Client) post(ctx context.Context, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/meta/whatsapp/v24.0/%s/messages", strings.TrimRight(c.BaseURL, "/"), c.PhoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("kapso: %w", sanitize(err, c.APIKey))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("kapso: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func sanitize(err error, secret string) error {
	if secret == "" {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), secret, "***"))
}

// VerifySignature comprueba el HMAC-SHA256 (hex) del cuerpo crudo con el secreto del webhook.
func VerifySignature(secret string, body []byte, sig string) bool {
	if secret == "" || sig == "" {
		return false
	}
	sig = strings.TrimPrefix(strings.TrimSpace(sig), "sha256=")
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

// Incoming es un mensaje recibido por WhatsApp.
type Incoming struct {
	From     string // teléfono normalizado
	Text     string
	ButtonID string
}

// IsBlock indica si el mensaje pide bloquear la cédula (botón o la palabra BLOQUEAR).
func (m Incoming) IsBlock() bool {
	if strings.EqualFold(m.ButtonID, "bloquear") {
		return true
	}
	t := strings.ToLower(strings.Trim(strings.TrimSpace(m.Text), " .!¡¿?\"'"))
	return t == "bloquear"
}

// Consent reconoce la respuesta a una solicitud de consentimiento: el botón "consent:<id>:si|no" o,
// como respaldo, el texto SI/NO. Con texto id vale 0: se aplica a la última solicitud pendiente.
func (m Incoming) Consent() (id int64, approved bool, ok bool) {
	if strings.HasPrefix(strings.ToLower(m.ButtonID), "consent:") {
		parts := strings.Split(strings.ToLower(m.ButtonID), ":")
		if len(parts) != 3 || (parts[2] != "si" && parts[2] != "no") {
			return 0, false, false
		}
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, false, false
		}
		return n, parts[2] == "si", true
	}
	if m.ButtonID != "" {
		return 0, false, false
	}
	switch strings.ToLower(strings.Trim(strings.TrimSpace(m.Text), " .!¡¿?\"'")) {
	case "si", "sí":
		return 0, true, true
	case "no":
		return 0, false, true
	}
	return 0, false, false
}

// ParseWebhook extrae los mensajes de un evento whatsapp.message.received, suelto o en lote.
func ParseWebhook(body []byte) ([]Incoming, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	items := []map[string]any{root}
	if data, ok := root["data"].([]any); ok && root["batch"] == true {
		items = nil
		for _, d := range data {
			if m, ok := d.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	var out []Incoming
	for _, it := range items {
		msg, _ := it["message"].(map[string]any)
		if msg == nil {
			continue
		}
		from := str(msg["from"])
		if conv, ok := it["conversation"].(map[string]any); ok && from == "" {
			from = str(conv["phone_number"])
		}
		in := Incoming{From: NormalizePhone(from)}
		if t, ok := msg["text"].(map[string]any); ok {
			in.Text = str(t["body"])
		}
		if i, ok := msg["interactive"].(map[string]any); ok {
			for _, k := range []string{"button_reply", "list_reply"} {
				if r, ok := i[k].(map[string]any); ok {
					in.ButtonID, in.Text = str(r["id"]), firstNonEmpty(in.Text, str(r["title"]))
				}
			}
		}
		if b, ok := msg["button"].(map[string]any); ok {
			in.ButtonID, in.Text = str(b["payload"]), firstNonEmpty(in.Text, str(b["text"]))
		}
		if in.From != "" {
			out = append(out, in)
		}
	}
	return out, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// NormalizePhone deja solo dígitos con código de país. Un número que empieza en 0
// se toma como venezolano: 0424... → 58424...
func NormalizePhone(s string) string {
	var d strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	p := strings.TrimPrefix(d.String(), "00")
	if strings.HasPrefix(p, "0") {
		p = "58" + p[1:]
	}
	return p
}
