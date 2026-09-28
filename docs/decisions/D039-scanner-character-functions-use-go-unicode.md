# D39. Scanner character functions use Go's unicode package

Status: Decided, amended by D46.

Ken decided on 2026-09-29, answering questions 19 and 42, that a C function
of `<wctype.h>` or `<ctype.h>` in a scanner, such as `iswalpha` or
`iswspace`, becomes a Go function that uses the package `unicode`.

The C scanners run in the `C` locale of the GNU C library, because the
upstream tool does not call `setlocale`. There, these functions answer as for
ASCII. So a Go scanner can answer differently from its C scanner for a
character that is not ASCII.

The test module measures each C function for every code point. Where the C
function and the Go function differ, the difference is expected, and the test
lists it for each function. Any other difference between a C scanner and a Go
scanner is a fault.
