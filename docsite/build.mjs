// build.mjs — builds the docs' interactive TypeScript components with esbuild,
// replacing the webpack setup the notations docs use.
//
// esbuild covers webpack's only irreplaceable job here: emitting the
// templates/gen.<Name>.html script-tag includes that BasePage.html pulls in for
// a page that opts into a playground. We read the build's metafile to find each
// entry's emitted (hashed) filename and write its include.
//
// Run: node build.mjs        (one-shot)
//      node build.mjs --watch (rebuild on change)
//
// The site renders without running this at all; it is only needed once a page
// opts into a playground bundle. esbuild is the same bundler web/build.mjs uses.

import * as esbuild from "esbuild";
import { writeFileSync } from "node:fs";
import path from "node:path";

// Must match PathPrefix in main.go.
const PATH_PREFIX = "/agni";

// One entry per playground bundle. DocsPage is the base bundle; add
// SideBySide / Viewer / Query entries here as they are built.
const entryPoints = {
  DocsPage: "components/DocsPage.ts",
};

const outdir = "static/js/gen";
const watch = process.argv.includes("--watch");

/** Write the templates/gen.<Name>.html include for each entry. */
function emitIncludes(metafile) {
  const outputs = metafile.outputs;
  // Map each entry name to its emitted js file.
  for (const [name, entry] of Object.entries(entryPoints)) {
    const src = path.resolve(entry);
    const outFile = Object.keys(outputs).find(
      (o) => outputs[o].entryPoint && path.resolve(outputs[o].entryPoint) === src,
    );
    if (!outFile) continue;
    const webPath = `${PATH_PREFIX}/${outFile.replace(/^static\//, "static/")}`;
    const include = `<script defer src="${webPath}"></script>\n`;
    writeFileSync(path.join("templates", `gen.${name}.html`), include);
    console.log(`emitted templates/gen.${name}.html -> ${webPath}`);
  }
}

const options = {
  entryPoints,
  bundle: true,
  format: "esm",
  splitting: false,
  sourcemap: true,
  minify: !watch,
  outdir,
  entryNames: "[name].[hash]",
  metafile: true,
  logLevel: "info",
};

if (watch) {
  const ctx = await esbuild.context(options);
  await ctx.watch();
  const result = await ctx.rebuild();
  emitIncludes(result.metafile);
  console.log("watching docs components...");
} else {
  const result = await esbuild.build(options);
  emitIncludes(result.metafile);
}
