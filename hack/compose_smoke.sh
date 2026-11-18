#!/usr/bin/env bash
# compose_smoke.sh brings up deploy/compose.yaml over the tutorial project and checks that a customer
# following the trial guide gets a working viewer, a review store that keeps a run, and a trial report
# that holds nothing from the design (agni issue 1057), and that a design uploaded to the store comes
# back after the container is recreated (agni issue 1698). The release workflow runs it against the
# published images, so the compose.yaml attached to a release has been brought up once.
#
#   hack/compose_smoke.sh <image> <tag>       e.g. agni compose-test, or ghcr.io/panyam/agni v0.2.4
#
# CONTAINER selects the engine (docker, the default, or podman).
set -euo pipefail

image=${1:?usage: compose_smoke.sh <image> <tag>}
tag=${2:?usage: compose_smoke.sh <image> <tag>}
container=${CONTAINER:-docker}
root=$(cd "$(dirname "$0")/.." && pwd)
port=${SMOKE_PORT:-18080}
project=agni-smoke-$$
work=$(mktemp -d)

export AGNI_IMAGE=$image AGNI_VERSION=$tag AGNI_PORT=$port AGNI_DESIGNS=$root/examples/tutorial-project
compose() { "$container" compose -f "$root/deploy/compose.yaml" -p "$project" "$@"; }
cleanup() { compose down -v >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT

fail() { echo "compose smoke: $*" >&2; exit 1; }

up() {
  compose up -d agni >/dev/null
  for _ in $(seq 1 60); do
    curl -fsS "http://127.0.0.1:$port/" >/dev/null 2>&1 && break
    sleep 1
  done
  curl -fsS "http://127.0.0.1:$port/" >/dev/null || fail "the viewer did not answer on :$port"
}
up

rpc() { curl -fsS -H 'Content-Type: application/json' -d "$2" "http://127.0.0.1:$port/agni.v1.webapi.$1"; }
design=mount://designs/designs/gateway
# Every question names a snapshot of the design, ingested once (agni 1061).
snapshot=$(rpc SnapshotService/Ingest "{\"snapshot\":{\"uri\":\"$design\"}}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["name"])') \
  || fail "Ingest did not take the mounted design"
rpc DesignViewService/GetSummary "{\"snapshot\":\"$snapshot\"}" | grep -q '"componentCount"' || fail "GetSummary did not read the mounted design"

# A run the store keeps proves the reviews volume is writable by the image's user.
manifest=$(rpc ReviewService/ListChecklists "{\"snapshot\":\"$snapshot\"}" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["checklists"][0]["manifest"]))')
rpc ReviewService/CreateReview "{\"snapshot\":\"$snapshot\",\"manifest\":$manifest}" | grep -Eq '"name": ?"reviews/' \
  || fail "CreateReview did not store a run, so the reviews volume is not writable"

# A design pushed to the store comes back after the container is recreated, which only the data
# volume survives. The image's own CLI pushes it, over the compose network, so the host needs no agni.
# The mounted gateway already holds the id "gateway", so the stored copy takes another.
count() { rpc DesignViewService/GetSummary "{\"snapshot\":\"$1\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["componentCount"])'; }
mounted=$(count "$snapshot")
compose run --rm -T --entrypoint agni trial designs push /designs/designs/gateway --id gateway-kept --remote http://agni:8080 >"$work/push.out" 2>&1 \
  || { cat "$work/push.out" >&2; fail "agni designs push did not store the design, so the store volume is not writable"; }
compose down >/dev/null 2>&1
up
rpc DesignService/ListDesigns '{}' | grep -q '"designs/gateway-kept"' || fail "the stored design did not come back after the container was recreated"
kept=$(rpc SnapshotService/Ingest '{"snapshot":{"design":"designs/gateway-kept"}}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["name"])') \
  || fail "Ingest did not read the stored design by name"
[ "$(count "$kept")" = "$mounted" ] || fail "the stored design has $(count "$kept") components, and the mounted folder $mounted"

compose run --rm trial /designs/designs/gateway >"$work/report.md" 2>"$work/trial.err" || { cat "$work/trial.err" >&2; fail "the trial did not run"; }
grep -q '| run every rule | ok |' "$work/report.md" || { cat "$work/report.md" >&2; fail "the trial report has no ok rule step"; }
grep -q 'github.com/panyam/agni/issues/new' "$work/trial.err" || fail "the trial printed no issue link"
if grep -Eqi '(^|[^a-z])(gateway|sample board|i2c_scl|pmic_main)' "$work/report.md"; then
  cat "$work/report.md" >&2
  fail "the trial report names something from the design"
fi
echo "compose smoke: ok ($image:$tag via $container compose)"
