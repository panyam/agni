import { describe, expect, it } from "vitest";
import { dropOf, memoryDropStore } from "./drops.js";

const bytes = (s: string) => new TextEncoder().encode(s);

// agni issue 1072: a drop is kept by id with every file under it, its design.yaml included, so the
// viewer reads after a navigation what the drop page proposed from.
describe("kept drops", () => {
  it("names the drop a browser-mount path belongs to, overlay included", () => {
    expect(dropOf("drop-abc/gateway/gateway.edn")).toBe("drop-abc");
    expect(dropOf(".agni-overlay/drop-abc/gateway/design.yaml")).toBe("drop-abc");
    expect(dropOf("other/file")).toBe("");
  });

  it("keeps a drop's files, adds its design.yaml, lists newest first and deletes", async () => {
    let t = 1000;
    const s = memoryDropStore(() => t);
    await s.save("drop-a", "gateway", { "drop-a/g.edn": bytes("ab") });
    t = 2000;
    await s.save("drop-b", "2 files", { "drop-b/x.edn": bytes("x"), "drop-b/x.eds": bytes("y") });
    expect((await s.list()).map((d) => d.id)).toEqual(["drop-b", "drop-a"]);
    const before = (await s.get("drop-a"))?.updatedAt ?? 0;
    await s.put("drop-a", { ".agni-overlay/drop-a/design.yaml": bytes("entry: g.edn") });
    const a = await s.get("drop-a");
    expect(a?.files).toBe(2);
    expect(a?.updatedAt).toBeGreaterThan(before); // what a record of its load is stamped with
    expect(Object.keys(await s.files("drop-a")).sort()).toEqual([".agni-overlay/drop-a/design.yaml", "drop-a/g.edn"]);
    await s.remove("drop-a");
    expect(await s.get("drop-a")).toBeUndefined();
    expect(await s.files("drop-a")).toEqual({});
    await expect(s.put("drop-a", {})).rejects.toThrow();
  });
});
