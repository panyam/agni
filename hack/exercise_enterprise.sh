#!/usr/bin/env bash
# The enterprise mission's exercise (agni issues 1799, 1800). It runs agni the way an IT admin would
# inside a company network, against goapplib's fake GitHub Enterprise Server (githubtest's
# cmd/fakegithub, agni issue 1841), so it needs no network and no GitHub account.
#
# The fake holds two private repos. hw/baseboard has the tutorial's gateway board at tags rev-b and
# rev-c, with the schematic and board kept in Git LFS (#1809), and hw/agni-packs has the tutorial project's checks as a pack. alice is in org acme and can
# read both. mallory is in org acme and can read neither, as a contractor might.
#
# The GitHub App is installed on hw with Contents at Read and no Members permission, as an operator
# who missed that box makes it. So the server lets alice and mallory in by name. bob is in acme too,
# and his sign-in through org:acme is refused with a message naming the missing permission (#1841).
#
# agni serve starts as the enterprise bundle starts it: TLS from a throwaway CA, --auth tokens,
# sign-in through a GitHub App on the fake, a connections key and a webhook secret. Every request it
# makes to a host other than the fake goes to a trap proxy, and the last step fails if the trap saw
# one. The CLI trusts the throwaway CA through AGNI_CA_FILE.
#
# EXERCISE_SERVE=bundle runs the same steps against deploy/enterprise/compose.yaml over AGNI_IMAGE at
# AGNI_VERSION, with no agni checkout behind the server (agni issue 1802). The fake and the trap then
# listen on EXERCISE_HOST, an address of this machine that both its own CLI and the container reach:
# the docker bridge's gateway on Linux, and this Mac's own address under Docker Desktop.
#
# EXERCISE_SERVE=helm runs them against deploy/helm/agni, installed into the cluster kubectl points
# at (agni issue 1803). That cluster needs an ingress-nginx controller on this machine's port 443
# and AGNI_IMAGE:AGNI_VERSION loaded, as `make kind-up` or ci-actions' kind-ingress leave it. The
# ingress ends TLS for https://localhost, and agni trusts it through --trusted-proxy. The fake and
# the trap listen on the kind network's gateway on Linux, or this Mac's own address.
#
# Each step prints PASS, FAIL with its reason, or FAIL (waits on #N) when an open ticket is what's
# missing. The script exits 1 on any FAIL. With EXERCISE_ALLOW_WAITS=1 it exits 0 when every FAIL
# waits on a ticket. CI runs it that way, and a step that used to pass still fails the job. Once
# #1799 is done, every step passes.
#
# It kills only the processes it started.
set -euo pipefail
agni=${AGNI:-bin/agni}
out=${EXERCISE_OUT:-/tmp/agni-exercise}/enterprise
port=${EXERCISE_PORT:-18799}
url=https://127.0.0.1:$port
namespace=agni-exercise-$$
root=$(cd "$(dirname "$0")/.." && pwd)
tutorial=$root/examples/tutorial-project
mode=${EXERCISE_SERVE:-binary}
container=${CONTAINER:-docker}
case "$mode" in
  binary) host=127.0.0.1 ;;
  bundle | helm)
    : "${AGNI_IMAGE:?EXERCISE_SERVE=$mode needs AGNI_IMAGE, such as ghcr.io/panyam/agni}"
    : "${AGNI_VERSION:?EXERCISE_SERVE=$mode needs AGNI_VERSION, the image tag}"
    [ "$mode" = helm ] && url=https://localhost
    network=bridge
    [ "$mode" = helm ] && network=kind
    host=${EXERCISE_HOST:-}
    if [ -z "$host" ]; then
      case "$(uname -s)" in
        Linux) host=$("$container" network inspect "$network" --format '{{range .IPAM.Config}}{{.Gateway}} {{end}}' | tr ' ' '\n' | grep -m1 '\.') ;;
        Darwin) host=$(ipconfig getifaddr en0 || ipconfig getifaddr en1) ;;
      esac
    fi
    [ -n "$host" ] || { echo "EXERCISE_SERVE=$mode found no address the server reaches; set EXERCISE_HOST" >&2; exit 1; }
    ;;
  *) echo "EXERCISE_SERVE=$mode: want binary, bundle or helm" >&2; exit 1 ;;
esac

