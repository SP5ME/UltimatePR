[CmdletBinding()]
param(
    [switch]$OpenBrowser,
    [switch]$RunTests
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$configPath = Join-Path $projectRoot "configs\example.yaml"
$runtimeDir = Join-Path $projectRoot ".runtime"
$serverExe = Join-Path $runtimeDir "ultimatepr-test.exe"
$goCache = Join-Path $env:TEMP "ultimatepr-go\cache"
$goModCache = Join-Path $env:TEMP "ultimatepr-go\mod"

$goExe = Get-Command go.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1
if (-not $goExe) {
    $standardGo = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path -LiteralPath $standardGo) {
        $goExe = $standardGo
    } else {
        Write-Host "Nie znaleziono Go. Zainstaluj Go lub dodaj go.exe do PATH." -ForegroundColor Red
        exit 1
    }
}

if (-not (Test-Path -LiteralPath $configPath)) {
    Write-Host "Brak konfiguracji: $configPath" -ForegroundColor Red
    exit 1
}

New-Item -ItemType Directory -Force -Path $runtimeDir, $goCache, $goModCache | Out-Null
$env:GOCACHE = $goCache
$env:GOMODCACHE = $goModCache
$env:GOFLAGS = "-buildvcs=false"

Push-Location $projectRoot
try {
    if ($RunTests) {
        Write-Host "Uruchamiam testy..." -ForegroundColor Cyan
        & $goExe test ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }

    Write-Host "Buduję serwer..." -ForegroundColor Cyan
    & $goExe build -o $serverExe ./cmd/server
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Write-Host ""
    Write-Host "Modern Packet BBS uruchamia się." -ForegroundColor Green
    Write-Host "Panel WWW:     http://127.0.0.1:8080"
    Write-Host "NODE Telnet:   127.0.0.1:8010"
    Write-Host "BBS Telnet:    127.0.0.1:8023"
    Write-Host "TNC KISS TCP:  127.0.0.1:8001 (serwer łączy się jako klient)"
    Write-Host "Zatrzymanie:   Ctrl+C"
    Write-Host ""
    Write-Host "Ostrzeżenia o 127.0.0.1:8001 są normalne, gdy Direwolf nie działa." -ForegroundColor Yellow
    Write-Host "Przykładowe połączenia do SR5DDD są wyłączone." -ForegroundColor Yellow
    Write-Host ""

    if ($OpenBrowser) {
        Start-Process "http://127.0.0.1:8080"
    }

    do {
        & $serverExe -config $configPath
        $serverExitCode = $LASTEXITCODE
        if ($serverExitCode -eq 75) {
            Write-Host "Konfiguracja zapisana. Ponownie uruchamiam serwer..." -ForegroundColor Cyan
            Start-Sleep -Milliseconds 500
        }
    } while ($serverExitCode -eq 75)
    exit $serverExitCode
} finally {
    Pop-Location
}
