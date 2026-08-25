import { describe, it, expect } from "vitest";
import { staleLinkNote, staleLinkStrip, shortHash } from "./stalelink.js";

const A = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const B = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";

describe("staleLinkNote", () => {
  it("says nothing when the link claimed no revision", () => {
    expect(staleLinkNote("", A)).toBeNull();
    expect(staleLinkNote("", "")).toBeNull();
  });

  it("says nothing when the claim held", () => {
    expect(staleLinkNote(A, A)).toBeNull();
  });

  it("reports a mismatch with both short digests", () => {
    const note = staleLinkNote(A, B);
    expect(note?.trust).toBe("mismatch");
    expect(note?.claimed).toBe("aaaaaaaaaaaa");
    expect(note?.served).toBe("bbbbbbbbbbbb");
  });

  // The point of the whole field. A server that could not hash must not be read as agreement: that
  // is the "absent looks like fine" failure the hash exists to remove, and folding this into null
  // would reintroduce it one layer down.
  it("keeps an unhashable server distinct from a match", () => {
    const note = staleLinkNote(A, "");
    expect(note).not.toBeNull();
    expect(note?.trust).toBe("unverifiable");
    expect(note?.served).toBe("");
  });
});

describe("shortHash", () => {
  it("trims the algorithm prefix and keeps twelve hex characters", () => {
    expect(shortHash(A)).toBe("aaaaaaaaaaaa");
  });

  it("passes through a value carrying no prefix", () => {
    expect(shortHash("deadbeefcafebabe")).toBe("deadbeefcafe");
  });
});

describe("staleLinkStrip", () => {
  it("is a no-op on a null element", () => {
    expect(() => staleLinkStrip(null)(staleLinkNote(A, B))).not.toThrow();
  });

  it("hides the element and empties it when there is nothing to say", () => {
    const el = document.createElement("div");
    const set = staleLinkStrip(el);
    set(staleLinkNote(A, B));
    expect(el.classList.contains("on")).toBe(true);
    set(null);
    expect(el.classList.contains("on")).toBe(false);
    expect(el.textContent).toBe("");
  });

  it("marks a mismatch warn and an unverifiable link not-warn", () => {
    const el = document.createElement("div");
    const set = staleLinkStrip(el);
    set(staleLinkNote(A, B));
    expect(el.classList.contains("warn")).toBe(true);
    expect(el.textContent).toContain("different revision");
    // Set again from the other state: the warn class must come OFF, not accumulate, or a link that
    // merely could not be checked would keep reading as a confirmed mismatch.
    set(staleLinkNote(A, ""));
    expect(el.classList.contains("on")).toBe(true);
    expect(el.classList.contains("warn")).toBe(false);
    expect(el.textContent).toContain("could not hash");
  });
});
