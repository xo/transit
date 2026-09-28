# What rline gets from transit

This document is for a coding agent that works in
[rline](https://github.com/xo/rline). It says what transit will give rline
for syntax highlighting, what is decided, and what is still open.

On 2026-09-29 transit holds no parser yet. Nothing here is an API yet. The target
API comes from a working C example in phase 1 of [`PLAN.md`](PLAN.md) (D10).
When it exists, this document will name each identifier that rline uses.

## The direction

rline takes its design from transit, and transit does not take a design from
rline (D6). rline will change its highlighting to follow transit. transit
does not import rline. If rline needs something that transit does not give,
ask Ken. Do not add a workaround in rline, and do not change transit from
rline.

## What is decided

1. rline imports the transit runtime, `github.com/xo/transit` (D11).
2. The root module of transit imports only the Go standard library, so rline
   gets no third party dependency from it (D11, D15).
3. rline imports no grammar package. Its caller, such as usql, gives it a
   `*transit.Language`, and rline highlights any language with it (D11).
4. transit gives parsing information only. rline decides what to draw and how
   (D6).
5. rline uses chroma, if at all, only for its styles. The tokens and their
   kinds come from transit, and never from a chroma lexer (D14).

rline has its own rule on its dependencies, rline D7. Adding transit to it is
a decision of rline. Ken makes it in rline.

## How highlighting will work

This is the plan, not an API. The names can change in phase 1.

1. rline hands the whole statement to its highlighter, across every row.
   rline's `docs/USQL.md` says this for `WithContinue`, and it says that
   nobody has built and tested it yet. The highlighter parses the whole
   statement, and not one row.
2. The highlighter keeps the tree from the last key. After each key, it tells
   the tree what changed, with the transit form of `ts_tree_edit`, and parses
   again with the old tree. transit then reuses the parts that did not change.
3. The highlighter runs the highlight query of the language on the new tree,
   limited to the rows that are visible.
4. Each capture has a name, such as `keyword` or `string.special`, and a byte
   range. rline marks the byte range with a style for that name through
   `LineStyle`.
5. transit counts every position in bytes, as rline does. No conversion is
   needed between the two.

## What transit gives rline

1. The Go API follows the Rust binding, in Go idioms (D25). An offset, a row
   and a column are an `int`, and each counts bytes. The matches and the
   captures of a query come back as an `iter.Seq`.
2. The query engine evaluates the predicates, such as `#match?` and `#eq?`,
   that most `highlights.scm` files use (D27).
3. The package `inject` builds the layers of an injection, for HTML or
   Markdown (D27). rline imports it only if it highlights such a language.
4. Each grammar package embeds its queries, so the caller gives rline the
   highlight query with the language (D31).
5. The module `github.com/xo/transit/chromastyles` matches capture names to chroma
   token types, so that rline and usql draw the same code in the same colors
   (D32). It has its own `go.mod`. rline imports it only if rline chooses to
   use chroma styles.
6. A parse after one key and the highlight query on the result take less than
   2 ms on a statement of 10 KB, as a target that phase 3 confirms (D37).
7. A compiled query and a query cursor can be kept and used again on each
   key. A `Query` is safe to share, and a `QueryCursor` belongs to one
   goroutine at a time (D52). The target API of phase 1 will say how.
8. A sample program in `_example/` of transit highlights a SQL statement as
   rline will, in phase 5 (D53).

## What rline must not do

1. Do not import a grammar package (D11).
2. Do not use a chroma lexer (D14).
3. Do not import usql or any package that imports it.
4. Do not decide what a node means from its text. Use the captures of the
   highlight query.
