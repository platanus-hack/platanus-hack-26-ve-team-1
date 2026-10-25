package api

import (
	"net/http"
	"net/url"
	"strings"
)

// CORS permite peticiones con credenciales desde los orígenes exactos indicados (CORS_ORIGINS).
// Con el front en el mismo origen (rewrite) no hace falta configurarlo.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			allowed[o] = true
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
				h := w.Header()
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, Last-Event-ID")
					h.Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originAllowed dice si el Origin de una petición con cookie es el de la propia API o uno permitido.
func (s *Server) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if s.isAllowedOrigin(origin) {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

func (s *Server) isAllowedOrigin(origin string) bool {
	for _, o := range s.AllowedOrigins {
		if strings.TrimRight(strings.TrimSpace(o), "/") == origin {
			return true
		}
	}
	return false
}

// crossSite indica que la petición viene de un origen permitido distinto del de la API: la cookie
// debe ser SameSite=None para que el navegador la envíe.
func (s *Server) crossSite(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || !s.isAllowedOrigin(origin) {
		return false
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != r.Host
}
