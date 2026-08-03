// DocsPage is the base client bundle loaded on every docs page.
//
// Today it does nothing beyond a readiness marker. It is the hydration entry
// point for inline interactive elements: once the wasm-FS demo backend lands,
// this file will querySelectorAll the custom playground tags (for example
// <agni-query> or <agni-viewer>) and mount the corresponding component against
// the in-browser engine, mirroring how the notations docs hydrate <notation>.
//
// It is built by build.mjs (esbuild) into static/js/gen/. The site renders
// without it; it is only needed once a page opts into a playground.

window.addEventListener("DOMContentLoaded", () => {
  document.documentElement.setAttribute("data-agni-docs", "ready");
});

export {};
