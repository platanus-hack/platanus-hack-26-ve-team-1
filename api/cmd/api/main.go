package main

import (
	"context"
	"crypto/ecdsa"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/kapso"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
)

func main() {
	var origins []string
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer, api.CORS(origins))
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if url := os.Getenv("DATABASE_URL"); url != "" {
		db, err := pgxpool.New(ctx, url)
		if err != nil {
			log.Fatalf("base de datos: %v", err)
		}
		defer db.Close()

		secret := os.Getenv("JWT_SECRET")
		if len(secret) < 16 {
			log.Fatal("JWT_SECRET falta o es muy corto (mínimo 16 caracteres)")
		}
		issuers := map[byte]*ecdsa.PrivateKey{}
		for env, code := range map[string]byte{
			"ISSUER_NACIONAL_KEY":    sign.IssuerNacional,
			"ISSUER_UNIVERSIDAD_KEY": sign.IssuerUniversidad,
		} {
			v := os.Getenv(env)
			if v == "" {
				log.Printf("aviso: %s vacío; el emisor 0x%02x no podrá firmar", env, code)
				continue
			}
			k, err := sign.ParsePrivateB64(v)
			if err != nil {
				log.Fatalf("%s inválida: %v", env, err)
			}
			issuers[code] = k
		}
		srv := &api.Server{DB: db, JWTSecret: []byte(secret), Issuers: issuers, ValidDays: 365,
			WebhookSecret: os.Getenv("KAPSO_WEBHOOK_SECRET"), AllowedOrigins: origins}
		if key, id := os.Getenv("KAPSO_API_KEY"), os.Getenv("KAPSO_PHONE_NUMBER_ID"); key != "" && id != "" {
			kc := kapso.New(key, id)
			if base := os.Getenv("KAPSO_BASE_URL"); base != "" { // opcional: apuntar a un mock en pruebas
				kc.BaseURL = base
			}
			srv.Notifier = kc
		} else {
			log.Print("aviso: KAPSO_API_KEY o KAPSO_PHONE_NUMBER_ID vacíos; no se enviarán avisos de WhatsApp")
		}
		if srv.WebhookSecret == "" {
			log.Print("aviso: KAPSO_WEBHOOK_SECRET vacío; el webhook de Kapso responderá 503")
		}
		srv.Routes(r)
		srv.Start(ctx)
		defer srv.Wait()
	} else {
		log.Print("DATABASE_URL vacío: solo /api/health")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("slj-api escuchando en :%s", port)
	log.Fatal(srv.ListenAndServe())
}
