import { defineConfig } from "vitest/config";

// The browser suite. It runs inside `make testall`, and has its own config rather than its own gate.
//
// It is separate from vitest.config.ts because it needs a real server and a real Chromium, which the
// unit run must not: `make test` has to stay something you can run on any machine in a few seconds.
// Separate config, same gate.
//
// It was outside the gate until PR 629, on the argument that a machine without a browser would
// go red for a reason unrelated to the change. v0.2.0 then shipped a viewer whose query surface
// booted hidden behind the Trace tab. The textarea was in the DOM throughout, so jsdom saw nothing
// wrong, and this suite caught it only after the tag was pushed. The browser download is now a CI
// step and a one-time cost per developer, which is cheaper than the class of bug it catches.
//
// globalSetup starts the servers the specs need and stops them afterwards: `serve.ts` runs the
// viewer's own `agni serve`, and `docsite.ts` runs the docsite, which is a separate Go module and a
// separate program. Each spec launches its own page (see browser.ts) so no assertion inherits
// another's scroll position or open drawer.
export default defineConfig({
  test: {
    include: ["browser/**/*.spec.ts"],
    globalSetup: ["./browser/serve.ts", "./browser/docsite.ts"],
    // One at a time. These measure layout in a shared browser and a shared server, and parallel
    // pages competing for CPU make geometry assertions flaky for reasons that have nothing to do
    // with the code under test.
    fileParallelism: false,
    testTimeout: 90_000,
    hookTimeout: 180_000,
  },
});
