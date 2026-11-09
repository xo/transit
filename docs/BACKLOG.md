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

### Finish the Oracle grammar

The Oracle grammar that xo writes (D124) reads the queries of Oracle
Database 23ai, the statements that change data, the statements of
transactions, of sessions and of privileges, the statements that define
objects, and PL/SQL. It leaves out these forms of a query:

- `GRAPH_TABLE` of the property graphs of 23ai, the analytic views, and the
  operations of `JSON_TRANSFORM`, such as `SET '$.a' = 1`.
- `FROM FIRST` and `FROM LAST` after `NTH_VALUE`. `FROM` after a function
  call can also start the `FROM` of the query, and the parser cannot tell
  the two apart with one token of lookahead.
- `PARTITION BY` in the arguments, as in `APPROX_RANK(PARTITION BY a ORDER
  BY b)`, and `OF ANOMALY` of `PREDICTION_PROBABILITY`.

It leaves out these forms of the other statements:

- A parameter of `ALTER SYSTEM SET` that holds a reserved word, as in
  `SET ENCRYPTION KEY IDENTIFIED BY w`, and the privilege
  `ON COMMIT REFRESH`, whose `ON` starts the object of `GRANT`.

It leaves out these forms of the statements that define objects:

- The source text of `CREATE JAVA SOURCE ... AS` and of
  `CREATE MLE MODULE ... AS`. It is Java or JavaScript, not SQL.
- The form of GraphQL of `CREATE JSON RELATIONAL DUALITY VIEW`, as in
  `AS dept @insert { _id: deptno }`, and `UNNEST` in the form of SQL.
- `DOMAIN d (a, b)` as an element of `CREATE TABLE`, which gives a domain
  of several columns to the columns. With this form, a column with the
  name `domain` would have an error.
- `CREATE OUTLINE ... ON` a statement. The statement is a list of words.
- `SCOPE FOR` and `REF ... WITH ROWID` of a column of type `REF`.

The physical options of a table, an index, a materialized view and the
other objects are a list of words, as are the clauses of `ALTER` but
`ALTER TABLE`. The words of a time of refresh, as in `NEXT SYSDATE + 1`,
and the `FROM` and the `WHERE` of a bitmap join index are words of such a
list too. An expression there costs large parse states, because many words
can come after it. A column without parentheses after `ADD` or `MODIFY`
of `ALTER TABLE` is the last clause of the statement, for the same reason.

A keyword that Oracle does not reserve and that can come after the table of
a statement, such as `LOG`, `RETURNING` or `WHEN`, is not an alias of the
table there. So `DELETE FROM t log WHERE a = 1` has an error. In the same
way, a view with the name `log` after `DROP MATERIALIZED VIEW` has an
error, because `LOG` continues the kind of the object.

It leaves out these forms of PL/SQL:

- A variable or a parameter whose name is a word that Oracle SQL reserves
  and PL/SQL does not, such as `row`, `level` or `uid`. A reserved set of
  tree-sitter applies to the steps of one rule, and an expression of
  PL/SQL uses the rules of an expression of SQL.
- A constructor with `NEW`, as in `NEW food_t('x')`. `new` can also be a
  column or a name of a trigger, as in `WHEN (new.a > 0)`, and the parser
  cannot tell the two forms apart with one token of lookahead.
- A function whose name is a reserved word, such as `SET(t)`, `USER()` and
  a method `EXISTS` in a package specification.
- A name of an element of an associative array that is an expression with
  an operator, as in `t('a' || 'b' => 1)`. A literal, a sign and a function
  call can be the name.
- An attribute of a cursor variable that is a bind variable, as in
  `:c%ISOPEN`.
- Conditional compilation in a part of a declaration or a statement, as in
  a parameter list. It can hold whole declarations and statements.
- A statement that starts with a word that is also a keyword of PL/SQL,
  such as a procedure with the name `open` or `close`.

