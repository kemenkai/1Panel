@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-interactive.ps1"
set "EXIT_CODE=%ERRORLEVEL%"
if not "%EXIT_CODE%"=="0" (
  echo.
  echo 1Panel install failed, exit code: %EXIT_CODE%
  pause
  exit /b %EXIT_CODE%
)
exit /b 0
