// DrawGate decides whether a WebGL view draws this frame (agni issue 1479). A view draws when
// something it shows has changed since its last draw: its camera, its size, or its content. Two
// views beside each other then cost nothing while the reader works in only one of them.
//
// It compares the camera's matrix rather than hooking every place the camera moves, so a camera
// moved from outside the view, by a locked peer or the other side of a diff, still draws.

// DrawState is what one frame would draw from. content is the renderer that holds the sheet, which
// a new sheet replaces, and version counts the changes made to it since (Renderer.version).
export interface DrawState {
  matrix: ArrayLike<number>;
  width: number;
  height: number;
  dpr: number;
  content: object;
  version: number;
}

export class DrawGate {
  private matrix: number[] | null = null;
  private width = -1;
  private height = -1;
  private dpr = -1;
  private content: object | null = null;
  private version = -1;

  // shouldDraw says whether s differs from the last state it drew, and records s when it does. A
  // view with no area, such as one behind another dock tab, draws nothing and records nothing, so it
  // draws as soon as it is shown again.
  shouldDraw(s: DrawState): boolean {
    if (s.width <= 0 || s.height <= 0) return false;
    if (
      this.matrix !== null &&
      s.content === this.content &&
      s.version === this.version &&
      s.width === this.width &&
      s.height === this.height &&
      s.dpr === this.dpr &&
      sameMatrix(this.matrix, s.matrix)
    ) {
      return false;
    }
    this.matrix = Array.from(s.matrix);
    this.width = s.width;
    this.height = s.height;
    this.dpr = s.dpr;
    this.content = s.content;
    this.version = s.version;
    return true;
  }

  // invalidate makes the next frame draw whatever it holds.
  invalidate(): void {
    this.matrix = null;
  }
}

function sameMatrix(a: number[], b: ArrayLike<number>): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}
