// The Load page's work (agni issue 1072): read a design into the engine's store, say what it is doing
// while it does, record it, and hand over to the viewer. The viewer then opens a design whose
// drawing, netlist and board are already parsed, so no panel there waits on a first read.
//
// The page is plain DOM over this module (load.ts), which takes its engine calls as ports so a test
// can drive the whole flow.

// A LoadStepId is one stage the page shows. files is fetching the design's files into the browser
// engine, and is skipped where nothing is fetched (the server engine, a dropped design).
export type LoadStepId = "files" | "drawing" | "board";

export type LoadStepState = "waiting" | "running" | "done" | "failed";

export interface LoadStep {
  id: LoadStepId;
  label: string;
  state: LoadStepState;
  // detail says how far the step got or what it found, "" when there is nothing to add.
  detail: string;
}

// LoadView is what the flow pushes to the page.
export interface LoadView {
  steps(steps: LoadStep[]): void;
  // failed says the design could not be read, with the engine's reason. The page then offers the
  // viewer anyway, since a design with no netlist still has a drawing to look at.
  failed(reason: string): void;
}

// A Listing is what the design's files are, for the files step's progress.
export interface Listing {
  // stamp is what a record of this load must match (engineboot.designStamp).
  stamp: string;
  files: number;
  bytes: number;
}

// BoardRead is what reading the board found, from the Ingest answer's tiers.
export interface BoardRead {
  boardRead: boolean;
  boardPersisted: boolean;
}

export interface LoadDeps {
  uri: string;
  // fetches is whether the page fetches the design's files itself (the browser engine reading a
  // design the server or a static host holds).
  fetches: boolean;
  listing(): Promise<Listing>;
  // drawing reads the design's sheets (GetDesign), board its netlist and board (Ingest with
  // read_board), each into the engine's store.
  drawing(): Promise<void>;
  board(): Promise<BoardRead>;
  record(stamp: string): void;
  // open hands over to the viewer.
  open(): void;
  now?: () => number;
  log?: (line: string) => void;
}

// LoadFlow runs one load and keeps the steps it shows. fetched is called as each design file
// arrives, so the files step counts them.
export class LoadFlow {
  private readonly list: LoadStep[];
  private total = { files: 0, bytes: 0 };
  private got = new Map<string, number>();

  constructor(
    private readonly deps: LoadDeps,
    private readonly view: LoadView,
  ) {
    this.list = [
      ...(deps.fetches ? [{ id: "files" as const, label: "Fetching the design's files", state: "waiting" as const, detail: "" }] : []),
      { id: "drawing", label: "Reading the schematic", state: "waiting", detail: "" },
      { id: "board", label: "Reading the netlist and the board", state: "waiting", detail: "" },
    ];
  }

  steps(): LoadStep[] {
    return this.list.map((s) => ({ ...s }));
  }

  // fetched counts one design file the page fetched. A file fetched again for another worker counts
  // once, since the step says how much of the design has arrived.
  fetched(path: string, bytes: number): void {
    this.got.set(path, bytes);
    this.set("files", { detail: this.filesDetail() });
  }

  async run(): Promise<boolean> {
    const now = this.deps.now ?? (() => performance.now());
    const started = now();
    let listing: Listing;
    try {
      if (this.deps.fetches) this.set("files", { state: "running" });
      listing = await this.deps.listing();
      this.total = { files: listing.files, bytes: listing.bytes };
      if (this.deps.fetches) this.set("files", { detail: this.filesDetail() });
      this.set("drawing", { state: "running" });
      await this.deps.drawing();
      this.set("drawing", { state: "done" });
      this.set("board", { state: "running" });
      const b = await this.deps.board();
      // Every file a read needs has arrived by now, the board's included.
      if (this.deps.fetches) this.set("files", { state: "done", detail: this.filesDetail() });
      this.set("board", { state: "done", detail: b.boardRead ? "" : "this design has no board" });
      const ms = Math.round(now() - started);
      // The public demo's walk reads this line (web/browser/exercise.mjs).
      this.deps.log?.(`agni loaded ${this.deps.uri} in ${ms} ms: board ${b.boardRead ? (b.boardPersisted ? "persisted" : "not persisted") : "absent"}`);
    } catch (e) {
      // The files step runs beside the reads, so the step that failed is the last one running, and
      // any other stops where it was.
      const running = this.list.filter((s) => s.state === "running");
      for (const s of running.slice(0, -1)) this.set(s.id, { state: "waiting" });
      const last = running[running.length - 1];
      if (last) this.set(last.id, { state: "failed" });
      this.view.failed(e instanceof Error ? e.message : String(e));
      return false;
    }
    this.deps.record(listing.stamp);
    this.deps.open();
    return true;
  }

  private filesDetail(): string {
    const n = this.got.size;
    const mb = (b: number) => (b / (1 << 20)).toFixed(1);
    const bytes = [...this.got.values()].reduce((a, b) => a + b, 0);
    if (!this.total.files) return n ? `${n} files, ${mb(bytes)} MB` : "";
    return `${Math.min(n, this.total.files)} of ${this.total.files} files, ${mb(Math.min(bytes, this.total.bytes))} of ${mb(this.total.bytes)} MB`;
  }

  private set(id: LoadStepId, change: Partial<LoadStep>): void {
    const s = this.list.find((x) => x.id === id);
    if (!s) return;
    Object.assign(s, change);
    this.view.steps(this.steps());
  }
}
