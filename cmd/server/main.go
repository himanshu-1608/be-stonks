package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"be-stonks/internal/alerts"
	"be-stonks/internal/auth"
	"be-stonks/internal/config"
	"be-stonks/internal/provider"
	_ "be-stonks/internal/providers/kite" // register the kite provider
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	p, err := provider.New(cfg.AlertProvider, provider.Config{
		APIKey:    cfg.KiteAPIKey,
		APISecret: cfg.KiteAPISecret,
		DataDir:   cfg.DataDir,
	})
	if err != nil {
		log.Fatalf("provider: %v", err)
	}

	authH := auth.NewHandler(p)
	syncer := alerts.NewSyncer(
		p,
		filepath.Join(cfg.DataDir, "alerts", "recommendation.csv"),
		filepath.Join(cfg.DataDir, "alerts", "mapping.csv"),
		filepath.Join(cfg.RunsDir, "alerts-sync"),
		time.Duration(cfg.CreateDelayMS)*time.Millisecond,
	)
	alertsH := alerts.NewHandler(syncer)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/auth/login", authH.Login)
	r.Get("/auth/callback", authH.Callback)
	r.Post("/alerts/sync", alertsH.Sync)

	addr := ":" + cfg.Port
	log.Printf("listening on %s (alert provider=%s)", addr, cfg.AlertProvider)
	log.Fatal(http.ListenAndServe(addr, r))
}
