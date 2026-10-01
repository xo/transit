package transit

import (
	"slices"
	"testing"
)

// cursorWalk is a node of treeSample in the order of a walk: its kind, its
// start byte, its depth and its field.
type cursorWalk struct {
	kind  string
	start int
	depth int
	field string
}

// cursorSampleWalk is the walk of treeSample. The index of an entry is the
// descendant index of the node.
var cursorSampleWalk = []cursorWalk{
	{"expression", 0, 0, ""},
	{"identifier", 0, 1, ""},
	{"identifier", 2, 1, ""},
	{"+", 4, 1, ""},
	{"expression", 6, 1, ""},
	{"identifier", 6, 2, "left"},
	{"alias_name", 8, 2, ""},
	{"identifier", 10, 2, "right"},
	{"identifier", 12, 1, ""},
	{"identifier", 14, 1, ""},
}

// cursorCheck fails the test when the cursor is not at the entry i of
// cursorSampleWalk.
func cursorCheck(t *testing.T, c *TreeCursor, i int) {
	t.Helper()
	want := cursorSampleWalk[i]
	n := c.Node()
	if n.Kind() != want.kind || n.StartByte() != want.start || c.Depth() != want.depth ||
		c.FieldName() != want.field || c.DescendantIndex() != i {
		t.Errorf("at %d: the cursor is at %s %d, depth %d, field %q, index %d",
			i, n.Kind(), n.StartByte(), c.Depth(), c.FieldName(), c.DescendantIndex())
	}
}

func TestCursorWalk(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	c := root.Walk()
	cursorCheck(t, c, 0)
	if c.GotoParent() || c.GotoNextSibling() || c.GotoPreviousSibling() {
		t.Error("the cursor moved out of the root")
	}

	// walk forward in the order of the tree
	var got []int
	for {
		got = append(got, c.DescendantIndex())
		cursorCheck(t, c, c.DescendantIndex())
		if c.GotoFirstChild() || c.GotoNextSibling() {
			continue
		}
		for c.GotoParent() {
			if c.GotoNextSibling() {
				break
			}
		}
		if c.Node() == root {
			break
		}
	}
	if want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}; !slices.Equal(got, want) {
		t.Errorf("the walk visited %v, want %v", got, want)
	}

	// the children of the root, backward. The entry of a previous sibling
	// has no descendant index in C, so the index is 0, or it counts from 0
	// in a hidden node, where the walk forward gives 8, 4, 3, 2 and 1.
	c.Reset(root)
	if !c.GotoLastChild() || c.Node().StartByte() != 14 {
		t.Fatalf("GotoLastChild moved to %s", c.Node().Kind())
	}
	var starts, indexes []int
	for c.GotoPreviousSibling() {
		starts = append(starts, c.Node().StartByte())
		indexes = append(indexes, c.DescendantIndex())
	}
	if want := []int{12, 6, 4, 2, 0}; !slices.Equal(starts, want) {
		t.Errorf("GotoPreviousSibling visited %v, want %v", starts, want)
	}
	if want := []int{0, 0, 0, 1, 0}; !slices.Equal(indexes, want) {
		t.Errorf("the descendant indexes after GotoPreviousSibling are %v, want %v", indexes, want)
	}
	if c.Node().Kind() != "identifier" || !c.GotoParent() || c.Node() != root {
		t.Error("GotoParent from the first child did not reach the root")
	}

	// GotoLastChild of a node with fields
	c.Reset(root)
	c.GotoFirstChildForByte(6)
	if !c.GotoLastChild() || c.FieldName() != "right" || c.FieldID() != 2 {
		t.Errorf("GotoLastChild of the expression moved to %s, field %q", c.Node().Kind(), c.FieldName())
	}
	if c.GotoFirstChild() || c.GotoLastChild() {
		t.Error("the cursor moved to a child of a leaf")
	}
}

// TestCursorMovesDoNotAllocate checks that a move to a sibling does not
// allocate. A query cursor moves a tree cursor for each node, and an
// allocation there grows with the size of the tree (D37).
func TestCursorMovesDoNotAllocate(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	c := root.Walk()
	allocs := testing.AllocsPerRun(100, func() {
		c.Reset(root)
		c.GotoFirstChild()
		for c.GotoNextSibling() {
		}
		for c.GotoPreviousSibling() {
		}
	})
	if allocs != 0 {
		t.Errorf("a walk over the siblings allocates %v times", allocs)
	}
}

