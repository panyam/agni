import { For, Show, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { type PartsState, type PartsView, emptyParts } from "./parts.js";
import type { Parameter } from "./gen/agni/v1/param/param_pb.js";

// LIMIT_LABEL maps the PartSpec LimitKind enum (0 unspecified / 1 abs-max / 2 recommended /
// 3 characteristic) to the short badge a parameter row shows; the `lk-<n>` class colors it.
const LIMIT_LABEL: Record<number, string> = {
  1: "abs-max",
  2: "recommended",
  3: "characteristic",
};

// fmtVal renders a parameter's min/typ/max triple plus unit.
function fmtVal(p: Parameter): string {
  const v = p.value;
  if (!v) return "";
  const parts: string[] = [];
  if (v.min !== undefined) parts.push(`min ${v.min}`);
  if (v.typ !== undefined) parts.push(`typ ${v.typ}`);
  if (v.max !== undefined) parts.push(`max ${v.max}`);
  return parts.join(" / ") + (p.unit ? ` ${p.unit}` : "");
}

// fmtConditions joins the test conditions a value is valid under (the printed form, else the symbol).
function fmtConditions(p: Parameter): string {
  return p.conditions
    .map((c) => c.raw || c.symbol)
    .filter(Boolean)
    .join(", ");
}

// fmtCitation renders the provenance back to the datasheet: doc ref + page + table/figure. WS9-034
// will turn this into a deep-link to the extraction page; for now it is the citation text.
function fmtCitation(p: Parameter): string {
  const pr = p.prov;
  if (!pr) return "";
  const bits: string[] = [];
  if (pr.docRef) bits.push(pr.docRef);
  if (pr.page) bits.push(`p${pr.page}`);
  if (pr.tableOrFigure) bits.push(pr.tableOrFigure);
  return bits.join(" ");
}

// PartsPanel lists every datasheet-backed component (ref-des + MPN + parameter count). Clicking the
// ref-des locates the component on the canvas (onLocate); the ▸ toggle expands the row into its
// parameter tree (name/symbol, value, limit-kind, conditions, citation). A design with no joined
// specs shows an empty-state hint.
function PartsPanel(props: { state: () => PartsState; onLocate: (refDes: string) => void }) {
  const [open, setOpen] = createSignal<Set<string>>(new Set());
  const toggle = (ref: string) =>
    setOpen((s) => {
      const n = new Set(s);
      if (n.has(ref)) n.delete(ref);
      else n.add(ref);
      return n;
    });
  return (
    <div class="parts">
      <Show
        when={props.state().parts.length > 0}
        fallback={<div class="parts-empty">No datasheet-backed parts in this design. Serve with --params to enable.</div>}
      >
        <For each={props.state().parts}>
          {(part) => (
            <div class="parts-row">
              <div class="parts-head">
                <button type="button" class="parts-toggle" onClick={() => toggle(part.refDes)} title="expand parameters">
                  {open().has(part.refDes) ? "▾" : "▸"}
                </button>
                <button type="button" class="parts-ref" onClick={() => props.onLocate(part.refDes)} title={`locate ${part.refDes}`}>
                  {part.refDes}
                </button>
                <span class="parts-mpn">{part.mpn}</span>
                <span class="parts-count">{part.spec?.parameters.length ?? 0} params</span>
              </div>
              <Show when={open().has(part.refDes)}>
                <ul class="parts-params">
                  <For each={part.spec?.parameters ?? []}>
                    {(p) => (
                      <li class="parts-param">
                        <span class="parts-pname">{p.name || p.symbol || "(unnamed)"}</span>
                        <span class="parts-pval">{fmtVal(p)}</span>
                        <Show when={p.limitKind}>
                          <span class={`parts-limit lk-${p.limitKind}`}>{LIMIT_LABEL[p.limitKind] ?? ""}</span>
                        </Show>
                        <Show when={fmtConditions(p)}>
                          <span class="parts-cond">@ {fmtConditions(p)}</span>
                        </Show>
                        <Show when={fmtCitation(p)}>
                          <span class="parts-cite">{fmtCitation(p)}</span>
                        </Show>
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
            </div>
          )}
        </For>
      </Show>
    </div>
  );
}

// partsPanelIsland mounts the panel and returns its command-down view (setState). onLocate is the
// intent up: the user clicked a component, so the presenter highlights it on the canvas. Same island
// shape as coveragePanelIsland / findingsPanelIsland.
export function partsPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onLocate: (refDes: string) => void },
): { island: SolidIsland; view: PartsView } {
  const [state, setState] = signalView<PartsState>(emptyParts());
  const island = new SolidIsland("parts", el, () => <PartsPanel state={state} onLocate={handlers.onLocate} />, eventBus);
  return { island, view: { setState } };
}
