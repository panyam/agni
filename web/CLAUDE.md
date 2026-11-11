# web

This file covers the viewer's TypeScript, the pnpm workspace it sits in, and how its unit tests run.

## The workspace

- **The web code is ONE pnpm workspace with one lockfile at the root.** It holds `web/` (the viewer), `datasheet/web/` (the workbench, with pdf.js, served by `agnids`) and `web-shared/`.
- `web-shared/` is plain TS with no npm dependency of its own. Both pages import it as `@agni/web-shared/<name>.js`, so neither page pulls in the other's libraries through it.
- Run `pnpm install` at the ROOT. `make ui`, `make web-test` and `make proto-web` cover all three packages.
- The workbench's TS generates from the whole buf workspace. The viewer's generates from `protos/` alone.
- The workbench has its OWN `BasePage.html` with only the style rules it uses, so `datasheet/web` deploys on its own.
- **mcpkit bumps in lock-step.** Bump its six Go modules in the root `go.mod`, `examples/ask-over-mcp`, and `@panyam/mcpkit-rooms` in `web/package.json` together.
- pnpm adds the new mcpkit version to `minimumReleaseAgeExclude` in `pnpm-workspace.yaml`. Commit that file with the bump.
- npm can lag mcpkit's publish job by a few minutes. A 404 right after a tag is not a failed release.

## Running the tests

- **vitest runs a test file without typechecking it.** `make web-test` runs `pnpm run typecheck` first. A test can pass under `pnpm exec vitest run` and fail CI on `Array.at` (the lib target lacks it) or on a mistyped mock callback. Run `pnpm run typecheck` in `web/` before pushing a TypeScript test (agni 1456).
- **Another session's load can fail the web tests.** At a load average of 40 or more, `composition.test.ts`'s 10 s boot hook and `load.test.ts` time out. The WebGL walk then reads Properties while WebGL still says `checking…`. Check `uptime` before reading a red `web-test`. Rerun with `pnpm exec vitest run --maxWorkers=3`, and run the walk again once the load drops (agni 1435).
- **`composition.test.ts` fills the viewer shell's template fields itself.** A new `{{ .Field }}` in `ViewerPage.html` goes in its `servedFields`, or the test throws.
- **A baseline run of `main`'s web code relinks `node_modules`.** Say you stash a branch's `web/` and lockfile to time `main`. Its `make ui` runs `pnpm install` against `main`'s lockfile. After `git stash pop` the old package versions are still linked, and `tsc` reports missing exports. Run `pnpm install` at the root before reading anything else.
- If `make ui` then fails with `_lruCache is not a constructor` in the Solid plugin, a package under `node_modules/.pnpm` is a copied directory and not pnpm's symlink. Plain `pnpm install` sees nothing to change. `pnpm install --force` relinks it.

## Drawing sheets

- **A page opens in WebGL on a hardware GPU, and in SVG otherwise** (`web/src/rendercaps.ts`, DECISIONS.md, agni 1329). Headless Chrome reports SwiftShader, so a headless spec opens in SVG. A spec that reads the SVG view names `?mode=svg`, so a headed run on a GPU tests the same view.
- **The canvas and every sheet view draw through one `SheetSurface`** (`web/src/sheetsurface.ts`, agni 1368).
- A view picks its renderer with a WebGL checkbox in its bar, as the canvas does (agni 1469). A new view opens in the canvas's renderer.
- Native has a checkbox only where the server offers it.
- A WebGL view turns through its camera (`orientTurn`). Seen from below, it draws by `strataDrawOrder`, and a click reads the same order.
- Locked views lock only within one renderer. SVG and WebGL measure a centre in different units.
- The page API's `OpenSheetView` takes a `renderer`.
- A sheet's first draw records the `agni:sheet-drawn` mark (`web/src/drawmark.ts`). The mark names its drawing in `detail.view`, either `canvas` or a view's id. A spec timing the canvas skips a view's draws by that field.
- The canvas writes `data-highlight` and `data-lit` for a highlight.
- **The busy overlay sets `data-busy` at once** and takes a wheel or click while it shows. Once the first sheet draws, the overlay turns into a pill that takes no input (agni 1454).
- **A zone fill blends** (`enableBlending` in `web/src/webgl.ts`, agni 1334). The canvas must stay OPAQUE. The browser composites a WebGL canvas as premultiplied alpha, so a translucent canvas pixel draws white over the page.
- **A board's outlines carry their width in the packed record's `stroke_um`.** WebGL draws a line as triangles once that width spans a canvas pixel.
- Outlines stay line primitives below that width. The picker reads a part's triangles as the part, so a triangle outline would take clicks meant for the pick area.
- **A board opens with its fab rows hidden.** Storage keeps "show every layer" as `[]`, and `null` means never chosen (agni 1486).
- **An idle SVG board view parks as an `<img>`** (`SvgView.parkWhenIdle`, agni 1482). After a second with no pointer over it, the view serialises the svg into a blob image and hides the live svg. Chrome's layerization of a live board svg slows every canvas pan.
- The parked svg remains in the DOM, so a spec finds it as `img.svg-parked + svg`. A pointer or a reveal wakes it.
- **A board view keeps showing the board.** If its board does not place the selected part, the view says so and does not follow to a schematic sheet (`chooseSheet`'s `absent`, agni 1481).

## The visual diff

- **The visual diff draws in the canvas's renderer** (agni 1335). Each side is a `SheetSurface`.
- The union draws the old sheet's removed parts as ghosts (`ghostOverlays` in `web/src/canvas.ts`).
- Each packed sheet sits about its own origin. The two sides mirror pan and zoom in design coordinates (`getCentre`/`setCentre`).
