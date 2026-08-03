// The Compare affordance (WS9-005): a top-bar button that arms "pick file B". The currently
// open file is side A; while armed, the next file clicked in the tree becomes side B (the
// composition root routes that click to the DiffPresenter instead of the viewer). Escape or
// a second click cancels. Like the panels menu, this is shell chrome, not presenter state,
// so it stays a plain DOM widget.

export interface CompareControl {
  // setEnabled reflects whether a file is open to serve as side A.
  setEnabled(on: boolean): void;
  isArmed(): boolean;
  // disarm ends the armed state silently (no onArmChange echo) — for the host to call once
  // it has consumed the arm (B was picked) or wants to cancel programmatically.
  disarm(): void;
}

const IDLE_LABEL = "Compare…";
const ARMED_LABEL = "pick file B in the tree (Esc cancels)";

// compareButton renders the button into host and reports arm/cancel transitions the USER
// makes via onArmChange; host-driven disarm() is silent.
export function compareButton(host: HTMLElement, onArmChange: (armed: boolean) => void): CompareControl {
  const doc = host.ownerDocument;
  const btn = doc.createElement("button");
  btn.type = "button";
  btn.className = "mode-btn compare-btn";
  btn.textContent = IDLE_LABEL;
  btn.disabled = true;
  btn.title = "open a file first — it becomes side A";
  let armed = false;

  const render = (): void => {
    btn.classList.toggle("active", armed);
    btn.textContent = armed ? ARMED_LABEL : IDLE_LABEL;
  };
  const setArmed = (on: boolean, notify: boolean): void => {
    if (armed === on) return;
    armed = on;
    render();
    if (notify) onArmChange(armed);
  };

  btn.addEventListener("click", () => setArmed(!armed, true));
  doc.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && armed) setArmed(false, true);
  });
  host.appendChild(btn);

  return {
    setEnabled: (on) => {
      btn.disabled = !on;
      btn.title = on ? "compare the open file (A) with another file (B)" : "open a file first — it becomes side A";
      if (!on) setArmed(false, true);
    },
    isArmed: () => armed,
    disarm: () => setArmed(false, false),
  };
}
