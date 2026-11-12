// Starts the docsite server for the figure sweep and stops it afterwards.
//
// A second global setup rather than more flags on `serve.ts`, because these are two different
// programs: `serve.ts` runs `cmd/agni serve`, which knows nothing about `docsite/content`, and the
// docsite is its own Go module with its own `main`. With one shared setup, the specs for one
// program would also start the other program's server and never touch it.
//
// It binds a port the OS picked, as `serve.ts` does and for the same reason. A developer commonly
// has a docsite on :8080 or :8085 already. A suite that fought it for a port would fail in a way
// that reads like a broken assertion.
//
// Spawning and stopping are in `childserver.ts`. Its header explains why teardown signals a pid
// only after it confirms the process is still alive.

import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { start, freePort, type Server } from "./childserver.js";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

// Declared here for the same reason serve.ts declares baseUrl. With it, `inject("docsiteUrl")` is
// typed rather than a string nobody checks.
declare module "vitest" {
  interface ProvidedContext {
    docsiteUrl: string;
  }
}

let server: Server | undefined;

export async function setup({ provide }: { provide: (key: string, value: unknown) => void }): Promise<void> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  // The docsite builds its pages once at startup and serves them, so an included figure is read
  // from disk exactly once. So nothing here tries to edit a figure mid-run.
  server = await start({
    what: "the docsite",
    command: "go",
    args: ["run", "."],
    cwd: resolve(repoRoot, "docsite"),
    // AGNI_DOCS_RUNS=stored serves every run capture as committed. The server renders every page at
    // startup, and without it a stale capture was regenerated into the working tree, leaving tracked
    // files modified after a test run (agni issue 1149). tutorial-runs-check judges staleness.
    env: { ...process.env, AGNI_DOCS_ENV: "dev", AGNI_DOCS_PORT: `:${port}`, AGNI_DOCS_RUNS: "stored" },
    base,
    // The docsite answers under a path prefix, so its health check is not the bare base.
    health: `${base}/agni/`,
  });
  provide("docsiteUrl", base);
}

export async function teardown(): Promise<void> {
  await server?.stop();
}
