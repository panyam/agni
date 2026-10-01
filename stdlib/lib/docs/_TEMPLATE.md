## <module.member>

<!--
Per-member doc template for the shipped library (stdlib/lib). Copy this file to docs/<path>.md,
where <path> is the member's full path (e.g. net.has_test_point). lib_test.go requires:
  - every public member of every stdlib/lib module has a doc, and every doc names a member,
  - the file opens with "## <path>".
Files whose name starts with "_" (like this one) are skipped.

Do not restate the rules here. `make catalog-docs` appends a "How it is defined" section to the
reference page with the typed signature, every clause, and a link to the .dl file, all read from
the registered module, so the page cannot drift from the definition. The one-line summary is the
"#" comment above the member's first rule in the .dl file.
-->

### What it is

What one row says, and what each argument is. Name the edge cases the definition includes or
leaves out, since a reader cannot see them from the name.

### For hardware engineers

Why the question matters on a real board, and when in a review you would ask it.

### For software engineers

The shape of the relation as a join or a set: what it is built from, whether rows are unique per
entity, and how negating it behaves.

### Datalog

One or two runnable queries.
