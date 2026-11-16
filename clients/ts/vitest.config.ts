import { defineConfig } from "vitest/config";

// The wire-fixture replayer starts and stops one agni serve, and builds agni first unless AGNI_BIN
// names a binary, so its hooks get minutes rather than vitest's default seconds.
export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    hookTimeout: 300_000,
    testTimeout: 60_000,
  },
});
