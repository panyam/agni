// A page's snapshots (agni issue 1061). Every analysis rpc names a snapshot of a design rather than
// its files, so the page ingests each design it opens once and sends the snapshot's name with every
// question about it.
//
// The page mints each snapshot's id itself and keeps its definition, because a host may not hold the
// snapshot when a question arrives: a server evicts and restarts, and each browser worker holds only
// what was ingested into it. A question answered NOT_FOUND or FAILED_PRECONDITION with a
// SnapshotUnavailable detail is recovered here, by ingesting the snapshot again under its id or
// updating it, and asked once more.
import { Code, ConnectError, type Interceptor } from "@connectrpc/connect";
import { toJsonString, type MessageInitShape } from "@bufbuild/protobuf";
import { OverlayConfigSchema, type OverlayConfig } from "./gen/agni/v1/webapi/checks_pb.js";
import {
  IngestRequestSchema,
  SnapshotUnavailableSchema,
  SnapshotUnavailable_Reason,
  UpdateSnapshotRequestSchema,
  type Snapshot,
} from "./gen/agni/v1/webapi/snapshot_pb.js";

type IngestInit = MessageInitShape<typeof IngestRequestSchema>;
type UpdateInit = MessageInitShape<typeof UpdateSnapshotRequestSchema>;

// SnapshotCalls is what a registry asks a host, the two SnapshotService methods it needs.
export interface SnapshotCalls {
  ingest(req: IngestInit): Promise<Snapshot>;
  updateSnapshot(req: UpdateInit): Promise<Snapshot>;
}

// A Definition is what a page ingests: a design, the config its rule-running questions carry, and
// whether its hierarchical netlist is read whole.
interface Definition {
  uri: string;
  overlay?: OverlayConfig;
  flattenHierarchy?: boolean;
}

// RegistryOptions are what every snapshot a page ingests shares.
export interface RegistryOptions {
  // flatten reads every design's hierarchical netlist whole (Snapshot.flatten_hierarchy, agni issue
  // 1102). It is the page's, from its address (pageFlattens), so a design read under one choice
  // is never asked about under the other.
  flatten?: boolean;
}

// SnapshotSource names a design's snapshot under a config, which is all a presenter asks of the
// registry.
export interface SnapshotSource {
  of(uri: string, overlay?: OverlayConfig): Promise<string>;
  readBoard(uri: string, overlay?: OverlayConfig): Promise<Snapshot>;
}

// SnapshotRegistry ingests each design the page names once per config and remembers how, so it can
// be ingested again wherever it is missing.
//
// The viewer asks for a design's drawing under no overlay and runs its rules under the convention
// the vocabulary bar shows, so one open design has up to two snapshots. Keeping them apart is what
// keeps a change of convention from drawing every sheet again, since a drawing read under one
// overlay is not reused under another.
export class SnapshotRegistry implements SnapshotSource {
  private readonly byKey = new Map<string, Promise<string>>();
  private readonly defs = new Map<string, Definition>(); // name -> definition

  constructor(
    private readonly calls: SnapshotCalls,
    private readonly newId: () => string = randomId,
    private readonly options: RegistryOptions = {},
  ) {}

  // def is the definition the page ingests uri under.
  private def(uri: string, overlay?: OverlayConfig): Definition {
    return this.options.flatten ? { uri, overlay, flattenHierarchy: true } : { uri, overlay };
  }

  // of returns the name of the snapshot of uri under overlay, ingesting it on first use. A failed
  // ingest is forgotten, so the next ask tries again rather than failing forever.
  of(uri: string, overlay?: OverlayConfig): Promise<string> {
    const key = keyOf(uri, overlay);
    let p = this.byKey.get(key);
    if (!p) {
      const def = this.def(uri, overlay);
      p = this.calls.ingest({ snapshot: def, snapshotId: this.newId() }).then((s) => {
        this.defs.set(s.name, def);
        return s.name;
      });
      p.catch(() => this.byKey.delete(key));
      this.byKey.set(key, p);
    }
    return p;
  }

  // readBoard asks the host to read the board of uri's snapshot under overlay now, so a later Run
  // checks finds it parsed (agni issue 1061). It answers the snapshot as the host reports it.
  async readBoard(uri: string, overlay?: OverlayConfig): Promise<Snapshot> {
    const name = await this.of(uri, overlay);
    return this.calls.ingest({ snapshot: this.def(uri, overlay), snapshotId: idOf(name), readBoard: true });
  }

  // uriOf is the design a snapshot this page ingested names, or undefined for one it did not.
  uriOf(name: string): string | undefined {
    return this.defs.get(name)?.uri;
  }

  // recover makes name answerable again: ingested under its id when the host lost it, updated when its
  // files changed. It reports whether it did anything, which is false for a snapshot this page did not
  // ingest.
  async recover(name: string, reason: SnapshotUnavailable_Reason): Promise<boolean> {
    const def = this.defs.get(name);
    if (!def) return false;
    if (reason === SnapshotUnavailable_Reason.STALE) {
      await this.calls.updateSnapshot({ snapshot: { name, ...def } });
    } else {
      await this.calls.ingest({ snapshot: def, snapshotId: idOf(name) });
    }
    return true;
  }
}

// keyOf is one key per design and config, the overlay compared by its JSON.
function keyOf(uri: string, overlay?: OverlayConfig): string {
  return overlay ? `${uri}\u0000${toJsonString(OverlayConfigSchema, overlay)}` : uri;
}

// recoveringInterceptor asks a question again, once, after the registry has made its snapshot
// answerable, when the host said the snapshot was missing or stale.
export function recoveringInterceptor(registry: () => SnapshotRegistry | undefined): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (e) {
      const ce = ConnectError.from(e);
      if (ce.code !== Code.NotFound && ce.code !== Code.FailedPrecondition) throw e;
      const d = ce.findDetails(SnapshotUnavailableSchema)[0];
      const reg = registry();
      if (!d || !reg || !(await reg.recover(d.name, d.reason))) throw e;
      return next(req);
    }
  };
}

// idOf is a snapshot name's id, "snapshots/{id}".
export function idOf(name: string): string {
  return name.replace(/^snapshots\//, "");
}

// randomId is 16 hex digits.
function randomId(): string {
  const b = new Uint8Array(8);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}
