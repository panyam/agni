// The browser half of the wasm bench (agni issue 852). It runs the real worker over one board and
// times the questions tools/wasmbench times natively, so the two can be compared row for row. It is
// a measurement entry, loaded only by web/browser/bench.mjs, and no page ships it.
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { DesignService } from "../gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "../gen/agni/v1/webapi/checks_pb.js";
import { QueryService } from "../gen/agni/v1/webapi/query_pb.js";
import { startEngine } from "./client.js";
import type { Files } from "./client.js";

export interface BenchArgs {
  // mount is the server mount holding the board, and files its paths within it, fetched from /raw/.
  mount: string;
  files: string[];
  design: string;
  query: string;
  countComponents: string;
  countNets: string;
  // oneLane runs every request on one worker, as the engine did before #911, for the comparison.
  oneLane?: boolean;
}

async function bench(a: BenchArgs): Promise<Record<string, number | string>> {
  const asset = (name: string) => new URL(name, import.meta.url).href;
  let t = performance.now();
  const engine = await startEngine({ worker: asset("agni-worker.js"), wasm: asset("agni.wasm"), exec: asset("wasm_exec.js") }, { oneLane: a.oneLane });
  const bootMS = performance.now() - t;

  t = performance.now();
  const files: Files = {};
  let bytes = 0;
  await Promise.all(
    a.files.map(async (p) => {
      const res = await fetch(`/raw/${a.mount}/${p.split("/").map(encodeURIComponent).join("/")}`);
      if (!res.ok) throw new Error(`${p}: ${res.status}`);
      files[p] = new Uint8Array(await res.arrayBuffer());
      bytes += files[p].byteLength;
    }),
  );
  const fetchMS = performance.now() - t;

  // Mounted under the same name as natively, so the URIs the two halves ask about match.
  t = performance.now();
  await engine.add("bench", files);
  const composeMS = performance.now() - t;

  const transport = createConnectTransport({ baseUrl: location.origin, fetch: (i, init) => engine.fetch(i, init) });
  const jobsOnly = createClient(CheckService, createConnectTransport({ baseUrl: location.origin, fetch: (i, init) => engine.lanes.jobs.fetch(i, init) }));
  const design = createClient(DesignService, transport);
  const checks = createClient(CheckService, transport);
  const query = createClient(QueryService, transport);
  const uri = `mount://bench/${a.design}`;

  t = performance.now();
  await design.getDesign({ uri });
  const readMS = performance.now() - t;

  const count = async (q: string) => Number((await query.runQuery({ uri, query: q })).rows[0]?.cells[0] ?? 0);
  const components = await count(a.countComponents);
  const nets = await count(a.countNets);

  // The first check is the one a visitor waits on, so every measurement under it happens while it
  // runs: a query asked mid-run (#900 wants under 300ms), then an abort, and how long until the jobs
  // lane answers again. A later check would reuse what this one memoized and prove nothing. On one
  // worker an abort ends the only worker, so the next request reads the design again.
  const ctl = new AbortController();
  t = performance.now();
  let firstCheckMS = -1; // stays -1 when the abort ended it
  const started = t;
  const running = checks.getCheckReport({ uri }, { signal: ctl.signal }).then(
    () => void (firstCheckMS = performance.now() - started),
    () => undefined,
  );
  await new Promise((r) => setTimeout(r, 500));
  let t2 = performance.now();
  await query.runQuery({ uri, query: a.query });
  const queryDuringCheckMS = performance.now() - t2;
  const stillRunning = await Promise.race([running.then(() => false), new Promise<boolean>((r) => setTimeout(() => r(true), 0))]);
  t2 = performance.now();
  ctl.abort();
  await running;
  await jobsOnly.listRules({});
  const abortToAnswerMS = performance.now() - t2;

  // A check after the abort. On two lanes it runs on a fresh worker, so it reads the design again.
  // On one worker it reuses the first check's work unless the abort landed.
  t = performance.now();
  const report = (await checks.getCheckReport({ uri })).report;
  const checkMS = performance.now() - t;

  t = performance.now();
  const rows = (await query.runQuery({ uri, query: a.query })).rows.length;
  const queryMS = performance.now() - t;
  const mem = await engine.memoryBytes();

  return {
    host: "wasm (Chromium worker)",
    files: a.files.length,
    bytes,
    components,
    nets,
    boot_ms: Math.round(bootMS),
    fetch_ms: Math.round(fetchMS),
    compose_ms: Math.round(composeMS),
    read_ms: Math.round(readMS),
    check_ms: Math.round(checkMS),
    rules: report?.rulesRun ?? 0,
    findings: (report?.sections ?? []).reduce((n, s) => n + s.count, 0),
    query_ms: Math.round(queryMS),
    query_rows: rows,
    query_during_check_ms: Math.round(queryDuringCheckMS),
    check_still_running: stillRunning ? "yes" : "no",
    first_check_ms: firstCheckMS < 0 ? "aborted" : Math.round(firstCheckMS),
    abort_to_answer_ms: Math.round(abortToAnswerMS),
    serve_mb: Math.round(mem.serve / (1 << 20)),
    jobs_mb: a.oneLane ? 0 : Math.round(mem.jobs / (1 << 20)),
    peak_mb: Math.round((a.oneLane ? mem.serve : mem.serve + mem.jobs) / (1 << 20)),
  };
}

(globalThis as unknown as { agniBench: typeof bench }).agniBench = bench;
