# D78. A large character set of surrogates is an error

Status: Decided.

A large character set that the lexer uses, and that holds only surrogate
code points, makes the C code that upstream writes read past the end of its
array. Upstream has no defined behavior there. The Go backend stops with the
error `RenderErrorCharacterSet` for such a set. Ken decided on 2026-09-30 to
keep the error. This is a deliberate difference from upstream (hard rule 6).
No grammar of the set has such a set on 2026-09-30.
