#!/usr/bin/env bash
# The twin of panyam/agni-samples' hack/fetch_remote_test.sh. Keep the two the same apart from
# the default paths, so a board fetches the same way in both checkouts (agni issue 1446).
# Checks hack/fetch_remote.sh and hack/clean_remote.sh against a local upstream served by file://
# URLs, so it needs no network.
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
fetch="$here/fetch_remote.sh"
clean="$here/clean_remote.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failures=0

ok() { echo "ok   $1"; }
fail() { echo "FAIL $1"; failures=$((failures + 1)); }

# An upstream with a plain file and a zip that holds another zip.
up="$work/upstream"
mkdir -p "$up/inner"
printf 'board bytes\n' > "$up/gadget.brd"
printf 'drill\n' > "$up/inner/drill.txt"
(cd "$up/inner" && zip -q ../films.zip drill.txt)
(cd "$up" && zip -q mfg.zip films.zip gadget.brd)
sum() { shasum -a 256 "$1" | cut -d' ' -f1; }

pin="$work/remote.pin"
printf '# a comment\n%s  gadget  file://%s/gadget.brd\n%s  widget  file://%s/mfg.zip  unzip\n' \
  "$(sum "$up/gadget.brd")" "$up" "$(sum "$up/mfg.zip")" "$up" > "$pin"
export AGNI_SAMPLES_CACHE="$work/cache"

dest="$work/remote"
if "$fetch" "$pin" "$dest" >/dev/null 2>&1 && [ -f "$dest/gadget/gadget.brd" ] \
   && [ -f "$dest/widget/gadget.brd" ] && [ -f "$dest/widget/films/drill.txt" ] && [ ! -e "$dest/widget/films.zip" ]; then
  ok "a plain file is copied, and a zip is extracted with the zip inside it"
else
  fail "the first fetch did not lay out both boards"
fi

# With the upstream gone, a second fetch into a fresh folder still succeeds from the cache.
mv "$up" "$work/upstream-gone"
dest2="$work/remote2"
if "$fetch" "$pin" "$dest2" >/dev/null 2>&1 && [ -f "$dest2/gadget/gadget.brd" ] && [ -f "$dest2/widget/films/drill.txt" ]; then
  ok "a cached file is not downloaded again"
else
  fail "a fetch with the upstream gone failed, so it did not read the cache"
fi
mv "$work/upstream-gone" "$up"

bad="$work/bad.pin"
printf '%s  gadget  file://%s/gadget.brd\n' "$(printf '0%.0s' {1..64})" "$up" > "$bad"
out=$("$fetch" "$bad" "$work/remote3" 2>&1)
if [ $? -ne 0 ] && printf '%s' "$out" | grep -q 'CHECKSUM MISMATCH'; then
  ok "a checksum that does not match fails"
else
  fail "a wrong checksum did not fail the fetch"
fi

if ! "$fetch" "$work/none.pin" "$work/remote4" >/dev/null 2>&1; then
  ok "a missing pin file fails"
else
  fail "a missing pin file passed"
fi

# An entry unused for 40 days is listed. Without a terminal nothing is deleted.
old=$(sum "$up/gadget.brd")
touch -t 202001010000 "$AGNI_SAMPLES_CACHE/$old"
out=$("$clean" < /dev/null 2>&1)
if printf '%s' "$out" | grep -q "$old" && [ -f "$AGNI_SAMPLES_CACHE/$old" ] && printf '%s' "$out" | grep -q 'nothing was deleted'; then
  ok "an old cache entry is listed and kept when nobody answers"
else
  fail "clean_remote without a terminal did not list and keep the old entry"
  printf '%s\n' "$out" | sed 's/^/    /'
fi
if ! printf '%s' "$out" | grep -q "$(sum "$up/mfg.zip")"; then
  ok "a recently used cache entry is not listed"
else
  fail "clean_remote listed an entry used today"
fi

# A deleted cache entry is downloaded again by the next fetch.
rm -f "$AGNI_SAMPLES_CACHE/$old"
if "$fetch" "$pin" "$work/remote5" >/dev/null 2>&1 && [ -f "$AGNI_SAMPLES_CACHE/$old" ]; then
  ok "a deleted cache entry is fetched again"
else
  fail "a deleted cache entry did not come back"
fi

[ $failures -eq 0 ] || { echo "$failures case(s) failed"; exit 1; }
