# D142. A backslash escape in a string is an option of the usql grammar

Status: Decided, amends D105 and D108.

D105 item 4 made a backslash escape the next character in a `'...'` string
of every family. So on PostgreSQL, SQL Server, Oracle and SQLite, the
statement `select 'C:\data\';` stayed open in usql, but psql runs it. A
script in Shift JIS, Big5, GBK or GB18030 can also hold the byte of a
backslash as the second byte of a character, such as 表 in Shift JIS. Such
a string then lost its closing quote. The usql peer raised this on
2026-11-05.

Ken decided on 2026-11-05, answering question 111:

1. `usql.Options` has a new field, `BackslashEscapes`. The sense is
   positive, as for `Brackets`. The MySQL dialects set it.
2. With it on, `\'` and `\\` escape in a `'...'` string, as before.
3. With it off, a backslash in a `'...'` string is text, and only `''`
   escapes a quote.
4. A string of the form `E'...'` or `e'...'` of PostgreSQL keeps the
   backslash escapes with the option off.
5. The default build of `src/scanner.c`, with no `USQL_OPTIONS`, and the
   default language keep the escapes, so the default corpus does not
   change.
