package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// Rutas que usa el front (nombres en español, comercio, emisión dirigida, estudiantes). SLJ_INTEGRATION=1.
func TestIntegrationFront(t *testing.T) {
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

	nat, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	uni, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	srv := &Server{DB: db, JWTSecret: []byte("secreto-de-prueba-123456"),
		Issuers: map[byte]*ecdsa.PrivateKey{sign.IssuerNacional: nat, sign.IssuerUniversidad: uni}, ValidDays: 365}
	r := chi.NewRouter()
	srv.Routes(r)
	ts := httptest.NewServer(r)

	tag := fmt.Sprintf("it%d", time.Now().UnixNano())
	var citizenIDs, terminalIDs []int64
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM events WHERE terminal_id IN (SELECT id FROM terminals WHERE owner_id = ANY($1) OR id = ANY($2))`, citizenIDs, terminalIDs)
		db.Exec(ctx, `DELETE FROM cards WHERE citizen_id = ANY($1)`, citizenIDs)
		db.Exec(ctx, `DELETE FROM terminals WHERE owner_id = ANY($1) OR id = ANY($2)`, citizenIDs, terminalIDs)
		db.Exec(ctx, `DELETE FROM citizens WHERE id = ANY($1)`, citizenIDs)
	})
	t.Cleanup(ts.Close)

	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar}
	}
	call := func(c *http.Client, method, path, token string, body any, want int) map[string]any {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, ts.URL+path, rd)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: %d (esperaba %d): %v", method, path, resp.StatusCode, want, out)
		}
		return out
	}
	mkUser := func(name, role string) (*http.Client, int64) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("clave-segura-1"), bcrypt.MinCost)
		var id int64
		if err := db.QueryRow(ctx, `INSERT INTO citizens (email, password_hash, role, name) VALUES ($1,$2,$3,$4) RETURNING id`,
			tag+name+"@test.dev", string(hash), role, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		citizenIDs = append(citizenIDs, id)
		c := newClient()
		call(c, "POST", "/api/auth/login", "", map[string]any{"email": tag + name + "@test.dev", "password": "clave-segura-1"}, 200)
		return c, id
	}

	// Registro con los nombres del front: usuario en la raíz y en "user".
	ana := newClient()
	reg := call(ana, "POST", "/api/auth/register", "", map[string]any{
		"nombre": "Ana", "email": tag + "ana@test.dev", "password": "clave-segura-1", "telefono": "+58 424 1234567"}, 201)
	anaID := int64(reg["id"].(float64))
	citizenIDs = append(citizenIDs, anaID)
	if reg["nombre"] != "Ana" || reg["rol"] != "ciudadano" || reg["telefono"] != "+584241234567" || reg["avisosActivados"] != true ||
		reg["user"].(map[string]any)["rol"] != "ciudadano" {
		t.Fatalf("usuario: %v", reg)
	}
	me := call(ana, "GET", "/api/me", "", nil, 200)
	if me["email"] != tag+"ana@test.dev" || me["user"].(map[string]any)["id"] != reg["id"] {
		t.Fatalf("/api/me: %v", me)
	}
	n := call(ana, "PUT", "/api/me/notifications", "", map[string]any{"telefono": "+58 414 9998877", "avisosActivados": false}, 200)
	if n["telefono"] != "+584149998877" || n["avisosActivados"] != false {
		t.Fatalf("notificaciones: %v", n)
	}
	var phone string
	var notif bool
	db.QueryRow(ctx, `SELECT whatsapp_phone, notifications FROM citizens WHERE id = $1`, anaID).Scan(&phone, &notif)
	if phone != "584149998877" || notif {
		t.Fatalf("en la base: %s %v", phone, notif)
	}
	call(ana, "PUT", "/api/me/notifications", "", map[string]any{"avisosActivados": true}, 200) // solo avisos: no pierde el teléfono
	if me := call(ana, "GET", "/api/me", "", nil, 200); me["telefono"] != "+584149998877" {
		t.Fatalf("el teléfono se perdió: %v", me)
	}

	// Sin cédula todavía.
	c0 := call(ana, "GET", "/api/card", "", nil, 200)
	if c0["estado"] != "sin cédula" || c0["emitidaEn"] != nil || len(c0["atributos"].([]any)) != 6 {
		t.Fatalf("sin cédula: %v", c0)
	}

	// Datos opcionales: se validan (16 B) y alimentan A4/A5/A6.
	opt := map[string]any{"sangre": "O+", "alergias": "penicilina", "contacto": map[string]any{"nombre": "Mama", "telefono": "04141234567"},
		"donante": true, "licencia": "B2"}
	call(ana, "PUT", "/api/me/optional-data", "", map[string]any{"sangre": "O+", "alergias": "una alergia demasiado larga"}, 400)
	got := call(ana, "PUT", "/api/me/optional-data", "", opt, 200)
	if got["sangre"] != "O+" || got["contacto"].(map[string]any)["telefono"] != "04141234567" || got["donante"] != true {
		t.Fatalf("optional-data: %v", got)
	}
	if g := call(ana, "GET", "/api/me/optional-data", "", nil, 200); g["alergias"] != "penicilina" || g["licencia"] != "B2" {
		t.Fatalf("GET optional-data: %v", g)
	}

	// Emisor: perfil, estudiantes y orden de emisión dirigida a un terminal.
	emisor, _ := mkUser("emisor", "emisor")
	call(emisor, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", map[string]any{
		"birthdate": "2000-03-04", "identity": map[string]any{"name": "ANA PEREZ", "id": "V-1", "valid_until": "2030-01-01"},
		"carnet": map[string]any{"name": "ANA PEREZ", "id": "UCV-9", "valid_until": "2027-07-31"}}, 200)
	// El perfil del emisor no pisa lo que declaró el ciudadano.
	if g := call(ana, "GET", "/api/me/optional-data", "", nil, 200); g["sangre"] != "O+" {
		t.Fatalf("el perfil del emisor borró los datos opcionales: %v", g)
	}
	cit := call(emisor, "GET", "/api/citizens", "", nil, 200)["citizens"].([]any)
	var anaRow map[string]any
	for _, c := range cit {
		if c.(map[string]any)["id"] == float64(anaID) {
			anaRow = c.(map[string]any)
		}
	}
	if anaRow == nil || anaRow["nombre"] != "Ana" || anaRow["estadoCedula"] != "sin emitir" || anaRow["emitidaEn"] != nil {
		t.Fatalf("citizens: %v", anaRow)
	}
	for _, banned := range []string{"handle", "uid", `"h"`} {
		if b, _ := json.Marshal(cit); strings.Contains(string(b), banned) {
			t.Fatalf("citizens contiene %q", banned)
		}
	}
	st := call(emisor, "GET", "/api/students", "", nil, 200)["students"].([]any)
	var anaStudent map[string]any
	for _, s := range st {
		if s.(map[string]any)["id"] == float64(anaID) {
			anaStudent = s.(map[string]any)
		}
	}
	if anaStudent == nil || anaStudent["estado"] != "Sin carnet" || anaStudent["carnet"] != "UCV-9" {
		t.Fatalf("students: %v", anaStudent)
	}
	call(emisor, "POST", "/api/credentials/university", "", map[string]any{"ciudadanoId": anaID}, 409) // sin celular vinculado

	mkTerm := func(role, modo string) (int64, string) {
		raw := make([]byte, 16)
		rand.Read(raw)
		token := hex.EncodeToString(raw)
		var id int64
		if err := db.QueryRow(ctx, `INSERT INTO terminals (name, role, modo, token_hash) VALUES ($1,$2,$3,$4) RETURNING id`,
			tag+role, role, modo, terminal.HashToken(token)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		terminalIDs = append(terminalIDs, id)
		return id, token
	}
	regID, regTok := mkTerm("registro", "verificar")
	otherID, otherTok := mkTerm("registro", "emitir")
	shopID, _ := mkTerm("comercio", "verificar")

	call(emisor, "POST", "/api/emision/order", "", map[string]any{"ciudadanoId": anaID, "terminalId": shopID}, 400) // no es de registro
	if s := call(emisor, "GET", "/api/emision/status", "", nil, 200); s["estado"] != "error" {
		t.Fatalf("estado sin orden: %v", s)
	}
	call(emisor, "POST", "/api/emision/order", "", map[string]any{"ciudadanoId": anaID, "terminalId": regID}, 201)
	if s := call(emisor, "GET", fmt.Sprintf("/api/emision/status?ciudadanoId=%d", anaID), "", nil, 200); s["estado"] != "pendiente" {
		t.Fatalf("estado pendiente: %v", s)
	}
	if c := call(newClient(), "GET", "/api/terminal/config", regTok, nil, 200); c["modo"] != "emitir" {
		t.Fatalf("la orden debía poner el terminal en modo emitir: %v", c)
	}
	// Otro terminal de registro no puede tomar una orden dirigida al primero.
	call(newClient(), "POST", "/api/emision/start", otherTok, map[string]any{"uid": "04A1B2C3"}, 409)
	em := call(newClient(), "POST", "/api/emision/start", regTok, map[string]any{"uid": "04A1B2C3"}, 200)
	handleHex := em["h"].(string)
	if s := call(emisor, "GET", "/api/emision/status", "", nil, 200); s["estado"] != "completado" {
		t.Fatalf("estado completado: %v", s)
	}
	_ = otherID

	// /api/card: activa, con los atributos presentes y sus valores.
	c1 := call(ana, "GET", "/api/card", "", nil, 200)
	if c1["estado"] != "activa" || c1["emitidaEn"] == nil {
		t.Fatalf("card: %v", c1)
	}
	atrs := c1["atributos"].([]any)
	a1, a3, a4 := atrs[0].(map[string]any), atrs[2].(map[string]any), atrs[3].(map[string]any)
	if a1["presente"] != true || a1["valor"] != "Sí" || a3["valor"] != "ANA PEREZ|V-1|2030-01-01" || a4["valor"] != "O+ penicilina" ||
		atrs[1].(map[string]any)["codigo"] != "A2" {
		t.Fatalf("atributos: %v", atrs)
	}
	for _, c := range call(emisor, "GET", "/api/citizens", "", nil, 200)["citizens"].([]any) {
		if c.(map[string]any)["id"] == float64(anaID) && c.(map[string]any)["estadoCedula"] != "activa" {
			t.Fatalf("estadoCedula: %v", c)
		}
	}

	// Revocar y reactivar sin cuerpo (por sesión) y por ciudadanoId (emisor).
	call(ana, "POST", "/api/revoke", "", nil, 200)
	if c := call(ana, "GET", "/api/card", "", nil, 200); c["estado"] != "revocada" {
		t.Fatalf("tras revocar: %v", c)
	}
	call(ana, "POST", "/api/revoke", "", nil, 404) // ya no hay activas
	call(ana, "POST", "/api/reactivate", "", nil, 200)
	if c := call(ana, "GET", "/api/card", "", nil, 200); c["estado"] != "activa" {
		t.Fatalf("tras reactivar: %v", c)
	}
	call(emisor, "POST", "/api/revoke", "", map[string]any{"ciudadanoId": anaID}, 200)
	call(emisor, "POST", "/api/revoke", "", nil, 400) // el emisor debe indicar a quién
	call(emisor, "POST", "/api/reactivate", "", map[string]any{"ciudadanoId": anaID}, 200)
	_ = handleHex

	// Comercio: CRUD de terminales (token una sola vez) y estadísticas.
	tienda, tiendaID := mkUser("tienda", "comercio")
	call(ana, "POST", "/api/terminals", "", map[string]any{"nombre": "X"}, 403)
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": ""}, 400)
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja", "atributos": []string{"A4"}}, 400) // un comercio no pide salud
	cr := call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja 1", "atributos": []string{"A1", "A2"}}, 201)
	term := cr["terminal"].(map[string]any)
	token := cr["token"].(string)
	tid := int64(term["id"].(float64))
	if len(token) != 32 || term["token"] != token || term["nombre"] != "Caja 1" || term["a"] != "A1" || len(term["atributos"].([]any)) != 2 {
		t.Fatalf("terminal creado: %v", cr)
	}
	list := call(tienda, "GET", "/api/terminals", "", nil, 200)
	if b, _ := json.Marshal(list); strings.Contains(string(b), token) || len(list["terminals"].([]any)) != 1 {
		t.Fatalf("el token reapareció o la lista no es la propia: %s", b)
	}
	cfg := call(newClient(), "GET", "/api/terminal/config", token, nil, 200)
	if cfg["a"] != "A1" || cfg["local"] != "Caja 1" || len(cfg["atributos"].([]any)) != 2 {
		t.Fatalf("config del terminal nuevo: %v", cfg)
	}
	call(tienda, "PUT", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"nombre": "Caja Uno", "atributos": []string{"A2"}}, 200)
	call(tienda, "PATCH", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"modo": "emitir"}, 403)
	otra, _ := mkUser("otra", "comercio")
	call(otra, "PUT", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"nombre": "Robado"}, 403)
	call(otra, "DELETE", fmt.Sprintf("/api/terminals/%d", tid), "", nil, 403)
	if l := call(otra, "GET", "/api/terminals", "", nil, 200)["terminals"].([]any); len(l) != 0 {
		t.Fatalf("otra tienda ve %d terminales ajenos", len(l))
	}
	call(newClient(), "POST", "/api/events", token, map[string]any{"t": "verif", "h": handleHex, "a": "A2", "res": "01", "ts": time.Now().Unix()}, 200)
	call(newClient(), "POST", "/api/events", token, map[string]any{"t": "verif", "h": handleHex, "a": "A2", "res": "02", "ts": time.Now().Unix()}, 200)
	stats := call(tienda, "GET", "/api/stats", "", nil, 200)["terminals"].([]any)
	if len(stats) != 1 || stats[0].(map[string]any)["hoy"] != float64(2) || stats[0].(map[string]any)["aprobadas"] != float64(1) ||
		stats[0].(map[string]any)["rechazadas"] != float64(1) {
		t.Fatalf("stats: %v", stats)
	}
	call(ana, "GET", "/api/stats", "", nil, 403)
	call(tienda, "DELETE", fmt.Sprintf("/api/terminals/%d", tid), "", nil, 200)
	call(newClient(), "GET", "/api/terminal/config", token, nil, 401) // el token murió con el terminal
	_ = tiendaID
}
