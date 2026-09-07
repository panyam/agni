#!/usr/bin/env bash
# Fetch pinned board corpora from the agni-samples release into tools/samples/.
#
# Usage: hack/fetch_samples.sh <artifact> [<artifact> ...]
#   artifacts: tutorial-board (schematics, one board) | oracle-corpus (both views, every board)
#
# EVERY failure path here exits non-zero. That is the whole design. A test that reads a board which
# quietly failed to download does not fail, it passes over an empty set, and
# docsite/content/build/the-gate.md already catalogues what that costs. There is deliberately no
# offline escape hatch and no skip-if-absent: if the corpus cannot be fetched, the gate is red.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
pin="$root/hack/samples.pin"
dest="$root/tools/samples"
repo=panyam/agni-samples

[ $# -gt 0 ] || { echo "fetch_samples: name at least one artifact" >&2; exit 2; }
[ -f "$pin" ] || { echo "fetch_samples: missing $pin" >&2; exit 1; }

version=$(awk '$1=="VERSION"{print $2}' "$pin")
[ -n "$version" ] || { echo "fetch_samples: no VERSION in $pin" >&2; exit 1; }

# The stamp is a hash of the pin file plus the requested set, so bumping the pin or asking for a
# different artifact re-fetches without anyone having to remember to clean.
stamp_want=$(cat "$pin" <(printf '%s\n' "$@") | shasum -a 256 | cut -d' ' -f1)
stamp_file="$dest/.stamp"
if [ -f "$stamp_file" ] && [ "$(cat "$stamp_file")" = "$stamp_want" ]; then
  exit 0
fi

command -v curl >/dev/null || { echo "fetch_samples: curl not found" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Download and verify EVERY artifact before touching what is already on disk. Doing it the other way
# round means a network blip or a bad checksum leaves the tree with no corpus at all, so a transient
# failure costs the working copy someone already had.
files=()
for name in "$@"; do
  want=$(awk -v n="$name" '$2==n{print $1}' "$pin")
  [ -n "$want" ] || { echo "fetch_samples: '$name' is not pinned in $pin" >&2; exit 1; }

  file="$name-$version.tar.gz"
  url="https://github.com/$repo/releases/download/$version/$file"
  echo "fetch_samples: $file"
  curl -sfL --retry 3 --retry-delay 2 -o "$tmp/$file" "$url" || {
    echo "fetch_samples: download failed: $url" >&2; exit 1; }

  got=$(shasum -a 256 "$tmp/$file" | cut -d' ' -f1)
  if [ "$got" != "$want" ]; then
    echo "fetch_samples: CHECKSUM MISMATCH for $file" >&2
    echo "  pinned:   $want" >&2
    echo "  received: $got" >&2
    echo "  The release artifact changed, or the download was corrupted. Do NOT update the pin to" >&2
    echo "  match without establishing why it moved: a published artifact is meant to be immutable." >&2
    exit 1
  fi
  files+=("$file")
done

# Everything verified. Stage into a sibling and swap, so an extract failure does not leave a
# half-populated tree that looks fetched.
staged="$tmp/staged"
mkdir -p "$staged"
for file in "${files[@]}"; do
  tar -xzf "$tmp/$file" -C "$staged" || { echo "fetch_samples: extract failed: $file" >&2; exit 1; }
done

# A tarball that extracts to nothing is the silent-skip failure wearing a green hat.
if [ -z "$(find "$staged" -name '*.kicad_sch' -print -quit)" ]; then
  echo "fetch_samples: extracted no schematics from ${files[*]}" >&2
  exit 1
fi

rm -rf "$dest"
mkdir -p "$(dirname "$dest")"
mv "$staged" "$dest"
echo "$stamp_want" > "$stamp_file"
