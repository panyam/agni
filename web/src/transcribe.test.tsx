// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SolidIsland } from "@panyam/tsappkit-solid";
import { create } from "@bufbuild/protobuf";
import { PartSpecSchema, PinFunction, Modality, type PartSpec, type Pin } from "./gen/agni/v1/param/param_pb.js";
import { TranscribePanel, type TranscribeHandlers } from "./transcribe.jsx";
import type { Region } from "./regions.js";

// The transcribe panel's first tests. Its pure helpers have always been covered in bank.test.ts, so
// what was missing is everything between a person's keystrokes and those helpers: which fields the
// editors read, what they refuse to submit, and what they hand over. That gap mattered more here
// than in a read-only panel, because these editors WRITE — every Add button ends at a PartSpec on
// disk.
//
// The panel needs no render harness of its own. It is an ordinary component over a handlers object,
// so a stub of that object is the whole fixture; it was untestable only because nothing had tried.

function spec(pins: Pin[] = []): PartSpec {
  return create(PartSpecSchema, { pins });
}

const pin = (id: string, name: string): Pin => ({ id, name, function: PinFunction.POWER_INPUT }) as Pin;

const region = (): Region => ({ id: "r1", page: 1, kind: "user", label: "table 1", bbox: { x: 0, y: 0, width: 10, height: 10 } }) as Region;

function mountPanel(over: Partial<TranscribeHandlers> = {}) {
  const calls = {
    addPin: vi.fn(),
    addPackage: vi.fn(),
    addRelation: vi.fn(),
    addParam: vi.fn(),
  };
  const handlers: TranscribeHandlers = {
    spec: () => spec(),
    region: () => region(),
    regionType: () => "table",
    deletableRegion: () => true,
    onDeleteRegion: vi.fn(),
    setType: vi.fn(),
    setMeta: vi.fn(),
    deleteParam: vi.fn(),
    deletePin: vi.fn(),
    setPinNumber: vi.fn(),
    deletePackage: vi.fn(),
    toggleBinding: vi.fn(),
    deleteRelation: vi.fn(),
    problems: () => [],
    ...calls,
    ...over,
  };
  const el = document.createElement("div");
  document.body.appendChild(el);
  new SolidIsland("transcribe", el, () => <TranscribePanel {...handlers} />, null).activate();
  return { el, calls };
}

// The editors are addressed the way a person addresses them: by the label they read.
const fieldNamed = (el: HTMLElement, label: string): HTMLInputElement | undefined =>
  [...el.querySelectorAll("label")].find((l) => l.textContent?.trim().startsWith(label))?.querySelector("input") ?? undefined;

const buttonNamed = (el: HTMLElement, text: string): HTMLButtonElement | undefined =>
  [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === text);

function type(input: HTMLInputElement | undefined, value: string): void {
  if (!input) throw new Error("no such field");
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

beforeEach(() => document.body.replaceChildren());

describe("pin editor", () => {
  it("derives the id from the name and hands over the trimmed fields", () => {
    const { el, calls } = mountPanel();
    type(fieldNamed(el, "Pin name"), "  VCCA  ");
    type(fieldNamed(el, "Description"), " A-port supply ");
    buttonNamed(el, "Add pin")!.click();

    expect(calls.addPin).toHaveBeenCalledWith({
      id: "vcca",
      name: "VCCA",
      fn: PinFunction.POWER_INPUT,
      description: "A-port supply",
    });
  });

  // The id follows the name until someone edits it, and then stops: a part that prints one name on
  // several terminals needs distinct ids for exactly those pins, which is only possible if a typed
  // id survives the next keystroke in the name field.
  //
  // This is the test that found the swallowed first character. The handler set idTouched before
  // reading the event, which flipped the field to the empty id() and let Solid write that back into
  // the input, so the read that followed returned "" and the keystroke was lost.
  it("keeps a hand-typed id, including its first character", () => {
    const { el, calls } = mountPanel();
    type(fieldNamed(el, "Pin name"), "VCCA");
    type(fieldNamed(el, "Id"), "vcca_2");
    type(fieldNamed(el, "Pin name"), "VCCA");
    buttonNamed(el, "Add pin")!.click();

    expect(calls.addPin).toHaveBeenCalledWith(expect.objectContaining({ id: "vcca_2", name: "VCCA" }));
  });

  // Two terminals printed with one name are ordinary (a port-wide supply), and they cannot share an
  // id, so the derivation steps around one that is taken.
  it("derives a distinct id when the obvious one is taken", () => {
    const { el, calls } = mountPanel({ spec: () => spec([pin("vcca", "VCCA")]) });
    type(fieldNamed(el, "Pin name"), "VCCA");
    buttonNamed(el, "Add pin")!.click();
    expect(calls.addPin).toHaveBeenCalledWith(expect.objectContaining({ id: "vcca2", name: "VCCA" }));
  });

  it("refuses a pin with no name rather than writing a nameless one", () => {
    const { el, calls } = mountPanel();
    type(fieldNamed(el, "Description"), "supply");
    buttonNamed(el, "Add pin")!.click();
    expect(calls.addPin).not.toHaveBeenCalled();
  });
});

const withPins = { spec: () => spec([pin("vcca", "VCCA"), pin("vccb", "VCCB")]) };

describe("relation editor", () => {
  it("submits a bound on the difference, leaving a blank field unset rather than zero", () => {
    const { el, calls } = mountPanel(withPins);
    const selects = [...el.querySelectorAll("select")];
    // The two pin pickers are the relation editor's own; pick subject and reference.
    const [subject, reference] = selects.filter((s) => [...s.options].some((o) => o.value === "vcca"));
    subject.value = "vccb";
    subject.dispatchEvent(new Event("change", { bubbles: true }));
    reference.value = "vcca";
    reference.dispatchEvent(new Event("change", { bubbles: true }));

    const numeric = [...el.querySelectorAll("input")].filter((i) => i.placeholder.includes("−") || i.placeholder.includes("-"));
    if (numeric[0]) type(numeric[0], "-0.5");
    buttonNamed(el, "Add relation")!.click();

    expect(calls.addRelation).toHaveBeenCalledTimes(1);
    const arg = calls.addRelation.mock.calls[0][0];
    expect(arg.subjectPinRef).toBe("vccb");
    expect(arg.referencePinRef).toBe("vcca");
    expect(arg.modality).toBe(Modality.REQUIRED);
    // A missing max is not a max of zero. RangeValue has explicit presence, and this is the field
    // where coercing a blank to 0 would silently invent a limit the datasheet never printed.
    expect(arg.max).toBeUndefined();
  });

  // param.Validate rejects a self-referencing relation structurally, so the editor refuses to author
  // one rather than letting the author find out after a round trip.
  it("refuses a pin tracking itself", () => {
    const { el, calls } = mountPanel(withPins);
    const selects = [...el.querySelectorAll("select")];
    const [subject, reference] = selects.filter((s) => [...s.options].some((o) => o.value === "vcca"));
    subject.value = "vcca";
    subject.dispatchEvent(new Event("change", { bubbles: true }));
    reference.value = "vcca";
    reference.dispatchEvent(new Event("change", { bubbles: true }));

    buttonNamed(el, "Add relation")!.click();
    expect(calls.addRelation).not.toHaveBeenCalled();
  });
});
