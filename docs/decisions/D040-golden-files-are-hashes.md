# D40. Golden files are hashes, and the test grammars keep their files

Status: Decided.

Ken decided on 2026-09-29, answering question 20:

1. For a real grammar, `grammars/grammars.json` holds the SHA-256 of the
   `parser.c` and the `node-types.json` that the upstream tool writes.
2. The 68 test grammars keep their `grammar.json`, `parser.c` and
   `node-types.json` as test data.
3. When a hash differs, the golden harness makes the files again on the
   machine that runs it, and CI keeps them as artifacts of the run.
