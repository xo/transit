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
| Phase 2, unit 10: the ast module of regex-syntax (ast/mod.rs, ast/parse.rs, ast/visitor.rs) | transit | 2026-09-29 |
| Phase 2, unit 11: the Unicode tables of regex-syntax, and the program that converts them | transit | 2026-09-29 |
| Phase 2, unit 12: the hir module of regex-syntax (hir/mod.rs, hir/interval.rs, hir/translate.rs, unicode.rs) | transit | 2026-09-29 |
| Phase 2, unit 13: pattern.rs and expand_tokens.rs | transit | 2026-09-29 |
| Phase 2, unit 14: the rest of extract_tokens.rs and prepare_grammar.rs, and the check of parse_grammar.rs that a pattern matches the empty string | transit | 2026-09-29 |

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
