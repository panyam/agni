import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type TraceState,
  type TraceView,
  type TraceEndItem,
  TraceOutcome,
  emptyTrace,
} from "./trace.js";

// endLabel is "U1.3 (SDA)", falling back to the bare pin where the source carries no pin names.
function endLabel(e: TraceEndItem): string {
  const pin = `${e.refDes}.${e.pin}`;
  return e.pinName ? `${pin} (${e.pinName})` : pin;
}

// TracePanel asks for two pins and shows the answer: the route as one line, then each net with what
// else sits on it, then what the answer rests on.
//
// The three outcomes render as three different things rather than as one message with a status word,
// because they are three different kinds of statement. A route is a fact about the design. A
// no-route is also a fact about the design and says which nets it is about. An unresolved endpoint is
// a failed QUESTION, so it says which name failed and does not mention connectivity at all: a reader
// who saw "not connected" there would go and look at the board instead of at what they typed.
function TracePanel(props: {
  state: () => TraceState;
  onTrace: (from: string, to: string) => void;
}) {
  let fromEl: HTMLInputElement | undefined;
  let toEl: HTMLInputElement | undefined;
  const run = () => props.onTrace(fromEl?.value.trim() ?? "", toEl?.value.trim() ?? "");
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") run();
  };
  const s = () => props.state();
  return (
    <div class="trace">
      <div class="trace-ask">
        <input
          ref={fromEl}
          class="trace-pin"
          type="text"
          placeholder="from, e.g. U1.3"
          aria-label="Start pin"
          onKeyDown={onKey}
        />
        <input
          ref={toEl}
          class="trace-pin"
          type="text"
          placeholder="to, e.g. J1.1"
          aria-label="End pin"
          onKeyDown={onKey}
        />
        <button type="button" class="trace-run" disabled={s().loading} onClick={run}>
          {s().loading ? "Tracing…" : "Trace"}
        </button>
      </div>

      <Show when={s().error}>
        <div class="trace-error">{s().error}</div>
      </Show>

      <Show when={s().ran && !s().loading && !s().error}>
        <Show when={s().outcome === TraceOutcome.UNRESOLVED}>
          <div class="trace-unresolved">
            <div class="trace-headline">Cannot trace</div>
            <div class="trace-reason">{s().reason}</div>
            <div class="trace-note">
              Nothing was walked. This is a failed question rather than a disconnection.
            </div>
          </div>
        </Show>

        <Show when={s().outcome === TraceOutcome.NO_ROUTE}>
          <div class="trace-noroute">
            <div class="trace-headline">
              No route: {endLabel(s().from)} to {endLabel(s().to)}
            </div>
            <div class="trace-reason">{s().reason}</div>
            <div class="trace-note">
              The walk crosses resistors, inductors, ferrites and fuses. A capacitor is a DC block and
              is never crossed. A rail or plane may end a route and is never passed through.
            </div>
          </div>
        </Show>

        <Show when={s().outcome === TraceOutcome.ROUTED}>
          <div class="trace-headline">
            {[endLabel(s().from), ...s().crossings.map((c) => c.refDes), endLabel(s().to)].join(" → ")}
          </div>
          <ol class="trace-route">
            <For each={s().nets}>
              {(n, i) => (
                <>
                  <li class={`trace-net${n.busLike ? " bus-like" : ""}`}>
                    <span class="trace-net-name">{n.name}</span>
                    <Show when={n.busLike}>
                      <span class="trace-rail-note">rail or plane: a route may end here, never pass through</span>
                    </Show>
                    {/* The elided COUNT lives inside this list, so the gate has to admit a net
                        whose stubs were all dropped. The server caps at twelve and so cannot
                        produce that today, and a panel that renders nothing when it does would be
                        silently hiding the one thing the count exists to say. */}
                    <Show when={n.stubs.length > 0 || n.stubsElided > 0}>
                      <ul class="trace-stubs">
                        <For each={n.stubs}>
                          {(st) => (
                            <li class={`trace-stub${st.cls === "test_point" ? " probe" : ""}`}>
                              {st.refDes}.{st.pin}
                              <Show when={st.cls}>
                                <span class="trace-stub-class"> ({st.cls.replace(/_/g, " ")})</span>
                              </Show>
                            </li>
                          )}
                        </For>
                        <Show when={n.stubsElided > 0}>
                          <li class="trace-stub elided">and {n.stubsElided} more</li>
                        </Show>
                      </ul>
                    </Show>
                  </li>
                  <Show when={i() < s().crossings.length}>
                    <li class="trace-cross">
                      cross {s().crossings[i()].refDes}
                      <Show when={s().crossings[i()].cls}>
                        <span class="trace-stub-class"> ({s().crossings[i()].cls.replace(/_/g, " ")})</span>
                      </Show>
                      <span class="trace-cross-pins">
                        {" "}
                        pin {s().crossings[i()].enterPin} to pin {s().crossings[i()].exitPin}
                      </span>
                    </li>
                  </Show>
                </>
              )}
            </For>
          </ol>
          <div class="trace-summary">
            {s().crossings.length} crossing{s().crossings.length === 1 ? "" : "s"}, {s().nets.length} net
            {s().nets.length === 1 ? "" : "s"}, searched to a radius of {s().radius}
          </div>
        </Show>
      </Show>

      <Show when={!s().ran}>
        <div class="trace-empty">
          Name two pins to follow a signal through the series parts between them.
        </div>
      </Show>
    </div>
  );
}

// tracePanelIsland mounts the panel and returns its command-down view. onTrace is the intent up: the
// reader named two pins, so the presenter runs the walk and highlights what came back. Same island
// shape as coveragePanelIsland / findingsPanelIsland.
export function tracePanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onTrace: (from: string, to: string) => void },
): { island: SolidIsland; view: TraceView } {
  const [state, setState] = signalView<TraceState>(emptyTrace());
  const island = new SolidIsland("trace", el, () => <TracePanel state={state} onTrace={handlers.onTrace} />, eventBus);
  return { island, view: { setState } };
}
