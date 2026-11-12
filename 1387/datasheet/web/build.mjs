// esbuild build for the datasheets workbench (agni issue 744), the page agnids serves. Like the
// viewer's web/build.mjs it runs esbuild through esbuild-plugin-solid, and it asserts the one-Solid-
// core property that build explains: two reactive cores in one bundle make an island's signal
// updates silently no-op.
//
// Two outputs: the workbench (src/datasheets.ts -> static/datasheets.js) and the pdf.js worker as a
// standalone same-origin script (static/pdf.worker.js), which pdf.js loads by URL at runtime.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import * as esbuild from "esbuild";
import { solidPlugin } from "esbuild-plugin-solid";

const require = createRequire(import.meta.url);
const watch = process.argv.includes("--watch");

// Pin Solid to a single physical runtime (see web/build.mjs for the bug this prevents).
const solidAlias = {
  "solid-js": require.resolve("solid-js/dist/solid.js"),
  "solid-js/web": require.resolve("solid-js/web/dist/web.js"),
  "solid-js/store": require.resolve("solid-js/store/dist/store.js"),
};

const appBundles = [{ entry: "src/datasheets.ts", outfile: "static/datasheets.js" }];

const solidBuild = (b) => ({
  entryPoints: [b.entry],
  bundle: true,
  format: "esm",
  outfile: b.outfile,
  alias: solidAlias,
  plugins: [solidPlugin()],
  logLevel: "info",
});

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
  for (const b of appBundles) {
    const cores = (readFileSync(b.outfile, "utf8").match(/function createSignal/g) || []).length;
    if (cores !== 1) {
      console.error(`build: expected exactly 1 Solid core in ${b.outfile}, found ${cores} (duplicate solid-js instances break cross-island reactivity; see solidAlias)`);
      process.exit(1);
    }
  }
}
