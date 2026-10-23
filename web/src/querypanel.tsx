import { For, Show, createEffect, createSignal, untrack } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type EntityQueryItem,
  type ExampleItem,
  type QueryResult,
  type QuerySetItem,
  type QueryView,
  type RelationItem,
  type SearchItem,
  type BindValue,
  type Bindings,
  LocateReason,
  cellKind,
  emptyResult,
  bindingsInUse,
  compareNatural,
  formatBinding,
  numberKey,
  searchBindings,
  tabsFromSet,
  ROOT_PATH,
  relationCrumbs,
  tierLabel,
  type RelationEntryItem,
  relationTemplate,
  STARTING_POINT_LIMIT,
  type WitnessItem,
  type WitnessValue,
} from "./query.js";
import {
  type CheckedState,
  type FindingsState,
  type SeverityTally,
  checkedState,
  findingsFor,
  tallySeverities,
} from "./findings.js";
import { renderMarkdown } from "./markdown.js";
import { type Selection, entityBindings, labelFor, sameSelection, selectionFromCell } from "./selection.js";
import { SheetBadges } from "./sheetbadges.jsx";
import type { QuerySet } from "./gen/agni/v1/webapi/query_pb.js";
import { type Located, type TabStrip, activeTab, askTitle, blankTab, closeTab, fromOrigin, openTab, updateTab } from "./querytabs.js";

