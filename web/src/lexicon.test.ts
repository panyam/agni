import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { ClassVocabSchema, NamingConventionSchema, NamingRuleSchema, type NamingConvention } from "./gen/agni/v1/config/naming_pb.js";
import { LexiconEntrySchema, LexiconSource, type LexiconEntry } from "./gen/agni/v1/webapi/lexicon_pb.js";
import { LexiconPresenter, addEntry, canRemove, emptyLexicon, removeEntry, rowsOf, setInference, setOff, subjectOf, type LexiconClient, type LexiconState } from "./lexicon.js";
import { TableSchema } from "./gen/agni/v1/webapi/tables_pb.js";

const entry = (vocabulary: string, kind: string, value: string, source = LexiconSource.BUILTIN, replaced = false): LexiconEntry =>
  create(LexiconEntrySchema, { vocabulary, kind, value, source, replaced });
const conv = () => create(NamingConventionSchema, { name: "house" });

describe("draft edits", () => {
  it("adds to a convention as a delta, so the file states one pattern", () => {
    const d = addEntry(conv(), "net.rail", "pattern", "^USBH_3V3$");
    expect(d.lexicon?.net?.rail?.patterns).toEqual(["^USBH_3V3$"]);
    expect(d.lexicon?.net?.rail?.replace).toBe(false);
    expect(conv().lexicon).toBeUndefined();
  });

  it("reaches a snake_case vocabulary and a class", () => {
    expect(addEntry(conv(), "net.gate_drive", "exclude", "x").lexicon?.net?.gateDrive?.exclude).toEqual(["x"]);
    expect(addEntry(conv(), "pin.exposed_pad", "pattern", "x").lexicon?.pin?.exposedPad?.patterns).toEqual(["x"]);
    expect(addEntry(conv(), "class.tvs", "family", "^acme").lexicon?.class.tvs.families).toEqual(["^acme"]);
    expect(addEntry(conv(), "class.tvs", "prefix", "TV").lexicon?.class.tvs.prefixes).toEqual(["TV"]);
  });

  it("removes a built-in by writing the vocabulary out with replace", () => {
    const rows = rowsOf([entry("net.i2c", "pattern", "SDA"), entry("net.i2c", "pattern", "SCL"), entry("net.i2c", "exclude", "I2S")], []);
    const d = removeEntry(conv(), rows, rows[0]);
    const v = d.lexicon?.net?.i2c;
    expect(v?.replace).toBe(true);
    expect(v?.patterns).toEqual(["SCL"]);
    expect(v?.exclude).toEqual(["I2S"]);
  });

  it("removes a convention's own entry from its list and leaves the built-ins alone", () => {
    const d = addEntry(conv(), "net.rail", "pattern", "^A$");
    const rows = rowsOf([entry("net.rail", "pattern", "^A$", LexiconSource.PROJECT)], []);
    const v = removeEntry(d, rows, rows[0]).lexicon?.net?.rail;
    expect(v?.patterns).toEqual([]);
    expect(v?.replace).toBe(false);
  });

  it("switches a vocabulary off and back on", () => {
    const off = setOff(addEntry(conv(), "net.control", "pattern", "x"), "net.control", true);
    expect(off.lexicon?.net?.control?.replace).toBe(true);
    expect(off.lexicon?.net?.control?.patterns).toEqual([]);
    expect(setOff(off, "net.control", false).lexicon?.net?.control).toBeUndefined();
  });

  it("keeps a class's prefixes when its patterns go off, since replace leaves them", () => {
    const d = conv();
    d.lexicon = { $typeName: "agni.v1.config.NamingLexicon", class: { tvs: create(ClassVocabSchema, { prefixes: ["TV"], patterns: ["x"] }) } };
    const v = setOff(d, "class.tvs", true).lexicon?.class.tvs;
    expect(v?.prefixes).toEqual(["TV"]);
    expect(v?.patterns).toEqual([]);
  });

  it("states the inference switch only when it is set", () => {
    expect(setInference(conv(), false).lexicon?.net?.inferRailFromSupplyPins).toBe(false);
    expect(conv().lexicon?.net?.inferRailFromSupplyPins).toBeUndefined();
  });

  it("offers no removal of a built-in prefix, a replaced entry or a switch", () => {
    const rows = rowsOf([entry("class.tvs", "prefix", "D"), entry("net.rail", "pattern", "x", LexiconSource.BUILTIN, true), entry("net", "switch", "a=true"), entry("net.rail", "pattern", "y")], []);
    expect(rows.map(canRemove)).toEqual([false, false, false, true]);
  });
});

