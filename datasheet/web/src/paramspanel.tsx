import { For, Show } from "solid-js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { Parameter } from "./gen/agni/v1/param/param_pb.js";
import { REGION_ATTR } from "./bank.js";

// ParamsView is the handle the workbench pushes its current parameter list to (on load and on every
// edit), so the panel stays in sync without the two islands sharing a store.
export interface ParamsView {
  setState: (p: Parameter[]) => void;
}

// fmtVal renders a parameter's min/typ/max plus unit for the list row.
function fmtVal(p: Parameter): string {
  const v = p.value;
  if (!v) return "";
  const parts: string[] = [];
  if (v.min !== undefined) parts.push(`min ${v.min}`);
  if (v.typ !== undefined) parts.push(`typ ${v.typ}`);
  if (v.max !== undefined) parts.push(`max ${v.max}`);
  return parts.join(" / ") + (p.unit ? ` ${p.unit}` : "");
}

function ParamsList(props: { params: () => Parameter[]; onLocate: (page: number, regionId: string) => void }) {
  return (
    <Show when={props.params().length} fallback={<div class="ps-empty">No parameters extracted yet.</div>}>
      <ul class="ps-list">
        <For each={props.params()}>
          {(p) => (
            <li>
              <button class="ps-row" onClick={() => props.onLocate(p.prov?.page ?? 1, p.attributes[REGION_ATTR] ?? "")}>
                <span class="ps-name">{p.name || p.symbol || "(unnamed)"}</span>
                <span class="ps-val">{fmtVal(p)}</span>
                <span class="ps-page">p{p.prov?.page ?? "?"}</span>
              </button>
            </li>
          )}
        </For>
      </ul>
    </Show>
  );
}

// paramsPanelIsland mounts the extracted-parameters list. onLocate is called with a row's page and
// region id when the user clicks it, so the workbench can jump there and select the region.
export function paramsPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  onLocate: (page: number, regionId: string) => void,
): { island: SolidIsland; view: ParamsView } {
  const [params, setParams] = signalView<Parameter[]>([]);
  const island = new SolidIsland("ds-params", el, () => <ParamsList params={params} onLocate={onLocate} />, eventBus);
  return { island, view: { setState: setParams } };
}
