# core/report

This file covers the two HTML reports and the viewer links they carry.

- **There are two HTML reports on different axes, and they share `core/report/style.css`.** `check --format html` is the verdict report. It is rule-major and implies `--verdicts`. `review --format html` is the checklist. It is question-major, in the manifest's order, with every finding per item. Both take `--server`.
- **`ReviewService.RenderReviewReport` builds the checklist page from a review the caller holds** (agni 1030). `review --format html` and the viewer's Review panel Save report both call it.
- **Against a REMOTE server a link is emitted only for a mount you DECLARED.** A mount minted for one run means nothing on a server started without it. agni asks that server through `ListMounts` whether it serves the name from the same root.
- **A withheld link always states its reason, on stderr AND in the report** (agni 626). The reader opens the saved page later, after the terminal is gone.
- **`--server self` reads the design and serves it from one process.** A minted mount is then as linkable as a declared one. The command blocks until Ctrl-C, because the links live as long as the server. `self:PORT` fails on a taken port and does not move.
- **A link names the design's declared ENTRY**, whatever the command was pointed at. It carries the revision the design was read at, and the viewer checks that revision before it draws. Both report rpcs resolve `ReportLinks.design_uri` to the entry themselves (`linkMountPath`, agni 1120), so a client may send the design's folder.
- **A diff link is the older revision's view with `?against=<mount>/<path>`** (`report.DiffURL`, agni 1126). That parameter starts the viewer's Compare as the picker does. `agni diff --server` prints the link, and the MCP `diff` tool returns it. A near-rename diff adds `rename_approx=1`, so the comparison's tables pair renames too.
- **A verdict link carries `rule=`, so the viewer runs ONE rule and not the catalog.** Each click from a report is a fresh page load, so the full catalog would cost seconds per click on a large board. The link mints the rule, since `VerdictID` is generated and never parsed. A link without `rule=` still resolves by the slow route, so older saved reports still work.
- **A verdict link carries the verdict's subjects too** (`subject=net:SDA`, in `check.SubjectParam`'s spelling, agni 897). The viewer asks that rule about those subjects alone. It holds the answer apart from the run in `Document.scoped`.
- **A TRACE link needs no rule hint.** It carries the question, not a conclusion.
- **`guide/checks-and-reports.md` has the link semantics.**
- **A saved report's link waits 2 s for an open viewer to speak, then loads the link** (agni 1156). The viewer posts `agni-viewer/v1/accepted` when it takes a request. The report then waits for the answer, because resolving a verdict link runs its rule.
- **A new note from the page goes on its OWN envelope.** An older saved report reads a reply on `agni-viewer/v1` with no `ok` as a refusal, and reloads.
