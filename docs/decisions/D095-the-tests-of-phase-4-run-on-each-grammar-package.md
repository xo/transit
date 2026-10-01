# D95. The tests of phase 4 run on each grammar package

Status: Decided.

The tests of phase 3 run on the grammar packages as well as on the C tables.
Ken decided on 2026-10-01:

1. A test that takes one fixture grammar from `fixtureGrammar` runs as
   itself, with the C tables, and again under `TestGoPackageTests` or
   `TestGoPackageParallelTests`, where `fixtureGrammar` gives the grammar
   package. `TestGoPackageTestsAreComplete` fails when a test that uses
   `fixtureGrammar` is missing from their lists.
2. The comparison of the lexers of a grammar package with C uses a lexer
   written in C as the reference, which runs a C lex function in every state
   at one offset in one call. `TestLexStatesMatchesBridge` checks that it
   agrees with the bridge that calls back into Go.
