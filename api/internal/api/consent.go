package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

// consentTTL debe coincidir con CONSENT_TIMEOUT_MS del lector (firmware/esp32/include/config.h).
const consentTTL = 60 * time.Second

// consentMessage arma la pregunta que recibe el ciudadano antes de compartir un dato.
func consentMessage(name, local string, attr int) string {
	hi := "Hola"
	if name != "" {
		hi = "Hola " + name
	}
	return fmt.Sprintf("%s: %s pide '%s'. ¿Lo compartes? Solo se comparte ese dato.", hi, local, attrNames[attr])
}

// sendConsent manda la pregunta con botones Sí/No; si los botones fallan, cae a texto.
func (s *Server) sendConsent(ctx context.Context, to, body string, id int64) {
	err := s.Notifier.SendButtons(ctx, to, body, []kapso.Button{
		{ID: fmt.Sprintf("consent:%d:si", id), Title: "Sí, compartir"},
		{ID: fmt.Sprintf("consent:%d:no", id), Title: "No compartir"},
	})
	if err == nil {
		return
	}
	log.Printf("consentimiento con botones falló, se envía texto: %v", err)
	if err := s.Notifier.SendText(ctx, to, body+" Responde SI o NO."); err != nil {
		log.Printf("consentimiento de WhatsApp falló: %v", err)
	}
}

// consentStart atiende POST /api/consent: registra la solicitud y le pregunta al titular por WhatsApp.
func (s *Server) consentStart(w http.ResponseWriter, r *http.Request) {
	term, _ := terminal.From(r.Context())
	var in struct {
		H string `json:"h"`
		A string `json:"a"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	handle, err := parseHandle(in.H)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	attr, ok := parseAttr(in.A)
	if !ok {
		writeErr(w, http.StatusBadRequest, "atributo inválido")
		return
	}

	var status string
	var name, phone *string
	var notifications *bool
	err = s.DB.QueryRow(r.Context(),
		`SELECT k.status, c.name, c.whatsapp_phone, c.notifications
		   FROM cards k LEFT JOIN citizens c ON c.id = k.citizen_id
		  WHERE k.handle = $1`, handle).Scan(&status, &name, &phone, &notifications)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "cédula desconocida")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if status == "revocada" {
		writeErr(w, http.StatusConflict, "cédula revocada")
		return
	}
	if phone == nil || *phone == "" || notifications == nil || !*notifications || s.Notifier == nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "estado": "sin_whatsapp"})
		return
	}

	var id int64
	err = s.DB.QueryRow(r.Context(),
		`INSERT INTO consent_requests (handle, terminal_id, attribute, phone, expires_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		handle, term.ID, attr, *phone, s.now().Add(consentTTL)).Scan(&id)
	if err != nil {
		serverErr(w, err)
		return
	}
	body := consentMessage(deref(name), term.Local, attr)
	to := *phone
	s.goNotify(func(ctx context.Context) { s.sendConsent(ctx, to, body, id) })
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "estado": "pendiente"})
}

// consentStatus atiende GET /api/consent/{id}: solo para el terminal que hizo la solicitud.
func (s *Server) consentStatus(w http.ResponseWriter, r *http.Request) {
	term, _ := terminal.From(r.Context())
	id, ok := atoi(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "solicitud no encontrada")
		return
	}
	var status string
	var expires time.Time
	err := s.DB.QueryRow(r.Context(),
		`SELECT status, expires_at FROM consent_requests WHERE id = $1 AND terminal_id = $2`,
		id, term.ID).Scan(&status, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "solicitud no encontrada")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if status == "pendiente" && expires.Before(s.now()) {
		status = "vencido"
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "estado": status})
}

// answerConsent registra la respuesta del titular. Solo cuenta si la solicitud sigue pendiente, no
// venció y el teléfono coincide: la primera respuesta gana y nadie responde por otro. id = 0 aplica
// a la última solicitud pendiente de ese teléfono.
func (s *Server) answerConsent(ctx context.Context, phone string, id int64, approved bool) error {
	newStatus := "rechazado"
	if approved {
		newStatus = "aprobado"
	}
	var attr int
	var terminalID int64
	err := s.DB.QueryRow(ctx,
		`UPDATE consent_requests
		    SET status = $1, answered_at = now()
		  WHERE status = 'pendiente' AND expires_at > $4 AND phone = $2
		    AND id = COALESCE(NULLIF($3::bigint, 0),
		             (SELECT id FROM consent_requests
		               WHERE phone = $2 AND status = 'pendiente' AND expires_at > $4
		               ORDER BY created_at DESC LIMIT 1))
		RETURNING attribute, terminal_id`, newStatus, phone, id, s.now()).Scan(&attr, &terminalID)
	if errors.Is(err, pgx.ErrNoRows) {
		if id != 0 {
			s.reply(phone, "Esa solicitud ya no está activa.")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !approved {
		s.reply(phone, "No se compartió nada.")
		return nil
	}
	var local string
	if err := s.DB.QueryRow(ctx, `SELECT name FROM terminals WHERE id = $1`, terminalID).Scan(&local); err != nil {
		return err
	}
	s.reply(phone, fmt.Sprintf("Listo: compartiste '%s' con %s.", attrNames[attr], local))
	return nil
}

func (s *Server) reply(phone, msg string) {
	s.goNotify(func(ctx context.Context) {
		if err := s.Notifier.SendText(ctx, phone, msg); err != nil {
			log.Printf("respuesta de consentimiento falló: %v", err)
		}
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
