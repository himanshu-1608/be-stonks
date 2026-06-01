package kite

import (
	"fmt"
	"path/filepath"
	"sync"

	kc "github.com/zerodha/gokiteconnect/v4"

	"be-stonks/internal/provider"
)

// Name is the registry key for this provider.
const Name = "kite"

func init() {
	provider.Register(Name, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

// Kite implements provider.Provider against Zerodha Kite Connect.
// gokiteconnect is used for the login/session flow; the alerts API is called
// directly over HTTP (see alerts.go).
type Kite struct {
	client      *kc.Client
	apiKey      string
	apiSecret   string
	sessionPath string

	mu          sync.RWMutex
	accessToken string
	ready       bool
}

// New constructs a Kite provider and attempts to load an existing session.
func New(cfg provider.Config) (*Kite, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, fmt.Errorf("kite: api key and secret are required")
	}
	k := &Kite{
		client:      kc.New(cfg.APIKey),
		apiKey:      cfg.APIKey,
		apiSecret:   cfg.APISecret,
		sessionPath: filepath.Join(cfg.DataDir, Name, "session.json"),
	}
	k.loadSession() // best-effort; no session is a valid startup state
	return k, nil
}

// Name returns the provider's registry key.
func (k *Kite) Name() string { return Name }

