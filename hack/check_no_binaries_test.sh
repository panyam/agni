#!/bin/sh
# Holds hack/check_no_binaries.sh to its two refusals, with a clean tree as the control.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
git -C "$t" init -q
echo "hello" >"$t/readme.txt"
git -C "$t" add readme.txt
"$here/check_no_binaries.sh" "$t" >/dev/null || { echo "FAIL: a clean tree was refused" >&2; exit 1; }
printf '\177ELF\002\001\001' >"$t/tool"
git -C "$t" add tool
if "$here/check_no_binaries.sh" "$t" >/dev/null 2>&1; then echo "FAIL: a tracked ELF passed" >&2; exit 1; fi
git -C "$t" rm -q --cached tool
head -c 5300000 /dev/zero >"$t/big.dat"
git -C "$t" add big.dat
if "$here/check_no_binaries.sh" "$t" >/dev/null 2>&1; then echo "FAIL: a tracked 5.3 MB file passed" >&2; exit 1; fi
echo "ok   check_no_binaries refuses an executable and a large file, and passes a clean tree"
