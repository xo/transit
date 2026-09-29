# Backlog

This document lists work that is known and not done. Each item names the
decision or the question that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open
questions for Ken. When an item here is done, delete it, and record in
`decisions/` anything that was decided on the way (dbmeta D110).

## Units in progress

An agent claims a unit of port work here before it starts, with the unit, the
agent and the date, and deletes the claim when Ken commits the unit (D50, D61).

| Unit | Agent | Date |
| --- | --- | --- |
| Phase 2, unit 23: the candidate set of the golden harness, and its first run over the whole set | transit | 2026-09-29 |

## The generator

### Run the corpus of each grammar on the C output

A grammar counts toward the gate of D9 only when the `parser.c` of the C
backend passes the corpus of the grammar with the upstream C runtime, with
the same result as `tree-sitter test` ("The gate for the Go backend" in
[`PLAN.md`](PLAN.md)). On 2026-09-29 the C backend writes the golden files of
all 185 grammars of the record, so this is the condition that is left. Build
the step in the test module (D12): compile the `parser.c` and the scanner of
each grammar with the upstream runtime, run its corpus, and compare the
result with what the upstream tool reports.

### Port the log of the generator

Upstream writes log lines with `debug!` and `info!` in `build_tables/`, and
`report_state_info` in `build_tables.rs` writes the parse states of one rule
for the option `--report-states-for-rule` of the tool. The generator has no
logger yet, so the port leaves them out. Decide how the generator logs, then
port the lines and `report_state_info`. Some functions of
`build_lex_table.go` then take the `StrPool` again, as upstream does.

## The setup of the repository

These items come in phase 1 and later, as the plan says.

### Add the script that writes the replace blocks

With the first module that imports another module of this repository, add
the script that writes the `replace` block of each `go.mod`, and a CI step
that fails when the script changes a file (D49).

### Add the tests of the port rules

[`UPSTREAM.md`](UPSTREAM.md) and [`GRAMMAR.md`](GRAMMAR.md) name tests that
hold their rules:

1. Each folder in `grammars/` has an entry in `grammars/grammars.json`, and
   each entry has a folder.
2. Each ported function has a doc comment that names its C function or its
   Rust function.
