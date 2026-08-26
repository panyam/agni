#!/usr/bin/env bash
# fixture_copies_check.sh — the same fixture file living in two places drifts, and every check in
# this repo passes from either copy while they describe different designs.
#
#   hack/fixture_copies_check.sh          check the tree (exit 1 on drift or an undeclared copy)
#   hack/fixture_copies_check.sh --dump   print the tree's copy groups, for seeding the manifest
#
# It does two things, and the second is the one that keeps working.
#
#   1. Every group declared in the manifest must be byte-identical. This is what catches the drift.
#   2. No copy may exist that the manifest does not declare. This is what stops a new duplicate from
#      quietly joining the population, which is how the population got to seventeen groups without
#      anyone deciding to have one.
#
# It cannot infer which files are MEANT to be identical, and that is not a shortcoming to fix later.
# The tree is full of near-twins that must differ: tjunc.fires against tjunc_labeled and tjunc_dotted,
# rev-a against rev-b, twosheet.fires against hier_root (identical but for the sheet filename each
# names). Nothing in the content separates those from a copy that drifted, so the manifest declares
# intent and this only enforces it.
#
# On drift, fix by hand rather than by regenerating: the check names both paths and does not know
# which one is right. --dump prints to STDOUT and never writes the manifest, because the manifest is
# mostly hand-written reasons and a regenerate would delete them. It also cannot repair a drifted
# group: once two files differ they are no longer a copy, so a dump drops the group entirely.
set -eu
cd "$(dirname "$0")/.."
MANIFEST="hack/fixture_copies.txt"

# Paths whose duplication is already someone else's job, or is not duplication at all.
#
# docsite/static/images/catalog is COPIED from stdlib/**/docs/images by `make catalog-docs`, and
# catalog-docs-check regenerates it and fails on a difference (CONSTRAINTS C27). Declaring those
# eighty-odd files here would be a second, weaker check on a generated artifact.
#
# An examples/ go.sum is identical because the modules are, which is what a lockfile is for.
EXCLUDE='^(docsite/static/images/catalog/|examples/[^/]+/go\.sum$|gen/|site/|web/src/gen/|node_modules/|\.git/)'

# Cross-directory duplicates in the tree, as `hash<TAB>path` sorted by hash. Files under 40 bytes are
# skipped: `{}` and a bare newline collide across the tree and say nothing.
duplicates() {
  # --others --exclude-standard includes files that are new and not yet staged, so a copy added in
  # the working tree fails HERE rather than passing locally and failing in CI once it is committed.
  # That is the commit-first ordering trap build/the-gate.md describes for catalog-docs-check and
  # proto-check, and it costs one flag to not have it. Ignored paths stay out.
  git ls-files -z --cached --others --exclude-standard \
    | tr '\0' '\n' \
    | grep -vE "$EXCLUDE" \
    | tr '\n' '\0' | xargs -0 shasum -a 256 2>/dev/null \
    | sort \
    | awk '{ h = $1; p = substr($0, index($0, "  ") + 2)
             if (h == ph) { g = g " " p } else { emit(); g = p; ph = h } }
           END { emit() }
           function emit(  n, a, i, dirs, d, seen) {
             if (g == "") return
             n = split(g, a, " ")
             if (n < 2) return
             delete dirs; seen = 0
             for (i = 1; i <= n; i++) { d = a[i]; sub(/\/[^\/]*$/, "", d); dirs[d] = 1 }
             for (d in dirs) seen++
             if (seen < 2) return            # same directory: a deliberate pair, not a copy
             print g                         # paths are already path-sorted: sort ran on hash+path
           }' \
    | sort \
    | while IFS= read -r line; do
        # The size floor is applied to the GROUP rather than to every file in the tree, which is
        # thirty stat calls instead of two thousand. `{}` and a bare newline collide across the repo
        # and say nothing: five unrelated .kicad_pro stubs and eleven identical example Makefiles
        # would all arrive here as copies of each other.
        set -- $line
        if [ "$(wc -c < "$1")" -lt 40 ]; then continue; fi
        for p in $line; do printf '%s\n' "$p"; done
        printf '\n'
      done
}

# Groups from the manifest, blank-line separated, `#` lines are the reason for the group.
groups() { grep -v '^[[:space:]]*#' "$MANIFEST" | awk 'NF { print } !NF { print "" }'; }

if [ "${1:-}" = "--dump" ]; then
  duplicates
  exit 0
fi

fail=0
group=""
declared=$(mktemp); trap 'rm -f "$declared"' EXIT

# 1. Declared groups agree.
ngroups=0
while IFS= read -r line; do
  if [ -n "$line" ]; then group="$group $line"; printf '%s\n' "$line" >> "$declared"; continue; fi
  if [ -z "$group" ]; then continue; fi
  ngroups=$((ngroups + 1))
  set -- $group
  first=$1
  if [ ! -f "$first" ]; then
    echo "fixture-copies: declared file is missing: $first"; fail=1; group=""; continue
  fi
  shift
  for f in "$@"; do
    if [ ! -f "$f" ]; then
      echo "fixture-copies: declared file is missing: $f"; fail=1; continue
    fi
    if ! cmp -s "$first" "$f"; then
      echo "fixture-copies: these are declared copies of each other and have DRIFTED:"
      echo "    $first"
      echo "    $f"
      echo "  Decide which is right and copy it over the other. Do not run --dump: it would drop the"
      echo "  group rather than repair it."
      fail=1
    fi
  done
  group=""
done <<EOF
$(groups)

EOF

# 2. Nothing is duplicated that the manifest does not declare. Reported as a whole GROUP, because
# "this file is a copy" is not actionable without the file it is a copy of.
undeclared=$(duplicates | awk -v decl="$declared" '
  BEGIN { while ((getline l < decl) > 0) d[l] = 1 }
  NF { g[++n] = $0; if (!($0 in d)) miss = 1; next }
  { if (n && miss) { for (i = 1; i <= n; i++) print g[i]; print "" } n = 0; miss = 0; delete g }
  END { if (n && miss) { for (i = 1; i <= n; i++) print g[i] } }')
if [ -n "$undeclared" ]; then
  echo "fixture-copies: these files are copies of each other and $MANIFEST does not declare them:"
  printf '%s\n' "$undeclared" | sed 's/^\(.\)/    \1/'
  echo "  Either stop keeping two, or add the group to the manifest WITH A REASON. A copy nobody"
  echo "  declared is one nobody is watching, which is how the last one drifted."
  fail=1
fi

[ "$fail" -eq 0 ] || exit 1
echo "fixture-copies: clean ($ngroups declared group(s))"
