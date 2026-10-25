package api

import (
	"context"
	"net/http"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

func (s *Server) issuers(w http.ResponseWriter, _ *http.Request) {
	out := map[string]sign.JWK{}
	for code, k := range s.Issuers {
		out["0x"+hexByte(code)] = sign.PublicJWK(&k.PublicKey)
	}
	writeJSON(w, http.StatusOK, out)
}

func hexByte(b byte) string {
	const d = "0123456789abcdef"
	return string([]byte{d[b>>4], d[b&15]})
}

// revocations devuelve los handles de las cédulas revocadas.
func (s *Server) revocations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(), `SELECT encode(handle, 'hex') FROM cards WHERE status = 'revocada' ORDER BY revoked_at`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	hs := []string{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			serverErr(w, err)
			return
		}
		hs = append(hs, h)
	}
	writeJSON(w, http.StatusOK, map[string]any{"hs": hs})
}

// revoke revoca cédulas y borra sus credenciales de billetera. Con {h} una cédula concreta; sin h, todas
// las activas de la cuenta (el emisor indica {ciudadanoId}). Lo puede pedir un emisor o el dueño.
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	var in struct {
		H           string  `json:"h"`
		CiudadanoID flexInt `json:"ciudadanoId"`
	}
	if !readOptionalJSON(w, r, &in) {
		return
	}
	u := userFrom(r.Context())
	if in.CiudadanoID != 0 && u.Role != "emisor" {
		writeErr(w, http.StatusForbidden, "Solo un emisor puede revocar la cédula de otra persona.")
		return
	}
	if in.H != "" {
		handle, err := parseHandle(in.H)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		ok, err := s.Revoke(r.Context(), handle, u.Role == "emisor", u.ID)
		if err != nil {
			serverErr(w, err)
			return
		}
		if !ok {
			writeErr(w, http.StatusNotFound, "cédula activa no encontrada")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	target, ok := s.targetCitizen(w, u, int64(in.CiudadanoID))
	if !ok {
		return
	}
	n, err := s.RevokeCitizen(r.Context(), target)
	if err != nil {
		serverErr(w, err)
		return
	}
	if n == 0 {
		writeErr(w, http.StatusNotFound, "no hay cédulas activas")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cedulas": n})
}

// Revoke marca la cédula como revocada. Si anyOwner es falso, solo se revoca si pertenece a citizenID.
func (s *Server) Revoke(ctx context.Context, handle []byte, anyOwner bool, citizenID int64) (bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE cards SET status = 'revocada', revoked_at = now()
		  WHERE handle = $1 AND status = 'activa' AND ($2 OR citizen_id = $3)`, handle, anyOwner, citizenID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM wallet_credentials WHERE handle = $1`, handle); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// events registra una lectura (verif) o una alarma de manipulación (tamper) del terminal.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	term, _ := terminal.From(r.Context())
	var in struct {
		T   string `json:"t"`
		H   string `json:"h"`
		A   string `json:"a"`
		Res string `json:"res"`
		TS  int64  `json:"ts"`
		Emg bool   `json:"emg"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	// El gateway puede reenviar eventos viejos de su cola; se acepta hasta 30 días atrás.
	now := s.now()
	at := time.Unix(in.TS, 0)
	if in.TS == 0 || at.After(now.Add(5*time.Minute)) || at.Before(now.AddDate(0, 0, -30)) {
		at = now
	}

	switch in.T {
	case "tamper":
		_, err := s.DB.Exec(r.Context(),
			`INSERT INTO events (kind, terminal_id, created_at) VALUES ('tamper', $1, $2)`, term.ID, at)
		if err != nil {
			serverErr(w, err)
			return
		}
		s.notifyTamper(term.ID, term.Local, at)
	case "verif":
		handle, err := parseHandle(in.H)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		attr, ok := parseAttr(in.A)
		res, ok2 := parseRes(in.Res)
		if !ok || !ok2 {
			writeErr(w, http.StatusBadRequest, "atributo o resultado inválido")
			return
		}
		var eventID int64
		err = s.DB.QueryRow(r.Context(),
			`INSERT INTO events (kind, handle, terminal_id, attribute, result, emergency, created_at)
			 VALUES ('verif', $1, $2, $3, $4, $5, $6) RETURNING id`, handle, term.ID, attr, res, in.Emg, at).Scan(&eventID)
		if err != nil {
			serverErr(w, err)
			return
		}
		s.publishEvent(r.Context(), eventID, handle, term.Local, attr, res, in.Emg, at)
		// Se avisa al ciudadano tanto si se autorizó el acceso como si se denegó.
		s.notifyUse(handle, term.Local, attr, res, in.Emg, at)
	default:
		writeErr(w, http.StatusBadRequest, "t debe ser verif o tamper")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// parseAttr convierte "A1".."A6" a 1..6.
func parseAttr(s string) (int, bool) {
	if len(s) == 2 && s[0] == 'A' && s[1] >= '1' && s[1] <= '6' {
		return int(s[1] - '0'), true
	}
	return 0, false
}

// parseRes convierte "01".."06" a 1..6.
func parseRes(s string) (int, bool) {
	if len(s) == 2 && s[0] == '0' && s[1] >= '1' && s[1] <= '6' {
		return int(s[1] - '0'), true
	}
	return 0, false
}
