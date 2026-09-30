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
2026-09-29 the gate of D9 holds: 151 grammars count, which ends phase 2.
Phase 1 ends when Ken accepts the target API in [docs/API.md](docs/API.md)
(D54). Phase 3 has ported the runtime: the parser, the tree, the node, the
tree cursor, and the query engine with the predicates of the Rust binding. The
Go runtime gives the same trees and the same query matches as the C runtime
for every corpus input of every fixture grammar. It parses with the tables and
the lexers of C grammars, through the test module. The ported runtime tests
of upstream pass. `StatesAt` gives the parse states at a cursor (D57, D70). The package
`inject` finds the injections of a text and parses their layers, as upstream
does (D72). The measurements of a prototype of the Go output are recorded in
D73, and they end phase 3. In phase 4, the Go backend writes a grammar
package, and `json` is the first one. Its tables, its lexer and its trees are
the ones of the C grammar.
[docs/PLAN.md](docs/PLAN.md) holds the plan, and the decisions record every
answer that shapes it.

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
(D28), such as `StatesAt` (D57, D70). It does not evaluate the predicates of
Neovim, and upstream does not either ([docs/NEOVIM.md](docs/NEOVIM.md)).
[docs/UPSTREAM.md](docs/UPSTREAM.md) says when one can be made. This section
will list each one with its decision.

## License

transit is under the MIT license. See [LICENSE](LICENSE). Upstream
tree-sitter is under the MIT license too, and `LICENSE` keeps its copyright
line. Each grammar keeps the license of its own repository.

[ts]: https://github.com/tree-sitter/tree-sitter
[rline]: https://github.com/xo/rline
[usql]: https://github.com/xo/usql
