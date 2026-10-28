package cgrammar

import (
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// This file ports crates/cli/src/tests/node_test.rs of upstream (D35). A
// comparison of two nodes uses Node.Equal (D68).

const urJSONExample = `

[
  123,
  false,
  {
    "x": null
  }
]
`

const urGrammarWithAliasesAndExtras = `{
  "name": "aliases_and_extras",

  "extras": [
    {"type": "PATTERN", "value": "\\s+"},
    {"type": "SYMBOL", "name": "comment"}
  ],

  "rules": {
    "a": {
      "type": "SEQ",
      "members": [
        {"type": "SYMBOL", "name": "b"},
        {
          "type": "ALIAS",
          "value": "B",
          "named": true,
          "content": {"type": "SYMBOL", "name": "b"}
        },
        {
          "type": "ALIAS",
          "value": "C",
          "named": true,
          "content": {"type": "SYMBOL", "name": "_c"}
        }
      ]
    },

    "b": {"type": "STRING", "value": "b"},

    "_c": {"type": "STRING", "value": "c"},

    "comment": {"type": "STRING", "value": "..."}
  }
}`

// urNodeAt checks the bytes and the points of a node.
func urNodeAt(t *testing.T, what string, n transit.Node, startByte, endByte int, start, end transit.Point) {
	t.Helper()
	urEqual(t, what+".start_byte", n.StartByte(), startByte)
	urEqual(t, what+".end_byte", n.EndByte(), endByte)
	urEqual(t, what+".start_position", n.StartPoint(), start)
	urEqual(t, what+".end_position", n.EndPoint(), end)
}

// urParentIs checks the parent of a node.
func urParentIs(t *testing.T, what string, n, parent transit.Node) {
	t.Helper()
	got, ok := n.Parent()
	if !ok {
		t.Errorf("%s.parent() is None", what)
		return
	}
	urSameNode(t, what+".parent()", got, parent)
}

// urChildWithDescendantIs checks child_with_descendant.
func urChildWithDescendantIs(t *testing.T, what string, n, descendant, want transit.Node) {
	t.Helper()
	got, ok := n.ChildWithDescendant(descendant)
	if !ok {
		t.Errorf("%s.child_with_descendant() is None", what)
		return
	}
	urSameNode(t, what+".child_with_descendant()", got, want)
}

func TestNodeChild(t *testing.T) {
	must := urMust(t)
	tree := urParseJSONExample(t)
	arrayNode := must(tree.RootNode().Child(0))
	find := func(s string) int { return strings.Index(urJSONExample, s) }

	urEqual(t, "array_node.kind", arrayNode.Kind(), "array")
	urEqual(t, "array_node.named_child_count", arrayNode.NamedChildCount(), 3)
	urNodeAt(t, "array_node", arrayNode, find("["), find("]")+1, urPoint(2, 0), urPoint(8, 1))
	urEqual(t, "array_node.child_count", arrayNode.ChildCount(), 7)

	leftBracketNode := must(arrayNode.Child(0))
	numberNode := must(arrayNode.Child(1))
	commaNode1 := must(arrayNode.Child(2))
	falseNode := must(arrayNode.Child(3))
	commaNode2 := must(arrayNode.Child(4))
	objectNode := must(arrayNode.Child(5))
	rightBracketNode := must(arrayNode.Child(6))

	urEqual(t, "left_bracket_node.kind", leftBracketNode.Kind(), "[")
	urEqual(t, "number_node.kind", numberNode.Kind(), "number")
	urEqual(t, "comma_node1.kind", commaNode1.Kind(), ",")
	urEqual(t, "false_node.kind", falseNode.Kind(), "false")
	urEqual(t, "comma_node2.kind", commaNode2.Kind(), ",")
	urEqual(t, "object_node.kind", objectNode.Kind(), "object")
	urEqual(t, "right_bracket_node.kind", rightBracketNode.Kind(), "]")

	urEqual(t, "left_bracket_node.is_named", leftBracketNode.IsNamed(), false)
	urEqual(t, "number_node.is_named", numberNode.IsNamed(), true)
	urEqual(t, "comma_node1.is_named", commaNode1.IsNamed(), false)
	urEqual(t, "false_node.is_named", falseNode.IsNamed(), true)
	urEqual(t, "comma_node2.is_named", commaNode2.IsNamed(), false)
	urEqual(t, "object_node.is_named", objectNode.IsNamed(), true)
	urEqual(t, "right_bracket_node.is_named", rightBracketNode.IsNamed(), false)

	urNodeAt(t, "number_node", numberNode, find("123"), find("123")+3, urPoint(3, 2), urPoint(3, 5))
	urNodeAt(t, "false_node", falseNode, find("false"), find("false")+5, urPoint(4, 2), urPoint(4, 7))

	urEqual(t, "object_node.start_byte", objectNode.StartByte(), find("{"))
	urEqual(t, "object_node.start_position", objectNode.StartPoint(), urPoint(5, 2))
	urEqual(t, "object_node.end_position", objectNode.EndPoint(), urPoint(7, 3))

	urEqual(t, "object_node.child_count", objectNode.ChildCount(), 3)
	leftBraceNode := must(objectNode.Child(0))
	pairNode := must(objectNode.Child(1))
	rightBraceNode := must(objectNode.Child(2))

	urEqual(t, "left_brace_node.kind", leftBraceNode.Kind(), "{")
	urEqual(t, "pair_node.kind", pairNode.Kind(), "pair")
	urEqual(t, "right_brace_node.kind", rightBraceNode.Kind(), "}")

	urEqual(t, "left_brace_node.is_named", leftBraceNode.IsNamed(), false)
	urEqual(t, "pair_node.is_named", pairNode.IsNamed(), true)
	urEqual(t, "right_brace_node.is_named", rightBraceNode.IsNamed(), false)

	urNodeAt(t, "pair_node", pairNode, find(`"x"`), find("null")+4, urPoint(6, 4), urPoint(6, 13))

	urEqual(t, "pair_node.child_count", pairNode.ChildCount(), 3)
	stringNode := must(pairNode.Child(0))
	colonNode := must(pairNode.Child(1))
	nullNode := must(pairNode.Child(2))

	urEqual(t, "string_node.kind", stringNode.Kind(), "string")
	urEqual(t, "colon_node.kind", colonNode.Kind(), ":")
	urEqual(t, "null_node.kind", nullNode.Kind(), "null")

	urEqual(t, "string_node.is_named", stringNode.IsNamed(), true)
	urEqual(t, "colon_node.is_named", colonNode.IsNamed(), false)
	urEqual(t, "null_node.is_named", nullNode.IsNamed(), true)

	urNodeAt(t, "string_node", stringNode, find(`"x"`), find(`"x"`)+3, urPoint(6, 4), urPoint(6, 7))
	urNodeAt(t, "null_node", nullNode, find("null"), find("null")+4, urPoint(6, 9), urPoint(6, 13))

	urParentIs(t, "string_node", stringNode, pairNode)
	urParentIs(t, "null_node", nullNode, pairNode)
	urParentIs(t, "pair_node", pairNode, objectNode)
	urParentIs(t, "number_node", numberNode, arrayNode)
	urParentIs(t, "false_node", falseNode, arrayNode)
	urParentIs(t, "object_node", objectNode, arrayNode)
	urParentIs(t, "array_node", arrayNode, tree.RootNode())
	if _, ok := tree.RootNode().Parent(); ok {
		t.Error("the root node has a parent")
	}

	urChildWithDescendantIs(t, "root", tree.RootNode(), nullNode, arrayNode)
	urChildWithDescendantIs(t, "array_node", arrayNode, nullNode, objectNode)
	urChildWithDescendantIs(t, "object_node", objectNode, nullNode, pairNode)
	urChildWithDescendantIs(t, "pair_node", pairNode, nullNode, nullNode)
	if _, ok := nullNode.ChildWithDescendant(nullNode); ok {
		t.Error("null_node.child_with_descendant(null_node) is not None")
	}
}

// urKinds returns the kinds of a sequence of nodes.
func urKinds(nodes iter.Seq[transit.Node]) []string {
	var out []string
	for n := range nodes {
		out = append(out, n.Kind())
	}
	return out
}

func TestNodeChildren(t *testing.T) {
	must := urMust(t)
	tree := urParseJSONExample(t)
	arrayNode := must(tree.RootNode().Child(0))
	urSlice(t, "children", urKinds(arrayNode.Children()), []string{"[", "number", ",", "false", ",", "object", "]"})
	urSlice(t, "named_children", urKinds(arrayNode.NamedChildren()), []string{"number", "false", "object"})
	var objectNode transit.Node
	found := false
	for n := range arrayNode.NamedChildren() {
		if n.Kind() == "object" {
			objectNode, found = n, true
			break
		}
	}
	if !found {
		t.Fatal("no object in the named children")
	}
	urSlice(t, "children", urKinds(objectNode.Children()), []string{"{", "pair", "}"})
}

func TestNodeChildrenByFieldName(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "python").Language)
	source := `
        if one:
            a()
        elif two:
            b()
        elif three:
            c()
        elif four:
            d()
    `

	tree := urParse(t, parser, source, nil)
	node := must(tree.RootNode().Child(0))
	urEqual(t, "kind", node.Kind(), "if_statement")
	var alternativeTexts []string
	for n := range node.ChildrenByFieldName("alternative") {
		condition := must(n.ChildByFieldName("condition"))
		alternativeTexts = append(alternativeTexts, source[condition.StartByte():condition.EndByte()])
	}
	urSlice(t, "alternative_texts", alternativeTexts, []string{"two", "three", "four"})
}

