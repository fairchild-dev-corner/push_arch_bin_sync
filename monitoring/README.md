# Monitoring: Prometheus + Grafana

Native binaries, no Docker required. All per-deployment settings (ports,
absolute paths, which server Prometheus scrapes) live in one file you edit
per server — nothing else in this folder needs touching when you deploy
elsewhere.

## Setup (once per server)

1. Download and extract:
   - Prometheus: https://prometheus.io/download/
   - Grafana: https://grafana.com/grafana/download
   - Loki: https://github.com/grafana/loki/releases (grab
     `loki-<os>-<arch>.zip`; recommended: extract into `bin/loki/`, same
     as Prometheus)
   (Windows/Linux/macOS all have official builds. Extract anywhere.)

2. Copy the config template and fill it in for this server:
   ```
   cp monitoring/.env.example monitoring/.env
   ```
   Edit `monitoring/.env`:
   - `PROMETHEUS_BIN` / `GRAFANA_BIN` — full path to each extracted binary
     (or just the binary name if it's on `PATH`).
   - `METRICS_TARGET` — `host:port` where the daemon's `/metrics` is
     reachable from Prometheus. Must match `METRICS_PORT` in the project's
     own `.env` (default `localhost:9308` when Prometheus runs alongside
     the daemon on the same host).
   - `GRAFANA_DASHBOARDS_PATH` / `GRAFANA_PROVISIONING_PATH` — absolute
     paths to `monitoring/grafana/dashboards` and
     `monitoring/grafana/provisioning` **on this server**.
   - `PROMETHEUS_URL` — where Grafana reaches Prometheus (usually
     `http://localhost:9090`).
   - `GRAFANA_PORT` — change if `3000` is already taken (e.g. by a
     frontend dev server).
   - `LOKI_BIN` — full path to the extracted Loki binary.
   - `LOKI_PORT` / `LOKI_GRPC_PORT` — ports Loki listens on (defaults
     `3100` / `9096`).
   - `LOKI_URL` — where Grafana reaches Loki (usually
     `http://localhost:3100`). Also set `LOKI_URL` in the **project root**
     `.env` (not this one) to the same value, so the daemon knows where to
     push its log lines to.

   `monitoring/.env` is gitignored — it's per-server, not committed.

## Running

Start the daemon first (`make run` in the project root), then:

- **Windows**:
  ```powershell
  .\monitoring\start-prometheus.ps1
  .\monitoring\start-loki.ps1
  .\monitoring\start-grafana.ps1
  ```
- **Linux/macOS**:
  ```bash
  ./monitoring/start-prometheus.sh
  ./monitoring/start-loki.sh
  ./monitoring/start-grafana.sh
  ```

Each script reads `monitoring/.env` and handles the rest:
`start-prometheus` generates `monitoring/prometheus/prometheus.yml` from
`prometheus.yml.template` (substituting `METRICS_TARGET`) and launches
Prometheus with it. `start-grafana` sets `GF_PATHS_PROVISIONING`,
`GF_SERVER_HTTP_PORT`, and the two `$__env{...}` values the provisioning
YAML files reference (`GRAFANA_DASHBOARDS_PATH`, `PROMETHEUS_URL`), then
launches Grafana — no manual datasource or dashboard import needed.

Open `http://localhost:<GRAFANA_PORT>` (default login `admin`/`admin`).
The **push_arch_bin_sync** dashboard is already there under Dashboards.

## How the per-server config actually gets applied

- **Prometheus** has no built-in env-var expansion, so
  `prometheus.yml.template` has a `__METRICS_TARGET__` placeholder that the
  start script substitutes into a real (gitignored) `prometheus.yml` before
  each run.
- **Grafana** (>= 9.1) supports `$__env{VAR}` directly inside provisioning
  YAML, so `dashboard.yml`'s `path:` and `datasource.yml`'s `url:` are
  literally `$__env{GRAFANA_DASHBOARDS_PATH}` / `$__env{PROMETHEUS_URL}` /
  `$__env{LOKI_URL}` — Grafana expands them from the environment at
  startup, so those files never need per-server edits.
- **Loki** supports `--config.expand-env=true`, so `loki-config.yml` uses
  `${LOKI_HTTP_PORT}` / `${LOKI_GRPC_PORT}` / `${LOKI_DATA_DIR}`
  placeholders that `start-loki.ps1`/`.sh` expand from `monitoring/.env` —
  same idea as the other two.

## How log lines reach the "Live Sync/Error Log" panel

There's no log-shipping agent (no Promtail/Alloy) — Promtail is
feature-frozen upstream (Grafana's last Windows build of it shipped with
Loki v2.9, three major versions behind the Loki server this project runs),
so adding it would mean maintaining a deprecated component for no benefit
here. Instead, the daemon pushes each `LogError`/`LogSync` line directly to
Loki's HTTP push API (`internal/utils/loki_logger.go`), tagged with
`job="push_arch_bin_sync"` and `level="error"|"sync"`. This is best-effort
and non-blocking — if `LOKI_URL` (project root `.env`) is unset or Loki
isn't running, log lines simply don't reach Grafana; the daemon's own
`err_logs/`/`sync_logs/` files and console output are unaffected either way.

## Dashboard panels

- **Daemon Up** — Prometheus's own scrape-health signal; goes red if the
  process is down or unreachable, with no separate heartbeat metric needed.
- **Time Since Last Successful Sync** / **Time Since Last Check** — staleness
  indicators; a process that's alive but stuck (e.g. destination
  unreachable) shows here even though "Daemon Up" stays green.
- **Total Read Errors**, **Successful/Failed Writes (24h)** — at-a-glance
  counters.
- **Destination Writes: Success vs Failure**, **Events by Operation**,
  **Events by Table** — rate graphs over time.
- **Live Sync/Error Log** — the actual `LogSync`/`LogError` lines (e.g.
  `INSERT: acctstat (AcctStatID=1)`, `✅ Replicated 6 changes to
  fccmpc_coop_main_sys_db@...`), live-tailing and searchable, from Loki.
