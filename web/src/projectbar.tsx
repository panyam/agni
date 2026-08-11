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
// reader take that project's config back off.
//
// It sits beside the convention bar and does the same job one level up. The convention bar answers
// "under which vocabulary", this answers "under whose rules at all", and the second question only
// became askable once config started resolving from the design's own project rather than from server
// flags. A findings list cannot answer either one: a rule that never ran because a project did not
// ask for it leaves nothing behind, so it reads exactly like a rule that ran and found nothing.
//
// The toggle is the honest form of the answer. "Are these findings yours or the engine's" is settled
// by subtraction, so the control re-runs the design under the built-in catalog and what appears is
// the difference. It is hidden for a design with no project, because there is nothing to subtract and
// a control implying otherwise would suggest a difference that does not exist.
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
      {/* The companion notice is the whole reason the viewer resolves out loud instead of letting the
          server swap the entry in behind it: the user picked this file in a tree, and analysis reads a
          different one. Saying so leaves acting on it their move. */}
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

// projectBarIsland mounts the bar and returns its command-down view. onPlain is the intent up: the
// reader asked to see this design under the built-in catalog, or asked for its project back.
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
