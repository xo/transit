# Following upstream

This document holds the rules for porting upstream tree-sitter to transit, and
for porting each change that upstream makes after that. Upstream is the
repository [`tree-sitter/tree-sitter`][ts], branch `master`.

The rules were written on 2026-09-29, before any code existed (D3). Each rule
names the decision that it comes from.

[ts]: https://github.com/tree-sitter/tree-sitter

## The upstream checkout

The checkout is in `tree-sitter/`, in the root of this repository, and git
ignores it (D4). It is a reference. Do not edit a file in it, and do not
commit it.

If the checkout is missing, make it:

```bash
git clone https://github.com/tree-sitter/tree-sitter.git tree-sitter
```

Before you port anything, make sure that the checkout is clean:

```bash
git -C tree-sitter status --short
```

The command must print nothing.

Go skips a folder that holds a `go.mod`, so `go build ./...` does not see the
Go files in `tree-sitter/crates/cli/src/templates`. If upstream adds a Go file
in another place, `./...` finds it and the build breaks. After each fetch, run
this command. It must print nothing:

```bash
find tree-sitter -name '*.go' -not -path 'tree-sitter/crates/cli/src/templates/*'
```

## Two commits that matter

The base commit is the upstream commit that the first port copies. It is
`dcdc8cc55e5dfedfc858080835f153999a29ec40` on `master`, from 2026-09-24
(D30). Phases 1 to 5 of the plan port it. It moves only when Ken moves it at
the end of a phase, by a decision.

`upstream.txt`, in the root, holds the upstream commit that transit matches,
as a full hash on one line. It is written at the end of phase 6, because until
then no commit is fully ported. After that, each commit that ports an upstream
change also changes `upstream.txt`.

## Where each upstream path goes

Every upstream path has one of three rules. "Port" means that a change to the
path is ported. "Review" means that a person reads the change and decides if
it changes something that transit ports. "Not applicable" means that a change
to the path is recorded and not ported.

| Upstream path | Rule | Where it goes in transit |
| --- | --- | --- |
| `lib/src/*.c`, except `wasm_store.c` | Port | the Go file with the same base name, in the root package. `alloc.c` and `lib.c` have none, as the note under "Where each header goes" says |
| `lib/src/*.h` | Port | the Go file of the C file that uses the header most. The table below names each one |
| `lib/src/parser.h` | Port | the tables of a language and the lexer that a lex function calls, in `internal/abi` (D63), and the output of the generator |
| `lib/include/tree_sitter/api.h` | Port | the exported API of the root package |
| `lib/src/wasm_store.*`, `lib/src/wasm-stdlib` | Not applicable | nothing (D1) |
| `lib/src/unicode/`, `lib/src/portable/` | Review | the decoder in `lexer.go`. The rule for decoding is below |
| `crates/generate/src`, except `render.rs` | Port | the package `generate` |
| `crates/generate/src/render.rs` | Port | the C backend (D8). A change here can also change what the Go backend must write, so read the diff for the Go backend too |
| `crates/generate/src/dsl.js`, `crates/generate/src/quickjs.rs` | Review | nothing, because the generator reads `grammar.json` (D17). A change of `dsl.js` can change what `grammar.json` holds, and then the parser of `grammar.json` changes |
| `crates/highlight`, the code that finds injections and builds their layers | Port | the package `inject` (D27) |
| `crates/highlight`, the rest | Not applicable | nothing (D7) |
| `crates/tags` | Not applicable | nothing (D7) |
| `crates/cli/src/tests`, the tests of the ported parts | Port | the Go tests of each package (D35) |
| `crates/cli/src/tests`, the tests of highlighting and tags | Not applicable | nothing (D7) |
| `crates/cli/src/test.rs`, `crates/cli/src/parse.rs`, `crates/cli/src/query.rs` | Port | the subcommands `test`, `parse` and `query` of `cmd/transit` (D41) |
| `crates/cli`, other files | Not applicable | nothing |
| `test/fixtures/test_grammars`, `test/fixtures/fixtures.json` | Port | the test data of the generator, and the list of fixture grammars |
| `test/fixtures/error_corpus`, `test/fixtures/template_corpus` | Port | the test data of the ported corpus tests |
| `test/fixtures/rust_wasm_web` | Not applicable | nothing (D1) |
| `lib/binding_rust/lib.rs`, the evaluation of query predicates | Port | `query_binding.go`, with the types `Query` and `QueryCursor` of the Rust binding (D27) |
| `lib/binding_rust`, the rest | Review | the exported API, if the change adds or changes a method (D25) |
| `docs/` | Review | the doc comments of the API, if the change says how a function behaves |
| `lib/binding_web`, `crates/loader`, `crates/xtask`, `crates/config`, `crates/language` | Not applicable | nothing |
| `Cargo.toml`, `Cargo.lock` | Review | the port of `regex-syntax`, if the change moves the version of that crate (D59) |
| `.github`, `Cargo.*`, `flake.*`, `build.zig*`, `CMakeLists.txt`, `Makefile` | Not applicable | nothing |
| any other file in the root of upstream, such as `README.md` | Not applicable | nothing |
| any other path | Review | a person decides, and adds a row here if the path returns |

