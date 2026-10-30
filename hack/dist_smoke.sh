#!/usr/bin/env bash
# Runs a release's binary the way someone who just downloaded it would (WS16-003): out of a temp
# directory with no checkout, no agni.yaml and an empty cache, so the viewer can only come from what
# the binary embeds. `make dist` and the release workflow call it.
#   hack/dist_smoke.sh dist/release v0.3.13
set -euo pipefail

dist=$(cd "${1:?usage: dist_smoke.sh <dist-dir> <tag>}" && pwd)
tag=${2:?usage: dist_smoke.sh <dist-dir> <tag>}
repo=$(cd "$(dirname "$0")/.." && pwd)

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "dist_smoke: no release binary for $(uname -m)"; exit 1 ;;
esac
name="agni_${tag}_${os}_${arch}"

# Every tarball is listed in checksums.txt and matches it, the viewer bundle included, since that
# list is what a `go install` binary checks its download against.
(cd "$dist" && for f in *.tar.gz; do
  grep -q "  $f\$" checksums.txt || { echo "dist_smoke: $f is not in checksums.txt"; exit 1; }
done && shasum -a 256 -c checksums.txt >/dev/null)
# macOS tar hides ._ entries when it lists an archive, so another reader lists them.
for f in "$dist"/*.tar.gz; do
  if python3 -c 'import sys, tarfile; sys.exit(any(n.split("/")[-1].startswith("._") for n in tarfile.open(sys.argv[1]).getnames()))' "$f"; then :; else
    echo "dist_smoke: $(basename "$f") carries macOS ._ shadow files"; exit 1
  fi
done
tar -tzf "$dist/agni-web-$tag.tar.gz" | grep -qx "templates/ViewerPage.html" \
  || { echo "dist_smoke: agni-web-$tag.tar.gz has no templates/ViewerPage.html"; exit 1; }

tmp=$(mktemp -d)
pid=""
cleanup() {
  [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT
tar -C "$tmp" -xzf "$dist/$name.tar.gz"
agni="$tmp/$name/agni"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home/.config" XDG_CACHE_HOME="$tmp/home/.cache"
mkdir -p "$tmp/work" "$HOME"
cp "$repo"/demo/showcase.fires.kicad_pro "$repo"/demo/showcase.fires.kicad_sch "$tmp/work/"
cd "$tmp/work"

got=$("$agni" version | head -1)
[ "$got" = "agni $tag" ] || { echo "dist_smoke: version says '$got', want 'agni $tag'"; exit 1; }

"$agni" check showcase.fires.kicad_sch > check.txt 2>&1 || true
grep -q "i2c-pull-up: /SCL" check.txt || { echo "dist_smoke: check found no i2c-pull-up on /SCL:"; cat check.txt; exit 1; }

"$agni" open showcase.fires.kicad_sch > open.log 2>&1 &
pid=$!
url=""
for _ in $(seq 1 100); do
  url=$(grep -o 'http://127\.0\.0\.1:[0-9]*/designs/[^ ]*' open.log | head -1 || true)
  [ -n "$url" ] && break
  kill -0 "$pid" 2>/dev/null || { echo "dist_smoke: open exited:"; cat open.log; exit 1; }
  sleep 0.1
done
[ -n "$url" ] || { echo "dist_smoke: open printed no URL:"; cat open.log; exit 1; }
base=${url%%/designs/*}
for path in "${url#"$base"}" /static/app.js /static/app.css; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$base$path")
  [ "$code" = 200 ] || { echo "dist_smoke: GET $path answered $code"; cat open.log; exit 1; }
done
echo "dist_smoke: $name runs version, check and open from an empty directory"
