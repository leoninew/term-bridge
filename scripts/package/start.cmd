@echo off
setlocal
cd /d "%~dp0"
set "TERMBRIDGE_ENV=local"

if not exist ".env.local" (
  echo .env.local is missing
  exit /b 1
)

for /f "usebackq eol=# tokens=1,* delims==" %%A in (".env.local") do (
  if not "%%A"=="" set "%%A=%%B"
)

if not defined TERMBRIDGE_AGENT__PUBLIC_URL (
  echo TERMBRIDGE_AGENT__PUBLIC_URL is not set in .env.local
  exit /b 1
)

start "" /min cmd /c "timeout /t 2 /nobreak >nul & start %TERMBRIDGE_AGENT__PUBLIC_URL%"
termbridge.exe agent
