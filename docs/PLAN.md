# transit Plan

This document holds the plan for `github.com/xo/transit`. The decisions are in
[`decisions/`](decisions/README.md), one file each (D5).

This plan was written on 2026-09-29, before any code existed (D3). Ken
answered every open question on that date, and each answer is a decision, D1
to D54. A part of this plan that names a decision follows it. A new question
goes at the end of this document until Ken answers it.

These documents hold the rules and the references that come from this plan:

- [`GRAMMAR.md`](GRAMMAR.md) says how a grammar is added and updated.
- [`UPSTREAM.md`](UPSTREAM.md) says how upstream is ported, and how each
  upstream change is ported after that.
- [`CANDIDATES.md`](CANDIDATES.md) holds the set of grammars that tests the
  generator and the runtime (D16, D18).
- [`RLINE.md`](RLINE.md) and [`USQL.md`](USQL.md) are for the coding agents
  that work in rline and in usql.

Gemini Pro and DeepSeek reviewed the framing, the whole plan, and the plan
again after D44, all on 2026-09-29. "The second review" and "The third
review" below record what came of them.

## Purpose

transit is a pure Go port of [tree-sitter][ts] (D1). tree-sitter is a parser
generator and an incremental parsing library. A grammar describes a language.
The generator turns the grammar into parse tables and a lexer. The runtime
uses them to build a concrete syntax tree of a source file, and it updates the
tree after an edit without parsing the whole file again. A query language
finds patterns in the tree.

transit exists to serve two `xo` projects (D6):

1. rline, a pure Go line editor, which highlights the syntax of a line as the
   user types it.
2. usql, a command line client for SQL databases, which completes what the
   user types from the context at the cursor. It uses the dialects that dbmeta
   maintains, and it queries the database for names. usql supports SQL and
   SQL-like languages (D21).

transit only gives parsing information. It does not highlight and it does not
complete (D6). It does not import rline, usql or dbmeta. rline and usql will
change their implementations to follow transit, so transit sets the design and
they follow it. transit gives them helpful APIs in the generated code, and the
reference documents `RLINE.md` and `USQL.md`.

The port and the code that it generates are idiomatic Go (D24). Every Go
package for tree-sitter today uses cgo, and a program that imports one needs a
C compiler for each target. A program that imports transit needs the Go
toolchain and nothing else.

[ts]: https://github.com/tree-sitter/tree-sitter

## What upstream holds

These numbers are from the checkout in `tree-sitter/` (D4), at the base commit
`dcdc8cc` from 2026-09-24 (D30). The latest release was `v0.27.0`. A line
count includes comments and tests that are in the same file.

| Part | Path | Language | Lines | transit |
| --- | --- | --- | --- | --- |
| Runtime | `lib/src`, `lib/include` | C | 18,362 | Ported, except `wasm_store.c` (D7) |
| WebAssembly store | `lib/src/wasm_store.c` | C | 2,176 | Not ported (D1) |
| Generator | `crates/generate` | Rust | 22,581 | Ported, with backends (D7, D8) |
| Highlighter | `crates/highlight` | Rust | 1,828 | The injection part is ported (D27). The rest is not |
| Tags | `crates/tags` | Rust | 1,050 | Not ported (D7) |
| Command line tool | `crates/cli` | Rust | 26,976 | `generate`, `test`, `parse` and `query` are ported (D41) |
| Runtime tests | `crates/cli/src/tests` | Rust | 15,030 | Ported for every part that transit ports (D35) |
| Rust binding | `lib/binding_rust` | Rust | 4,153 in `lib.rs` | The model of the Go API (D25). The evaluation of predicates is ported (D27) |
| Loader | `crates/loader` | Rust | 2,327 | Not ported. It compiles C grammars |
| Web binding | `lib/binding_web` | TypeScript | | Not ported |
| Build tasks | `crates/xtask` | Rust | 2,790 | Not ported |

The runtime has these files. Each one becomes one Go file (D24).

| File | Lines | What it does |
| --- | --- | --- |
| `parser.c` | 2,312 | The GLR parser: shift, reduce, error recovery, reuse of old nodes |
| `query.c` | 4,880 | The query language: the pattern parser and the matcher |
| `stack.c` | 912 | The graph structured stack that holds parallel parse states |
| `subtree.c` | 1,095 | The nodes of a tree as the parser builds them |
| `node.c` | 869 | The public `TSNode` view of a subtree |
| `tree_cursor.c` | 724 | A cursor that walks a tree |
| `get_changed_ranges.c` | 557 | The ranges that differ between an old tree and a new tree |
| `lexer.c` | 511 | The lexer state that a generated lexer and a scanner call |
| `language.c` | 394 | The lookups in the parse table, and the lookahead iterator |
| `tree.c` | 182 | The tree, edits and included ranges |
| `api.h` | 1,525 | The public C API, 150 functions |

