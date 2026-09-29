# D62. A subtree is a pointer to a node from a chunk

Status: Decided, amended by D64.

Ken decided on 2026-09-29, from the benchmark that D29 asks for, that a
subtree of the Go runtime is a pointer to a struct that holds a slice of its
children. A parser takes the nodes and the slices of children from chunks of
a fixed size, and does not allocate each node alone.

The benchmark is in `_samples/subtree`. It builds, walks and edits trees of
1,000 and of 200,000 leaves, whose height is 20 and 30, as the trees of a
real grammar are. Each form holds the fields of `SubtreeHeapData`. It
compared four forms:

| Form | Build of 200,000 leaves | Walk | One edit | Allocations of a build |
| --- | --- | --- | --- | --- |
| A, a pointer and a slice of children | 15.3 ms | 2.08 ms | 2.08 µs | 524,211 |
| A with small leaves inline, as in C | 12.6 ms | 3.26 ms | 2.20 µs | 324,213 |
| A from chunks | 8.3 ms | 1.88 ms | 1.23 µs | 1,420 |
| B, an index into a shared arena | 14.7 ms | 1.32 ms | 2.12 µs | 70 |

A from chunks builds about 1.8 times as fast as A and B, and edits about 1.7
times as fast. Only B walks a large tree faster, by about 30%. The pauses of
the garbage collector were the same for every form.

The choice also rests on these facts:

1. A subtree does not change once it is made, and trees share subtrees
   through pointers. The garbage collector frees a node when no tree holds it,
   in place of the reference counts of upstream.
2. A chunk never grows, so a parse that writes new nodes into a chunk never
   moves the nodes that an old tree reads. A tree is safe for reads from many
   goroutines while a parse runs (D52). Form B shares one arena that grows,
   and it is not safe for that without a lock or a copy.
3. The cost is memory. A chunk stays alive while one of its nodes is alive.
   The benchmark kept 16 MB on a small tree after 10,000 edits, where plain A
   kept 4 MB. The size of a chunk is tuned against memory with the benchmarks
   of D37.
4. A small leaf inline in a value, as C does it, saves allocations but makes
   every walk 1.6 to 5 times as slow, so the Go runtime does not do it.

This is the pool of subtrees that D37 expects the benchmarks to find. It is a
translation of the memory of upstream, and not a difference in behavior
(D24).
