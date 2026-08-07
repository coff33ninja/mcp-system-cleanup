# Build and launch system-cleanup-mcp
$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$exe = Join-Path $root "mcp-server.exe"
$version = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()

if (-not (Test-Path -LiteralPath $exe)) {
    Write-Host "[*] Building mcp-server.exe (v$version)..." -ForegroundColor Cyan
    Push-Location $root
    try {
        go build -ldflags="-X main.Version=$version" -o mcp-server.exe ./cmd/mcp-server
    } finally {
        Pop-Location
    }
}

Write-Host "[+] mcp-server.exe v$version ready at $exe" -ForegroundColor Green
Write-Host "[*] Add to opencode.json:"
Write-Host @"
{
  "mcp": {
    "system-cleanup": {
      "type": "local",
      "command": ["$exe"],
      "enabled": true
    }
  }
}
"@ -ForegroundColor Yellow
