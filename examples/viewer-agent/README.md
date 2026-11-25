# viewer-agent

An agent that answers a reader's question in their own viewer tab, and acts on the step the reader
accepts (agni issue 1907). Its thinking is scripted, so it shows the same thing every run. The calls
it makes are the ones an LLM agent makes through agni's `/mcp`.

## What it shows

- Hearing a question from the tab's Ask box with `next_request`, which names the design and the tab
  it came from.
- Asking the design with agni's `query` tool, as a model would.
- `viewer_show_answer`, which puts the answer under the Ask box with a button for each part it
  names. The reader clicks one to highlight it.
- `viewer_propose`, which shows a next step with Accept and Dismiss. The reader's click comes back
  as the event `page.proposal_answered`, and the agent then does what it proposed.

## Run it

```bash
make -C ../.. agni ui
../../bin/agni serve --mount kicad=../../readers/kicad/testdata --mcp --viewer-rooms
```

Open <http://127.0.0.1:8080/mounts/kicad/hier_numeric_root.kicad_sch/view> and turn on "Let agents
drive this tab". Then, in another terminal:

```bash
go run . --design kicad/hier_numeric_root.kicad_sch
```

Type any question in the tab's Ask box. The agent answers that the design has 4 parts, lists them,
and proposes highlighting R1. Accept, and R1 lights up.

On a server run with `--auth tokens --local-accounts`, the agent signs in as you. The server's
local sign-in answers with a token, and `--token` or `AGNI_TOKEN` passes it on:

```bash
export AGNI_TOKEN=$(curl -s http://127.0.0.1:8080/signin/local -H 'content-type: application/json' \
  -d '{"username":"ada","password":"'"$PASSWORD"'"}' | jq -r .token)
```

The agent then reaches your own tabs, and another person's tab only after they pair it.

`go test` runs one round against `../../bin/agni` with a tab played in Go. The browser exercise
`make exercise-viewer-rooms` runs it against a real tab, with `--once`.

## The same loop with Claude Code

Claude Code makes the agent unscripted. Register agni and mcpkit's `rooms-channel` in `.mcp.json`
as [Asking through an LLM](https://panyam.github.io/agni/guide/asking-with-an-llm/) shows, start
`claude --dangerously-load-development-channels server:rooms`, and tell it:

> Attach to kicad/hier_numeric_root.kicad_sch. When I ask a question in the viewer, answer it with
> agni's tools, show the answer in my tab with viewer_show_answer, and propose one next step with
> viewer_propose. Do the step when I accept it.

Each question you type in the tab then arrives as a turn. Your choice on a proposal arrives through
`next_request` or the room's events, as `page.proposal_answered`.
