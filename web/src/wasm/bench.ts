// The browser half of the wasm bench (agni issue 852). It runs the real worker over one board and
// times the questions tools/wasmbench times natively, so the two can be compared row for row. It is
// a measurement entry, loaded only by web/browser/bench.mjs, and no page ships it.
import { SnapshotService } from "../gen/agni/v1/webapi/snapshot_pb.js";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { DesignService } from "../gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "../gen/agni/v1/webapi/checks_pb.js";
import { QueryService } from "../gen/agni/v1/webapi/query_pb.js";
import { WorkspaceService } from "../gen/agni/v1/webapi/workspace_pb.js";
import { mountingFetch, startEngine } from "./client.js";
import type { Files, WorkerStats } from "./client.js";
import { loadSplit, type LoadSplit } from "../../browser/exercise-report.js";

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
  // serveMaxBytes is the serve lane's memory limit, unset for none.
  serveMaxBytes?: number;
  // serveShrinkRatio overrides the lane's wouldShrink ratio, for measuring where it should sit.
  serveShrinkRatio?: number;
  // pageMount, when set, brings the files in as the page does, through mountingFetch, each lane's own
  // as its first request names the design, listed by the server under this mount, where the design is
  // pagePath. Otherwise every file is fetched up front and given to every lane.
  pageMount?: string;
  pagePath?: string;
  // holdBoard false drops the listing's board flags, so the serve lane is given the board as it was
  // before agni issue 992, for the comparison.
  holdBoard?: boolean;
}

