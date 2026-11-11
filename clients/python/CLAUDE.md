# clients/python

This file covers how the Python client is installed, tested and published.

- **The client publishes to PyPI as `agni-eda`**, from the release workflow's `pypi` job by trusted publishing. The import is still `agni`, since PyPI's `agni` belongs to someone else.
- The tree's `pyproject.toml` says `0.0.0`. `make python-dist V=...` stamps the tag into a copy, so only the tag records a version.
- The gate's `python-dist-check` builds the wheel and installs it outside the checkout, so a packaging break shows on the PR. RELEASING.md has the one-time PyPI setup.
- **The client is installed only in `clients/python/.venv`** (`make python-venv`). A bare `python3 examples/…` fails with `No module named 'agni'`.
- **The CLI transport runs whatever `agni` is on PATH** unless given `--agni` or `AGNI_BIN`. A stale `agni` on PATH then answers instead of your build.
- A make target that runs a client script uses `$(PY_VENV)/bin/python` and `--agni $(CURDIR)/bin/agni`, as `python-test` and `exercise-revision-audit` do.
- `clients/python/examples/revision_audit.summary.md` is a golden file. The client's tests regenerate and compare it, under `make examples-test` too. `AGNI_UPDATE_GOLDEN=1` rewrites it, and its diff is the review.
