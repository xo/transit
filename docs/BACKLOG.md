# Backlog

This document lists work that is known and not done. Each item names the
decision or the question that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open
questions for Ken. When an item here is done, delete it, and record in
`decisions/` anything that was decided on the way (dbmeta D110).

## Units in progress

An agent claims a unit of port work here before it starts, with the unit, the
agent and the date, and deletes the claim when Ken commits the unit (D50, D61).

| Unit | Agent | Date |
| --- | --- | --- |

## The generator

### Port the log of the generator

Upstream writes log lines with `debug!` and `info!` in `build_tables/`, and
`report_state_info` in `build_tables.rs` writes the parse states of one rule
for the option `--report-states-for-rule` of the tool. The generator has no
logger yet, so the port leaves them out. The generator logs with `log/slog`
(D67). Add the logger to the options, then port the lines,
`report_state_info` and `--report-states-for-rule`. Some functions of
`build_lex_table.go` then take the `StrPool` again, as upstream does.

Since 4c6466d7 of upstream, the checks of `minimize_parse_table.rs` return
an `Option<Conflict>`, and only `log_conflict` reads it. The Go checks
return a bool, because nothing reads the conflict yet. Port `Conflict` and
`log_conflict` with the logger.

### Measure the small parse table as a string

D121 writes the dense parse table as a string and keeps the small parse
table as a literal. With the small parse table as a string too, a cold build
of postgres takes 0.77 s in place of 1.08 s, and of sql 0.61 s in place of
1.11 s. Its parse cost was measured only without the pad of `Language`.
Measure it with the pad, and change the form if D121 still holds.

## The runtime

### Compare the highlight test with the upstream tool

The highlight test of a grammar module follows the highlighter of upstream
(D80, D84). `tree-sitter test` passes the files of `test/highlight` of 12
grammars, and so does the port, in `test/cgrammar`: c, c_sharp, css, html,
java, javascript, lua, php, python, ruby, toml and yaml. The test also passes
in the modules c, cpp, html, java, javascript, php, python and ruby. The
cache holds 48 more folders of `test/highlight`. Compare the port with the
upstream tool on them, and record each difference.

### Cut the memory of the generator for large grammars

The generator takes 1 minute 58 seconds and 10.9 GB of memory for the
grammar `postgres` of `gmr/tree-sitter-postgres`, as the C backend does
(D73). That grammar has no Go package since D122, so CI no longer generates
it, but a runner with less memory than that cannot generate it. Measure
where the memory goes, and compare it with the upstream tool.

### Compare the trees at ABI 14

`test/cgrammar` compares the Go trees with the C trees at ABI 15 only. Compare
them at ABI 14 too, from the `src/parser.c` of each grammar. The files
`upstream_*_test.go` there port the tests of `crates/cli/src/tests` of
upstream (D35).

### Support the Neovim dialect when the tier 1 grammars need it

D69 defers `#lua-match?` of D55 and the rest of the Neovim dialect of
queries. When the work on the tier 1 grammars needs them, build the parts
that "What support would take" in [`NEOVIM.md`](NEOVIM.md) lists. The first
part is the port of the matcher of LuaJIT 2.1 from `src/lib_string.c`, for
`#lua-match?`.

### Keep the tables of NEOVIM.md current

The tables of [`NEOVIM.md`](NEOVIM.md) come from one scan of the query files
of the set on 2026-09-30. Add a check to the golden harness that scans the
query files of each grammar in the cache, and that fails when a table of
`NEOVIM.md` or a mark [N] of `CANDIDATES.md` differs from the scan. Until the
check exists, step 8 of "Steps to add a grammar to the set" in
[`GRAMMAR.md`](GRAMMAR.md) keeps them current by hand.

### Compare the decoders with the C decoders

[`UPSTREAM.md`](UPSTREAM.md) says that the test of the decoder compares the
Go decoders with the C decoders for every sequence of up to four bytes, in
the test module. The tests of `lexer.go` compare `decodeUTF8` with the
maximal subpart of the Unicode standard, which ICU follows, and they test the
UTF-16 decoders case by case. Add the comparison with `U8_NEXT` and
`U16_NEXT` of C when the test module can call the decoders of the runtime.

## The grammars

### Finish the queries of the PostgreSQL grammar

The grammar `postgres` that xo writes (D122) leaves out these forms of a
query of PostgreSQL 19:

- `JSON_TABLE` and `XMLTABLE` in `FROM`.
- `FORMAT JSON` after an argument of a function, as in
  `json_object('a': x FORMAT JSON)`, and `RETURNING` after the query of
  `json_array`. Each of them splits the parse states of each expression or
  each query in two. Find a cheaper form.
- A typed literal with a type modifier, such as `interval(2) '1 day'` and
  `char(20) 'x'`.
