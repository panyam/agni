// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { overlayPath, showDeclared } from "./declareboard.js";

beforeEach(() => document.body.replaceChildren());

describe("saving an attached board", () => {
  it("writes a drop's descriptor into the overlay, and nothing for a served mount", () => {
    expect(overlayPath("mount://local/drop-1/gw/design.yaml", "local")).toBe(".agni-overlay/drop-1/gw/design.yaml");
    expect(overlayPath("mount://common/odb/design.yaml", "local")).toBe("");
    expect(overlayPath("mount://localx/a/design.yaml", "local")).toBe("");
  });

  it("shows a served design's descriptor for the reader to save", async () => {
    const shown = showDeclared(document, { descriptorUri: "mount://common/odb/design.yaml", designYaml: "name: gateway\n", changed: true });
    expect((document.querySelector(".board-declared-yaml") as HTMLTextAreaElement).value).toBe("name: gateway\n");
    expect(document.querySelector(".board-declared .board-picker-where")?.textContent).toContain("common/odb/design.yaml");
    (document.querySelector(".board-declared-close") as HTMLButtonElement).click();
    await shown;
    expect(document.querySelector(".board-declared")).toBeNull();
  });

  it("says when the design declares the board already", () => {
    void showDeclared(document, { descriptorUri: "mount://common/odb/design.yaml", designYaml: "x\n", changed: false });
    expect(document.querySelector(".board-declared .board-picker-where")?.textContent).toBe("common/odb/design.yaml declares this board already.");
  });
});
