// Show-delay for the render loader (WS7-043), so a fast local render does not flash the
// full-screen loader on and off.

// RENDER_BUSY_DELAY_MS is just above the ~100ms flicker-perception threshold. 50ms still flashes
// for medium-fast renders, and much longer feels unresponsive on real loads.
export const RENDER_BUSY_DELAY_MS = 120;

// Busy is the loader's state. true blocks the page behind the full-screen overlay, after a show
// delay. "soft" shows the label in a small pill that takes no input, for work that goes on after
// the sheet is on screen (agni issue 1454). false hides both.
export type Busy = boolean | "soft";

// delayedBusy wraps a busy-overlay element with a show-delay. busy(true) adds the "on" class only
// after delayMs of continuous busy, and busy(false) cancels a pending show and hides immediately.
// busy("soft") cancels a pending show and swaps the overlay for the "soft" pill at once, since the
// reader can already see the sheet behind it. If a label is given, it is written to the
// ".render-busy-label" child at once, even before the overlay shows. Repeated true calls keep the
// one pending timer and a single false clears it. That relies on the presenter's setBusy collapsing
// nested renders first. A null element is a no-op.
// The element's data-busy reads "on" from the first busy call until busy(false), with no delay and
// in either form. A browser spec waits on it before acting, since its delay leaves no other way to
// tell busy from done (agni issue 1336).
export function delayedBusy(
  el: HTMLElement | null,
  delayMs = RENDER_BUSY_DELAY_MS,
): (busy: Busy, label?: string) => void {
  const labelEl = el?.querySelector<HTMLElement>(".render-busy-label") ?? null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const cancel = () => {
    if (timer !== undefined) {
      clearTimeout(timer);
      timer = undefined;
    }
  };
  return (busy: Busy, label?: string) => {
    if (label !== undefined && labelEl) labelEl.textContent = label;
    if (!el) return;
    if (busy === false) {
      cancel();
      el.classList.remove("on", "soft");
      delete el.dataset.busy;
      return;
    }
    el.dataset.busy = "on";
    if (busy === "soft") {
      cancel();
      el.classList.remove("on");
      el.classList.add("soft");
      return;
    }
    el.classList.remove("soft");
    if (timer === undefined && !el.classList.contains("on")) timer = setTimeout(() => el.classList.add("on"), delayMs);
  };
}
