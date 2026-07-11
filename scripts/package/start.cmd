@echo off
setlocal
cd /d "%~dp0"
if "%TERMBRIDGE_ENV%"=="" set "TERMBRIDGE_ENV=prod"
set "TERMBRIDGE_PROFILE=.env.prod"
if /I "%TERMBRIDGE_ENV%"=="test" set "TERMBRIDGE_PROFILE=.env.test"

if not exist "%TERMBRIDGE_PROFILE%" (
  echo %TERMBRIDGE_PROFILE% is missing
  exit /b 1
)

for /f "usebackq eol=# tokens=1,* delims==" %%A in ("%TERMBRIDGE_PROFILE%") do (
  if not "%%A"=="" set "%%A=%%B"
)

if "%TERMBRIDGE_LOCAL__PUBLIC_URL%"=="" (
  echo TERMBRIDGE_LOCAL__PUBLIC_URL is not set in %TERMBRIDGE_PROFILE%
  exit /b 1
)

start "" "%TERMBRIDGE_LOCAL__PUBLIC_URL%"
termbridge.exe agent
