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
