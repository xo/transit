# What usql gets from transit

This document is for a coding agent that works in
[usql](https://github.com/xo/usql). It says what transit will give usql for
tab completion and for highlighting, what is decided, and what is still open.

On 2026-09-29 transit holds no code. Nothing here is an API yet. The target
API comes from a working C example in phase 1 of [`PLAN.md`](PLAN.md) (D10).
When it exists, this document will name each identifier that usql uses.

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
3. usql uses chroma only for its styles (D14). usql does this today with the
   `styles` package. The tokens and their kinds come from transit, and never
   from a chroma lexer.
4. usql gives rline a `*transit.Language` for highlighting, because rline
   imports no grammar (D11).
5. The Go API follows the Rust binding, in Go idioms (D25). An offset, a row
   and a column are an `int`, and each counts bytes.
6. transit writes the usql grammar, MySQL, and the SQL-like grammars that do
   not exist, in `grammars/xo/` (D42).
7. Each grammar repository is a Go module of its own, so usql downloads only
   the grammars that it imports (D26). Each grammar package embeds its
   queries (D31).
8. The module `github.com/xo/transit/chromastyles` matches capture names to chroma
   token types, so that usql and rline draw the same code in the same colors
   (D32).

## What usql will replace

1. `drivers/completer`, a completer of 1,097 lines on 2026-09-29 that matches
   keywords and prefixes by hand.
2. The highlighting in `handler/handler.go`: `outputHighlighter`, and the
   chroma lexer that `drivers.Lexer` returns.

## How completion will work

This is the plan, not an API. The names can change in phase 1.

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
   symbols or none, because the parser recovered first. transit adds an API
   that gives the parse states of each stack version at a byte offset, before
   the recovery (D28).
2. The usql grammar hands SQL to a SQL grammar through an injection. The
   package `inject` builds the layers (D27).
3. A keyword can be written in any case in most SQL dialects. The generated
   package of a grammar lists its keywords, so that usql can offer them.
4. The usql grammar also splits the input into statements (D13), so usql
   does not need a splitter of its own.
5. A sample program in `_example/` of transit completes at a cursor as usql
   will, in phase 5 (D53).

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
the PostgreSQL source. MySQL has no grammar of its own. SQL++, InfluxQL,
KSQL, PartiQL, ES|QL and PPL have no grammar at all. transit writes the
missing grammars, and the usql grammar, in `grammars/xo/` (D42).

## What usql must not do

1. Do not use a chroma lexer (D14).
2. Do not decide what kind of name goes at the cursor from the text alone.
   Use the tree.
3. Do not write a parser for SQL or for the backslash commands in usql. The
   usql grammar parses them (D13).
