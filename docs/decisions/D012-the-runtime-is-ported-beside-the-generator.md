# D12. The runtime is ported beside the generator, and a cgo test module tests it

Status: Decided.

Ken decided on 2026-09-29 that the runtime is ported and tested before the Go
backend exists, at the same time as the generator. A test module makes this
possible.

## The test module

The test module is in `test/`, and it has its own `go.mod`. No user imports
it. It can use cgo, and it links the upstream C runtime and C grammars that
the upstream tool generates. The root module stays pure Go (D1).

The test module does two things:

1. It drives the Go runtime with the tables of a C grammar. It copies the
   tables out of the C `TSLanguage`, and it calls the C lexer and the C
   external scanner through cgo. The Go runtime can then parse real grammars
   before the Go backend writes any.
2. It parses the same input with the C runtime and with the Go runtime, and
   it compares the two trees.

When the Go backend exists, the same tests run on the Go grammars as well.
