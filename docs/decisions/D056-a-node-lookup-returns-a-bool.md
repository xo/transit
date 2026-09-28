# D56. A node lookup returns the node and a bool

Status: Decided.

Ken decided on 2026-09-29, answering question 54, that a method of `Node` that
can find no node returns the node and a `bool`, as a map lookup does:

    func (n Node) Parent() (Node, bool)

This holds for `Parent`, `Child`, `NamedChild`, the siblings, the descendants,
`ChildByFieldName`, `ChildByFieldID` and the first child for a byte. The caller
cannot forget the case, and it is the form that the Go standard library uses
for a lookup. The Rust binding returns an `Option` in these places (D25).
