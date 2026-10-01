# D92. The benchmarks of phase 4

Status: Decided.

Ken decided on 2026-10-01:

1. The benchmarks of the speed targets on the grammar packages, in
   `test/cgrammar/speed_test.go`, are the record of the measurements of
   phase 4. No decision holds the numbers. `TestSpeedTargets` checks the
   targets with a margin.
2. The C runtime and grammars of the benchmarks are built with `-O2`, as
   upstream builds them. The tests that compare trees keep their own build.
3. The size of a chunk of D62 stays at 512 slots. A tuning can come with a
   measurement of memory in the real use of rline and usql.
