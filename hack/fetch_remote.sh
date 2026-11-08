#!/usr/bin/env bash
# The twin of panyam/agni-samples' hack/fetch_remote.sh. Keep the two the same apart from
# the default paths, so a board fetches the same way in both checkouts (agni issue 1446).
# Fetches the boards remote/remote.pin names from their publishers' own repos:
#   hack/fetch_remote.sh [pin-file] [dest-dir]
#
# These boards come with no licence that grants redistribution, so this repo ships only where to get
# them (PROVENANCE.md, "Boards fetched from upstream"). Each pin line names a file at a fixed commit
# and its sha256:
#
#   <sha256>  <board>  <url>  [unzip]
#
# A download lands in a cache keyed on its sha256, so a board is downloaded once per machine and a
# cached copy is never stale. The cache is $AGNI_SAMPLES_CACHE, else $XDG_CACHE_HOME/agni/samples,
# else ~/.cache/agni/samples. Each file is then copied into <dest>/<board>/ under its own name. With
# `unzip`, the archive is extracted there instead, and so is every zip inside it, each into a folder
# named after it.
#
# Every failure exits non-zero, as hack/verify.sh's reads do. A dead URL or a checksum that does not
# match is never skipped, since a test reading a board that did not arrive passes over nothing.
# hack/clean_remote.sh lists old cache entries and deletes them only when asked.
set -euo pipefail

pin=${1:-tools/samples/remote/remote.pin}
dest=${2:-tools/samples/remote}
cache=${AGNI_SAMPLES_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/agni/samples}

[ -f "$pin" ] || { echo "fetch_remote: no pin file at $pin" >&2; exit 1; }
command -v curl >/dev/null || { echo "fetch_remote: curl not found" >&2; exit 1; }
mkdir -p "$cache" "$dest"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# A copy from the cache is a clone on APFS (cp -c), so it costs no space there. Elsewhere it is a
# plain copy. A symlink would be cheaper still, but a walk that skips symlinked files would then read
# a folder as empty.
clone() { cp -c "$1" "$2" 2>/dev/null || cp "$1" "$2"; }

# Extracts every zip under a folder into a folder named after it, then removes the zip, until none
# is left. The outer archive's checksum covers everything inside it.
unzip_nested() {
  local z
  while z=$(find "$1" -type f -iname '*.zip' -print -quit) && [ -n "$z" ]; do
    unzip -q -o "$z" -d "${z%.*}" || { echo "fetch_remote: cannot extract $z" >&2; exit 1; }
    rm -f "$z"
  done
}

n=0
while read -r sum board url action extra; do
  case $sum in '' | '#'*) continue ;; esac
  n=$((n + 1))
  if [ -n "${extra:-}" ] || [ -z "${url:-}" ] || { [ -n "${action:-}" ] && [ "$action" != unzip ]; }; then
    echo "fetch_remote: $pin: cannot read the line for $board" >&2; exit 1
  fi
  case $board in */* | .* ) echo "fetch_remote: $pin: board name $board must be a plain folder name" >&2; exit 1 ;; esac

  cached="$cache/$sum"
  if [ ! -f "$cached" ]; then
    echo "fetch_remote: $board: $url"
    curl -sSfL --retry 5 --retry-delay 3 --retry-all-errors -o "$tmp/download" "$url" || {
      echo "fetch_remote: download failed: $url" >&2; exit 1; }
    got=$(shasum -a 256 "$tmp/download" | cut -d' ' -f1)
    if [ "$got" != "$sum" ]; then
      echo "fetch_remote: CHECKSUM MISMATCH for $url" >&2
      echo "  pinned:   $sum" >&2
      echo "  received: $got" >&2
      echo "  The upstream file changed, or the download was corrupted. Do NOT update the pin to" >&2
      echo "  match without finding out why: the URL names a fixed commit." >&2
      exit 1
    fi
    mv "$tmp/download" "$cached"
  fi
  # The cache file's time records its last use, which clean_remote.sh reads.
  touch "$cached"

  # A board's folder is filled once per pinned file. The stamp names the file, so a pin that moves
  # fills the folder again.
  stamp="$dest/$board/.fetched-$sum"
  [ -f "$stamp" ] && continue
  mkdir -p "$dest/$board"
  if [ "${action:-}" = unzip ]; then
    staged="$tmp/staged-$n"
    mkdir -p "$staged"
    unzip -q -o "$cached" -d "$staged" || { echo "fetch_remote: cannot extract $url" >&2; exit 1; }
    unzip_nested "$staged"
    cp -R "$staged"/. "$dest/$board"/
  else
    name=$(basename "${url%%\?*}")
    name=$(printf '%b' "${name//%/\\x}")
    clone "$cached" "$dest/$board/$name"
  fi
  touch "$stamp"
done < "$pin"

[ "$n" -gt 0 ] || { echo "fetch_remote: $pin names no files" >&2; exit 1; }