The words that only PL/SQL reserves, such as `BEGIN` and `END`, are not in
the reserved set. A reserved set changes only the lexer, so these words
save no parse states (D125).

### Forms that the SQL Server grammar leaves out

The SQL Server grammar that xo writes (D124) reads each statement of the
Transact-SQL reference of SQL Server 2025. It leaves out these forms:

- `MATCH` and `SHORTEST_PATH` of the graph tables, as in
  `WHERE MATCH(p-(f)->q)`, and `WITHIN GROUP (GRAPH PATH)`.
- A statement that changes data with `OUTPUT` as a derived table, as in
  `SELECT * FROM (DELETE FROM t OUTPUT deleted.*) AS d`. It costs about 720
  parse states, because the parser needs the states of each such statement
  again with `)` after it.
- The number of a procedure in a group, as in `EXEC p;2` and `CREATE
  PROCEDURE p;2`, because the `;` ends the statement.
- The old form of `RAISERROR` with no parentheses, as in
  `RAISERROR 50001 'text'`, which SQL Server 2012 removed.
- `MOVE CONVERSATION` right after a statement with no semicolon. `MOVE` is
  not a word that starts a statement for the external scanner, because
  `MOVE` comes after a comma in the options of `RESTORE`, where the list of
  words can end.
- `ORDER` after the columns of a CLR table-valued function, as in `RETURNS
  TABLE (a int) ORDER (a) AS EXTERNAL NAME x.y.z`.
- A join on the right side of a join with no parentheses, as in `a JOIN b
  JOIN c ON c.x = b.x ON b.y = a.y`.
- A query in parentheses as a statement right after a statement with no
  semicolon, as in `SELECT 1 (SELECT 2)`. The external scanner gives the end
  of a statement only before a word, because `RETURN (SELECT ...)` returns
  the value of the query.
- `WITH XMLNAMESPACES` in `CREATE SELECTIVE XML INDEX`. The external scanner
  gives the end of a statement before `WITH XMLNAMESPACES`, because it
  starts a query.
- `ADD SIGNATURE` and `ADD SENSITIVITY CLASSIFICATION` right after a
  statement with no semicolon. `ADD` is not a word that starts a statement
  for the external scanner, because `ADD` comes in the options of a
  statement, as in `CREATE EVENT SESSION s ON SERVER ADD EVENT e`.
- The statements of Azure Synapse Analytics, Analytics Platform System and
  Microsoft Fabric that SQL Server does not have, such as `CREATE
  MATERIALIZED VIEW`, `CREATE REMOTE TABLE AS SELECT`, `RENAME OBJECT`,
  `COPY INTO` and `EXPLAIN`.

These faults remain:

- `THROW` is not reserved, but a word at the end of a statement that is
  `THROW` starts a statement, as in `SELECT * FROM t THROW 50000, 'x', 1`.
  The server reads such a `THROW` as the alias of the table.
- The grammar takes `GO` in a block, and the server does not.
- The grammar takes the clauses of `UPDATE`, `DELETE` and `MERGE` after
  their first part in any order, and more than once, as it takes the
  clauses of a query.
- The grammar takes the parts of a constraint and the properties of a
  column in any order, and `OR ALTER` after `CREATE` for each kind of
  object.
- The statements that define objects with no rule of their own, such as
  `CREATE LOGIN` and `ALTER DATABASE`, read their options as a list of
  words. The words in the tree are keywords, also where they name an
  object, as in `CREATE USER u FOR LOGIN l`.
- `FOREIGN` is not in the reserved set, so it can be a name where the
  server finds an error.

- The reserved words that name functions, such as `LEFT`, `COALESCE` and
  `CURRENT_USER`, are not in the reserved set, because each group of them
  is one token. So such a word can be an alias, as in `SELECT a left`,
  where the server finds an error.
- `PRECISION` is not in the reserved set, because scripts read the column
  `precision` of `sys.columns` with no brackets.
