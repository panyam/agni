// query.ts is the framework-neutral state for the datalog query panel (WS9-036). The presenter
// pushes a QueryResult and the panel renders it. The shape lives here rather than in the .tsx, as in
// rules.ts and findings.ts, so the presenter and tests build state without touching Solid.
import { type QuerySet, type QueryValue, type QueryWitness, type RelationEntry, type RunQueryResponse } from "./gen/agni/v1/webapi/query_pb.js";
import { LocateReason } from "./gen/agni/v1/checks/checks_pb.js";
import type { FindingsState, SheetBadge } from "./findings.js";
import type { Selection } from "./selection.js";

export { LocateReason };

// QueryRowItem is one answer row. cells align positionally with QueryResult.columns, and cites is
// the row's provenance, shown on demand so the table stays narrow. cellSheets aligns with cells and
// holds the sheet badges a navigable cell (component/net) can jump to (WS9-038), [] for a scalar or
// unlocated cell.
export interface QueryRowItem {
  cells: string[];
  cites: string[];
  cellSheets: SheetBadge[][];
  cellReasons: LocateReason[];
  // cellKinds aligns with cells and overrides columnKinds for this row (agni issue 338). It is ""
  // everywhere except a POLYMORPHIC column, where the answer set holds more than one sort of thing
  // and the column cannot say which. Read it through cellKind, never directly.
  cellKinds: string[];
  // cellRefs aligns with cells and carries the other half of a cell's identity, which today means a
  // pin cell's component. "" everywhere else.
  cellRefs: string[];
  // witnesses say how the row holds (agni issue 794), one per literal of the goal, and are empty
  // until the answer was asked for them (QueryResult.witnessed).
  witnesses: WitnessItem[];
}

// WitnessItem is one fact behind a row: a leaf with its citations, or a derived fact with the rule
// that derived it and that rule's body facts as children, in the rule's written order. A negated
// leaf is a `not` that held, an absence, with neither citations nor children.
export interface WitnessItem {
  relation: string;
  rule: string;
  negated: boolean;
  cites: string[];
  values: WitnessValue[];
  children: WitnessItem[];
}

// WitnessValue is one value of a witness, typed as a cell is: kind is "" for a scalar, and an entity
// value carries what a cell click needs to locate it (its pin's component in ref, its sheets, and
// why it cannot be drawn).
export interface WitnessValue {
  text: string;
  kind: string;
  ref: string;
  sheets: SheetBadge[];
  reason: LocateReason;
}

// QueryResult is the panel's whole state. `ran` distinguishes "not run yet" (blank panel) from
// "ran and matched nothing" (no results). `error` is the inline message for a bad query or an
// unloadable design; when set, columns/rows are empty. `loading` gates the run button. columnKinds
// aligns with columns ("component" | "net" | "" scalar) and decides which cells are clickable to
// locate the entity in the canvas (WS9-038).
export interface QueryResult {
  columns: string[];
  columnKinds: string[];
  rows: QueryRowItem[];
  error: string;
  loading: boolean;
  ran: boolean;
  // total is how many rows the answer had when a limit cut it to rows (agni issue 896), and 0 when
  // rows is the whole answer. The panel then says how many it left out and offers the rest.
  total: number;
  // errorAt is the line of query text a query that would not parse stopped on, with a caret under
  // the place, as the engine drew it (TextPosition.caret), shown under the error. Absent for every
  // other error.
  errorAt?: string;
  // witnessed says the answer was asked for witnesses (agni issue 794), so a row with none has none
  // to show, rather than none fetched yet.
  witnessed?: boolean;
}

// STARTING_POINT_LIMIT caps the rows a question about one picked entity asks for (agni issue 896), so
// a click on a ground net carrying a thousand pins draws the first fifty fast and offers the rest
// rather than waiting for all of them.
export const STARTING_POINT_LIMIT = 50;

// RelationItem is one entry in the relation picker (WS9-037). It holds the name, the arg labels a
// template inserts as ?arg, a one-line summary (hover), a kind for grouping, and (WS14-005) the
// reference markdown the panel renders on demand ("" when the relation has no doc yet).
export interface RelationItem {
  name: string;
  args: string[];
  summary: string;
  kind: string;
  detail: string;
}

