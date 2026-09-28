# D25. The Go API follows the Rust binding, in Go idioms

Status: Decided.

Ken decided on 2026-09-29, answering questions 7, 37 and 38:

1. The exported API has the types and the method names of the Rust binding in
   `lib/binding_rust/lib.rs`: `Parser`, `Tree`, `Node`, `TreeCursor`,
   `Query`, `QueryCursor`, `Language`, `LookaheadIterator`, `Point`, `Range`
   and `InputEdit`. It is written in idiomatic Go (D24). The target API of
   phase 1 tests it (D10).
2. Inside the package, tables and arithmetic keep the integer widths of
   upstream, such as `uint32` and `uint16`, so that overflow behaves as in C.
   The exported API uses `int` for byte offsets, rows, columns and counts, and
   converts at the boundary.
3. A sequence, such as the matches or the captures of a query, comes back as
   an `iter.Seq` or an `iter.Seq2`. Stopping a loop early releases what the
   iterator holds.
4. A parse takes a `context.Context`. The parser reads it at the points where
   the C parser calls its progress callback, once in every 100 operations.
