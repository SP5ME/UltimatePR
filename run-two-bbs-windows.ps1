[CmdletBinding()]
param(
    [switch]$NoBrowser,
    [switch]$RunTests,
    [int]$DurationSeconds = 0
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$runtimeDir = Join-Path $projectRoot ".runtime"
$serverExe = Join-Path $runtimeDir "modernbbs-two-test.exe"
$goCache = Join-Path $env:TEMP "modernbbs-go\cache"
$goModCache = Join-Path $env:TEMP "modernbbs-go\mod"
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("modernbbs-two-" + [guid]::NewGuid().ToString("N"))
$processA = $null
$processB = $null

function Find-Go {
    $command = Get-Command go.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    $standard = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path -LiteralPath $standard) { return $standard }
    throw "Nie znaleziono go.exe. Zainstaluj Go lub dodaj je do PATH."
}

function Start-BBSProcess([string]$configPath, [string]$logPath, [string]$errorPath) {
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $serverExe
    $info.Arguments = '-config "' + $configPath + '"'
    $info.WorkingDirectory = $projectRoot
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
	$info.RedirectStandardOutput = $true
	$info.RedirectStandardError = $true
    return [System.Diagnostics.Process]::Start($info)
}

function Stop-BBSProcess($process) {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -ErrorAction SilentlyContinue
        $process.WaitForExit(3000) | Out-Null
    }
}

