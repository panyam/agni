// The Lexicon panel's presenter and its edits (agni issue 1267). The panel edits a DRAFT of the
// convention as written, not the effective lexicon, so a saved file holds what the team changed and
// later engine defaults still reach it, and so a convention's rules stay in what Try sends:
// CompareLexicons replaces rules as well as the lexicon, and a candidate without them would report
// their findings as gone. The engine lists every draft (GetLexicon's candidate), so the page merges
// nothing itself.
import { clone, create } from "@bufbuild/protobuf";
import {
  ClassVocabSchema,
  NamingConventionSchema,
  NamingLexiconSchema,
  NetNameVocabSchema,
  PinNameVocabSchema,
  VocabPatternsSchema,
  type ClassVocab,
  type NamingConvention,
  type NamingLexicon,
  type VocabPatterns,
} from "./gen/agni/v1/config/naming_pb.js";
import { LexiconSource, type CompareLexiconsResponse, type GetLexiconResponse, type LexiconEntry, type RenderConventionResponse } from "./gen/agni/v1/webapi/lexicon_pb.js";
import type { Table } from "./gen/agni/v1/webapi/tables_pb.js";
import { conventionError } from "./conventions.js";
import { messageOf } from "./document.js";

// LexiconRow is one entry of the list on screen. source is what the panel prints, and edited marks an
// entry the draft added.
export interface LexiconRow {
  key: string;
  vocabulary: string;
  kind: string;
  value: string;
  source: string;
  replaced: boolean;
  edited: boolean;
  builtin: boolean;
}

export interface LexiconState {
  rows: LexiconRow[];
  name: string;
  // dirty says the draft differs from the convention in effect.
  dirty: boolean;
  busy: boolean;
  error: string;
  // tables is Try's answer, the role changes then the finding changes. Null until Try ran on this
  // draft, since an answer about the previous draft would read as this one's.
  tables: Table[] | null;
}

export interface LexiconView {
  setState: (s: LexiconState) => void;
}

export function emptyLexicon(): LexiconState {
  return { rows: [], name: "", dirty: false, busy: false, error: "", tables: null };
}

const SOURCE_LABELS: Partial<Record<LexiconSource, string>> = {
  [LexiconSource.BUILTIN]: "built-in",
  [LexiconSource.SERVER]: "server",
  [LexiconSource.PROJECT]: "project",
  [LexiconSource.DESIGN]: "design",
  [LexiconSource.REQUEST]: "request",
};

function sourceLabel(e: LexiconEntry): string {
  const s = SOURCE_LABELS[e.source] ?? "";
  return e.convention ? `${s} (${e.convention})` : s;
}

const entryKey = (e: { vocabulary: string; kind: string; value: string }) => `${e.vocabulary}\u0000${e.kind}\u0000${e.value}`;

// rowsOf turns a listing of the draft into rows, each keeping the source it has in the convention in
// effect, so an edit does not relabel every project pattern as this request's.
export function rowsOf(entries: LexiconEntry[], base: LexiconEntry[]): LexiconRow[] {
  const was = new Map(base.map((e) => [entryKey(e), e]));
  return entries.map((e) => {
    const b = was.get(entryKey(e));
    return {
      key: `${entryKey(e)}\u0000${e.replaced}\u0000${e.source}`,
      vocabulary: e.vocabulary,
      kind: e.kind,
      value: e.value,
      source: b ? sourceLabel(b) : "this edit",
      replaced: e.replaced,
      edited: !b,
      builtin: e.source === LexiconSource.BUILTIN,
    };
  });
}

// vocabOf finds a vocabulary of the draft by the path LexiconEntry names it with ("net.gate_drive",
// "class.tvs"), making it when make is set.
function vocabOf(conv: NamingConvention, path: string, make: boolean): (VocabPatterns | ClassVocab) | undefined {
  const [space, name] = path.split(".");
  if (make) conv.lexicon ??= create(NamingLexiconSchema);
  const lx: NamingLexicon | undefined = conv.lexicon;
  if (!lx) return undefined;
  if (space === "class") {
    if (make) lx.class[name] ??= create(ClassVocabSchema);
    return lx.class[name];
  }
  if (make && space === "net") lx.net ??= create(NetNameVocabSchema);
  if (make && space === "pin") lx.pin ??= create(PinNameVocabSchema);
  const group = (space === "net" ? lx.net : lx.pin) as unknown as Record<string, VocabPatterns | undefined> | undefined;
  if (!group) return undefined;
  const field = name.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
  if (make) group[field] ??= create(VocabPatternsSchema);
  return group[field];
}

