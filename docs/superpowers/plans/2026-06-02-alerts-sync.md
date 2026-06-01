# Alerts Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A provider-agnostic Go backend with a `POST /alerts/sync` endpoint that turns a recommendation CSV into 3 LTP price alerts per stock on the active broker (Kite), idempotently.

**Architecture:** Modular monolith. Feature logic (`internal/alerts`) depends only on neutral interfaces in `internal/provider`; the Kite implementation lives in `internal/providers/kite` and is selected at startup by config (`ALERT_PROVIDER`). Auth is a generic manual-login flow. Every `/sync` run writes a JSON report. See `context.md` for cross-cutting rules.

**Tech Stack:** Go 1.22, `chi` router, `gokiteconnect/v4` (auth only), Kite alerts via direct HTTP. No automated tests (owner's decision) — verification is `go build ./...` + `go vet ./...` per task, plus manual `curl` at the end.

**Note on TDD:** This project skips automated tests per owner instruction (see `context.md` §10). Each task therefore: write complete code → `go build ./...` and `go vet ./...` must pass → commit. Keep code interface-driven so tests can be added later.

---

### Task 1: Module init + config + health server

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Create: `cmd/server/main.go` (minimal, health only — expanded in Task 10)

- [ ] **Step 1: Init module**

Run:
```bash
go mod init be-stonks
go get github.com/go-chi/chi/v5@latest
go get github.com/zerodha/gokiteconnect/v4@latest
```

- [ ] **Step 2: Write `internal/config/config.go`**

```go
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
```

- [ ] **Step 3: Write minimal `cmd/server/main.go`**

```go
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
```

- [ ] **Step 4: Build & vet**

Run: `go mod tidy && go build ./... && go vet ./...`
Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/config/config.go cmd/server/main.go
git commit -m "feat: module init, config loader, health endpoint"
```

---

### Task 2: Provider-neutral contracts, types, registry

**Files:**
- Create: `internal/provider/types.go`
- Create: `internal/provider/provider.go`
- Create: `internal/provider/registry.go`

- [ ] **Step 1: Write `internal/provider/types.go`**

```go
package provider

// Exchange identifies a stock exchange.
type Exchange string

const ExchangeNSE Exchange = "NSE"

// Operator is a comparison used in an alert condition.
type Operator string

const OpGTE Operator = ">="

// Attribute is the price field an alert watches.
type Attribute string

const AttrLTP Attribute = "LTP"

// AlertSpec is a broker-neutral description of an alert to create.
type AlertSpec struct {
	Name      string
	Symbol    string // tradingsymbol, no exchange suffix (e.g. "TARIL")
	Exchange  Exchange
	Attribute Attribute
	Operator  Operator
	Value     float64
}

// Alert is a broker-neutral view of an existing alert.
type Alert struct {
	Name string
	UUID string
}

// Config carries the inputs a provider constructor needs.
// Kept minimal and broker-neutral to avoid import cycles with the config package.
type Config struct {
	APIKey    string
	APISecret string
	DataDir   string // base data dir; providers store under DataDir/<name>/
}
```

- [ ] **Step 2: Write `internal/provider/provider.go`**

```go
package provider

import "context"

// AlertProvider creates and lists alerts on a broker.
type AlertProvider interface {
	ListAlerts(ctx context.Context) ([]Alert, error)
	CreateAlert(ctx context.Context, spec AlertSpec) error
}

// Authenticator handles a broker's login/session lifecycle.
type Authenticator interface {
	LoginURL() string
	ExchangeToken(requestToken string) error // mints and persists a session
	Ready() bool                             // a session is loaded
}

// Provider is a fully capable broker integration.
// Future capabilities (e.g. TickProvider) get embedded here as they are added.
type Provider interface {
	Name() string
	Authenticator
	AlertProvider
}
```

- [ ] **Step 3: Write `internal/provider/registry.go`**

```go
package provider

import "fmt"

// Constructor builds a Provider from neutral config.
type Constructor func(cfg Config) (Provider, error)

var registry = map[string]Constructor{}

// Register adds a provider constructor under a name. Called from provider
// packages' init() functions.
func Register(name string, c Constructor) {
	registry[name] = c
}

// New constructs the named provider.
func New(name string, cfg Config) (Provider, error) {
	c, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return c(cfg)
}
```

- [ ] **Step 4: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/
git commit -m "feat: provider-neutral contracts, types, and registry"
```

---

### Task 3: Shared IST time helper

**Files:**
- Create: `internal/timeutil/timeutil.go`

- [ ] **Step 1: Write `internal/timeutil/timeutil.go`**

```go
package timeutil

import "time"

// IST is the India Standard Time zone (UTC+5:30).
var IST = time.FixedZone("IST", 5*3600+30*60)

// NowIST returns the current time in IST.
func NowIST() time.Time { return time.Now().In(IST) }

// FileStamp formats a time for use in filenames: 2006-01-02-15-04-05.
func FileStamp(t time.Time) string { return t.Format("2006-01-02-15-04-05") }

// Label formats a human/report-friendly timestamp: 2006-01-02-15-04-05 IST.
func Label(t time.Time) string { return t.Format("2006-01-02-15-04-05") + " IST" }
```

- [ ] **Step 2: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/timeutil/
git commit -m "feat: IST time helpers"
```

---

### Task 4: Kite provider — client + session (Authenticator)

**Files:**
- Create: `internal/providers/kite/kite.go`
- Create: `internal/providers/kite/session.go`

- [ ] **Step 1: Write `internal/providers/kite/kite.go`**

```go
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
```

- [ ] **Step 2: Write `internal/providers/kite/session.go`**

```go
package kite

import (
	"encoding/json"
	"os"
	"path/filepath"

	"be-stonks/internal/timeutil"
)

type sessionFile struct {
	AccessToken string `json:"access_token"`
	SavedAt     string `json:"saved_at"`
}

// LoginURL returns the Kite hosted login URL.
func (k *Kite) LoginURL() string {
	return k.client.GetLoginURL()
}

// ExchangeToken trades a request_token for an access_token, sets it on the
// client, and persists it.
func (k *Kite) ExchangeToken(requestToken string) error {
	sess, err := k.client.GenerateSession(requestToken, k.apiSecret)
	if err != nil {
		return err
	}
	k.client.SetAccessToken(sess.AccessToken)

	k.mu.Lock()
	k.accessToken = sess.AccessToken
	k.ready = true
	k.mu.Unlock()

	return k.saveSession(sess.AccessToken)
}

// Ready reports whether an access token is loaded.
func (k *Kite) Ready() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.ready
}

