# cmd/agni

This file covers the `agni` CLI, its `agni.yaml` config, its output flags and the traps in its tests.

## Config and private designs

- **Nothing here takes a private path.** A private workspace never reimplements a target. Tier-1 config goes in an `agni.yaml` and covers mounts, symbol paths and native tools. The CLI looks for that file by walking up from the working directory, then in `~/.config/agni/`. So `agni check mount://corpus/...` works from anywhere with no flags. `cmd/agni/envconfig.go` guards the tier boundary.
- **Everything else is a variable on a make target.** Examples are `EXTRA_MOUNTS` and `OVERLAY_FLAGS` on `serve`, `DESIGNS` and `OVERLAY_DIR` on `dockserve`, `NATIVE_DOCKER_MOUNTS` on `natup`, and `DATASHEET_DIR` on the datasheet targets. A workflow that exists only as a wrapper in someone's local Makefile means this repo lacks a target.
- **A relative path in `agni.yaml` resolves beside the file**, not against the working directory (agni 1373).
- **A flag wins outright over `agni.yaml` and does not merge.** A Makefile default that passes `--mount` shuts the file out. `make serve MOUNTS=` hands the mount table back to the file.
- **The four keys are `mounts`, `symbol_paths`, `web_dir` and `native_tools`.** Without a `web_dir`, `--server self` cannot run outside a checkout. The lookup falls back to a relative `./web`, and a run from a design folder fails with `--web-dir "web" is not a directory` before it writes the artifact (agni 637). `web_dir` must name a BUILT bundle, because `checkWebAssets` stats the viewer's templates and `static/*.js` before the listener opens.
- **An unknown key in `agni.yaml` is a hard ERROR.** Every lane, every released binary and the container image share the file. So a new key goes in only after every reader on the machine understands it.
- **`agni <cmd> --help` proves nothing about a config key.** It short-circuits before the config loads and accepts anything. Probe with a command that reads a design, and run the same probe with a bogus key first as a positive control.

## Commands

- **The commands include `agni stats|check|diff|render|query|trace|review|serve|open <file>` and `agni params <mpn>`.** `open` serves ONE design, mints its mount and prints its URL. `serve` takes `--mount` per folder and `--web-dir`.
- **The reader is chosen by extension, case-insensitively.** `.xml` and `.sch` are sniffed by root element or header.
- **`--symbol-path <dir>` searches each dir's SUBTREE** for external symbol files, so a dir can be a library root.
- **A command reads a design through `readDesign` or a service, never a bare `newLoader().ReadDesign`.** The design's PROJECT config enters the read in `readDesign`. Six commands have no service between them and the read: stats, diff, emit, render, intake and profilediag. Net roles resolve once at ingestion, so a read that skips `readDesign` uses the built-in naming vocabulary and none of the project's symbol libraries (agni 228).
- **`stats`, `diff` and `render` take `--conventions`** for a convention not yet in the project (agni 1275). `withConventions` sets `readOverlay` for one command, and `designOverlay` and `ingest` read it. A new command with that flag calls `withConventions`.
- **A rule NAME missing from the catalog is refused** by `--rule`, `CheckDesign` and `GetCheckReport` alike (agni 1121, `check.Catalog.CheckNames`). The refusal offers the nearest names and names a superseded rule as superseded. Some rules exist only under some config, such as `intent/exposure-declared`, which needs a declared exposure. Such a rule is named only on a run that has that config.
- **`diff --rename-approx` is OFF by default.** Without it, a net that was renamed AND changed reports as New plus Deleted. The pass ASSIGNS a best match and does not recover a fact. A run without the flag reads as "we detect no approximate renames", so check the flag before calling a missing rename an engine gap.

## Output