The test fixtures are in `test/fixtures`. `test_grammars/` holds 68 small
grammars, each for one feature of the generator. `fixtures.json` names 15 real
grammars, each at a release tag, that the upstream tests fetch and parse:
bash, c, cpp, embedded-template, go, html, java, javascript, jsdoc, json, php,
python, ruby, rust and typescript.

Upstream made 290 commits in the six months up to 2026-09-29, which is about
1.6 each day. Of these, 41 changed `lib/src`, 6 changed `lib/include` and 73
changed `crates/generate`. Many of the generator commits change only Rust
types, error types or lint findings, and they change nothing that transit
ports. [`UPSTREAM.md`](UPSTREAM.md) says how each commit is sorted.

## Scope

transit ports these parts:

1. The runtime, which parses, edits, walks and queries a tree. It includes the
   query engine and the lookahead iterator (D7).
2. The evaluation of query predicates, from the Rust binding (D27).
3. The injections, from `crates/highlight`, without the highlighting (D27).
4. The generator, which reads a grammar, builds the tables, and gives them to
   a backend (D7, D8).
5. The C backend and the Go backend (D8).
6. The subcommands `generate`, `test`, `parse` and `query` of `cmd/transit`
   (D41).
7. Every upstream test of these parts (D35).

transit also holds these parts, which are not ports:

1. The golden harness, which runs the upstream tool on each grammar to make
   the output that the C backend must match (D17, D19, D40).
2. The test module in `test/`, which uses cgo (D12).
3. The grammars that the Go backend generates, each with its external scanner
   ported to Go.
4. The grammars that `xo` writes, in `grammars/xo/` (D42).
5. The chromastyles module, which matches capture names to chroma token
   types (D32, D48).
6. The APIs that upstream does not have, one decision each (D28).
7. The documents `RLINE.md` and `USQL.md` (D6).

transit does not port the WebAssembly store, the highlighting itself, the tags
extractor, the loader, the web binding or the other subcommands of the
upstream tool (D1, D7).

## Architecture

### The packages and modules

D26 names each package:

| Path | Package | What it holds |
| --- | --- | --- |
| `github.com/xo/transit` | `transit` | the runtime. It imports only the standard library (D11) |
| `generate` | `generate` | the generator |
| `generate/backend/c` | `c` | the C backend |
| `generate/backend/go` | `golang` | the Go backend |
| `inject` | `inject` | the injections |
| `cmd/transit` | `main` | the command |
| `chromastyles` | `chromastyles` | a module of its own, which requires chroma (D32, D48) |
| `grammars/<repository>` | one for each grammar | one module for each grammar repository |
| `grammars/xo/<name>` | the grammar | a grammar that `xo` writes (D42) |
| `test` | | the test module, which uses cgo (D12) |

rline imports the root package and no grammar (D11). usql imports the root
package, `inject`, the grammars that it needs and, if it wants the shared
colors, `chromastyles`.

### The port is idiomatic Go

The Go is idiomatic Go, and the `go-pedantry` skill applies to it (D24). Names
are MixedCaps. A function returns an error value and several results, not an
out parameter. A slice replaces a pointer and a length. The garbage collector
replaces the reference counts and the `_delete` functions. A C enum becomes a
typed constant. A sequence comes back as an `iter.Seq`. Every exported
identifier has a doc comment.

The runtime uses 130 macros, 6 unions, 25 bit fields and 7 `goto` statements,
measured on 2026-09-29. Each has a Go form: a function or a constant, a struct
or typed flags, and a loop.

### The port still follows upstream file by file

Idiomatic Go does not break the link to upstream (D24). Each C file in
`lib/src` becomes one Go file with the same base name: `parser.c` becomes
`parser.go`. The functions of a Go file are in the same order as in the C
file. The doc comment of each ported function names its C function: the C
function `ts_parser__advance` becomes the method `(*Parser).advance`, and its
comment starts with `advance is ts_parser__advance.` The comments of the C
code are ported with the code, and the behavior is the same.

When upstream changes a function, the person who ports the change finds the
Go function by its C name and changes the same lines.
[`UPSTREAM.md`](UPSTREAM.md) holds the full rule.

Most headers do not become a file of their own. A C header holds types and
inline functions, and each goes into the Go file of the C file that uses it
most. Go itself replaces `array.h`, `alloc.h`, `atomic.h` and `host.h`, with
slices, the garbage collector and `sync/atomic`. The `unicode/` headers become
a decoder in `lexer.go` that returns what the ICU macros return, and not what
`unicode/utf8` returns. The tables in `UPSTREAM.md` name where each header
goes.

### Memory

The C runtime counts references to each subtree, and it keeps a pool of freed
subtrees. The Go runtime uses the garbage collector in place of the reference
counts. rline parses on each key that the user presses, so a pause of the
garbage collector is a pause that the user sees. The benchmarks of phase 3
decide where the Go runtime needs a pool (D37).

In C, a small leaf lives inside the pointer itself, and a node keeps its
children in the memory just before the node, where `ts_subtree_children`
finds them. Go has neither form. At the start of phase 3, a benchmark chooses
between two Go forms, and the choice becomes a decision before `subtree.c` is
ported (D29).