A commit that changes paths with different rules takes the strongest rule of
its paths. Port is stronger than Review, and Review is stronger than Not
applicable.

### Where each header goes

| Header | Where it goes |
| --- | --- |
| `alloc.h` | nothing. Go allocates |
| `array.h` | nothing. A slice replaces `Array(T)` |
| `atomic.h` | `sync/atomic`, in the file that uses it |
| `error_costs.h` | `parser.go` |
| `get_changed_ranges.h` | `get_changed_ranges.go` |
| `host.h` | nothing |
| `language.h` | `language.go` |
| `length.h` | `length.go` |
| `lexer.h` | `lexer.go` |
| `parser.h` | `internal/abi/parser.go` (D63) |
| `point.h` | `point.go` |
| `reduce_action.h` | `parser.go` |
| `reusable_node.h` | `reusable_node.go` |
| `stack.h` | `stack.go` |
| `subtree.h` | `subtree.go` |
| `tree.h` | `tree.go` |
| `tree_cursor.h` | `tree_cursor.go` |
| `ts_assert.h` | `assert.go` |
| `unicode.h` | `lexer.go` |

`length.go`, `reusable_node.go` and `assert.go` have no C file of the same
name. They exist because their header holds enough code for a file of its
own. `point.go` ports `point.c` and `point.h`.

`alloc.c` becomes nothing, because Go allocates. `lib.c` includes every other
C file, to build the runtime as one file, and it becomes nothing.

## How Go code follows C code

The Go is idiomatic Go (D24), and it still follows the C closely enough that
a port of a later upstream change is a small edit. These rules do both.

The link to the C:

1. One C file becomes one Go file with the same base name.
2. The functions in the Go file are in the same order as in the C file.
3. A Go function has a name that a person finds from the C name. A C function
   `ts_<type>__<name>` becomes the unexported method `<name>` on the Go
   type. A C function `ts_<type>_<name>` becomes the exported method
   `<Name>`, with the Rust binding name where they differ (D25).
4. The doc comment of each ported function names the C function: `advance is
   ts_parser__advance.` Search the Go code for a C name to find its port.
5. A C local variable keeps its name, in Go form. `did_merge` becomes
   `didMerge`.
6. A comment in the C code is ported with the code that it describes.
7. The behavior is the same, including the faults of upstream.

The idiomatic Go:

8. Inside the package, an integer keeps its width and its sign, so that
   overflow and conversion behave as in C. A `uint32_t` becomes a `uint32`.
   The exported API uses `int` for offsets, rows, columns and counts, and
   converts at the boundary (D25).
9. A function that returns a `bool` and writes an out parameter returns its
   results and an error, or its results and a `bool` where Go uses that form,
   such as a lookup.
10. A C function that takes a pointer and a length takes a slice in Go.
11. A C callback with a `void *payload` becomes a Go interface or a Go func.
12. A macro becomes a function or a constant. A union becomes a struct, or an
    interface with one type for each case. A bit field becomes a field or a
    typed flag. A `goto` becomes a loop or a labeled `break` or `continue`.
13. A `_delete` or `_free` function has no port, because the garbage
    collector frees the memory. The mapping table of the file names it as not
    needed.
14. A C enum becomes a typed constant with a `String` method.
15. A sequence that the API returns is an `iter.Seq` or an `iter.Seq2` (D25).
16. `ts_assert` becomes a call to `assert`. The C macro still evaluates its
    argument when assertions are off, so a Go port must keep any side effect
    of the argument.
