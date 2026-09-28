# D16. The grammar set covers the C template and the runtime, and prefers upstream grammars

Status: Decided.

Ken decided on 2026-09-29 that transit builds a comprehensive set of
grammars with two goals:

1. The set reaches nearly every path of the C code that upstream `render.rs`
   writes, so that each path of the C backend (D8) is tested.
2. The set reaches most of the API of the Go runtime, when it exists.

The set prefers the grammars of the upstream `tree-sitter` project. Past
those, it takes the grammars that are most used and maintained.

Gemini and DeepSeek were asked for the most used grammars. The proposed set,
and how it was measured, is in [`docs/CANDIDATES.md`](../CANDIDATES.md). Ken
accepted the set on 2026-09-29 (D18).
