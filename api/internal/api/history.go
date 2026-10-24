package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// eventOut es lo único que ve el ciudadano de un evento: nunca el handle ni el UID.
type eventOut struct {
	ID         int64  `json:"id"`
	Local      string `json:"local"`
	Atributo   string `json:"atributo"`  // "A1".."A6"
	Resultado  int    `json:"resultado"` // 1..6
	Hora       string `json:"hora"`      // RFC 3339, UTC
	Emergencia bool   `json:"emergencia"`
}

const eventsSelect = `
	SELECT e.id, coalesce(t.name, ''), e.attribute, e.result, e.created_at, e.emergency
	  FROM events e
	  JOIN cards c ON c.handle = e.handle
	  LEFT JOIN terminals t ON t.id = e.terminal_id
	 WHERE c.citizen_id = $1 AND e.kind = 'verif'`

func scanEvents(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]eventOut, error) {
	defer rows.Close()
	out := []eventOut{}
	for rows.Next() {
		var e eventOut
		var attr, res int
		var at time.Time
		if err := rows.Scan(&e.ID, &e.Local, &attr, &res, &at, &e.Emergencia); err != nil {
			return nil, err
		}
		e.Atributo, e.Resultado, e.Hora = attrLabel(attr), res, at.UTC().Format(time.RFC3339)
		out = append(out, e)
	}
	return out, rows.Err()
}

// listEvents: GET /api/events?limit=50&before=<id>. Los más recientes primero; solo los de la cuenta.
func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = min(v, 200)
	}
	before := int64(1<<62 - 1)
	if v, ok := atoi(r.URL.Query().Get("before")); ok {
		before = v
	}
	rows, err := s.DB.Query(r.Context(), eventsSelect+` AND e.id < $2 ORDER BY e.id DESC LIMIT $3`,
		userFrom(r.Context()).ID, before, limit)
	if err != nil {
		serverErr(w, err)
		return
	}
	evs, err := scanEvents(rows)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "events": evs})
}

func (s *Server) eventsAfter(ctx context.Context, citizenID, after int64) ([]eventOut, error) {
	rows, err := s.DB.Query(ctx, eventsSelect+` AND e.id > $2 ORDER BY e.id LIMIT 200`, citizenID, after)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// --- Tiempo real: LISTEN/NOTIFY + difusión en memoria ---

const notifyChannel = "slj_events"

type hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan eventOut]struct{}
}

func (h *hub) subscribe(citizenID int64) (chan eventOut, func()) {
	ch := make(chan eventOut, 16)
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[int64]map[chan eventOut]struct{}{}
	}
	if h.subs[citizenID] == nil {
		h.subs[citizenID] = map[chan eventOut]struct{}{}
	}
	h.subs[citizenID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[citizenID], ch)
		h.mu.Unlock()
	}
}

func (h *hub) publish(citizenID int64, e eventOut) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[citizenID] {
		select {
		case ch <- e:
		default: // consumidor lento: el sondeo de respaldo lo pone al día
		}
	}
}

type notifyPayload struct {
	CitizenID int64    `json:"citizen_id"`
	Event     eventOut `json:"event"`
}

// publishEvent avisa por NOTIFY de un evento nuevo a todas las instancias de la API.
func (s *Server) publishEvent(ctx context.Context, id int64, handle []byte, local string, attr, res int, emg bool, at time.Time) {
	var citizenID *int64
	if err := s.DB.QueryRow(ctx, `SELECT citizen_id FROM cards WHERE handle = $1`, handle).Scan(&citizenID); err != nil || citizenID == nil {
		return // cédula desconocida o sin dueño: nadie a quién mostrárselo
	}
	b, _ := json.Marshal(notifyPayload{*citizenID, eventOut{id, local, attrLabel(attr), res, at.UTC().Format(time.RFC3339), emg}})
	if _, err := s.DB.Exec(ctx, `SELECT pg_notify($1, $2)`, notifyChannel, string(b)); err != nil {
		log.Printf("notify: %v", err)
	}
}

// Start abre la conexión que escucha NOTIFY y reparte los eventos a los clientes SSE.
// Termina cuando ctx se cancela; si la conexión se cae, reintenta.
func (s *Server) Start(ctx context.Context) {
	go func() {
		for ctx.Err() == nil {
			if err := s.listen(ctx, s.DB); err != nil && ctx.Err() == nil {
				log.Printf("listen: %v (se reintenta)", err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}()
}

func (s *Server) listen(ctx context.Context, db *pgxpool.Pool) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var p notifyPayload
		if json.Unmarshal([]byte(n.Payload), &p) == nil {
			s.hub.publish(p.CitizenID, p.Event)
		}
	}
}

// eventsStream: GET /api/events/stream (SSE). Cada ciudadano recibe solo sus eventos.
// Admite Last-Event-ID para recuperar lo perdido y sondea cada 10 s como respaldo.
func (s *Server) eventsStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "el servidor no admite streaming")
		return
	}
	user := userFrom(r.Context())
	ch, cancel := s.hub.subscribe(user.ID)
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": conectado\n\n")
	fl.Flush()

	last := int64(0)
	if v, ok := atoi(r.Header.Get("Last-Event-ID")); ok {
		last = v
	} else {
		// Sin Last-Event-ID solo se envía lo nuevo; el historial viene de GET /api/events.
		_ = s.DB.QueryRow(r.Context(),
			`SELECT coalesce(max(e.id), 0) FROM events e JOIN cards c ON c.handle = e.handle WHERE c.citizen_id = $1`,
			user.ID).Scan(&last)
	}
	send := func(e eventOut) bool {
		if e.ID <= last {
			return true
		}
		b, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "id: %d\nevent: evento\ndata: %s\n\n", e.ID, b); err != nil {
			return false
		}
		last = e.ID
		fl.Flush()
		return true
	}
	catchUp := func() bool {
		evs, err := s.eventsAfter(r.Context(), user.ID, last)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("stream: %v", err)
			}
			return r.Context().Err() == nil
		}
		for _, e := range evs {
			if !send(e) {
				return false
			}
		}
		return true
	}
	if r.Header.Get("Last-Event-ID") != "" && !catchUp() {
		return
	}

	poll, beat := time.NewTicker(10*time.Second), time.NewTicker(25*time.Second)
	defer poll.Stop()
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			if !send(e) {
				return
			}
		case <-poll.C:
			if !catchUp() {
				return
			}
		case <-beat.C:
			if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