### The public API

The exported API has the types and the method names of the Rust binding, in
Go idioms (D25): `Parser`, `Tree`, `Node`, `TreeCursor`, `Query`,
`QueryCursor`, `Language`, `LookaheadIterator`, `Point`, `Range` and
`InputEdit`. The target API of phase 1 tests it against a working C example
before any Go is written (D10).

A `Node` is a value, as `TSNode` is in C. Text input is a `[]byte` in UTF-8 or
UTF-16, or a caller type that returns the text in chunks, as `TSInput` does.

Inside the package, tables and arithmetic keep the integer widths of
upstream, so that overflow behaves as in C. The exported API uses `int` for
byte offsets, rows, columns and counts (D25). A column counts bytes, as in C,
and rline counts bytes too.

A parse takes a `context.Context`. The C parser calls its progress callback
once in every 100 operations (`OP_COUNT_PER_PARSER_CALLBACK_CHECK` in
`parser.c`), and the Go parser reads its context at the same points, because
reading it on each step is slow (D25).

A `Language` and a `Query` are safe to share between goroutines. A `Tree` is
safe for reads from many goroutines, and a caller that keeps a version before
`Tree.Edit` calls `Tree.Copy`, as upstream uses `ts_tree_copy`. A `Parser`, a
`QueryCursor`, a `TreeCursor` and a `LookaheadIterator` belong to one
goroutine at a time (D52).

The parser logger, `ts_parser_set_logger`, becomes a Go func. The debug
graphs of `ts_parser_print_dot_graphs` go to an `io.Writer` in place of a file
descriptor. The query engine keeps the controls of upstream: the match limit,
the maximum start depth, the byte and point ranges, and the disabling of a
pattern or a capture.

transit can add an API that upstream does not have, in files that port no
upstream file, with one decision for each (D28).

### The generator and its backends

The generator ports these parts of `crates/generate`:

1. `parse_grammar.rs`, which reads `grammar.json`.
2. `prepare_grammar/`, which interns the symbols, extracts the tokens,
   expands the repeats, flattens the rules and turns each regular expression
   into an automaton (`nfa.rs`).
3. `build_tables/`, which builds the parse table and the lexer tables, finds
   the conflicts between tokens and minimizes the tables.
4. `node_types.rs`, which writes `node-types.json`.

A backend takes the result, which is what `render.rs` reads upstream: the
syntax grammar, the lexical grammar, the parse table, the main lexer table,
the keyword lexer table, the default aliases, the supertype map and the ABI
version. The backend interface takes these as Go values, in the order that
upstream builds them. It does not take C arrays or C text (D8).

Both reviews gave the same warning. If the interface is shaped like C, the Go
backend has to write C in Go syntax. The Go backend is free to write the lexer
as code or as data, and to pack the tables as it likes. Measurements in phase
3 choose the form in phase 4 (D31), and the output is idiomatic Go in either
form (D24).

The C backend is a port of `render.rs`. It turns the same values into the
same `parser.c` that upstream writes. The upstream generator keeps a fixed
order with `IndexMap` and `IndexSet`, and it hashes with `rustc-hash`, which
has no random seed. So the same order can be kept in Go. The generator never
writes output in the order of a Go map.

### The inputs of the generator

A grammar is written in JavaScript, in `grammar.js`. The upstream tool runs it
and writes the result to `src/grammar.json`. The transit generator reads only
`grammar.json`, and it runs no JavaScript (D17). A grammar that commits no
`grammar.json`, such as `DerekStride/tree-sitter-sql`, gets the one that the
golden harness makes with the upstream tool.

The generator takes four more inputs:

1. The ABI version, 14 or 15 (D17, D19). It does not write ABI 13 (D22).
2. The version of the grammar, from `tree-sitter.json` (D17).
3. Whether parse states are merged (D17).
4. The Unicode tables for a class such as `\p{L}` (D38). The generator uses
   the tables of the Go package `unicode`, which is at Unicode 17.0.0 on
   2026-09-29. The test of the C backend gives it tables at the Unicode
   version of the Rust crate `regex-syntax`, 16.0.0, so that its output
   matches the golden files.

### External scanners

Many grammars have a scanner, a hand written C file, `src/scanner.c`, that
recognizes tokens that a regular expression cannot, such as the indentation
of Python or a heredoc of Bash. The C runtime calls five functions of the
scanner: create, destroy, scan, serialize and deserialize.

In transit, a scanner is a Go type with the same methods. The scanner of each
grammar is ported by hand, line by line, from its `scanner.c`, into idiomatic
Go (D24). A C function such as `iswalpha` becomes a function that uses the
package `unicode` (D39). [`GRAMMAR.md`](GRAMMAR.md) holds the rules.

Until the Go backend exists, the test module runs the C scanner of each
grammar through cgo (D12). The Go port of a scanner is needed only for the
grammars that the Go backend writes.

