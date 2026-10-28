#!/usr/bin/env bash
# python_dist_smoke.sh installs the Python client's built wheel into a fresh venv, outside the
# checkout, and checks what a stranger running `pip install agni-eda` would get: the version the
# release tag names, every generated module the source tree carries, the licence, and a client that
# imports. `make python-dist` runs it on what it built, and the release workflow runs that target
# before publishing.
#
#   hack/python_dist_smoke.sh <dist-dir> <version>      e.g. dist/python 0.3.12
set -euo pipefail

dist=${1:?usage: python_dist_smoke.sh <dist-dir> <version>}
version=${2:?usage: python_dist_smoke.sh <dist-dir> <version>}
root=$(cd "$(dirname "$0")/.." && pwd)
dist=$(cd "$dist" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export PIP_DISABLE_PIP_VERSION_CHECK=1

fail() { echo "python dist smoke: $*" >&2; exit 1; }

wheels=("$dist"/agni_eda-"$version"-*.whl)
[ -f "${wheels[0]}" ] || fail "no agni_eda-$version wheel in $dist"
[ -f "$dist/agni_eda-$version.tar.gz" ] || fail "no agni_eda-$version sdist in $dist"

python3 -m venv "$work/venv"
"$work/venv/bin/pip" install -q "${wheels[0]}"

# A count guard: an install that dropped the generated tree would still import `agni` itself.
want=$(find "$root/clients/python/src/agni" -name '*.py' -not -path '*__pycache__*' | wc -l | tr -d ' ')

# -I and an empty working directory, so nothing from the checkout can stand in for the install.
cd "$work"
"$work/venv/bin/python" -I - "$version" "$want" <<'EOF'
import importlib.metadata as md
import pathlib
import sys

version, want = sys.argv[1], int(sys.argv[2])
got = md.version("agni-eda")
if got != version:
    sys.exit(f"installed version is {got}, want {version}")

import agni
from agni import Client, CliTransport, ConnectTransport  # noqa: F401
from agni.v1.checks import checks_pb2  # noqa: F401

pkg = pathlib.Path(agni.__file__).parent
if "site-packages" not in pkg.parts:
    sys.exit(f"agni imported from {pkg}, not the installed wheel")
n = sum(1 for p in pkg.rglob("*.py") if "__pycache__" not in p.parts)
if n != want:
    sys.exit(f"the wheel holds {n} modules, the source tree {want}")
if not (pkg / "py.typed").is_file():
    sys.exit("the wheel has no py.typed")

files = md.files("agni-eda") or []
if not any(f.name.startswith("LICENSE") for f in files):
    sys.exit("the wheel carries no LICENSE")
print(f"agni-eda {got}: {n} modules, imported from {pkg}")
EOF
