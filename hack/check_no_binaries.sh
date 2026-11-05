#!/bin/sh
# Fails if any tracked file is a compiled executable or larger than 5 MB: hack/check_no_binaries.sh [root].
#
# A bare `go build` in an example's folder writes an executable named after the folder, with no
# extension, and `.gitignore` names each example's binary one by one. examples/orcad-design's 17 MB
# binary reached main that way. So this reads each file's first four bytes rather than its name. It
# scans the whole tracked tree, since a diff-only scan would let one that slipped in earlier stay.
# mcpkit's scripts/check-no-binaries.sh does the same.
set -eu
cd "${1:-$(git rev-parse --show-toplevel)}"
limit=5242880
found=""
# git ls-files -z with tr, so a path with spaces survives.
for f in $(git ls-files -z | tr '\0' '\n'); do
  [ -f "$f" ] || continue
  # ELF, Mach-O (both byte orders, 32 and 64 bit), a universal binary, PE, wasm, and an ar archive.
  magic=$(od -An -tx1 -N4 "$f" 2>/dev/null | tr -d ' \n')
  case "$magic" in
    7f454c46|cffaedfe|cefaedfe|feedfacf|feedface|cafebabe|4d5a*|0061736d|213c6172)
      found="$found  $f (a compiled executable)\n"; continue ;;
  esac
  size=$(wc -c <"$f" | tr -d ' ')
  if [ "$size" -gt "$limit" ]; then
    found="$found  $f ($size bytes, over 5 MB)\n"
  fi
done
if [ -n "$found" ]; then
  echo "check-no-binaries: these tracked files do not belong in the repo:" >&2
  printf "$found" >&2
  echo "Untrack each one, keeping the local file, and ignore it:" >&2
  echo "  git rm --cached <file> && echo '<file>' >> .gitignore" >&2
  exit 1
fi
echo "check-no-binaries: no compiled executables or files over 5 MB are tracked."
