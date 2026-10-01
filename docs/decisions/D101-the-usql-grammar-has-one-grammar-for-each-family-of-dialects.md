# D101. The usql grammar has one grammar for each family of dialects

Status: Decided, amends D13, amended by D105.

D13 says that a usql grammar parses the meta commands and the variables of
the input of usql, and hands each SQL statement to a SQL grammar through an
injection. On 2026-10-01 the usql agent gave the requirements from the code
of usql, and the transit agent asked Gemini and DeepSeek how to build the
grammar. Both found no tree-sitter grammar for the input of psql to start
from. The client lexer of PostgreSQL, `src/bin/psql/psqlscan.l`, is the
reference for the rules.

The statement scanner depends on the dialect. Dollar quotes, `#` comments,
`//` comments and backticks are strings or comments in some dialects and
not in others, and a tree-sitter grammar has no parameters.

Ken decided on 2026-10-01, answering questions 68 to 71:

1. One `grammar.js` and one scanner build one grammar for each family of
   dialects, as `typescript` and `tsx` share one source. Each one is a normal
   grammar, so the upstream tool, the golden files and the tests of the test
   module apply to it (D42). The families come from the options of usql:
   - postgres: dollar quotes and block comments
   - mysql: block comments, `#` comments and backticks
   - sqlite: block comments and backticks
   - block comments only, for SQL Server, Oracle, ClickHouse, Trino, DuckDB
     and the others
   - cql: dollar quotes, block comments and `//` comments
   - no options, for the dialects that set none
2. A variable reaches the SQL grammar as a placeholder of the same length.
   Before the package `inject` parses a SQL layer, it replaces each variable
   with text that the SQL grammar accepts, such as `_id` for `:id`, so the
   SQL tree has no error from it and every offset stays the same. This is a
   new option of the package `inject`, an API that upstream does not have
   (D28). `docs/API.md` will give its form.
3. Outside a string, `\\` is the separator meta command of psql. It ends
   the arguments of the meta command before it, and the text after it on the
   line is SQL. usql treats `\\` as an escaped backslash today. The grammar
   follows psql.
4. The scanner splits a meta command name into the base name and its
   modifiers, so `\dS+` is the name `\d` with the modifiers `S` and `+`. The
   scanner knows the commands of usql from `metacmd/descs.go` of usql. A name
   that it does not know is one whole name.
