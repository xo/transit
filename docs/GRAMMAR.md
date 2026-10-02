# Adding a grammar

This document holds the rules for adding a grammar to transit, for porting its
external scanner, and for updating it to a new release. A grammar describes
one language, such as JSON or Go. In transit, a grammar is a Go package that
exports one function, `Language`, which returns a `*transit.Language`.

The rules were written on 2026-09-29, before any code existed (D3). Today
`transit generate` works with the C backend, and the golden harness runs it.
`transit generate --backend go` writes a Go package, and `grammars/json` is
the first one. Each rule names the decision that it comes from.

## Two stages

A grammar enters transit in two stages, because the Go backend comes after
the generator is correct for 50 grammars (D9):

1. The golden stage. The grammar joins the set that tests the generator. It
   has an entry in `grammars/grammars.json`, and the golden harness makes its
   golden files. It has no Go package yet. The steps are in "Steps to add a
   grammar to the set" below.
2. The Go stage. After the gate of D9, the Go backend writes a Go package for
   the grammar, and a person ports its scanner. The steps are in "Steps to
   add a Go package" below.

Most grammars stay in the golden stage. A grammar gets a Go package when Ken
chooses it (D18), for example the SQL grammars that usql needs.

## Where a grammar comes from

A grammar comes from one of two places:

1. Its upstream repository, such as `github.com/tree-sitter/tree-sitter-json`,
   at a release tag. transit does not change such a grammar. If it has a
   fault, Ken decides whether to report it. An agent does not post to the
   repository of another project (hard rule 11 in `AGENTS.md`).
2. `xo` writes it, in `grammars/<name>`, because no grammar exists that
   usql can use (D42, D104). The usql grammar (D13), MySQL and the SQL-like
   languages without a grammar are of this kind. "Grammars that xo writes"
   below holds their rules.

Ken chooses each grammar. [`CANDIDATES.md`](CANDIDATES.md) holds the set
that he accepted (D16, D18). The 15 grammars that upstream lists in
`test/fixtures/fixtures.json` are in it, at the tags that it lists, because
the upstream tests use them.

One upstream repository can hold more than one grammar. `tree-sitter.json`,
in the root of the repository, lists them under `grammars`, each with a
`path`. `tree-sitter-typescript` holds `typescript` and `tsx`, and
`tree-sitter-php` holds `php` and `php_only`.

## The record of each grammar

`grammars/grammars.json` lists every grammar in transit. The golden harness
writes it (D58). Its top level names the upstream commit that made the golden
files (`upstream`), the version of the upstream tool (`tool`) and the version
of the Rust compiler that built the tool (`rust`). It has one entry for each
grammar under `grammars`, with these fields:

| Field | What it holds |
| --- | --- |
| `name` | the `name` in `grammar.json`, such as `c_sharp` |
| `package` | the Go package name, as "Names" below says |
| `repository` | the URL of the upstream repository |
| `tag` | the release tag, such as `v0.24.8` |
| `branch` | the branch, when upstream names a branch in place of a tag |
| `commit` | the full hash of the commit that the harness fetched (D51), or empty for a grammar that xo writes, which is in this repository |
| `path` | the folder of the grammar in the repository, or `.` |
| `license` | the license that `tree-sitter.json` names, such as `MIT` |
| `scanner` | `true` if the grammar has `src/scanner.c` |
| `set` | why the grammar is in the set, such as `fixture` |
| `status` | `available`, or `unavailable` when the repository disappeared (D51) |
| `golden` | the golden files at `abi14` and `abi15` (D19) |
| `corpus` | the result of the corpus of the grammar, which the set `corpus` of the golden harness writes. The grammar has none when it has no `test/corpus` or the tool rejects it |

Each golden file has these fields:

| Field | What it holds |
| --- | --- |
| `parser_c` | the SHA-256 of the `parser.c` that the upstream tool writes, as "The generator test" says |
| `node_types` | the SHA-256 of the `node-types.json` that the upstream tool writes |
| `error` | the error text, in place of the two hashes, when the tool rejects the grammar |
| `paths` | the paths of `render.rs` that the `parser.c` reaches |

The field `corpus` has these fields:

