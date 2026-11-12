#!/usr/bin/env bash
# The opt-in pre-push hook `make hooks` installs (agni issue 1147). It runs the browser suite, the part
# of the gate a layout break shows up in, before the push leaves the machine. CI runs the same suite on
# every push, so this only moves the answer earlier. Skip it once with `git push --no-verify`, or for
# a while with AGNI_SKIP_PREPUSH=1.
set -euo pipefail

if [ -n "${AGNI_SKIP_PREPUSH:-}" ]; then
	echo "pre-push: AGNI_SKIP_PREPUSH is set, not running the browser suite"
	exit 0
fi

# A push that deletes a branch sends nothing to test.
while read -r _local_ref local_sha _remote_ref _remote_sha; do
	if [ "$local_sha" != "0000000000000000000000000000000000000000" ]; then
		echo "pre-push: running make browser-test (skip with git push --no-verify)"
		exec make -C "$(git rev-parse --show-toplevel)" browser-test
	fi
done
