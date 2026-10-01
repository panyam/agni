// Show-delay for the render loader (WS7-043), so a fast local render does not flash the
// full-screen loader on and off.

// RENDER_BUSY_DELAY_MS sits just above the ~100ms flicker-perception threshold. 50ms still flashes
// for medium-fast renders, and much longer feels unresponsive on real loads.
export const RENDER_BUSY_DELAY_MS = 120;

// delayedBusy wraps a busy-overlay element with a show-delay. busy(true) adds the "on" class only
// after delayMs of continuous busy, and busy(false) cancels a pending show and hides immediately.
// A label, when given, is written to the ".render-busy-label" child at once, even before the overlay
// shows. Repeated true calls keep the one pending timer and a single false clears it, which relies
// on the presenter's setBusy collapsing nested renders first. A null element is a no-op.
export function delayedBusy(
  el: HTMLElement | null,
  delayMs = RENDER_BUSY_DELAY_MS,
): (busy: boolean, label?: string) => void {
  const labelEl = el?.querySelector<HTMLElement>(".render-busy-label") ?? null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  return (busy: boolean, label?: string) => {
    if (label !== undefined && labelEl) labelEl.textContent = label;
    if (busy) {
      if (timer === undefined) timer = setTimeout(() => el?.classList.add("on"), delayMs);
    } else {
      if (timer !== undefined) {
        clearTimeout(timer);
        timer = undefined;
      }
      el?.classList.remove("on");
    }
  };
}
