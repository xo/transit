# D84. A module copies the queries of another grammar that it lists

Status: Decided, amends D80.

The `tree-sitter.json` of tree-sitter-typescript and of tree-sitter-cpp
lists query files of another grammar, such as
`node_modules/tree-sitter-javascript/queries/highlights.scm` and
`node_modules/tree-sitter-c/queries/highlights.scm`, before its own. Ken
decided on 2026-09-30 that a grammar module copies each such file from the
checkout of the other grammar, at the tag that `grammars/grammars.json`
records for that grammar, into `queries/<other grammar>/` of the package. The
package embeds the files, and the highlight test of D80 reads them in the
order of the list.
