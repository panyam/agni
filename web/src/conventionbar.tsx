import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type ConventionState,
  type ConventionBarView,
  activeLabel,
  conventionError,
  emptyConvention,
  isOverridden,
  SERVER_DEFAULT_LABEL,
} from "./conventions.js";

// ConventionBar is the top-bar control that chooses the naming vocabulary a run is answered under,
// and says which one is in effect.
//
// A request convention REPLACES the server's rather than adding to it, so choosing one can stop the
// deployment's rules running. A rule that stops running produces no findings, which in a findings
// list looks the same as a design that got better. The bar takes a distinct style while a request
// convention is applied so that state stays visible on screen.
function ConventionBar(props: { state: () => ConventionState; onSelect: (ref: string) => void }) {
  return (
    <div class={`convbar${isOverridden(props.state()) ? " convbar-overridden" : ""}`}>
      <span class="convbar-label" title="the naming vocabulary these answers were computed under">
        vocabulary
      </span>
      <select
        class="convbar-pick"
        value={props.state().active}
        disabled={props.state().busy}
        onChange={(e) => props.onSelect(e.currentTarget.value)}
      >
        <option value="">{SERVER_DEFAULT_LABEL}</option>
        <For each={props.state().choices}>{(c) => <option value={c.ref}>{c.label}</option>}</For>
      </select>
      <span class="convbar-active">{activeLabel(props.state())}</span>
      <Show when={props.state().error}>
        <span class="convbar-error" role="alert" title={props.state().error}>
          {conventionError(props.state().error)}
        </span>
      </Show>
    </div>
  );
}

// conventionBarIsland mounts the bar and returns its command-down view. onSelect carries the intent
// up, with the convention ref the user chose or "" to go back to the server's.
export function conventionBarIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelect: (ref: string) => void },
): { island: SolidIsland; view: ConventionBarView } {
  const [state, setState] = signalView<ConventionState>(emptyConvention());
  const island = new SolidIsland(
    "conventionbar",
    el,
    () => <ConventionBar state={state} onSelect={handlers.onSelect} />,
    eventBus,
  );
  return { island, view: { setState } };
}
