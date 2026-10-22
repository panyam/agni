import { For, Show, createMemo } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { type Crumb, type Datasheet, type Focus, type PropertiesState, type Ref, reportLines } from "./properties.js";
import type { EntityQueryItem } from "./query.js";
import type { Selection } from "./selection.js";

// PropertiesView is the command-down port: the presenter pushes one state per focus.
export interface PropertiesView {
  setState: (s: PropertiesState) => void;
}

export interface PropertiesHandlers {
  // onFocus moves the selection, from a crumb or a reference inside the panel.
  onFocus: (f: Focus) => void;
  // onAsk opens one served question about the selection in a query tab of its own.
  onAsk: (sel: Selection, question: EntityQueryItem) => void;
  // onVerdict shows one of the selection's verdicts with its proof on the drawing (agni issue 897).
  onVerdict?: (id: string) => void;
  // onTrace names the selected part or pin as one end of a trace (agni issue 1133).
  onTrace?: (end: "from" | "to", sel: Selection) => void;
}

function emptyState(): PropertiesState {
  return { crumbs: [], kind: "", title: "", rows: [], groups: [] };
}

// The same three-way outcome the parts panel's empty state hints at, said per part.
function datasheetLine(d: Datasheet): string {
  switch (d.kind) {
    case "no-corpus":
      return "No datasheet corpus is connected, so nothing was looked up.";
    case "no-mpn":
      return "This part has no part number, so there is nothing to look up.";
    case "no-spec":
      return `The corpus has no spec for ${d.mpn}.`;
    case "spec":
      return `${d.mpn}, ${d.spec.parameters.length} parameters${d.corpus ? `, from the ${d.corpus} corpus` : ""}`;
  }
}

function RefList(props: { refs: Ref[]; empty?: string; onFocus: (f: Focus) => void; onVerdict?: (id: string) => void; sheetId?: string }) {
  return (
    <Show when={props.refs.length > 0} fallback={<div class="pr-empty">{props.empty ?? "none"}</div>}>
      <ul class="pr-refs">
        <For each={props.refs}>
          {(r) => (
            <li class="pr-ref">
              <Show
                when={r.target}
                fallback={
                  <Show
                    when={r.sheet}
                    fallback={
                      <Show when={r.verdict && props.onVerdict} fallback={<span class="pr-ref-label">{r.label}</span>}>
                        <button
                          type="button"
                          class="pr-ref-go pr-ref-verdict"
                          data-verdict={r.verdict}
                          title="show this verdict's proof on the drawing"
                          onClick={() => props.onVerdict?.(r.verdict ?? "")}
                        >
                          {r.label}
                        </button>
                      </Show>
                    }
                  >
                    {(id) => (
                      <button type="button" class="pr-ref-go pr-ref-sheet" title={`show sheet ${r.label}`} onClick={() => props.onFocus({ level: "sheet", sheetId: id() })}>
                        {r.label}
                      </button>
                    )}
                  </Show>
                }
              >
                {(t) => (
                  <button type="button" class="pr-ref-go" onClick={() => props.onFocus({ level: "entity", sel: t(), sheetId: props.sheetId })}>
                    {r.label}
                  </button>
                )}
              </Show>
              <Show when={r.detail}>
                <span class="pr-ref-detail">{r.detail}</span>
              </Show>
            </li>
          )}
        </For>
      </ul>
    </Show>
  );
}

