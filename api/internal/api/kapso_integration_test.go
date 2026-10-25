package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

type sent struct {
	To, Body string
	Buttons  bool
}

type fakeNotifier struct {
	mu   sync.Mutex
	msgs []sent
	// failButtons simula un número que no admite botones.
	failButtons bool
}

func (f *fakeNotifier) SendText(_ context.Context, to, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, sent{to, body, false})
	return nil
}

func (f *fakeNotifier) SendButtons(_ context.Context, to, body string, _ []kapso.Button) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failButtons {
		return fmt.Errorf("botones no soportados")
	}
	f.msgs = append(f.msgs, sent{to, body, true})
	return nil
}

func (f *fakeNotifier) take() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.msgs
	f.msgs = nil
	return out
}

// Avisos de WhatsApp y webhook de BLOQUEAR contra la base real (SLJ_INTEGRATION=1).
func TestIntegrationKapso(t *testing.T) {
	if os.Getenv("SLJ_INTEGRATION") != "1" {
		t.Skip("define SLJ_INTEGRATION=1 para correr contra la base")
	}
	url, err := repoutil.DatabaseURL()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	fake := &fakeNotifier{}
	srv := &Server{DB: db, JWTSecret: []byte("secreto-de-prueba-123456"), Issuers: map[byte]*ecdsa.PrivateKey{},
		ValidDays: 365, Notifier: fake, WebhookSecret: "whsec"}
	r := chi.NewRouter()
	srv.Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	tag := fmt.Sprintf("it%d", time.Now().UnixNano())
	var citizenIDs, terminalIDs []int64
	t.Cleanup(func() {
		srv.Wait()
		db.Exec(ctx, `DELETE FROM events WHERE terminal_id = ANY($1)`, terminalIDs)
		db.Exec(ctx, `DELETE FROM cards WHERE citizen_id = ANY($1)`, citizenIDs)
		db.Exec(ctx, `DELETE FROM terminals WHERE id = ANY($1)`, terminalIDs)
		db.Exec(ctx, `DELETE FROM citizens WHERE id = ANY($1)`, citizenIDs)
	})

	// Teléfonos únicos por corrida para no chocar con datos reales.
	suffix := fmt.Sprintf("%07d", time.Now().UnixNano()%10_000_000)
	anaPhone, ownerPhone := "58414"+suffix, "58416"+suffix
	mkCitizen := func(name, phone string, notifications bool) int64 {
		var id int64
		if err := db.QueryRow(ctx,
			`INSERT INTO citizens (email, password_hash, name, whatsapp_phone, notifications) VALUES ($1,'x',$2,$3,$4) RETURNING id`,
			tag+name+"@test.dev", name, phone, notifications).Scan(&id); err != nil {
			t.Fatal(err)
		}
		citizenIDs = append(citizenIDs, id)
		return id
	}
	anaID := mkCitizen("ana", anaPhone, true)
	ownerID := mkCitizen("dueno", ownerPhone, true)
	mutedID := mkCitizen("silencio", "58412"+suffix, false)

	mkCard := func(citizen int64) string {
		h := make([]byte, 8)
		rand.Read(h)
		if _, err := db.Exec(ctx, `INSERT INTO cards (handle, uid, citizen_id) VALUES ($1,$2,$3)`, h, []byte{1, 2, 3, 4}, citizen); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(h)
	}
	anaCard, mutedCard := mkCard(anaID), mkCard(mutedID)

	mkTerm := func(role, modo string, owner *int64, name string) string {
		raw := make([]byte, 16)
		rand.Read(raw)
		token := hex.EncodeToString(raw)
		var id int64
		if err := db.QueryRow(ctx,
			`INSERT INTO terminals (name, role, modo, owner_id, token_hash) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			name, role, modo, owner, terminal.HashToken(token)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		terminalIDs = append(terminalIDs, id)
		return token
	}
	shop := mkTerm("comercio", "verificar", &ownerID, "Bar La Esquina")
	emerg := mkTerm("emergencia", "emergencia", nil, "Ambulancia 1")

	post := func(path, token string, body string, hdr map[string]string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return resp.StatusCode, buf.String()
	}
	event := func(token, body string) {
		t.Helper()
		if code, out := post("/api/events", token, body, nil); code != 200 {
			t.Fatalf("events: %d %s", code, out)
		}
		srv.Wait()
	}
	now := time.Now().Unix()

	// Uso normal aprobado: aviso con botón al dueño de la cédula.
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A1","res":"01","ts":%d,"emg":false}`, anaCard, now))
	got := fake.take()
	if len(got) != 1 || got[0].To != anaPhone || !got[0].Buttons ||
		!strings.Contains(got[0].Body, "'Mayor de 18' con Bar La Esquina hoy a las") {
		t.Fatalf("aviso de uso: %+v", got)
	}

	// Un acceso denegado también se avisa, con el motivo.
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A1","res":"02","ts":%d}`, anaCard, now))
	got = fake.take()
	if len(got) != 1 || got[0].To != anaPhone || !got[0].Buttons || !strings.Contains(got[0].Body, "firma de la tarjeta no es válida") {
		t.Fatalf("aviso de firma inválida: %+v", got)
	}
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A1","res":"03","ts":%d}`, anaCard, now))
	got = fake.take()
	if len(got) != 1 || got[0].Buttons || !strings.Contains(got[0].Body, "bloqueada") {
		t.Fatalf("aviso de cédula revocada (sin botón): %+v", got)
	}
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A4","res":"05","ts":%d}`, anaCard, now))
	got = fake.take()
	if len(got) != 1 || !strings.Contains(got[0].Body, "no tiene permiso") {
		t.Fatalf("aviso de sin permiso: %+v", got)
	}

	// Cuentas con avisos apagados y cédulas desconocidas no generan mensaje.
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A1","res":"01","ts":%d}`, mutedCard, now))
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A1","res":"02","ts":%d}`, mutedCard, now))
	event(shop, `{"t":"verif","h":"00000000000000aa","a":"A1","res":"01"}`) // cédula desconocida
	if got := fake.take(); len(got) != 0 {
		t.Fatalf("no debía enviarse nada: %+v", got)
	}

	// Emergencia.
	event(emerg, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A4","res":"01","ts":%d,"emg":true}`, anaCard, now))
	got = fake.take()
	if len(got) != 1 || !strings.Contains(got[0].Body, "Un terminal de emergencia (Ambulancia 1) leyó tu tipo de sangre y contacto") {
		t.Fatalf("aviso de emergencia: %+v", got)
	}

	// Si los botones fallan, cae a texto con la instrucción.
	fake.failButtons = true
	event(shop, fmt.Sprintf(`{"t":"verif","h":"%s","a":"A2","res":"01","ts":%d}`, anaCard, now))
	got = fake.take()
	if len(got) != 1 || got[0].Buttons || !strings.HasSuffix(got[0].Body, "Responde BLOQUEAR.") {
		t.Fatalf("respaldo en texto: %+v", got)
	}
	fake.failButtons = false

	// Manipulación: alerta al dueño del terminal.
	event(shop, fmt.Sprintf(`{"t":"tamper","ts":%d}`, now))
	got = fake.take()
	if len(got) != 1 || got[0].To != ownerPhone || !strings.Contains(got[0].Body, "'Bar La Esquina' detectó manipulación") {
		t.Fatalf("alerta de manipulación: %+v", got)
	}
	event(emerg, fmt.Sprintf(`{"t":"tamper","ts":%d}`, now)) // sin dueño: no hay a quién avisar
	if got := fake.take(); len(got) != 0 {
		t.Fatalf("terminal sin dueño: %+v", got)
	}

	// Webhook: exige firma válida.
	sign := func(body string) string {
		m := hmac.New(sha256.New, []byte("whsec"))
		m.Write([]byte(body))
		return hex.EncodeToString(m.Sum(nil))
	}
	hook := func(body, key string) int {
		code, _ := post("/api/webhooks/kapso", "", body, map[string]string{
			"X-Webhook-Signature": sign(body), "X-Webhook-Event": "whatsapp.message.received", "X-Idempotency-Key": key})
		srv.Wait()
		return code
	}
	body := fmt.Sprintf(`{"message":{"from":"%s","text":{"body":"BLOQUEAR"}}}`, anaPhone)
	if code, _ := post("/api/webhooks/kapso", "", body, map[string]string{"X-Webhook-Signature": "00"}); code != 401 {
		t.Fatalf("firma falsa: %d", code)
	}
	if code, _ := post("/api/webhooks/kapso", "", body, nil); code != 401 {
		t.Fatalf("sin firma: %d", code)
	}

	// Un mensaje cualquiera no revoca; BLOQUEAR sí; el reintento no duplica la respuesta.
	other := fmt.Sprintf(`{"message":{"from":"%s","text":{"body":"hola"}}}`, anaPhone)
	if code := hook(other, "k0"); code != 200 {
		t.Fatalf("hola: %d", code)
	}
	var status string
	db.QueryRow(ctx, `SELECT status FROM cards WHERE handle = decode($1,'hex')`, anaCard).Scan(&status)
	if status != "activa" || len(fake.take()) != 0 {
		t.Fatalf("'hola' no debía revocar (estado %s)", status)
	}
	if code := hook(body, "k1"); code != 200 {
		t.Fatalf("bloquear: %d", code)
	}
	db.QueryRow(ctx, `SELECT status FROM cards WHERE handle = decode($1,'hex')`, anaCard).Scan(&status)
	got = fake.take()
	if status != "revocada" || len(got) != 1 || got[0].To != anaPhone || !strings.Contains(got[0].Body, "Tu cédula fue bloqueada") {
		t.Fatalf("bloqueo: estado %s, mensajes %+v", status, got)
	}
	hook(body, "k1")
	if got := fake.take(); len(got) != 0 {
		t.Fatalf("el reintento duplicó la respuesta: %+v", got)
	}
	// Con otra clave de idempotencia ya no hay cédulas activas.
	hook(body, "k2")
	got = fake.take()
	if len(got) != 1 || !strings.Contains(got[0].Body, "No tienes cédulas activas") {
		t.Fatalf("sin cédulas activas: %+v", got)
	}

	// Botón de respuesta y un número desconocido (no se responde).
	btn := fmt.Sprintf(`{"message":{"from":"%s","interactive":{"button_reply":{"id":"bloquear","title":"Bloquear cédula"}}}}`, "5800000"+suffix)
	hook(btn, "k3")
	if got := fake.take(); len(got) != 0 {
		t.Fatalf("número desconocido: %+v", got)
	}
}