func TestNodeParentOfChildByFieldName(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "foo(a().b[0].c.d.e())", nil)
	callNode := must(must(tree.RootNode().NamedChild(0)).NamedChild(0))
	urEqual(t, "kind", callNode.Kind(), "call_expression")

	// Regression test - when a field points to a hidden node (in this case, `_expression`)
	// the hidden node should not be added to the node parent cache.
	urParentIs(t, "child_by_field_name(function)", must(callNode.ChildByFieldName("function")), callNode)
}

func TestParentOfZeroWidthNode(t *testing.T) {
	must := urMust(t)
	code := "def dupa(foo):"

	parser := urParser(t, fixtureGrammar(t, "python").Language)

	tree := urParse(t, parser, code, nil)
	root := tree.RootNode()
	functionDefinition := must(root.Child(0))
	block := must(functionDefinition.Child(4))
	blockParent := must(block.Parent())

	urEqual(t, "block.to_string", block.String(), "(block)")
	urEqual(t, "block_parent.kind", blockParent.Kind(), "function_definition")
	urEqual(t, "block_parent.to_string", blockParent.String(),
		"(function_definition name: (identifier) parameters: (parameters (identifier)) body: (block))")

	urChildWithDescendantIs(t, "root", root, block, functionDefinition)
	urChildWithDescendantIs(t, "function_definition", functionDefinition, block, block)
	if _, ok := block.ChildWithDescendant(block); ok {
		t.Error("block.child_with_descendant(block) is not None")
	}

	code = "<script></script>"
	if err := parser.SetLanguage(fixtureGrammar(t, "html").Language); err != nil {
		t.Fatal(err)
	}

	tree = urParse(t, parser, code, nil)
	root = tree.RootNode()
	scriptElement := must(root.Child(0))
	rawText := must(scriptElement.Child(1))
	urParentIs(t, "raw_text", rawText, scriptElement)
}

func TestNextSiblingOfZeroWidthNode(t *testing.T) {
	must := urMust(t)
	language := urTestFixtureLanguage(t, "next_sibling_from_zwt")
	parser := urParser(t, language)

	tree := urParse(t, parser, "abdef", nil)

	rootNode := tree.RootNode()
	missingC := must(rootNode.Child(2))
	urEqual(t, "missing_c.is_missing", missingC.IsMissing(), true)
	urEqual(t, "missing_c.kind", missingC.Kind(), "c")
	nodeD := must(rootNode.Child(3))
	urSameNode(t, "missing_c.next_sibling()", must(missingC.NextSibling()), nodeD)

	prevSibling := must(nodeD.PrevSibling())
	urSameNode(t, "node_d.prev_sibling()", prevSibling, missingC)
}

func TestFirstChildForOffset(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "x10 + 100", nil)
	sumNode := must(must(tree.RootNode().Child(0)).Child(0))

	urEqual(t, "first_child_for_byte(0)", must(sumNode.FirstChildForByte(0)).Kind(), "identifier")
	urEqual(t, "first_child_for_byte(1)", must(sumNode.FirstChildForByte(1)).Kind(), "identifier")
	urEqual(t, "first_child_for_byte(3)", must(sumNode.FirstChildForByte(3)).Kind(), "+")
	urEqual(t, "first_child_for_byte(5)", must(sumNode.FirstChildForByte(5)).Kind(), "number")
}

func TestFirstNamedChildForOffset(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "x10 + 100", nil)
	sumNode := must(must(tree.RootNode().Child(0)).Child(0))

	urEqual(t, "first_named_child_for_byte(0)", must(sumNode.FirstNamedChildForByte(0)).Kind(), "identifier")
	urEqual(t, "first_named_child_for_byte(1)", must(sumNode.FirstNamedChildForByte(1)).Kind(), "identifier")
	urEqual(t, "first_named_child_for_byte(3)", must(sumNode.FirstNamedChildForByte(3)).Kind(), "number")
}

func TestFirstNamedChildForOffsetAfterAnonymousChildren(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "c").Language)
	tree := urParse(t, parser, "for (;;);", nil)
	urEqual(t, "has_error", tree.RootNode().HasError(), false)

	forNode := must(tree.RootNode().NamedChild(0))
	urEqual(t, "kind", forNode.Kind(), "for_statement")
	bodyNode := must(forNode.ChildByFieldName("body"))
	urEqual(t, "kind", bodyNode.Kind(), "expression_statement")
	urSameNode(t, "first_named_child_for_byte(0)", must(forNode.FirstNamedChildForByte(0)), bodyNode)
}

