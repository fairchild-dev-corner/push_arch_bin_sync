#!/usr/bin/env bash
# Loads monitoring/.env and starts Grafana with the right provisioning
# path/port/datasource URL set as env vars, so no provisioning YAML needs
# per-server edits.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
env_file="$root/.env"

if [ ! -f "$env_file" ]; then
    echo "monitoring/.env not found - copy monitoring/.env.example to monitoring/.env and fill it in first." >&2
    exit 1
fi

# Parsed manually rather than `source`d - values (e.g. Windows paths with
# spaces) aren't guaranteed to be shell-quoted in .env, so a plain `source`
# would break on them.
while IFS='=' read -r key value; do
    key="${key%$'\r'}"
    value="${value%$'\r'}"
    case "$key" in
        ''|'#'*) continue ;;
    esac
    export "$key=$value"
done < "$env_file"

data_dir="$root/grafana/data"
mkdir -p "$data_dir"

export GF_PATHS_PROVISIONING="$GRAFANA_PROVISIONING_PATH"
export GRAFANA_DASHBOARDS_PATH="$GRAFANA_DASHBOARDS_PATH"
export PROMETHEUS_URL="$PROMETHEUS_URL"
export LOKI_URL="$LOKI_URL"
export GF_SERVER_HTTP_PORT="$GRAFANA_PORT"
# Own data dir (grafana.db, logs, plugins) so this instance never shares
# state with a Grafana installed as a Windows Service (a separate SQLite
# writer on the same grafana.db fails with "attempt to write a readonly
# database" - tested and confirmed on this machine).
export GF_PATHS_DATA="$data_dir"
export GF_PATHS_LOGS="$data_dir/log"

grafana_args=()
base="$(basename "$GRAFANA_BIN")"
if [ "$base" = "grafana" ] || [ "$base" = "grafana.exe" ]; then
    # Grafana >= 10's unified binary needs the "server" subcommand, and
    # (unlike grafana-server.exe) doesn't auto-detect its own install root
    # when invoked by absolute path from elsewhere - it needs --homepath
    # pointing at the directory containing conf/ and public/ (one level up
    # from bin/) or it fails with "Could not find config defaults".
    grafana_home="$(dirname "$(dirname "$GRAFANA_BIN")")"
    grafana_args=(server "--homepath=${grafana_home}")
fi

echo "Starting Grafana on port ${GRAFANA_PORT}..."
echo "Provisioning from: ${GRAFANA_PROVISIONING_PATH}"
"${GRAFANA_BIN}" "${grafana_args[@]}"
