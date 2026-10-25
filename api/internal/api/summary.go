package api

import (
	"net/http"
	"strconv"
)

type dayRow struct {
	Dia       string `json:"dia"`
	Total     int    `json:"total"`
	Aprobadas int    `json:"aprobadas"`
}

type terminalSummary struct {
	ID             int64  `json:"id"`
	Local          string `json:"local"`
	Total          int    `json:"total"`
	Aprobadas      int    `json:"aprobadas"`
	Rechazadas     int    `json:"rechazadas"`
	Manipulaciones int    `json:"manipulaciones"`
}

// commerceSummary: GET /api/commerce/summary?days=7. Totales de los terminales del comercio
// (o de todos, para el emisor). Nunca incluye handles ni datos de personas.
func (s *Server) commerceSummary(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	days := 7
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 {
		days = min(v, 30)
	}
	ctx := r.Context()
	since := s.now().AddDate(0, 0, -days)
	all := u.Role == "emisor"

	rows, err := s.DB.Query(ctx,
		`SELECT t.id, t.name,
		        count(e.id) FILTER (WHERE e.kind = 'verif'),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result = 1),
		        count(e.id) FILTER (WHERE e.kind = 'verif' AND e.result <> 1),
		        count(e.id) FILTER (WHERE e.kind = 'tamper')
		   FROM terminals t
		   LEFT JOIN events e ON e.terminal_id = t.id AND e.created_at >= $2
		  WHERE ($3 OR t.owner_id = $1)
		  GROUP BY t.id, t.name ORDER BY t.id`, u.ID, since, all)
	if err != nil {
		serverErr(w, err)
		return
	}
	terms := []terminalSummary{}
	var tot terminalSummary
	for rows.Next() {
		var t terminalSummary
		if err := rows.Scan(&t.ID, &t.Local, &t.Total, &t.Aprobadas, &t.Rechazadas, &t.Manipulaciones); err != nil {
			rows.Close()
			serverErr(w, err)
			return
		}
		terms = append(terms, t)
		tot.Total += t.Total
		tot.Aprobadas += t.Aprobadas
		tot.Rechazadas += t.Rechazadas
		tot.Manipulaciones += t.Manipulaciones
	}
	rows.Close()

	porResultado, porAtributo := map[string]int{}, map[string]int{}
	rows, err = s.DB.Query(ctx,
		`SELECT e.result, e.attribute, count(*) FROM events e JOIN terminals t ON t.id = e.terminal_id
		  WHERE e.kind = 'verif' AND e.created_at >= $2 AND ($3 OR t.owner_id = $1)
		  GROUP BY e.result, e.attribute`, u.ID, since, all)
	if err != nil {
		serverErr(w, err)
		return
	}
	for rows.Next() {
		var res, attr, n int
		if err := rows.Scan(&res, &attr, &n); err != nil {
			rows.Close()
			serverErr(w, err)
			return
		}
		porResultado[strconv.Itoa(res)] += n
		porAtributo[attrLabel(attr)] += n
	}
	rows.Close()

	porDia := []dayRow{}
	rows, err = s.DB.Query(ctx,
		`SELECT to_char(e.created_at AT TIME ZONE 'America/Caracas', 'YYYY-MM-DD') AS dia,
		        count(*), count(*) FILTER (WHERE e.result = 1)
		   FROM events e JOIN terminals t ON t.id = e.terminal_id
		  WHERE e.kind = 'verif' AND e.created_at >= $2 AND ($3 OR t.owner_id = $1)
		  GROUP BY dia ORDER BY dia`, u.ID, since, all)
	if err != nil {
		serverErr(w, err)
		return
	}
	for rows.Next() {
		var d dayRow
		if err := rows.Scan(&d.Dia, &d.Total, &d.Aprobadas); err != nil {
			rows.Close()
			serverErr(w, err)
			return
		}
		porDia = append(porDia, d)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "days": days, "totals": tot, "por_resultado": porResultado,
		"por_atributo": porAtributo, "por_dia": porDia, "terminales": terms,
	})
}
