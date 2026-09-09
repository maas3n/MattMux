@echo off
setlocal
cd /d "%~dp0"
echo MattMux 1.2.0 Fully Portable / Offline builder
echo.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Build-Offline-Package.ps1"
echo.
if errorlevel 1 (
  echo Build failed. Review the error above.
) else (
  echo Finished.
)
pause