const LIST_OF: Record<string, string> = { pattern: "patterns", exclude: "exclude", prefix: "prefixes", family: "families" };

function listOf(v: VocabPatterns | ClassVocab, kind: string): string[] {
  const lists = v as unknown as Record<string, string[] | undefined>;
  const name = LIST_OF[kind];
  lists[name] ??= [];
  return lists[name] as string[];
}

// addEntry returns the draft with value added to a vocabulary's list of kind.
export function addEntry(conv: NamingConvention, vocabulary: string, kind: string, value: string): NamingConvention {
  const next = clone(NamingConventionSchema, conv);
  const list = listOf(vocabOf(next, vocabulary, true)!, kind);
  if (!list.includes(value)) list.push(value);
  return next;
}

// canRemove says whether a row can be taken out. A built-in prefix cannot, since `replace` applies to
// patterns and families and leaves the prefix table alone; a house claims a prefix under another class
// instead. A switch is toggled rather than removed.
export function canRemove(r: LexiconRow): boolean {
  return !r.replaced && r.kind !== "switch" && !(r.builtin && r.kind === "prefix");
}

// removeEntry returns the draft without one row. A convention's own entry leaves its list. A built-in
// has no list to leave, so the vocabulary becomes `replace: true` with every other live entry written
// out, which is the one way a convention can drop a single built-in.
export function removeEntry(conv: NamingConvention, rows: LexiconRow[], row: LexiconRow): NamingConvention {
  const next = clone(NamingConventionSchema, conv);
  const v = vocabOf(next, row.vocabulary, true)!;
  if (!row.builtin) {
    const list = listOf(v, row.kind);
    const i = list.indexOf(row.value);
    if (i >= 0) list.splice(i, 1);
    return next;
  }
  v.replace = true;
  const kinds = row.vocabulary.startsWith("class.") ? ["pattern", "family"] : ["pattern", "exclude"];
  for (const kind of kinds) {
    const live = rows.filter((r) => r.vocabulary === row.vocabulary && r.kind === kind && !r.replaced && r.key !== row.key);
    const list = listOf(v, kind);
    list.length = 0;
    list.push(...live.map((r) => r.value));
  }
  return next;
}

// setOff switches a vocabulary's matching off, `replace: true` with nothing listed, or on again by
// dropping what the draft says about it.
export function setOff(conv: NamingConvention, vocabulary: string, off: boolean): NamingConvention {
  const next = clone(NamingConventionSchema, conv);
  const [space, name] = vocabulary.split(".");
  if (!off) {
    if (space === "class") {
      if (next.lexicon) delete next.lexicon.class[name];
    } else {
      const group = (space === "net" ? next.lexicon?.net : next.lexicon?.pin) as unknown as Record<string, unknown> | undefined;
      if (group) group[name.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase())] = undefined;
    }
    return next;
  }
  const v = vocabOf(next, vocabulary, true)!;
  v.replace = true;
  for (const kind of space === "class" ? ["pattern", "family"] : ["pattern", "exclude"]) listOf(v, kind).length = 0;
  return next;
}

// setInference sets the rail-inference switch.
export function setInference(conv: NamingConvention, on: boolean): NamingConvention {
  const next = clone(NamingConventionSchema, conv);
  next.lexicon ??= create(NamingLexiconSchema);
  next.lexicon.net ??= create(NetNameVocabSchema);
  next.lexicon.net.inferRailFromSupplyPins = on;
  return next;
}

// LexiconClient is the part of LexiconService the panel calls.
export interface LexiconClient {
  getLexicon(req: { snapshot: string; candidate?: NamingConvention }): Promise<GetLexiconResponse>;
  compareLexicons(req: { snapshot: string; conventions: NamingConvention }): Promise<CompareLexiconsResponse>;
  renderConvention(req: { conventions: NamingConvention }): Promise<RenderConventionResponse>;
}

export interface LexiconPorts {
  // snapshot is the open design under the convention in effect, "" with none open.
  snapshot: () => Promise<string>;
  download: (file: { filename: string; contentType: string; content: Uint8Array }) => void;
  apply: (convention: NamingConvention) => Promise<void>;
}

