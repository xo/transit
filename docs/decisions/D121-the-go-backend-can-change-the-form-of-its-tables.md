# D121. The Go backend can change the form of its tables

Status: Decided, amends D74.

D74 chose Go literals for the tables that the Go backend writes. The
PostgreSQL grammar package has a `parser.go` of 510,701 lines, and 432,086 of
them are its dense parse table of 6.8 million `uint16` values. The compiler
is slow on such a literal and needs much memory, and CI ran out of memory on
it before D109.

Ken decided on 2026-10-31 that the Go backend can write its tables in another
form if the measurements show that it is better:

1. The new form must build faster or with less memory than the literals, or
   give a smaller binary.
2. It must not make a parse slower. The speed targets of D37, with target 3
   as D97 states it, must still hold.
3. It must not keep a second copy of a table on the heap, which the embedded
   form of D74 did.
4. The root module stays pure Go and imports only the standard library (D1,
   D11). The generated code stays idiomatic Go (D24).

The form that the measurements choose, and the measurements, are added to
this decision.

## The form that the measurements chose

Unit 36 measured the forms on 2026-10-31, on the machine of D73 with Go
1.27.2. The Go backend now writes the dense parse table, `ts_parse_table`,
as a string constant of the type `abi.Uint16Table`:

1. Each value is two bytes in little-endian order. `Uint16Table.At` reads the
   two bytes in place, so the table is static data and no copy of it is on
   the heap. The code uses no `unsafe`, so it reads the same values on a
   big-endian target, where the compiler joins the two bytes with one load
   that swaps them.
2. The bytes hold the value plus `0x2020`. Most values are 0 or small, so
   most bytes are printable ASCII, and the string literal holds them as one
   character, not as an escape of four. A 0 is two spaces.
3. A line of the constant holds the values of whole states, 8 KB of values
   or a little more. The
   compiler takes time in proportion to the square of the number of strings
   that a constant joins, so one line for each state made the build slower.
4. The small parse table stays a Go literal. The search of a small state
   reads each of its symbols, and that search was slower on a string.

The lint of a grammar module turns off `unqueryvet`. It reads each string
that a constant joins again for each join, and it took more than 11 minutes
on the parse table of postgres.

`Language` of the root package holds 8 bytes more than its tables, so that it
stays in the size class of 640 bytes of the Go allocator. In the size class
of 576 bytes, a first parse took 4 to 9 percent more time.

## The measurements

A cold build is a build of the package with a build cache that holds the
runtime and not the package. The memory is the peak of the build. The binary
is a small program that calls `Language`. A program with the runtime alone
is 2.42 MB. The first call of `Language` allocates 640 bytes once in each
form, and it takes 2 to 6 microseconds.

| Grammar | Form | `parser.go` | Lines | Cold build | Binary |
| --- | --- | --- | --- | --- | --- |
| json | literals | 23.8 KB | 546 | 0.45 s, 168 MB | 2.81 MB |
| json | string | 23.6 KB | 528 | 0.34 s, 168 MB | 2.81 MB |
| mysql | literals | 5.29 MB | 98,219 | 1.22 s, 361 MB | 5.34 MB |
| mysql | string | 4.32 MB | 42,022 | 0.64 s, 196 MB | 5.34 MB |
| sql | literals | 6.91 MB | 125,952 | 1.34 s, 455 MB | 5.53 MB |
| sql | string | 6.57 MB | 107,614 | 1.11 s, 391 MB | 5.53 MB |
| postgres | literals | 31.1 MB | 510,701 | 5.89 s, 2,520 MB | 17.88 MB |
| postgres | string | 21.6 MB | 81,324 | 1.08 s, 389 MB | 17.88 MB |

A cold lint of `grammars/postgres` as CI runs it took 121 s and 12.5 GB with
the literals, and 22 s and 1.6 GB with the string.

The speed of a parse is the median of runs of the old and the new runtime in
turns. The SQL statement and the JSON text are 10 KB. The benchmarks of D37
in `test/cgrammar` hold the Go runtime and the C runtime:

| Benchmark | Literals | String |
| --- | --- | --- |
| json, first parse, Go | 0.806 ms | 0.800 ms |
| json, parse after one key, Go | 22.1 µs | 22.0 µs |
| json, key and highlight query, Go | 0.845 ms | 0.847 ms |
| postgres, first parse, Go | 1.148 ms | 1.170 ms |
| postgres, first parse, C | 1.056 ms | 1.060 ms |
| postgres, parse after one key, Go | 28.3 µs | 28.4 µs |
| postgres, key and highlight query, Go | 1.199 ms | 1.190 ms |

A benchmark of the Go runtime alone parsed one SELECT statement with mysql,
sql and postgres. The first parse took 735, 1,117 and 1,094 µs with the
string, and 741, 1,137 and 1,103 µs with the literals. A parse after one key
took 61, 67 and 48 µs with the string, and 61, 68 and 49 µs with the
literals. The allocations did not change. So targets 1, 2 and 3 of D37 still
hold.

## The forms that the measurements did not choose

| Form | postgres `parser.go` | postgres cold build | Parse |
| --- | --- | --- | --- |
| Both parse tables as strings with no bias, one line for each state | 56.7 MB | 2.48 s, 363 MB | not measured |
| Both parse tables as strings with the bias, one line for each state | 21.2 MB | 2.28 s, 298 MB | not measured |
| Both parse tables as strings with the bias, lines of 8 KB | 20.9 MB | 0.77 s, 286 MB | 3 to 11 percent slower |
| The same, with a search of a small state that compares bytes | 20.9 MB | 0.77 s, 286 MB | 2 to 37 percent slower |

The two slower forms used the size class of 576 bytes for `Language`, so a
part of their cost came from it. The search of a small state took 10 percent
more time on a string in the profile of a parse after one key of postgres.

A compressed dense table did not help. 35 percent of the values of the dense
table of postgres are not 0. A comb vector, which packs the rows into one
vector with a check vector for an O(1) lookup, needs 1.10 times the size of
the dense table for postgres, 0.75 times for mysql and 0.61 times for sql. A
form that searches a row adds a search to each lookup of a large state,
which the second slower form above shows is too slow.

## The choices that Ken made

Ken decided on 2026-10-31:

1. The lint of a grammar module turns off `unqueryvet`, as D109 turned off
   three other linters. `unqueryvet` reads the whole string again at each
   `+` of the constant, and it ran for more than 11 minutes on postgres.
2. `Language` keeps its pad of 8 bytes, so that it stays in the size class
   of 640 bytes.