func TestNodeFieldNameForChild(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "c").Language)
	tree := urParse(t, parser, "int w = x + /* y is special! */ y;", nil)
	translationUnitNode := tree.RootNode()
	declarationNode := must(translationUnitNode.NamedChild(0))

	binaryExpressionNode := must(must(declarationNode.ChildByFieldName("declarator")).ChildByFieldName("value"))

	// -------------------
	// left: (identifier)  0
	// operator: "+"       1 <--- (not a named child)
	// (comment)           2 <--- (is an extra)
	// right: (identifier) 3
	// -------------------

	urEqual(t, "field_name_for_child(0)", binaryExpressionNode.FieldNameForChild(0), "left")
	urEqual(t, "field_name_for_child(1)", binaryExpressionNode.FieldNameForChild(1), "operator")
	// The comment should not have a field name, as it's just an extra
	urEqual(t, "field_name_for_child(2)", binaryExpressionNode.FieldNameForChild(2), "")
	urEqual(t, "field_name_for_child(3)", binaryExpressionNode.FieldNameForChild(3), "right")
	// Negative test - Not a valid child index
	urEqual(t, "field_name_for_child(4)", binaryExpressionNode.FieldNameForChild(4), "")
}

func TestNodeFieldNameForNamedChild(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "c").Language)
	tree := urParse(t, parser, "int w = x + /* y is special! */ y;", nil)
	translationUnitNode := tree.RootNode()
	declarationNode := must(translationUnitNode.NamedChild(0))

	binaryExpressionNode := must(must(declarationNode.ChildByFieldName("declarator")).ChildByFieldName("value"))

	// -------------------
	// left: (identifier)  0
	// operator: "+"       _ <--- (not a named child)
	// (comment)           1 <--- (is an extra)
	// right: (identifier) 2
	// -------------------

	urEqual(t, "field_name_for_named_child(0)", binaryExpressionNode.FieldNameForNamedChild(0), "left")
	// The comment should not have a field name, as it's just an extra
	urEqual(t, "field_name_for_named_child(1)", binaryExpressionNode.FieldNameForNamedChild(1), "")
	// The operator is not a named child, so the named child at index 2 is the right child
	urEqual(t, "field_name_for_named_child(2)", binaryExpressionNode.FieldNameForNamedChild(2), "right")
	// Negative test - Not a valid child index
	urEqual(t, "field_name_for_named_child(3)", binaryExpressionNode.FieldNameForNamedChild(3), "")
}

func TestNodeChildByFieldNameWithExtraHiddenChildren(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "python").Language)

	// In the Python grammar, some fields are applied to `suite` nodes,
	// which consist of an invisible `indent` token followed by a block.
	// Check that when searching for a child with a field name, we don't
	// return a hidden child node.
	tree := urParse(t, parser, "while a:\n  pass", nil)
	whileNode := must(tree.RootNode().Child(0))
	urEqual(t, "kind", whileNode.Kind(), "while_statement")
	urSameNode(t, "child_by_field_name(body)", must(whileNode.ChildByFieldName("body")), must(whileNode.Child(3)))
}

func TestNodeChildByFieldNameThroughHiddenNodes(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, urTestFixtureLanguage(t, "field_lookup_through_hidden_nodes"))

	// The first child that the tree cursor gives each field, if any. An
	// empty expected text means no child.
	for _, c := range []struct {
		source, fieldName, expected string
	}{
		{"[a]", "spread", ""},
		{"[a, ...b]", "spread", "...b"},
		{"let a = b;", "outer", "="},
		{"let a // c\n= b;", "outer", "="},
		{"let a @x = b;", "outer", "x"},
		{"shadow a;", "outer", ""},
		{"alias ...a;", "spread", ""},
		{"pair ...a, b;", "spread", ""},
		{"pair ...a, ...b;", "spread", "...b"},
		{"multi a a = b;", "outer", "="},
		{"restore a;", "outer", "a"},
		{"back a b;", "outer", "b"},
		{"maybe none end;", "outer", ""},
		{"maybe a end;", "outer", "a"},
		{"order a b c;", "outer", "c"},
		{"skip a b c;", "outer", "c"},
		{"sum a + b + c;", "left", "a"},
	} {
		tree := urParse(t, parser, c.source, nil)
		node := must(tree.RootNode().NamedChild(0))
		actual := ""
		if child, ok := node.ChildByFieldName(c.fieldName); ok {
			actual = c.source[child.StartByte():child.EndByte()]
		}
		urEqual(t, "`"+c.fieldName+"` of `"+c.source+"`", actual, c.expected)
	}
}

func TestNodeFieldNameForChildThroughHiddenExtras(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, urTestFixtureLanguage(t, "field_lookup_through_hidden_nodes"))

	// The tree cursor gives the nodes inside a hidden extra only the fields set
	// inside it, not the field of the child before it or of a rule around it.
	// An empty string stands for no field.
	for _, c := range []struct {
		source      string
		fields      []string
		namedFields []string
	}{
		{
			"let a @x = b;",
			[]string{"", "inner", "", "outer", "outer", "outer", ""},
			[]string{"inner", "outer", "outer"},
		},
		{
			"let a #y = b;",
			[]string{"", "inner", "", "", "outer", "outer", ""},
			[]string{"inner", "", "outer"},
		},
	} {
		tree := urParse(t, parser, c.source, nil)
		binding := must(tree.RootNode().NamedChild(0))
		var actual []string
		for i := range binding.ChildCount() {
			actual = append(actual, binding.FieldNameForChild(i))
		}
		urSlice(t, "`"+c.source+"`", actual, c.fields)
		actual = nil
		for i := range binding.NamedChildCount() {
			actual = append(actual, binding.FieldNameForNamedChild(i))
		}
		urSlice(t, "named children of `"+c.source+"`", actual, c.namedFields)
	}
}

