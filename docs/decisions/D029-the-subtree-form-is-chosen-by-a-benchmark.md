# D29. The Go form of a subtree is chosen by a benchmark

Status: Decided.

Ken decided on 2026-09-29, answering question 32, that the Go form of a
subtree is chosen at the start of phase 3, by a benchmark of two forms:

1. A pointer to a struct that holds a slice of its children.
2. An index into slices that one tree owns.

In C, a small leaf lives inside the pointer, and a node keeps its children in
the memory before itself (`ts_subtree_children`). Go has neither form. The
choice is recorded as a decision before `subtree.c` is ported, because every
other file of the runtime uses it. It is a translation, not a difference in
behavior (D24).
