import { describe, it, expect } from "vitest";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { workspaceClient, designClient } from "./api.js";

// Guards the generated service contract the frontend depends on: the proto must emit a
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
});