rm -rf "$out"
mkdir -p "$out"
out=$(cd "$out" && pwd)
agni=$(cd "$(dirname "$agni")" && pwd)/$(basename "$agni")
store=$out/store
uploads=$out/uploads

pids=()
serve_pid=
project=agni-exercise-$$
bundle() {
  "$container" compose -f "$root/deploy/enterprise/compose.yaml" -f "$root/hack/exercise-enterprise.compose.yaml" \
    --env-file "$out/bundle.env" -p "$project" "$@"
}
# kube runs kubectl in this run's own namespace.
kube() { kubectl -n "$namespace" "$@"; }
stop_serve() {
  if [ "$mode" = helm ]; then
    serve_pid=
    return
  fi
  if [ "$mode" = bundle ]; then
    [ -n "$serve_pid" ] && bundle stop agni >/dev/null 2>&1
  else
    [ -n "$serve_pid" ] && kill "$serve_pid" 2>/dev/null && wait "$serve_pid" 2>/dev/null
  fi
  serve_pid=
}
cleanup() {
  if [ "$mode" = helm ] && [ -f "$out/values.yaml" ]; then
    kube logs deploy/agni --all-containers >"$out/serve.log" 2>&1 || true
    kube get pods,events >"$out/kube.log" 2>&1 || true
    helm uninstall agni -n "$namespace" >/dev/null 2>&1 || true
    kubectl delete namespace "$namespace" --wait=false >/dev/null 2>&1 || true
  elif [ "$mode" = bundle ] && [ -f "$out/bundle.env" ]; then
    bundle logs agni >"$out/serve.log" 2>&1 || true
    bundle down -v >/dev/null 2>&1 || true
  else
    stop_serve
  fi
  for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done
}
trap cleanup EXIT

step() { printf '\n== %s\n' "$*"; }
fail=0
waiting=0
pass() { echo "PASS: $*"; }
no() { echo "FAIL: $*" >&2; fail=1; }
waits() { echo "FAIL (waits on #$1): ${*:2}" >&2; waiting=1; }
findings() { python3 -c 'import json,sys; print(len(json.load(sys.stdin)["findings"]))'; }
wait_for() { for _ in $(seq 1 100); do curl -s -o /dev/null "$@" && return 0; sleep 0.1; done; return 1; }

step "the repos' contents"
# rev-b is the tutorial's gateway board. rev-c changes the CAN termination resistor, so the two
# tags hold different commits.
mkdir -p "$out/repos/rev-b" "$out/repos/rev-c" "$out/repos/pack"
for f in gateway.kicad_sch gateway.kicad_pcb gateway.kicad_pro; do
  cp "$tutorial/boards/gateway/$f" "$out/repos/rev-b/"
done
cp "$out/repos/rev-b/"* "$out/repos/rev-c/"
sed -i.bak 's/(property "Value" "120"/(property "Value" "60"/' "$out/repos/rev-c/gateway.kicad_sch" && rm "$out/repos/rev-c/gateway.kicad_sch.bak"
cmp -s "$out/repos/rev-b/gateway.kicad_sch" "$out/repos/rev-c/gateway.kicad_sch" && { echo "rev-c changed nothing" >&2; exit 1; }
# The pack starts without the datasheet parameters. A push below adds them, as a team adds the
# parameters for a new regulator, and the next check runs the rules that read them.
cp -R "$tutorial/project.yaml" "$tutorial/profiles" "$tutorial/lib" "$tutorial/queries" "$out/repos/pack/"
grep -q '^params: \[params\]$' "$out/repos/pack/project.yaml" || { echo "the tutorial's project.yaml names no params folder" >&2; exit 1; }
sed -i.bak '/^params: \[params\]$/d' "$out/repos/pack/project.yaml" && rm "$out/repos/pack/project.yaml.bak"

