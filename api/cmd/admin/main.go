// admin crea terminales (con su token) y usuarios con rol emisor o comercio.
//
//	admin terminal -name "Bar La Esquina" -role comercio -attr A1 -modo verificar
//	admin demo [-ana +58... -luis +58... -maria +58...]   siembra o restaura los datos de demo
//	admin user -email emisor@slj.test -password ... -role emisor -name "Registro Civil"
//
// Usa DATABASE_URL del entorno o la de .env.local.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("uso: admin terminal|user [opciones]")
	}
	url, err := repoutil.DatabaseURL()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal("no se pudo conectar a la base")
	}
	defer db.Close()

	args := os.Args[2:]
	switch os.Args[1] {
	case "terminal":
		fs := flag.NewFlagSet("terminal", flag.ExitOnError)
		name := fs.String("name", "", "nombre del local")
		role := fs.String("role", "comercio", "comercio|registro|salud|transito|emergencia")
		attr := fs.String("attr", "", "atributo que pide (A1..A6), opcional")
		modo := fs.String("modo", "verificar", "verificar|emergencia|emitir|vincular")
		owner := fs.String("owner", "", "correo del dueño (recibe la alerta de manipulación), opcional")
		fs.Parse(args)
		if *name == "" {
			log.Fatal("falta -name")
		}
		a := []int16{}
		if *attr != "" {
			if len(*attr) != 2 || (*attr)[0] != 'A' || (*attr)[1] < '1' || (*attr)[1] > '6' {
				log.Fatal("-attr debe ser A1..A6")
			}
			a = []int16{int16((*attr)[1] - '0')}
		}
		raw := make([]byte, 32)
		rand.Read(raw)
		token := hex.EncodeToString(raw)
		var ownerID *int64
		if *owner != "" {
			var oid int64
			if err := db.QueryRow(ctx, `SELECT id FROM citizens WHERE email = $1`, strings.ToLower(*owner)).Scan(&oid); err != nil {
				log.Fatalf("no existe un usuario con el correo %s", *owner)
			}
			ownerID = &oid
		}
		var id int64
		err := db.QueryRow(ctx,
			`INSERT INTO terminals (name, role, attributes, modo, owner_id, token_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			*name, *role, a, *modo, ownerID, terminal.HashToken(token)).Scan(&id)
		if err != nil {
			log.Fatalf("no se pudo crear el terminal: %v", err)
		}
		fmt.Printf("terminal %d creado. Token (se muestra una sola vez):\n%s\n", id, token)
	case "user":
		fs := flag.NewFlagSet("user", flag.ExitOnError)
		email := fs.String("email", "", "correo")
		pass := fs.String("password", "", "contraseña")
		role := fs.String("role", "ciudadano", "ciudadano|emisor|comercio")
		name := fs.String("name", "", "nombre")
		phone := fs.String("whatsapp", "", "teléfono de WhatsApp (opcional)")
		fs.Parse(args)
		if *email == "" || len(*pass) < 8 {
			log.Fatal("falta -email o -password (mínimo 8 caracteres)")
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(*pass), bcrypt.DefaultCost)
		var ph *string
		if p := kapso.NormalizePhone(*phone); p != "" {
			ph = &p
		}
		var id int64
		err := db.QueryRow(ctx,
			`INSERT INTO citizens (email, password_hash, role, name, whatsapp_phone) VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role, name = EXCLUDED.name,
			   whatsapp_phone = coalesce(EXCLUDED.whatsapp_phone, citizens.whatsapp_phone)
			 RETURNING id`, strings.ToLower(*email), string(hash), *role, *name, ph).Scan(&id)
		if err != nil {
			log.Fatalf("no se pudo crear el usuario: %v", err)
		}
		fmt.Printf("usuario %d (%s) listo\n", id, *role)
	case "demo":
		demo(ctx, db, args)
	default:
		log.Fatal("uso: admin terminal|user|demo [opciones]")
	}
}
