#!/usr/bin/env bash
# The twin of panyam/agni-samples' hack/clean_remote.sh. Keep the two the same apart from
# the default paths, so a board fetches the same way in both checkouts (agni issue 1446).
# Lists the cached upstream files hack/fetch_remote.sh has not used for DAYS days (default 30), and
# deletes them only after you answer y:
#   hack/clean_remote.sh [days]
#
# Run without a terminal, it prints the list and deletes nothing, so no target or CI job can empty
# the cache by accident. A deleted file is downloaded again by the next fetch.
set -euo pipefail

days=${1:-30}
cache=${AGNI_SAMPLES_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/agni/samples}

[ -d "$cache" ] || { echo "clean_remote: no cache at $cache"; exit 0; }

old=()
while IFS= read -r f; do old+=("$f"); done < <(find "$cache" -maxdepth 1 -type f -mtime +"$days" | sort)
if [ ${#old[@]} -eq 0 ]; then
  echo "clean_remote: nothing in $cache is older than $days days"
  exit 0
fi

echo "clean_remote: these cached files have not been used for $days days:"
for f in "${old[@]}"; do
  printf '  %8s  %s\n' "$(du -h "$f" | cut -f1)" "$(basename "$f")"
done

if [ ! -t 0 ]; then
  echo "clean_remote: not a terminal, so nothing was deleted"
  exit 0
fi
printf 'Delete them? [y/N] '
read -r answer
case $answer in
  y | Y | yes) for f in "${old[@]}"; do rm -f "$f"; done; echo "clean_remote: deleted ${#old[@]} files" ;;
  *) echo "clean_remote: nothing was deleted" ;;
esac
