# D41. cmd/transit generates, tests, parses and queries

Status: Decided, amends D7.

Ken decided on 2026-09-29, answering question 21, that `cmd/transit` has four
subcommands:

1. `generate` runs the generator with a backend.
2. `test` runs the corpus of a grammar and prints the same report as
   `tree-sitter test`.
3. `parse` parses a file and prints its tree.
4. `query` runs a query on a file and prints the captures.

This amends D7, which ported only a command that runs the generator. The
subcommands port `crates/cli/src/test.rs`, `parse.rs` and `query.rs`.
