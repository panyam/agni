// The Compare affordance (WS9-005) is a top-bar button that opens the compare picker. It arms no
// mode, so there is no hidden state to echo, cancel with Escape, or leak when the open design
// changes (WS9-049 phase 3).
//
// Like the panels menu, this is shell chrome rather than presenter state, so it stays a plain DOM
// widget.

export interface CompareControl {
  // setEnabled reflects whether a design is open to compare AGAINST. With none open the button is
  // disabled, since a choice in the picker would have nothing to compare to.
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
