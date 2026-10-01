# D97. Target 3 counts the chunks of new nodes

Status: Decided, amends D37 and D91.

Target 3 of D37 said that a parse after one key allocates a small, fixed
number of times, and that the number does not grow with the size of the
statement. D91 added `Tree.Close`, a port of `ts_tree_delete`, and said that
target 3 holds when the program closes the old tree after each parse.

The port showed that this cannot hold. `ts_tree_delete` frees the nodes of a
tree into a pool of size 0, so C never gives them back to the parser, and a
key makes new nodes in proportion to the statement, in C as in Go. The Go
runtime takes new nodes from chunks of 512 (D62), so one allocation for each
512 new nodes grows with the statement. With C at `-O2`, a key allocates 6 to
8 times for json, 7 to 20 for c, 8 to 20 for javascript, 13 to 43 for python
and 8 to 15 for rust, from 5 KB to 40 KB, and Go takes about the time of C.

Ken decided on 2026-10-01, answering question 64:

1. Target 3 is: a parse after one key allocates a small, fixed number of
   times, plus one chunk for each 512 new nodes that the parse makes. So the
   number of allocations grows no faster than the work of the C runtime for
   the same key.
2. `Tree.Close` stays, as the port of `ts_tree_delete`. Point 5 of D91 does
   not hold, because closing a tree does not change the allocations of the
   next parse.
