# D127. xo writes the ClickHouse and Trino grammars

Status: Decided, amends D106.

usql gives the dialects `clickhouse`, `trino` and `presto` of dbmeta the generic SQL
grammar, because transit has no grammar of their own, and the search of
CANDIDATES.md found none.

Ken decided on 2026-11-01:

1. xo writes a ClickHouse grammar from the SQL reference of ClickHouse, and
   a Trino grammar from the SQL documentation of Trino, each of the current
   release, as it wrote MySQL (D42, D106).
2. Their modules have the names of the dialects, `grammars/clickhouse` and
   `grammars/trino` (D107), and each package has the name of its folder.
3. Each grammar keeps its parse tables small with the technique of D122
   point 4, and each reports what its parts cost.
4. The work comes after the SQLite and DuckDB grammars of D126.
5. The Trino grammar also serves the dialect `presto`, because the SQL of
   Presto is the SQL that Trino grew from. usql chooses the grammar of a
   dialect (docs/USQL.md), so it maps `presto` to the Trino grammar when the
   grammar is done.
