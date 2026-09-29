# D68. Node.Equal is ts_node_eq, and == also compares the position

Status: Decided.

Ken decided on 2026-09-29 that the type `Node` has the method `Equal`, which
is `ts_node_eq`, and that two nodes still compare with `==`.

A `Node` holds what `TSNode` holds: the position of the node, its alias, the
address of its subtree and its tree. `==` compares all of them.
`ts_node_eq` compares only the tree and the subtree. The two differ only for
a node that `Node.Edit` moved: C calls the moved node and the node before
the edit equal, and `==` does not. `Equal` gives the result of C.

This is a deliberate difference from upstream (hard rule 6): `==` is the
comparison of a Go value, and `Equal` is the comparison of upstream.
