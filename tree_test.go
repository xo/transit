package transit

import (
	"strings"
	"testing"
)

// treeSampleText is the text of treeSample.
const treeSampleText = "x y + a + b c z"

// treeSample returns a tree of the test language for treeSampleText, built
// by hand:
//
//	expression
//	  program_repeat1 (hidden): identifier "x", identifier "y"
//	  "+"
//	  expression, production 1: identifier "a", "+" as alias_name, identifier "b"
//	  identifier "c", an extra
//	  _statement (hidden): identifier "z"
func treeSample(l *Language) *Tree {
	return treeSampleWith(l, testSymIdentifier, testSymIdentifier)
}

// treeSampleWith returns treeSample with other symbols for "b" and "z".
func treeSampleWith(l *Language, b, z Symbol) *Tree {
	pool := newSubtreePool(0)
	rep := newNode(&pool, testSymRepeat, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 0, l)
	// The test sets the flag as setExtra does. A second call of setExtra
	// with true makes unparam report it, until the parser calls it with
	// false.
	extra := leaf(&pool, l, testSymIdentifier, 1, 1)
	extra.ptr.extra = true
	statement := newNode(&pool, testSymStatement, subtreeArray{
		leaf(&pool, l, z, 1, 1),
	}, 0, l)
	root := newNode(&pool, testSymExpression, subtreeArray{
		rep,
		leaf(&pool, l, testSymPlus, 1, 1),
		newNode(&pool, testSymExpression, subtreeArray{
			leaf(&pool, l, testSymIdentifier, 1, 1),
			leaf(&pool, l, testSymPlus, 1, 1),
			leaf(&pool, l, b, 1, 1),
		}, 1, l),
		extra,
		statement,
	}, 0, l)
	return newTree(root, l, nil)
}

// treeSampleString is the S-expression of treeSample.
const treeSampleString = "(expression (identifier) (identifier) " +
	"(expression left: (identifier) (alias_name) right: (identifier)) " +
	"(identifier) (identifier))"

func TestTreeRootNode(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	root := tree.RootNode()
	if root.Kind() != "expression" || root.StartByte() != 0 || root.EndByte() != len(treeSampleText) {
		t.Errorf("the root is %s at %d - %d", root.Kind(), root.StartByte(), root.EndByte())
	}
	if root.id != &tree.root || root.tree != tree {
		t.Error("the root node does not point to the root of the tree")
	}
	if got := root.String(); got != treeSampleString {
		t.Errorf("String() = %s, want %s", got, treeSampleString)
	}
	if tree.Language() != l || root.Language() != l {
		t.Error("the tree does not have the test language")
	}

	moved := tree.RootNodeWithOffset(10, Point{Row: 1, Column: 2})
	if moved.StartByte() != 10 || moved.StartPoint() != (Point{Row: 1, Column: 2}) {
		t.Errorf("the root with an offset starts at %d %v", moved.StartByte(), moved.StartPoint())
	}
	if moved.EndByte() != 10+len(treeSampleText) || moved.EndPoint() != (Point{Row: 1, Column: 2 + len(treeSampleText)}) {
		t.Errorf("the root with an offset ends at %d %v", moved.EndByte(), moved.EndPoint())
	}
	if moved == root {
		t.Error("the root with an offset is the root")
	}
}

func TestTreeCopy(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	tree.includedRanges = []textRange{{point{0, 0}, point{0, 20}, 0, 20}}
	if got := tree.root.ptr.refCount.Load(); got != 1 {
		t.Fatalf("the count of the root is %d, want 1", got)
	}
	copied := tree.Copy()
	if got := tree.root.ptr.refCount.Load(); got != 2 {
		t.Errorf("the count of the root after Copy is %d, want 2", got)
	}
	if copied.root.ptr != tree.root.ptr || copied.language != l {
		t.Error("the copy does not share the root")
	}
	if &copied.includedRanges[0] == &tree.includedRanges[0] {
		t.Error("the copy shares the included ranges")
	}

	// an edit of the copy copies the nodes that two trees hold
	copied.Edit(InputEdit{
		StartByte: 0, OldEndByte: 0, NewEndByte: 2,
		StartPoint: Point{}, OldEndPoint: Point{}, NewEndPoint: Point{Column: 2},
	})
	if copied.root.ptr == tree.root.ptr {
		t.Error("the edit of the copy changed the shared root")
	}
	if got := copied.RootNode().EndByte(); got != len(treeSampleText)+2 {
		t.Errorf("the copy ends at %d after the edit", got)
	}
	if got := tree.RootNode().EndByte(); got != len(treeSampleText) || tree.RootNode().HasChanges() {
		t.Errorf("the tree ends at %d after an edit of the copy", got)
	}
	if got := copied.IncludedRanges()[0]; got.EndByte != 22 {
		t.Errorf("the included range of the copy is %+v", got)
	}
	if got := tree.IncludedRanges()[0]; got.EndByte != 20 {
		t.Errorf("the included range of the tree is %+v", got)
	}
}

