#!/usr/bin/env bash
# The runtime-mounts exercise (agni issue 1735). It starts its own agni serve over a fresh --store and
# --uploads, makes a GitHub mount over Antmicro's Jetson baseboard at its 1.1.2 revision, browses the
# mount, restarts the server over the same folders, and lists the mount again. Then it checks the
# store holds records and no file, and that a --mount cannot be deleted. It kills only the server it
# started.
#
# It drives the server with `agni mounts` (agni issue 1749), and reads the tree over the API. It reads
# GitHub, so it is run by hand and not in the gate. The Go tests TestServeMakesAGitHubMountOverConnect
# and TestTheMountsCommandAddsListsAndRemovesAGitHubMount run the same steps against a fake GitHub.
set -euo pipefail

agni=${AGNI:-bin/agni}
out=${EXERCISE_OUT:-/tmp/agni-exercise}/mounts
port=${EXERCISE_PORT:-18735}
url=http://127.0.0.1:$port
source=${MOUNT_SOURCE:-github://antmicro/jetson-agx-thor-baseboard}
# Antmicro tags no revisions. 1.1.2 is the last commit before "Bump version on schematic to 1.1.3".
ref=${MOUNT_REF:-8fef9146afba9ce7b08f9331a3a2a128d97784c2}

rm -rf "$out"
mkdir -p "$out/boards"
out=$(cd "$out" && pwd)
agni=$(cd "$(dirname "$agni")" && pwd)/$(basename "$agni")
store=$out/store
uploads=$out/uploads
pid=

serve() {
  (cd "$out" && exec "$agni" serve --store "file://$store" --uploads "file://$uploads" --mount "boards=$out/boards" --addr "127.0.0.1:$port") >>"$out/serve.log" 2>&1 &
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
# rpc calls one WorkspaceService method over Connect's JSON and prints the answer. A non-2xx answer
# fails the exercise unless the caller expects it.
rpc() { curl -sS -X POST -H 'Content-Type: application/json' -d "$2" "$url/agni.v1.webapi.WorkspaceService/$1"; }

serve
step "agni mounts add jetson $source --ref $ref"
"$agni" mounts add jetson "$source" --ref "$ref" --remote "$url" | tee "$out/create.txt"
grep -q "@$ref"$'\t'"runtime$" "$out/create.txt" || { echo "agni mounts add did not snapshot $ref" >&2; exit 1; }

step "browse mount://jetson"
rpc ListDir '{"uri":"mount://jetson"}' | python3 -c 'import json,sys
for e in json.load(sys.stdin).get("entries",[]): print(e["name"], e.get("kind",""))' | tee "$out/list-before.txt"
grep -q 'jetson-agx-thor-baseboard.kicad_sch FILE_KIND_DESIGN' "$out/list-before.txt"
curl -fs -o /dev/null "$url/mounts/jetson/" || echo "(the /mounts/ page needs the viewer's web dir; the API answered)"

step "restart over the same --store and --uploads"
stop
serve
"$agni" mounts ls --remote "$url" | tee "$out/mounts-after.txt"
grep -q "^jetson"$'\t'"github"$'\t'"$source@$ref"$'\t'"runtime$" "$out/mounts-after.txt"
rpc ListDir '{"uri":"mount://jetson"}' | python3 -c 'import json,sys
for e in json.load(sys.stdin).get("entries",[]): print(e["name"], e.get("kind",""))' > "$out/list-after.txt"
diff "$out/list-before.txt" "$out/list-after.txt" && echo "the mount lists the same after the restart"

step "the store holds records and no file"
if find "$store" -type f ! -name '*.yaml' | grep .; then
  echo "the store holds files that are not records" >&2
  exit 1
fi
echo "store: $(find "$store" -type f | wc -l | tr -d ' ') record(s), uploads: $(du -sh "$uploads" | cut -f1)"

step "a --mount cannot be removed, and a runtime one can"
if "$agni" mounts rm boards --remote "$url" 2>"$out/rm-boards.txt"; then
  echo "agni mounts rm boards removed a --mount" >&2
  exit 1
fi
grep -q "comes from the server's flags" "$out/rm-boards.txt" && cat "$out/rm-boards.txt"
"$agni" mounts rm jetson --remote "$url"
if "$agni" mounts ls --remote "$url" | grep -q '^jetson'; then
  echo "jetson is still listed after agni mounts rm" >&2
  exit 1
fi
echo
echo "exercise-mounts: ok"
