# D137. A highlight test can name its options

Status: Decided, amends D136, amended by D138.

D136 point 3 moves the highlight tests of the usql grammar that need options
other than the default into `grammars/usql/testdata/highlight`. The test
`TestHighlight`, which the Go backend writes, runs every file of that folder
with the default options. So a test of the node `slash`, which needs
`AnonymousBlocks`, fails there.

Ken decided on 2026-11-04, answering question 107:

1. A highlight test whose first line names a set of options, such as
   `-- options: oracle_anonymous`, runs only with those options.
2. The default run of `internal/grammartest` skips such a file, and
   `TestOptionsHighlight` of the usql grammar runs it with the options that
   it names.
3. The files stay in one folder, `testdata/highlight`. Only
   `internal/grammartest` and the tests of the usql grammar change. The Go
   backend does not change.