- `ENABLE` and `DISABLE` are one token, so a batch cannot start with the
  call of a procedure with the name `enable` or `disable` and no `EXECUTE`.
- The command of `DBCC` and the words of its options are keywords in the
  tree, as the options of the other statements that end in a list of words
  are.

### Forms that the SQLite grammar leaves out

The SQLite grammar that xo writes (D126) reads each statement of the SQL
reference of SQLite. It leaves out these forms:

- A block comment with no `*/`, which SQLite reads to the end of the input.
  A regular expression of tree-sitter cannot find the end of the input, so
  the grammar needs an external scanner for it.
- A name in single quotes where SQLite takes a name, as in
  `SELECT * FROM 'a'`. A string is a name only as an alias and as the
  schema of `ATTACH`.
- Two constraints of a table with no comma between them, as in
  `PRIMARY KEY (a) UNIQUE (b)`. SQLite takes them. After a column, the
  grammar cannot tell a constraint of the table from a constraint of the
  column with one token of lookahead.
- A semicolon in an argument of a module, as in
  `USING define((SELECT 1; SELECT 2))`. SQLite gives the text of the
  argument to the module.

### Forms that the DuckDB grammar leaves out

The DuckDB grammar that xo writes (D126) reads each statement of the SQL
documentation of DuckDB 2.0. It does not read the statements of the
community extension DuckPGQ, such as `CREATE PROPERTY GRAPH`, `DROP
PROPERTY GRAPH` and `GRAPH_TABLE` in `FROM`. A user turns on their syntax
with `SET active_grammar_extensions`, and they are not a part of the SQL of
DuckDB. The grammar leaves out these forms of a query:

- A name with more parts than four for a column, three for a table and
  three for a function or a type, as in `SELECT * FROM db.s1.s2.t`. DuckDB
  2.0 lets a schema hold other schemas. With one token of lookahead, the
  grammar cannot tell where the schemas of such a name end. A column with
  more parts is a field of a struct, as in `db.s.t.c.f`.
- A join on the right side of a join with no parentheses, as in
  `a JOIN b JOIN c ON x ON y`.
- `INTERVAL` as the name of a column, because `INTERVAL` starts an
  interval, as in `INTERVAL (x) DAYS`.
- `TRY_CAST` as the name of a column.
- A star in `ORDER BY` and `GROUP BY`, as in `ORDER BY *`. It costs about
  40 states.
- `SELECT * EXCEPT SELECT ...`, a set operation after a star with no
  `FROM`. The grammar reads `EXCEPT` after a star as the start of the
  columns that the star leaves out.
- A type with modifiers before a string, as in `DECIMAL(4,2) '1.23'`, and
  a cast after `IS NULL`, as in `x IS NULL::VARCHAR`.
- `//` before a comment with no space, as in `8//**/2`, which the lexer
  reads as `//` and `**`.
- A comma after the last item of a list, as in `GROUP BY a,`. The grammar
  takes it only after the rows of `VALUES` and their values, the columns
  of `SELECT`, the tables of `FROM`, a list in brackets and a list of
  names, because each such list costs parse states.

The grammar leaves out these forms of the statements that change rows or
the schema:

- `COLLATE` after the type of `ALTER COLUMN ... TYPE`, as in `TYPE VARCHAR
  COLLATE nocase`. It costs about 50 states.
- A table macro with more than one definition, when the query of a
  definition before a comma has no parentheses. A comma after such a query
  can continue the query, so the grammar takes the query of such a
  definition only in parentheses, as in `AS TABLE (SELECT 1), (a) AS TABLE
  SELECT a`.
- `OVERRIDING SYSTEM VALUE` and `OVERRIDING USER VALUE` of `INSERT`, which
  DuckDB reads and ignores, an alias after `TRUNCATE`, a name in quotes
  after `CONSTRAINT`, a dotted name of an option, as in `WITH (a.b = 1)`,
  and a comma after the last assignment of `UPDATE`. The documentation does
  not show them.
