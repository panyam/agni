---
title: "Running the server"
description: "Bring up the web viewer on your own machine with Docker, and bring your designs with you."
---

The [viewer](../../architecture/web-app/) is a server you run, not a hosted service. This page is
about bringing it up on your own machine so a team can open a browser and work, rather than each
engineer installing a toolchain.

If you only want the CLI, [Getting started](../getting-started/) is the shorter path.

## The one-liner

```
docker run -p 8080:8080 -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.1
```

Open `http://localhost:8080`. Your boards are in the file tree on the left, under a mount named
`boards`.

With no `-v` at all it still runs, serving the two demo boards baked into the image, so you can
confirm the thing works before pointing it at your own designs.

The examples on this page pin a version, for the same reason
[Getting started](../getting-started/#install) does: a report is only reproducible if you can say
which build produced it. Every published version is on
[the package page](https://github.com/panyam/agni/pkgs/container/agni), and `:latest` tracks the
newest if you would rather not pin. `agni version` inside the container always tells you what you
actually have.

## Bringing your designs

Every subdirectory of `/workspace` becomes a mount named after itself. One `-v` per folder, no
flags:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v ~/datasheets:/workspace/datasheets \
  ghcr.io/panyam/agni:v0.1.1
```

That gives two mounts, `boards` and `datasheets`. The mount is the containment boundary: the
browser addresses files as a mount name plus a mount-relative path, never an absolute one, and the
server rejects any path that escapes its mount.

{{ includeFile "figures/mount-topology.svg" }}

Dotfile directories are skipped, so a `.git` sitting in a bind-mounted parent does not show up as a
design folder.

## Without Docker

The image exists because it carries the pieces a server needs beyond the engine: the viewer's built
assets, and the symbol libraries. Running the binary directly means supplying the first of those
yourself.

```
agni serve --addr :8080 --mount boards=~/boards --web-dir /path/to/agni/web
```

`--web-dir` is the directory holding the viewer's own `templates/` and its built `static/*.js`, not a
folder of designs. It defaults to `web`, which is where a repo checkout keeps them, so from a checkout
you can leave it off. From anywhere else there is no relative answer, and the run stops with
`--web-dir "web" is not a directory` rather than serving a broken page.

Two ways to avoid typing it every time:

| where you put it | what it suits |
|---|---|
| `web_dir:` in an `agni.yaml` ([machine configuration](../cli-reference/#machine-configuration-agniyaml)) | per-directory, and it travels with a checkout |
| `AGNI_WEB_DIR` in the environment | an installed binary whose assets sit at a fixed path |

A run says on stderr when the value came from the environment, because an `AGNI_WEB_DIR` exported
months ago outlives the memory of exporting it.

**For one design, reach for [`agni open`](../cli-reference/#open-design) instead.** It works the
assets out for itself, picks a free port, and prints the URL of the board rather than of a file
browser. `agni serve` is for several designs at once, or for a server other people reach.

## Symbol libraries are already there

A schematic that names rather than embeds its symbols needs the symbol library before it can
resolve to a pin-level {{ explainable "netlist" }}. The image ships the KiCad, xschem, and gEDA
symbol libraries and points `--symbol-path` at all three, so external symbols resolve with nothing
to install.

This is worth knowing because the failure it prevents is a quiet one. Without the libraries the
design still reads, it just reads short: fewer components, fewer nets, and therefore fewer
findings, with no error to tell you so. If your component or net counts look low, that is the first
thing to check, whether you are running in Docker or not.

The libraries are most of the image's ~310MB. That is the trade being made deliberately: a smaller
image that renders your board wrong is the worse default.

## Running one-shot commands

The image's entrypoint is the `agni` binary and the server is only its default command, so any
subcommand works with the same environment the server has:

```
docker run -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.1 \
  check /workspace/boards/board.kicad_pro --format json
```

This is mostly for CI, where a container is easier than provisioning a Go toolchain, and where it
matters that the gate and the viewer run identical engine and symbol data. At your own desk,
installing the CLI is nicer. See [the CLI reference](../cli-reference/).

## Health

`GET /healthz` returns 200 while the server is up. The image declares a `HEALTHCHECK` that calls
`agni healthcheck` against it, so an orchestrator gets liveness with no extra tooling in the image.

The probe reports that the process is serving HTTP and nothing more. Mounts, the rule catalog, and
the parameter set are all validated at startup, so a misconfigured server fails before it ever
listens. A 200 is not evidence that your `--profile-path` was right.

## House rules for the whole server

The catalog overlay is deployment configuration, applied once at startup to every rule-running
surface:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v ~/houserules:/etc/agni \
  ghcr.io/panyam/agni:v0.1.1 \
  serve --addr :8080 --mount-root /workspace \
        --profile-path /etc/agni/profiles \
        --conventions /etc/agni/conventions.yaml
```

Overriding the command like this replaces the default arguments, so `--mount-root` has to be
repeated. The symbol paths do not, because they come from the environment.

Note what an overlay means here. A profile named after a built-in supersedes it for **every**
design this server reads, so point it at rules that suit the whole mounted set rather than one
project. See [interface profiles](../interface-profiles/) and
[naming conventions](../naming-conventions/).

## Keeping review runs

`serve --review-store <dir>` turns review runs into things the server keeps. Without it the server
still serves everything else, and the review endpoints answer that they store no reviews rather than
running a checklist and dropping the result.

The store is a directory the server writes to, so in a container give it its own volume:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v agni-reviews:/var/lib/agni/reviews \
  ghcr.io/panyam/agni:v0.1.1 \
  serve --addr :8080 --mount-root /workspace \
        --review-store /var/lib/agni/reviews
```

It is deliberately a different volume from your board folders.

<details>
<summary>Why the store sits outside your design mounts, and what one stored run is</summary>

Design mounts are read-only, and keeping runs somewhere else is what preserves that: nothing the
server saves ever lands beside your schematics. A named volume also survives `docker rm`, without
which storing runs would be pointless.

Each run is one file, written in the same format `agni review --results-out` produces, so the volume
stays readable with ordinary tools and a run can be copied out and rendered anywhere. A run records
the checklist it scored, not a pointer to it, so editing your `review.yaml` afterwards never
rewrites what an older run says it asked.

</details>

Two things to know before you rely on it. Runs are visible to every client of the server, because
`agni serve` has no authentication yet, so treat the store the way you treat the mounts: fine for one
team on a trusted network, not a boundary between teams. And nothing prunes it, so a CI job creating
a run per commit will grow the volume until you delete runs yourself.

## Writes and file ownership

The datasheets workbench writes back into a mount: saving a PartSpec or an annotation lands a file
in your bind-mounted folder. The container runs as a non-root user (uid 10001) so those files are
never written as root.

On **Docker Desktop** (macOS, Windows) that is the whole story. Its file-sharing layer maps
ownership to you, so a file the container wrote as uid 10001 appears on your host owned by your own
account, and there is nothing to configure.

On **Linux**, bind mounts pass ownership through unchanged, so those files land owned by uid 10001
and you may not be able to edit them afterwards. Run as yourself instead:

```
docker run --user $(id -u):$(id -g) -p 8080:8080 -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.1
```

## What is not in the image

Two capabilities shell out to external programs and are deliberately left out, because both are
large and neither is needed to read, check, diff, or query a design.

- **Native golden renders.** `agni native render/open` drives the format's own tool (`kicad-cli`,
  xschem, Lepton). See [native verification](../../build/native-verification/).
- **Datasheet extraction.** The `/datasheets` workbench's "Extract (first pass)" action shells out
  to a doc-IR producer configured with `--pdf2doc`. Transcribing parameters by hand and reading an
  already-extracted doc-IR both work without it.

## A shared deployment

Everything above assumes one engineer, one machine. There is no authentication, no per-user
scoping, and no session isolation: anyone who can reach the port sees every mount and can write
through the datasheets workbench. That is the right trade for localhost and the wrong one for a
shared host. Put it behind something that authenticates before you expose it.