func (k *Kite) token() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.accessToken
}

func (k *Kite) saveSession(token string) error {
	if err := os.MkdirAll(filepath.Dir(k.sessionPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sessionFile{
		AccessToken: token,
		SavedAt:     timeutil.Label(timeutil.NowIST()),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(k.sessionPath, b, 0o600)
}

func (k *Kite) loadSession() {
	b, err := os.ReadFile(k.sessionPath)
	if err != nil {
		return
	}
	var sf sessionFile
	if err := json.Unmarshal(b, &sf); err != nil || sf.AccessToken == "" {
		return
	}
	k.client.SetAccessToken(sf.AccessToken)
	k.mu.Lock()
	k.accessToken = sf.AccessToken
	k.ready = true
	k.mu.Unlock()
}
```

- [ ] **Step 3: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0. (Kite does not yet satisfy `provider.Provider` — alerts methods come in Task 5 — but it compiles as a standalone type. The registry constructor returns `provider.Provider`, so this WILL fail to compile until Task 5 adds the alert methods.)

> Because the `init()` registers a constructor returning `provider.Provider`, the compiler requires the alert methods now. To keep this task self-contained, add temporary stubs at the end of `kite.go` and delete them in Task 5:

```go
// TEMPORARY stubs — replaced by real implementations in Task 5 (alerts.go).
// Delete these two methods when alerts.go is added.
import "context"

func (k *Kite) ListAlerts(ctx context.Context) ([]provider.Alert, error) { return nil, nil }
func (k *Kite) CreateAlert(ctx context.Context, spec provider.AlertSpec) error { return nil }
```

(Place the `import "context"` with the other imports in `kite.go`, not inline.)

- [ ] **Step 4: Commit**

```bash
git add internal/providers/kite/
git commit -m "feat: kite provider client and session (auth) with temporary alert stubs"
```

---

### Task 5: Kite provider — alerts over HTTP (AlertProvider)

**Files:**
- Create: `internal/providers/kite/alerts.go`
- Modify: `internal/providers/kite/kite.go` (remove the temporary stubs + `context` import added in Task 4)

- [ ] **Step 1: Remove the temporary stubs from `kite.go`**

Delete the two stub methods (`ListAlerts`, `CreateAlert`) and the `context` import added in Task 4.

- [ ] **Step 2: Verify the Kite alert LTP attribute string**

Kite's alert operand attribute for last traded price must match its API exactly. Confirm the value before relying on it.

Run: open https://kite.trade/docs/connect/v3/alerts/ and confirm the `lhs_attribute` value used for last traded price (commonly `LastTradedPrice`).
Expected: note the exact string; set `ltpAttribute` below to it.

- [ ] **Step 3: Write `internal/providers/kite/alerts.go`**

```go
package kite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"be-stonks/internal/provider"
)

const kiteAPIBase = "https://api.kite.trade"

// ltpAttribute is Kite's alert operand attribute for last traded price.
// Verified against https://kite.trade/docs/connect/v3/alerts/ (Task 5, Step 2).
const ltpAttribute = "LastTradedPrice"

func (k *Kite) authHeader() string {
	return "token " + k.apiKey + ":" + k.token()
}

// ListAlerts returns all alerts currently configured on the Kite account.
func (k *Kite) ListAlerts(ctx context.Context) ([]provider.Alert, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kiteAPIBase+"/alerts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", k.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kite list alerts: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out struct {
		Data []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("kite list alerts: decode: %w", err)
	}
	alerts := make([]provider.Alert, 0, len(out.Data))
	for _, a := range out.Data {
		alerts = append(alerts, provider.Alert{Name: a.Name, UUID: a.UUID})
	}
	return alerts, nil
}

// CreateAlert creates a single simple LTP alert on Kite.
func (k *Kite) CreateAlert(ctx context.Context, spec provider.AlertSpec) error {
	form := url.Values{}
	form.Set("name", spec.Name)
	form.Set("type", "simple")
	form.Set("lhs_exchange", string(spec.Exchange))
	form.Set("lhs_tradingsymbol", spec.Symbol)
	form.Set("lhs_attribute", ltpAttribute)
	form.Set("operator", string(spec.Operator))
	form.Set("rhs_type", "constant")
	form.Set("rhs_constant", strconv.FormatFloat(spec.Value, 'f', -1, 64))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kiteAPIBase+"/alerts", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", k.authHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kite create alert %q: %s: %s", spec.Name, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
```

- [ ] **Step 4: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0. `*Kite` now satisfies `provider.Provider`.

- [ ] **Step 5: Commit**

```bash
git add internal/providers/kite/
git commit -m "feat: kite alerts list/create over HTTP; drop temporary stubs"
```

---

### Task 6: Recommendation CSV parser

**Files:**
- Create: `internal/alerts/recommendation/csv.go`

- [ ] **Step 1: Write `internal/alerts/recommendation/csv.go`**

```go
package recommendation

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Recommendation is one parsed row used to build alerts.
type Recommendation struct {
	Symbol  string // exchange suffix stripped (e.g. "TARIL")
	Target1 float64
	Target2 float64
	Line    int // 1-based source line, for reporting
}

// Load parses the recommendation CSV. It returns valid recommendations and a
// list of human-readable skip reasons for malformed rows. A malformed row never
// aborts the parse; only an unreadable file returns an error.
//
// Expected columns: Stock Code/Name, Recommendation Date, Target 1, Target 2, Buy Price.
func Load(path string) ([]Recommendation, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row

	var recs []Recommendation
	var skipped []string
	line := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		line++
		if line == 1 {
			continue // header
		}
		if len(row) < 4 {
			skipped = append(skipped, fmt.Sprintf("line %d: too few columns", line))
			continue
		}
		sym := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(row[0])), ".NS")
		t1, err1 := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		t2, err2 := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
		if sym == "" || err1 != nil || err2 != nil {
			skipped = append(skipped, fmt.Sprintf("line %d: bad symbol or targets", line))
			continue
		}
		recs = append(recs, Recommendation{Symbol: sym, Target1: t1, Target2: t2, Line: line})
	}
	return recs, skipped, nil
}
```

- [ ] **Step 2: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/alerts/recommendation/
git commit -m "feat: recommendation CSV parser"
```

