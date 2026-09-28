# Backlog

This document lists work that is known and not done. Each item names the
decision or the question that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open
questions for Ken. When an item here is done, delete it, and record in
`decisions/` anything that was decided on the way (dbmeta D110).

## Units in progress

An agent claims a unit of port work here before it starts, with the unit, the
agent and the date, and deletes the claim when Ken commits the unit (D50).

| Unit | Agent | Date |
| --- | --- | --- |
| Phase 1, unit 1: the tests of the agent setup and the documents, CI and the lint configuration (D54) | transit | 2026-09-29 |

## The setup of the repository

These items come in phase 1 and later, as the plan says.

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
