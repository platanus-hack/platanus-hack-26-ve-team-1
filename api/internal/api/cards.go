package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type cardOut struct {
	H            string   `json:"h"` // handle: solo lo ve su dueño (o el emisor)
	Estado       string   `json:"estado"`
	Emitida      string   `json:"emitida"`
	Revocada     *string  `json:"revocada,omitempty"`
	Atributos    []string `json:"atributos"`
	Dispositivos int      `json:"dispositivos"`
}

// listCards: GET /api/cards. Las cédulas de la cuenta, para que el portal muestre su estado.
func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT c.handle, c.status, c.issued_at, c.revoked_at,
		        coalesce((SELECT array_agg(a.code ORDER BY a.code) FROM attributes a WHERE a.handle = c.handle), '{}'),
		        (SELECT count(*) FROM devices d WHERE d.handle = c.handle)
		   FROM cards c WHERE c.citizen_id = $1 ORDER BY c.issued_at DESC`, userFrom(r.Context()).ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	out := []cardOut{}
	for rows.Next() {
		var c cardOut
		var h []byte
		var issued time.Time
		var revoked *time.Time
		var codes []int16
		if err := rows.Scan(&h, &c.Estado, &issued, &revoked, &codes, &c.Dispositivos); err != nil {
			serverErr(w, err)
			return
		}
		c.H, c.Emitida = hex.EncodeToString(h), issued.UTC().Format(time.RFC3339)
		if revoked != nil {
			v := revoked.UTC().Format(time.RFC3339)
			c.Revocada = &v
		}
		c.Atributos = []string{}
		for _, code := range codes {
			c.Atributos = append(c.Atributos, attrLabel(int(code)))
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cards": out})
}

// reactivate: POST /api/reactivate {h}. Reactiva una cédula revocada (dueño o emisor)
// y vuelve a emitir las credenciales de billetera de sus dispositivos.
func (s *Server) reactivate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		H string `json:"h"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	handle, err := parseHandle(in.H)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u := userFrom(r.Context())
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(ctx)

	var owner *int64
	err = tx.QueryRow(ctx,
		`UPDATE cards SET status = 'activa', revoked_at = NULL
		  WHERE handle = $1 AND status = 'revocada' AND ($2 OR citizen_id = $3)
		  RETURNING citizen_id`, handle, u.Role == "emisor", u.ID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "cédula revocada no encontrada")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	creds := []credential{}
	if owner != nil {
		rows, err := tx.Query(ctx, `SELECT id, public_key FROM devices WHERE handle = $1`, handle)
		if err != nil {
			serverErr(w, err)
			return
		}
		type dev struct {
			id  int64
			pub []byte
		}
		var devs []dev
		for rows.Next() {
			var d dev
			if err := rows.Scan(&d.id, &d.pub); err != nil {
				rows.Close()
				serverErr(w, err)
				return
			}
			devs = append(devs, d)
		}
		rows.Close()
		for _, d := range devs {
			c, err := s.issueWallet(ctx, tx, d.id, handle, d.pub, *owner)
			if err != nil {
				serverErr(w, err)
				return
			}
			creds = append(creds, c...)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credentials": len(creds)})
}

// issueCarnet: POST /api/carnet/issue {citizen_id}. El emisor (universidad) emite o renueva el
// carnet universitario en los celulares ya vinculados del ciudadano.
func (s *Server) issueCarnet(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CitizenID int64 `json:"citizen_id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(ctx)

	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT profile FROM citizens WHERE id = $1 AND role = 'ciudadano'`, in.CitizenID).Scan(&raw); errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "ciudadano no encontrado")
		return
	} else if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	if _, ok, err := p.CarnetValue(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	} else if !ok {
		writeErr(w, http.StatusUnprocessableEntity, "el perfil no tiene datos de carnet")
		return
	}
	if s.Issuers[2] == nil {
		writeErr(w, http.StatusServiceUnavailable, "falta la clave del emisor universidad")
		return
	}
	rows, err := tx.Query(ctx,
		`SELECT d.id, d.handle, d.public_key FROM devices d JOIN cards c ON c.handle = d.handle
		  WHERE d.citizen_id = $1 AND c.status = 'activa'`, in.CitizenID)
	if err != nil {
		serverErr(w, err)
		return
	}
	type dev struct {
		id     int64
		handle []byte
		pub    []byte
	}
	var devs []dev
	for rows.Next() {
		var d dev
		if err := rows.Scan(&d.id, &d.handle, &d.pub); err != nil {
			rows.Close()
			serverErr(w, err)
			return
		}
		devs = append(devs, d)
	}
	rows.Close()
	if len(devs) == 0 {
		writeErr(w, http.StatusConflict, "el ciudadano debe vincular su celular a una cédula activa primero")
		return
	}
	n := 0
	for _, d := range devs {
		cs, err := s.issueWallet(ctx, tx, d.id, d.handle, d.pub, in.CitizenID)
		if err != nil {
			serverErr(w, err)
			return
		}
		n += len(cs)
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dispositivos": len(devs)})
}

// --- Terminales (emisor) ---

type terminalOut struct {
	ID    int64  `json:"id"`
	Local string `json:"local"`
	Rol   string `json:"rol"`
	A     string `json:"a"`
	Modo  string `json:"modo"`
}

func (s *Server) listTerminals(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(), `SELECT id, name, role, attribute, modo FROM terminals ORDER BY id`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	out := []terminalOut{}
	for rows.Next() {
		var t terminalOut
		var a *int16
		if err := rows.Scan(&t.ID, &t.Local, &t.Rol, &a, &t.Modo); err != nil {
			serverErr(w, err)
			return
		}
		if a != nil {
			t.A = attrLabel(int(*a))
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "terminals": out})
}

// patchTerminal: PATCH /api/terminals/{id} {modo?, a?}. Cambia el modo (el gateway lo lee en /api/terminal/config).
func (s *Server) patchTerminal(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	var in struct {
		Modo *string `json:"modo"`
		A    *string `json:"a"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Modo != nil {
		switch *in.Modo {
		case "verificar", "emergencia", "emitir", "vincular":
		default:
			writeErr(w, http.StatusBadRequest, "modo inválido")
			return
		}
	}
	var attr *int
	clearAttr := false
	if in.A != nil {
		if *in.A == "" {
			clearAttr = true
		} else if n, ok := parseAttr(*in.A); ok {
			attr = &n
		} else {
			writeErr(w, http.StatusBadRequest, "atributo inválido")
			return
		}
	}
	tag, err := s.DB.Exec(r.Context(),
		`UPDATE terminals SET modo = coalesce($2, modo), attribute = CASE WHEN $4 THEN NULL ELSE coalesce($3, attribute) END WHERE id = $1`,
		id, in.Modo, attr, clearAttr)
	if err != nil {
		serverErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "terminal no encontrado")
		return
	}
	s.listTerminals(w, r)
}
