// SHEET_DRAWN names the performance mark the canvas records once a sheet it was handed is on
// screen, in either renderer, so a browser spec can time a sheet's first draw the same way in both
// (agni issue 1330, `make exercise-webgl`). Its detail says which renderer drew it, as
// `{ mode: "svg" | "webgl" }`.
//
// "On screen" is the animation frame AFTER the one that first shows the sheet. For SVG that is the
// second frame after the markup goes in, since the first frame lays it out and paints it. For WebGL
// it is the frame after the first render call. Both then include the browser's own paint, and
// neither includes a frame the reader could not have seen.
export const SHEET_DRAWN = "agni:sheet-drawn";
