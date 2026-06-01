package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DotEnvPath is the optional env file loaded at startup, if present.
const DotEnvPath = ".env"

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

// Load reads configuration from the environment, applying defaults. If a .env
// file is present in the working directory it is loaded first (without
// overriding variables already set in the real environment).
func Load() (Config, error) {
	loadDotEnv(DotEnvPath)

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

// loadDotEnv loads KEY=VALUE pairs from the file at path into the process
// environment. A missing file is ignored. Existing environment variables are
// never overridden, so real env (e.g. systemd EnvironmentFile) wins over .env.
// Lines that are blank or start with '#' are skipped; surrounding quotes on the
// value are trimmed.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
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
