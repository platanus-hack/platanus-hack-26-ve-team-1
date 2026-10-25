package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
)

// Notifier envía mensajes de WhatsApp. *kapso.Client lo cumple; en las pruebas se sustituye.
type Notifier interface {
	SendText(ctx context.Context, to, body string) error
	SendButtons(ctx context.Context, to, body string, buttons []kapso.Button) error
}

var attrNames = map[int]string{
	1: "Mayor de 18", 2: "Mayor de 21", 3: "Identidad",
	4: "Sangre y alergias", 5: "Contacto de emergencia", 6: "Donante y licencia",
}

// Hora de Venezuela (UTC-4, sin horario de verano); no depende de tzdata en el servidor.
var caracas = time.FixedZone("VET", -4*3600)

func fmtWhen(at, now time.Time) string {
	at, now = at.In(caracas), now.In(caracas)
	if at.Format("2006-01-02") == now.Format("2006-01-02") {
		return "hoy a las " + at.Format("15:04")
	}
	return "el " + at.Format("02/01") + " a las " + at.Format("15:04")
}

// goNotify corre fn en segundo plano para no retrasar la respuesta al terminal.
func (s *Server) goNotify(fn func(ctx context.Context)) {
	if s.Notifier == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fn(ctx)
	}()
}

// Wait espera a que terminen los avisos pendientes (para las pruebas y el apagado).
func (s *Server) Wait() { s.wg.Wait() }

// sendBlockable manda el aviso con el botón de bloqueo; si los botones fallan, cae a texto.
func (s *Server) sendBlockable(ctx context.Context, to, body string) {
	err := s.Notifier.SendButtons(ctx, to, body, []kapso.Button{{ID: "bloquear", Title: "Bloquear cédula"}})
	if err == nil {
		return
	}
	log.Printf("aviso con botones falló, se envía texto: %v", err)
	if err := s.Notifier.SendText(ctx, to, body+" Responde BLOQUEAR."); err != nil {
		log.Printf("aviso de WhatsApp falló: %v", err)
	}
}

// useMessage arma el aviso de una lectura según su resultado (1..6). blockable indica si lleva el
// botón "Bloquear cédula"; con la cédula ya revocada (3) no tiene sentido ofrecerlo.
func useMessage(name, local string, attr, res int, emergency bool, when string) (body string, blockable bool) {
	hi := "Hola"
	if name != "" {
		hi = "Hola " + name
	}
	dato := attrNames[attr]
	switch res {
	case 1:
		if emergency {
			return fmt.Sprintf("%s: un terminal de emergencia (%s) leyó tu tipo de sangre y contacto %s. ¿No fuiste tú?", hi, local, when), true
		}
		return fmt.Sprintf("%s: se autorizó el acceso. Tu cédula compartió '%s' con %s %s. ¿No fuiste tú?", hi, dato, local, when), true
	case 2:
		return fmt.Sprintf("%s: se negó el acceso en %s %s porque la firma de la tarjeta no es válida (posible copia o tarjeta alterada). ¿No fuiste tú?", hi, local, when), true
	case 3:
		return fmt.Sprintf("%s: se negó el acceso en %s %s porque tu cédula está bloqueada. Puedes reactivarla desde el portal.", hi, local, when), false
	case 4:
		return fmt.Sprintf("%s: se negó el acceso en %s %s porque tu cédula está vencida. ¿No fuiste tú?", hi, local, when), true
	case 5:
		return fmt.Sprintf("%s: %s pidió '%s' %s, un dato que su terminal no tiene permiso para leer. No se compartió nada. ¿No fuiste tú?", hi, local, dato, when), true
	case 6:
		return fmt.Sprintf("%s: se canceló el acceso en %s %s porque no aceptaste compartir '%s'. No se compartió nada. ¿No fuiste tú?", hi, local, when, dato), true
	}
	return "", false
}

// notifyUse avisa al dueño de la cédula del resultado de una lectura: acceso autorizado o denegado.
func (s *Server) notifyUse(handle []byte, terminalID int64, local string, attr, res int, emergency bool, at time.Time) {
	s.goNotify(func(ctx context.Context) {
		var name, phone string
		err := s.DB.QueryRow(ctx,
			`SELECT c.name, c.whatsapp_phone FROM cards k JOIN citizens c ON c.id = k.citizen_id
			  WHERE k.handle = $1 AND c.notifications AND c.whatsapp_phone IS NOT NULL`, handle).Scan(&name, &phone)
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		if err != nil {
			log.Printf("buscar teléfono para el aviso: %v", err)
			return
		}
		body, blockable := useMessage(name, local, attr, res, emergency, fmtWhen(at, s.now()))
		if !emergency && (res == 1 || res == 6) {
			// Si hubo una solicitud de consentimiento reciente, el titular ya recibió la pregunta y su
			// confirmación: no se repite el aviso. Un 06 por tiempo agotado sí se avisa.
			var cstatus string
			err := s.DB.QueryRow(ctx,
				`SELECT status FROM consent_requests
				  WHERE handle = $1 AND terminal_id = $2 AND created_at > $3
				  ORDER BY created_at DESC LIMIT 1`, handle, terminalID, s.now().Add(-2*time.Minute)).Scan(&cstatus)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				log.Printf("buscar solicitud de consentimiento: %v", err)
			}
			if err == nil {
				if cstatus != "pendiente" {
					return
				}
				body, blockable = fmt.Sprintf("La solicitud de %s venció y no se compartió nada.", local), false
			}
		}
		if body == "" {
			return
		}
		if blockable {
			s.sendBlockable(ctx, phone, body)
			return
		}
		if err := s.Notifier.SendText(ctx, phone, body); err != nil {
			log.Printf("aviso de WhatsApp falló: %v", err)
		}
	})
}

