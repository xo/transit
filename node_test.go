package transit

import (
	"slices"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// nodeKinds returns the kinds of a sequence of nodes.
func nodeKinds(nodes func(func(Node) bool)) []string {
	var kinds []string
	for n := range nodes {
		kinds = append(kinds, n.Kind())
	}
	return kinds
}

// nodeAt returns the node for a byte range of the sample tree, and fails
// the test when there is none.
func nodeAt(t *testing.T, n Node, start, end int) Node {
	t.Helper()
	result, ok := n.DescendantForByteRange(start, end)
	if !ok {
		t.Fatalf("no node for %d - %d", start, end)
	}
	return result
}

func TestNodeChildren(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	if root.ChildCount() != 6 || root.NamedChildCount() != 5 {
		t.Fatalf("the root has %d children and %d named children", root.ChildCount(), root.NamedChildCount())
	}

	// the children of the hidden nodes are children of the root
	for i, want := range []struct {
		kind       string
		start, end int
		named      bool
		extra      bool
	}{
		{"identifier", 0, 1, true, false},
		{"identifier", 2, 3, true, false},
		{"+", 4, 5, false, false},
		{"expression", 6, 11, true, false},
		{"identifier", 12, 13, true, true},
		{"identifier", 14, 15, true, false},
	} {
		child, ok := root.Child(i)
		if !ok {
			t.Fatalf("Child(%d) found nothing", i)
		}
		if child.Kind() != want.kind || child.StartByte() != want.start || child.EndByte() != want.end ||
			child.IsNamed() != want.named || child.IsExtra() != want.extra {
			t.Errorf("Child(%d) is %s at %d - %d, named %t, extra %t",
				i, child.Kind(), child.StartByte(), child.EndByte(), child.IsNamed(), child.IsExtra())
		}
		if child.StartPoint() != (Point{Column: want.start}) || child.EndPoint() != (Point{Column: want.end}) {
			t.Errorf("Child(%d) has the points %v - %v", i, child.StartPoint(), child.EndPoint())
		}
		if got := child.Text([]byte(treeSampleText)); got != treeSampleText[want.start:want.end] {
			t.Errorf("Child(%d).Text() = %q", i, got)
		}
		if got := child.Range(); got != (Range{want.start, want.end, Point{Column: want.start}, Point{Column: want.end}}) {
			t.Errorf("Child(%d).Range() = %+v", i, got)
		}
		parent, ok := child.Parent()
		if !ok || parent != root {
			t.Errorf("the parent of Child(%d) is %s", i, parent.Kind())
		}
	}
	if _, ok := root.Child(6); ok {
		t.Error("Child(6) found a node")
	}
	if _, ok := root.Parent(); ok {
		t.Error("the root has a parent")
	}

	named, _ := root.NamedChild(2)
	if named.Kind() != "expression" {
		t.Errorf("NamedChild(2) is %s", named.Kind())
	}
	if _, ok := root.NamedChild(5); ok {
		t.Error("NamedChild(5) found a node")
	}

	want := []string{"identifier", "identifier", "+", "expression", "identifier", "identifier"}
	if got := nodeKinds(root.Children()); !slices.Equal(got, want) {
		t.Errorf("Children() = %v, want %v", got, want)
	}
	want = []string{"identifier", "identifier", "expression", "identifier", "identifier"}
	if got := nodeKinds(root.NamedChildren()); !slices.Equal(got, want) {
		t.Errorf("NamedChildren() = %v, want %v", got, want)
	}
	for n := range root.Children() {
		if n.Kind() != "identifier" {
			t.Error("the loop did not stop")
		}
		break
	}
	for n := range root.NamedChildren() {
		if n.Kind() != "identifier" {
			t.Error("the loop did not stop")
		}
		break
	}
	a := nodeAt(t, root, 6, 7)
	if got := nodeKinds(a.Children()); len(got) != 0 {
		t.Errorf("the children of a leaf are %v", got)
	}

	if got := root.DescendantCount(); got != 10 {
		t.Errorf("DescendantCount() = %d, want 10", got)
	}
	if got := a.DescendantCount(); got != 1 {
		t.Errorf("DescendantCount() of a leaf = %d, want 1", got)
	}
}

func TestNodeAlias(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	expr, _ := root.Child(3)
	alias, _ := expr.Child(1)
	if alias.Kind() != "alias_name" || alias.KindID() != testSymAlias || !alias.IsNamed() {
		t.Errorf("the alias is %s %d, named %t", alias.Kind(), alias.KindID(), alias.IsNamed())
	}
	if alias.GrammarKind() != "+" || alias.GrammarID() != testSymPlus {
		t.Errorf("the grammar symbol of the alias is %s %d", alias.GrammarKind(), alias.GrammarID())
	}
	if got := alias.String(); got != "(alias_name)" {
		t.Errorf("String() of the alias = %s", got)
	}
	if expr.NamedChildCount() != 3 {
		t.Errorf("the expression has %d named children", expr.NamedChildCount())
	}
	plus, _ := root.Child(2)
	if plus.IsNamed() || plus.String() != `("+")` {
		t.Errorf("the anonymous child is named %t and is %s", plus.IsNamed(), plus.String())
	}
}

func TestNodeSiblings(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	x, _ := root.Child(0)
	y, _ := root.Child(1)
	plus, _ := root.Child(2)
	expr, _ := root.Child(3)
	c, _ := root.Child(4)
	z, _ := root.Child(5)

	for _, test := range []struct {
		name string
		got  func() (Node, bool)
		want Node
		ok   bool
	}{
		{"NextSibling of x", x.NextSibling, y, true},
		{"NextSibling of y", y.NextSibling, plus, true},
		{"NextNamedSibling of y", y.NextNamedSibling, expr, true},
		{"NextSibling of expression", expr.NextSibling, c, true},
		{"NextSibling of c", c.NextSibling, z, true},
		{"NextSibling of z", z.NextSibling, Node{}, false},
		{"PrevSibling of z", z.PrevSibling, c, true},
		{"PrevSibling of expression", expr.PrevSibling, plus, true},
		{"PrevNamedSibling of expression", expr.PrevNamedSibling, y, true},
		{"PrevSibling of y", y.PrevSibling, x, true},
		{"PrevSibling of x", x.PrevSibling, Node{}, false},
		{"PrevNamedSibling of x", x.PrevNamedSibling, Node{}, false},
	} {
		got, ok := test.got()
		if got != test.want || ok != test.ok {
			t.Errorf("%s is %s at %d, %t", test.name, got.Kind(), got.StartByte(), ok)
		}
	}

	a, _ := expr.Child(0)
	b, _ := expr.Child(2)
	if got, _ := a.NextNamedSibling(); got.Kind() != "alias_name" {
		t.Errorf("NextNamedSibling of a is %s", got.Kind())
	}
	if got, _ := b.PrevSibling(); got.Kind() != "alias_name" {
		t.Errorf("PrevSibling of b is %s", got.Kind())
	}
	if _, ok := b.NextSibling(); ok {
		t.Error("the last child of the expression has a next sibling")
	}
}

func TestNodeFields(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	expr, _ := root.Child(3)
	left, ok := expr.ChildByFieldName("left")
	if !ok || left.StartByte() != 6 {
		t.Errorf("the field left is at %d, %t", left.StartByte(), ok)
	}
	right, ok := expr.ChildByFieldID(2)
	if !ok || right.StartByte() != 10 {
		t.Errorf("the field right is at %d, %t", right.StartByte(), ok)
	}
	for _, name := range []string{"nope", ""} {
		if _, ok := expr.ChildByFieldName(name); ok {
			t.Errorf("the field %q found a node", name)
		}
	}
	if _, ok := expr.ChildByFieldID(0); ok {
		t.Error("the field 0 found a node")
	}
	if _, ok := root.ChildByFieldName("left"); ok {
		t.Error("a production without fields found a node")
	}

	for i, want := range []string{"left", "", "right", ""} {
		if got := expr.FieldNameForChild(i); got != want {
			t.Errorf("FieldNameForChild(%d) = %q, want %q", i, got, want)
		}
		if got := expr.FieldNameForNamedChild(i); got != want {
			t.Errorf("FieldNameForNamedChild(%d) = %q, want %q", i, got, want)
		}
	}
	if got := root.FieldNameForChild(4); got != "" {
		t.Errorf("the field of an extra is %q", got)
	}
	if got := root.FieldNameForChild(5); got != "" {
		t.Errorf("the field of a child without a field is %q", got)
	}

	if got := nodeKinds(expr.ChildrenByFieldName("right")); !slices.Equal(got, []string{"identifier"}) {
		t.Errorf("ChildrenByFieldName(right) = %v", got)
	}
	if got := nodeKinds(expr.ChildrenByFieldName("nope")); len(got) != 0 {
		t.Errorf("ChildrenByFieldName(nope) = %v", got)
	}
	if got := nodeKinds(root.ChildrenByFieldName("left")); len(got) != 0 {
		t.Errorf("ChildrenByFieldName(left) of the root = %v", got)
	}
	for range expr.ChildrenByFieldName("left") {
		break
	}
}

func TestNodeFieldOfAHiddenNode(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool()
	// a node of production 1 whose field left is a hidden node, with a
	// visible child
	hidden := newNode(&pool, testSymStatement, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
	}, 0, l)
	root := newNode(&pool, testSymExpression, subtreeArray{
		hidden,
		leaf(&pool, l, testSymPlus, 1, 1),
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 1, l)
	n := newTree(root, l, nil).RootNode()
	left, ok := n.ChildByFieldName("left")
	if !ok || left.Kind() != "identifier" || left.StartByte() != 0 {
		t.Errorf("the field left is %s at %d, %t", left.Kind(), left.StartByte(), ok)
	}
	if got := n.FieldNameForChild(0); got != "left" {
		t.Errorf("FieldNameForChild(0) = %q", got)
	}
	if got := n.FieldNameForNamedChild(0); got != "left" {
		t.Errorf("FieldNameForNamedChild(0) = %q", got)
	}
}

func TestNodeDescendants(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	for _, test := range []struct {
		start, end int
		kind       string
		named      string
		at         int
	}{
		{0, 1, "identifier", "identifier", 0},
		{0, 3, "expression", "expression", 0},
		{4, 5, "+", "expression", 4},
		{6, 7, "identifier", "identifier", 6},
		{8, 9, "alias_name", "alias_name", 8},
		{6, 11, "expression", "expression", 6},
		{7, 8, "expression", "expression", 6},
		{14, 15, "identifier", "identifier", 14},
		{5, 5, "expression", "expression", 0},
	} {
		got, ok := root.DescendantForByteRange(test.start, test.end)
		if !ok || got.Kind() != test.kind || got.StartByte() != test.at {
			t.Errorf("DescendantForByteRange(%d, %d) is %s at %d", test.start, test.end, got.Kind(), got.StartByte())
		}
		start, end := Point{Column: test.start}, Point{Column: test.end}
		if byPoint, _ := root.DescendantForPointRange(start, end); byPoint != got {
			t.Errorf("DescendantForPointRange(%v, %v) is %s", start, end, byPoint.Kind())
		}
		named, _ := root.NamedDescendantForByteRange(test.start, test.end)
		if named.Kind() != test.named {
			t.Errorf("NamedDescendantForByteRange(%d, %d) is %s", test.start, test.end, named.Kind())
		}
		if byPoint, _ := root.NamedDescendantForPointRange(start, end); byPoint != named {
			t.Errorf("NamedDescendantForPointRange(%v, %v) is %s", start, end, byPoint.Kind())
		}
	}
	if _, ok := root.DescendantForByteRange(3, 2); ok {
		t.Error("a range whose start is after its end found a node")
	}
	if _, ok := root.DescendantForPointRange(Point{Column: 3}, Point{Column: 2}); ok {
		t.Error("a range of points whose start is after its end found a node")
	}

	expr, _ := root.Child(3)
	a := nodeAt(t, root, 6, 7)
	if got, ok := root.ChildWithDescendant(a); !ok || got != expr {
		t.Errorf("ChildWithDescendant(a) of the root is %s", got.Kind())
	}
	if got, ok := expr.ChildWithDescendant(a); !ok || got != a {
		t.Errorf("ChildWithDescendant(a) of the expression is %s", got.Kind())
	}
	if _, ok := a.ChildWithDescendant(expr); ok {
		t.Error("a leaf holds a node")
	}
	if got, _ := a.Parent(); got != expr {
		t.Errorf("the parent of a is %s", got.Kind())
	}
}

func TestNodeFirstChildForByte(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	for _, test := range []struct {
		offset int
		all    int
		named  int
	}{
		{0, 0, 0},
		{1, 2, 2},
		{3, 4, 6},
		{11, 12, 12},
		{14, 14, 14},
		{15, -1, -1},
	} {
		got, ok := root.FirstChildForByte(test.offset)
		if ok != (test.all >= 0) || ok && got.StartByte() != test.all {
			t.Errorf("FirstChildForByte(%d) is %s at %d, %t", test.offset, got.Kind(), got.StartByte(), ok)
		}
		got, ok = root.FirstNamedChildForByte(test.offset)
		if ok != (test.named >= 0) || ok && got.StartByte() != test.named {
			t.Errorf("FirstNamedChildForByte(%d) is %s at %d, %t", test.offset, got.Kind(), got.StartByte(), ok)
		}
	}
}

func TestNodeErrorAndMissing(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool()
	root := newNode(&pool, testSymExpression, subtreeArray{
		newErrorNode(&pool, subtreeArray{leaf(&pool, l, testSymPlus, 0, 1)}, false, l),
		leaf(&pool, l, testSymIdentifier, 1, 1),
		newMissingLeaf(&pool, testSymIdentifier, 0, ln(0), 0, l),
	}, 0, l)
	n := newTree(root, l, nil).RootNode()
	if got, want := n.String(), "(expression (ERROR) (identifier) (MISSING identifier))"; got != want {
		t.Errorf("String() = %s, want %s", got, want)
	}
	if !n.HasError() || n.IsError() {
		t.Errorf("the root has an error %t and is an error %t", n.HasError(), n.IsError())
	}
	errorNode, _ := n.Child(0)
	if !errorNode.IsError() || !errorNode.HasError() || errorNode.Kind() != "ERROR" {
		t.Errorf("the error node is %s, an error %t", errorNode.Kind(), errorNode.IsError())
	}
	missing, _ := n.Child(2)
	if !missing.IsMissing() || missing.StartByte() != 3 || missing.EndByte() != 3 {
		t.Errorf("the missing node is missing %t at %d - %d", missing.IsMissing(), missing.StartByte(), missing.EndByte())
	}
	identifier, _ := n.Child(1)
	if identifier.IsMissing() || identifier.HasError() {
		t.Error("the identifier is missing or has an error")
	}
	// the missing node is empty, and it is after the identifier that ends
	// where it starts
	if got, ok := missing.PrevSibling(); !ok || got != identifier {
		t.Errorf("PrevSibling of the missing node is %s", got.Kind())
	}
	// C skips a child that ends where the node ends, so an empty node at
	// the end is no next sibling
	if got, ok := identifier.NextSibling(); ok {
		t.Errorf("NextSibling of the identifier is %s", got.Kind())
	}

	// an error child takes the parse state of the parent
	if n.ParseState() != tsTreeStateNone || n.NextParseState() != tsTreeStateNone {
		t.Errorf("the parse states of the root are %d and %d", n.ParseState(), n.NextParseState())
	}
}

func TestNodeParseState(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool()
	s := newLeaf(&pool, testSymIdentifier, ln(0), ln(1), 1, 0, false, false, false, l)
	n := newTree(s, l, nil).RootNode()
	// in state 0, identifier shifts to state 2
	if n.ParseState() != 0 || n.NextParseState() != 2 {
		t.Errorf("the parse states are %d and %d", n.ParseState(), n.NextParseState())
	}
}

func TestNodeNull(t *testing.T) {
	if !nullNode().isNull() || nullNode() != (Node{}) {
		t.Error("the null node is not the zero Node")
	}
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	if root.isNull() {
		t.Error("the root is the null node")
	}
}

// nodeInheritedLanguage returns the test language with a production 2 of
// two children. Its field left is inherited from child 0, and it is child 1
// when child 0 has no field left.
func nodeInheritedLanguage() *Language {
	tables := testTables()
	tables.ProductionIDCount = 3
	tables.AliasSequences = append(tables.AliasSequences, 0, 0, 0)
	tables.FieldMapSlices = append(tables.FieldMapSlices, abi.MapSlice{Index: 2, Length: 2})
	tables.FieldMapEntries = append(tables.FieldMapEntries,
		abi.FieldMapEntry{FieldID: 1, ChildIndex: 0, Inherited: true},
		abi.FieldMapEntry{FieldID: 1, ChildIndex: 1},
	)
	return NewLanguage(tables)
}

func TestNodeInheritedField(t *testing.T) {
	l := nodeInheritedLanguage()
	pool := newSubtreePool()

	// child 0 is a hidden node of production 1, and its field left is the
	// field left of the root
	hidden := newNode(&pool, testSymStatement, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 1, l)
	root := newNode(&pool, testSymExpression, subtreeArray{
		hidden,
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 2, l)
	n := newTree(root, l, nil).RootNode()
	left, ok := n.ChildByFieldName("left")
	if !ok || left.StartByte() != 0 {
		t.Errorf("the inherited field left is at %d, %t", left.StartByte(), ok)
	}

	// child 0 is a hidden node of production 0, which has no field left, so
	// the field left is child 1
	hidden = newNode(&pool, testSymStatement, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
	}, 0, l)
	root = newNode(&pool, testSymExpression, subtreeArray{
		hidden,
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 2, l)
	n = newTree(root, l, nil).RootNode()
	left, ok = n.ChildByFieldName("left")
	if !ok || left.StartByte() != 2 {
		t.Errorf("the field left after an inherited field is at %d, %t", left.StartByte(), ok)
	}
	if got := n.FieldNameForChild(1); got != "left" {
		t.Errorf("FieldNameForChild(1) = %q", got)
	}
}

func TestNodeEqual(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	root := tree.RootNode()
	child, ok := root.Child(1)
	if !ok {
		t.Fatal("the root has no child 1")
	}
	again, _ := root.Child(1)
	if again != child || !again.Equal(child) {
		t.Error("the same child twice is not == and Equal")
	}

	// a node that Edit moved is Equal to the old node, and not ==
	moved := child
	moved.Edit(InputEdit{
		StartByte: 0, OldEndByte: 0, NewEndByte: 2,
		StartPoint: Point{0, 0}, OldEndPoint: Point{0, 0}, NewEndPoint: Point{0, 2},
	})
	if moved.StartByte() == child.StartByte() {
		t.Fatalf("Edit did not move the node from %d", child.StartByte())
	}
	if moved == child {
		t.Error("the moved node is == to the old node")
	}
	if !moved.Equal(child) {
		t.Error("the moved node is not Equal to the old node")
	}

	other, _ := root.Child(2)
	if other.Equal(child) {
		t.Error("two children are Equal")
	}
	if treeSample(l).RootNode().Equal(root) {
		t.Error("the roots of two trees are Equal")
	}
	if !(Node{}).Equal(Node{}) {
		t.Error("two null nodes are not Equal, as ts_node_eq says they are")
	}
}
