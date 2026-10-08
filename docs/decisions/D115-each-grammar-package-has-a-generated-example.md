# D115. Each grammar package has a generated example

Status: Decided, amends D53.

D53 puts example functions in each package. Only `grammars/json` and
`grammars/html` had them, written by hand.

Ken decided on 2026-10-08, answering question 84:

1. The Go backend writes `example_test.go` into each grammar package. Its
   example parses the input of the first corpus case of the package that
   passes, prints the tree, and compiles each query of the package.
2. A hand-written example of a grammar package moves to
   `example_more_test.go`, which the generator does not write.
3. The generator test checks `example_test.go` as it checks the other files
   that the generator writes.
