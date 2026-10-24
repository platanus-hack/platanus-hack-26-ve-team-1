package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// demoState se guarda en secrets/demo.json para que la contraseña y los tokens sean estables
// entre restauraciones (P2 no tiene que reconfigurar sus terminales).
type demoState struct {
	Password string            `json:"password"`
	Tokens   map[string]string `json:"tokens"`
}

type demoUser struct {
	email, name, role string
	profile           map[string]any
}

type demoTerminal struct {
	name, role, attr, modo, owner string
}

var demoUsers = []demoUser{
	{"emisor@sisa.demo", "Registro Civil", "emisor", nil},
	{"comercio@sisa.demo", "Bar La Esquina", "comercio", nil},
	{"ana@sisa.demo", "Ana Pérez", "ciudadano", map[string]any{
		"birthdate": "2000-03-04",
		"identity":  map[string]any{"name": "ANA PEREZ", "id": "V-12345678", "valid_until": "2030-01-01"},
		"carnet":    map[string]any{"name": "ANA PEREZ", "id": "UCV-2020-0451", "valid_until": "2027-07-31"},
		"blood":     "O+ penicilina", "contact": "Mama 04141234567", "donor_license": "D;LIC-B2",
	}},
	{"luis@sisa.demo", "Luis Gómez", "ciudadano", map[string]any{
		"birthdate": "2006-05-20", // 20 años: mayor de 18, no de 21
		"identity":  map[string]any{"name": "LUIS GOMEZ", "id": "V-23456789", "valid_until": "2030-01-01"},
		"carnet":    map[string]any{"name": "LUIS GOMEZ", "id": "UCV-2024-1187", "valid_until": "2027-07-31"},
		"blood":     "A- ninguna", "contact": "Papa 04161234567",
	}},
	{"maria@sisa.demo", "María López", "ciudadano", map[string]any{
		"birthdate": "2010-08-15", // menor de edad: no recibe A1 ni A2
		"identity":  map[string]any{"name": "MARIA LOPEZ", "id": "V-30123456", "valid_until": "2030-01-01"},
		"blood":     "B+ polen", "contact": "Mama 04121234567",
	}},
}

var demoTerminals = []demoTerminal{
	{"Registro", "registro", "", "emitir", ""},
	{"Bar La Esquina", "comercio", "A1", "verificar", "comercio@sisa.demo"},
	{"Clinica Salud", "salud", "A4", "verificar", ""},
	{"Ambulancia 1", "emergencia", "", "emergencia", ""},
	{"Transito", "transito", "A6", "verificar", ""},
}

