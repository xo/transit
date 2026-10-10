# D118. The nightly tier of CI starts

Status: Decided, amends D36.

D36 sets three tiers of CI. On 2026-09-29 Ken chose to wait with the nightly
tier, so the tests of `test/cgrammar` skip in CI. They need a checkout of
upstream and a cache of grammars, and CI has neither.

Ken decided on 2026-10-10, answering question 90:

1. The nightly tier starts now. A workflow runs each night on linux/amd64.
2. The workflow checks out upstream at the commit in `upstream.txt`, fetches
   the fixture grammars with the golden harness, and runs the tests of the
   test module, so that `test/cgrammar` compares the Go runtime with the C
   runtime.
3. The other parts of the nightly tier of D36, every grammar at both ABI
   versions with the corpus of each C output, come in the same workflow
   when the runner has the time and the memory for them.
