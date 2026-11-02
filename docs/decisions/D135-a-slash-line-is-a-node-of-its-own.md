# D135. A line with only a slash is a node of its own

Status: Decided, amends D128.

In an Oracle script, a line with only `/` follows each PL/SQL unit, because
SQL*Plus needs it to run the unit. With the option `AnonymousBlocks` of
D128, the usql grammar ends a block at the `;` after its last `END`. A `/`
line after that `;` then started the next statement. usql sent
`/\nselect ...` to Oracle, and Oracle answered ORA-00900. The usql peer
found this on 2026-11-02 on a live Oracle 26ai. The same holds after
`CREATE PROCEDURE ... END;` and after a statement with no `;`, such as
`select 1 from dual` followed by a `/` line.

Ken decided on 2026-11-02, answering question 106:

1. With `AnonymousBlocks` on, a line with only `/` and white space is never
   the text of a statement.
2. After a statement that ended with `;`, such a line is a node of its own
   between the statements. usql drops it.
3. After a statement with no `;`, such a line ends that statement. It is
   the terminator of the statement, and it is outside the text that usql
   sends to the server.
4. With `AnonymousBlocks` off, nothing changes.