func TestNodeNamedChild(t *testing.T) {
	must := urMust(t)
	tree := urParseJSONExample(t)
	arrayNode := must(tree.RootNode().Child(0))
	find := func(s string) int { return strings.Index(urJSONExample, s) }

	numberNode := must(arrayNode.NamedChild(0))
	falseNode := must(arrayNode.NamedChild(1))
	objectNode := must(arrayNode.NamedChild(2))

	urEqual(t, "number_node.kind", numberNode.Kind(), "number")
	urNodeAt(t, "number_node", numberNode, find("123"), find("123")+3, urPoint(3, 2), urPoint(3, 5))

	urEqual(t, "false_node.kind", falseNode.Kind(), "false")
	urNodeAt(t, "false_node", falseNode, find("false"), find("false")+5, urPoint(4, 2), urPoint(4, 7))

	urEqual(t, "object_node.kind", objectNode.Kind(), "object")
	urEqual(t, "object_node.start_byte", objectNode.StartByte(), find("{"))
	urEqual(t, "object_node.start_position", objectNode.StartPoint(), urPoint(5, 2))
	urEqual(t, "object_node.end_position", objectNode.EndPoint(), urPoint(7, 3))

	urEqual(t, "object_node.named_child_count", objectNode.NamedChildCount(), 1)

	pairNode := must(objectNode.NamedChild(0))
	urEqual(t, "pair_node.kind", pairNode.Kind(), "pair")
	urNodeAt(t, "pair_node", pairNode, find(`"x"`), find("null")+4, urPoint(6, 4), urPoint(6, 13))

	stringNode := must(pairNode.NamedChild(0))
	nullNode := must(pairNode.NamedChild(1))

	urEqual(t, "string_node.kind", stringNode.Kind(), "string")
	urEqual(t, "null_node.kind", nullNode.Kind(), "null")

	urNodeAt(t, "string_node", stringNode, find(`"x"`), find(`"x"`)+3, urPoint(6, 4), urPoint(6, 7))
	urNodeAt(t, "null_node", nullNode, find("null"), find("null")+4, urPoint(6, 9), urPoint(6, 13))

	urParentIs(t, "string_node", stringNode, pairNode)
	urParentIs(t, "null_node", nullNode, pairNode)
	urParentIs(t, "pair_node", pairNode, objectNode)
	urParentIs(t, "number_node", numberNode, arrayNode)
	urParentIs(t, "false_node", falseNode, arrayNode)
	urParentIs(t, "object_node", objectNode, arrayNode)
	urParentIs(t, "array_node", arrayNode, tree.RootNode())
	if _, ok := tree.RootNode().Parent(); ok {
		t.Error("the root node has a parent")
	}

	urChildWithDescendantIs(t, "root", tree.RootNode(), nullNode, arrayNode)
	urChildWithDescendantIs(t, "array_node", arrayNode, nullNode, objectNode)
	urChildWithDescendantIs(t, "object_node", objectNode, nullNode, pairNode)
	urChildWithDescendantIs(t, "pair_node", pairNode, nullNode, nullNode)
	if _, ok := nullNode.ChildWithDescendant(nullNode); ok {
		t.Error("null_node.child_with_descendant(null_node) is not None")
	}
}

func TestNodeNamedChildWithAliasesAndExtras(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, urTestLanguage(t, urGrammarWithAliasesAndExtras, ""))

	tree := urParse(t, parser, "b ... b ... c", nil)
	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(), "(a (b) (comment) (B) (comment) (C))")
	urEqual(t, "named_child_count", root.NamedChildCount(), 5)
	urEqual(t, "named_child(0)", must(root.NamedChild(0)).Kind(), "b")
	urEqual(t, "named_child(1)", must(root.NamedChild(1)).Kind(), "comment")
	urEqual(t, "named_child(2)", must(root.NamedChild(2)).Kind(), "B")
	urEqual(t, "named_child(3)", must(root.NamedChild(3)).Kind(), "comment")
	urEqual(t, "named_child(4)", must(root.NamedChild(4)).Kind(), "C")
}

func TestNodeDescendantCount(t *testing.T) {
	tree := urParseJSONExample(t)
	valueNode := tree.RootNode()
	allNodes := urGetAllNodes(tree)

	urEqual(t, "descendant_count", valueNode.DescendantCount(), len(allNodes))

	cursor := valueNode.Walk()
	for i, node := range allNodes {
		cursor.GotoDescendant(i)
		urSameNode(t, fmt.Sprintf("index %d", i), cursor.Node(), node)
	}

	for i, node := range slices.Backward(allNodes) {
		cursor.GotoDescendant(i)
		urSameNode(t, fmt.Sprintf("rev index %d", i), cursor.Node(), node)
	}
}

func TestDescendantCountSingleNodeTree(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "embedded_template").Language)
	tree := urParse(t, parser, "hello", nil)

	nodes := urGetAllNodes(tree)
	urEqual(t, "nodes.len", len(nodes), 2)
	urEqual(t, "descendant_count", tree.RootNode().DescendantCount(), 2)

	cursor := tree.RootNode().Walk()

	cursor.GotoDescendant(0)
	urEqual(t, "depth", cursor.Depth(), 0)
	urSameNode(t, "node", cursor.Node(), nodes[0])
	cursor.GotoDescendant(1)
	urEqual(t, "depth", cursor.Depth(), 1)
	urSameNode(t, "node", cursor.Node(), nodes[1])
}

func TestNodeDescendantForRange(t *testing.T) {
	must := urMust(t)
	tree := urParseJSONExample(t)
	arrayNode := tree.RootNode()

	// Leaf node exactly matches the given bounds - byte query
	colonIndex := strings.Index(urJSONExample, ":")
	colonNode := must(arrayNode.DescendantForByteRange(colonIndex, colonIndex+1))
	urEqual(t, "colon_node.kind", colonNode.Kind(), ":")
	urNodeAt(t, "colon_node", colonNode, colonIndex, colonIndex+1, urPoint(6, 7), urPoint(6, 8))

	// Leaf node exactly matches the given bounds - point query
	colonNode = must(arrayNode.DescendantForPointRange(urPoint(6, 7), urPoint(6, 8)))
	urEqual(t, "colon_node.kind", colonNode.Kind(), ":")
	urNodeAt(t, "colon_node", colonNode, colonIndex, colonIndex+1, urPoint(6, 7), urPoint(6, 8))

	// The given point is between two adjacent leaf nodes - byte query
	colonIndex = strings.Index(urJSONExample, ":")
	colonNode = must(arrayNode.DescendantForByteRange(colonIndex, colonIndex))
	urEqual(t, "colon_node.kind", colonNode.Kind(), ":")
	urNodeAt(t, "colon_node", colonNode, colonIndex, colonIndex+1, urPoint(6, 7), urPoint(6, 8))

	// The given point is between two adjacent leaf nodes - point query
	colonNode = must(arrayNode.DescendantForPointRange(urPoint(6, 7), urPoint(6, 7)))
	urEqual(t, "colon_node.kind", colonNode.Kind(), ":")
	urNodeAt(t, "colon_node", colonNode, colonIndex, colonIndex+1, urPoint(6, 7), urPoint(6, 8))

	// Leaf node starts at the lower bound, ends after the upper bound - byte query
	stringIndex := strings.Index(urJSONExample, `"x"`)
	stringNode := must(arrayNode.DescendantForByteRange(stringIndex, stringIndex+2))
	urEqual(t, "string_node.kind", stringNode.Kind(), "string")
	urNodeAt(t, "string_node", stringNode, stringIndex, stringIndex+3, urPoint(6, 4), urPoint(6, 7))

	// Leaf node starts at the lower bound, ends after the upper bound - point query
	stringNode = must(arrayNode.DescendantForPointRange(urPoint(6, 4), urPoint(6, 6)))
	urEqual(t, "string_node.kind", stringNode.Kind(), "string")
	urNodeAt(t, "string_node", stringNode, stringIndex, stringIndex+3, urPoint(6, 4), urPoint(6, 7))

	// Leaf node starts before the lower bound, ends at the upper bound - byte query
	nullIndex := strings.Index(urJSONExample, "null")
	nullNode := must(arrayNode.DescendantForByteRange(nullIndex+1, nullIndex+4))
	urEqual(t, "null_node.kind", nullNode.Kind(), "null")
	urNodeAt(t, "null_node", nullNode, nullIndex, nullIndex+4, urPoint(6, 9), urPoint(6, 13))

	// Leaf node starts before the lower bound, ends at the upper bound - point query
	nullNode = must(arrayNode.DescendantForPointRange(urPoint(6, 11), urPoint(6, 13)))
	urEqual(t, "null_node.kind", nullNode.Kind(), "null")
	urNodeAt(t, "null_node", nullNode, nullIndex, nullIndex+4, urPoint(6, 9), urPoint(6, 13))

	// The bounds span multiple leaf nodes - return the smallest node that does span it.
	pairNode := must(arrayNode.DescendantForByteRange(stringIndex+2, stringIndex+4))
	urEqual(t, "pair_node.kind", pairNode.Kind(), "pair")
	urNodeAt(t, "pair_node", pairNode, stringIndex, stringIndex+9, urPoint(6, 4), urPoint(6, 13))

	urParentIs(t, "colon_node", colonNode, pairNode)

	// no leaf spans the given range - return the smallest node that does span it.
	pairNode = must(arrayNode.NamedDescendantForPointRange(urPoint(6, 6), urPoint(6, 8)))
	urEqual(t, "pair_node.kind", pairNode.Kind(), "pair")
	urNodeAt(t, "pair_node", pairNode, stringIndex, stringIndex+9, urPoint(6, 4), urPoint(6, 13))

	// Zero-width token
	{
		code := "<script></script>"
		parser := urParser(t, fixtureGrammar(t, "html").Language)

		tree := urParse(t, parser, code, nil)
		root := tree.RootNode()

		child := must(root.NamedDescendantForPointRange(urPoint(0, 8), urPoint(0, 8)))
		urEqual(t, "child.kind", child.Kind(), "raw_text")

		child2 := must(root.NamedDescendantForByteRange(8, 8))
		urEqual(t, "child2.kind", child2.Kind(), "raw_text")

		urSameNode(t, "child", child, child2)
	}

	// Negative test, start > end
	if _, ok := arrayNode.DescendantForByteRange(1, 0); ok {
		t.Error("descendant_for_byte_range(1, 0) is not None")
	}
	if _, ok := arrayNode.DescendantForPointRange(urPoint(6, 8), urPoint(6, 7)); ok {
		t.Error("descendant_for_point_range((6, 8), (6, 7)) is not None")
	}
}

