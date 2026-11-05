// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { pickBoard, parentOf, type PickerEntry } from "./boardpicker.js";

const tree: Record<string, PickerEntry[]> = {
  "mount://m/odb": [
    { name: "gateway-layout", uri: "mount://m/odb/gateway-layout", isDir: true, boardUri: "mount://m/odb/gateway-layout/matrix/matrix" },
    { name: "notes", uri: "mount://m/odb/notes", isDir: true, boardUri: "" },
    { name: "gateway.edn", uri: "mount://m/odb/gateway.edn", isDir: false, boardUri: "" },
    { name: "x.kicad_pcb", uri: "mount://m/odb/x.kicad_pcb", isDir: false, boardUri: "mount://m/odb/x.kicad_pcb" },
  ],
  "mount://m/odb/notes": [],
  "mount://m": [{ name: "odb", uri: "mount://m/odb", isDir: true, boardUri: "" }],
};
const list = async (uri: string) => tree[uri] ?? [];
const settle = () => new Promise((r) => setTimeout(r, 0));
const rows = () => [...document.querySelectorAll(".board-picker-row")].map((r) => r.textContent);

beforeEach(() => document.body.replaceChildren());

describe("the board picker", () => {
  it("lists folders and boards, not other files, and a board folder can be used", async () => {
    const picked = pickBoard(document, "mount://m/odb", list);
    await settle();
    expect(rows()).toEqual(["..", "gateway-layout/Use", "notes/", "x.kicad_pcbUse"]);
    (document.querySelector('[data-uri="mount://m/odb/gateway-layout"] .board-picker-use') as HTMLButtonElement).click();
    expect(await picked).toBe("mount://m/odb/gateway-layout/matrix/matrix");
    expect(document.querySelector(".board-picker")).toBeNull();
  });

  it("enters a folder and goes back up", async () => {
    const picked = pickBoard(document, "mount://m/odb", list);
    await settle();
    (document.querySelector('[data-uri="mount://m/odb/notes"] .board-picker-enter') as HTMLButtonElement).click();
    await settle();
    expect(document.querySelector(".board-picker-where")?.textContent).toBe("m/odb/notes");
    expect(document.querySelector(".board-picker-note")?.textContent).toBe("no boards or folders here");
    (document.querySelector(".board-picker-enter") as HTMLButtonElement).click();
    await settle();
    expect(document.querySelector(".board-picker-where")?.textContent).toBe("m/odb");
    (document.querySelector(".board-picker-cancel") as HTMLButtonElement).click();
    expect(await picked).toBeNull();
  });

  it("knows the folder above a URI, and none above a mount's root", () => {
    expect(parentOf("mount://m/odb/notes")).toBe("mount://m/odb");
    expect(parentOf("mount://m/odb")).toBe("mount://m");
    expect(parentOf("mount://m")).toBe("");
  });
});
