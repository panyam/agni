#!/usr/bin/env bash
# Fail when an engine-drawn image the docsite embeds disagrees with what the engine draws today.
#
# docsite/images.sh renders them from fixtures in this repo, and they are committed so the docs build
# needs no engine. A reader or renderer change moves them without touching anything a stamp would
# hash, which is how rungs 1 and 3 came to show a drawing the engine no longer made (agni issue 453),
# so this regenerates and compares, as CONSTRAINTS.md C27 asks.
#
# It snapshots and restores rather than reading `git status`, for the reason tutorial_runs_check.sh
# gives, so the tree is left exactly as it was found, pass or fail.
set -euo pipefail

cd "$(dirname "$0")/.."

dirs=(docsite/static/designs docsite/static/images/learn)

tmp=$(mktemp -d)
restore() {
  rm -rf "${dirs[@]}"
  tar xf "$tmp/before.tar" 2>/dev/null || true
  rm -rf "$tmp"
}
tar cf "$tmp/before.tar" "${dirs[@]}"
trap restore EXIT

# A file images.sh no longer writes must show up as a difference, so the directories are emptied
# rather than overwritten in place.
rm -rf "${dirs[@]}"
if ! ./docsite/images.sh >"$tmp/render.log" 2>&1; then
  echo "docsite-images-check: docsite/images.sh failed, so the images could not be verified" >&2
  cat "$tmp/render.log" >&2
  exit 1
fi
tar cf "$tmp/after.tar" "${dirs[@]}"

mkdir -p "$tmp/before" "$tmp/after"
tar xf "$tmp/before.tar" -C "$tmp/before"
tar xf "$tmp/after.tar" -C "$tmp/after"

if diff -r "$tmp/before" "$tmp/after" >/dev/null 2>&1; then
  exit 0
fi
echo "docsite images are stale - run 'make -C docsite images', look at the pictures, and commit them:" >&2
diff -rq "$tmp/before" "$tmp/after" 2>&1 | sed "s#$tmp/before/##; s#$tmp/after/##; s/^/  /" >&2 || true
exit 1
