# D47. Phase 3 measures with a prototype of the Go output

Status: Decided, amends D31 and D37.

Ken decided on 2026-09-29, answering question 46, that phase 3 writes a
prototype of the lexer and the tables that the Go backend will write, for the
JSON grammar and one large grammar, and measures with it:

1. The compile time of the tables as Go literals and as embedded data (D31).
2. The speed of the lexer as code and as data (D31).
3. The speed targets of D37.

A Go lexer does not exist before the Go backend of phase 4, and a C lexer
that the Go runtime calls through cgo gives numbers that do not show the
speed of Go. The prototype is not kept. Phase 4 confirms the targets again on
the real Go grammars.

D73 records the measurements.
