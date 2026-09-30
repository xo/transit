# Backlog

This document lists work that is known and not done. Each item names the
decision or the question that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open
questions for Ken. When an item here is done, delete it, and record in
`decisions/` anything that was decided on the way (dbmeta D110).

## Units in progress

An agent claims a unit of port work here before it starts, with the unit, the
agent and the date, and deletes the claim when Ken commits the unit (D50, D61).

| Unit | Agent | Date |
| --- | --- | --- |

## The generator

### Port the log of the generator

Upstream writes log lines with `debug!` and `info!` in `build_tables/`, and
`report_state_info` in `build_tables.rs` writes the parse states of one rule
for the option `--report-states-for-rule` of the tool. The generator has no
logger yet, so the port leaves them out. The generator logs with `log/slog`
(D67). Add the logger to the options, then port the lines,
`report_state_info` and `--report-states-for-rule`. Some functions of
`build_lex_table.go` then take the `StrPool` again, as upstream does.

## The runtime

### Move the list of upstream failures into each module

D88 says that a grammar package holds `testdata/failing.txt` with the names
of its corpus cases that fail upstream, and that the golden harness writes
it. Today the generator writes the names into `grammar_test.go` from
`grammars/grammars.json`. Change the harness, the generator and
`internal/grammartest`. No fixture grammar has a failing case, so no module
gets the file yet.

### Add QueryMatch.Remove

D89 gives the Go API a form of `QueryMatch::remove` of the Rust binding.
Add it to `query_binding.go` and to `docs/API.md`, and call it from the
package `inject` and from the highlight test of `internal/grammartest` in
place of their imitation. Run the two tests of `upstream_query_test.go` that
skip for want of it, and delete item 3 of "Choose whether some upstream tests
get a Go form".

### Compare the highlight test with the upstream tool

The highlight test of a grammar module follows the highlighter of upstream
(D80, D84). `tree-sitter test` passes the files of `test/highlight` of 12
grammars, and so does the port, in `test/cgrammar`: c, c_sharp, css, html,
java, javascript, lua, php, python, ruby, toml and yaml. The test also passes
in the modules c, cpp, html, java, javascript, php, python and ruby. The
cache holds 48 more folders of `test/highlight`. Compare the port with the
upstream tool on them, and record each difference.

The port in `internal/grammartest` emulates `QueryMatch::remove` of the Rust
binding, because the Go API has no form of it. It knows a removed match by
its pattern and by the captures that it had. Two matches of one pattern
with the same first captures, such as two branches of a quantifier, look
like one match. The package `inject` does the same for its injections.

### Meet target 3 of D37

A parse after one key allocates more on a longer statement (D73). On the
postgres statement of D73, it allocates 200 times at 5 KB and 805 times at
40 KB, and the time of C grows the same way. Most of the allocations are
stack nodes. `stack.go` leaves out the free list of stack nodes of C, which
keeps up to 50 freed nodes. Ken kept the target as it is, and phase 4 works
on it with the real Go grammars.

### Cut the memory of the generator for large grammars

The generator takes 1 minute 58 seconds and 10.9 GB of memory for postgres,
as the C backend does (D73). CI runs the PostgreSQL grammar on each push. A
runner with less memory than that cannot generate it. Measure where the
memory goes, and compare it with the upstream tool.

### Compare the trees at ABI 14

`test/cgrammar` compares the Go trees with the C trees at ABI 15 only. Compare
them at ABI 14 too, from the `src/parser.c` of each grammar. The files
`upstream_*_test.go` there port the tests of `crates/cli/src/tests` of
upstream (D35). The tests that the Go API has no form for are listed below.

### Choose whether some upstream tests get a Go form

Some tests of `crates/cli/src/tests` test a part of the Rust binding that the
Go API does not have, so they skip or are not ported:

1. `test_decode_utf32`, `test_decode_cp1252`, `test_decode_macintosh` and
   `test_decode_utf24le` of `parser_test.rs` give the parser a decode
   function. `docs/API.md` gives `ts_parser_parse_with_options` no Go form.
2. `test_edit_point` and `test_edit_range` of `node_test.rs` test
   `ts_point_edit` and `ts_range_edit`. `docs/API.md` says that these become
   methods if a consumer needs them.
3. `QueryMatch::remove` calls `ts_query_cursor_remove_match`, and
   `TestQueryCapturesWithMatchesRemoved` and
   `TestQueryCapturesWithMatchesRemovedBeforeTheyFinish` of `query_test.rs`
   test it. `docs/API.md` uses that function only inside `Captures`. The two
   tests skip.
4. `Query::deep_clone` calls `ts_query_copy`, and `TestQueryDeepClone` tests
   it. `docs/API.md` gives `ts_query_copy` no Go form (D24). The test skips.

Ken chooses in the review of `docs/API.md` whether to keep this. If a part
gets a Go form, add it to the runtime, and port its tests or remove their
`t.Skip`.

### Support the Neovim dialect when the tier 1 grammars need it

D69 defers `#lua-match?` of D55 and the rest of the Neovim dialect of
queries. When the work on the tier 1 grammars needs them, build the parts
that "What support would take" in [`NEOVIM.md`](NEOVIM.md) lists. The first
part is the port of the matcher of LuaJIT 2.1 from `src/lib_string.c`, for
`#lua-match?`.

### Keep the tables of NEOVIM.md current

The tables of [`NEOVIM.md`](NEOVIM.md) come from one scan of the query files
of the set on 2026-09-30. Add a check to the golden harness that scans the
query files of each grammar in the cache, and that fails when a table of
`NEOVIM.md` or a mark [N] of `CANDIDATES.md` differs from the scan. Until the
check exists, step 8 of "Steps to add a grammar to the set" in
[`GRAMMAR.md`](GRAMMAR.md) keeps them current by hand.

### Run the comparison with C in a nightly job

The tests of `test/cgrammar` skip in CI, because CI has no checkout of
upstream and no cache of grammars. The nightly tier of D36 can check out
upstream at the base commit, fetch the fixture grammars with the golden
harness, and run the test module. On 2026-09-29 Ken chose to wait with it.

### Compare the decoders with the C decoders

[`UPSTREAM.md`](UPSTREAM.md) says that the test of the decoder compares the
Go decoders with the C decoders for every sequence of up to four bytes, in
the test module. The tests of `lexer.go` compare `decodeUTF8` with the
maximal subpart of the Unicode standard, which ICU follows, and they test the
UTF-16 decoders case by case. Add the comparison with `U8_NEXT` and
`U16_NEXT` of C when the test module can call the decoders of the runtime.

## The setup of the repository

These items come in phase 1 and later, as the plan says.

### Add the tests of the port rules

[`UPSTREAM.md`](UPSTREAM.md) and [`GRAMMAR.md`](GRAMMAR.md) name tests that
hold their rules:

1. Each folder in `grammars/` has an entry in `grammars/grammars.json`, and
   each entry has a folder.
2. Each ported function has a doc comment that names its C function or its
   Rust function.
3. From phase 6 on, the ledger has one line for each commit from the base
   commit to `upstream.txt`, in order, with no gap and no line twice, as
   "The ledger" in `UPSTREAM.md` says.
