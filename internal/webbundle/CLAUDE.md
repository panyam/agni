# internal/webbundle

This file covers how an installed `agni` binary finds its viewer, and how scripts start a server.

- **With NOTHING naming a web dir and no `./web`, an installed binary finds its own viewer** (WS16-003).
- **A release binary embeds the viewer** (`-tags embedweb`, package `web`) and writes it under the user cache once.
- **A `go install` binary downloads `agni-web-<tag>.tar.gz` from its release** and checks it against `checksums.txt`. Its module carries no built JS.
- **A build that is not a release tag has neither** (`make agni`, `@main`). `serve` then runs the API alone (agni 735), and `open` refuses.
- **An absent dir is a choice and a broken one is a mistake.** A NAMED dir that is wrong, or a `./web` without its bundle, fails and never reaches for the bundle.
- **`make dist V=...` builds the release tarballs, and `hack/dist_smoke.sh` runs one from an empty directory.** The gate's `dist-check` runs both for the host platform, since a release job can fail on Linux where a Mac passes.
- **macOS tar needs `COPYFILE_DISABLE=1`.** Without it every file gets a `._` shadow, and the template loader reads the shadow.
- **A loose file gets the minted mount `files`, never `local`.** `local` is the browser's drop mount, and the Load page refuses it on the server engine.
- **`agni` does not serve the datasheets workbench** (agni 744). `agnids serve` serves it (`make dsserve`). `agni serve --datasheets-url <url>` makes the landing page link to it.
- **A script starting a server for itself passes `--addr 127.0.0.1:0` and `--web-dir`**, then reads the URL off the banner. The banner names the port the kernel chose (agni 1121).
- **A server started outside the repo root with no `--web-dir` serves the API alone.** Every MCP answer then says it has no link.