func TestNodeEdit(t *testing.T) {
	code := []byte(urJSONExample)
	tree := urParseJSONExample(t)
	rand := urNewRand(0)

	for range 10 {
		nodesBefore := urGetAllNodes(tree)

		edit := urGetRandomEdit(rand, code)
		tree2 := tree.Copy()
		e := urPerformEdit(t, tree2, &code, edit)
		for i := range nodesBefore {
			nodesBefore[i].Edit(e)
		}

		nodesAfter := urGetAllNodes(tree2)
		for i, node := range nodesBefore {
			type kindAt struct {
				kind  string
				start int
				at    transit.Point
			}
			urEqual(t, fmt.Sprintf("node %d", i),
				kindAt{node.Kind(), node.StartByte(), node.StartPoint()},
				kindAt{nodesAfter[i].Kind(), nodesAfter[i].StartByte(), nodesAfter[i].StartPoint()})
		}

		tree = tree2
	}
}

func TestRootNodeWithOffset(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "  if (a) b", nil)

	node := tree.RootNodeWithOffset(6, urPoint(2, 2))
	urEqual(t, "node.byte_range", urByteRange(node), [2]int{8, 16})
	urEqual(t, "node.start_position", node.StartPoint(), urPoint(2, 4))
	urEqual(t, "node.end_position", node.EndPoint(), urPoint(2, 12))

	child := must(must(node.Child(0)).Child(2))
	urEqual(t, "child.kind", child.Kind(), "expression_statement")
	urEqual(t, "child.byte_range", urByteRange(child), [2]int{15, 16})
	urEqual(t, "child.start_position", child.StartPoint(), urPoint(2, 11))
	urEqual(t, "child.end_position", child.EndPoint(), urPoint(2, 12))

	cursor := node.Walk()
	cursor.GotoFirstChild()
	cursor.GotoFirstChild()
	cursor.GotoNextSibling()
	child = cursor.Node()
	urEqual(t, "child.kind", child.Kind(), "parenthesized_expression")
	urEqual(t, "child.byte_range", urByteRange(child), [2]int{11, 14})
	urEqual(t, "child.start_position", child.StartPoint(), urPoint(2, 7))
	urEqual(t, "child.end_position", child.EndPoint(), urPoint(2, 10))
}

func TestNodeIsExtra(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "foo(/* hi */);", nil)

	rootNode := tree.RootNode()
	commentNode := must(rootNode.DescendantForByteRange(7, 7))

	urEqual(t, "root_node.kind", rootNode.Kind(), "program")
	urEqual(t, "comment_node.kind", commentNode.Kind(), "comment")
	urEqual(t, "root_node.is_extra", rootNode.IsExtra(), false)
	urEqual(t, "comment_node.is_extra", commentNode.IsExtra(), true)
}

func TestNodeIsError(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "foo(", nil)
	rootNode := tree.RootNode()
	urEqual(t, "root_node.kind", rootNode.Kind(), "program")
	urEqual(t, "root_node.has_error", rootNode.HasError(), true)

	child := must(rootNode.Child(0))
	urEqual(t, "child.kind", child.Kind(), "ERROR")
	urEqual(t, "child.is_error", child.IsError(), true)
}

func TestNodeHasError(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "json").Language)
	tree := urParse(t, parser, "!", nil)

	root := tree.RootNode()
	urEqual(t, "root.has_error", root.HasError(), true)

	errorNode := must(root.Child(0))
	urEqual(t, "error.is_error", errorNode.IsError(), true)
	urEqual(t, "error.has_error", errorNode.HasError(), true)

	terminalError := must(errorNode.Child(0))
	urEqual(t, "terminal_error.is_error", terminalError.IsError(), true)
	urEqual(t, "terminal_error.child_count", terminalError.ChildCount(), 0)
	urEqual(t, "terminal_error.has_error", terminalError.HasError(), true)
}

func TestEditPoint(t *testing.T) {
	edit := transit.InputEdit{
		StartByte:   5,
		OldEndByte:  5,
		NewEndByte:  10,
		StartPoint:  urPoint(0, 5),
		OldEndPoint: urPoint(0, 5),
		NewEndPoint: urPoint(0, 10),
	}

	// Point after edit
	point := urPoint(0, 8)
	byteOffset := 8
	edit.EditPoint(&point, &byteOffset)
	urEqual(t, "point", point, urPoint(0, 13))
	urEqual(t, "byte", byteOffset, 13)

	// Point before edit
	point = urPoint(0, 2)
	byteOffset = 2
	edit.EditPoint(&point, &byteOffset)
	urEqual(t, "point", point, urPoint(0, 2))
	urEqual(t, "byte", byteOffset, 2)

	// Point at edit start
	point = urPoint(0, 5)
	byteOffset = 5
	edit.EditPoint(&point, &byteOffset)
	urEqual(t, "point", point, urPoint(0, 10))
	urEqual(t, "byte", byteOffset, 10)
}