async function bench(a: BenchArgs): Promise<Record<string, number | string>> {
  const asset = (name: string) => new URL(name, import.meta.url).href;
  let restarts = 0;
  // Each idle measurement the serve lane took past its limit, as "memory MB/live MB decision".
  const seen: string[] = [];
  let t = performance.now();
  const engine = await startEngine({ worker: asset("agni-worker.js"), wasm: asset("agni.wasm"), exec: asset("wasm_exec.js") }, {
    oneLane: a.oneLane,
    serveMaxBytes: a.serveMaxBytes,
    serveShrinkRatio: a.serveShrinkRatio,
    onRestart: () => restarts++,
    onServeStats: (st, replaced) => seen.push(`${mb(st.memoryBytes)}/${mb(st.heap?.heapAlloc)} ${replaced ? "replaced" : "kept"}`),
  });
  const bootMS = performance.now() - t;

  const raw = async (mount: string, p: string) => {
    const res = await fetch(`/raw/${mount}/${p.split("/").map(encodeURIComponent).join("/")}`);
    if (!res.ok) throw new Error(`${p}: ${res.status}`);
    return new Uint8Array(await res.arrayBuffer());
  };
  let fetchMS = -1;
  let composeMS = -1;
  let bytes = 0;
  let fetchedFiles = 0;
  // The files given up front live in the page, as a dropped design's do, so mountingFetch brings none
  // in and only ingests the snapshot into each worker before its first question, a replacement's
  // included (agni issue 1061).
  let engineFetch = mountingFetch({
    engine,
    source: { list: async () => Promise.reject(new Error("nothing to list")), fetchFile: async () => Promise.reject(new Error("nothing to fetch")) },
    network: globalThis.fetch.bind(globalThis),
    browserMounts: new Set(["bench"]),
  });
  let uri = `mount://bench/${a.design}`;
  if (a.pageMount !== undefined) {
    const listing = createClient(WorkspaceService, createConnectTransport({ baseUrl: location.origin }));
    engineFetch = mountingFetch({
      engine,
      source: {
        list: async (u) => {
          const set = await listing.listDesignFiles({ uri: u });
          return a.holdBoard === false ? { ...set, files: set.files.map((f) => ({ path: f.path, sha256: f.sha256 })) } : set;
        },
        fetchFile: async (mount, p) => {
          const b = await raw(mount, p);
          bytes += b.byteLength;
          fetchedFiles++;
          return b;
        },
      },
      network: globalThis.fetch.bind(globalThis),
    });
    uri = `mount://${a.pageMount}/${a.pagePath ?? a.design}`;
  } else {
    t = performance.now();
    const files: Files = {};
    await Promise.all(
      a.files.map(async (p) => {
        files[p] = await raw(a.mount, p);
        bytes += files[p].byteLength;
      }),
    );
    fetchedFiles = a.files.length;
    fetchMS = performance.now() - t;

    // Mounted under the same name as natively, so the URIs the two halves ask about match.
    t = performance.now();
    await engine.add("bench", files);
    composeMS = performance.now() - t;
  }

  const transport = createConnectTransport({ baseUrl: location.origin, fetch: engineFetch });
  const jobsOnly = createClient(CheckService, createConnectTransport({ baseUrl: location.origin, fetch: (i, init) => engine.lanes.jobs.fetch(i, init) }));
  const design = createClient(DesignService, transport);
  const checks = createClient(CheckService, transport);
  const query = createClient(QueryService, transport);

  // The read is the ingest and the open's GetDesign, as the page's first sheet waits on both. Each
  // lane's worker is ingested under the same id before its first question (agni issue 1061).
  t = performance.now();
  const snapshot = (await createClient(SnapshotService, transport).ingest({ snapshot: { uri }, snapshotId: "bench" })).name;
  await design.getDesign({ snapshot });
  const readMS = performance.now() - t;

  const count = async (q: string) => Number((await query.runQuery({ snapshot, query: q })).rows[0]?.cells[0] ?? 0);
  const components = await count(a.countComponents);
  const nets = await count(a.countNets);
  // The worker after reading the design and building its model: its memory beside its live heap, and
  // how long the collection that measures it takes, which a lane with a limit pays at every idle moment
  // (agni issue 990). Asked directly only on one lane, which has no limit, since asking a worker a
  // lane is measuring races the lane's own decision to end it.
  let read: WorkerStats | undefined;
  let statsGcMS = -1;
  if (a.oneLane) {
    t = performance.now();
    read = await engine.lanes.serve.stats();
    statsGcMS = performance.now() - t;
  }

  // The first check is the one a visitor waits on, so every measurement under it happens while it
  // runs: a query asked mid-run (#900 wants under 300ms), then an abort, and how long until the jobs
  // lane answers again. A later check would reuse what this one memoized and prove nothing. On one
  // worker an abort ends the only worker, so the next request reads the design again.
  const ctl = new AbortController();
  t = performance.now();
  let firstCheckMS = -1; // stays -1 when the abort ended it
  const started = t;
  const running = checks.getCheckReport({ snapshot }, { signal: ctl.signal }).then(
    () => void (firstCheckMS = performance.now() - started),
    () => undefined,
  );
  await new Promise((r) => setTimeout(r, 500));
  let t2 = performance.now();
  await query.runQuery({ snapshot, query: a.query });
  const queryDuringCheckMS = performance.now() - t2;
  const stillRunning = await Promise.race([running.then(() => false), new Promise<boolean>((r) => setTimeout(() => r(true), 0))]);
  t2 = performance.now();
  ctl.abort();
  await running;
  await jobsOnly.listRules({});
  const abortToAnswerMS = performance.now() - t2;

  // A check after the abort. On two lanes it runs on a fresh worker, so it reads the design again.
  // On one worker it reuses the first check's work unless the abort landed. It asks for each stage's
  // memory (agni issue 994), and the jobs worker's memory is read first, with the design's files in
  // it and nothing else, so the stages' figures say which one raised the peak.
  const jobsBytesBeforeCheck = await engine.lanes.jobs.memoryBytes();
  let stageMemory = "";
  let allocRules = "";
  let split = undefined as LoadSplit | undefined;
  t = performance.now();
  const report = (
    await checks.getCheckReport(
      { snapshot },
      {
        headers: { "Agni-Timing": "memory" },
        onHeader: (h) => {
          stageMemory = memoryStages(h.get("Agni-Timing"), jobsBytesBeforeCheck);
          allocRules = allocators(h.get("Agni-Timing"));
          split = loadSplit(`agni timing CheckDesign: ${h.get("Agni-Timing") ?? ""}`);
        },
      },
    )
  ).report;
  const checkMS = performance.now() - t;

  // The same check again on the worker that just ran it, which keeps each rule's results on the
  // design's model (agni 390): the wait once the design is loaded, against check_load_ms and
  // check_rules_ms above, the one-time part and the evaluation of the run before (agni issue 1060).
  t = performance.now();
  await checks.getCheckReport({ snapshot });
  const checkAgainMS = performance.now() - t;

  t = performance.now();
  const rows = (await query.runQuery({ snapshot, query: a.query })).rows.length;
  const queryMS = performance.now() - t;
  const mem = await engine.memoryBytes();

  // With a limit, the serve lane restarts once idle past it, and its next request restores the design
  // from the browser's cache rather than reading it (agni issue 911).
  let queryAfterRestartMS = -1;
  let restoredMB = -1;
  if (a.serveMaxBytes) {
    await new Promise((r) => setTimeout(r, 1000));
    t = performance.now();
    await query.runQuery({ snapshot, query: a.query });
    queryAfterRestartMS = performance.now() - t;
    restoredMB = Math.round((await engine.memoryBytes()).serve / (1 << 20));
  }

  return {
    host: "wasm (Chromium worker)",
    files: fetchedFiles,
    bytes,
    components,
    nets,
    boot_ms: Math.round(bootMS),
    fetch_ms: fetchMS < 0 ? "" : Math.round(fetchMS),
    compose_ms: composeMS < 0 ? "" : Math.round(composeMS),
    read_ms: Math.round(readMS),
    check_ms: Math.round(checkMS),
    check_load_ms: split ? Math.round(split.loadMs) : "",
    check_rules_ms: split ? Math.round(split.rulesMs) : "",
    check_again_ms: Math.round(checkAgainMS),
    rules: report?.rulesRun ?? 0,
    findings: (report?.sections ?? []).reduce((n, s) => n + s.count, 0),
    query_ms: Math.round(queryMS),
    query_rows: rows,
    lane_restarts: restarts,
    query_after_restart_ms: queryAfterRestartMS < 0 ? "" : Math.round(queryAfterRestartMS),
    serve_mb_after_restart: restoredMB < 0 ? "" : restoredMB,
    query_during_check_ms: Math.round(queryDuringCheckMS),
    check_still_running: stillRunning ? "yes" : "no",
    first_check_ms: firstCheckMS < 0 ? "aborted" : Math.round(firstCheckMS),
    abort_to_answer_ms: Math.round(abortToAnswerMS),
    serve_mb_after_read: read ? mb(read.memoryBytes) : "",
    serve_live_mb_after_read: read ? mb(read.heap?.heapAlloc) : "",
    stats_gc_ms: statsGcMS < 0 ? "" : Math.round(statsGcMS),
    serve_idle_checks: seen.join("; "),
    jobs_mb_before_check: mb(jobsBytesBeforeCheck),
    check_memory_stages: stageMemory,
    check_alloc_rules: allocRules,
    serve_mb: Math.round(mem.serve / (1 << 20)),
    jobs_mb: a.oneLane ? 0 : Math.round(mem.jobs / (1 << 20)),
    peak_mb: Math.round((a.oneLane ? mem.serve : mem.serve + mem.jobs) / (1 << 20)),
  };
}

