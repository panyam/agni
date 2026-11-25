// An MCP client for the viewer room specs, spoken by hand so they need no SDK. The "session" wire
// initializes and sends its session id. The "stateless" wire is the 2026-07-28 one, and it sends the
// protocol version on every request instead. A token, when given, goes on every request as a bearer
// token, so the agent acts for the person who signed in for it.

export type CallResult = { isError?: boolean; content?: { text?: string }[]; structuredContent?: { location?: { mount: string; path: string; sheet: string } } };

export const text = (r: CallResult): string => (r.content ?? []).map((c) => c.text ?? "").join("");

export class Agent {
  private session = "";
  private id = 0;
  constructor(
    private readonly url: string,
    readonly wire: "session" | "stateless",
    private readonly token = "",
  ) {}

  async connect(): Promise<void> {
    if (this.wire === "stateless") return;
    const res = await this.post("initialize", { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "spec-agent", version: "0" } });
    this.session = res.headers.get("mcp-session-id") ?? "";
    if (this.session === "") throw new Error("the server gave the session no id");
    await this.post("notifications/initialized", undefined, true);
  }

  async listTools(): Promise<string[]> {
    const r = (await this.rpc("tools/list", {})) as { tools: { name: string }[] };
    return r.tools.map((t) => t.name);
  }

  async call(name: string, args: Record<string, unknown>): Promise<CallResult> {
    return (await this.rpc("tools/call", { name, arguments: args })) as CallResult;
  }

  private async rpc(method: string, params: Record<string, unknown>): Promise<unknown> {
    const res = await this.post(method, params);
    const body = await res.text();
    // A streamable HTTP answer is JSON or one SSE event carrying it.
    const json = res.headers.get("content-type")?.includes("text/event-stream")
      ? body.split("\n").filter((l) => l.startsWith("data:")).map((l) => l.slice(5).trim()).pop() ?? ""
      : body;
    const msg = JSON.parse(json) as { result?: unknown; error?: { message: string } };
    if (msg.error) throw new Error(`${method}: ${msg.error.message}`);
    return msg.result;
  }

  private async post(method: string, params: Record<string, unknown> | undefined, notification = false): Promise<Response> {
    const headers: Record<string, string> = { "content-type": "application/json", accept: "application/json, text/event-stream" };
    if (this.token) headers.authorization = `Bearer ${this.token}`;
    if (this.session) headers["mcp-session-id"] = this.session;
    let p = params;
    if (this.wire === "stateless") {
      // The stateless wire routes on headers as well as the body (SEP-2243).
      headers["mcp-protocol-version"] = "2026-07-28";
      headers["mcp-method"] = method;
      if (method === "tools/call" && typeof params?.name === "string") headers["mcp-name"] = params.name;
      p = {
        ...params,
        _meta: {
          "io.modelcontextprotocol/protocolVersion": "2026-07-28",
          "io.modelcontextprotocol/clientInfo": { name: "spec-agent", version: "0" },
          "io.modelcontextprotocol/clientCapabilities": {},
        },
      };
    }
    const body = notification ? { jsonrpc: "2.0", method } : { jsonrpc: "2.0", id: ++this.id, method, params: p };
    const res = await fetch(this.url, { method: "POST", headers, body: JSON.stringify(body) });
    if (!res.ok && res.status !== 202) throw new Error(`${method}: HTTP ${res.status} ${await res.text()}`);
    return res;
  }
}
