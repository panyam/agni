import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";

import { NOT_COMPARED, type Agreement, type AgreementEntry } from "./agreement.js";

// AgreementBarState is what the canvas bar's agreement control shows (agni issue 1590). agreement
// is null until the engine has answered, and busy is set while it works. open says whether the
// panel is showing.
export interface AgreementBarState {
  agreement: Agreement | null;
  busy: boolean;
  open: boolean;
}

// AgreementBarView receives AgreementBarState from the page.
export interface AgreementBarView {
  setState(s: AgreementBarState): void;
}

type Handlers = { onToggle: () => void; onPick: (e: AgreementEntry) => void };

// label is the button's text. It names the comparison until the engine has answered, then reads
// its short count, or "not compared".
function label(s: AgreementBarState): string {
  if (s.agreement) return s.agreement.summary;
  return s.busy ? "comparing…" : "schematic ↔ board";
}

// The button names its action in data-action, which the cross-probe walk clicks by, and the panel
// carries data-agreement, which the walk reads.
function AgreementBar(props: { state: () => AgreementBarState } & Handlers) {
  const s = () => props.state();
  return (
    <div class="agreement-bar">
      <button
        type="button"
        class="mode-btn agreement-btn"
        classList={{ "agreement-off": s().agreement?.summary === NOT_COMPARED }}
        data-action="agreement"
        aria-expanded={s().open}
        title="how well the schematic and the board agree: parts by ref-des, nets by name or by identical pins"
        onClick={() => props.onToggle()}
      >
        {label(s())}
      </button>
      <div class="agreement-pop" data-agreement hidden={!s().open}>
        <Show when={s().agreement} fallback={<p class="agreement-line">{s().busy ? "Comparing the schematic with the board…" : ""}</p>}>
          {(a) => (
            <>
              <For each={a().lines}>{(line) => <p class="agreement-line">{line}</p>}</For>
              <Show when={a().compared && a().sections.length === 0}>
                <p class="agreement-line">Every part, pin and net on the schematic matches the board.</p>
              </Show>
              <For each={a().sections}>
                {(sec) => (
                  <section class="agreement-section">
                    <h4>
                      {sec.title} ({sec.entries.length})
                    </h4>
                    <ul>
                      <For each={sec.entries}>
                        {(e) => (
                          <li>
                            <button type="button" class="agreement-entry" data-agreement-view={e.view} onClick={() => props.onPick(e)}>
                              {e.label}
                            </button>
                          </li>
                        )}
                      </For>
                    </ul>
                  </section>
                )}
              </For>
            </>
          )}
        </Show>
      </div>
    </div>
  );
}

// agreementBarIsland mounts the agreement control and returns the view the page pushes state to.
export function agreementBarIsland(el: HTMLElement, handlers: Handlers): { island: SolidIsland; view: AgreementBarView } {
  const [state, setState] = signalView<AgreementBarState>({ agreement: null, busy: false, open: false });
  const island = new SolidIsland("agreement", el, () => <AgreementBar state={state} {...handlers} />, null);
  return { island, view: { setState } };
}