server_token=$(openssl rand -hex 20)
client_secret=$(openssl rand -hex 20)
cat >"$out/repos/seed.json" <<EOF
{
  "app": {"client_id": "Iv1.exercise", "client_secret": "$client_secret",
          "permissions": {"contents": "read"}, "installations": [{"account": "hw"}]},
  "users": [{"login": "alice", "orgs": ["acme"]}, {"login": "mallory", "orgs": ["acme"]}, {"login": "bob", "orgs": ["acme"]}],
  "tokens": ["$server_token"],
  "repos": [
    {"repo": "hw/baseboard", "private": true, "readers": ["alice"], "tokens": ["$server_token"],
     "commits": [{"tag": "rev-b", "dir": "rev-b", "lfs": ["gateway.kicad_sch", "gateway.kicad_pcb"]},
                 {"tag": "rev-c", "dir": "rev-c", "lfs": ["gateway.kicad_sch", "gateway.kicad_pcb"]}]},
    {"repo": "hw/agni-packs", "private": true, "readers": ["alice"], "tokens": ["$server_token"],
     "commits": [{"dir": "pack"}]}
  ]
}
EOF
echo "seeded hw/baseboard and hw/agni-packs"

step "a throwaway CA, and a certificate for the server"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=exercise CA" \
  -keyout "$out/ca.key" -out "$out/ca.pem" 2>/dev/null
openssl req -newkey rsa:2048 -nodes -subj "/CN=127.0.0.1" -keyout "$out/server.key" -out "$out/server.csr" 2>/dev/null
printf 'subjectAltName=IP:127.0.0.1,DNS:localhost\n' >"$out/san.ext"
openssl x509 -req -in "$out/server.csr" -CA "$out/ca.pem" -CAkey "$out/ca.key" -CAcreateserial -days 1 \
  -extfile "$out/san.ext" -out "$out/server.pem" 2>/dev/null
export AGNI_CA_FILE=$out/ca.pem
# Off loopback the fake serves HTTPS, since oneauth refuses a plain-HTTP GitHub Enterprise host
# there, so the bundle's run gives the fake a certificate of its own for its address.
if [ "$mode" != binary ]; then
  case "$host" in *[!0-9.]*) fake_san=DNS:$host ;; *) fake_san=IP:$host ;; esac
  openssl req -newkey rsa:2048 -nodes -subj "/CN=$host" -keyout "$out/fake.key" -out "$out/fake.csr" 2>/dev/null
  printf 'subjectAltName=%s\n' "$fake_san" >"$out/fake-san.ext"
  openssl x509 -req -in "$out/fake.csr" -CA "$out/ca.pem" -CAkey "$out/ca.key" -CAcreateserial -days 1 \
    -extfile "$out/fake-san.ext" -out "$out/fake.pem" 2>/dev/null
fi
echo "made $out/ca.pem and a server certificate it signed"

step "start the fake GitHub Enterprise Server"
(cd "$root" && go build -o "$out/fakegithub" github.com/panyam/goapplib/storage/mounts/github/githubtest/cmd/fakegithub)
ctl_port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')
ctl=http://127.0.0.1:$ctl_port
fake_host=()
if [ "$mode" != binary ]; then
  fake_port=$(python3 -c 'import socket,sys; s=socket.socket(); s.bind((sys.argv[1],0)); print(s.getsockname()[1])' "$host")
  fake_host=(--listen "$host:$fake_port" --base-url "https://$host:$fake_port" --tls-cert "$out/fake.pem" --tls-key "$out/fake.key")
fi
"$out/fakegithub" --seed "$out/repos/seed.json" --addr "127.0.0.1:$ctl_port" ${fake_host[@]+"${fake_host[@]}"} --hook-ca "$out/ca.pem" >>"$out/fakegithub.log" 2>&1 &
pids+=($!)
wait_for "$ctl/" || { echo "the fake didn't start; see $out/fakegithub.log" >&2; exit 1; }
answer=$(curl -s "$ctl/")
ghes=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["web"])' "$answer")
revb=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["commits"]["hw/baseboard"]["rev-b"])' "$answer")
revc=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["commits"]["hw/baseboard"]["rev-c"])' "$answer")
echo "the fake is at $ghes; rev-b is $revb and rev-c is $revc"
# The repo itself holds LFS pointers, so a check that matches the files' findings later shows that
# agni read the objects.
if curl -s --cacert "$out/ca.pem" -H "Authorization: Bearer $server_token" "$ghes/raw/hw/baseboard/$revb/gateway.kicad_sch" | grep -q '^version https://git-lfs.github.com/spec/v1'; then
  pass "hw/baseboard keeps gateway.kicad_sch at rev-b as a Git LFS pointer"
else
  no "hw/baseboard's gateway.kicad_sch at rev-b isn't an LFS pointer, so the exercise checks no LFS read"
fi

