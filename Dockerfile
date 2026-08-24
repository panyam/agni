# The packaged agni server: the engine, the web viewer, and the symbol libraries the readers
# need, in one image an engineer can bring up on their own machine.
#
#   docker run -p 8080:8080 -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.0
#
# Every subdirectory of /workspace becomes a mount named after itself (serve --mount-root), so
# bringing your designs is one -v per folder and no flags. With no volumes at all the image still
# serves its own demo boards, so `docker run -p 8080:8080 ghcr.io/panyam/agni` shows something.
#
# ENTRYPOINT is the bare `agni` binary and serve is only the default CMD, so the same image is
# also the CLI, with the same environment the server has:
#
#   docker run -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.0 \
#       check /workspace/boards/board.kicad_pro --format json
#
# That is not only convenience. `go install` gives you the engine WITHOUT the symbol libraries
# below, and a schematic that names rather than embeds its symbols then reads short: the
# component and net counts come out low, the rules evaluate cleanly over the short read, and the
# result is fewer findings with no error to explain them. This image is the artifact that pins
# the engine and the symbol data together, which is what makes an archived report reproducible.
#
# What is deliberately NOT here: the native EDA tool binaries (kicad-cli, xschem, Lepton) that
# `agni native render/open` shells out to, and the Python/docling stack behind the /datasheets
# "Extract (first pass)" action. Both are large and both are reached by shelling out to a
# command, so they belong in their own images rather than inflating this one. See
# Dockerfile.nattools for the tool host.

# ---------------------------------------------------------------------------------------------
# Stage 1: the browser bundle. Node is needed only to produce web/static/*.js, which is a
# gitignored build artifact (web/.gitignore), so it cannot simply be copied from the context.
# ---------------------------------------------------------------------------------------------
# $BUILDPLATFORM, not the target: the bundle is JavaScript and identical on every architecture, so
# building it natively lets a multi-arch build produce it once instead of once per platform under
# emulation.
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS web
WORKDIR /src/web
# Manifest first so a dependency-unchanged rebuild reuses the install layer; the source copy
# below is what actually churns.
COPY web/package.json web/pnpm-lock.yaml ./
# pnpm is pinned here rather than taken from corepack's default, because web/package.json has no
# packageManager field to pin it. The major must match the lockfile (lockfileVersion 9.0 = pnpm
# 9 or 10); --frozen-lockfile then fails loudly on a mismatch instead of silently resolving
# different dependency versions than a local build.
RUN corepack enable && corepack prepare pnpm@10.14.0 --activate
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---------------------------------------------------------------------------------------------
# Stage 2: the engine binary. CGO off so it runs on the distroless-ish runtime below with no
# libc coupling to the builder.
# ---------------------------------------------------------------------------------------------
# $BUILDPLATFORM with an explicit GOARCH below: the binary is CGO-free, so Go cross-compiles it far
# faster than emulating the whole toolchain on the target architecture.
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
# VERSION stamps the build's identity. It must be passed explicitly here, unlike every other way
# agni is built, because .dockerignore excludes .git to keep the context small: the toolchain has
# no repository to read, so it records neither a vcs.revision nor a module version and
# internal/version would resolve to "unknown". That string is not cosmetic — it is what a results
# document names as its producer — so the image would otherwise write reports that cannot say
# which build made them, in the one artifact whose whole point is pinning engine and symbol data
# together. `make image IMAGE_TAG=v0.1.0` passes it; a bare `docker build` gets "dev".
ARG VERSION=dev
WORKDIR /src
# Module files first, so `go mod download` caches independently of source edits.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# TARGETARCH/TARGETOS are supplied by buildx per target platform. They are the ONLY thing that
# differs between a linux/amd64 and a linux/arm64 build of this image, since everything else it
# carries is architecture-independent data.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath \
      -ldflags "-X github.com/panyam/agni/internal/version.stamped=${VERSION}" \
      -o /out/agni ./cmd/agni

