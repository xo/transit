# D119. The Go API gets the forms that the upstream tests need

Status: Decided, amends D103.

Some tests of `crates/cli/src/tests` of upstream test a part of the Rust
binding that the Go API did not have, so they skip or are not ported. The
list in "Choose whether some upstream tests get a Go form" of
`docs/BACKLOG.md` named them.

Ken decided on 2026-10-10, answering question 91, that the Go API gets these
forms, and that their tests are ported:

1. A decode function that the parser calls to read a text in an encoding
   that the runtime does not know, for `test_decode_utf32`,
   `test_decode_cp1252`, `test_decode_macintosh` and `test_decode_utf24le`
   of `parser_test.rs`.
2. The methods `EditPoint` and `EditRange` of `InputEdit` for
   `ts_point_edit` and `ts_range_edit`, as the Rust binding has them, for
   `test_edit_point` and `test_edit_range` of `node_test.rs`.
3. A copy of a query for `ts_query_copy`, for `TestQueryDeepClone`.

The names and the shapes follow the Rust binding in Go idioms (D25), and
`docs/API.md` records them. Ken chose the form of point 2 on 2026-10-10,
answering question 92.
