package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// Tiempo que una solicitud de emisión espera a que llegue una tarjeta virgen.
const requestTTL = 10 * time.Minute

// putProfile: PUT /api/citizens/{id}/profile. Con {fechaNacimiento, cedula} completa el perfil del ciudadano
// (nombre de la cuenta + V-cédula como identidad); también acepta el perfil completo en la forma de Profile.
func (s *Server) putProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	var body map[string]json.RawMessage
	if !readJSON(w, r, &body) {
		return
	}
	var name string
	var raw []byte
	err := s.DB.QueryRow(r.Context(), `SELECT name, profile FROM citizens WHERE id = $1 AND role = 'ciudadano'`, id).Scan(&name, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "Ciudadano no encontrado.")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)

	if _, simple := body["fechaNacimiento"]; simple || body["cedula"] != nil {
		var in struct {
			FechaNacimiento string `json:"fechaNacimiento"`
			Cedula          string `json:"cedula"`
		}
		b, _ := json.Marshal(body)
		_ = json.Unmarshal(b, &in)
		birth, err := time.Parse("2006-01-02", in.FechaNacimiento)
		if err != nil || birth.After(s.now()) || birth.Year() < 1900 {
			writeErr(w, http.StatusBadRequest, "La fecha de nacimiento debe tener el formato AAAA-MM-DD y no puede ser futura.")
			return
		}
		digits := strings.TrimSpace(in.Cedula)
		if len(digits) < 6 || len(digits) > 9 || digitsOnly(digits) != digits {
			writeErr(w, http.StatusBadRequest, "La cédula debe tener entre 6 y 9 dígitos.")
			return
		}
		p.Birthdate = in.FechaNacimiento
		p.Identity = &struct {
			Name       string `json:"name"`
			ID         string `json:"id"`
			ValidUntil string `json:"valid_until"`
		}{strings.ToUpper(strings.TrimSpace(name)), "V-" + digits, s.now().AddDate(5, 0, 0).Format("2006-01-02")}
	} else {
		keep := p.Optional
		p = Profile{}
		b, _ := json.Marshal(body)
		if err := json.Unmarshal(b, &p); err != nil {
			writeErr(w, http.StatusBadRequest, "Perfil inválido.")
			return
		}
		if p.Optional == nil {
			p.Optional = keep // lo que declaró el ciudadano se conserva
		}
	}
	attrs, err := p.Attributes(s.now())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, _ := json.Marshal(p)
	if _, err := s.DB.Exec(r.Context(), `UPDATE citizens SET profile = $1::jsonb WHERE id = $2`, string(out), id); err != nil {
		serverErr(w, err)
		return
	}
	codes := []string{}
	for _, a := range attrs {
		codes = append(codes, attrLabel(int(a.Code)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "perfilCompleto": p.perfilCompleto(), "attributes": codes})
}

// emisionRequest deja pendiente la emisión para el próximo UID que llegue de un terminal de registro.
// Con terminalId (o terminal_id) la orden queda dirigida a ese terminal, que pasa a modo emitir.
func (s *Server) emisionRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CitizenID   flexInt `json:"citizen_id"`
		CiudadanoID flexInt `json:"ciudadanoId"`
		TerminalID  flexInt `json:"terminalId"`
		TerminalID2 flexInt `json:"terminal_id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	citizenID := firstNonZero(int64(in.CiudadanoID), int64(in.CitizenID))
	terminalID := firstNonZero(int64(in.TerminalID), int64(in.TerminalID2))
	ctx := r.Context()
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT profile FROM citizens WHERE id = $1 AND role = 'ciudadano'`, citizenID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "Ciudadano no encontrado.")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	if !p.perfilCompleto() {
		writeErr(w, http.StatusConflict, "Falta completar el perfil del ciudadano (fecha de nacimiento y cédula).")
		return
	}
	if attrs, err := p.Attributes(s.now()); err != nil || len(attrs) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "El ciudadano no tiene datos para emitir atributos.")
		return
	}
	var term *int64
	if terminalID != 0 {
		var role string
		err := s.DB.QueryRow(ctx, `SELECT role FROM terminals WHERE id = $1`, terminalID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "terminal no encontrado")
			return
		}
		if err != nil {
			serverErr(w, err)
			return
		}
		if role != "registro" {
			writeErr(w, http.StatusBadRequest, "solo un terminal de registro puede emitir")
			return
		}
		if _, err := s.DB.Exec(ctx, `UPDATE terminals SET modo = 'emitir' WHERE id = $1`, terminalID); err != nil {
			serverErr(w, err)
			return
		}
		term = &terminalID
	} else {
		// Sin terminal elegido, lo atiende cualquier terminal de registro en modo emitir; si ninguno lo está, se activa.
		if _, err := s.DB.Exec(ctx,
			`UPDATE terminals SET modo = 'emitir' WHERE role = 'registro'
			   AND NOT EXISTS (SELECT 1 FROM terminals WHERE role = 'registro' AND modo = 'emitir')`); err != nil {
			serverErr(w, err)
			return
		}
	}
	var id int64
	err = s.DB.QueryRow(ctx,
		`INSERT INTO issuance_requests (citizen_id, requested_by, terminal_id) VALUES ($1, $2, $3) RETURNING id`,
		citizenID, userFrom(ctx).ID, term).Scan(&id)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id, "expires_in": int(requestTTL.Seconds())})
}

