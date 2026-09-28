# D3. The plan and the rules come before any code

Status: Decided.

Ken decided on 2026-09-29 that no code is written until he says that the plan
is ready. Three documents come first:

1. [`docs/PLAN.md`](../PLAN.md), which holds the plan and any open question.
2. [`docs/GRAMMAR.md`](../GRAMMAR.md), which holds the rules for adding a
   grammar.
3. [`docs/UPSTREAM.md`](../UPSTREAM.md), which holds the rules for porting a
   change that upstream tree-sitter makes.

Code includes Go source, Go tests, scripts and the CI workflow. A document,
the license and the files that git reads, such as `.gitignore`, are not code.

## Why

A port of this size is hard to change after it starts. The runtime is about
18,000 lines of C and the parser generator is about 22,000 lines of Rust.
Upstream made 290 commits in the six months up to 2026-09-29. If the rules for
following upstream are not settled before the first file is ported, every file
that is ported before the rules follows a different rule.
