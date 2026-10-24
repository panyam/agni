// esbuild build for the web bundles. Solid components (.tsx) need Solid's compiler, which plain
// esbuild does not do, so we run esbuild through esbuild-plugin-solid (babel-preset-solid) to
// compile JSX into Solid's reactive runtime calls.
//
// Three app bundles: the viewer (src/main.ts -> static/app.js), the design browser
// (src/browse.ts -> static/browse.js, WS9-049) and the landing page (src/landing.ts), plus the Load
// page's plain-DOM bundle (src/load.ts, agni issue 1072). They are
// separate pages with separate entries so each page downloads only what it uses: the browse page
// carries neither dockview nor the WebGL renderer. The datasheets workbench, and pdf.js with it, is
// built from datasheet/web, its own package (agni issue 744).
import { copyFileSync, mkdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import * as esbuild from "esbuild";
import { solidPlugin } from "esbuild-plugin-solid";

const require = createRequire(import.meta.url);
const watch = process.argv.includes("--watch");

// Pin Solid to a single physical runtime. `solid-js/web` pulls in the reactive core, and
// `@panyam/tsappkit-solid` imports the core too; without pinning, esbuild resolves those to a
// different core file than the app's own `solid-js` import and bundles TWO reactive graphs. When
// that happens a signal created on one graph (an island's `signalView`) is invisible to the
// effects/JSX running on the other, so `island.view.setState(...)` updates silently no-op — the
// file tree never highlights the open file, the sheet tabs never appear, the control bar never
// reflects the active mode. Aliasing every solid entry to one resolved file collapses it to a
// single instance, so cross-island reactivity works.
const solidAlias = {
  "solid-js": require.resolve("solid-js/dist/solid.js"),
  "solid-js/web": require.resolve("solid-js/web/dist/web.js"),
  "solid-js/store": require.resolve("solid-js/store/dist/store.js"),
};

// @panyam/servicekit-client's "browser" field names a script that defines a global and exports
// nothing, and esbuild prefers that field for a browser bundle, so the room client's
// `new GRPCWSClient()` met undefined and no tab could join its design's room (agni issue 1154). The
// alias points it at the package's ES module, resolved from the room client that imports it, since
// pnpm keeps it out of this package's own node_modules.
const roomsResolve = createRequire(createRequire(require.resolve("@panyam/mcpkit-rooms")).resolve("@panyam/massrelay"));
const roomsAlias = { "@panyam/servicekit-client": roomsResolve.resolve("@panyam/servicekit-client/dist/index.mjs") };

// The Solid app bundles, one per page. Each must contain exactly one reactive core (asserted below).
const appBundles = [
  { entry: "src/main.ts", outfile: "static/app.js" },
  { entry: "src/browse.ts", outfile: "static/browse.js" },
  { entry: "src/landing.ts", outfile: "static/landing.js" },
];

const solidBuild = (b) => ({
  entryPoints: [b.entry],
  bundle: true,
  format: "esm",
  outfile: b.outfile,
  alias: { ...solidAlias, ...roomsAlias },
  plugins: [solidPlugin()],
  logLevel: "info",
});

// The wasm bench's page entry (agni issue 852), loaded only by browser/bench.mjs.
const benchBuild = {
  entryPoints: ["src/wasm/bench.ts"],
  bundle: true,
  format: "esm",
  outfile: "static/agni-bench.js",
  logLevel: "info",
};

// The Load page (agni issue 1072), which reads a design before the viewer opens it. It is plain DOM,
// with no Solid core to count, so it is not one of appBundles.
const loadBuild = {
  entryPoints: ["src/load.ts"],
  bundle: true,
  format: "esm",
  outfile: "static/load.js",
  logLevel: "info",
};

const builds = [...appBundles.map(solidBuild), benchBuild, loadBuild];

// The engine worker (agni issues 178 and 863) is goapplib's wasmhost worker, shipped prebuilt as a
// classic script because it loads Go's wasm_exec.js with importScripts. It is copied beside the
// bundles rather than bundled, so the page names it by URL.
// static/ isn't in the repo, and esbuild only creates it when it writes the bundles below, so a fresh
// checkout (CI, the Docker build) needs it made before the copy.
mkdirSync("static", { recursive: true });
copyFileSync(require.resolve("@panyam/tsappkit/wasmhost/worker.js"), "static/agni-worker.js");

if (watch) {
  for (const b of builds) {
    const ctx = await esbuild.context(b);
    await ctx.watch();
  }
} else {
  await Promise.all(builds.map((b) => esbuild.build(b)));
  // Enforce the single-instance invariant the alias exists for: exactly one reactive core in each
  // Solid bundle. A second copy shows up as a second `function createSignal`, and the failure it
  // causes (island setState silently no-ops) is otherwise invisible to tests. Checked per app
  // bundle so a new page is guarded too, not just app.js.
  for (const b of appBundles) {
    const cores = (readFileSync(b.outfile, "utf8").match(/function createSignal/g) || []).length;
    if (cores !== 1) {
      console.error(`build: expected exactly 1 Solid core in ${b.outfile}, found ${cores} — duplicate solid-js instances break cross-island reactivity (see solidAlias above)`);
      process.exit(1);
    }
  }
}
