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
   names: `DollarQuotes`, `BlockComments`, `SlashComments`, `HashComments`
   and `Backticks`. `usql.Language` has dollar quotes and block comments,
   the options of PostgreSQL. A comment that starts with `--` is a comment
   with every set of options.
10. `usql.Options` has one more option, `BeginEndBlocks`, which keeps the
    `BEGIN ... END` body of a stored program in one statement (D112). It is
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
grammar, and most dialects have only it. `gmr/tree-sitter-postgres` follows
the PostgreSQL source. MySQL had no grammar of its own, so transit writes
one in `grammars/mysql` (D106). SQL++, InfluxQL, KSQL, PartiQL, ES|QL and
PPL have no grammar at all. transit writes the missing grammars, and the
usql grammar, in `grammars/` (D42, D104). The other missing grammars come
after phase 5, in the order of the dialects of dbmeta (D106). `bigquery` and
`spanner` get no grammar (D110).

These are the Go packages of the SQL dialects. The sample programs use them
for the flag `-dialect`, with these options of `usql.Options`:

| Dialect | Package | Options |
| --- | --- | --- |
| `postgres` | `github.com/xo/transit/grammars/postgres/postgres` | `DollarQuotes`, `BlockComments` |
| `mysql` | `github.com/xo/transit/grammars/mysql` | `BlockComments`, `HashComments`, `Backticks`, `BeginEndBlocks` |
| `sqlserver` | `github.com/xo/transit/grammars/sqlserver` | `BlockComments`, `BeginEndBlocks` |
| `oracle` | `github.com/xo/transit/grammars/oracle` | `BlockComments`, `BeginEndBlocks` |
| `cql` | `github.com/xo/transit/grammars/cql` | `DollarQuotes`, `BlockComments`, `SlashComments` |
| `generic` | `github.com/xo/transit/grammars/sql` | `BlockComments` |

usql keeps this match itself. The repositories of `sqlserver`, `oracle` and
`cql` have no highlight query, so xo wrote the highlight query of each of
these packages (D113).

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

1. The statement must start with `CREATE`. Then `OR`, `REPLACE`, `ALTER`,
   `DEFINER = <user>`, `EDITIONABLE`, `NONEDITIONABLE`, `AGGREGATE`,
   `CONSTRAINT`, `TEMP` or `TEMPORARY` can come before `PROCEDURE`, `PROC`,
   `FUNCTION`, `TRIGGER` or `EVENT`.
2. The first `BEGIN` after that starts the body. A `;` before it ends the
   statement, so a body that is one statement with no `BEGIN` ends at its
   `;`, as it did before.
3. In the body, `BEGIN` and `CASE` open a block, and `END` closes one.
   `END IF`, `END LOOP`, `END WHILE` and `END REPEAT` do not close a block.
   `END CASE`, `END TRY`, `END CATCH` and `END <label>` close one. The
   statement ends at the first `;` after the `END` of the body.
4. A `BEGIN` starts a transaction, and not a block, when `;`, `WORK`,
   `TRANSACTION`, `TRAN` or `DISTRIBUTED` comes next.
5. After `PROCEDURE` or `FUNCTION`, an `IS` or `AS` outside parentheses that
   a declaration follows starts the declarations of PL/SQL, and a `;` does
   not end the statement until the body ends. A `DECLARE` before the body
   does the same, for an Oracle trigger. `IS` or `AS` does not start
   declarations when a string, `BEGIN`, `LANGUAGE`, `EXTERNAL`, or a word
   that starts a statement such as `SELECT`, `SET`, `RETURN` or `IF` comes
   next.
6. A word right after `.`, `@`, `$`, `#` or `[` is a name, such as
   `NEW.end`, `@end` or `[end]`, and not a keyword.
7. A meta command ends the statement, as it ends any statement.

The option does not cover these texts:

1. An anonymous block of PL/SQL or of T-SQL, such as `BEGIN ... END;` or
   `DECLARE ... BEGIN ... END;` with no `CREATE`.
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
6. `BEGIN BATCH ... APPLY BATCH` of CQL.

## What usql must not do

1. Do not use a chroma lexer (D14).
2. Do not decide what kind of name goes at the cursor from the text alone.
   Use the tree.
3. Do not write a parser for SQL or for the backslash commands in usql. The
   usql grammar parses them (D13).
