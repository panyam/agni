import { describe, it, expect, vi } from "vitest";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";
import { workspaceClient, designClient, reviewClient, timingInterceptor } from "./api.js";

// Guards the generated service contract the frontend depends on. The proto must emit a
// WorkspaceService with a ListMounts RPC, and the typed client must construct against it.
describe("web api", () => {
  it("exposes the WorkspaceService contract (ListMounts, ListDir)", () => {
    expect(WorkspaceService.typeName).toBe("agni.v1.webapi.WorkspaceService");
    expect(WorkspaceService.method.listMounts).toBeDefined();
    expect(WorkspaceService.method.listDir).toBeDefined();
  });

  it("builds a typed workspace client", () => {
    const client = workspaceClient("http://localhost:8080");
    expect(typeof client.listMounts).toBe("function");
    expect(typeof client.listDir).toBe("function");
  });

  it("exposes the DesignService contract (GetDesign, GetSheet)", () => {
    expect(DesignService.typeName).toBe("agni.v1.webapi.DesignService");
    expect(DesignService.method.getDesign).toBeDefined();
    expect(DesignService.method.getSheet).toBeDefined();
  });

  it("builds a typed design client", () => {
    const client = designClient("http://localhost:8080");
    expect(typeof client.getDesign).toBe("function");
    expect(typeof client.getSheet).toBe("function");
  });

  // Reviews are the one RESOURCE surface in this API (CONSTRAINTS C23), so the contract check is
  // that the four standard methods exist alongside the manifest resolver. A review run that could
  // be created but not listed would put the panel's history back where it started.
  it("exposes the ReviewService resource contract", () => {
    expect(ReviewService.typeName).toBe("agni.v1.webapi.ReviewService");
    for (const method of ["createReview", "getReview", "listReviews", "deleteReview", "getReviewManifest"] as const) {
      expect(ReviewService.method[method], method).toBeDefined();
    }
  });

  it("builds a typed review client", () => {
    const client = reviewClient("http://localhost:8080");
    expect(typeof client.createReview).toBe("function");
    expect(typeof client.listReviews).toBe("function");
    expect(typeof client.getReviewManifest).toBe("function");
  });
});

// The page's ?timing asks every request where its time went (agni issue 914), and the breakdown the
// engine sends back is logged so it can be read in the console.
describe("timing interceptor", () => {
  it("asks for the breakdown and logs what comes back", async () => {
    const info = vi.spyOn(console, "info").mockImplementation(() => {});
    const req = { header: new Headers(), method: { name: "CheckDesign" } };
    const res = { header: new Headers({ "Agni-Timing": '{"totalMicros":"1200"}' }) };
    const next = vi.fn(async () => res);
    // The interceptor reads only the header and the method's name, so a minimal request stands in.
    const call = timingInterceptor("explain")(next as never) as unknown as (r: typeof req) => Promise<typeof res>;
    expect(await call(req)).toBe(res);
    expect(req.header.get("Agni-Timing")).toBe("explain");
    expect(info).toHaveBeenCalledWith('agni timing CheckDesign: {"totalMicros":"1200"}');
    info.mockRestore();
  });

  it("logs nothing when the engine sent no breakdown", async () => {
    const info = vi.spyOn(console, "info").mockImplementation(() => {});
    const call = timingInterceptor("1")((async () => ({ header: new Headers() })) as never) as unknown as (r: object) => Promise<unknown>;
    await call({ header: new Headers(), method: { name: "GetDesign" } });
    expect(info).not.toHaveBeenCalled();
    info.mockRestore();
  });
});
