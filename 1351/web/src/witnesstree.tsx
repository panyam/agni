import { For, Show } from "solid-js";
import type { WitnessItem, WitnessValue } from "./query.js";
import { SheetBadges } from "./sheetbadges.jsx";

// WitnessTree draws how a row holds (agni issue 794): each fact on a line, the rule that derived a
// derived fact under it, a leaf's citations, and each body fact one level in, in the rule's written
// order. A value naming an entity is a locate button, as a cell naming it is. A negated leaf is a
// `not` that held, an absence, and says so.
export function WitnessTree(props: {
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
