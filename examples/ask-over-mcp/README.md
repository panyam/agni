# ask-over-mcp

Ask a design questions the way an LLM client does, through agni's MCP server. It starts `agni mcp`
over a folder, connects mcpkit's client, and makes the calls a model would make, printing what the
model reads back.

## What it shows

- `agni mcp` serving the query language's tools (from jaala) and agni's own (`check`, `trace`,
  `diff`, `design`, `review` and the rest) over stdio, the way Claude Code starts it.
- An answer that opens with the question that produced it, carries each row's provenance, and says
  why it has no viewer link when the server has no viewer to link to.
- The order a model usually asks in: find the design, look up a relation, ask, follow a signal,
  compare two revisions.

## Run it

```bash
make -C ../.. agni   # the walkthrough runs ../../bin/agni when it exists
make run             # plain text, interactive
make runquiet        # non-interactive defaults (CI-safe)
```

`AGNI_BIN` names another agni to run. The default folder is `../tutorial-project`; give the first
step another folder, or a design file, to ask about your own.

See [Asking through an LLM](https://panyam.github.io/agni/guide/asking-with-an-llm/) for pointing a
real client at agni.