function Remove-TestDirectory([string]$path) {
    if (-not (Test-Path -LiteralPath $path)) { return }
    $resolved = (Resolve-Path -LiteralPath $path).Path
    $tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    $leaf = Split-Path -Leaf $resolved
    if (-not $resolved.StartsWith($tempRoot, [System.StringComparison]::OrdinalIgnoreCase) -or
        -not $leaf.StartsWith("modernbbs-two-", [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Odmowa usuniecia nieoczekiwanego katalogu: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

$configA = @"
server:
  callsign: SP5AAA
  ssid: 7
web:
  listen: "127.0.0.1:8080"
ports: []
terminal:
  callsign: SP5AAA
  ssid: 9
bbs:
  enabled: true
  language: pl
  listen: "127.0.0.1:8023"
  forward_listen: "127.0.0.1:7300"
  database: "$($testRoot.Replace('\','/'))/bbs-a.json"
  title: "Testowy BBS A"
  callsign: SP5AAA
  ssid: 8
  hierarchical_address: "SP5AAA.#PL.POL.EU"
  forwarding:
    enabled: true
    interval_minutes: 1
    connect_timeout_seconds: 5
    session_timeout_seconds: 30
    max_messages_per_session: 50
    max_body_bytes: 20000
    peers:
      - id: bbs-b
        callsign: SP5BBB-8
        enabled: true
        transport: telnet
        host: 127.0.0.1
        port: 7301
        schedule: ["00:00-23:59"]
        private_routes: ["SP5BBB", "#PL"]
        bulletin_scopes: ["POL", "EU"]
node:
  enabled: true
  language: pl
  alias: NODEA
  listen: "127.0.0.1:8010"
  neighbors: []
  routes: []
  services:
    - name: BBS
      callsign: SP5AAA-8
      command: BBS
      enabled: true
"@

$configB = @"
server:
  callsign: SP5BBB
  ssid: 7
web:
  listen: "127.0.0.1:8081"
ports: []
terminal:
  callsign: SP5BBB
  ssid: 9
bbs:
  enabled: true
  language: pl
  listen: "127.0.0.1:8024"
  forward_listen: "127.0.0.1:7301"
  database: "$($testRoot.Replace('\','/'))/bbs-b.json"
  title: "Testowy BBS B"
  callsign: SP5BBB
  ssid: 8
  hierarchical_address: "SP5BBB.#PL.POL.EU"
  forwarding:
    enabled: true
    interval_minutes: 1
    connect_timeout_seconds: 5
    session_timeout_seconds: 30
    max_messages_per_session: 50
    max_body_bytes: 20000
    peers:
      - id: bbs-a
        callsign: SP5AAA-8
        enabled: true
        transport: telnet
        host: 127.0.0.1
        port: 7300
        schedule: ["00:00-23:59"]
        private_routes: ["SP5AAA", "#PL"]
        bulletin_scopes: ["POL", "EU"]
node:
  enabled: true
  language: pl
  alias: NODEB
  listen: "127.0.0.1:8011"
  neighbors: []
  routes: []
  services:
    - name: BBS
      callsign: SP5BBB-8
      command: BBS
      enabled: true
"@

try {
    $goExe = Find-Go
    New-Item -ItemType Directory -Force -Path $runtimeDir, $goCache, $goModCache, $testRoot | Out-Null
    $env:GOCACHE = $goCache
    $env:GOMODCACHE = $goModCache
    $env:GOFLAGS = "-buildvcs=false"

    $configAPath = Join-Path $testRoot "test-a.yaml"
    $configBPath = Join-Path $testRoot "test-b.yaml"
    Set-Content -LiteralPath $configAPath -Value $configA -Encoding UTF8
    Set-Content -LiteralPath $configBPath -Value $configB -Encoding UTF8

    Push-Location $projectRoot
    try {
        if ($RunTests) {
            Write-Host "Uruchamiam testy projektu..." -ForegroundColor Cyan
            & $goExe test ./...
            if ($LASTEXITCODE -ne 0) { throw "Testy nie powiodly sie." }
        }
        Write-Host "Buduje serwer..." -ForegroundColor Cyan
        & $goExe build -o $serverExe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Kompilacja nie powiodla sie." }
    } finally {
        Pop-Location
    }

    $processA = Start-BBSProcess $configAPath (Join-Path $testRoot "bbs-a.log") (Join-Path $testRoot "bbs-a.err")
    $processB = Start-BBSProcess $configBPath (Join-Path $testRoot "bbs-b.log") (Join-Path $testRoot "bbs-b.err")
    Start-Sleep -Seconds 2

    if ($processA.HasExited -or $processB.HasExited) {
		if ($processA.HasExited) { Write-Host "BBS A output:"; Write-Host $processA.StandardOutput.ReadToEnd(); Write-Host $processA.StandardError.ReadToEnd() }
		if ($processB.HasExited) { Write-Host "BBS B output:"; Write-Host $processB.StandardOutput.ReadToEnd(); Write-Host $processB.StandardError.ReadToEnd() }
        throw "Jedna z instancji zakonczyla prace. Sprawdz zajecie portow 8080/8081, 8010/8011 i 8023/8024."
    }

    Write-Host ""
    Write-Host "Dwie tymczasowe instancje dzialaja:" -ForegroundColor Green
    Write-Host "Wersja: ModernBBS 0.4.0-dev | forwarding: B2F/LZHUF" -ForegroundColor Green
    Write-Host "BBS A: WWW 8080 | NODE 8010 | BBS 8023 | FWD 7300"
    Write-Host "BBS B: WWW 8081 | NODE 8011 | BBS 8024 | FWD 7301"
    Write-Host "Katalog tymczasowy: $testRoot"
    Write-Host "Zatrzymanie i sprzatanie: Ctrl+C" -ForegroundColor Yellow
    Write-Host ""

    if (-not $NoBrowser) {
        Start-Process "http://127.0.0.1:8080"
        Start-Process "http://127.0.0.1:8081"
    }

    $deadline = if ($DurationSeconds -gt 0) { [DateTime]::UtcNow.AddSeconds($DurationSeconds) } else { [DateTime]::MaxValue }
    while (-not $processA.HasExited -and -not $processB.HasExited -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host "Zatrzymuje instancje testowe..." -ForegroundColor Cyan
    Stop-BBSProcess $processA
    Stop-BBSProcess $processB
    Remove-TestDirectory $testRoot
    Write-Host "Usunieto tymczasowe konfiguracje, bazy i logi. Obecny projekt pozostal bez zmian." -ForegroundColor Green
}