// notifyTamper avisa al dueño del terminal que se detectó manipulación.
func (s *Server) notifyTamper(terminalID int64, local string, at time.Time) {
	s.goNotify(func(ctx context.Context) {
		var phone string
		err := s.DB.QueryRow(ctx,
			`SELECT c.whatsapp_phone FROM terminals t JOIN citizens c ON c.id = t.owner_id
			  WHERE t.id = $1 AND c.notifications AND c.whatsapp_phone IS NOT NULL`, terminalID).Scan(&phone)
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		if err != nil {
			log.Printf("buscar dueño del terminal: %v", err)
			return
		}
		body := fmt.Sprintf("Alerta: el terminal '%s' detectó manipulación %s. Revisa el equipo.", local, fmtWhen(at, s.now()))
		if err := s.Notifier.SendText(ctx, phone, body); err != nil {
			log.Printf("alerta de manipulación falló: %v", err)
		}
	})
}

// --- Webhook de entrada ---

// recentKeys recuerda las claves de idempotencia ya procesadas, porque Kapso reintenta entregas.
type recentKeys struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func (r *recentKeys) first(key string, now time.Time) bool {
	if key == "" {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen == nil {
		r.seen = map[string]time.Time{}
	}
	for k, t := range r.seen {
		if now.Sub(t) > time.Hour {
			delete(r.seen, k)
		}
	}
	if _, dup := r.seen[key]; dup {
		return false
	}
	r.seen[key] = now
	return true
}

// kapsoWebhook atiende POST /api/webhooks/kapso: BLOQUEAR revoca las cédulas de quien escribe.
func (s *Server) kapsoWebhook(w http.ResponseWriter, r *http.Request) {
	if s.WebhookSecret == "" {
		writeErr(w, http.StatusServiceUnavailable, "webhook sin configurar")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if !kapso.VerifySignature(s.WebhookSecret, body, r.Header.Get("X-Webhook-Signature")) {
		writeErr(w, http.StatusUnauthorized, "firma inválida")
		return
	}
	if ev := r.Header.Get("X-Webhook-Event"); ev != "" && ev != "whatsapp.message.received" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if !s.keys.first(r.Header.Get("X-Idempotency-Key"), s.now()) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "duplicate": true})
		return
	}
	msgs, err := kapso.ParseWebhook(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	for _, m := range msgs {
		if id, approved, ok := m.Consent(); ok {
			if err := s.answerConsent(r.Context(), m.From, id, approved); err != nil {
				log.Printf("consentimiento por WhatsApp: %v", err)
			}
			continue
		}
		if !m.IsBlock() {
			continue
		}
		if err := s.blockByPhone(r.Context(), m.From); err != nil {
			log.Printf("bloqueo por WhatsApp: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// blockByPhone revoca las cédulas activas de las cuentas con ese teléfono y confirma por WhatsApp.
func (s *Server) blockByPhone(ctx context.Context, phone string) error {
	rows, err := s.DB.Query(ctx, `SELECT id FROM citizens WHERE whatsapp_phone = $1`, phone)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil // número desconocido: no se responde
	}
	total := 0
	for _, id := range ids {
		n, err := s.RevokeCitizen(ctx, id)
		if err != nil {
			return err
		}
		total += n
	}
	msg := "Tu cédula fue bloqueada. Puedes reactivarla desde el portal."
	if total == 0 {
		msg = "No tienes cédulas activas para bloquear."
	}
	s.goNotify(func(ctx context.Context) {
		if err := s.Notifier.SendText(ctx, phone, msg); err != nil {
			log.Printf("confirmación de bloqueo falló: %v", err)
		}
	})
	return nil
}

// RevokeCitizen revoca todas las cédulas activas de una cuenta y borra sus credenciales de billetera.
func (s *Server) RevokeCitizen(ctx context.Context, citizenID int64) (int, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx,
		`UPDATE cards SET status = 'revocada', revoked_at = now()
		  WHERE citizen_id = $1 AND status = 'activa' RETURNING handle`, citizenID)
	if err != nil {
		return 0, err
	}
	var handles [][]byte
	for rows.Next() {
		var h []byte
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			return 0, err
		}
		handles = append(handles, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(handles) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM wallet_credentials WHERE handle = ANY($1)`, handles); err != nil {
			return 0, err
		}
	}
	return len(handles), tx.Commit(ctx)
}
