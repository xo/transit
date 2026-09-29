# D45. Go grammars are compared with C grammars at the Unicode version of upstream

Status: Decided, amended by D60.

Ken decided on 2026-09-29, answering question 44, that the tests that
compare a Go grammar with its C grammar use a test build of the Go grammar.
The generator makes the test build with the Unicode tables of `regex-syntax`,
16.0.0 on 2026-09-29, as the C backend test does (D38). A released grammar
package uses the tables of the Go package `unicode`, 17.0.0 on that date.

Without the test build, the Go tree and the C tree differ on each character
that Unicode 17.0 added, and such a difference hides a real fault.
