# D116. Phase 6 ports upstream in strict order

Status: Decided, amends D30.

Some upstream commits after the base commit add test grammars, such as
`error_recovery_loop` in `a5add39b`. The golden harness makes the test data
of a test grammar from the upstream checkout, and it ran only at the base
commit. So a runtime commit could not be ported ahead of the generator
commits that come before it.

Ken decided on 2026-10-10, answering questions 86 and 87:

1. Phase 6 ports the upstream commits in the order of upstream, the
   generator commits too, one transit commit for each upstream commit.
2. The checkout of upstream moves one commit at a time, to the commit that
   is ported. The golden harness checks the checkout against
   `upstream.txt`, and against the base commit only while `upstream.txt`
   does not exist.
3. Each transit commit that ports an upstream commit, or that records a run
   of commits that change nothing transit ports, writes the full hash of the
   last of them to `upstream.txt`, as step 11 of "Porting an upstream
   change" in `docs/UPSTREAM.md` says.
4. A commit of upstream that speeds up the generator must leave the golden
   files the same. The harness checks the test grammars and the fixture
   grammars at each commit.
