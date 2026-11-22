#!/usr/bin/env bash
# The enterprise mission's exercise (agni issues 1799, 1800). It runs agni the way an IT admin would
# inside a company network, against a fake GitHub Enterprise Server (tools/fakegithub --seed), so it
# needs no network and no GitHub account.
#
# The fake holds two private repos. hw/baseboard has the tutorial's gateway board at tags rev-b and
# rev-c, with the schematic and board kept in Git LFS (#1809), and hw/agni-packs has the tutorial project's checks as a pack. alice is in org acme and can
# read both. mallory is in org acme and can read neither, as a contractor might.
#
# agni serve starts as the bundle will start it (#1802): TLS from a throwaway CA, --auth tokens,
# sign-in through a GitHub App on the fake, a connections key and a webhook secret. Every request it
# makes to a host other than the fake goes to a trap proxy, and the last step fails if the trap saw
# one. The CLI trusts the throwaway CA through AGNI_CA_FILE.
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
root=$(cd "$(dirname "$0")/.." && pwd)
tutorial=$root/examples/tutorial-project

rm -rf "$out"
mkdir -p "$out"
out=$(cd "$out" && pwd)
agni=$(cd "$(dirname "$agni")" && pwd)/$(basename "$agni")
store=$out/store
uploads=$out/uploads

pids=()
serve_pid=
stop_serve() { [ -n "$serve_pid" ] && kill "$serve_pid" 2>/dev/null && wait "$serve_pid" 2>/dev/null; serve_pid=; }
cleanup() {
  stop_serve
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
  cp "$tutorial/designs/gateway/$f" "$out/repos/rev-b/"
done
cp "$out/repos/rev-b/"* "$out/repos/rev-c/"
sed -i.bak 's/(property "Value" "120"/(property "Value" "60"/' "$out/repos/rev-c/gateway.kicad_sch" && rm "$out/repos/rev-c/gateway.kicad_sch.bak"
cmp -s "$out/repos/rev-b/gateway.kicad_sch" "$out/repos/rev-c/gateway.kicad_sch" && { echo "rev-c changed nothing" >&2; exit 1; }
cp -R "$tutorial/project.yaml" "$tutorial/profiles" "$tutorial/params" "$tutorial/lib" "$tutorial/queries" "$out/repos/pack/"

server_token=$(openssl rand -hex 20)
client_secret=$(openssl rand -hex 20)
cat >"$out/repos/seed.json" <<EOF
{
  "app": {"client_id": "Iv1.exercise", "client_secret": "$client_secret"},
  "users": [{"login": "alice", "orgs": ["acme"]}, {"login": "mallory", "orgs": ["acme"]}],
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
echo "made $out/ca.pem and a server certificate it signed"

step "start the fake GitHub Enterprise Server"
(cd "$root" && go build -o "$out/fakegithub" ./tools/fakegithub)
ctl_port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')
ctl=http://127.0.0.1:$ctl_port
"$out/fakegithub" --seed "$out/repos/seed.json" --addr "127.0.0.1:$ctl_port" --hook-ca "$out/ca.pem" >>"$out/fakegithub.log" 2>&1 &
pids+=($!)
wait_for "$ctl/" || { echo "the fake didn't start; see $out/fakegithub.log" >&2; exit 1; }
answer=$(curl -s "$ctl/")
ghes=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["web"])' "$answer")
revb=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["commits"]["hw/baseboard"]["rev-b"])' "$answer")
revc=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["commits"]["hw/baseboard"]["rev-c"])' "$answer")
echo "the fake is at $ghes; rev-b is $revb and rev-c is $revc"
# The repo itself holds LFS pointers, so a check that matches the files' findings later shows that
# agni read the objects.
if curl -s -H "Authorization: Bearer $server_token" "$ghes/raw/hw/baseboard/$revb/gateway.kicad_sch" | grep -q '^version https://git-lfs.github.com/spec/v1'; then
  pass "hw/baseboard keeps gateway.kicad_sch at rev-b as a Git LFS pointer"
else
  no "hw/baseboard's gateway.kicad_sch at rev-b isn't an LFS pointer, so the exercise checks no LFS read"
fi

