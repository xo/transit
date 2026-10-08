# Contributing to transit

Before you change anything, read three things:

1. [`AGENTS.md`](AGENTS.md) holds the rules. It is written for a coding agent,
   and every rule in it holds for a person too. `CLAUDE.md` holds one line
   that imports it, for Claude Code.
2. [`docs/PLAN.md`](docs/PLAN.md) holds the plan, and any open question at
   its end.
3. [`docs/decisions/`](docs/decisions/README.md) holds every decision, one
   file each. Read the status of a decision before the decision.

Do not decide an open question on your own. Add it to the end of
`docs/PLAN.md`, and ask Ken.

Ken accepted `docs/API.md` on 2026-10-01, and that ended phase 1 (D103).
Phases 2 to 5 are done. The latest tag of each module is `v0.3.0`. The module
`_example` has no tag. Code follows the plan and the decisions.

## The upstream checkout

transit is a port, and the port is made from a checkout of upstream
tree-sitter in `tree-sitter/`, in the root of the repository. Git ignores it
(D4). If you do not have it, make it in the root of the repository:

```bash
git clone https://github.com/tree-sitter/tree-sitter.git tree-sitter
```

Do not edit a file in it. [`docs/UPSTREAM.md`](docs/UPSTREAM.md) says how to
port upstream code, and [`docs/GRAMMAR.md`](docs/GRAMMAR.md) says how to add a
grammar.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one that the other `xo`
repositories measured. It writes each skill into two folders. Codex and the
other agents read `.agents/skills/<name>`, and Claude Code reads
`.claude/skills/<name>`.

To add a skill or to update one, run the command for it in the repository
root:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

```bash
npx skills@1.7.0 add oborchers/fractional-cto --skill go-pedantry --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and reports nothing. See dbmeta D89.

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` ignores it.

## Before you send a change

Run the commands under "Before you stage" in [`AGENTS.md`](AGENTS.md). Then
stage the change for review. Ken commits.

## Releases

Ken makes each release. Each module has tags of its own (D43, D99). To
release every module at a version such as `v0.2.0`:

1. Run `./gen.sh -r v0.2.0`. Each `go.mod` then requires that version of each
   module of the repository that it requires. The replace blocks stay, so a
   build in the repository still uses the folders.
2. Run the commands under "Before you stage" in `AGENTS.md`, and commit.
3. Tag the commit `v0.2.0` for the root module, and `<folder>/v0.2.0` for
   each other module except `test` and `_example`, such as
   `grammars/json/v0.2.0` and `styles/v0.2.0`. No program imports the test
   module or the module of the sample programs.
4. Push the commit and the tags.
