# Loads monitoring/.env, generates prometheus.yml from the template, and
# starts Prometheus. Run from anywhere; paths are resolved relative to
# this script's own location.

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

$template = Join-Path $root "prometheus\prometheus.yml.template"
$generated = Join-Path $root "prometheus\prometheus.yml"
(Get-Content $template) -replace [regex]::Escape("__METRICS_TARGET__"), $config["METRICS_TARGET"] | Set-Content $generated

$dataDir = Join-Path $root "prometheus\data"
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

Write-Host "Generated $generated (target: $($config['METRICS_TARGET']))"
Write-Host "Starting Prometheus (data: $dataDir)..."
& $config["PROMETHEUS_BIN"] --config.file="$generated" --storage.tsdb.path="$dataDir"
