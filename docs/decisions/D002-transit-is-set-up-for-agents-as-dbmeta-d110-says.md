# D2. transit is set up for coding agents as dbmeta D110 says

Status: Decided.

Ken decided on 2026-09-29 that transit uses the same layout and the same base
rules for coding agents as the other repositories in the `xo` namespace.
dbmeta D110 is the standard, and this decision adopts it.

The repository has these files in its root:

- `README.md`, for a person who finds the project.
- `AGENTS.md`, which holds the rules for a coding agent.
- `CLAUDE.md`, which holds one line, `@AGENTS.md`.
- `CONTRIBUTING.md`, for a person who changes the project.
- `LICENSE`, `go.mod`, `skills-lock.json`, `.gitignore` and `.gitattributes`.

The `docs/` folder holds `PLAN.md`, `BACKLOG.md` and the decisions. The
`simple-english` and `go-pedantry` skills are ordinary folders in
`.agents/skills/` and in `.claude/skills/`, as dbmeta D89 says.

`AGENTS.md` opens with the three standing rules of dbmeta D110. The first
rule is the one that Ken repeated for this repository: stage every change for
review, and commit only when Ken says so.

## What is not here yet

The other `xo` repositories have `skills_test.go` and `docs_test.go`. These
tests make sure that the skills are copies, that the root holds only four
documents, and that every decision reference exists. They are Go code, and D3
holds all code until Ken says the plan is ready. `docs/BACKLOG.md` holds them.
