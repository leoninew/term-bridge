@echo off
setlocal EnableExtensions

set "TERMBRIDGE_DIR=%~dp0"
set "TERMBRIDGE_EXE=%TERMBRIDGE_DIR%termbridge.exe"
set "TERMBRIDGE_CMD=%TERMBRIDGE_DIR%termbridge.cmd"
set "TERMBRIDGE_TOOL_BIN=%USERPROFILE%\.local\bin"

if "%TERMBRIDGE_ENV%"=="" set "TERMBRIDGE_ENV=preflite"

if "%TERMBRIDGE_LOCAL__PUBLIC_URL%"=="" set "TERMBRIDGE_LOCAL__PUBLIC_URL=http://localhost:9030"

if exist "%TERMBRIDGE_TOOL_BIN%" set "PATH=%TERMBRIDGE_TOOL_BIN%;%PATH%"

cd /d "%TERMBRIDGE_DIR%" || exit /b 1

if "%~1"=="" goto :start
if /I "%~1"=="start" goto :start
if /I "%~1"=="stop" goto :stop
if /I "%~1"=="restart" goto :restart
if /I "%~1"=="reschedule" goto :reschedule
if /I "%~1"=="status" goto :status

echo Usage: %~nx0 ^<start^|stop^|restart^|reschedule^|status^>
exit /b 64

:start
call :locate_agent
if defined TERMBRIDGE_PIDS (
    echo TermBridge agent is already running with PIDs: %TERMBRIDGE_PIDS%
    exit /b 0
)
goto :run_agent

:stop
call :stop_running
exit /b %ERRORLEVEL%

:restart
call :stop_running
if errorlevel 1 exit /b %ERRORLEVEL%
goto :run_agent_without_browser

:reschedule
set "TERMBRIDGE_RESCHEDULE_TASK=TermBridge-Reschedule-%RANDOM%-%RANDOM%"
echo Scheduling an independent TermBridge restart in 5 seconds.
powershell.exe -NoProfile -Command "$taskName = $env:TERMBRIDGE_RESCHEDULE_TASK; $script = $env:TERMBRIDGE_CMD; $user = $env:USERDOMAIN + '\' + $env:USERNAME; $argument = '/d /c timeout /t 5 /nobreak ' + [char]38 + ' call ' + [char]34 + $script + [char]34 + ' restart'; $action = New-ScheduledTaskAction -Execute $env:ComSpec -Argument $argument; $principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Highest; $task = New-ScheduledTask -Action $action -Principal $principal; $null = Register-ScheduledTask -TaskName $taskName -InputObject $task -Force; Start-ScheduledTask -TaskName $taskName"
if errorlevel 1 (
    echo Failed to schedule the TermBridge restart.
    exit /b 1
)
echo TermBridge restart scheduled as %TERMBRIDGE_RESCHEDULE_TASK%.
exit /b 0

:status
call :locate_agent
if not defined TERMBRIDGE_PIDS (
    echo TermBridge agent is not running from "%TERMBRIDGE_EXE%".
    exit /b 3
)
echo TermBridge agent is running from "%TERMBRIDGE_EXE%" with PIDs: %TERMBRIDGE_PIDS%
exit /b 0

:run_agent
start "" "%TERMBRIDGE_LOCAL__PUBLIC_URL%"

:run_agent_without_browser
"%TERMBRIDGE_EXE%" agent
exit /b %ERRORLEVEL%

:stop_running
call :locate_agent
if not defined TERMBRIDGE_PIDS (
    echo TermBridge agent is not running from "%TERMBRIDGE_EXE%".
    exit /b 0
)

echo Stopping TermBridge agent PIDs: %TERMBRIDGE_PIDS%
for %%P in (%TERMBRIDGE_PIDS%) do taskkill /PID %%P /T >nul 2>&1
timeout /t 3 /nobreak >nul

call :locate_agent
if defined TERMBRIDGE_PIDS (
    echo Forcing TermBridge agent PIDs: %TERMBRIDGE_PIDS%
    for %%P in (%TERMBRIDGE_PIDS%) do taskkill /PID %%P /T /F >nul 2>&1
)

call :locate_agent
if defined TERMBRIDGE_PIDS (
    echo Failed to stop TermBridge agent PIDs: %TERMBRIDGE_PIDS%
    exit /b 1
)

echo TermBridge agent stopped.
exit /b 0

:locate_agent
set "TERMBRIDGE_PIDS="
for /f %%P in ('powershell.exe -NoProfile -Command "$target = [System.IO.Path]::GetFullPath($env:TERMBRIDGE_EXE); foreach ($process in Get-CimInstance Win32_Process) { $command = $process.CommandLine; if ($process.Name -ieq 'termbridge.exe' -and $process.ExecutablePath -and [System.IO.Path]::GetFullPath($process.ExecutablePath) -ieq $target -and $command -and $command.TrimEnd().EndsWith(' agent', [System.StringComparison]::OrdinalIgnoreCase)) { $process.ProcessId } }"') do (
    if defined TERMBRIDGE_PIDS (
        set "TERMBRIDGE_PIDS=%TERMBRIDGE_PIDS% %%P"
    ) else (
        set "TERMBRIDGE_PIDS=%%P"
    )
)
exit /b 0
