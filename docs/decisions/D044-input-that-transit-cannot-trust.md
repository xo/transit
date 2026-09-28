# D44. Input that transit cannot trust

Status: Decided.

Ken decided on 2026-09-29, answering question 36:

1. `Parser.Parse` stops at the deadline of its context, as the C parser stops
   at its timeout.
2. A panic in a scanner or in the runtime is a fault. The fuzz tests look for
   it. The public API does not hide it with `recover`.
3. A test in the test module parses input that nests 10,000 levels deep for
   each grammar. The Go runtime must not overflow where the C runtime does
   not.