# ---------------------------------------------------------------------------------------------
# Stage 3: the symbol libraries. These are DATA, not tools: --symbol-path wants directories of
# .kicad_sym / .sym / xschem device files, and needs no EDA binary to read them. Debian draws that
# line for KiCad, shipping kicad-symbols separately from kicad itself. It does not for xschem or
# Lepton, whose symbols ship inside the tool package, so this stage installs those tools purely to
# harvest their symbol trees.
#
# That is exactly why this is a separate stage. Only /symbols is copied forward, so the tool
# binaries, apt's package lists, and the caches are all discarded and the runtime image below
# carries the symbol data without the tools.
# ---------------------------------------------------------------------------------------------
# $BUILDPLATFORM again. Symbol libraries are text files describing parts, identical on every
# architecture: kicad-symbols is even `Architecture: all` in Debian, and only /usr/share data is
# taken from the xschem and Lepton packages. Emulating this stage per target would cost minutes of
# apt under QEMU to produce byte-identical output.
FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS symbols
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      kicad-symbols \
      xschem \
      lepton-eda \
 && rm -rf /var/lib/apt/lists/*
# The gEDA/Lepton symbol library is spread over subdirectories, and the resolver does not recurse
# into them, so flatten it into one searchable directory. -n keeps the first of any duplicate
# basename rather than letting a later subdirectory silently win.
RUN mkdir -p /symbols/geda \
 && find /usr/share/lepton-eda/sym -name '*.sym' -exec cp -n {} /symbols/geda/ \; \
 && mkdir -p /symbols/kicad /symbols/xschem \
 && cp -r /usr/share/kicad/symbols/. /symbols/kicad/ \
 && cp -r /usr/share/xschem/xschem_library/devices/. /symbols/xschem/
# Carry each library's license text alongside it. These are third-party works under their own
# terms (the KiCad libraries under CC-BY-SA with a design exception, the xschem and Lepton symbols
# under the GPL), redistributed verbatim here; the engine itself is Apache-2.0.
RUN mkdir -p /symbols/licenses \
 && cp -r /usr/share/doc/kicad-symbols/copyright /symbols/licenses/kicad-symbols.copyright \
 && cp -r /usr/share/doc/xschem/copyright /symbols/licenses/xschem.copyright \
 && cp -r /usr/share/doc/lepton-eda/copyright /symbols/licenses/lepton-eda.copyright

# ---------------------------------------------------------------------------------------------
# Stage 4: the runtime.
# ---------------------------------------------------------------------------------------------
FROM debian:bookworm-slim
# ca-certificates only; nothing here reaches the network, but a future overlay fetched over HTTPS
# would fail confusingly without them.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build   /out/agni            /usr/local/bin/agni
COPY --from=symbols /symbols             /usr/share/agni/symbols
# serve's positional argument is the web ASSETS dir: the server-rendered templates plus the built
# bundle. checkWebAssets validates both at startup, so a broken copy here fails fast and by name
# rather than 404ing on the first request.
COPY            web/templates            /srv/agni/web/templates
COPY --from=web /src/web/static          /srv/agni/web/static
# The demo boards, so the image is useful with no volumes mounted at all.
COPY            demo                     /workspace/demo
COPY            LICENSE NOTICE           /usr/share/agni/

# Run as a non-root user. This matters beyond general hygiene: the datasheets workbench writes
# back into a mount (SavePartSpec / SaveAnnotations), and as root those files land in the
# operator's bind-mounted host directory owned by root. Override with `docker run --user $(id -u)`
# to have writes land as your own uid.
RUN useradd --system --create-home --uid 10001 agni \
 && mkdir -p /workspace \
 && chown -R agni:agni /workspace
USER agni

WORKDIR /srv/agni
EXPOSE 8080

# The symbol libraries reach EVERY subcommand, not just the one in the default CMD below.
# Overriding CMD (`docker run <image> check ...`) replaces the whole argument list, so passing
# --symbol-path there would silently lose it for exactly the CLI use this image is meant to
# support, and a symbol-short read reports fewer findings without erroring. The environment is
# the only channel that survives a CMD override; an explicit --symbol-path still wins over it.
ENV AGNI_SYMBOL_PATH=/usr/share/agni/symbols/kicad:/usr/share/agni/symbols/xschem:/usr/share/agni/symbols/geda

# A liveness probe the orchestrator can act on. It reports that the process is up and serving
# HTTP; it does not re-check mounts or the catalog, which are startup-validated (see
# healthHandler in cmd/agni/serve.go).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["agni", "healthcheck"]

# ENTRYPOINT is the bare binary so any subcommand works; CMD is only the default. Overriding CMD
# (`docker run <image> check ...`) replaces these serve args entirely, which is the intent, and
# the symbol paths survive it because they come from the environment above rather than from here.
ENTRYPOINT ["agni"]
CMD ["serve", "--addr", ":8080", "--mount-root", "/workspace"]
