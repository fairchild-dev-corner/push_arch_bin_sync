# Loads monitoring/.env and starts Loki with its data dir/ports passed in as
# env vars, expanded into loki-config.yml via -config.expand-env - same
# per-server-config-without-editing-files approach as Prometheus/Grafana.

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

$dataDir = Join-Path $root "loki\data"
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

$env:LOKI_HTTP_PORT = $config["LOKI_PORT"]
$env:LOKI_GRPC_PORT = $config["LOKI_GRPC_PORT"]
$env:LOKI_DATA_DIR = $dataDir

Write-Host "Starting Loki on port $($config['LOKI_PORT']) (data: $dataDir)..."
& $config["LOKI_BIN"] --config.file="$root\loki\loki-config.yml" --config.expand-env=true
