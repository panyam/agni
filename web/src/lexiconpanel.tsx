import { For, Show, createMemo, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { type LexiconRow, type LexiconState, type LexiconView, canRemove, emptyLexicon, subjectOf } from "./lexicon.js";
import type { Table } from "./gen/agni/v1/webapi/tables_pb.js";

export interface LexiconHandlers {
  onAdd: (vocabulary: string, kind: string, value: string) => void;
  onRemove: (row: LexiconRow) => void;
  onOff: (vocabulary: string, off: boolean) => void;
  onInfer: (on: boolean) => void;
  onRename: (name: string) => void;
  onTry: () => void;
  onSave: () => void;
  onApply: () => void;
  onLocate: (kind: string, name: string) => void;
}

interface Group {
  vocabulary: string;
  rows: LexiconRow[];
  off: boolean;
}

function kindsOf(vocabulary: string): string[] {
  return vocabulary.startsWith("class.") ? ["pattern", "family", "prefix"] : ["pattern", "exclude"];
}

// groupRows folds the listing into its vocabularies. A vocabulary is off when something was replaced
// and nothing that matches is left.
function groupRows(rows: LexiconRow[]): Group[] {
  const groups = new Map<string, Group>();
  for (const r of rows) {
    if (r.kind === "switch") continue;
    let g = groups.get(r.vocabulary);
    if (!g) groups.set(r.vocabulary, (g = { vocabulary: r.vocabulary, rows: [], off: false }));
    g.rows.push(r);
  }
  for (const g of groups.values()) {
    const judged = g.rows.filter((r) => r.kind !== "prefix");
    g.off = judged.some((r) => r.replaced) && !judged.some((r) => !r.replaced);
  }
  return [...groups.values()];
}

function AddEntry(props: { vocabulary: string; onAdd: LexiconHandlers["onAdd"] }) {
  const [text, setText] = createSignal("");
  const [kind, setKind] = createSignal("pattern");
  const add = () => {
    if (!text().trim()) return;
    props.onAdd(props.vocabulary, kind(), text());
    setText("");
  };
  return (
    <div class="lex-add">
      <select aria-label="entry kind" value={kind()} onChange={(e) => setKind(e.currentTarget.value)}>
        <For each={kindsOf(props.vocabulary)}>{(k) => <option value={k}>{k}</option>}</For>
      </select>
      <input
        placeholder="RE2 pattern"
        aria-label={`add to ${props.vocabulary}`}
        value={text()}
        onInput={(e) => setText(e.currentTarget.value)}
        onKeyDown={(e) => e.key === "Enter" && add()}
      />
      <button type="button" onClick={add}>
        Add
      </button>
    </div>
  );
}

function ChangeTable(props: { table: Table; title: string; onLocate: LexiconHandlers["onLocate"] }) {
  return (
    <div class="lex-result" data-table={props.table.name}>
      <div class="lex-result-title">{props.title}</div>
      <Show when={props.table.rows.length > 0}>
        <table class="query-table">
          <thead>
            <tr>
              <For each={props.table.columns}>{(c) => <th>{c.name}</th>}</For>
            </tr>
          </thead>
          <tbody>
            <For each={props.table.rows}>
              {(r) => {
                const s = subjectOf(props.table, r.cells);
                return (
                  <tr class="lex-change" onClick={() => s && props.onLocate(s.kind, s.name)}>
                    <For each={r.cells}>{(c) => <td>{c}</td>}</For>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </Show>
    </div>
  );
}

// LexiconPanel lists every vocabulary a run reads names with and where each entry came from, and edits
// a draft of the convention. Nothing it edits reaches the page until Apply: Try asks what the draft
// would change and Save writes the file.
function LexiconPanel(props: { state: () => LexiconState; on: LexiconHandlers }) {
  const [filter, setFilter] = createSignal("");
  const groups = createMemo(() => groupRows(props.state().rows));
  const shown = createMemo(() => {
    const f = filter().toLowerCase();
    return f ? groups().filter((g) => g.vocabulary.includes(f) || g.rows.some((r) => r.value.toLowerCase().includes(f))) : groups();
  });
  // Keyed by name, so an edit, which lists every row again, leaves a reader's open group and half-typed
  // pattern alone.
  const names = createMemo(() => shown().map((g) => g.vocabulary), undefined, { equals: (a, b) => a.join() === b.join() });
  const groupOf = (name: string) => groups().find((g) => g.vocabulary === name);
  const inference = createMemo(() => props.state().rows.find((r) => r.kind === "switch"));
  const tableTitle = (t: Table) =>
    t.name === "role_changes" ? `${t.rows.length} net(s) change roles` : `${t.rows.length} finding(s) change`;
  return (
    <div class="lex-panel">
      <div class="lex-bar">
        <input aria-label="convention name" title="the convention's name, which its rules appear under" value={props.state().name} onChange={(e) => props.on.onRename(e.currentTarget.value)} />
        <button type="button" class="lex-try" disabled={props.state().busy} onClick={props.on.onTry}>
          Try
        </button>
        <button type="button" class="lex-save" onClick={props.on.onSave}>
          Save YAML
        </button>
        <button type="button" class="lex-apply" disabled={props.state().busy || !props.state().dirty} onClick={props.on.onApply}>
          Apply
        </button>
      </div>
      <Show when={props.state().error}>
        <div class="checks-error" role="alert">
          {props.state().error}
        </div>
      </Show>
      <Show when={props.state().busy}>
        <div class="lex-note">running the checks under both vocabularies…</div>
      </Show>
      <Show when={props.state().dirty && !props.state().busy}>
        <div class="lex-note">This draft changes nothing on the page until Apply, and Apply lasts for this session. Save the YAML into the project to keep it.</div>
      </Show>
      <Show when={props.state().tables}>
        {(ts) => (
          <div class="lex-results">
            <For each={ts()}>{(t) => <ChangeTable table={t} title={tableTitle(t)} onLocate={props.on.onLocate} />}</For>
          </div>
        )}
      </Show>
      <Show when={inference()}>
        {(s) => (
          <label class="lex-note lex-switch">
            <input type="checkbox" class="lex-infer" checked={s().value.endsWith("=true")} onChange={(e) => props.on.onInfer(e.currentTarget.checked)} /> infer rails from the supply pins a net feeds <span class="lex-source">{s().source}</span>
          </label>
        )}
      </Show>
      <input class="lex-filter" placeholder="filter vocabularies" aria-label="filter vocabularies" value={filter()} onInput={(e) => setFilter(e.currentTarget.value)} />
      <For each={names()}>
        {(name) => (
          <details class="lex-group" data-vocabulary={name}>
            <summary>
              {name}
              <Show when={groupOf(name)?.off}>
                <span class="lex-off"> (off)</span>
              </Show>
            </summary>
            <label class="lex-off-switch">
              <input type="checkbox" checked={groupOf(name)?.off ?? false} onChange={(e) => props.on.onOff(name, e.currentTarget.checked)} /> turn this vocabulary off
            </label>
            <For each={groupOf(name)?.rows ?? []}>
              {(r) => (
                <div class={`lex-row${r.replaced ? " lex-row-replaced" : ""}${r.edited ? " lex-row-edited" : ""}`}>
                  <span class="lex-kind">{r.kind}</span>
                  <span class="lex-value">{r.value}</span>
                  <span class="lex-source">{r.source}</span>
                  <Show when={canRemove(r)}>
                    <button type="button" class="lex-remove" title={r.builtin ? "remove; this writes the vocabulary out with replace: true" : "remove"} aria-label={`remove ${r.value}`} onClick={() => props.on.onRemove(r)}>
                      ×
                    </button>
                  </Show>
                </div>
              )}
            </For>
            <AddEntry vocabulary={name} onAdd={props.on.onAdd} />
          </details>
        )}
      </For>
    </div>
  );
}

// lexiconPanelIsland mounts the panel and returns its view.
export function lexiconPanelIsland(el: HTMLElement, eventBus: EventBus | null, handlers: LexiconHandlers): { island: SolidIsland; view: LexiconView } {
  const [state, setState] = signalView<LexiconState>(emptyLexicon());
  const island = new SolidIsland("lexicon", el, () => <LexiconPanel state={state} on={handlers} />, eventBus);
  return { island, view: { setState } };
}
