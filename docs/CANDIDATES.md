# The candidate grammars

This document holds the set of grammars that tests the transit generator, and
later the Go runtime. Ken accepted the set on 2026-09-29 (D18). He adds or
removes a grammar by name.

The set has two goals (D16):

1. It reaches nearly every path of the C code that upstream `render.rs`
   writes, so that the C backend (D8) is tested on each one.
2. It reaches the parts of the runtime that rline and usql use: external
   scanners, injections, conflicts that the parser resolves at run time,
   error recovery and large grammars.

It prefers the grammars of the upstream `tree-sitter` organization.

## How the list was made

Every number here was measured on 2026-09-29:

1. The repositories of the `tree-sitter` and `tree-sitter-grammars`
   organizations on GitHub: 33 and 84 repositories.
2. The parser list of nvim-treesitter, `lua/nvim-treesitter/parsers.lua` at
   commit `728e031f6b11`. It names 323 parsers, with the repository and the
   folder of each. nvim-treesitter is the largest public list of grammars that
   editors install.
3. For each of the 340 repositories and folders, GitHub gave the stars, the
   date of the last push, whether it is archived or a fork, and the license.
   It also gave the size of `src/grammar.json`, `src/scanner.c`,
   `src/parser.c`, `tree-sitter.json` and `queries/highlights.scm`.
4. The `src/grammar.json` of 335 of them was read, and the features that the
   generator acts on were counted. "Features" below lists them.

Gemini and DeepSeek were asked for the most used grammars and for the SQL
grammars. Their lists of the 40 most used grammars match the measured list:
each language that they name is in it. Every SQL grammar that Gemini named
exists, and "SQL" below gives the facts of each.

## The input is grammar.json

A grammar is written in JavaScript, in `grammar.js`. The upstream tool
evaluates it with `dsl.js` and writes `src/grammar.json`. The generator then
reads only the JSON. `tree-sitter generate src/grammar.json` runs no
JavaScript. The code is `load_grammar_file` and `generate_parser_in_directory`
in `crates/generate/src/generate.rs`. Gemini and DeepSeek agree.

transit uses the upstream tool in the same way, and it starts from
`grammar.json` (D17). Three more inputs change `parser.c`, and the golden
files record each one:

1. The ABI version, from `--abi`. It is 14 or 15, and 15 is the default.
2. The version of the grammar, from `metadata.version` in `tree-sitter.json`.
   The tool looks for the file in the folder of the grammar and in each
   folder above it. `parser.c` holds the major, minor and patch numbers.
3. The merge of parse states, which `--disable-optimizations` turns off.

A grammar that requires another, as `typescript` requires `javascript`, still
gives a `grammar.json` that holds every rule. A parser that works can still
need the scanner of the other grammar.

Four of the candidates commit no `parser.c`, and one commits no
`grammar.json` either: `DerekStride/tree-sitter-sql`. For that one, the golden
harness keeps the `grammar.json` that the upstream tool writes.

## The tiers

| Tier | Rule | Grammars |
| --- | --- | --- |
| 1 | In the `tree-sitter` organization, not archived, pushed since 2024-01-01 | 30 |
| 2 | In the `tree-sitter-grammars` organization, not archived, pushed since 2025-01-01 | 83 |
| 3 | Elsewhere, in nvim-treesitter and not marked unmaintained there, 40 stars or more, pushed since 2025-06-01, a permissive license or no license (D20), not a fork | 58 |

The total is 171 grammars in the tiers. The SQL grammars of "SQL" and the
grammars of "The grammars for dbmeta's languages" join them (D18, D23). The 68
test grammars of upstream, `test/fixtures/test_grammars`, come in addition,
and they are always in the set. The 15 fixture grammars of `fixtures.json` are
all in tier 1.

These are left out of tier 1:

1. `tree-sitter-graph`. It is a tool, not a grammar.
2. `tree-sitter-cli`, `tree-sitter-swift`, `tree-sitter-toml`,
   `tree-sitter-tsq` and `tree-sitter-razor`. They are archived. The Swift and
   TOML grammars that editors use are in tier 3 and tier 2.

GitHub marks 14 repositories of `tree-sitter-grammars` as forks, among them
`toml`, `yaml`, `vue` and `vim`, because each one started as a fork of an older
repository. The organization now keeps them as the maintained copies, so tier
2 takes them. It does not take its forks of `haskell` and `julia`, because
tier 1 has the upstream repositories.

A grammar under the GPL, the LGPL or the MPL is not in tier 3. The golden
stage stores only hashes of files, but the Go stage copies files from the
grammar (see `GRAMMAR.md`). Ken can add such a grammar by name.

A grammar that is freely available and names no license is accepted (D20).
Three are in the set: `tree-sitter-grammars/tree-sitter-move`,
`madskjeldgaard/tree-sitter-supercollider` and `udovin/tree-sitter-yql`.
The rule of D20 holds for them in the Go stage too.

## Features

The scan of `grammar.json` counts these features. The letters are the ones in
the table at the end.

| Letter | Feature | What the generator does with it |
| --- | --- | --- |
| R | `reserved` word sets | writes `ts_reserved_words` and a reserved word set for each lexer mode (ABI 15) |
| N | named precedences, such as `prec('member', ...)` | orders the precedences by the `precedences` list |
| P | a `precedences` list | the same |
| E | an external token that the grammar also defines, or a string external | maps an external token to an internal one |
| X | an extra that is a rule and not a token (a candidate for a non-terminal extra) | writes a lexer mode of `(TSStateId)(-1)` at the end of a non-terminal extra |
| F | pattern flags, such as `i` for case | builds a lexer that ignores case, or reads Unicode sets |
| U | a Unicode class in a pattern, such as `\p{L}` | writes large character sets and `set_contains` |
| D | dynamic precedence, `prec.dynamic` | writes a `REDUCE` with a precedence that is not zero |
| S | supertypes | writes the supertype map (ABI 15) |
| W | a `word` token | writes the keyword lexer, `ts_lex_keywords` |

