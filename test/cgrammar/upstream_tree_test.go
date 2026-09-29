package cgrammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// This file ports crates/cli/src/tests/tree_test.rs of upstream (D35). A
// clone of a Rust tree is Tree.Copy, and Range::byte_range is the pair of
// StartByte and EndByte.

// urByteRange returns the byte range of a node, as byte_range does.
func urByteRange(n transit.Node) [2]int {
	return [2]int{n.StartByte(), n.EndByte()}
}

func TestTreeEdit(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "  abc  !==  def", nil)

	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (binary_expression left: (identifier) right: (identifier))))")

	// urChildren returns the binary expression of the tree and its children.
	urChildren := func(tree *transit.Tree, count int) (transit.Node, []transit.Node) {
		expr := must(must(tree.RootNode().Child(0)).Child(0))
		var children []transit.Node
		for i := range count {
			children = append(children, must(expr.Child(i)))
		}
		return expr, children
	}

	// edit entirely within the tree's padding:
	// resize the padding of the tree and its leftmost descendants.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   1,
			OldEndByte:  1,
			NewEndByte:  2,
			StartPoint:  urPoint(0, 1),
			OldEndPoint: urPoint(0, 1),
			NewEndPoint: urPoint(0, 2),
		})

		expr, c := urChildren(tree, 2)
		child1, child2 := c[0], c[1]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.start_byte", expr.StartByte(), 3)
		urEqual(t, "expr.end_byte", expr.EndByte(), 16)
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child1.start_byte", child1.StartByte(), 3)
		urEqual(t, "child1.end_byte", child1.EndByte(), 6)
		urEqual(t, "child2.has_changes", child2.HasChanges(), false)
		urEqual(t, "child2.start_byte", child2.StartByte(), 8)
		urEqual(t, "child2.end_byte", child2.EndByte(), 11)
	}

	// edit starting in the tree's padding but extending into its content:
	// shrink the content to compensate for the expanded padding.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   1,
			OldEndByte:  4,
			NewEndByte:  5,
			StartPoint:  urPoint(0, 1),
			OldEndPoint: urPoint(0, 5),
			NewEndPoint: urPoint(0, 5),
		})

		expr, c := urChildren(tree, 2)
		child1, child2 := c[0], c[1]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.start_byte", expr.StartByte(), 5)
		urEqual(t, "expr.end_byte", expr.EndByte(), 16)
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child1.start_byte", child1.StartByte(), 5)
		urEqual(t, "child1.end_byte", child1.EndByte(), 6)
		urEqual(t, "child2.has_changes", child2.HasChanges(), false)
		urEqual(t, "child2.start_byte", child2.StartByte(), 8)
		urEqual(t, "child2.end_byte", child2.EndByte(), 11)
	}

	// insertion at the edge of a tree's padding:
	// expand the tree's padding.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   2,
			OldEndByte:  2,
			NewEndByte:  4,
			StartPoint:  urPoint(0, 2),
			OldEndPoint: urPoint(0, 2),
			NewEndPoint: urPoint(0, 4),
		})

		expr, c := urChildren(tree, 2)
		child1, child2 := c[0], c[1]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{4, 17})
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{4, 7})
		urEqual(t, "child2.has_changes", child2.HasChanges(), false)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{9, 12})
	}

	// replacement starting at the edge of the tree's padding:
	// resize the content and not the padding.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   2,
			OldEndByte:  2,
			NewEndByte:  4,
			StartPoint:  urPoint(0, 2),
			OldEndPoint: urPoint(0, 2),
			NewEndPoint: urPoint(0, 4),
		})

		expr, c := urChildren(tree, 2)
		child1, child2 := c[0], c[1]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{4, 17})
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{4, 7})
		urEqual(t, "child2.has_changes", child2.HasChanges(), false)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{9, 12})
	}

	// deletion that spans more than one child node:
	// shrink subsequent child nodes.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   1,
			OldEndByte:  11,
			NewEndByte:  4,
			StartPoint:  urPoint(0, 1),
			OldEndPoint: urPoint(0, 11),
			NewEndPoint: urPoint(0, 4),
		})

		expr, c := urChildren(tree, 3)
		child1, child2, child3 := c[0], c[1], c[2]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{4, 8})
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{4, 4})
		urEqual(t, "child2.has_changes", child2.HasChanges(), true)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{4, 4})
		urEqual(t, "child3.has_changes", child3.HasChanges(), true)
		urEqual(t, "child3.byte_range", urByteRange(child3), [2]int{5, 8})
	}

	// insertion at the end of the tree:
	// extend the tree's content.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   15,
			OldEndByte:  15,
			NewEndByte:  16,
			StartPoint:  urPoint(0, 15),
			OldEndPoint: urPoint(0, 15),
			NewEndPoint: urPoint(0, 16),
		})

		expr, c := urChildren(tree, 3)
		child1, child2, child3 := c[0], c[1], c[2]

		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{2, 16})
		urEqual(t, "child1.has_changes", child1.HasChanges(), false)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{2, 5})
		urEqual(t, "child2.has_changes", child2.HasChanges(), false)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{7, 10})
		urEqual(t, "child3.has_changes", child3.HasChanges(), true)
		urEqual(t, "child3.byte_range", urByteRange(child3), [2]int{12, 16})
	}

	// replacement that starts within a token and extends beyond the end of the tree:
	// resize the token and empty out any subsequent child nodes.
	{
		tree := tree.Copy()
		tree.Edit(transit.InputEdit{
			StartByte:   3,
			OldEndByte:  90,
			NewEndByte:  4,
			StartPoint:  urPoint(0, 3),
			OldEndPoint: urPoint(0, 90),
			NewEndPoint: urPoint(0, 4),
		})

		expr, c := urChildren(tree, 3)
		child1, child2, child3 := c[0], c[1], c[2]
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{2, 4})
		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{2, 4})
		urEqual(t, "child1.has_changes", child1.HasChanges(), true)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{4, 4})
		urEqual(t, "child2.has_changes", child2.HasChanges(), true)
		urEqual(t, "child3.byte_range", urByteRange(child3), [2]int{4, 4})
		urEqual(t, "child3.has_changes", child3.HasChanges(), true)
	}

	// replacement that starts in whitespace and extends beyond the end of the tree:
	// shift the token's start position and empty out its content.
	{
		tree.Edit(transit.InputEdit{
			StartByte:   6,
			OldEndByte:  90,
			NewEndByte:  8,
			StartPoint:  urPoint(0, 6),
			OldEndPoint: urPoint(0, 90),
			NewEndPoint: urPoint(0, 8),
		})

		expr, c := urChildren(tree, 3)
		child1, child2, child3 := c[0], c[1], c[2]
		urEqual(t, "expr.byte_range", urByteRange(expr), [2]int{2, 8})
		urEqual(t, "expr.has_changes", expr.HasChanges(), true)
		urEqual(t, "child1.byte_range", urByteRange(child1), [2]int{2, 5})
		urEqual(t, "child1.has_changes", child1.HasChanges(), false)
		urEqual(t, "child2.byte_range", urByteRange(child2), [2]int{8, 8})
		urEqual(t, "child2.has_changes", child2.HasChanges(), true)
		urEqual(t, "child3.byte_range", urByteRange(child3), [2]int{8, 8})
		urEqual(t, "child3.has_changes", child3.HasChanges(), true)
	}
}