## What the consumers get

transit gives parsing information, and the consumers decide what it means
(D6). transit does not know that an identifier is a table or a column. usql
finds that out with its own queries over the tree.

### The target API comes first

D10 makes the target API of a generated Go parser part of the first work. It
comes from a working C example, which uses the upstream C runtime and a real
grammar that the upstream tool generates.

Ken's sample, `_samples/sample.c`, is the start. It parses a SQL statement
with `tree_sitter_sql`, runs a highlight query, and prints each capture. The
reviews found these gaps in it:

1. It calls `malloc` and `free` without `#include <stdlib.h>`.
2. Its query names the nodes `keyword`, `string`, `number` and
   `identifier`. A real grammar can name them otherwise. For example,
   `DerekStride/tree-sitter-sql` is expected to name its keywords
   `keyword_select` and so on. This is not measured yet. The example must use
   the `highlights.scm` of the grammar.
3. It covers highlighting only. It does not cover completion.

The working example adds these parts, so that it covers what rline and usql
need:

1. The grammar is generated by the upstream tool at the base commit, and the
   example is built against the upstream runtime at the same commit.
2. It highlights with the `highlights.scm` of the grammar, on a statement of
   more than one line, and it evaluates the predicates as the Rust binding
   does (D27).
3. It edits the text as a user types, one key at a time, with `ts_tree_edit`,
   and it parses again with the old tree. It prints the changed ranges.
4. It finds the node at a cursor offset, with
   `ts_node_descendant_for_byte_range`, and it reads the field of the node in
   its parent.
5. It parses statements that are not finished, such as `SELECT * FROM ` and
   `SELECT id, FROM users`, and it prints the `ERROR` and `MISSING` nodes.
6. At the cursor, it reads the parse state with `ts_node_parse_state` and
   `ts_language_next_state`, and it lists the symbols that can come next with
   the lookahead iterator.
7. It limits a query to the visible rows with
   `ts_query_cursor_set_byte_range`.
8. It measures the time of each step, so that the benchmarks of the Go
   runtime have a C baseline.

The example is code, so it waits for Ken to say that the plan is ready (D3).
From it comes a document of the target API: the exported identifiers of the
runtime and of a generated grammar package, each mapped to the C function
that the example calls.

### The generated grammar package

A generated grammar package exports these identifiers. The target API will
test the list:

1. `Language`, which returns the `*transit.Language`.
2. A typed constant for each symbol and each field, such as `SymSelect` and
   `FieldName`. Code that completes can then `switch` on a symbol and does not
   compare strings.
3. The node types from `node-types.json`: the fields of each node, its
   children and the supertypes. usql can learn from them which node can hold
   which child.
4. The keywords of the grammar: the string tokens that the word token
   captures, and the reserved words of each set. A grammar that ignores case
   in its keywords has patterns with the `i` flag, and usql needs the list to
   offer them.
5. The queries of the grammar, from `queries/*.scm`, embedded in the package
   (D31).

### The runtime

These parts of the runtime serve the consumers:

1. Incremental parsing: `Tree.Edit`, and `Parser.Parse` with the old tree.
2. The node at an offset, and its field, its parent and its children.
3. `ERROR` and `MISSING` nodes, and `Node.HasError`.
4. The parse state of a node, the next state, and the lookahead iterator,
   which lists the symbols that the parser can accept in a state. This is the
   base of completion.
5. The query engine, with a byte range and a point range, and the evaluation
   of predicates (D27).
6. Included ranges, and the package `inject`, which builds the layers of an
   injection (D27).

### Predicates and injections

The C runtime parses query predicates, such as `#eq?`, `#match?` and
`#any-of?`, but it does not evaluate them. The Rust binding evaluates them. The
C runtime also has no engine for injections: it gives
`ts_parser_set_included_ranges`, and `crates/highlight` does the rest. transit
ports both (D27). The predicates go in the query code of the root package, and
`match?` uses the package `regexp`. The injections go in the package
`inject`, without the highlighting.

The reviews warned that a pattern written for the Rust crate `regex` can fail
to compile in Go. On 2026-09-29, every pattern of `#match?` and its relatives
in the highlight, injection, locals and tags queries of the set was compiled
with `regexp`: 202 patterns, and none failed. A difference in what a pattern
matches is still possible, and each one that a test finds becomes a decision
(D27).

### Completion at an error

A statement that is not finished, such as `SELECT * FROM `, makes the parser
recover from an error before it builds the tree. The parse state at the cursor
can then be a state of the recovery, and the lookahead iterator in that state
can list too many symbols or none. Upstream gives no API for the states before
the recovery. transit adds one: it gives the parse states of each stack
version at a byte offset (D28).

### Highlighting and chroma

A consumer uses chroma only for its styles (D14). The tokens and their kinds
come from the tree and the highlight query of the grammar. A capture name,
such as `keyword` or `string.special`, is matched to a chroma token type, and
the chroma style gives the color. The module `github.com/xo/transit/chromastyles`
holds that match, so that rline and usql draw the same code in the same
colors (D32). It has its own `go.mod`, and its tests draw the captures of each
grammar with a chroma style.