X is a candidate only. The scan finds an extra that names a rule. It cannot
tell whether the generator makes that rule a token, so the golden harness
measures X in the output.

The number of grammars in the set that have each feature:

| Feature | Tier 1 | Tiers 1 and 2 | All tiers | All 335 grammars |
| --- | --- | --- | --- | --- |
| external scanner tokens | 21 | 61 | 97 | 175 |
| E, external tokens that are also internal | 11 | 19 | 27 | 42 |
| W, word token | 23 | 66 | 93 | 162 |
| R, reserved words | 8 | 8 | 13 | 16 |
| S, supertypes | 21 | 57 | 72 | 102 |
| inline rules | 22 | 52 | 75 | 119 |
| declared conflicts | 21 | 57 | 89 | 161 |
| P, a precedences list | 9 | 18 | 22 | 45 |
| X, a rule as an extra | 8 | 13 | 29 | 67 |
| D, dynamic precedence | 18 | 36 | 59 | 89 |
| N, named precedences | 7 | 7 | 11 | 25 |
| immediate tokens | 23 | 72 | 108 | 199 |
| fields | 23 | 91 | 141 | 261 |
| named aliases | 27 | 93 | 146 | 261 |
| anonymous aliases | 21 | 50 | 83 | 130 |
| an alias of a visible rule | 27 | 63 | 106 | 181 |
| the `RESERVED` rule | 5 | 5 | 6 | 7 |
| U, Unicode classes | 15 | 34 | 51 | 89 |
| F, pattern flags | 3 | 15 | 22 | 42 |

Tier 1 alone has every feature, at least three times each. The reserved word
sets are the rarest feature. 16 grammars of 335 use them, and 13 of them are
in the set.

## The paths of the C template

`render.rs` writes a part of `parser.c` only when the grammar needs it. These
are the paths, and what reaches each:

| Path in `render.rs` | What reaches it | Measured by |
| --- | --- | --- |
| field names and field maps, with inherited fields | fields, and a field inside a hidden rule | the scan (fields), the harness (inherited) |
| alias sequences and unique aliases, named and anonymous | aliases | the scan |
| the non-terminal alias map | an alias of a rule | the scan |
| the supertype map | S, at ABI 15 | the scan |
| the keyword lexer | W | the scan |
| large character sets and `set_contains` | U, and any class of 8 or more ranges | the harness |
| reserved word sets | R, at ABI 15 | the scan |
| the external scanner tables and states | external tokens | the scan |
| the `#pragma` for a lexer of more than 300 states | a large lexer | the harness |
| `ADVANCE_MAP` | a lexer state with 8 or more simple transitions | the harness |
| the small parse table | a grammar with more states than large states | the harness |
| the lexer mode at the end of a non-terminal extra | X | the harness |
| `SKIP`, `SHIFT_EXTRA`, `SHIFT_REPEAT`, `RECOVER` | extras, repeats, and every grammar | the harness |
| `REDUCE` with dynamic precedence | D | the scan |
| identifiers for tokens that are only punctuation | almost every grammar | the harness |
| the ABI 14 forms, such as `TSLexMode` | any grammar generated with `--abi 14` | the harness |
| the grammar version in the metadata | a `tree-sitter.json` with a version | the scan. 123 of the 171 have a `tree-sitter.json`, and the other 48 give no version |
| the limit of 65,535 parse action lists | the largest grammars | the harness |

"The harness" means that only the `parser.c` that the upstream tool writes
can show the path. The golden harness scans each `parser.c` for the code of
each path and reports which grammars reach it (D58). A path that no grammar
reaches is a gap, and a grammar or a test grammar that reaches it joins the
set.

The first run of the harness, on 2026-09-29, covered the 68 test grammars and
the 15 fixture grammars, which hold 17 grammars. It looked for 26 paths, and
each path was reached by at least one output. The rarest were the anonymous
unique alias, reached only by the test grammar
`named_rule_aliased_as_anonymous`, the pragma for a large lexer (bash, c, cpp
and ruby), and the reserved word sets (the test grammar `reserved_words`, and
go, javascript, php and php_only). The python fixture is at `v0.23.6`, which
has no reserved words yet. The rest of the set followed in a later run (D58).

The run over the whole set, on 2026-09-29, made 516 outputs of 228
grammars: the 56 test grammars that the tool accepts, at three variants, and
the 185 grammars of the record, at ABI 14 and ABI 15, less the 22 variants
that the tool rejects. Each of the 26 paths was reached by at least one
output. The rarest were the reserved words, in 11 grammars, among them
fsharp, go, javascript and ocaml, the anonymous unique alias, in 18, and the
end of a non-terminal extra, in 32. The generator of transit writes the same
files as upstream for every one of these outputs.

The harness makes each grammar at ABI 14 and at ABI 15 (D19). It makes the
test grammars also with the merge of states off.

The committed `parser.c` files of the set are at three ABI versions: 94 at
ABI 15, 71 at ABI 14, and one at ABI 8, `tree-sitter/tree-sitter-fluent`. Four
commit no `parser.c`. None is at ABI 13. The harness makes every file again at
the upstream commit that transit ports, so the version of a committed file
does not matter to the golden files.

## The runtime

The set also reaches the parts of the runtime that rline and usql use:

1. External scanners: 98 of the 171 grammars have `src/scanner.c`. Python and
   YAML keep a stack of indentation, Bash and Ruby keep heredocs, and HTML and
   XML keep a stack of tags.
2. Injections, where one grammar parses a range of another: `html` with
   `javascript` and `css`, `markdown` with `markdown_inline`,
   `embedded_template`, `php`, `vue`, `svelte`, `angular`, `heex`, `templ`, `blade`,
   `twig`, `gotmpl`, `helm`, `latex`, and the small grammars that others
   inject: `comment`, `jsdoc`, `regex`, `query`, `doxygen` and `luadoc`.
3. Conflicts that the parser resolves at run time: `cpp`, `typescript`,
   `haskell`, `scala` and the PostgreSQL grammar.