- **A command's `--format json` is protojson of that command's WIRE MESSAGE** (C31). A script reading the CLI and a client reading the rpc parse one shape. `TestEveryJSONFormatEmitsAProto` enforces it over this package's source. `TestNoCommandCallsAHandRolledEncoderInAnotherPackage` follows the commands' calls into the module's other packages (agni 734).
- **`agni intake` and `agni trial` are the two declared exceptions.** `intake.Skeleton` and the `feedback` package each carry a structural confidentiality guarantee. A proto twin would carry that type into a file that people edit for other reasons.
- **`review --format json` is the `Review` that the rpc returns**, with a `summary` computed on the way out.
- **A failed request under `--format json` is written as the Connect error** (`code`, `message`, `details`) through connect's own error writer in `cmd/agni/rpcerror.go`. A new service call in a command returns its error through `rpcFailed`, or a script gets text where it expected JSON. An unmarked error, such as a flag mistake or a set's after-the-answer status, is text only on purpose.
- **`GetDesign` serves the IR on request** (agni 836). A request sets a `read_mask` over the response (`design`, `design.nets`, `design.components.mpn`, `*`) plus `nets`/`ref_des` filters. The CLI form is `stats --format json --mask`. UNMASKED it returns the viewer's summary with no IR, against AIP-157's "empty mask is everything", because the viewer calls it on every open.
- **A check run is SAVED through `CheckService.RenderCheckReport`, from the run the caller holds** (agni 127). `check --results-out` writes the `CheckResults` document through it. `check --verdicts --format html` and the viewer's Save report and Save JSON write through it too. A new way to save a run calls it and does not render on its own.
- **The saved document's JSON is protojson.** Its whitespace varies between builds on purpose, so two builds' documents compare as parsed JSON, never as bytes. One build's two hosts compare as bytes (`TestWasmEngineAnswersAsTheServerDoes`).
- **`-o/--out` writes the `--format` output to a file** on `check`, `review`, `trace` and `query`. `-` means stdout and is the default. The file is created at once, so a bad path fails early. A run that wrote nothing removes it and announces nothing (agni 1342). The written-file note goes to stderr, so `-o` composes with a pipe.
- **`-o` differs from `--results-out`.** `--results-out` writes the check-result DOCUMENT, and `agni results` re-renders it.
- **`review --coverage` renders as markdown ALONE and refuses an explicit `--format`.** Coverage and format are different axes, and the render switch tests coverage first. For a coverage page, use the per-item `--format html`, whose header band carries the same rollup.
- **`render --report` prints to stdout and IGNORES `-o`.** The report replaces the drawing and is not a format of it. It exits 0 having written no file.
- **The `--verdicts` list caps NON-ACTIONABLE rows at twenty per rule.** It states the elided count and never elides a fail or an inconclusive. csv and html are not capped.

## Remote

- **`--remote <url>` on `query`, `check`, `trace` and `review` sends the request to a running `agni serve`** (agni 998). The command renders the response exactly as a local run does, and `TestRemoteAnswersAsTheLocalCommandDoes` holds the two byte for byte. `--server` is a different flag and only says where LINKS point.
- **The design's files go with the request** (agni 1061). The client sends a manifest of the files `ListDesignFiles` would list, under this machine's mount names. Then it sends the contents the server reports missing, through `BlobsMissing` and `UploadBlobs`. The server needs no shared filesystem. An unreachable server is an error, never a local read.
- **A server reads an uploaded design through a host and a cache of its own per manifest.** The upload keeps the client's mount name, so a cache keyed on the URI would serve one upload's reads for another.
- **A remote command refuses flags that configure this process's engine** (`--params`, `--profile-path`, `--timing`) and does not ignore them. A remote review is `CreateReview` with `validate_only`, so the server stores nothing.

## Tests

- **A personal `~/.config/agni/agni.yaml` leaks into tests.** A test that runs `rootCmd()` or the binary reads it, and its mounts or `web_dir` join the run. Point `HOME` and `XDG_CONFIG_HOME` at a temp dir and run from an empty working directory (`TestOpenStillRequiresTheViewer`, `clients/python/tests/conftest.py`). CI has no such file, so the failure shows only on a developer's machine.
- **The CLI's mount table is parsed ONCE per process** (`cliWSOnce`). In this package's tests the first test to resolve a mount fixes it for every later one. A test whose output names a design by its path within a mount, or that needs an `agni.yaml` mount, calls `freshWorkspace(t)` or `withMount` (agni 869). Without it a test passes alone and fails in the package run. Another order dependency is parked as #875, and `-shuffle=on` finds them.
