# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A long-running Go daemon that tails a source MySQL server's binary log (via
`github.com/go-mysql-org/go-mysql`) and mirrors row-level changes on 15
legacy tables into a destination MySQL database (a DigitalOcean Managed
MySQL instance), over TLS with a pinned CA cert. It is a forward-only live
sync (change data capture), not a backfill/migration tool — see
"Binlog semantics" below for why that distinction matters.

## Commands

```
make build   # go build -o bin/push_arch_bin_sync cmd/main.go
make run     # build, then run bin/push_arch_bin_sync
go vet ./... # static checks; run after any change
```

There is no test suite yet. Verify changes with `go build ./...` /
`go vet ./...`, and where relevant a manual run against real source/
destination credentials (see "Local smoke testing" below).

## Configuration

All config loads from `.env` (via `config/env.go`'s `Envs` singleton) and is
validated/assembled into `*models.MonitorConfig` by `config.GetMonitorConfig()`.
Required variables:

- Source: `DB_HOST`, `DB_PORT`, `BINLOG_USER`, `BINLOG_PASSWORD`, `DB_NAME`
- Destination: `DEST_DB_HOST`, `DEST_DB_PORT`, `DEST_DB_USER`,
  `DEST_DB_PASSWORD`, `DEST_DB_NAME`
- `CERT_PATH`: file path (can be relative to the project root, e.g.
  `cert/ca-certificate.crt`) to the destination's pinned CA cert. Required —
  the destination is a managed MySQL instance that enforces TLS.

`GetMonitorConfig()` fails startup if any of these are missing.

Optional: `DEST_DB_TLS_SERVER_NAME` — the hostname the destination's TLS cert
is verified against. Set it to the real DigitalOcean cluster hostname when
`DEST_DB_HOST` points at a TCP proxy (e.g. an NGINX `stream` block over
WireGuard) instead of DO directly; otherwise the driver verifies the cert
against `DEST_DB_HOST` and fails with an x509 name mismatch. Leave unset for
a direct connection. The proxy must pass TLS through untouched (MySQL's TLS
is negotiated in-protocol, so NGINX can't terminate it with `listen ... ssl`).

Optional: `METRICS_PORT` (defaults to `9308` if unset) — see "Metrics" below.
Also optional: `PROMETHEUS_HEALTH_URL`, `GRAFANA_HEALTH_URL`, `LOKI_URL`
(defaults `http://localhost:9090/-/ready`, `http://localhost:3000/api/health`,
`http://localhost:3100`) — all three are informational/best-effort only and
never block startup or fail the daemon if unreachable.

## Architecture

The call chain is `cmd/main.go` → `cmd/monitor` (package `core`) →
`internal/services/monitor` → `internal/services/binlog` +
`internal/services/replicator`:

1. **`internal/services/binlog`** (`BinaryLogMonitor`) — reads the source's
   binary log via the MySQL replication protocol and decodes row events into
   `change.TableChange` values. Filters to the 15 tables in
   `internal/models/table` (`isTableMonitored`).
2. **`internal/services/monitor`** (`Monitor`) — the orchestration loop.
   Polls `BinaryLogMonitor.MonitorBinaryLogs` every ~1s (5s soft batch
   window, 10s hard ceiling per poll), tallies stats, and hands each batch
   to the `Replicator`.
3. **`internal/services/replicator`** (`MySQLReplicator`) — applies one
   poll's batch of changes to the destination inside a single transaction
   (all-or-nothing rollback on first failure). INSERT/UPDATE become a
   generic `INSERT ... ON DUPLICATE KEY UPDATE`; DELETE becomes a
   multi-column `DELETE ... WHERE`.

`Monitor` and `MySQLReplicator`/`BinaryLogMonitor` are decoupled via
`internal/interfaces` (`BinlogMonitor`, `Replicator`), so the monitor loop
doesn't know about MySQL specifics directly.

### Primary keys are the load-bearing detail

`internal/models/table/table_config.go`'s `TableConfig.PrimaryKey` holds the
**verified** (via `information_schema.KEY_COLUMN_USAGE`, not guessed)
primary-key columns for each of the 15 tables. Most are **composite** — up
to 8 columns for `sldtl`. This matters because:

- `TableChange.RecordID` (`internal/models/change`) is a
  `map[string]interface{}` (PK column → value), not a scalar, specifically
  to represent composite keys.
- `binlog_service.go`'s `buildRecordID` resolves these from the decoded row
  via `table.GetPrimaryKey(tableName)` — there is no generic `id`/`recid`
  fallback; a table with no PK mapping is skipped with a warning, not
  guessed.
- `mysql_replicator.go`'s DELETE path depends on `RecordID` containing
  exactly the table's real PK columns to build a correct `WHERE` clause.

If a new table is ever added to `AllTables`, its real primary key **must**
be looked up from `information_schema` first — do not assume `id`/`recid`
or a single-column key.

### Binlog semantics (read before changing sync behavior)

- `BinaryLogMonitor.NewBinaryLogMonitor` queries `SHOW MASTER STATUS` on
  startup and begins syncing from the **current** binlog position, not the
  oldest retained file. This is intentional, not a shortcut: MySQL's
  `binlog_row_metadata` setting controls whether column *names* are sent in
  replication events, and that setting only affects events written *after*
  it's changed — it can never be applied retroactively to already-written
  binlog files. A daemon that tried to replay old history would permanently
  fail on any event predating a metadata fix. If historical backfill into
  the destination is ever needed, it must be a separate one-time job
  (dump/restore or bulk copy), not binlog replay.
- The source MySQL server must have `binlog_row_metadata=FULL` (not the
  default `MINIMAL`) for column names to be resolvable at all — otherwise
  every row decodes to an empty values map and every change fails to apply.
  This is a server-wide setting (`SET GLOBAL binlog_row_metadata='FULL'`)
  outside this codebase's control; it does not persist across a MySQL
  restart unless also set in the server's config file.
- The in-memory binlog position is not persisted across process restarts.
  A restart re-queries the current tip and resumes from there — any gap
  between a crash and restart is silently skipped (documented tradeoff, not
  a bug to "fix" without deliberately designing durable checkpointing).
- Within a single run, a dropped connection (network blip, MySQL restart)
  *is* handled: `MonitorBinaryLogs` treats any `GetEvent` error other than
  its own batch-window timeout as a broken stream, and `resetConnection`
  tears down the syncer/streamer so the next call's `ensureStreamer`
  reconnects from `blm.lastPos` — it doesn't silently keep retrying a dead
  connection. Binlog file *rotation* (a new file, same server) needs none
  of this — it's transparent, handled by the `ROTATE_EVENT` case in
  `parseEvent` updating `blm.lastPos.Name` while the same stream keeps
  flowing.

### Table/column identifiers in generated SQL

`mysql_replicator.go` builds SQL with table/column names interpolated
directly (backtick-quoted) rather than parameterized, because SQL doesn't
allow parameterizing identifiers. This is safe here specifically because
those names come from `go-mysql`'s own binlog metadata and are checked
against the fixed `table.GetTableNames()` allowlist before a `TableChange`
is ever constructed — not from external input. Don't reuse this pattern for
identifiers that could originate outside this allowlist.

## Error and sync logging

`internal/utils/error_logger.go` provides `LogError(format, args...)`, which
writes to both stdout and a per-run file `err_logs/error_<timestamp>.logs`.
`internal/utils/sync_logger.go` mirrors this for successful replications via
`LogSync(format, args...)`, writing to `sync_logs/sync_logs_<timestamp>.logs`.
Both `utils.InitErrorLogger()` and `utils.InitSyncLogger()` must be called
once at process start (done in `cmd/main.go`) before their respective log
functions are used. Only actual errors/warnings (prefixed `❌`/`⚠️` by
convention) go through `LogError`, and only successful destination writes go
through `LogSync`; routine progress logging (`📝`/`📋` etc.) uses the
standard `log` package directly and is console-only.

Both `LogError` and `LogSync` also push their rendered line to Loki
(`internal/utils/loki_logger.go`'s `pushToLoki`, called internally — not a
separate call site to maintain) if `utils.InitLokiLogger(url)` was given a
non-empty URL (`LOKI_URL` from `.env`, wired in `cmd/main.go` right after
config load). This feeds the "Live Sync/Error Log" panel in Grafana. The
push is fire-and-forget in its own goroutine with a 2s timeout — a slow or
absent Loki never blocks a log call, and repeated failures log one `⚠️`
warning (not one per line) via `sync.Once`. There's no Promtail/log-tailing
agent in this pipeline; the daemon pushes directly to Loki's HTTP push API,
deliberately, since Promtail is feature-frozen upstream — see
`monitoring/README.md`'s "How log lines reach the Live Sync/Error Log
panel" for the full reasoning.

Per-row change events (one per detected INSERT/UPDATE/DELETE, feeding
Grafana's "Change Feed" table panel) go through a separate function,
`utils.PushEvent(action, table, info)` (`internal/utils/loki_logger.go`),
called from `monitor_service.go`'s `Run()` loop. Unlike `LogError`/`LogSync`,
this one writes to **two independent destinations from one call site**: a
local `transaction_logs/transactions_<timestamp>.logs` file (durable,
always-on, via `internal/utils/transaction_logger.go`'s `logTransaction`)
and, best-effort, Loki's "event" stream. The file copy exists specifically
so a per-transaction record survives even if Loki isn't running, was never
configured, or its data directory is ever cleared — Loki here is
observability tooling for the live dashboard, not the source of truth for
"what changes did this daemon detect." `transaction_logs/` is file-only (no
stdout) since the same line is already printed to the console via the
standard `log` package in the `Run()` loop; duplicating it through this
logger too would double the console output.

## Metrics

`internal/metrics/metrics.go` exposes a Prometheus `/metrics` endpoint via
`metrics.Serve(addr)`, started at the top of `MonitorServer.Run()`
(`cmd/monitor/monitor.go`) on `MonitorConfig.MetricsAddr` (from
`METRICS_PORT`, default `9308`). Metrics are updated inline in
`monitor_service.go` alongside the existing `Stats` struct — the two aren't
merged into one; `Stats` drives the console `PrintStatistics()` ticker,
while the Prometheus metrics are for external scraping:

- `binlog_sync_events_total{operation,table}` — counter, incremented per
  detected row change.
- `binlog_sync_destination_writes_total{result="success"|"failure"}` —
  counter, incremented once per `ApplyBatch` call in `applyChanges`.
- `binlog_sync_read_errors_total` — counter, incremented on binlog read
  errors in the `Run()` loop.
- `binlog_sync_last_success_timestamp_seconds` /
  `binlog_sync_last_check_timestamp_seconds` — gauges, for staleness-based
  alerting (a stuck-but-alive process, or a scrape failure entirely, both
  show up naturally without a separate heartbeat metric).

Point a Prometheus (or Grafana Agent/Alloy) scrape config at this endpoint;
visualize/alert in Grafana (or Perses — either just reads from Prometheus,
so this instrumentation doesn't depend on which one you pick).

A ready-to-use native (no Docker) Prometheus + Grafana setup, pre-provisioned
with a dashboard covering all of these metrics, lives in `monitoring/` — see
`monitoring/README.md`.

## Local smoke testing

There's no destination sandbox by default. To verify a change end-to-end:
point `DEST_DB_*`/`CERT_PATH` at a real (or second local) MySQL instance
with the same 15 tables already created, run `make run`, then make a change
on the source (e.g. `UPDATE dept SET DeptDesc=... WHERE DeptID=...` — `dept`
has a single-column PK, `sldtl` or `client` exercise the composite-key path)
and confirm it appears on the destination and in the daemon's log output.
