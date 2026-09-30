# D80. The highlight test follows the upstream highlighter

Status: Decided, amends D79.

D79 point 4 gave the highlight test of a grammar module a rule of its own.
The javascript module showed that the rule differs from upstream in three
ways. Ken decided on 2026-09-30 that the test follows the upstream
highlighter, `crates/highlight/src/highlight.rs`, in all three:

1. When two patterns capture one node, the last pattern counts, as the
   highlighter keeps the last pattern that matches the node.
2. The test reads each file that `tree-sitter.json` lists under
   `highlights`, in that order, and each file under `injections`. When the
   list is missing, it reads `queries/highlights.scm` and
   `queries/injections.scm`.
3. The test builds the layers of the text with the package `inject` (D72).
   It finds the grammar of an injected name with the `injection-regex` of
   each grammar of the module in `tree-sitter.json`, as the loader of
   upstream does, and it skips a name that no grammar of the module
   matches. Each assertion is checked against the highlight of the deepest
   layer at its position.

The other parts of D79 point 4 stay: a negative assertion with no capture
fails, and a column counts code points.
