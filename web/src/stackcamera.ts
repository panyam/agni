// The camera a board view uses while it shows the board's stack apart (agni issue 1593). It is an
// orthographic camera that orbits a target point. Orthographic keeps a length the same size wherever
// it sits, so the stack's drawn height can be checked against the board's thickness, and a click
// becomes a ray straight into the scene that meets each layer's plane at one point.
//
// yaw turns the camera about the board's z axis, and pitch raises it above the board's plane, both in
// degrees. Pitch 90 looks straight down, as the flat board view does, and yaw 0 with a low pitch looks
// at the board's front edge. Pixels are device pixels with y running down the canvas.

export interface Vec3 {
  x: number;
  y: number;
  z: number;
}

export interface StackPose {
  yaw: number;
  pitch: number;
}

export type StackPreset = "top" | "iso" | "front" | "side";

// PRESETS are the four views the toolbar offers. Front and side keep a little pitch, so each layer
// shows as a thin band rather than a line.
export const PRESETS: Readonly<Record<StackPreset, StackPose>> = {
  top: { yaw: 0, pitch: 90 },
  iso: { yaw: -40, pitch: 32 },
  front: { yaw: 0, pitch: 8 },
  side: { yaw: 90, pitch: 8 },
};

// ORBIT_DEG_PER_PX is how far a drag of one CSS pixel turns the camera.
export const ORBIT_DEG_PER_PX = 0.4;

const dot = (a: Vec3, b: Vec3) => a.x * b.x + a.y * b.y + a.z * b.z;

// Basis is the camera's frame. right and up run along the screen, and eye points from the target
// toward the camera.
export interface Basis {
  right: Vec3;
  up: Vec3;
  eye: Vec3;
}

export function basisOf(p: StackPose): Basis {
  const y = (p.yaw * Math.PI) / 180;
  const t = (p.pitch * Math.PI) / 180;
  const right = { x: Math.cos(y), y: -Math.sin(y), z: 0 };
  const eye = { x: -Math.sin(y) * Math.cos(t), y: -Math.cos(y) * Math.cos(t), z: Math.sin(t) };
  // up = eye x right
  const up = {
    x: eye.y * right.z - eye.z * right.y,
    y: eye.z * right.x - eye.x * right.z,
    z: eye.x * right.y - eye.y * right.x,
  };
  return { right, up, eye };
}

export class StackCamera {
  pose: StackPose = { ...PRESETS.iso };
  target: Vec3;
  worldPerPixel = 1;
  private width = 1;
  private height = 1;
  // depth is the half-depth of the scene, which maps to the clip volume's z.
  private depth: number;

  // centre is the scene's middle, and radius the distance from it to its farthest corner.
  constructor(
    private readonly centre: Vec3,
    private radius: number,
  ) {
    this.target = { ...centre };
    this.depth = Math.max(radius, 1) * 2;
  }

  setViewport(width: number, height: number): void {
    this.width = Math.max(width, 1);
    this.height = Math.max(height, 1);
  }

  // setRadius changes how far the scene reaches, as the explode control stretches it.
  setRadius(radius: number): void {
    this.radius = radius;
    this.depth = Math.max(radius, 1) * 2;
  }

  // fit shows the whole scene, centred.
  fit(): void {
    this.target = { ...this.centre };
    this.worldPerPixel = (2.2 * Math.max(this.radius, 1)) / Math.min(this.width, this.height);
  }

  setPose(p: StackPose): void {
    this.pose = { yaw: p.yaw, pitch: Math.min(Math.max(p.pitch, -90), 90) };
  }

  basis(): Basis {
    return basisOf(this.pose);
  }

  // matrix maps a world point into clip space, as a column-major mat4 for the GLSL uniform. A point
  // nearer the camera gets a smaller z, so the default depth test keeps the nearest.
  matrix(): Float32Array {
    const { right, up, eye } = this.basis();
    const sx = 2 / (this.width * this.worldPerPixel);
    const sy = 2 / (this.height * this.worldPerPixel);
    const sz = -1 / this.depth;
    const t = this.target;
    const rows = [
      [right.x * sx, right.y * sx, right.z * sx, -dot(t, right) * sx],
      [up.x * sy, up.y * sy, up.z * sy, -dot(t, up) * sy],
      [eye.x * sz, eye.y * sz, eye.z * sz, -dot(t, eye) * sz],
      [0, 0, 0, 1],
    ];
    const m = new Float32Array(16);
    for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) m[c * 4 + r] = rows[r][c];
    return m;
  }

  // project is the device pixel a world point draws at.
  project(p: Vec3): { x: number; y: number } {
    const { right, up } = this.basis();
    const d = { x: p.x - this.target.x, y: p.y - this.target.y, z: p.z - this.target.z };
    return { x: this.width / 2 + dot(d, right) / this.worldPerPixel, y: this.height / 2 - dot(d, up) / this.worldPerPixel };
  }

  // hitPlane is the world point where the ray through a device pixel meets the plane at height z, or
  // null when the camera looks along the plane.
  hitPlane(px: number, py: number, z: number): { x: number; y: number } | null {
    const { right, up, eye } = this.basis();
    if (Math.abs(eye.z) < 1e-6) return null;
    const sx = (px - this.width / 2) * this.worldPerPixel;
    const sy = (this.height / 2 - py) * this.worldPerPixel;
    const o = {
      x: this.target.x + right.x * sx + up.x * sy,
      y: this.target.y + right.y * sx + up.y * sy,
      z: this.target.z + right.z * sx + up.z * sy,
    };
    // The ray runs from o along -eye.
    const s = (o.z - z) / eye.z;
    return { x: o.x - eye.x * s, y: o.y - eye.y * s };
  }

  // orbit turns the camera by a drag of dx, dy CSS pixels.
  orbit(dx: number, dy: number): void {
    this.setPose({ yaw: this.pose.yaw - dx * ORBIT_DEG_PER_PX, pitch: this.pose.pitch + dy * ORBIT_DEG_PER_PX });
  }

  // pan moves the scene with a drag of dx, dy device pixels.
  pan(dx: number, dy: number): void {
    const { right, up } = this.basis();
    const w = this.worldPerPixel;
    this.target = {
      x: this.target.x - right.x * dx * w + up.x * dy * w,
      y: this.target.y - right.y * dx * w + up.y * dy * w,
      z: this.target.z - right.z * dx * w + up.z * dy * w,
    };
  }

  // zoomAt zooms by factor, as Camera.zoomAt does, keeping the point under the cursor in place on the
  // screen's plane through the target.
  zoomAt(px: number, py: number, factor: number): void {
    const { right, up } = this.basis();
    const sx = px - this.width / 2;
    const sy = this.height / 2 - py;
    const before = this.worldPerPixel;
    this.worldPerPixel /= factor;
    const k = before - this.worldPerPixel;
    this.target = {
      x: this.target.x + (right.x * sx + up.x * sy) * k,
      y: this.target.y + (right.y * sx + up.y * sy) * k,
      z: this.target.z + (right.z * sx + up.z * sy) * k,
    };
  }
}
