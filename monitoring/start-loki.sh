#!/usr/bin/env bash
# Loads monitoring/.env and starts Loki with its data dir/ports passed in as
# env vars, expanded into loki-config.yml via -config.expand-env - same
# per-server-config-without-editing-files approach as Prometheus/Grafana.
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

data_dir="$root/loki/data"
mkdir -p "$data_dir"

export LOKI_HTTP_PORT="$LOKI_PORT"
export LOKI_GRPC_PORT="$LOKI_GRPC_PORT"
export LOKI_DATA_DIR="$data_dir"

echo "Starting Loki on port ${LOKI_PORT} (data: $data_dir)..."
"${LOKI_BIN}" --config.file="$root/loki/loki-config.yml" --config.expand-env=true