| Field | What it holds |
| --- | --- |
| `tests` | the number of corpus tests that `tree-sitter test` runs with the `parser.c` of the upstream tool |
| `failures` | the number of those tests that fail |
| `failing` | the name of each test that fails, in the order of the run, such as `expressions/Binary operators`. The name joins the names of the groups of the test and its own name with `/` |
| `transit` | `same` when the `parser.c` of transit gives the same result, and `different` when it does not |

`TestTheGoldenFilesNameTheBaseCommit`, in `upstream_test.go`, makes sure of the
form of the file.

A test will make sure that each folder in `grammars/` has an entry, and that
each entry has a folder.

The golden harness fetches the commit of an entry, and not its tag, so a tag
that moves changes nothing. If a repository disappears, its entry and its
hashes stay, marked unavailable, and the grammar leaves the count of the gate
of D9. Ken decides whether another grammar replaces it (D51).

## Names

A package has the name of its folder, in lowercase (D107). The package of a
grammar at the root of its module has the name of the module folder. The
folder `go` gives the package `golang`, because `go` is a keyword of Go
(D77, D86). A folder whose name is not a Go package name stops the generator
with an error. A Go package name has no underscore, as the `go-pedantry`
skill says. The name of the grammar does not change. `Language.Name` gives
it, and the queries and the injections use it. The grammar `TSQL` lives in
`grammars/sqlserver`, so its package is `sqlserver`, and `Language.Name`
gives `TSQL`.

If the language of a grammar is the language of one dialect of dbmeta, the
folder of its module has the name of the dialect. The table of D107 lists
them. `Crary-Systems/tree-sitter-tsql` lives in `grammars/sqlserver`, and
`andreasmaierde/tree-sitter-plsql` in `grammars/oracle`. The folder of any other module has the name of the
upstream repository without the `tree-sitter-` prefix and with each `-`
removed. `tree-sitter-embedded-template` becomes `embeddedtemplate`. If two
grammars or two repositories get the same name, ask Ken.

The two repositories `tree-sitter-sql` give the same folder and the same
grammar name, `sql`. `DerekStride/tree-sitter-sql` gets `grammars/sql`, and
`m-novikov/tree-sitter-sql` gets no Go package (D106). Two entries of
`grammars/grammars.json` can give one module folder. Then the tests and the
golden harness take the entry of the repository that `tree-sitter.json` of
the module names under `metadata.links.repository`.

## The layout of a grammar

D26 holds where a grammar lives: one Go module for each upstream repository,
under `grammars/<repository name>`, and one package for each grammar in it.

A repository that holds one grammar, such as JSON, looks like this:

| Path | What it holds | Who writes it |
| --- | --- | --- |
| `grammars/json/go.mod` | the module `github.com/xo/transit/grammars/json` | a person, once |
| `grammars/json/LICENSE` | the license file of the upstream repository | copied |
| `grammars/json/grammar.json` | `src/grammar.json` of the upstream repository | copied |
| `grammars/json/tree-sitter.json` | `tree-sitter.json` of the upstream repository, which gives the version of the grammar | copied |
| `grammars/json/parser.go` | `Language`, the constants, `Queries`, `NodeTypes`, `Keywords`, the tables and the lexer | `transit generate` |
| `grammars/json/node-types.json` | the node types, which `parser.go` embeds | `transit generate` |
| `grammars/json/grammar_test.go` | the tests of "Tests" below, which call the package `internal/grammartest` | `transit generate` |
| `grammars/json/scanner.go` | the external scanner, if the grammar has one | a person, as a port |
| `grammars/json/example_test.go` | the example functions that pkg.go.dev shows (D53), if the package has them | a person |
| `grammars/json/queries/*.scm` | `queries/*.scm` of the upstream repository | copied |
| `grammars/json/testdata/corpus/` | `test/corpus/` of the upstream repository | copied |
| `grammars/json/testdata/highlight/` | `test/highlight/`, if it exists | copied |
| `grammars/json/testdata/failing.txt` | the names of the corpus cases that fail upstream, if a case fails, as "Tests" below says | the golden harness |

A repository that holds more than one grammar has one folder for each grammar
under the folder of the module, such as `grammars/typescript/typescript` and
`grammars/typescript/tsx`. The folder comes from the `path` of the entry of
the grammar in `tree-sitter.json`, with each `_` and `-` removed, so
`php_only` gives `phponly`, and it is also the name of the package (D90). `LICENSE` and `tree-sitter.json` are in the folder
of the module. Each package holds its own copy of `queries/` and of
`testdata/`, because each package embeds its own queries, and each copies
only the query files that its entry in `tree-sitter.json` lists (D86). Each
package holds the whole corpus, and a case runs in the package of the grammar
that its `:language` names, or of the first grammar of `tree-sitter.json`
(D83). Code that two scanners share, such as `common/scanner.h`, becomes the
package `internal/scan` of the module (D85).

