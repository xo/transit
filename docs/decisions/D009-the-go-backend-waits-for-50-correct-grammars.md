# D9. The Go backend waits for 50 correct grammars

Status: Decided.

Ken decided on 2026-09-29 that the Go backend is written only when the
generator is stable and correct for a large number of tree-sitter grammars:
at least 50, and 100 or more if possible. Correct means that the C backend
(D8) writes the same output as the upstream tool for each of them.

The measurable form of this gate is in [`docs/PLAN.md`](../PLAN.md), and the
list of grammars is in [`docs/CANDIDATES.md`](../CANDIDATES.md). Ken accepted
both on 2026-09-29 (D18).
