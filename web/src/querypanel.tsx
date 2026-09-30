import { For, Show, createEffect, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type EntityQueryItem,
  type ExampleItem,
  type QueryResult,
  type QueryView,
  type RelationItem,
  type SearchItem,
  LocateReason,
  cellKind,
  emptyResult,
  fillSearchQuery,
  groupRelations,
  relationTemplate,
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
import { type Selection, askLabel, fillEntityQuery, labelFor, sameSelection, selectionFromCell } from "./selection.js";
import { SheetBadges } from "./sheetbadges.jsx";

// resolveRelationImages rewrites a relation Detail's relative image refs (images/<rel>.svg) to the
// server's /relation-docs/ route BEFORE markdown rendering. renderMarkdown prepends the /rule-docs/
// base only to still-relative refs, so it leaves these alone.
function resolveRelationImages(md: string): string {
  return md.replace(/\]\(images\//g, "](/relation-docs/images/");
}

// emptyFindings is the state before the presenter has pushed anything. It has no rules selected, so
// the count reads "no rules selected" rather than "no findings".
function emptyFindings(): FindingsState {
  return { findings: [], verdicts: [], focusedVerdict: "", selected: "", ruleCount: 0, pending: 0, running: false, skipped: [], ruleSummaries: {} };
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

// QueryPanel is the ad-hoc datalog query panel (WS9-036 / WS3-029). It owns the query text and emits
// onRun; the presenter evaluates it server-side against the open design and pushes the QueryResult
// back through state(), reporting "no file open" as the result's error. Provenance expands per row so
// many-citation rows do not widen the table. The relation catalog (WS9-037) renders as click-to-insert
// chips grouped by kind.
function QueryPanel(props: {
  state: () => QueryResult;
  relations: () => RelationItem[];
  examples: () => ExampleItem[];
  entityQueries: () => EntityQueryItem[];
  search: () => SearchItem | null;
  locateNote: () => string;
  prefill: () => { text: string; n: number };
  selection: () => Selection | null;
  setSelection: (sel: Selection | null) => void;
  currentSheet: () => string;
  findings: () => FindingsState;
  onRun: (text: string) => void;
  onInspect: (sel: Selection) => void;
  onLocate: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, pin?: string) => void;
}) {
  const [text, setText] = createSignal("");
  // expanded holds the row indices whose provenance is open. It is view state, so it lives here
  // rather than in the pushed result.
  const [expanded, setExpanded] = createSignal<Set<number>>(new Set());
  // detailRel is the relation whose reference doc (Detail) is open in the inspect pane, or null
  // (WS14-005).
  const [detailRel, setDetailRel] = createSignal<RelationItem | null>(null);
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

  // A query written for the reader (a click on the drawing) lands in the box AND runs, and leaves an
  // editable query behind so the click teaches the language.
  createEffect(() => {
    const p = props.prefill();
    if (!p.n) return;
    setText(p.text);
    setDrawerOpen(false);
    props.onRun(p.text);
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
  const toggleCites = (i: number) => {
    const next = new Set<number>(expanded());
    if (next.has(i)) next.delete(i);
    else next.add(i);
    setExpanded(next);
  };
  // resetForRun clears the per-result view state (expanded provenance, sort) and closes the drawer.
  const resetForRun = () => {
    setExpanded(new Set<number>());
    setSortCol(-1);
    setDrawerOpen(false);
  };
  const run = () => {
    const q = text().trim();
    if (q === "" || props.state().loading) return;
    resetForRun();
    props.onRun(q);
  };
  // runExample fills the textarea with a starter query AND runs it (WS14-002), leaving the query in
  // the box to edit.
  const runExample = (e: ExampleItem) => {
    setText(e.query);
    if (props.state().loading) return;
    resetForRun();
    props.onRun(e.query);
  };
  // doSearch fills the box with the datalog that answers the reader's name, runs it, and switches the
  // panel BACK to query mode, so the reader sees the query that answered them and can edit it.
  const doSearch = () => {
    const tmpl = props.search();
    const t = term().trim();
    if (!tmpl || t === "" || props.state().loading) return;
    const q = fillSearchQuery(tmpl.query, t);
    setText(q);
    setMode("query");
    resetForRun();
    props.onRun(q);
  };
  // presetFor is the served click-to-ask query for a selection's kind, or undefined before the
  // catalog has arrived (or for a kind the server writes no preset for).
  const presetFor = (kind: string): EntityQueryItem | undefined => props.entityQueries().find((p) => p.kind === kind);
  // askAbout fills the box with the preset for what is selected and runs it. It is runExample with the
  // selection's values spliced in, and leaves the same editable query behind.
  const askAbout = (sel: Selection) => {
    const preset = presetFor(sel.kind);
    if (!preset || props.state().loading) return;
    const q = fillEntityQuery(preset.query, sel);
    setText(q);
    resetForRun();
    props.onRun(q);
  };
  // pickCell is a click on a result cell or one of its sheet badges. It locates the entity AND
  // selects it, so one answer becomes the subject of the next question. A kind with no selection
  // shape (scalars never reach here) locates and deselects rather than leaving the bar naming the
  // previous pick.
  //
  // `ref` is the row's other half for a pin cell, "" for every other kind. A pin inverts the locate
  // arguments, since the entity is the COMPONENT and the cell is its designator, so the call is
  // (pin, U7, ..., "5") where a component cell sends (component, U7).
  const pickCell = (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, ref = "") => {
    const sel = selectionFromCell(kind, subject, ref);
    props.setSelection(sel);
    if (kind === "pin") {
      if (ref) props.onLocate(kind, ref, sheet, reason, subject);
      return;
    }
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
    const st = props.state();
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
      state === "not-run"
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
      ? `\n\n${st.skipped.length} selected rule(s) could not run on this design at all, so they report nothing anywhere:\n` +
        st.skipped.map((s) => `  ${s.rule}: ${s.reason}`).join("\n")
      : "";
    return (
      `${head ? head + "\n\n" : ""}Findings from the SELECTED rules that name this subject. Not a coverage statement: ` +
      `a design-global rule has no subject and can never appear here, and a rule that checks two terminals names one of them, ` +
      `so the other end shows nothing.${gated}`
    );
  };
  // cmpCells sorts two numeric cells by value (so 9 < 10) and any other pair lexicographically.
  // displayRows applies it, carrying each row's original index.
  const cmpCells = (a: string, b: string): number => {
    const na = Number(a);
    const nb = Number(b);
    const aNum = a.trim() !== "" && Number.isFinite(na);
    const bNum = b.trim() !== "" && Number.isFinite(nb);
    if (aNum && bNum) return na - nb;
    return a.localeCompare(b);
  };
  const displayRows = (): { row: QueryResult["rows"][number]; idx: number }[] => {
    const indexed = props.state().rows.map((row, idx) => ({ row, idx }));
    const col = sortCol();
    if (col < 0) return indexed;
    const dir = sortDir() === "asc" ? 1 : -1;
    return [...indexed].sort((x, y) => dir * cmpCells(x.row.cells[col] ?? "", y.row.cells[col] ?? ""));
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
  const insertRelation = (r: RelationItem) => {
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
                disabled={props.state().loading || term().trim() === ""}
                title={tmpl().teaches}
                onClick={doSearch}
              >
                {props.state().loading ? "Searching…" : "Find"}
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
          placeholder={'component-on-net(?r,?n), net.max_voltage(?n,?v), ?v < 30 => ?r, ?n'}
          value={text()}
          onInput={(e) => setText(e.currentTarget.value)}
          onPointerDown={() => setDrawerOpen(false)}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") run();
          }}
        />
        <div class="query-actions">
          <button type="button" class="query-run" disabled={props.state().loading || text().trim() === ""} onClick={run}>
            {props.state().loading ? "Running…" : "Run"}
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
                  disabled={props.state().loading}
                  title={`${fillEntityQuery(preset().query, sel())}\n\n${preset().teaches}`}
                  onClick={() => askAbout(sel())}
                >
                  {askLabel(sel())}
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
            when={props.relations().length > 0}
            fallback={
              <p class="query-relhint">Join on shared ?variables; =&gt; projects.</p>
            }
          >
            <div class="query-relations">
              <div class="query-relations-hint">Click a relation to insert it. Join on shared ?variables; =&gt; projects.</div>
              <For each={groupRelations(props.relations())}>
                {(group) => (
                  <div class="query-relgroup">
                    <span class="query-relgroup-name">{group.label}</span>
                    <For each={group.items}>
                      {(r) => (
                        <span class="query-relchip-wrap">
                          <button
                            type="button"
                            class="query-relchip"
                            title={`${relationTemplate(r)} — ${r.summary}`}
                            onClick={() => insertRelation(r)}
                          >
                            {r.name}
                          </button>
                          <Show when={r.detail !== ""}>
                            <button
                              type="button"
                              class="query-relinfo"
                              title={`Reference: ${r.name}`}
                              aria-label={`Reference for ${r.name}`}
                              onClick={() => setDetailRel(r)}
                            >
                              ?
                            </button>
                          </Show>
                        </span>
                      )}
                    </For>
                  </div>
                )}
              </For>
            </div>
          </Show>

          <Show when={detailRel()}>
            <div class="query-reldetail">
              <div class="query-reldetail-head">
                <span class="query-reldetail-name">{detailRel()!.name}</span>
                <button
                  type="button"
                  class="query-reldetail-close"
                  aria-label="Close reference"
                  onClick={() => setDetailRel(null)}
                >
                  ✕
                </button>
              </div>
              <div
                class="query-reldetail-body md"
                innerHTML={renderMarkdown(resolveRelationImages(detailRel()!.detail))}
              />
            </div>
          </Show>
        </div>
      </div>

      <Show when={props.state().error !== ""}>
        <div class="query-error">{props.state().error}</div>
      </Show>

      <Show when={props.locateNote() !== ""}>
        <div class="query-locate-note" role="status">{props.locateNote()}</div>
      </Show>

      <Show when={props.state().error === "" && props.state().ran}>
        <Show
          when={props.state().rows.length > 0}
          fallback={<div class="query-empty">No results.</div>}
        >
          <div class="query-results">
            <table class="query-table">
              {/* A colgroup carries the widths, so a drag sets ONE number rather than restyling every
                  cell in the column, and the fixed layout below honours it. */}
              <colgroup>
                <col class="query-cite-col" />
                <For each={props.state().columns}>{(_, ci) => <col style={widths()[ci()] ? { width: `${widths()[ci()]}px` } : undefined} />}</For>
              </colgroup>
              <thead>
                <tr>
                  <th class="query-cite-col" />
                  <For each={props.state().columns}>
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
                          <Show when={row.cites.length > 0}>
                            <button
                              type="button"
                              class={`query-cite-toggle${expanded().has(i) ? " open" : ""}`}
                              title="provenance: the facts that produced this row"
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
                            const rawKind = cellKind(props.state(), row, ci());
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
                          <td colspan={props.state().columns.length}>
                            <ul class="query-cites">
                              <For each={row.cites}>{(c) => <li>{c}</li>}</For>
                            </ul>
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
              {props.state().rows.length} result(s)
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
// docsite/content/architecture/web-client.md#wiring-a-new-panel. onRun is answered through
// view.setState. onLocate (WS9-038) takes the same navigate-and-highlight path a finding click does,
// and the presenter pushes a note through setLocateNote when the highlight paints nothing (WS9-039).
export function queryPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onRun: (text: string) => void;
    onLocate?: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason, pin?: string) => void;
    // onInspect opens the check results for one entity. Optional like onLocate, since an embedding
    // host may mount the query panel with no checks panel (C13).
    onInspect?: (sel: Selection) => void;
  },
): { island: SolidIsland; view: QueryView } {
  const [state, setState] = signalView<QueryResult>(emptyResult());
  const [relations, setRelations] = signalView<RelationItem[]>([]);
  const [examples, setExamples] = signalView<ExampleItem[]>([]);
  const [locateNote, setLocateNote] = signalView<string>("");
  // prefill carries a query written FOR the reader (a click on the drawing generates one). The
  // counter makes clicking the same pin twice re-fill and re-run, since an effect over identical
  // text does not fire.
  const [prefill, setPrefill] = signalView<{ text: string; n: number }>({ text: "", n: 0 });
  let prefills = 0;
  // The click-to-ask presets. A signal because the panel RENDERS one (the ask button's hover shows
  // the query and what it teaches) as well as running it. The host looks one up by kind through
  // entityQuery.
  const [entityQueries, setEntityQueries] = signalView<EntityQueryItem[]>([]);
  // The find-by-name template, null until the catalog arrives. The panel offers no search mode
  // while it is null (agni issue 338).
  const [search, setSearch] = signalView<SearchItem | null>(null);
  // selection is what the reader last picked. The canvas pushes one through the view, and a click on
  // a result cell sets it from inside the panel, which is why the setter goes down as a prop.
  const [selection, setSelection] = signalView<Selection | null>(null);
  // The sheet on screen, pushed by the presenter on every navigation. The panel needs it to mark
  // which of a cell's badges is the one being shown.
  const [currentSheet, setCurrentSheet] = signalView<string>("");
  // The check results, whole. The panel projects them onto the selection and the answer set and
  // never asks for a scoped re-run (agni issue 259).
  const [findings, setFindings] = signalView<FindingsState>(emptyFindings());
  const onLocate = handlers.onLocate ?? (() => {});
  const onInspect = handlers.onInspect ?? (() => {});
  // A fresh query result clears any stale locate note from the previous run.
  const setStateClearing = (s: QueryResult) => {
    setLocateNote("");
    setState(s);
  };
  const island = new SolidIsland(
    "query",
    el,
    () => (
      <QueryPanel
        state={state}
        relations={relations}
        examples={examples}
        entityQueries={entityQueries}
        search={search}
        locateNote={locateNote}
        prefill={prefill}
        selection={selection}
        setSelection={setSelection}
        currentSheet={currentSheet}
        findings={findings}
        onRun={handlers.onRun}
        onLocate={onLocate}
        onInspect={onInspect}
      />
    ),
    eventBus,
  );
  return {
    island,
    view: {
      setState: setStateClearing,
      setRelations,
      setExamples,
      setLocateNote,
      setQuery: (text: string) => setPrefill({ text, n: ++prefills }),
      setEntityQueries,
      setSearch,
      setSelection,
      setCurrentSheet,
      setFindings,
      entityQuery: (kind: string) => entityQueries().find((p) => p.kind === kind)?.query ?? "",
    },
  };
}
