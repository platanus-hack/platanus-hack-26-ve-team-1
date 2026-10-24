package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type attrInfo struct {
	Codigo   string  `json:"codigo"`
	Nombre   string  `json:"nombre"`
	Presente bool    `json:"presente"`
	Valor    string  `json:"valor,omitempty"`
	Vence    *string `json:"vence,omitempty"`
}

// cardInfo: GET /api/card. La cédula de la cuenta: estado y los seis atributos.
// Toma la activa; si no hay, la revocada más reciente; si no hay ninguna, "sin cédula".
func (s *Server) cardInfo(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var handle []byte
	var status string
	var issued time.Time
	err := s.DB.QueryRow(r.Context(),
		`SELECT handle, status, issued_at FROM cards WHERE citizen_id = $1
		  ORDER BY (status = 'activa') DESC, issued_at DESC LIMIT 1`, u.ID).Scan(&handle, &status, &issued)
	noCard := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !noCard {
		serverErr(w, err)
		return
	}
	have := map[int]attrInfo{}
	var emitida *string
	estado := "sin cédula"
	if !noCard {
		estado = status
		v := issued.UTC().Format(time.RFC3339)
		emitida = &v
		rows, err := s.DB.Query(r.Context(), `SELECT code, value, expires_at FROM attributes WHERE handle = $1`, handle)
		if err != nil {
			serverErr(w, err)
			return
		}
		for rows.Next() {
			var code int
			var val []byte
			var exp time.Time
			if err := rows.Scan(&code, &val, &exp); err != nil {
				rows.Close()
				serverErr(w, err)
				return
			}
			v := exp.UTC().Format(time.RFC3339)
			text := string(val)
			if code == 1 || code == 2 {
				text = "Sí"
			}
			have[code] = attrInfo{Presente: true, Valor: text, Vence: &v}
		}
		rows.Close()
	}
	list := []attrInfo{}
	for code := 1; code <= 6; code++ {
		a := have[code]
		a.Codigo, a.Nombre = attrLabel(code), attrNames[code]
		list = append(list, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "estado": estado, "emitidaEn": emitida, "atributos": list})
}

type optionalOut struct {
	OK       bool   `json:"ok"`
	Sangre   string `json:"sangre"`
	Alergias string `json:"alergias"`
	Contacto struct {
		Nombre   string `json:"nombre"`
		Telefono string `json:"telefono"`
	} `json:"contacto"`
	Donante  bool   `json:"donante"`
	Licencia string `json:"licencia"`
}

func optionalResponse(o *OptionalData) optionalOut {
	out := optionalOut{OK: true}
	if o == nil {
		return out
	}
	out.Sangre, out.Alergias, out.Donante, out.Licencia = o.Sangre, o.Alergias, o.Donante, o.Licencia
	if o.Contacto != nil {
		out.Contacto.Nombre, out.Contacto.Telefono = o.Contacto.Nombre, o.Contacto.Telefono
	}
	return out
}

func (s *Server) getOptionalData(w http.ResponseWriter, r *http.Request) {
	var raw []byte
	if err := s.DB.QueryRow(r.Context(), `SELECT profile FROM citizens WHERE id = $1`, userFrom(r.Context()).ID).Scan(&raw); err != nil {
		serverErr(w, err)
		return
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	writeJSON(w, http.StatusOK, optionalResponse(p.Optional))
}

// putOptionalData: PUT /api/me/optional-data. Sangre, alergias, contacto, donante y licencia.
// Se escriben en el chip en la próxima emisión; los textos de A4, A5 y A6 caben en 16 bytes.
func (s *Server) putOptionalData(w http.ResponseWriter, r *http.Request) {
	var in OptionalData
	if !readJSON(w, r, &in) {
		return
	}
	if _, err := (Profile{Optional: &in}).Attributes(s.now()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error()+": acorta los datos (máximo 16 bytes por atributo)")
		return
	}
	b, _ := json.Marshal(in)
	if _, err := s.DB.Exec(r.Context(),
		`UPDATE citizens SET profile = jsonb_set(profile, '{optional}', $1::jsonb, true) WHERE id = $2`,
		string(b), userFrom(r.Context()).ID); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, optionalResponse(&in))
}

// --- Panel del emisor ---

// listCitizens: GET /api/citizens. Sin handle ni UID.
func (s *Server) listCitizens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT c.id, c.name, c.email, c.profile,
		        coalesce((SELECT k.status FROM cards k WHERE k.citizen_id = c.id ORDER BY (k.status = 'activa') DESC, k.issued_at DESC LIMIT 1), 'sin emitir'),
		        (SELECT k.issued_at FROM cards k WHERE k.citizen_id = c.id ORDER BY (k.status = 'activa') DESC, k.issued_at DESC LIMIT 1)
		   FROM citizens c WHERE c.role = 'ciudadano' ORDER BY c.id`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID           int64           `json:"id"`
		Nombre       string          `json:"nombre"`
		Correo       string          `json:"correo"`
		EstadoCedula string          `json:"estadoCedula"`
		EmitidaEn    *string         `json:"emitidaEn"`
		Profile      json.RawMessage `json:"profile"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		var issued *time.Time
		if err := rows.Scan(&it.ID, &it.Nombre, &it.Correo, &it.Profile, &it.EstadoCedula, &issued); err != nil {
			serverErr(w, err)
			return
		}
		if issued != nil {
			v := issued.UTC().Format(time.RFC3339)
			it.EmitidaEn = &v
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "citizens": out})
}

// emisionStatus: GET /api/emision/status[?ciudadanoId=]. Estado de la última orden del emisor.
func (s *Server) emisionStatus(w http.ResponseWriter, r *http.Request) {
	citizen, _ := atoi(r.URL.Query().Get("ciudadanoId"))
	var id, cid int64
	var created time.Time
	var used *time.Time
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, citizen_id, created_at, used_at FROM issuance_requests
		  WHERE requested_by = $1 AND ($2 = 0 OR citizen_id = $2) ORDER BY created_at DESC, id DESC LIMIT 1`,
		userFrom(r.Context()).ID, citizen).Scan(&id, &cid, &created, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "estado": "error", "mensaje": "No hay una orden de emisión."})
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	out := map[string]any{"ok": true, "ciudadanoId": cid}
	switch {
	case used != nil:
		out["estado"] = "completado"
	case created.Before(s.now().Add(-requestTTL)):
		out["estado"], out["mensaje"] = "error", "Nadie acercó una tarjeta al terminal a tiempo."
	default:
		out["estado"] = "pendiente"
	}
	writeJSON(w, http.StatusOK, out)
}

// listStudents: GET /api/students. Cuentas con datos de carnet y si ya lo tienen en la billetera.
func (s *Server) listStudents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT c.id, c.name, c.email, coalesce(c.profile->'carnet'->>'id', ''),
		        EXISTS (SELECT 1 FROM wallet_credentials wc JOIN devices d ON d.id = wc.device_id
		                 WHERE d.citizen_id = c.id AND wc.issuer = 2 AND wc.expires_at > now())
		   FROM citizens c WHERE c.role = 'ciudadano' AND c.profile ? 'carnet' ORDER BY c.id`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID     int64  `json:"id"`
		Nombre string `json:"nombre"`
		Correo string `json:"correo"`
		Carnet string `json:"carnet"`
		Estado string `json:"estado"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		var emitido bool
		if err := rows.Scan(&it.ID, &it.Nombre, &it.Correo, &it.Carnet, &emitido); err != nil {
			serverErr(w, err)
			return
		}
		it.Estado = "Sin carnet"
		if emitido {
			it.Estado = "Carnet emitido"
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "students": out})
}
