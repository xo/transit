# D31. The form of the Go tables is chosen by measurement, and grammar packages embed their queries

Status: Decided, amended by D47.

Ken decided on 2026-09-29, answering questions 4 and 24:

1. The form of the tables and of the lexer that the Go backend writes is
   chosen in phase 4, from measurements in phase 3: the compile time of the
   largest fixture grammar as Go literals, and the speed of the lexer as code
   and as data. The output is idiomatic Go in either form (D24).
2. A generated grammar package embeds the queries of its grammar, from
   `queries/*.scm`, so that the queries always match the grammar.

D73 records the measurements of phase 3, and D74 sets the form.
