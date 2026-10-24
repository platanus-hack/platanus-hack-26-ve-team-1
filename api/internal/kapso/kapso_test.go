package kapso

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	for in, want := range map[string]string{
		"0424-258 7407":   "584242587407",
		"+58 424 2587407": "584242587407",
		"584242587407":    "584242587407",
		"00584242587407":  "584242587407",
		"":                "",
	} {
		if got := NormalizePhone(in); got != want {
			t.Errorf("%q: %q, esperaba %q", in, got, want)
		}
	}
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"a":1}`)
	m := hmac.New(sha256.New, []byte("s3cret"))
	m.Write(body)
	sig := hex.EncodeToString(m.Sum(nil))
	if !VerifySignature("s3cret", body, sig) || !VerifySignature("s3cret", body, "sha256="+sig) {
		t.Fatal("firma válida rechazada")
	}
	if VerifySignature("otro", body, sig) || VerifySignature("s3cret", []byte(`{"a":2}`), sig) ||
		VerifySignature("s3cret", body, "") || VerifySignature("", body, sig) || VerifySignature("s3cret", body, "zz") {
		t.Fatal("firma inválida aceptada")
	}
}

func TestParseWebhook(t *testing.T) {
	text := `{"message":{"from":"584242587407","type":"text","text":{"body":" Bloquear. "}},"phone_number_id":"1"}`
	got, err := ParseWebhook([]byte(text))
	if err != nil || len(got) != 1 || got[0].From != "584242587407" || !got[0].IsBlock() {
		t.Fatalf("texto: %v %+v", err, got)
	}
	btn := `{"batch":true,"data":[{"message":{"from":"584141112222","interactive":{"type":"button_reply","button_reply":{"id":"bloquear","title":"Bloquear cédula"}}}},
	  {"message":{"from":"584143334444","text":{"body":"hola"}}},{"no":"message"}]}`
	got, _ = ParseWebhook([]byte(btn))
	if len(got) != 2 || !got[0].IsBlock() || got[1].IsBlock() {
		t.Fatalf("lote: %+v", got)
	}
	conv := `{"message":{"text":{"body":"BLOQUEAR"}},"conversation":{"phone_number":"+58 414 111 2222"}}`
	got, _ = ParseWebhook([]byte(conv))
	if len(got) != 1 || got[0].From != "584141112222" {
		t.Fatalf("conversación: %+v", got)
	}
	if (Incoming{Text: "no quiero bloquear"}).IsBlock() {
		t.Fatal("una frase que contiene la palabra no debe bloquear")
	}
	if _, err := ParseWebhook([]byte("no json")); err == nil {
		t.Fatal("JSON inválido aceptado")
	}
}

func TestSend(t *testing.T) {
	var path, key string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, key = r.URL.Path, r.Header.Get("X-API-Key")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New("clave", "123")
	c.BaseURL = srv.URL
	if err := c.SendButtons(context.Background(), "58414", "¿Fuiste tú?", []Button{{"bloquear", "Bloquear cédula"}}); err != nil {
		t.Fatal(err)
	}
	if path != "/meta/whatsapp/v24.0/123/messages" || key != "clave" || body["type"] != "interactive" {
		t.Fatalf("petición: %s %s %v", path, key, body)
	}
	if err := c.SendText(context.Background(), "58414", "hola"); err != nil || body["type"] != "text" {
		t.Fatalf("texto: %v %v", err, body)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"WhatsApp configuration not found"}`))
	}))
	defer bad.Close()
	c.BaseURL = bad.URL
	if err := c.SendText(context.Background(), "58414", "x"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error esperado: %v", err)
	}
}
