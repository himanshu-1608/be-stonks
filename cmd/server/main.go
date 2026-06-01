package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"be-stonks/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":" + cfg.Port
	log.Printf("listening on %s (alert provider=%s)", addr, cfg.AlertProvider)
	log.Fatal(http.ListenAndServe(addr, r))
}
