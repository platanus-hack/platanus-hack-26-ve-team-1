package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	if url := os.Getenv("DATABASE_URL"); url != "" {
		db, err := pgxpool.New(context.Background(), url)
		if err != nil {
			log.Fatalf("base de datos: %v", err)
		}
		defer db.Close()
		r.Group(func(r chi.Router) {
			r.Use(terminal.Auth(db))
			r.Get("/api/terminal/config", terminal.Config)
		})
	} else {
		log.Print("DATABASE_URL vacío: solo /api/health")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("slj-api escuchando en :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
