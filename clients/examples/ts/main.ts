// The TypeScript example for `make exercise-clients` (agni issue 1630). It does what
// clients/examples/go does, through @panyam/agni-client: it checks the server, ingests a design, asks
// it three questions, and prints one line of JSON summing up the answers. The exercise compares the
// line with the CLI's answers. docsite/content/build/client-contract.md names the line's fields.
//
//	clients/examples/ts/run -server http://127.0.0.1:8080 -design mount://probe/boards/jetson-cut

import { create, merge, type AgniClient } from "@panyam/agni-client";
import { BoardGeometrySchema, type BoardGeometry } from "@panyam/agni-client/gen/agni/v1/geom/geom_board_pb";
import { createAgniClient } from "@panyam/agni-client/node";
import { readFileSync } from "node:fs";

// The query every example asks, so their answers compare. The check runs every rule.
const resistorsOf = `component.class(?c, "resistor") => ?c`;

// boardPageBytes is small, so the cut's 51 KB board comes in several pages and every example pages.
const boardPageBytes = 16n << 10n;

// boardGeometry reads every page of the snapshot's board and merges them, as
// docsite/content/build/board-geometry.md says a client does.
async function boardGeometry(client: AgniClient, snapshot: string): Promise<BoardGeometry> {
  const board = create(BoardGeometrySchema);
  let pageToken = "";
  do {
    const page = await client.designView.getBoardGeometry({ snapshot, maxBytes: boardPageBytes, pageToken });
    if (page.board) merge(BoardGeometrySchema, board, page.board);
    pageToken = page.nextPageToken;
  } while (pageToken !== "");
  return board;
}

// flags reads Go-style flags, -server <url> and -design <uri>, with one or two dashes.
function flags(args: string[]): { server: string; design: string } {
  const out = { server: "http://127.0.0.1:8080", design: "mount://probe/boards/jetson-cut" };
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
  // A shared server asks every call for a token (agni issue 1627), passed in AGNI_TOKEN, and its
  // certificate may come from a team's own CA (agni issue 1765), passed in AGNI_CA.
  const ca = process.env.AGNI_CA ? readFileSync(process.env.AGNI_CA) : undefined;
  const client = createAgniClient(server, { token: process.env.AGNI_TOKEN || undefined, ca });
  const info = await client.checkServer();
  const snap = await client.snapshot.ingest({ snapshot: { uri: design } });
  try {
    const d = await client.designView.getSummary({ snapshot: snap.name });
    const c = await client.check.checkDesign({ snapshot: snap.name });
    const q = await client.query.runQuery({ snapshot: snap.name, query: resistorsOf });
    const board = await boardGeometry(client, snap.name);
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
        board_layers: board.layers.length,
        board_parts: board.placements.length,
        board_tracks: board.nets.reduce((n, net) => n + net.segments.length, 0),
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
