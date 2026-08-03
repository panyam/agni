#!/usr/bin/env bash
# ir_model_check.sh — ratchet for CONSTRAINT C19 (WS1-042): engine processing reads a design through
# check.Model, not by scanning a raw *ir.Design. It flags any `func ... *ir.Design ...` outside the
# allowed producer/entry paths that is not grandfathered in the baseline, so NEW raw-design sites
# fail while existing ones migrate opportunistically (each removal ratchets the baseline down).
#
#   hack/ir_model_check.sh          check the tree (exit 1 on a new, unlisted site)
#   hack/ir_model_check.sh --dump   regenerate the baseline from the current tree
#
# Keys are `<dir>:<func>`. Heuristic: it matches a `*ir.Design` on the func-declaration line, so a
# signature that wraps *ir.Design onto a continuation line is not seen (rare; add such a site to the
# baseline by hand if it ever appears).
set -eu
cd "$(dirname "$0")/.."
BASELINE="hack/ir_model_baseline.txt"

# Directories where a raw *ir.Design is legitimate — it is PRODUCED, CONSTRUCTED, or is a
# top-level ANALYSIS/TRANSFORM that takes designs as its input (not a helper handed the design to
# scan). Never scanned. Readers (edif..geda) + formats + netgraph produce IR; graph/diff/validate
# are top-level netlist consumers that use no Model index; examples are demos. See C19.
EXCLUDE='^(gen|readers|internal/netgraph|examples|graph|diff|validate)/'

current() {
  grep -rEn 'func [^{]*\*ir\.Design' --include='*.go' . 2>/dev/null | sed -E 's#^\./##' \
    | grep -vE '_test\.go:' | while IFS= read -r line; do
      file=${line%%:*}
      dir=$(dirname "$file")
      case "$dir/" in
        */) ;;
      esac
      if printf '%s/' "$dir" | grep -qE "$EXCLUDE"; then continue; fi
      fn=$(printf '%s\n' "$line" | sed -E 's/.*func (\([^)]*\) )?([A-Za-z_][A-Za-z0-9_]*)\(.*/\2/')
      printf '%s:%s\n' "$dir" "$fn"
    done | sort -u
}

if [ "${1:-}" = "--dump" ]; then
  current > "$BASELINE"
  echo "wrote $(grep -c . "$BASELINE") entries to $BASELINE"
  exit 0
fi

cur=$(current)
base=$(sort -u "$BASELINE")
new=$(comm -23 <(printf '%s\n' "$cur") <(printf '%s\n' "$base") | grep -c . || true)
newlist=$(comm -23 <(printf '%s\n' "$cur") <(printf '%s\n' "$base") || true)
stale=$(comm -13 <(printf '%s\n' "$cur") <(printf '%s\n' "$base") || true)

if [ -n "$stale" ]; then
  echo "note: baseline entries no longer in the tree (migrated? prune from $BASELINE):"
  printf '%s\n' "$stale" | sed 's/^/  /'
fi
if [ "$new" -gt 0 ]; then
  echo "C19 violation: new raw *ir.Design parameter(s) outside allowed paths — read via check.Model:"
  printf '%s\n' "$newlist" | sed 's/^/  /'
  echo "(if this is a sanctioned producer/entry site, add it to $BASELINE and justify in review.)"
  exit 1
fi
echo "ir-model-check: clean ($(printf '%s\n' "$cur" | grep -c . ) grandfathered sites)"
