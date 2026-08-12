@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0run-test-windows.ps1" -OpenBrowser %*
