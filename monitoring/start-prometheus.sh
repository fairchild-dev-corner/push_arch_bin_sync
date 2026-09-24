#!/usr/bin/env bash
# Loads monitoring/.env, generates prometheus.yml from the template, and
# starts Prometheus. Run from anywhere; paths are resolved relative to
# this script's own location.
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

template="$root/prometheus/prometheus.yml.template"
generated="$root/prometheus/prometheus.yml"
sed "s|__METRICS_TARGET__|${METRICS_TARGET}|g" "$template" > "$generated"

data_dir="$root/prometheus/data"
mkdir -p "$data_dir"

echo "Generated $generated (target: ${METRICS_TARGET})"
echo "Starting Prometheus (data: $data_dir)..."
"${PROMETHEUS_BIN}" --config.file="$generated" --storage.tsdb.path="$data_dir"
