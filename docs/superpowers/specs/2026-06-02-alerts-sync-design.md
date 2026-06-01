# Alerts Sync — Design Spec

**Date:** 2026-06-02
**Feature:** Daily diff that turns a stock recommendation CSV into broker price alerts.
**Cross-cutting rules:** see root `context.md` (provider-agnostic, folder structure, run reports, etc.). This spec only covers what is specific to the alerts feature.

---

## 1. Goal

Given a recommendation CSV (one row per stock recommendation), create **3 LTP price alerts** per recommendation on the active broker (Kite today):

1. **Target 1** — LTP `>=` Target 1.
2. **Mid-hit** — LTP `>=` exact midpoint `(Target1 + Target2) / 2`.
3. **Target 2** — LTP `>=` Target 2.

Run daily (cron at 08:30 and 16:00 IST) and on-demand. Idempotent: never create duplicates. Alerts only — **no ATO**, **NSE only**, **LTP only**.

## 2. Input — recommendation CSV

Path: `data/alerts/recommendation.csv`. Maintained by the owner; updated daily.

Columns:
```
Stock Code/Name, Recommendation Date, Target 1, Target 2, Buy Price Recommendation
TARIL.NS, 2026-04-02, 305, 336, 275
```

- Symbol carries `.NS`; stripped to broker tradingsymbol (`TARIL`).
- The **same symbol can recur** with different targets (e.g. `TARIL.NS` at T1 305 and later at T1 380). Both sets of alerts must be creatable — uniqueness comes from the value in the alert name, not the symbol.
- Only `Stock Code/Name`, `Target 1`, `Target 2` are used. Buy price and date are ignored for alert creation.

## 3. Alert spec (neutral)

Builder maps one recommendation row → 3 `provider.AlertSpec`:

| Tier      | Value                       | Operator | Attribute | Exchange |
|-----------|-----------------------------|----------|-----------|----------|
| Target 1  | `Target1`                   | `>=`     | LTP       | NSE      |
| Mid-hit   | `(Target1 + Target2) / 2`   | `>=`     | LTP       | NSE      |
| Target 2  | `Target2`                   | `>=`     | LTP       | NSE      |

**Names** (human-readable, no underscores, unique by value):
- `TARIL Target 1 305`
- `TARIL Midpoint 320.5`
- `TARIL Target 2 336`

Values trimmed of trailing zeros (`320.5`, not `320.50`).

The Kite provider translates each `AlertSpec` → Kite alert params (`lhs_exchange=NSE`, `lhs_tradingsymbol=<symbol>`, `lhs_attribute=LastTradedPrice`, `operator=>=`, `rhs_type=constant`, `rhs_constant=<value>`, `type=simple`).

## 4. Dedup / mapping log

Path: `data/alerts/mapping.csv`. Local cache/log, **not** the source of truth.

Columns: `provider, alert_name, symbol, exchange, tier, value, created_at`.

The `provider` column keeps dedup correct if the active broker changes.

## 5. /sync algorithm

`POST /alerts/sync`. Steps:

1. **Session check.** If active provider has no valid session → write report `status=no_session`, return `503`.
2. **List once.** Call provider `ListAlerts()` a single time; cache the set of existing alert names in memory. On failure → write report `status=list_failed`, return `502` (never blind-create).
3. **Reconcile drift.** Any alert on the provider but absent from `mapping.csv` → append to `mapping.csv` (recovers past write-succeeded-but-log-failed cases).
4. **Build desired.** Parse `recommendation.csv` → for each valid row, 3 `AlertSpec`s. Skip malformed rows (record in report).
5. **Diff & create.** For each desired spec: if its name is in the cached provider set → skip. Else `CreateAlert()`, then append to `mapping.csv` and add the name to the in-memory set. Throttle creates (fixed delay; retry once on 429).
6. **Report.** Always write `runs/alerts-sync/YYYY-MM-DD-HH-MM-SS.json` via `defer`.

No second list call even when a name is missing from the CSV — the full provider list is already cached in memory.

## 6. Error handling

- No session → `503`, report.
- `ListAlerts` fail → `502`, report, abort (no creates).
- One `CreateAlert` fail → log, continue; appears in `failed[]`; retried next run.
- Mapping append fail after a successful create → log; next run's reconcile (step 3) recovers it.
- Bad CSV row → skip, record, continue.
- Idempotent; safe at 08:30, 16:00, and ad-hoc (e.g. an 11:00 re-run after a missed-login morning).

## 7. Run report shape

`runs/alerts-sync/2026-06-02-11-02-15.json`:
```json
{
  "started_at": "2026-06-02-11-02-15 IST",
  "finished_at": "2026-06-02-11-02-19 IST",
  "provider": "kite",
  "status": "ok | no_session | list_failed | partial",
  "session_ready": true,
  "recommendations_read": 23,
  "alerts_desired": 69,
  "existing_on_provider": 60,
  "reconciled_into_mapping": 3,
  "created": ["TARIL Target 1 305"],
  "skipped": 64,
  "failed": [{"name": "WABAG Target 2 1750", "error": "..."}],
  "errors": []
}
```

## 8. Auth (Kite, manual daily)

- `GET /auth/login` → Kite login URL. Owner logs in each morning.
- `GET /auth/callback?request_token=…` → exchange → persist `data/kite/session.json`.
- Kite access tokens expire daily; this manual step is expected and honest to Kite's design.

## 9. Packages (this feature)

```
internal/alerts/
  handler.go              # POST /alerts/sync
  sync.go                 # algorithm above; holds a provider.AlertProvider
  builder.go              # row -> []AlertSpec, name + midpoint logic
  recommendation/csv.go   # parse recommendation.csv
  mapping/csv.go          # read/append mapping.csv
```

Shared: `internal/provider` (contracts), `internal/providers/kite` (impl), `internal/config`, `internal/auth`.

## 10. Out of scope (YAGNI)

- Deleting/updating stale alerts when a recommendation leaves the CSV (create-only diff).
- ATO orders.
- BSE.
- Automated token refresh (manual login by design).
- Automated tests (owner's decision; see `context.md`).