- `ALTER TYPE ... ADD VALUE` and `ALTER TYPE ... REMOVE VALUE`, which the
  tests of DuckDB use and the documentation does not show.

The grammar leaves out these forms of the other statements:

- A path of `ATTACH` that is a name or any other expression, as in
  `ATTACH x.path`. The grammar takes a string, a parameter, a function call,
  an expression in parentheses, and such values with an operator between
  them, as `docs/GRAMMAR.md` says.
- The options of `ATTACH TO EXTERNAL RESOURCE` before `AS`. The grammar
  takes them after the alias.
- The options of `COPY` with no parentheses, as in `COPY t TO 'f' CSV
  HEADER`, the form of PostgreSQL, and an option whose value holds the
  items of `ORDER BY`, as in `ORDER_BY (a DESC)`.
- `DEFAULT` as the value of an option, as in `EXPLAIN (FORMAT default)`.
- `DESCRIBE`, `SUMMARIZE` and `SHOW` in parentheses where a query can
  come, as in `CREATE VIEW v AS (SHOW TABLES)`. The grammar takes them in
  parentheses in `FROM`.
- `SET SCHEMA 'name'` and `LOAD ... AS alias`, which the tests of DuckDB
  use and the documentation does not show.

### Forms that the ClickHouse grammar leaves out

The ClickHouse grammar that xo writes (D127) reads each statement of the
SQL reference of ClickHouse. The grammar leaves out these forms of a
query:

- The names of the columns of a common table expression, as in `WITH t(a,
  b) AS (SELECT 1, 2)`. After `WITH t(`, the grammar cannot tell the
  columns from the arguments of a function `t`.
- The white space of Unicode, such as U+3000, between two tokens. It cost
  more than 1,000 parse states.
- `INTERVAL` with an expression that holds an operator, as in `INTERVAL
  number * 20 DAY`. The value is a number, a string, a name, a function
  call, a query parameter or an expression in parentheses.
- `SETTINGS` among the arguments of a table function, as in
  `generateRandom('x JSON', 1, SETTINGS max_json_depth = 4)`, because
  `Settings` names a column of `system.query_log`.
- `EXPLAIN` in parentheses as a value, as in `SELECT (EXPLAIN SYNTAX
  SELECT 1)`. It puts 12 more parse states in the large form. `EXPLAIN` in
  parentheses in `FROM` is a subquery.
- A column named `from` after an opening parenthesis, as in `SELECT (from
  IN (1))`, because a query in parentheses can start with `FROM`. The
  grammar reserves no word (D131), and where the keyword `FROM` can come,
  the word is the keyword. For the same reason, `SELECT a, from FROM t`
  reads `from` as the start of the clause `FROM`.
- The pipe operator `|>`, the composition of functions with `|`, `LIMIT
  AFTER`, `LIMIT UNTIL`, `STREAM`, the operators `~` and `!~` of
  PostgreSQL, `* ILIKE 'pattern'`, `VALUES` and `ARRAY[...]` of the
  dialect of Trino, the variables `@@name` of MySQL, and a star as a
  value, as in `[*, 1]` and `(*,).1`. The stateless tests of ClickHouse
  use them in a few files.
- A lambda after `APPLY` with no parentheses, as in `* APPLY x ->
  toString(x)`. After `APPLY x`, the upstream tool cannot tell the name of
  a function from the parameter of a lambda. `APPLY (x -> toString(x))`
  works.

The grammar leaves out these forms of the changes of data and schema:

- An expression as the value of a setting of `INSERT`, as in `INSERT INTO
  t SETTINGS a = 1 + 1 VALUES (1)`. A query can come after the settings,
  so the value is a literal or a name. Any expression cost about 1,050
  parse states.
