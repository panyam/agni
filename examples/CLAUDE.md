# examples

This file covers how examples are written, and how to point them at a board this repo cannot carry.

- **When you build a feature, ship an example** (CONSTRAINTS C10). `examples/CONVENTIONS.md` has the how-to. `examples/tutorial-project/README.md` describes the fixture that the docsite tutorial runs on.
- **`AGNI_EXAMPLE_DESIGN` points every example at a board that this repo cannot carry.** Each example asks for its design through `common.AskPath`, which defaults to a bundled synthetic fixture.
- The variable replaces that DEFAULT. The prompt still shows it, and a typed path still wins. `--non-interactive`, `--record` and `--replay` pick it up, and they cannot pick up a path typed at the prompt. A blank value counts as unset.
- **A demo over someone's real board is an example driven this way**, not a script of its own. `design-review` is the checklist walk, `dft-coverage` the coverage one and `whole-enchilada` the tour. A bash script of beats duplicates demokit and produces no recording.
- **Run an example over a REAL board before trusting it** (agni 644). The fixtures are small enough to hide scale bugs, such as a list of every ref-des in a bucket.
- **`AGNI_EXAMPLE_REVIEW` does the same for a review manifest.** `design-review` needs one beside the design, and a team's checklist is as unshippable here as their board.
- **Clear both variables in a package's `TestMain`.** They replace the defaults the tests assert against. A developer who exports either one otherwise fails the gate locally, and the failure prints their path into the log.
