#!/bin/sh
# Fold one built page into a single self-contained HTML file, for reviewing a branch before it merges.
#
# This exists because `make gh-pages` cannot do it. GitHub Pages for this repo is configured with
# build_type "workflow", so the live site is whatever docs.yml uploaded on the last push to main, and
# the gh-pages branch it force-pushes has not been served since the MkDocs tree was retired. Pushing
# a branch there changes nothing and destroys the branch's history in passing.
#
# So this inlines instead: CSS and JS go into the page, and the result opens anywhere with no server
# and no repo state. The links out to other pages will 404, which is the trade for the file being
# one file.
#
# Usage:  docsite/preview.sh learn/05-who-drives-this-net  [out.html]
set -e
page="$1"
out="${2:-preview.html}"
here=$(cd "$(dirname "$0")" && pwd)
src="$here/dist/$page/index.html"
[ -f "$src" ] || { echo "no build at $src; run 'make -C docsite build' first" >&2; exit 1; }

python3 - "$src" "$here/dist" "$out" <<'PY'
import pathlib, re, sys
src, dist, out = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), pathlib.Path(sys.argv[3])
html = src.read_text()

def asset(url):
    # Site URLs are absolute under the path prefix (/agni/static/...); map back onto dist/.
    return dist / re.sub(r'^/[^/]+/', '', url)

def inline_css(m):
    p = asset(m.group(1))
    return f"<style>\n{p.read_text()}\n</style>" if p.exists() else m.group(0)

def inline_js(m):
    p = asset(m.group(1))
    return f"<script>\n{p.read_text()}\n</script>" if p.exists() else m.group(0)

html = re.sub(r'<link[^>]+href="([^"]+\.css)"[^>]*>', inline_css, html)
html = re.sub(r'<script[^>]+src="([^"]+\.js)"[^>]*></script>', inline_js, html)
out.write_text(html)
print(f"{out}  ({len(html)//1024}KB, {html.count('<style>')} css, {html.count('<script>')} js inlined)")
PY
