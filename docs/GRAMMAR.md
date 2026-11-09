# Adding a grammar

This document holds the rules for adding a grammar to transit, for porting its
external scanner, and for updating it to a new release. A grammar describes
one language, such as JSON or Go. In transit, a grammar is a Go package that
exports one function, `Language`, which returns a `*transit.Language`.

The rules were written on 2026-09-29, before any code existed (D3). Today
`transit generate` works with the C backend, and the golden harness runs it.
`transit generate --backend go` writes a Go package. It wrote the 34 grammar
packages in the 31 modules under `grammars/`. Each rule names the decision
that it comes from.

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
it, and the queries and the injections use it. The grammar `cypher` lives in
`grammars/neo4j`, so its package is `neo4j`, and `Language.Name` gives
`cypher`.

If the language of a grammar is the language of one dialect of dbmeta, the
folder of its module has the name of the dialect. The table of D107 lists
them. The SQL Server grammar that xo writes lives in `grammars/sqlserver`,
the Oracle grammar that xo writes lives in `grammars/oracle`, the SQLite
grammar that xo writes lives in `grammars/sqlite3`, the DuckDB grammar
that xo writes lives in `grammars/duckdb`, the ClickHouse grammar that
xo writes lives in `grammars/clickhouse`, and the Trino grammar that xo
writes lives in `grammars/trino`. The folder of
any other module has the name of the upstream repository without the `tree-sitter-` prefix and with each `-`
removed. `tree-sitter-embedded-template` becomes `embeddedtemplate`. If two
grammars or two repositories get the same name, ask Ken.

The two repositories `tree-sitter-sql` give the same folder and the same
grammar name, `sql`, and so does the generic SQL grammar that xo writes.
`tree-sitter.json` of `grammars/sql` names `xo/transit`, so the package
`sql` takes the entry of xo (D143). `DerekStride/tree-sitter-sql` and
`m-novikov/tree-sitter-sql` get no Go package (D106, D143). Two entries of
`grammars/grammars.json` can give one module folder. Then the tests and the
golden harness take the entry of the repository that `tree-sitter.json` of
the module names under `metadata.links.repository`. The same rule holds
for a grammar that xo writes in place of the grammar of another
repository. The grammars `postgres` and `plpgsql` of
`gmr/tree-sitter-postgres` and the grammars that xo writes have the same
names. `tree-sitter.json` of `grammars/postgres` names `xo/transit`, so the
packages `postgres` and `plpgsql` take the entries of xo (D122). The
Oracle grammar that xo writes has the name `oracle`, and the grammar of
`andreasmaierde/tree-sitter-plsql` has the name `plsql`, so each has one
entry, and only `oracle` has a Go package (D124). In the same way, the SQL
Server grammar that xo writes has the name `sqlserver`, and the grammar of
`Crary-Systems/tree-sitter-tsql` has the name `TSQL`, and only `sqlserver`
has a Go package. The SQLite grammar that xo writes has the name of its
dialect, `sqlite3` (D126). The archived repository
`dhcmrlchtdj/tree-sitter-sqlite` is not in the golden set. The DuckDB
grammar that xo writes has the name of its dialect, `duckdb`, and the
ClickHouse grammar that xo writes has the name of its dialect,
`clickhouse`, and the Trino grammar that xo writes has the name of its
dialect, `trino` (D127).

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
| `grammars/json/parser.go` | `Language`, the constants, `Queries`, `NodeTypes`, `Keywords`, the tables and the lexer. The dense parse table is a string constant, and the other tables are Go literals (D121) | `transit generate` |
| `grammars/json/node-types.json` | the node types, which `parser.go` embeds | `transit generate` |
| `grammars/json/grammar_test.go` | the tests of "Tests" below, which call the package `internal/grammartest` | `transit generate` |
| `grammars/json/scanner.go` | the external scanner, if the grammar has one | a person, as a port |
| `grammars/json/example_test.go` | `ExampleLanguage`, the example that pkg.go.dev shows for `Language`, as "The example" below says (D115) | `transit generate` |
| `grammars/json/example_more_test.go` | more example functions, if a person writes them, as in `json` and `html` (D53, D115) | a person |
| `grammars/json/queries/*.scm` | `queries/*.scm` of the upstream repository | copied |
| `grammars/json/queries/highlights.scm` | the highlight query, if the upstream repository has none, as in `cql` (D113) | a person |
| `grammars/json/highlight_test.go` | a test that makes sure of the capture name of each word that it names in a text, if the grammar has no comment node to hold the assertions of `testdata/highlight`, as in `cql` | a person |
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

