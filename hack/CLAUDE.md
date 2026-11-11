# hack

This file covers the release bundle, the images, the native tools, the fetched sample boards and the seed triage records.

## Release trial and images

- **A release is tried through `deploy/compose.yaml` on a machine whose boards we never see** (agni 1057). `make compose-release V=...` pins that file, and the release attaches it. Its `trial` service runs `agni trial`.
- **`agni trial` reports no design content.** Its `feedback.Report` has no field for it. It names a rule only through `feedback.Shipped` against `service.ShippedRules()`, since a project's rule namespace can name the customer.
- `feedback.PinMapTally` counts a design's pin map (agni 1196).
- **An unchecked declared function travels as a KIND, since the sentence names parts.** `intent.FunctionReasonOf` reads the KIND off the rule's fixed phrases. A new not-considered path in `io-map-function` builds its reason from a new phrase, or it reads as `other`.
- `hack/compose_smoke.sh` runs the bundle in the `images` and `release` workflows. Pipe `compose run` output only with `-T`, or it arrives empty.
- **One release tag publishes two images.** `Dockerfile` builds `ghcr.io/panyam/agni` with the engine, viewer and symbol libraries. `Dockerfile.agnids` builds `ghcr.io/panyam/agnids` with the workbench and the docling stack. `make image` and `make dsimage` build them.
- The `images` workflow builds and smoke-tests both images through `hack/image_smoke.sh` on a PR that touches their source files. `release` runs that script against what it published.
- **The agnids image prefetches its models by RUNNING pdf2doc over a synthetic page**, and `make setup` (`datasheet-models`) does the same. Docling reads the Hugging Face cache, and `docling-tools models download` fills a different one (agni 786).

## Native tools

- `make natrender FILE=... OUT=...` and `make natopen FILE=...` drive the native tools in the `natup` container. Both take paths INSIDE the container, so a path must fall under a `NATIVE_DOCKER_MOUNTS` dir.
- The image carries xschem, Lepton and gerbv, and not KiCad. An oracle's `kicad-cli` comes from a KiCad install on the machine. KiCad's Ubuntu PPA has no arm64 build, and Flathub's does.

## Fetched sample boards

- **Some boards are FETCHED, not committed.** `make samples` pulls a pinned tarball from `panyam/agni-samples` into the gitignored `tools/samples/`, and `testall` depends on it. Those designs carry their own licences, so fetching them keeps this repo uniformly Apache-2.0.
- `hack/samples.pin` holds the version and one checksum per artifact. Each artifact has its own stamp, so `make samples` and `make samples-oracle` compose.
- **There is no offline escape hatch, by design.** Every failure path exits non-zero, and a test reading the corpus fatals rather than skips. A silent missing corpus would let its tests pass over an empty set.
- The gate takes `samples-oracle`, the 19MB both-views corpus. The cross-view tests read a board file, and a test whose fixture is absent skips.
- The gate fetches `tutorial-board`, `oracle-corpus`, `odb-altium` (foxBMS's Altium-written ODB++ jobs), `allegro` (the BeagleBone AI with its Allegro extract and copper films) and `cadence-exports`. `make odb-oracle` adds `odb-kicad`.
- **A pin bump needs the samples release to exist first**, or every CI job fails at the fetch. A samples PR merges and is tagged before the agni PR that pins it goes green.
- **agni-samples' CI reads every design with its pinned agni RELEASE.** `AGNI_VERSION` in its `verify.yml` names that release, and the README calls it the oldest supported version. A samples change that needs a newer reader fails there. The order is then an agni release, a samples PR raising that floor, merge, tag samples, then the pin (agni 1382).
- **A samples release reuses the previous file for every artifact it does not change**, renamed, byte for byte, and checked against the old `SHA256SUMS`. A rebuild on another machine records new file times and moves every checksum.
- **A pin bump restamps every tutorial capture over `tools/samples/`.** Commit that stamp-only diff with the pin. Two branches that both move the pin conflict there and in `hack/samples.pin`.
- `kicad-cli` writes a `.kicad_prl` beside any board it exports. An export run over `tools/samples` or a fixture folder leaves one there, and both export scripts remove it.
- The oracle corpus carries a `design.yaml` per real design, naming the entry, the board companion and the internal connectors. The tutorial tarball carries none. That tarball leaves the boards out, and a declared companion missing from a tarball fails every read.
- **A board whose repo states no licence is fetched from its publisher, never shipped** (agni 1446). The `remote-boards` artifact carries `remote/remote.pin` alone. Each line is a URL at a fixed commit and a sha256.
- `hack/fetch_remote.sh` is the twin of agni-samples' own script. It downloads each file into `~/.cache/agni/samples/<sha256>` and copies it into `tools/samples/remote/<board>/`. It never downloads a cached file again.
- `make samples-clean` lists files unused for 30 days and deletes them only when you answer y.
- **`panyam/agni-samples` builds the tarballs with its `make dist`**, on Linux or macOS. Any walk over `tools/samples/` skips `._` names, since a macOS AppleDouble shadow sorts ahead of the file it shadows (the oracle's `globCorpus`, agni 812). That Makefile runs under `pipefail`, so an empty tarball fails `make verify`.

## Seed triage

- **The public demo's seeds have a verdict for every finding** in `hack/seed-triage/<design>.triage.yaml` (agni 857). `TestSeedTriage` fails on any finding that appears, disappears or sits untriaged.
- A change that moves a seed's findings regenerates the records with `make seed-triage`. Then classify each new group as `defect`, `design-choice`, `needs-intent`, or `false-positive` with its ticket.
- A class is a judgement in its `why` unless an oracle backs it. KiCad's own ERC and DRC are one oracle (agni 967).
- `hack/kicad_oracle.sh` writes `<seed>.kicad.oracle` beside each record. A group for a rule that KiCad mirrors says `kicad: agrees` or `disagrees`. Findings that only KiCad reports go under `kicad_only`. `TestSeedTriageAgainstKiCad` holds all of it.
- **Draw a planted copper fault as a near miss, never an overlap** (agni 1082). KiCad reads overlapping copper as one island and names it inconsistently between runs.
- **`make seed-triage` rewrites every record file whole.** It reorders keys and requotes `'*'`. A change that only REMOVES records edits them by hand and lets `TestSeedTriage` confirm the result.
- Restore the record files before running the update a second time. It diffs against the file on disk, so the records the first run removed would turn into `untriaged` groups.