- `PARTITION BY` after the table function of `INSERT`, as in `INSERT INTO
  FUNCTION s3(...) PARTITION BY a VALUES (1)`. After the value of
  `PARTITION BY`, an opening parenthesis can start the arguments of a
  function or the columns, and the grammar cannot tell them apart.
- `RETURNING` of `INSERT`, which is new in ClickHouse.
- `IN PARTITION` after the assignments of `UPDATE` and of `ALTER TABLE
  UPDATE`, as in `UPDATE t SET a = 1 IN PARTITION 2 WHERE 1`. `IN` after
  the value of an assignment starts the operator `IN`. `IN PARTITION`
  with more than one partition, as in `DELETE FROM t IN PARTITION 1, 2
  WHERE 1`.
- An expression with an operator and no parentheses in `CREATE INDEX`, as
  in `CREATE INDEX i ON t a + b TYPE minmax`. With no parentheses, the
  index takes a column or a function call.
- A column named `index`, `projection`, `constraint` or `primary` in the
  columns of `CREATE TABLE`. The word is the keyword that starts an
  element.
- `ON CLUSTER` after the columns of `CREATE VIEW`. It comes after the name
  of the view.
- `INNER ENGINE` of a window view. It starts with a word of a join, and
  the tool reports a conflict with a join after the query.
- `TO DISK` and `TO VOLUME` in a `TTL` after an interval with a string, as
  in `TTL d + INTERVAL '1' DAY TO DISK 'a'`, because `TO` after the unit
  starts a second unit, as in `INTERVAL '1:30' HOUR TO MINUTE`.
- `MODIFY QUERY` with other actions in one `ALTER TABLE`, a list of
  actions in parentheses, and `ADD ENUM VALUES` of `MODIFY COLUMN`.
- The clauses of the engine `TimeSeries`, such as `SAMPLES INNER COLUMNS`,
  `UNIQUE KEY`, `CODEC` of a subcolumn in the columns of a table, the
  options of MySQL in `CREATE TABLE`, such as `DEFAULT CHARSET`, `CREATE
  FUNCTION` with `LANGUAGE WASM`, and the casts in the arguments of a type,
  as in `AggregateFunction(quantile(0.5::Float64), Float64)`.

The grammar leaves out these forms of the other statements:

- A table or a column that has the name of a word of a command of
  `SYSTEM` or of a form of `SHOW`, as in `SYSTEM FLUSH LOGS logs`. The word
  is a keyword there.
- A clause of `DESCRIBE`, such as `FORMAT`, after a set operation, as in
  `DESCRIBE (SELECT 1) UNION ALL (SELECT 2) FORMAT TSV`, and an alias after
  a query in parentheses, as in `DESCRIBE (SELECT 1) AS s`. The clauses of
  `DESCRIBE` after any query put more than 400 more parse states in the
  large form (D133).
- The form of the limits of a quota that puts `MAX` after the name, as in
  `FOR 1 YEAR ERRORS MAX 1`, and `RESULT ROWS MAX`.
- `FOR` and a resource after a setting of a workload, as in `SETTINGS
  max_io_requests = 10 FOR disk_write`.
- An expression with an operator after `VALID FOR`, as in `VALID FOR
  INTERVAL 1 DAY + INTERVAL 1 HOUR`. The value is one interval.
- `EXPLAIN` of `OPTIMIZE`, because `optimize` names a setting of
  `EXPLAIN`, and `EXPLAIN` in parentheses after `EXPLAIN`.
- `REPLACE INTO` of MySQL. The SQL reference of ClickHouse does not show
  it.
- The statements of the other query languages of ClickHouse, such as KQL,
  PRQL and LogsQL.

### Forms that the Trino grammar leaves out

The Trino grammar that xo writes (D127) reads each statement of the SQL
documentation of Trino. The grammar leaves out these forms of a query:

