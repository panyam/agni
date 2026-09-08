// Starts an agni server for the browser tests and stops it afterwards.
//
// It binds a port the OS picked rather than a fixed one. Two long-lived agni servers commonly sit on
// :8080 and :8099 during development, and a test suite that fought them for a port would fail in a
// way that looks like a broken assertion. Asking the kernel for a free port and handing it straight
// to the server keeps a test run invisible to whatever else is running.
//
// The mounts are the repo's own reader fixtures, so the pages under test are the synthetic designs
// the rest of the suite uses and nothing here reaches a real board.
//
// Spawning and stopping live in `childserver.ts`, shared with `docsite.ts`. Read its header before
// changing anything about teardown: signalling a process GROUP by number is only safe while the
// child is alive, and getting that wrong killed unrelated servers on the developer's machine.

import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { start, freePort, type Server } from "./childserver.js";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

// The base URL travels to the specs through vitest's provide/inject channel, declared here so
// `inject("baseUrl")` is typed rather than a string nobody checks.
declare module "vitest" {
  interface ProvidedContext {
    baseUrl: string;
  }
}

let server: Server | undefined;

// setup starts the server and publishes its base URL for the specs. vitest calls it once per run.
export async function setup({ provide }: { provide: (key: string, value: unknown) => void }): Promise<void> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  server = await start({
    what: "agni serve",
    command: "go",
    args: [
      "run", "./cmd/agni", "serve",
      "--addr", `:${port}`,
      "--mount", "kicad=readers/kicad/testdata",
      "--mount", "edif=readers/edif/testdata",
    ],
    // cwd is the repo root, so serve's default --web-dir ("web") resolves without a flag. This used
    // to pass "web" positionally, which was the same value by another route.
    cwd: repoRoot,
    base,
    health: base,
  });
  provide("baseUrl", base);
}

export async function teardown(): Promise<void> {
  await server?.stop();
}
