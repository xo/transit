# D42. Grammars that xo writes live in transit

Status: Decided, amended by D104 and D106.

Ken decided on 2026-09-29, answering questions 22 and 43, that transit holds
the grammars that `xo` writes: the usql grammar (D13), MySQL, and the SQL-like
languages that have no grammar, such as SQL++, InfluxQL, PartiQL, ES|QL, PPL
and KSQL (D21).

Each one is a normal tree-sitter grammar, with a `grammar.js`, and a
`src/scanner.c` if it needs one. It lives in `grammars/xo/<name>`. The
upstream tool makes its `grammar.json` and its golden files, so the C backend
test and the tests of the test module apply to it as to any grammar. Its Go
scanner is a port of its own `scanner.c`.

The rule that transit does not write a grammar holds for every grammar that
another project owns. `docs/GRAMMAR.md` holds the rules for both.
