// DocsPage is the base client bundle loaded on a page that opts into a
// playground (front-matter `playground: viewer`, injected by BasePage.html via
// the gen.DocsPage.html include esbuild emits).
//
// It hydrates the inline interactive tags on the page. Today that is
// <agni-viewer> (a pan/zoom canvas over a build-time-baked design SVG). Once
// the wasm-FS demo backend lands, the same hydration mounts <agni-query> /
// <agni-diff> against the in-browser engine, mirroring how the notations docs
// hydrate <notation>.
//
// Built by build.mjs (esbuild) into static/js/gen/.

import { hydrateViewers } from "./AgniViewer";

function hydrate(): void {
  document.documentElement.setAttribute("data-agni-docs", "ready");
  hydrateViewers();
}

if (document.readyState === "loading") {
  window.addEventListener("DOMContentLoaded", hydrate);
} else {
  hydrate();
}

export {};