17. A port commit translates the C into idiomatic Go and keeps the behavior.
    It does not restructure the code more than idiomatic Go needs, because a
    change that does more makes the next port harder. If the Go code can be
    better in another way, write the idea in [`BACKLOG.md`](BACKLOG.md).

### The decoder

The C runtime decodes UTF-8 with the ICU macro `U8_NEXT`. For a byte that is
not valid UTF-8, it returns the code point `-1` (`TS_DECODE_ERROR`), and it
consumes the bytes that ICU calls the maximal subpart. The Go function
`utf8.DecodeRune` returns `utf8.RuneError` and consumes one byte in some of
these cases. These two results are different. The lexer, the byte offsets of
the tree and the column of each point depend on them.

The Go decoder must return what the C decoder returns, for every input. The
test of the decoder compares the two for every sequence of up to four bytes,
in the test module (D12).
The same rule holds for the two UTF-16 decoders.

### The generator

The same rules hold for `crates/generate` and for the parts of
`lib/binding_rust` and `crates/highlight` that transit ports (D27), with Rust
names in place of C names. A Rust module becomes a Go file with the same base
name. A Rust `impl` method becomes a Go method on the same type. A Rust `enum`
with data becomes a Go interface with one type for each variant, or a struct
with a kind field. The file of each type names the choice.

The generator must never write output in the order of a Go map. The upstream
generator uses `IndexMap` and `IndexSet`, which keep the order of insertion.
Where upstream uses one, transit uses a slice with a map beside it, or the
keys in the same order.

The generator reads a pattern with a port of the Rust crate `regex-syntax`,
and it uses the Unicode tables of that port for every output (D59, D60).
Upstream pins the version of the crate in its `Cargo.toml` and `Cargo.lock`.
When upstream moves to a new version, follow these steps:

1. Port the changes of the crate between the two versions to the packages
   `ast` and `hir` in `generate/internal/regexsyntax`, in the same way as a
   change of upstream.
2. Fetch the new version of the crate with `cargo fetch` in the upstream
   checkout. Cargo keeps its source under `~/.cargo/registry/src`.
3. Write the tables again with `cd test && go run ./cmd/regextables -version
   <version>`. The command writes the package `unicodetables`, and its test
   compares the files with the crate.

## Porting an upstream change

These steps start in phase 6, after the base port is done. Until then,
`upstream.txt` does not exist, and the ledger test does the work of steps 3 to
11: it sorts each new upstream commit as it arrives (D30), as "The ledger"
below says. Only the commits that transit ports wait for phase 6.

1. Fetch the upstream changes:

   ```bash
   git -C tree-sitter fetch origin
   ```

2. Run the command from "The upstream checkout" that finds new Go files. It
   must print nothing.

3. List the commits that transit has not ported, oldest first:

   ```bash
   git -C tree-sitter log --reverse --format='%H %cs %s' "$(cat upstream.txt)..origin/master"
   ```

4. Take the oldest commit. List the paths that it changes:

   ```bash
   git -C tree-sitter show --stat --format='%H %s' <commit>
   ```

5. Find the rule of each path in the table above. The commit takes the
   strongest rule.

6. If the rule is Not applicable, add the commit to the ledger with the
   status `not-applicable` and the reason. Go to step 11.

7. If the rule is Review, read the diff:

   ```bash
   git -C tree-sitter show <commit>
   ```

   If the diff changes nothing that transit ports, add the commit to the
   ledger with the status `not-applicable` and the reason. Go to step 11.
   Otherwise, continue at step 8.

8. Port the diff. Find each changed C function or Rust function by its name in
   the Go code, and change the same lines. Port each test that the commit adds
   or changes.

9. If the commit changes the generator, make the test data of the generator
   again, as "The test data of the generator" says, and generate every grammar
   package again, as [`GRAMMAR.md`](GRAMMAR.md) says.

10. Run every test, as "Before you stage" in [`AGENTS.md`](../AGENTS.md) says.
    Every test must pass.

11. Write the full hash of the commit to `upstream.txt`.

12. Stage the change, and give Ken a commit message in the form below. Do not
    commit. Ken commits (D2).

13. Go back to step 4 for the next commit.

