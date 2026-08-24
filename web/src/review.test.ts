import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { describe, it, expect } from "vitest";
import {
  tally,
  covered,
  emptyTally,
  outcomeClass,
  checklistOptions,
  reviewFromWire,
  runTally,
  areaTally,
  selectedRun,
  emptyReview,
  OUTCOME_LABEL,
  OUTCOME_PASS,
} from "./review.js";

const here = dirname(fileURLToPath(import.meta.url));

interface TwinCase {
  name: string;
  outcomes: string[];
  want: Record<string, number>;
}

// The SAME fixture core/review's TestTallyTwinFixture reads. Both surfaces derive the tally from item
// outcomes rather than reading it off the wire, so nothing but a shared set of numbers stops them
// drifting — and `covered` in particular is what a team reads as "how much of our checklist is
// mechanised", so a client that bucketed not-automated differently would report a checklist as
// answered when nobody had answered it.
describe("tally — twinned with core/review via tally_twin.json", () => {
  const fixture = JSON.parse(
    readFileSync(join(here, "..", "..", "core", "review", "testdata", "tally_twin.json"), "utf8"),
  ) as { cases: TwinCase[] };

  it("has cases to check", () => {
    expect(fixture.cases.length).toBeGreaterThan(0);
  });

  for (const tc of fixture.cases) {
    it(`matches the Go tally: ${tc.name}`, () => {
      const got = tally(tc.outcomes);
      expect(got.pass).toBe(tc.want.pass);
      expect(got.fail).toBe(tc.want.fail);
      expect(got.notApplicable).toBe(tc.want.notApplicable);
      expect(got.notAutomated).toBe(tc.want.notAutomated);
      expect(got.provisional).toBe(tc.want.provisional);
      expect(got.needsDesignIntent).toBe(tc.want.needsDesignIntent);
      expect(got.computedNA).toBe(tc.want.computedNA);
      expect(got.needsData).toBe(tc.want.needsData);
      expect(got.inconclusive).toBe(tc.want.inconclusive);
      expect(got.total).toBe(tc.want.total);
      expect(covered(got)).toBe(tc.want.covered);
    });
  }

  // Every outcome the panel labels must appear somewhere in the fixture, so adding a verdict to the
  // vocabulary cannot quietly skip the twinning. The Go side asserts the mirror of this.
  it("exercises every outcome the panel knows how to label", () => {
    const seen = new Set(fixture.cases.flatMap((c) => c.outcomes));
    for (const outcome of Object.keys(OUTCOME_LABEL)) {
      expect(seen.has(outcome), `${outcome} appears in no twin case`).toBe(true);
    }
  });
});

describe("tally", () => {
  it("counts an unknown outcome toward the total but no bucket", () => {
    const got = tally(["definitely-not-a-verdict"]);
    expect(got.total).toBe(1);
    expect({ ...got, total: 0 }).toEqual(emptyTally());
    // It is covered, because only not-automated is not: a verdict this build cannot name is still a
    // verdict the engine produced, and calling it uncovered would understate the checklist.
    expect(covered(got)).toBe(1);
  });

  it("counts an empty run as nothing rather than throwing", () => {
    expect(tally([])).toEqual(emptyTally());
    expect(covered(emptyTally())).toBe(0);
  });
});

describe("outcomeClass", () => {
  it("gives pass its own class and every other verdict a different one", () => {
    const passClass = outcomeClass(OUTCOME_PASS);
    for (const outcome of Object.keys(OUTCOME_LABEL)) {
      if (outcome === OUTCOME_PASS) continue;
      expect(outcomeClass(outcome), `${outcome} must not share the pass style`).not.toBe(passClass);
    }
  });

  // The load-bearing one. An outcome this build has never heard of must not be styled as a pass,
  // because guessing optimistically about a verdict we cannot interpret is the single guess that can
  // report an unanswered question as answered.
  it("styles an unknown outcome as unknown, never as a pass", () => {
    expect(outcomeClass("covered-externally")).toBe("rv-unknown");
    expect(outcomeClass("covered-externally")).not.toBe(outcomeClass(OUTCOME_PASS));
  });

  it("produces a css-safe class for outcomes with punctuation", () => {
    expect(outcomeClass("computed-n/a")).toBe("rv-computed-n-a");
    expect(outcomeClass("computed-n/a")).not.toContain("/");
  });
});

