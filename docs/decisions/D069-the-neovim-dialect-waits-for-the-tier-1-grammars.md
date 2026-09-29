# D69. The Neovim dialect of queries waits for the tier 1 grammars

Status: Decided, amends D55.

Ken decided on 2026-09-30 that transit does not build `#lua-match?` and
`#not-lua-match?` now, and that support for the rest of the Neovim dialect of
queries is an excursion that is not worth doing now. D55 added `#lua-match?`
as an API that upstream does not have (D28). It waits, with the other
predicates and directives of Neovim, until the work on the tier 1 grammars
needs it. [`NEOVIM.md`](../NEOVIM.md) lists the names of the dialect, the
grammars whose queries use them, and what support would take.

Upstream has no predicate of Neovim. The Rust binding keeps a predicate or a
directive that it does not know in `general_predicates` and does not evaluate
it, so the pattern matches whatever the text is. Until support comes, transit
does the same, as a port of the Rust binding (D27).

These facts were measured on 2026-09-30, before the decision:

1. Of the 185 grammars of the set, 25 have queries that use `#lua-match?` or
   `#not-lua-match?`: 86 uses in 31 files. 23 of the 25 use them in a query
   file that transit reads. All 25 are in tier 2 or tier 3.
2. 37 grammars use a predicate or a directive of Neovim, and 32 of them use
   one in a query file that transit reads. Two of the 32 are in tier 1, and
   each uses one name once: julia uses `#has-ancestor?` in a highlight
   pattern, and javascript uses `#offset!` in an injection.
3. `#lua-match?` does not fix the query of `DerekStride/tree-sitter-sql` that
   raised D55. That query writes its Lua patterns in `#match?`, and D55 keeps
   the query as it is.
4. Neovim evaluates `#lua-match?` with `string.find` of LuaJIT. When the
   predicate is built, the matcher of LuaJIT 2.1, in `src/lib_string.c`, is
   the one to port. It is the matcher of Lua 5.1 with the class `%g` and a
   limit of 200 levels of recursion. Its license is MIT, and it holds the
   notices of Mike Pall and of Lua.org.

The questions that D55 left open, where the code comes from and its tests,
are decided when the predicate is built.
