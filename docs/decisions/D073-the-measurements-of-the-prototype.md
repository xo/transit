# D73. The measurements of the prototype of phase 3

Status: Decided.

Ken decided on 2026-09-30 that this decision records the measurements of
the prototype of D47. The prototype is not kept (D47). It was a scratch
module outside the repository with these parts:

1. A Go backend, made from a copy of `generate/backend/c/render.go`. It
   writes a Go package from `RenderInput`, with the same symbol numbers and
   tables as the C backend. It never reads the text of a `parser.c`.
2. The external scanner of postgres, ported to Go.
3. Benchmarks against the C runtime.

## The machine and the inputs

The machine is an AMD Ryzen 9 9950X with 32 threads and 60 GB of memory, on
Linux, with Go 1.27.1. The grammars are json of `tree-sitter/tree-sitter-json`
at `v0.24.8`, and postgres of `gmr/tree-sitter-postgres` at `v1.2.4`. The
parser.c of postgres is 95 MB, the largest of the set. postgres has 16,991
parse states, 1,261 symbols, 89 main lex states and 1,996 keyword lex states.

The inputs of the speed targets are one SQL statement of 10 KB, and of 5, 20
and 40 KB, and a JSON text of 10 KB. The statement is a SELECT with a long
column list, six joins, a long WHERE clause, a GROUP BY and an ORDER BY. It
parses with no error. The benchmarks of the lexer use 1 MB of text.

## The checks

Each of the four outputs matches C:

1. Each table field of `abi.Language` is equal to the table that the test
   module copies from the C grammar.
2. The Go lex function and the C one give the same result, the same symbol
   and the same calls of `Advance` and `MarkEnd` at each position of the
   texts: 626,000 calls for json, and 10.5 million for postgres.
3. The trees are equal to the C trees for every corpus input, and for 100 KB
   of JSON and 200 KB of SQL.

The Go scanner of postgres gives the same trees as the C scanner for 90
inputs, and the same calls for 20,232 inputs. It keeps a fault of the C
scanner: a character of a dollar-quote tag keeps only its low byte.

## The generator

The generator takes 1 minute 58 seconds for postgres, with a peak of 10.9 GB
of memory. The C backend takes the same time. Writing a Go output takes less
than 0.4 seconds more.

## The form of the tables and the lexer (D31)

The tables are Go literals, or data that the package embeds and decodes on
the first call of `Language`. The lexer is code, a Go function with the
structure of the C lex function, or data, a table of character ranges for
each state with one small interpreter.

| Output | Go source | Embedded data | Build of the package, cold | Binary |
| --- | --- | --- | --- | --- |
| json, literal tables, lexer as code | 21.6 KB | none | 0.03 s | 2.40 MB |
| json, literal tables, lexer as data | 19.5 KB | none | 0.03 s | 2.40 MB |
| json, embedded tables, lexer as code | 12.8 KB | 2.4 KB | 0.04 s | 2.41 MB |
| json, embedded tables, lexer as data | 6.8 KB | 4.5 KB | 0.04 s | 2.41 MB |
| postgres, literal tables, lexer as code | 30.8 MB | none | 4.99 s, 2.0 GB | 17.6 MB |
| postgres, literal tables, lexer as data | 30.7 MB | none | 4.92 s, 2.5 GB | 17.5 MB |
| postgres, embedded tables, lexer as code | 276 KB | 14.6 MB | 0.25 s, 105 MB | 17.2 MB |
| postgres, embedded tables, lexer as data | 7.7 KB | 14.7 MB | 0.08 s, 54 MB | 17.1 MB |

A cold build of the package is a build cache that holds the runtime and not
the package. A build with an empty cache adds about 1.5 seconds. A warm build
takes 0.03 seconds. The binary is a small program that uses the package. A
program with the runtime alone is 2.40 MB.

Literal tables need no work at run time. The compiler stores them as static
data, and the first call of `Language` takes 1.5 microseconds. Embedded
tables are decoded on the first call: 4 ms for postgres, with a second copy
of 15 MB on the heap.

| Lexer, 1 MB of text | json | postgres |
| --- | --- | --- |
| Lexer as code | 116 to 123 MB/s | 54 to 58 MB/s |
| Lexer as data | 120 to 127 MB/s | 67 to 71 MB/s |

The form of the tables does not change the speed of a parse. The lexer takes
about a tenth of the time of a parse of postgres.

## The download of a module

A grammar module holds one grammar repository (D26), so the limit of 500 MB
applies to each repository. The Go source of postgres with literal tables is
31 MB, and its embedded form is 15 MB. The repository of postgres holds 93 MB
of parser.c, more than any other repository of the set, so no grammar module
comes near the limit.

## The speed targets (D37)

These numbers use the Go lexer and the Go scanner, with literal tables and
the lexer as data. The C runtime parses the same input with the C grammar.

| postgres | 5 KB | 10 KB | 20 KB | 40 KB |
| --- | --- | --- | --- | --- |
| First parse, C | 0.55 ms | 1.08 ms | 2.15 ms | 4.17 ms |
| First parse, Go | 0.83 ms | 1.68 ms | 3.15 ms | 6.08 ms |
| Parse after one key, C | 19 µs | 24 µs | 43 µs | 82 µs |
| Parse after one key, Go | 19 µs | 19 µs | 36 µs | 76 µs |
| Key and highlight query, Go | 0.56 ms | 1.15 ms | 2.23 ms | 4.35 ms |
| Allocations of a key, Go | 200 | 231 | 399 | 805 |

For json at 10 KB, a first parse takes 0.82 ms in C and 1.35 ms in Go, and a
parse after one key takes 43 µs in C and 34 µs in Go.

1. Target 1 holds. A parse after one key and the highlight query take
   1.15 ms on the statement of 10 KB. The query runs over the whole tree, and
   it takes most of the time. rline runs it on the rows that it shows
   (`RLINE.md`), which takes less.
2. Target 2 holds. A first parse runs at 0.64 of the speed of C for postgres
   and at 0.61 for json.
3. Target 3 does not hold. The allocations of a key grow with the size of
   the statement. The time of C for a key grows the same way, because the
   parse after an edit in a long statement redoes more of it. Most of the
   allocations are stack nodes. `stack.go` leaves out the free list of stack
   nodes of C, because the garbage collector frees them. Ken decided to keep
   the target as it is, and phase 4 works on it.

## A fault that the measurements found

`TreeCursor.gotoSiblingInternal` took the function that advances its child
iterator as a parameter, as C does. The call through a function value moved
the iterator to the heap, so each move of a tree cursor allocated. It was 97
percent of the allocations of the highlight query: 16,760 on the statement
of 10 KB. The function now takes a bool that picks the direction, and the
highlight query allocates nothing. A test in `tree_cursor_test.go` keeps it
so.
