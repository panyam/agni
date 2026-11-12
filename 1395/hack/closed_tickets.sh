#!/usr/bin/env bash
# closed_tickets.sh <spec>: print, comma-separated, the tickets a walk's steps wait on (`waitsOn:
# ["#N", ...]`) that GitHub reports closed. make exercise-cross-probe passes the list to the walk as
# CLOSED_TICKETS, so a step stops waiting on a ticket that is done and has to pass (agni issue 1326's
# close went unnoticed until the walk learned this). Without gh, or offline, it prints nothing and
# says so on stderr, and the walk keeps the waits as written.
set -uo pipefail
spec=$1
if ! command -v gh >/dev/null 2>&1; then
  echo "closed_tickets: no gh on the PATH, so every wait in $spec stands" >&2
  exit 0
fi
closed=()
for n in $(grep -o 'waitsOn: \[[^]]*\]' "$spec" | grep -o '[0-9][0-9]*' | sort -un); do
  state=$(gh issue view "$n" --repo panyam/agni --json state -q .state 2>/dev/null) || {
    echo "closed_tickets: could not ask GitHub about #$n, so its wait stands" >&2
    continue
  }
  [ "$state" = "CLOSED" ] && closed+=("$n")
done
(IFS=,; echo "${closed[*]:-}")
