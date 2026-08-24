#!/usr/bin/env bash
# dockserve.sh — run the packaged agni image as serve's container twin: the same MOUNTS /
# EXTRA_MOUNTS / DESIGNS / ADDR, served from the image instead of `go run`.
#
# Driven by `make dockserve`, which supplies every input below as an environment variable. Run it
# directly only if you are reproducing what make does; the Makefile is where the defaults live.
#
# Each "--mount NAME=PATH" becomes "-v PATH:/workspace/NAME", and the image's own CMD already passes
# --mount-root /workspace, so nothing needs a --mount flag inside the container.
#
# THREE of serve's parameters cannot be forwarded, because the image does not contain what they
# describe:
#
#   NATIVE_TOOLS  no kicad-cli/xschem/Lepton inside (see the Dockerfile header). Use serve, or
#                 reach the tools through the nattools container.
#   PDF2DOC       no Python/docling inside, and the value is a host path.
#   OVERLAY_FLAGS host paths that do not resolve in the container. Pass OVERLAY_DIR instead: the
#                 folder is mounted at /overlay and the flags are rebuilt against it.
#
# WHETHER THAT IS A HARD ERROR OR A DROPPED FLAG DEPENDS ON WHO SET THE VALUE. Typed on the make
# command line, it is an instruction, and the honest answer is to refuse rather than to run
# something other than what was asked for. Inherited from the environment or from the Makefile's own
# defaults, it is ambient config for the whole tree: a shell that exports the serve settings, or a
# `make setup` that turned the Extract default on. Refusing those would mean dockserve stops working
# the moment serve is configured, so they are announced and dropped instead. Announced, never
# silent, because the hazard here is a run that quietly describes less than you think it does.
#
# CLI_SET carries that distinction across the boundary: make computes it with $(origin) and passes
# the names the caller typed. A script cannot work this out for itself.
#
# The reason any of this is an error rather than a warning: --symbol-path or --profile-path pointing
# at a directory that does not exist inside the container yields a SHORT read, and a short read is
# the quiet kind of wrong. The rules evaluate cleanly over it and report fewer findings, with no
# error to explain them. See the Dockerfile header for the same argument about `go install`.
#
# SYMBOL_PATH is always ignored rather than ever refused, because the image ships better libraries
# than a host path would name (AGNI_SYMBOL_PATH, baked in stage 3). Refusing it would block a caller
# who simply has the variable set for serve.
set -euo pipefail

# Expand an array that may be empty. Under `set -u` bash 3.2, which is what macOS ships, a bare
# "${arr[@]}" on an empty array is an unbound-variable error rather than zero words. Every array
# below can legitimately be empty (no overlay, no review store), so they all expand through this
# idiom: ${arr[@]+"${arr[@]}"} yields nothing when unset and the quoted elements otherwise.

: "${MAKE:=make}"
: "${DOCKER_NAME:=agni-dockserve}"

# Was NAME typed on the make command line, as opposed to inherited from the environment or from a
# Makefile default? CLI_SET is the space-separated list make computed with $(origin).
typed_by_caller() {
	case " ${CLI_SET:-} " in *" $1 "*) return 0 ;; *) return 1 ;; esac
}

# Refuse a parameter the caller asked for; drop one that merely came along for the ride.
refuse_or_drop() {
	local name=$1 dropped=$2
	shift 2
	if typed_by_caller "$name"; then
		printf 'dockserve: %s\n' "$@" >&2
		exit 1
	fi
	printf 'dockserve: %s\n' "$dropped"
}

if [ -n "${NATIVE_TOOLS:-}" ]; then
	refuse_or_drop NATIVE_TOOLS \
		"ignoring NATIVE_TOOLS=$NATIVE_TOOLS; no native tools in the image." \
		"NATIVE_TOOLS is not available in the image (no kicad-cli/xschem/Lepton inside)." \
		"          Use 'make serve' for native golden renders."
fi

