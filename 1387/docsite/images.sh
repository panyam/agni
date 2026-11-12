#!/bin/sh
# Re-render every engine-drawn image the docsite embeds, from the fixtures in this repo.
#
# A figure exported once drifts from the fixture it claims to show, and nothing says so. These are
# committed so the docs build needs no engine, and `make docsite-images-check` regenerates them in the
# gate and fails on any difference (CONSTRAINTS.md C27, agni issue 453). Regenerate with
# `make -C docsite images` and read the diff, exactly as with `make tutorial-runs`.
#
# Never `--layout force` here. It is the one layout that is not bit-identical across architectures
# (agni issue 472), so the gate would fail on the runner's architecture rather than on any change.
#
# The binary is built fresh into a temp dir, because a stale bin/agni renders yesterday's engine and
# the diff then reads as clean.
set -e
cd "$(dirname "$0")/.."
GO=${GO:-go}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
$GO build -o "$tmp/agni" ./cmd/agni

render() { out=$1; shift; "$tmp/agni" render "$@" -o "$out" 2>&1 | tail -1; }

designs=docsite/static/designs
learn=docsite/static/images/learn
mkdir -p "$designs" "$learn"

# Tutorial rung 3's netlist graph. LAYERED, matching the run on rung 3 rather than beating it, since
# the command and the figure agreeing is worth more than the prettier force layout.
render "$designs/gateway-netlist.svg" examples/tutorial-project/designs/gateway/gateway.edn --layout layered
# The Jetson baseboard rungs 1 and 3 teach from is fetched rather than committed (make
# samples-oracle), so its drawing is committed instead.
jetson=tools/samples/boards/jetson-agx-thor-baseboard
if [ ! -d "$jetson" ]; then
  echo "images.sh: $jetson is missing; run 'make samples-oracle' first" >&2
  exit 1
fi
render "$designs/jetson-power.svg" "$jetson" --sheet Power

# The learn course. The tutorial board is reachable as a DESIGN rather than a file because its
# design.yaml declares the schematic companion (agni PR 433).
render "$learn/gateway.svg" examples/tutorial-project/designs/gateway
# Chapter 2's pair, identical but for one junction dot.
render "$learn/tjunc.svg" readers/kicad/testdata/tjunc.kicad_sch
render "$learn/tjunc-dotted.svg" readers/kicad/testdata/tjunc_dotted.kicad_sch
# Chapter 6, the reversed LED, where seeing the drawing is most of the lesson.
render "$learn/ledpol.svg" cmd/agni/testdata/conformance/ledpol.fires.kicad_sch

# The demos page shows two of the renderer's own goldens, which its tests keep current. They are
# declared copies in hack/fixture_copies.txt, so fixture-copies-check also fails when one diverges.
cp core/render/testdata/golden/board-fixture.svg "$designs/demo-board.svg"
cp core/render/testdata/golden/faithful-sample.svg "$designs/demo-schematic.svg"
echo "copied the demos page's two render goldens"