// LexiconPresenter holds the draft and asks the engine about it.
export class LexiconPresenter {
  private draft = create(NamingConventionSchema, { name: "conventions" });
  private base: LexiconEntry[] = [];
  private state = emptyLexicon();
  // ticket drops an answer a later edit or design overtook.
  private ticket = 0;

  constructor(
    private client: LexiconClient,
    private view: LexiconView,
    private ports: LexiconPorts,
  ) {}

  private push(patch: Partial<LexiconState>): void {
    this.state = { ...this.state, ...patch };
    this.view.setState(this.state);
  }

  // reload starts over from the convention in effect.
  reload(): void {
    void this.load();
  }

  private async load(): Promise<void> {
    const t = ++this.ticket;
    try {
      const snapshot = await this.ports.snapshot();
      if (!snapshot) return;
      const resp = await this.client.getLexicon({ snapshot });
      if (t !== this.ticket) return;
      this.base = resp.entries;
      this.draft = resp.convention ? clone(NamingConventionSchema, resp.convention) : create(NamingConventionSchema, { name: "conventions" });
      this.push({ rows: rowsOf(resp.entries, resp.entries), name: this.draft.name, dirty: false, busy: false, error: "", tables: null });
    } catch (e) {
      if (t === this.ticket) this.push({ busy: false, error: conventionError(messageOf(e)) });
    }
  }

  // edit lists a new draft through the engine and takes it only if it compiles, so a bad pattern stays
  // an inline error and never reaches a run.
  private async edit(next: NamingConvention): Promise<void> {
    const t = ++this.ticket;
    try {
      const snapshot = await this.ports.snapshot();
      if (!snapshot) return;
      const resp = await this.client.getLexicon({ snapshot, candidate: next });
      if (t !== this.ticket) return;
      this.draft = next;
      this.push({ rows: rowsOf(resp.entries, this.base), name: next.name, dirty: true, error: "", tables: null });
    } catch (e) {
      if (t === this.ticket) this.push({ error: conventionError(messageOf(e)) });
    }
  }

  add(vocabulary: string, kind: string, value: string): Promise<void> {
    return this.edit(addEntry(this.draft, vocabulary, kind, value.trim()));
  }

  remove(row: LexiconRow): Promise<void> {
    return this.edit(removeEntry(this.draft, this.state.rows, row));
  }

  switchOff(vocabulary: string, off: boolean): Promise<void> {
    return this.edit(setOff(this.draft, vocabulary, off));
  }

  infer(on: boolean): Promise<void> {
    return this.edit(setInference(this.draft, on));
  }

  rename(name: string): Promise<void> {
    const next = clone(NamingConventionSchema, this.draft);
    next.name = name;
    return this.edit(next);
  }

  async tryDraft(): Promise<void> {
    const t = ++this.ticket;
    this.push({ busy: true, error: "" });
    try {
      const snapshot = await this.ports.snapshot();
      if (!snapshot) return;
      const resp = await this.client.compareLexicons({ snapshot, conventions: this.draft });
      if (t === this.ticket) this.push({ busy: false, tables: resp.tables });
    } catch (e) {
      if (t === this.ticket) this.push({ busy: false, error: conventionError(messageOf(e)) });
    }
  }

  async save(): Promise<void> {
    try {
      const resp = await this.client.renderConvention({ conventions: this.draft });
      this.ports.download({ filename: resp.filename, contentType: resp.contentType, content: resp.content });
    } catch (e) {
      this.push({ error: conventionError(messageOf(e)) });
    }
  }

  async apply(): Promise<void> {
    this.push({ busy: true, error: "" });
    try {
      await this.ports.apply(clone(NamingConventionSchema, this.draft));
    } catch (e) {
      this.push({ busy: false, error: conventionError(messageOf(e)) });
    }
  }
}

// subjectOf is what a comparison row selects: its net, or a finding's subject, since an appeared
// finding is not in the page's run and has no row to focus.
export function subjectOf(t: Table, cells: string[]): { kind: string; name: string } | null {
  const at = (c: string) => t.columns.findIndex((x) => x.name === c);
  const net = at("net");
  if (net >= 0) return { kind: "net", name: cells[net] };
  const subject = at("subject");
  if (subject < 0 || !cells[subject]) return null;
  return { kind: cells[at("subject_kind")] || "net", name: cells[subject] };
}
