# What usql gets from transit

This document is for a coding agent that works in
[usql](https://github.com/xo/usql). It says what transit gives usql for tab
completion and for highlighting, what is decided, and what is still open.

transit holds the runtime, with the query engine, the package `inject` and
the Go packages of the grammars. [`API.md`](API.md) holds the API, which came
from the working C example of phase 1 (D10). This document names the parts
that usql uses. The sample program `_example/complete` completes at a cursor
as usql will, and `_example/highlight` highlights the input of usql as rline
will for usql (D53).

## The direction

usql takes its design from transit, and transit does not take a design from
usql (D6). usql will replace its completion and its highlighting to follow
transit. transit does not import usql or dbmeta. If usql needs something that
transit does not give, ask Ken. Do not add a workaround in usql, and do not
change transit from usql.

## What is decided

1. transit gives parsing information only. usql decides what the information
   means, and it completes (D6).
2. The input of usql is parsed by a usql grammar, which parses the backslash
   commands and the variables, and hands each SQL statement to a SQL grammar
   through an injection (D13). An injection is a range of the input that
   another grammar parses.
3. The tokens and their kinds come from transit, and never from a chroma
   lexer (D14). usql uses chroma styles today, in its own `styles` package.
   The package `styles` of transit replaces them (D65).
4. usql gives rline a `*transit.Language` for highlighting, because rline
   imports no grammar (D11).
5. The Go API follows the Rust binding, in Go idioms (D25). An offset, a row
   and a column are an `int`, and each counts bytes.
6. transit writes the usql grammar, MySQL, and the SQL-like grammars that do
   not exist, in `grammars/` (D42, D104). The usql grammar is the module
   `github.com/xo/transit/grammars/usql`, and the MySQL grammar is the
   module `github.com/xo/transit/grammars/mysql`.
7. Each grammar repository is a Go module of its own, so usql downloads only
   the grammars that it imports (D26). Each grammar package embeds its
   queries (D31).
8. The package `github.com/xo/transit/styles` holds styles that are keyed on
   capture names, so that usql and rline draw the same code in the same
   colors (D65). It is a module of its own (D99).
9. The usql grammar is one language, the package `usql` of
   `github.com/xo/transit/grammars/usql` (D108). usql gives it the options of
   the syntax of the dialect with `usql.LanguageFor`. The fields of
   `usql.Options` are the flags of the type `Syntax` of dbmeta, with the same
   names: `DollarQuotes`, `BlockComments`, `SlashComments`, `HashComments`,
   `Backticks`, `DashComments`, `Brackets` and `BackslashEscapes`. dbmeta
   holds the value of each flag for each dialect. `usql.Language` has dollar
   quotes, block comments, dash comments and backslash escapes. With
   `DashComments`, `--` starts a comment to the end of the line.
   Without it, `--` is the text of the statement (D139). Each SQL dialect
   turns it on. A dialect of Cypher, such as Neo4j, does not, because
   `(a)--(b)` is a relationship there. With `Brackets`, a name in square
   brackets, such as `[l'état]`, is a `quoted_identifier`, and each
   character in it is the text of the name (D140). SQL Server and SQLite
   turn it on. Without it, `[` is text, as in `a[1]` and `ARRAY[1,2]` of
   PostgreSQL. With `BackslashEscapes`, a backslash escapes the next
   character in a string in single quotes, as `\'` and `\\`, and in a name
   in double quotes, as in `"a\"b"` (D142). Without it, a backslash in such
   a string or name is text, as in `'C:\data\'`, and only a doubled quote
   escapes a quote. A string of the form `E'...'` of PostgreSQL keeps the
   backslash escapes. The `E` must start a word, so `name'x'` is not such a
   string. A backslash in backticks or in brackets is text. In an argument
   of a meta command, a backslash escapes in a string and it is text in a
   name in double quotes, with it on or off, as in psql.
10. `usql.Options` has three more options. `BeginEndBlocks` keeps the
    `BEGIN ... END` body of a stored program, of `EXECUTE BLOCK`, of
    `DO BEGIN` and of `WITH FUNCTION` in one statement (D112, D130, D134).
    `AnonymousBlocks` keeps an anonymous block of PL/SQL, which starts with
    `BEGIN` or `DECLARE`, in one statement (D128). `Batches` keeps a batch
    of CQL, from `BEGIN BATCH` to `APPLY BATCH`, in one statement. They are
    off by default. "Dialects" below says more.

## What usql will replace

1. `drivers/completer`, a completer of 1,097 lines on 2026-09-29 that matches
   keywords and prefixes by hand.
2. The highlighting in `handler/handler.go`: `outputHighlighter`, and the
   chroma lexer that `drivers.Lexer` returns.
3. The chroma styles of its `styles` package, with the package `styles` of
   transit (D65).

## How completion will work

These are the steps that usql takes. [`API.md`](API.md) gives the API of
each step, and `_example/complete` runs them.

1. usql parses the input with the usql grammar. It keeps the tree, and after
   each key it tells the tree what changed and parses again with the old tree.
2. At the cursor, usql asks transit for the node at the byte offset, its
   field in its parent, and its parent. A statement that is not finished, such
   as `SELECT * FROM `, gives an `ERROR` node or a `MISSING` node. transit
   reports both.
3. usql asks transit for the parse state at the cursor, and for the symbols
   that the parser can accept in that state, with the lookahead iterator. The
   keywords among them are the keywords that can come next.
4. usql decides what kind of name goes at the cursor, such as a table or a
   column. transit does not know this (D6). usql finds it with its own queries
   over the tree and with the node types of the grammar.
5. usql gets the names from dbmeta and from the database.

## Completion at the cursor

1. After an error at the cursor, the lookahead iterator can list too many
   symbols or none, because the parser recovered first. transit adds
   `StatesAt`, which gives the parse states of each stack version at a byte
   offset, before the recovery (D28, D57). To complete a word, usql gives the
   offset of the start of the word (D70).
2. The usql grammar hands SQL to a SQL grammar through an injection. The
   package `inject` gives the layers, and usql takes the layer at the cursor
   (D27, D72). A variable such as `:id` reaches the SQL grammar as a
   placeholder of the same length, which `inject.WithReplacer` gives (D101).
   `Layer.Text` holds the text of the layer with those placeholders, and
   `Layer.StatesAt` gives the parse states of the layer at the cursor
   (D111). It sets the ranges of the layer on the parser, so the text of the
   meta commands and of the other statements does not change the states.
   usql does not make the replacements again.
3. A keyword can be written in any case in most SQL dialects. The generated
   package of a grammar lists its keywords, so that usql can offer them.
4. The usql grammar also splits the input into statements (D13), so usql
   does not need a splitter of its own.
5. The sample program `_example/complete` of transit completes at a cursor
   as usql will (D53). It parses the text with `usql.LanguageFor` and the
   options of a dialect, injects each statement into the SQL grammar of that
   dialect with `inject.WithReplacer`, and finds whether the cursor is in a
   meta command, in a variable or in a SQL statement. In a statement, it
   lists the keywords and the kinds of node that can come next, from
   `Layer.StatesAt`. It prints the nodes around the cursor, from
   `DescendantForByteRange`, `Parent` and `FieldNameForChild`. Run it in the
   folder `_example`:

   ```bash
   go run ./complete -dialect postgres -text 'select * from :tbl where '
   ```
6. The C example found that `Node.NextParseState` gives state 0 for a token
   that the parser lexed before a reduce, such as `FROM` in
   `SELECT id FROM u`. The state after such a token comes from the state of
   its parent or of the sibling before it. `API.md` gives both methods, and
   `StatesAt` adds the states before a recovery (D57). After
   `SELECT * FROM `, it gives the state after `FROM`, which accepts 14
   symbols, where state 0 accepts 408 (D70).
7. The generic SQL grammar parses `SELECT id, FROM users` with no error. It
   reads `FROM` as a column. usql cannot find every mistake from `ERROR`
   nodes.

## Dialects

dbmeta maintains the SQL dialects. transit knows nothing about dbmeta. usql
chooses the SQL grammar for each dbmeta dialect, and it keeps that match
itself.

usql supports SQL and SQL-like languages, such as CQL, SQL++, Cypher and
SurrealQL. transit will support the language of every dialect that dbmeta
tests, verifies or stages, and of every hosted service in dbmeta (D21). "The
dialects of dbmeta" in [`CANDIDATES.md`](CANDIDATES.md) lists each dialect,
its language and the grammars that exist for it on 2026-09-29.

The grammars are weak. `DerekStride/tree-sitter-sql` is the one generic SQL
grammar that exists, and most dialects have only a generic grammar. It
follows no reference, so xo writes the generic SQL grammar in
`grammars/sql` in its place (D143). It reads the SQL that most dialects
share and the forms that several of them add, from the SQL documentation
of Snowflake, BigQuery and Databricks, and it is lenient. usql gives it to
each dialect that has no grammar of its own, such as `snowflake`,
`bigquery`, `databricks`, `hive`, `vertica` and `exasol`. The package and
its functions stay the same, so usql needs no change. `gmr/tree-sitter-postgres` follows
the PostgreSQL source, and its package adds about 15 MB to a binary, so xo
writes the PostgreSQL grammar and the PL/pgSQL grammar from the manual in
`grammars/postgres` (D122). MySQL had no grammar of its own, so transit writes one in
`grammars/mysql` (D106). Its package adds about 1 MB to a binary (D123).
`andreasmaierde/tree-sitter-plsql` has not changed since 2023 and has no
corpus, so xo writes the Oracle grammar from the references of Oracle
Database 23ai in `grammars/oracle` (D124). One grammar reads SQL and
PL/SQL, because PL/SQL is not in a quoted string in Oracle. A `/` alone on
its line ends a statement or a PL/SQL unit, as in a script of SQL*Plus.
The Oracle grammar reads it as the end of a statement. The Oracle grammar
reads a whole PL/SQL unit, such as a package body, as one statement. The
usql grammar reads a line with only `/` as the text of a statement, as
psql does (D138). `Crary-Systems/tree-sitter-tsql` covers
little of Transact-SQL, so xo writes the SQL Server grammar from the
Transact-SQL reference of SQL Server 2025 in `grammars/sqlserver` (D124).
A line with only `GO` and an optional count ends a batch in sqlcmd. It is
not Transact-SQL. usql ends a statement at `;` or `\g`, and it gives `GO`
no meaning: it does not end a statement there and it does not remove the
line. So a `GO` line reaches the SQL layer as a part of the text of a
statement, as in `select 1`, a line with `GO`, and `select 2;`. The SQL
Server grammar reads such a line as the end of a batch. Then the
statements before and after it parse with no error, also when no `;` comes
before the `GO`. The body of a procedure or a trigger runs to the end of
its batch, so the `GO` line also ends the body. If usql gives `GO` a meaning one day, that is a decision
of Ken for the usql grammar, and not for the SQL Server grammar.
usql gives `sqlite3` the generic grammar today, which does not read the
syntax of SQLite, such as `PRAGMA`, `ATTACH` and `WITHOUT ROWID`. So xo
writes the SQLite grammar from the SQL reference of SQLite in
`grammars/sqlite3` (D126). It reads each statement of the SQL reference.
usql will give it to `sqlite3`, and to `libsql` and `rqlite` too, because
they speak the SQL of SQLite. The commands of the sqlite3 shell, such as
`.tables`, are not SQL, and the grammar does not read them. usql has no such
commands and does not remove them, so a line such as `.mode csv` reaches
the grammar as a part of the text of a statement. The grammar puts the line
in an ERROR node, and the statement after the line parses with no error.
usql gives `duckdb` the generic grammar today, which does not read the
syntax of DuckDB, such as a query that starts with `FROM`, `QUALIFY` and
the lambdas. So xo writes the DuckDB grammar from the SQL documentation of
DuckDB in `grammars/duckdb` (D126). It reads each statement of the
documentation. usql will give it to `duckdb`, and to
`gizmosql` too, because GizmoSQL serves DuckDB. The commands of the duckdb
shell, such as `.tables`, are not SQL. The grammar puts such a line in an
ERROR node, as the SQLite grammar does.
usql gives `clickhouse` the generic grammar today, which does not read the
syntax of ClickHouse, such as `PREWHERE`, `ARRAY JOIN`, `FINAL`, `SAMPLE`,
the lambdas and the query parameters `{name:Type}`. So xo writes the
ClickHouse grammar from the SQL reference of ClickHouse in
`grammars/clickhouse` (D127). It reads each statement of the
reference. usql will give it to `clickhouse`.
ClickHouse quotes a name with backquotes. A comment starts with `--`,
`//`, `#` and a space, or `#!`, or it is `/* */`, and the grammar reads
each of them.
usql gives `trino` and `presto` the generic grammar today, which does not
read the syntax of Trino, such as `MATCH_RECOGNIZE`, the table functions,
`JSON_TABLE` and the lambdas. So xo writes the Trino grammar from the SQL
documentation of Trino in `grammars/trino` (D127). It reads each
statement of the documentation. usql will give it to `trino`,
and to `presto` and `athena` too, because the SQL of Presto is the SQL
that Trino grew from, and Athena runs Trino for its queries. The grammar
does not read HiveQL, which Athena uses for its DDL, or the syntax that
only Presto has. A SQL routine of Trino has a body of `BEGIN ... END`, so
usql turns on `BeginEndBlocks` for `trino`. Presto and Athena refuse
`BEGIN ... END`, so usql does not turn it on for them. With the option,
the usql grammar opens a block at `CREATE` and at `WITH FUNCTION`, so a
`CREATE FUNCTION` with a body of `BEGIN ... END`, and a query that
defines such a routine with `WITH FUNCTION`, stay one statement (D134). A
statement of HiveQL, such
as `CREATE EXTERNAL TABLE` of Athena, gets an ERROR node.
SQL++, InfluxQL, KSQL, PartiQL, ES|QL and
PPL have no grammar at all. transit writes the missing grammars, and the
usql grammar, in `grammars/` (D42, D104). The other missing grammars come
after phase 5, in the order of the dialects of dbmeta (D106). `bigquery` and
`spanner` get no grammar (D110).

These are the Go packages of the SQL dialects. The sample programs use them
for the flag `-dialect`, with these options of `usql.Options`:

| Dialect | Package | Options |
| --- | --- | --- |
| `postgres` | `github.com/xo/transit/grammars/postgres/postgres` | `DollarQuotes`, `BlockComments`, `BeginEndBlocks`, `DashComments` |
| `mysql` | `github.com/xo/transit/grammars/mysql` | `BlockComments`, `HashComments`, `Backticks`, `BeginEndBlocks`, `DashComments`, `BackslashEscapes` |
| `sqlite3` | `github.com/xo/transit/grammars/sqlite3` | `BlockComments`, `Backticks`, `BeginEndBlocks`, `DashComments`, `Brackets` |
| `duckdb` | `github.com/xo/transit/grammars/duckdb` | `DollarQuotes`, `BlockComments`, `DashComments` |
| `clickhouse` | `github.com/xo/transit/grammars/clickhouse` | `DollarQuotes`, `BlockComments`, `SlashComments`, `HashComments`, `Backticks`, `DashComments`, `BackslashEscapes` |
| `trino` | `github.com/xo/transit/grammars/trino` | `BlockComments`, `BeginEndBlocks`, `DashComments` |
| `presto` | `github.com/xo/transit/grammars/trino` | `BlockComments`, `DashComments` |
| `athena` | `github.com/xo/transit/grammars/trino` | `BlockComments`, `DashComments` |
| `sqlserver` | `github.com/xo/transit/grammars/sqlserver` | `BlockComments`, `BeginEndBlocks`, `DashComments`, `Brackets` |
| `oracle` | `github.com/xo/transit/grammars/oracle` | `BlockComments`, `BeginEndBlocks`, `AnonymousBlocks`, `DashComments` |
| `cql` | `github.com/xo/transit/grammars/cql` | `DollarQuotes`, `BlockComments`, `SlashComments`, `Batches`, `DashComments` |
| `generic` | `github.com/xo/transit/grammars/sql` | `BlockComments`, `DashComments` |

usql keeps this match itself. The repository of `cql` has no highlight
query, so xo wrote the highlight query of its package (D113). xo writes the
highlight queries of `oracle`, `sqlite3`, `duckdb`, `clickhouse`, `trino`,
`sqlserver` and `sql` with their grammars. The generic grammar reads names
in backticks, the comments with `#` and `//`, backslash escapes and strings
in `$$`, but the dialect `generic` turns on only `BlockComments` and
`DashComments`. usql chooses these options for each dialect.

The usql grammar ends a statement at each semicolon outside parentheses.
The body of a stored program of MySQL, SQL Server or Oracle that is a
`BEGIN ... END` block holds semicolons. Without the option `BeginEndBlocks`,
the injection query of the usql grammar gives each part of the body a layer
of its own, and the SQL grammar finds an error in each part.

With `BeginEndBlocks` on, the procedure is one statement (D112), and the
MySQL grammar parses it with no error. `TestInjectMysqlProcedureBody` in
`test/cgrammar` shows both. usql turns the option on for the dialects that
need it, and it is off by default. The scanner reads the words of the SQL
text, outside strings, quoted identifiers and comments, in any case. These
rules hold:

1. The statement must start with `CREATE`, `EXECUTE BLOCK`, `DO BEGIN` or
   `WITH FUNCTION`. White space and comments can come before it. After
   `CREATE`, the words `OR`, `REPLACE`, `ALTER`, `DEFINER = <user>`,
   `EDITIONABLE`, `NONEDITIONABLE`, `AGGREGATE`, `CONSTRAINT`, `TEMP` or
   `TEMPORARY` can come before `PROCEDURE`, `PROC`, `FUNCTION`, `TRIGGER` or
   `EVENT`.
2. The first `BEGIN` after that starts the body. A `;` before it ends the
   statement, so a body that is one statement with no `BEGIN` ends at its
   `;`, as it did before. After `EXECUTE BLOCK`, a `;` before the `BEGIN`
   does not end the statement, so the `RETURNS (...)` list and the
   `DECLARE` lines of Firebird stay in it (D130). The `BEGIN` of
   `DO BEGIN` of SAP HANA is the start of the body.
3. After `WITH FUNCTION` of Trino, each `BEGIN` outside a body starts the
   body of a routine, so each routine of the list keeps its body (D134).
   A routine with `RETURN` and no `BEGIN` holds no `;`. A `SELECT` or
   `VALUES` outside parentheses and outside a body starts the query, and
   after it a `BEGIN` does not start a body, because it can name a
   column. The statement ends at the first `;` outside a body. A `WITH`
   that `FUNCTION` does not follow changes nothing.
4. In the body, `BEGIN` and `CASE` open a block, and `END` closes one.
   `END IF`, `END LOOP`, `END WHILE` and `END REPEAT` do not close a block.
   `END CASE`, `END TRY`, `END CATCH` and `END <label>` close one. The
   statement ends at the first `;` after the `END` of the body.
5. A `BEGIN` starts a transaction, and not a block, when `;`, `WORK`,
   `TRANSACTION`, `TRAN` or `DISTRIBUTED` comes next.
6. After `PROCEDURE` or `FUNCTION`, an `IS` or `AS` outside parentheses that
   a declaration follows starts the declarations of PL/SQL, and a `;` does
   not end the statement until the body ends. A `DECLARE` before the body
   does the same, for an Oracle trigger. `IS` or `AS` does not start
   declarations when a string, `BEGIN`, `LANGUAGE`, `EXTERNAL`, or a word
   that starts a statement such as `SELECT`, `SET`, `RETURN` or `IF` comes
   next.
7. A word right after `.`, `@`, `$`, `#` or `[` is a name, such as
   `NEW.end`, `@end` or `[end]`, and not a keyword.
8. A meta command ends the statement, as it ends any statement.

The option does not cover these texts:

1. An anonymous block of PL/SQL or of T-SQL, such as `BEGIN ... END;` or
   `DECLARE ... BEGIN ... END;` with no `CREATE`. The option
   `AnonymousBlocks` below covers an anonymous block of PL/SQL.
2. A body of a stored program with no `BEGIN`, such as a MySQL
   `IF ... END IF;`, or a T-SQL procedure whose statements run to `GO`.
3. A subprogram in the declarations of PL/SQL, such as
   `PROCEDURE q IS BEGIN ... END;` inside a procedure. Its `END` ends the
   statement.
4. `CREATE PACKAGE BODY` and `CREATE TYPE BODY` of Oracle.
5. A T-SQL `END` with no `;` that `IF` or `WHILE` follows, as in
   `END IF @a = 1`. The scanner reads it as the end of an `IF` or a
   `WHILE`, so the block stays open. A T-SQL `;WITH` right after `BEGIN`
   makes that `BEGIN` a transaction.

The Oracle grammar reads each of these texts of Oracle when it gets the
whole text. When the usql grammar gives it one part of such a text, the
Oracle grammar finds an error in that part.

An anonymous block of PL/SQL has no `CREATE`. It starts with `BEGIN` or
`DECLARE`. In SQLite, MySQL and PostgreSQL, a `BEGIN` with no `CREATE`
starts a transaction, and in SQL Server `DECLARE @x int;` is a statement of
its own. So the option `AnonymousBlocks` keeps an anonymous block in one
statement (D128). It is off by default, and usql turns it on for Oracle
only. These rules hold:

1. The statement must start with `BEGIN` or `DECLARE`, in any case. White
   space and comments can come before it.
2. The scanner counts `BEGIN` and `END` with rules 4 and 7 of
   `BeginEndBlocks` above. In the block, a `BEGIN` opens a block, but not
   when `;`, `WORK` or `TRANSACTION` comes next (D131). A `BEGIN` that
   `TRAN` or `DISTRIBUTED` follows opens a block. After `DECLARE`, a `;`
   does not end the statement until the body ends.
3. The statement ends at the first `;` after the `END` of the outer
   `BEGIN`.
4. A meta command ends the statement, as it ends any statement.
5. The option does not change a statement that starts with another word.

usql works as psql does, and psql has no `/` terminator. So the usql
grammar has no node for a line with only `/`, with the option on or off
(D138). Such a line is the text of a statement. A statement ends only at a
`;` outside parentheses, at a meta command or at the end of the input. So
the script `begin null; end;`, then a `/` line, then `select 1 from dual;`
holds two statements. The text of the second statement is `/`, a new line
and `select 1 from dual`, and Oracle refuses it.

The option keeps only an anonymous block in one statement. The grammar
does not read the other PL/SQL units, such as a function or a procedure
with declarations before `BEGIN`, a trigger with `DECLARE`, a package spec
or a package body. The person who writes the script escapes each `;` inside
such a unit as `\;`, as in psql. The scanner keeps `\;` in the statement. A
query of Oracle that defines a function with `WITH FUNCTION` needs the
escape too, because the `;` after the `END` of the function ends the
statement.

A batch of CQL holds semicolons too. usql collects a batch from
`BEGIN BATCH` to `APPLY BATCH` today. With the option `Batches` on, the
usql grammar keeps the batch in one statement. It is off by default, and
usql turns it on for CQL. These rules hold:

1. The statement must start with `BEGIN BATCH`, `BEGIN UNLOGGED BATCH` or
   `BEGIN COUNTER BATCH`, in any case.
2. In the batch, a `;` does not end the statement. `APPLY BATCH` ends the
   batch, and the first `;` after it ends the statement.
3. A `BEGIN` that `BATCH`, `UNLOGGED` or `COUNTER` does not follow starts a
   statement that is not a batch.
4. The scanner reads the words outside strings, quoted identifiers and
   comments, as it does for `BeginEndBlocks`. A meta command ends the
   statement, as it ends any statement.

## How usql reads a statement

The package `feed` of usql reads the statements from the tree, and
`drivers.Process` of usql changes the text before it runs. usql relies on
these facts of the tree of the usql grammar. The usql peer gave them on
2026-11-02, from `feed/feed.go` and `drivers/drivers.go` of usql:

1. `feed` walks the children of `source_file`. It reads a `statement` and a
   `meta_command`, and it skips any other node with no error.
2. `feed` sends the bytes of the whole `statement` node, with its `;`. It
   marks the statement as terminated when the statement has an anonymous
   child of kind `;`.
3. `drivers.Process` removes the `;` at the end of the text later, with the
   regular expression `;?\s*$`. It does this only for a dialect whose rule
   `Terminator` of dbmeta says so. Nothing in usql removes a `/`. So the
   anonymous `;` must stay in the text of the statement.
4. A line with only `/` is in the text of a statement (D138). So usql
   sends it to the server with the text of that statement.
5. U+FEFF, the byte order mark, is white space outside a string, a quoted
   name, a dollar quote and a comment, with any options (D141). So no
   `statement` starts or ends with it, and usql does not send it to the
   server at the start of a file that a script joins. In the middle of a
   statement, it stays in the text, as a space does. Inside a string, a
   quoted name, a dollar quote and a comment, it is text. No
   `meta_command` starts with it. A `meta_command` holds the white space
   after its last argument, so its text can end with U+FEFF, as it can end
   with a space.

## What usql must not do

1. Do not use a chroma lexer (D14).
2. Do not decide what kind of name goes at the cursor from the text alone.
   Use the tree.
3. Do not write a parser for SQL or for the backslash commands in usql. The
   usql grammar parses them (D13).
