<div align="center">
  <a href="#status" title="Status">Status</a> |
  <a href="#sample-programs" title="Sample programs">Sample programs</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="#grammars" title="Grammars">Grammars</a> |
  <a href="#differences-from-upstream" title="Differences from upstream">Differences from upstream</a> |
  <a href="https://pkg.go.dev/github.com/xo/transit" title="Go Reference">Reference</a> |
  <a href="#license" title="License">License</a>
</div>

<br/>

[![Unit Tests][transit-ci-status]][transit-ci]
[![Go Reference][goref-transit-status]][goref-transit]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[transit-ci]: https://github.com/xo/transit/actions/workflows/test.yml "Test CI"
[transit-ci-status]: https://github.com/xo/transit/actions/workflows/test.yml/badge.svg "Test CI"
[goref-transit]: https://pkg.go.dev/github.com/xo/transit "Go Reference"
[goref-transit-status]: https://pkg.go.dev/badge/github.com/xo/transit.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/transit?display_name=tag "Latest Release"
[releases]: https://github.com/xo/transit/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# transit

`transit` is a pure Go port of [tree-sitter][ts]. tree-sitter is a parser
generator and an incremental parsing library. It builds a concrete syntax tree
of a source file, and it updates the tree after an edit without parsing the
whole file again. Editors use it to highlight code and to find structure in
code.

transit exists to serve two `xo` projects. [rline][rline] uses it to
highlight syntax as the user types, and [usql][usql] uses it to complete from
the context at the cursor. transit gives parsing information only.

Every Go package for tree-sitter today uses cgo, and a program that imports
one needs a C compiler for each target. A program that imports transit needs
the Go toolchain and nothing else.

## Status

The generator and its C backend are ported. They write the golden `parser.c`
and `node-types.json` of all 185 grammars of the set byte for byte, and on
2026-09-29 the gate of D9 holds: 151 grammars count, which ends phase 2. Ken
accepted the target API in [docs/API.md](docs/API.md) on 2026-10-01, which
ended phase 1 (D103). Phase 3 has ported the runtime: the parser, the tree,
the node, the tree cursor, and the query engine with the predicates of the
Rust binding. The Go runtime gives the same trees and the same query matches
as the C runtime for every corpus input of every fixture grammar. It parses
with the tables and the lexers of C grammars, through the test module. The
ported runtime tests of upstream pass. `StatesAt` gives the parse states at a
cursor (D57, D70). The package `inject` finds the injections of a text and
parses their layers, as upstream does (D72). The measurements of a prototype
of the Go output are recorded in D73, and they end phase 3. In phase 4, the Go
backend writes a grammar package with literal tables and a lexer as data
(D74). The 17 fixture grammars are Go packages in 15 modules under
`grammars/`, with their scanners ported to Go. Every test of phase 3 passes on
them, and the speed targets of D37 hold on them. Ken tagged `v0.1.0` of every
module on 2026-10-01, which ended phase 4.

Phase 5 makes the grammars for rline and usql. The SQL grammars, the grammars
of the languages of dbmeta, the usql grammar and the MySQL grammar are Go
packages, in 27 grammar modules in all. The package `inject` gives the
replaced text of a layer and its parse states, for completion in a SQL
statement (D111). Two sample programs show how rline and usql use transit
(D53). The option `BeginEndBlocks` of the usql grammar keeps the `BEGIN ...
END` body of a stored program in one statement (D112). On 2026-10-09 every
test of these grammars passes in Go, and the sample programs run, and that
ended phase 5.

Phase 6 ports the upstream commits after the base commit, in order (D116).
On 2026-10-10, phase 6 ported every upstream commit up to `e9930e09`, which
was the head of the upstream `master` on that day. transit follows upstream
from there. [docs/PLAN.md](docs/PLAN.md) holds the plan, and the decisions
record every answer that shapes it.

## Sample programs

The module `github.com/xo/transit/_example` holds two programs that use
transit as rline and usql will (D53). Run them in the folder `_example`:

| Program | What it does |
| --- | --- |
| `go run ./highlight -dialect mysql -style monokai -colors 256` | It types a text of usql one key at a time, parses again with the old tree after each key, and finds the layer of each SQL statement with `inject`. It prints the text in the colors of a style of the module `styles`, and the time of each edit |
| `go run ./complete -dialect postgres -text 'select * from '` | It completes at each cursor that it gets as a byte offset. It says whether the cursor is in a meta command, in a variable or in a SQL statement, and in a statement it lists the keywords and the kinds of node that can come next, from `Layer.StatesAt` |

The flag `-dialect` takes `athena`, `clickhouse`, `cql`, `duckdb`,
`generic`, `mysql`, `oracle`, `postgres`, `presto`, `sqlite3`,
`sqlserver` or `trino`.

## Documents