func TestEditRange(t *testing.T) {
	edit := transit.InputEdit{
		StartByte:   10,
		OldEndByte:  15,
		NewEndByte:  20,
		StartPoint:  urPoint(1, 0),
		OldEndPoint: urPoint(1, 5),
		NewEndPoint: urPoint(2, 0),
	}

	// Range after edit
	r := transit.Range{
		StartByte:  20,
		EndByte:    25,
		StartPoint: urPoint(2, 0),
		EndPoint:   urPoint(2, 5),
	}
	edit.EditRange(&r)
	urEqual(t, "range.start_byte", r.StartByte, 25)
	urEqual(t, "range.end_byte", r.EndByte, 30)
	urEqual(t, "range.start_point", r.StartPoint, urPoint(3, 0))
	urEqual(t, "range.end_point", r.EndPoint, urPoint(3, 5))

	// Range before edit
	r = transit.Range{
		StartByte:  5,
		EndByte:    8,
		StartPoint: urPoint(0, 5),
		EndPoint:   urPoint(0, 8),
	}
	edit.EditRange(&r)
	urEqual(t, "range.start_byte", r.StartByte, 5)
	urEqual(t, "range.end_byte", r.EndByte, 8)
	urEqual(t, "range.start_point", r.StartPoint, urPoint(0, 5))
	urEqual(t, "range.end_point", r.EndPoint, urPoint(0, 8))

	// Range overlapping edit
	r = transit.Range{
		StartByte:  8,
		EndByte:    12,
		StartPoint: urPoint(0, 8),
		EndPoint:   urPoint(1, 2),
	}
	edit.EditRange(&r)
	urEqual(t, "range.start_byte", r.StartByte, 8)
	urEqual(t, "range.end_byte", r.EndByte, 10)
	urEqual(t, "range.start_point", r.StartPoint, urPoint(0, 8))
	urEqual(t, "range.end_point", r.EndPoint, urPoint(1, 0))
}

func TestNodeSexp(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, "if (a) b", nil)
	rootNode := tree.RootNode()
	ifNode := must(rootNode.DescendantForByteRange(0, 0))
	parenNode := must(rootNode.DescendantForByteRange(3, 3))
	identifierNode := must(rootNode.DescendantForByteRange(4, 4))
	urEqual(t, "if_node.kind", ifNode.Kind(), "if")
	urEqual(t, "if_node.to_sexp", ifNode.String(), `("if")`)
	urEqual(t, "paren_node.kind", parenNode.Kind(), "(")
	urEqual(t, "paren_node.to_sexp", parenNode.String(), `("(")`)
	urEqual(t, "identifier_node.kind", identifierNode.Kind(), "identifier")
	urEqual(t, "identifier_node.to_sexp", identifierNode.String(), "(identifier)")
}

func TestNodeFieldNames(t *testing.T) {
	must := urMust(t)
	// - "x":
	//      This isn't used in the test, but prevents `_hidden_rule1` from being eliminated as a
	//      unit reduction.
	// - "_hidden_rule1":
	//      Fields pointing to hidden nodes with a single child resolve to the child.
	// - "_hidden_rule2":
	//      Fields within hidden nodes can be referenced through the parent node.
	language := urTestLanguage(t, `
        {
            "name": "test_grammar_with_fields",
            "extras": [
                {"type": "PATTERN", "value": "\\s+"}
            ],
            "rules": {
                "rule_a": {
                    "type": "SEQ",
                    "members": [
                        {
                            "type": "FIELD",
                            "name": "field_1",
                            "content": {"type": "STRING", "value": "child-0"}
                        },
                        {
                            "type": "CHOICE",
                            "members": [
                                {"type": "STRING", "value": "child-1"},
                                {"type": "BLANK"},

                                {
                                    "type": "ALIAS",
                                    "value": "x",
                                    "named": true,
                                    "content": {
                                        "type": "SYMBOL",
                                        "name": "_hidden_rule1"
                                    }
                                }
                            ]
                        },
                        {
                            "type": "FIELD",
                            "name": "field_2",
                            "content": {"type": "SYMBOL", "name": "_hidden_rule1"}
                        },
                        {"type": "SYMBOL", "name": "_hidden_rule2"}
                    ]
                },

                "_hidden_rule1": {
                    "type": "CHOICE",
                    "members": [
                        {"type": "STRING", "value": "child-2"},
                        {"type": "STRING", "value": "child-2.5"}
                    ]
                },

                "_hidden_rule2": {
                    "type": "SEQ",
                    "members": [
                        {"type": "STRING", "value": "child-3"},
                        {
                            "type": "FIELD",
                            "name": "field_3",
                            "content": {"type": "STRING", "value": "child-4"}
                        }
                    ]
                }
            }
        }
    `, "")

	parser := urParser(t, language)

	tree := urParse(t, parser, "child-0 child-1 child-2 child-3 child-4", nil)
	rootNode := tree.RootNode()

	urSameNode(t, "child_by_field_name(field_1)", must(rootNode.ChildByFieldName("field_1")), must(rootNode.Child(0)))
	urSameNode(t, "child_by_field_name(field_2)", must(rootNode.ChildByFieldName("field_2")), must(rootNode.Child(2)))
	urSameNode(t, "child_by_field_name(field_3)", must(rootNode.ChildByFieldName("field_3")), must(rootNode.Child(4)))
	if _, ok := must(rootNode.Child(0)).ChildByFieldName("field_1"); ok {
		t.Error("child(0).child_by_field_name(field_1) is not None")
	}
	if _, ok := rootNode.ChildByFieldName("not_a_real_field"); ok {
		t.Error("child_by_field_name(not_a_real_field) is not None")
	}

	cursor := rootNode.Walk()
	urEqual(t, "field_name", cursor.FieldName(), "")
	cursor.GotoFirstChild()
	urEqual(t, "kind", cursor.Node().Kind(), "child-0")
	urEqual(t, "field_name", cursor.FieldName(), "field_1")
	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "child-1")
	urEqual(t, "field_name", cursor.FieldName(), "")
	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "child-2")
	urEqual(t, "field_name", cursor.FieldName(), "field_2")
	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "child-3")
	urEqual(t, "field_name", cursor.FieldName(), "")
	cursor.GotoNextSibling()
	urEqual(t, "kind", cursor.Node().Kind(), "child-4")
	urEqual(t, "field_name", cursor.FieldName(), "field_3")
}

func TestNodeFieldCallsInLanguageWithoutFields(t *testing.T) {
	language := urTestLanguage(t, `
        {
            "name": "test_grammar_with_no_fields",
            "extras": [
                {"type": "PATTERN", "value": "\\s+"}
            ],
            "rules": {
                "a": {
                    "type": "SEQ",
                    "members": [
                        {
                            "type": "STRING",
                            "value": "b"
                        },
                        {
                            "type": "STRING",
                            "value": "c"
                        },
                        {
                            "type": "STRING",
                            "value": "d"
                        }
                    ]
                }
            }
        }
    `, "")

	parser := urParser(t, language)

	tree := urParse(t, parser, "b c d", nil)

	rootNode := tree.RootNode()
	urEqual(t, "kind", rootNode.Kind(), "a")
	if _, ok := rootNode.ChildByFieldName("something"); ok {
		t.Error("child_by_field_name(something) is not None")
	}

	cursor := rootNode.Walk()
	urEqual(t, "field_name", cursor.FieldName(), "")
	urEqual(t, "goto_first_child", cursor.GotoFirstChild(), true)
	urEqual(t, "field_name", cursor.FieldName(), "")
}

