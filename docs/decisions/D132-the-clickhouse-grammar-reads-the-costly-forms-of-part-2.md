# D132. The ClickHouse grammar reads the costly forms of part 2

Status: Decided, amends D131.

The second part of the ClickHouse grammar of D127 left out four forms
because of their cost in parse states. With no reserved words (D131 point
4), the grammar also reads `SELECT 1 SELECT 2` with no semicolon as one
query with the alias `SELECT`.

Ken decided on 2026-11-02:

1. The ClickHouse grammar reads these forms, although each one costs parse
   states:
   - `ASC` and `DESC` in `CREATE INDEX`, at about 330 states.
   - `COMMENT` after the query of `CREATE ... AS SELECT`, at about 690.
   - `SETTINGS` before the query of `INSERT`, at about 1,050.
   - A `REFRESH` interval with more than one unit, at about 1,000, and
     `WITH REFRESH` and `WITH TIMEOUT` of a live view, at about 610.
2. An alias with no `AS` cannot be a word that ClickHouse refuses as such an
   alias, such as `SELECT`, `FROM` or `WHERE`. The grammar uses a reserved
   set for these aliases only. Every word can still name a column, and an
   alias with `AS` can still be any word.