A grammar of a repository can have its own `test/corpus` in its own folder,
as each grammar of `tree-sitter-postgres` has. Then its package holds that
corpus only. `tree-sitter.json` of `tree-sitter-postgres` lists `postgres`
and not `plpgsql`. Upstream runs the corpus of `plpgsql` in its own folder,
so the package of a grammar that `tree-sitter.json` does not list runs each
case of its corpus that names no grammar.

When `tree-sitter.json` lists a query file of another grammar, such as
`node_modules/tree-sitter-javascript/queries/highlights.scm`, the package
copies it from the checkout of that grammar, at the tag that
`grammars/grammars.json` records for it, into `queries/<grammar>/` (D84).

Do not edit a file that `transit generate` writes, and do not edit a copied
file. To change a generated file, change the generator. To change a copied
file, update the grammar to a new tag.

## The external scanner

A grammar can have an external scanner, `src/scanner.c`. It is C code, written
by hand, that recognizes the tokens that a regular expression cannot, such as
the indentation of Python. The grammar lists these tokens under `externals`.

Port the scanner by hand. The rules are the same as the rules for the runtime
in [`UPSTREAM.md`](UPSTREAM.md), under "How Go code follows C code", and
these rules add to them:

1. Port the file line by line into idiomatic Go (D24). Keep the order of the
   functions, the names of the functions in Go form, and the comments.
2. The doc comment of the scanner type names the upstream file, the tag and
   the commit: `scanner is a port of src/scanner.c of tree-sitter-python at
   v0.23.6 (<commit>).`
3. The C `enum TokenType` becomes Go constants in the same order. The order
   must be the order of `externals` in `grammar.json`, because the runtime
   uses the index.
4. The five C functions become one Go type:

   | C function | Go |
   | --- | --- |
   | `tree_sitter_<name>_external_scanner_create` | `newScanner`, which returns the scanner |
   | `tree_sitter_<name>_external_scanner_destroy` | nothing, unless it does more than free memory |
   | `tree_sitter_<name>_external_scanner_scan` | the method `Scan` |
   | `tree_sitter_<name>_external_scanner_serialize` | the method `Serialize` |
   | `tree_sitter_<name>_external_scanner_deserialize` | the method `Deserialize` |

5. The scanner calls the lexer through the Go form of each member of the C
   `TSLexer`: `lookahead`, `result_symbol`, `advance`, `mark_end`,
   `get_column`, `is_at_included_range_start`, `eof` and `log`.
6. `Serialize` writes the same bytes as the C function, in the same order.
   Where the C code copies an integer with `memcpy`, the Go code writes it in
   little-endian order, which is the order of every platform that CI tests.
   Where the C code copies a whole struct, the comment of `Serialize` gives
   the offset of each field. The result must be no longer than 1024 bytes,
   which is `TREE_SITTER_SERIALIZATION_BUFFER_SIZE`.
7. The scanner keeps no state in a package variable. A C `static` variable
   that the scanner changes becomes a field of the scanner type. A C `static
   const` table becomes a package variable that nothing writes.
8. The C `Array(T)` of `tree_sitter/array.h` becomes a Go slice.
9. `Deserialize` gets the bytes that `Serialize` wrote, or none. Port each
   length test of the C function. Where C reads past the end of the buffer,
   the Go function stops at the end, a byte past the end reads as 0, and
   bytes after the state are ignored. A C `assert` has no Go form. So no
   buffer makes a Go scanner panic (D85). A fuzz test calls `Deserialize`
   with random bytes.
10. A C function of `<wctype.h>` or `<ctype.h>`, such as `iswspace`,
    `iswalpha` or `towupper`, becomes the function of the package
    `internal/wctype` with the name of the C function, such as
    `wctype.Iswspace` (D46). Released code answers as the package `unicode`
    does (D39). The test module sets the package to the C locale, where it
    answers as the C library does, so that the C scanner and the Go scanner
    give the same tokens. `TestCharFuncsMatchC` measures each function for
    every code point and lists how `unicode` differs from C. If a scanner
    needs a function that the package lacks, add it there and to both files
    `wctype` of `test/cgrammar`.