### The usql grammar and the other xo grammars

The input of usql holds backslash commands and variables, which are not SQL.
A usql grammar parses them and hands each SQL statement to a SQL grammar
through an injection (D13). transit holds it, in `grammars/xo/`, with the
other grammars that `xo` writes: MySQL, and the SQL-like languages that have
no grammar (D42). Each is a normal tree-sitter grammar, with `grammar.js` and,
if it needs one, `src/scanner.c`, so the golden files and every test apply to
it.

## How a grammar becomes a Go package

1. The golden harness fetches the grammar repository at a release tag, runs
   the upstream tool, and keeps `grammar.json`, `parser.c` and
   `node-types.json`.
2. The transit generator reads `grammar.json`, and the C backend writes
   `parser.c` and `node-types.json`. Both must match the golden files.
3. After the gate of D9, the Go backend writes the Go package.
4. A person ports `src/scanner.c` to `scanner.go`, if the grammar has one.
5. The queries in `queries/*.scm` and the tests in `test/corpus/` are copied
   into the package.
6. `go test` runs the corpus tests and the query tests of the grammar.

[`GRAMMAR.md`](GRAMMAR.md) holds every step and every rule. Each grammar
repository is a Go module of its own (D26).

## Testing plan

A port is correct when it does what upstream does. transit measures that in
the ways below.

### 1. The upstream tests, ported

Every upstream test of the parts that transit ports is ported (D35).
`crates/cli/src/tests` holds 15,030 lines of Rust tests. A Go test has a name
that a person can find from the Rust name: `test_node_child` becomes
`TestNodeChild`. The tests of the generator are inside the files of
`crates/generate`, and they are ported the same way. A test that cannot be
ported is listed with the reason.

### 2. The C backend, compared with the golden files

For each grammar, the golden harness makes `parser.c` and `node-types.json`
with the upstream tool at the base commit, at ABI 14 and ABI 15 (D19). The C
backend must write the same two files, byte for byte (D8). A difference of one
byte is a fault. The test gives the generator the Unicode tables of
`regex-syntax` (D38).

The golden harness runs the upstream tool twice on each grammar and makes sure
that the two outputs are the same, so that a difference in upstream itself is
found before it is blamed on the port. It records the version of the upstream
tool and of the Rust toolchain that built it. When the upstream tool rejects a
grammar, the harness keeps the error text as the golden output, and the
transit generator must give the same error. Some test grammars exist to test
such an error.

For a real grammar, transit stores the SHA-256 of each golden file. The 68
test grammars keep their files (D40).

`parser.c` holds every table and the whole lexer, so the same file means the
same tables. It does not prove that the runtime is correct, that a scanner is
correct, or that a query is correct. The other parts of the testing plan do
that.

### 3. The C backend output, run

The `parser.c` that the C backend writes is compiled with the upstream C
runtime in the test module, and the corpus of the grammar runs on it. The
result must be what `tree-sitter test` reports. This proves that the output
works, and not only that it matches.

### 4. The Go runtime, compared with the C runtime

The test module drives the Go runtime with the tables of each C grammar, and
it calls the C lexer and the C scanner through cgo (D12). It parses the same
input with the C runtime and with the Go runtime, and it compares the two
trees node by node: the symbol, the byte range, the point range, the field
name and the flags. It does the same after each edit, and it compares the
changed ranges. It runs each query on both and compares the captures. It
compares the symbols that the lookahead iterator lists at each offset.

When the Go backend exists, the same tests also run on the Go grammars. They
use a test build of each Go grammar, made with the Unicode tables of
`regex-syntax`, so that a character that Unicode 17.0 added does not make the
trees differ (D45).

### 4a. The lexer, compared with the C lexer

In part 4, the C lexer function of each grammar runs in the Go runtime. It
calls back into the Go port of `lexer.c`, so the Go decoder, the columns, the
end of the input and the included ranges run in every test. The lexer state
machine itself is C until the Go backend exists. So a second test lexes each
grammar at each byte offset of each corpus input, in each lexer state that
the parse table uses there, with the C runtime and with the Go runtime, and
it compares each token, its end, and the state of the lexer. The same test
runs on the Go lexers of phase 4.

### 4b. Scanners, errors, timeouts and cancellation

A Go scanner is compared with its C scanner: the tokens, and the bytes of each
serialized state. The character functions of Go and of C answer differently
for some characters that are not ASCII. The test module measures each C
function for every code point, and it lists each difference (D39). In the
test module, the Go scanners call character functions that behave as the C
library does, so the trees match exactly (D46). Any other difference is a
fault.

A difference between the Go tree and the C tree after an error is a fault, as
any difference is. The recovery of upstream depends on costs and on the order
of the stack versions, and the port keeps both. If a difference comes from the
C code depending on a memory address, the difference is recorded as a
decision, and the test knows about it.