4. Large grammars. The committed `parser.c` of the largest candidates is
   63 MB for `systemverilog`, 53 MB for `fsharp`, 44 MB for `verilog`, 35 MB for
   `tlaplus`, 34 MB for `fortran`, 30 MB for `c_sharp` and 29 MB for `cuda`. The
   committed files of the set add up to 929 MB, so transit stores hashes, not
   files (D40).
5. Queries: 133 of the 171 grammars have `queries/highlights.scm`.

## SQL

usql needs SQL grammars most. These SQL grammars exist on 2026-09-29. The
feature counts come from their `grammar.json`.

| Grammar | Stars | Last push | Notes |
| --- | --- | --- | --- |
| `DerekStride/tree-sitter-sql` | 247 | 2026-09-19 | Generic SQL, MIT. Has a scanner. Commits no `grammar.json` and no `parser.c` on `main`. In tier 3 |
| `gmr/tree-sitter-postgres`, `postgres` | 15 | 2026-09-23 | PostgreSQL, BSD-3-Clause. Its `tree-sitter.json` says that it is "generated from PostgreSQL source", at version `19.0.0-beta.4.1`. 1,255 rules, 702 uses of dynamic precedence, a scanner |
| `gmr/tree-sitter-postgres`, `plpgsql` | 15 | 2026-09-23 | PL/pgSQL, in the same repository. 17 external tokens |
| `m-novikov/tree-sitter-sql` | 126 | 2024-03-06 | PostgreSQL flavor, MIT. Not pushed since 2024 |
| `takegue/tree-sitter-sql-bigquery` | 31 | 2025-03-28 | BigQuery, MIT. Has a scanner and named precedences |
| `kitagry/tree-sitter-bigquery` | 2 | 2025-12-06 | BigQuery, MIT |
| `Crary-Systems/tree-sitter-tsql` | 5 | 2025-05-26 | T-SQL, BSD-2-Clause. Ignores case with the `i` flag |
| `andreasmaierde/tree-sitter-plsql` | 13 | 2023-02-17 | PL/SQL, MIT. Not pushed since 2023 |
| `shotover/tree-sitter-cql` | 3 | 2026-04-01 | Cassandra CQL, Apache-2.0 |
| `dhcmrlchtdj/tree-sitter-sqlite` | 23 | 2023-06-24 | SQLite, MIT. Archived |

No grammar was found for MySQL, MariaDB, Oracle SQL other than PL/SQL,
Snowflake, ClickHouse or DuckDB.

Only `DerekStride/tree-sitter-sql` meets the rule of tier 3. The set also
takes every other SQL grammar in this table that is not archived (D18). Both
grammars of `gmr/tree-sitter-postgres` are among them. They follow the
PostgreSQL source, as dbmeta does. A SQL grammar that does not exist is
written in transit (D42).

## The dialects of dbmeta

usql supports SQL and SQL-like languages, and transit will support the
language of every dialect of dbmeta in time (D21). dbmeta listed its dialects
on 2026-09-29. Its tiers mean this:

1. Tested: CI runs the release on every push.
2. Nightly: CI runs the release once a night.
3. Verified: a person runs the release before a dbmeta release, and CI does
   not.
4. Staged: no dbmeta model reads the product yet, and CI does not run it.
5. Hosted: a service in `hosted/`, with no tier. dbmeta reaches it only when
   a person gives it credentials.

The column "Grammar" names the grammars that were found on GitHub on
2026-09-29. "Generic" means that only `DerekStride/tree-sitter-sql` can parse
it, as generic SQL. "None" means that no grammar was found. The languages of
the staged products come from product knowledge, and dbmeta did not measure
them.

### Dialects that a dbmeta model reads

| Product | Dialect | Language | Grammar |
| --- | --- | --- | --- |
| PostgreSQL | `postgres` | PostgreSQL SQL, PL/pgSQL | `gmr/tree-sitter-postgres` (both), `m-novikov/tree-sitter-sql`, generic |
| CockroachDB | `cockroachdb` | PostgreSQL SQL, a dialect of its own (dbmeta D123) | as PostgreSQL |
| CrateDB | `cratedb` | its own SQL, on the PostgreSQL protocol | generic |
| MySQL | `mysql` | MySQL SQL | generic |
| MariaDB | `mysql`, a flavor | MySQL SQL | generic |
| SQLite | `sqlite3` | SQLite SQL | `dhcmrlchtdj/tree-sitter-sqlite` (archived), generic |
| DuckDB | `duckdb` | DuckDB SQL | generic |
| SQL Server | `sqlserver` | T-SQL | `Crary-Systems/tree-sitter-tsql` |
| Oracle | `oracle` | Oracle SQL, PL/SQL | `andreasmaierde/tree-sitter-plsql` (not pushed since 2023) |
| Cassandra | `cql` | CQL | `shotover/tree-sitter-cql` |
| ScyllaDB | `cql`, a flavor | CQL | as Cassandra |
| ClickHouse | `clickhouse` | ClickHouse SQL | generic |
| Trino | `trino` | Trino SQL | generic |
| Presto | `presto` | Presto SQL | generic |
| Firebird | `firebirdsql` | Firebird SQL | generic |
| SAP HANA | `hdb` | HANA SQL | generic |
| Apache Hive | `hive` | HiveQL | generic |
| Exasol | `exasol` | Exasol SQL | generic |
| Vertica | `vertica` | Vertica SQL | generic |
| Couchbase | `couchbase` | SQL++, which was N1QL | none |

### Staged dialects with no model yet

| Product | Dialect | Language | Grammar |
| --- | --- | --- | --- |
| Chai | `chai` | SQL, embedded | generic |
| csvq | `csvq` | SQL over CSV, embedded | generic |
| QL | `ql` | SQL-like, embedded | generic |
| Databend | `databend` | SQL | generic |
| InfluxDB 3 | `influxdb` | SQL | generic |
| InfluxDB | `influxql` | InfluxQL | none |
| Neo4j | `neo4j` | Cypher | `taekwombo/tree-sitter-cypher` |
| SurrealDB | `surrealdb` | SurrealQL | `surrealdb/surrealql-tree-sitter`, `Ce11an/tree-sitter-surrealql` |

