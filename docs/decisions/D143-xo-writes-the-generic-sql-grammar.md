# D143. xo writes the generic SQL grammar

Status: Decided, amends D106.

usql uses the grammar of a dialect only to highlight its syntax. The usql
grammar splits the statements. usql gives every dialect that has no grammar
of its own the generic SQL grammar of `DerekStride/tree-sitter-sql`, in
`grammars/sql`. On 2026-11-09 that is about 32 dialects of dburl, such as
`snowflake`, `bigquery`, `databricks`, `hive`, `impala`, `spanner`,
`vertica`, `exasol`, `firebirdsql`, `hdb`, `databend`, `druid`, `pinot` and
`questdb`. That grammar follows no reference, its `parser.go` is 6.4 MB, and
transit cannot fix its faults without a fork.

Ken decided on 2026-11-09, answering question 112:

1. xo writes a generic SQL grammar, as it wrote the grammars of D126 and
   D127. It replaces the grammar of `DerekStride/tree-sitter-sql` in
   `grammars/sql`. The module, the package path
   `github.com/xo/transit/grammars/sql`, the package name `sql` and the
   functions of the package stay the same, so usql needs no change.
2. The grammar reads the SQL of the standard that most of those dialects
   share, and the forms that several of them add. Examples are `QUALIFY`,
   `PIVOT` and `UNPIVOT`, `LATERAL`, `ARRAY` and `STRUCT` types and values,
   names in backticks, `CREATE ... CLONE`, `MERGE` and `COPY INTO`. The
   references are the SQL documentation of Snowflake, BigQuery and
   Databricks, the three largest of those dialects. The grammar does not
   try to read each dialect in full.
3. The grammar is lenient, because it serves highlighting. A form that only
   one dialect has can parse as a generic form.
4. The grammar keeps its parse tables small with the technique of D122
   point 4, and it reports what its parts cost. Its `parser.go` must be
   smaller than the 6.4 MB of the grammar that it replaces.
5. The work can use the code examples of the documentation of those
   dialects to measure coverage and to write corpus cases, as fair use, as
   D131 says. A corpus case does not copy an example. xo writes each case in
   its own words. xo writes `queries/highlights.scm` too.
6. `DerekStride/tree-sitter-sql` stays in the golden set, but it gets no Go
   package, as `m-novikov/tree-sitter-sql` does (D106 point 1).

Ken decided on 2026-11-09, after part 1:

7. A string takes backslash escapes, as in Snowflake, BigQuery and
   Databricks. The grammar is one language for every fallback dialect, so
   it cannot follow `BackslashEscapes` of dbmeta. A dialect with no escapes
   can show wrong colors after a string such as `'C:\'`.
8. The grammar has no statement that takes any list of words. The error
   recovery of tree-sitter keeps an error inside its statement, and a typo
   in a known statement stays visible.

Ken decided on 2026-11-09, after part 2:

9. The grammar keeps the scripting of the three dialects, which costs about
   190 parse states, and the pipe syntax, the queries that start with
   `FROM` and the `WITH` expression of BigQuery. Together they raise the
   large states from 18 to 88.
10. The forms that part 2 left out stay in docs/BACKLOG.md for now: `->>`,
    a join word before a set operation, `PIVOT` and `UNPIVOT` as pipe
    operators, and nested comments.
