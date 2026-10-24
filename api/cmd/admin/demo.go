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

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

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
	{"emisor@slj.test", "Registro Civil", "emisor", nil},
	{"comercio@slj.test", "Bar La Esquina", "comercio", nil},
	{"ana@slj.test", "Ana Pérez", "ciudadano", map[string]any{
		"birthdate": "2000-03-04",
		"identity":  map[string]any{"name": "ANA PEREZ", "id": "V-12345678", "valid_until": "2030-01-01"},
		"carnet":    map[string]any{"name": "ANA PEREZ", "id": "UCV-2020-0451", "valid_until": "2027-07-31"},
		"blood":     "O+ penicilina", "contact": "Mama 04141234567", "donor_license": "D;LIC-B2",
	}},
	{"luis@slj.test", "Luis Gómez", "ciudadano", map[string]any{
		"birthdate": "2006-05-20", // 20 años: mayor de 18, no de 21
		"identity":  map[string]any{"name": "LUIS GOMEZ", "id": "V-23456789", "valid_until": "2030-01-01"},
		"blood":     "A- ninguna", "contact": "Papa 04161234567",
	}},
	{"maria@slj.test", "María López", "ciudadano", map[string]any{
		"birthdate": "2010-08-15", // menor de edad: no recibe A1 ni A2
		"identity":  map[string]any{"name": "MARIA LOPEZ", "id": "V-30123456", "valid_until": "2030-01-01"},
		"blood":     "B+ polen", "contact": "Mama 04121234567",
	}},
}

var demoTerminals = []demoTerminal{
	{"Registro", "registro", "", "emitir", ""},
	{"Bar La Esquina", "comercio", "A1", "verificar", "comercio@slj.test"},
	{"Clinica Salud", "salud", "A4", "verificar", ""},
	{"Ambulancia 1", "emergencia", "", "emergencia", ""},
	{"Transito", "transito", "A6", "verificar", ""},
}

// demo siembra o restaura los datos de demo: cuentas, perfiles y terminales con tokens estables.
// Borra cédulas, credenciales, eventos y códigos de las cuentas demo; no toca a nadie más.
func demo(ctx context.Context, db *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	phones := map[string]*string{
		"ana@slj.test":   fs.String("ana", "", "teléfono de WhatsApp de Ana"),
		"luis@slj.test":  fs.String("luis", "", "teléfono de WhatsApp de Luis"),
		"maria@slj.test": fs.String("maria", "", "teléfono de WhatsApp de María"),
	}
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
		var attr *int
		if t.attr != "" {
			n := int(t.attr[1] - '0')
			attr = &n
		}
		var owner *int64
		if t.owner != "" {
			o := ids[t.owner]
			owner = &o
		}
		var id int64
		err := db.QueryRow(ctx, `SELECT id FROM terminals WHERE name = $1 ORDER BY id LIMIT 1`, t.name).Scan(&id)
		if err == nil {
			_, err = db.Exec(ctx, `UPDATE terminals SET role=$2, attribute=$3, modo=$4, owner_id=$5, token_hash=$6 WHERE id=$1`,
				id, t.role, attr, t.modo, owner, terminal.HashToken(st.Tokens[t.name]))
		} else {
			err = db.QueryRow(ctx,
				`INSERT INTO terminals (name, role, attribute, modo, owner_id, token_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
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

	fmt.Printf("demo lista: %d cuentas, %d terminales\n", len(demoUsers), len(demoTerminals))
	fmt.Printf("restaurado: %d cédulas, %d eventos, %d solicitudes y %d dispositivos borrados\n",
		cards.RowsAffected(), events.RowsAffected(), reqs.RowsAffected(), devs.RowsAffected())
	fmt.Printf("contraseña y tokens en %s (no se muestran)\n", path)
}
