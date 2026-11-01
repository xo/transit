# D130. BeginEndBlocks opens a block at EXECUTE BLOCK and DO BEGIN

Status: Decided, amends D112.

The option `BeginEndBlocks` of D112 keeps a stored program in one statement
when `CREATE ... PROCEDURE`, `FUNCTION`, `TRIGGER` or `EVENT` comes before
its `BEGIN`. The usql peer found on 2026-11-01 that two more forms split at
the semicolons of their body, although dbmeta turns the option on for their
dialects:

- `EXECUTE BLOCK [RETURNS (...)] AS [DECLARE ...;] BEGIN ... END` of
  Firebird.
- `DO BEGIN ... END` of SAP HANA.

dbmeta measured both on live servers. `EXECUTE BLOCK` exists only in
Firebird. `DO BEGIN` occurs in no other dialect, because the `DO` of
PostgreSQL takes a string and the `DO` of MySQL takes expressions.

Ken decided on 2026-11-01, answering question 104:

1. With `BeginEndBlocks` on, `EXECUTE BLOCK` and `DO BEGIN` at the start of
   a statement open a block, as `CREATE ... PROCEDURE` does.
2. The declarations between `AS` and `BEGIN` of `EXECUTE BLOCK` do not end
   the statement, as they do not after `CREATE PROCEDURE`.
3. No new option is needed.
