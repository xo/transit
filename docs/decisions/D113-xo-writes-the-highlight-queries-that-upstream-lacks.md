# D113. xo writes the highlight queries that upstream lacks

Status: Decided.

The repositories of `Crary-Systems/tree-sitter-tsql`,
`andreasmaierde/tree-sitter-plsql` and `shotover/tree-sitter-cql` have no
highlight query. So the SQL of the dialects `sqlserver`, `oracle` and `cql`
had no colors, and only the layer of the usql grammar had them.

Ken decided on 2026-10-08, answering question 82:

1. xo writes a `queries/highlights.scm` for `grammars/sqlserver`,
   `grammars/oracle` and `grammars/cql`, from the node types of each
   grammar, with the standard capture names of `styles/captures.txt`.
2. The query is a file that a person writes, and the first line of its
   comment says that xo wrote it. No file copied from the repository of the
   grammar changes.
3. The same rule holds for any other grammar of the set that has no
   highlight query.
