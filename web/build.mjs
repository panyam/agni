// esbuild build for the web bundles. Solid components (.tsx) need Solid's compiler, which plain
// esbuild does not do, so we run esbuild through esbuild-plugin-solid (babel-preset-solid) to
// compile JSX into Solid's reactive runtime calls.
//
// Three app bundles: the viewer (src/main.ts -> static/app.js), the extraction workbench
// (src/datasheets.ts -> static/datasheets.js, WS13-006), and the design browser
// (src/browse.ts -> static/browse.js, WS9-049). They are separate pages with separate entries so
// each page downloads only what it uses: the workbench's heavier deps (pdf.js) never bloat the
// viewer bundle, and the browse page carries neither pdf.js nor dockview and the WebGL renderer.
// The datasheets page also needs the pdf.js worker as a standalone script (static/pdf.worker.js),
// which pdf.js loads by URL at runtime.
import { readFileSync } from "node:fs";
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

// The Solid app bundles, one per page. Each must contain exactly one reactive core (asserted below).
const appBundles = [
  { entry: "src/main.ts", outfile: "static/app.js" },
  { entry: "src/datasheets.ts", outfile: "static/datasheets.js" },
  { entry: "src/browse.ts", outfile: "static/browse.js" },
  { entry: "src/landing.ts", outfile: "static/landing.js" },
];

const solidBuild = (b) => ({
  entryPoints: [b.entry],
  bundle: true,
  format: "esm",
  outfile: b.outfile,
  alias: solidAlias,
  plugins: [solidPlugin()],
  logLevel: "info",
});

// The pdf.js worker, bundled as a standalone same-origin script the region viewer points
// GlobalWorkerOptions.workerSrc at. No Solid, so it is not subject to the single-core check.
const workerBuild = {
  entryPoints: [require.resolve("pdfjs-dist/build/pdf.worker.mjs")],
  bundle: true,
  format: "iife",
  outfile: "static/pdf.worker.js",
  logLevel: "info",
};

const builds = [...appBundles.map(solidBuild), workerBuild];

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
  // bundle so a new page (datasheets.js) is guarded too, not just app.js.
  for (const b of appBundles) {
    const cores = (readFileSync(b.outfile, "utf8").match(/function createSignal/g) || []).length;
    if (cores !== 1) {
      console.error(`build: expected exactly 1 Solid core in ${b.outfile}, found ${cores} — duplicate solid-js instances break cross-island reactivity (see solidAlias above)`);
      process.exit(1);
    }
  }
}