---

### Task 7: Mapping CSV reader/appender

**Files:**
- Create: `internal/alerts/mapping/csv.go`

- [ ] **Step 1: Write `internal/alerts/mapping/csv.go`**

```go
package mapping

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
)

// Entry is one row in the dedup/log CSV.
type Entry struct {
	Provider  string
	AlertName string
	Symbol    string
	Exchange  string
	Tier      string
	Value     float64
	CreatedAt string
}

var header = []string{"provider", "alert_name", "symbol", "exchange", "tier", "value", "created_at"}

// Load reads all entries. A missing file is not an error (returns nil, nil).
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var out []Entry
	for i, row := range rows {
		if i == 0 || len(row) < 7 {
			continue // header or malformed
		}
		v, _ := strconv.ParseFloat(row[5], 64)
		out = append(out, Entry{
			Provider:  row[0],
			AlertName: row[1],
			Symbol:    row[2],
			Exchange:  row[3],
			Tier:      row[4],
			Value:     v,
			CreatedAt: row[6],
		})
	}
	return out, nil
}

// Append adds one entry, creating the file (with header) if needed.
func Append(path string, e Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, statErr := os.Stat(path)
	newFile := os.IsNotExist(statErr)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if newFile {
		if err := w.Write(header); err != nil {
			return err
		}
	}
	return w.Write([]string{
		e.Provider, e.AlertName, e.Symbol, e.Exchange, e.Tier,
		strconv.FormatFloat(e.Value, 'f', -1, 64), e.CreatedAt,
	})
}
```

