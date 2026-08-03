// The user-tunable focus-highlighter style (WS9-044): the color, opacity, and width-scale
// applied to a focused net's PATH marker (WS9-040/043). It is dock chrome, persisted per-browser
// in localStorage, NOT presenter state. The presenter takes a style and stamps it onto the focus
// specs; an unset style (no saved value) leaves the built-in look untouched, so an untouched
// viewer renders exactly as before.
import { DEFAULT_HIGHLIGHT_COLOR } from "./highlights.js";

// HighlightStyle carries the three tunable properties of the focus highlighter. alpha is the
// PATH marker opacity in (0,1]; scale multiplies its width (1 = the WS9-043 default).
export interface HighlightStyle {
  color: string; // "#rrggbb"
  alpha: number; // (0,1]
  scale: number; // width multiplier, > 0
}

// DEFAULT_HIGHLIGHT_STYLE reproduces the built-in focus highlighter: the default magenta at the
// PATH translucent alpha and unit width. Passing it is equivalent to passing no style.
export const DEFAULT_HIGHLIGHT_STYLE: HighlightStyle = { color: DEFAULT_HIGHLIGHT_COLOR, alpha: 0.4, scale: 1 };

export const HIGHLIGHT_STYLE_KEY = "agni-highlight-style";

type StyleStorage = Pick<Storage, "getItem" | "setItem">;
type MenuStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

// loadHighlightStyle reads the saved style, or null when nothing is saved (so the caller can
// keep the built-in look rather than forcing the defaults onto every spec). Out-of-range fields
// fall back to their defaults, so a corrupt or partial save never yields an invisible highlight.
export function loadHighlightStyle(storage: StyleStorage): HighlightStyle | null {
  const raw = storage.getItem(HIGHLIGHT_STYLE_KEY);
  if (!raw) return null;
  try {
    const p = JSON.parse(raw) as Partial<HighlightStyle>;
    return {
      color: typeof p.color === "string" && /^#[0-9a-fA-F]{6}$/.test(p.color) ? p.color : DEFAULT_HIGHLIGHT_STYLE.color,
      alpha: typeof p.alpha === "number" && p.alpha > 0 && p.alpha <= 1 ? p.alpha : DEFAULT_HIGHLIGHT_STYLE.alpha,
      scale: typeof p.scale === "number" && p.scale > 0 ? p.scale : DEFAULT_HIGHLIGHT_STYLE.scale,
    };
  } catch {
    return null;
  }
}

// saveHighlightStyle persists the style; a storage failure (quota, private mode) must not take
// the viewer down, so it is swallowed.
export function saveHighlightStyle(storage: StyleStorage, style: HighlightStyle): void {
  try {
    storage.setItem(HIGHLIGHT_STYLE_KEY, JSON.stringify(style));
  } catch {
    // best-effort persistence
  }
}

// highlightMenu renders the "Highlight" dropdown in the top bar (WS9-044): a color picker plus
// opacity and width sliders for the focus marker. Editing a control persists the style and calls
// onChange with it; Reset clears the saved style and calls onChange(undefined) so the presenter
// restores the built-in look. Like the Panels menu, this is dock chrome (a plain DOM widget), not
// presenter state.
export function highlightMenu(host: HTMLElement, storage: MenuStorage, onChange: (style: HighlightStyle | undefined) => void): void {
  const doc = host.ownerDocument;
  const style = loadHighlightStyle(storage) ?? { ...DEFAULT_HIGHLIGHT_STYLE };
  const btn = doc.createElement("button");
  btn.type = "button";
  btn.className = "mode-btn highlight-btn";
  btn.textContent = "Highlight ▾";
  const panel = doc.createElement("div");
  panel.className = "panels-list highlight-panel";
  panel.style.display = "none";

  const color = doc.createElement("input");
  color.type = "color";
  color.value = style.color;
  color.className = "highlight-color";
  const opacity = slider(doc, "0.1", "1", "0.05", style.alpha, "highlight-opacity");
  const width = slider(doc, "0.5", "3", "0.1", style.scale, "highlight-width");
  const reset = doc.createElement("button");
  reset.type = "button";
  reset.className = "mode-btn highlight-reset";
  reset.textContent = "Reset";

  const apply = (): void => {
    const next: HighlightStyle = { color: color.value, alpha: parseFloat(opacity.value), scale: parseFloat(width.value) };
    saveHighlightStyle(storage, next);
    onChange(next);
  };
  for (const el of [color, opacity, width]) el.addEventListener("input", apply);
  reset.addEventListener("click", () => {
    storage.removeItem(HIGHLIGHT_STYLE_KEY);
    color.value = DEFAULT_HIGHLIGHT_STYLE.color;
    opacity.value = String(DEFAULT_HIGHLIGHT_STYLE.alpha);
    width.value = String(DEFAULT_HIGHLIGHT_STYLE.scale);
    onChange(undefined);
  });

  panel.append(labeledRow(doc, "Color", color), labeledRow(doc, "Opacity", opacity), labeledRow(doc, "Width", width), reset);
  btn.addEventListener("click", () => {
    panel.style.display = panel.style.display === "none" ? "block" : "none";
  });
  doc.addEventListener("click", (e) => {
    if (!host.contains(e.target as Node)) panel.style.display = "none";
  });
  host.append(btn, panel);
}

function slider(doc: Document, min: string, max: string, step: string, value: number, cls: string): HTMLInputElement {
  const s = doc.createElement("input");
  s.type = "range";
  s.min = min;
  s.max = max;
  s.step = step;
  s.value = String(value);
  s.className = cls;
  return s;
}

function labeledRow(doc: Document, label: string, control: HTMLElement): HTMLElement {
  const row = doc.createElement("label");
  row.className = "highlight-row";
  const span = doc.createElement("span");
  span.textContent = label;
  row.append(span, control);
  return row;
}
