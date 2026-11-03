# D136. A line with only a slash inside parentheses is text

Status: Decided, amends D135, amended by D137.

The node `slash` of D135 ended a statement at a line with only `/`, even
inside parentheses or inside a block that had not ended. So `select a`, then
a `/` line, then `b` split into two statements. The highlight tests that
need options other than the default went into a new folder,
`grammars/usql/testdata/options_highlight`, which no decision named.

Ken decided on 2026-11-04:

1. With `AnonymousBlocks` on, a line with only `/` inside parentheses is the
   text of the statement. It does not end the statement, and it is not a
   node `slash`.
2. Outside parentheses, a line with only `/` stays as D135 says.
3. The highlight tests that need options other than the default go into
   `grammars/usql/testdata/highlight`, with the other highlight tests. The
   folder `testdata/options_highlight` goes away.
