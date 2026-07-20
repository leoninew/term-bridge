@echo off
setlocal
cd /d "%~dp0"
if "%TERMBRIDGE_ENV%"=="" set "TERMBRIDGE_ENV=preflite"

if "%TERMBRIDGE_LOCAL__PUBLIC_URL%"=="" set "TERMBRIDGE_LOCAL__PUBLIC_URL=http://localhost:9030"

start "" "%TERMBRIDGE_LOCAL__PUBLIC_URL%"
termbridge.exe agent