type block struct {
	S int    `json:"s"`
	B int    `json:"b"`
	D string `json:"d"`
}

// emisionStart recibe el UID de una tarjeta virgen, firma los atributos del ciudadano pendiente
// y devuelve los bloques a escribir (índice primero).
func (s *Server) emisionStart(w http.ResponseWriter, r *http.Request) {
	term, _ := terminal.From(r.Context())
	if term.Rol != "registro" || term.Modo != "emitir" {
		writeErr(w, http.StatusForbidden, "el terminal no está en modo emitir")
		return
	}
	var in struct {
		UID string `json:"uid"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	uid, err := parseUID(in.UID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(r.Context())

	var reqID, citizenID int64
	var raw []byte
	err = tx.QueryRow(r.Context(),
		`SELECT q.id, q.citizen_id, c.profile
		   FROM issuance_requests q JOIN citizens c ON c.id = q.citizen_id
		  WHERE q.used_at IS NULL AND q.created_at > $1 AND (q.terminal_id IS NULL OR q.terminal_id = $2)
		  ORDER BY (q.terminal_id IS NULL), q.created_at LIMIT 1 FOR UPDATE OF q SKIP LOCKED`,
		s.now().Add(-requestTTL), term.ID).Scan(&reqID, &citizenID, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusConflict, "no hay una emisión pendiente")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	attrs, err := p.Attributes(s.now())
	if err != nil || len(attrs) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "el ciudadano no tiene datos para emitir atributos")
		return
	}

	var handle [8]byte
	if _, err := rand.Read(handle[:]); err != nil {
		serverErr(w, err)
		return
	}
	issuer := byte(sign.IssuerNacional)
	key := s.Issuers[issuer]
	if key == nil {
		writeErr(w, http.StatusServiceUnavailable, "falta la clave del emisor")
		return
	}
	expires := s.now().AddDate(0, 0, s.ValidDays)

	if _, err := tx.Exec(r.Context(), `INSERT INTO cards (handle, uid, citizen_id) VALUES ($1,$2,$3)`,
		handle[:], uid, citizenID); err != nil {
		serverErr(w, err)
		return
	}
	blocks := []block{}
	for i, d := range sign.IndexBlocks(handle) {
		blocks = append(blocks, block{1, i, hex.EncodeToString(d)})
	}
	for _, a := range attrs {
		at := sign.Attribute{Issuer: issuer, Code: a.Code, Expires: uint32(expires.Unix()), Handle: handle, Value: a.Value}
		sig, err := at.Sign(key, uid)
		if err != nil {
			serverErr(w, err)
			return
		}
		recBlocks, err := at.Record(sig)
		if err != nil {
			serverErr(w, err)
			return
		}
		for i, sec := range sign.SectorsFor(a.Code) {
			for j := 0; j < 3; j++ {
				blocks = append(blocks, block{sec, j, hex.EncodeToString(recBlocks[i*3+j])})
			}
		}
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO attributes (handle, code, issuer, value, expires_at) VALUES ($1,$2,$3,$4,$5)`,
			handle[:], int(a.Code), int(issuer), a.Value, expires); err != nil {
			serverErr(w, err)
			return
		}
	}
	if _, err := tx.Exec(r.Context(), `UPDATE issuance_requests SET used_at = now() WHERE id = $1`, reqID); err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "h": hex.EncodeToString(handle[:]), "blocks": blocks})
}