func TestTreeEditWithIncludedRanges(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "html").Language)

	source := "<div><% if a %><span>a</span><% else %><span>b</span><% end %></div>"

	var ranges []transit.Range
	for _, r := range [][2]int{{0, 5}, {15, 29}, {39, 53}, {62, 68}} {
		ranges = append(ranges, transit.Range{
			StartByte:  r[0],
			EndByte:    r[1],
			StartPoint: urPoint(0, r[0]),
			EndPoint:   urPoint(0, r[1]),
		})
	}
	if err := parser.SetIncludedRanges(ranges); err != nil {
		t.Fatal(err)
	}

	tree := urParse(t, parser, source, nil)

	tree.Edit(transit.InputEdit{
		StartByte:   29,
		OldEndByte:  53,
		NewEndByte:  29,
		StartPoint:  urPoint(0, 29),
		OldEndPoint: urPoint(0, 53),
		NewEndPoint: urPoint(0, 29),
	})

	urSlice(t, "included_ranges", tree.IncludedRanges(), []transit.Range{
		{
			StartByte:  0,
			EndByte:    5,
			StartPoint: urPoint(0, 0),
			EndPoint:   urPoint(0, 5),
		},
		{
			StartByte:  15,
			EndByte:    29,
			StartPoint: urPoint(0, 15),
			EndPoint:   urPoint(0, 29),
		},
		{
			StartByte:  29,
			EndByte:    29,
			StartPoint: urPoint(0, 29),
			EndPoint:   urPoint(0, 29),
		},
		{
			StartByte:  38,
			EndByte:    44,
			StartPoint: urPoint(0, 38),
			EndPoint:   urPoint(0, 44),
		},
	})
}

