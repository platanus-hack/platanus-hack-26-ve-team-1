// Package api implementa los endpoints HTTP de SoloLoJusto.
package api

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

type Server struct {
	DB        *pgxpool.Pool
	JWTSecret []byte
	Issuers   map[byte]*ecdsa.PrivateKey
	ValidDays int // vigencia de lo que se firma
	Now       func() time.Time

	AllowedOrigins []string // orígenes del front permitidos con credenciales (CORS_ORIGINS)

	Notifier      Notifier // nil: no se envían avisos de WhatsApp
	WebhookSecret string   // secreto del webhook de Kapso

	wg   sync.WaitGroup
	keys recentKeys
	hub  hub
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Routes registra todas las rutas en r.
func (s *Server) Routes(r chi.Router) {
	// Públicas
	r.Get("/api/issuers", s.issuers)
	r.Get("/api/revocations", s.revocations)
	r.Post("/api/auth/register", s.register)
	r.Post("/api/auth/login", s.login)
	r.Post("/api/auth/logout", s.logout)
	r.Post("/api/webhooks/kapso", s.kapsoWebhook) // autenticado por firma HMAC

	// Sesión (ciudadano, emisor, comercio)
	r.Group(func(r chi.Router) {
		r.Use(s.session)
		r.Get("/api/me", s.me)
		r.Patch("/api/me", s.patchMe)
		r.Put("/api/me/notifications", s.putNotifications)
		r.Get("/api/wallet", s.wallet)
		r.Post("/api/link/confirm", s.linkConfirm)
		r.Post("/api/revoke", s.revoke)
		r.Get("/api/card", s.cardInfo)
		r.Get("/api/me/optional-data", s.getOptionalData)
		r.Put("/api/me/optional-data", s.putOptionalData)
		r.Get("/api/cards", s.listCards)
		r.Post("/api/reactivate", s.reactivate)
		r.Get("/api/events", s.listEvents)
		r.Get("/api/events/stream", s.eventsStream)

		r.Group(func(r chi.Router) {
			r.Use(requireRole("emisor"))
			r.Get("/api/citizens", s.listCitizens)
			r.Put("/api/citizens/{id}/profile", s.putProfile)
			r.Post("/api/emision/request", s.emisionRequest)
			r.Post("/api/carnet/issue", s.issueCarnet)
			r.Get("/api/students", s.listStudents)
			r.Post("/api/credentials/university", s.issueCarnet)
			r.Post("/api/emision/order", s.emisionRequest)
			r.Get("/api/emision/status", s.emisionStatus)
		})
		r.Group(func(r chi.Router) {
			r.Use(requireRole("comercio", "emisor"))
			r.Get("/api/commerce/summary", s.commerceSummary)
			r.Get("/api/stats", s.stats)
			r.Get("/api/terminals", s.listTerminals)
			r.Put("/api/terminals/{id}", s.updateTerminal)
			r.Patch("/api/terminals/{id}", s.updateTerminal)
			r.Delete("/api/terminals/{id}", s.deleteTerminal)
		})
		r.With(requireRole("comercio")).Post("/api/terminals", s.createTerminal)
	})

	// Terminales (gateway): Authorization: Bearer <token>
	r.Group(func(r chi.Router) {
		r.Use(terminal.Auth(s.DB))
		r.Get("/api/terminal/config", terminal.Config)
		r.Post("/api/events", s.events)
		r.Post("/api/emision/start", s.emisionStart)
		r.Post("/api/link/start", s.linkStart)
	})
}

// --- utilidades HTTP ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}

func serverErr(w http.ResponseWriter, err error) {
	writeErr(w, http.StatusInternalServerError, "error interno")
	println("error:", err.Error())
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

func parseHandle(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 8 {
		return nil, errors.New("handle inválido: se esperan 16 caracteres hex")
	}
	return b, nil
}

func parseUID(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || (len(b) != 4 && len(b) != 7 && len(b) != 10) {
		return nil, errors.New("uid inválido: se esperan 4, 7 o 10 bytes en hex")
	}
	return b, nil
}

func parsePub33(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != sign.PubLen {
		return nil, errors.New("public_key inválida: se esperan 33 bytes comprimidos en hex")
	}
	if _, err := sign.DecompressPub(b); err != nil {
		return nil, err
	}
	return b, nil
}

func attrLabel(code int) string { return "A" + string(rune('0'+code)) }
