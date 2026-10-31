# D126. xo writes the SQLite and DuckDB grammars

Status: Decided, amends D106.

usql gives the dialects `sqlite3` and `duckdb` of dbmeta the generic SQL
grammar of `DerekStride/tree-sitter-sql`, because transit has no grammar of
their own. The one SQLite grammar that the search of CANDIDATES.md found,
`dhcmrlchtdj/tree-sitter-sqlite`, is archived, and no DuckDB grammar was
found. The generic grammar does not read the syntax of their own, such as
`PRAGMA`, `ATTACH` and `WITHOUT ROWID` of SQLite, or the queries that start
with `FROM`, `QUALIFY` and the lambdas of DuckDB.

Ken decided on 2026-11-01, answering question 102:

1. xo writes a SQLite grammar from the SQL reference of sqlite.org, and a
   DuckDB grammar from the documentation of DuckDB, each of the current
   release, as it wrote MySQL (D42, D106).
2. Their modules have the names of the dialects, `grammars/sqlite3` and
   `grammars/duckdb` (D107), and each package has the name of its folder.
3. Each grammar keeps its parse tables small with the technique of D122
   point 4, and each reports what its parts cost.
4. The work comes after the SQL Server grammar of D124.