- [ ] **Step 2: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/alerts/mapping/
git commit -m "feat: mapping CSV reader/appender with provider column"
```

---

### Task 8: Alert builder (row → 3 specs)

**Files:**
- Create: `internal/alerts/builder.go`

- [ ] **Step 1: Write `internal/alerts/builder.go`**

```go
package alerts

import (
	"strconv"

	"be-stonks/internal/alerts/recommendation"
	"be-stonks/internal/provider"
)

// SpecWithTier pairs an alert spec with its human tier label (for the log).
type SpecWithTier struct {
	Spec provider.AlertSpec
	Tier string
}

// Build produces the 3 alert specs for one recommendation:
// Target 1, the exact Midpoint of Target 1 and 2, and Target 2.
// Names are human-readable, underscore-free, and made unique by the value,
// e.g. "TARIL Target 1 305", "TARIL Midpoint 320.5", "TARIL Target 2 336".
func Build(rec recommendation.Recommendation) []SpecWithTier {
	mid := (rec.Target1 + rec.Target2) / 2
	tiers := []struct {
		label string
		value float64
	}{
		{"Target 1", rec.Target1},
		{"Midpoint", mid},
		{"Target 2", rec.Target2},
	}

	out := make([]SpecWithTier, 0, 3)
	for _, t := range tiers {
		name := rec.Symbol + " " + t.label + " " + trim(t.value)
		out = append(out, SpecWithTier{
			Tier: t.label,
			Spec: provider.AlertSpec{
				Name:      name,
				Symbol:    rec.Symbol,
				Exchange:  provider.ExchangeNSE,
				Attribute: provider.AttrLTP,
				Operator:  provider.OpGTE,
				Value:     t.value,
			},
		})
	}
	return out
}

