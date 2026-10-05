// Measures the engine on one board natively and in a Chromium Web Worker, and prints the two side by
// side (agni issue 852). `make wasm-bench BOARD=<folder> DESIGN=<path within it>` runs it; it needs
// `make ui wasm` first, and a Chromium (pnpm exec playwright-core install chromium).
//
// The native half is tools/wasmbench, the same composition as the worker less the wasm runtime. Its
// JSON names the queries it asked, and the browser half asks those, so the rows compare like for like.
import { spawn, execFileSync } from "node:child_process";
import { readdirSync, statSync } from "node:fs";
import { createServer } from "node:net";
import { join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
const [board, design] = process.argv.slice(2);
if (!board || !design) {
  console.error("usage: node browser/bench.mjs <board folder> <design path within it>");
  process.exit(2);
}
const dir = resolve(board);

const walk = (d) =>
  readdirSync(d).flatMap((n) => {
    const p = join(d, n);
    return statSync(p).isDirectory() ? walk(p) : [relative(dir, p).split(sep).join("/")];
  });
const files = walk(dir);

const native = JSON.parse(
  execFileSync("go", ["run", "./tools/wasmbench", "--dir", dir, "--design", design], { cwd: repo, encoding: "utf8", maxBuffer: 1 << 26 }),
);

const port = await new Promise((ok) => {
  const s = createServer().listen(0, "127.0.0.1", () => {
    const p = s.address().port;
    s.close(() => ok(p));
  });
});
const base = `http://127.0.0.1:${port}`;
const server = spawn(join(repo, "bin", "agni"), ["serve", "--addr", `127.0.0.1:${port}`, "--mount", `board=${dir}`, "--web-dir", join(repo, "web")], {
  cwd: repo,
  env: { ...process.env, HOME: "/tmp", XDG_CONFIG_HOME: "/tmp" },
  stdio: "ignore",
});
try {
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(`${base}/healthz`)).ok) break;
    } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  const browser = await chromium.launch();
  // Each configuration in a context of its own, so none restores what another stored in the browser's
  // cache. "reload" is a second page in the lanes run's context, as a visitor reloading the board.
  const SERVE_MAX_BYTES = Number(process.env.SERVE_MAX_MB ?? 700) * (1 << 20);
  const run = async (ctx, cfg) => {
    const page = await ctx.newPage();
    await page.goto(`${base}/healthz`);
    await page.addScriptTag({ url: "/static/agni-bench.js", type: "module" });
    await page.waitForFunction(() => typeof globalThis.agniBench === "function");
    const out = await page.evaluate(
      (a) => globalThis.agniBench(a),
      { mount: "board", files, design, query: native.query, countComponents: native.count_components, countNets: native.count_nets, ...cfg },
    );
    await page.close();
    return out;
  };
  const one = await run(await browser.newContext(), { oneLane: true });
  const lanesCtx = await browser.newContext();
  const wasm = await run(lanesCtx, { serveMaxBytes: SERVE_MAX_BYTES });
  const reload = await run(lanesCtx, { serveMaxBytes: SERVE_MAX_BYTES });
  await browser.close();

  const rows = ["files", "bytes", "components", "nets", "boot_ms", "fetch_ms", "compose_ms", "read_ms", "check_ms", "rules", "findings", "query_ms", "query_rows",
    "query_during_check_ms", "check_still_running", "first_check_ms", "lane_restarts", "query_after_restart_ms", "serve_mb_after_restart", "abort_to_answer_ms", "serve_mb", "jobs_mb", "peak_mb"];
  console.log(`| ${board} | native | wasm, one worker | two lanes, serve limit ${Math.round(SERVE_MAX_BYTES / (1 << 20))} MB | the same reloaded, limit ${Math.round(SERVE_MAX_BYTES / (1 << 20))} MB | lanes / native |`);
  console.log("|---|---|---|---|---|---|");
  for (const r of rows) {
    const n = native[r], w = wasm[r];
    const ratio = r.endsWith("_ms") && n > 0 && w !== undefined ? (w / n).toFixed(1) + "x" : "";
    console.log(`| ${r} | ${n ?? ""} | ${one[r] ?? ""} | ${w ?? ""} | ${reload[r] ?? ""} | ${ratio} |`);
  }
  console.log(JSON.stringify({ native, one, wasm, reload }));
} finally {
  server.kill();
}
