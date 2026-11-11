import { describe, expect, it } from "vitest";
import { defaultRenderMode } from "./rendercaps.js";

describe("defaultRenderMode", () => {
  it("opens in WebGL on a hardware GPU", () => {
    expect(defaultRenderMode(() => ({ renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M5 Pro, Unspecified Version)" }))).toBe("webgl");
    expect(defaultRenderMode(() => ({ renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Direct3D11 vs_5_0 ps_5_0, D3D11)" }))).toBe("webgl");
  });

  it("opens in SVG when WebGL draws in software", () => {
    expect(defaultRenderMode(() => ({ renderer: "ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero) (0x0000C0DE)), SwiftShader driver)" }))).toBe("svg");
    expect(defaultRenderMode(() => ({ renderer: "llvmpipe (LLVM 15.0.7, 256 bits)" }))).toBe("svg");
    expect(defaultRenderMode(() => ({ renderer: "ANGLE (Microsoft, Microsoft Basic Render Driver Direct3D11 vs_5_0 ps_5_0, D3D11)" }))).toBe("svg");
  });

  it("opens in SVG with no WebGL2 context", () => {
    expect(defaultRenderMode(() => null)).toBe("svg");
  });
});