// RelationEntryItem is one place in the relation namespace as ListRelations describes it for a path
// (agni issue 774): a module with its members one level deep, or a member with its signature, doc,
// the argument names a template inserts, which argument types the engine inferred, and a derived
// member's definition. The picker drills through these rather than a flat list.
export interface RelationEntryItem {
  path: string;
  // kind is "module", "base", "predicate" or "derived".
  kind: string;
  signature: string;
  doc: string;
  detail: string;
  module: string;
  definition: string[];
  inferred: string[];
  args: string[];
  members: RelationEntryItem[];
}

// relationEntryItem converts ListRelations' answer for a path to the picker's item, members one level
// deep as the server sends them.
export function relationEntryItem(e: RelationEntry): RelationEntryItem {
  return {
    path: e.path,
    kind: e.entryKind,
    signature: e.signature,
    doc: e.doc,
    detail: e.detail,
    module: e.module,
    definition: [...e.definition],
    inferred: [...e.inferred],
    args: [...e.args],
    members: e.members.map(relationEntryItem),
  };
}

// ROOT_PATH is the path the picker opens on, the namespace's root as ListRelations spells it.
export const ROOT_PATH = ".";

// relationCrumbs splits a path into the breadcrumb the picker draws, root first: "net.has_test_point"
// is the root, "net", then "net.has_test_point".
export function relationCrumbs(path: string): { label: string; path: string }[] {
  const out = [{ label: "Relations", path: ROOT_PATH }];
  if (path === ROOT_PATH || path === "") return out;
  const parts = path.split(".");
  for (let i = 0; i < parts.length; i++) out.push({ label: parts[i], path: parts.slice(0, i + 1).join(".") });
  return out;
}

// tierLabel is the picker's label for a relation's tier (Netlist, Board, ...), read off the flat
// catalog by name, or "" for a module or a relation the catalog does not list.
export function tierLabel(path: string, relations: readonly RelationItem[]): string {
  const r = relations.find((x) => x.name === path);
  return r ? (RELATION_KINDS.find((k) => k.key === r.kind)?.label ?? "") : "";
}

// ExampleItem is one starter query (WS14-002), holding a plain-language label, the datalog query a
// click fills and runs, and the concept it teaches. The server orders them as a concept ladder.
export interface ExampleItem {
  label: string;
  query: string;
  teaches: string;
}

// QueryView is the command-down surface. The presenter (or the composition root, for the static
// relation and example catalog) calls setState with each result, and setRelations/setExamples once
// at boot. setLocateNote pushes the WS9-039 explanation shown when a cell click highlighted nothing
// ("" to clear it). The presenter drives the note because only it knows whether the highlight
// resolved.
export interface QueryView {
  // setState pushes one tab's answer (agni issue 902). tab is the id the panel's onRun named, so an
  // answer lands in the tab that asked even after the reader has moved to another. With no tab it
  // goes to the tab in front.
  setState: (s: QueryResult, tab?: string) => void;
  // clearAnswers empties every tab's answer and keeps their questions, for a change that makes every
  // answer on screen stale, such as a different naming convention.
  clearAnswers: () => void;
  setRelations: (rels: RelationItem[]) => void;
  setExamples: (examples: ExampleItem[]) => void;
  setLocateNote: (message: string) => void;
  // setQuery hands the panel a query written FOR the reader, with the values its variables are bound
  // to (agni issue 793). It opens a tab holding the query and its binding chips and runs it, leaving
  // an editable query behind.
  // opts names the tab and asks for the whole answer, which a question arriving by link wants (agni
  // issue 634); without it the tab is numbered and shows a starting page of rows.
  setQuery: (text: string, bindings?: Bindings, opts?: { title?: string; whole?: boolean }) => void;
  // setEntityQueries hands over the served questions about a selected entity, several per kind.
  setEntityQueries: (presets: EntityQueryItem[]) => void;
  // setSearch hands over the find-by-name template. Until it arrives the panel offers no search
  // mode, rather than one that would run a guessed query.
  setSearch: (search: SearchItem | null) => void;
  // setSelection names the entity the reader last picked on the drawing, so the panel can say what
  // is selected and offer the question to ask about it next. Clicking a result cell selects too,
  // but the panel handles that itself. This is the entry point for the OTHER picker, the canvas.
  setSelection: (sel: Selection | null) => void;
  // setFindings hands over the CURRENT check results, whole (agni issue 259). The panel projects
  // them onto whatever is selected and re-runs nothing, since it holds no client. It takes the full
  // state rather than a count because `pending` and `running` separate "nothing is wrong with this
  // net" from "nobody has checked".
  setFindings: (state: FindingsState) => void;
  // setCurrentSheet names the sheet on screen. The panel marks the badge that points at it, so a
  // reader who navigated away can still see which answer they navigated FROM.
  setCurrentSheet: (sheetId: string) => void;
  // questionsFor returns the served questions for one entity kind, in the server's order, and none
  // before the catalog has arrived, so nothing offers a question that would run a guess.
  questionsFor: (kind: string) => EntityQueryItem[];
  // askAbout selects sel and opens a tab asking one served question about it, the kind's first when
  // none is named. It is how Properties' Ask rows reach the query panel (agni issue 902).
  askAbout: (sel: Selection, question?: EntityQueryItem) => void;
  // setQuerySets lists the query sets the design's project keeps (agni issue 745), which the panel
  // offers to open. Empty for a design in no project or one keeping none.
  setQuerySets: (sets: QuerySetItem[]) => void;
  // openSet opens one tab per query of a set, in the set's order, and asks the host to answer them
  // over one read through onRunSet.
  openSet: (set: QuerySet) => void;
}

