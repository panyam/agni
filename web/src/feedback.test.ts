import { describe, it, expect } from "vitest";
import { feedbackIssueURL, findingFeedback } from "./feedback.js";
import { DocumentStore } from "./document.js";

// A finding as the panel holds it, every field a design supplies filled with a name to look for.
const finding = {
  rule: "i2c-pull-up",
  severity: "error",
  inconclusive: false,
  subject: "I2C_SCL_SECRET",
  pin: "7",
  message: "net I2C_SCL_SECRET has no pull-up to VDD_SECRET near U17",
  context: [{ kind: "component", subject: "U17", pin: "", role: "device" }],
};
const decoded = (u: string) => decodeURIComponent(u.replace(/\+/g, " "));

// agni issues 858 and 1057: a "this looks wrong" report names agni's rule and nothing the design holds.
describe("a wrong-finding report", () => {
  it("names a shipped rule, its severity and the version, and nothing from the finding", () => {
    const url = decoded(feedbackIssueURL(findingFeedback(finding, true, "v0.2.4")));
    expect(url).toContain("i2c-pull-up");
    expect(url).toContain("v0.2.4");
    expect(url).toContain("(severity error)");
    for (const name of ["I2C_SCL_SECRET", "VDD_SECRET", "U17", "pull-up to"]) expect(url).not.toContain(name);
  });

  it("says when the finding was inconclusive", () => {
    const url = decoded(feedbackIssueURL(findingFeedback({ ...finding, severity: "warning", inconclusive: true }, true, "v0.2.4")));
    expect(url).toContain("(severity warning, inconclusive)");
  });

  it("never names a rule the project wrote", () => {
    const url = decoded(feedbackIssueURL(findingFeedback({ ...finding, rule: "acme-profiles/can-esd-missing" }, false, "v0.2.4")));
    expect(url).not.toContain("acme");
    expect(url).toContain("a rule my project wrote");
  });

  it("is what the document store puts on each finding, by the catalog's shipped flag", async () => {
    const doc = new DocumentStore();
    doc.mount = "m";
    doc.path = "b.edn";
    const rule = (name: string, shipped: boolean) => ({ name, severity: "error", summary: "", impact: "", remedy: "", detail: "", reads: [], tags: {}, available: true, unavailableReason: "", shipped });
    await doc.loadCatalog({ listRules: async () => ({ rules: [rule("i2c-pull-up", true), rule("acme-profiles/x", false)], engineVersion: "v0.2.4" }) } as never, async () => "snapshots/t");
    const item = (r: string) => doc.findingItem({ rule: r, severity: "error", message: "about I2C_SCL_SECRET", subject: { kind: "net", ref: "I2C_SCL_SECRET" } } as never);
    const shipped = decoded(item("i2c-pull-up").reportUrl ?? "");
    const project = decoded(item("acme-profiles/x").reportUrl ?? "");
    expect(shipped).toContain("i2c-pull-up");
    expect(shipped).toContain("v0.2.4");
    expect(project).not.toContain("acme");
    for (const u of [shipped, project]) expect(u).not.toContain("I2C_SCL_SECRET");
  });
});
