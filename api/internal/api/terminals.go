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

// Qué datos puede pedir cada tipo de terminal (rol del terminal, no del usuario).
type roleRule struct {
	allowed    []int
	defaults   []int
	exactlyOne bool
	modo       string
}

var roleRules = map[string]roleRule{
	"comercio":   {allowed: []int{1, 2}, defaults: []int{1}, exactlyOne: true, modo: "verificar"},
	"registro":   {allowed: []int{3}, defaults: []int{3}, modo: "emitir"},
	"salud":      {allowed: []int{4, 5, 6}, defaults: []int{4, 5, 6}, modo: "verificar"},
	"emergencia": {allowed: []int{4, 5}, defaults: []int{4, 5}, modo: "emergencia"},
	"transito":   {allowed: []int{6}, defaults: []int{6}, modo: "verificar"},
}

type terminalOut struct {
	ID        string   `json:"id"`
	Nombre    string   `json:"nombre"`
	Rol       string   `json:"rol"`
	Atributos []string `json:"atributos"`
	Token     string   `json:"token,omitempty"` // solo en la respuesta de POST /api/terminals
	// Extras para el emisor y el gateway.
	Local string `json:"local"`
	A     string `json:"a"`
	Modo  string `json:"modo"`
}

func newTerminalOut(id int64, name, role string, attrs []int16, modo string) terminalOut {
	t := terminalOut{ID: itoa(id), Nombre: name, Local: name, Rol: role, Modo: modo, Atributos: []string{}}
	for i, n := range attrs {
		t.Atributos = append(t.Atributos, attrKey(int(n)))
		if i == 0 {
			t.A = attrLabel(int(n))
		}
	}
	return t
}

// validateAttrs convierte los nombres del front a códigos y aplica las reglas del rol del terminal.
func validateAttrs(role string, keys []string) ([]int16, string) {
	rule, ok := roleRules[role]
	if !ok {
		return nil, "Tipo de terminal inválido."
	}
	seen := map[int]bool{}
	var codes []int16
	for _, k := range keys {
		n, ok := parseAttrKey(k)
		if !ok {
			return nil, "Atributo inválido: " + k + "."
		}
		allowed := false
		for _, a := range rule.allowed {
			allowed = allowed || a == n
		}
		if !allowed {
			return nil, "Un terminal de tipo " + role + " no puede pedir " + attrNames[n] + "."
		}
		if !seen[n] {
			seen[n] = true
			codes = append(codes, int16(n))
		}
	}
	if len(codes) == 0 {
		for _, d := range rule.defaults {
			codes = append(codes, int16(d))
		}
	}
	if rule.exactlyOne && len(codes) != 1 {
		return nil, "Un terminal de comercio pide un solo dato: mayor de 18 o mayor de 21."
	}
	return codes, ""
}

func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, len(s) >= 1 && len(s) <= 60
}

// terminalAccess indica si el usuario puede administrar el terminal (el emisor, todos; el comercio, los suyos).
func (s *Server) terminalAccess(ctx context.Context, u User, id int64) (found, allowed bool, role string, err error) {
	var owner *int64
	err = s.DB.QueryRow(ctx, `SELECT owner_id, role FROM terminals WHERE id = $1`, id).Scan(&owner, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, "", nil
	}
	if err != nil {
		return false, false, "", err
	}
	return true, u.Role == "emisor" || (owner != nil && *owner == u.ID), role, nil
}

func (s *Server) terminalList(r *http.Request, u User) ([]terminalOut, error) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT id, name, role, attributes, modo FROM terminals WHERE ($1 OR owner_id = $2) ORDER BY id`, u.Role == "emisor", u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []terminalOut{}
	for rows.Next() {
		var id int64
		var name, role, modo string
		var attrs []int16
		if err := rows.Scan(&id, &name, &role, &attrs, &modo); err != nil {
			return nil, err
		}
		out = append(out, newTerminalOut(id, name, role, attrs, modo))
	}
	return out, rows.Err()
}

// listTerminals: GET /api/terminals → Terminal[] (el comercio ve los suyos; el emisor, todos). Sin tokens.
func (s *Server) listTerminals(w http.ResponseWriter, r *http.Request) {
	out, err := s.terminalList(r, userFrom(r.Context()))
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) terminalByID(r *http.Request, id int64) (terminalOut, error) {
	var name, role, modo string
	var attrs []int16
	err := s.DB.QueryRow(r.Context(), `SELECT name, role, attributes, modo FROM terminals WHERE id = $1`, id).Scan(&name, &role, &attrs, &modo)
	return newTerminalOut(id, name, role, attrs, modo), err
}

// createTerminal: POST /api/terminals {nombre, rol, atributos} → Terminal con token. Solo comercio.
// El token se entrega una única vez.
func (s *Server) createTerminal(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var in struct {
		Nombre    string   `json:"nombre"`
		Rol       string   `json:"rol"`
		Atributos []string `json:"atributos"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	name, ok := cleanName(in.Nombre)
	if !ok {
		writeErr(w, http.StatusBadRequest, "El nombre del terminal debe tener entre 1 y 60 caracteres.")
		return
	}
	if in.Rol == "" {
		in.Rol = "comercio"
	}
	if in.Rol == "transito" {
		writeErr(w, http.StatusBadRequest, "Tipo de terminal inválido.")
		return
	}
	attrs, msg := validateAttrs(in.Rol, in.Atributos)
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
		writeErr(w, http.StatusConflict, "Alcanzaste el máximo de terminales.")
		return
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		serverErr(w, err)
		return
	}
	token := hex.EncodeToString(raw)
	modo := roleRules[in.Rol].modo
	var id int64
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO terminals (name, role, attributes, modo, owner_id, token_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, in.Rol, attrs, modo, u.ID, terminal.HashToken(token)).Scan(&id)
	if err != nil {
		serverErr(w, err)
		return
	}
	t := newTerminalOut(id, name, in.Rol, attrs, modo)
	t.Token = token
	writeJSON(w, http.StatusCreated, t)
}