Other tests stop a parse with a canceled context and with a deadline, at many
points in the input, and then parse again with the old tree. The tree after
the second parse must be the same as a tree from a parse that never stopped.
A test parses input that nests 10,000 levels deep for each grammar (D44).

### 5. Fuzzing

Go fuzz tests feed random input and random edits to the parser of each
grammar in the test module, and they compare each tree with the C tree. A
panic is a fault (D44). A fuzz test also calls the `Deserialize` method of
each Go scanner with random bytes.

### Speed

The benchmarks run in the test module, so that each one has a C baseline from
the same machine. They measure a first parse, a parse after one key, the
highlight query on the result, and the allocations of each. D37 holds the
targets, and phase 3 confirms them.

### CI

CI comes with the first Go package, and it runs on linux/amd64 only (D36). It
runs in three tiers:

1. On each push: the test grammars, the tests of the root package, and json,
   c, python, javascript, the PostgreSQL grammar and one large grammar.
2. Each night: every grammar, at both ABI versions, with the corpus of each C
   output.
3. Each week: the long fuzz tests.

## The gate for the Go backend

D9 says that the Go backend starts when the generator is correct for at least
50 grammars, and 100 or more if possible. Ken accepted this measurable form of
the gate with the set (D18).

A grammar counts toward the gate when all of these are true:

1. The C backend writes the same `parser.c` and `node-types.json` as the
   golden files.
2. The `parser.c` of the C backend passes the corpus of the grammar with the
   upstream C runtime, with the same result as `tree-sitter test`.
3. The golden files come from the upstream commit that transit ports.

The whole set counts when all of these are true:

1. The set holds at least 50 grammars that count.
2. The set holds every one of the 68 test grammars and the 15 fixture
   grammars.
3. The set holds at least one grammar for each feature of the generator:
   external tokens, reserved words, supertypes, each kind of precedence,
   inline rules, declared conflicts, the word token and keyword extraction,
   aliases and extras that are not tokens.
4. The set holds at least five large grammars, such as C++, C#, Kotlin,
   Haskell and TypeScript.
5. The set holds the SQL grammars that usql needs.

[`CANDIDATES.md`](CANDIDATES.md) holds the set that Ken accepted (D18). It was
measured on 2026-09-29: 171 grammars in three tiers, with the SQL grammars,
the grammars for dbmeta's languages (D23) and the 68 test grammars in
addition. Tier 1 is the 30 grammars of the `tree-sitter` organization, and it
has every feature of the generator that the scan counts. The set holds 98
grammars with a scanner, grammars with a `parser.c` of up to 63 MB, and the
SQL grammars that exist.

## Phases

Each phase ends when its tests pass, and each phase below says which. Phases 2
and 3 run at the same time (D12). Agents split each phase into units, one
upstream file or one Rust module with its tests each. An agent claims a unit
in `BACKLOG.md`, and one staged change covers one unit (D50).

### Phase 0. The plan and the rules

This phase wrote this plan, the rules, the references and the decisions. It
ended on 2026-09-29, when Ken said that the plan is ready (D54).

### Phase 1. The working C example, the target API and the golden harness

Build the working C example of "The target API comes first", and write the
document of the target API from it (D10). Write the golden harness, and make
the golden files for the test grammars and the grammars of the set. Start the
ledger of upstream commits (D30). Write the first Go package, the tests of the
agent setup, with CI and the lint configuration (D36), and the script that
writes the `replace` blocks (D49).

The phase ends when all of these are true:

1. The C example runs every step of its list, and Ken accepts the document of
   the target API.
2. The golden harness makes the golden files of every grammar of the set at
   ABI 14 and ABI 15, twice with the same result, and reports which paths of
   `render.rs` each grammar reaches.
3. The ledger holds every upstream commit after the base commit.
4. CI passes.

### Phase 2. The generator and the C backend

Port `crates/generate` at the base commit, and port `render.rs` as the C
backend (D8). Port the generator tests. Add grammars until the gate holds
(D9). The phase ends when the gate holds.

### Phase 3. The runtime, beside phase 2

Choose the Go form of a subtree with a benchmark (D29). Port the runtime and
its tests, the query engine, the evaluation of predicates and the injections
(D27), and build the test module (D12). The Go runtime runs on the tables of C
grammars that the upstream tool generates, so this phase does not wait for
phase 2.

Write a prototype of the Go lexer and tables for the JSON grammar and one
large grammar, and measure with it the compile time, the lexer speed and the
size of the module download that D31 needs, and the speed targets of D37
(D47). A Go module download is limited to 500 MB. The prototype is not kept.

The phase ends when the Go trees match the C trees for every corpus input of
every fixture grammar, and the measurements are recorded.

### Phase 4. The Go backend

Starts when the gate holds (D9). Choose the form of the tables (D31), and
write the Go backend. Generate the fixture grammars in Go, port their
scanners, and run every test of phase 3 on the Go grammars as well. The target
API of phase 1 is the design of what the backend writes. Make and tag the
first grammar module (D43).

