@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall-interactive.ps1"
set "EXIT_CODE=%ERRORLEVEL%"
if not "%EXIT_CODE%"=="0" (
  echo.
  echo 1Panel uninstall failed, exit code: %EXIT_CODE%
  pause
  exit /b %EXIT_CODE%
)
exit /b 0
