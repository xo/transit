# D104. A grammar that xo writes lives in grammars

Status: Decided, amends D42.

D42 put each grammar that xo writes in `grammars/xo/<name>`. Ken decided on
2026-10-01 that such a grammar needs no folder of its own:

1. A grammar that xo writes is a module `grammars/<name>`, beside the
   modules of the grammars of other repositories. The usql grammar is the
   module `github.com/xo/transit/grammars/usql`.
2. Its name must not be the name of a module of another repository in
   `grammars/`.
3. Its packages hold their `grammar.js`, and the module holds a
   `tree-sitter.json`. A module of another repository holds no `grammar.js`,
   so the set `xo` of the golden harness finds the grammars that xo writes
   this way.
