# D71. The package inject takes UTF-8, and a Rust oracle tests it

Status: Decided.

Ken decided on 2026-09-30 two parts of the package `inject` (D27):

1. `inject` takes the text as `[]byte` in UTF-8, as `Parser.Parse` does. The
   highlighter of upstream also parses UTF-16, and `inject` does not. rline
   and usql use UTF-8. An encoding can be added later with no break.
2. A Rust oracle tests the port. The C runtime has no injections, and the
   tests of `crates/highlight` check highlight events, which transit does not
   port. The test module builds a small Rust program with cargo against
   `crates/highlight` of the checkout. The program prints each layer that
   upstream builds, and the test compares those layers with the layers of
   `inject`. The checkout is not edited (hard rule 4).

D72 records the Go API of `inject`.
