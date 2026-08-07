param(
    [switch]$Release,
    [string]$Output = "mcp-server.exe"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$ver = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()

Write-Host "=== system-cleanup-mcp build ===" -ForegroundColor Cyan
Write-Host "Version: $ver" -ForegroundColor Gray

$go = Get-Command "go" -ErrorAction SilentlyContinue
if (-not $go) { Write-Host "Go not found." -ForegroundColor Red; exit 1 }

$ldflags = "-s -w -X main.Version=$ver"
if (-not $Release) {
    $ldflags = "-X main.Version=$ver"
}

Write-Host "Building..." -ForegroundColor Gray
go build -ldflags="$ldflags" -o $Output .\cmd\mcp-server\
if (-not $?) { exit 1 }

$sizeBytes = (Get-Item $Output).Length
$mib = [math]::Round($sizeBytes / 1048576, 1)
Write-Host "OK: $Output ($mib` MB)" -ForegroundColor Green
