// How a journey's reader brings a part on the board into view and up to a size they can click
// (agni issue 1600). The board view zooms about the cursor, so where the cursor sits decides where
// the part ends up.

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

// zoomStep is the reader's next wheel turn, or null when the target is in view and big enough. A
// target out of view gets a zoom out about the view's middle. A target in view but too small gets a
// zoom in about the target itself, which holds it under the cursor as it grows. Zooming in about
// the view's middle pushes a target near the edge back out, and a small board view then went back
// and forth without end.
export function zoomStep(view: Box, target: Box, margin = 20, size = 8): { x: number; y: number; deltaY: number } | null {
  const cx = target.x + target.width / 2;
  const cy = target.y + target.height / 2;
  const inside = cx > view.x + margin && cx < view.x + view.width - margin && cy > view.y + margin && cy < view.y + view.height - margin;
  if (inside && Math.min(target.width, target.height) >= size) return null;
  if (inside) return { x: cx, y: cy, deltaY: -300 };
  return { x: view.x + view.width / 2, y: view.y + view.height / 2, deltaY: 300 };
}
