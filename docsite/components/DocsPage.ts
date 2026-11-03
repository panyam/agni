// DocsPage is the base client bundle for a page that opts into a playground
// with the front-matter `playground: viewer`. esbuild emits the
// gen.DocsPage.html include, and BasePage.html injects the bundle through it.
//
// It hydrates the inline interactive tags on the page. Today that is
// <agni-viewer>, a pan/zoom canvas over a design SVG baked at build time. Once
// the wasm-FS demo backend ships, the same hydration will mount <agni-query> /
// <agni-diff> against the in-browser engine, as the notations docs hydrate
// <notation>.
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
