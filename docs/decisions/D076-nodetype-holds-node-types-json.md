# D76. NodeType holds node-types.json

Status: Decided.

The function `NodeTypes` of a generated grammar package (`docs/API.md`) needs
a type of the runtime for the node types of `node-types.json`. Upstream has
no such type in the runtime, so it is an API that upstream does not have
(D28). Ken decided on 2026-09-30 that it is this, in `node_type.go`:

```go
type NodeType struct {
	Kind     string               `json:"type"`
	Named    bool                 `json:"named"`
	Root     bool                 `json:"root,omitempty"`
	Extra    bool                 `json:"extra,omitempty"`
	Fields   map[string]FieldInfo `json:"fields,omitempty"`
	Children *FieldInfo           `json:"children,omitempty"`
	Subtypes []NodeKind           `json:"subtypes,omitempty"`
}

type FieldInfo struct {
	Multiple bool       `json:"multiple"`
	Required bool       `json:"required"`
	Types    []NodeKind `json:"types"`
}

type NodeKind struct {
	Kind  string `json:"type"`
	Named bool   `json:"named"`
}
```

1. The fields follow `NodeInfoJSON`, `FieldInfoJSON` and `NodeTypeJSON` of
   `node_types.rs` in the generator of upstream. `Kind` holds "type", as
   `Node.Kind` does.
2. `Children` is nil when a node type has no children.
3. The runtime has no parser of the file. The generated `NodeTypes` reads the
   embedded `node-types.json` with `encoding/json` on each call, and returns
   a new slice.