function PropertiesPanel(props: { state: () => PropertiesState; handlers: PropertiesHandlers }) {
  const s = props.state;
  // The sheet an entity was picked on carries into the entity it leads to, so a walk inside the
  // panel keeps its breadcrumb.
  const sheetId = createMemo(() => {
    const sheet = s().crumbs.find((c) => c.focus.level === "sheet")?.focus;
    return sheet?.level === "sheet" ? sheet.sheetId : undefined;
  });
  const report = createMemo(() => reportLines(s().report));
  // traceable is a selected part or pin, either of which can end a trace.
  const traceable = () => {
    const sel = s().sel;
    return sel && (sel.kind === "component" || sel.kind === "pin") ? sel : undefined;
  };
  return (
    <div class="properties">
      <nav class="pr-crumbs" aria-label="Selection">
        <For each={s().crumbs}>
          {(c: Crumb, i) => (
            <>
              <Show when={i() > 0}>
                <span class="pr-crumb-sep">›</span>
              </Show>
              <button
                type="button"
                class="pr-crumb"
                classList={{ "pr-crumb-current": i() === s().crumbs.length - 1 }}
                onClick={() => props.handlers.onFocus(c.focus)}
              >
                {c.label}
              </button>
            </>
          )}
        </For>
      </nav>
      <Show when={s().kind}>
        <div class="pr-head">
          <span class="pr-kind">{s().kind}</span>
          <span class="pr-title">{s().title}</span>
        </div>
      </Show>
      <Show when={s().note}>
        <div class="pr-note">{s().note}</div>
      </Show>
      <Show when={s().rows.length > 0}>
        <dl class="pr-rows">
          <For each={s().rows}>
            {(r) => (
              <>
                <dt>{r.label}</dt>
                <dd data-row={r.label}>{r.value}</dd>
              </>
            )}
          </For>
        </dl>
      </Show>
      <Show when={s().datasheet}>
        {(d) => (
          <div class={`pr-datasheet pr-ds-${d().kind}`}>
            <span class="pr-section">datasheet</span> {datasheetLine(d())}
          </div>
        )}
      </Show>
      <For each={s().groups}>
        {(g) => (
          <details class="pr-group" data-group={g.name} open={g.refs.length > 0 && g.refs.length <= 12}>
            <summary>
              {g.name} <span class="pr-count">({g.refs.length})</span>
            </summary>
            <RefList refs={g.refs} empty={g.empty} onFocus={props.handlers.onFocus} onVerdict={props.handlers.onVerdict} sheetId={sheetId()} />
          </details>
        )}
      </For>
      <Show when={report()}>
        {(r) => (
          <div class="pr-report">
            <span class="pr-section">conversion</span> {r().counts}
            <Show when={r().box.length}>
              <div class="report-box">{r().box.length} unmapped (no device glyph): {r().box.join(" ")}</div>
            </Show>
            <Show when={r().unresolved.length}>
              <div class="report-unresolved">
                {r().unresolved.length} unresolved, pass --symbol-path: {r().unresolved.join(" ")}
              </div>
            </Show>
          </div>
        )}
      </Show>
      {/* The served questions about the selection, each opening a query tab of its own. The hover
          shows the query, so a reader sees what will run before running it. */}
      <Show when={s().ask}>
        {(a) => (
          <div class="pr-ask">
            <span class="pr-section">ask</span>
            <For each={a().questions}>
              {(q) => (
                <button type="button" class="pr-ask-go" title={`${q.query}\n\n${q.teaches}`} onClick={() => props.handlers.onAsk(a().sel, q)}>
                  {q.label}
                </button>
              )}
            </For>
          </div>
        )}
      </Show>
      <Show when={props.handlers.onTrace && traceable()}>
        {(sel) => (
          <div class="pr-trace">
            <span class="pr-section">trace</span>
            <button type="button" class="pr-ask-go pr-trace-from" onClick={() => props.handlers.onTrace!("from", sel())}>
              from here
            </button>
            <button type="button" class="pr-ask-go pr-trace-to" onClick={() => props.handlers.onTrace!("to", sel())}>
              to here
            </button>
          </div>
        )}
      </Show>
    </div>
  );
}

// propertiesPanelIsland mounts the panel and returns its view. Same island shape as the parts panel.
export function propertiesPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: PropertiesHandlers,
): { island: SolidIsland; view: PropertiesView } {
  const [state, setState] = signalView<PropertiesState>(emptyState());
  const island = new SolidIsland("properties", el, () => <PropertiesPanel state={state} handlers={handlers} />, eventBus);
  return { island, view: { setState } };
}