// urCursorAt checks the kind and the name of the node of a cursor.
func urCursorAt(t *testing.T, cursor *transit.TreeCursor, kind string, named bool) {
	t.Helper()
	urEqual(t, "kind", cursor.Node().Kind(), kind)
	urEqual(t, "is_named of "+kind, cursor.Node().IsNamed(), named)
}

func TestTreeCursor(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParse(t, parser, `
                struct Stuff {
                    a: A,
                    b: Option<B>,
                }
            `, nil)

	cursor := tree.Walk()
	urEqual(t, "kind", cursor.Node().Kind(), "source_file")

	urEqual(t, "goto_first_child", cursor.GotoFirstChild(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "struct_item")

	urEqual(t, "goto_first_child", cursor.GotoFirstChild(), true)
	urCursorAt(t, cursor, "struct", false)

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urCursorAt(t, cursor, "type_identifier", true)

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urCursorAt(t, cursor, "field_declaration_list", true)

	urEqual(t, "goto_last_child", cursor.GotoLastChild(), true)
	urCursorAt(t, cursor, "}", false)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(4, 16))

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urCursorAt(t, cursor, ",", false)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(3, 32))

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urCursorAt(t, cursor, "field_declaration", true)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(3, 20))

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urCursorAt(t, cursor, ",", false)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(2, 24))

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urCursorAt(t, cursor, "field_declaration", true)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(2, 20))

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urCursorAt(t, cursor, "{", false)
	urEqual(t, "start_position", cursor.Node().StartPoint(), urPoint(1, 29))

	cp := tree.Walk()
	cp.ResetTo(cursor)

	urCursorAt(t, cp, "{", false)

	urEqual(t, "goto_parent", cp.GotoParent(), true)
	urCursorAt(t, cp, "field_declaration_list", true)

	urEqual(t, "goto_parent", cp.GotoParent(), true)
	urEqual(t, "kind", cp.Node().Kind(), "struct_item")
}

func TestTreeCursorPreviousSiblingWithAliases(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "aliases_in_root"))

	text := "# comment\n# \nfoo foo"
	tree := urParse(t, parser, text, nil)
	cursor := tree.Walk()
	urEqual(t, "kind", cursor.Node().Kind(), "document")

	cursor.GotoFirstChild()
	urEqual(t, "kind", cursor.Node().Kind(), "comment")

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "comment")

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "bar")

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "comment")

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "comment")

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "comment")

	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "bar")
}

func TestTreeCursorPreviousSibling(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	text := `
    // Hi there
    // This is fun!
    // Another one!
`
	tree := urParse(t, parser, text, nil)

	cursor := tree.Walk()
	urEqual(t, "kind", cursor.Node().Kind(), "source_file")

	urEqual(t, "goto_last_child", cursor.GotoLastChild(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "line_comment")
	urEqual(t, "utf8_text", cursor.Node().Text([]byte(text)), "// Another one!")

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "line_comment")
	urEqual(t, "utf8_text", cursor.Node().Text([]byte(text)), "// This is fun!")

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "line_comment")
	urEqual(t, "utf8_text", cursor.Node().Text([]byte(text)), "// Hi there")

	urEqual(t, "goto_previous_sibling", cursor.GotoPreviousSibling(), false)
}

func TestTreeCursorFields(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	tree := urParse(t, parser, "function /*1*/ bar /*2*/ () {}", nil)

	cursor := tree.Walk()
	urEqual(t, "kind", cursor.Node().Kind(), "program")

	cursor.GotoFirstChild()
	urEqual(t, "kind", cursor.Node().Kind(), "function_declaration")
	urEqual(t, "field_name", cursor.FieldName(), "")

	cursor.GotoFirstChild()
	urEqual(t, "kind", cursor.Node().Kind(), "function")
	urEqual(t, "field_name", cursor.FieldName(), "")

	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "comment")
	urEqual(t, "field_name", cursor.FieldName(), "")

	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "identifier")
	urEqual(t, "field_name", cursor.FieldName(), "name")

	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "comment")
	urEqual(t, "field_name", cursor.FieldName(), "")

	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "formal_parameters")
	urEqual(t, "field_name", cursor.FieldName(), "parameters")
}