The phase ends when every test of phase 3 passes on the Go fixture grammars,
and the speed targets hold on them (D37, D47).

### Phase 5. The grammars and the modules for rline and usql

Generate the SQL grammars in Go. Write the usql grammar and the other grammars
that `xo` needs (D13, D42). Write the chromastyles module (D32, D48), the APIs
that upstream does not have (D28), the example functions and the two sample
programs (D53), and bring `RLINE.md` and `USQL.md` up to date with the code.

The phase ends when the usql grammar and the SQL grammars pass every test in
Go, and the two sample programs run.

### Phase 6. Follow upstream

Port each upstream commit after the base commit, in order, by
[`UPSTREAM.md`](UPSTREAM.md), until transit is at the upstream `master`. From
then on, follow upstream as it changes.

While phases 1 to 5 run, the base commit does not move unless Ken moves it at
the end of a phase (D30). The ledger sorts each upstream commit as it
arrives, so that phase 6 starts with only the commits that transit ports.

## Following upstream

[`UPSTREAM.md`](UPSTREAM.md) holds the rules. In brief:

1. The base commit is `dcdc8cc` (D30). `upstream.txt`, in the root, holds the
   upstream commit that transit matches. It is written at the end of phase 6,
   when it is true.
2. Each upstream commit is sorted by the paths that it changes: it is ported,
   or it is not applicable, with a reason.
3. One upstream commit that is ported becomes one transit commit, and the
   message of the transit commit names the upstream commit.
4. The ledger, `docs/upstream/ledger.tsv`, lists every upstream commit after
   the base commit and what transit did with it (D30).
5. A behavior of upstream is ported as it is, and that includes a fault. A
   deliberate difference from upstream is a decision.

## Adding a grammar

[`GRAMMAR.md`](GRAMMAR.md) holds the rules. In brief:

1. A grammar comes from its upstream repository, at a release tag, or it is
   a grammar that `xo` writes (D42). A list records the repository, the tag,
   the commit and the license of each one. The golden harness fetches the
   commit, not the tag (D51).
2. The generated files are never edited by hand.
3. The scanner is a line by line port of `scanner.c`, in idiomatic Go. It
   writes the same bytes when it serializes its state, so that a C state and
   a Go state can be compared.
4. A grammar is done when its golden files match, its corpus gives the
   upstream result, its queries compile, and its Go trees match the C trees.

## Risks

This list merges the risks of the two reviews, most serious first:

1. The SQL grammars are weak. Most dialects of dbmeta have only the one
   generic grammar, MySQL has none of its own, and 10 SQL-like languages have
   none at all (D21). transit writes the missing ones (D42), which is work
   that no phase can size yet.
2. The garbage collector can pause the parser on a key that the user presses.
   The benchmarks measure it in phase 3, before the design of the Go backend.
3. The golden files go stale when upstream moves. They are made at one pinned
   commit, and they are made again for each upstream commit in phase 6.
4. Each scanner is a port by hand. The set holds 98 grammars with a scanner,
   but only a grammar that gets a Go package needs its scanner in Go
   (`GRAMMAR.md`). Most grammars stay in the golden stage.
5. A large grammar gives large tables, and the Go compiler can be slow on
   them. Phase 3 measures it (D31).
6. The Unicode tables of Go and of `regex-syntax` differ, so a real transit
   grammar lexes some characters otherwise than upstream (D38). The
   differences are in the characters that Unicode 17.0 added.
7. Upstream made about 1.6 commits each day. A freeze of one year leaves about
   580 commits, of which about 220 touch the runtime or the generator, at the
   rate of 2026. The ledger sorts them as they arrive (D30).
8. Ken reviews every change. A run of commits that change nothing that
   transit ports goes in one change (`UPSTREAM.md`), and the tests of each
   phase are the gate, so that Ken reviews results and not each line.
9. transit parses input that it cannot trust. A scanner that loops, input that
   nests thousands of levels deep, or a fault that makes Go panic, must not
   hang or crash the program that parses (D44).

## Prior art

These projects exist on 2026-09-29. transit is not based on any of them.

1. [`github.com/tree-sitter/go-tree-sitter`][gots], the official Go binding. It
   uses cgo.
2. [`github.com/smacker/go-tree-sitter`][smacker], an older Go binding. It uses
   cgo.
3. [`github.com/odvcencio/gotreesitter`][gotreesitter], a pure Go runtime,
   under the MIT license. It reads the tables out of the `parser.c` of each
   grammar and stores them as compressed data. It says that it ships 206
   grammars, and that 119 of them have a scanner written in Go. It does not
   port the generator. Several forks of it exist.

transit differs from the third project in three ways. It ports the generator,
with backends, so that a grammar goes from `grammar.json` to Go with no C
step. It follows the upstream source file by file, so that each upstream
commit can be ported. It gives the APIs that rline and usql need. transit can
read gotreesitter, and copies no code from it (D34).

