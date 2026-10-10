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
ended phase 5. [docs/PLAN.md](docs/PLAN.md) holds the plan, and the decisions
record every answer that shapes it.

## Sample programs

The module `github.com/xo/transit/_example` holds two programs that use
transit as rline and usql will (D53). Run them in the folder `_example`:

| Program | What it does |
| --- | --- |
| `go run ./highlight -dialect mysql -style monokai -colors 256` | It types a text of usql one key at a time, parses again with the old tree after each key, and finds the layer of each SQL statement with `inject`. It prints the text in the colors of a style of the module `styles`, and the time of each edit |
| `go run ./complete -dialect postgres -text 'select * from '` | It completes at each cursor that it gets as a byte offset. It says whether the cursor is in a meta command, in a variable or in a SQL statement, and in a statement it lists the keywords and the kinds of node that can come next, from `Layer.StatesAt` |

The flag `-dialect` takes `cql`, `generic`, `mysql`, `oracle`, `postgres` or
`sqlserver`.

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
| `github.com/xo/transit/grammars/sql` | `sql` | [DerekStride/tree-sitter-sql](https://github.com/DerekStride/tree-sitter-sql) | `v0.3.11` |
| `github.com/xo/transit/grammars/postgres/postgres` | `postgres` | [gmr/tree-sitter-postgres](https://github.com/gmr/tree-sitter-postgres) | `v1.2.4` |
| `github.com/xo/transit/grammars/postgres/plpgsql` | `plpgsql` | [gmr/tree-sitter-postgres](https://github.com/gmr/tree-sitter-postgres) | `v1.2.4` |
| `github.com/xo/transit/grammars/sqlserver` | `TSQL` | [Crary-Systems/tree-sitter-tsql](https://github.com/Crary-Systems/tree-sitter-tsql) | `0.0.1` |
| `github.com/xo/transit/grammars/oracle` | `plsql` | [andreasmaierde/tree-sitter-plsql](https://github.com/andreasmaierde/tree-sitter-plsql) | the branch `main` at `28aebef` |
| `github.com/xo/transit/grammars/cql` | `cql` | [shotover/tree-sitter-cql](https://github.com/shotover/tree-sitter-cql) | `v0.2.0` |

The repositories of `sqlserver`, `oracle` and `cql` have no highlight query,
so xo wrote `queries/highlights.scm` of these three packages (D113).

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
gives the language with dollar quotes and block comments, the options of
PostgreSQL. `LanguageFor` gives the language with the options of another
dialect, with the same tables:

```go
lang := usql.LanguageFor(usql.Options{BlockComments: true, HashComments: true, Backticks: true})
```

The fields of `usql.Options` are the flags of the type `Syntax` of dbmeta:
`DollarQuotes`, `BlockComments`, `SlashComments`, `HashComments` and
`Backticks`. Two more fields are not flags of dbmeta. `BeginEndBlocks` is
described below. `Batches` keeps a batch of CQL, from `BEGIN BATCH` to
`APPLY BATCH`, in one statement. It is off by default.

The package `github.com/xo/transit/grammars/mysql` holds the grammar
`mysql`, which xo writes from the reference manual of MySQL 8.4 (D42, D106).
It covers the SQL that MySQL 8.4 and MariaDB share, with backticks, `#` and
`-- ` comments, version comments such as `/*!40101 ... */`, the variables
`@name` and `@@session.name`, and stored programs with `BEGIN ... END`
bodies. It does not read the `DELIMITER` command of the mysql client. The
usql grammar ends a statement at each `;`, so by default it splits such a
body into several statements, and the MySQL grammar finds an error in each
part. The option `BeginEndBlocks` of `usql.Options` keeps the body in one
statement (D112). It is off by default.

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
