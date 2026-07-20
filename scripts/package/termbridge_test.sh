#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
launcher="$repo_root/scripts/package/termbridge.sh"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

package_root="$fixture/package"
mkdir -p "$package_root/bin"
cp "$launcher" "$package_root/termbridge.sh"
cat > "$package_root/.env.preflite" <<'EOF'
TERMBRIDGE_RUNTIME__STATE_DIR=~/.termbridge
TERMBRIDGE_LOCAL__DATABASE__SQLITE__PATH=~/.termbridge/agent.db
TERMBRIDGE_LOG__DIR=~/.termbridge/logs
EOF
cat > "$package_root/termbridge" <<'EOF'
#!/usr/bin/env sh
printf '%s\n' "${TERMBRIDGE_ENV-unset}" "${TERMBRIDGE_RUNTIME__STATE_DIR-unset}" "${TERMBRIDGE_LOCAL__DATABASE__SQLITE__PATH-unset}" "${TERMBRIDGE_LOG__DIR-unset}" > "$TERMBRIDGE_TEST_OUTPUT"
EOF
cat > "$package_root/bin/cygstart" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
chmod +x "$package_root/termbridge" "$package_root/bin/cygstart"

output="$fixture/environment"
(
  cd "$package_root"
  env -i \
    PATH="$package_root/bin:$PATH" \
    HOME=/c/Users/cygwin-user \
    USERPROFILE='C:\Users\native-user' \
    HOMEDRIVE='C:' \
    HOMEPATH='\Users\native-user' \
    TERMBRIDGE_TEST_OUTPUT="$output" \
    sh ./termbridge.sh
)

expected=$(printf '%s\n' preflite unset unset unset)
actual=$(cat "$output")
if [ "$actual" != "$expected" ]; then
  printf '%s\n' "launcher exported package profile values: got $actual, want $expected" >&2
  exit 1
fi