// trim formats a price without trailing zeros: 320.5 not 320.50, 305 not 305.0.
func trim(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
```

- [ ] **Step 2: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Quick sanity check of naming/midpoint (temporary)**

Create a throwaway file `internal/alerts/tmp_check.go`:

```go
//go:build ignore

package main

import (
	"fmt"

	"be-stonks/internal/alerts"
	"be-stonks/internal/alerts/recommendation"
)

func main() {
	for _, s := range alerts.Build(recommendation.Recommendation{Symbol: "TARIL", Target1: 305, Target2: 336}) {
		fmt.Printf("%-22s value=%v\n", s.Spec.Name, s.Spec.Value)
	}
}
```

Run: `go run internal/alerts/tmp_check.go`
Expected output:
```
TARIL Target 1 305     value=305
TARIL Midpoint 320.5   value=320.5
TARIL Target 2 336     value=336
```
Then delete the file: `rm internal/alerts/tmp_check.go`

- [ ] **Step 4: Commit**

```bash
git add internal/alerts/builder.go
git commit -m "feat: alert builder (target1, midpoint, target2) with human names"
```

---

### Task 9: Run report

**Files:**
- Create: `internal/alerts/report.go`

- [ ] **Step 1: Write `internal/alerts/report.go`**

```go
package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"be-stonks/internal/timeutil"
)

// FailedItem records a single alert that failed to create.
type FailedItem struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// Report is the per-run summary written to runs/alerts-sync/<stamp>.json.
type Report struct {
	StartedAt             string       `json:"started_at"`
	FinishedAt            string       `json:"finished_at"`
	Provider              string       `json:"provider"`
	Status                string       `json:"status"` // ok | no_session | list_failed | partial
	SessionReady          bool         `json:"session_ready"`
	RecommendationsRead   int          `json:"recommendations_read"`
	AlertsDesired         int          `json:"alerts_desired"`
	ExistingOnProvider    int          `json:"existing_on_provider"`
	ReconciledIntoMapping int          `json:"reconciled_into_mapping"`
	Created               []string     `json:"created"`
	Skipped               int          `json:"skipped"`
	Failed                []FailedItem `json:"failed"`
	Errors                []string     `json:"errors"`
}

// writeReport stamps the finish time and writes the report. Best-effort: it
// must never panic, since it runs from a defer on every code path.
func writeReport(dir string, r *Report, started time.Time) {
	r.FinishedAt = timeutil.Label(timeutil.NowIST())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, timeutil.FileStamp(started)+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}
```

- [ ] **Step 2: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0. (`writeReport`/`Report` are unused until Task 10 — `go build` does not flag unused functions/types, only unused imports/locals, so this passes.)

- [ ] **Step 3: Commit**

```bash
git add internal/alerts/report.go
git commit -m "feat: run report struct and writer"
```

---

### Task 10: Sync orchestration + handler + wiring

**Files:**
- Create: `internal/alerts/sync.go`
- Create: `internal/alerts/handler.go`
- Create: `internal/auth/handler.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write `internal/alerts/sync.go`**

