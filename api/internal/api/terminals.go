package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

const maxTerminalsPerCommerce = 20

type terminalOut struct {
	ID        int64    `json:"id"`
	Local     string   `json:"local"`
	Nombre    string   `json:"nombre"`
	Rol       string   `json:"rol"`
	A         string   `json:"a"`
	Atributos []string `json:"atributos"`
	Modo      string   `json:"modo"`
	Token     string   `json:"token,omitempty"` // solo en la respuesta de POST /api/terminals
}

func newTerminalOut(id int64, name, role string, attrs []int16, modo string) terminalOut {
	t := terminalOut{ID: id, Local: name, Nombre: name, Rol: role, Modo: modo, Atributos: []string{}}
	for i, n := range attrs {
		l := attrLabel(int(n))
		t.Atributos = append(t.Atributos, l)
		if i == 0 {
			t.A = l
		}
	}
	return t
}

// parseAttrList convierte ["A1","A2"] en [1,2]; sin repetidos. Quien actúa como comercio solo puede pedir A1 y A2.
func parseAttrList(list []string, actorRole string) ([]int16, string) {
	seen := map[int]bool{}
	out := []int16{}
	for _, l := range list {
		n, ok := parseAttr(l)
		if !ok {
			return nil, "atributo inválido: " + l
		}
		if actorRole == "comercio" && n > 2 {
			return nil, "un comercio solo puede pedir A1 o A2"
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, int16(n))
		}
	}
	return out, ""
}

func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, len(s) >= 1 && len(s) <= 60
}

// terminalAccess indica si el usuario puede administrar el terminal (el emisor, todos; el comercio, los suyos).
func (s *Server) terminalAccess(ctx context.Context, u User, id int64) (found, allowed bool, err error) {
	var owner *int64
	err = s.DB.QueryRow(ctx, `SELECT owner_id FROM terminals WHERE id = $1`, id).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, u.Role == "emisor" || (owner != nil && *owner == u.ID), nil
}

// listTerminals: GET /api/terminals. El comercio ve los suyos; el emisor, todos. Nunca devuelve tokens.
func (s *Server) listTerminals(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	rows, err := s.DB.Query(r.Context(),
		`SELECT id, name, role, attributes, modo FROM terminals WHERE ($1 OR owner_id = $2) ORDER BY id`, u.Role == "emisor", u.ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	out := []terminalOut{}
	for rows.Next() {
		var id int64
		var name, role, modo string
		var attrs []int16
		if err := rows.Scan(&id, &name, &role, &attrs, &modo); err != nil {
			serverErr(w, err)
			return
		}
		out = append(out, newTerminalOut(id, name, role, attrs, modo))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "terminals": out})
}

// createTerminal: POST /api/terminals {nombre, atributos}. Solo comercio. El token se muestra una única vez.
func (s *Server) createTerminal(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var in struct {
		Nombre    string   `json:"nombre"`
		Atributos []string `json:"atributos"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	name, ok := cleanName(in.Nombre)
	if !ok {
		writeErr(w, http.StatusBadRequest, "el nombre del local debe tener entre 1 y 60 caracteres")
		return
	}
	if len(in.Atributos) == 0 {
		in.Atributos = []string{"A1"}
	}
	attrs, msg := parseAttrList(in.Atributos, "comercio")
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	var count int
	if err := s.DB.QueryRow(r.Context(), `SELECT count(*) FROM terminals WHERE owner_id = $1`, u.ID).Scan(&count); err != nil {
		serverErr(w, err)
		return
	}
	if count >= maxTerminalsPerCommerce {
		writeErr(w, http.StatusConflict, "alcanzaste el máximo de terminales")
		return
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		serverErr(w, err)
		return
	}
	token := hex.EncodeToString(raw)
	var id int64
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO terminals (name, role, attributes, modo, owner_id, token_hash) VALUES ($1,'comercio',$2,'verificar',$3,$4) RETURNING id`,
		name, attrs, u.ID, terminal.HashToken(token)).Scan(&id)
	if err != nil {
		serverErr(w, err)
		return
	}
	t := newTerminalOut(id, name, "comercio", attrs, "verificar")
	t.Token = token
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "terminal": t, "token": token})
}

// updateTerminal: PUT o PATCH /api/terminals/{id} con {nombre?, atributos?, a?, modo?}.
// El comercio solo edita los suyos y solo en modo verificar; el emisor cambia cualquier modo.
func (s *Server) updateTerminal(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	u := userFrom(r.Context())
	found, allowed, err := s.terminalAccess(r.Context(), u, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "terminal no encontrado")
		return
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "no tienes permiso")
		return
	}
	var in struct {
		Nombre    *string   `json:"nombre"`
		Atributos *[]string `json:"atributos"`
		A         *string   `json:"a"`
		Modo      *string   `json:"modo"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	var name, modo any
	if in.Nombre != nil {
		n, ok := cleanName(*in.Nombre)
		if !ok {
			writeErr(w, http.StatusBadRequest, "el nombre del local debe tener entre 1 y 60 caracteres")
			return
		}
		name = n
	}
	if in.Modo != nil {
		switch *in.Modo {
		case "verificar", "emergencia", "emitir", "vincular":
		default:
			writeErr(w, http.StatusBadRequest, "modo inválido")
			return
		}
		if u.Role != "emisor" && *in.Modo != "verificar" {
			writeErr(w, http.StatusForbidden, "solo el emisor puede cambiar el modo")
			return
		}
		modo = *in.Modo
	}
	var attrs any
	list := in.Atributos
	if list == nil && in.A != nil {
		if *in.A == "" {
			list = &[]string{}
		} else {
			list = &[]string{*in.A}
		}
	}
	if list != nil {
		a, msg := parseAttrList(*list, u.Role)
		if msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		attrs = a
	}
	if _, err := s.DB.Exec(r.Context(),
		`UPDATE terminals SET name = coalesce($2::text, name), attributes = coalesce($3::smallint[], attributes), modo = coalesce($4::text, modo) WHERE id = $1`,
		id, name, attrs, modo); err != nil {
		serverErr(w, err)
		return
	}
	s.listTerminals(w, r)
}

// deleteTerminal: DELETE /api/terminals/{id}. Sus eventos se conservan sin terminal.
func (s *Server) deleteTerminal(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, allowed, err := s.terminalAccess(r.Context(), userFrom(r.Context()), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "terminal no encontrado")
		return
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "no tienes permiso")
		return
	}
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM terminals WHERE id = $1`, id); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// stats: GET /api/stats. Por terminal del comercio: lecturas de hoy (hora de Venezuela), aprobadas y rechazadas.
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	rows, err := s.DB.Query(r.Context(),
		`SELECT t.id, t.name,
		        count(e.id) FILTER (WHERE e.kind = 'verif'),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result = 1),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result <> 1)
		   FROM terminals t
		   LEFT JOIN events e ON e.terminal_id = t.id
		        AND e.created_at >= (date_trunc('day', now() AT TIME ZONE 'America/Caracas') AT TIME ZONE 'America/Caracas')
		  WHERE ($1 OR t.owner_id = $2)
		  GROUP BY t.id, t.name ORDER BY t.id`, u.Role == "emisor", u.ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID         int64  `json:"id"`
		Local      string `json:"local"`
		Nombre     string `json:"nombre"`
		Hoy        int    `json:"hoy"`
		Aprobadas  int    `json:"aprobadas"`
		Rechazadas int    `json:"rechazadas"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Local, &it.Hoy, &it.Aprobadas, &it.Rechazadas); err != nil {
			serverErr(w, err)
			return
		}
		it.Nombre = it.Local
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "terminals": out})
}
