package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// Tiempo que una solicitud de emisión espera a que llegue una tarjeta virgen.
const requestTTL = 10 * time.Minute

func (s *Server) putProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	var p Profile
	if !readJSON(w, r, &p) {
		return
	}
	attrs, err := p.Attributes(s.now())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, _ := json.Marshal(p)
	// Lo que el ciudadano declaró por su cuenta (optional) se conserva si el emisor no lo envía.
	tag, err := s.DB.Exec(r.Context(),
		`UPDATE citizens SET profile = CASE
		     WHEN profile ? 'optional' AND NOT ($1::jsonb ? 'optional') THEN $1::jsonb || jsonb_build_object('optional', profile->'optional')
		     ELSE $1::jsonb END
		  WHERE id = $2 AND role = 'ciudadano'`, string(raw), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "ciudadano no encontrado")
		return
	}
	codes := []string{}
	for _, a := range attrs {
		codes = append(codes, attrLabel(int(a.Code)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "attributes": codes})
}

// emisionRequest deja pendiente la emisión para el próximo UID que llegue de un terminal de registro.
// Con terminalId (o terminal_id) la orden queda dirigida a ese terminal, que pasa a modo emitir.
func (s *Server) emisionRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CitizenID   int64 `json:"citizen_id"`
		CiudadanoID int64 `json:"ciudadanoId"`
		TerminalID  int64 `json:"terminalId"`
		TerminalID2 int64 `json:"terminal_id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	citizenID := firstNonZero(in.CiudadanoID, in.CitizenID)
	terminalID := firstNonZero(in.TerminalID, in.TerminalID2)
	ctx := r.Context()
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT profile FROM citizens WHERE id = $1 AND role = 'ciudadano'`, citizenID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "ciudadano no encontrado")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	if attrs, err := p.Attributes(s.now()); err != nil || len(attrs) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "el ciudadano no tiene datos para emitir atributos")
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