(globalThis as unknown as { agniBench: typeof bench }).agniBench = bench;

function mb(bytes: number | undefined): number | string {
  return bytes === undefined ? "" : Math.round(bytes / (1 << 20));
}

// memoryStages renders a RequestTiming's per-stage memory as "stage MB (+growth)" in the order the
// stages ended, keeping only the stages that raised the wasm memory past `before` and every stage
// before them, since it never shrinks and the rest add nothing (agni issue 994).
function memoryStages(header: string | null, before: number): string {
  if (!header) return "";
  const t = JSON.parse(header) as { stages?: { name: string; startMicros?: string; micros?: string; sysBytes?: string }[] };
  const ended = (t.stages ?? [])
    .filter((st) => st.sysBytes !== undefined)
    .map((st) => ({ name: st.name, end: Number(st.startMicros ?? 0) + Number(st.micros ?? 0), sys: Number(st.sysBytes) }))
    .sort((a, b) => a.end - b.end);
  const out: string[] = [];
  let peak = before;
  for (const st of ended) {
    if (st.sys > peak) {
      out.push(`${st.name} ${mb(st.sys)} (+${mb(st.sys - peak)})`);
      peak = st.sys;
    }
  }
  return out.join("; ");
}

// allocators names the five rules of a RequestTiming that allocated most, as "rule MB".
function allocators(header: string | null): string {
  if (!header) return "";
  const t = JSON.parse(header) as { rules?: { rule: string; allocBytes?: string }[] };
  return (t.rules ?? [])
    .map((r) => ({ rule: r.rule, alloc: Number(r.allocBytes ?? 0) }))
    .filter((r) => r.alloc > 0)
    .sort((a, b) => b.alloc - a.alloc)
    .slice(0, 5)
    .map((r) => `${r.rule} ${mb(r.alloc)}`)
    .join("; ");
}