- `RUNNING` and `FINAL` before a function call inside another expression,
  as in `MEASURES concat_ws('', RUNNING array_agg(x)) AS m`. The grammar
  takes them at the start of a measure and of a condition of `DEFINE`. A
  column named `final` or `running` breaks where they can start any
  function call.
- The JSON simplified accessor with a string after the dot, as in
  `j.'foo'`, and an item method after a path of three or more members, as
  in `j.o.qty.integer()`. `j.name.bigint()` works.
- A column named `interval`, `row` or `array` where an expression starts,
  because the word starts a literal or a constructor there.
- The precedence of the predicate `MATCH`. The grammar binds it more
  loosely than `AND` and `OR`, so `a AND r MATCH (q)` reads as `(a AND r)
  MATCH (q)`. Then `MATCH` after the condition of `WHERE` in `NEAREST`
  starts the clause `MATCH`.
- `COPARTITION` among the arguments of a table function, `OVERLAY` with
  `PLACING`, `DOUBLE PRECISION`, and names in backquotes. The
  documentation of Trino does not show them, and Trino refuses names in
  backquotes.
- The types `ARRAY<T>` and `MAP<K, V>` of earlier releases. The page of
  the types does not show them. One example of the SQL routines shows
  `map<varchar, bigint>`.

The grammar leaves out these forms of the other statements:

- A value with an operator after `DEFAULT` in a column, as in `c integer
  DEFAULT 1 + 1`. The value ends before an operator, so that `NOT NULL`
  after it is a part of the column. `DEFAULT (1 + 1)` works.
- A definition of a function with no `CREATE` and no `WITH`, as in
  `FUNCTION f() RETURNS integer RETURN 1`. The pages of the SQL routines
  show such definitions, but Trino takes them only after `CREATE` or
  `WITH`.

### Widen the generic SQL grammar

The first part of the generic SQL grammar that xo writes (D143) reads the
query, the changes of data, the common statements of the schema, and the
other statements as lists of options. The second part will read these
forms, which the documentation of Snowflake, BigQuery and Databricks
shows:

- The scripting of each dialect: the blocks of `BEGIN ... END` with
  `DECLARE`, `LET`, `IF`, `LOOP` and `FOR` of Snowflake, the procedural
  language of BigQuery, and the compound statements of Databricks. A
  procedure with such a body gets an error today.
- `CONNECT BY` with `START WITH`, `PRIOR` and `CONNECT_BY_ROOT`,
  `MATCH_RECOGNIZE`, `RESAMPLE`, the outer join with `(+)`, and `FOR
  UPDATE` of Snowflake.
- The strings in double quotes of BigQuery, as in `TIMESTAMP "2024-01-01"`.
  A name in double quotes is a name in the grammar.
- The pipe syntax with `|>` of BigQuery and Databricks, the operator `->>`
  of Snowflake, the expression `WITH(a AS x, a + 1)` of BigQuery, and
  `WHERE` and `HAVING MAX` among the arguments of an aggregate, as in
  `COUNT(DISTINCT x WHERE x > 0)`.
- `STRICT CORRESPONDING`, and the set operations with `INNER`, `LEFT` or
  `FULL` before them and `ON` after them, as in `FULL UNION ALL BY NAME`.
- `INSERT OVERWRITE DIRECTORY`, `ALTER TABLE ... ADD PARTITION` and the
  changes of a partition of Databricks, the version after `@` of a table,
  as in `events@v1`, and `WITH (options)` after a table.
- `CHANGES ... END(...)` of Snowflake, because `END` is a reserved word.
- The operator `DIV`, which put 13 parse states in the large form.
- The comments of Databricks that nest.
- The statements that the grammar does not read at all, such as `PUT`,
  `GET`, `LIST`, `REMOVE` and `UNDROP` of Snowflake, `EXPORT DATA`,
  `LOAD DATA` and `ASSERT` of BigQuery, and `OPTIMIZE` and `VACUUM` of
  Databricks.

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
