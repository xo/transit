# D134. The choices of the Trino grammar

Status: Decided, amends D127 and D130.

The two parts of the Trino grammar of D127 limited some forms to keep the
parse tables small, and left out forms that the documentation of Trino does
not show. usql also splits a query that defines a SQL routine inline, as in
`WITH FUNCTION f() ... BEGIN ... END SELECT f()`, at the semicolons of the
body of the routine.

Ken decided on 2026-11-02, answering question 105 and three choices of the
Trino grammar:

1. The value of a property of `WITH SESSION`, and the version after
   `FOR VERSION AS OF` and `FOR TIMESTAMP AS OF`, take any expression,
   although that costs parse states.
2. `GRANT`, `REVOKE` and `DENY` take every kind of object that the parser of
   Trino accepts, although the documentation shows only `TABLE` and
   `SCHEMA`.
3. `WITH DATA` and `WITH NO DATA` come only after the query of
   `CREATE TABLE ... AS`, although that costs more parse states than to
   take them after any query.
4. The MATCH predicate stays.
5. With the option `BeginEndBlocks` of D112 on, `WITH FUNCTION` at the
   start of a statement opens a block, as `CREATE FUNCTION` does.