// QuerySetItem is one set a project keeps: its name (the file's stem), the file, and the set.
export interface QuerySetItem {
  name: string;
  source: string;
  set: QuerySet;
}

// SetTab is what one query of a set opens as: a tab named for the query, holding the set's preamble
// and the query as one text, so editing and running the tab again asks what the set asked.
export interface SetTab {
  title: string;
  text: string;
  bindings: Bindings;
  // numbers names the bindings the set gives as numbers, which a chip spells bare (formatBinding). A
  // number inside a list is named by its position, numberKey(name, i).
  numbers: string[];
}

// tabsFromSet turns a set into its tabs, in the set's order. A preamble is rules alone, so it joins
// the query as the clauses before its goal, which is how a single query carries its own rules.
export function tabsFromSet(set: QuerySet): SetTab[] {
  const preamble = set.preamble.trim().replace(/;\s*$/, "");
  return set.queries.map((q) => ({
    title: q.name,
    text: preamble ? `${preamble};\n${q.query.trim()}` : q.query.trim(),
    bindings: textBindings(q.bindings),
    numbers: Object.entries(q.bindings).flatMap(([name, v]) => {
      if (v.kind.case === "number") return [name];
      if (v.kind.case !== "list") return [];
      return v.kind.value.values.flatMap((e, i) => (e.kind.case === "number" ? [numberKey(name, i)] : []));
    }),
  }));
}

// BindValue is what one variable is bound to in the panel, a value or a list of values the goal
// answers for each of (agni issue 1121). Values are held as text, as the chips show them.
export type BindValue = string | string[];
export type Bindings = Record<string, BindValue>;

// numberKey names the i-th value of a list binding in a SetTab's numbers.
export function numberKey(name: string, i: number): string {
  return `${name}[${i}]`;
}

// wireBinding is how one chip's value travels in a RunQueryRequest. The panel binds text, so a list
// travels as a list of text values.
export function wireBinding(value: BindValue) {
  if (Array.isArray(value)) return { kind: { case: "list" as const, value: { values: value.map((v) => ({ kind: { case: "text" as const, value: v } })) } } };
  return { kind: { case: "text" as const, value } };
}

