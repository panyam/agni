// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { DEFAULT_HIGHLIGHT_STYLE, HIGHLIGHT_STYLE_KEY, highlightMenu, loadHighlightStyle, saveHighlightStyle, type HighlightStyle } from "./highlightstyle.js";

function memStorage(initial: Record<string, string> = {}): Pick<Storage, "getItem" | "setItem" | "removeItem"> {
  const data = new Map(Object.entries(initial));
  return {
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  };
}

describe("highlight style persistence", () => {
  it("returns null when nothing is saved (so the built-in look is kept)", () => {
    expect(loadHighlightStyle(memStorage())).toBeNull();
  });

  it("round-trips a style", () => {
    const storage = memStorage();
    const style: HighlightStyle = { color: "#00ff00", alpha: 0.6, scale: 1.5 };
    saveHighlightStyle(storage, style);
    expect(loadHighlightStyle(storage)).toEqual(style);
  });

  it("falls back to defaults for out-of-range or malformed fields", () => {
    const storage = memStorage({ [HIGHLIGHT_STYLE_KEY]: JSON.stringify({ color: "nope", alpha: 5, scale: -1 }) });
    expect(loadHighlightStyle(storage)).toEqual(DEFAULT_HIGHLIGHT_STYLE);
  });

  it("returns null for a corrupt save instead of throwing", () => {
    expect(loadHighlightStyle(memStorage({ [HIGHLIGHT_STYLE_KEY]: "{bad" }))).toBeNull();
  });

  it("swallows storage write failures", () => {
    const storage = {
      getItem: () => null,
      setItem: () => {
        throw new Error("quota");
      },
    };
    expect(() => saveHighlightStyle(storage, DEFAULT_HIGHLIGHT_STYLE)).not.toThrow();
  });
});

describe("highlightMenu", () => {
  it("emits the edited style on input and persists it", () => {
    const storage = memStorage();
    const host = document.createElement("div");
    document.body.appendChild(host);
    const seen: (HighlightStyle | undefined)[] = [];
    highlightMenu(host, storage, (s) => seen.push(s));

    const color = host.querySelector<HTMLInputElement>(".highlight-color")!;
    color.value = "#123456";
    color.dispatchEvent(new Event("input", { bubbles: true }));

    expect(seen[seen.length - 1]?.color).toBe("#123456");
    expect(loadHighlightStyle(storage)?.color).toBe("#123456");
    document.body.replaceChildren();
  });

  it("Reset clears the saved style and emits undefined (built-in look)", () => {
    const storage = memStorage({ [HIGHLIGHT_STYLE_KEY]: JSON.stringify({ color: "#123456", alpha: 0.5, scale: 2 }) });
    const host = document.createElement("div");
    document.body.appendChild(host);
    const seen: (HighlightStyle | undefined)[] = [];
    highlightMenu(host, storage, (s) => seen.push(s));

    host.querySelector<HTMLButtonElement>(".highlight-reset")!.click();

    expect(seen[seen.length - 1]).toBeUndefined();
    expect(loadHighlightStyle(storage)).toBeNull();
    document.body.replaceChildren();
  });
});
