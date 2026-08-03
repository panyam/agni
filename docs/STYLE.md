# Documentation style guide

How we write docs in this repo. It applies to both documentation tracks and to the rule
explainers under `check/docs/`. If you are adding or editing a page, read this first.

## Two tracks, split by build-vs-use

Documentation is organized by what the reader is doing, not by their job title.

- **User track: [`docs/userguide/`](userguide/README.md).** For people who *use* the
  engine: run reports, load datasheets, compare revisions, encode a house style. The
  audience is mostly hardware engineers. No Go knowledge assumed.
- **Builder track: the numbered docs (`docs/13` to `docs/24`), and a future `docs/devguide/`
  front door.** For people who *extend* the engine: add a reader, a rule, a render backend.
  Architecture and internals.

When you write a page, decide which track it belongs to and stay at that altitude. A user
page that drifts into engine internals, or a builder page that re-explains the CLI, is in
the wrong track. When unsure, ask before placing it.

## The core principle: map across the audience gap

Every reader arrives fluent in one domain and needs the other explained. Good docs here do
that mapping explicitly, and the direction depends on the track:

- **User track maps software to hardware.** A hardware engineer knows what a resistor is;
  what they lack intuition for is the tool's own software abstractions (an "IR", "tiers",
  "provenance"). Map those back to bench intuition. The canonical example is
  [`userguide/concepts.md`](userguide/concepts.md).
- **Builder track maps hardware to software.** A software engineer knows what middleware is;
  they need the EE nouns decoded. That is what [`ANALOGY.md`](ANALOGY.md) and the "For
  software readers" sections in `check/docs/*.md` do.

`ANALOGY.md` is the shared spine both directions point at. The two tracks are the same table
read from opposite ends, but they are not interchangeable content: pick the direction your
reader needs and commit to it.

## The four-part concept shape

When you introduce a concept the reader has to internalize (not just a command they run),
use a fixed shape so the mapping is the substance, not decoration:

- **What the tool calls it**: the word they will see in the UI and docs.
- **What it's like for you**: the analogy in the reader's own domain.
- **Why it matters**: the practical consequence when they use the tool.

`userguide/concepts.md` uses this throughout; follow it for any new concept entry.

## Commands and output must be real

Never invent a command, a flag, or program output. Every transcript in a page is captured
from the current tool:

- Run the real command against a committed fixture and paste the actual output.
- You may shorten file paths for readability (drop a long `cmd/agni/testdata/...` prefix),
  but never alter the substance of the output.
- If a flag or count could drift, note that `--help` is authoritative and keep the page the
  map, not the source of truth.

This is the same "silence is not a pass" discipline the tool itself follows: a plausible
example is worse than a real one, because a reader will run it.

## Prose rules

These follow the workspace-wide writing style and apply to all pages, commit messages, and
PR text:

- **No em-dashes.** Rewrite as two sentences, or use a plain comma where it genuinely fits.
- **No marketing cadence.** No hype adjectives, no rule-of-three padding, no "not just X but
  Y" framings, no "The result: X" reveals.
- **Plain declarative sentences.** Say the thing.
- Quoted program output is exempt from the prose rules: a transcript is reproduced verbatim,
  em-dashes and all.
- The numbered-doc title separator is a structural label, not prose, so it keeps its dash:
  `# 19 — Rules & checks` and the matching cross-links `[19 — Rules & checks](19-rules-dsl.md)`
  stay as written. The no-em-dash rule applies to sentences.

## Page conventions

- **Relative links** between pages (`[Concepts](concepts.md)`, `../ANALOGY.md`), so they
  work in the repo and on the published site.
- **End a task page with "Where to go next"** pointing at the two or three pages a reader
  most likely wants after this one.
- **Lead the track's flagship with the concept map**, then getting-started, then tasks. A
  reader should be able to follow the pages in order.

## Placement and naming

- **Settled, shippable docs live in this repo** (`Agni`). Research, competitor-format
  comparisons, and strategy live in a separate private repository, referenced only obliquely
  so a link never breaks for an external reader.
- Use `github.com/panyam/agni` for the repo and module path in examples.
