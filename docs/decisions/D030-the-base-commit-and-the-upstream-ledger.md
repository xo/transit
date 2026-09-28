# D30. The base commit, and how the gap to upstream stays small

Status: Decided.

Ken decided on 2026-09-29, answering questions 1, 13 and 35:

1. The base commit is `dcdc8cc55e5dfedfc858080835f153999a29ec40` on
   `master`, from 2026-09-24, 30 commits after `v0.27.0`.
2. The ledger is `docs/upstream/ledger.tsv`, with the columns `upstream`,
   `date`, `status`, `transit` and `note`. A test makes sure that it has one
   line for each upstream commit from the base commit on, with no gap.
3. The ledger is kept up to date from phase 1. Each new upstream commit is
   sorted as it arrives: `ported`, `not-applicable` or `pending`.
4. Ken can move the base commit at the end of a phase, by a decision. The
   golden files are then made again at the new commit, and the ported code
   follows the ledger to it.

At the rate of 2026, a freeze of one year leaves about 580 upstream commits,
of which about 220 touch the runtime or the generator. Sorting them as they
arrive leaves only the ones that transit ports.
