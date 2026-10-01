# D91. Tree.Close returns the nodes to a free list

Status: Decided, amends D37, amended by D97.

Target 3 of D37 says a parse after one key allocates a small, fixed number of
times. With the free list of stack nodes of C ported, a key at 10 KB makes 5
to 17 allocations, but one chunk (D62) for each 512 nodes that the reparse
makes still grows with the statement. C reuses a freed subtree from the list
`free_trees` of its subtree pool, and it frees the subtrees of a tree in
`ts_tree_delete`. The Go port has neither, because the garbage collector
frees a tree, and a tree that a program drops never takes the counts of its
nodes to 0 (D64).

Ken decided on 2026-10-01:

1. The runtime ports the free list of subtrees of the subtree pool of C, so a
   subtree whose count reaches 0 can be used again.
2. `Tree` gets the method `Close`, a port of `ts_tree_delete`, which releases
   the root of the tree. A node that another tree shares, after `Copy` or a
   parse with the old tree, goes back to the list only when its last tree is
   closed.
3. After `Close`, the tree and its nodes must not be used, as in C. The
   documentation says so, and the runtime does not check.
4. `Close` is optional. A tree that is not closed is freed by the garbage
   collector, as before, and only the number of allocations differs.
5. Target 3 holds when the program closes the old tree after each parse.
