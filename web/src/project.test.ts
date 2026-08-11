import { describe, it, expect } from "vitest";
import {
  emptyProject,
  projectLabel,
  isOverridden,
  canGoPlain,
  entryNotice,
  NO_PROJECT_LABEL,
  PLAIN_LABEL,
  type ProjectState,
} from "./project.js";

function state(over: Partial<ProjectState> = {}): ProjectState {
  return { ...emptyProject(), ...over };
}

describe("projectLabel", () => {
  it("names the project whose config produced the answers", () => {
    expect(projectLabel(state({ project: "projects/gateway", title: "Gateway ECU" }))).toBe("Gateway ECU");
  });

  it("falls back to the resource name when the project is untitled", () => {
    expect(projectLabel(state({ project: "projects/gateway" }))).toBe("projects/gateway");
  });

  // Blank would read as "not checked yet". Most files on a mounted folder belong to no project, so
  // this is an ordinary answer and has to look like one.
  it("states no-project rather than leaving it blank", () => {
    expect(projectLabel(state())).toBe(NO_PROJECT_LABEL);
  });

  // The two produce identical findings and mean different things: one is a choice, the other is a
  // fact about the design. Spelling them the same would hide the choice.
  it("distinguishes the built-in catalog by choice from having no project", () => {
    const chosen = projectLabel(state({ project: "projects/gateway", plain: true }));
    expect(chosen).toBe(PLAIN_LABEL);
    expect(chosen).not.toBe(NO_PROJECT_LABEL);
  });

  it("says it is resolving rather than claiming an answer it does not have", () => {
    expect(projectLabel(state({ busy: true, project: "projects/gateway" }))).toBe("resolving…");
  });
});

describe("isOverridden", () => {
  it("marks the built-in catalog as a non-default state", () => {
    expect(isOverridden(state({ project: "projects/gateway", plain: true }))).toBe(true);
  });

  // A design running its own project's config is the default, not an override.
  it("does not mark a design running its own project's config", () => {
    expect(isOverridden(state({ project: "projects/gateway" }))).toBe(false);
  });
});

describe("canGoPlain", () => {
  // Offering the toggle here would imply a difference that does not exist: a design with no project
  // is already running the built-in catalog.
  it("is meaningless for a design with no project", () => {
    expect(canGoPlain(state())).toBe(false);
  });

  it("is offered for a design in a project", () => {
    expect(canGoPlain(state({ project: "projects/gateway" }))).toBe(true);
  });
});

describe("entryNotice", () => {
  // The served viewer shows the file it was asked for. A silent swap has no browser equivalent: the
  // user picked a file in a tree and would be looking at a different one with nothing to say so.
  it("says so when the open file is a companion rather than the entry", () => {
    const s = state({ entry: "mount://m/d/gateway.edn", namedIsEntry: false });
    expect(entryNotice(s)).toContain("gateway.edn");
    expect(entryNotice(s)).toContain("companion");
  });

  it("is silent when the open file IS the entry", () => {
    expect(entryNotice(state({ entry: "mount://m/d/gateway.edn", namedIsEntry: true }))).toBe("");
  });

  it("is silent while resolving, rather than claiming a companion it has not confirmed", () => {
    expect(entryNotice(state({ entry: "mount://m/d/gateway.edn", namedIsEntry: false, busy: true }))).toBe("");
  });

  it("is silent for a design that resolved to nothing", () => {
    expect(entryNotice(state({ namedIsEntry: false }))).toBe("");
  });
});
