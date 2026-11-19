#!/usr/bin/env bash
# The linked-sources mission's exercise (agni issues 1722, 1723). It starts its own agni serve over a
# fresh --store and --uploads, mounts Antmicro's Jetson baseboard repo from GitHub, and links a
# design's schematic and board to the repo at the 1.1.2 commit. It checks the design by name and
# compares the count with agni check on the same files from the samples. Then it moves both links to
# 1.1.3, checks again, and counts the files the move downloaded against the files that differ. A sync
# of the pinned links must fetch nothing, and the mount must refuse removal while the links point in.
# Last, it restarts the server over the same folders and reads everything again. It kills only the
# server it started.
#
# It reads GitHub, so it is run by hand and not in the gate. The Go tests in
# cmd/agni/linkreaders_test.go run the same steps against a fake GitHub.
set -euo pipefail

agni=${AGNI:-bin/agni}
revs=${SAMPLES_REVISIONS:-tools/samples/revisions}
out=${EXERCISE_OUT:-/tmp/agni-exercise}/linked-sources
port=${EXERCISE_PORT:-18722}
url=http://127.0.0.1:$port
source=github://antmicro/jetson-agx-thor-baseboard
# Antmicro tags no revisions. agni-samples' PROVENANCE.md names the commits its 1.1.2 and 1.1.3
# folders were taken from, byte for byte.
v112=3411086e6d6ad2384d0609c53cd6b96323f97c24
v113=8e087f4f946a925041380699dcde51ae8519f621
board=jetson-agx-thor-baseboard

for r in 1.1.2 1.1.3; do
  [ -d "$revs/$board-$r" ] || { echo "missing $revs/$board-$r; run make samples-revisions" >&2; exit 1; }
done
rm -rf "$out"
mkdir -p "$out"
out=$(cd "$out" && pwd)
revs=$(cd "$revs" && pwd)
agni=$(cd "$(dirname "$agni")" && pwd)/$(basename "$agni")
store=$out/store
uploads=$out/uploads
pid=

serve() {
  (cd "$out" && exec "$agni" serve --store "file://$store" --uploads "file://$uploads" --addr "127.0.0.1:$port") >>"$out/serve.log" 2>&1 &
  pid=$!
  for _ in $(seq 1 100); do
    curl -fs -o /dev/null "$url/healthz" && return 0
    sleep 0.1
  done
  echo "agni serve did not answer at $url; see $out/serve.log" >&2
  exit 1
}
stop() { [ -n "$pid" ] && kill "$pid" 2>/dev/null && wait "$pid" 2>/dev/null; pid=; }
trap stop EXIT

step() { printf '\n== %s\n' "$*"; }
findings() { python3 -c 'import json,sys; print(len(json.load(sys.stdin)["findings"]))'; }
# Each samples revision is copied without the design.yaml agni-samples adds, so the copy holds what
# the repo holds at that commit.
for r in 1.1.2 1.1.3; do
  mkdir -p "$out/folder-$r"
  for f in "$revs/$board-$r"/*; do
    case $(basename "$f") in design.yaml) ;; *) cp "$f" "$out/folder-$r/" ;; esac
  done
done
downloaded() { find "$uploads/blobs/aliases/git" -type f ! -name '.*' | wc -l | tr -d ' '; }
fail=0
check() {
  local rev=$1 stored local_
  stored=$("$agni" check designs/bb --remote "$url" --format json | findings)
  local_=$("$agni" check "$out/folder-$rev" --format json 2>/dev/null | findings)
  echo "designs/bb: $stored findings; the $rev files: $local_ findings"
  if [ "$stored" -gt 0 ] && [ "$stored" = "$local_" ]; then
    echo "PASS: a check on designs/bb at $rev gives the folder's $local_ findings"
  else
    echo "FAIL: a check on designs/bb at $rev differs from the folder" >&2
    fail=1
  fi
}

serve
step "agni mounts add jetson $source"
"$agni" mounts add jetson "$source" --ref "$v112" --remote "$url"

step "link designs/bb's schematic and board to the repo at 1.1.2"
"$agni" designs create bb --remote "$url"
"$agni" designs link bb schematic "jetson/$board.kicad_sch@$v112" --remote "$url"
"$agni" designs link bb board "jetson/$board.kicad_pcb@$v112" --remote "$url"
at112=$(downloaded)
echo "downloaded $at112 files"
check 1.1.2

step "move both links to 1.1.3"
"$agni" designs link bb schematic "jetson/$board.kicad_sch@$v113" --remote "$url"
"$agni" designs link bb board "jetson/$board.kicad_pcb@$v113" --remote "$url"
moved=$(($(downloaded) - at112))
differ=0
for f in "$out/folder-1.1.2"/*.kicad_*; do
  n=$out/folder-1.1.3/$(basename "$f")
  [ -f "$n" ] && ! cmp -s "$f" "$n" && differ=$((differ + 1))
done
for f in "$out/folder-1.1.3"/*.kicad_*; do
  [ -f "$out/folder-1.1.2/$(basename "$f")" ] || differ=$((differ + 1))
done
echo "the move downloaded $moved files; $differ design files differ between 1.1.2 and 1.1.3"
check 1.1.3
if [ "$moved" = "$differ" ]; then
  echo "PASS: moving the links downloaded only the $differ files that changed"
else
  echo "FAIL: moving the links downloaded $moved files, and $differ changed" >&2
  fail=1
fi

step "sync the pinned links"
"$agni" designs sync bb --remote "$url" | tee "$out/sync.txt"
if [ "$(grep -c 'unchanged.*fetched 0 files' "$out/sync.txt")" = 2 ]; then
  echo "PASS: a sync of the pinned links fetched nothing"
else
  echo "FAIL: a sync of the pinned links changed or fetched something" >&2
  fail=1
fi

step "the mount refuses removal while the links point in"
if "$agni" mounts rm jetson --remote "$url" 2>"$out/rm.txt"; then
  echo "FAIL: the mount was removed with designs/bb linked to it" >&2
  fail=1
else
  cat "$out/rm.txt"
  echo "PASS: the mount refused removal"
fi

step "restart the server over the same store"
"$agni" designs artifacts bb --remote "$url" >"$out/artifacts-before.txt"
stop
serve
"$agni" designs artifacts bb --remote "$url" | tee "$out/artifacts-after.txt"
if cmp -s "$out/artifacts-before.txt" "$out/artifacts-after.txt" && grep -q "@$v113, pinned" "$out/artifacts-after.txt"; then
  echo "PASS: both links read back at 1.1.3 after the restart"
else
  echo "FAIL: the artifacts differ after the restart; see $out" >&2
  fail=1
fi
check 1.1.3

step "result"
[ "$fail" = 0 ] && echo "exercise-linked-sources: ok"
exit "$fail"
