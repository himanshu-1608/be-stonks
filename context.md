# Project Context & Conventions — be-stonks

Backend for stock-trading activities (Zerodha Kite + future brokers). This file holds
the cross-cutting rules every feature must follow. Read it before building any new
feature so these decisions never need re-explaining.

---

## 1. Tech stack

- **Language:** Go 1.22+.
- **HTTP:** `net/http` + `chi` router.
- **Broker SDKs:** official per-provider (Kite: `gokiteconnect/v4`). Lives only inside that provider's package.
- **Deploy:** single static binary on AWS EC2, run as a `systemd` service.
- **Scheduling:** OS `cron` calls local HTTP endpoints via `curl` (e.g. `curl -X POST localhost:8000/alerts/sync`). Cron does not run business logic directly — the long-lived service owns it, because more APIs (price ticks, etc.) will live in the same service.

## 2. Provider-agnostic architecture (NON-NEGOTIABLE)

Brokers are pluggable. Any broker (Kite today; Dhan, Fyers, etc. tomorrow) must be swappable by config + a new package, with zero churn in feature logic.

- **Neutral contracts** live in `internal/provider/`:
  - `provider.go` — interfaces (`AlertProvider`, `Authenticator`, future `TickProvider`).
  - `types.go` — neutral domain types (`AlertSpec`, `Alert`, `Exchange`, `Operator`, …).
  - `registry.go` — maps a provider name → constructor, chosen from config.
- **Concrete implementations** live in `internal/providers/<name>/` (e.g. `internal/providers/kite/`). They translate neutral domain types ↔ the broker's own API.
- **Feature code never imports a concrete provider package.** It depends only on `internal/provider` interfaces. The provider is injected at startup.
- **Selection is env-driven**, one var per capability: `ALERT_PROVIDER=kite` (future: `TICK_PROVIDER=…`).

## 3. Feature-grouped folder structure

Group by feature/domain, not by technical layer. Each feature is self-contained and readable top-to-bottom.

```
be-stonks/
  cmd/server/main.go             # wiring, router, startup
  internal/
    config/                      # env loading, provider selection, paths
    provider/                    # neutral contracts + domain types + registry
    providers/<broker>/          # concrete broker impls (kite, dhan, fyers, …)
    auth/                        # generic auth endpoints; delegate to active Authenticator
    <feature>/                   # e.g. alerts/ — handler + logic + sub-parsers
  data/
    <broker>/session.json        # per-provider session/token storage
    <feature>/                   # feature input/state files
  runs/
    <feature>-<job>/             # per-run report files (see §6)
  docs/superpowers/specs/        # design specs, one per feature
```

A new feature = add `internal/<feature>/`, `data/<feature>/`, `runs/<feature>-<job>/`. Shared infra (`config`, `provider`, `providers/*`) is reused, not duplicated.

## 4. Sessions / auth

- Broker access tokens are per-provider and may expire (Kite: daily, requires manual browser login).
- Manual login flow exposed generically: `GET /auth/login` (returns broker login URL), `GET /auth/callback?request_token=…` (exchanges + persists token).
- Token persisted at `data/<provider>/session.json` (gitignored).
- `Authenticator` interface abstracts this: `LoginURL()`, `ExchangeToken(requestToken)`, `Ready()`.
- If no valid session, jobs fail **loud** (HTTP 503 + run report), never silent.

## 5. Idempotency & dedup (source-of-truth rule)

- **The provider's own state is the source of truth.** Local CSV/files are a cache/log, not authority.
- Minimize provider API calls (they are rate-limited): fetch the provider's list **once per run**, cache in memory, dedup against that cached set for the whole run.
- **Reconcile drift:** anything present on the provider but missing from the local log gets appended to the log (self-heals timeouts where a write succeeded but logging didn't).
- Jobs are **idempotent** — safe to run multiple times a day and to re-run manually.
- Never blind-write: if fetching the provider's current state fails, abort the run; do not create/modify.

## 6. Run reports (observability)

- Every job invocation writes one report file: `runs/<feature>-<job>/YYYY-MM-DD-HH-MM-SS.json` (dashes throughout, IST timestamps inside).
- **Always written**, including early exits (no-session, list-failed, panic) — written via `defer` so the trace survives any failure path.
- Captures: start/finish, provider, status (`ok|no_session|list_failed|partial`), counts, what was created/skipped, and a `failed[]` with per-item errors.
- `runs/` is gitignored (runtime artifacts).

## 7. Error handling

- Fail loud on prerequisites (no session, list fetch failed): clear HTTP status + report.
- Per-item failures (one create fails) are logged and the run continues; report lists them; they retry naturally next run (still missing on provider → recreated).
- Bad input rows are skipped (not fatal) and recorded in the report.

## 8. Rate limits

- One list/read call per run, cached.
- Throttle writes with a fixed delay between calls; retry once on HTTP 429.
- Delay/retry counts configurable via env.

## 9. Market / data conventions

- Exchange: **NSE** (not BSE) unless stated otherwise.
- Price trigger attribute: **LTP** (last traded price).
- Input symbols carry a `.NS` suffix (e.g. `TARIL.NS`); strip it for the broker tradingsymbol (`TARIL`).
- Human-readable, predictable, unique names for broker-side artifacts (alerts etc.): no underscores, no jargon. Disambiguate recurring symbols by value, not date (e.g. `TARIL Target 1 305`).
- Decimal values trimmed of trailing zeros (`320.5`, not `320.50`).

## 10. Testing

- No automated tests required for this project (owner's decision). Keep code modular and interface-driven so it stays trivially testable if that changes.

## 11. Git hygiene

- Gitignored: `data/<broker>/session.json` and all sessions, `runs/`, generated state files, build artifacts.
- Input files (`data/<feature>/*.csv` you maintain by hand) and generated logs are runtime data — keep secrets and tokens out of git always.

---

## Endpoints conventions

- `GET /health` — liveness.
- `GET /auth/login`, `GET /auth/callback` — generic broker auth.
- `POST /<feature>/sync` (or other verbs) — feature jobs, cron-friendly, idempotent.
