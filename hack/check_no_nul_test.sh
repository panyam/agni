#!/bin/sh
# Holds hack/check_no_nul.sh to its refusal, with a clean tree and a binary image as controls.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
git -C "$t" init -q
echo 'const sep = "\u0000";' >"$t/keys.ts"
printf '\211PNG\r\n\032\n\000\000' >"$t/icon.png"
git -C "$t" add keys.ts icon.png
"$here/check_no_nul.sh" "$t" >/dev/null || { echo "FAIL: a clean tree with a PNG was refused" >&2; exit 1; }
printf 'const sep = "\000";\n' >"$t/raw.ts"
git -C "$t" add raw.ts
if "$here/check_no_nul.sh" "$t" >/dev/null 2>&1; then echo "FAIL: a tracked .ts with a NUL byte passed" >&2; exit 1; fi
echo "ok   check_no_nul refuses a text file with a NUL byte, and passes escapes and a PNG"
