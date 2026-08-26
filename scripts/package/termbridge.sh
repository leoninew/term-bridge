#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"

server_path=$script_dir/termbridge
if [ -f ./termbridge.exe ]; then
  server_path=$script_dir/termbridge.exe
fi
server_name=${server_path##*/}

: "${TERMBRIDGE_ENV:=preflite}"
export TERMBRIDGE_ENV

: "${TERMBRIDGE_LOCAL__PUBLIC_URL:=http://localhost:9030}"
export TERMBRIDGE_LOCAL__PUBLIC_URL

tool_bin=${HOME:-}/.local/bin
if [ -d "$tool_bin" ]; then
  PATH=$tool_bin:$PATH
  export PATH
fi

find_running_pids() {
  ps -ef | awk -v server_name="$server_name" '
    NR > 1 && /[[:space:]]agent[[:space:]]*$/ && index($0, server_name) { print $2 }
  '
}

open_browser() {
  if command -v cygstart >/dev/null 2>&1; then
    cygstart "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
  elif command -v start >/dev/null 2>&1; then
    start "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
  elif command -v open >/dev/null 2>&1; then
    open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
  fi
}

stop_agent() {
  pids=$(find_running_pids || true)
  if [ -z "$pids" ]; then
    printf '%s\n' "TermBridge agent is not running from $server_path."
    return 0
  fi

  printf '%s\n' "Stopping TermBridge agent PID(s): $pids"
  # Request a normal termination before falling back to SIGKILL.
  # shellcheck disable=SC2086
  kill $pids 2>/dev/null || true
  sleep 3

  pids=$(find_running_pids || true)
  if [ -n "$pids" ]; then
    printf '%s\n' "Forcing TermBridge agent PID(s): $pids"
    # shellcheck disable=SC2086
    kill -KILL $pids 2>/dev/null || true
  fi

  pids=$(find_running_pids || true)
  if [ -n "$pids" ]; then
    printf '%s\n' "Failed to stop TermBridge agent PID(s): $pids" >&2
    return 1
  fi

  printf '%s\n' 'TermBridge agent stopped.'
}

start_agent() {
  pids=$(find_running_pids || true)
  if [ -n "$pids" ]; then
    printf '%s\n' "TermBridge agent is already running with PID(s): $pids"
    return 0
  fi
  open_browser
  exec "$server_path" agent
}

restart_agent() {
  stop_agent
  exec "$server_path" agent
}

case "${1:-start}" in
  start)
    start_agent
    ;;
  stop)
    stop_agent
    ;;
  restart)
    restart_agent
    ;;
  status)
    pids=$(find_running_pids || true)
    if [ -z "$pids" ]; then
      printf '%s\n' "TermBridge agent is not running from $server_path."
      exit 3
    fi
    printf '%s\n' "TermBridge agent is running from $server_path with PID(s): $pids"
    ;;
  *)
    printf '%s\n' "Usage: ${0##*/} <start|stop|restart|status>" >&2
    exit 64
    ;;
esac