// urChildForPoint is goto_first_child_for_point, with the Option of Rust
// as an index and a bool.
type urChildForPoint struct {
	index int
	ok    bool
}

// urGotoChildForPoint calls GotoFirstChildForPoint.
func urGotoChildForPoint(c *transit.TreeCursor, at transit.Point) urChildForPoint {
	i, ok := c.GotoFirstChildForPoint(at)
	if !ok {
		return urChildForPoint{}
	}
	return urChildForPoint{i, true}
}

// urKindAt is the kind and the start point of the node of a cursor.
type urKindAt struct {
	kind  string
	start transit.Point
}

// urNodeKindAt returns the kind and the start point of the node of a
// cursor.
func urNodeKindAt(c *transit.TreeCursor) urKindAt {
	return urKindAt{c.Node().Kind(), c.Node().StartPoint()}
}

func TestTreeCursorChildForPoint(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	source := `
    [
        one,
        {
            two: tree
        },
        four, five, six
    ];`[1:]
	tree := urParse(t, parser, source, nil)

	none := urChildForPoint{}
	some := func(i int) urChildForPoint { return urChildForPoint{i, true} }

	c := tree.Walk()
	urEqual(t, "kind", c.Node().Kind(), "program")

	urEqual(t, "goto_first_child_for_point(7, 0)", urGotoChildForPoint(c, urPoint(7, 0)), none)
	urEqual(t, "goto_first_child_for_point(6, 7)", urGotoChildForPoint(c, urPoint(6, 7)), none)
	urEqual(t, "kind", c.Node().Kind(), "program")

	// descend to expression statement
	urEqual(t, "goto_first_child_for_point(6, 5)", urGotoChildForPoint(c, urPoint(6, 5)), some(0))
	urEqual(t, "kind", c.Node().Kind(), "expression_statement")

	// step into ';' and back up
	urEqual(t, "goto_first_child_for_point(7, 0)", urGotoChildForPoint(c, urPoint(7, 0)), none)
	urEqual(t, "goto_first_child_for_point(6, 6)", urGotoChildForPoint(c, urPoint(6, 6)), none)
	urEqual(t, "goto_first_child_for_point(6, 5)", urGotoChildForPoint(c, urPoint(6, 5)), some(1))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{";", urPoint(6, 5)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// descend into array
	urEqual(t, "goto_first_child_for_point(6, 4)", urGotoChildForPoint(c, urPoint(6, 4)), some(0))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"array", urPoint(0, 4)})

	// step into '[' and back up
	urEqual(t, "goto_first_child_for_point(0, 4)", urGotoChildForPoint(c, urPoint(0, 4)), some(0))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"[", urPoint(0, 4)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// step into identifier 'one' and back up
	urEqual(t, "goto_first_child_for_point(1, 0)", urGotoChildForPoint(c, urPoint(1, 0)), some(1))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"identifier", urPoint(1, 8)})
	urEqual(t, "goto_parent", c.GotoParent(), true)
	urEqual(t, "goto_first_child_for_point(1, 10)", urGotoChildForPoint(c, urPoint(1, 10)), some(1))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"identifier", urPoint(1, 8)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// step into first ',' and back up
	urEqual(t, "goto_first_child_for_point(1, 11)", urGotoChildForPoint(c, urPoint(1, 11)), some(2))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{",", urPoint(1, 11)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// step into identifier 'four' and back up
	urEqual(t, "goto_first_child_for_point(5, 0)", urGotoChildForPoint(c, urPoint(5, 0)), some(5))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"identifier", urPoint(5, 8)})
	urEqual(t, "goto_parent", c.GotoParent(), true)
	urEqual(t, "goto_first_child_for_point(5, 0)", urGotoChildForPoint(c, urPoint(5, 0)), some(5))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"identifier", urPoint(5, 8)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// step into ']' and back up
	urEqual(t, "goto_first_child_for_point(6, 0)", urGotoChildForPoint(c, urPoint(6, 0)), some(10))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"]", urPoint(6, 4)})
	urEqual(t, "goto_parent", c.GotoParent(), true)
	urEqual(t, "goto_first_child_for_point(6, 0)", urGotoChildForPoint(c, urPoint(6, 0)), some(10))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"]", urPoint(6, 4)})
	urEqual(t, "goto_parent", c.GotoParent(), true)

	// descend into object
	urEqual(t, "goto_first_child_for_point(2, 0)", urGotoChildForPoint(c, urPoint(2, 0)), some(3))
	urEqual(t, "node", urNodeKindAt(c), urKindAt{"object", urPoint(2, 8)})
}

