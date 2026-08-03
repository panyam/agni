import { describe, it, expect } from "vitest";
import { renderMarkdown } from "./markdown.js";

describe("renderMarkdown", () => {
  it("returns empty for empty input", () => {
    expect(renderMarkdown("")).toBe("");
  });

  it("renders prose to HTML", () => {
    expect(renderMarkdown("a **bold** word")).toContain("<strong>bold</strong>");
  });

  it("resolves a relative rule-doc image ref to the rule-doc route (WS9-030)", () => {
    const html = renderMarkdown("![reach cases](reach-cases.png)");
    expect(html).toContain('src="/rule-docs/reach-cases.png"');
    expect(html).toContain('alt="reach cases"');
  });

  it("leaves an absolute or external image src untouched", () => {
    expect(renderMarkdown("![x](https://example.com/a.png)")).toContain('src="https://example.com/a.png"');
    expect(renderMarkdown("![x](/already/absolute.png)")).toContain('src="/already/absolute.png"');
    expect(renderMarkdown("![x](data:image/png;base64,AAAA)")).toContain('src="data:image/png;base64,AAAA"');
  });
});
