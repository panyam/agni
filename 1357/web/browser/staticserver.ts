// A plain file server for a built static site, as a static host answers it: a path under the prefix
// maps into the folder, a folder serves its index.html, and anything else is a 404. site.spec.ts and
// the public demo exercise (exercise.mjs) both serve `agni site` output with it.
//
// It imports only Node builtins and uses only erasable TypeScript, so exercise.mjs can import it
// with Node's own type stripping.

import { createReadStream, existsSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize } from "node:path";

const types: Record<string, string> = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".wasm": "application/wasm",
};

export interface StaticServer {
  // base is the site's URL, ending in the prefix.
  base: string;
  close(): void;
}

// serveStatic serves dir under prefix on a port the kernel picks, so it never collides with a dev
// server already running.
export async function serveStatic(dir: string, prefix: string): Promise<StaticServer> {
  const server = createServer((req, res) => {
    const path = decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname);
    if (!path.startsWith(prefix)) {
      res.writeHead(404).end();
      return;
    }
    let file = normalize(join(dir, path.slice(prefix.length)));
    if (!file.startsWith(dir)) {
      res.writeHead(404).end();
      return;
    }
    if (existsSync(file) && statSync(file).isDirectory()) file = join(file, "index.html");
    if (!existsSync(file)) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
    createReadStream(file).pipe(res);
  });
  await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
  const addr = server.address();
  const port = typeof addr === "object" && addr ? addr.port : 0;
  return { base: `http://127.0.0.1:${port}${prefix}`, close: () => server.close() };
}
