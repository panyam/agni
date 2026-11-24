#!/bin/sh
# Fails if any tracked text file holds a NUL byte: hack/check_no_nul.sh [root].
#
# grep reads a file with a NUL byte as binary and prints no match from it, so a symbol defined there
# looks absent (agni issue 360). Source writes the character as an escape such as \u0000. A file
# counts as binary by its extension, and every other tracked file is text.
set -eu
cd "${1:-$(git rev-parse --show-toplevel)}"
found=$(git ls-files -z | perl -0 -ne '
  chomp;
  next if /\.(png|jpe?g|gif|webp|ico|bmp|pdf|Z|gz|tgz|bz2|xz|zst|zip|7z|jar|pb|binpb|wasm|woff2?|ttf|otf|glb|step|stp|PcbDoc|SchDoc|PcbLib|SchLib|brd|dra|psm|pad|DSN|OLB)$/i;
  next unless -f $_;
  open(my $fh, "<", $_) or next;
  binmode $fh;
  local $/;
  my $b = <$fh>;
  print "  $_\n" if defined $b && index($b, "\0") >= 0;
')
if [ -n "$found" ]; then
  echo "check-no-nul: these tracked text files hold a NUL byte, so grep reads them as binary:" >&2
  printf '%s\n' "$found" >&2
  printf '%s\n' "Write the character as an escape such as \\u0000, or add a binary extension to $0." >&2
  exit 1
fi
echo "check-no-nul: no tracked text file holds a NUL byte."
