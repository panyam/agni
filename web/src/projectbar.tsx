import { Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type ProjectState,
  type ProjectBarView,
  canGoPlain,
  emptyProject,
  entryNotice,
  isOverridden,
  projectLabel,
  PLAIN_LABEL,
} from "./project.js";

// ProjectBar is the top-bar strip that states which project the open design resolved to, and lets a
// reader take that project's config back off. Why it exists is on project.ts.
//
// The toggle answers "are these findings yours or the engine's" by subtraction, re-running the design
// under the built-in catalog. It is hidden for a design with no project (canGoPlain).
function ProjectBar(props: { state: () => ProjectState; onPlain: (plain: boolean) => void }) {
  return (
    <div class={`projbar${isOverridden(props.state()) ? " projbar-overridden" : ""}`}>
      <span class="projbar-label" title="the project whose config produced the answers on screen">
        project
      </span>
      <span class="projbar-name">{projectLabel(props.state())}</span>
      {/* The control is labelled for the ACTION and the name beside it states the RESULT, so the two
          do not render the same string twice when the box is ticked. */}
      <Show when={canGoPlain(props.state())}>
        <label class="projbar-plain" title={`re-run this design under the ${PLAIN_LABEL}, ignoring its project's config`}>
          <input
            type="checkbox"
            class="projbar-plain-box"
            checked={props.state().plain}
            disabled={props.state().busy}
            onChange={(e) => props.onPlain(e.currentTarget.checked)}
          />
          built-in only
        </label>
      </Show>
      {/* The companion notice says analysis reads a different file than the one picked in the tree
          (see entryNotice in project.ts). */}
      <Show when={entryNotice(props.state())}>
        <span class="projbar-entry">{entryNotice(props.state())}</span>
      </Show>
      <Show when={props.state().error}>
        <span class="projbar-error" role="alert" title={props.state().error}>
          {props.state().error}
        </span>
      </Show>
    </div>
  );
}

// projectBarIsland mounts the bar and returns its command-down view. onPlain is the intent up, fired
// when the reader asks for the built-in catalog or for the project back.
export function projectBarIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onPlain: (plain: boolean) => void },
): { island: SolidIsland; view: ProjectBarView } {
  const [state, setState] = signalView<ProjectState>(emptyProject());
  const island = new SolidIsland(
    "projectbar",
    el,
    () => <ProjectBar state={state} onPlain={handlers.onPlain} />,
    eventBus,
  );
  return { island, view: { setState } };
}
