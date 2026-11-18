// A smoke test of a built agni on a machine the browser suite cannot run on, such as Windows (agni
// issue 1595). The suite's server harness stops its server by process group, which Windows has no
// way to signal, so this runs one server and one page by hand.
//
// It starts `agni serve` with one mount, opens a design in headless Chromium, and waits for the
// viewer to draw a sheet. It exits 0 when the sheet draws and 1 otherwise.
//
//   node browser/smoke.mjs <agni binary> <mount root> <design path in the mount>
//   node browser/smoke.mjs ../agni.exe 'D:\a\agni\agni\examples\tutorial-project' designs/gateway

import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { chromium } from "playwright-core";

const [agni, root, design] = process.argv.slice(2);
if (!agni || !root || !design) {
  console.error("usage: node browser/smoke.mjs <agni binary> <mount root> <design path in the mount>");
  process.exit(2);
}

// freePort asks the OS for a port no one holds.
const freePort = () =>
  new Promise((done, fail) => {
    const s = createServer();
    s.once("error", fail);
    s.listen(0, "127.0.0.1", () => {
      const { port } = s.address();
      s.close(() => done(port));
    });
  });

const port = await freePort();
const base = `http://127.0.0.1:${port}`;
const log = [];
const server = spawn(agni, ["serve", "--addr", `127.0.0.1:${port}`, "--mount", `smoke=${root}`], { stdio: ["ignore", "pipe", "pipe"] });
server.stdout.on("data", (b) => log.push(String(b)));
server.stderr.on("data", (b) => log.push(String(b)));
let exited = false;
server.on("exit", () => (exited = true));

let browser;
let ok = false;
try {
  for (let i = 0; ; i++) {
    if (exited) throw new Error("agni serve exited before it answered");
    const up = await fetch(base).then((r) => r.ok, () => false);
    if (up) break;
    if (i === 300) throw new Error(`agni serve did not answer at ${base} in 30 s`);
    await new Promise((r) => setTimeout(r, 100));
  }
  browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  // A first visit reads the design on the Load page, then opens the viewer.
  await page.goto(`${base}/files/smoke/${design}/view?mode=svg`, { waitUntil: "networkidle", timeout: 180_000 });
  await page.waitForFunction(() => document.querySelectorAll("#svg-view svg *").length > 0, null, { timeout: 180_000 });
  const drawn = await page.evaluate(() => document.querySelectorAll("#svg-view svg *").length);
  if (errors.length) throw new Error(`the page logged errors: ${errors.join("; ")}`);
  console.log(`smoke: ${agni} served ${design} from ${root}, and the viewer drew a sheet of ${drawn} elements`);
  ok = true;
} catch (e) {
  console.error(`smoke: ${e instanceof Error ? e.message : String(e)}`);
  console.error(log.join(""));
} finally {
  await browser?.close();
  server.kill();
}
process.exit(ok ? 0 : 1);
