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
import { renderMarkdown } from "./markdown.js";
import { type Selection, askLabel, fillEntityQuery, labelFor, selectionFromCell } from "./selection.js";
import { SheetBadges } from "./sheetbadges.jsx";

// resolveRelationImages rewrites a relation Detail's relative image refs (images/<rel>.svg) to the
// server's /relation-docs/ route BEFORE markdown rendering. Making them root-absolute means the
// shared renderMarkdown walk (which prepends the /rule-docs/ base to still-relative refs) leaves
// them alone, so relation cards resolve to their own handler without touching rule-doc rendering.
function resolveRelationImages(md: string): string {
  return md.replace(/\]\(images\//g, "](/relation-docs/images/");
}

// QueryPanel is the ad-hoc datalog search surface (WS9-036 / WS3-029): type a query, run it, and
// see provenance-linked answer rows. The panel owns the ephemeral query text (a local signal) and
// emits onRun with it; the presenter loads the current design, evaluates the query server-side, and
// pushes the QueryResult back through state(). Provenance is per-row expandable so many-citation
// rows do not widen the results table. A run needs a file open — with none, the presenter reports
// that as the result's error. The relation catalog (WS9-037), pushed once via relations(), renders
// as click-to-insert chips grouped by kind so a user discovers the vocabulary without knowing it.
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
  onRun: (text: string) => void;
  onLocate: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason) => void;
}) {
  const [text, setText] = createSignal("");
  // expanded holds the row indices whose provenance is open — ephemeral view state, so it lives
  // here rather than in the pushed result.
  const [expanded, setExpanded] = createSignal<Set<number>>(new Set());
  // detailRel is the relation whose reference doc (Detail) is open in the inspect pane, or null
  // (WS14-005). Clicking a chip's info affordance opens it; the ✕ closes it. Ephemeral view state.
  const [detailRel, setDetailRel] = createSignal<RelationItem | null>(null);
  // widths holds a per-column pixel width once the reader has dragged that column's edge. Columns
  // nobody touched stay unset and share the table equally (see .query-table's fixed layout), which
  // is the fix for the old auto layout: it sized every column to its widest cell, so one long
  // provenance string or net name took most of the panel and the rest were unreadable slivers.
  const [widths, setWidths] = createSignal<Record<number, number>>({});
  const MIN_COL_PX = 48;

  // startResize drags one column edge. It is a pointer capture rather than window listeners so the
  // drag survives the cursor leaving the header, and it stops the click reaching the sort handler
  // underneath — dragging an edge must not also re-sort the table.
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

  // drawerOpen controls the slide-in helper drawer (examples + relations + reference). The main
  // surface is just the query textarea and the results table; the discovery chrome lives in the
  // drawer so it does not eat vertical space. A left-edge handle opens it, and clicking the textarea
  // (or running anything) closes it so the results are unobscured. The drawer stays mounted at all
  // times (translated off-screen when closed) so chip clicks work without opening it.
  // A query written for the reader lands in the box AND runs, so a click on the drawing answers
  // immediately; what it leaves behind is an editable query, which is how the click teaches the
  // language rather than routing around it.
  createEffect(() => {
    const p = props.prefill();
    if (!p.n) return;
    setText(p.text);
    setDrawerOpen(false);
    props.onRun(p.text);
  });

  const [drawerOpen, setDrawerOpen] = createSignal(false);
  // mode is which of the two ways in the reader is using: write a query, or type a name. They are
  // two ends of one lesson. A query says where to look and gets back what is there; a search says
  // what a thing is called and gets back where it is. Both leave editable datalog in the box, which
  // is why search is a mode on this panel rather than a search widget somewhere else.
  const [mode, setMode] = createSignal<"query" | "search">("query");
  const [term, setTerm] = createSignal("");
  // sortCol/sortDir are the results table's client-side sort (WS: sortable columns). sortCol is a
  // column index into state().columns (-1 = natural row order); clicking a header cycles
  // asc → desc → off. Sorting is view-only: rows carry their ORIGINAL index so the provenance
  // expand-state (keyed by original index) survives a re-sort.
  const [sortCol, setSortCol] = createSignal(-1);
  const [sortDir, setSortDir] = createSignal<"asc" | "desc">("asc");
  let taRef: HTMLTextAreaElement | undefined;
  const toggleCites = (i: number) => {
    const next = new Set<number>(expanded());
    if (next.has(i)) next.delete(i);
    else next.add(i);
    setExpanded(next);
  };
  // resetForRun clears the per-result view state (expanded provenance, sort) and closes the drawer,
  // so a fresh run starts from natural order with the results unobscured.
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
  // runExample fills the textarea with a starter query AND runs it (WS14-002): the immediacy — click,
  // see results — is the point, and the query stays visible/editable for the user to tweak next.
  const runExample = (e: ExampleItem) => {
    setText(e.query);
    if (props.state().loading) return;
    resetForRun();
    props.onRun(e.query);
  };
  // doSearch fills the box with the datalog that answers the reader's name, runs it, and hands the
  // panel BACK to query mode. Flipping back is the point rather than a shortcut: the reader ends up
  // looking at the sentence that answered them, one edit away from asking a better question. It is
  // the same bargain a click on the drawing makes, and search is the other end of it.
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
  // askAbout takes the next hop: fill the box with the preset for what is selected, and run it. It is
  // runExample with the values spliced in, and it leaves the same editable query behind, because a
  // hop is meant to teach the sentence that made it as well as answer the question.
  const askAbout = (sel: Selection) => {
    const preset = presetFor(sel.kind);
    if (!preset || props.state().loading) return;
    const q = fillEntityQuery(preset.query, sel);
    setText(q);
    resetForRun();
    props.onRun(q);
  };
  // pickCell is a click on a result cell (or on one of its sheet badges). It locates the entity, as
  // it always has, AND selects it — so the answer to one question becomes the subject of the next,
  // which is what makes the table a place to walk from rather than a dead end. A cell whose kind has
  // no selection shape (a scalar never reaches here; a future entity kind might) locates and
  // deselects rather than leaving the bar naming the previous pick.
  const pickCell = (kind: string, subject: string, sheet: string | undefined, reason: LocateReason) => {
    props.setSelection(selectionFromCell(kind, subject));
    props.onLocate(kind, subject, sheet, reason);
  };
  // cmpCells is a numeric-aware string compare: two numeric cells sort by value (so 9 < 10), any
  // other pair sorts lexicographically. sortRows applies it, carrying each row's original index.
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
  // sortBy cycles a column: first click sorts ascending, second descending, third clears back to the
  // server's natural order.
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
  // insertRelation splices a relation template into the query at the caret (replacing any
  // selection), so a chip click extends the query in place rather than clobbering it. It restores
  // focus and drops the caret just after the inserted snippet.
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
      {/* Two ways in, offered only once the server has sent a search template: without one, a
          search mode could do nothing but guess at a query. */}
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

      {/* The selection bar names what the reader last picked — on the drawing or in the results — and
          offers the one question there is a served preset for. It is the visible half of the walk:
          without it, a click on a result cell highlights something and says nothing about where the
          reader can go from there. */}
      <Show when={props.selection()}>
        {(sel) => (
          <div class="query-selection">
            <span class="query-selection-what">
              <span class="query-selection-kind">{sel().kind}</span>
              <span class="query-selection-name">{labelFor(sel())}</span>
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

      {/* The drawer stays mounted (translated off-screen when closed) so chip clicks and the
          reference pane work whether or not it is open — the tests drive them without opening it. */}
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
                            const kind = cellKind(props.state(), row, ci());
                            const reason = row.cellReasons[ci()] ?? LocateReason.UNSPECIFIED;
                            // Only an entity cell is locatable; a scalar (a voltage, an mpn
                            // string) stays plain text. Which cells those are can vary ROW BY ROW
                            // under a polymorphic column, so the kind comes from cellKind rather
                            // than straight off the column (agni issue 338). A located cell shows its subject as a link
                            // and its sheet badge(s) inline (the findings-panel idiom): the subject
                            // highlights the entity, a badge navigates to that sheet. The reason
                            // (WS9-039) rides along so the presenter can explain a click that paints
                            // nothing (a power rail, a virtual symbol).
                            return kind === "" ? (
                              <td>{cell}</td>
                            ) : (
                              <td class="query-cell-locate">
                                <button
                                  type="button"
                                  class="query-locate"
                                  title={`locate ${kind} ${cell}`}
                                  onClick={() => pickCell(kind, cell, undefined, reason)}
                                >
                                  {cell}
                                </button>
                                <SheetBadges
                                  items={row.cellSheets[ci()] ?? []}
                                  label={(b) => b.name}
                                  title={(b) => `show sheet ${b.name}`}
                                  onSelect={(b) => pickCell(kind, cell, b.id, reason)}
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
            <div class="query-count">{props.state().rows.length} result(s)</div>
          </div>
        </Show>
      </Show>
    </div>
  );
}

// queryPanelIsland mounts the panel and returns its command-down view. onRun is the intent up (the
// user ran a query); the presenter answers by pushing a QueryResult through view.setState. onLocate
// is the second intent up (WS9-038: the user clicked a result cell or its sheet badge) — the
// presenter navigates to the sheet and highlights the entity, the same path a finding click takes,
// and (WS9-039) pushes back a locate note via setLocateNote when the highlight paints nothing.
// Same island shape as rulesPanelIsland / findingsPanelIsland.
export function queryPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onRun: (text: string) => void;
    onLocate?: (kind: string, subject: string, sheet: string | undefined, reason: LocateReason) => void;
  },
): { island: SolidIsland; view: QueryView } {
  const [state, setState] = signalView<QueryResult>(emptyResult());
  const [relations, setRelations] = signalView<RelationItem[]>([]);
  const [examples, setExamples] = signalView<ExampleItem[]>([]);
  const [locateNote, setLocateNote] = signalView<string>("");
  // prefill carries a query written FOR the reader (a click on the drawing generates one). The
  // counter is what makes clicking the same pin twice re-fill and re-run: the text would be
  // identical, and an effect over identical text does not fire.
  const [prefill, setPrefill] = signalView<{ text: string; n: number }>({ text: "", n: 0 });
  let prefills = 0;
  // The click-to-ask presets, held as a signal because the panel now RENDERS one (the ask button's
  // wording and its hover show the query and what it teaches) as well as running it; the host still
  // looks one up by the picked entity's kind (see entityQuery).
  const [entityQueries, setEntityQueries] = signalView<EntityQueryItem[]>([]);
  // The find-by-name template, null until the catalog arrives. The panel offers no search mode
  // while it is null (agni issue 338).
  const [search, setSearch] = signalView<SearchItem | null>(null);
  // selection is what the reader last picked. The canvas pushes one through the view; a click on a
  // result cell sets it from inside the panel, which is why the setter goes down as a prop.
  const [selection, setSelection] = signalView<Selection | null>(null);
  const onLocate = handlers.onLocate ?? (() => {});
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
        onRun={handlers.onRun}
        onLocate={onLocate}
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
      entityQuery: (kind: string) => entityQueries().find((p) => p.kind === kind)?.query ?? "",
    },
  };
}
