# D112. The usql grammar can keep a block in one statement

Status: Decided, amends D108.

A stored program of MySQL, SQL Server or Oracle has a body between `BEGIN`
and `END` that holds `;`. The usql grammar ends a statement at each `;`, so it
split a `CREATE PROCEDURE`, `FUNCTION`, `TRIGGER` or `EVENT` into several
statements, and each SQL layer had an error. The splitter of usql does the
same today.

Ken decided on 2026-10-08, answering question 81:

1. `usql.Options` gets a new option. With it on, the scanner counts `BEGIN`
   and `END` after `CREATE ... PROCEDURE`, `FUNCTION`, `TRIGGER` or `EVENT`,
   and a `;` inside the body does not end the statement.
2. A `BEGIN` that starts a transaction does not start a block.
3. usql turns the option on for the dialects that need it. It is off by
   default.
