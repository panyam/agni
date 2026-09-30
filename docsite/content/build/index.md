---
title: "Build on it"
description: "Extend, embed and verify the engine, from new readers and rules to the gate and the evidence habits."
---

These guides are for people extending Agni. Most assume you read Go.

- **[Adding a format reader](format-reader/)** shows how to wire a new EDA format into the neutral IR.
- **[Authoring a check rule](check-rule/)** follows one rule from a checklist item to a shipped rule.
- **[Extending and embedding the engine](extending/)** builds a private module with your own readers and
  house rules, depending on the public engine without forking it.
- **[Calling agni from another language](other-languages/)** covers the typed Python client over the
  CLI or a server, and what the same contract offers any other language.
- **[Native verification](native-verification/)** checks a reader against the format's own EDA
  tool as an oracle.
- **[Running the gate](the-gate/)** says what `make testall` covers, and the three ways it reads
  green when it is not.
- **[Evidence](evidence/)** is about measuring, testing, and the habits that keep a result falsifiable.