step "a trap for any request to another host"
# agni serve sends every outbound request through this proxy, except those to 127.0.0.1, which
# NO_PROXY lets through to the fake. The trap logs whatever reaches it and answers 502.
trap_port=$(python3 -c 'import socket,sys; s=socket.socket(); s.bind((sys.argv[1],0)); print(s.getsockname()[1])' "$host")
python3 - "$host" "$trap_port" "$out/trap.log" <<'PY' >>"$out/trap.out" 2>&1 &
import http.server, sys
log = open(sys.argv[3], "a", buffering=1)
class Trap(http.server.BaseHTTPRequestHandler):
    def handle_one_request(self):
        line = self.rfile.readline(65537).decode("latin-1").strip()
        if line:
            log.write(line + "\n")
            self.wfile.write(b"HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
        self.close_connection = True
http.server.ThreadingHTTPServer((sys.argv[1], int(sys.argv[2])), Trap).serve_forever()
PY
pids+=($!)
touch "$out/trap.log"
echo "the trap listens on $host:$trap_port"

AGNI_CONNECTIONS_KEY=$(openssl rand -hex 32)
AGNI_GITHUB_CLIENT_SECRET=$client_secret
AGNI_WEBHOOK_SECRET=$(openssl rand -hex 32)
HW_TOKEN=$server_token
export AGNI_CONNECTIONS_KEY AGNI_GITHUB_CLIENT_SECRET AGNI_WEBHOOK_SECRET HW_TOKEN

# The bundle reads each secret from a file. They sit in this run's own folder, readable by the
# image's uid, as the bundle's env.example asks.
if [ "$mode" = bundle ]; then
  mkdir -p "$out/secrets"
  cp "$out/server.pem" "$out/secrets/tls.crt"
  cp "$out/server.key" "$out/secrets/tls.key"
  printf '%s' "$AGNI_CONNECTIONS_KEY" >"$out/secrets/connections.key"
  printf '%s' "$AGNI_GITHUB_CLIENT_SECRET" >"$out/secrets/github-client-secret"
  printf '%s' "$AGNI_WEBHOOK_SECRET" >"$out/secrets/github-webhook-secret"
  # The fake's certificate comes from the exercise's CA, which the bundle trusts through its own
  # folder of CA certificates, as a GitHub Enterprise Server behind a team's CA would be.
  mkdir -p "$out/ca-dir"
  cp "$out/ca.pem" "$out/ca-dir/exercise-ca.pem"
  chmod 0644 "$out/ca-dir/exercise-ca.pem"
  mkdir -p "$out/connection-secrets"
  chmod 0644 "$out/secrets/"*
  cat >"$out/bundle.env" <<ENV
AGNI_IMAGE=$AGNI_IMAGE
AGNI_VERSION=$AGNI_VERSION
AGNI_PUBLIC_URL=$url
AGNI_PORT=$port
AGNI_TLS_CERT=$out/secrets/tls.crt
AGNI_TLS_KEY=$out/secrets/tls.key
AGNI_CONNECTIONS_KEY_FILE=$out/secrets/connections.key
AGNI_GITHUB_API=$ghes/api/v3
AGNI_GITHUB_CLIENT_ID=Iv1.exercise
AGNI_GITHUB_CLIENT_SECRET_FILE=$out/secrets/github-client-secret
AGNI_GITHUB_ALLOW=org:acme,user:alice,user:mallory
AGNI_GITHUB_WEBHOOK_SECRET_FILE=$out/secrets/github-webhook-secret
EXERCISE_PROXY=http://$host:$trap_port
EXERCISE_NO_PROXY=$host
AGNI_CONNECTION_SECRETS=$out/connection-secrets
AGNI_CA_DIR=$out/ca-dir
ENV
fi

# The chart reads each secret from a Secret, and the CA from a ConfigMap, made here as a team makes
# them with kubectl. Its values send agni's outbound requests through the trap.
if [ "$mode" = helm ]; then
  kubectl create namespace "$namespace" >/dev/null
  printf '%s' "$AGNI_CONNECTIONS_KEY" >"$out/connections.key"
  printf '%s' "$AGNI_GITHUB_CLIENT_SECRET" >"$out/github-client-secret"
  printf '%s' "$AGNI_WEBHOOK_SECRET" >"$out/github-webhook-secret"
  kube create secret generic agni --from-file=connections-key="$out/connections.key" \
    --from-file=github-client-secret="$out/github-client-secret" --from-file=github-webhook-secret="$out/github-webhook-secret" >/dev/null
  kube create secret tls agni-tls --cert="$out/server.pem" --key="$out/server.key" >/dev/null
  kube create configmap agni-ca --from-file=exercise-ca.pem="$out/ca.pem" >/dev/null
  cat >"$out/values.yaml" <<VALUES
image:
  repository: $AGNI_IMAGE
  tag: "$AGNI_VERSION"
publicURL: $url
trustedProxies: [${EXERCISE_POD_CIDR:-10.244.0.0/16}]
github:
  api: $ghes/api/v3
  clientID: Iv1.exercise
  allow: [org:acme, "user:alice", "user:mallory"]
caConfigMap: agni-ca
ingress:
  host: localhost
env:
  - {name: HTTPS_PROXY, value: "http://$host:$trap_port"}
  - {name: HTTP_PROXY, value: "http://$host:$trap_port"}
  - {name: NO_PROXY, value: "$host"}
VALUES
fi

serve() {
  if [ "$mode" = helm ]; then
    if [ -z "${installed:-}" ]; then
      helm install agni "$root/deploy/helm/agni" -n "$namespace" -f "$out/values.yaml" --wait --timeout 3m >>"$out/helm.log" 2>&1 ||
        { echo "the chart didn't install; see $out/helm.log" >&2; exit 1; }
      installed=1
    else
      { kube rollout restart deploy/agni && kube rollout status deploy/agni --timeout 3m; } >>"$out/helm.log" 2>&1 ||
        { echo "agni didn't restart; see $out/helm.log" >&2; exit 1; }
    fi
    serve_pid=helm
    # The ingress answers 503 until it routes to the new pod, so this waits for a 200.
    wait_for -f --cacert "$out/ca.pem" "$url/healthz" || { echo "the chart's agni serve didn't answer through the ingress; see $out/helm.log" >&2; exit 1; }
    return
  fi
  if [ "$mode" = bundle ]; then
    bundle up -d agni >>"$out/bundle.log" 2>&1 || { echo "the bundle didn't start; see $out/bundle.log" >&2; exit 1; }
    serve_pid=bundle
    wait_for --cacert "$out/ca.pem" "$url/healthz" || { bundle logs agni >"$out/serve.log" 2>&1; echo "the bundle's agni serve didn't answer; see $out/serve.log" >&2; exit 1; }
    return
  fi
  (cd "$out" && HTTPS_PROXY="http://127.0.0.1:$trap_port" HTTP_PROXY="http://127.0.0.1:$trap_port" NO_PROXY=127.0.0.1,localhost \
    exec "$agni" serve --addr "127.0.0.1:$port" --store "file://$store" --uploads "file://$uploads" \
    --auth tokens --tls-cert "$out/server.pem" --tls-key "$out/server.key" \
    --connections-key env:AGNI_CONNECTIONS_KEY --public-url "$url" \
    --github-api "$ghes/api/v3" --github-raw "$ghes/raw" \
    --github-client-id Iv1.exercise --github-client-secret env:AGNI_GITHUB_CLIENT_SECRET --github-allow org:acme,user:alice,user:mallory \
    --github-webhook-secret env:AGNI_WEBHOOK_SECRET) >>"$out/serve.log" 2>&1 &
  serve_pid=$!
  wait_for --cacert "$out/ca.pem" "$url/healthz" || { echo "agni serve didn't start; see $out/serve.log" >&2; exit 1; }
}

step "start agni serve with TLS, tokens, sign-in through the App, a connections key and webhooks"
if [ "$mode" = bundle ]; then
  bundle run --rm --no-deps -T agni token create admin --store file:///var/lib/agni/meta >"$out/admin.token" 2>>"$out/bundle.log"
elif [ "$mode" = binary ]; then
  "$agni" token create admin --store "file://$store" >"$out/admin.token" 2>/dev/null
fi
serve
# A chart's server is running before anyone can reach its claim, so the token is made beside it, as
# the chart's notes say.
[ "$mode" = helm ] && kube exec deploy/agni -- agni token create admin --store file:///var/lib/agni/meta >"$out/admin.token" 2>>"$out/helm.log"
if curl -s -o /dev/null --cacert "$out/ca.pem" "$url/healthz" && ! curl -s -o /dev/null "$url/healthz" 2>/dev/null; then
  pass "agni serve answers over TLS with the throwaway CA, and a client without it is refused"
else
  no "agni serve's TLS; see $out/serve.log"
fi
code=$(curl -s -o /dev/null -w '%{http_code}' --cacert "$out/ca.pem" -X POST "$url/agni.v1.webapi.WorkspaceService/ListMounts" -H 'content-type: application/json' -d '{}')
if [ "$code" = 401 ]; then
  pass "a call with no token is refused"
else
  no "a call with no token answered $code, want 401"
fi
# A browser that signs in gets its session cookie marked Secure only when agni sees HTTPS. Behind the
# chart's ingress, agni sees it only when --trusted-proxy names the ingress's pods.
curl -s -o /dev/null -D "$out/signin.headers" --cacert "$out/ca.pem" -X POST "$url/signin" --data-urlencode "token=$(cat "$out/admin.token")"
if grep -i '^set-cookie:' "$out/signin.headers" | grep -qi '; *secure'; then
  pass "a browser signing in with a token gets a Secure session cookie"
else
  no "the sign-in's session cookie isn't Secure; see $out/signin.headers"
fi

# admin runs as the operator, with the token made on the server's machine.
admin() { AGNI_TOKEN=$(cat "$out/admin.token") "$agni" "$@" --remote "$url"; }
# as runs a command as alice or mallory, each with their own config folder, so each keeps the token
# agni login minted for them.
as() { local who=$1; shift; HOME="$out/people/$who" XDG_CONFIG_HOME="$out/people/$who/.config" "$agni" "$@" --remote "$url"; }

login() {
  local who=$1 log=$out/login-$1.log code
  mkdir -p "$out/people/$who"
  HOME="$out/people/$who" XDG_CONFIG_HOME="$out/people/$who/.config" "$agni" login --remote "$url" >"$log" 2>&1 &
  local lpid=$!
  for _ in $(seq 1 100); do
    code=$(sed -n 's/.*enter the code \([A-Z0-9-]*\).*/\1/p' "$log")
    [ -n "$code" ] && break
    sleep 0.1
  done
  if [ -z "$code" ]; then
    kill "$lpid" 2>/dev/null
    return 1
  fi
  curl -s -f -o /dev/null -X POST "$ctl/approve" --data-urlencode "login=$who" --data-urlencode "code=$code" || { kill "$lpid" 2>/dev/null; return 1; }
  wait "$lpid"
}

step "alice and mallory sign in from a terminal, through the App's device flow"
for who in alice mallory; do
  if login "$who" && grep -q "signed in to $url as github:$who" "$out/login-$who.log"; then
    pass "$who signed in as github:$who"
  else
    no "$who's sign-in; see $out/login-$who.log and $out/serve.log"
  fi
done

step "bob, in acme, is refused because the App can't read acme's members"
if ! login bob && grep -q "Members organization permission" "$out/login-bob.log"; then
  pass "bob's sign-in through org:acme is refused, naming the App's missing Members permission"
else
  no "bob's sign-in wasn't refused with the Members permission named; see $out/login-bob.log"
fi

step "the admin adds a server connection and mounts hw/baseboard through it"
# The local server reads the connection's token from its own environment. A server in a container
# can't reach this machine's environment, so the admin's CLI reads the token here and the server
# seals it.
if [ "$mode" != binary ]; then
  admin connections add hw --seal --github-token env:HW_TOKEN >"$out/connection.txt"
else
  admin connections add hw --github-token env:HW_TOKEN >"$out/connection.txt"
fi
sealed_ok=1
[ "$mode" != binary ] && ! grep -q "^hw$(printf '\t')github-token$(printf '\t')sealed" "$out/connection.txt" && sealed_ok=
if [ -n "$sealed_ok" ] && admin mounts add board github://hw/baseboard --ref rev-b --connection hw | tee "$out/mount.txt" | grep -q "via hw"; then
  pass "hw/baseboard mounted at rev-b through the server connection$([ "$mode" != binary ] && echo ", its token sealed")"
else
  no "mounting hw/baseboard; see $out/connection.txt, $out/mount.txt and $out/serve.log"
fi
if [ "$mode" = bundle ]; then
  store=$out/store-copy
  rm -rf "$store" && bundle cp agni:/var/lib/agni/meta "$store" >/dev/null
elif [ "$mode" = helm ]; then
  store=$out/store-copy
  rm -rf "$store" && kube cp "$(kube get pod -l app.kubernetes.io/component=server -o name | head -1 | cut -d/ -f2):/var/lib/agni/meta" "$store" >/dev/null
fi
if grep -rqF "$HW_TOKEN" "$store"; then
  no "the store holds the server connection's token in the clear"
else
  pass "the store holds no token in the clear"
fi

step "alice links designs/bb from hw/baseboard at rev-b, and checks it"
as alice designs create bb >/dev/null
as alice designs link bb schematic "board/gateway.kicad_sch@$revb" >/dev/null
as alice designs link bb board "board/gateway.kicad_pcb@$revb" >/dev/null
stored=$(as alice check designs/bb --format json 2>"$out/check-bb.err" | findings || true)
local_=$("$agni" check "$out/repos/rev-b" --format json 2>/dev/null | findings)
echo "designs/bb: ${stored:-none} findings; the rev-b files: $local_ findings"
if [ -n "$stored" ] && [ "$stored" -gt 0 ] && [ "$stored" = "$local_" ]; then
  pass "alice's check on designs/bb at rev-b gives the files' $local_ findings, read from Git LFS"
else
  no "alice's check on designs/bb; see $out/check-bb.err"
fi

step "alice links the team's pack from hw/agni-packs on main"
if admin mounts add packs github://hw/agni-packs --ref main --connection hw >/dev/null 2>"$out/packs-mount.err" &&
  as alice packs link house packs@main >"$out/packs-link.txt" 2>&1 && grep -q "from packs@main at .*, followed" "$out/packs-link.txt"; then
  pass "the pack is linked from hw/agni-packs on main, and follows it"
  pack_linked=1
else
  no "linking a pack from a repo; see $out/packs-link.txt and $out/packs-mount.err"
  pack_linked=
fi
# fromParams counts the findings of rules that read a part's datasheet parameters, which the pack
# holds none of until the push below.
fromParams() { python3 -c 'import json,sys; print(sum(1 for f in json.load(sys.stdin)["findings"] if f["rule"] in ("supply-exceeds-abs-max", "pin-exceeds-abs-max", "regulator-output-exceeds-abs-max", "cap-voltage")))'; }
before=
if [ -n "$pack_linked" ] && as alice check designs/bb --pack house --format json >"$out/check-pack.json" 2>"$out/check-pack.err"; then
  before=$(fromParams <"$out/check-pack.json")
  pass "alice checks designs/bb under the linked pack, with $(findings <"$out/check-pack.json") findings"
else
  no "checking designs/bb under the linked pack; see $out/check-pack.err"
fi

step "a push of the regulators' datasheet parameters to hw/agni-packs moves the pack, and the next check reads them"
rm -rf "$out/repos/pack-2" && cp -R "$out/repos/pack" "$out/repos/pack-2" && cp -R "$tutorial/params" "$out/repos/pack-2/"
sed -i.bak 's/^profiles: \[profiles\]$/profiles: [profiles]\
params: [params]/' "$out/repos/pack-2/project.yaml" && rm "$out/repos/pack-2/project.yaml.bak"
pushed=$(curl -s -X POST "$ctl/push" --data-urlencode "repo=hw/agni-packs" --data-urlencode "branch=main" \
  --data-urlencode "dir=$out/repos/pack-2" --data-urlencode "hook=$url/hooks/github" --data-urlencode "secret=$AGNI_WEBHOOK_SECRET")
psha=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("sha",""))' "$pushed" 2>/dev/null || true)
moved=
for _ in $(seq 1 50); do
  as alice packs get house >"$out/pack-house.txt" 2>&1 || true
  [ -n "$psha" ] && grep -q "at $psha, followed" "$out/pack-house.txt" && moved=1 && break
  sleep 0.2