// TestNodeIsNamedButAliasedAsAnonymous reads the grammar.json that the
// golden harness writes from the grammar.js of the test grammar, and builds
// it with no scanner, as upstream does.
func TestNodeIsNamedButAliasedAsAnonymous(t *testing.T) {
	must := urMust(t)
	root, _ := setup(t)
	jsonDir, _ := urTestGrammarDirs(root, "named_rule_aliased_as_anonymous")
	grammarJSON, err := os.ReadFile(filepath.Join(jsonDir, "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}

	language := urTestLanguage(t, string(grammarJSON), "")
	parser := urParser(t, language)

	tree := urParse(t, parser, "B C B", nil)

	rootNode := tree.RootNode()
	urEqual(t, "has_error", rootNode.HasError(), false)
	urEqual(t, "child_count", rootNode.ChildCount(), 3)
	urEqual(t, "named_child_count", rootNode.NamedChildCount(), 2)

	aliased := must(rootNode.Child(0))
	urEqual(t, "aliased.is_named", aliased.IsNamed(), false)
	urEqual(t, "aliased.kind", aliased.Kind(), "the-alias")

	urEqual(t, "named_child(0).kind", must(rootNode.NamedChild(0)).Kind(), "c")
}

func TestNodeNumericSymbolsRespectSimpleAliases(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "python").Language)

	// Example 1:
	// Python argument lists can contain "splat" arguments, which are not allowed
	// within other expressions. This includes `parenthesized_list_splat` nodes
	// like `(*b)`. These `parenthesized_list_splat` nodes are aliased as
	// `parenthesized_expression`. Their numeric `symbol`, aka `kind_id` should
	// match that of a normal `parenthesized_expression`.
	tree := urParse(t, parser, "(a((*b)))", nil)
	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(),
		"(module (expression_statement (parenthesized_expression (call function: (identifier) arguments: (argument_list (parenthesized_expression (list_splat (identifier))))))))")

	outerExprNode := must(must(root.Child(0)).Child(0))
	urEqual(t, "outer_expr_node.kind", outerExprNode.Kind(), "parenthesized_expression")

	innerExprNode := must(must(must(outerExprNode.NamedChild(0)).ChildByFieldName("arguments")).NamedChild(0))
	urEqual(t, "inner_expr_node.kind", innerExprNode.Kind(), "parenthesized_expression")
	urEqual(t, "inner_expr_node.kind_id", innerExprNode.KindID(), outerExprNode.KindID())

	// Example 2:
	// Ruby handles the unary (negative) and binary (minus) `-` operators using two
	// different tokens. One or more of these is an external token that's
	// aliased as `-`. Their numeric kind ids should match.
	if err := parser.SetLanguage(fixtureGrammar(t, "ruby").Language); err != nil {
		t.Fatal(err)
	}
	tree = urParse(t, parser, "-a - b", nil)
	root = tree.RootNode()
	urEqual(t, "to_sexp", root.String(),
		"(program (binary left: (unary operand: (identifier)) right: (identifier)))")

	binaryNode := must(root.Child(0))
	urEqual(t, "binary_node.kind", binaryNode.Kind(), "binary")

	unaryMinusNode := must(must(binaryNode.ChildByFieldName("left")).Child(0))
	urEqual(t, "unary_minus_node.kind", unaryMinusNode.Kind(), "-")

	binaryMinusNode := must(binaryNode.ChildByFieldName("operator"))
	urEqual(t, "binary_minus_node.kind", binaryMinusNode.Kind(), "-")
	urEqual(t, "unary_minus_node.kind_id", unaryMinusNode.KindID(), binaryMinusNode.KindID())
}

func TestHiddenZeroWidthNodeWithVisibleChild(t *testing.T) {
	must := urMust(t)
	code := `
class Foo {
  std::
private:
  std::string s;
};
`

	parser := urParser(t, fixtureGrammar(t, "cpp").Language)
	tree := urParse(t, parser, code, nil)
	root := tree.RootNode()

	classSpecifier := must(root.Child(0))
	fieldDeclList := must(classSpecifier.ChildByFieldName("body"))
	fieldDecl := must(fieldDeclList.NamedChild(0))
	fieldIdent := must(fieldDecl.ChildByFieldName("declarator"))
	urChildWithDescendantIs(t, "field_decl", fieldDecl, fieldIdent, fieldIdent)
}

// urGetAllNodes is get_all_nodes of node_test.rs.
func urGetAllNodes(tree *transit.Tree) []transit.Node {
	var result []transit.Node
	visitedChildren := false
	cursor := tree.Walk()
	for {
		switch {
		case !visitedChildren:
			result = append(result, cursor.Node())
			if !cursor.GotoFirstChild() {
				visitedChildren = true
			}
		case cursor.GotoNextSibling():
			visitedChildren = false
		case !cursor.GotoParent():
			return result
		}
	}
}

// urParseJSONExample is parse_json_example of node_test.rs.
func urParseJSONExample(t *testing.T) *transit.Tree {
	t.Helper()
	parser := urParser(t, fixtureGrammar(t, "json").Language)
	return urParse(t, parser, urJSONExample, nil)
}

// urParseDeeplyNestedTree is parse_deeply_nested_tree of node_test.rs.
func urParseDeeplyNestedTree(t *testing.T, depth int) *transit.Tree {
	t.Helper()
	language := urTestLanguage(t, `{
            "name": "deeply_nested",
            "rules": {
                "document": {"type": "SYMBOL", "name": "expression"},
                "expression": {
                    "type": "CHOICE",
                    "members": [
                        {"type": "STRING", "value": "x"},
                        {
                            "type": "SEQ",
                            "members": [
                                {"type": "STRING", "value": "("},
                                {"type": "SYMBOL", "name": "expression"},
                                {"type": "STRING", "value": ")"}
                            ]
                        }
                    ]
                }
            }
        }`, "")
	source := strings.Repeat("(", depth) + "x" + strings.Repeat(")", depth)
	parser := urParser(t, language)
	return urParse(t, parser, source, nil)
}

// urCollapseDeeplyNestedTree is collapse_deeply_nested_tree of
// node_test.rs.
func urCollapseDeeplyNestedTree(tree *transit.Tree, depth int) {
	oldEndByte := depth*2 + 1
	tree.Edit(transit.InputEdit{
		StartByte:   0,
		OldEndByte:  oldEndByte,
		NewEndByte:  0,
		StartPoint:  urPoint(0, 0),
		OldEndPoint: urPoint(0, oldEndByte),
		NewEndPoint: urPoint(0, 0),
	})
}

// urSomeNode checks that a lookup found a node, and that the node is want.
// It is assert_eq!(got, Some(want)).
func urSomeNode(t *testing.T, what string, got transit.Node, ok bool, want transit.Node) {
	t.Helper()
	if !ok {
		t.Errorf("%s is None", what)
		return
	}
	urSameNode(t, what, got, want)
}