func TestTreeClose(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	copied := tree.Copy()
	root := tree.root.ptr

	// Close releases the root, and the copy keeps the nodes
	tree.Close()
	if got := root.refCount.Load(); got != 1 {
		t.Errorf("the count of the root after Close of one of two trees is %d, want 1", got)
	}
	if tree.root.ptr != nil {
		t.Error("the closed tree still holds the root")
	}
	if got := copied.RootNode().String(); got != treeSampleString {
		t.Errorf("the copy after Close of the tree is %s", got)
	}

	// Close of the last tree takes the counts to 0
	child := root.children[0].ptr
	copied.Close()
	if root.refCount.Load() != 0 || child.refCount.Load() != 0 {
		t.Errorf("the counts after Close of the last tree are %d and %d, want 0", root.refCount.Load(), child.refCount.Load())
	}

	// Close of a nil tree does nothing
	var none *Tree
	none.Close()
}

func TestTreeEdit(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	root := tree.RootNode()
	a, _ := root.DescendantForByteRange(6, 7)
	z, _ := root.Child(5)

	// insert a byte after the "a"
	edit := InputEdit{
		StartByte: 7, OldEndByte: 7, NewEndByte: 8,
		StartPoint: Point{Column: 7}, OldEndPoint: Point{Column: 7}, NewEndPoint: Point{Column: 8},
	}
	tree.Edit(edit)
	root = tree.RootNode()
	if !root.HasChanges() || root.EndByte() != len(treeSampleText)+1 {
		t.Errorf("the edited root ends at %d, with changes %t", root.EndByte(), root.HasChanges())
	}
	x, _ := root.Child(0)
	if x.HasChanges() {
		t.Error("the edit changed the node before it")
	}
	newA, _ := root.DescendantForByteRange(6, 7)
	if newA.Kind() != "identifier" || !newA.HasChanges() {
		t.Errorf("the node at the edit is %s, with changes %t", newA.Kind(), newA.HasChanges())
	}
	newZ, _ := root.Child(5)
	if newZ.StartByte() != 15 {
		t.Errorf("the last node starts at %d after the edit", newZ.StartByte())
	}

	// a node from before the edit moves with Node.Edit
	a.Edit(edit)
	z.Edit(edit)
	if a.StartByte() != 6 || z.StartByte() != 15 || z.StartPoint() != (Point{Column: 15}) {
		t.Errorf("the kept nodes start at %d and %d %v", a.StartByte(), z.StartByte(), z.StartPoint())
	}
	if z != newZ {
		t.Error("the kept node is not the node of the edited tree")
	}
}

func TestTreeIncludedRanges(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	ranges := []textRange{
		{point{0, 0}, point{0, 4}, 0, 4},
		{point{1, 0}, point{1, 4}, 10, 14},
	}
	tree := newTree(expression(&pool, l), l, ranges)
	ranges[0].endByte = 99
	got := tree.IncludedRanges()
	want := []Range{
		{StartByte: 0, EndByte: 4, StartPoint: Point{}, EndPoint: Point{Column: 4}},
		{StartByte: 10, EndByte: 14, StartPoint: Point{Row: 1}, EndPoint: Point{Row: 1, Column: 4}},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("IncludedRanges() = %+v, want %+v", got, want)
	}
	got[0].EndByte = 99
	if tree.includedRanges[0].endByte != 4 {
		t.Error("IncludedRanges does not return a copy")
	}

	// an edit before the second range moves it
	tree.Edit(InputEdit{
		StartByte: 6, OldEndByte: 8, NewEndByte: 6,
		StartPoint: Point{Column: 6}, OldEndPoint: Point{Column: 8}, NewEndPoint: Point{Column: 6},
	})
	if got := tree.IncludedRanges(); got[0] != want[0] || got[1].StartByte != 8 || got[1].EndByte != 12 {
		t.Errorf("IncludedRanges() after an edit = %+v", got)
	}
}

func TestTreePrintDotGraph(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	var b strings.Builder
	tree.PrintDotGraph(&b)
	got := b.String()
	for _, want := range []string{
		"digraph tree {\nedge [arrowhead=none]\n",
		`[label="expression", tooltip="range: 0 - 15`,
		`[label="program_repeat1", tooltip="range: 0 - 3`,
		`[label="alias_name", shape=plaintext`,
		"}\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the graph has no %q:\n%s", want, got)
		}
	}
}

func TestTreeWalk(t *testing.T) {
	l := testLanguage(15)
	tree := treeSample(l)
	c := tree.Walk()
	if c.Node() != tree.RootNode() || !c.GotoFirstChild() {
		t.Error("the cursor of the tree does not start at the root")
	}
}
