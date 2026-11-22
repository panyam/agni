#!/usr/bin/env bash
# Runs the store tests once on each backend --store takes besides a folder: sqlite, postgres and
# Datastore (agni issue 1733). `make test` already runs them on a folder. It starts postgres and the
# Datastore emulator in Docker, pinned by digest (C37), unless AGNI_TEST_POSTGRES or
# DATASTORE_EMULATOR_HOST already names one, and removes only the containers it started.
#
#   hack/test_stores.sh                 # every backend
#   hack/test_stores.sh postgres        # one
set -euo pipefail

postgres_image=postgres:17-alpine@sha256:f9cf3e1f0066f0eb0c48707d864f0ac5941e9ff454a072077d97a4d278d07d73
emulator_image=gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators@sha256:c138242bdb2a18afe7433fad5f30aba7b296c3f3ed3a3f587d9176ff38237ecf
pkgs=(./internal/records/... ./internal/projects/... ./internal/authgate/... ./internal/storeurl/... ./cmd/agni/...)
backends=("${@:-sqlite postgres datastore}")
backends=(${backends[*]})
go=${GO:-go}

started=()
cleanup() { for c in "${started[@]:-}"; do [ -n "$c" ] && docker rm -f "$c" >/dev/null 2>&1 || true; done; }
trap cleanup EXIT

wants() { for b in "${backends[@]}"; do [ "$b" = "$1" ] && return 0; done; return 1; }

if wants postgres && [ -z "${AGNI_TEST_POSTGRES:-}" ]; then
  pg=agni-test-pg-$$
  docker run -d --rm --name "$pg" -e POSTGRES_USER=agni -e POSTGRES_PASSWORD=agni -e POSTGRES_DB=agni \
    -p 127.0.0.1::5432 "$postgres_image" >/dev/null
  started+=("$pg")
  # Over TCP: the image first runs a server for initdb on the unix socket alone, then stops it, so a
  # socket check can pass against that one and the tests then meet a server shutting down.
  for _ in $(seq 1 60); do docker exec "$pg" pg_isready -h 127.0.0.1 -U agni -d agni >/dev/null 2>&1 && break; sleep 1; done
  docker exec "$pg" pg_isready -h 127.0.0.1 -U agni -d agni >/dev/null || { docker logs "$pg" >&2; exit 1; }
  export AGNI_TEST_POSTGRES="postgres://agni@127.0.0.1:$(docker port "$pg" 5432 | head -1 | sed 's/.*://')/agni?sslmode=disable"
  export PGPASSWORD=agni
fi

if wants datastore && [ -z "${DATASTORE_EMULATOR_HOST:-}" ]; then
  ds=agni-test-ds-$$
  docker run -d --rm --name "$ds" -p 127.0.0.1::8081 "$emulator_image" \
    gcloud beta emulators datastore start --host-port=0.0.0.0:8081 --no-store-on-disk --consistency=1.0 --project=agni-test >/dev/null
  started+=("$ds")
  export DATASTORE_EMULATOR_HOST="127.0.0.1:$(docker port "$ds" 8081 | head -1 | sed 's/.*://')"
  for _ in $(seq 1 120); do curl -sf "http://$DATASTORE_EMULATOR_HOST/" >/dev/null && break; sleep 1; done
  curl -sf "http://$DATASTORE_EMULATOR_HOST/" >/dev/null || { docker logs "$ds" >&2; exit 1; }
  export DATASTORE_PROJECT_ID=agni-test
fi

fail=0
for b in "${backends[@]}"; do
  echo "== store tests on $b"
  AGNI_TEST_RECORDS=$b "$go" test -count=1 "${pkgs[@]}" || fail=1
  # agnids reads the same store, from the datasheet module.
  (cd datasheet && AGNI_TEST_RECORDS=$b "$go" test -count=1 ./cmd/agnids/...) || fail=1
done
exit $fail
