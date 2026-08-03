import type { ExpectationCaption } from "./expectations.js";

// expectationCaptionStrip wraps the conformance caption element (WS9-045). It renders the sidecar's
// NON-anchored verdict — the set-equality counts and the fires:{} "nothing may fire" assertion — as a
// one-glance pass/fail strip over the canvas, and hides the element entirely when the caption is null
// (a real design has no sidecar, so the strip is zero dev-only chrome there). A null element is a
// no-op. The anchored assertions are the status-colored highlight overlay, not this strip.
export function expectationCaptionStrip(
  el: HTMLElement | null,
): (caption: ExpectationCaption | null) => void {
  return (caption) => {
    if (!el) return;
    if (!caption) {
      el.classList.remove("on", "pass", "fail");
      el.textContent = "";
      return;
    }
    const mark = caption.pass ? "✓" : "✗"; // ✓ / ✗
    let text: string;
    if (caption.silent) {
      // fires:{} — a passes-variant asserting nothing may fire on the whole sheet.
      text = `${mark} silent${caption.unexpected > 0 ? ` — ${caption.unexpected} unexpected` : ""}`;
    } else {
      const parts = [`${caption.matched}/${caption.expected} expected`];
      if (caption.missing.length > 0) parts.push(`${caption.missing.length} missing`);
      if (caption.unexpected > 0) parts.push(`${caption.unexpected} unexpected`);
      text = `${mark} ${parts.join(", ")}`;
    }
    el.textContent = `expectations ${text}`;
    el.classList.add("on");
    el.classList.toggle("pass", caption.pass);
    el.classList.toggle("fail", !caption.pass);
  };
}
