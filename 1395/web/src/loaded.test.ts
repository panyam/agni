import { describe, expect, it } from "vitest";
import { MAX_RECORDS, filesStamp, recordLoaded, wasLoaded, type LoadedKey } from "./loaded.js";

function memoryStorage(): Storage {
  const m = new Map<string, string>();
  return {
    get length() {
      return m.size;
    },
    clear: () => m.clear(),
    getItem: (k) => m.get(k) ?? null,
    key: (i) => [...m.keys()][i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, v),
  };
}

const key: LoadedKey = { engine: "wasm", build: "v1", uri: "mount://m/b.edn" };

describe("a record of what the Load page read (agni issue 1072)", () => {
  it("matches the same files, engine and build, and nothing else", () => {
    const s = memoryStorage();
    const stamp = filesStamp([{ path: "b.edn", sha256: "sha256:1" }, { path: "a.eds", sha256: "sha256:2" }]);
    expect(wasLoaded(s, key, stamp)).toBe(false);
    recordLoaded(s, key, stamp);
    expect(wasLoaded(s, key, stamp)).toBe(true);
    // The listing's order does not matter; a file's content does.
    expect(wasLoaded(s, key, filesStamp([{ path: "a.eds", sha256: "sha256:2" }, { path: "b.edn", sha256: "sha256:1" }]))).toBe(true);
    expect(wasLoaded(s, key, filesStamp([{ path: "b.edn", sha256: "sha256:9" }, { path: "a.eds", sha256: "sha256:2" }]))).toBe(false);
    expect(wasLoaded(s, { ...key, build: "v2" }, stamp)).toBe(false);
    expect(wasLoaded(s, { ...key, engine: "server" }, stamp)).toBe(false);
    expect(wasLoaded(s, { ...key, uri: "mount://m/c.edn" }, stamp)).toBe(false);
  });

  it("keeps the newest records and drops the oldest past the limit", () => {
    const s = memoryStorage();
    for (let i = 0; i <= MAX_RECORDS; i++) recordLoaded(s, { ...key, uri: `mount://m/${i}.edn` }, "x");
    expect(wasLoaded(s, { ...key, uri: "mount://m/0.edn" }, "x")).toBe(false);
    expect(wasLoaded(s, { ...key, uri: `mount://m/${MAX_RECORDS}.edn` }, "x")).toBe(true);
  });

  it("survives a storage that refuses or holds garbage", () => {
    const refusing = { ...memoryStorage(), setItem: () => { throw new Error("quota"); }, getItem: () => { throw new Error("blocked"); } } as Storage;
    expect(() => recordLoaded(refusing, key, "x")).not.toThrow();
    expect(wasLoaded(refusing, key, "x")).toBe(false);
    expect(wasLoaded(undefined, key, "x")).toBe(false);
    const garbage = memoryStorage();
    garbage.setItem("agni.loaded", "{not json");
    expect(wasLoaded(garbage, key, "x")).toBe(false);
    recordLoaded(garbage, key, "x");
    expect(wasLoaded(garbage, key, "x")).toBe(true);
  });
});
