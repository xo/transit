# D1. transit is a pure Go port of tree-sitter

Status: Decided, amended by D24.

Ken decided on 2026-09-29 that `github.com/xo/transit` is a pure Go port of
[tree-sitter](https://github.com/tree-sitter/tree-sitter). tree-sitter is a
parser generator and an incremental parsing library. Its runtime is written in
C, and its parser generator and its command line tool are written in Rust.

## What pure Go means

A program that imports transit needs the Go toolchain and nothing else. The
module that a user imports does not use cgo, it does not link a C library, and
it does not run WebAssembly. It builds for every `GOOS` and `GOARCH` that Go
supports, with `CGO_ENABLED=0`.

## What a port means

transit is a translation of the upstream source, not a new design that has the
same purpose. The Go code follows the upstream code closely enough that a
person can port an upstream change by reading its diff. The plan in
[`docs/PLAN.md`](../PLAN.md) and the rules in
[`docs/UPSTREAM.md`](../UPSTREAM.md) say how close.

## What is not decided here

This decision does not say which parts of upstream transit ports, and it does
not say whether a separate test module can use cgo to compare transit with the
C runtime. D7 answers the first, and D12 answers the second.