done
after=$(as alice check designs/bb --pack house --format json 2>"$out/check-pack-2.err" | fromParams || true)
as alice reports create bb --pack house >"$out/report-pack.txt" 2>&1 || true
echo "packs/house: ${psha:-no push}; datasheet findings before ${before:-none}, after ${after:-none}"
if [ -n "$moved" ] && [ "${before:-x}" = 0 ] && [ "${after:-0}" -gt 0 ] && grep -q "packs/house at ${psha:0:12}" "$out/report-pack.txt"; then
  pass "the push moved packs/house to ${psha:0:12}, the next check read the parameters ($after findings), and the report names the commit"
else
  no "the pack didn't move to ${psha:-a new commit}, read the parameters, or name it in a report; see $out/pack-house.txt and $out/report-pack.txt"
fi

step "a signed push to hw/baseboard's main moves a design that follows main"
as alice designs create head >/dev/null
as alice designs link head schematic "board/gateway.kicad_sch@main" >/dev/null
pushed=$(curl -s -X POST "$ctl/push" --data-urlencode "repo=hw/baseboard" --data-urlencode "branch=main" \
  --data-urlencode "dir=$out/repos/rev-b" --data-urlencode "hook=$url/hooks/github" --data-urlencode "secret=$AGNI_WEBHOOK_SECRET")