func TestCursorGotoDescendant(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	c := root.Walk()
	for _, i := range []int{0, 5, 9, 1, 7, 4, 6, 8, 2, 3, 0} {
		c.GotoDescendant(i)
		cursorCheck(t, c, i)
	}
	// an index past the end moves to the root and stays there
	c.GotoDescendant(3)
	c.GotoDescendant(100)
	cursorCheck(t, c, 0)
}

func TestCursorGotoFirstChildForByte(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	for _, test := range []struct {
		offset int
		index  int
		at     int
	}{
		{0, 0, 0},
		{1, 1, 2},
		{3, 2, 4},
		{7, 3, 6},
		{14, 5, 14},
		{15, -1, 0},
	} {
		c := root.Walk()
		index, ok := c.GotoFirstChildForByte(test.offset)
		if ok != (test.index >= 0) || ok && index != test.index || c.Node().StartByte() != test.at {
			t.Errorf("GotoFirstChildForByte(%d) = %d, %t, at %d", test.offset, index, ok, c.Node().StartByte())
		}
		c = root.Walk()
		index, ok = c.GotoFirstChildForPoint(Point{Column: test.offset})
		if ok != (test.index >= 0) || ok && index != test.index || c.Node().StartByte() != test.at {
			t.Errorf("GotoFirstChildForPoint(%d) = %d, %t, at %d", test.offset, index, ok, c.Node().StartByte())
		}
		if !ok && c.Node() != root {
			t.Errorf("GotoFirstChildForPoint(%d) moved the cursor", test.offset)
		}
	}
}

func TestCursorResetAndCopy(t *testing.T) {
	l := testLanguage(15)
	root := treeSample(l).RootNode()
	expr, _ := root.Child(3)
	alias, _ := expr.Child(1)

	c := root.Walk()
	c.GotoDescendant(5)
	copied := c.Copy()
	c.GotoDescendant(9)
	cursorCheck(t, copied, 5)
	cursorCheck(t, c, 9)

	c.ResetTo(copied)
	cursorCheck(t, c, 5)
	copied.GotoNextSibling()
	cursorCheck(t, c, 5)

	// a node is the root of its cursor
	c.Reset(expr)
	if c.Node() != expr || c.Depth() != 0 || c.DescendantIndex() != 0 {
		t.Errorf("the cursor of the expression is at %s", c.Node().Kind())
	}
	c.GotoLastChild()
	if c.Depth() != 1 || c.DescendantIndex() != 3 {
		t.Errorf("the last child of the expression has the depth %d and the index %d", c.Depth(), c.DescendantIndex())
	}
	c.GotoParent()
	if c.GotoParent() || c.GotoNextSibling() {
		t.Error("the cursor moved out of its root")
	}

	// the cursor of an alias keeps the alias
	c = alias.Walk()
	if c.Node() != alias || c.Node().Kind() != "alias_name" {
		t.Errorf("the cursor of the alias is at %s", c.Node().Kind())
	}
}

