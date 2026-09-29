# D66. An error of grammar.json keeps the text of Go

Status: Decided.

Ken decided on 2026-09-29, answering question 58, that the generator gives
the text of Go's `encoding/json` for a `grammar.json` that is not valid JSON.
This is a deliberate difference from upstream (hard rule 6).

Upstream decodes `grammar.json` with the Rust crate `serde_json`, and its
error gives the text of that crate, such as
``expected `,` or `}` at line 3 column 5``. `ParseGrammar` gives the text of
`encoding/json` in its place. The upstream tool writes `grammar.json` itself,
so the error comes only from a file that a person edits by hand, and no golden
file holds such an error.
