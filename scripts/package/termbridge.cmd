@echo off
setlocal
cd /d "%~dp0"
if "%TERMBRIDGE_ENV%"=="" set "TERMBRIDGE_ENV=preflite"
set "TERMBRIDGE_PROFILE=.env.preflite"

if not exist "%TERMBRIDGE_PROFILE%" (
  echo %TERMBRIDGE_PROFILE% is missing
  exit /b 1
)

for /f "usebackq eol=# tokens=1,* delims==" %%A in ("%TERMBRIDGE_PROFILE%") do (
  if not "%%A"=="" set "%%A=%%B"
)

if "%TERMBRIDGE_LOCAL__PUBLIC_URL%"=="" set "TERMBRIDGE_LOCAL__PUBLIC_URL=http://localhost:9030"

start "" "%TERMBRIDGE_LOCAL__PUBLIC_URL%"
termbridge.exe agent
