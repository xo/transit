# D8. The generator has pluggable backends, C first, then Go

Status: Decided.

Ken decided on 2026-09-29 that the transit generator has pluggable backends,
and upstream has none. A backend takes the tables and the grammar that the
generator builds, and it writes a parser in one target language. Upstream
writes only C, from `render.rs`.

transit has two backends, and they are written in this order:

1. The C backend. It is a port of `render.rs`.
2. The Go backend. It writes a Go package for a grammar. D9 says when it
   starts.

## Why C comes first

The port is tested against many open source grammars. For each grammar, the
upstream tool makes a golden `parser.c` from the original grammar. The C
backend of transit must write the same file for the same grammar. This test
shows whether the port of the generator is correct before any Go output
exists, because `parser.c` holds every table and the whole lexer.

## What the backend takes

The backend interface is not shaped like C. The C backend must reproduce
upstream byte for byte, but the Go backend must be free to write Go that suits
Go. The interface takes the data that `render.rs` reads, in the order that
upstream builds it, and not the C arrays that `render.rs` writes. The plan in
[`docs/PLAN.md`](../PLAN.md) describes it.
