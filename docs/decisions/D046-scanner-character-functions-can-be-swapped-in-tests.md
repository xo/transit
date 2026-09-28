# D46. Scanner character functions can be swapped in tests

Status: Decided, amends D39.

Ken decided on 2026-09-29, answering question 45, that the Go scanners call
their character functions, such as the Go forms of `iswalpha` and
`iswspace`, through one small package. Released code uses the package
`unicode` (D39). The test module sets the package to the behavior of the GNU
C library in the `C` locale, so that a Go scanner and its C scanner give the
same tokens, and the Go tree and the C tree match exactly.

The test module still measures each C function for every code point and lists
each difference from the Go function (D39). Without this, a difference in one
character function changes a whole tree on input that is not ASCII, and the
tree tests cannot tell it from a fault.
