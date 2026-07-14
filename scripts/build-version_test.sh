#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
helper="$repo_root/scripts/build-version.sh"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

create_repo() {
  dir=$1
  git init -q "$dir"
  git -C "$dir" config user.name test
  git -C "$dir" config user.email test@example.com
  printf 'initial\n' > "$dir/file"
  git -C "$dir" add file
  git -C "$dir" commit -qm initial
}

assert_equal() {
  actual=$1
  expected=$2
  scenario=$3
  if [ "$actual" != "$expected" ]; then
    printf '%s: got %s, want %s\n' "$scenario" "$actual" "$expected" >&2
    exit 1
  fi
}

no_tag_repo="$fixture/no-tag"
create_repo "$no_tag_repo"
no_tag_short=$(git -C "$no_tag_repo" rev-parse --short=12 HEAD)
assert_equal "$(cd "$no_tag_repo" && sh "$helper")" "dev-1-g$no_tag_short" "no tag"

git -C "$no_tag_repo" tag v1.2.3
assert_equal "$(cd "$no_tag_repo" && sh "$helper")" "v1.2.3" "exact tag"
printf 'next\n' >> "$no_tag_repo/file"
git -C "$no_tag_repo" commit -qam next
after_tag_short=$(git -C "$no_tag_repo" rev-parse --short=12 HEAD)
assert_equal "$(cd "$no_tag_repo" && sh "$helper")" "v1.2.3-1-g$after_tag_short" "commit after tag"
printf 'dirty\n' >> "$no_tag_repo/file"
assert_equal "$(cd "$no_tag_repo" && sh "$helper")" "v1.2.3-1-g$after_tag_short-dirty" "dirty worktree"
assert_equal "$(cd "$no_tag_repo" && TERMBRIDGE_BUILD_VERSION=v9.9.9 sh "$helper")" "v9.9.9" "explicit override"
