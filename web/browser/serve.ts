// Starts an agni server for the browser tests and stops it afterwards.
//
// It binds a port the OS picked rather than a fixed one. Two long-lived agni servers commonly sit on
// :8080 and :8099 during development, and a test suite that fought them for a port would fail in a
// way that looks like a broken assertion. Asking the kernel for a free port and handing it straight
// to the server keeps a test run invisible to whatever else is running.
//
// The mounts are the repo's own reader fixtures, so the pages under test are the synthetic designs
// the rest of the suite uses and nothing here reaches a real board.

import { spawn, type ChildProcess } from "node:child_process";
import { createServer } from "node:net";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

// freePort asks the kernel for an unused port and gives it back. There is a race between closing
// the probe and the server binding, which is why nothing retries on it: on a developer machine the
// window is microseconds, and a collision fails loudly rather than silently using the wrong server.
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

// waitForServer polls until the server answers or the deadline passes. `go run` compiles the CLI on
// first use, so the first call in a clean checkout can take tens of seconds; the timeout is sized
// for that rather than for a warm binary.
async function waitForServer(base: string, ms: number): Promise<void> {
  const deadline = Date.now() + ms;
  for (;;) {
    try {
      const res = await fetch(base, { signal: AbortSignal.timeout(2000) });
      if (res.ok) return;
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`agni serve did not answer at ${base} within ${ms}ms`);
    await new Promise((r) => setTimeout(r, 250));
  }
}

// The base URL travels to the specs through vitest's provide/inject channel, declared here so
// `inject("baseUrl")` is typed rather than a string nobody checks.
declare module "vitest" {
  interface ProvidedContext {
    baseUrl: string;
  }
}

let child: ChildProcess | undefined;

// setup starts the server and publishes its base URL for the specs. vitest calls it once per run.
export async function setup({ provide }: { provide: (key: string, value: unknown) => void }): Promise<void> {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  child = spawn(
    "go",
    [
      "run", "./cmd/agni", "serve",
      "--addr", `:${port}`,
      "--mount", "kicad=readers/kicad/testdata",
      "--mount", "edif=readers/edif/testdata",
      "web",
    ],
    // detached so the spawn becomes a process-group leader: `go run` execs the compiled binary as
    // its own child, and killing only the go process would orphan the server holding the port.
    { cwd: repoRoot, stdio: ["ignore", "pipe", "pipe"], detached: true },
  );
  // Server output is kept and only printed on a failure to start. Streaming it would bury the test
  // output; discarding it would make a startup failure unreadable.
  let log = "";
  child.stdout?.on("data", (d: Buffer) => (log += d.toString()));
  child.stderr?.on("data", (d: Buffer) => (log += d.toString()));
  try {
    await waitForServer(base, 120_000);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.error(log);
    throw e;
  }
  provide("baseUrl", base);
}

// teardown stops the server. `go run` execs the compiled binary as a CHILD, so killing the go
// process alone orphans the server and leaves the port held. Killing the process group takes both.
export async function teardown(): Promise<void> {
  if (!child?.pid) return;
  try {
    process.kill(-child.pid, "SIGTERM");
  } catch {
    child.kill("SIGTERM");
  }
  await new Promise((r) => setTimeout(r, 300));
}
