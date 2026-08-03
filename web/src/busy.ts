// Show-delay for the render loader (WS7-043). The busy overlay tracks the actual render duration,
// so an instantaneous local render would flash the full-screen loader on and off. delayedBusy shows
// the overlay only if the busy state persists past a short window, so fast renders stay
// uninterrupted while a genuinely heavy load still shows the loader promptly.

// RENDER_BUSY_DELAY_MS is above the ~100ms flicker-perception threshold: shorter (e.g. 50ms) still
// flashes for medium-fast renders, longer starts to feel unresponsive on real loads.
export const RENDER_BUSY_DELAY_MS = 120;

// delayedBusy wraps a busy-overlay element with a show-delay. busy(true) adds the "on" class only
// after delayMs of continuous busy; busy(false) cancels a still-pending show and hides immediately.
// An optional label names the current phase; it updates the element's ".render-busy-label" child
// immediately (even before the overlay shows) so the loader says what it is doing, and is left
// unchanged when omitted. The presenter's own setBusy collapses nested renders to one net boolean
// before calling this, so repeated true calls keep the single pending timer (they do not restack
// it) and one false clears it. A null element is a no-op (the render overlay is nullable in the
// shell).
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
