// The agni binary a spec runs to compare the page against the CLI.
//
// serve.ts builds it once per run and provides it. A config with no such setup (vitest.site.config.ts,
// which tests a prebuilt demo) gets one built here instead, once per worker. Either way it is built
// with the developer's own environment, never under a spec's empty HOME, which would point Go's build
// cache at an empty directory and rebuild agni on every call (see serve.ts).

import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { inject } from "vitest";

const repo = resolve(fileURLToPath(new URL("../..", import.meta.url)));
let built: string | undefined;

export function agniBin(): string {
  const given = inject("agniBin");
  if (given) return given;
  if (!built) {
    built = join(mkdtempSync(join(tmpdir(), "agni-spec-bin-")), "agni");
    execFileSync("go", ["build", "-o", built, "./cmd/agni"], { cwd: repo, stdio: "inherit" });
  }
  return built;
}
