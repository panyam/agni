// The Compare affordance (WS9-005, reshaped by WS9-049 phase 3): a top-bar button that opens the
// compare picker. It used to ARM a mode — the next file clicked in the Files tree became side B —
// which made the interaction depend on invisible state and on a dock panel being open. Now the
// button just asks for the picker; choosing there is the whole interaction, so there is no armed
// state to enter, echo, cancel with Escape, or leak when the open design changes.
//
// Like the panels menu, this is shell chrome rather than presenter state, so it stays a plain DOM
// widget.

export interface CompareControl {
  // setEnabled reflects whether a design is open to compare AGAINST. With none open there is
  // nothing to be the other side of a comparison, so the button is disabled rather than opening a
  // picker whose choice could not be acted on.
  setEnabled(on: boolean): void;
}

const LABEL = "Compare…";
const ENABLED_TITLE = "compare the open design against another";
const DISABLED_TITLE = "open a design first";

// compareButton renders the button into host and calls onOpen when the user asks to compare.
export function compareButton(host: HTMLElement, onOpen: () => void): CompareControl {
  const doc = host.ownerDocument;
  const btn = doc.createElement("button");
  btn.type = "button";
  btn.className = "mode-btn compare-btn";
  btn.textContent = LABEL;
  btn.disabled = true;
  btn.title = DISABLED_TITLE;

  btn.addEventListener("click", () => onOpen());
  host.appendChild(btn);

  return {
    setEnabled: (on) => {
      btn.disabled = !on;
      btn.title = on ? ENABLED_TITLE : DISABLED_TITLE;
    },
  };
}
