# D5. Each decision is a file of its own

Status: Decided.

dbmeta D111 says that a large project keeps each decision in a file of its own
under `docs/decisions/`, and that a small library keeps its decisions in
`docs/PLAN.md`. It names dbmeta, dbimp and usql as the large projects.

Ken decided on 2026-09-29, answering question 12, that transit is a large
project. The port covers a C runtime of about
18,000 lines, a Rust generator of about 22,000 lines, the upstream tests and
one package for each grammar. The plan held more than 20 open questions, and
each answer became a decision.

The layout follows dbmeta D111. Each decision is
`docs/decisions/D<nnn>-<title>.md`, and [`README.md`](README.md) in this
folder is the index.
