import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";

// Scope vitest to the unit tests under src/ (stray specs outside src/ would fail to collect
// and turn `pnpm test` red despite every unit test passing).
//
// The solid plugin compiles .tsx islands with Solid's JSX transform (vitest's default esbuild
// transform would produce React-style calls that break reactivity). solid-js and the island
// wrapper are inlined so the tests and the components share ONE reactive core — externalizing
// them recreates the two-cores setState-no-op bug inside the test runner itself (see the
// solidAlias note in build.mjs).
export default defineConfig({
  plugins: [solid()],
  resolve: { conditions: ["development", "browser"] },
  test: {
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    server: { deps: { inline: [/solid-js/, /@panyam\/tsappkit-solid/] } },
  },
});
