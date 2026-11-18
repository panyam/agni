#!/usr/bin/env bash
# The design-store mission's exercise (agni issues 1691, 1696, 1739). It starts its own agni serve
# over a fresh --store and --uploads in separate folders, pushes Jetson 1.1.2 and 1.1.3 as two designs of one series, replaces 1.1.3's
# schematic, restarts the server over the same store, and lists everything again. Then it checks and
# diffs the stored designs by name (agni issue 1695) and compares each answer with the same command on
# the folders, and checks the store's folder holds records and no board file. #1704 adds packs. It
# kills only the server it started.
set -euo pipefail

agni=${AGNI:-bin/agni}
revs=${SAMPLES_REVISIONS:-tools/samples/revisions}
out=${EXERCISE_OUT:-/tmp/agni-exercise}/design-store
port=${EXERCISE_PORT:-18691}
url=http://127.0.0.1:$port

for r in 1.1.2 1.1.3; do
  [ -d "$revs/jetson-agx-thor-baseboard-$r" ] || { echo "missing $revs/jetson-agx-thor-baseboard-$r; run make samples-revisions" >&2; exit 1; }
done
rm -rf "$out"
mkdir -p "$out"
out=$(cd "$out" && pwd)
agni=$(cd "$(dirname "$agni")" && pwd)/$(basename "$agni")
store=$out/store
uploads=$out/uploads
pid=

serve() {
  # From $out, so a checkout's unbuilt web/ is not taken for the viewer. The server answers the API.
  (cd "$out" && exec "$agni" serve --store "file://$store" --uploads "file://$uploads" --addr "127.0.0.1:$port") >"$out/serve.log" 2>&1 &
  pid=$!
  for _ in $(seq 1 100); do
    curl -fs -o /dev/null "$url/" && return 0
    sleep 0.1
  done
  echo "agni serve did not answer at $url; see $out/serve.log" >&2
  exit 1
}
stop() { [ -n "$pid" ] && kill "$pid" 2>/dev/null && wait "$pid" 2>/dev/null; pid=; }
trap stop EXIT

step() { printf '\n== %s\n' "$*"; }

serve
step "push 1.1.2 and 1.1.3 as two designs of the series jetson"
"$agni" designs push "$revs/jetson-agx-thor-baseboard-1.1.2" --id bb-1.1.2 --series jetson --remote "$url"
"$agni" designs push "$revs/jetson-agx-thor-baseboard-1.1.3" --id bb-1.1.3 --series jetson --remote "$url"

step "list the series"
"$agni" designs list --series jetson --remote "$url" | tee "$out/series-before.txt"

"$agni" designs artifacts bb-1.1.3 --remote "$url" | grep /schematic | tee "$out/schematic-before.txt"

# findings counts a check's findings, and changes lists a diff's changed parts and nets, from JSON.
findings() { python3 -c 'import json,sys; print(len(json.load(sys.stdin)["findings"]))'; }
changes() { python3 -c 'import json,sys
d=json.load(sys.stdin)
for k in ("componentStatus","netStatus"):
    for n,v in sorted(d.get(k,{}).items()):
        if v!="equal": print(k,n,v)'; }

step "check bb-1.1.3 by name, with the folder's intent, against agni check on the folder"
f3=$revs/jetson-agx-thor-baseboard-1.1.3
{ echo "name: jetson-agx-thor-baseboard-1.1.3"; awk '/^intent:/ { found=1 } found' "$f3/design.yaml"; } >"$out/intent-1.1.3.yaml"
stored=$("$agni" check designs/bb-1.1.3 --intent-path "$out/intent-1.1.3.yaml" --remote "$url" --format json | findings)
folder=$("$agni" check "$f3" --format json | findings)
echo "stored: $stored findings; folder: $folder findings"

step "diff 1.1.2 and 1.1.3 by name against agni diff on the folders"
"$agni" diff designs/bb-1.1.2 designs/bb-1.1.3 --remote "$url" --format json | changes >"$out/diff-stored.txt"
"$agni" diff "$revs/jetson-agx-thor-baseboard-1.1.2" "$f3" --format json | changes >"$out/diff-folder.txt"
echo "stored: $(wc -l <"$out/diff-stored.txt" | tr -d " ") changes; folder: $(wc -l <"$out/diff-folder.txt" | tr -d " ") changes"

step "replace 1.1.3's schematic, every sheet in one write"
edited=$out/schematic-1.1.3
mkdir -p "$edited"
for f in "$revs/jetson-agx-thor-baseboard-1.1.3"/*.kicad_*; do
  case $f in *.kicad_pcb) ;; *) cp "$f" "$edited/" ;; esac
done
printf '\n' >>"$edited/power.kicad_sch"
"$agni" designs put bb-1.1.3 schematic "$edited" --remote "$url"
"$agni" designs get bb-1.1.3 --remote "$url" | tee "$out/bb-1.1.3-after-put.txt"

step "restart the server over the same store"
stop
serve
"$agni" designs list --series jetson --remote "$url" | tee "$out/series-after-restart.txt"
"$agni" designs artifacts bb-1.1.3 --remote "$url" | tee "$out/artifacts-after-restart.txt"

step "result"
before=$(grep -o 'content [0-9a-f]*' "$out/schematic-before.txt")
after=$(grep /schematic "$out/artifacts-after-restart.txt" | grep -o 'content [0-9a-f]*')
fail=0
if [ "$(wc -l <"$out/series-after-restart.txt")" -eq 2 ] && grep -q 'bb-1.1.3.*latest' "$out/series-after-restart.txt" &&
  [ -n "$before" ] && [ -n "$after" ] && [ "$before" != "$after" ]; then
  echo "PASS: both designs and the series survived the restart, 1.1.3 is latest, and its schematic holds the replaced files ($before, now $after)"
else
  echo "FAIL: the series after the restart differs; see $out" >&2
  fail=1
fi
if [ "$stored" -gt 0 ] && [ "$stored" = "$folder" ]; then
  echo "PASS: a check on designs/bb-1.1.3 gives the folder's $folder findings"
else
  echo "FAIL: a check on designs/bb-1.1.3 gives $stored findings, and the folder $folder" >&2
  fail=1
fi
if [ -s "$out/diff-stored.txt" ] && cmp -s "$out/diff-stored.txt" "$out/diff-folder.txt"; then
  echo "PASS: the diff by name lists the folders' $(wc -l <"$out/diff-folder.txt" | tr -d ' ') changes"
else
  echo "FAIL: the diff by name and the folders' diff differ; see $out/diff-stored.txt and $out/diff-folder.txt" >&2
  fail=1
fi
# The store keeps records alone. Any file there that is not a YAML or JSON record is a design's
# bytes in the wrong place.
strays=$(find "$store" -type f ! -name '*.yaml' ! -name '*.json' | wc -l | tr -d ' ')
records=$(find "$store/designs" -name '*.yaml' | wc -l | tr -d ' ')
blobs=$(find "$uploads/blobs" -type f | wc -l | tr -d ' ')
if [ "$strays" -eq 0 ] && [ "$records" -eq 2 ] && [ "$blobs" -gt 0 ]; then
  echo "PASS: the store holds $records design records and no other file, and the uploads hold $blobs files by hash"
else
  echo "FAIL: the store holds $strays files that are not records and $records design records, and the uploads $blobs files; see $store" >&2
  fail=1
fi
exit $fail
