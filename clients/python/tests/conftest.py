"""A real binary and a real server over the tutorial fixture.

Nothing here skips. A missing binary or an unbuilt viewer bundle fails the run, because a suite that
skips when its inputs are absent passes over an empty set (build/evidence.md).
"""

from __future__ import annotations

import os
import socket
import subprocess
import time
import urllib.request
from pathlib import Path

import pytest

from agni import Client, CliTransport, ConnectTransport

REPO = Path(__file__).resolve().parents[3]
FIXTURE = REPO / "examples" / "tutorial-project"
AGNI = os.environ.get("AGNI_BIN", str(REPO / "bin" / "agni"))
WEB_DIR = os.environ.get("AGNI_TEST_WEB_DIR", str(REPO / "web"))
MOUNTS = {"tut": str(FIXTURE)}

DESIGN = "mount://tut/designs/gateway"


@pytest.fixture(scope="session")
def isolated(tmp_path_factory):
    """An environment and working directory no personal agni.yaml can reach.

    The CLI reads ~/.config/agni/agni.yaml and walks up from its working directory for another, and
    either can add mounts or a web_dir the test did not ask for. So both lookups are pointed at an
    empty temporary tree.
    """
    root = tmp_path_factory.mktemp("agni-home")
    env = dict(os.environ)
    env.pop("AGNI_WEB_DIR", None)
    env.update(HOME=str(root), XDG_CONFIG_HOME=str(root / "xdg"))
    return env, str(root)


@pytest.fixture(scope="session")
def agni_bin():
    if not os.access(AGNI, os.X_OK):
        pytest.fail(f"no agni binary at {AGNI}: run `make agni`, or set AGNI_BIN")
    return AGNI


@pytest.fixture(scope="session")
def cli(agni_bin, isolated):
    env, cwd = isolated
    return Client(CliTransport(agni_bin, mounts=MOUNTS, env=env, cwd=cwd, strict=True))


def _free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.fixture(scope="session")
def server(agni_bin, isolated):
    env, cwd = isolated
    port = _free_port()
    argv = [agni_bin, "serve", "--addr", f"127.0.0.1:{port}", "--web-dir", WEB_DIR]
    for name, path in MOUNTS.items():
        argv += ["--mount", f"{name}={path}"]
    proc = subprocess.Popen(argv, env=env, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    base = f"http://127.0.0.1:{port}"
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            pytest.fail(f"agni serve exited {proc.returncode}:\n{proc.stdout.read()}")
        try:
            with urllib.request.urlopen(base + "/healthz", timeout=1) as r:
                if r.status == 200:
                    break
        except OSError:
            time.sleep(0.1)
    else:
        proc.kill()
        pytest.fail("agni serve did not answer /healthz within 30s")
    yield base
    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()


@pytest.fixture(scope="session")
def connect(server):
    return Client(ConnectTransport(server, strict=True))
