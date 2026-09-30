# D90. The folder of a package comes from its path

Status: Decided.

In a module with more than one grammar, the folder of each package comes
from the `path` of its entry in `tree-sitter.json`, with each `_` and `-`
removed, so `php_only` gives `phponly`. Ken decided on 2026-09-30 that
`docs/GRAMMAR.md` states the rule this way. The folder is also the name of
the package, as for every grammar but go (D86).
