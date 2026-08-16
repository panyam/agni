// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { ago, openUrl, projectsIsland, recentsIsland } from "./landingpanels.jsx";
import { noteOpen } from "./recents.js";

const fake = vi.hoisted(() => ({
  projects: [] as { name: string; title: string }[],
  designs: {} as Record<string, { name: string; title: string; entryUri: string }[]>,
  fail: false,
}));
vi.mock("./api.js", () => ({
  projectClient: () => ({
    listProjects: async () => {
      if (fake.fail) throw new Error("no project service");
      return { projects: fake.projects };
    },
    listDesigns: async ({ parent }: { parent: string }) => ({ designs: fake.designs[parent] ?? [] }),
  }),
}));

function mount(island: (el: HTMLElement) => { activate: () => void }) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  island(el).activate();
  return el;
}

const settle = () => vi.waitFor(() => {}, { timeout: 500 });
const rows = (el: HTMLElement) => [...el.querySelectorAll(".ld-row")].map((r) => r.textContent?.trim());
const links = (el: HTMLElement) => [...el.querySelectorAll("a.ld-link")].map((a) => a.getAttribute("href"));

beforeEach(() => {
  document.body.replaceChildren();
  localStorage.clear();
  fake.projects = [];
  fake.designs = {};
  fake.fail = false;
});

describe("openUrl", () => {
  // Each half goes through its own page's URL builder, so these assertions are what pins the two
  // spaces apart: a datasheet routed into the /designs/ space would open a page that cannot show it.
  it("routes a design to the viewer and a datasheet to the workbench", () => {
    expect(openUrl("design", "m", "boards/a.edn")).toBe("/designs/m/boards/a.edn/view");
    expect(openUrl("datasheet", "m", "vendor/x.pdf")).toBe("/datasheets/files/m/vendor/x.pdf");
  });
});

describe("ago", () => {
  const now = 1_000_000_000;
  it("words an age coarsely", () => {
    expect(ago(now, now)).toBe("just now");
    expect(ago(now - 5 * 60_000, now)).toBe("5m ago");
    expect(ago(now - 3 * 3_600_000, now)).toBe("3h ago");
    expect(ago(now - 26 * 3_600_000, now)).toBe("yesterday");
    expect(ago(now - 5 * 86_400_000, now)).toBe("5d ago");
  });
});

describe("recents island", () => {
  it("says so when nothing has been opened", async () => {
    const el = mount((e) => recentsIsland(e, null, 1000));
    await settle();
    expect(el.querySelector(".ld-empty")?.textContent).toContain("Nothing opened yet");
    expect(rows(el)).toHaveLength(0);
  });

  it("lists what was opened, newest first, each linking to its own page", async () => {
    noteOpen({ kind: "design", mount: "m", path: "boards/a.edn", label: "a.edn" }, 1000);
    noteOpen({ kind: "datasheet", mount: "ds", path: "vendor/x.pdf", label: "x.pdf" }, 2000);

    const el = mount((e) => recentsIsland(e, null, 2000));
    await settle();
    expect(links(el)).toEqual(["/datasheets/files/ds/vendor/x.pdf", "/designs/m/boards/a.edn/view"]);
    expect(rows(el)[0]).toContain("x.pdf");
    expect(rows(el)[0]).toContain("just now");
  });

  it("clears the list from the page", async () => {
    noteOpen({ kind: "design", mount: "m", path: "a.edn", label: "a.edn" }, 1000);
    const el = mount((e) => recentsIsland(e, null, 1000));
    await settle();

    (el.querySelector(".ld-clear") as HTMLButtonElement).click();
    await settle();
    expect(rows(el)).toHaveLength(0);
    // Cleared in the store too, not just on screen: a reload must not bring it back.
    expect(localStorage.getItem("agni.recents")).toBeNull();
  });
});

describe("projects island", () => {
  it("lists each project's declared designs, linking to the ENTRY file", async () => {
    fake.projects = [{ name: "projects/gateway", title: "Gateway ECU project" }];
    fake.designs["projects/gateway"] = [
      { name: "projects/gateway/designs/gw", title: "Gateway ECU", entryUri: "mount://m/gateway/gw.edn" },
    ];

    const el = mount((e) => projectsIsland(e, null));
    await vi.waitFor(() => expect(rows(el)).toHaveLength(1));
    expect(el.textContent).toContain("Gateway ECU project");
    expect(links(el)).toEqual(["/designs/m/gateway/gw.edn/view"]);
  });

  // A deployment with no descriptors is the ordinary case (project.proto says so), and so is a
  // server that cannot answer. Both render nothing at all rather than an empty section or an error,
  // because the destinations above are the page and this list is a shortcut past them.
  it("renders nothing when there are no projects, and nothing when the call fails", async () => {
    const empty = mount((e) => projectsIsland(e, null));
    await settle();
    expect(empty.querySelector(".ld-section")).toBeNull();

    fake.fail = true;
    const failed = mount((e) => projectsIsland(e, null));
    await settle();
    expect(failed.querySelector(".ld-section")).toBeNull();
  });

  // A project whose designs are all undeclared would render a heading over nothing.
  it("drops a project with no designs", async () => {
    fake.projects = [{ name: "projects/empty", title: "Empty" }];
    const el = mount((e) => projectsIsland(e, null));
    await settle();
    expect(el.querySelector(".ld-section")).toBeNull();
  });
});

// The wiring bug this guards is the one composition.test.ts was written for: an island that mounts
// into a hole the page does not declare is silently absent, and every unit test still passes. So
// the ids landing.ts looks up are checked against the SHIPPED template rather than a copy.
describe("landing page composition", () => {
  it("declares the holes the entry point mounts into", () => {
    const page = readFileSync(join(process.cwd(), "templates/LandingPage.html"), "utf8");
    const entry = readFileSync(join(process.cwd(), "src/landing.ts"), "utf8");
    for (const id of ["landing-recents", "landing-projects"]) {
      expect(entry, `landing.ts should mount #${id}`).toContain(`getElementById("${id}")`);
      expect(page, `LandingPage.html should declare #${id}`).toContain(`id="${id}"`);
    }
    // The page's own bundle, not the viewer's: loading app.js here would drag the renderer and the
    // dock into the first page anyone visits.
    expect(page).toContain("/static/landing.js");
    expect(page).not.toContain("/static/app.js");
  });
});