// demo siembra o restaura los datos de demo: cuentas, perfiles y terminales con tokens estables.
// Borra cédulas, credenciales, eventos y códigos de las cuentas demo; no toca a nadie más.
func demo(ctx context.Context, db *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	phones := map[string]*string{
		"ana@sisa.demo":   fs.String("ana", "", "teléfono de WhatsApp de Ana"),
		"luis@sisa.demo":  fs.String("luis", "", "teléfono de WhatsApp de Luis"),
		"maria@sisa.demo": fs.String("maria", "", "teléfono de WhatsApp de María"),
	}
	historial := fs.Bool("historial", false, "siembra cédulas e historial de ejemplo (Ana con 3 eventos, Luis con 1, María revocada)")
	fs.Parse(args)

	root, err := repoutil.Root()
	if err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(root, "secrets", "demo.json")
	st := demoState{Tokens: map[string]string{}}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &st)
	}
	if st.Tokens == nil {
		st.Tokens = map[string]string{}
	}
	rnd := func() string {
		b := make([]byte, 16)
		rand.Read(b)
		return hex.EncodeToString(b)
	}
	if st.Password == "" {
		st.Password = rnd()
	}
	for _, t := range demoTerminals {
		if st.Tokens[t.name] == "" {
			st.Tokens[t.name] = rnd()
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Fatal(err)
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		log.Fatal(err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(st.Password), bcrypt.DefaultCost)
	ids := map[string]int64{}
	var citizenIDs []int64
	for _, u := range demoUsers {
		var ph *string
		if p := phones[u.email]; p != nil {
			if n := kapso.NormalizePhone(*p); n != "" {
				ph = &n
			}
		}
		profile := []byte("{}")
		if u.profile != nil {
			profile, _ = json.Marshal(u.profile)
		}
		var id int64
		err := db.QueryRow(ctx,
			`INSERT INTO citizens (email, password_hash, role, name, profile, whatsapp_phone, notifications) VALUES ($1,$2,$3,$4,$5,$6,TRUE)
			 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role, name = EXCLUDED.name,
			   profile = EXCLUDED.profile, notifications = TRUE, whatsapp_phone = coalesce(EXCLUDED.whatsapp_phone, citizens.whatsapp_phone)
			 RETURNING id`, u.email, string(hash), u.role, u.name, profile, ph).Scan(&id)
		if err != nil {
			log.Fatalf("cuenta %s: %v", u.email, err)
		}
		ids[u.email] = id
		citizenIDs = append(citizenIDs, id)
	}

	var termIDs []int64
	for _, t := range demoTerminals {
		attr := []int16{}
		if t.attr != "" {
			attr = []int16{int16(t.attr[1] - '0')}
		}
		var owner *int64
		if t.owner != "" {
			o := ids[t.owner]
			owner = &o
		}
		var id int64
		err := db.QueryRow(ctx, `SELECT id FROM terminals WHERE name = $1 ORDER BY id LIMIT 1`, t.name).Scan(&id)
		if err == nil {
			_, err = db.Exec(ctx, `UPDATE terminals SET role=$2, attributes=$3, modo=$4, owner_id=$5, token_hash=$6 WHERE id=$1`,
				id, t.role, attr, t.modo, owner, terminal.HashToken(st.Tokens[t.name]))
		} else {
			err = db.QueryRow(ctx,
				`INSERT INTO terminals (name, role, attributes, modo, owner_id, token_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
				t.name, t.role, attr, t.modo, owner, terminal.HashToken(st.Tokens[t.name])).Scan(&id)
		}
		if err != nil {
			log.Fatalf("terminal %s: %v", t.name, err)
		}
		termIDs = append(termIDs, id)
	}

	// Restaurar: lo que generó el uso de la demo.
	cards, _ := db.Exec(ctx, `DELETE FROM cards WHERE citizen_id = ANY($1)`, citizenIDs)
	events, _ := db.Exec(ctx, `DELETE FROM events WHERE terminal_id = ANY($1)`, termIDs)
	reqs, _ := db.Exec(ctx, `DELETE FROM issuance_requests WHERE citizen_id = ANY($1)`, citizenIDs)
	devs, _ := db.Exec(ctx, `DELETE FROM devices WHERE citizen_id = ANY($1)`, citizenIDs)

	// Cuentas de una versión anterior de la demo.
	for _, old := range []string{"emisor", "comercio", "ana", "luis", "maria"} {
		db.Exec(ctx, `DELETE FROM cards WHERE citizen_id IN (SELECT id FROM citizens WHERE email = $1)`, old+"@slj.test")
		db.Exec(ctx, `DELETE FROM citizens WHERE email = $1`, old+"@slj.test")
	}
	if *historial {
		seedHistory(ctx, db, ids, termIDs)
	}

	fmt.Printf("demo lista: %d cuentas, %d terminales\n", len(demoUsers), len(demoTerminals))
	fmt.Printf("restaurado: %d cédulas, %d eventos, %d solicitudes y %d dispositivos borrados\n",
		cards.RowsAffected(), events.RowsAffected(), reqs.RowsAffected(), devs.RowsAffected())
	fmt.Printf("contraseña y tokens en %s (no se muestran)\n", path)
}

// seedHistory crea cédulas de ejemplo (con UID inventado) y eventos para probar el portal sin hardware.
// Para emitir una tarjeta física hay que restaurar sin esta opción: la emisión real crea su propia cédula.
func seedHistory(ctx context.Context, db *pgxpool.Pool, ids map[string]int64, termIDs []int64) {
	term := map[string]int64{}
	for i, t := range demoTerminals {
		term[t.name] = termIDs[i]
	}
	card := func(email string, revoked bool) []byte {
		h := make([]byte, 8)
		rand.Read(h)
		status := "activa"
		if revoked {
			status = "revocada"
		}
		if _, err := db.Exec(ctx,
			`INSERT INTO cards (handle, uid, citizen_id, status, revoked_at) VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'revocada' THEN now() END)`,
			h, []byte{0xDE, 0xAD, 0xBE, 0xEF}, ids[email], status); err != nil {
			log.Fatalf("historial: %v", err)
		}
		for _, u := range demoUsers {
			if u.email != email {
				continue
			}
			var p api.Profile
			raw, _ := json.Marshal(u.profile)
			json.Unmarshal(raw, &p)
			attrs, _ := p.Attributes(time.Now())
			for _, a := range attrs {
				if _, err := db.Exec(ctx,
					`INSERT INTO attributes (handle, code, issuer, value, expires_at) VALUES ($1,$2,1,$3,now() + interval '365 days')`,
					h, int(a.Code), a.Value); err != nil {
					log.Fatalf("historial: %v", err)
				}
			}
		}
		return h
	}
	ana, luis, maria := card("ana@sisa.demo", false), card("luis@sisa.demo", false), card("maria@sisa.demo", true)
	_ = maria
	ev := func(h []byte, terminalName string, attr, res int, emg bool, minutesAgo int) {
		if _, err := db.Exec(ctx,
			`INSERT INTO events (kind, handle, terminal_id, attribute, result, emergency, created_at)
			 VALUES ('verif', $1, $2, $3, $4, $5, now() - make_interval(mins => $6))`,
			h, term[terminalName], attr, res, emg, minutesAgo); err != nil {
			log.Fatalf("historial: %v", err)
		}
	}
	ev(ana, "Bar La Esquina", 1, 1, false, 180)
	ev(ana, "Bar La Esquina", 2, 1, false, 120)
	ev(ana, "Ambulancia 1", 4, 1, true, 30)
	ev(luis, "Bar La Esquina", 1, 1, false, 60)
}