// resolveRelationImages rewrites a relation Detail's relative image refs (images/<rel>.svg) to the
// server's /relation-docs/ route BEFORE markdown rendering. renderMarkdown prepends the /rule-docs/
// base only to still-relative refs, so it leaves these alone.
function resolveRelationImages(md: string): string {
  return md.replace(/\]\(images\//g, "](/relation-docs/images/");
}

// emptyFindings is the state before the presenter has pushed anything. It has no rules selected, so
// the count reads "no rules selected" rather than "no findings".
function emptyFindings(): FindingsState {
  return { findings: [], verdicts: [], focusedVerdict: "", selected: "", ruleCount: 0, pending: 0, running: false, catalogLoading: false, skipped: [], ruleSummaries: {}, saveNote: "" };
}

// FindingsCount says what is already CHECKED about a selection, for one entity or for a whole answer
// set (agni issue 259). A zero can mean not run, no rules, partially run, or clean, and a bare 0 reads
// as clean, so the text names the state. See
// docsite/content/architecture/web-picking.md#what-a-count-means.
//
// `scope` names what was counted when position does not ("these 5 entities"). The selection bar
// passes "" because the entity is named immediately to its left.
function FindingsCount(props: {
  tally: SeverityTally;
  state: CheckedState;
  title: string;
  scope: string;
  gated: number;
  onOpen?: () => void;
}) {
  const noun = (n: number): string => (n === 1 ? "1 finding" : `${n} findings`);
  const where = (): string => (props.scope ? ` across ${props.scope}` : "");
  const label = (): string => {
    switch (props.state) {
      case "loading":
        return "loading rules…";
      case "no-rules":
        return "no rules selected";
      case "running":
        return "checking…";
      case "not-run":
        return "not checked yet";
      case "partial":
        return props.tally.total === 0 ? `nothing yet${where()}` : `${noun(props.tally.total)} so far${where()}`;
      default:
        return props.tally.total === 0 ? `no findings${where()}` : `${noun(props.tally.total)}${where()}`;
    }
  };
  const counted = (): boolean => props.state === "partial" || props.state === "complete";
  const body = () => (
    <>
      <span class="query-findings-label">{label()}</span>
      <Show when={counted() && props.tally.total > 0}>
        <span class="query-findings-pips">
          <Show when={props.tally.error > 0}>
            <span class="query-pip error">{props.tally.error}</span>
          </Show>
          <Show when={props.tally.warning > 0}>
            <span class="query-pip warning">{props.tally.warning}</span>
          </Show>
          <Show when={props.tally.info > 0}>
            <span class="query-pip info">{props.tally.info}</span>
          </Show>
        </span>
      </Show>
      {/* Neither a pass nor a fail, so it is worded as an open question rather than counted with the
          defects. The reader can often clear one by supplying whatever the rule named. */}
      <Show when={counted() && props.tally.inconclusive > 0}>
        <span class="query-findings-open-q">{props.tally.inconclusive} unresolved</span>
      </Show>
    </>
  );
  return (
    <span class="query-findings" title={props.title}>
      <Show when={props.onOpen && counted() && props.tally.total > 0} fallback={body()}>
        <button type="button" class="query-findings-open" onClick={() => props.onOpen?.()}>
          {body()}
        </button>
      </Show>
      {/* Visible text rather than only a hover, so the reader sees what the count does not cover. */}
      <span class="query-findings-caveat">
        selected rules, this subject only
        {/* A rule gated before it evaluated reports nothing ANYWHERE, so a clean entity under a
            half-gated ruleset means much less than it looks. The gating is design-wide, so it is
            stated here rather than counted above. */}
        <Show when={props.gated > 0}>
          <span class="query-findings-gated"> · {props.gated} rule(s) could not run</span>
        </Show>
      </span>
    </span>
  );
}

// QueryPanel is the ad-hoc datalog query panel (WS9-036 / WS3-029). It hosts any number of query
// tabs (agni issue 902), each with its own query, bindings and answer, and emits onRun naming the tab
// that asked; the presenter evaluates the query against the open design and pushes the QueryResult
// back for that tab, reporting "no file open" as the result's error. Provenance expands per row so
// many-citation rows do not widen the table. The relation catalog (WS9-037) renders as click-to-insert
// chips grouped by kind.
// WitnessTree draws how a row holds (agni issue 794): each fact on a line, the rule that derived a
// derived fact under it, a leaf's citations, and each body fact one level in, in the rule's written
// order. A value naming an entity is a locate button, as a cell naming it is. A negated leaf is a
// `not` that held, an absence, and says so.
function WitnessTree(props: {
  items: WitnessItem[];
  pick: (v: WitnessValue, sheet?: string) => void;
  isCurrent: (v: WitnessValue) => boolean;
  isCurrentSheet: (v: WitnessValue, sheet: string) => boolean;
}) {
  return (
    <ul class="query-witnesses">
      <For each={props.items}>
        {(w) => (
          <li class="query-witness" classList={{ negated: w.negated }}>
            <span class="query-witness-fact">
              <Show when={w.negated}>
                <span class="query-witness-not">not </span>
              </Show>
              <span class="query-witness-rel">{w.relation}</span>(
              <For each={w.values}>
                {(v, vi) => (
                  <>
                    {vi() > 0 ? ", " : ""}
                    {v.kind === "" || v.text === "" ? (
                      <span class="query-witness-value">{v.text === "" ? "_" : v.text}</span>
                    ) : (
                      <span class="query-witness-value">
                        <button
                          type="button"
                          class={`query-locate${props.isCurrent(v) ? " on" : ""}`}
                          title={v.kind === "pin" ? `locate pin ${v.ref}.${v.text}` : `locate ${v.kind} ${v.text}`}
                          onClick={() => props.pick(v)}
                        >
                          {v.text}
                        </button>
                        <SheetBadges
                          items={v.sheets}
                          label={(b) => b.name}
                          title={(b) => `show sheet ${b.name}`}
                          onSelect={(b) => props.pick(v, b.id)}
                          active={(b) => props.isCurrentSheet(v, b.id)}
                        />
                      </span>
                    )}
                  </>
                )}
              </For>
              )
              <Show when={w.negated}>
                <span class="query-witness-absent" title="a `not` that held: the design has no such fact">absent</span>
              </Show>
            </span>
            <Show when={w.rule !== ""}>
              <code class="query-witness-rule" title="the rule that derived this fact">{w.rule}</code>
            </Show>
            <Show when={w.cites.length > 0}>
              <span class="query-witness-cites">{w.cites.join(" ; ")}</span>
            </Show>
            <Show when={w.children.length > 0}>
              <WitnessTree items={w.children} pick={props.pick} isCurrent={props.isCurrent} isCurrentSheet={props.isCurrentSheet} />
            </Show>
          </li>
        )}
      </For>
    </ul>
  );
}

function QueryPanel(props: {
  results: () => Record<string, QueryResult>;
  strip: () => TabStrip;
  setStrip: (s: TabStrip) => void;
  newTabId: () => string;
  relations: () => RelationItem[];
  // describe asks for one place in the relation namespace, which the picker drills through (agni
  // issue 774). Absent, the picker shows only the syntax hint.
  describe?: (path: string) => Promise<RelationEntryItem>;
  examples: () => ExampleItem[];
  entityQueries: () => EntityQueryItem[];
  search: () => SearchItem | null;
  locateNote: () => string;
  setLocateNote: (note: string) => void;
  prefill: () => { text: string; bindings: Bindings; n: number; title?: string; whole?: boolean };
  selection: () => Selection | null;
  setSelection: (sel: Selection | null) => void;
  currentSheet: () => string;
  findings: () => FindingsState;
  onRun: (text: string, bindings: Bindings, limit: number, tab: string) => void;
  // onExplain asks the tab's last question again with witnesses (agni issue 794), when a row is first
  // opened. Absent, an opened row shows its citations alone.
  onExplain?: (tab: string) => void;
  onSaveCSV?: (tab: string) => void;
  onInspect: (sel: Selection) => void;
  onLocate: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, pin?: string) => void;
  onActivate: (tab: string, located: Located | undefined, previous: Located | undefined) => void;
  onClose: (tab: string) => void;
  // querySets are the sets the design's project keeps, offered in the Sets menu.
  querySets: () => QuerySetItem[];
  // onRunSet answers a set opened as tabs, one tab id per query in the set's order.
  onRunSet: (set: QuerySet, tabs: string[]) => void;
  // onSetFile hands over a set file the reader picked, for the host to parse and open.
  onSetFile?: (name: string, text: string) => void;
  registerOpenSet?: (open: (set: QuerySet) => void) => void;
  // registerAsk hands the island the panel's askAbout, so a question asked from elsewhere (the
  // Properties panel's Ask rows) opens its tab exactly as the panel's own ask button does.
  registerAsk?: (ask: (sel: Selection, question?: EntityQueryItem) => void) => void;
}) {
  // The tab in front, whose question the editor shows and whose answer the table renders.
  const front = () => activeTab(props.strip());
  const state = (): QueryResult => props.results()[front().id] ?? emptyResult();
  const text = () => front().text;
  const setText = (t: string) => props.setStrip(updateTab(props.strip(), front().id, { text: t }));
  // bindings are the values the query's variables are bound to (agni issue 793), set by a search or a
  // click and shown as chips under the box, so the box keeps the query as the server wrote it and
  // the values stay visible and editable beside it.
  const bindings = () => front().bindings;
  // isSetNumber reports whether a chip still holds the number its set bound, which spells bare. For a
  // list it answers per value, and only while the list is still the one the set bound.
  const isSetNumber = (name: string, value: BindValue): boolean | boolean[] => {
    const o = front().origin;
    const bound = o?.bindings[name];
    if (Array.isArray(value)) {
      const same = !!o && Array.isArray(bound) && bound.length === value.length && bound.every((b, i) => b === value[i]);
      return value.map((_, i) => same && o.numbers.includes(numberKey(name, i)));
    }
    return !!o && o.numbers.includes(name) && bound === value;
  };
  const setBindings = (b: Bindings) => props.setStrip(updateTab(props.strip(), front().id, { bindings: b }));
  // ask runs a query in the tab in front and remembers it there. A question about one picked entity
  // passes STARTING_POINT_LIMIT (agni issue 896); every other run asks for the whole answer (0).
  const ask = (q: string, b: Bindings, limit = 0) => {
    const id = front().id;
    props.setStrip(updateTab(props.strip(), id, { lastRun: { text: q, bindings: b } }));
    send(q, b, limit, id);
  };
  // send runs a question in tab id. A tab still asking the set member it was opened from runs that
  // member through the set, as the engine composes it; any other question runs as written.
  const send = (q: string, b: Bindings, limit: number, id: string) => {
    const o = props.strip().tabs.find((t) => t.id === id)?.origin;
    if (fromOrigin(o, q, b)) props.onRunSet({ ...o.set, queries: [o.set.queries[o.index]] }, [id]);
    else props.onRun(q, b, limit, id);
  };
  const showAll = () => {
    if (state().loading) return;
    resetForRun();
    const last = front().lastRun;
    send(last.text, last.bindings, 0, front().id);
  };
  // openQuestion opens a tab holding a question and runs it there. The new tab keeps the drawing's
  // highlight as it is, since the question is about what is already selected.
  const openQuestion = (title: string, q: string, b: Bindings, limit = 0) => {
    const id = props.newTabId();
    props.setStrip(openTab(props.strip(), { ...blankTab(id, title), text: q, bindings: b, located: front().located }));
    resetForRun();
    setMode("query");
    ask(q, b, limit);
  };
  // blankTitle numbers the tabs "+" opens, Query 1, Query 2, so each reads as its own.
  let blanks = 1;
  const newBlank = () => {
    const previous = front().located;
    const id = props.newTabId();
    props.setStrip(openTab(props.strip(), blankTab(id, `Query ${++blanks}`)));
    resetForRun();
    setMode("query");
    props.setLocateNote("");
    props.onActivate(id, undefined, previous);
    queueMicrotask(() => taRef?.focus());
  };
  // openSet opens one tab per query of a set, after the tab in front and in the set's order, brings
  // the first forward, and asks the host to answer all of them over one read.
  const openSet = (set: QuerySet) => {
    const tabs = tabsFromSet(set);
    if (tabs.length === 0) return;
    const previous = front().located;
    const ids: string[] = [];
    let strip = props.strip();
    tabs.forEach((t, index) => {
      const id = props.newTabId();
      ids.push(id);
      strip = openTab(strip, {
        ...blankTab(id, t.title),
        text: t.text,
        bindings: t.bindings,
        lastRun: { text: t.text, bindings: t.bindings },
        origin: { set, index, text: t.text, bindings: t.bindings, numbers: t.numbers },
      });
    });
    props.setStrip({ ...strip, active: ids[0] });
    resetForRun();
    setMode("query");
    setSetsOpen(false);
    props.setLocateNote("");
    props.onActivate(ids[0], undefined, previous);
    props.onRunSet(set, ids);
  };
  props.registerOpenSet?.(openSet);
  // setsOpen shows the Sets menu: the project's sets, and a set file from disk.
  const [setsOpen, setSetsOpen] = createSignal(false);
  let fileRef: HTMLInputElement | undefined;
  const pickFile = async (f: File | undefined) => {
    setSetsOpen(false);
    if (!f || !props.onSetFile) return;
    props.onSetFile(f.name, await f.text());
    if (fileRef) fileRef.value = "";
  };
  // activate brings a tab forward. The host then shows what that tab last located, so the canvas
  // matches the answer in front.
  const activate = (id: string) => {
    const previous = front();
    if (previous.id === id) return;
    props.setStrip({ ...props.strip(), active: id });
    resetForRun();
    props.setLocateNote("");
    props.onActivate(id, front().located, previous.located);
  };
  const close = (id: string) => {
    const before = props.strip();
    const previous = activeTab(before).located;
    const after = closeTab(before, id);
    if (after === before) return;
    props.setStrip(after);
    props.onClose(id);
    if (before.active === id) {
      resetForRun();
      props.setLocateNote("");
      props.onActivate(after.active, activeTab(after).located, previous);
    }
  };
  // inUse is the bindings the current text still names, which are the ones a run sends and the
  // chips show.
  const inUse = () => bindingsInUse(text(), bindings());
  // expanded holds the row indices whose provenance is open. It is view state, so it lives here
  // rather than in the pushed result.
  const [expanded, setExpanded] = createSignal<Set<number>>(new Set());
  // The relation picker drills the namespace (agni issue 774). relPath is the place on screen, and
  // entries keeps each place ListRelations has described, so walking back up asks nothing again.
  const [relPath, setRelPath] = createSignal(ROOT_PATH);
  const [entries, setEntries] = createSignal<Record<string, RelationEntryItem>>({});
  const [relError, setRelError] = createSignal("");
  const openRel = (path: string) => {
    setRelPath(path);
    setRelError("");
    if (entries()[path] || !props.describe) return;
    props.describe(path).then(
      (e) => setEntries({ ...entries(), [path]: e }),
      (err: unknown) => setRelError(err instanceof Error ? err.message : String(err)),
    );
  };
  // The root is described once the catalog arrives, so the picker opens on it.
  createEffect(() => {
    if (props.relations().length > 0) untrack(() => !entries()[ROOT_PATH] && openRel(ROOT_PATH));
  });
  const relEntry = () => entries()[relPath()];
  // widths holds a per-column pixel width once the reader has dragged that column's edge. Untouched
  // columns share the table equally under .query-table's fixed layout. Auto layout sizes each column
  // to its widest cell, so one long provenance string or net name squeezes the rest to slivers.
  const [widths, setWidths] = createSignal<Record<number, number>>({});
  const MIN_COL_PX = 48;

  // startResize drags one column edge. Pointer capture rather than window listeners keeps the drag
  // alive when the cursor leaves the header, and stopPropagation keeps a drag from also re-sorting
  // the table through the header's click handler.
  const startResize = (e: PointerEvent, col: number): void => {
    e.preventDefault();
    e.stopPropagation();
    const handle = e.currentTarget as HTMLElement;
    const th = handle.closest("th");
    const startX = e.clientX;
    const startW = th?.getBoundingClientRect().width ?? MIN_COL_PX;
    handle.setPointerCapture(e.pointerId);
    const move = (ev: PointerEvent): void => {
      setWidths({ ...widths(), [col]: Math.max(MIN_COL_PX, Math.round(startW + ev.clientX - startX)) });
    };
    const up = (): void => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", up);
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", up);
  };

  // A query written for the reader opens its own tab AND runs, and leaves an editable query behind so
  // it teaches the language.
  // Only the prefill is tracked, since opening a tab reads and writes the strip.
  createEffect(() => {
    const p = props.prefill();
    if (!p.n) return;
    untrack(() => openQuestion(p.title || `Query ${++blanks}`, p.text, p.bindings, p.whole ? 0 : STARTING_POINT_LIMIT));
  });

  // drawerOpen controls the slide-in helper drawer (examples, relations, reference), which keeps
  // that chrome out of the panel's vertical space. A left-edge handle opens it; a pointer-down in
  // the input or any run closes it so the results are unobscured.
  const [drawerOpen, setDrawerOpen] = createSignal(false);
  // mode is "query" (say where to look, get back what is there) or "search" (say what a thing is
  // called, get back where it is). Both leave editable datalog in the box, which is why search is a
  // mode of this panel rather than a separate widget.
  const [mode, setMode] = createSignal<"query" | "search">("query");
  const [term, setTerm] = createSignal("");
  // sortCol/sortDir are the results table's client-side sort. sortCol indexes state().columns, with
  // -1 meaning natural row order. Rows carry their ORIGINAL index, which keys the provenance
  // expand-state, so a re-sort keeps it.
  const [sortCol, setSortCol] = createSignal(-1);
  const [sortDir, setSortDir] = createSignal<"asc" | "desc">("asc");
  let taRef: HTMLTextAreaElement | undefined;
  // Opening a row on an answer asked without witnesses asks again with them, once, so a query costs
  // what it always did until a reader wants to know how a row holds (agni issue 794).
  const toggleCites = (i: number) => {
    const next = new Set<number>(expanded());
    if (next.has(i)) next.delete(i);
    else next.add(i);
    setExpanded(next);
    if (next.has(i) && !state().witnessed && props.onExplain) props.onExplain(front().id);
  };
  // resetForRun clears the per-result view state (expanded provenance, sort) and closes the drawer.
  const resetForRun = () => {
    setExpanded(new Set<number>());
    setSortCol(-1);
    setDrawerOpen(false);
  };
  const run = () => {
    const q = text().trim();
    if (q === "" || state().loading) return;
    resetForRun();
    ask(q, bindingsInUse(q, bindings()));
  };
  // runExample fills the textarea with a starter query AND runs it (WS14-002), leaving the query in
  // the box to edit.
  const runExample = (e: ExampleItem) => {
    props.setStrip(updateTab(props.strip(), front().id, { text: e.query, bindings: {} }));
    if (state().loading) return;
    resetForRun();
    ask(e.query, {});
  };
  // doSearch fills the box with the datalog that answers the reader's name, binds the pattern, runs
  // it, and switches the panel BACK to query mode, so the reader sees the query and its binding and
  // can edit either.
  const doSearch = () => {
    const s = props.search();
    const t = term().trim();
    if (!s || t === "" || state().loading) return;
    const b = searchBindings(s, t);
    props.setStrip(updateTab(props.strip(), front().id, { text: s.query, bindings: b }));
    setMode("query");
    resetForRun();
    ask(s.query, b);
  };
  // presetFor is the first served question for a selection's kind, the one the selection bar offers,
  // or undefined before the catalog has arrived (or for a kind the server writes none for).
  const presetFor = (kind: string): EntityQueryItem | undefined => props.entityQueries().find((p) => p.kind === kind);
  // askAbout opens a tab asking one served question about what is selected, with the selection's
  // values bound, leaving the same editable query behind. The question defaults to the kind's first.
  const askAbout = (sel: Selection, question?: EntityQueryItem) => {
    const q = question ?? presetFor(sel.kind);
    if (!q) return;
    openQuestion(askTitle(sel, q.label), q.query, entityBindings(q.binds, sel), STARTING_POINT_LIMIT);
  };
  props.registerAsk?.(askAbout);
  // pickCell is a click on a result cell or one of its sheet badges. It locates the entity AND
  // selects it, so one answer becomes the subject of the next question. A kind with no selection
  // shape (scalars never reach here) locates and deselects rather than leaving the bar naming the
  // previous pick.
  //
  // `ref` is the row's other half for a pin cell, "" for every other kind. A pin inverts the locate
  // arguments, since the entity is the COMPONENT and the cell is its designator, so the call is
  // (pin, U7, ..., "5") where a component cell sends (component, U7).
  //
  // The tab remembers what it located, so coming back to it shows the same thing on the drawing.
  const pickCell = (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, ref = "") => {
    const sel = selectionFromCell(kind, subject, ref);
    props.setSelection(sel);
    if (kind === "pin") {
      if (!ref) return;
      props.setStrip(updateTab(props.strip(), front().id, { located: { kind, subject: ref, sheet, reason, pin: subject } }));
      props.onLocate(kind, ref, sheet, reason, subject);
      return;
    }
    props.setStrip(updateTab(props.strip(), front().id, { located: { kind, subject, sheet, reason } }));
    props.onLocate(kind, subject, sheet, reason);
  };
  // isCurrent reports whether a cell names the entity on screen now, so the table marks which answer
  // the canvas is showing. It is DERIVED from the selection rather than remembered from the click,
  // because a remembered mark goes silently wrong once the reader picks on the drawing or opens a
  // finding. An entity appearing in several rows marks all of them.
  const isCurrent = (kind: string, cell: string): boolean =>
    sameSelection(selectionFromCell(kind, cell), props.selection());
  // A badge is current when its sheet is the one rendered AND its cell is the entity being shown.
  // Without the second half every row's badge for this sheet would light up.
  const isCurrentSheet = (kind: string, cell: string, sheet: string): boolean =>
    isCurrent(kind, cell) && sheet !== "" && sheet === props.currentSheet();
  // What is CHECKED about the selection and about the whole answer set (agni issue 259). Both run the
  // findings the panel was handed through findingsFor, which takes a set, so a click and a result
  // table share one path.
  //
  // resultSubjects is every locatable cell in the table, deduped by identity. It goes through
  // selectionFromCell (and so cellKind), so a polymorphic search result contributes each row's own
  // kind rather than a column's.
  const resultSubjects = (): Selection[] => {
    const out: Selection[] = [];
    const st = state();
    for (const row of st.rows) {
      for (let i = 0; i < row.cells.length; i++) {
        const sel = selectionFromCell(cellKind(st, row, i), row.cells[i]);
        if (sel && !out.some((o) => sameSelection(o, sel))) out.push(sel);
      }
    }
    return out;
  };
  const selectionTally = () => tallySeverities(findingsFor(props.findings().findings, [props.selection()]));
  const resultTally = () => tallySeverities(findingsFor(props.findings().findings, resultSubjects()));
  // countTitle is the hover text saying WHY the count reads the way it does. An entity view projects
  // attention where a review pass enumerates the design, so a zero here means much less than a zero
  // there.
  const countTitle = (): string => {
    const st = props.findings();
    const state = checkedState(st);
    const head =
      state === "loading"
        ? "The rule catalog has not loaded yet, so this is not a count of anything."
        : state === "not-run"
        ? "No rule has run yet, so this is not a count of anything."
        : state === "partial"
          ? `${st.pending} selected rule(s) have not run, so this count is a floor.`
          : state === "no-rules"
            ? "No rules are selected."
            : "";
    // The skipped list is DESIGN-WIDE, and a gated rule produces no findings anywhere. The reasons
    // are the engine's own words, passed through unreworded as the checks panel does, because the
    // rule decides why it cannot run.
    const gated = st.skipped.length
      ? `\n\n${st.skipped.length} rule(s) could not run on this design at all, so they report nothing anywhere:\n` +
        st.skipped.map((s) => `  ${s.rule}: ${s.reason}`).join("\n")
      : "";
    return (
      `${head ? head + "\n\n" : ""}Findings from the SELECTED rules that name this subject. Not a coverage statement: ` +
      `a design-global rule has no subject and can never appear here, and a rule that checks two terminals names one of them, ` +
      `so the other end shows nothing.${gated}`
    );
  };
  // cmpCells sorts a column the answer types as an entity kind (a part, a net) in the engine's name
  // order, so R2 < R10 as every csv has it (compareNatural, agni issue 1045), and any other column's
  // two numeric cells by value (so 9 < 10) and other pairs lexicographically. displayRows applies it,
  // carrying each row's original index.
  const cmpCells = (a: string, b: string, named = false): number => {
    if (named) return compareNatural(a, b);
    const na = Number(a);
    const nb = Number(b);
    const aNum = a.trim() !== "" && Number.isFinite(na);
    const bNum = b.trim() !== "" && Number.isFinite(nb);
    if (aNum && bNum) return na - nb;
    return a.localeCompare(b);
  };
  const displayRows = (): { row: QueryResult["rows"][number]; idx: number }[] => {
    const indexed = state().rows.map((row, idx) => ({ row, idx }));
    const col = sortCol();
    if (col < 0) return indexed;
    const dir = sortDir() === "asc" ? 1 : -1;
    const named = (state().columnKinds[col] ?? "") !== "";
    return [...indexed].sort((x, y) => dir * cmpCells(x.row.cells[col] ?? "", y.row.cells[col] ?? "", named));
  };
  // sortBy cycles a column through ascending, descending, and back to the server's natural order.
  const sortBy = (ci: number) => {
    if (sortCol() !== ci) {
      setSortCol(ci);
      setSortDir("asc");
    } else if (sortDir() === "asc") {
      setSortDir("desc");
    } else {
      setSortCol(-1);
    }
  };
  // insertRelation splices a relation template into the query at the caret, replacing any selection,
  // so a chip click extends the query rather than clobbering it. It restores focus with the caret
  // just after the snippet.
  const insertRelation = (r: { name: string; args: readonly string[] }) => {
    const snip = relationTemplate(r);
    const ta = taRef;
    const cur = text();
    const start = ta ? ta.selectionStart : cur.length;
    const end = ta ? ta.selectionEnd : cur.length;
    const next = cur.slice(0, start) + snip + cur.slice(end);
    setText(next);
    if (ta) {
      const caret = start + snip.length;
      queueMicrotask(() => {
        ta.focus();
        ta.setSelectionRange(caret, caret);
      });
    }
  };

  return (
    <div class="query">
      {/* The panel's own tab strip, one tab per question. A tab's close button hides while it is the
          last one, so the panel always shows one. */}
      <div class="query-tabbar">
        <div class="query-tabs" role="tablist" aria-label="Queries">
          <For each={props.strip().tabs}>
            {(t) => (
              <span class="query-tab" classList={{ on: t.id === props.strip().active }}>
                <button
                  type="button"
                  role="tab"
                  class="query-tab-go"
                  aria-selected={t.id === props.strip().active}
                  title={t.title}
                  onClick={() => activate(t.id)}
                >
                  {t.title}
                  <Show when={props.results()[t.id]?.loading}>
                    <span class="query-tab-busy" aria-label="running">…</span>
                  </Show>
                </button>
                <Show when={props.strip().tabs.length > 1}>
                  <button type="button" class="query-tab-close" aria-label={`Close ${t.title}`} title="Close" onClick={() => close(t.id)}>
                    ×
                  </button>
                </Show>
              </span>
            )}
          </For>
          <button type="button" class="query-tab-new" aria-label="New query" title="New query" onClick={newBlank}>
            +
          </button>
        </div>
        {/* Opening a query set (agni issue 745) opens one tab per query. The menu lists the sets the
            design's project keeps in queries/, and takes a set file from disk. */}
        <Show when={props.querySets().length > 0 || props.onSetFile}>
          <span class="query-sets">
            <button
              type="button"
              class="query-sets-open"
              aria-haspopup="menu"
              aria-expanded={setsOpen()}
              title="Open a query set as one tab per query"
              onClick={() => setSetsOpen(!setsOpen())}
            >
              Sets ▾
            </button>
            <Show when={setsOpen()}>
              <div class="query-sets-menu" role="menu">
                <For each={props.querySets()}>
                  {(s) => (
                    <button type="button" role="menuitem" class="query-set-item" title={s.source} onClick={() => openSet(s.set)}>
                      {s.set.title || s.name}
                      <span class="query-set-count"> ({s.set.queries.length})</span>
                    </button>
                  )}
                </For>
                <Show when={props.querySets().length === 0}>
                  <div class="query-sets-none">This design's project keeps no query sets.</div>
                </Show>
                <Show when={props.onSetFile}>
                  <button type="button" role="menuitem" class="query-set-file" onClick={() => fileRef?.click()}>
                    From a file…
                  </button>
                </Show>
              </div>
            </Show>
            <input
              ref={fileRef}
              type="file"
              accept=".yaml,.yml"
              class="query-set-input"
              hidden
              onChange={(e) => void pickFile(e.currentTarget.files?.[0])}
            />
          </span>
        </Show>
      </div>

      {/* The mode tabs appear only once the server has sent a search template, since without one a
          search mode could only guess at a query. */}
      <Show when={props.search()}>
        <div class="query-modes" role="tablist" aria-label="Query or search">
          <button
            type="button"
            role="tab"
            class={`query-mode${mode() === "query" ? " on" : ""}`}
            aria-selected={mode() === "query"}
            onClick={() => setMode("query")}
          >
            Query
          </button>
          <button
            type="button"
            role="tab"
            class={`query-mode${mode() === "search" ? " on" : ""}`}
            aria-selected={mode() === "search"}
            onClick={() => setMode("search")}
          >
            Find by name
          </button>
        </div>
      </Show>

      <Show when={mode() === "search" && props.search()}>
        {(tmpl) => (
          <div class="query-input query-search">
            <input
              type="text"
              class="query-term"
              spellcheck={false}
              placeholder="part of a name: CAN, U1, 3V3"
              aria-label="Find by name"
              value={term()}
              onInput={(e) => setTerm(e.currentTarget.value)}
              onPointerDown={() => setDrawerOpen(false)}
              onKeyDown={(e) => {
                if (e.key === "Enter") doSearch();
              }}
            />
            <div class="query-actions">
              <button
                type="button"
                class="query-run"
                disabled={state().loading || term().trim() === ""}
                title={tmpl().teaches}
                onClick={doSearch}
              >
                {state().loading ? "Searching…" : "Find"}
              </button>
              <span class="query-hint">names anything the design declares, connected or not</span>
            </div>
          </div>
        )}
      </Show>

      <div class="query-input" classList={{ hidden: mode() === "search" }}>
        <textarea
          ref={taRef}
          class="query-text"
          rows="3"
          spellcheck={false}
          placeholder={'component.net(?r,?n), net.max_voltage(?n,?v), ?v < 30 => ?r, ?n'}
          value={text()}
          onInput={(e) => setText(e.currentTarget.value)}
          onPointerDown={() => setDrawerOpen(false)}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") run();
          }}
        />
        {/* The values a search or a click bound, one chip per variable the query still names. Each
            value can be edited, and removing one leaves its variable free, which widens the
            question. A list, which arrives only from a link or a set, shows as the query would
            spell it and can be removed but not edited, since one text box has no spelling for
            several values. */}
        <Show when={Object.keys(inUse()).length > 0}>
          <div class="query-bindings" aria-label="bound values">
            <For each={Object.entries(inUse())}>
              {([name, value]) => (
                <span class="query-binding" title={formatBinding(name, value, isSetNumber(name, value))}>
                  <Show
                    when={typeof value === "string"}
                    fallback={<span class="query-binding-list">{formatBinding(name, value, isSetNumber(name, value))}</span>}
                  >
                    <>
                      <span class="query-binding-name">?{name} =</span>
                      <input
                        class="query-binding-value"
                        aria-label={`value of ?${name}`}
                        size={Math.max(4, value.length)}
                        value={value as string}
                        onInput={(e) => setBindings({ ...bindings(), [name]: e.currentTarget.value })}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") run();
                        }}
                      />
                    </>
                  </Show>
                  <button
                    type="button"
                    class="query-binding-remove"
                    aria-label={`unbind ?${name}`}
                    onClick={() => {
                      const next = { ...bindings() };
                      delete next[name];
                      setBindings(next);
                    }}
                  >
                    ×
                  </button>
                </span>
              )}
            </For>
          </div>
        </Show>
        <div class="query-actions">
          <button type="button" class="query-run" disabled={state().loading || text().trim() === ""} onClick={run}>
            {state().loading ? "Running…" : "Run"}
          </button>
          <span class="query-hint">⌘/Ctrl+Enter</span>
        </div>
      </div>

      {/* The selection bar names what the reader last picked, on the drawing or in the results, and
          offers the one question there is a served preset for. */}
      <Show when={props.selection()}>
        {(sel) => (
          <div class="query-selection">
            <span class="query-selection-what">
              <span class="query-selection-kind">{sel().kind}</span>
              <span class="query-selection-name">{labelFor(sel())}</span>
              <FindingsCount
                tally={selectionTally()}
                state={checkedState(props.findings())}
                title={countTitle()}
                scope=""
                gated={props.findings().skipped.length}
                onOpen={() => props.onInspect(sel())}
              />
            </span>
            <Show when={presetFor(sel().kind)}>
              {(preset) => (
                <button
                  type="button"
                  class="query-ask"
                  title={`${preset().query}\n${Object.entries(entityBindings(preset().binds, sel()))
                    .map(([k, v]) => formatBinding(k, v))
                    .join("\n")}\n\n${preset().teaches}`}
                  onClick={() => askAbout(sel())}
                >
                  {preset().label}
                </button>
              )}
            </Show>
          </div>
        )}
      </Show>

      {/* Left-edge handle opens the helper drawer; it hides while the drawer is open. */}
      <button
        type="button"
        class={`query-drawer-handle${drawerOpen() ? " hidden" : ""}`}
        title="Examples & relations"
        aria-label="Open examples and relations"
        onClick={() => setDrawerOpen(true)}
      >
        <span class="query-drawer-handle-label">Examples &amp; relations</span>
      </button>

      {/* The drawer stays mounted, translated off-screen when closed, so chip clicks and the
          reference pane work while it is closed. The tests drive them without opening it. */}
      <div class={`query-drawer${drawerOpen() ? " open" : ""}`} role="dialog" aria-label="Examples and relations">
        <div class="query-drawer-head">
          <span class="query-drawer-title">Examples &amp; relations</span>
          <button type="button" class="query-drawer-close" aria-label="Close" onClick={() => setDrawerOpen(false)}>
            ✕
          </button>
        </div>
        <div class="query-drawer-body">
          <Show when={props.examples().length > 0}>
            <div class="query-examples">
              <div class="query-examples-hint">Try one — click to run, then edit:</div>
              <div class="query-examples-chips">
                <For each={props.examples()}>
                  {(e) => (
                    <button
                      type="button"
                      class="query-example"
                      title={`${e.query}\n\n${e.teaches}`}
                      onClick={() => runExample(e)}
                    >
                      {e.label}
                    </button>
                  )}
                </For>
              </div>
            </div>
          </Show>

          <Show
            when={props.describe && props.relations().length > 0}
            fallback={
              <p class="query-relhint">Join on shared ?variables; =&gt; projects.</p>
            }
          >
            <div class="query-relations">
              <nav class="query-relcrumbs" aria-label="Relation namespace">
                <For each={relationCrumbs(relPath())}>
                  {(c, i) => (
                    <>
                      <Show when={i() > 0}>
                        <span class="query-relcrumb-sep">›</span>
                      </Show>
                      <button type="button" class="query-relcrumb" disabled={c.path === relPath()} onClick={() => openRel(c.path)}>
                        {c.label}
                      </button>
                    </>
                  )}
                </For>
              </nav>
              <Show when={relError() !== ""}>
                <div class="query-relerror">{relError()}</div>
              </Show>
              <Show when={relEntry()} fallback={<Show when={relError() === ""}><div class="query-relhint">Loading…</div></Show>}>
                <Show
                  when={relEntry()!.kind === "module"}
                  fallback={
                    <div class="query-relmember">
                      <code class="query-relsignature">{relEntry()!.signature}</code>
                      <Show when={tierLabel(relEntry()!.path, props.relations()) !== ""}>
                        <span class="query-reltier">{tierLabel(relEntry()!.path, props.relations())}</span>
                      </Show>
                      <p class="query-reldoc">{relEntry()!.doc}</p>
                      <Show when={relEntry()!.inferred.length > 0}>
                        <p class="query-relinferred">Types inferred from its rules: {relEntry()!.inferred.join(", ")}</p>
                      </Show>
                      <button
                        type="button"
                        class="query-relinsert"
                        onClick={() => insertRelation({ name: relEntry()!.path, args: relEntry()!.args })}
                      >
                        Insert {relationTemplate({ name: relEntry()!.path, args: relEntry()!.args })}
                      </button>
                      <Show when={relEntry()!.definition.length > 0}>
                        <pre class="query-reldefinition">{relEntry()!.definition.join("\n")}</pre>
                      </Show>
                      <Show when={relEntry()!.detail !== ""}>
                        <div
                          class="query-reldetail-body md"
                          innerHTML={renderMarkdown(resolveRelationImages(relEntry()!.detail))}
                        />
                      </Show>
                    </div>
                  }
                >
                  <div class="query-relations-hint">Choose a module to list its members, or a relation to see it and insert it. Join on shared ?variables; =&gt; projects.</div>
                  <ul class="query-relmembers">
                    <For each={relEntry()!.members}>
                      {(m) => (
                        <li class="query-relrow">
                          <button
                            type="button"
                            class="query-relchip"
                            data-kind={m.kind}
                            title={m.kind === "module" ? `${m.path}: a module` : `${m.signature} — ${m.doc}`}
                            onClick={() => openRel(m.path)}
                          >
                            {m.kind === "module" ? `${m.path} ›` : m.path}
                          </button>
                          <Show when={tierLabel(m.path, props.relations()) !== ""}>
                            <span class="query-reltier">{tierLabel(m.path, props.relations())}</span>
                          </Show>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </Show>
            </div>
          </Show>
        </div>
      </div>

      <Show when={state().error !== ""}>
        <div class="query-error">
          {state().error}
          <Show when={state().errorAt}>
            <pre class="query-error-at">
              {state().errorAt!.slice(0, state().errorAt!.lastIndexOf("\n") + 1)}
              <span class="query-error-caret">{state().errorAt!.slice(state().errorAt!.lastIndexOf("\n") + 1)}</span>
            </pre>
          </Show>
        </div>
      </Show>

      <Show when={props.locateNote() !== ""}>
        <div class="query-locate-note" role="status">{props.locateNote()}</div>
      </Show>

      <Show when={state().error === "" && state().ran}>
        <Show
          when={state().rows.length > 0}
          fallback={<div class="query-empty">No results.</div>}
        >
          <div class="query-results">
            <table class="query-table">
              {/* A colgroup carries the widths, so a drag sets ONE number rather than restyling every
                  cell in the column, and the fixed layout below honours it. */}
              <colgroup>
                <col class="query-cite-col" />
                <For each={state().columns}>{(_, ci) => <col style={widths()[ci()] ? { width: `${widths()[ci()]}px` } : undefined} />}</For>
              </colgroup>
              <thead>
                <tr>
                  <th class="query-cite-col" />
                  <For each={state().columns}>
                    {(c, ci) => (
                      <th
                        class="query-sortable"
                        title="click to sort; click again to reverse, once more to clear"
                        onClick={() => sortBy(ci())}
                      >
                        <span class="query-th-label">{c}</span>
                        <Show when={sortCol() === ci()}>
                          <span class="query-sort-ind">{sortDir() === "asc" ? " ▲" : " ▼"}</span>
                        </Show>
                        <span
                          class="query-col-grip"
                          title="drag to resize this column"
                          onPointerDown={(e) => startResize(e, ci())}
                          onClick={(e) => e.stopPropagation()}
                        />
                      </th>
                    )}
                  </For>
                </tr>
              </thead>
              <tbody>
                <For each={displayRows()}>
                  {(d) => {
                    const row = d.row;
                    const i = d.idx;
                    return (
                    <>
                      <tr class="query-row">
                        <td class="query-cite-col">
                          <Show when={row.cites.length > 0 || row.witnesses.length > 0 || props.onExplain}>
                            <button
                              type="button"
                              class={`query-cite-toggle${expanded().has(i) ? " open" : ""}`}
                              title="how this row holds: the facts and rules behind it"
                              onClick={() => toggleCites(i)}
                            >
                              {expanded().has(i) ? "▾" : "▸"}
                            </button>
                          </Show>
                        </td>
                        <For each={row.cells}>
                          {(cell, ci) => {
                            // A pin cell whose component did not resolve names nothing, so it
                            // renders as plain text. The server already blanks the kind then, and
                            // this covers a server that does not.
                            const rowRef = row.cellRefs[ci()] ?? "";
                            const rawKind = cellKind(state(), row, ci());
                            const kind = rawKind === "pin" && rowRef === "" ? "" : rawKind;
                            const reason = row.cellReasons[ci()] ?? LocateReason.UNSPECIFIED;
                            // Only an entity cell is locatable; a scalar (a voltage, an mpn) stays
                            // plain text. That can vary ROW BY ROW under a polymorphic column, so the
                            // kind comes from cellKind rather than the column (agni issue 338). The
                            // reason (WS9-039) rides along so the presenter can explain a click that
                            // paints nothing (a power rail, a virtual symbol).
                            return kind === "" ? (
                              <td>{cell}</td>
                            ) : (
                              <td class="query-cell-locate">
                                <button
                                  type="button"
                                  class={`query-locate${isCurrent(kind, cell) ? " on" : ""}`}
                                  title={kind === "pin" ? `locate pin ${rowRef}.${cell}` : `locate ${kind} ${cell}`}
                                  onClick={() => pickCell(kind, cell, undefined, reason, row.cellRefs[ci()] ?? "")}
                                >
                                  {cell}
                                </button>
                                <SheetBadges
                                  items={row.cellSheets[ci()] ?? []}
                                  label={(b) => b.name}
                                  title={(b) => `show sheet ${b.name}`}
                                  onSelect={(b) => pickCell(kind, cell, b.id, reason, row.cellRefs[ci()] ?? "")}
                                  active={(b) => isCurrentSheet(kind, cell, b.id)}
                                />
                              </td>
                            );
                          }}
                        </For>
                      </tr>
                      <Show when={expanded().has(i)}>
                        <tr class="query-cite-row">
                          <td />
                          <td colspan={state().columns.length}>
                            <Show
                              when={row.witnesses.length > 0}
                              fallback={
                                <>
                                  <ul class="query-cites">
                                    <For each={row.cites}>{(c) => <li>{c}</li>}</For>
                                  </ul>
                                  <Show when={!state().witnessed && props.onExplain}>
                                    <div class="query-witness-note" role="status">Working out how this row holds…</div>
                                  </Show>
                                  <Show when={state().witnessed}>
                                    <div class="query-witness-note">An aggregate row merges many derivations, so it has no single one to show.</div>
                                  </Show>
                                </>
                              }
                            >
                              <WitnessTree
                                items={row.witnesses}
                                pick={(v, sheet) => pickCell(v.kind, v.text, sheet, v.reason, v.ref)}
                                isCurrent={(v) => isCurrent(v.kind, v.text)}
                                isCurrentSheet={(v, sheet) => isCurrentSheet(v.kind, v.text, sheet)}
                              />
                            </Show>
                          </td>
                        </tr>
                      </Show>
                    </>
                    );
                  }}
                </For>
              </tbody>
            </table>
            <div class="query-count">
              <Show when={state().total > 0} fallback={<>{state().rows.length} result(s)</>}>
                {state().rows.length} of {state().total} result(s){" "}
                <button type="button" class="query-show-all" onClick={showAll} title="Run the same question again without the limit.">
                  show all
                </button>
              </Show>
              <Show when={props.onSaveCSV}>
                {" "}
                <button type="button" class="query-save-csv" title="Save this answer as a csv file, the same file agni query writes as csv" onClick={() => props.onSaveCSV?.(front().id)}>
                  Save CSV
                </button>
              </Show>
              {/* The set case. The answers on screen ARE a selection, which is why findingsFor takes
                  a set rather than one subject. */}
              <Show when={resultSubjects().length > 0}>
                <FindingsCount
                  tally={resultTally()}
                  state={checkedState(props.findings())}
                  title={countTitle()}
                  scope={`these ${resultSubjects().length} ${resultSubjects().length === 1 ? "entity" : "entities"}`}
                  gated={props.findings().skipped.length}
                />
              </Show>
            </div>
          </div>
        </Show>
      </Show>
    </div>
  );
}

