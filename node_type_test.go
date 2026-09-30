package transit

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestNodeTypeReadsNodeTypesJSON reads each form of an entry of
// node-types.json: a supertype, a node with fields and children, the root
// and an extra.
func TestNodeTypeReadsNodeTypesJSON(t *testing.T) {
	const text = `[
  {"type": "_value", "named": true, "subtypes": [{"type": "array", "named": true}]},
  {
    "type": "pair",
    "named": true,
    "fields": {
      "key": {"multiple": false, "required": true, "types": [{"type": "string", "named": true}]}
    },
    "children": {"multiple": true, "required": false, "types": [{"type": ",", "named": false}]}
  },
  {"type": "document", "named": true, "root": true, "fields": {}},
  {"type": "comment", "named": true, "extra": true}
]`
	var got []NodeType
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	want := []NodeType{
		{Kind: "_value", Named: true, Subtypes: []NodeKind{{Kind: "array", Named: true}}},
		{
			Kind:  "pair",
			Named: true,
			Fields: map[string]FieldInfo{
				"key": {Required: true, Types: []NodeKind{{Kind: "string", Named: true}}},
			},
			Children: &FieldInfo{Multiple: true, Types: []NodeKind{{Kind: ","}}},
		},
		{Kind: "document", Named: true, Root: true, Fields: map[string]FieldInfo{}},
		{Kind: "comment", Named: true, Extra: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the node types are\n%+v\nwant\n%+v", got, want)
	}
}