11. A C `char` is signed, as on x86-64, where the golden files are made and
    the scanners are compared with C. A byte above 0x7f that the C code
    widens becomes a negative number (D85).
12. A C header that the scanners of two grammars of one repository share,
    such as `common/scanner.h`, becomes the package `internal/scan` of the
    module (D85).
13. A typed constant of a scanner needs no `String` method. An assignment of
    C that nothing reads, and that a linter reports, is left out, with a
    comment that says so (D85).
14. Where the `deserialize` of C frees an array and allocates it again, the
    Go port keeps its slice and sets its length to 0 (D96).

A scanner is correct when two things are true. The corpus tests pass. And the
test module (D12) shows that the C scanner and the Go scanner give the same
tokens and the same serialized bytes for every corpus input. The test file
`test/cgrammar/gopackage_<package>_test.go` of the package registers it and
calls `compareScanners`, which compares every call of the two scanners.

## Grammars that xo writes

`xo` writes a grammar only when no grammar exists that usql can use (D42).
Each one follows these rules, in addition to the rest of this document:

1. It is a normal tree-sitter grammar: a `grammar.js`, and a `src/scanner.c`
   if it needs one. The upstream tool makes its `grammar.json`, and the golden
   harness makes its golden files, as for any grammar.
2. It lives in `grammars/<name>`, beside the modules of other repositories,
   with a `tree-sitter.json` that gives its version (D104). Its name is not
   the name of another module. It has an entry in `grammars/grammars.json`
   like any other, with the repository `github.com/xo/transit` and the path
   of its folder.
3. It has a corpus in `test/corpus/` and a `queries/highlights.scm`, written
   with the grammar.
4. Its Go scanner is a port of its own `scanner.c`, under the rules of "The
   external scanner" above. The C scanner stays the reference.
5. Its license is the license of transit.
6. A grammar that embeds another language, as the usql grammar embeds SQL,
   does it with an injection query, `queries/injections.scm`, and not by
   copying the rules of the other grammar (D13, D27).

A change to such a grammar is a change to transit. It follows "Before you
stage" in `AGENTS.md`, and its golden files are made again.

The folder `grammars/<name>` is the module of the grammar, and it is laid
out as the checkout of a grammar repository and as a grammar module at once.
`grammars/usql` holds one grammar, `usql`, at the root of the module. Its
package is `usql`. The grammar is one language for every SQL dialect, and
the options of the dialect change only its scanner (D108):

| Path | What it holds | Who writes it |
| --- | --- | --- |
| `go.mod`, `LICENSE`, `tree-sitter.json` | the module, the license of transit, and the grammar with its version | a person |
| `grammar.js` | the rules of the grammar | a person |
| `src/scanner.c` | the external scanner, which reads the options from the macro `USQL_OPTIONS` | a person |
| `queries/`, `test/corpus/` | the queries and the corpus of the grammar | a person |
| `grammar.json` | the grammar that the upstream tool makes from `grammar.js` | the golden harness |
| `parser.go`, `node-types.json`, `grammar_test.go` | the files of the grammar package | `transit generate` |
| `testdata/corpus/` | a copy of `test/corpus/` | copied |
| `scanner.go` | the port of `src/scanner.c` | a person |
| `options.go` | `Options`, the options of a dialect, and `LanguageFor`, which gives the language with a scanner that reads them | a person |
| `testdata/options/<name>.txt` | the corpus cases that need options other than the default, which `options_test.go` runs with the options `<name>` | a person |

The upstream tool, the golden harness and the corpus test use the default
options of `src/scanner.c`: dollar quotes and block comments. A new option
is one new field of `Options` and one new flag of `src/scanner.c`, and the
field and the flag are in the same order. The test module builds the C
scanner once for each set of options that it tests, with
`-DUSQL_OPTIONS=<n>`, and compares it with the Go scanner of
`LanguageFor` with the same options.