// queryPanelIsland mounts the panel and returns its command-down view, per
// docsite/content/architecture/web-client.md#wiring-a-new-panel. onRun names the tab that asked and
// is answered through view.setState for that tab. onLocate (WS9-038) takes the same
// navigate-and-highlight path a finding click does, and the presenter pushes a note through
// setLocateNote when the highlight paints nothing (WS9-039).
export function queryPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    // onRun asks one tab's question. limit is 0 for the whole answer.
    onRun: (text: string, bindings: Bindings, limit: number, tab: string) => void;
    // onExplain asks one tab's last question again with witnesses, answered through view.setState
    // for that tab (agni issue 794). Absent, an opened row shows its citations alone.
    onExplain?: (tab: string) => void;
    // onSaveCSV saves the answer in the named tab as csv (agni issue 956). Absent, the panel offers
    // no save.
    onSaveCSV?: (tab: string) => void;
    onLocate?: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, pin?: string) => void;
    // onInspect opens the check results for one entity. Optional like onLocate, since an embedding
    // host may mount the query panel with no checks panel (C13).
    onInspect?: (sel: Selection) => void;
    // onActivate says a tab came to the front, with what it last located and what the tab it
    // replaced had located, so the host can make the drawing match the answer now in front.
    onActivate?: (tab: string, located: Located | undefined, previous: Located | undefined) => void;
    // onClose says a tab closed, so the host can drop the answer it kept for it.
    onClose?: (tab: string) => void;
    // onRunSet answers a set the panel opened as tabs, one tab id per query in the set's order.
    onRunSet?: (set: QuerySet, tabs: string[]) => void;
    // onSetFile hands over a set file the reader picked, for the host to parse and pass back through
    // view.openSet. Absent, the Sets menu offers no file.
    onSetFile?: (name: string, text: string) => void;
    // describe asks ListRelations about one place in the namespace, under the open design's snapshot
    // so a project's own library lists beside the shipped one. Absent, the picker shows the hint.
    describe?: (path: string) => Promise<RelationEntryItem>;
  },
): { island: SolidIsland; view: QueryView } {
  // results holds each tab's answer by tab id, so an answer that arrives after the reader moved to
  // another tab still lands in the tab that asked.
  const [results, setResults] = signalView<Record<string, QueryResult>>({});
  let tabs = 1;
  const newTabId = () => `q${++tabs}`;
  const [strip, setStrip] = signalView<TabStrip>({ tabs: [blankTab("q1", "Query 1")], active: "q1" });
  const [relations, setRelations] = signalView<RelationItem[]>([]);
  const [examples, setExamples] = signalView<ExampleItem[]>([]);
  const [locateNote, setLocateNote] = signalView<string>("");
  // prefill carries a query written FOR the reader. The counter makes asking the same question twice
  // open and run it again, since an effect over identical text does not fire.
  const [prefill, setPrefill] = signalView<{ text: string; bindings: Bindings; n: number; title?: string; whole?: boolean }>({ text: "", bindings: {}, n: 0 });
  let prefills = 0;
  // The served questions about a selection. A signal because the panel RENDERS one (the ask button's
  // hover shows the query and what it teaches) as well as running it.
  const [entityQueries, setEntityQueries] = signalView<EntityQueryItem[]>([]);
  // The find-by-name template, null until the catalog arrives. The panel offers no search mode
  // while it is null (agni issue 338).
  const [search, setSearch] = signalView<SearchItem | null>(null);
  // selection is what the reader last picked. A click on a result cell sets it from inside the panel,
  // which is why the setter goes down as a prop.
  const [selection, setSelection] = signalView<Selection | null>(null);
  // The sheet on screen, pushed by the presenter on every navigation. The panel needs it to mark
  // which of a cell's badges is the one being shown.
  const [currentSheet, setCurrentSheet] = signalView<string>("");
  // The check results, whole. The panel projects them onto the selection and the answer set and
  // never asks for a scoped re-run (agni issue 259).
  const [findings, setFindings] = signalView<FindingsState>(emptyFindings());
  const onLocate = handlers.onLocate ?? (() => {});
  const onInspect = handlers.onInspect ?? (() => {});
  let askHook: (sel: Selection, question?: EntityQueryItem) => void = () => {};
  let openSetHook: (set: QuerySet) => void = () => {};
  const [querySets, setQuerySets] = signalView<QuerySetItem[]>([]);
  // A fresh answer for the tab in front clears any stale locate note from its previous run.
  const setTabState = (s: QueryResult, tab?: string) => {
    const id = tab ?? strip().active;
    // A tab closed while its query ran has nowhere to show the answer, so it is not kept.
    if (!strip().tabs.some((t) => t.id === id)) return;
    if (id === strip().active) setLocateNote("");
    setResults({ ...results(), [id]: s });
  };
  const island = new SolidIsland(
    "query",
    el,
    () => (
      <QueryPanel
        results={results}
        strip={strip}
        setStrip={setStrip}
        newTabId={newTabId}
        relations={relations}
        describe={handlers.describe}
        examples={examples}
        entityQueries={entityQueries}
        search={search}
        locateNote={locateNote}
        setLocateNote={setLocateNote}
        prefill={prefill}
        selection={selection}
        setSelection={setSelection}
        currentSheet={currentSheet}
        findings={findings}
        onRun={handlers.onRun}
        onExplain={handlers.onExplain}
        onSaveCSV={handlers.onSaveCSV}
        onLocate={onLocate}
        onInspect={onInspect}
        onActivate={handlers.onActivate ?? (() => {})}
        onClose={(tab) => {
          const next = { ...results() };
          delete next[tab];
          setResults(next);
          handlers.onClose?.(tab);
        }}
        registerAsk={(ask) => (askHook = ask)}
        querySets={querySets}
        onRunSet={handlers.onRunSet ?? (() => {})}
        onSetFile={handlers.onSetFile}
        registerOpenSet={(open) => (openSetHook = open)}
      />
    ),
    eventBus,
  );
  return {
    island,
    view: {
      setState: setTabState,
      clearAnswers: () => setResults({}),
      setRelations,
      setExamples,
      setLocateNote,
      setQuery: (text: string, bindings: Bindings = {}, opts: { title?: string; whole?: boolean } = {}) =>
        setPrefill({ text, bindings, n: ++prefills, ...opts }),
      setEntityQueries,
      setSearch,
      setSelection,
      setCurrentSheet,
      setFindings,
      questionsFor: (kind: string) => entityQueries().filter((p) => p.kind === kind),
      askAbout: (sel: Selection, question?: EntityQueryItem) => {
        setSelection(sel);
        askHook(sel, question);
      },
      setQuerySets,
      openSet: (set: QuerySet) => openSetHook(set),
    },
  };
}
