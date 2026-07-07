#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
export TERMBRIDGE_ENV=production

./termbridge.exe agent
