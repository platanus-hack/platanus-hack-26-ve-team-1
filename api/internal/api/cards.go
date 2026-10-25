package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
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

// reactivateOne reactiva una cédula revocada y vuelve a emitir las credenciales de sus celulares.
// Si anyOwner es falso, solo lo hace si la cédula es de citizenID.
func (s *Server) reactivateOne(ctx context.Context, handle []byte, anyOwner bool, citizenID int64) (found bool, creds int, err error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	var owner *int64
	err = tx.QueryRow(ctx,
		`UPDATE cards SET status = 'activa', revoked_at = NULL
		  WHERE handle = $1 AND status = 'revocada' AND ($2 OR citizen_id = $3)
		  RETURNING citizen_id`, handle, anyOwner, citizenID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if owner != nil {
		rows, err := tx.Query(ctx, `SELECT id, public_key FROM devices WHERE handle = $1`, handle)
		if err != nil {
			return false, 0, err
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
				return false, 0, err
			}
			devs = append(devs, d)
		}
		rows.Close()
		for _, d := range devs {
			c, err := s.issueWallet(ctx, tx, d.id, handle, d.pub, *owner)
			if err != nil {
				return false, 0, err
			}
			creds += len(c)
		}
	}
	return true, creds, tx.Commit(ctx)
}

// reactivate: POST /api/reactivate. Con {h} reactiva esa cédula; sin cuerpo, las revocadas de la cuenta
// (el emisor indica {ciudadanoId}). Dueño o emisor.
func (s *Server) reactivate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		H           string  `json:"h"`
		CiudadanoID flexInt `json:"ciudadanoId"`
	}
	if !readOptionalJSON(w, r, &in) {
		return
	}
	u := userFrom(r.Context())
	ctx := r.Context()
	if in.CiudadanoID != 0 && u.Role != "emisor" {
		writeErr(w, http.StatusForbidden, "Solo un emisor puede reactivar la cédula de otra persona.")
		return
	}
	if in.H != "" {
		handle, err := parseHandle(in.H)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		found, n, err := s.reactivateOne(ctx, handle, u.Role == "emisor", u.ID)
		if err != nil {
			serverErr(w, err)
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "cédula revocada no encontrada")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credentials": n})
		return
	}
	target, ok := s.targetCitizen(w, u, int64(in.CiudadanoID))
	if !ok {
		return
	}
	rows, err := s.DB.Query(ctx, `SELECT handle FROM cards WHERE citizen_id = $1 AND status = 'revocada'`, target)
	if err != nil {
		serverErr(w, err)
		return
	}
	var handles [][]byte
	for rows.Next() {
		var h []byte
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			serverErr(w, err)
			return
		}
		handles = append(handles, h)
	}
	rows.Close()
	total, n := 0, 0
	for _, h := range handles {
		found, c, err := s.reactivateOne(ctx, h, true, target)
		if err != nil {
			serverErr(w, err)
			return
		}
		if found {
			n++
			total += c
		}
	}
	if n == 0 {
		writeErr(w, http.StatusNotFound, "no hay cédulas revocadas")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cedulas": n, "credentials": total})
}

// targetCitizen: el ciudadano actúa sobre sí mismo; el emisor debe indicar a quién.
func (s *Server) targetCitizen(w http.ResponseWriter, u User, requested int64) (int64, bool) {
	if u.Role != "emisor" {
		return u.ID, true
	}
	if requested == 0 {
		writeErr(w, http.StatusBadRequest, "falta ciudadanoId")
		return 0, false
	}
	return requested, true
}

// issueCarnet: POST /api/credentials/university {estudianteId} (alias /api/carnet/issue).
// El carnet sale del perfil (carnet, o se deriva de la identidad: UCV-<cédula>) y lo firma la universidad
// (emisor 0x02) en los celulares vinculados. 409 si ya tiene carnet; si aún no vinculó su celular,
// queda autorizado y se emite al vincular.
func (s *Server) issueCarnet(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		CitizenID    flexInt `json:"citizen_id"`
		CiudadanoID  flexInt `json:"ciudadanoId"`
		EstudianteID flexInt `json:"estudianteId"`
		ID           flexInt `json:"id"`
	}
	if !readJSON(w, r, &raw) {
		return
	}
	citizenID := firstNonZero(int64(raw.EstudianteID), int64(raw.CiudadanoID), int64(raw.CitizenID), int64(raw.ID))
	ctx := r.Context()
	if s.Issuers[sign.IssuerUniversidad] == nil {
		writeErr(w, http.StatusServiceUnavailable, "Falta la clave del emisor universidad.")
		return
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(ctx)

	var prof []byte
	if err := tx.QueryRow(ctx, `SELECT profile FROM citizens WHERE id = $1 AND role = 'ciudadano' FOR UPDATE`, citizenID).Scan(&prof); errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "Estudiante no encontrado.")
		return
	} else if err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(prof, &p)
	if p.Carnet == nil && p.Identity != nil && p.Identity.Name != "" && p.Identity.ID != "" {
		p.Carnet = &struct {
			Name       string `json:"name"`
			ID         string `json:"id"`
			ValidUntil string `json:"valid_until"`
		}{p.Identity.Name, "UCV-" + digitsOnly(p.Identity.ID), s.now().AddDate(0, 9, 0).Format("2006-01-02")}
	}
	if _, ok, err := p.CarnetValue(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	} else if !ok {
		writeErr(w, http.StatusUnprocessableEntity, "El estudiante no tiene el perfil completo para emitir un carnet.")
		return
	}
	var issued bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM wallet_credentials wc JOIN devices d ON d.id = wc.device_id
		                 WHERE d.citizen_id = $1 AND wc.issuer = 2 AND wc.expires_at > now())`, citizenID).Scan(&issued); err != nil {
		serverErr(w, err)
		return
	}
	if issued {
		writeErr(w, http.StatusConflict, "El estudiante ya tiene carnet.")
		return
	}
	out, _ := json.Marshal(p)
	if _, err := tx.Exec(ctx, `UPDATE citizens SET profile = $1::jsonb WHERE id = $2`, string(out), citizenID); err != nil {
		serverErr(w, err)
		return
	}
	rows, err := tx.Query(ctx,
		`SELECT d.id, d.handle, d.public_key FROM devices d JOIN cards c ON c.handle = d.handle
		  WHERE d.citizen_id = $1 AND c.status = 'activa'`, citizenID)
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
	for _, d := range devs {
		if _, err := s.issueWallet(ctx, tx, d.id, d.handle, d.pub, citizenID); err != nil {
			serverErr(w, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w, err)
		return
	}
	resp := map[string]any{"ok": true, "dispositivos": len(devs)}
	if len(devs) == 0 {
		resp["pendiente"] = true
		resp["mensaje"] = "El carnet se emitirá cuando el estudiante vincule su celular."
	}
	writeJSON(w, http.StatusOK, resp)
}
