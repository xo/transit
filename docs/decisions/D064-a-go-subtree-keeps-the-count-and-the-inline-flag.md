# D64. A Go subtree keeps the reference count and the inline flag of C

Status: Proposed, amends D62.

Question 60 asks Ken to decide this. The port of `subtree.c` in phase 3
found two facts that D62 did not know.

## The reference count

D62 says that a subtree does not change once it is made, and that the
garbage collector frees a node in place of the reference counts of upstream.
The garbage collector does free the memory. But upstream also reads the count
to decide what to do, in three places:

1. `ts_subtree_make_mut` changes a node in place when one tree holds it, and
   it copies the node when two trees hold it.
2. `ts_subtree_compress` stops at a node that two trees hold.
3. The balancing of the parser, in `ts_parser__balance_subtree`, stops at a
   node that two trees hold.

The second and the third change the shape of the hidden nodes of a
repetition. So the Go node keeps an atomic count, and `retain` and `release`
change it as C does. A count of 0 frees nothing.

A Go program cannot delete a tree, so the count of a node in a tree that the
program drops stays above 0. `make_mut` then copies a node that C changes in
place. The copy holds the same values, so the result of the edit is the same.
The count of a node that two live trees hold is the same as in C.

## The inline flag

C stores a small leaf inline, in the `Subtree` value itself. D62 chose not to
do this in Go, for the speed of a walk. But the inline form changes what a
leaf keeps:

1. The size of an inline leaf has no column of its own. C reads it back as
   the number of bytes of the leaf.
2. An inline leaf has no `depends_on_column`, so `ts_subtree_new_leaf` drops
   it for a leaf that fits inline.
3. `ts_subtree_set_symbol` asserts that the new symbol of an inline leaf is
   below 255.
4. C copies an inline leaf as a value, so a change of the copy does not
   change the leaf in the tree. Go shares the node, so `make_mut` copies an
   inline node before a change.

So a Go node that C would store inline has the flag `isInline`, and it keeps
only what the inline form keeps. The node itself is a pointer from a chunk,
as D62 says.

## What stays as D62 says

A subtree is a pointer to a node, and the node holds a slice of its children.
A parser takes the nodes and the slices from chunks of a fixed size. The
garbage collector frees the memory.
