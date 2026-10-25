package api

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// Historial y tiempo real: cada ciudadano ve solo lo suyo, sin handle ni UID (SLJ_INTEGRATION=1).
func TestIntegrationHistory(t *testing.T) {
	if os.Getenv("SLJ_INTEGRATION") != "1" {
		t.Skip("define SLJ_INTEGRATION=1 para correr contra la base")
	}
	url, err := repoutil.DatabaseURL()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	srv := &Server{DB: db, JWTSecret: []byte("secreto-de-prueba-123456"), Issuers: map[byte]*ecdsa.PrivateKey{}, ValidDays: 365}
	srv.Start(ctx)
	r := chi.NewRouter()
	srv.Routes(r)
	ts := httptest.NewServer(r)

	tag := fmt.Sprintf("it%d", time.Now().UnixNano())
	var citizenIDs, terminalIDs []int64
	t.Cleanup(func() {
		bg := context.Background()
		db.Exec(bg, `DELETE FROM events WHERE terminal_id = ANY($1)`, terminalIDs)
		db.Exec(bg, `DELETE FROM cards WHERE citizen_id = ANY($1)`, citizenIDs)
		db.Exec(bg, `DELETE FROM terminals WHERE id = ANY($1)`, terminalIDs)
		db.Exec(bg, `DELETE FROM citizens WHERE id = ANY($1)`, citizenIDs)
	})
	// Se registran al final para que corran primero: ts.Close espera a los streams abiertos.
	t.Cleanup(ts.Close)
	t.Cleanup(cancel)

	register := func(name string) (*http.Client, int64) {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar}
		body := fmt.Sprintf(`{"email":"%s-%s@test.dev","password":"clave-segura-1","name":"%s"}`, tag, name, name)
		resp, err := c.Post(ts.URL+"/api/auth/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out struct{ User struct{ ID string } }
		json.NewDecoder(resp.Body).Decode(&out)
		id, _ := atoi(out.User.ID)
		citizenIDs = append(citizenIDs, id)
		return c, id
	}
	ana, anaID := register("ana")
	luis, luisID := register("luis")
	tienda, tiendaID := register("tienda")
	otra, otraID := register("otra")
	db.Exec(ctx, `UPDATE citizens SET role = 'comercio' WHERE id = ANY($1)`, []int64{tiendaID, otraID})

	mkCard := func(citizen int64) string {
		h := make([]byte, 8)
		rand.Read(h)
		if _, err := db.Exec(ctx, `INSERT INTO cards (handle, uid, citizen_id) VALUES ($1,$2,$3)`, h, []byte{9, 9, 9, 9}, citizen); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(h)
	}
	anaCard, luisCard := mkCard(anaID), mkCard(luisID)

	raw := make([]byte, 16)
	rand.Read(raw)
	token := hex.EncodeToString(raw)
	var termID int64
	if err := db.QueryRow(ctx, `INSERT INTO terminals (name, role, modo, token_hash) VALUES ($1,'comercio','verificar',$2) RETURNING id`,
		"Bar "+tag, terminal.HashToken(token)).Scan(&termID); err != nil {
		t.Fatal(err)
	}
	terminalIDs = append(terminalIDs, termID)
	db.Exec(ctx, `UPDATE terminals SET owner_id = $1 WHERE id = $2`, tiendaID, termID)

	post := func(body string) {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+"/api/events", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("POST /api/events: %v %v", err, resp)
		}
		resp.Body.Close()
	}
	ev := func(h, a, res string, emg bool) string {
		return fmt.Sprintf(`{"t":"verif","h":"%s","a":"%s","res":"%s","ts":%d,"emg":%v}`, h, a, res, time.Now().Unix(), emg)
	}

	// Abrimos los streams antes de generar eventos.
	type stream struct {
		lines chan string
	}
	open := func(c *http.Client) *stream {
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream", nil)
		resp, err := c.Do(req)
		if err != nil || resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Fatalf("stream: %v %v", err, resp)
		}
		s := &stream{lines: make(chan string, 64)}
		go func() {
			defer resp.Body.Close()
			sc := bufio.NewScanner(resp.Body)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "data: ") {
					s.lines <- strings.TrimPrefix(sc.Text(), "data: ")
				}
			}
		}()
		return s
	}
	anaStream, luisStream := open(ana), open(luis)
	time.Sleep(1500 * time.Millisecond) // deja que el LISTEN esté activo

	post(ev(anaCard, "A1", "01", false))
	post(ev(anaCard, "A4", "01", true))
	post(ev(anaCard, "A2", "02", false))
	post(ev(luisCard, "A1", "01", false))
	post(`{"t":"tamper","ts":1}`) // las alarmas no son del ciudadano

	// Stream de Ana: sus 3 eventos, en orden, con la forma acordada.
	var got []eventOut
	for len(got) < 3 {
		select {
		case l := <-anaStream.lines:
			var e eventOut
			if err := json.Unmarshal([]byte(l), &e); err != nil {
				t.Fatalf("data inválida %q: %v", l, err)
			}
			got = append(got, e)
		case <-time.After(8 * time.Second):
			t.Fatalf("el stream de Ana recibió solo %d eventos", len(got))
		}
	}
	if got[0].Atributo != "A1" || got[0].Resultado != 1 || got[0].Emergencia || got[0].Local != "Bar "+tag ||
		got[1].Atributo != "A4" || !got[1].Emergencia || got[2].Resultado != 2 || got[0].ID >= got[1].ID {
		t.Fatalf("eventos del stream: %+v", got)
	}
	if _, err := time.Parse(time.RFC3339, got[0].Hora); err != nil {
		t.Fatalf("hora: %v", err)
	}
	// El stream de Luis recibe únicamente el suyo.
	select {
	case l := <-luisStream.lines:
		var e eventOut
		json.Unmarshal([]byte(l), &e)
		if e.Atributo != "A1" || e.ID <= got[2].ID {
			t.Fatalf("evento de Luis: %s", l)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("el stream de Luis no recibió su evento")
	}
	select {
	case l := <-luisStream.lines:
		t.Fatalf("Luis recibió un evento de más: %s", l)
	case <-time.After(500 * time.Millisecond):
	}

	// GET /api/events: solo lo propio, más reciente primero, sin handle ni UID.
	get := func(c *http.Client, path string) (int, []byte, map[string]any) {
		resp, err := c.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		var buf strings.Builder
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			buf.WriteString(sc.Text())
		}
		json.Unmarshal([]byte(buf.String()), &m)
		return resp.StatusCode, []byte(buf.String()), m
	}
	code, rawBody, m := get(ana, "/api/events")
	list := m["events"].([]any)
	if code != 200 || len(list) != 3 {
		t.Fatalf("eventos de Ana: %d %s", code, rawBody)
	}
	first := list[0].(map[string]any)
	if len(first) != 6 || first["id"] == nil || first["local"] != "Bar "+tag || first["atributo"] != "A2" ||
		first["resultado"] != float64(2) || first["hora"] == nil || first["emergencia"] != false {
		t.Fatalf("forma del evento: %v", first)
	}
	for _, banned := range []string{anaCard, luisCard, "handle", "uid", `"h"`} {
		if strings.Contains(string(rawBody), banned) {
			t.Fatalf("la respuesta contiene %q: %s", banned, rawBody)
		}
	}
	_, _, m = get(luis, "/api/events")
	if n := len(m["events"].([]any)); n != 1 {
		t.Fatalf("Luis ve %d eventos (esperaba 1)", n)
	}
	// Paginación y límite.
	_, _, m = get(ana, fmt.Sprintf("/api/events?limit=1&before=%v", list[0].(map[string]any)["id"]))
	if p := m["events"].([]any); len(p) != 1 || p[0].(map[string]any)["atributo"] != "A4" {
		t.Fatalf("paginación: %v", p)
	}
	// Vista del comercio: totales de sus terminales, sin handles; otros no ven nada suyo.
	code, rawBody, m = get(tienda, "/api/commerce/summary")
	tot := m["totals"].(map[string]any)
	pr := m["por_resultado"].(map[string]any)
	if code != 200 || tot["total"] != float64(4) || tot["aprobadas"] != float64(3) || tot["rechazadas"] != float64(1) ||
		tot["manipulaciones"] != float64(1) || pr["1"] != float64(3) || pr["2"] != float64(1) || len(m["terminales"].([]any)) != 1 {
		t.Fatalf("resumen del comercio: %d %s", code, rawBody)
	}
	for _, banned := range []string{anaCard, luisCard, "handle", "uid", `"h"`} {
		if strings.Contains(string(rawBody), banned) {
			t.Fatalf("el resumen contiene %q: %s", banned, rawBody)
		}
	}
	_, _, m = get(otra, "/api/commerce/summary")
	if m["totals"].(map[string]any)["total"] != float64(0) || len(m["terminales"].([]any)) != 0 {
		t.Fatalf("otro comercio ve datos ajenos: %v", m)
	}
	if code, _, _ := get(ana, "/api/commerce/summary"); code != 403 {
		t.Fatalf("un ciudadano no debe ver el resumen: %d", code)
	}

	// Sin sesión no hay nada.
	if code, _, _ := get(&http.Client{}, "/api/events"); code != 401 {
		t.Fatalf("sin sesión: %d", code)
	}
	if resp, _ := http.Get(ts.URL + "/api/events/stream"); resp.StatusCode != 401 {
		t.Fatalf("stream sin sesión: %d", resp.StatusCode)
	}
}