sha=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("sha",""))' "$pushed" 2>/dev/null || true)
status=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("status",""))' "$pushed" 2>/dev/null || true)
moved=
for _ in $(seq 1 50); do
  as alice designs artifacts head >"$out/head.txt" 2>&1 || true
  grep -q "$sha" "$out/head.txt" && moved=1 && break
  sleep 0.2
done
if [ "$status" = 202 ] && [ -n "$moved" ]; then
  pass "the push was delivered over TLS, and designs/head moved to $sha"
else
  no "the push answered ${status:-nothing}, and designs/head didn't reach ${sha:-a new commit}; see $out/head.txt"
fi

refused() {
  local what=$1 log=$2
  grep -q "permission_denied" "$log" && pass "mallory is refused $what" || no "mallory was not refused $what; see $log"
}

step "mallory, who can't read hw/baseboard, can't open designs/bb"
as mallory designs get bb >"$out/mallory-get.txt" 2>&1 || true
refused "designs/bb by name" "$out/mallory-get.txt"
as mallory packs get house >"$out/mallory-pack.txt" 2>&1 || true
refused "packs/house, read from hw/agni-packs, by name" "$out/mallory-pack.txt"
as mallory check designs/bb >"$out/mallory-check.txt" 2>&1 || true
refused "a check on designs/bb" "$out/mallory-check.txt"
as mallory designs list >"$out/mallory-list.txt" 2>&1 || true
if grep -qw bb "$out/mallory-list.txt"; then
  no "mallory's listing names designs/bb; see $out/mallory-list.txt"
