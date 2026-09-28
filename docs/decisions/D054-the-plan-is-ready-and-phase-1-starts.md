# D54. The plan is ready, and phase 1 starts

Status: Decided.

Ken said on 2026-09-29 that the plan is ready, and that phase 1 starts. This
is the moment that D3 waits for. From now on, code can be written, and it
follows the plan and the decisions. A change to the plan is a decision first.

The plan was committed as `3b5d271` and pushed to `github.com/xo/transit` on
the same day. Phase 1 has four units (D50):

1. The tests of the agent setup and the documents, which are the first Go
   package, with CI and the lint configuration (D36).
2. The ledger of upstream commits and its test (D30).
3. The golden harness and `grammars/grammars.json` (D17, D19, D40, D51).
4. The working C example and the document of the target API (D10).
