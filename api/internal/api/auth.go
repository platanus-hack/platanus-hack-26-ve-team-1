package api

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
)

const cookieName = "slj_session"
const sessionTTL = 7 * 24 * time.Hour

// User usa los nombres del front. Telefono va en formato +58...
type User struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"nombre"`
	Role          string `json:"rol"`
	Whatsapp      string `json:"telefono"`
	Notifications bool   `json:"avisosActivados"`
}

// userEnv devuelve los campos del usuario en la raíz y también dentro de "user".
type userEnv struct {
	OK bool `json:"ok"`
	User
	Nested User `json:"user"`
}

func envelope(u User) userEnv { return userEnv{OK: true, User: u, Nested: u} }

// phoneOut da el teléfono guardado (solo dígitos) como +58...
func phoneOut(digits string) string {
	if digits == "" {
		return ""
	}
	return "+" + digits
}

type userKey struct{}

// dummyHash iguala el tiempo de respuesta cuando el correo no existe.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, id int64) error {
	exp := s.now().Add(sessionTTL)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   itoa(id),
		IssuedAt:  jwt.NewNumericDate(s.now()),
		ExpiresAt: jwt.NewNumericDate(exp),
	}).SignedString(s.JWTSecret)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/", Expires: exp,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	return nil
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Nombre   string `json:"nombre"`
		Phone    string `json:"whatsapp"`
		Telefono string `json:"telefono"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Name = firstNonEmpty(in.Nombre, in.Name)
	in.Phone = firstNonEmpty(in.Telefono, in.Phone)
	addr, err := mail.ParseAddress(strings.TrimSpace(in.Email))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "correo inválido")
		return
	}
	email := strings.ToLower(addr.Address)
	if len(in.Password) < 8 || len(in.Password) > 72 {
		writeErr(w, http.StatusBadRequest, "la contraseña debe tener entre 8 y 72 caracteres")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		serverErr(w, err)
		return
	}
	var phone *string
	if p := kapso.NormalizePhone(in.Phone); p != "" {
		phone = &p
	}
	// El rol siempre es ciudadano: emisor y comercio se asignan fuera de la API.
	u := User{Email: email, Name: strings.TrimSpace(in.Name), Role: "ciudadano", Notifications: true}
	if phone != nil {
		u.Whatsapp = phoneOut(*phone)
	}
	err = s.DB.QueryRow(r.Context(),
		`INSERT INTO citizens (email, password_hash, name, whatsapp_phone) VALUES ($1,$2,$3,$4) RETURNING id`,
		email, string(hash), u.Name, phone).Scan(&u.ID)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := s.setCookie(w, r, u.ID); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, envelope(u))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !readJSON(w, r, &in) {
		return
	}
	var u User
	var hash string
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, email, name, role, coalesce('+' || whatsapp_phone, ''), notifications, password_hash FROM citizens WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Whatsapp, &u.Notifications, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		writeErr(w, http.StatusUnauthorized, "correo o contraseña incorrectos")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "correo o contraseña incorrectos")
		return
	}
	if err := s.setCookie(w, r, u.ID); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope(u))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// session valida la cookie y carga al usuario desde la base, así los cambios de rol aplican de inmediato.
func (s *Server) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "inicia sesión")
			return
		}
		claims := &jwt.RegisteredClaims{}
		_, err = jwt.ParseWithClaims(c.Value, claims, func(*jwt.Token) (any, error) { return s.JWTSecret, nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		id, ok := atoi(claims.Subject)
		if err != nil || !ok {
			writeErr(w, http.StatusUnauthorized, "sesión inválida")
			return
		}
		u := User{ID: id}
		err = s.DB.QueryRow(r.Context(),
			`SELECT email, name, role, coalesce('+' || whatsapp_phone, ''), notifications FROM citizens WHERE id = $1`, id).
			Scan(&u.Email, &u.Name, &u.Role, &u.Whatsapp, &u.Notifications)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusUnauthorized, "sesión inválida")
			return
		}
		if err != nil {
			serverErr(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

func requireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r.Context())
			for _, role := range roles {
				if u.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeErr(w, http.StatusForbidden, "no tienes permiso")
		})
	}
}

func userFrom(ctx context.Context) User {
	u, _ := ctx.Value(userKey{}).(User)
	return u
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, envelope(userFrom(r.Context())))
}

// patchMe actualiza el teléfono de WhatsApp y si se reciben avisos.
func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	var in notificationsIn
	if !readJSON(w, r, &in) {
		return
	}
	s.updateNotifications(w, r, in)
}

type notificationsIn struct {
	Whatsapp      *string `json:"whatsapp"`
	Telefono      *string `json:"telefono"`
	Notifications *bool   `json:"notifications"`
	Avisos        *bool   `json:"avisosActivados"`
}

// putNotifications: PUT /api/me/notifications {telefono, avisosActivados}.
func (s *Server) putNotifications(w http.ResponseWriter, r *http.Request) {
	var in notificationsIn
	if !readJSON(w, r, &in) {
		return
	}
	s.updateNotifications(w, r, in)
}

func (s *Server) updateNotifications(w http.ResponseWriter, r *http.Request, in notificationsIn) {
	if in.Telefono != nil {
		in.Whatsapp = in.Telefono
	}
	if in.Avisos != nil {
		in.Notifications = in.Avisos
	}
	u := userFrom(r.Context())
	u.Whatsapp = kapso.NormalizePhone(u.Whatsapp)
	if in.Whatsapp != nil {
		p := kapso.NormalizePhone(*in.Whatsapp)
		if p != "" && (len(p) < 8 || len(p) > 15) {
			writeErr(w, http.StatusBadRequest, "teléfono inválido")
			return
		}
		u.Whatsapp = p
	}
	if in.Notifications != nil {
		u.Notifications = *in.Notifications
	}
	var phone *string
	if u.Whatsapp != "" {
		phone = &u.Whatsapp
	}
	if _, err := s.DB.Exec(r.Context(), `UPDATE citizens SET whatsapp_phone = $1, notifications = $2 WHERE id = $3`,
		phone, u.Notifications, u.ID); err != nil {
		serverErr(w, err)
		return
	}
	u.Whatsapp = phoneOut(u.Whatsapp)
	writeJSON(w, http.StatusOK, envelope(u))
}
