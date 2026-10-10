# D117. transit follows upstream on request

Status: Decided.

Phase 6 ported every upstream commit up to `e9930e09`, the upstream `master`
on 2026-10-10. Upstream keeps moving, and the plan says that transit
follows it from there.

Ken decided on 2026-10-10, answering question 89:

1. transit follows upstream when Ken asks. No scheduled task fetches or
   ports upstream.
2. When Ken asks, the agent fetches upstream, sorts each new commit in the
   ledger, and ports the new commits in the order of upstream, as D116 and
   `docs/UPSTREAM.md` say for phase 6.
