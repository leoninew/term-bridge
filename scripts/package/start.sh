#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
export TERMBRIDGE_ENV=local

if [ ! -f .env.local ]; then
  printf '%s\n' '.env.local is missing' >&2
  exit 1
fi

set -a
. ./.env.local
set +a

if [ -z "${TERMBRIDGE_AGENT__PUBLIC_URL:-}" ]; then
  printf '%s\n' 'TERMBRIDGE_AGENT__PUBLIC_URL is not set in .env.local' >&2
  exit 1
fi

(
  sleep 2
  if command -v cygstart >/dev/null 2>&1; then
    cygstart "$TERMBRIDGE_AGENT__PUBLIC_URL" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$TERMBRIDGE_AGENT__PUBLIC_URL" >/dev/null 2>&1 || true
  elif command -v open >/dev/null 2>&1; then
    open "$TERMBRIDGE_AGENT__PUBLIC_URL" >/dev/null 2>&1 || true
  fi
) &
./termbridge.exe agent