- `ANY`, `SOME` and `ALL` after an arithmetic operator, as in
  `33 * ANY (a)`.
- A cast as the second argument of `position`, as in
  `position(a IN b::bytea)`.

It also leaves out these forms of the statements that define objects:

- The statements that `CREATE SCHEMA` can create in the same statement, as
  in `CREATE SCHEMA s CREATE TABLE t (a int)`. Each statement in it has
  parse states of its own, because `CREATE` can come after it, and that
  costs about 50 large states. Find a cheaper form.
- A type as the value of an option, as in
  `CREATE TYPE r AS RANGE (subtype = int4[])`. A value of an option is a
  string, a number or a name.
- A column named `exclude` or `period` at the place where `EXCLUDE` or
  `PERIOD` can start a constraint or a column of a key. PostgreSQL does not
  reserve these words, but the grammar reads them as keywords there.
- A body in SQL of `CREATE FUNCTION` before an option, as in
  `CREATE FUNCTION f() RETURNS int RETURN 1 LANGUAGE sql`. The grammar
  takes `RETURN` and `BEGIN ATOMIC` only after the options. An expression
  that an option can follow has parse states of its own, and that costs
  about 700 parse states and 22 large states. Find a cheaper form.
- A statement other than a query, `INSERT`, `UPDATE`, `DELETE` and `MERGE`
  in `BEGIN ATOMIC`. `END` is also a statement, so the full list of
  statements makes a conflict.

### Finish the PL/pgSQL grammar

The grammar `plpgsql` that xo writes (D122) reads each expression and each
SQL statement of a body as one token, and its injection query gives the
token to the grammar `postgres`. These faults remain:

- An expression alone, such as the condition of `IF`, is not a statement of
  SQL, so the grammar `postgres` gives an `ERROR` node around it in its
  layer. Its keywords, names and literals keep their colors, but a function
  name gets the color of a variable. The grammar `postgres` can take an
  expression alone as its whole input. That costs 203 parse states and 90
  large states, and the package gets about 0.3 MB larger. Ken decides
  whether usql, which reads no injection, pays for it.
- The body of a function or of a `DO` block in single quotes gets no
  injection, because its text holds the quotes and each quote in it is
  doubled.
- The count of `FETCH` and `MOVE` is a number or a name, where PostgreSQL
  takes an expression. After `FORWARD` and `BACKWARD`, it is only a number,
  because a name there can also be the cursor.
- An operator of the grammar, such as `=` or `<>`, as the value of an
  option of `ALTER OPERATOR`, as in `SET (COMMUTATOR = =)`. The value of an
  option in `SET` can be an operator that is not a token of the grammar,
  such as `===`.

### Finish the MySQL grammar

The MySQL grammar that xo writes keeps its parse tables small (D123), and
so it leaves out these forms:

- An expression with an operator as the value of `INTERVAL`, as in
  `INTERVAL 1 + 1 DAY`. The value is a primary expression or a signed
  number, so `INTERVAL (1 + 1) DAY` parses. An operator in the value
  gives each operator a parse state with the units after it, and that
  costs about 100 large states.
- An expression with an operator as the text of `AGAINST`, as in
  `AGAINST (a || b)`. The text is a primary expression.
- A character set before a hexadecimal or binary number with white space
  between them, as in `_utf8mb4 0x41`. A token for it can match a name and
  then the white space, so the generator merges fewer parse states, and
  that costs about 80 large states. 755 statements of the tests of MySQL
  8.4 hold this form.
- A system variable whose first part is a reserved word, as in
  `SET GLOBAL default.key_buffer_size = 1`.

These faults remain:

- `SOUNDS` and `MEMBER` are the token `identifier`, and an identifier after
  an expression with an operator ends that expression first, so that the
  identifier can be an alias. So `a AND b SOUNDS LIKE c` is
  `(a AND b) SOUNDS LIKE c`, where MySQL reads `a AND (b SOUNDS LIKE c)`.
- The words of an interval unit that MySQL reserves, such as `DAY_HOUR`,
  are one token with the other units, so they are not in the reserved set.

## The setup of the repository

These items come in phase 1 and later, as the plan says.

### Add the tests of the port rules

[`UPSTREAM.md`](UPSTREAM.md) and [`GRAMMAR.md`](GRAMMAR.md) name tests that
hold their rules:

1. Each folder in `grammars/` has an entry in `grammars/grammars.json`, and
   each entry has a folder.
2. Each ported function has a doc comment that names its C function or its
   Rust function.
3. From phase 6 on, the ledger has one line for each commit from the base
   commit to `upstream.txt`, in order, with no gap and no line twice, as
   "The ledger" in `UPSTREAM.md` says.