describe("checklistOptions", () => {
  it("keeps yaml files and drops directories and other files", () => {
    const got = checklistOptions([
      { name: "review.yaml", path: "proj/review.yaml", isDir: false },
      { name: "conventions.yml", path: "proj/conventions.yml", isDir: false },
      { name: "board.kicad_sch", path: "proj/board.kicad_sch", isDir: false },
      { name: "profiles", path: "proj/profiles", isDir: true },
    ]);
    expect(got.map((c) => c.label)).toEqual(["review.yaml", "conventions.yml"]);
    expect(got[0].ref).toBe("proj/review.yaml");
  });

  // Deciding whether a YAML file is really a checklist means parsing it, which is the server's job:
  // GetReviewManifest validates and says so. Offering a file that turns out not to be one costs a
  // clear error; hiding a real one costs a user their own file.
  it("does not try to guess which yaml is a manifest", () => {
    const got = checklistOptions([{ name: "anything.yaml", path: "a/anything.yaml", isDir: false }]);
    expect(got).toHaveLength(1);
  });
});

describe("reviewFromWire", () => {
  const wire = {
    name: "reviews/20260810T200422.087841000Z-372fd0a5",
    results: {
      meta: { createdAt: "2026-08-10T20:04:22Z", producerVersion: "v0.1.1" },
      design: { source: "review/can-broken.edn", contentHash: "sha256:abc" },
      manifest: "Mini board review",
      areas: [
        {
          name: "CAN Interface",
          items: [
            {
              id: "202",
              title: "termination",
              outcome: "fail",
              note: "",
              findings: [
                { rule: "profile/can-termination-missing", severity: "error", message: "no 120R", subject: { kind: "net", ref: "CANH", pin: "", netId: "n1", busId: "" } },
              ],
            },
            { id: "196", title: "manual", outcome: "not-automated", note: "the EE signs this off", findings: [] },
          ],
        },
      ],
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;

  it("maps identity, provenance, and per-item outcomes off the document", () => {
    const run = reviewFromWire(wire);
    expect(run.name).toBe("reviews/20260810T200422.087841000Z-372fd0a5");
    expect(run.design).toBe("review/can-broken.edn");
    expect(run.designHash).toBe("sha256:abc");
    expect(run.createdAt).toBe("2026-08-10T20:04:22Z");
    expect(run.producerVersion).toBe("v0.1.1");
    expect(run.manifest).toBe("Mini board review");
    expect(run.areas).toHaveLength(1);
    expect(run.areas[0].items.map((i) => i.outcome)).toEqual(["fail", "not-automated"]);
  });

  it("carries a failing item's findings with the keys the locate path needs", () => {
    const item = reviewFromWire(wire).areas[0].items[0];
    expect(item.findings).toHaveLength(1);
    expect(item.findings[0].subject).toBe("CANH");
    expect(item.findings[0].kind).toBe("net");
    expect(item.findings[0].netId).toBe("n1");
  });

  it("survives a document with no areas rather than throwing", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const run = reviewFromWire({ name: "reviews/x", results: {} } as any);
    expect(run.areas).toEqual([]);
    expect(runTally(run).total).toBe(0);
  });

  it("tallies the run and each area from the mapped outcomes", () => {
    const run = reviewFromWire(wire);
    expect(runTally(run).fail).toBe(1);
    expect(runTally(run).notAutomated).toBe(1);
    expect(covered(runTally(run))).toBe(1);
    expect(areaTally(run.areas[0]).total).toBe(2);
  });
});

describe("selectedRun", () => {
  it("returns the run the state points at, and undefined when it points nowhere", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const run = reviewFromWire({ name: "reviews/a", results: {} } as any);
    const state = { ...emptyReview(), runs: [run], selected: "reviews/a" };
    expect(selectedRun(state)?.name).toBe("reviews/a");
    expect(selectedRun({ ...state, selected: "reviews/gone" })).toBeUndefined();
    expect(selectedRun(emptyReview())).toBeUndefined();
  });
});