func TestTreeNodeEquality(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)
	tree := urParse(t, parser, "struct A {}", nil)
	node1 := tree.RootNode()
	node2 := tree.RootNode()
	urSameNode(t, "node1", node1, node2)
	urSameNode(t, "node1.child(0)", must(node1.Child(0)), must(node2.Child(0)))
	if must(node1.Child(0)).Equal(node2) {
		t.Error("node1.child(0) equals node2")
	}
}

func TestGetChangedRanges(t *testing.T) {
	sourceCode := []byte("{a: null};\n")

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, sourceCode, nil)

	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (object (pair key: (property_identifier) value: (null)))))")

	// Updating one token
	{
		tree := tree.Copy()
		sourceCode := slices.Clone(sourceCode)

		// Replace `null` with `nothing` - that token has changed syntax
		edit := urEdit{
			position:      urIndexOf(sourceCode, "ull"),
			deletedLength: 3,
			insertedText:  []byte("othing"),
		}
		inverseEdit := urInvertEdit(sourceCode, edit)
		ranges := urGetChangedRanges(t, parser, &tree, &sourceCode, edit)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, "nothing")})

		// Replace `nothing` with `null` - that token has changed syntax
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, inverseEdit)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, "null")})
	}

	// Changing only leading whitespace
	{
		tree := tree.Copy()
		sourceCode := slices.Clone(sourceCode)

		// Insert leading newline - no changed ranges
		edit := urEdit{
			position:      0,
			deletedLength: 0,
			insertedText:  []byte("\n"),
		}
		inverseEdit := urInvertEdit(sourceCode, edit)
		ranges := urGetChangedRanges(t, parser, &tree, &sourceCode, edit)
		urSlice(t, "ranges", ranges, []transit.Range{})

		// Remove leading newline - no changed ranges
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, inverseEdit)
		urSlice(t, "ranges", ranges, []transit.Range{})
	}

	// Inserting elements
	{
		tree := tree.Copy()
		sourceCode := slices.Clone(sourceCode)

		// Insert a key-value pair before the `}` - those tokens are changed
		edit1 := urEdit{
			position:      urIndexOf(sourceCode, "}"),
			deletedLength: 0,
			insertedText:  []byte(", b: false"),
		}
		inverseEdit1 := urInvertEdit(sourceCode, edit1)
		ranges := urGetChangedRanges(t, parser, &tree, &sourceCode, edit1)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, ", b: false")})

		edit2 := urEdit{
			position:      urIndexOf(sourceCode, ", b"),
			deletedLength: 0,
			insertedText:  []byte(", c: 1"),
		}
		inverseEdit2 := urInvertEdit(sourceCode, edit2)
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, edit2)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, ", c: 1")})

		// Remove the middle pair
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, inverseEdit2)
		urSlice(t, "ranges", ranges, []transit.Range{})

		// Remove the second pair
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, inverseEdit1)
		urSlice(t, "ranges", ranges, []transit.Range{})
	}

	// Wrapping elements in larger expressions
	{
		sourceCode := slices.Clone(sourceCode)

		// Replace `null` with the binary expression `b === null`
		edit1 := urEdit{
			position:      urIndexOf(sourceCode, "null"),
			deletedLength: 0,
			insertedText:  []byte("b === "),
		}
		inverseEdit1 := urInvertEdit(sourceCode, edit1)
		ranges := urGetChangedRanges(t, parser, &tree, &sourceCode, edit1)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, "b === null")})

		// Undo
		ranges = urGetChangedRanges(t, parser, &tree, &sourceCode, inverseEdit1)
		urSlice(t, "ranges", ranges, []transit.Range{urRangeOf(sourceCode, "null")})
	}
}

