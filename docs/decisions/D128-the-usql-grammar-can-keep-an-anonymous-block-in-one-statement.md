# D128. The usql grammar can keep an anonymous block in one statement

Status: Decided, amends D112.

With the option `BeginEndBlocks` of D112, the usql grammar keeps a stored
program in one statement when `CREATE ... PROCEDURE`, `FUNCTION`, `TRIGGER`
or `EVENT` comes before its `BEGIN`. An anonymous block of Oracle has no
`CREATE`. It starts with `BEGIN` or `DECLARE`, so the grammar split
`begin null; null; end;` into three statements. The usql peer found this on
2026-11-01. In SQLite, MySQL and PostgreSQL, a `BEGIN` with no `CREATE`
starts a transaction, and in SQL Server `DECLARE @x int;` is a statement of
its own.

Ken decided on 2026-11-01, answering question 103:

1. `usql.Options` gets a new option, `AnonymousBlocks`. It is off by
   default, and usql turns it on for Oracle only.
2. With the option on, a statement that starts with `BEGIN` or `DECLARE` is
   a block. The scanner counts `BEGIN` and `END`, as it does for
   `BeginEndBlocks`, and the statement ends at the `;` after the last `END`.
3. A line with only `/` also ends the block, as it does in SQL*Plus.
4. The option does not change a statement that starts with another word.
