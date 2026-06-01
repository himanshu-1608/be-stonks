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
	Status                string       `json:"status"` // ok | no_session | list_failed | input_error | partial
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