```go
package alerts

import (
	"context"
	"time"

	"be-stonks/internal/alerts/mapping"
	"be-stonks/internal/alerts/recommendation"
	"be-stonks/internal/provider"
	"be-stonks/internal/timeutil"
)

// Syncer runs the recommendation -> alerts diff against a provider.
type Syncer struct {
	provider           provider.Provider
	recommendationPath string
	mappingPath        string
	runsDir            string
	createDelay        time.Duration
}

// NewSyncer builds a Syncer.
func NewSyncer(p provider.Provider, recPath, mapPath, runsDir string, createDelay time.Duration) *Syncer {
	return &Syncer{
		provider:           p,
		recommendationPath: recPath,
		mappingPath:        mapPath,
		runsDir:            runsDir,
		createDelay:        createDelay,
	}
}

// Sync executes one diff run and returns the report plus the HTTP status the
// handler should send. A report is ALWAYS written via defer.
func (s *Syncer) Sync(ctx context.Context) (*Report, int) {
	started := timeutil.NowIST()
	rep := &Report{
		StartedAt: timeutil.Label(started),
		Provider:  s.provider.Name(),
		Created:   []string{},
		Failed:    []FailedItem{},
		Errors:    []string{},
	}
	status := 200
	defer func() { writeReport(s.runsDir, rep, started) }()

	// 1. Session prerequisite.
	rep.SessionReady = s.provider.Ready()
	if !rep.SessionReady {
		rep.Status = "no_session"
		return rep, 503
	}

	// 2. List existing alerts once (source of truth, cached in memory).
	existing, err := s.provider.ListAlerts(ctx)
	if err != nil {
		rep.Status = "list_failed"
		rep.Errors = append(rep.Errors, "list alerts: "+err.Error())
		return rep, 502
	}
	rep.ExistingOnProvider = len(existing)
	existingNames := make(map[string]bool, len(existing))
	for _, a := range existing {
		existingNames[a.Name] = true
	}

	// 3. Reconcile drift: provider alerts missing from the local log get logged.
	logged, err := mapping.Load(s.mappingPath)
	if err != nil {
		rep.Errors = append(rep.Errors, "load mapping: "+err.Error())
	}
	loggedNames := make(map[string]bool, len(logged))
	for _, e := range logged {
		loggedNames[e.AlertName] = true
	}
	for _, a := range existing {
		if loggedNames[a.Name] {
			continue
		}
		e := mapping.Entry{
			Provider:  s.provider.Name(),
			AlertName: a.Name,
			CreatedAt: timeutil.Label(timeutil.NowIST()),
		}
		if err := mapping.Append(s.mappingPath, e); err != nil {
			rep.Errors = append(rep.Errors, "reconcile append "+a.Name+": "+err.Error())
			continue
		}
		loggedNames[a.Name] = true
		rep.ReconciledIntoMapping++
	}

	// 4. Read desired recommendations.
	recs, skippedRows, err := recommendation.Load(s.recommendationPath)
	if err != nil {
		rep.Status = "list_failed"
		rep.Errors = append(rep.Errors, "read recommendations: "+err.Error())
		return rep, 500
	}
	rep.RecommendationsRead = len(recs)
	rep.Errors = append(rep.Errors, skippedRows...)

	// 5. Diff and create only what's missing.
	for _, rec := range recs {
		for _, sw := range Build(rec) {
			rep.AlertsDesired++
			if existingNames[sw.Spec.Name] {
				rep.Skipped++
				continue
			}
			if err := s.provider.CreateAlert(ctx, sw.Spec); err != nil {
				rep.Failed = append(rep.Failed, FailedItem{Name: sw.Spec.Name, Error: err.Error()})
				continue
			}
			existingNames[sw.Spec.Name] = true
			rep.Created = append(rep.Created, sw.Spec.Name)

			e := mapping.Entry{
				Provider:  s.provider.Name(),
				AlertName: sw.Spec.Name,
				Symbol:    sw.Spec.Symbol,
				Exchange:  string(sw.Spec.Exchange),
				Tier:      sw.Tier,
				Value:     sw.Spec.Value,
				CreatedAt: timeutil.Label(timeutil.NowIST()),
			}
			if err := mapping.Append(s.mappingPath, e); err != nil {
				rep.Errors = append(rep.Errors, "mapping append "+sw.Spec.Name+": "+err.Error())
			}
			if s.createDelay > 0 {
				time.Sleep(s.createDelay)
			}
		}
	}

	if len(rep.Failed) > 0 {
		rep.Status = "partial"
	} else {
		rep.Status = "ok"
	}
	return rep, status
}
```