If the upstream repository of a grammar has no highlight query, xo writes
`queries/highlights.scm` of the package (D113). The first line of its
comment says that xo wrote it. It uses only the capture names of
`styles/captures.txt`. After you add the file, generate the package again,
so that `parser.go` embeds `queries/`. A grammar can hide a keyword in a
token that a query cannot capture. Then the
query captures the node that holds the token, and the patterns of its
children color the rest of the node.

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
   copying the rules of the other grammar (D13, D27). The grammar `plpgsql`
   reads each expression and each SQL statement of a body as one token, and
   its injection query gives the token to the grammar `postgres`. The
   injection query of `postgres` gives the body of a function or of a `DO`
   block to `plpgsql` or to `postgres`, as its `LANGUAGE` says. Oracle
   does not put PL/SQL in a quoted string, so the Oracle grammar reads SQL
   and PL/SQL in one grammar, and it has no injection query (D124). The SQL
   Server grammar reads the procedural statements of Transact-SQL in the
   same way.
7. A SQL grammar keeps its parse tables small (D122, D123). The token
   `identifier` is the word of the grammar, so one token covers each
   keyword that the language does not reserve. The reserved words of the
   language are in a reserved set of tree-sitter. A parse state with more
   than 64 columns takes the large form, which holds a column for each
   symbol of the grammar. The comment at the top of
   `grammars/mysql/grammar.js` gives the ways that keep the states few and
   small. The Oracle grammar reads a long list of options, such as the
   physical options of a table, with one identifier that takes an empty
   reserved set, as the comment at the top of `grammars/oracle/grammar.js`
   says. Then a reserved word in the list does not give a large state of
   its own. The SQL Server grammar reads the reserved words that name
   functions, such as `LEFT` and `CURRENT_USER`, with one token for each
   group of them, and not with one keyword for each word. Then each state
   where an expression starts has fewer than 64 columns, and it does not
   take the large form.
   The SQLite grammar takes the operators of each level of precedence
   from one token, as the SQL Server grammar does. It reads `TRUE` and
   `FALSE` as one token, and `CURRENT_DATE`, `CURRENT_TIME` and
   `CURRENT_TIMESTAMP` as one token.
   The DuckDB grammar reads the `%` of `LIMIT 10%` with the token of the
   operators `*`, `/` and `%`. A token of its own for `%` cost more than
   400 states, because the states after the count of `LIMIT` could then
   not merge with the states after another expression. The grammar takes
   the clauses after the arguments of a function, such as `FILTER` and
   `OVER`, and the clauses of a window, in any order, as it takes the
   clauses of a query. Each set of optional clauses in a sequence costs
   states, and the two rules cost about 340 states as sequences.
   The upstream tool makes one production for each choice and for each
   optional part of a rule, and the states after such a part are copied for
   each production. So when the first parts of a statement of the DuckDB
   grammar hold a choice or an optional part, they are a hidden rule of
   their own, such as `_insert_target` and `_trigger_head`. The parts of
   `CREATE TRIGGER` before its body cost 283 states in one rule, and 16 as
   a hidden rule. The options of a sequence take expressions, and an
   expression before `INCREMENT` costs 366 states. The grammar reads `WITH
   DATA` and `WITH NO DATA` after the query of `CREATE TABLE`, and they cost
   542 states. The upstream tool does not merge the states after an
   expression where `WITH` can come with the states where `WITHIN` can
   come. A token of its own for `WITH DATA` costs more. Ken accepted both
   costs (D129). Together they add 908 states, 17 of them large, and
   about 160 KB to a binary.
   The path of `ATTACH` can be an expression, and the options in
   parentheses can come after it, as in `ATTACH 'file.db' (READ_ONLY)`. A
   name before `(` would then read as a column or as the name of a
   function, and the tool reports a conflict. So the path is a string, a
   parameter, a function call or an expression in parentheses, or such
   values with an operator such as `||` between them. `ATTACH` costs 92
   states. `EXPLAIN` and `PREPARE`, which take any statement, cost 24
   states together, because the states of the statement after them are the
   states of a statement alone. `UPDATE EXTENSIONS` is a statement of its
   own, so `EXTENSIONS` after `UPDATE` is a keyword, and `UPDATE` also
   takes the keyword as the name of a table.
   A statement of Transact-SQL does not need a semicolon at its end. So
   each word that starts a statement can come after each expression, and
   each state after an expression would hold about 40 such words. More
   than 900 states took the large form. The external scanner of the SQL
   Server grammar gives a token that holds no characters, the end of a
   statement, where the grammar can end a statement and the next word
   starts a statement. Then a state after an expression holds this token
   in place of the words, and 18 states take the large form. The comment
   at the top of `grammars/sqlserver/src/scanner.c` gives the rules of the
   token.
   The parser cannot merge two states when only one of them takes a token
   of the external scanner. So a statement of PL/SQL ends at a `;` or at a
   `/` alone on its line, as a statement of SQL does, and the states after
   them are the same.
   The upstream tool merges two states that reduce the same rule in two
   places, and the merged state takes the reserved words and the keywords
   of both places. So a word of a list of options can be read as a
   keyword after a name or after another word of the list, and the parse
   fails. The SQL Server grammar reads its lists of words with a rule that
   is right recursive, and its parts are inlined, so that each word is
   read in a state that expects a word of the list. A list ends a
   statement or comes in parentheses, so that no keyword comes after a
   list. A name before a list has a rule of its own, as the comments of
   `_options` and `_object_name` in `grammars/sqlserver/grammar.js` say.
   The parts of a procedure, a function or a trigger before its body are
   a hidden rule of their own in the SQL Server grammar, and so are the
   body and the events of a trigger. Each optional part of a rule makes
   one more production of the rule. Without the hidden rules, the parse
   states that start the body were copied for each production, and more
   than 200 states took the large form.
   The upstream tool also does not merge two states when one of them takes
   a keyword that starts with another keyword of the other state, such as
   `ORDER` and `OR`, `INTERSECT` and `IN`, or `ASC` and `AS`. The two
   keywords are both read by the word rule, but the tool counts them as a
   conflict. So the states after an expression are copied for each place
   that takes a different set of such keywords after the expression. A
   word that a token with spaces starts, such as `INNER` in a token for
   `INNER JOIN`, is not a keyword of the word rule at all. Then the states
   split more often. The SQLite grammar reads `INNER JOIN` as two keywords
   for this reason, so that `IN` stays a keyword, and it has 73 fewer
   states.
   The ClickHouse grammar reads each word before `JOIN`, such as `GLOBAL`,
   `ANY`, `ASOF` and `LEFT`, as a keyword, in any order. One token for the
   words and `JOIN` took `AS`, `IN`, `INNER` and `GLOBAL` out of the
   keywords of the word rule, and the grammar had 908 more states. The
   keywords put 120 more states in the large form, but the tables are
   about 10% smaller. An alias with `AS` can come after any expression in
   ClickHouse, as in `(1 AS x) + x`, and it costs about 300 states. The
   white space of Unicode in `extras` cost more than 1,000 states, because
   an identifier can hold such a character, so the grammar reads only the
   white space of ASCII. A comma after the last column of `SELECT` is part
   of the rule of `SELECT`, with the `FROM` after it, so that only `FROM`
   can come after the comma, and it costs 8 states (D131). The grammar
   reserves no word (D131), and that saves 13 states. `ORDER BY` in a
   window costs about 630 states, because the states after its items do
   not merge with the states after an expression where `INTERSECT` can
   come next.
   The query of `CREATE TABLE ... AS SELECT` comes after the other
   clauses of `CREATE`. In the choice of the clauses, it cost about 940
   states, because each state of the query then took each word that
   starts a clause. A clause of `CREATE` that ends with an expression,
   such as `ORDER BY`, has a precedence above the alias with `AS`, so that
   `AS` after it starts the query. `SETTINGS` and the opening parenthesis
   of the settings of a column as one token cost about 620 states, because
   `SETTINGS` was then not a keyword of the word rule. So `SETTINGS` after
   a column of `ALTER TABLE` is a conflict that the parser solves at run
   time: the settings of the column, as in `SETTINGS
   (max_compress_block_size = 1024)`, or the settings of the statement.
   The data of `INSERT` after `FORMAT` is a token of the external scanner
   that runs to the end of the line, as the client of ClickHouse reads it,
   so such an `INSERT` needs no semicolon after it.
   The ClickHouse grammar reads the forms of D132, each in the cheapest
   form that the measurements found. `COMMENT` after the query of `CREATE`
   costs 8 states, because the states after the query merge with the
   states of a query alone. So `COMMENT` after an expression of any query
   is the keyword. The settings before
   the query of `INSERT` take a literal or a name as their values, as the
   settings of `EXPLAIN` do. They cost 13 states. With any expression as a
   value, they cost about 1,050 states. `ASC` and `DESC` in the
   parentheses of `CREATE INDEX` cost about 310 states. The states after an
   expression in the parentheses are copied for them.
   An alias with no `AS` takes the reserved set `implicit_alias`, the
   words that ClickHouse does not take as such an alias and `SELECT`
   (D132). The set costs 116 states. `SELECT 1 SELECT 2` then gives two
   statements, with a missing semicolon between them.
   A number where an identifier cannot come costs states in all the
   grammar. A number and an identifier can match the same text, such as
   `1e5`. The upstream tool then does not merge states that it merges
   otherwise. An interval of `REFRESH` can have more than one unit, as in
   `EVERY 1 HOUR 30 MINUTE`. With a number as the value of the second
   unit, it cost about 920 states. With a number or a name, it costs 4
   states.
   For the same reason, the seconds of `WITH REFRESH` of a live view take
   a number or a name. The value of a setting of a profile does too, and
   that saves about 570 states.
   Many statements of the third part, such as `DROP`, `SHOW`, `KILL` and
   `BACKUP`, end with the clauses of one hidden rule, `_tail`, in any
   order. The states after an expression are copied for each set of
   tokens that can come after it. One set for all these statements saves
   about 350 states. The options of the
   entities of access control, such as `CREATE USER`, are a choice of
   their own. In `_tail`, each state after an expression had more than 64
   columns, and 236 more states took the large form. The commands of
   `SYSTEM` and the forms of `SHOW` are many, so each statement takes its
   words in any order. The words that no other statement uses, such as
   `FLUSH` and `PROCESSLIST`, are one token of the word rule, the node
   `keyword`. One token in place of 90 keywords saves 88 symbols and about
   60 KB of the tables. `EXPLAIN` takes each statement but `OPTIMIZE`,
   because `optimize` names a setting of `EXPLAIN`.
   The ClickHouse grammar reads the forms of D133, each in the cheapest
   form that the measurements found. `PARALLEL WITH` between two
   statements costs 8 states, but 46 more states take the large form, and
   the tables grow by about 65 KB. One token for `PARALLEL WITH` cost 734
   more states, because the token and an identifier can match the same
   text. `AT TIME ZONE` and `AT LOCAL` cost 140 states, 49 of them large,
   and about 87 KB, because `AT` can come after each expression. Tokens
   for them cost 100 more large states. An alias with no `AS` can be
   `comment`, because the alias takes the keyword `COMMENT` too. After the
   query of `CREATE`, the parser tries the alias and the comment of the
   table at run time, and the string after `COMMENT` tells them apart. So
   a table, a table function, a subquery in `FROM` and `ARRAY JOIN` do
   not take their alias by precedence, but by a conflict that the parser
   solves at run time. This costs 70 states, 3 of them large. A token for
   `COMMENT` and its string cost about 900 states. `DESCRIBE` takes a
   query with no parentheses, and a set operation, with no clauses of
   `DESCRIBE` after it. A clause after the query, such as `FORMAT`,
   belongs to the query. This costs 6 states. With the clauses of
   `DESCRIBE` after any query, more than 400 more states took the large
   form.
   The Trino grammar reads `UESCAPE` and its string as a part of the token
   of a Unicode string, as in `U&'x' UESCAPE '#'`. As a keyword after the
   string, `UESCAPE` cost about 1,200 states, because the states after an
   expression were copied for it. The names of the functions of JSON,
   such as `JSON_VALUE`, are one token, and the type before `::` of a
   function of a type, as in `varchar::chr(65)`, is an inlined rule. Then
   each state where an expression starts has fewer than 64 columns. With a
   keyword for each name, 39 more states took the large form. Only `ROW`
   takes an argument with `AS`, as in `ROW(1 AS a)`. `AS` after any
   argument of any function cost about 370 states. The value of a session
   property of `WITH SESSION` is any expression, and it costs no states
   (D134). A name and `(` after it start a function call, as in Trino, so
   the body of the query after a name as the value cannot start with `(`.
   The version after `FOR TIMESTAMP AS OF` and `FOR VERSION AS OF` is any
   expression too (D134). It costs about 400 states and 52 KB of the tables,
   because an alias and the clauses of `FROM` can come after it, and the
   states after an expression are copied for them. The predicate `MATCH`, as
   in `r MATCH (SELECT a, b FROM t)`, costs about 320 states, 4 of them
   large. `MATCH` can come after each expression, and the upstream tool does
   not merge the states where `MATCH` can come with the states where
   `MATCH_RECOGNIZE` can come, so about 150 of the states come from
   `MATCH_RECOGNIZE`. `MATCH` binds more loosely than `OR` in the grammar,
   so that `MATCH` after the condition of `WHERE` in `NEAREST` starts the
   clause `MATCH` of `NEAREST`. The Trino grammar reads `WITH DATA` and
   `WITH NO DATA` only after the query of `CREATE TABLE AS` (D134). That
   query is a rule of its own, `_create_query`, which takes them among the
   clauses after the body of the query. They cost about 820 states and about
   100 KB of the tables. After any query, they cost about 580 states. The
   states after an expression where `WITH` can come do not merge with the
   states where `WITHIN` can come, as in the DuckDB grammar. An optional
   `WITH DATA` after the query costs 12 states more than the rule of its
   own, and one token for `WITH DATA` costs 36 states more. `GRANT`,
   `REVOKE`, `DENY` and `SHOW GRANTS` take the name of a kind of object that
   a connector defines before the name of the object, as in `GRANT SELECT ON
   kind t TO u`, and `ALTER kind t SET AUTHORIZATION u` is a statement of
   its own (D134). They cost 7 states. The other statements of the
   documentation cost about 440 states together. `CREATE TABLE`, `ALTER
   TABLE` and `SHOW` cost about 50 states each, and no other statement costs
   more than 40. The options of `EXPLAIN` are a hidden rule of their own, so
   that the statement after the options starts in one set of states. Each
   state where a statement starts takes the large form, and the hidden rule
   keeps 3 of them out. The values of `INSERT` in a clause of `MERGE` are
   one expression, a row value in parentheses. A list of expressions of
   its own cost about 40 states. `WITH` after a type can start `WITH TIME
   ZONE`, the properties of a column, a characteristic of a routine or the
   wrapper of `JSON_QUERY`. So the rule `type` has no precedence, and the
   parser tries both forms at run time.
   The generic SQL grammar reads the options of an object, such as the
   options of a table, of a stage or of `COPY INTO`, as a list of options.
   The statements that it does not read word by word, such as `SHOW`,
   `GRANT` and the `CREATE` of an object that is not a table, a view, an
   index or a function, read such a list too (D143). The rules of an item
   of a list are inlined, and the lists of `GRANT`, of `SHOW` and of the
   other statements have rules of their own. The parse states of a list
   take each keyword that can come after the list, and the upstream tool
   merges the states of one rule. With one rule for all the lists, `FROM`
   after the list of `REVOKE` was a keyword in `SHOW TABLES FROM db` too.
   The states that end an expression take each token that can come after
   an expression in any place of the grammar. Such a state of the generic
   grammar has about 60 columns. So the forms that end before a list of
   options or before the clauses of a table, such as the default value of
   a column, `CHECK`, the version after `FOR SYSTEM_TIME AS OF` and the
   arguments of `AT` and of a stage, have their own parentheses and their
   own function call, and they do not end with the rules of an expression.
   As any expression, the version of `AS OF` put 150 more states in the
   large form. The words that come only before `JOIN`, such as `INNER`,
   `CROSS` and `ASOF`, are one token, and so are `UNION`, `EXCEPT` and
   `MINUS`, `TABLESAMPLE` and `SAMPLE`, `AT`, `BEFORE` and `CHANGES`, and
   `SORT`, `CLUSTER` and `DISTRIBUTE`. Each token takes one column of the
   states after a table and after an expression. With a keyword for each
   word, about 150 more states took the large form. `DIV` of Databricks put 13
   states in the large form, so the grammar leaves it out. `PIVOT` and
   `UNPIVOT` cost about 310 states, `MERGE` about 350, `INSERT` about 270,
   the time travel about 140, and a comma after the last item of `SELECT`
   6 large states. The grammar has 5,398 parse states, 18 of them large.

