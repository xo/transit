# D55. transit adds the #lua-match? predicate

Status: Decided, amended by D69.

Ken decided on 2026-09-29, answering question 53, that transit adds the query
predicate `#lua-match?` now, as an API that upstream does not have (D28).

`#lua-match?` and `#not-lua-match?` hold when the text of a capture matches a
Lua pattern, as Neovim evaluates them. `#match?` and its relatives stay as the
Rust binding evaluates them, with regular expressions (D27).

The working C example found the need. The highlight query of
`DerekStride/tree-sitter-sql` uses `#match?` with the Lua patterns
`^[-+]?%d+$` and `^[-+]?%d*\.%d*$`, which never match as regular expressions.
The queries of Neovim use `#lua-match?` for such patterns, and a consumer that
takes a query from Neovim needs it. The query of that grammar is embedded as
it is (D31), so its `#match?` patterns still do not match.

The matcher follows the pattern rules of Lua 5.1, which Neovim uses through
LuaJIT. It lives in files that port no upstream file (D28), and its tests
compare it with the examples of the Lua manual. Whether the code is written
fresh or ported from Lua, whose license is MIT, is decided when it is written.

D69 defers this predicate, with the rest of the Neovim dialect, until the
tier 1 grammars need it. [`NEOVIM.md`](../NEOVIM.md) lists the grammars whose
queries use the dialect.
