# D106. The grammars of phase 5

Status: Decided, amends D42, amended by D107.

Ken decided on 2026-10-01, answering questions 73 to 75:

1. `m-novikov/tree-sitter-sql` stays in the golden set, but it gets no Go
   package. Its module folder and its grammar name, `sql`, are the same as
   those of `DerekStride/tree-sitter-sql`, which is the generic SQL grammar
   of tier 3 and gets `grammars/sql`. It has not been pushed since 2024, and
   `gmr/tree-sitter-postgres` covers PostgreSQL better.
2. A package name is the name of the grammar in lowercase, with each `_`
   removed. The grammar `TSQL` gets the package `tsql`, in `grammars/tsql`.
3. Phase 5 writes MySQL, the grammar that xo needs most. The other SQL-like
   languages that have no grammar, such as SQL++, InfluxQL, KSQL, PartiQL,
   ES|QL and PPL, come after phase 5, in the order of the dialects of
   dbmeta.
