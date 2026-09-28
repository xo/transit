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

transit is in phase 1 of its plan (D54). It holds no parser yet.
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
| [docs/BACKLOG.md](docs/BACKLOG.md) | the work that is known and not done |
| [docs/decisions/](docs/decisions/README.md) | every decision, one file each, with an index |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to change transit |
| [AGENTS.md](AGENTS.md) | the rules for a coding agent, which also hold for a person |

## Grammars

transit has no grammars yet. [docs/GRAMMAR.md](docs/GRAMMAR.md) says how a
grammar is added, and this section will list each one with its upstream
repository and tag.

## Differences from upstream

transit has no deliberate difference from upstream tree-sitter.
[docs/UPSTREAM.md](docs/UPSTREAM.md) says when one can be made. This section
will list each one with its decision.

## License

transit is under the MIT license. See [LICENSE](LICENSE). Upstream
tree-sitter is under the MIT license too, and `LICENSE` keeps its copyright
line. Each grammar keeps the license of its own repository.

[ts]: https://github.com/tree-sitter/tree-sitter
[rline]: https://github.com/xo/rline
[usql]: https://github.com/xo/usql
