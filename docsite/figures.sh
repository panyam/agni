#!/bin/sh
# Re-render the schematics the learn course embeds.
#
# Same reasoning as the generated command captures: a figure hand-exported once drifts from the
# fixture it claims to show, and nothing says so. Regenerate with `make -C docsite figures` and read
# the diff, exactly as with `make tutorial-runs`.
#
# Deliberately NOT in the gate. A render depends on the engine build, so a code change would
# invalidate every figure on every branch and turn `testall` into a rendering test.
set -e
cd "$(dirname "$0")/.."
out=docsite/static/images/learn
mkdir -p "$out"
render() { ./bin/agni render "$1" -o "$out/$2" 2>&1 | tail -1; }

# The tutorial board, which every query in chapter 1 runs against. Reachable as a DESIGN rather than
# a file because its design.yaml declares the schematic companion (agni PR 433).
render examples/tutorial-project/designs/gateway gateway.svg
# Chapter 2's pair: identical but for one junction dot.
render readers/kicad/testdata/tjunc.kicad_sch tjunc.svg
render readers/kicad/testdata/tjunc_dotted.kicad_sch tjunc-dotted.svg
# Chapter 6: the reversed LED, where seeing the drawing is most of the lesson.
render cmd/agni/testdata/conformance/ledpol.fires.kicad_sch ledpol.svg