step "a trap for any request to another host"
# agni serve sends every outbound request through this proxy, except those to 127.0.0.1, which
# NO_PROXY lets through to the fake. The trap logs whatever reaches it and answers 502.
trap_port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')
python3 - "$trap_port" "$out/trap.log" <<'PY' >>"$out/trap.out" 2>&1 &
import http.server, sys
log = open(sys.argv[2], "a", buffering=1)
class Trap(http.server.BaseHTTPRequestHandler):
    def handle_one_request(self):
        line = self.rfile.readline(65537).decode("latin-1").strip()
        if line:
            log.write(line + "\n")
            self.wfile.write(b"HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
        self.close_connection = True
http.server.ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Trap).serve_forever()
PY
pids+=($!)
touch "$out/trap.log"
echo "the trap listens on 127.0.0.1:$trap_port"

AGNI_CONNECTIONS_KEY=$(openssl rand -hex 32)
AGNI_GITHUB_CLIENT_SECRET=$client_secret
AGNI_WEBHOOK_SECRET=$(openssl rand -hex 32)
HW_TOKEN=$server_token
export AGNI_CONNECTIONS_KEY AGNI_GITHUB_CLIENT_SECRET AGNI_WEBHOOK_SECRET HW_TOKEN

serve() {
  (cd "$out" && HTTPS_PROXY="http://127.0.0.1:$trap_port" HTTP_PROXY="http://127.0.0.1:$trap_port" NO_PROXY=127.0.0.1,localhost \
    exec "$agni" serve --addr "127.0.0.1:$port" --store "file://$store" --uploads "file://$uploads" \
    --auth tokens --tls-cert "$out/server.pem" --tls-key "$out/server.key" \
    --connections-key env:AGNI_CONNECTIONS_KEY --public-url "$url" \
    --github-api "$ghes/api/v3" --github-raw "$ghes/raw" \
    --github-client-id Iv1.exercise --github-client-secret env:AGNI_GITHUB_CLIENT_SECRET --github-allow org:acme \
    --github-webhook-secret env:AGNI_WEBHOOK_SECRET) >>"$out/serve.log" 2>&1 &
  serve_pid=$!
  wait_for --cacert "$out/ca.pem" "$url/healthz" || { echo "agni serve didn't start; see $out/serve.log" >&2; exit 1; }
}

step "start agni serve with TLS, tokens, sign-in through the App, a connections key and webhooks"
"$agni" token create admin --store "file://$store" >"$out/admin.token" 2>/dev/null
serve
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

step "the admin adds a server connection and mounts hw/baseboard through it"
admin connections add hw --github-token env:HW_TOKEN >/dev/null
if admin mounts add board github://hw/baseboard --ref rev-b --connection hw | tee "$out/mount.txt" | grep -q "via hw"; then
  pass "hw/baseboard mounted at rev-b through the server connection"
else
  no "mounting hw/baseboard; see $out/mount.txt and $out/serve.log"
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
# agni packs link is #1801. Until then the command doesn't exist, and the three steps that need the
# pack wait on it.
if admin mounts add packs github://hw/agni-packs --ref main --connection hw >/dev/null 2>"$out/packs-mount.err" &&
  as alice packs link house --mount packs --ref main >"$out/packs-link.txt" 2>&1; then
  pass "the pack is linked from hw/agni-packs on main"
  pack_linked=1
else
  waits 1801 "linking a pack from a repo; see $out/packs-link.txt"
  pack_linked=
fi
if [ -n "$pack_linked" ] && as alice check designs/bb --pack house --format json >"$out/check-pack.json" 2>&1; then
  pass "alice checks designs/bb under the linked pack"
else
  waits 1801 "checking designs/bb under the linked pack"
fi
waits 1801 "a push of a new rule to hw/agni-packs moves the pack, and the next check runs it"

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
as mallory check designs/bb >"$out/mallory-check.txt" 2>&1 || true
refused "a check on designs/bb" "$out/mallory-check.txt"
as mallory designs list >"$out/mallory-list.txt" 2>&1 || true
if grep -qw bb "$out/mallory-list.txt"; then
  no "mallory's listing names designs/bb; see $out/mallory-list.txt"
else
  pass "mallory's listing leaves designs/bb out"
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

step "the deployment bundle"
waits 1802 "the same run from the release's deployment bundle, with no agni checkout"

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