[gots]: https://github.com/tree-sitter/go-tree-sitter
[smacker]: https://github.com/smacker/go-tree-sitter
[gotreesitter]: https://github.com/odvcencio/gotreesitter

## The second review

Gemini and DeepSeek reviewed the whole plan a second time on 2026-09-29, in
four parts: the architecture, the testing, the consumers and the process.
This section records what came of it. The sections above hold the changes.

### What the plan took from the review

1. The C runtime does not evaluate query predicates and has no injection
   engine. transit ports both (D27).
2. The lookahead iterator can mislead after the parser recovers from an
   error at the cursor. transit adds an API for the states before the
   recovery (D28).
3. The layout of a subtree needs a Go form before `subtree.c` is ported
   (D29).
4. The Go parser reads its context at the points where C looks for a
   timeout, and not on each step (D25).
5. A token by token test of the lexer, tests of timeouts and cancellation,
   and the rule for differences after an error.
6. The golden harness makes sure that upstream is stable, and keeps the
   error text of a grammar that upstream rejects.
7. CI in tiers (D36), releases (D43), the gap after the freeze (D30), and
   input that transit cannot trust (D44).
8. The keywords of a grammar in its generated package.
9. A scanner must not panic on a short buffer (`GRAMMAR.md`).

### What the review got wrong

Each of these was measured in the upstream source on 2026-09-29:

1. Gemini said that the runtime does not decode with the ICU macro `U8_NEXT`.
   It does: `ts_decode_utf8` in `lib/src/unicode.h` calls `U8_NEXT`.
2. Gemini said that the port of each C file to one Go file makes the names of
   `static` functions collide. None collide. The runtime has 174 `static`
   functions, and no name is in two files.
3. DeepSeek said that the runtime keeps each tree in one arena. It does not.
   It allocates each subtree, counts references, and keeps a pool of freed
   subtrees.
4. DeepSeek said that the fixture grammars have no corpus. They are real
   grammars, and each has `test/corpus`.

### What the plan did not take

1. Both reviews asked to port the runtime and the Go backend first, from the
   tables of the upstream tool, so that rline and usql get a parser sooner.
   That is not taken, because Ken chose the C backend first and the gate of
   D9 (D8, D9). The first thing that rline and usql get is the target API of
   phase 1 (D10). They can design against it before the Go backend exists.
2. Gemini asked to compare trees in place of the bytes of `parser.c`. That is
   not taken, because the byte comparison is the oracle of D8. The test of
   part 3 compares the trees as well.
3. Gemini asked to follow upstream only at its releases. That is not taken,
   because Ken asked that transit port each upstream change as it is made.
   D30 keeps the gap small.
4. DeepSeek asked for columns in UTF-16 and in code points. That is not
   taken. rline and usql count in bytes, as upstream does.

## The third review

Gemini and DeepSeek reviewed the plan a last time on 2026-09-29, after D44.

### What the plan took from it

1. The Go grammars are compared with the C grammars through a test build at
   the Unicode version of upstream (D45).
2. The character functions of the Go scanners behave as the C library does in
   the test module (D46).
3. Phase 3 measures with a prototype of the Go lexer, because no Go lexer
   exists before phase 4 (D47).
4. The chroma package is `chromastyles`, so that it does not clash with the
   package `chroma` (D48).
5. The modules of this repository find each other through generated `replace`
   blocks (D49).
6. Work goes in units, and each phase has a test that ends it (D50).
7. A grammar is fetched by its commit, and a grammar that disappears keeps its
   entry (D51).
8. The concurrency guarantees of each type (D52), the logger, the debug
   graphs and the controls of the query engine.
9. Example functions and two sample programs (D53), and the size of a module
   download.

### What it got wrong

1. Both reviews said that the Unicode tables of Go make the golden files
   impossible to match. D38 answers it: the test of the C backend gives the
   generator the tables of `regex-syntax`.
2. Both warned that patterns for the Rust crate `regex` fail to compile in
   Go. None of the 202 patterns in the queries of the set failed.
3. DeepSeek said that Go 1.27.1 does not exist. It is the toolchain of this
   machine on 2026-09-29.
4. DeepSeek said that the grammars that `xo` writes have no C reference. They
   have one, because each has `grammar.js` and `scanner.c` (D42).

### What the plan did not take

1. Gemini asked again to write the Go backend before the runtime is tested.
   That conflicts with D9 and D12.
2. DeepSeek asked for `int32` or `uint32` in the exported API. D25 chose
   `int`.
3. DeepSeek asked to choose the form of a subtree in phase 1. The form is
   internal to the runtime, and the target API does not depend on it (D29).

## Open questions for Ken

An open question stays here until Ken answers it. Then it becomes a decision
in [`decisions/`](decisions/README.md), and it is deleted from this list.

No question is open. Ken answered the last ones on 2026-09-29, and D24 to D53
record the answers, and D54 starts phase 1. The next question is question 53. Raise a new question
here rather than deciding one alone.