- [ ] **Step 2: Write `internal/alerts/handler.go`**

```go
package alerts

import (
	"encoding/json"
	"net/http"
)

// Handler exposes the alerts feature over HTTP.
type Handler struct {
	syncer *Syncer
}

// NewHandler builds an alerts HTTP handler.
func NewHandler(s *Syncer) *Handler { return &Handler{syncer: s} }

// Sync handles POST /alerts/sync.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	rep, status := h.syncer.Sync(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rep)
}
```

- [ ] **Step 3: Write `internal/auth/handler.go`**

```go
package auth

import (
	"net/http"

	"be-stonks/internal/provider"
)

// Handler exposes the generic broker login flow.
type Handler struct {
	p provider.Provider
}

// NewHandler builds an auth HTTP handler for the active provider.
func NewHandler(p provider.Provider) *Handler { return &Handler{p: p} }

// Login handles GET /auth/login by redirecting to the broker login URL.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.p.LoginURL(), http.StatusFound)
}

// Callback handles GET /auth/callback?request_token=... and persists the session.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("request_token")
	if token == "" {
		http.Error(w, "missing request_token", http.StatusBadRequest)
		return
	}
	if err := h.p.ExchangeToken(token); err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("login successful — session saved"))
}
```

- [ ] **Step 4: Replace `cmd/server/main.go` with the full wiring**

```go
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
```

- [ ] **Step 5: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Manual smoke — health + no-session report**

Run (in one shell):
```bash
KITE_API_KEY=dummy KITE_API_SECRET=dummy DATA_DIR=/tmp/bestonks-data RUNS_DIR=/tmp/bestonks-runs go run ./cmd/server &
sleep 1
curl -s localhost:8000/health; echo
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8000/alerts/sync
ls /tmp/bestonks-runs/alerts-sync/
kill %1
```
Expected: `health` prints `ok`; `/alerts/sync` returns `503` (no session); a `*.json` report exists with `"status": "no_session"`.

- [ ] **Step 7: Commit**

```bash
git add internal/alerts/sync.go internal/alerts/handler.go internal/auth/handler.go cmd/server/main.go
git commit -m "feat: /alerts/sync orchestration, handlers, and server wiring"
```

---

### Task 11: Deployment docs — sample data, README, systemd, cron

**Files:**
- Create: `data/alerts/recommendation.sample.csv`
- Create: `deploy/be-stonks.service`
- Create: `deploy/crontab.example`
- Create: `README.md`

- [ ] **Step 1: Write `data/alerts/recommendation.sample.csv`**

```csv
Stock Code/Name,Recommendation Date,Target 1,Target 2,Buy Price Recommendation
TARIL.NS,2026-04-02,305,336,275
WELCORP.NS,2026-04-07,950,951,885
```

- [ ] **Step 2: Write `deploy/be-stonks.service`**

```ini
[Unit]
Description=be-stonks trading backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/be-stonks
ExecStart=/opt/be-stonks/bin/server
Restart=on-failure
RestartSec=5
Environment=PORT=8000
Environment=ALERT_PROVIDER=kite
Environment=DATA_DIR=/opt/be-stonks/data
Environment=RUNS_DIR=/opt/be-stonks/runs
# Secrets: prefer an EnvironmentFile not committed to git.
EnvironmentFile=/opt/be-stonks/.env

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 3: Write `deploy/crontab.example`**

```cron
# be-stonks alerts sync — 08:30 and 16:00 IST daily.
# Ensure the machine's timezone is IST, or adjust these hours.
30 8 * * * curl -fsS -X POST http://localhost:8000/alerts/sync >> /opt/be-stonks/runs/cron.log 2>&1
0 16 * * * curl -fsS -X POST http://localhost:8000/alerts/sync >> /opt/be-stonks/runs/cron.log 2>&1
```

- [ ] **Step 4: Write `README.md`**

````markdown
# be-stonks

Backend for stock-trading activities. Provider-agnostic (Kite today; Dhan/Fyers later). See `context.md` for architecture conventions and `docs/superpowers/specs/` for feature specs.

## Build & run

```bash
go build -o bin/server ./cmd/server

