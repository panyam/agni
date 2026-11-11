# web/browser

This file covers the browser specs, the walks they drive, and the `make exercise-*` targets that run them.

## Running specs

- **`make browser-test` is IN the gate.** It needs a Chromium on the machine (`cd web && pnpm exec playwright-core install chromium`).
- It drives a real browser against a real server for assertions that need layout. jsdom has no layout, and a panel can be present in the DOM and invisible to a reader. `docsite/content/build/the-gate.md` says what belongs here, and `build/evidence.md` gives two ways a layout assertion passes while proving nothing.
- **`HEADED=1` shows any spec in a visible Chrome** (`make browser-test HEADED=1`). `SLOWMO=<ms>` paces it.
- **One spec run by hand uses the `web/static/agni.wasm` and `bin/agni` on disk** (`cd web && pnpm exec vitest run -c vitest.browser.config.ts browser/<spec>`). `make browser-test` rebuilds both. Run `make wasm agni` after a Go change first (agni 1136).
- The suite's own server is built with the same build id as `make wasm` (`serve.ts`), so its seeds match the page's `agni.wasm` (agni 1525). A wasm built before a later edit misses every seed, and `wasm.spec.ts` then sees the board fetched twice. Run `make wasm` again first.
- **A spec that times out on CI in a different test each run often clicks inside a boot window.** Locally that window is about 20 ms wide. Hold the wasm download back with `page.route` and sample the element's state every 20 ms from first paint.
- A viewer change that breaks a walk fails it on that PR. Update the spec there and do not loosen it (agni 1118).

## Headed and headless

- **A headed run is a different page from a headless one** (agni 1124).
- A headed page takes the window's own size (`viewportFor` in `web/browser/browser.ts`). A fixed 1000 px page in a shorter window makes Chrome pan to every click.
- The guided walk freezes the page while a panel waits, and blocks clicks while a step runs.
- The walk's own clicks lift that shield through `act()`. `Input.setIgnoreInputEvents` drops Playwright's dispatched input as well as the reader's.
- A query tab opens in the page (`openTab`). A real click on one times out in a scrolled strip.
- Headless CI uses 1400x1000 and never pauses.
- **A Playwright click scrolls its target into view first.** It can pass a tour step that a person cannot take. Each tour step scrolls its control into view (`reveals` in `web/src/reviewtour.ts`), and the walk fails a released step with no hole. Look at a `page.screenshot` of each step after its release before trusting a tour change.
- A `:has()` locator over a large answer is re-resolved on every action and costs seconds on Jetson. Find a row by index.

## Picking and panels

- **A schematic resistor's body is a hollow outline.** Only the outline takes a click, and the picker probes a ring and prefers a pin. `clearPoint` (`picks.ts`) finds no point inside it. A spec clicking one searches its outline for a point whose ring touches the part alone (`outlinePoint` in `walk-mission.spec.ts`).
- **The dock's boot sizes are fractions with floors** (`web/src/dock.ts`, agni 759). Query opens at a fifth of the height, at least 320 px and at most 45%. The east rail opens at least 260 px wide.
- **A layout the browser saved keeps its old sizes** until the reader picks Panels → Reset layout (`resetLayout`). Check a dock change in a fresh profile.
- A spec that needs a taller Query panel drags the sash (`growQueryPanel` in `web/browser/panels.ts`). Moving the panel's overlay leaves the sash across the result rows, where it takes their clicks.

## Walks and exercises

