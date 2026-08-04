// AgniViewer hydrates an <agni-viewer> tag into a pan/zoom canvas over a
// pre-rendered design SVG.
//
// This is the Tier-1 proof-of-pattern: the SVG is baked at build time (an
// `agni render` golden), so the widget needs no engine. The SAME tag and mount
// contract is what the Tier-2 wasm backend plugs into later — it swaps the
// static `src` fetch for a live GetSheet/HighlightSheet call, the pan/zoom
// shell is unchanged.
//
// Usage in a page (front-matter must set `playground: viewer`):
//   <agni-viewer src="{{.Site.PathPrefix}}/static/designs/demo-board.svg"
//                caption="Demo board — B.Cu/F.Cu copper"></agni-viewer>

interface ViewState {
  scale: number;
  tx: number;
  ty: number;
}

const MIN_SCALE = 0.2;
const MAX_SCALE = 12;
const ZOOM_STEP = 1.2;

/** Mount a single <agni-viewer> element. Idempotent (guards on a data flag). */
async function mount(host: HTMLElement): Promise<void> {
  if (host.getAttribute("data-agni-mounted") === "true") return;
  host.setAttribute("data-agni-mounted", "true");

  const src = host.getAttribute("src");
  const caption = host.getAttribute("caption") || "";
  if (!src) {
    host.textContent = "agni-viewer: missing src";
    return;
  }

  // Shell: toolbar + a clipped stage that holds the transformed SVG.
  const shell = document.createElement("div");
  shell.className = "agni-viewer";

  const toolbar = document.createElement("div");
  toolbar.className = "agni-viewer-toolbar";
  const zoomInBtn = button("+", "Zoom in");
  const zoomOutBtn = button("−", "Zoom out");
  const fitBtn = button("Fit", "Fit to view");
  const label = document.createElement("span");
  label.className = "agni-viewer-caption";
  label.textContent = caption;
  toolbar.append(zoomInBtn, zoomOutBtn, fitBtn, label);

  const stage = document.createElement("div");
  stage.className = "agni-viewer-stage";
  const canvas = document.createElement("div");
  canvas.className = "agni-viewer-canvas";
  stage.append(canvas);

  const hint = document.createElement("div");
  hint.className = "agni-viewer-hint";
  hint.textContent = "drag to pan · scroll to zoom";
  stage.append(hint);

  shell.append(toolbar, stage);
  host.replaceChildren(shell);

  // Load the baked SVG inline so it inherits transforms and stays crisp.
  let svg: SVGSVGElement | null = null;
  try {
    const res = await fetch(src);
    const text = await res.text();
    const doc = new DOMParser().parseFromString(text, "image/svg+xml");
    svg = doc.documentElement as unknown as SVGSVGElement;
    canvas.append(svg);
  } catch {
    canvas.textContent = `agni-viewer: could not load ${src}`;
    return;
  }

  const natW = parseFloat(svg.getAttribute("width") || "0") || 800;
  const natH = parseFloat(svg.getAttribute("height") || "0") || 600;
  // Let the transform own sizing; drop the SVG's own width/height.
  svg.removeAttribute("width");
  svg.removeAttribute("height");
  svg.style.display = "block";
  svg.style.width = `${natW}px`;
  svg.style.height = `${natH}px`;

  const view: ViewState = { scale: 1, tx: 0, ty: 0 };

  function apply(): void {
    canvas.style.transform = `translate(${view.tx}px, ${view.ty}px) scale(${view.scale})`;
  }

  function fit(): void {
    const r = stage.getBoundingClientRect();
    const pad = 24;
    const s = Math.min((r.width - pad) / natW, (r.height - pad) / natH);
    view.scale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, s));
    view.tx = (r.width - natW * view.scale) / 2;
    view.ty = (r.height - natH * view.scale) / 2;
    apply();
  }

  function zoomAt(clientX: number, clientY: number, factor: number): void {
    const r = stage.getBoundingClientRect();
    const px = clientX - r.left;
    const py = clientY - r.top;
    const next = Math.max(MIN_SCALE, Math.min(MAX_SCALE, view.scale * factor));
    const k = next / view.scale;
    // Keep the point under the cursor fixed.
    view.tx = px - (px - view.tx) * k;
    view.ty = py - (py - view.ty) * k;
    view.scale = next;
    apply();
  }

  stage.addEventListener(
    "wheel",
    (e: WheelEvent) => {
      e.preventDefault();
      zoomAt(e.clientX, e.clientY, e.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP);
    },
    { passive: false },
  );

  let dragging = false;
  let lastX = 0;
  let lastY = 0;
  stage.addEventListener("pointerdown", (e: PointerEvent) => {
    dragging = true;
    lastX = e.clientX;
    lastY = e.clientY;
    stage.setPointerCapture(e.pointerId);
    stage.classList.add("agni-viewer-dragging");
  });
  stage.addEventListener("pointermove", (e: PointerEvent) => {
    if (!dragging) return;
    view.tx += e.clientX - lastX;
    view.ty += e.clientY - lastY;
    lastX = e.clientX;
    lastY = e.clientY;
    apply();
  });
  const endDrag = (e: PointerEvent) => {
    dragging = false;
    stage.classList.remove("agni-viewer-dragging");
    if (stage.hasPointerCapture(e.pointerId)) stage.releasePointerCapture(e.pointerId);
  };
  stage.addEventListener("pointerup", endDrag);
  stage.addEventListener("pointercancel", endDrag);

  const center = () => {
    const r = stage.getBoundingClientRect();
    return { x: r.width / 2, y: r.height / 2 };
  };
  zoomInBtn.addEventListener("click", () => {
    const c = center();
    const r = stage.getBoundingClientRect();
    zoomAt(r.left + c.x, r.top + c.y, ZOOM_STEP);
  });
  zoomOutBtn.addEventListener("click", () => {
    const c = center();
    const r = stage.getBoundingClientRect();
    zoomAt(r.left + c.x, r.top + c.y, 1 / ZOOM_STEP);
  });
  fitBtn.addEventListener("click", fit);

  // Fit once the stage has a real size.
  requestAnimationFrame(fit);
}

function button(text: string, title: string): HTMLButtonElement {
  const b = document.createElement("button");
  b.type = "button";
  b.className = "agni-viewer-btn";
  b.textContent = text;
  b.title = title;
  b.setAttribute("aria-label", title);
  return b;
}

/** Hydrate every <agni-viewer> on the page. */
export function hydrateViewers(): void {
  document.querySelectorAll<HTMLElement>("agni-viewer").forEach((el) => {
    void mount(el);
  });
}
