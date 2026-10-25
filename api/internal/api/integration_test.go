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

// Prueba de punta a punta contra la base real. Se activa con SLJ_INTEGRATION=1 y limpia lo que crea.
func TestIntegration(t *testing.T) {
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
	t.Cleanup(db.Close) // se registra antes que la limpieza, así corre después

	issuerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	uniKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	srv := &Server{DB: db, JWTSecret: []byte("secreto-de-prueba-123456"),
		Issuers: map[byte]*ecdsa.PrivateKey{sign.IssuerNacional: issuerKey, sign.IssuerUniversidad: uniKey}, ValidDays: 365}
	r := chi.NewRouter()
	srv.Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	tag := fmt.Sprintf("it%d", time.Now().UnixNano())
	var citizenIDs, terminalIDs []int64
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM events WHERE terminal_id = ANY($1)`, terminalIDs)
		db.Exec(ctx, `DELETE FROM cards WHERE citizen_id = ANY($1)`, citizenIDs)
		db.Exec(ctx, `DELETE FROM terminals WHERE id = ANY($1)`, terminalIDs)
		db.Exec(ctx, `DELETE FROM citizens WHERE id = ANY($1)`, citizenIDs)
	})

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
	mkTerminal := func(role, modo string) string {
		raw := make([]byte, 16)
		rand.Read(raw)
		token := hex.EncodeToString(raw)
		var id int64
		if err := db.QueryRow(ctx, `INSERT INTO terminals (name, role, modo, token_hash) VALUES ($1,$2,$3,$4) RETURNING id`,
			tag+role, role, modo, terminal.HashToken(token)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		terminalIDs = append(terminalIDs, id)
		return token
	}

	// Cuentas: Ana y Luis (registro), un emisor (insertado directo).
	ana, luis, emisor := newClient(), newClient(), newClient()
	a := call(ana, "POST", "/api/auth/register", "", map[string]any{"email": tag + "-ana@test.dev", "password": "clave-segura-1", "name": "Ana"}, 201)
	anaID, _ := atoi(a["user"].(map[string]any)["id"].(string))
	l := call(luis, "POST", "/api/auth/register", "", map[string]any{"email": tag + "-luis@test.dev", "password": "clave-segura-2", "name": "Luis"}, 201)
	luisID, _ := atoi(l["user"].(map[string]any)["id"].(string))
	hash, _ := bcrypt.GenerateFromPassword([]byte("clave-emisor-1"), bcrypt.MinCost)
	var emisorID int64
	db.QueryRow(ctx, `INSERT INTO citizens (email, password_hash, role, name) VALUES ($1,$2,'emisor','Emisor') RETURNING id`,
		tag+"-emisor@test.dev", string(hash)).Scan(&emisorID)
	citizenIDs = append(citizenIDs, anaID, luisID, emisorID)

	call(newClient(), "POST", "/api/auth/register", "", map[string]any{"email": tag + "-ana@test.dev", "password": "clave-segura-1"}, 409)
	call(newClient(), "POST", "/api/auth/login", "", map[string]any{"email": tag + "-ana@test.dev", "password": "mala"}, 401)
	call(emisor, "POST", "/api/auth/login", "", map[string]any{"email": tag + "-emisor@test.dev", "password": "clave-emisor-1"}, 200)
	call(newClient(), "GET", "/api/me", "", nil, 401)
	call(ana, "GET", "/api/citizens", "", nil, 403) // un ciudadano no es emisor

	// Emisor carga el perfil de Ana y pide la emisión.
	prof := map[string]any{"birthdate": "2000-03-04",
		"identity": map[string]any{"name": "ANA PEREZ", "id": "V-12345678", "valid_until": "2030-01-01"},
		"carnet":   map[string]any{"name": "ANA PEREZ", "id": "UCV-1", "valid_until": "2027-07-31"},
		"blood":    "O+ penicilina"}
	out := call(emisor, "PUT", fmt.Sprintf("/api/citizens/%d/profile", anaID), "", prof, 200)
	if fmt.Sprint(out["attributes"]) != "[A1 A2 A3 A4]" {
		t.Fatalf("atributos: %v", out["attributes"])
	}
	call(emisor, "POST", "/api/emision/request", "", map[string]any{"citizen_id": anaID}, 201)

	// Terminal de registro emite.
	reg := mkTerminal("registro", "emitir")
	call(newClient(), "POST", "/api/emision/start", "", map[string]any{"uid": "A1B2C3D4"}, 401)
	uidHex := "A1B2C3D4"
	em := call(newClient(), "POST", "/api/emision/start", reg, map[string]any{"uid": uidHex}, 200)
	call(newClient(), "POST", "/api/emision/start", reg, map[string]any{"uid": uidHex}, 409) // ya se consumió
	handleHex := em["h"].(string)

	// Reensambla los bloques y verifica la firma del chip con la clave pública del emisor.
	secs := map[int][]byte{}
	for _, b := range em["blocks"].([]any) {
		bm := b.(map[string]any)
		d, _ := hex.DecodeString(bm["d"].(string))
		s := int(bm["s"].(float64))
		secs[s] = append(secs[s], d...)
	}
	if len(secs) != 1+2+2+3+2 || secs[1][0] != 1 || hex.EncodeToString(secs[1][1:9]) != handleHex {
		t.Fatalf("sectores inesperados: %d", len(secs))
	}
	uid, _ := hex.DecodeString(uidHex)
	var rec []byte
	for s := 2; s <= 15; s++ {
		rec = append(rec, secs[s]...)
	}
	checked := 0
	for len(rec) >= sign.HeaderLen {
		at, sig, err := sign.ParseRecord(rec)
		if err != nil {
			t.Fatal(err)
		}
		if !at.Verify(&issuerKey.PublicKey, uid, sig) {
			t.Fatalf("firma inválida en A%d", at.Code)
		}
		rec = rec[sign.HeaderLen+sign.MaxValueLen(at.Code)+sign.SigLen:]
		checked++
	}
	if checked != 4 {
		t.Fatalf("se verificaron %d atributos", checked)
	}

	// Vinculación.
	link := mkTerminal("registro", "vincular")
	call(newClient(), "POST", "/api/link/start", reg, map[string]any{"h": handleHex}, 403) // modo emitir
	code := call(newClient(), "POST", "/api/link/start", link, map[string]any{"h": handleHex}, 200)["c"].(string)
	if len(code) != 6 {
		t.Fatalf("código: %q", code)
	}
	phone, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub := hex.EncodeToString(sign.CompressPub(&phone.PublicKey))
	call(luis, "POST", "/api/link/confirm", "", map[string]any{"code": code, "public_key": pub}, 403) // otra cuenta
	code = call(newClient(), "POST", "/api/link/start", link, map[string]any{"h": handleHex}, 200)["c"].(string)
	call(ana, "POST", "/api/link/confirm", "", map[string]any{"code": "000000", "public_key": pub}, 404)
	conf := call(ana, "POST", "/api/link/confirm", "", map[string]any{"code": code, "public_key": pub}, 200)
	call(ana, "POST", "/api/link/confirm", "", map[string]any{"code": code, "public_key": pub}, 404) // un solo uso
	if n := len(conf["credentials"].([]any)); n != 3 {
		t.Fatalf("credenciales: %d (esperaba A1, A2 y el carnet)", n)
	}

	// Billetera: Ana ve las suyas y se verifican; Luis no ve nada.
	w := call(ana, "GET", "/api/wallet", "", nil, 200)["credentials"].([]any)
	if len(w) != 3 {
		t.Fatalf("wallet: %d", len(w))
	}
	checkWallet := func(list []any) {
		t.Helper()
		carnets := 0
		for _, c := range list {
			cm := c.(map[string]any)
			blob, _ := hex.DecodeString(cm["blob_hex"].(string))
			at, p, sig, err := sign.ParseWalletBlob(blob)
			key := issuerKey
			if at.Issuer == sign.IssuerUniversidad {
				key = uniKey
				carnets++
				if at.Code != sign.AttrIdentidad || string(at.Value) != "ANA PEREZ|UCV-1|2027-07-31" || cm["issuer"] != "0x02" {
					t.Fatalf("carnet inesperado: %+v %v", at, cm)
				}
			}
			if err != nil || !at.VerifyWallet(&key.PublicKey, p, sig) || hex.EncodeToString(p) != pub {
				t.Fatalf("credencial de billetera inválida: %v", err)
			}
		}
		if carnets != 1 {
			t.Fatalf("carnets: %d", carnets)
		}
	}
	checkWallet(w)
	if n := len(call(luis, "GET", "/api/wallet", "", nil, 200)["credentials"].([]any)); n != 0 {
		t.Fatalf("Luis ve %d credenciales de Ana", n)
	}

	// Eventos.
	shop := mkTerminal("comercio", "verificar")
	call(newClient(), "POST", "/api/events", shop, map[string]any{"t": "verif", "h": handleHex, "a": "A1", "res": "01", "ts": time.Now().Unix(), "emg": false}, 200)
	call(newClient(), "POST", "/api/events", shop, map[string]any{"t": "tamper", "ts": time.Now().Unix()}, 200)
	call(newClient(), "POST", "/api/events", shop, map[string]any{"t": "verif", "h": "zz", "a": "A1", "res": "01"}, 400)
	call(newClient(), "POST", "/api/events", shop, map[string]any{"t": "verif", "h": handleHex, "a": "A9", "res": "01"}, 400)
	var nEv int
	db.QueryRow(ctx, `SELECT count(*) FROM events WHERE terminal_id = ANY($1)`, terminalIDs).Scan(&nEv)
	if nEv != 2 {
		t.Fatalf("eventos guardados: %d", nEv)
	}
	cfg := call(newClient(), "GET", "/api/terminal/config", shop, nil, 200)
	if cfg["rol"] != "comercio" || cfg["modo"] != "verificar" {
		t.Fatalf("config: %v", cfg)
	}

	// Revocación: Luis no puede; Ana sí; las credenciales desaparecen.
	call(luis, "POST", "/api/revoke", "", map[string]any{"h": handleHex}, 404)
	call(ana, "POST", "/api/revoke", "", map[string]any{"h": handleHex}, 200)
	call(ana, "POST", "/api/revoke", "", map[string]any{"h": handleHex}, 404)
	if n := len(call(ana, "GET", "/api/wallet", "", nil, 200)["credentials"].([]any)); n != 0 {
		t.Fatalf("quedan %d credenciales tras revocar", n)
	}
	resp, _ := http.Get(ts.URL + "/api/revocations")
	var revs struct{ Hs []string }
	json.NewDecoder(resp.Body).Decode(&revs)
	resp.Body.Close()
	if !strings.Contains(strings.Join(revs.Hs, ","), handleHex) {
		t.Fatal("el handle revocado no aparece en /api/revocations")
	}
	call(newClient(), "POST", "/api/link/start", link, map[string]any{"h": handleHex}, 409)

	// Mis cédulas: Ana ve la suya revocada; Luis ninguna.
	cs := call(ana, "GET", "/api/cards", "", nil, 200)["cards"].([]any)
	if len(cs) != 1 || cs[0].(map[string]any)["estado"] != "revocada" || cs[0].(map[string]any)["h"] != handleHex {
		t.Fatalf("cédulas de Ana: %v", cs)
	}
	if n := len(call(luis, "GET", "/api/cards", "", nil, 200)["cards"].([]any)); n != 0 {
		t.Fatalf("Luis ve %d cédulas", n)
	}

	// Reactivar: Luis no puede; Ana sí, y las credenciales (con el carnet) vuelven.
	call(luis, "POST", "/api/reactivate", "", map[string]any{"h": handleHex}, 404)
	call(newClient(), "POST", "/api/reactivate", "", map[string]any{"h": handleHex}, 401)
	if n := call(ana, "POST", "/api/reactivate", "", map[string]any{"h": handleHex}, 200)["credentials"]; n != float64(3) {
		t.Fatalf("credenciales reemitidas: %v", n)
	}
	call(ana, "POST", "/api/reactivate", "", map[string]any{"h": handleHex}, 404) // ya está activa
	checkWallet(call(ana, "GET", "/api/wallet", "", nil, 200)["credentials"].([]any))
	resp, _ = http.Get(ts.URL + "/api/revocations")
	revs.Hs = nil
	json.NewDecoder(resp.Body).Decode(&revs)
	resp.Body.Close()
	if strings.Contains(strings.Join(revs.Hs, ","), handleHex) {
		t.Fatal("la cédula reactivada sigue en /api/revocations")
	}

	// Carnet: renovar lo pide el emisor; un ciudadano no puede.
	call(ana, "POST", "/api/carnet/issue", "", map[string]any{"citizen_id": anaID}, 403)
	call(emisor, "POST", "/api/carnet/issue", "", map[string]any{"citizen_id": luisID}, 422) // sin perfil
	call(emisor, "POST", "/api/carnet/issue", "", map[string]any{"citizen_id": anaID}, 409)  // ya lo recibió al vincular
	if n := len(call(ana, "GET", "/api/wallet", "", nil, 200)["credentials"].([]any)); n != 3 {
		t.Fatalf("la renovación duplicó credenciales: %d", n)
	}

	// Terminales: el emisor cambia el modo y el terminal lo ve en su config.
	call(ana, "GET", "/api/terminals", "", nil, 403)
	var shopID int64
	db.QueryRow(ctx, `SELECT id FROM terminals WHERE name = $1`, tag+"comercio").Scan(&shopID)
	call(emisor, "PATCH", fmt.Sprintf("/api/terminals/%d", shopID), "", map[string]any{"rol": "emergencia", "atributos": []string{"sangre_alergias"}, "modo": "emergencia"}, 200)
	cfg2 := call(newClient(), "GET", "/api/terminal/config", shop, nil, 200)
	if cfg2["modo"] != "emergencia" || cfg2["a"] != "A4" {
		t.Fatalf("config tras PATCH: %v", cfg2)
	}
	call(emisor, "PATCH", fmt.Sprintf("/api/terminals/%d", shopID), "", map[string]any{"modo": "nada"}, 400)
}
