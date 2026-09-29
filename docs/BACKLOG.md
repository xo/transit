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
| Phase 2, unit 2: nfa.rs, and LexicalGrammar of grammars.rs | transit | 2026-09-29 |

## The generator

### Port the parser and the translator of regex-syntax

Port the parts of `regex-syntax` 0.8.11 that the generator calls, with their
tests and their Unicode tables, to an internal package of `generate` (D59,
D60). `pattern.rs` and `expand_tokens.rs` need it.

### Use the port in parse_grammar.go

`ParseGrammar` asks Go's `regexp` whether a pattern matches the empty string,
where upstream asks the Rust crate `regex`. When the port of `regex-syntax`
exists, use it there (D59).

## The setup of the repository

These items come in phase 1 and later, as the plan says.

### Add the script that writes the replace blocks

With the first module that imports another module of this repository, add
the script that writes the `replace` block of each `go.mod`, and a CI step
that fails when the script changes a file (D49).

### Run the golden harness on the rest of the set

The first run covered the test grammars and the fixture grammars (D58). Add
the other grammars of `CANDIDATES.md` to the harness, with their sets, and
run it on them. A grammar that commits no `src/grammar.json`, such as
`DerekStride/tree-sitter-sql`, needs the path of D51 that runs `grammar.js`.

### Add the tests of the port rules

[`UPSTREAM.md`](UPSTREAM.md) and [`GRAMMAR.md`](GRAMMAR.md) name tests that
hold their rules:

1. Each folder in `grammars/` has an entry in `grammars/grammars.json`, and
   each entry has a folder.
2. Each ported function has a doc comment that names its C function or its
   Rust function.
