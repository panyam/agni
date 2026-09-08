// Starting a server for the browser suite, and stopping it without taking anything else down.
//
// Both global setups (`serve.ts`, `docsite.ts`) spawn `go run` detached, so the spawn becomes a
// process-group leader and one signal reaches both the `go` process and the binary it execs. Killing
// only the `go` process orphans the server and leaves the port held.
//
// The hazard that motivated this file is the other half of that. `process.kill(-pid)` names a
// process GROUP by number, and a pid is only meaningful while the process is alive: once the child
// has exited, the number is free for the OS to reuse, and `-pid` then names whatever group inherited
// it. `child.pid` keeps its value after exit, so the naive teardown cannot tell the two apart and
// will happily signal a stranger.
//
// That is not theoretical. Inside `make testall`, which spawns thousands of short-lived processes,
// this took down a developer's unrelated `agni serve` and SIGTERM'd the `make` invocation running the
// suite. Standalone it never reproduced, because nothing else was churning pids.
//
// So the rule here: never signal a pid we have not confirmed is still ours. `exited` is set from the
// child's own "exit" event, which Node delivers before the pid can be recycled, and teardown returns
// early when it is set.
//
// The second job is making a server that dies MID-RUN legible. Before this, the specs simply got
// ERR_CONNECTION_REFUSED, seven times, with nothing saying the server was gone. Now the exit is
// recorded with the output the server managed to produce, and `assertAlive` turns it into one
// sentence naming the exit code.

import { spawn, type ChildProcess } from "node:child_process";
import { createServer } from "node:net";

export interface Server {
  base: string;
  stop: () => Promise<void>;
  assertAlive: () => void;
}

// freePort asks the kernel for an unused port and gives it back. There is a race between closing the
// probe and the server binding, which is why nothing retries on it: the window is microseconds, and a
// collision fails loudly rather than silently using the wrong server.
export async function freePort(): Promise<number> {
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

// start spawns a server, waits for it to answer, and hands back the handle the setup provides to the
// specs. `health` is the URL that must return 2xx; it is not always the base (the docsite answers
// under a path prefix).
export async function start(opts: {
  what: string;
  command: string;
  args: string[];
  cwd: string;
  env?: NodeJS.ProcessEnv;
  base: string;
  health: string;
  timeoutMs?: number;
}): Promise<Server> {
  const child: ChildProcess = spawn(opts.command, opts.args, {
    cwd: opts.cwd,
    env: opts.env,
    stdio: ["ignore", "pipe", "pipe"],
    detached: true,
  });

  // Output is kept and printed only on a failure. Streaming it would bury the test output;
  // discarding it would make a startup failure or a mid-run death unreadable.
  let log = "";
  child.stdout?.on("data", (d: Buffer) => (log += d.toString()));
  child.stderr?.on("data", (d: Buffer) => (log += d.toString()));

  // The whole point of the file. Once this fires the pid is no longer ours to signal.
  let exited: { code: number | null; signal: NodeJS.Signals | null } | undefined;
  child.on("exit", (code, signal) => (exited = { code, signal }));

  const pid = child.pid;

  const assertAlive = (): void => {
    if (!exited) return;
    const how = exited.signal ? `signal ${exited.signal}` : `exit code ${exited.code}`;
    throw new Error(`${opts.what} died during the run (${how}). Its output was:\n${log}`);
  };

  const stop = async (): Promise<void> => {
    // Already gone: nothing to signal, and the pid may belong to somebody else by now.
    if (exited || !pid) return;
    try {
      process.kill(-pid, "SIGTERM");
    } catch {
      // No such group, or not ours. Fall back to the child handle, which Node scopes to the
      // process it actually spawned rather than to a number.
      try {
        child.kill("SIGTERM");
      } catch {
        // already reaped between the check and here
      }
    }
    // Give it a moment to release the port before the next suite asks for one.
    for (let i = 0; i < 20 && !exited; i++) await new Promise((r) => setTimeout(r, 50));
  };

  const deadline = Date.now() + (opts.timeoutMs ?? 120_000);
  for (;;) {
    if (exited) {
      const how = exited.signal ? `signal ${exited.signal}` : `exit code ${exited.code}`;
      throw new Error(`${opts.what} exited before it answered (${how}). Its output was:\n${log}`);
    }
    try {
      const res = await fetch(opts.health, { signal: AbortSignal.timeout(2000) });
      if (res.ok) return { base: opts.base, stop, assertAlive };
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) {
      await stop();
      throw new Error(`${opts.what} did not answer at ${opts.health} in time. Its output was:\n${log}`);
    }
    await new Promise((r) => setTimeout(r, 250));
  }
}
