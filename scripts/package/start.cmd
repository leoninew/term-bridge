@echo off
setlocal
cd /d "%~dp0"
set "TERMBRIDGE_ENV=production"

termbridge.exe agent