else
  pass "mallory's listing leaves designs/bb out"
fi
as alice packs list >"$out/alice-packs.txt" 2>&1 || true
if as mallory packs list >"$out/mallory-packs.txt" 2>&1 && ! grep -q "packs/house" "$out/mallory-packs.txt" && grep -q "packs/house" "$out/alice-packs.txt"; then
  pass "mallory's pack listing leaves packs/house out, and alice's names it"
else
  no "mallory's pack listing names packs/house, or alice's doesn't; see $out/mallory-packs.txt and $out/alice-packs.txt"
fi

step "restart the server over the same folders, and read everything back"
as alice designs artifacts bb >"$out/artifacts-before.txt" 2>&1
stop_serve
serve
as alice designs artifacts bb >"$out/artifacts-after.txt" 2>&1 || true
if cmp -s "$out/artifacts-before.txt" "$out/artifacts-after.txt" && grep -q "$revb" "$out/artifacts-after.txt"; then
  pass "designs/bb reads back at rev-b after the restart"
else
  no "designs/bb differs after the restart; see $out/artifacts-before.txt and -after.txt"
fi
as mallory designs get bb >"$out/mallory-get-2.txt" 2>&1 || true
refused "designs/bb after the restart" "$out/mallory-get-2.txt"

step "nothing reached a host other than the fake"
if [ -s "$out/trap.log" ]; then
  no "agni serve tried to reach another host: $(sort -u "$out/trap.log" | tr '\n' ';')"
else
  pass "every request agni serve made went to the fake"
fi

step "result"
if [ "$fail" = 0 ] && [ "$waiting" = 0 ]; then
  echo "exercise-enterprise: ok"
elif [ "$fail" = 0 ]; then
  echo "exercise-enterprise: every step passes but those that wait on an open ticket"
  [ "${EXERCISE_ALLOW_WAITS:-}" = 1 ] || fail=1
fi
exit "$fail"
