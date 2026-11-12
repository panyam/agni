#!/usr/bin/env bash
# govulncheck over each module agni ships. `govulncheck ./...` stays inside one module, so the root
# scan alone never reaches datasheet. Every module runs even after one fails, so one advisory does not
# hide the rest, and the summary names them all.
#
# A clean run is reachability-based: it exits 0 while an advisory sits in a required module whose
# vulnerable code agni never calls.
set -uo pipefail
cd "$(dirname "$0")/.."
tool=${GOVULNCHECK:?set GOVULNCHECK to the pinned module@version}
failed=""
for m in ${MODULES:-. datasheet}; do
  echo "==> govulncheck $m"
  (cd "$m" && go run "$tool" ./...) || failed="$failed $m"
done
if [ -n "$failed" ]; then
  echo "govulncheck failed in:$failed"
  exit 1
fi
echo "govulncheck clean in every shipped module"
