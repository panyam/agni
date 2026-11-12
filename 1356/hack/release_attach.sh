#!/usr/bin/env bash
# Attaches files to a tag's GitHub Release, creating the release as a draft when none exists yet.
# The release workflow attaches the binaries before or after the release is published by hand
# (RELEASING.md, step 6), so either order ends with one release carrying both. A re-run replaces
# what it attached before.
#   hack/release_attach.sh v0.3.13 dist/release/*
set -euo pipefail
tag=${1:?usage: release_attach.sh <tag> <file>...}
shift
[ $# -gt 0 ] || { echo "release_attach: no files to attach"; exit 1; }
if ! gh release view "$tag" >/dev/null 2>&1; then
  gh release create "$tag" --draft --title "$tag" --notes "Release notes to follow." --verify-tag
fi
gh release upload "$tag" "$@" --clobber
