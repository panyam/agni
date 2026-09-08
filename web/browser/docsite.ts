// Starts the docsite server for the figure sweep and stops it afterwards.
//
// A second global setup rather than more flags on `serve.ts`, because these are two different
// programs: `serve.ts` runs `cmd/agni serve`, which knows nothing about `docsite/content`, and the
// docsite is its own Go module with its own `main`. Sharing one setup would mean one of them
// starting a server the specs using it never touch.
//
// It binds a port the OS picked, for the reason `serve.ts` does: a developer commonly has a docsite
// on :8080 or :8085 already, and a suite that fought it for a port would fail in a way that reads
// like a broken assertion.
//
// Spawning and stopping live in `childserver.ts`. See its header on why teardown never signals a pid
// it has not confirmed is still alive.

import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { start, freePort, type Server } from "./childserver.js";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

// Declared here for the same reason serve.ts declares baseUrl: so `inject("docsiteUrl")` is typed
// rather than a string nobody checks.
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
  // from disk exactly once. That is why nothing here tries to edit a figure mid-run.
  server = await start({
    what: "the docsite",
    command: "go",
    args: ["run", "."],
    cwd: resolve(repoRoot, "docsite"),
    env: { ...process.env, AGNI_DOCS_ENV: "dev", AGNI_DOCS_PORT: `:${port}` },
    base,
    // The docsite answers under a path prefix, so its health check is not the bare base.
    health: `${base}/agni/`,
  });
  provide("docsiteUrl", base);
}

export async function teardown(): Promise<void> {
  await server?.stop();
}