describe("rowsOf", () => {
  it("keeps a row's source from the convention in effect and marks what the draft added", () => {
    const base = [entry("net.rail", "pattern", "^P$", LexiconSource.PROJECT)];
    const rows = rowsOf([entry("net.rail", "pattern", "^P$", LexiconSource.REQUEST), entry("net.rail", "pattern", "^N$", LexiconSource.REQUEST)], base);
    expect(rows.map((r) => [r.source, r.edited])).toEqual([["project", false], ["this edit", true]]);
  });
});

describe("subjectOf", () => {
  it("selects a role row's net and a finding row's subject", () => {
    const roles = create(TableSchema, { name: "role_changes", columns: [{ name: "net" }, { name: "roles_before" }, { name: "roles_after" }] });
    const findings = create(TableSchema, { name: "finding_changes", columns: [{ name: "change" }, { name: "rule" }, { name: "subject_kind" }, { name: "subject" }, { name: "message" }] });
    expect(subjectOf(roles, ["USBH_3V3", "", "rail"])).toEqual({ kind: "net", name: "USBH_3V3" });
    expect(subjectOf(findings, ["gone", "r", "component", "U1", "m"])).toEqual({ kind: "component", name: "U1" });
    expect(subjectOf(findings, ["gone", "r", "", "", "m"])).toBeNull();
  });
});

describe("LexiconPresenter", () => {
  function harness(start?: NamingConvention) {
    const sent: NamingConvention[] = [];
    const client: LexiconClient = {
      getLexicon: vi.fn(async (req) => {
        if (req.candidate?.lexicon?.net?.rail?.patterns.includes("(")) throw new Error("[invalid_argument] invalid argument: bad pattern");
        return { entries: [entry("net.rail", "pattern", "x")], convention: req.candidate ?? start } as never;
      }),
      compareLexicons: vi.fn(async (req) => {
        sent.push(req.conventions);
        return { tables: [create(TableSchema, { name: "role_changes" })] } as never;
      }),
      renderConvention: vi.fn(async () => ({ content: new Uint8Array([1]), contentType: "application/yaml", filename: "house.yaml" }) as never),
    };
    let state: LexiconState = emptyLexicon();
    const ports = { snapshot: async () => "snapshots/1", download: vi.fn(), apply: vi.fn(async () => {}) };
    const p = new LexiconPresenter(client, { setState: (s) => (state = s) }, ports);
    return { p, client, sent, ports, state: () => state };
  }

  it("sends the convention's rules with a Try, so they do not read as gone", async () => {
    const start = conv();
    start.rules = [create(NamingRuleSchema, { name: "signal", allow: ["^A"] })];
    const h = harness(start);
    h.p.reload();
    await vi.waitFor(() => expect(h.state().name).toBe("house"));
    await h.p.add("net.rail", "pattern", "^USBH_3V3$");
    await h.p.tryDraft();
    expect(h.sent[0].rules.map((r) => r.name)).toEqual(["signal"]);
    expect(h.sent[0].lexicon?.net?.rail?.patterns).toEqual(["^USBH_3V3$"]);
    expect(h.state().tables).toHaveLength(1);
  });

  it("drops Try's answer when the draft changes", async () => {
    const h = harness();
    await h.p.tryDraft();
    expect(h.state().tables).not.toBeNull();
    await h.p.add("net.rail", "pattern", "y");
    expect(h.state().tables).toBeNull();
  });

  it("keeps the draft and shows the error when a pattern does not compile", async () => {
    const h = harness();
    await h.p.add("net.rail", "pattern", "good");
    await h.p.add("net.rail", "pattern", "(");
    expect(h.state().error).toBe("bad pattern");
    await h.p.tryDraft();
    expect(h.sent[0].lexicon?.net?.rail?.patterns).toEqual(["good"]);
  });

  it("downloads what the engine rendered, and applies a copy of the draft", async () => {
    const h = harness();
    await h.p.add("net.rail", "pattern", "y");
    await h.p.save();
    expect(h.ports.download).toHaveBeenCalledWith({ filename: "house.yaml", contentType: "application/yaml", content: new Uint8Array([1]) });
    await h.p.apply();
    expect(h.ports.apply).toHaveBeenCalledTimes(1);
  });
});