Port one upstream commit in one transit commit. You can put a run of
`not-applicable` commits in one transit commit, because each one changes only
the ledger and `upstream.txt`.

### The commit message

A transit commit that ports an upstream commit has this form:

```text
<area>: <the subject of the upstream commit>

<what changed in transit, in plain English, and why, if the upstream subject
does not say it>

Upstream: tree-sitter/tree-sitter@<full hash>
```

`<area>` is `runtime`, `query`, `inject`, `generate`, `backend/c`,
`backend/go`, `cli` or `test`. The `Upstream:` line is the last line before
the trailers that the agent adds. A tool can read the ledger again from these
lines.

A commit that records only `not-applicable` commits has this form:

```text
upstream: record <n> commits that change nothing transit ports

Upstream: tree-sitter/tree-sitter@<first full hash>..<last full hash>
```

## The ledger

The ledger lists every upstream commit after the base commit, oldest first,
and what transit did with it (D30). It is the file `docs/upstream/ledger.tsv`,
with one line for each upstream commit and these columns, separated by a
tab:

| Column | What it holds |
| --- | --- |
| `upstream` | the full hash of the upstream commit |
| `date` | the commit date, as `YYYY-MM-DD` |
| `status` | `ported`, `not-applicable` or `pending` |
| `transit` | the short hash of the transit commit, or empty until Ken commits |
| `note` | the reason for `not-applicable` or `pending`, or the subject for `ported` |

`pending` means that the commit is known and not ported yet. While the base
commit is frozen, a commit that transit will port waits for phase 6 as
`pending`, and the ledger is the list of them, with no item in
[`BACKLOG.md`](BACKLOG.md). After phase 6 starts, a commit that stays `pending`
for another reason, such as an answer from Ken, also has an item in
`BACKLOG.md`. A later commit can be ported while an earlier one is `pending`
only if the two change different files.

`TestTheLedgerIsComplete`, in `upstream_test.go`, makes sure of the form of
the ledger. When the checkout in `tree-sitter/` is there, it also compares the
ledger with the history of upstream, and it fails when a commit is missing.
To add the new commits, fetch upstream and run:

```bash
git -C tree-sitter fetch origin
go test -run TestTheLedgerIsComplete -update .
```

The test sorts each new commit by the table in "Where each upstream path
goes", and writes one of three notes:

1. `not-applicable`, with a note that names the paths, when no path is
   ported or reviewed.
2. `pending`, with the note `port in phase 6:` and the paths, when a path is
   ported.
3. `pending`, with the note `review:` and the paths, when a path needs a
   person to decide.

Read each commit whose note starts with `review:`, and change its line to
`not-applicable` with the reason, or to `port in phase 6:`. The rules are in
the Go variable `pathRules`, and `TestThePathRulesFollowUpstreamMD` makes sure
that each rule is in the table above.

A test will make sure that the ledger has one line for each commit from the
base commit to `upstream.txt`, in order, with no gap and no line twice.

## The test data of the generator

The test of part 2 of the testing plan compares the `parser.c` that transit
writes with the `parser.c` that upstream writes. The upstream files are made
once for each upstream commit, and they are committed, so that nobody needs
Rust to run the test.

The test grammars in `tree-sitter/test/fixtures/test_grammars` hold only
`grammar.js`, and transit reads only `grammar.json` (D17). So the
upstream tool writes three files for each test grammar: `grammar.json`,
`parser.c` and `node-types.json`. These files are small, and the package
`generate` holds them as test data.

A real grammar, such as a fixture grammar, commits its own `grammar.json`. Its
`parser.c` can be tens of megabytes, so transit holds only the SHA-256 of the two
upstream files, in `grammars/grammars.json`. [`GRAMMAR.md`](GRAMMAR.md) says
how. D40 holds this.

The golden harness makes the test data. It is the command `test/cmd/golden`
of the test module (D58). To make the golden files again, run:

```bash
cd test && go run ./cmd/golden
```

It does these steps:

1. It makes sure that the checkout in `tree-sitter/` is at the base commit,
   and builds the upstream tool with `cargo build --release -p
   tree-sitter-cli` if it is not built.