A change to such a grammar is a change to transit. It follows "Before you
stage" in `AGENTS.md`, and its golden files are made again.

The folder `grammars/<name>` is the module of the grammar, and it is laid
out as the checkout of a grammar repository and as a grammar module at once.
`grammars/usql` holds one grammar, `usql`, at the root of the module. Its
package is `usql`. `grammars/mysql`, `grammars/oracle` and
`grammars/sqlserver` are laid out the same way. `grammars/sqlite3` is laid
out the same way, but it has no `src/scanner.c` and no `scanner.go`,
because the SQLite grammar needs no external scanner. `grammars/trino` is
laid out as `grammars/sqlite3`, because the Trino grammar needs no external
scanner either, and so is `grammars/sql`. `grammars/duckdb` and
`grammars/clickhouse` are laid out as `grammars/mysql`. `grammars/postgres`
holds the grammars `postgres` and `plpgsql` in the folders of their
packages, `grammars/postgres/postgres` and `grammars/postgres/plpgsql`. Each folder holds the same files as
`grammars/usql` but `options.go` and `testdata/options/`, and the
`tree-sitter.json` of the module lists both grammars. The usql grammar is
one language for every SQL dialect, and the options of the dialect change
only its scanner (D108):

| Path | What it holds | Who writes it |
| --- | --- | --- |
| `go.mod`, `LICENSE`, `tree-sitter.json` | the module, the license of transit, and the grammar with its version | a person |
| `grammar.js` | the rules of the grammar | a person |
| `src/scanner.c` | the external scanner, which reads the options from the macro `USQL_OPTIONS` | a person |
| `queries/`, `test/corpus/` | the queries and the corpus of the grammar | a person |
| `grammar.json` | the grammar that the upstream tool makes from `grammar.js` | the golden harness |
| `parser.go`, `node-types.json`, `grammar_test.go`, `example_test.go` | the files of the grammar package | `transit generate` |
| `testdata/corpus/` | a copy of `test/corpus/` | copied |
| `scanner.go` | the port of `src/scanner.c` | a person |
| `options.go` | `Options`, the options of a dialect, and `LanguageFor`, which gives the language with a scanner that reads them | a person |
| `testdata/options/<name>.txt` | the corpus cases that need options other than the default, which `options_test.go` runs with the options `<name>` | a person |
| `testdata/highlight/` | the highlight tests, with their assertions in `--` comments | a person |

