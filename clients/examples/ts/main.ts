// The TypeScript example for `make exercise-clients` (agni issue 1630). It does what
// clients/examples/go does, through @panyam/agni-client: it checks the server, ingests a design, asks
// it three questions, and prints one line of JSON summing up the answers. The exercise compares the
// line with the CLI's answers. docsite/content/build/client-contract.md names the line's fields.
//
//	clients/examples/ts/run -server http://127.0.0.1:8080 -design mount://probe/designs/jetson-cut

import { createAgniClient } from "@panyam/agni-client/node";

// The query every example asks, so their answers compare. The check runs every rule.
const resistorsOf = `component.class(?c, "resistor") => ?c`;

// flags reads Go-style flags, -server <url> and -design <uri>, with one or two dashes.
function flags(args: string[]): { server: string; design: string } {
  const out = { server: "http://127.0.0.1:8080", design: "mount://probe/designs/jetson-cut" };
  for (let i = 0; i < args.length; i++) {
    const [name, inline] = args[i].replace(/^--?/, "").split(/=(.*)/s, 2);
    if (name !== "server" && name !== "design") throw new Error(`unknown flag ${args[i]}`);
    const value = inline ?? args[++i];
    if (value === undefined) throw new Error(`flag -${name} needs a value`);
    out[name] = value;
  }
  return out;
}

async function main(): Promise<void> {
  const { server, design } = flags(process.argv.slice(2));
  // checkServer calls GetServerInfo and refuses a server that does not speak agni.v1.
  const client = createAgniClient(server);
  const info = await client.checkServer();
  const snap = await client.snapshot.ingest({ snapshot: { uri: design } });
  try {
    const d = await client.design.getDesign({ snapshot: snap.name });
    const c = await client.check.checkDesign({ snapshot: snap.name });
    const q = await client.query.runQuery({ snapshot: snap.name, query: resistorsOf });
    console.log(
      JSON.stringify({
        version: info.version,
        api: info.api,
        features: [...info.features].sort(),
        components: d.componentCount,
        nets: d.netCount,
        sheets: d.sheets.length,
        findings: c.findings.length,
        query_rows: q.rows.length,
      }),
    );
  } finally {
    await client.snapshot.deleteSnapshot({ name: snap.name });
  }
}

main().catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
