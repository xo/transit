# D79. The tests of a grammar module

Status: Decided, amended by D80.

Ken decided on 2026-09-30 these rules of the tests that `transit generate`
writes into a grammar module (`docs/GRAMMAR.md`, "Tests"):

1. The golden harness records the names of the corpus cases that fail
   upstream in the corpus record of each grammar in `grammars/grammars.json`.
   The corpus test of the grammar expects those cases to fail, and no other
   case.
2. A corpus case with the attribute `:cst` is tested now. The test ports the
   CST output of the upstream tool.
3. A case with `:language(x)` for another grammar fails with "Language not
   found", as upstream does.
4. The highlight test compares each assertion of `test/highlight` with the
   innermost capture at its position. Of two captures of one node, the first
   counts. A negative assertion with no capture fails, as
   `iterate_assertions` does. A column counts code points. This rule holds
   for now. A later check compares it with the upstream tool on the grammars
   that have `test/highlight`.
5. CI and the commands of "Before you stage" in `AGENTS.md` run `go vet`,
   `go test -race` and `golangci-lint` in each module under `grammars/`.