// updateTerminal: PUT o PATCH /api/terminals/{id} con {nombre?, rol?, atributos?, modo?} → Terminal.
// El comercio solo edita los suyos y no cambia el modo; el emisor puede cambiar cualquier modo.
func (s *Server) updateTerminal(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "Id inválido.")
		return
	}
	u := userFrom(r.Context())
	found, allowed, curRole, err := s.terminalAccess(r.Context(), u, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "Terminal no encontrado.")
		return
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "No tienes permiso para este terminal.")
		return
	}
	var in struct {
		Nombre    *string   `json:"nombre"`
		Rol       *string   `json:"rol"`
		Atributos *[]string `json:"atributos"`
		Modo      *string   `json:"modo"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	var name, role, modo any
	if in.Nombre != nil {
		n, ok := cleanName(*in.Nombre)
		if !ok {
			writeErr(w, http.StatusBadRequest, "El nombre del terminal debe tener entre 1 y 60 caracteres.")
			return
		}
		name = n
	}
	newRole := curRole
	if in.Rol != nil && *in.Rol != curRole {
		if _, ok := roleRules[*in.Rol]; !ok || (*in.Rol == "transito" && u.Role != "emisor") {
			writeErr(w, http.StatusBadRequest, "Tipo de terminal inválido.")
			return
		}
		newRole = *in.Rol
		role = newRole
		modo = roleRules[newRole].modo // un cambio de tipo reinicia el modo
	}
	if in.Modo != nil {
		switch *in.Modo {
		case "verificar", "emergencia", "emitir", "vincular":
		default:
			writeErr(w, http.StatusBadRequest, "Modo inválido.")
			return
		}
		if u.Role != "emisor" {
			writeErr(w, http.StatusForbidden, "Solo el emisor puede cambiar el modo.")
			return
		}
		modo = *in.Modo
	}
	var attrs any
	if in.Atributos != nil || role != nil {
		list := []string{}
		if in.Atributos != nil {
			list = *in.Atributos
		}
		a, msg := validateAttrs(newRole, list)
		if msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		attrs = a
	}
	if _, err := s.DB.Exec(r.Context(),
		`UPDATE terminals SET name = coalesce($2::text, name), role = coalesce($3::text, role),
		        attributes = coalesce($4::smallint[], attributes), modo = coalesce($5::text, modo) WHERE id = $1`,
		id, name, role, attrs, modo); err != nil {
		serverErr(w, err)
		return
	}
	t, err := s.terminalByID(r, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// deleteTerminal: DELETE /api/terminals/{id}. Su token deja de valer; sus eventos se conservan sin terminal.
func (s *Server) deleteTerminal(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "Id inválido.")
		return
	}
	found, allowed, _, err := s.terminalAccess(r.Context(), userFrom(r.Context()), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "Terminal no encontrado.")
		return
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "No tienes permiso para este terminal.")
		return
	}
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM terminals WHERE id = $1`, id); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// stats: GET /api/stats → [{terminalId, hoy, aprobadas, rechazadas}] de hoy (hora de Venezuela).
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	rows, err := s.DB.Query(r.Context(),
		`SELECT t.id,
		        count(e.id) FILTER (WHERE e.kind = 'verif'),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result = 1),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result <> 1)
		   FROM terminals t
		   LEFT JOIN events e ON e.terminal_id = t.id
		        AND e.created_at >= (date_trunc('day', now() AT TIME ZONE 'America/Caracas') AT TIME ZONE 'America/Caracas')
		  WHERE ($1 OR t.owner_id = $2)
		  GROUP BY t.id ORDER BY t.id`, u.Role == "emisor", u.ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		TerminalID string `json:"terminalId"`
		Hoy        int    `json:"hoy"`
		Aprobadas  int    `json:"aprobadas"`
		Rechazadas int    `json:"rechazadas"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		var id int64
		if err := rows.Scan(&id, &it.Hoy, &it.Aprobadas, &it.Rechazadas); err != nil {
			serverErr(w, err)
			return
		}
		it.TerminalID = itoa(id)
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, out)
}
