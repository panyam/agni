#!/usr/bin/env bash
# Publish the docs site by running the docs workflow on main, then follow that run to its end.
# Pages serves only that workflow's artifact, so there is no local publish (`make docs-deploy`).
#
# `gh workflow run` prints no run id, so the run is found as the newest dispatch on main created
# after the request, waiting a little for it to be listed.
set -euo pipefail

since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
gh workflow run docs.yml --ref main

for _ in $(seq 1 30); do
  id=$(gh run list --workflow docs.yml --branch main --event workflow_dispatch --limit 5 \
    --json databaseId,createdAt --jq "[.[] | select(.createdAt >= \"$since\")][0].databaseId // empty")
  if [[ -n "$id" ]]; then
    gh run watch "$id" --exit-status
    exit
  fi
  sleep 2
done
echo "docs-deploy: dispatched, but no docs.yml run on main appeared after $since" >&2
exit 1