// TestDeeplyNestedDotGraphDoesNotOverflowStack writes the graph to
// io.Discard, the Go form of the null device of upstream.
func TestDeeplyNestedDotGraphDoesNotOverflowStack(t *testing.T) {
	tree := urParseDeeplyNestedTree(t, 100_000)
	tree.PrintDotGraph(io.Discard)
}

func TestPreviousSiblingOfIdenticalZeroWidthLeaves(t *testing.T) {
	must := urMust(t)
	for _, depth := range []int{3, 63, 64, 65, 66} {
		tree := urParseDeeplyNestedTree(t, depth)
		urCollapseDeeplyNestedTree(tree, depth)
		expression := must(tree.RootNode().NamedChild(0))
		innerExpression := must(expression.NamedChild(0))
		closingParenthesis := must(expression.Child(expression.ChildCount() - 1))

		// Different closing parentheses can share inline data after the edit.
		got, ok := closingParenthesis.PrevSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: prev_sibling()", depth), got, ok, innerExpression)
		got, ok = closingParenthesis.PrevNamedSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: prev_named_sibling()", depth), got, ok, innerExpression)
	}
}

func TestPreviousSiblingOfZeroWidthNodesInHiddenRules(t *testing.T) {
	must := urMust(t)
	language := urTestLanguage(t, `{
            "name": "deeply_nested_hidden_siblings",
            "rules": {
                "document": {"type": "SYMBOL", "name": "_expression"},
                "_expression": {
                    "type": "CHOICE",
                    "members": [
                        {"type": "STRING", "value": "x"},
                        {
                            "type": "SEQ",
                            "members": [
                                {"type": "STRING", "value": "("},
                                {"type": "SYMBOL", "name": "_expression"},
                                {"type": "STRING", "value": ")"}
                            ]
                        }
                    ]
                }
            }
        }`, "")
	parser := urParser(t, language)

	for _, depth := range []int{3, 63, 64, 65, 66, 128} {
		source := strings.Repeat("(", depth) + "x" + strings.Repeat(")", depth)
		tree := urParse(t, parser, source, nil)
		urCollapseDeeplyNestedTree(tree, depth)
		root := tree.RootNode()
		xIndex := depth
		openingParenthesis := must(root.Child(xIndex - 1))
		x := must(root.Child(xIndex))
		firstClosingParenthesis := must(root.Child(xIndex + 1))
		lastClosingParenthesis := must(root.Child(root.ChildCount() - 1))
		previousClosingParenthesis := must(root.Child(root.ChildCount() - 2))

		got, ok := x.PrevSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: x.prev_sibling()", depth), got, ok, openingParenthesis)
		got, ok = firstClosingParenthesis.PrevSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: first_closing_parenthesis.prev_sibling()", depth), got, ok, x)
		got, ok = lastClosingParenthesis.PrevSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: last_closing_parenthesis.prev_sibling()", depth), got, ok, previousClosingParenthesis)
	}
}

func TestDeeplyNestedNodeSearchesDoNotOverflowStack(t *testing.T) {
	must := urMust(t)
	const depth = 100_000
	tree := urParseDeeplyNestedTree(t, depth)
	urCollapseDeeplyNestedTree(tree, depth)
	root := tree.RootNode()
	expression := must(root.NamedChild(0))
	descendant := expression
	for {
		child, ok := descendant.NamedChild(0)
		if !ok {
			break
		}
		descendant = child
	}
	innerExpression := must(expression.NamedChild(0))
	closingParenthesis := must(expression.Child(expression.ChildCount() - 1))

	got, ok := root.ChildWithDescendant(descendant)
	urSomeNode(t, "root.child_with_descendant()", got, ok, expression)
	urEqual(t, "closing_parenthesis.kind()", closingParenthesis.Kind(), ")")
	got, ok = closingParenthesis.PrevSibling()
	urSomeNode(t, "closing_parenthesis.prev_sibling()", got, ok, innerExpression)
	got, ok = closingParenthesis.PrevNamedSibling()
	urSomeNode(t, "closing_parenthesis.prev_named_sibling()", got, ok, innerExpression)
}

func TestDeeplyNestedTrailingEmptyDescendantSearch(t *testing.T) {
	must := urMust(t)
	language := urTestLanguage(t, `{
            "name": "deeply_nested_with_marker",
            "rules": {
                "document": {
                    "type": "SEQ",
                    "members": [
                        {"type": "SYMBOL", "name": "expression"},
                        {"type": "SYMBOL", "name": "marker"}
                    ]
                },
                "expression": {
                    "type": "CHOICE",
                    "members": [
                        {"type": "STRING", "value": "x"},
                        {
                            "type": "SEQ",
                            "members": [
                                {"type": "STRING", "value": "("},
                                {"type": "SYMBOL", "name": "expression"},
                                {"type": "STRING", "value": ")"}
                            ]
                        }
                    ]
                },
                "marker": {
                    "type": "SEQ",
                    "members": [
                        {"type": "STRING", "value": "a"},
                        {"type": "STRING", "value": "b"}
                    ]
                }
            }
        }`, "")
	parser := urParser(t, language)

	for _, depth := range []int{63, 64, 65, 100_000} {
		source := strings.Repeat("(", depth) + "x" + strings.Repeat(")", depth) + "ab"
		tree := urParse(t, parser, source, nil)
		tree.Edit(transit.InputEdit{
			StartByte:   0,
			OldEndByte:  len(source),
			NewEndByte:  0,
			StartPoint:  urPoint(0, 0),
			OldEndPoint: urPoint(0, len(source)),
			NewEndPoint: urPoint(0, 0),
		})
		root := tree.RootNode()
		expression := must(root.NamedChild(0))
		marker := must(root.NamedChild(1))

		// A non-leaf marker forces the search through the preceding empty subtree.
		urEqual(t, "marker.child_count()", marker.ChildCount(), 2)
		got, ok := marker.PrevSibling()
		urSomeNode(t, fmt.Sprintf("depth %d: marker.prev_sibling()", depth), got, ok, expression)
	}
}

func TestDeeplyNestedFieldSearchDoesNotOverflowStack(t *testing.T) {
	must := urMust(t)
	language := urTestLanguage(t, `{
            "name": "deeply_nested_fields",
            "rules": {
                "document": {"type": "SYMBOL", "name": "_nested"},
                "_nested": {
                    "type": "CHOICE",
                    "members": [
                        {
                            "type": "FIELD",
                            "name": "target",
                            "content": {"type": "STRING", "value": "x"}
                        },
                        {
                            "type": "SEQ",
                            "members": [
                                {"type": "STRING", "value": "("},
                                {"type": "SYMBOL", "name": "_nested"},
                                {"type": "STRING", "value": ")"},
                                {
                                    "type": "FIELD",
                                    "name": "target",
                                    "content": {"type": "STRING", "value": "z"}
                                }
                            ]
                        }
                    ]
                }
            }
        }`, "")
	const depth = 100_000
	source := strings.Repeat("(", depth) + "x" + strings.Repeat(")z", depth)
	parser := urParser(t, language)
	tree := urParse(t, parser, source, nil)
	target := must(tree.RootNode().ChildByFieldName("target"))

	urEqual(t, "the text of target", source[target.StartByte():target.EndByte()], "x")
}
