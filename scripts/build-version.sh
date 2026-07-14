#!/usr/bin/env sh
set -eu

if [ -n "${TERMBRIDGE_BUILD_VERSION:-}" ]; then
  printf '%s\n' "$TERMBRIDGE_BUILD_VERSION"
  exit 0
fi

short_commit=$(git rev-parse --short=12 HEAD)
exact_tag=$(git tag --points-at HEAD --list 'v[0-9]*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$' | head -n 1 || true)

if [ -n "$exact_tag" ]; then
  version=$exact_tag
else
  described_version=$(git describe --tags --long --abbrev=12 --match 'v[0-9]*.[0-9]*.[0-9]*' HEAD 2>/dev/null || true)
  if printf '%s\n' "$described_version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?-[0-9]+-g[0-9a-f]+$'; then
    version=$described_version
  else
    version="dev-$(git rev-list --count HEAD)-g$short_commit"
  fi
fi

if [ -n "$(git status --porcelain --untracked-files=all)" ]; then
  version="$version-dirty"
fi

printf '%s\n' "$version"
