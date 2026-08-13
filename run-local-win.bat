@echo off
setlocal EnableExtensions

rem Run UltimatePR locally on Windows from the repository root.

set "ROOT=%~dp0"
cd /d "%ROOT%"

set "GO_BIN=C:\Program Files\Go\bin\go.exe"
if not exist "%GO_BIN%" set "GO_BIN=go"

set "GOCACHE=%ROOT%.gocache"
set "GOPATH=%ROOT%.gopath"
set "GOMODCACHE=%ROOT%.gomodcache"

if not exist "%GOCACHE%" mkdir "%GOCACHE%"
if not exist "%GOPATH%" mkdir "%GOPATH%"
if not exist "%GOMODCACHE%" mkdir "%GOMODCACHE%"

if not exist "%ROOT%config.yaml" (
  if exist "%ROOT%configs\example.yaml" (
    copy /Y "%ROOT%configs\example.yaml" "%ROOT%config.yaml" >nul
    echo Created config.yaml from configs\example.yaml
  ) else (
    echo Missing config.yaml and configs\example.yaml.
    exit /b 1
  )
)

echo Starting UltimatePR...
echo Open http://127.0.0.1:8080 in your browser.
echo Press Ctrl+C to stop.

:restart
"%GO_BIN%" run ./cmd/server -config config.yaml
if "%ERRORLEVEL%"=="75" (
  echo UltimatePR requested a restart.
  timeout /t 1 /nobreak >nul
  goto restart
)

endlocal