func TestConsistencyWithMidCodepointEdit(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "php").Language)
	sourceCode := []byte("\n<?php\n\n<<<'\xE5\xAD\x97\xE6\xBC\xA2'\n  T\n\xE5\xAD\x97\xE6\xBC\xA2;")
	tree := urParse(t, parser, sourceCode, nil)

	edit := urEdit{
		position:      17,
		deletedLength: 0,
		insertedText:  []byte{46},
	}
	urPerformEdit(t, tree, &sourceCode, edit)
	tree2 := urParse(t, parser, sourceCode, tree)

	inverted := urInvertEdit(sourceCode, edit)
	urPerformEdit(t, tree2, &sourceCode, inverted)
	tree3 := urParse(t, parser, sourceCode, tree2)

	urEqual(t, "to_sexp", tree3.RootNode().String(), tree.RootNode().String())
}

func TestTreeCursorOnAliasedRootWithExtraChild(t *testing.T) {
	must := urMust(t)
	source := `
fn main() {
    C/* hi */::<D>::E;
}
`

	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParse(t, parser, source, nil)

	function := must(tree.RootNode().Child(0))
	block := must(function.Child(3))
	expressionStatement := must(block.Child(1))
	scopedIdentifier := must(expressionStatement.Child(0))
	genericType := must(scopedIdentifier.Child(0))
	urEqual(t, "kind", genericType.Kind(), "generic_type")

	cursor := genericType.Walk()
	urEqual(t, "goto_first_child", cursor.GotoFirstChild(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "type_identifier")
	urEqual(t, "goto_next_sibling", cursor.GotoNextSibling(), true)
	urEqual(t, "kind", cursor.Node().Kind(), "block_comment")
}

// urIndexOf is index_of of tree_test.rs.
func urIndexOf(text []byte, substring string) int {
	return strings.Index(string(text), substring)
}

// urRangeOf is range_of of tree_test.rs.
func urRangeOf(text []byte, substring string) transit.Range {
	startByte := urIndexOf(text, substring)
	endByte := startByte + len(substring)
	return transit.Range{
		StartByte:  startByte,
		EndByte:    endByte,
		StartPoint: urPoint(0, startByte),
		EndPoint:   urPoint(0, endByte),
	}
}

// urGetChangedRanges is get_changed_ranges of tree_test.rs.
func urGetChangedRanges(t *testing.T, parser *transit.Parser, tree **transit.Tree, sourceCode *[]byte, edit urEdit) []transit.Range {
	t.Helper()
	urPerformEdit(t, *tree, sourceCode, edit)
	newTree := urParse(t, parser, *sourceCode, *tree)
	result := (*tree).ChangedRanges(newTree)
	*tree = newTree
	return result
}

// Regression test for an incremental reparse bug where an external
// scanner's choice depends on lexer->eof() (and thus on the parser's
// current included ranges). The cached token at byte 0 was emitted when
// the included range stopped just after the opener. Widening the range
// to include a later matching delimiter must invalidate that cached
// token so the scanner re-runs and emits the open form instead of the
// unclosed form.
func TestReuseInvalidatesScannerTokenWhenIncludedRangeExpands(t *testing.T) {
	language := urTestFixtureLanguage(t, "external_lookahead_eof_boundary")
	parser := urParser(t, language)

	source := "``"

	if err := parser.SetIncludedRanges([]transit.Range{{
		StartByte:  0,
		EndByte:    1,
		StartPoint: urPoint(0, 0),
		EndPoint:   urPoint(0, 1),
	}}); err != nil {
		t.Fatal(err)
	}
	tree1 := urParse(t, parser, source, nil)
	urEqual(t, "to_sexp", tree1.RootNode().String(), "(document (unclosed_delim))")

	if err := parser.SetIncludedRanges([]transit.Range{{
		StartByte:  0,
		EndByte:    2,
		StartPoint: urPoint(0, 0),
		EndPoint:   urPoint(0, 2),
	}}); err != nil {
		t.Fatal(err)
	}
	tree2 := urParse(t, parser, source, tree1)
	urEqual(t, "to_sexp", tree2.RootNode().String(), "(document (span (open_delim) (close_delim)))")
}
