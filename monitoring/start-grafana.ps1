# Loads monitoring/.env and starts Grafana with the right provisioning
# path/port/datasource URL set as env vars, so no provisioning YAML needs
# per-server edits.

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

$envFile = Join-Path $root ".env"
if (-not (Test-Path $envFile)) {
    Write-Error "monitoring\.env not found - copy monitoring\.env.example to monitoring\.env and fill it in first."
    exit 1
}

$config = @{}
Get-Content $envFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
        $key, $value = $line.Split("=", 2)
        $config[$key.Trim()] = $value.Trim()
    }
}

$dataDir = Join-Path $root "grafana\data"
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

$env:GF_PATHS_PROVISIONING = $config["GRAFANA_PROVISIONING_PATH"]
$env:GRAFANA_DASHBOARDS_PATH = $config["GRAFANA_DASHBOARDS_PATH"]
$env:PROMETHEUS_URL = $config["PROMETHEUS_URL"]
$env:LOKI_URL = $config["LOKI_URL"]
$env:GF_SERVER_HTTP_PORT = $config["GRAFANA_PORT"]
# Own data dir (grafana.db, logs, plugins) so this instance never shares
# state with a Grafana installed as a Windows Service (a separate SQLite
# writer on the same grafana.db fails with "attempt to write a readonly
# database" - tested and confirmed on this machine).
$env:GF_PATHS_DATA = $dataDir
$env:GF_PATHS_LOGS = Join-Path $dataDir "log"

$grafanaBin = $config["GRAFANA_BIN"]
$grafanaArgs = @()
if ((Split-Path $grafanaBin -Leaf) -match '^grafana(\.exe)?$') {
    # Grafana >= 10's unified binary needs the "server" subcommand, and
    # (unlike grafana-server.exe) doesn't auto-detect its own install root
    # when invoked by absolute path from elsewhere - it needs --homepath
    # pointing at the directory containing conf/ and public/ (one level up
    # from bin/) or it fails with "Could not find config defaults".
    $grafanaHome = Split-Path (Split-Path $grafanaBin -Parent) -Parent
    $grafanaArgs = @("server", "--homepath=$grafanaHome")
}

Write-Host "Starting Grafana on port $($config['GRAFANA_PORT'])..."
Write-Host "Provisioning from: $($config['GRAFANA_PROVISIONING_PATH'])"
& $grafanaBin @grafanaArgs