- **`web/browser/review-walk.spec.ts`** walks the viewer's review tour (`web/src/reviewtour.ts`, `?tour=review&tourpace=host`, tsappkit's `Tour` over the events in `viewerevents.ts`). It runs in the gate on gateway. `make exercise-browser-review` runs it alone on Jetson, and `WALK_DESIGN=gateway` picks the small board.
- That target is HEADED by default, and the reader does each tour step. `STEP_BY_STEP=0` has the walk do them. An agent passes `HEADED=0`, or the run waits on a reader. Headless, the walk acts through the tour's holes, so a broken hole or event fails the gate.
- **`make exercise-cross-probe` runs `web/browser/cross-probe-walk.spec.ts`.** The gate walks a cut of Jetson (`examples/cross-probe-project`). `WALK_DESIGN=clay` walks Clay R6, an OrCAD design on its Allegro board (agni 1382).
- The cross-probe walk names, per step, the tickets it waits on. A waiting step that passes FAILS the walk. The PR that makes a step pass also removes its wait.
- The make target asks GitHub which of those tickets are closed (`hack/closed_tickets.sh`). A step waiting only on closed tickets must pass. The gate runs offline and skips that check.
- **`make exercise-webgl` runs `web/browser/webgl-walk.spec.ts`** in SVG and then in WebGL (agni 1330).
- It lines the two views up by fitting their label anchors (`fitFrame` in `browser/frametimes.ts`), so a click hits one place in both. A zoom whose refit finds too few anchors moves the old frame by the zoom (`zoomFrame`). Keeping the old frame makes later clicks drift.
- The walk waits for `data-busy` to clear (`idle`) before acting.
- Its timings mean something only headed on an idle machine. Headless Chrome draws WebGL in SwiftShader, so the gate's cut run never judges them.
- Its compare step needs Jetson's rev B, which `make exercise-webgl` builds. It clicks the first change, because the first sheet pair need not draw one.
- Its last steps pan a board view beside the schematic. A headless Jetson walk takes most of an hour, and `WALK_TIMEOUT_MS` in the spec bounds it.
- **`make exercise-board-open` runs `web/browser/board-open.spec.ts`** (agni 1500, mission #1499). It times Jetson's board to its first WebGL draw against a fresh server cold, reloaded and from a new browser, then in the browser engine cold and reloaded. It judges medians of `RUNS` against 1.5 s cold and 0.5 s reloaded. The gate opens the cut once and judges nothing.
- Each row opens with every page's startup timeline, from its `agni boot` line (agni 1524): scripts, each wasm worker loaded, the engine ready, the first request and the draw, on that page's own clock.
- Each row keeps only the requests that started before the draw. The page's clock restarts at the `/load` redirect, so the spec places a line by its arrival less its `wallMs`. Every open's lines go to `board-open-lines.json` beside the table.
- **Read server cold on macOS by its fast cluster** (agni 1531). On Apple Silicon a fresh server's first board parse runs about 6x slower roughly half the time: Jetson's `build.board` takes about 0.15 to 0.2 s or about 0.9 to 1.4 s, while the machine sits mostly idle. The same CPU-bound tokenizer runs slower, with the same GC and the same work, so the OS has most likely put the woken request thread on an efficiency core. A CLI parse and an in-process loop never split. `go run ./tools/coldparse -n 10` reproduces it outside the exercise.
- **`make exercise-allegro` and `make exercise-altium`** each run their example, then open the native board in the viewer and click a pad (`web/browser/native-board.spec.ts`, agni 1383).
- **`make exercise-walk` runs `web/browser/walk-mission.spec.ts`** (agni 1338). Each step has an engine row that asks `WalkDesign` and a viewer row that drives the Trace tab's walk by the names the spec lists (agni 1341).
- The gate walks the cut from R119. `WALK_DESIGN=jetson` walks from R361, because every pull-up on U80's 1.8 V side is do-not-populate. The cut holds no pull-up, so its pull-up step expects no route.
- Other exercises are `make exercise-pin-functions`, `make exercise-revision-audit`, `make exercise-mcp-ask` and `make exercise-viewer-page`.
- **`make exercise-public-demo`** walks the built static demo. With `DEMO_URL` it walks the deployed one.
- Against the deployed demo, wait for the `docs.yml` run and then about a minute more. Pages serves the design files with `max-age=600`, and a fresh deploy's files are uncached, so the first walk after a deploy fetches Jetson's board slowly. The open row names the slowest file fetch, and a timed-out open names the files still pending (agni 1002).
