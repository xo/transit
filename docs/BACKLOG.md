# Backlog

This document lists work that is known and not done. Each item names the
decision or the question that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open
questions for Ken. When an item here is done, delete it, and record in
`decisions/` anything that was decided on the way (dbmeta D110).

## Units in progress

An agent claims a unit of port work here before it starts, with the unit, the
agent and the date, and deletes the claim when Ken commits the unit (D50). No
unit is claimed on 2026-09-29, because no code is written yet (D3).

## The setup of the repository

These items wait for Ken to say that the plan is ready, because each one is
code (D3).

### Add the tests of the agent setup and the documents

Every other `xo` repository has `skills_test.go` and `docs_test.go` (dbmeta
D110). Copy them from dbmeta or resvg, and change them for the layout here.
They make sure of these facts:

1. Each skill is an ordinary folder in `.agents/skills` and in
   `.claude/skills`, and the two copies are the same (`TestSkillsAreCopies`).
2. `CLAUDE.md` holds `@AGENTS.md` and nothing else
   (`TestClaudeImportsAgents`).
3. The root holds only `README.md`, `AGENTS.md`, `CLAUDE.md` and
   `CONTRIBUTING.md` as Markdown files (`TestTheRootHoldsFourDocuments`).
4. Every document in `docs/` is in the table of `AGENTS.md` and in the list of
   `README.md` (`TestEveryDocumentIsInBothTables`).
5. Every row of `docs/decisions/README.md` matches its file, and every file
   has a row (`TestTheDecisionIndexIsComplete`).
6. An amendment names the decision that it amends, and that decision names it
   back (`TestAnAmendmentPointsBothWays`).
7. Every bare decision number, such as D3, names a decision that exists
   (`TestEveryDecisionReferenceExists`).

The repository has no Go package yet, and `go test ./...` fails on a module
that has no package. These tests are the first package.

### Add CI and the lint configuration

Add `.github/workflows/test.yml` and `.golangci.yml` with the first Go
package. The workflow also runs the test module in `test/`, which needs a C
compiler (D12). D36 holds the details.

### Add the script that writes the replace blocks

With the first module that imports another module of this repository, add
the script that writes the `replace` block of each `go.mod`, and a CI step
that fails when the script changes a file (D49).

### Add the tests of the port rules

[`UPSTREAM.md`](UPSTREAM.md) and [`GRAMMAR.md`](GRAMMAR.md) name tests that
hold their rules:

1. The ledger has one line for each upstream commit, with no gap (D30).
2. Each folder in `grammars/` has an entry in `grammars/grammars.json`, and
   each entry has a folder.
3. The test data of the generator names the commit that transit ports.
4. Each ported function has a doc comment that names its C function or its
   Rust function.