// textBindings spells a set's bound values as the panel's chips hold them. A number sent back as
// text is still read as a number where the relation declares one (panyam/jaala#65). A list keeps its
// order, and the server refuses a list inside a list, so an element is a value or nothing.
export function textBindings(b: { [name: string]: QueryValue }): Bindings {
  const out: Bindings = {};
  for (const [name, v] of Object.entries(b)) {
    if (v.kind.case === "list") out[name] = v.kind.value.values.flatMap((e) => (e.kind.case === "text" ? [e.kind.value] : e.kind.case === "number" ? [String(e.kind.value)] : []));
    else if (v.kind.case === "text") out[name] = v.kind.value;
    else if (v.kind.case === "number") out[name] = String(v.kind.value);
  }
  return out;
}

// EntityQueryItem is one served question about a selected entity. label words it for an Ask row,
// query is the datalog it runs, and binds names the variables the client binds from the selection
// (see entityBindings in selection.ts).
export interface EntityQueryItem {
  kind: string;
  label: string;
  query: string;
  teaches: string;
  binds: string[];
}

// RELATION_KINDS is the group order + display label for the picker; a kind with no members renders no
// group. Its keys are facts.KindOrder on the server, in order, which query.test.ts and core/facts
// both hold to core/facts/testdata/kind_order.json (agni issue 1045); the client owns only the labels.
export const RELATION_KINDS: readonly { key: string; label: string }[] = [
  { key: "netlist", label: "Netlist" },
  { key: "board", label: "Board" },
  { key: "datasheet", label: "Datasheet" },
  { key: "derived", label: "Library" },
  { key: "predicate", label: "Predicates" },
  { key: "extension", label: "Extensions" },
];

// relationTemplate renders a relation as an insertable snippet: name(?arg1, ?arg2). A zero-arg
// relation (none today) inserts name().
export function relationTemplate(r: { name: string; args: readonly string[] }): string {
  return `${r.name}(${r.args.map((a) => "?" + a).join(", ")})`;
}

// cellKind returns the kind of one result cell, the row's own if it has one, else the column's.
// Kind is usually a column property because a variable binds at the same relation position in every
// row, EXCEPT under a relation whose answer says what each row is about (entity(?name, ?kind), agni
// issue 338). Every reader of a cell's kind goes through here so the two sources never drift apart.
// An ordinary query carries no cellKinds and falls through to the column.
export function cellKind(result: QueryResult, row: QueryRowItem, i: number): string {
  return row.cellKinds[i] || result.columnKinds[i] || "";
}

// SearchItem is the served find-by-name query (agni issue 338) and the concept it leaves behind. A
// search binds the variable `bind` names to `pattern` with {term} replaced by the escaped term
// (agni issue 793), so the query text never changes. It is served rather than written here because
// it names `entity` and `str.match`, both defined in Go, and the pattern holds the (?i).
export interface SearchItem {
  query: string;
  teaches: string;
  bind: string;
  pattern: string;
}

// searchPattern turns what a reader TYPED into a regex that matches it literally, mirroring Go's
// regexp.QuoteMeta. A design is full of names the regex engine would otherwise read as syntax
// (VDD+, USB_D-, DATA[7:0], VREF(A)), and a reader typing one of those means the characters. A
// double quote is kept, since the pattern is bound as a value rather than spliced into a string
// literal (agni issue 793).
export function searchPattern(term: string): string {
  return term.replace(/[\\.+*?()|[\]{}^$]/g, "\\$&");
}

// searchBindings is what a search binds: the served pattern with {term} replaced by the escaped
// term. split/join rather than replace, because a replacement string reads `$&` and `$1` as syntax.
// The caller handles an empty term, and the panel refuses to run one rather than enumerate a whole
// design into a table that renders every row.
export function searchBindings(search: SearchItem, term: string): Record<string, string> {
  return { [search.bind]: search.pattern.split("{term}").join(searchPattern(term)) };
}

// bindingsInUse keeps the bindings whose variable the query text still names. A reader who edits a
// bound variable out of the query has stopped asking about it, and the server refuses a binding the
// goal does not use.
export function bindingsInUse(text: string, bindings: Bindings): Bindings {
  const out: Bindings = {};
  for (const [name, value] of Object.entries(bindings)) {
    if (new RegExp(`\\?${name.replace(/[^\w]/g, "")}\\b`).test(text)) out[name] = value;
  }
  return out;
}

