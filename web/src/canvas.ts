import { BaseComponent } from "@panyam/tsappkit";
import { toRenderable, type DecodedSheet } from "./packed.js";
import { Camera, boundsOfVertices, type CameraView } from "./camera.js";
import { wheelZoomFactor } from "./panzoom.js";
import { hexToRgba, Renderer } from "./webgl.js";
import { TextOverlay } from "./textoverlay.js";
import {
  DEFAULT_HIGHLIGHT_COLOR,
  HighlightShape,
  PATH_WIDTH_FRACTION,
  circleTriangles,
  entityBounds,
  entityFrame,
  pathQuads,
  rectTriangles,
  resolveHighlights,
  type HighlightSpec,
} from "./highlights.js";
import type { PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";

// CanvasComponent owns the WebGL2 schematic canvas as a tsappkit lifecycle component. It
// mounts into the shell's center region (#view) and renders whatever sheet it is handed:
// the file tree drives it via showSheet (WS9-003). Switching sheets just swaps the
// renderer/camera; the input handlers and the animation loop are set up once.
export class CanvasComponent extends BaseComponent {
  private gl?: WebGL2RenderingContext;
  private renderer?: Renderer;
  private camera?: Camera;
  private overlay?: TextOverlay;
  private started = false;
  // The sheet currently drawn (kept for the highlight join: specs -> primitive keys) and the
  // active highlight specs. The specs persist across sheet redraws so highlights survive a
  // re-render.
  private sheet?: DecodedSheet;
  private highlights: HighlightSpec[] = [];

  activate(): void {
    const canvas = this.rootElement as HTMLCanvasElement;
    const gl = canvas.getContext("webgl2");
    if (!gl) {
      this.setReadout("WebGL2 not available in this browser.");
      return;
    }
    this.gl = gl;
    this.setReadout("no sheet loaded — pick a file");
  }

  // showSheet renders a packed sheet delivered by the design API (GetSheet). Safe to call
  // repeatedly to switch sheets.
  showSheet(sheet: PackedSheet): void {
    if (!this.gl) return;
    this.draw(toRenderable(sheet));
  }

  // setHiddenGroups forwards board layer visibility to the renderer (WS7-035); kept and
  // re-applied when a new sheet arrives, so the chosen view survives sheet swaps.
  setHiddenGroups(groups: Iterable<number>): void {
    this.hidden = [...groups];
    this.renderer?.setHiddenGroups(this.hidden);
  }
  private hidden: number[] = [];

  // getView / setView snapshot and restore the camera's pan/zoom, so the presenter can
  // remember a view per (mode, file, sheet). null before any sheet has been drawn.
  getView(): CameraView | null {
    return this.camera ? this.camera.getView() : null;
  }

  setView(v: CameraView): void {
    this.camera?.setView(v);
  }

  private draw(sheet: DecodedSheet): void {
    const gl = this.gl!;
    const canvas = this.rootElement as HTMLCanvasElement;
    this.sheet = sheet;
    this.renderer = new Renderer(gl, sheet);
    this.renderer.setHiddenGroups(this.hidden);
    this.camera = new Camera(boundsOfVertices(sheet.vertices));
    this.overlay ??= this.makeOverlay();
    this.overlay?.setContent(sheet.images, sheet.labels, sheet.fontFamily);
    this.applyHighlights(); // highlights chosen before this sheet was (re)drawn are reapplied
    this.resize();
    this.camera.fit(canvas.width, canvas.height);
    this.setReadout(`sheet ${sheet.sheetId} — ${sheet.primitives.length} primitives, ${sheet.vertices.length / 2} vertices`);
    this.start();
  }

  // setHighlights tints the elements each spec selects (components, nets, pins) in the spec's
  // color/alpha; an empty array clears every highlight. Safe to call before or after a sheet is
  // drawn — the specs are remembered and reapplied on the next draw.
  setHighlights(specs: HighlightSpec[]): void {
    this.highlights = specs;
    this.applyHighlights();
  }

  // applyHighlights resolves the current specs to primitive-index groups via the sheet's keys
  // (the local mirror of render.HighlightPacked) and pushes them to the renderer: outline
  // groups recolor their primitives in place, bounding-shape groups (WS9-017) tessellate one
  // framing rect/circle per matched entity from the vertices already held — no round-trip —
  // and go to the renderer's overlay path. A no-op without a sheet or renderer yet.
  private applyHighlights(): void {
    if (!this.renderer || !this.sheet) return;
    const sheet = this.sheet;
    const fallback = hexToRgba(DEFAULT_HIGHLIGHT_COLOR, [1, 0, 1, 1]);
    const draws: { color: ReturnType<typeof hexToRgba>; primitives: Set<number> }[] = [];
    const overlays: { color: ReturnType<typeof hexToRgba>; vertices: Int32Array }[] = [];
    // PATH marker half-width in world units, a fraction of the sheet's smaller span (WS9-043) so
    // it is visible at any coordinate scale (geometry is in nanometers for KiCad).
    const sb = boundsOfVertices(sheet.vertices);
    const pathHalfWidth = PATH_WIDTH_FRACTION * Math.min(sb.maxX - sb.minX, sb.maxY - sb.minY);
    for (const g of resolveHighlights(sheet.keys, this.highlights)) {
      const color = hexToRgba(g.color, fallback, g.alpha);
      // OUTLINE recolors the entity's primitives in place. PATH (WS9-043) draws a wider
      // translucent marker: GL lines are always 1px, so its width comes from filled quads
      // tessellated along each wire segment (as the board copper tracks do), sent to the
      // overlay path — no recolor, so the wire shows through the translucent quad like the SVG
      // stroke does. The bounding shapes tessellate a per-entity frame instead.
      if (g.shape === HighlightShape.OUTLINE) {
        draws.push({ color, primitives: g.primitives });
        continue;
      }
      const tris: number[] = [];
      if (g.shape === HighlightShape.PATH) {
        const w = pathHalfWidth * g.strokeScale; // user width scale (WS9-044)
        for (const entity of g.entities) tris.push(...pathQuads(sheet.vertices, sheet.primitives, entity, w));
      } else {
        for (const entity of g.entities) {
          const b = entityBounds(sheet.vertices, sheet.primitives, entity);
          if (!b) continue;
          const frame = entityFrame(b.minX, b.minY, b.maxX, b.maxY);
          tris.push(...(g.shape === HighlightShape.BOUNDING_RECT ? rectTriangles(frame) : circleTriangles(frame)));
        }
      }
      overlays.push({ color, vertices: Int32Array.from(tris, Math.round) });
    }
    this.renderer.setHighlights(draws);
    this.renderer.setOverlays(overlays);
  }

  // makeOverlay attaches the text layer to the #text-overlay host beside the canvas. Returns
  // undefined if the host is absent, so text is simply not drawn.
  private makeOverlay(): TextOverlay | undefined {
    const host = document.getElementById("text-overlay");
    return host ? new TextOverlay(host) : undefined;
  }

  // showText / hideText follow the active render mode: the text layer belongs to the WebGL
  // view, so the composition root shows it in WebGL mode and hides it for SVG/Native.
  showText(): void {
    this.overlay?.show();
  }
  hideText(): void {
    this.overlay?.hide();
  }

  private dpr(): number {
    return window.devicePixelRatio || 1;
  }

  private resize(): void {
    const canvas = this.rootElement as HTMLCanvasElement;
    const w = Math.floor(canvas.clientWidth * this.dpr());
    const h = Math.floor(canvas.clientHeight * this.dpr());
    // A parked (closed) dock panel measures 0x0; keep the last real framebuffer so reopening
    // the panel doesn't flash a collapsed viewport before the observer fires again.
    if (w === 0 || h === 0) return;
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
    this.gl!.viewport(0, 0, canvas.width, canvas.height);
    this.camera?.setViewport(canvas.width, canvas.height);
  }

  // start wires input + the animation loop once; both read the current renderer/camera, so a
  // later showSheet swap is picked up without re-binding.
  private start(): void {
    if (this.started) return;
    this.started = true;
    const canvas = this.rootElement as HTMLCanvasElement;

    window.addEventListener("resize", () => this.resize());
    // The window listener misses dock-panel resizes (dragging a dockview splitter fires no
    // window resize), so observe the canvas element itself. Guarded: jsdom has no ResizeObserver.
    if (typeof ResizeObserver !== "undefined") {
      new ResizeObserver(() => this.resize()).observe(canvas);
    }

    let dragging = false;
    let lastX = 0;
    let lastY = 0;
    canvas.addEventListener("mousedown", (e) => {
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
    });
    window.addEventListener("mouseup", () => {
      dragging = false;
    });
    window.addEventListener("mousemove", (e) => {
      if (!dragging || !this.camera) return;
      this.camera.panPixels((e.clientX - lastX) * this.dpr(), (e.clientY - lastY) * this.dpr());
      lastX = e.clientX;
      lastY = e.clientY;
    });
    canvas.addEventListener(
      "wheel",
      (e) => {
        e.preventDefault();
        if (!this.camera) return;
        const rect = canvas.getBoundingClientRect();
        this.camera.zoomAt((e.clientX - rect.left) * this.dpr(), (e.clientY - rect.top) * this.dpr(), wheelZoomFactor(e.deltaY));
      },
      { passive: false },
    );

    const frame = (): void => {
      if (this.renderer && this.camera) {
        this.renderer.render(this.camera.matrix(), this.camera.viewRect());
        // The overlay reads the same camera in the same tick, so text stays locked to geometry.
        this.overlay?.update(this.camera.getView(), canvas.clientWidth, canvas.clientHeight, this.dpr());
      }
      requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  }

  private setReadout(text: string): void {
    const readout = document.getElementById("readout");
    if (readout) readout.textContent = text;
  }
}
