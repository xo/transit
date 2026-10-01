# D108. The usql grammar is one language that takes options

Status: Decided, amends D101, D105 and D107.

D101 built one usql grammar for each family of dialects, six in all, because
the statement scanner depends on the options of the dialect. Ken decided on
2026-10-02 that this does not make sense and does not scale: a new set of
options would need a new grammar. The options of usql are options that usql
feeds into the grammar.

1. The usql grammar is one language: the module `grammars/usql`, the package
   `usql` and the grammar `usql`. The six packages of the families go away.
2. usql gives the grammar the options of the dialect, the flags of the
   syntax of dbmeta: dollar quotes, block comments, `#` comments, `//`
   comments and backticks. The package returns a `*transit.Language` whose
   external scanner reads those options. Every set of options uses the same
   tables.
3. A new option is a new field and a new flag of the scanner, and no new
   grammar.
4. The test module builds the C scanner once for each set of options that it
   tests, and compares it with the Go scanner.
5. Point 1 of D101, the names of the families in point 1 of D105, and point
   4 of D107 no longer hold.
