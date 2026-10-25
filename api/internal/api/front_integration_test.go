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
	callList := func(c *http.Client, path string, want int) []any {
		t.Helper()
		resp, err := c.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out []any
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("GET %s: %d (esperaba %d): %s", path, resp.StatusCode, want, raw)
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("GET %s no devolvió un array: %s", path, raw)
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
	anaID, _ := atoi(reg["id"].(string)) // el front espera el id como texto
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

	// Sin teléfono: telefono es null y no se pueden activar avisos.
	solo := newClient()
	sreg := call(solo, "POST", "/api/auth/register", "", map[string]any{"nombre": "Solo", "email": tag + "solo@test.dev", "password": "clave-segura-1"}, 201)
	soloID, _ := atoi(sreg["id"].(string))
	citizenIDs = append(citizenIDs, soloID)
	if v, present := sreg["telefono"]; !present || v != nil {
		t.Fatalf("telefono debía ser null: %v", sreg)
	}
	call(solo, "PUT", "/api/me/notifications", "", map[string]any{"telefono": nil, "avisosActivados": true}, 400)
	nn := call(solo, "PUT", "/api/me/notifications", "", map[string]any{"telefono": nil, "avisosActivados": false}, 200)
	if nn["telefono"] != nil || nn["avisosActivados"] != false {
		t.Fatalf("notificaciones sin teléfono: %v", nn)
	}
	call(solo, "PUT", "/api/me/notifications", "", map[string]any{"telefono": "123", "avisosActivados": false}, 400)

	// Sin cédula todavía: 404.
	call(ana, "GET", "/api/card", "", nil, 404)

	// Datos opcionales (forma del front): se validan (16 B) y alimentan A4/A5/A6.
	opt := map[string]any{"tipoSangre": "O+", "alergias": "penicilina",
		"contactoEmergencia": map[string]any{"nombre": "Mama", "telefono": "+584141234567"}, "donante": true}
	call(ana, "PUT", "/api/me/optional-data", "", map[string]any{"tipoSangre": "XX", "alergias": ""}, 400)
	call(ana, "PUT", "/api/me/optional-data", "", map[string]any{"tipoSangre": "O+", "alergias": "una alergia demasiado larga"}, 400)
	call(ana, "PUT", "/api/me/optional-data", "", map[string]any{"tipoSangre": "O+", "alergias": "",
		"contactoEmergencia": map[string]any{"nombre": "Mamá Muy Querida", "telefono": "+584141234567"}}, 400) // 16 B de nombre + teléfono no caben
	call(ana, "PUT", "/api/me/optional-data", "", map[string]any{"tipoSangre": "O+", "alergias": "",
		"contactoEmergencia": map[string]any{"nombre": "Mama", "telefono": ""}}, 400) // nombre sin teléfono
	got := call(ana, "PUT", "/api/me/optional-data", "", opt, 200)
	if got["tipoSangre"] != "O+" || got["contactoEmergencia"].(map[string]any)["telefono"] != "+584141234567" || got["donante"] != true {
		t.Fatalf("optional-data: %v", got)
	}
	if g := call(ana, "GET", "/api/me/optional-data", "", nil, 200); g["alergias"] != "penicilina" || g["tipoSangre"] != "O+" {
		t.Fatalf("GET optional-data: %v", g)
	}
	if g := call(solo, "GET", "/api/me/optional-data", "", nil, 200); g["tipoSangre"] != nil || g["donante"] != false {
		t.Fatalf("sin datos guardados: %v", g)
	}

	// Emisor: completa el perfil con {fechaNacimiento, cedula}; sin él no se puede emitir.
	emisor, _ := mkUser("emisor", "emisor")
	cit := callList(emisor, "/api/citizens", 200)
	find := func(list []any, id int64) map[string]any {
		for _, c := range list {
			if c.(map[string]any)["id"] == itoa(id) {
				return c.(map[string]any)
			}
		}
		return nil
	}
	row := find(cit, anaID)
	if row == nil || row["nombre"] != "Ana" || row["email"] != tag+"ana@test.dev" || row["estadoCedula"] != "sin_emitir" ||
		row["emitidaEn"] != nil || row["perfilCompleto"] != false || len(row) != 6 {
		t.Fatalf("citizens antes del perfil: %v", row)
	}
	call(emisor, "POST", "/api/emision/order", "", map[string]any{"ciudadanoId": itoa(anaID)}, 409) // perfil incompleto
	call(emisor, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", map[string]any{"fechaNacimiento": "2999-01-01", "cedula": "12345678"}, 400)
	call(emisor, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", map[string]any{"fechaNacimiento": "2000-03-04", "cedula": "123"}, 400)
	call(ana, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", map[string]any{"fechaNacimiento": "2000-03-04", "cedula": "12345678"}, 403)
	call(emisor, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", map[string]any{"fechaNacimiento": "2000-03-04", "cedula": "12345678"}, 200)
	// El perfil del emisor no pisa lo que declaró el ciudadano.
	if g := call(ana, "GET", "/api/me/optional-data", "", nil, 200); g["tipoSangre"] != "O+" {
		t.Fatalf("el perfil del emisor borró los datos opcionales: %v", g)
	}
	cit = callList(emisor, "/api/citizens", 200)
	if row := find(cit, anaID); row == nil || row["perfilCompleto"] != true {
		t.Fatalf("perfilCompleto: %v", row)
	}
	for _, banned := range []string{"handle", "uid", `"h"`, "fechaNacimiento", "birthdate", "12345678", "2000-03-04"} {
		if b, _ := json.Marshal(cit); strings.Contains(string(b), banned) {
			t.Fatalf("citizens contiene %q", banned)
		}
	}

	// Estudiantes: el carnet se deriva de la identidad y se emite al vincular el celular.
	st := callList(emisor, "/api/students", 200)
	stu := find(st, anaID)
	if stu == nil || stu["estadoCarnet"] != "sin_carnet" || stu["emitidoEn"] != nil || stu["email"] != tag+"ana@test.dev" {
		t.Fatalf("students: %v", stu)
	}
	if r := call(emisor, "POST", "/api/credentials/university", "", map[string]any{"estudianteId": itoa(anaID)}, 200); r["pendiente"] != true {
		t.Fatalf("sin celular vinculado debía quedar pendiente: %v", r)
	}

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

	call(emisor, "POST", "/api/emision/order", "", map[string]any{"ciudadanoId": itoa(anaID), "terminalId": itoa(shopID)}, 400) // no es de registro
	if s := call(emisor, "GET", "/api/emision/status", "", nil, 200); s["estado"] != "error" {
		t.Fatalf("estado sin orden: %v", s)
	}
	ord := call(emisor, "POST", "/api/emision/order", "", map[string]any{"ciudadanoId": itoa(anaID), "terminalId": itoa(regID)}, 201)
	if s := call(emisor, "GET", fmt.Sprintf("/api/emision/status?order=%v", ord["id"]), "", nil, 200); s["estado"] != "pendiente" {
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

	// /api/card: {id, estado, emitidaEn, atributos:[nombres]}; el carnet no es un atributo del chip.
	c1 := call(ana, "GET", "/api/card", "", nil, 200)
	if c1["estado"] != "activa" || c1["emitidaEn"] == nil || c1["id"] == nil {
		t.Fatalf("card: %v", c1)
	}
	want := []any{"mayor_de_18", "mayor_de_21", "identidad", "sangre_alergias", "contacto_emergencia", "donante_licencia"}
	if fmt.Sprint(c1["atributos"]) != fmt.Sprint(want) {
		t.Fatalf("atributos: %v", c1["atributos"])
	}
	for _, c := range callList(emisor, "/api/citizens", 200) {
		if c.(map[string]any)["id"] == itoa(anaID) && c.(map[string]any)["estadoCedula"] != "activa" {
			t.Fatalf("estadoCedula: %v", c)
		}
	}

	// Revocar y reactivar sin cuerpo (por sesión) y por ciudadanoId (emisor).
	call(ana, "POST", "/api/revoke", "", nil, 200)
	if c := call(ana, "GET", "/api/card", "", nil, 200); c["estado"] != "revocada" || len(c["atributos"].([]any)) != 0 {
		t.Fatalf("tras revocar: %v", c)
	}
	call(ana, "POST", "/api/revoke", "", nil, 404) // ya no hay activas
	call(ana, "POST", "/api/reactivate", "", nil, 200)
	if c := call(ana, "GET", "/api/card", "", nil, 200); c["estado"] != "activa" {
		t.Fatalf("tras reactivar: %v", c)
	}
	call(ana, "POST", "/api/revoke", "", map[string]any{"ciudadanoId": itoa(anaID)}, 403) // solo el emisor indica a quién
	call(emisor, "POST", "/api/revoke", "", map[string]any{"ciudadanoId": itoa(anaID)}, 200)
	call(emisor, "POST", "/api/revoke", "", nil, 400) // el emisor debe indicar a quién
	call(emisor, "POST", "/api/reactivate", "", map[string]any{"ciudadanoId": itoa(anaID)}, 200)
	_ = handleHex

	// Comercio: terminales (forma del front, token una sola vez) y estadísticas.
	tienda, _ := mkUser("tienda", "comercio")
	call(ana, "POST", "/api/terminals", "", map[string]any{"nombre": "X", "rol": "comercio"}, 403)
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "", "rol": "comercio"}, 400)
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja", "rol": "comercio", "atributos": []string{"sangre_alergias"}}, 400)
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja", "rol": "comercio", "atributos": []string{"mayor_de_18", "mayor_de_21"}}, 400) // un solo dato
	call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja", "rol": "transito"}, 400)
	term := call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Caja 1", "rol": "comercio", "atributos": []string{"mayor_de_21"}}, 201)
	token, _ := term["token"].(string)
	tid, _ := atoi(term["id"].(string))
	if len(token) != 32 || term["nombre"] != "Caja 1" || term["rol"] != "comercio" || fmt.Sprint(term["atributos"]) != "[mayor_de_21]" {
		t.Fatalf("terminal creado: %v", term)
	}
	list := callList(tienda, "/api/terminals", 200)
	if b, _ := json.Marshal(list); strings.Contains(string(b), token) || len(list) != 1 {
		t.Fatalf("el token reapareció o la lista no es la propia: %s", b)
	}
	cfg := call(newClient(), "GET", "/api/terminal/config", token, nil, 200)
	if cfg["a"] != "A2" || cfg["local"] != "Caja 1" || cfg["modo"] != "verificar" {
		t.Fatalf("config del terminal nuevo: %v", cfg)
	}
	// Terminales de salud y emergencia, con sus datos por defecto.
	salud := call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Clínica", "rol": "salud"}, 201)
	if fmt.Sprint(salud["atributos"]) != "[sangre_alergias contacto_emergencia donante_licencia]" {
		t.Fatalf("salud: %v", salud)
	}
	emer := call(tienda, "POST", "/api/terminals", "", map[string]any{"nombre": "Ambulancia", "rol": "emergencia"}, 201)
	if emer["modo"] != "emergencia" || fmt.Sprint(emer["atributos"]) != "[sangre_alergias contacto_emergencia]" {
		t.Fatalf("emergencia: %v", emer)
	}
	upd := call(tienda, "PUT", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"nombre": "Caja Uno", "rol": "comercio", "atributos": []string{"mayor_de_18"}}, 200)
	if upd["nombre"] != "Caja Uno" || upd["token"] != nil || fmt.Sprint(upd["atributos"]) != "[mayor_de_18]" {
		t.Fatalf("terminal editado: %v", upd)
	}
	call(tienda, "PATCH", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"modo": "emitir"}, 403)
	otra, _ := mkUser("otra", "comercio")
	call(otra, "PUT", fmt.Sprintf("/api/terminals/%d", tid), "", map[string]any{"nombre": "Robado"}, 403)
	call(otra, "DELETE", fmt.Sprintf("/api/terminals/%d", tid), "", nil, 403)
	if l := callList(otra, "/api/terminals", 200); len(l) != 0 {
		t.Fatalf("otra tienda ve %d terminales ajenos", len(l))
	}
	call(newClient(), "POST", "/api/events", token, map[string]any{"t": "verif", "h": handleHex, "a": "A2", "res": "01", "ts": time.Now().Unix()}, 200)
	call(newClient(), "POST", "/api/events", token, map[string]any{"t": "verif", "h": handleHex, "a": "A2", "res": "02", "ts": time.Now().Unix()}, 200)
	stats := callList(tienda, "/api/stats", 200)
	var mine map[string]any
	for _, x := range stats {
		if x.(map[string]any)["terminalId"] == itoa(tid) {
			mine = x.(map[string]any)
		}
	}
	if len(stats) != 3 || mine == nil || mine["hoy"] != float64(2) || mine["aprobadas"] != float64(1) || mine["rechazadas"] != float64(1) {
		t.Fatalf("stats: %v", stats)
	}
	call(ana, "GET", "/api/stats", "", nil, 403)
	call(tienda, "DELETE", fmt.Sprintf("/api/terminals/%d", tid), "", nil, 200)
	call(newClient(), "GET", "/api/terminal/config", token, nil, 401) // el token murió con el terminal
}

// CORS con credenciales, cookie entre sitios y protección contra peticiones de otros orígenes.
func TestCORSAndCookie(t *testing.T) {
	front := "https://front.example.com"
	h := CORS([]string{front + "/"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))

	do := func(method, origin string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/events", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	pre := do("OPTIONS", front, map[string]string{"Access-Control-Request-Method": "PUT"})
	if pre.Code != 204 || pre.Header().Get("Access-Control-Allow-Origin") != front ||
		pre.Header().Get("Access-Control-Allow-Credentials") != "true" ||
		!strings.Contains(pre.Header().Get("Access-Control-Allow-Methods"), "PUT") ||
		!strings.Contains(pre.Header().Get("Access-Control-Allow-Headers"), "Content-Type") ||
		pre.Header().Get("Vary") != "Origin" {
		t.Fatalf("preflight: %d %v", pre.Code, pre.Header())
	}
	get := do("GET", front, nil)
	if get.Code != 200 || get.Header().Get("Access-Control-Allow-Origin") != front || get.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("GET permitido: %d %v", get.Code, get.Header())
	}
	if other := do("GET", "https://evil.example.com", nil); other.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("origen no permitido recibió cabeceras CORS: %v", other.Header())
	}
	if none := do("GET", "", nil); none.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("sin Origin no debe haber cabeceras CORS")
	}

	// Cookie: SameSite=None solo si el front permitido está en otro sitio y hay HTTPS.
	srv := &Server{JWTSecret: []byte("secreto-de-prueba-123456"), AllowedOrigins: []string{front}}
	cookie := func(origin, proto string) *http.Cookie {
		scheme := "http" // sin HTTPS directo, solo cuenta el reenvío del proxy
		req := httptest.NewRequest("POST", scheme+"://api.example.com/api/auth/login", nil)
		req.Host = "api.example.com"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if proto != "" {
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		rec := httptest.NewRecorder()
		if err := srv.setCookie(rec, req, 1); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	if c := cookie(front, "https"); c.SameSite != http.SameSiteNoneMode || !c.Secure || !c.HttpOnly {
		t.Fatalf("cookie entre sitios: %+v", c)
	}
	if c := cookie("", "https"); c.SameSite != http.SameSiteLaxMode || !c.Secure || !c.HttpOnly {
		t.Fatalf("cookie del mismo origen: %+v", c)
	}
	if c := cookie("https://evil.example.com", "https"); c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("origen no permitido debe quedar en Lax: %+v", c)
	}
	if c := cookie(front, ""); c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Fatalf("sin HTTPS no puede ser SameSite=None: %+v", c)
	}

	// Una petición con cookie que cambia datos y viene de otro origen se rechaza.
	passed := false
	guarded := srv.session(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { passed = true }))
	req := httptest.NewRequest("POST", "https://api.example.com/api/revoke", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	if rec.Code != 403 || passed {
		t.Fatalf("origen ajeno debía dar 403: %d", rec.Code)
	}
}
