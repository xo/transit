# What rline gets from transit

This document is for a coding agent that works in
[rline](https://github.com/xo/rline). It says what transit gives rline for
syntax highlighting, what is decided, and what is still open.

transit holds the runtime, with the query engine, the package `inject` and
the Go packages of the grammars. [`API.md`](API.md) holds the API, which came
from the working C example of phase 1 (D10). This document names the parts
that rline uses. The sample program `_example/highlight` highlights the
input of usql as rline will (D53).

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
5. The tokens and their kinds come from transit, and never from a chroma
   lexer (D14). rline takes its colors, if it wants shared ones, from the
   package `styles` (D65).

rline has its own rule on its dependencies, rline D7. Adding transit to it is
a decision of rline. Ken makes it in rline.

## How highlighting will work

These are the steps that rline takes. [`API.md`](API.md) gives the API of
each step, and `_example/highlight` runs them.

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
   captures of a query come back as an `iter.Seq` or an `iter.Seq2`.
2. The query engine evaluates the predicates, such as `#match?` and `#eq?`,
   that most `highlights.scm` files use (D27).
3. The package `inject` gives the layers of an injection (D72), for HTML or
   Markdown (D27). rline imports it only if it highlights such a language.
4. Each grammar package embeds its queries, so the caller gives rline the
   highlight query with the language (D31).
5. The package `github.com/xo/transit/styles` holds styles that are keyed on
   capture names, so that rline and usql draw the same code in the same
   colors (D65). It is a module of its own, and it imports only the
   standard library (D99). The background of a style applies only when rline asks for it,
   and rline chooses the color depth.
6. A parse after one key and the highlight query on the result take less than
   2 ms on a statement of 10 KB (D37). Phase 3 measured 1.15 ms on a SQL
   statement of 10 KB, with the highlight query over the whole tree (D73).
7. A compiled query and a query cursor can be kept and used again on each
   key. A `Query` is safe to share, and a `QueryCursor` belongs to one
   goroutine at a time (D52). [`API.md`](API.md) says how.
8. The sample program `_example/highlight` of transit highlights the input
   of usql as rline will (D53). It types a text one key at a time. After
   each key, it edits the tree of the usql grammar and parses again with the
   old tree. It finds the layer of each SQL statement with `inject.Layers`
   and `inject.WithReplacer`, and runs the highlight query of the usql
   grammar and of the SQL grammar of a dialect. It prints the text with the
   escapes of a terminal of 256 or 16 colors, in a style of the module
   `styles`, and the time of each edit. `inject.Layers` parses each layer
   with no old tree (D72), so the time of an edit counts the parse of every
   layer. Run it in the folder `_example`:

   ```bash
   go run ./highlight -dialect postgres -style monokai -colors 256
   ```
9. The C example measured the cost of one key: edit, parse again, changed
   ranges and highlight took 12.2 µs on average in C, for the SQL grammar. A
   query took 6.7 ms to compile, so rline compiles each query once.
10. A query of a grammar can hold patterns written for Neovim, such as the
    Lua pattern `%d` in `#match?`. Under the rules of the Rust binding they do
    not match. `#lua-match?` and the other predicates of Neovim wait until
    the tier 1 grammars need them (D69). Until then transit does not
    evaluate them, as the Rust binding does not. [`NEOVIM.md`](NEOVIM.md)
    lists the grammars whose queries use them.

## What rline must not do

1. Do not import a grammar package (D11).
2. Do not use a chroma lexer (D14).
3. Do not import usql or any package that imports it.
4. Do not decide what a node means from its text. Use the captures of the
   highlight query.
