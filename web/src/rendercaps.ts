// defaultRenderMode picks the renderer a page opens in when its address names none (agni issue
// 1329). WebGL pans Jetson's board at p95 7.5 ms against SVG's 20.9 ms on a GPU. A browser that
// renders WebGL in software is different: headless Chrome's SwiftShader took 4.9 s for that board's
// first draw, where SVG took 0.4 s. So a page opens in WebGL only on a hardware GPU.

import type { RenderMode } from "./viewer.js";

// GpuProbe reports whether the browser draws WebGL2 on hardware: null when no WebGL2 context is
// available, or when Chrome reports a major performance caveat. Otherwise it names the renderer.
export type GpuProbe = () => { renderer: string } | null;

const SOFTWARE = /swiftshader|llvmpipe|softpipe|software|basic render/i;

export function defaultRenderMode(probe: GpuProbe = probeGpu): Exclude<RenderMode, "native"> {
  const gpu = probe();
  if (!gpu) return "svg";
  return SOFTWARE.test(gpu.renderer) ? "svg" : "webgl";
}

function probeGpu(): { renderer: string } | null {
  try {
    const canvas = document.createElement("canvas");
    // failIfMajorPerformanceCaveat refuses a context that Chrome would draw in software.
    const gl = canvas.getContext("webgl2", { failIfMajorPerformanceCaveat: true });
    if (!gl) return null;
    const info = gl.getExtension("WEBGL_debug_renderer_info");
    const renderer = String(gl.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : gl.RENDERER) ?? "");
    gl.getExtension("WEBGL_lose_context")?.loseContext();
    return { renderer };
  } catch {
    return null;
  }
}
