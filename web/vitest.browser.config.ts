import { defineConfig } from "vitest/config";

// The browser suite, run on its own and never as part of `make testall`.
//
// It is separate from vitest.config.ts for two reasons rather than one. It needs a real server and a
// real Chromium, neither of which the gate should require: a machine without a browser would turn CI
// red for a reason unrelated to the change under test. And it is slow enough that folding it into
// the 700-test unit run would change what `make test` is for.
//
// globalSetup starts one agni server for the whole run and stops it afterwards. Each spec launches
// its own page (see browser.ts) so no assertion inherits another's scroll position or open drawer.
export default defineConfig({
  test: {
    include: ["browser/**/*.spec.ts"],
    globalSetup: ["./browser/serve.ts"],
    // One at a time. These measure layout in a shared browser and a shared server, and parallel
    // pages competing for CPU make geometry assertions flaky for reasons that have nothing to do
    // with the code under test.
    fileParallelism: false,
    testTimeout: 90_000,
    hookTimeout: 180_000,
  },
});
