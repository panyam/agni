// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { DEFAULT_HIGHLIGHT_STYLE, HIGHLIGHT_STYLE_KEY, SHOW_ALL_KEY, highlightMenu, loadHighlightStyle, loadShowAll, saveHighlightStyle, type HighlightStyle } from "./highlightstyle.js";

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

// agni issue 356: whether a run lights every finding at once is the reader's choice, off by default.
describe("Show every finding", () => {
  const mountMenu = (storage: ReturnType<typeof memStorage>, onShowAll?: (on: boolean) => void) => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    highlightMenu(host, storage, () => {}, onShowAll);
    return host;
  };

  it("is off unless saved on, and off when the store cannot be read", () => {
    expect(loadShowAll(memStorage())).toBe(false);
    expect(loadShowAll(memStorage({ [SHOW_ALL_KEY]: "1" }))).toBe(true);
    expect(loadShowAll({ getItem: () => { throw new Error("blocked"); } })).toBe(false);
  });

  it("reports and saves the reader's choice, and shows it when the menu mounts again", () => {
    const storage = memStorage();
    const onShowAll = vi.fn();
    const box = mountMenu(storage, onShowAll).querySelector<HTMLInputElement>(".highlight-show-all")!;
    expect(box.checked).toBe(false);
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    expect(onShowAll).toHaveBeenCalledWith(true);
    expect(mountMenu(storage, vi.fn()).querySelector<HTMLInputElement>(".highlight-show-all")!.checked).toBe(true);
  });

  it("offers no toggle to a host that does not handle it", () => {
    expect(mountMenu(memStorage()).querySelector(".highlight-show-all")).toBeNull();
  });
});