### Staged products with no dialect yet

| Product | Language | Grammar |
| --- | --- | --- |
| Avatica, Drill, H2, Phoenix, Pinot, QuestDB, GizmoSQL | SQL | generic |
| libSQL, rqlite | SQLite SQL | as SQLite |
| TiDB, Vitess | MySQL SQL, as flavors | generic |
| ksqlDB | KSQL | none |
| TDengine | its own SQL | generic |
| YDB | YQL | `udovin/tree-sitter-yql`, with no stars |
| Spanner emulator | GoogleSQL, and a PostgreSQL dialect | `takegue/tree-sitter-sql-bigquery`, `kitagry/tree-sitter-bigquery`, and as PostgreSQL |
| BigQuery emulator | GoogleSQL | as Spanner |
| Druid | Druid SQL, and native queries in JSON | generic, and `tree-sitter-json` |
| ArangoDB | AQL | two repositories with no stars |
| Dgraph | DQL and GraphQL | `bkegley/tree-sitter-graphql` for GraphQL, none for DQL |
| MongoDB | MQL and aggregation pipelines, in JSON | `tree-sitter-json` |
| CouchDB | Mango queries, in JSON | `tree-sitter-json` |
| Cosmos emulator | the NoSQL SQL of Cosmos | generic |
| DynamoDB local, Alternator | the DynamoDB API, and PartiQL | none for PartiQL |
| Elasticsearch | ES\|QL, SQL and the Query DSL in JSON | none for ES\|QL, generic, `tree-sitter-json` |
| OpenSearch | PPL, SQL and the Query DSL in JSON | none for PPL, generic, `tree-sitter-json` |
| Solr | Lucene syntax, streaming expressions and Solr SQL | none, none, generic |
| Fuseki | SPARQL | `GordianDziwis/tree-sitter-sparql` |
| Virtuoso | SPARQL and SQL | as Fuseki, generic |
| TerminusDB | WOQL and GraphQL | none, as Dgraph |
| Weaviate | GraphQL | as Dgraph |
| PostgREST | a query string in HTTP | none |

Qdrant, Chroma, Milvus, Meilisearch and Typesense have search or vector APIs
and no query language. Milvus has filter expressions.

### Hosted services

| Service | Dialect | Language | Grammar |
| --- | --- | --- | --- |
| Athena | `awsathena` | Trino SQL | generic |
| BigQuery | `bigquery` | GoogleSQL | as the BigQuery emulator |
| Cosmos | `cosmos` | the NoSQL SQL of Cosmos | generic |
| Databricks | `databricks` | Spark SQL | generic |
| DynamoDB | `godynamo` | PartiQL | none |
| MaxCompute | `maxcompute` | MaxCompute SQL | generic |
| Snowflake | `snowflake` | Snowflake SQL | generic |
| Spanner | `spanner` | GoogleSQL | as the Spanner emulator |
| Tablestore | `ots` | its own SQL | generic |
| Neon, Redshift | `postgres` | PostgreSQL SQL. Redshift is a flavor | as PostgreSQL |
| PlanetScale | `mysql`, a flavor | MySQL SQL | generic |

dbmeta plans no other product. Db2 is the one database that usql supports and
dbmeta does not list. Whether Db2 is in scope is open in dbmeta.

### The grammars for dbmeta's languages

These grammars join the set now, although they have fewer stars than tier 3
asks for (D23). Each commits `src/grammar.json`. The features use the letters
of "Features" above.

| Grammar | Language | Stars | Last push | License | Scanner | Rules | Features |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `taekwombo/tree-sitter-cypher` | Cypher | 13 | 2026-08-14 | Unlicense | no | 115 | XFU |
| `surrealdb/surrealql-tree-sitter` | SurrealQL | 7 | 2026-09-13 | Apache-2.0 | yes | 610 | NPDW |
| `GordianDziwis/tree-sitter-sparql` | SPARQL | 16 | 2025-10-15 | MIT | no | 141 | FSW |
| `bkegley/tree-sitter-graphql` | GraphQL | 32 | 2024-06-07 | MIT | no | 74 | none |
| `udovin/tree-sitter-yql` | YQL | 0 | 2026-03-18 | none (D20) | no | 142 | FSW |

### What is missing

These languages of dbmeta have no grammar that was found: SQL++, InfluxQL,
KSQL, PartiQL, ES|QL, PPL, DQL, WOQL, the Lucene syntax and the query string
of PostgREST. Most of the SQL dialects have only the generic grammar, and
MySQL, the most used of them, has no grammar of its own. transit writes these
grammars, in `grammars/` (D42, D104).

## Not in the set yet

Some grammars that editors use miss the rule of their tier by a small margin,
for example `astro` (last push 2025-04-23). Add such a grammar by name if it
reaches a path that the harness finds as a gap.

## The candidates

The columns come from GitHub on 2026-09-29. "`parser.c` MB" is the size of the
file that the grammar commits. The grammar made that file with its own version
of the tool, so the size is an estimate. "Features" uses the letters of the
table above.

[N] marks a grammar whose queries use a predicate or a directive of Neovim in
a query file that transit reads. transit does not support them now (D69), so
some of its patterns match more often than in Neovim.
[`NEOVIM.md`](NEOVIM.md) lists the names that each grammar uses.

