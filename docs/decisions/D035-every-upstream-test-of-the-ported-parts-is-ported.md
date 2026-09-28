# D35. Every upstream test of the ported parts is ported

Status: Decided.

Ken decided on 2026-09-29, answering question 14, that every upstream test of
the parts that transit ports is ported: the runtime, the query engine, the
predicates, the injections and the generator. A test that cannot be ported,
such as a test of WebAssembly or of highlighting, is listed with the reason.
