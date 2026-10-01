# D88. A grammar module holds its list of upstream failures

Status: Decided, amends D79, amended by D93.

D79 says that the corpus test of a grammar module expects the cases that
fail upstream to fail, and that `grammars/grammars.json` records their
names. The generator found that file above the package, so outside a
checkout of transit, such as in the Go module cache, it had no list. Ken
decided on 2026-09-30:

1. A grammar package holds `testdata/failing.txt`, one name of a case on
   each line, when some case of its corpus fails upstream. The golden
   harness writes it from the record of the grammar in `grammars.json`.
2. The corpus test reads it, and `grammar_test.go` holds no list, so the
   generator test compares `grammar_test.go` everywhere.
3. In a checkout of transit, the generator test compares `failing.txt` with
   the record in `grammars.json`.
