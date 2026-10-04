import { defineConfig } from "vitest/config";

// The static demo's spec on its own, for the docs workflow (agni issue 856), which builds the demo
// into the Pages artifact and checks those files before uploading them. It starts none of the
// servers vitest.browser.config.ts does, because a static site has nothing to answer but files.
// Point AGNI_SITE_DIR at the built demo; without it the spec builds its own, as in the gate.
export default defineConfig({
  test: {
    include: ["browser/site.spec.ts"],
    testTimeout: 90_000,
    hookTimeout: 300_000,
  },
});
