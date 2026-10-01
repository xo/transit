package json_test

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/json"
)

// This example parses a text and prints its tree as an S-expression.
func ExampleLanguage() {
	src := []byte(`{"name": "transit", "tags": [1, 2]}`)
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tree.RootNode())
	// Output:
	// (document (object (pair key: (string (string_content)) value: (string (string_content))) (pair key: (string (string_content)) value: (array (number) (number)))))
}

// This example changes a text as a line editor does after a key. It tells
// the old tree about the edit, parses the new text with the old tree, and
// prints the ranges whose syntax changed. A line editor highlights only
// those ranges again.
func Example_edit() {
	ctx := context.Background()
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	src := []byte(`[1, 2]`)
	tree, err := parser.Parse(ctx, src, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Insert `, "three"` at byte 5, before the `]`.
	insert := []byte(`, "three"`)
	newSrc := slices.Concat(src[:5], insert, src[5:])
	tree.Edit(transit.InputEdit{
		StartByte:   5,
		OldEndByte:  5,
		NewEndByte:  5 + len(insert),
		StartPoint:  transit.Point{Row: 0, Column: 5},
		OldEndPoint: transit.Point{Row: 0, Column: 5},
		NewEndPoint: transit.Point{Row: 0, Column: 5 + len(insert)},
	})
	newTree, err := parser.Parse(ctx, newSrc, tree)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(newTree.RootNode())
	for _, r := range tree.ChangedRanges(newTree) {
		fmt.Printf("changed: %s\n", newSrc[r.StartByte:r.EndByte])
	}
	// Output:
	// (document (array (number) (number) (string (string_content))))
	// changed: , "three"
}

// This example walks the whole tree with a cursor, and prints each named
// node with its field name. A cursor is faster than Node.Child for a walk
// of many nodes.
func Example_treeCursor() {
	src := []byte(`{"a": [true, null]}`)
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		log.Fatal(err)
	}
	cursor := tree.Walk()
	for {
		if n := cursor.Node(); n.IsNamed() {
			field := cursor.FieldName()
			if field != "" {
				field += ": "
			}
			fmt.Printf("%*s%s%s %q\n", 2*cursor.Depth(), "", field, n.Kind(), n.Text(src))
		}
		if cursor.GotoFirstChild() {
			continue
		}
		for !cursor.GotoNextSibling() {
			if !cursor.GotoParent() {
				return
			}
		}
	}
	// Output:
	// document "{\"a\": [true, null]}"
	//   object "{\"a\": [true, null]}"
	//     pair "\"a\": [true, null]"
	//       key: string "\"a\""
	//         string_content "a"
	//       value: array "[true, null]"
	//         true "true"
	//         null "null"
}

// This example highlights a text with the highlight query of the grammar.
// Captures gives the captured nodes in the order of the text, which is the
// order that a highlighter needs. A program compiles a query once and
// shares it, because a query costs much more to compile than to run.
func Example_captures() {
	src := []byte(`{"id": 7, "ok": true}`)
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		log.Fatal(err)
	}
	highlights, err := json.Queries.ReadFile("queries/highlights.scm")
	if err != nil {
		log.Fatal(err)
	}
	query, err := transit.NewQuery(json.Language(), string(highlights))
	if err != nil {
		log.Fatal(err)
	}
	names := query.CaptureNames()
	cursor := transit.NewQueryCursor()
	for m, i := range cursor.Captures(context.Background(), query, tree.RootNode(), src) {
		c := m.Captures[i]
		fmt.Printf("%-18s %s\n", names[c.Index], c.Node.Text(src))
	}
	// Output:
	// string.special.key "id"
	// string             "id"
	// number             7
	// string.special.key "ok"
	// string             "ok"
	// constant.builtin   true
}

// This example finds each pair whose key is "id" with a query and the
// predicate #eq?. Matches evaluates the predicates of the query, so it gives
// only the pairs that pass.
func Example_matches() {
	src := []byte(`{"id": 1, "name": "a", "items": [{"id": 2}]}`)
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		log.Fatal(err)
	}
	query, err := transit.NewQuery(json.Language(), `(pair
  key: (string (string_content) @key)
  value: (_) @value
  (#eq? @key "id"))`)
	if err != nil {
		log.Fatal(err)
	}
	value, _ := query.CaptureIndexForName("value")
	cursor := transit.NewQueryCursor()
	for m := range cursor.Matches(context.Background(), query, tree.RootNode(), src) {
		for _, c := range m.Captures {
			if c.Index == value {
				fmt.Printf("id %s at byte %d\n", c.Node.Text(src), c.Node.StartByte())
			}
		}
	}
	// Output:
	// id 1 at byte 7
	// id 2 at byte 40
}

// This example lists what can come at a cursor, as a completion does.
// StatesAt gives the parse states at the offset of the cursor, and the
// lookahead iterator of each state gives the symbols that the parser can
// accept there. An anonymous symbol is a token that the user can type, and
// a regular symbol is a kind of node.
func Example_statesAt() {
	src := []byte(`{"a": 1, "b": `)
	language := json.Language()
	parser := transit.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		log.Fatal(err)
	}
	states, err := parser.StatesAt(context.Background(), src, len(src), nil)
	if err != nil {
		log.Fatal(err)
	}
	var tokens, nodes []string
	for _, state := range states {
		it, ok := language.LookaheadIterator(state)
		if !ok {
			continue
		}
		for sym := range it.Symbols() {
			switch language.SymbolType(sym) {
			case transit.SymbolAnonymous:
				tokens = append(tokens, language.SymbolName(sym))
			case transit.SymbolRegular:
				nodes = append(nodes, language.SymbolName(sym))
			}
		}
	}
	fmt.Println("tokens:", tokens)
	fmt.Println("nodes:", nodes)
	// Output:
	// tokens: [{ [ "]
	// nodes: [comment object array string number true false null]
}

// This example finds the node at a cursor, then each pair above it, with
// the key of the pair. A completion reads such nodes to decide what kind of
// text goes at the cursor.
func Example_descendantForByteRange() {
	src := []byte(`{"user": {"name": "ann"}}`)
	parser := transit.NewParser()
	if err := parser.SetLanguage(json.Language()); err != nil {
		log.Fatal(err)
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		log.Fatal(err)
	}
	cursor := 20 // the offset of the first "n" of "ann"
	n, ok := tree.RootNode().DescendantForByteRange(cursor, cursor)
	if !ok {
		log.Fatal("no node at the cursor")
	}
	fmt.Printf("%s %q\n", n.Kind(), n.Text(src))
	for p, ok := n.Parent(); ok; p, ok = p.Parent() {
		if p.Kind() == "pair" {
			key, _ := p.ChildByFieldName("key")
			fmt.Println("in the value of the key", key.Text(src))
		}
	}
	// Output:
	// string_content "ann"
	// in the value of the key "name"
	// in the value of the key "user"
}
