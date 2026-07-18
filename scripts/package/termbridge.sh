#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
: "${TERMBRIDGE_ENV:=preflite}"
export TERMBRIDGE_ENV
profile=.env.preflite

if [ ! -f "$profile" ]; then
  printf '%s\n' "$profile is missing" >&2
  exit 1
fi

set -a
. "./$profile"
set +a

: "${TERMBRIDGE_LOCAL__PUBLIC_URL:=http://localhost:9030}"
export TERMBRIDGE_LOCAL__PUBLIC_URL

if command -v cygstart >/dev/null 2>&1; then
  cygstart "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v start >/dev/null 2>&1; then
  start "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v open >/dev/null 2>&1; then
  open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
fi

./termbridge agent