// formatBinding spells one binding as the query would write the constant, for a chip or a tooltip.
// The text goes between quotes as it is, since a query string takes no escapes (jaala v0.1.62), so a
// search pattern's backslash shows once. A value with a bare quote has no spelling and is shown the
// same way; it still binds, since a binding never travels as text. A number is spelled bare. Both
// spellings are FormatBindings' in core/query, and query.test.ts and core/query read one fixture,
// testdata/binding_twin.json, so the chip and the CLI's document cannot drift (agni issue 1045).
//
// A list is spelled `?n in ["A", "B"]`, each value as a single one would be (agni issue 1121). number
// then says which of its values are numbers, one flag per value, or one flag for all of them.
export function formatBinding(name: string, value: BindValue, number: boolean | readonly boolean[] = false): string {
  const spell = (v: string, i: number) => ((Array.isArray(number) ? number[i] : number) ? v : `"${v}"`);
  if (Array.isArray(value)) return `?${name} in [${value.map(spell).join(", ")}]`;
  return `?${name} = ${spell(value, 0)}`;
}

// emptyResult is the initial state (nothing run yet) and the reset the panel shows while a query
// is in flight.
export function emptyResult(loading = false): QueryResult {
  return { columns: [], columnKinds: [], rows: [], error: "", loading, ran: false, total: 0 };
}

// resultFromResponse projects a RunQueryResponse into panel state. The injected resolver turns
// each navigable cell's sheet ids into display badges (the presenter's sheetBadges, which
// suppresses badges on a single-sheet design and maps id->name). The default yields no badges, so
// tests need not pass one. witnessed says the request asked for witnesses.
export function resultFromResponse(
  resp: RunQueryResponse,
  resolveSheets: (ids: string[]) => SheetBadge[] = () => [],
  witnessed = false,
): QueryResult {
  return {
    columns: resp.columns,
    columnKinds: resp.columnKinds ?? [],
    // An entity cell's sheets and reason, and a row's citations, are kept once on the response and
    // the row points into them (agni issue 916).
    rows: resp.rows.map((r) => {
      const entity = (i: number) => {
        const n = r.cellEntity?.[i] ?? 0;
        return n > 0 ? resp.entities?.[n - 1] : undefined;
      };
      return {
        cells: r.cells,
        cites: (r.citeIndex ?? []).map((i) => resp.sources?.[i] ?? ""),
        cellSheets: r.cells.map((_, i) => resolveSheets(entity(i)?.sheetIds ?? [])),
        cellReasons: r.cells.map((_, i) => entity(i)?.reason ?? LocateReason.UNSPECIFIED),
        cellKinds: r.cells.map((_, i) => r.cellKinds?.[i] ?? ""),
        cellRefs: r.cells.map((_, i) => r.cellRefs?.[i] ?? ""),
        witnesses: (r.witnesses ?? []).map((w) => witnessItem(resp, w, resolveSheets)),
      };
    }),
    error: "",
    loading: false,
    ran: true,
    total: resp.total > resp.rows.length ? resp.total : 0,
    witnessed,
  };
}

// witnessItem reads one witness off the wire, its values through the response's entities as a cell
// is read and its citations through the response's sources as a row's are (agni issue 916).
function witnessItem(resp: RunQueryResponse, w: QueryWitness, resolveSheets: (ids: string[]) => SheetBadge[]): WitnessItem {
  return {
    relation: w.relation,
    rule: w.rule,
    negated: w.negated,
    cites: (w.citeIndex ?? []).map((i) => resp.sources?.[i] ?? ""),
    values: w.values.map((text, i) => {
      const n = w.valueEntity?.[i] ?? 0;
      const e = n > 0 ? resp.entities?.[n - 1] : undefined;
      if (!e) return { text, kind: "", ref: "", sheets: [], reason: LocateReason.UNSPECIFIED };
      return { text, kind: e.kind, ref: e.kind === "pin" ? e.ref : "", sheets: resolveSheets(e.sheetIds ?? []), reason: e.reason };
    }),
    children: (w.children ?? []).map((c) => witnessItem(resp, c, resolveSheets)),
  };
}