| Tier | Grammar | Repository | Stars | Last push | Scanner | `parser.c` MB | License | Features |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | python | tree-sitter/tree-sitter-python | 569 | 2026-09-13 | yes | 3.3 | MIT | REUDSW |
| 1 | tsx | tree-sitter/tree-sitter-typescript `tsx` | 533 | 2026-09-17 | yes | 8.4 | MIT | NPEUDSW |
| 1 | typescript | tree-sitter/tree-sitter-typescript `typescript` | 533 | 2026-09-17 | yes | 8.3 | MIT | NPEUDSW |
| 1 | rust | tree-sitter/tree-sitter-rust | 532 | 2026-09-16 | yes | 6.2 | MIT | XUSW |
| 1 | javascript [N](NEOVIM.md) | tree-sitter/tree-sitter-javascript | 494 | 2026-09-17 | yes | 2.7 | MIT | RNPEUDSW |
| 1 | cpp | tree-sitter/tree-sitter-cpp | 457 | 2026-09-17 | yes | 24.7 | MIT | PUDSW |
| 1 | go | tree-sitter/tree-sitter-go | 419 | 2026-09-13 | no | 1.5 | MIT | RUDSW |
| 1 | c | tree-sitter/tree-sitter-c | 395 | 2026-09-16 | no | 3.7 | MIT | UDSW |
| 1 | bash | tree-sitter/tree-sitter-bash | 330 | 2026-09-13 | yes | 9.5 | MIT | EDSW |
| 1 | c_sharp | tree-sitter/tree-sitter-c-sharp | 321 | 2026-09-13 | yes | 30.5 | MIT | PXUDSW |
| 1 | java | tree-sitter/tree-sitter-java | 274 | 2026-09-13 | no | 2.4 | MIT | UDSW |
| 1 | php | tree-sitter/tree-sitter-php `php` | 235 | 2026-09-24 | yes | 6.9 | MIT | RXFDSW |
| 1 | php_only | tree-sitter/tree-sitter-php `php_only` | 235 | 2026-09-24 | yes | 6.6 | MIT | RFDSW |
| 1 | ruby | tree-sitter/tree-sitter-ruby | 229 | 2026-09-13 | yes | 14.6 | MIT | EXSW |
| 1 | html | tree-sitter/tree-sitter-html | 215 | 2026-09-13 | yes | 0.1 | MIT | E |
| 1 | json | tree-sitter/tree-sitter-json | 204 | 2026-09-13 | no | 0.0 | MIT | S |
| 1 | scala | tree-sitter/tree-sitter-scala | 199 | 2026-09-13 | yes | 26.0 | MIT | RNPEXFUDSW |
| 1 | haskell | tree-sitter/tree-sitter-haskell | 188 | 2025-08-29 | yes | 18.9 | MIT | NPUDSW |
| 1 | css | tree-sitter/tree-sitter-css | 139 | 2026-09-13 | yes | 0.5 | MIT |  |
| 1 | julia [N](NEOVIM.md) | tree-sitter/tree-sitter-julia | 129 | 2025-11-08 | yes | 23.0 | MIT | XUDSW |
| 1 | verilog | tree-sitter/tree-sitter-verilog | 123 | 2026-09-13 | no | 44.2 | MIT | W |
| 1 | regex | tree-sitter/tree-sitter-regex | 105 | 2025-09-13 | no | 0.1 | MIT |  |
| 1 | ocaml | tree-sitter/tree-sitter-ocaml `grammars/ocaml` | 102 | 2026-09-19 | yes | 23.2 | MIT | RNPEXUDSW |
| 1 | ocaml_interface | tree-sitter/tree-sitter-ocaml `grammars/interface` | 102 | 2026-09-19 | yes | 19.9 | MIT | RNPEXUDSW |
| 1 | embedded_template | tree-sitter/tree-sitter-embedded-template | 82 | 2025-08-31 | no | 0.0 | MIT |  |
| 1 | jsdoc | tree-sitter/tree-sitter-jsdoc | 51 | 2025-09-13 | yes | 0.1 | MIT | ES |
| 1 | agda | tree-sitter/tree-sitter-agda | 48 | 2026-09-13 | yes | 15.4 | MIT | W |
| 1 | ql | tree-sitter/tree-sitter-ql | 37 | 2026-07-11 | no | 2.3 | MIT | DW |
| 1 | fluent | tree-sitter/tree-sitter-fluent | 7 | 2024-07-18 | yes | 0.1 | MIT |  |
| 1 | ql-dbscheme | tree-sitter/tree-sitter-ql-dbscheme | 5 | 2024-11-11 | no | 0.1 | MIT | W |
| 2 | markdown | tree-sitter-grammars/tree-sitter-markdown `tree-sitter-markdown` | 626 | 2026-09-13 | yes | 2.0 | MIT | PD |
| 2 | markdown_inline | tree-sitter-grammars/tree-sitter-markdown `tree-sitter-markdown-inline` | 626 | 2026-09-13 | yes | 2.2 | MIT | PD |
| 2 | hyprlang | tree-sitter-grammars/tree-sitter-hyprlang | 161 | 2026-09-13 | no | 0.1 | MIT | XW |
| 2 | hcl | tree-sitter-grammars/tree-sitter-hcl | 146 | 2026-09-17 | yes | 0.6 | Apache-2.0 | FU |
| 2 | terraform | tree-sitter-grammars/tree-sitter-hcl `dialects/terraform` | 146 | 2026-09-17 | yes | 0.6 | Apache-2.0 | FU |
| 2 | lua | tree-sitter-grammars/tree-sitter-lua | 104 | 2026-06-19 | yes | 0.3 | MIT | XFUSW |
| 2 | query [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-query | 80 | 2026-09-13 | no | 0.1 | Apache-2.0 | S |
| 2 | diff [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-diff | 70 | 2026-09-13 | no | 0.1 | MIT |  |
| 2 | commonlisp | tree-sitter-grammars/tree-sitter-commonlisp | 63 | 2026-09-13 | no | 5.6 | MIT |  |
| 2 | yaml | tree-sitter-grammars/tree-sitter-yaml | 58 | 2026-05-22 | yes | 1.2 | MIT |  |
| 2 | dtd | tree-sitter-grammars/tree-sitter-xml `dtd` | 52 | 2026-09-13 | yes | 0.2 | MIT | SW |
| 2 | xml | tree-sitter-grammars/tree-sitter-xml `xml` | 52 | 2026-09-13 | yes | 0.2 | MIT | ESW |
| 2 | glsl [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-glsl | 50 | 2026-09-13 | no | 5.3 | MIT | UDSW |
| 2 | kdl | tree-sitter-grammars/tree-sitter-kdl | 48 | 2026-09-13 | yes | 0.5 | MIT | W |
| 2 | zig [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-zig | 42 | 2026-09-13 | no | 5.5 | MIT | PDSW |
| 2 | objc [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-objc | 41 | 2026-09-13 | no | 26.9 | MIT | FUDSW |
| 2 | odin [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-odin | 39 | 2026-09-13 | yes | 14.1 | MIT | EFUSW |
| 2 | vim | tree-sitter-grammars/tree-sitter-vim | 39 | 2026-09-13 | yes | 4.6 | MIT | DW |
| 2 | cuda | tree-sitter-grammars/tree-sitter-cuda | 36 | 2026-09-15 | yes | 29.7 | MIT | PUDSW |
| 2 | tcl | tree-sitter-grammars/tree-sitter-tcl | 26 | 2026-09-13 | yes | 0.4 | MIT | W |
| 2 | vue [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-vue | 26 | 2026-09-13 | yes | 0.1 | MIT | E |
| 2 | starlark | tree-sitter-grammars/tree-sitter-starlark | 24 | 2026-09-13 | yes | 2.4 | MIT | EUDSW |
| 2 | svelte | tree-sitter-grammars/tree-sitter-svelte | 23 | 2026-09-13 | yes | 0.2 | MIT | ES |
| 2 | luadoc | tree-sitter-grammars/tree-sitter-luadoc | 21 | 2026-09-13 | no | 0.6 | MIT | S |
| 2 | gitattributes | tree-sitter-grammars/tree-sitter-gitattributes | 20 | 2026-09-13 | no | 0.1 | MIT | W |
| 2 | doxygen | tree-sitter-grammars/tree-sitter-doxygen | 17 | 2026-09-13 | yes | 0.3 | MIT |  |
| 2 | make | tree-sitter-grammars/tree-sitter-make | 17 | 2026-09-13 | no | 0.9 | MIT | W |
| 2 | toml | tree-sitter-grammars/tree-sitter-toml | 17 | 2025-07-10 | yes | 0.1 | MIT |  |
| 2 | bitbake [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-bitbake | 14 | 2026-09-13 | yes | 3.2 | MIT | EUDS |
| 2 | hlsl | tree-sitter-grammars/tree-sitter-hlsl | 14 | 2026-09-13 | yes | 19.9 | MIT | PUDSW |
| 2 | hare [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-hare | 13 | 2026-09-13 | no | 0.9 | MIT | SW |
| 2 | ssh_config | tree-sitter-grammars/tree-sitter-ssh-config | 13 | 2026-09-13 | no | 1.1 | MIT | F |
| 2 | csv | tree-sitter-grammars/tree-sitter-csv `csv` | 12 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | psv | tree-sitter-grammars/tree-sitter-csv `psv` | 12 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | tsv | tree-sitter-grammars/tree-sitter-csv `tsv` | 12 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | wgsl_bevy | tree-sitter-grammars/tree-sitter-wgsl-bevy | 12 | 2026-09-13 | yes | 0.5 | MIT | W |
| 2 | linkerscript [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-linkerscript | 11 | 2026-09-13 | no | 0.5 | MIT | FSW |
| 2 | test | tree-sitter-grammars/tree-sitter-test | 11 | 2025-08-29 | yes | 0.0 | MIT |  |
| 2 | bicep | tree-sitter-grammars/tree-sitter-bicep | 10 | 2026-09-13 | yes | 1.1 | MIT | PSW |
| 2 | gosum | tree-sitter-grammars/tree-sitter-go-sum | 10 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | meson | tree-sitter-grammars/tree-sitter-meson | 10 | 2026-09-23 | no | 0.7 | MIT |  |
| 2 | requirements | tree-sitter-grammars/tree-sitter-requirements | 10 | 2026-09-13 | no | 0.2 | MIT | XW |
| 2 | luap | tree-sitter-grammars/tree-sitter-luap | 9 | 2026-09-13 | no | 0.1 | MIT | D |
| 2 | scss | tree-sitter-grammars/tree-sitter-scss | 9 | 2026-09-13 | yes | 0.7 | MIT |  |
| 2 | smali [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-smali | 9 | 2026-09-13 | yes | 1.6 | MIT | SW |
| 2 | kotlin [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-kotlin | 8 | 2026-09-13 | yes | 21.4 | MIT | PEFUDSW |
| 2 | pony [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-pony | 8 | 2026-09-13 | yes | 4.5 | MIT | SW |
| 2 | printf | tree-sitter-grammars/tree-sitter-printf | 8 | 2026-09-13 | no | 0.0 | ISC |  |
| 2 | arduino | tree-sitter-grammars/tree-sitter-arduino | 7 | 2026-09-13 | yes | 17.0 | MIT | PUDSW |
| 2 | gn | tree-sitter-grammars/tree-sitter-gn | 7 | 2026-09-13 | yes | 0.2 | MIT | SW |
| 2 | luau [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-luau | 7 | 2026-09-13 | yes | 0.6 | MIT | XFUSW |
| 2 | properties | tree-sitter-grammars/tree-sitter-properties | 7 | 2026-09-13 | yes | 0.1 | MIT |  |
| 2 | ron | tree-sitter-grammars/tree-sitter-ron | 7 | 2026-09-13 | yes | 0.1 | Apache-2.0 | U |
| 2 | slang | tree-sitter-grammars/tree-sitter-slang | 7 | 2026-09-13 | yes | 25.7 | MIT | PUDSW |
| 2 | thrift [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-thrift | 7 | 2026-09-13 | no | 0.7 | MIT | SW |
| 2 | func | tree-sitter-grammars/tree-sitter-func | 6 | 2026-09-13 | no | 0.5 | MIT | W |
| 2 | kconfig [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-kconfig | 6 | 2026-09-13 | yes | 0.3 | MIT | DSW |
| 2 | po | tree-sitter-grammars/tree-sitter-po | 6 | 2026-09-13 | no | 0.1 | MIT |  |
| 2 | udev | tree-sitter-grammars/tree-sitter-udev | 6 | 2026-09-13 | no | 0.2 | MIT |  |
| 2 | yuck | tree-sitter-grammars/tree-sitter-yuck | 6 | 2026-09-13 | yes | 0.1 | MIT | SW |
| 2 | capnp | tree-sitter-grammars/tree-sitter-capnp | 5 | 2026-09-13 | no | 0.4 | MIT | SW |
| 2 | ungrammar | tree-sitter-grammars/tree-sitter-ungrammar | 5 | 2026-09-13 | no | 0.0 | MIT | W |
| 2 | gpg | tree-sitter-grammars/tree-sitter-gpg-config | 4 | 2026-09-13 | no | 0.7 | MIT | F |
| 2 | poe_filter | tree-sitter-grammars/tree-sitter-poe-filter | 4 | 2026-09-13 | no | 0.3 | MIT | FU |
| 2 | squirrel [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-squirrel | 4 | 2026-09-13 | yes | 3.0 | MIT | SW |
| 2 | tablegen [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-tablegen | 4 | 2026-09-13 | yes | 0.4 | MIT | SW |
| 2 | cairo [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-cairo | 3 | 2026-09-13 | yes | 2.1 | MIT | ESW |
| 2 | cst | tree-sitter-grammars/tree-sitter-cst | 3 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | ispc [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-ispc | 3 | 2026-09-13 | no | 7.5 | MIT | UDSW |
| 2 | move | tree-sitter-grammars/tree-sitter-move | 3 | 2026-09-13 | no | 0.4 | none | W |
| 2 | pem | tree-sitter-grammars/tree-sitter-pem | 3 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | qmldir | tree-sitter-grammars/tree-sitter-qmldir | 3 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | xcompose | tree-sitter-grammars/tree-sitter-xcompose | 3 | 2026-09-13 | no | 0.0 | MIT |  |
| 2 | nqc | tree-sitter-grammars/tree-sitter-nqc | 2 | 2026-09-13 | no | 4.2 | MIT | UDSW |
| 2 | pymanifest | tree-sitter-grammars/tree-sitter-pymanifest | 2 | 2026-09-13 | no | 0.1 | MIT |  |
| 2 | readline | tree-sitter-grammars/tree-sitter-readline | 2 | 2026-09-13 | no | 0.4 | MIT | F |
| 2 | uxntal [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-uxntal | 2 | 2026-09-13 | yes | 0.3 | MIT | W |
| 2 | chatito | tree-sitter-grammars/tree-sitter-chatito | 1 | 2026-09-13 | no | 0.1 | MIT | S |
| 2 | cpon | tree-sitter-grammars/tree-sitter-cpon | 1 | 2026-09-13 | no | 0.1 | MIT |  |
| 2 | firrtl [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-firrtl | 1 | 2026-09-13 | yes | 0.5 | Apache-2.0 | SW |
| 2 | gstlaunch | tree-sitter-grammars/tree-sitter-gstlaunch | 1 | 2026-09-13 | no | 0.1 | MIT | U |
| 2 | re2c [N](NEOVIM.md) | tree-sitter-grammars/tree-sitter-re2c | 1 | 2026-09-13 | no | 0.4 | MIT | XDSW |
| 2 | cyberchef | tree-sitter-grammars/tree-sitter-cyberchef | 0 | 2026-09-13 | no | 0.0 | MIT |  |
| 3 | superhtml | kristoff-it/superhtml `tree-sitter-superhtml` | 1384 | 2026-09-26 | yes | 0.1 | MIT | E |
| 3 | elixir | elixir-lang/tree-sitter-elixir | 286 | 2026-07-20 | yes | 12.3 | Apache-2.0 | FUD |
| 3 | blade [N](NEOVIM.md) | EmranMR/tree-sitter-blade | 263 | 2026-08-31 | yes | 22.0 | MIT | E |
| 3 | sql | DerekStride/tree-sitter-sql | 247 | 2026-09-19 | yes | none | MIT | not measured |
| 3 | nix | nix-community/tree-sitter-nix | 240 | 2026-09-28 | yes | 0.6 | MIT | SW |
| 3 | swift | alex-pinkus/tree-sitter-swift | 224 | 2026-09-28 | yes | none | MIT | UD |
| 3 | v | vlang/v-analyzer `tree_sitter_v` | 204 | 2026-06-20 | no | 11.8 | MIT | XDSW |
| 3 | just | casey/tree-sitter-just | 195 | 2026-03-25 | yes | 0.3 | Apache-2.0 | W |
| 3 | kotlin [N](NEOVIM.md) | fwcd/tree-sitter-kotlin | 190 | 2026-09-09 | yes | 32.2 | MIT | UDW |
| 3 | clojure | sogaiu/tree-sitter-clojure | 189 | 2025-08-26 | no | 0.8 | CC0-1.0 |  |
| 3 | solidity | JoranHonig/tree-sitter-solidity | 186 | 2026-02-11 | no | 2.4 | MIT | DW |
| 3 | nu | nushell/tree-sitter-nu | 180 | 2026-09-14 | yes | 8.3 | MIT | XFUDW |
| 3 | latex | latex-lsp/tree-sitter-latex | 173 | 2026-08-01 | yes | none | MIT | W |
| 3 | comment | stsewd/tree-sitter-comment | 169 | 2025-12-16 | yes | 0.0 | MIT |  |
| 3 | r | r-lib/tree-sitter-r | 155 | 2026-06-22 | yes | 3.8 | MIT | UW |
| 3 | vimdoc | neovim/tree-sitter-vimdoc | 154 | 2026-06-22 | no | 0.5 | Apache-2.0 | D |
| 3 | gotmpl | ngalaiko/tree-sitter-go-template | 140 | 2026-03-21 | no | 0.3 | MIT | UD |
| 3 | helm | ngalaiko/tree-sitter-go-template `dialects/helm` | 140 | 2026-03-21 | no | 0.3 | MIT | UD |
| 3 | vhs | charmbracelet/tree-sitter-vhs | 120 | 2026-08-12 | no | 0.1 | MIT |  |
| 3 | dart | UserNobody14/tree-sitter-dart | 107 | 2026-07-07 | yes | 6.8 | MIT | XDSW |
| 3 | dockerfile | camdencheek/tree-sitter-dockerfile | 104 | 2025-08-06 | yes | 0.2 | MIT |  |
| 3 | gleam | gleam-lang/tree-sitter-gleam | 104 | 2026-09-18 | yes | 2.9 | Apache-2.0 | X |
| 3 | templ | vrischmann/tree-sitter-templ | 100 | 2026-09-10 | yes | 2.8 | MIT | RUDSW |
| 3 | fsharp [N](NEOVIM.md) | ionide/tree-sitter-fsharp `fsharp` | 98 | 2026-09-14 | yes | 53.3 | MIT | REXUSW |
| 3 | erlang | WhatsApp/tree-sitter-erlang | 97 | 2026-07-31 | yes | 2.1 | Apache-2.0 | DSW |
| 3 | apex | aheber/tree-sitter-sfapex `apex` | 92 | 2026-08-19 | no | 7.2 | MIT | USW |
| 3 | sflog | aheber/tree-sitter-sfapex `sflog` | 92 | 2026-08-19 | no | 0.0 | MIT |  |
| 3 | soql | aheber/tree-sitter-sfapex `soql` | 92 | 2026-08-19 | no | 0.7 | MIT | X |
| 3 | sosl | aheber/tree-sitter-sfapex `sosl` | 92 | 2026-08-19 | no | 0.7 | MIT | X |
| 3 | gitcommit | gbprod/tree-sitter-gitcommit | 88 | 2026-09-08 | yes | 3.1 | MIT |  |
| 3 | elm | elm-tooling/tree-sitter-elm | 87 | 2026-09-03 | yes | 1.1 | MIT | XUDW |
| 3 | powershell | airbus-cert/tree-sitter-powershell | 86 | 2026-07-10 | yes | 4.5 | MIT |  |
| 3 | pascal | Isopod/tree-sitter-pascal | 80 | 2025-12-23 | no | 3.4 | MIT | FW |
| 3 | tlaplus | tlaplus-community/tree-sitter-tlaplus | 78 | 2026-02-17 | yes | 35.3 | MIT | NPEXDSW |
| 3 | heex [N](NEOVIM.md) | phoenixframework/tree-sitter-heex | 75 | 2026-03-23 | no | 0.1 | MIT |  |
| 3 | d | gdamore/tree-sitter-d | 67 | 2026-06-19 | yes | 21.9 | MIT | NPUDW |
| 3 | angular [N](NEOVIM.md) | dlvandenberg/tree-sitter-angular | 64 | 2026-05-15 | yes | 0.7 | MIT | EF |
| 3 | c3 | c3lang/tree-sitter-c3 | 64 | 2026-09-16 | yes | 5.5 | MIT | XSW |
| 3 | gomod | camdencheek/tree-sitter-go-mod | 64 | 2025-10-23 | no | 0.1 | MIT | X |
| 3 | http [N](NEOVIM.md) | rest-nvim/tree-sitter-http | 63 | 2025-09-24 | no | 0.4 | MIT | FUD |
| 3 | perl [N](NEOVIM.md) | tree-sitter-perl/tree-sitter-perl | 62 | 2026-09-07 | yes | none | MIT | XFUDSW |
| 3 | rescript | rescript-lang/tree-sitter-rescript | 60 | 2026-09-25 | yes | 7.1 | MIT | RNPEXDSW |
| 3 | ledger | cbarrete/tree-sitter-ledger | 59 | 2026-09-25 | no | 0.5 | MIT | U |
| 3 | supercollider | madskjeldgaard/tree-sitter-supercollider | 59 | 2026-03-24 | yes | 1.2 | none | XW |
| 3 | systemverilog | gmlarumbe/tree-sitter-systemverilog | 59 | 2026-09-15 | no | 63.2 | MIT | RNPDW |
| 3 | cmake [N](NEOVIM.md) | uyha/tree-sitter-cmake | 58 | 2026-09-13 | yes | 0.5 | MIT |  |
| 3 | rst | stsewd/tree-sitter-rst | 57 | 2026-09-15 | yes | 0.3 | MIT | S |
| 3 | asm | RubixDev/tree-sitter-asm | 56 | 2025-11-08 | no | 0.1 | MIT | X |
| 3 | beancount | polarmutex/tree-sitter-beancount | 56 | 2026-09-13 | yes | 0.5 | MIT | USW |
| 3 | zsh | georgeharker/tree-sitter-zsh | 56 | 2026-07-19 | yes | 33.6 | MIT | EDSW |
| 3 | fortran | stadelmanma/tree-sitter-fortran | 53 | 2026-09-14 | yes | 34.9 | MIT | EDS |
| 3 | fish | ram02z/tree-sitter-fish | 51 | 2026-08-24 | yes | 0.5 | Unlicense | W |
| 3 | pkl | apple/tree-sitter-pkl | 51 | 2026-09-25 | yes | 1.7 | Apache-2.0 | RXW |
| 3 | scheme | 6cdh/tree-sitter-scheme | 51 | 2026-09-26 | no | 0.3 | MIT | U |
| 3 | teal | euclidianAce/tree-sitter-teal | 48 | 2026-08-13 | yes | 0.8 | MIT | FDW |
| 3 | matlab | acristoffers/tree-sitter-matlab | 44 | 2026-09-02 | yes | 3.1 | MIT | D |
| 3 | twig | gbprod/tree-sitter-twig | 44 | 2026-09-08 | no | 0.6 | MIT |  |
| 3 | devicetree | joelspadin/tree-sitter-devicetree | 43 | 2026-05-02 | no | 1.0 | MIT |  |