2. For each test grammar, it runs `tree-sitter generate` in a copy of its
   folder, at ABI 15, at ABI 14, and at ABI 15 with `--disable-optimizations`.
   It writes `generate/testdata/<grammar>/grammar.json`, and `parser.c` and
   `node-types.json` in a folder for each variant, such as
   `generate/testdata/<grammar>/abi14/`. A grammar that the tool rejects gets
   `error.txt` in place of the two files. Upstream expects 12 of the 68 test
   grammars to fail.
3. For each fixture grammar, it fetches the repository at its tag or branch,
   runs `tree-sitter generate` on the committed `src/grammar.json` at ABI 14
   and ABI 15, and writes the hashes and the fetched commit to
   `grammars/grammars.json`.
4. For each candidate grammar of [`CANDIDATES.md`](CANDIDATES.md), it does
   the same. It reads the tables of the tiers, of SQL and of the languages of
   dbmeta. It fetches a grammar that the record holds at its commit (D51),
   and a new one at the latest release of its repository, or its latest tag,
   or its default branch. A grammar that commits no `src/grammar.json` gets
   one from `grammar.js`, as [`GRAMMAR.md`](GRAMMAR.md) says. A fixture
   grammar keeps its tag.
5. It runs the tool twice for each file, and it fails when the two runs
   differ.
6. The set `corpus` runs the corpus of each recorded grammar, which the other
   sets must have fetched. It copies the grammar twice. It generates one copy
   with the upstream tool and one with `transit generate`, at ABI 15, and runs
   `tree-sitter test` on each. It writes the result of the upstream copy to the
   field `corpus` of the record, and whether transit gives the same result. It
   fails when transit gives another result. This is the test of the gate of D9
   that the `parser.c` of transit passes the corpus.
7. It writes the upstream commit, the version of the tool and the version of
   Rust to `generate/testdata/upstream.json`, and to the top of
   `grammars/grammars.json`.
8. It prints which paths of `render.rs` the outputs reach, and names each
   path that no output reaches.

The flag `-set` chooses `tests`, `fixtures`, `candidates` and `corpus`,
separated by commas, and the default is `tests,fixtures`. The flag `-only`
names single grammars, or for the candidates, single repositories such as
`gmr/tree-sitter-postgres`. The harness keeps each repository in its cache
under its owner and its name, such as
`~/.cache/transit/grammars/tree-sitter/tree-sitter-json`. The environment
variables `TREE_SITTER_ABI_VERSION` and `TREE_SITTER_JS_RUNTIME` change the
output of the tool, so the harness clears them. It sets `NO_COLOR`, so that an
error has no codes for the colors of a terminal.

`TestTheGoldenFilesNameTheBaseCommit` fails if the upstream commit beside the
test data is not the commit that transit ports. That is the base commit until
phase 6 ends, and the commit in `upstream.txt` after that.

## A change to the ABI

A language has an ABI version (the version of the layout of its tables). It
is `TREE_SITTER_LANGUAGE_VERSION` in `api.h`, and it was 15 on 2026-09-29.
The runtime accepts versions from `TREE_SITTER_MIN_COMPATIBLE_LANGUAGE_VERSION`,
which was 13, up to it.

transit makes each grammar from `grammar.json`, so a transit grammar is always
at the ABI version of the transit generator. If an upstream commit changes the
ABI version, port the commit, and generate every grammar package again in the
same change.

If upstream moves to a new ABI version, such as 16, while the base commit is
frozen, the commit gets the status `pending` in the ledger. Ken decides
whether to move the base commit at the end of the phase (D30).

## A difference from upstream

Port the behavior of upstream as it is, including a fault. A port that fixes
an upstream fault is no longer a port of that code, and the next upstream
change to the same lines does not apply.

A Go port can panic where the C code reads past the end of an array and goes
on with a wrong value. That is a fault of upstream, which the port makes
visible. Treat it as the steps below say, and do not hide the panic.

If you find a fault in upstream:

1. Write it in [`BACKLOG.md`](BACKLOG.md), with the upstream file, the line
   and a case that shows it.
2. Tell Ken. Ken decides whether transit reports it upstream. An agent does
   not post to the upstream repository (hard rule 11 in `AGENTS.md`).
3. Do not fix it in transit unless Ken decides to. A fix that Ken decides on
   is a decision in [`decisions/`](decisions/README.md), and it is listed
   under "Differences from upstream" in the root `README.md`.

When upstream fixes the fault later, the port of that fix removes the
difference, and the decision gets the status `Superseded by` the decision
that records it.
