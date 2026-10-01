# D10. The target Go API comes from a working C example

Status: Decided.

Ken decided on 2026-09-29 that part of the first work is to write the target
API of a generated Go parser: what a Go package for a grammar exports, and
what the runtime exports beside it. The target comes from a working C example.
The example is a C program that uses the upstream C runtime and a real grammar
that the upstream tool generates.

Ken added a first sample, `_samples/sample.c`, on 2026-09-29. It parses a SQL
statement with `tree_sitter_sql`, runs a highlight query, and prints each
capture with its byte range. [`docs/PLAN.md`](../PLAN.md) says what the
working example must add to it, so that it covers both highlighting and
completion. Ken removed the sample on 2026-10-01, after the working example
in `_samples/example/` replaced it.
