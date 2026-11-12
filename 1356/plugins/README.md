# Agent skills

`agni/` is the plugin that carries agni's agent skills. Each skill is a folder under
`agni/skills/` holding a `SKILL.md`, in the [Agent Skills](https://agentskills.io/specification)
format, and a skill reads only the installed `agni` CLI, so it works without a checkout.

The same folder reaches three kinds of installer.

- Claude Code reads `.claude-plugin/marketplace.json` at the repo root and installs the plugin with
  `claude plugin marketplace add panyam/agni`, then `claude plugin install agni@agni`.
- Cursor reads `.cursor-plugin/marketplace.json` when a team imports the repo as a marketplace.
- `gh skill install panyam/agni <skill> --agent <agent>` finds any `skills/*/SKILL.md` and copies one
  skill into an agent's own skills folder. The agent can be Copilot, Cursor, Claude Code or another
  one. The command resolves the latest release tag, so a new skill reaches it with the next release
  rather than on merge.

The user-facing instructions are in `docsite/content/guide/asking-with-an-llm.md`.

## Adding a skill

Create `agni/skills/<name>/SKILL.md`, with frontmatter giving the skill's `name` (its folder's name,
lowercase kebab-case) and a `description` of up to 1024 characters saying what it does and when an
agent should use it. Nothing else needs an edit, since every installer discovers the folder.
`plugins_test.go` checks that layout. It fails on a `SKILL.md` placed anywhere else in the repo,
since no installer would find one there.

A skill runs on someone's own board, so it never asks the agent to paste net names, ref-des or part
numbers into an issue or anywhere outside their workspace, and its closing report holds counts and
shapes only.

No manifest carries a `version`. The git tag is agni's only version, and without one Claude Code
tracks the plugin's updates by commit.