| Document | What it holds |
| --- | --- |
| [docs/PLAN.md](docs/PLAN.md) | the plan, the testing plan, the phases and any open question |
| [docs/GRAMMAR.md](docs/GRAMMAR.md) | the rules for adding a grammar and porting its scanner |
| [docs/CANDIDATES.md](docs/CANDIDATES.md) | the set of grammars that Ken accepted, and how it was measured |
| [docs/API.md](docs/API.md) | the target Go API, from the working C example |
| [docs/UPSTREAM.md](docs/UPSTREAM.md) | the rules for porting upstream tree-sitter and each change that it makes |
| [docs/RLINE.md](docs/RLINE.md) | what rline gets from transit, for the coding agents that work in rline |
| [docs/USQL.md](docs/USQL.md) | what usql gets from transit, for the coding agents that work in usql |
| [docs/NEOVIM.md](docs/NEOVIM.md) | the predicates of Neovim that transit does not support, and the grammars whose queries use them |
| [docs/BACKLOG.md](docs/BACKLOG.md) | the work that is known and not done |
| [docs/decisions/](docs/decisions/README.md) | every decision, one file each, with an index |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to change transit |
| [AGENTS.md](AGENTS.md) | the rules for a coding agent, which also hold for a person |

## Grammars

The Go backend writes a package for each grammar that Ken chooses. The set of
185 grammars is in [docs/CANDIDATES.md](docs/CANDIDATES.md), and
[docs/GRAMMAR.md](docs/GRAMMAR.md) says how a grammar is added. These are the
packages, one for each fixture grammar:

| Package | Upstream repository | Tag |
| --- | --- | --- |
| `github.com/xo/transit/grammars/bash` | [tree-sitter/tree-sitter-bash](https://github.com/tree-sitter/tree-sitter-bash) | `v0.25.0` |
| `github.com/xo/transit/grammars/c` | [tree-sitter/tree-sitter-c](https://github.com/tree-sitter/tree-sitter-c) | `v0.24.2` |
| `github.com/xo/transit/grammars/cpp` | [tree-sitter/tree-sitter-cpp](https://github.com/tree-sitter/tree-sitter-cpp) | `v0.23.4` |
| `github.com/xo/transit/grammars/embeddedtemplate` | [tree-sitter/tree-sitter-embedded-template](https://github.com/tree-sitter/tree-sitter-embedded-template) | `v0.25.0` |
| `github.com/xo/transit/grammars/go` | [tree-sitter/tree-sitter-go](https://github.com/tree-sitter/tree-sitter-go) | `v0.25.0` |
| `github.com/xo/transit/grammars/html` | [tree-sitter/tree-sitter-html](https://github.com/tree-sitter/tree-sitter-html) | `v0.23.2` |
| `github.com/xo/transit/grammars/java` | [tree-sitter/tree-sitter-java](https://github.com/tree-sitter/tree-sitter-java) | `v0.23.5` |
| `github.com/xo/transit/grammars/javascript` | [tree-sitter/tree-sitter-javascript](https://github.com/tree-sitter/tree-sitter-javascript) | `v0.25.0` |
| `github.com/xo/transit/grammars/jsdoc` | [tree-sitter/tree-sitter-jsdoc](https://github.com/tree-sitter/tree-sitter-jsdoc) | `v0.23.2` |
| `github.com/xo/transit/grammars/json` | [tree-sitter/tree-sitter-json](https://github.com/tree-sitter/tree-sitter-json) | `v0.24.8` |
| `github.com/xo/transit/grammars/php/php` | [tree-sitter/tree-sitter-php](https://github.com/tree-sitter/tree-sitter-php) | `v0.24.2` |
| `github.com/xo/transit/grammars/php/phponly` | [tree-sitter/tree-sitter-php](https://github.com/tree-sitter/tree-sitter-php) | `v0.24.2` |
| `github.com/xo/transit/grammars/python` | [tree-sitter/tree-sitter-python](https://github.com/tree-sitter/tree-sitter-python) | `v0.23.6` |
| `github.com/xo/transit/grammars/ruby` | [tree-sitter/tree-sitter-ruby](https://github.com/tree-sitter/tree-sitter-ruby) | `v0.23.1` |
| `github.com/xo/transit/grammars/rust` | [tree-sitter/tree-sitter-rust](https://github.com/tree-sitter/tree-sitter-rust) | `v0.24.0` |
| `github.com/xo/transit/grammars/typescript/tsx` | [tree-sitter/tree-sitter-typescript](https://github.com/tree-sitter/tree-sitter-typescript) | `v0.23.2` |
| `github.com/xo/transit/grammars/typescript/typescript` | [tree-sitter/tree-sitter-typescript](https://github.com/tree-sitter/tree-sitter-typescript) | `v0.23.2` |

These packages hold SQL grammars of the set for usql (D106). A module that
holds the language of one dialect has the name of the dialect, so the
package name can differ from the grammar name (D107):

| Package | Grammar | Upstream repository | Tag |
| --- | --- | --- | --- |
| `github.com/xo/transit/grammars/cql` | `cql` | [shotover/tree-sitter-cql](https://github.com/shotover/tree-sitter-cql) | `v0.2.0` |

The repository of `cql` has no highlight query, so xo wrote
`queries/highlights.scm` of this package (D113).

These packages hold the grammars of the languages of dbmeta that are not SQL
(D21, D23). A module that holds the language of one dialect has the name of
the dialect, so the package name can differ from the grammar name (D107).
`udovin/tree-sitter-yql` has no license file, so `grammars/ydb` has none
(D20):

| Package | Grammar | Upstream repository | Tag |
| --- | --- | --- | --- |
| `github.com/xo/transit/grammars/neo4j` | `cypher` | [taekwombo/tree-sitter-cypher](https://github.com/taekwombo/tree-sitter-cypher) | `M23-legacy` |
| `github.com/xo/transit/grammars/surrealdb` | `surrealql` | [surrealdb/surrealql-tree-sitter](https://github.com/surrealdb/surrealql-tree-sitter) | the branch `master` at `329dcec` |
| `github.com/xo/transit/grammars/sparql` | `sparql` | [GordianDziwis/tree-sitter-sparql](https://github.com/GordianDziwis/tree-sitter-sparql) | `0.1.0` |
| `github.com/xo/transit/grammars/graphql` | `graphql` | [bkegley/tree-sitter-graphql](https://github.com/bkegley/tree-sitter-graphql) | the branch `master` at `5e66e96` |
| `github.com/xo/transit/grammars/ydb` | `yql` | [udovin/tree-sitter-yql](https://github.com/udovin/tree-sitter-yql) | the branch `main` at `7e8d3e1` |

xo writes some grammars in this repository (D42, D104). The package
`github.com/xo/transit/grammars/usql` holds the grammar `usql` of the input
of usql: SQL statements, meta commands such as `\d` and variables such as
`:name` (D13). It is one language for every SQL dialect (D108). `Language`
gives the language with dollar quotes, block comments, dash comments and
backslash escapes. `LanguageFor` gives the language with the options of a
dialect, with the same tables:

```go
lang := usql.LanguageFor(usql.Options{BlockComments: true, HashComments: true, Backticks: true, DashComments: true, BackslashEscapes: true})
```

The fields of `usql.Options` are the flags of the type `Syntax` of dbmeta:
`DollarQuotes`, `BlockComments`, `SlashComments`, `HashComments`,
`Backticks`, `DashComments`, `Brackets` and `BackslashEscapes`. dbmeta
holds the value of each flag for each dialect. With `DashComments`, `--`
starts a comment. Without it, `--` is text, as in the relationship
`(a)--(b)` of Cypher (D139). With `Brackets`, a name in square brackets,
such as `[a b]` of SQL Server and SQLite, is a quoted identifier. Without
it, `[` is text, as in `a[1]` of PostgreSQL (D140). With
`BackslashEscapes`, a backslash escapes the next character in a string in
single quotes and in a name in double quotes, as in MySQL. Without it, a
backslash in such a string or name is text, as in `'C:\data\'`, and only
a doubled quote escapes a quote. A string of the
form `E'...'` of PostgreSQL keeps the backslash escapes (D142). Three more
fields are not flags of dbmeta. `BeginEndBlocks` and `AnonymousBlocks` are
described below. `Batches` keeps a batch of CQL,
from `BEGIN BATCH` to `APPLY BATCH`, in one statement. It is off by
default.

The package `github.com/xo/transit/grammars/mysql` holds the grammar
`mysql`, which xo writes from the reference manual of MySQL 8.4 (D42, D106).
It covers the SQL that MySQL 8.4 and MariaDB share, with backticks, `#` and
`-- ` comments, version comments such as `/*!40101 ... */`, the variables
`@name` and `@@session.name`, and stored programs with `BEGIN ... END`
bodies. It does not read the `DELIMITER` command of the mysql client. The
usql grammar ends a statement at each `;`, so by default it splits such a
body into several statements, and the MySQL grammar finds an error in each
part. The option `BeginEndBlocks` of `usql.Options` keeps the body in one
statement (D112). It is off by default. One identifier token covers each
keyword that MySQL does not reserve, and the reserved words of MySQL 8.4
are not identifiers, so the package adds about 1 MB to a binary (D123).

The package `github.com/xo/transit/grammars/oracle` holds the grammar
`oracle`, which xo writes from the SQL Language Reference and the PL/SQL
Language Reference of Oracle Database 23ai (D124). It covers the lexical
structure, with the strings of the form `q'[...]'`, the expressions, the
types, the function calls with their analytic clauses, and the full
`SELECT`, with `CONNECT BY`, `PIVOT`, `MODEL`, `MATCH_RECOGNIZE` and
`JSON_TABLE`. It also covers the statements that change data, such as
`INSERT`, `UPDATE`, `DELETE` and `MERGE`, the statements of transactions
and of sessions, such as `COMMIT` and `ALTER SESSION`, and `GRANT`,
`REVOKE`, `COMMENT`, `TRUNCATE` and the other statements of the SQL
Language Reference that do not create or change an object. It reads the
statements that define objects: `CREATE`, `ALTER` and `DROP` of tables,
indexes, views, materialized views, types, domains, property graphs and
the other objects of the manual, with the columns, the constraints and the
partitions of a table. The physical options of an object, such as
`STORAGE`, are a list of words. It reads PL/SQL in the same grammar,
because Oracle does not put PL/SQL in a quoted string: anonymous blocks,
`CREATE PROCEDURE`, `CREATE FUNCTION`, `CREATE PACKAGE`, `CREATE PACKAGE
BODY`, `CREATE TRIGGER`, `CREATE TYPE BODY`, the functions of `WITH`, each
declaration and statement of PL/SQL, and the conditional compilation with
`$IF`. A `/` alone on its line ends a statement, as in a script of
SQL*Plus. The option `BeginEndBlocks` of `usql.Options` keeps a stored
program, a package spec, a package body and a type body each in one
statement (D112, D144), and the option `AnonymousBlocks` keeps an
anonymous block, which starts with `BEGIN` or `DECLARE`, in one statement
(D128). Both are off by default. The package adds about 3.3 MB to a binary
(D125). The
grammar of `andreasmaierde/tree-sitter-plsql` stays in the golden set, but
it has no Go package.

The package `github.com/xo/transit/grammars/sqlserver` holds the grammar
`sqlserver`, which xo writes from the Transact-SQL reference of SQL Server
2025 (D124). It covers the lexical structure, with bracketed identifiers,
variables such as `@name` and `@@ROWCOUNT`, temporary tables such as `#t`,
binary and money constants and comments that nest. It covers the
expressions, the types, the function calls with `OVER` and `WITHIN GROUP`,
and the full `SELECT`, with `TOP`, `INTO`, `APPLY`, `PIVOT`, the table
hints, `FOR SYSTEM_TIME`, the rowset functions such as `OPENJSON`, `FOR
XML`, `FOR JSON` and `OPTION`. It also covers `INSERT`, `UPDATE`, `DELETE`
and `MERGE` with `OUTPUT`, `BULK INSERT`, `TRUNCATE TABLE`, the statements
of transactions, `SET`, `USE`, `EXECUTE`, `PRINT`, `RAISERROR`, `THROW`,
the control-of-flow language, such as `IF`, `WHILE`, `BEGIN ... END` and
`TRY ... CATCH`, `DECLARE` and the statements of cursors. It covers the
statements that define objects: `CREATE`, `ALTER` and `DROP` of tables,
with temporal, ledger, graph and memory-optimized tables, of each kind of
index, of views, schemas, synonyms, types, statistics, databases, logins,
users, roles and the other objects of the reference, and `GRANT`, `DENY`,
`REVOKE` and `UPDATE STATISTICS`. It reads `CREATE` and `ALTER` of
procedures, functions and triggers, with their parameters and options, the
natively compiled modules with `BEGIN ATOMIC`, and the modules of an
assembly. It reads `ENABLE TRIGGER`, the statements of Service Broker, such
as `SEND`, `RECEIVE` and `WAITFOR (RECEIVE ...)`, `BACKUP`, `RESTORE`,
`DBCC`, `KILL`, `CHECKPOINT` and the other statements of the reference. A
statement does not need a semicolon at its end. A line with only `GO` and
an optional count ends a batch, as in sqlcmd. The body of a procedure or a
trigger runs to the end of its batch. The package adds about 1.5 MB to a
binary. The grammar of `Crary-Systems/tree-sitter-tsql` stays in the
golden set, but it has no Go package.

The package `github.com/xo/transit/grammars/sqlite3` holds the grammar
`sqlite3`, which xo writes from the SQL reference of SQLite (D126). It
covers the lexical structure, with the names in double quotes, in brackets
and in backquotes, the BLOB literals such as `X'0A'`, the numbers with `_`
between their digits, the parameters `?NNN`, `:name`, `@name` and `$name`,
and the comments. It covers the expressions with the precedence of the
reference, with `->` and `->>`, `IS DISTINCT FROM`, `GLOB`, `REGEXP`,
`MATCH`, `CAST` and `RAISE`, the function calls with `FILTER`, `OVER` and
`ORDER BY` in their arguments, and the full `SELECT`, with `WITH`, the
joins, the table-valued functions, `INDEXED BY`, `VALUES`, `WINDOW` and the
compound operators. It covers each other statement of the reference:
`INSERT` and `REPLACE` with the upsert, `UPDATE` with `FROM`, `DELETE`,
`RETURNING`, `CREATE TABLE` with each constraint, the generated columns,
`WITHOUT ROWID` and `STRICT`, `CREATE VIRTUAL TABLE` with the arguments of
a module, `ALTER TABLE`, `CREATE INDEX`, `CREATE VIEW`, `CREATE TRIGGER`,
`DROP`, `ATTACH`, `DETACH`, `PRAGMA`, the statements of transactions and
savepoints, `ANALYZE`, `REINDEX`, `VACUUM` and `EXPLAIN`. SQLite reserves
few of its keywords, so one identifier token covers each keyword that
SQLite does not reserve. The grammar has no external scanner. The package
adds about 0.6 MB to a binary.

The package `github.com/xo/transit/grammars/duckdb` holds the grammar
`duckdb`, which xo writes from the SQL documentation of DuckDB 2.0 (D126).
It covers the lexical structure, with the names in double quotes and in
backquotes, the strings with `E` before them, the dollar-quoted strings,
the bit strings such as `B'0101'` and the hex strings such as `X'CAFE'`,
the numbers with `_` between their digits, the parameters `?`, `$1` and
`$name`, and the comments, which nest. It covers the expressions with the
precedence of DuckDB, with `::` and `TRY_CAST`, the lists, the structs and
the maps, the subscripts and the slices, the lambdas in both forms, the
list comprehensions, the calls of a function after a dot, `COLUMNS` and
`*COLUMNS`, the star with `EXCLUDE`, `REPLACE` and `RENAME`, the named
arguments, `FILTER`, `WITHIN GROUP`, `IGNORE NULLS` and the window
functions. It covers the types, and the full query: `WITH` with
`RECURSIVE` and `USING KEY`, `SELECT` with `DISTINCT ON`, a query that
starts with `FROM`, each kind of join with `ASOF`, `POSITIONAL`, `SEMI`,
`ANTI` and `NEAREST`, the table functions, a file as a table, `PIVOT` and
`UNPIVOT` in both forms, the samples, `GROUP BY ALL`, `QUALIFY`, the set
operations with `BY NAME`, `ORDER BY ALL` and `LIMIT` with a percent. It
covers the statements that change rows: `INSERT` with `OR REPLACE`, `BY
NAME`, `ON CONFLICT` and `RETURNING`, `UPDATE`, `DELETE`, `TRUNCATE` and
`MERGE INTO`. It covers the statements that change the schema: `CREATE
TABLE` with its constraints and its generated columns, `ALTER TABLE`, the
views, the indexes, the sequences, the schemas, the types, the macros, the
secrets, the triggers and `DROP`. It covers each other statement of the
documentation: `COPY`, `EXPORT DATABASE`, `IMPORT DATABASE`, `ATTACH`,
`DETACH`, `USE`, `CONNECT`, `CHECKPOINT`, `VACUUM`, `ANALYZE`, `INSTALL`,
`LOAD`, `UPDATE EXTENSIONS`, the extension repositories, `SET`, `RESET`,
`PRAGMA`, `DESCRIBE`, `SUMMARIZE`, `SHOW`, `EXPLAIN`, the prepared
statements, `CALL`, the statements of transactions and `COMMENT ON`. It
does not read the statements of the extension DuckPGQ, such as `CREATE
PROPERTY GRAPH`. The external scanner reads the comments and the
dollar-quoted strings. The package adds about 1.3 MB to a binary.

The package `github.com/xo/transit/grammars/clickhouse` holds the grammar
`clickhouse`, which xo writes from the SQL reference of ClickHouse (D127).
It covers the lexical structure, with the names in backquotes and in
double quotes, the strings with backslash escapes, the heredoc strings
such as `$doc$text$doc$`, the numbers in each form with `0x`, `0b`, `inf`
and `nan`, the query parameters such as `{id:UInt32}`, and the comments
with `--`, `//`, `#` and `/* */`, which nest. It covers the expressions
with the precedence of ClickHouse, with the ternary operator `?:`, the
lambdas, `::` and `CAST`, `AT TIME ZONE` and `AT LOCAL`, the arrays, the
tuples and the maps, the elements of a tuple such as `t.1`, the paths of a
JSON column, an alias
with `AS` in any expression, `GLOBAL IN`, the parametric aggregate
functions such as `quantile(0.9)(x)`, `FILTER`, the window functions,
`COLUMNS` and the star with `EXCEPT`, `REPLACE` and `APPLY`. It covers the
types, such as `Nullable`, `LowCardinality`, `Tuple` with names, `Map`,
`Nested`, `Enum8` with its values, `DateTime64` with a time zone,
`AggregateFunction`, `JSON`, `Variant` and `Dynamic`. It covers the full
query: `WITH` with expressions and subqueries, `SELECT` with `DISTINCT
ON` and `TOP`, a query that starts with `FROM`, `FINAL`, `SAMPLE`, `ARRAY
JOIN`, each kind of join with `GLOBAL`, `ANY`, `ASOF`, `SEMI`, `ANTI` and
`PASTE`, the table functions, `PREWHERE`, `GROUP BY` with `WITH ROLLUP`,
`WITH CUBE` and `WITH TOTALS`, `QUALIFY`, `ORDER BY` with `WITH FILL` and
`INTERPOLATE`, `LIMIT BY`, `LIMIT` with `WITH TIES`, the set operations,
`SETTINGS`, `INTO OUTFILE` and `FORMAT`. It covers `EXPLAIN` of a query.
It covers `INSERT` with `VALUES`, a query, `FROM INFILE`, a table function
and the data of a format after `FORMAT`, and `DELETE`, `UPDATE` and
`TRUNCATE`. It covers `CREATE DATABASE`, `CREATE TABLE` with its columns,
indexes, projections, constraints, engine, keys, `TTL` and `AS`, `CREATE
VIEW`, `MATERIALIZED VIEW` with `REFRESH`, `LIVE VIEW` and `WINDOW VIEW`,
`CREATE DICTIONARY`, `CREATE FUNCTION`, `CREATE NAMED COLLECTION`, `CREATE
INDEX` with `ASC` and `DESC`, and each action of `ALTER TABLE`. It covers
`DROP`, `DETACH`, `ATTACH`, `UNDROP`, `RENAME`, `EXCHANGE` and `OPTIMIZE`.
It covers each form of `SHOW`, `DESCRIBE`, `EXISTS`, `CHECK`, `KILL`, each
command of `SYSTEM`, `SET`, `SET ROLE` and `USE`. It covers `GRANT`,
`REVOKE`, and `CREATE` and `ALTER` of the users, the roles, the policies,
the quotas and the settings profiles. It covers `MOVE`, `WATCH`, `BACKUP`,
`RESTORE`, `EXECUTE AS`, the statements of transactions, the workloads,
the resources, the handlers, the tokens, `EXPLAIN` of each statement, and
`PARALLEL WITH`. The external scanner reads the comments, the heredoc
strings and the data of `INSERT` after `FORMAT`. The package adds about
1.9 MB to a binary.

The package `github.com/xo/transit/grammars/trino` holds the grammar
`trino`, which xo writes from the SQL documentation of Trino (D127). It
also serves the SQL of Presto and of Athena. It covers the lexical
structure, with the names in double quotes, the strings with doubled
quotes, the Unicode strings such as `U&'caf\00E9'`, the binary strings
such as `X'CAFE'`, the numbers with `_` and in hexadecimal, octal and
binary form, the parameter `?`, and the comments with `--` and `/* */`. It
covers the expressions with the precedence of Trino, with `CAST` and
`TRY_CAST`, `AT TIME ZONE` and `AT LOCAL`, `||`, `LIKE` with `ESCAPE`, `IS
DISTINCT FROM`, the quantified comparisons, `EXISTS`, `UNIQUE` and
`MATCH`, `CASE` with the predicates of its simple form, the lambdas, the
arrays, `ROW`, the subscripts, the fields of a row, the methods of a value
such as `'abc'.length()`, the functions of a type such as
`varchar::chr(65)`, the typed literals, the intervals, the values such as
`CURRENT_DATE`, the functions of JSON, `LISTAGG`, `FILTER`, the window
functions with their frames, and the special forms such as `EXTRACT`,
`TRIM` and `SUBSTRING`. It covers the types, such as `DECIMAL(10, 2)`,
`ROW` with names, `MAP`, `TIMESTAMP(6) WITH TIME ZONE` and `INTERVAL DAY
TO SECOND`. It covers the full query: `WITH` with `RECURSIVE`, `WITH
SESSION` and `WITH FUNCTION` with the body of a SQL routine, each kind of
join, `LATERAL`, `UNNEST`, the table functions with `DESCRIPTOR`,
`TABLESAMPLE`, `MATCH_RECOGNIZE`, `PIVOT`, `JSON_TABLE`, `NEAREST`, `GROUP
BY` with `AUTO`, `ROLLUP`, `CUBE` and `GROUPING SETS`, `WINDOW`, the set
operations with `CORRESPONDING`, `ORDER BY`, `OFFSET`, `LIMIT` and `FETCH`
with `WITH TIES`. It covers `INSERT`, `UPDATE`, `DELETE` and `MERGE`, with
the branches of a table after `@`, and `TRUNCATE`. It covers `CREATE`,
`ALTER` and `DROP` of the tables, the views, the materialized views, the
schemas, the catalogs, the roles, the branches and the functions, with
the properties after `WITH`, `CREATE TABLE AS` with `WITH NO DATA`, and
each action of `ALTER TABLE`, with `EXECUTE`, and `ALTER ... SET
AUTHORIZATION` of each kind of object. It covers `COMMENT ON`, `REFRESH
MATERIALIZED VIEW`, `ANALYZE`, `GRANT`, `REVOKE` and `DENY` on each kind of
object, each form of `SET` and `RESET`, `USE`, `PREPARE`, `EXECUTE` with `IMMEDIATE`,
`DEALLOCATE PREPARE`, `DESCRIBE`, each form of `SHOW`, `EXPLAIN` and
`EXPLAIN ANALYZE` with their options, `CALL`, and the statements of
transactions. The grammar needs no external scanner. The package adds
about 1.0 MB to a binary.

The package `github.com/xo/transit/grammars/sql` holds the grammar `sql`,
the generic SQL grammar, which xo writes for the dialects that have no
grammar of their own (D143). It replaces the grammar of
`DerekStride/tree-sitter-sql`, and its package has the same functions. It
reads the SQL that most dialects share, and the forms that several of them
add, from the SQL documentation of Snowflake, BigQuery and Databricks. It
is lenient, because it serves highlighting, so a form of one dialect can
parse as a generic form. It covers the comments with `--`, `//`, `#` and
`/* */`, the strings with doubled quotes and backslash escapes, the strings
in three quotes, the raw strings, the bytes and the strings in `$$`, the
names in double quotes and in backticks, and the parameters `?`, `:name`,
`@name`, `$1` and `${name}`. It covers the expressions, with `CAST`,
`TRY_CAST`, `SAFE_CAST` and `::`, `LIKE`, `ILIKE`, `RLIKE` and `REGEXP`
with `ANY` and `ALL`, `IS DISTINCT FROM`, the function calls with
`DISTINCT`, `FILTER`, `WITHIN GROUP`, `IGNORE NULLS` and `OVER`, the window
frames, the named arguments, the lambdas, the arrays, the `STRUCT` values,
the object constants, the access to a field with `:` and `.`, the
subscripts, the intervals, `DIV`, the expression `WITH` of BigQuery, and
`WHERE` and `HAVING MAX` in the arguments of an aggregate. A string in
double quotes after a type, as in `DATE "2024-01-01"`, is a constant of
BigQuery where no alias can come. It covers the full query: `WITH` with
`RECURSIVE`, `SELECT` with `DISTINCT`, `TOP` and `AS STRUCT`, the star with
`EXCEPT`, `EXCLUDE`, `REPLACE`, `RENAME` and `ILIKE`, each kind of join with
`LATERAL`, `APPLY`, `ASOF` and `MATCH_CONDITION`, `UNNEST`, the table
functions, the stages of Snowflake, `VALUES` in `FROM`, the time travel of
each of the three dialects, `PIVOT`, `UNPIVOT`, `TABLESAMPLE`, `SAMPLE`,
`LATERAL VIEW`, `GROUP BY` with `ALL`, `ROLLUP`, `CUBE` and `GROUPING
SETS`, `HAVING`, `WINDOW`, `QUALIFY`, `ORDER BY`, `LIMIT`, `OFFSET`,
`FETCH`, `SORT BY`, `FOR UPDATE`, `START WITH` and `CONNECT BY`,
`MATCH_RECOGNIZE`, the outer join with `(+)`, the version of a table
after `@`, the pipe syntax with `|>` and the set operations with `MINUS`,
`BY NAME` and `CORRESPONDING`. It
covers `INSERT` with `OVERWRITE`, `PARTITION`, `REPLACE WHERE` and the
multi-table form and `OVERWRITE DIRECTORY`, `UPDATE`, `DELETE`, `MERGE`,
`TRUNCATE` and `COPY INTO`.
It covers `CREATE`, `ALTER` and `DROP` of the tables, the views, the
materialized views, the indexes, the functions and the procedures, with
`OR REPLACE`, `IF NOT EXISTS`, `TEMPORARY`, `TRANSIENT`, `CLONE`, the types
`ARRAY<T>`, `STRUCT<...>` and `MAP<K, V>` and the constraints. The CREATE,
ALTER and DROP of any other object, such as a schema, a sequence or a
stage, and the options of each object, are lists of options. It covers
`COMMENT ON`, `USE`, `SET`, `UNSET`, `SHOW`, `DESCRIBE`, `EXPLAIN`,
`GRANT`, `REVOKE`, `CALL`, `EXECUTE IMMEDIATE`, the statements of
transactions, `PUT`, `GET`, `UNDROP`, `EXPORT DATA`, `LOAD DATA`,
`ASSERT`, `OPTIMIZE` and `VACUUM`. It covers the scripting of the three
dialects: the blocks with `DECLARE` and `EXCEPTION`, `IF`, `CASE`,
`LOOP`, `WHILE`, `REPEAT`, `FOR`, the cursors and the handlers, also as
the body of a procedure. The grammar needs no external scanner, so a
comment does not nest. The package adds about 1.3 MB to a binary, where
the package of the grammar of `DerekStride/tree-sitter-sql` added about
3.0 MB.

The package `github.com/xo/transit/grammars/postgres/postgres` holds the
grammar `postgres`, which xo writes from the reference manual of
PostgreSQL 19 (D122). It covers the lexical structure, the expressions, the
types, the function calls and the full `SELECT`, with `WITH`, `VALUES` and
`TABLE`. It also covers `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `COPY`, the
statements of transactions, of prepared statements and of cursors, and the
utility statements, such as `EXPLAIN`, `SET` and `VACUUM`. It covers each
statement that defines objects, such as `CREATE TABLE`, `CREATE FUNCTION`,
`CREATE TRIGGER`, `CREATE ROLE`, `GRANT` and `CREATE PUBLICATION`, and each
other command of the manual. `docs/BACKLOG.md` lists the forms that it
leaves out. The variables of psql are part of the usql grammar, which gives
this grammar a placeholder of the same length in place of each variable
(D101).

The package `github.com/xo/transit/grammars/postgres/plpgsql` of the same
module holds the grammar `plpgsql`, which xo writes from the chapter on
PL/pgSQL of the same manual (D122). It reads the statements of PL/pgSQL in
the body of a function or of a `DO` block. Each expression and each SQL
statement in the body is one node, `sql_expression` or `sql_statement`, and
the injection query of `plpgsql` gives its text to the grammar `postgres`.
The body of a function or of a `DO` block in dollar quotes is the node
`function_body` of `postgres`, and the injection query of `postgres` gives
it to `plpgsql` or to `postgres`, as the `LANGUAGE` of the statement says.

Some grammars ship queries written for Neovim, which transit does not support
now. [docs/NEOVIM.md](docs/NEOVIM.md) lists them.

## Differences from upstream

transit differs from upstream tree-sitter only where a decision says so:

1. For a `grammar.json` that is not valid JSON, the generator gives the error
   text of Go's `encoding/json` (D66).
2. Two nodes compare with `==`, which also compares their positions.
   `Node.Equal` compares them as `ts_node_eq` does (D68).
3. The Go backend stops with an error for a large character set of
   surrogates only, where the C code of upstream reads past its array (D78).

transit also adds an API that upstream does not have, one decision each
(D28). [docs/UPSTREAM.md](docs/UPSTREAM.md) says when one can be made, and
[docs/API.md](docs/API.md) gives each one:

1. `Parser.StatesAt` gives the parse states at a byte offset, before the
   parser recovers from an error (D57, D70).
2. `NodeType`, `FieldInfo` and `NodeKind` give `node-types.json` in Go, as
   the function `NodeTypes` of a grammar package returns it (D76).
3. `inject.WithReplacer` replaces the text of nodes before a layer is parsed
   (D101).
4. `inject.Layer.Text` and `inject.Layer.StatesAt` give the text that a layer
   parses and the parse states of the layer at an offset (D111).

transit does not evaluate the predicates of Neovim, and upstream does not
either ([docs/NEOVIM.md](docs/NEOVIM.md)).

## License

transit is under the MIT license. See [LICENSE](LICENSE). Upstream
tree-sitter is under the MIT license too, and `LICENSE` keeps its copyright
line. Each grammar keeps the license of its own repository.

<div align="center">
  <a href="https://github.com/xo/usql" title="A command line client for many databases">usql</a> |
  <a href="https://github.com/xo/dburl" title="Database connection URLs">dburl</a> |
  <a href="https://github.com/xo/dbmeta" title="Database metadata">dbmeta</a> |
  <a href="https://github.com/xo/dbimp" title="Database drivers in pure Go">dbimp</a> |
  <a href="https://github.com/xo/cql" title="A database/sql driver for Cassandra">cql</a> |
  <a href="https://github.com/xo/dbtpl" title="Go code generated from a database">dbtpl</a> |
  <a href="https://github.com/xo/tblfmt" title="Tables of database results">tblfmt</a> |
  <a href="https://github.com/xo/rline" title="The line editor of usql">rline</a> |
  <a href="https://github.com/xo/transit" title="tree-sitter in pure Go, this project">transit</a>
</div>

[ts]: https://github.com/tree-sitter/tree-sitter
[rline]: https://github.com/xo/rline
[usql]: https://github.com/xo/usql