The set `xo` of the golden harness copies each module that xo writes and
that it runs into the cache, as part of the checkout of the repository
`xo/transit`. It replaces only the copies of the modules that it runs. It
finds such a module by its `grammar.js`, at the root of the module or in a
package folder, which no module of another repository holds (D104). The
cache is where the other sets and the test module find a grammar. The
upstream tool makes the `grammar.json` of each grammar from its
`grammar.js`, and the harness writes it into the folder of the package. The
entry of the grammar has no tag and no commit. After a change to a grammar,
run the harness on it, then generate its package again:

```bash
cd test && go run ./cmd/golden -set xo,corpus -only usql,xo/transit
cd .. && go run ./cmd/transit generate --backend go grammars/usql
```

## Tests

Every grammar has the same tests, and `transit generate` writes them in
`grammar_test.go`:

1. The corpus test. It parses each case in `testdata/corpus/` and compares
   the tree with the expected tree, or the CST of a case with `:cst`. The
   result of each case must be what `tree-sitter test` reports for the
   same grammar at the same tag. The golden harness records the names of
   the cases that fail upstream in the field `failing` of
   `grammars/grammars.json` (D79). At the end of each run, it writes them
   into `testdata/failing.txt` of each grammar package under `grammars/`,
   so the test expects them to fail and names them (D88). The file holds
   one name on each line, in the order of the record, and each line ends
   with a newline. A name that the record holds twice is on two lines. In
   a module with more than one grammar, the file of a package holds only
   the cases that the package runs (D93). The file has no blank line and
   no comment. If no case of the package fails upstream, the file does not
   exist. `grammar_test.go` holds no list of cases, so it is the same in
   every checkout and in the Go module cache. Each package of a module
   with more than one grammar holds the whole corpus. A case runs in the
   package of the grammar that its `:language` names, or of the first
   grammar of `tree-sitter.json`, and the other packages skip it (D83).
2. The query test. Each file in `queries/` must compile with the transit query
   engine.
3. The highlight test, if the grammar has `test/highlight/`. transit does not
   port the upstream highlighter (D7), so the test compares the captures of
   the highlight queries with the assertions in each file, as the highlighter
   of upstream finds them (D80). It reads each file that `tree-sitter.json`
   lists under `highlights` and `injections`, in order. A file of another
   grammar, `node_modules/tree-sitter-<g>/queries/<f>`, is read from
   `queries/<g>/<f>` of the package (D84). When two patterns capture one
   node, the last one counts. The test builds the injection layers with the
   package `inject`, and checks each assertion against the deepest layer.
   Each capture name of the highlight queries is also in the list of
   captures of the package `styles`, and it reaches an entry of each bundled
   style (D65). The package `styles` exists, and that part of the test is
   not written yet.
4. The test of the node types and the test of the keywords. Each type of
   `node-types.json` and each keyword is a symbol of the language.
5. The generator test, as below.

The package `internal/grammartest` holds the code of each test, so
`grammar_test.go` holds one call for each test. A test uses no cgo, so a
grammar module stays pure Go (D1).

The test module (D12) also parses each corpus input
with the C grammar and the C runtime, and it compares the two trees node by
node.

### The generator test

The golden harness runs the upstream tool at the commit that transit ports,
and it writes `parser.c` and `node-types.json` from the same `grammar.json`.
The transit generator, with its C backend (D8), must write the same two files,
byte for byte.

A `parser.c` of a large grammar can be tens of megabytes, so a grammar package
does not hold it. `grammars/grammars.json` holds the SHA-256 of each upstream
file, and the test compares the hash. If the hashes differ, run the upstream
tool on your machine and compare the files with `diff` to find the difference.
D40 holds this.

The same run of the generator writes the files of the Go package again, and
the test makes sure that they are the files of the package. If they differ,
generate the package again. The test finds `grammars/grammars.json` in a
folder above the package. In a checkout of transit, it also makes sure that
`testdata/failing.txt` is the file that the golden harness writes from the
record (D88). If the file differs, run the golden harness again. The test
skips the hashes and `failing.txt` when the package is not in a checkout of
transit. It skips the whole test with `go test -short`, because the
generator takes minutes for a large grammar.

## Steps to add a grammar to the set

1. Find the release tag. Use the latest release of the grammar, or the tag
   that `fixtures.json` names for a fixture grammar.
2. Clone the grammar outside this repository:

   ```bash
   git clone --depth 1 --branch <tag> <repository> /tmp/tree-sitter-<name>
   ```

3. Read its license. If it is not MIT, Apache 2.0 or a BSD license, ask Ken
   before you go on.
