# D22. transit does not generate ABI 13, and the runtime keeps upstream's ABI 13 branch

Status: Decided.

Ken accepted on 2026-09-29 the recommendation of question 27:

1. The transit generator does not write ABI 13, as the upstream generator
   does not. `ABI_VERSION_MIN` is 14 in `render.rs`, so no golden file exists
   for ABI 13.
2. The runtime ports the version check and the ABI 13 branch as they are,
   because the port rule keeps upstream code (hard rule 5 in `AGENTS.md`).
   Upstream accepts ABI 13 to 15, as `TREE_SITTER_MIN_COMPATIBLE_LANGUAGE_VERSION`
   says, and `ts_language_state_is_primary` in `language.h` returns true for
   every state at ABI 13.
3. A test in the test module (D12) sets the ABI of one C grammar to 13, and
   shows that the Go branch behaves as the C branch does.

If upstream removes ABI 13 from the runtime, phase 6 ports that commit.

On 2026-09-29 no grammar in the set committed a `parser.c` at ABI 13. 94 were
at ABI 15, 71 at ABI 14, and one, `tree-sitter/tree-sitter-fluent`, at ABI 8.
