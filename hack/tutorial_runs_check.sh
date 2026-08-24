#!/usr/bin/env bash
# Fail when a committed tutorial capture disagrees with what the engine produces today.
#
# The captures under docsite/content/**/runs/*.output are generated and committed, so the docs build
# stays hermetic. Their freshness stamp hashes the SPEC and the FIXTURE, never the engine, so an
# engine change that alters output leaves every stamp valid and an ordinary docsite build regenerates
# nothing. A capture edited by hand keeps its stamp too. Neither is detectable without regenerating.
#
# It restores the tree before exiting, pass or fail, and never leaves a half-regenerated checkout.
# That is deliberate, for the reason proto-check gives in the Makefile: a check that inspects
# `git status` forces a regenerate -> commit -> gate ordering, so freshly regenerated but uncommitted
# output reads as stale. Captures move on any fixture or output change, and the natural loop is to
# regenerate and run the gate before committing.
set -euo pipefail

cd "$(dirname "$0")/.."
GO=${GO:-go}

outputs() { find docsite/content -path '*/runs/*.output' -print0; }

tmp=$(mktemp -d)
# Restore on ANY exit path, including the build failing or the operator pressing Ctrl-C. A checkout
# missing every capture is a worse state than the one this was asked to report on.
restore() {
  outputs | xargs -0 rm -f 2>/dev/null || true
  tar xf "$tmp/before.tar" 2>/dev/null || true
  rm -rf "$tmp"
}
trap restore EXIT

outputs | xargs -0 tar cf "$tmp/before.tar"

outputs | xargs -0 rm -f
if ! (cd docsite && $GO run . -build >/dev/null 2>&1); then
  echo "tutorial-runs-check: the docsite build failed, so captures could not be verified" >&2
  exit 1
fi
outputs | xargs -0 tar cf "$tmp/after.tar"

mkdir -p "$tmp/before" "$tmp/after"
tar xf "$tmp/before.tar" -C "$tmp/before"
tar xf "$tmp/after.tar" -C "$tmp/after"

if diff -r "$tmp/before" "$tmp/after" >/dev/null 2>&1; then
  exit 0
fi
echo "tutorial captures are stale - run 'make tutorial-runs' and commit the result:" >&2
diff -rq "$tmp/before" "$tmp/after" 2>&1 | sed "s#$tmp/before/##; s#$tmp/after/##; s/^/  /" >&2
exit 1