4. Read `tree-sitter.json`, and find each grammar and its `path`.
5. Add an entry for each grammar to `grammars/grammars.json`.
6. Run the golden harness on the grammar. It builds the upstream tool at the
   commit that transit ports, generates the grammar from its committed
   `src/grammar.json`, and writes the SHA-256 of `parser.c` and
   `node-types.json` to the entry. If the repository commits no
   `src/grammar.json`, the harness installs the npm packages that `grammar.js`
   requires, at the versions in its `package.json`, runs `grammar.js`, and
   keeps the `grammar.json` that the tool writes (D51). It installs the
   packages with `npm install --ignore-scripts`, so that no install script of
   a package runs.
7. Run the generator test of the grammar, and the corpus of its C output in
   the test module.
8. Search the query files of the grammar for the predicates and directives
   of Neovim that [`NEOVIM.md`](NEOVIM.md) lists. If a query file that
   transit reads uses one, add the grammar to the table of `NEOVIM.md`, and
   mark it with [N] in `CANDIDATES.md`.
9. Stage the change, and give Ken a commit message of this form:

   ```text
   grammars: add tree-sitter-<name> <tag> to the set
   ```

   Do not commit. Ken commits (D2).

The golden harness does step 6 for every grammar of `CANDIDATES.md`. After
you add a grammar to `CANDIDATES.md`, run it on the repository of the
grammar:

```bash
cd test && go run ./cmd/golden -set candidates -only <owner>/<repository>
```

Then run the generator test on every recorded grammar. The fixture grammars
run without the flag:

```bash
go test ./generate/backend/c -run Recorded -args -all-grammars
```

## Steps to add a Go package

These steps start after the gate of D9, for a grammar that is in the set and
that Ken chose.

1. Clone the grammar at the tag in its entry, outside this repository.
2. Make the folder of the module and write its `go.mod`. It requires the
   module `github.com/xo/transit`. Run `./gen.sh -m`, which writes its
   `replace` block (D49).
3. Copy `LICENSE`, `grammar.json`, `tree-sitter.json`, `queries/` and the
   folders of `test/` into the layout above. Take `grammar.json` from the
   golden harness if the repository does not commit it.
4. Generate the package:

   ```bash
   go run ./cmd/transit generate --backend go grammars/<name>
   ```

   If the entry of the grammar in `grammars/grammars.json` names corpus
   cases that fail upstream, run the golden harness once more. It writes
   `testdata/failing.txt` of the new package, as "Tests" below says.

5. If the grammar has `src/scanner.c`, port it to `scanner.go`, as "The
   external scanner" says.
6. Run the tests of the grammar, from the folder of its module:

   ```bash
   go test -race -count=1 ./...
   ```

7. Add the grammar to the list of grammars in the root `README.md`.
8. Stage the change, and give Ken a commit message of this form:

   ```text
   grammars/<name>: add the Go package for tree-sitter-<name> <tag>
   ```

   Do not commit. Ken commits (D2).

## Steps to update a grammar

1. Find the new tag.
2. Clone the grammar at the old tag and fetch the new tag:

   ```bash
   git clone --branch <new tag> <repository> /tmp/tree-sitter-<name>
   git -C /tmp/tree-sitter-<name> fetch --depth 1 origin tag <old tag>
   ```

3. Read the changes of the scanner:

   ```bash
   git -C /tmp/tree-sitter-<name> diff <old tag> <new tag> -- src/scanner.c src/*.h
   ```

4. Port each change to `scanner.go`, and change the tag and the commit in
   its doc comment.
5. Copy `LICENSE`, `src/grammar.json`, `queries/` and the folders of `test/`
   again. Delete a copied file that the new tag no longer has.
6. Change `tag`, `commit`, `parser_c` and `node_types` in
   `grammars/grammars.json`.
7. Run the golden harness on the new tag. If the grammar has a Go package,
   generate it again and run its tests. Stage the change, as the steps above
   say. The commit message has this form:

   ```text
   grammars/<name>: update tree-sitter-<name> to <tag>
   ```

## When the generator changes

A change of the transit generator can change every `parser.go`. After each
such change, generate every grammar package again, write the new upstream
hashes, and run the tests of every grammar. Stage all of it in the same change
as the generator. [`UPSTREAM.md`](UPSTREAM.md) says when an upstream commit
changes the generator.
