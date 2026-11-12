#!/usr/bin/env bash
# Fail when a committed go.mod or go.sum disagrees with what `go mod tidy` produces today.
#
# WHY THIS NEEDED A CHECK. The gate BUILDS every module and never asked whether any of them was
# tidy, and the two are different questions: an untidy module keeps building until some later change
# happens to need a requirement it never recorded. Eleven of the example modules had drifted this way
# before anyone looked, and what surfaced it was unrelated (promoting one root dependency to direct
# made `examples-test` fail with "updates to go.mod needed", and tidying to fix that swept up years of
# accumulated drift in the same commit). The drift itself was invisible for as long as nobody added a
# dependency.
#
# The examples are their own modules ON PURPOSE (C10, so demokit and its terminal-UI deps stay out of
# the engine's go.mod), which is exactly what makes this drift possible: each one carries its own
# resolved graph, and nothing recomputes them together.
#
# It restores the tree before exiting, pass or fail, rather than inspecting `git status` the way
# catalog-docs-check does. Same reasoning tutorial-runs-check gives: a git-status check forces a
# regenerate -> COMMIT -> gate ordering, so freshly tidied but uncommitted files read as stale. A
# dependency change is precisely when someone runs `make tidyall` and then the gate before
# committing, so the git-status shape would fail on the correct workflow.
set -euo pipefail

cd "$(dirname "$0")/.."
GO=${GO:-go}

# Every module in the repo: the root plus each example. Found rather than listed, so a module added
# later is covered without a second edit (the failure mode this whole check exists to close).
# NUL-delimited throughout, and no `sed -z` or `cp --parents`: both are GNU-only and neither is on
# macOS, which is the trap tutorial_runs_check.sh already records.
manifests() { find . -name go.mod -not -path './.git/*' -print0; }
files() {
  manifests | while IFS= read -r -d '' m; do
    m=${m#./}
    printf '%s\0' "$m"
    if [ -f "${m%go.mod}go.sum" ]; then printf '%s\0' "${m%go.mod}go.sum"; fi
  done
}

tmp=$(mktemp -d)
# Restore on ANY exit path, the build failing and Ctrl-C included. A checkout with half its manifests
# rewritten is a worse state than the one this was asked to report on. Files tidy CREATED (a go.sum
# where a module had none) are removed as well, since extracting the snapshot cannot undo an addition.
restore() {
  if [ -f "$tmp/after.list" ]; then
    comm -13 "$tmp/before.list" "$tmp/after.list" | while IFS= read -r f; do rm -f "$f"; done
  fi
  tar xf "$tmp/before.tar" 2>/dev/null || true
  rm -rf "$tmp"
}
trap restore EXIT

files | xargs -0 tar cf "$tmp/before.tar"
files | xargs -0 -n1 echo | sort > "$tmp/before.list"

# Tidy every module. Errors are kept rather than discarded: a module that cannot resolve its graph is
# a real failure and it must not read as "tidy".
if ! (
  manifests | while IFS= read -r -d '' m; do
    (cd "$(dirname "$m")" && $GO mod tidy) || exit 1
  done
) >"$tmp/tidy.out" 2>&1; then
  echo "tidy-check: 'go mod tidy' failed, so tidiness could not be verified" >&2
  cat "$tmp/tidy.out" >&2
  exit 1
fi

files | xargs -0 tar cf "$tmp/after.tar"
files | xargs -0 -n1 echo | sort > "$tmp/after.list"

# Compare the SNAPSHOT against the tidied result, never the working tree against git. Reaching for
# `git diff` here is wrong twice over, and both ways were caught by deliberately drifting a manifest
# and watching this pass. It misses the drift whenever tidy happens to restore the file to what HEAD
# already had, and it reports every unrelated uncommitted edit as untidiness, which is the
# regenerate -> commit -> gate ordering this shape exists to avoid.
mkdir -p "$tmp/before" "$tmp/after"
tar xf "$tmp/before.tar" -C "$tmp/before"
tar xf "$tmp/after.tar" -C "$tmp/after"

if diff -r "$tmp/before" "$tmp/after" >/dev/null 2>&1; then
  exit 0
fi
echo "module manifests are stale - run 'make tidyall' and commit the result:" >&2
# `|| true` because diff exits non-zero here by definition, and set -e with pipefail would end the
# script on it before the exit below is reached.
diff -rq "$tmp/before" "$tmp/after" 2>&1 | sed "s#$tmp/before/##; s#$tmp/after/##; s/^/  /" >&2 || true
exit 1
