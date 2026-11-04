# D140. A name in square brackets is an option of the usql grammar

Status: Decided, amends D108.

SQL Server and SQLite quote a name between square brackets, as in
`[l'état]`. The usql grammar read the `'` inside the brackets as the start
of a string, and the string never closed. So usql lost the rest of a
script. The usql peer found this on 2026-11-05 (xo/usql discussion #600).
dbmeta records the fact as `Syntax.Brackets` (dbmeta D264).

Ken decided on 2026-11-05, answering question 109:

1. `usql.Options` has a new field, `Brackets`. The sense is positive, as
   for `Backticks`. The SQL Server and SQLite dialects set it.
2. With it on, a `[` outside a string, a comment or another quoted name
   starts a quoted identifier. It ends at the next `]` that a second `]`
   does not follow. `]]` is an escaped `]`.
3. Inside the brackets, each character is the text of the name: `'`, `"`,
   a backtick, `;`, `--`, `/*`, `#`, a backslash, `:` and a new line. A
   backslash does not start a meta command or `\;`, and a colon does not
   start a variable.
4. The node is `quoted_identifier`, as for a name between double quotes.
5. An open `[` at the end of the input keeps the statement open, as an
   open quote does.
6. With it off, nothing changes. So `a[1]` and `ARRAY[1,2]` of PostgreSQL
   are not names.
