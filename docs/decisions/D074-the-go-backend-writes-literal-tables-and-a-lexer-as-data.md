# D74. The Go backend writes literal tables and a lexer as data

Status: Proposed.

D31 leaves the form of the tables and of the lexer that the Go backend writes
to phase 4, and D73 records the measurements of phase 3. From them, this
decision proposes the form. Question 63 asks Ken to confirm it, and phase 4
checks it on the real Go grammars.

1. The tables are Go literals. For postgres, the largest grammar of the set,
   a cold build of the package takes 5 seconds and 2.5 GB of memory, and the
   tables need no work at run time. Embedded tables build in 0.08 to 0.25
   seconds, but the first call decodes them for 4 ms and keeps a second copy
   of 15 MB on the heap.
2. The lexer is data: a table of character ranges for each lex state, and one
   small interpreter. It was as fast as the lexer as code for json, and 25
   percent faster for postgres. Its Go source is also smaller.

The form of the tables does not change the speed of a parse. If a build of a
grammar package in phase 4 is too slow or too large, the embedded form is the
other choice, and it needs a decision.
