// AreaBand draws the rectangle a Shift-drag spans (agni issue 904), in its host's own space. The SVG
// view and the WebGL canvas both draw it (agni issue 1333), so the two drags look alike.

import type { ClientRect } from "./area.js";

export class AreaBand {
  private band: HTMLDivElement | null = null;

  // host must be positioned, since the band is absolutely placed inside it.
  constructor(private readonly host: HTMLElement) {}

  draw(r: ClientRect): void {
    if (!this.band) {
      this.band = document.createElement("div");
      this.band.className = "area-band";
      this.host.appendChild(this.band);
    }
    const h = this.host.getBoundingClientRect();
    const s = this.band.style;
    s.left = `${r.left - h.left}px`;
    s.top = `${r.top - h.top}px`;
    s.width = `${r.right - r.left}px`;
    s.height = `${r.bottom - r.top}px`;
  }

  clear(): void {
    this.band?.remove();
    this.band = null;
  }
}
