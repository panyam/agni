#!/usr/bin/env bash
# Copies one design tree under web/static/seed/<mount>/ and writes the manifest the viewer's
# `?engine=wasm&seed=` reads (web/src/wasm/client.ts, mountSeed). It is the stand-in for the
# loaders #853 designs, so it lists every file and the page fetches all of them.
#
#   hack/wasm_seed.sh <mount> <dir>
set -euo pipefail
mount=$1
src=$2
out=web/static/seed
rm -rf "${out:?}/$mount" "$out/$mount.json"
mkdir -p "$out/$mount"
cp -R "$src"/. "$out/$mount"/
(
  cd "$out/$mount"
  printf '{"mount":"%s","base":"%s/","files":[' "$mount" "$mount"
  find . -type f | sed 's|^\./||' | LC_ALL=C sort | awk 'NR>1{printf ","} {printf "\"%s\"", $0}'
  printf ']}\n'
) >"$out/$mount.json"
echo "wrote $out/$mount.json ($(find "$out/$mount" -type f | wc -l | tr -d ' ') files)"