if [ -n "${PDF2DOC:-}" ]; then
	refuse_or_drop PDF2DOC \
		"ignoring PDF2DOC; no Python/docling in the image, so no Extract action." \
		"PDF2DOC is not available in the image (no Python/docling inside), and the" \
		"          value is a host path. Use 'make serve' for the datasheet Extract action."
fi

if [ -n "${OVERLAY_FLAGS:-}" ] && [ -z "${OVERLAY_DIR:-}" ]; then
	refuse_or_drop OVERLAY_FLAGS \
		"ignoring OVERLAY_FLAGS; its host paths do not resolve in the container, so this run uses NO overlay. Pass OVERLAY_DIR=<dir> to serve one." \
		"OVERLAY_FLAGS names host paths that do not exist in the container." \
		"          Pass OVERLAY_DIR=<dir> instead; it is mounted at /overlay."
fi

if [ -n "${SYMBOL_PATH:-}" ]; then
	echo "dockserve: ignoring SYMBOL_PATH; the image ships its own KiCad/xschem/gEDA libraries."
fi

docker image inspect "$IMAGE" >/dev/null 2>&1 || "$MAKE" image

# Absolute path, resolving a leading ~ and treating anything else as relative to the repo root
# (which is where make runs this from). Docker -v takes nothing else.
abs() {
	case $1 in
	/*) printf %s "$1" ;;
	'~'*) printf %s "$HOME${1#\~}" ;;
	*) printf %s "$PWD/$1" ;;
	esac
}

vols=()
for spec in ${MOUNTS:-} ${EXTRA_MOUNTS:-}; do
	[ "$spec" = "--mount" ] && continue
	name=${spec%%=*}
	path=$(abs "${spec#*=}")
	if [ ! -e "$path" ]; then
		echo "dockserve: mount '$name' has no such path: $path" >&2
		exit 1
	fi
	vols+=(-v "$path:/workspace/$name")
done

if [ -n "${DESIGNS:-}" ]; then
	d=$(abs "$DESIGNS")
	if [ ! -e "$d" ]; then
		echo "dockserve: DESIGNS has no such path: $d" >&2
		exit 1
	fi
	vols+=(-v "$d:/workspace/$(basename "$d")")
fi

# The overlay crosses as a DIRECTORY rather than as assembled flags, and the flags are rebuilt here
# against the path it lands on inside the container.
overlay=()
found=
if [ -n "${OVERLAY_DIR:-}" ]; then
	o=$(abs "$OVERLAY_DIR")
	if [ ! -d "$o" ]; then
		echo "dockserve: OVERLAY_DIR has no such directory: $o" >&2
		exit 1
	fi
	vols+=(-v "$o:/overlay:ro")
	if [ -d "$o/profiles" ]; then
		overlay+=(--profile-path /overlay/profiles)
		found=1
	fi
	if [ -f "$o/conventions.yaml" ]; then
		overlay+=(--conventions /overlay/conventions.yaml)
		found=1
	fi
	if [ -z "$found" ]; then
		echo "dockserve: OVERLAY_DIR=$o holds neither profiles/ nor conventions.yaml" >&2
		exit 1
	fi
	echo "dockserve: overlay ${overlay[*]}"
fi

# The review store crosses the way OVERLAY_DIR does, except WRITABLE: stored runs are the one thing
# this server produces rather than reads. Created on the host first so the bind mount does not
# materialize as a root-owned directory.
review=()
if [ -n "${REVIEW_STORE:-}" ]; then
	r=$(abs "$REVIEW_STORE")
	mkdir -p "$r"
	vols+=(-v "$r:/var/lib/agni/reviews")
	review=(--review-store /var/lib/agni/reviews)
fi

port=${ADDR#:}
echo "serving $IMAGE at http://localhost:$port/"
exec docker run --rm --name "$DOCKER_NAME" -p "$port:8080" \
	--user "$(id -u):$(id -g)" ${vols[@]+"${vols[@]}"} ${DOCKER_FLAGS:-} "$IMAGE" \
	serve --addr :8080 --mount-root /workspace ${overlay[@]+"${overlay[@]}"} ${review[@]+"${review[@]}"}
