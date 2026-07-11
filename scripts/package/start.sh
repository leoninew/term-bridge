#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
: "${TERMBRIDGE_ENV:=prod}"
export TERMBRIDGE_ENV
profile=.env.prod
if [ "$TERMBRIDGE_ENV" = test ]; then
  profile=.env.test
fi

if [ ! -f "$profile" ]; then
  printf '%s\n' "$profile is missing" >&2
  exit 1
fi

set -a
. "./$profile"
set +a

if [ -z "${TERMBRIDGE_LOCAL__PUBLIC_URL:-}" ]; then
  printf '%s\n' "TERMBRIDGE_LOCAL__PUBLIC_URL is not set in $profile" >&2
  exit 1
fi

if command -v cygstart >/dev/null 2>&1; then
  cygstart "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v start >/dev/null 2>&1; then
  start "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
elif command -v open >/dev/null 2>&1; then
  open "$TERMBRIDGE_LOCAL__PUBLIC_URL" || true
fi

./termbridge.exe agent