The upstream tool, the golden harness and the corpus test use the default
options of `src/scanner.c`: dollar quotes, block comments and dash comments
(D139). A new option is one new field of `Options` and one new flag of
`src/scanner.c`, and the field and the flag are in the same order. The test module builds the C
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
   not written yet. If the first line of a file is `-- options: <name>`,
   the file needs the options `<name>` of the grammar, and the test skips
   it. The function `HighlightOptions` of `internal/grammartest` runs such
   a file with the language that the caller gives for `<name>`. The test
   `TestOptionsHighlight` of the usql grammar calls it (D137).
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
the test makes sure that they are the files of the package: `parser.go`,
`node-types.json`, `grammar_test.go` and `example_test.go`. If they differ,
generate the package again. The test finds `grammars/grammars.json` in a
folder above the package. In a checkout of transit, it also makes sure that
`testdata/failing.txt` is the file that the golden harness writes from the
record (D88). If the file differs, run the golden harness again. The test
skips the hashes and `failing.txt` when the package is not in a checkout of
transit. It skips the whole test with `go test -short`, because the
generator takes minutes for a large grammar.

### The example

`transit generate` writes `example_test.go` into each grammar package
(D115). Its function `ExampleLanguage` parses the input of a corpus case
and prints the tree with `Node.String`. Then it compiles each query file of
the package and prints the number of its patterns. Its block `Output` holds
what it prints, so `go test` runs the example and makes sure of the output.

The generator has no external scanner, so it does not parse the input. The
tree in the block `Output` is the expected tree of the case in the corpus,
and the corpus test makes sure that the package parses the same tree. An
expected tree can leave out the names of the fields, and `Node.String`
prints them. So the case is the first case of the corpus that meets all of
these conditions:

1. The case is not in `testdata/failing.txt`. It expects no error and has no
   attribute `:cst`.
2. The package runs the case (D83), and its input is not blank.
3. Its expected tree names its fields, or it holds no node of a type that has
   fields in `node-types.json`, and no node `ERROR`.

If no case meets the conditions, as in a package with no corpus, the
example only compiles the queries. The package `internal/corpus` reads the
corpus for the example and for the corpus test.

A hand-written example of a grammar package goes in `example_more_test.go`,
which the generator does not write.

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
