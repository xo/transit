# D83. A corpus case runs in the package of its grammar

Status: Decided, amends D79.

A repository with two grammars, such as tree-sitter-php with php and
php_only, or tree-sitter-typescript with typescript and tsx, holds one
corpus for both. Ken decided on 2026-09-30 this rule for the corpus test of
such a module:

1. Each package of the module holds the whole copied corpus.
2. A case runs in the package of the grammar that its `:language` names. A
   case with no `:language` runs in the package of the first grammar of
   `tree-sitter.json`, as the upstream tool does.
3. The other packages of the module skip the case, and name the package
   that runs it.
4. A `:language` that no grammar of the module has fails with "Language not
   found", as D79 says.