export KITE_API_KEY=xxx
export KITE_API_SECRET=yyy
export ALERT_PROVIDER=kite   # default
./bin/server
```

## Endpoints

- `GET /health` — liveness.
- `GET /auth/login` — redirects to the broker login. Open in a browser.
- `GET /auth/callback?request_token=...` — broker redirects here; saves the session to `data/kite/session.json`.
- `POST /alerts/sync` — reads `data/alerts/recommendation.csv`, creates the 3 LTP alerts per stock (Target 1, Midpoint, Target 2) on NSE, dedup against the broker. Writes a report to `runs/alerts-sync/<timestamp>.json`.

## Daily flow

1. Each morning, open `GET /auth/login`, complete Kite login. Kite tokens expire daily.
2. Update `data/alerts/recommendation.csv`.
3. Cron POSTs `/alerts/sync` at 08:30 and 16:00 IST (see `deploy/crontab.example`). Re-run any time — it is idempotent.

## Config (env)

| Var | Default | Purpose |
|-----|---------|---------|
| `PORT` | `8000` | HTTP port |
| `ALERT_PROVIDER` | `kite` | Active alerts broker |
| `KITE_API_KEY` / `KITE_API_SECRET` | — | Kite credentials (required for kite) |
| `DATA_DIR` | `data` | Base data directory |
| `RUNS_DIR` | `runs` | Run reports directory |
| `CREATE_DELAY_MS` | `250` | Throttle between alert creations |

## Deploy (EC2)

Build the binary, copy to `/opt/be-stonks/`, put secrets in `/opt/be-stonks/.env`, install `deploy/be-stonks.service` to `/etc/systemd/system/`, `systemctl enable --now be-stonks`, then install `deploy/crontab.example`.
````

- [ ] **Step 5: Build & vet (sanity, nothing new to compile)**

Run: `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add data/alerts/recommendation.sample.csv deploy/ README.md
git commit -m "docs: README, systemd unit, cron example, sample recommendation CSV"
```

---

## Final verification (manual, requires real Kite credentials)

These confirm the live path; run on a machine with valid Kite API credentials.

- [ ] Start the server with real `KITE_API_KEY` / `KITE_API_SECRET`.
- [ ] Open `GET /auth/login` in a browser, complete login; confirm `data/kite/session.json` is written.
- [ ] Put a couple of rows in `data/alerts/recommendation.csv`.
- [ ] `curl -X POST localhost:8000/alerts/sync` → expect `200`, report `status: ok`, alerts visible in the Kite web console.
- [ ] Re-run `/alerts/sync` → expect all `skipped`, `created` empty (idempotent).
- [ ] Delete an alert in Kite, remove its line from `mapping.csv`, re-run → it is recreated.

---

## Self-Review (completed by plan author)

- **Spec coverage:** 3 alerts/Target1/Midpoint/Target2 (Task 8); LTP+NSE+simple/no-ATO (Task 5 + types Task 2); consistent human names (Task 8); mapping CSV dedup + provider column (Task 7); check-before-create via cached list (Task 10); daily diff endpoint (Task 10); cron 08:30/16:00 (Task 11); /sync-only backend, room for more APIs (router Task 10); provider-agnostic (Tasks 2,4,5); per-run reports incl. failure paths (Tasks 9,10); manual daily token (Task 4); session under `data/kite/` (Task 4); run reports under `runs/alerts-sync/` dashed timestamps (Tasks 9,3). All covered.
- **Placeholder scan:** one deliberate external-verification step (Task 5 Step 2, Kite `lhs_attribute` string) — required because it must match a live API; default value provided. No other placeholders.
- **Type consistency:** `provider.Config`, `provider.Provider`, `AlertSpec`, `Alert`, `SpecWithTier`, `mapping.Entry`, `Report`, `Syncer.Sync` signatures are consistent across tasks 2→10.
