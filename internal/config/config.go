package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	Port          string
	AlertProvider string
	KiteAPIKey    string
	KiteAPISecret string
	DataDir       string
	RunsDir       string
	CreateDelayMS int
}

// Load reads configuration from the environment, applying defaults.
func Load() (Config, error) {
	c := Config{
		Port:          getenv("PORT", "8000"),
		AlertProvider: getenv("ALERT_PROVIDER", "kite"),
		KiteAPIKey:    os.Getenv("KITE_API_KEY"),
		KiteAPISecret: os.Getenv("KITE_API_SECRET"),
		DataDir:       getenv("DATA_DIR", "data"),
		RunsDir:       getenv("RUNS_DIR", "runs"),
		CreateDelayMS: getenvInt("CREATE_DELAY_MS", 250),
	}
	if c.AlertProvider == "kite" && (c.KiteAPIKey == "" || c.KiteAPISecret == "") {
		return c, fmt.Errorf("KITE_API_KEY and KITE_API_SECRET are required for the kite provider")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
