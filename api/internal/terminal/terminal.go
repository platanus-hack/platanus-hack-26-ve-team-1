// Package terminal autentica a los terminales por token y sirve su configuración.
package terminal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// Terminal es el terminal autenticado por su token.
type Terminal struct {
	ID    int64
	Local string
	Rol   string
	Attrs []int16 // atributos que pide; el lector usa el primero
	Modo  string
}

// HashToken devuelve el SHA-256 en hex del token, que es lo que se guarda en terminals.token_hash.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Auth exige Authorization: Bearer <token> y deja el terminal en el contexto.
func Auth(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				writeErr(w, http.StatusUnauthorized, "falta el token")
				return
			}
			var t Terminal
			err := db.QueryRow(r.Context(),
				`SELECT id, name, role, attributes, modo FROM terminals WHERE token_hash = $1`,
				HashToken(token)).Scan(&t.ID, &t.Local, &t.Rol, &t.Attrs, &t.Modo)
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusUnauthorized, "token inválido")
				return
			}
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "error interno")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, t)))
		})
	}
}

// From devuelve el terminal autenticado del contexto.
func From(ctx context.Context) (Terminal, bool) {
	t, ok := ctx.Value(ctxKey{}).(Terminal)
	return t, ok
}

// Config atiende GET /api/terminal/config: {"rol","a","local","modo"}.
func Config(w http.ResponseWriter, r *http.Request) {
	t, ok := From(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "sin terminal")
		return
	}
	a, list := "", []string{}
	for i, n := range t.Attrs {
		l := fmt.Sprintf("A%d", n)
		list = append(list, l)
		if i == 0 {
			a = l
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rol": t.Rol, "a": a, "atributos": list, "local": t.Local, "modo": t.Modo})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}
