# D129. The size choices of the SQLite and DuckDB grammars

Status: Decided.

The agents that wrote the SQLite and DuckDB grammars of D126 left some forms
out to keep the parse tables small, and kept some forms that the reference
does not need. The MySQL grammar of D123 and the PostgreSQL and Oracle
grammars of D122 and D124 left three choices open.

Ken decided on 2026-11-01:

1. The DuckDB grammar reads `WITH DATA` and `WITH NO DATA` after
   `CREATE TABLE ... AS`, although they cost about 720 parse states.
2. The options of a DuckDB sequence take an expression, not only a signed
   number, although that costs about 370 parse states.
3. The DuckDB grammar keeps the statements of external resources, which the
   tests of DuckDB use and its documentation does not show.
4. The SQLite grammar keeps `ORDER BY` and `LIMIT` on `UPDATE` and `DELETE`,
   which need a build option of SQLite.
5. The node types that the MySQL grammar changed under D123 stay: the
   `arguments` node of `TRIM`, the `interval_unit` node, one
   `version_comment` node and one node for each kind of join.
6. The grammar `postgres` does not read a bare expression where the grammar
   `plpgsql` injects one.
7. The option clauses of the Oracle grammar that are lists of words stay
   lists of words.
