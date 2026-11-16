import { defineConfig } from "vitest/config";

// `make try` (agni issue 1600). It plays one journey from demo-video.spec.ts in a visible browser and
// leaves the browser to the reviewer. It needs only the viewer's own `agni serve`, so it starts that
// alone and not the docsite the browser suite also starts.
export default defineConfig({
  test: {
    include: ["browser/demo-video.spec.ts"],
    globalSetup: ["./browser/serve.ts"],
    hookTimeout: 180_000,
  },
});
