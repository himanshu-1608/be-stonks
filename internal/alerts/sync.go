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
		if e.Provider == s.provider.Name() {
			loggedNames[e.AlertName] = true
		}
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
		rep.Status = "input_error"
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
