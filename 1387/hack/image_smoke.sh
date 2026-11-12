#!/usr/bin/env bash
# image_smoke.sh — prove a built container image works, not merely that it built.
#
#   hack/image_smoke.sh agni   <image> [want-version]
#   hack/image_smoke.sh agnids <image> [want-version]
#
# Run by the `images` workflow on a PR that touches the images, and by `release` against each
# published image, so the two checks cannot drift apart. With a want-version, the first line of
# `<binary> version` must be exactly "<binary> <want-version>", which is the failure the release
# workflow exists to prevent (an image whose tag and binary disagree).
#
# agni: run the catalog over a demo board baked into the image, so a broken symbol path or a missing
# asset fails here rather than for a user.
#
# agnids: serve the workbench over a mounted folder holding the synthetic warm-up PDF, then call
# ExtractDocIR and require the doc-IR sibling on disk. That exercises everything the image adds over
# the binary: the bundle, --mount-root, the docling venv and its system libraries, and the models it
# must find offline. It also starts a draft, publishes it, and reads it back through PartSpecService,
# which is the corpus store at /corpus that the default command serves. The container runs as the caller's uid so it can write into the mounted folder,
# which is also how an operator is told to run it.
set -euo pipefail

# Every command's output is captured WHOLE and trimmed afterwards, never piped into head. Under
# pipefail, `docker run ... | head -1` fails whenever the container is still writing when head
# exits: docker takes a broken pipe and exits 1, and whether that happens depends on how the
# output was chunked. One CI run passed and the next failed on the same commit.
first_lines() { sed -n "1,${1}p" <<<"$2"; }

kind=${1:?usage: image_smoke.sh agni|agnids <image> [want-version]}
img=${2:?usage: image_smoke.sh agni|agnids <image> [want-version]}
want=${3:-}

if [ -n "$want" ]; then
	out="$(docker run --rm "$img" version)"
	got="$(first_lines 1 "$out")"
	echo "reported: $got"
	[ "$got" = "$kind $want" ] || { echo "::error::$img reports '$got', expected '$kind $want'"; exit 1; }
fi

case "$kind" in
agni)
	out="$(docker run --rm "$img" check /workspace/demo/showcase.fires.kicad_pro)"
	first_lines 5 "$out"
	;;
agnids)
	here="$(cd "$(dirname "$0")/.." && pwd)"
	dir="$(mktemp -d)"
	name="agnids-smoke-$$"
	trap 'docker rm -f "$name" >/dev/null 2>&1 || true; rm -rf "$dir"' EXIT
	mkdir -p "$dir/smoke/PART" "$dir/corpus"
	cp "$here/datasheet/tools/pdf2doc/testdata/warmup.pdf" "$dir/smoke/PART/part.pdf"
	docker run -d --name "$name" --user "$(id -u):$(id -g)" -p 127.0.0.1:18090:8090 \
		-v "$dir/smoke:/datasheets/smoke" -v "$dir/corpus:/corpus" "$img" >/dev/null
	for _ in $(seq 1 60); do
		docker exec "$name" agnids healthcheck >/dev/null 2>&1 && break
		sleep 1
	done
	docker exec "$name" agnids healthcheck
	code="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18090/datasheets/)"
	[ "$code" = 200 ] || { echo "::error::workbench page answered $code"; docker logs "$name"; exit 1; }
	resp="$(curl -sf -X POST -H 'Content-Type: application/json' \
		http://127.0.0.1:18090/agni.v1.dsapi.DatasheetService/ExtractDocIR \
		-d '{"uri":"mount://smoke/PART/part.pdf"}')" || { echo "::error::ExtractDocIR failed"; docker logs "$name"; exit 1; }
	echo "${resp:0:300}"
	[ -s "$dir/smoke/PART/part.doc.textproto" ] || { echo "::error::Extract wrote no doc-IR"; docker logs "$name"; exit 1; }
	grep -q 'agnids image warm-up page' "$dir/smoke/PART/part.doc.textproto" \
		|| { echo "::error::doc-IR does not carry the page's text"; exit 1; }
	echo "agnids: Extract produced $(wc -c <"$dir/smoke/PART/part.doc.textproto") bytes of doc-IR offline"
	rpc() {
		curl -sf -X POST -H 'Content-Type: application/json' "http://127.0.0.1:18090/$1" -d "$2" \
			|| { echo "::error::$1 failed"; docker logs "$name"; exit 1; }
	}
	rpc agni.v1.dsapi.DatasheetService/SaveDraft \
		'{"draft": {"mpn": "SMOKE-1", "spec": {"mpn": "SMOKE-1"}, "documentUris": ["mount://smoke/PART/part.pdf"]}}' >/dev/null
	pub="$(rpc agni.v1.dsapi.DatasheetService/PublishDraft '{"mpn": "SMOKE-1"}')"
	# protojson varies its whitespace on purpose, so match with a pattern rather than a literal.
	grep -Eq '"published": ?true' <<<"$pub" || { echo "::error::PublishDraft did not publish: $pub"; exit 1; }
	got="$(rpc agni.v1.param.PartSpecService/BatchGetPartSpecs '{"mpns": ["smoke-1"]}')"
	grep -Eq '"mpn": ?"SMOKE-1"' <<<"$got" || { echo "::error::the published draft was not served: $got"; exit 1; }
	echo "agnids: a draft saved, published and served back through PartSpecService"
	;;
*)
	echo "image_smoke.sh: unknown kind '$kind' (want agni or agnids)" >&2
	exit 2
	;;
esac