// drawnSheets reads where one entity is drawn off a query answer naming it, the sheet ids its answer
// entity carries, or none when the answer does not name it. Properties asks the find-by-name query
// with the whole name anchored and reads the row of the right kind, since a part and a net can share
// a name.
export function drawnSheets(resp: RunQueryResponse, kind: string, name: string): string[] {
  for (const r of resp.rows) {
    for (let i = 0; i < r.cells.length; i++) {
      const k = r.cellKinds?.[i] || resp.columnKinds?.[i] || "";
      if (k !== kind || r.cells[i] !== name) continue;
      const n = r.cellEntity?.[i] ?? 0;
      return n > 0 ? [...(resp.entities?.[n - 1]?.sheetIds ?? [])] : [];
    }
  }
  return [];
}

// errorResult wraps a failure message (a parse error, or an unloadable design) as panel state, with
// the caret line for a parse error when there is one.
export function errorResult(message: string, errorAt = ""): QueryResult {
  const r: QueryResult = { columns: [], columnKinds: [], rows: [], error: message, loading: false, ran: true, total: 0 };
  if (errorAt) r.errorAt = errorAt;
  return r;
}

// reasonMessage renders a locate reason (WS9-039) as the explanation shown when clicking a cell
// highlighted nothing. UNSPECIFIED is the fallback for a normal entity that still did not resolve on
// the current view (an unplaced part, or one that lives on a layout/sheet not shown). The panel owns
// this copy, mirroring how it owns the relation-picker labels.
export function reasonMessage(reason: LocateReason, kind: string, subject: string): string {
  switch (reason) {
    case LocateReason.VIRTUAL_SYMBOL:
      return `${subject} is a power-port/flag symbol (a virtual “#” part), not a placed component — there's nothing to highlight.`;
    case LocateReason.POWER_RAIL_NO_WIRE:
      return `${subject} is a power rail distributed through power-symbol taps, so it has no drawn wire to trace on this schematic view.`;
    case LocateReason.NOT_IN_DESIGN:
      return `${subject} is not in the loaded design.`;
    case LocateReason.BUS_NOT_DRAWN:
      return `${subject} is a bus with no drawn wire on this schematic (a bus alias or a port whose members aren't modeled), so there's nothing to highlight.`;
    case LocateReason.NO_GEOMETRY:
      return `${subject} is in the design but drawn on no sheet, so there's nothing to highlight.`;
    default:
      return `Couldn't locate ${kind || "entity"} ${subject} on this view.`;
  }
}

// compareNatural orders two names with their digit runs read as numbers, so R2 < R10 and U10.3 <
// U10.12, text runs case-insensitively, and the plain strings breaking a tie. It is the engine's
// ident.CompareNatural, the order Tabulate and every csv give a name column, and query.test.ts and
// core/ident read one fixture, core/ident/testdata/natural_twin.json, so the panel's sort cannot
// disagree with them (agni issue 1045).
export function compareNatural(x: string, y: string): number {
  const isDigit = (c: string) => c >= "0" && c <= "9";
  const run = (s: string, digits: boolean) => {
    let i = 0;
    while (i < s.length && isDigit(s[i]) === digits) i++;
    return s.slice(0, i);
  };
  const cmp = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
  let a = x;
  let b = y;
  while (a !== "" && b !== "") {
    const da = isDigit(a[0]);
    const db = isDigit(b[0]);
    if (da && db) {
      const ra = run(a, true);
      const rb = run(b, true);
      const na = ra.replace(/^0+/, "");
      const nb = rb.replace(/^0+/, "");
      if (na.length !== nb.length) return na.length < nb.length ? -1 : 1;
      const c = cmp(na, nb);
      if (c !== 0) return c;
      a = a.slice(ra.length);
      b = b.slice(rb.length);
    } else if (da !== db) {
      return da ? -1 : 1;
    } else {
      const ta = run(a, false);
      const tb = run(b, false);
      const c = cmp(ta.toLowerCase(), tb.toLowerCase());
      if (c !== 0) return c;
      a = a.slice(ta.length);
      b = b.slice(tb.length);
    }
  }
  if (a !== b) return a.length < b.length ? -1 : a.length > b.length ? 1 : 0;
  return cmp(x, y);
}
