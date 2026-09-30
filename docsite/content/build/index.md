---
title: "Build on it"
description: "Extend the engine: readers, rules, overlays."
---

These guides are for people extending Agni. They assume you read Go.

- **[Adding a format reader](format-reader/)**: wire a new EDA format into the neutral IR.
- **[Authoring a check rule](check-rule/)**: from a checklist item to a shipped rule.
- **[Extending and embedding the engine](extending/)**: a private module with your own readers and house rules,
  depending on the public engine without forking it.
- **[Calling agni from another language](other-languages/)**: the typed Python client over the CLI
  or a server, and what the same contract offers any other language.
- **[Native verification](native-verification/)**: check a reader against the format's own EDA
  tool as an oracle.
- **[Running the gate](the-gate/)**: what `make testall` covers, and the three ways it reads green
  when it is not.
- **[Evidence](evidence/)**: measuring, testing, and the habits that keep a result falsifiable.
