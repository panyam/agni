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

import { spawn, type ChildProcess } from "node:child_process";
import { createServer } from "node:net";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

async function freePort(): Promise<number> {
  return new Promise((ok, fail) => {
    const probe = createServer();
    probe.once("error", fail);
    probe.listen(0, "127.0.0.1", () => {
      const addr = probe.address();
      if (addr === null || typeof addr === "string") {
        probe.close(() => fail(new Error("no port from probe")));
        return;
      }
      const port = addr.port;
      probe.close(() => ok(port));
    });
  });
}

async function waitForServer(base: string, ms: number): Promise<void> {
  const deadline = Date.now() + ms;
  for (;;) {
    try {
      const res = await fetch(base, { signal: AbortSignal.timeout(2000) });
      if (res.ok) return;
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`the docsite did not answer at ${base} within ${ms}ms`);
    await new Promise((r) => setTimeout(r, 250));
  }
}

declare module "vitest" {
  interface ProvidedContext {
    docsiteUrl: string;
  }
}

let child: ChildProcess | undefined;

export async function setup({ provide }: { provide: (key: string, value: unknown) => void }): Promise<void> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  // The docsite builds its pages once at startup and serves them, so an included figure is read
  // from disk exactly once. That is why nothing here tries to edit a figure mid-run.
  child = spawn("go", ["run", "."], {
    cwd: resolve(repoRoot, "docsite"),
    env: { ...process.env, AGNI_DOCS_ENV: "dev", AGNI_DOCS_PORT: `:${port}` },
    stdio: ["ignore", "pipe", "pipe"],
    detached: true,
  });
  let log = "";
  child.stdout?.on("data", (d: Buffer) => (log += d.toString()));
  child.stderr?.on("data", (d: Buffer) => (log += d.toString()));
  try {
    await waitForServer(`${base}/agni/`, 120_000);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.error(log);
    throw e;
  }
  provide("docsiteUrl", base);
}

// teardown kills the process GROUP: `go run` execs the compiled binary as a child, so killing the
// go process alone orphans the server and leaves the port held.
export async function teardown(): Promise<void> {
  if (!child?.pid) return;
  try {
    process.kill(-child.pid, "SIGTERM");
  } catch {
    child.kill("SIGTERM");
  }
  await new Promise((r) => setTimeout(r, 300));
}
