# internal/projects

This file covers project and design descriptors, the config YAML they bind, and how a project's tiers layer over flags.

- **Every config YAML binds through `internal/yamlpb` against its proto** (agni 827). That covers a design's `intent:`, a project's `conventions:`, a `--conventions` file, and a profile file (`ProfileDef`, or `ProfileNamingMap` for an `override:` file). Keys are proto field NAMES only, never the camelCase JSON spelling. An unknown key fails with its line.
- **A new config file binds the same way.** Go structs with yaml tags are the bug shape, because they drop an unknown key silently.
- **A descriptor NAMES every tier a run reads** (agni 1136). Nothing is found by its folder's name. `project.yaml` and `design.yaml` share one schema, `descriptorYAML` in `internal/projects/descriptor.go`.
- **The schema has sections and path lists.** `conventions:`, `checklists:` and `intent:` are sections. `profiles`, `params`, `symbols`, `lib` and `queries` are lists of paths, each a file or a directory, relative to the descriptor or a `mount://` URI.
- **Only a design names its own files**, through `entry`, `companions`, `revisions` and `flatten_hierarchy`.
- **A design names its project with `extends: ../../project.yaml`**, as a project names shared config. A design is never found under a project by where it sits. A design that names no project, with a `project.yaml` above it, fails to load as a forgotten extends.
- **A design's own sections layer over its project's** (`mergeConfig`). A board can carry its own convention or checklist.
- **A source that names nothing is a load ERROR**, not an absent tier. To reach a tier's off state, delete its line. Tutorial rungs 5, 6 and 11 open with `sed -i.orig '/^params:/d' project.yaml` for this reason.
- **A design folder copied out of its project names a parent the copy lacks.** A test or seed that mounts the folder alone strips its `extends` line (`gatewayAlone`, `gatewayInNoProject`).
- **agni refuses a `--profile-path` that names a profiles directory the project already names** (agni 450). It would load every profile rule twice. A `--params` that names the project's own corpus is only redundant.
- **`queries` names the query sets** (agni 745). `QueryService.ListQuerySets` lists them, the viewer's Sets menu opens them, and `agni query --set <name>` runs them.
- **`lib` names the derived relations** (agni 773). Every query surface and a review manifest's inline queries read them through `Overlay.Registry`.
- **`--checklist` takes a project checklist's NAME or a file ending `.yaml`/`.yml`.** With neither, the project's first checklist runs.
- **`ReviewService.ListChecklists` is the one source of a design's checklists** (agni 859), and `agni checklists <design>` prints it. It lists inherited ones first. The CLI's `--checklist`, the viewer's picker and clients all read it.
- **A client runs a checklist by sending its manifest to `CreateReview`**, which takes values only (C22). `GetReviewManifest` serves a checklist FILE beside a design in no project.
- **Between a project tier and its flag, the project wins** (agni 749). This is the opposite of the mount rule, on purpose, because a project owns its parameters as it owns its profiles. DECISIONS.md records the reversal from wholesale replacement.
- **For the datasheet corpus the project wins PER MPN** (`Overlay.SpecsOver`). The project's `params/` is layered over `--params` or `--params-url` through `param.Layered`.
- **Every datasheet citation names its corpus**, `project` or `shared`. `check.Run` and `RunVerdicts` stamp it through the model. A rule never stamps it.
- **A command that reads a tier from its flag alone is the bug shape** (agni 474). `readDesignWithConfig` returns the overlay that the read already composed. A non-service command takes a tier from that overlay and does not resolve the project a second time.
