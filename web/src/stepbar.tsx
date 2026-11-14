import { SolidIsland, signalView } from "@panyam/tsappkit-solid";

// StepBarState is what the canvas's step controls show. canBack and canForward say whether the
// selection trail goes further either way (agni issue 1588). canPrev and canNext say whether a sheet
// step goes anywhere, and place reads where the canvas is among the stops, such as "3 of 6" (agni
// issue 1589).
export interface StepBarState {
  canBack: boolean;
  canForward: boolean;
  canPrev: boolean;
  canNext: boolean;
  place: string;
}

// StepBarView receives StepBarState from the page.
export interface StepBarView {
  setState(s: StepBarState): void;
}

// STEP_KEYS names the keys the page binds to each step, for the controls' titles and the guide.
export const STEP_KEYS = { back: "[", forward: "]", prev: ",", next: "." } as const;

type Handlers = { onBack: () => void; onForward: () => void; onPrev: () => void; onNext: () => void };

// Every control is always drawn and disabled where it goes nowhere, so a reader finds each once
// (agni issue 348). Each names its action in data-action, which the cross-probe walk clicks by.
function StepBar(props: { state: () => StepBarState } & Handlers) {
  return (
    <div class="step-bar">
      <button
        type="button"
        class="mode-btn step-back"
        data-action="selection-back"
        disabled={!props.state().canBack}
        title={`step back to the previous selection (${STEP_KEYS.back})`}
        onClick={() => props.onBack()}
      >
        ← Back
      </button>
      <button
        type="button"
        class="mode-btn step-forward"
        data-action="selection-forward"
        disabled={!props.state().canForward}
        title={`step forward to the next selection (${STEP_KEYS.forward})`}
        onClick={() => props.onForward()}
      >
        Forward →
      </button>
      <button
        type="button"
        class="mode-btn step-prev-sheet"
        data-action="prev-sheet"
        disabled={!props.state().canPrev}
        title={`open the previous sheet that draws the selection, or the previous sheet with nothing selected (${STEP_KEYS.prev})`}
        onClick={() => props.onPrev()}
      >
        ‹
      </button>
      <span class="step-place" data-sheet-place>
        {props.state().place}
      </span>
      <button
        type="button"
        class="mode-btn step-next-sheet"
        data-action="next-sheet"
        disabled={!props.state().canNext}
        title={`open the next sheet that draws the selection, or the next sheet with nothing selected (${STEP_KEYS.next})`}
        onClick={() => props.onNext()}
      >
        ›
      </button>
    </div>
  );
}

// stepBarIsland mounts the canvas's step controls and returns the view the page pushes state to.
export function stepBarIsland(el: HTMLElement, handlers: Handlers): { island: SolidIsland; view: StepBarView } {
  const [state, setState] = signalView<StepBarState>({ canBack: false, canForward: false, canPrev: false, canNext: false, place: "" });
  const island = new SolidIsland("steps", el, () => <StepBar state={state} {...handlers} />, null);
  return { island, view: { setState } };
}
