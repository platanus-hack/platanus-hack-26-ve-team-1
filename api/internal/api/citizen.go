package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// --- Portal del ciudadano ---

// cardInfo: GET /api/card → {id, estado, emitidaEn, atributos:["mayor_de_18", ...]}; 404 si aún no tiene cédula.
// Toma la activa; si no hay, la revocada más reciente (que no lista atributos).
func (s *Server) cardInfo(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var handle []byte
	var status string
	var issued time.Time
	err := s.DB.QueryRow(r.Context(),
		`SELECT handle, status, issued_at FROM cards WHERE citizen_id = $1
		  ORDER BY (status = 'activa') DESC, issued_at DESC LIMIT 1`, u.ID).Scan(&handle, &status, &issued)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "Aún no tienes una cédula emitida.")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	atributos := []string{}
	if status == "activa" {
		rows, err := s.DB.Query(r.Context(), `SELECT code FROM attributes WHERE handle = $1 AND expires_at > now() ORDER BY code`, handle)
		if err != nil {
			serverErr(w, err)
			return
		}
		for rows.Next() {
			var code int
			if err := rows.Scan(&code); err != nil {
				rows.Close()
				serverErr(w, err)
				return
			}
			atributos = append(atributos, attrKey(code))
		}
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": "c-" + itoa(u.ID), "estado": status, "emitidaEn": issued.UTC().Format(time.RFC3339), "atributos": atributos,
	})
}

type optionalFront struct {
	TipoSangre         *string `json:"tipoSangre"`
	Alergias           string  `json:"alergias"`
	ContactoEmergencia struct {
		Nombre   string `json:"nombre"`
		Telefono string `json:"telefono"`
	} `json:"contactoEmergencia"`
	Donante bool `json:"donante"`
}

var bloodTypes = map[string]bool{"A+": true, "A-": true, "B+": true, "B-": true, "AB+": true, "AB-": true, "O+": true, "O-": true}
var e164 = regexp.MustCompile(`^\+[0-9]{8,15}$`)

func toFront(o *OptionalData) optionalFront {
	var f optionalFront
	if o == nil {
		return f
	}
	if o.Sangre != "" {
		v := o.Sangre
		f.TipoSangre = &v
	}
	f.Alergias, f.Donante = o.Alergias, o.Donante
	if o.Contacto != nil {
		f.ContactoEmergencia.Nombre, f.ContactoEmergencia.Telefono = o.Contacto.Nombre, o.Contacto.Telefono
	}
	return f
}

func (s *Server) loadProfile(r *http.Request, id int64) (Profile, error) {
	var raw []byte
	var p Profile
	if err := s.DB.QueryRow(r.Context(), `SELECT profile FROM citizens WHERE id = $1`, id).Scan(&raw); err != nil {
		return p, err
	}
	_ = json.Unmarshal(raw, &p)
	return p, nil
}

func (s *Server) getOptionalData(w http.ResponseWriter, r *http.Request) {
	p, err := s.loadProfile(r, userFrom(r.Context()).ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toFront(p.Optional))
}

// putOptionalData: PUT /api/me/optional-data. Los textos de A4 (sangre y alergias) y A5 (contacto)
// deben caber en 16 bytes UTF-8; se escriben en el chip en la próxima emisión.
func (s *Server) putOptionalData(w http.ResponseWriter, r *http.Request) {
	var in optionalFront
	if !readJSON(w, r, &in) {
		return
	}
	sangre := ""
	if in.TipoSangre != nil {
		sangre = strings.TrimSpace(*in.TipoSangre)
		if sangre != "" && !bloodTypes[sangre] {
			writeErr(w, http.StatusBadRequest, "Tipo de sangre inválido.")
			return
		}
	}
	nombre, tel := strings.TrimSpace(in.ContactoEmergencia.Nombre), strings.TrimSpace(in.ContactoEmergencia.Telefono)
	if (nombre == "") != (tel == "") {
		writeErr(w, http.StatusBadRequest, "El contacto de emergencia necesita nombre y teléfono, o ambos vacíos.")
		return
	}
	if tel != "" && !e164.MatchString(tel) {
		writeErr(w, http.StatusBadRequest, "El teléfono del contacto debe tener formato internacional, por ejemplo +584141234567.")
		return
	}
	cur, err := s.loadProfile(r, userFrom(r.Context()).ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	o := &OptionalData{Sangre: sangre, Alergias: strings.TrimSpace(in.Alergias), Donante: in.Donante}
	if cur.Optional != nil {
		o.Licencia = cur.Optional.Licencia // la licencia no se edita desde el portal
	}
	if nombre != "" {
		o.Contacto = &struct {
			Nombre   string `json:"nombre"`
			Telefono string `json:"telefono"`
		}{nombre, tel}
	}
	blood, contact, _ := o.derived()
	if n := len(blood); n > 16 {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf(
			"El tipo de sangre y las alergias suman %d bytes y el máximo es 16 (el tipo de sangre cuenta). Acorta las alergias.", n))
		return
	}
	if n := len(contact); n > 16 {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf(
			"El contacto de emergencia (nombre y teléfono) suma %d bytes y el máximo es 16. Acorta el nombre.", n))
		return
	}
	if !utf8.ValidString(blood + contact) {
		writeErr(w, http.StatusBadRequest, "Los datos contienen caracteres inválidos.")
		return
	}
	b, _ := json.Marshal(o)
	if _, err := s.DB.Exec(r.Context(),
		`UPDATE citizens SET profile = jsonb_set(profile, '{optional}', $1::jsonb, true) WHERE id = $2`,
		string(b), userFrom(r.Context()).ID); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toFront(o))
}

