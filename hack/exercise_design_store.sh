#!/usr/bin/env bash
# The design-store mission's exercise (agni issues 1691, 1696). It starts its own agni serve over a
# fresh --store-dir, pushes Jetson 1.1.2 and 1.1.3 as two designs of one series, replaces 1.1.3's
# schematic, restarts the server over the same store, and lists everything again. #1695 and #1704
# add the report and diff steps. It kills only the server it started.
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
pid=

serve() {
  # From $out, so a checkout's unbuilt web/ is not taken for the viewer. The server answers the API.
  (cd "$out" && exec "$agni" serve --store-dir "$store" --addr "127.0.0.1:$port") >"$out/serve.log" 2>&1 &
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
if [ "$(wc -l <"$out/series-after-restart.txt")" -eq 2 ] && grep -q 'bb-1.1.3.*latest' "$out/series-after-restart.txt" &&
  [ -n "$before" ] && [ -n "$after" ] && [ "$before" != "$after" ]; then
  echo "PASS: both designs and the series survived the restart, 1.1.3 is latest, and its schematic holds the replaced files ($before, now $after)"
else
  echo "FAIL: the series after the restart differs; see $out" >&2
  exit 1
fi