func TestCursorInternals(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	root := tree.RootNode()
	c := root.Walk()
	supertypes := make([]Symbol, 4)

	if c.parentNode() != (Node{}) {
		t.Error("the root has a parent node")
	}
	if c.currentSubtree().ptr != tree.root.ptr {
		t.Error("the subtree of the root is not the root")
	}

	// at a: the field left, later named siblings and no later field left
	c.GotoDescendant(5)
	if got := c.parentNode(); got.Kind() != "expression" || got.StartByte() != 6 {
		t.Errorf("the parent node of a is %s at %d", got.Kind(), got.StartByte())
	}
	fieldID, later, laterNamed, laterField, count := c.currentStatus(supertypes)
	if fieldID != 1 || !later || !laterNamed || laterField || count != 0 {
		t.Errorf("the status of a is %d %t %t %t %d", fieldID, later, laterNamed, laterField, count)
	}

	// at b: no later siblings
	c.GotoDescendant(7)
	fieldID, later, laterNamed, laterField, count = c.currentStatus(supertypes)
	if fieldID != 2 || later || laterNamed || laterField || count != 0 {
		t.Errorf("the status of b is %d %t %t %t %d", fieldID, later, laterNamed, laterField, count)
	}

	// at y: the next siblings, "+" and a named one
	c.GotoDescendant(2)
	if got := c.parentNode(); got != root {
		t.Errorf("the parent node of y is %s", got.Kind())
	}
	if c.currentSubtree().symbol() != testSymIdentifier {
		t.Error("the subtree of y is not an identifier")
	}
	fieldID, later, laterNamed, laterField, count = c.currentStatus(supertypes)
	if fieldID != 0 || !later || !laterNamed || laterField || count != 0 {
		t.Errorf("the status of y is %d %t %t %t %d", fieldID, later, laterNamed, laterField, count)
	}

	// at z: the hidden supertype _statement above it
	c.GotoDescendant(9)
	fieldID, later, laterNamed, laterField, count = c.currentStatus(supertypes)
	if fieldID != 0 || later || laterNamed || laterField || count != 1 || supertypes[0] != testSymStatement {
		t.Errorf("the status of z is %d %t %t %t, supertypes %v", fieldID, later, laterNamed, laterField, supertypes[:count])
	}
	if fieldID, later, laterNamed, laterField, count = c.currentStatus(nil); count != 0 {
		t.Errorf("the status of z without room for supertypes is %d %t %t %t %d", fieldID, later, laterNamed, laterField, count)
	}
}

func TestCursorPreviousSiblingStopsAtIndex255(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	children := make(subtreeArray, 300)
	for i := range children {
		children[i] = leaf(&pool, l, testSymIdentifier, 0, 1)
	}
	root := newTree(newNode(&pool, testSymExpression, children, 0, l), l, nil).RootNode()
	c := root.Walk()

	// the descendant index of child i is i + 1. C cuts the child index to
	// 8 bits, so it stops at the children 255 and 256.
	for _, test := range []struct {
		child int
		ok    bool
	}{
		{254, true},
		{255, false},
		{256, false},
		{257, true},
		{299, true},
	} {
		c.GotoDescendant(test.child + 1)
		if got := c.GotoPreviousSibling(); got != test.ok {
			t.Errorf("GotoPreviousSibling from the child %d = %t, want %t", test.child, got, test.ok)
		}
		if test.ok && c.Node().StartByte() != test.child-1 {
			t.Errorf("GotoPreviousSibling from the child %d moved to %d", test.child, c.Node().StartByte())
		}
		c.Reset(root)
	}
}

func TestCursorPreviousSiblingOnTwoRows(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	// b has a padding of one row, so the cursor finds the position of a
	// from its parent
	root := newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		leaf(&pool, l, testSymIdentifier, 0, 2),
		newLeaf(&pool, testSymIdentifier, length{2, point{1, 1}}, ln(1), 1, 7, false, false, false, l),
	}, 0, l)
	n := newTree(root, l, nil).RootNode()
	c := n.Walk()
	c.GotoLastChild()
	if c.Node().StartPoint() != (Point{Row: 1, Column: 1}) {
		t.Fatalf("the last child starts at %v", c.Node().StartPoint())
	}
	if !c.GotoPreviousSibling() || c.Node().StartByte() != 1 || c.Node().StartPoint() != (Point{Column: 1}) {
		t.Errorf("GotoPreviousSibling moved to %d %v", c.Node().StartByte(), c.Node().StartPoint())
	}
	if !c.GotoPreviousSibling() || c.Node().StartByte() != 0 || c.Node().StartPoint() != (Point{}) {
		t.Errorf("GotoPreviousSibling moved to %d %v", c.Node().StartByte(), c.Node().StartPoint())
	}
}

func TestCursorEnumStrings(t *testing.T) {
	for step, want := range map[treeCursorStep]string{
		treeCursorStepNone:    "none",
		treeCursorStepHidden:  "hidden",
		treeCursorStepVisible: "visible",
		treeCursorStep(9):     unknownName,
	} {
		if got := step.String(); got != want {
			t.Errorf("String() = %s, want %s", got, want)
		}
	}
}
