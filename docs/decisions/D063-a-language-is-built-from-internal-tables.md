# D63. A language is built from tables of an internal type

Status: Decided.

Ken decided on 2026-09-29, answering question 59, how a `*transit.Language`
is built. The package `github.com/xo/transit/internal/abi` holds the tables
of a grammar in the shape of `TSLanguage` of `parser.h`. The root package
exports `NewLanguage(*abi.Language) *Language`.

Go lets code import an `internal` package only under the path of its parent,
so only code under `github.com/xo/transit/` can build the argument. That code
is:

1. The grammar packages that the Go backend writes, in the modules of
   `grammars/` (D43).
2. The test module in `test/`, which fills the tables from a C grammar
   through cgo in phase 3 (D12).

A program outside transit can call `NewLanguage`, but it cannot build a
table, so the form of the tables is not part of the public API. Phase 4 can
change that form (D31) without a change of the API of `docs/API.md`.
