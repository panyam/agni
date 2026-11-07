// Starts an agni server for the browser tests and stops it afterwards.
//
// It binds a port the OS picked rather than a fixed one. Two long-lived agni servers commonly sit on
// :8080 and :8099 during development, and a test suite that fought them for a port would fail in a
// way that looks like a broken assertion. The setup asks the kernel for a free port and hands it
// straight to the server, so a test run is invisible to whatever else is running.
//
// The mounts are the repo's own reader fixtures. So the pages under test show the same synthetic
// designs as the rest of the suite, and nothing here reaches a real board.
//
// Spawning and stopping are in `childserver.ts`, shared with `docsite.ts`. Read its header before
// changing anything about teardown. Signalling a process GROUP by number is safe only while the
// child is alive, and getting that wrong killed unrelated servers on the developer's machine.

import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import { start, freePort, type Server } from "./childserver.js";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");

// The base URL reaches the specs through vitest's provide/inject channel. It is declared here so
// `inject("baseUrl")` is typed rather than a string nobody checks.
declare module "vitest" {
  interface ProvidedContext {
    baseUrl: string;
    // agniBin is the agni binary built from this checkout. setup builds it once, for the server and
    // for every spec that runs the CLI to compare against.
    agniBin: string;
  }
}

let server: Server | undefined;

// setup starts the server and publishes its base URL for the specs. vitest calls it once per run.
//
// agni is built ONCE, with the developer's own environment. That one binary serves the pages and
// answers every spec's CLI comparison. A spec runs the CLI with HOME pointed at an empty directory,
// so a personal agni.yaml is kept out of the run. Under `go run`, that also pointed Go's build cache
// at the empty directory, so each such call rebuilt agni from nothing. That took about 30 s per call
// on a CI runner, and most of the browser suite's six minutes (measured 5.7 s against 0.36 s for the
// binary, on a laptop).
export async function setup({ provide }: { provide: (key: string, value: unknown) => void }): Promise<void> {
  const agniBin = join(mkdtempSync(join(tmpdir(), "agni-browser-bin-")), "agni");
  execFileSync("go", ["build", "-o", agniBin, "./cmd/agni"], { cwd: repoRoot, stdio: "inherit" });
  provide("agniBin", agniBin);
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  server = await start({
    what: "agni serve",
    command: agniBin,
    args: [
      "serve",
      "--addr", `:${port}`,
      "--mount", "kicad=readers/kicad/testdata",
      "--mount", "edif=readers/edif/testdata",
      // The conformance fixtures, for the one design in the tree whose check run reports an
      // INCONCLUSIVE finding (agni issue 350).
      "--mount", "conformance=cmd/agni/testdata/conformance",
      // The tutorial project. It has a descriptor, companions and project config, and the
      // in-browser engine's spec opens it under both engines (agni issue 853).
      "--mount", "tut=examples/tutorial-project",
      // The Jetson review project, which the browser-review walk opens when WALK_DESIGN=jetson (agni
      // issue 1118). Its board is copied in by `make -C examples/jetson-project board`; without the
      // copy the mount holds only descriptors and no spec opens it.
      "--mount", "jetson=examples/jetson-project",
      // The cross-probe walk's cut of Jetson, schematic and board (agni issue 1323).
      "--mount", "probe=examples/cross-probe-project",
      // The shared example designs, for an EDIF netlist whose ODB++ board it does not declare (agni
      // issue 1367): odb/gateway.edn beside odb/gateway-layout.
      "--mount", "common=examples/common/designs",
      // The fetched boards, for the naming walk on the MNT Reform motherboard (agni issue 1267).
      "--mount", "boards=tools/samples/boards",
      // Clay R6, the cross-probe walk's OrCAD design on its Allegro board (agni issue 1382).
      "--mount", "cadence=tools/samples/cadence",
      // Mission #1353's native boards: the BeagleBone AI's Allegro .brd and foxBMS's Altium .PcbDoc
      // (agni issue 1383).
      "--mount", "allegro=tools/samples/allegro",
      "--mount", "altium=tools/samples/altium",
      // A store, so the Review panel can run a checklist and save it (agni issue 1030). Fresh per run,
      // so no spec sees another run's reviews.
      "--review-store", mkdtempSync(join(tmpdir(), "agni-browser-reviews-")),
      // Named explicitly because a flag wins outright over agni.yaml. Without it, suppose a
      // developer's ~/.config/agni/agni.yaml names a web_dir, say another checkout's. These tests
      // then run against THAT viewer, which passes or fails for reasons unrelated to this tree.
      "--web-dir", "web",
    ],
    // cwd is the repo root, so "web" is this checkout's own built viewer.
    cwd: repoRoot,
    base,
    health: base,
  });
  provide("baseUrl", base);
}

export async function teardown(): Promise<void> {
  await server?.stop();
}
