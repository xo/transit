package transit

// This file holds the form of node-types.json in Go, which the NodeTypes
// function of a grammar package returns. It ports no upstream file (D28,
// D76).
// The generator writes node-types.json in generate/node_types.go, and the
// names of the types follow NodeInfoJSON, FieldInfoJSON and NodeTypeJSON of
// crates/generate/src/node_types.rs.

// NodeType is one entry of node-types.json: a kind of node that a tree can
// hold, with the children that it can have. A supertype, such as the
// _expression of many grammars, has Subtypes and no children.
type NodeType struct {
	// Kind and Named name the node type, as Node.Kind and Node.IsNamed give
	// them for a node of the type.
	Kind  string `json:"type"`
	Named bool   `json:"named"`
	// Root is true for the type of the root node of a tree.
	Root bool `json:"root,omitempty"`
	// Extra is true for a type that can appear anywhere, such as a comment.
	Extra bool `json:"extra,omitempty"`
	// Fields holds the children of each field, by the name of the field.
	Fields map[string]FieldInfo `json:"fields,omitempty"`
	// Children holds the named children that no field holds, or nil when
	// there are none.
	Children *FieldInfo `json:"children,omitempty"`
	// Subtypes holds the types of a supertype.
	Subtypes []NodeKind `json:"subtypes,omitempty"`
}

// FieldInfo says which nodes a field, or the children of a node, can hold.
type FieldInfo struct {
	// Multiple is true when it can hold more than one node.
	Multiple bool `json:"multiple"`
	// Required is true when it holds a node in every node of the type.
	Required bool `json:"required"`
	// Types holds the types of the nodes.
	Types []NodeKind `json:"types"`
}

// NodeKind names a node type: its kind, and whether it is named.
type NodeKind struct {
	Kind  string `json:"type"`
	Named bool   `json:"named"`
}
