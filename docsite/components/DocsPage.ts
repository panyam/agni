// DocsPage is the base client bundle for a page that opts into a playground
// with the front-matter `playground: viewer`. esbuild emits the
// gen.DocsPage.html include, and BasePage.html injects the bundle through it.
//
// It hydrates the inline interactive tags on the page. Today that is
// <agni-viewer>, a pan/zoom canvas over a design SVG baked at build time. The
// in-browser engine has shipped (mission #851), and the docsite does not use it
// yet. The planned next step is the same hydration mounting <agni-query> /
// <agni-diff> against that engine, as the notations docs hydrate <notation>.
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