// --- Panel del emisor ---

// perfilCompleto: tiene fecha de nacimiento y número de cédula.
func (p Profile) perfilCompleto() bool {
	return p.Birthdate != "" && p.Identity != nil && p.Identity.ID != ""
}

// listCitizens: GET /api/citizens → Citizen[]. Sin handle, UID, fecha de nacimiento ni cédula.
func (s *Server) listCitizens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT c.id, c.name, c.email, c.profile,
		        coalesce((SELECT k.status FROM cards k WHERE k.citizen_id = c.id ORDER BY (k.status = 'activa') DESC, k.issued_at DESC LIMIT 1), 'sin_emitir'),
		        (SELECT k.issued_at FROM cards k WHERE k.citizen_id = c.id ORDER BY (k.status = 'activa') DESC, k.issued_at DESC LIMIT 1)
		   FROM citizens c WHERE c.role = 'ciudadano' ORDER BY c.id`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID             string  `json:"id"`
		Nombre         string  `json:"nombre"`
		Email          string  `json:"email"`
		EstadoCedula   string  `json:"estadoCedula"`
		EmitidaEn      *string `json:"emitidaEn"`
		PerfilCompleto bool    `json:"perfilCompleto"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		var id int64
		var raw []byte
		var issued *time.Time
		if err := rows.Scan(&id, &it.Nombre, &it.Email, &raw, &it.EstadoCedula, &issued); err != nil {
			serverErr(w, err)
			return
		}
		it.ID = itoa(id)
		var p Profile
		_ = json.Unmarshal(raw, &p)
		it.PerfilCompleto = p.perfilCompleto()
		if issued != nil {
			v := issued.UTC().Format(time.RFC3339)
			it.EmitidaEn = &v
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, out)
}

// emisionStatus: GET /api/emision/status[?order=<id>][&ciudadanoId=]. Estado de la última orden del emisor.
func (s *Server) emisionStatus(w http.ResponseWriter, r *http.Request) {
	citizen, _ := atoi(r.URL.Query().Get("ciudadanoId"))
	order, _ := atoi(r.URL.Query().Get("order"))
	var id, cid int64
	var created time.Time
	var used *time.Time
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, citizen_id, created_at, used_at FROM issuance_requests
		  WHERE requested_by = $1 AND ($2 = 0 OR citizen_id = $2) AND ($3 = 0 OR id = $3)
		  ORDER BY created_at DESC, id DESC LIMIT 1`,
		userFrom(r.Context()).ID, citizen, order).Scan(&id, &cid, &created, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"estado": "error", "mensaje": "No hay una orden de emisión."})
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	out := map[string]any{"id": id, "ciudadanoId": itoa(cid)}
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

// listStudents: GET /api/students → Student[]. Cuentas con perfil de identidad o carnet.
func (s *Server) listStudents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT c.id, c.name, c.email,
		        (SELECT max(wc.created_at) FROM wallet_credentials wc JOIN devices d ON d.id = wc.device_id
		          WHERE d.citizen_id = c.id AND wc.issuer = 2 AND wc.expires_at > now())
		   FROM citizens c WHERE c.role = 'ciudadano' AND (c.profile ? 'carnet' OR c.profile ? 'identity') ORDER BY c.id`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID           string  `json:"id"`
		Nombre       string  `json:"nombre"`
		Email        string  `json:"email"`
		EstadoCarnet string  `json:"estadoCarnet"`
		EmitidoEn    *string `json:"emitidoEn"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		var id int64
		var at *time.Time
		if err := rows.Scan(&id, &it.Nombre, &it.Email, &at); err != nil {
			serverErr(w, err)
			return
		}
		it.ID, it.EstadoCarnet = itoa(id), "sin_carnet"
		if at != nil {
			v := at.UTC().Format(time.RFC3339)
			it.EmitidoEn, it.EstadoCarnet = &v, "emitido"
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, out)
}
