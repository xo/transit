package transit

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// ln returns a length of bytes on one row.
func ln(bytes uint32) length {
	return length{bytes, point{0, bytes}}
}

// leaf returns a leaf of the test language with a padding and a size of
// bytes on one row.
func leaf(pool *subtreePool, l *Language, symbol Symbol, padding, size uint32) subtree {
	return newLeaf(pool, symbol, ln(padding), ln(size), 1, 7, false, false, false, l)
}

// expression returns the node expression of production 1 of the test
// language: identifier "+" identifier, with the fields left and right and
// the "+" aliased as alias_name.
func expression(pool *subtreePool, l *Language) subtree {
	return newNode(pool, testSymExpression, subtreeArray{
		leaf(pool, l, testSymIdentifier, 0, 1),
		leaf(pool, l, testSymPlus, 1, 1),
		leaf(pool, l, testSymIdentifier, 1, 1),
	}, 1, l)
}

func TestNewLeafInline(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)

	// a small leaf is inline, and its size has the column of its bytes
	s := newLeaf(&pool, testSymIdentifier, ln(2), length{4, point{0, 9}}, 3, 7, false, true, true, l)
	if !s.ptr.isInline {
		t.Fatal("a small leaf is not inline")
	}
	if s.size() != ln(4) || s.padding() != ln(2) {
		t.Errorf("the inline leaf has the size %v and the padding %v", s.size(), s.padding())
	}
	if !s.dependsOnColumn() {
		t.Error("an inline leaf does not keep dependsOnColumn")
	}
	if !s.isKeyword() || !s.visible() || !s.named() || s.extra() || s.parseState() != 7 || s.lookaheadBytes() != 3 {
		t.Errorf("the inline leaf has the fields %+v", s.ptr.heapFields)
	}

	// a leaf that does not fit is not inline, and it keeps its fields. The
	// language wide has the metadata of 300 symbols.
	tables := testTables()
	tables.SymbolMetadata = append(tables.SymbolMetadata, make([]abi.SymbolMetadata, 300)...)
	wide := NewLanguage(tables)
	for _, test := range []struct {
		name      string
		symbol    Symbol
		padding   length
		size      length
		lookahead uint32
		external  bool
	}{
		{"a symbol past 255", 256, ln(1), ln(1), 1, false},
		{"external tokens", testSymIdentifier, ln(1), ln(1), 1, true},
		{"a long padding", testSymIdentifier, ln(255), ln(1), 1, false},
		{"a padding of 16 rows", testSymIdentifier, length{20, point{16, 0}}, ln(1), 1, false},
		{"a size on two rows", testSymIdentifier, ln(1), length{3, point{1, 1}}, 1, false},
		{"a long lookahead", testSymIdentifier, ln(1), ln(1), 16, false},
	} {
		s := newLeaf(&pool, test.symbol, test.padding, test.size, test.lookahead, 7, test.external, true, false, wide)
		if s.ptr.isInline {
			t.Errorf("%s: the leaf is inline", test.name)
		}
		if s.size() != test.size || !s.dependsOnColumn() || s.hasExternalTokens() != test.external {
			t.Errorf("%s: the leaf has the size %v and dependsOnColumn %t", test.name, s.size(), s.dependsOnColumn())
		}
	}

	// the end of the input is an extra
	if !leaf(&pool, l, testSymEnd, 0, 0).extra() {
		t.Error("the leaf of the end is not an extra")
	}
	if !leaf(&pool, l, testSymEnd, 0, 0).isEOF() {
		t.Error("isEOF is false for the leaf of the end")
	}
}

func TestNewErrorAndMissingLeaf(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	e := newError(&pool, 'x', ln(1), ln(2), 5, 3, l)
	if !e.isError() || !e.fragileLeft() || !e.fragileRight() || !e.isFragile() || e.ptr.lookaheadChar != 'x' {
		t.Errorf("the error leaf has the fields %+v", e.ptr.heapFields)
	}
	if e.ptr.isInline {
		t.Error("an error leaf is inline")
	}
	m := newMissingLeaf(&pool, testSymIdentifier, 2, ln(1), 1, l)
	if !m.missing() || m.size() != lengthZero() {
		t.Errorf("the missing leaf has the fields %+v", m.ptr.heapFields)
	}
	if got := m.errorCost(); got != errorCostPerMissingTree+errorCostPerRecovery {
		t.Errorf("the error cost of a missing leaf is %d", got)
	}
}

func TestNewNodeSummarizesItsChildren(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	n := expression(&pool, l)
	if got := n.childCount(); got != 3 {
		t.Fatalf("childCount() = %d, want 3", got)
	}
	if n.padding() != ln(0) || n.size() != ln(5) || n.totalBytes() != 5 {
		t.Errorf("the node has the padding %v and the size %v", n.padding(), n.size())
	}
	// the alias of "+" is named, so all three children are visible and named
	if n.visibleChildCount() != 3 || n.ptr.namedChildCount != 3 || n.visibleDescendantCount() != 3 {
		t.Errorf("the node has %d visible children, %d named children and %d visible descendants",
			n.visibleChildCount(), n.ptr.namedChildCount, n.visibleDescendantCount())
	}
	if n.productionID() != 1 || n.leafSymbol() != testSymIdentifier || n.leafParseState() != 7 {
		t.Errorf("the node has the production %d and the first leaf %+v", n.productionID(), n.ptr.firstLeaf)
	}
	// the lookahead of the last child ends one byte past the node
	if n.lookaheadBytes() != 1 {
		t.Errorf("lookaheadBytes() = %d, want 1", n.lookaheadBytes())
	}
	if n.errorCost() != 0 || n.isFragile() || n.dynamicPrecedence() != 0 || n.repeatDepth() != 0 {
		t.Errorf("the node has the fields %+v", n.ptr.heapFields)
	}
	// a leaf has no production and no visible children
	child := n.ptr.children[0]
	if child.productionID() != 0 || child.visibleChildCount() != 0 || child.visibleDescendantCount() != 0 || child.dynamicPrecedence() != 0 {
		t.Error("a leaf has the fields of a node")
	}
}

func TestErrorNodeCosts(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	e := newErrorNode(&pool, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 3),
		leaf(&pool, l, testSymPlus, 0, 1),
	}, true, l)
	if !e.isError() || !e.extra() || !e.fragileLeft() || !e.fragileRight() {
		t.Errorf("the error node has the fields %+v", e.ptr.heapFields)
	}
	// two skipped trees, a recovery, and four skipped characters
	want := uint32(2*errorCostPerSkippedTree + errorCostPerRecovery + 4*errorCostPerSkippedChar)
	if got := e.errorCost(); got != want {
		t.Errorf("errorCost() = %d, want %d", got, want)
	}

	// an _ERROR child is refunded its extent, which the parent charges again
	repeat := newNode(&pool, builtinSymErrorRepeat, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 3),
	}, 0, l)
	parent := newErrorNode(&pool, subtreeArray{repeat}, false, l)
	wantRepeat := uint32(errorCostPerSkippedTree) + errorExtentCost(ln(3))
	if got := repeat.errorCost(); got != wantRepeat {
		t.Errorf("the error cost of _ERROR = %d, want %d", got, wantRepeat)
	}
	if got := parent.errorCost(); got != wantRepeat {
		t.Errorf("the error cost of ERROR over _ERROR = %d, want %d", got, wantRepeat)
	}

	// a node with an error child is fragile and has no parse state
	n := newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		newError(&pool, 'x', ln(0), ln(1), 1, 0, l),
	}, 0, l)
	if !n.isFragile() || n.parseState() != tsTreeStateNone {
		t.Errorf("a node over an error has fragile %t and the state %d", n.isFragile(), n.parseState())
	}
}

func TestRetainAndRelease(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	child := newLeaf(&pool, testSymIdentifier, ln(0), ln(300), 1, 0, false, false, false, l)
	n := newNode(&pool, testSymExpression, subtreeArray{child}, 0, l)
	if child.ptr.refCount.Load() != 1 || n.ptr.refCount.Load() != 1 {
		t.Fatal("a new subtree does not have a count of 1")
	}
	child.retain()
	n.retain()
	n.release(&pool)
	if n.ptr.refCount.Load() != 1 || child.ptr.refCount.Load() != 2 {
		t.Errorf("after a retain and a release, the counts are %d and %d", n.ptr.refCount.Load(), child.ptr.refCount.Load())
	}
	// a node that reaches 0 releases its children
	n.release(&pool)
	if n.ptr.refCount.Load() != 0 || child.ptr.refCount.Load() != 1 {
		t.Errorf("after the last release, the counts are %d and %d", n.ptr.refCount.Load(), child.ptr.refCount.Load())
	}
	// an inline leaf has no count
	inline := leaf(&pool, l, testSymIdentifier, 0, 1)
	inline.retain()
	inline.release(&pool)
	if inline.ptr.refCount.Load() != 1 {
		t.Error("retain or release changed the count of an inline leaf")
	}
}

func TestReleaseAssertsACount(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	n := expression(&pool, l)
	n.release(&pool)
	defer func() {
		if recover() == nil {
			t.Error("a release of a subtree with a count of 0 did not panic")
		}
	}()
	n.release(&pool)
}

func TestMakeMut(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	n := expression(&pool, l)
	if got := n.makeMut(&pool); got != n {
		t.Error("makeMut of a subtree with one owner made a copy")
	}

	n.retain()
	m := n.makeMut(&pool)
	if m == n {
		t.Fatal("makeMut of a shared subtree did not make a copy")
	}
	if n.ptr.refCount.Load() != 1 || m.ptr.refCount.Load() != 1 {
		t.Errorf("after makeMut the counts are %d and %d, want 1 and 1", n.ptr.refCount.Load(), m.ptr.refCount.Load())
	}
	// the copy shares the children, and it holds them
	for i, child := range m.ptr.children {
		if child != n.ptr.children[i] {
			t.Errorf("the child %d of the copy is another subtree", i)
		}
	}
	if &m.ptr.children[0] == &n.ptr.children[0] {
		t.Error("the copy shares the slice of children")
	}
	m.setExtra(true)
	if n.extra() {
		t.Error("a change of the copy changed the original")
	}

	// an inline leaf is a value in C, so makeMut copies it
	inline := n.ptr.children[0]
	copied := inline.makeMut(&pool)
	if copied == inline || !reflect.DeepEqual(copied.ptr.heapFields, inline.ptr.heapFields) {
		t.Error("makeMut of an inline leaf did not make an equal copy")
	}

	// the copy of a leaf with external tokens has its own state
	ext := newLeaf(&pool, testSymIdentifier, ln(0), ln(1), 1, 0, true, false, false, l)
	ext.ptr.externalScannerState.init(&pool, []byte{1, 2, 3})
	ext.retain()
	extCopy := ext.makeMut(&pool)
	extCopy.ptr.externalScannerState.buf[0] = 9
	if !ext.ptr.externalScannerState.eq([]byte{1, 2, 3}) {
		t.Error("a change of the state of the copy changed the original")
	}
}

// repetition returns a left-deep chain of program_repeat1 nodes over n
// leaves: (((a b) c) d).
func repetition(pool *subtreePool, l *Language, n int) subtree {
	tree := newNode(pool, testSymRepeat, subtreeArray{
		leaf(pool, l, testSymIdentifier, 0, 1),
		leaf(pool, l, testSymIdentifier, 0, 1),
	}, 0, l)
	for range n - 2 {
		tree = newNode(pool, testSymRepeat, subtreeArray{
			tree,
			leaf(pool, l, testSymIdentifier, 0, 1),
		}, 0, l)
	}
	return tree
}

// shape returns the shape of a tree as a string: a leaf is x, and a node is
// its children in parentheses.
func shape(s subtree) string {
	if s.childCount() == 0 {
		return "x"
	}
	parts := make([]string, 0, s.childCount())
	for _, child := range s.ptr.children {
		parts = append(parts, shape(child))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func TestCompress(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	tree := repetition(&pool, l, 5)
	if got := tree.repeatDepth(); got != 3 {
		t.Errorf("the depth of the chain is %d, want 3", got)
	}
	var stack subtreeArray
	tree.compress(1, l, &stack)
	if len(stack) != 0 {
		t.Errorf("compress left %d subtrees on the stack", len(stack))
	}
	if got, want := shape(tree), "(((x x) (x x)) x)"; got != want {
		t.Errorf("the shape after compress is %s, want %s", got, want)
	}
	if got := tree.repeatDepth(); got != 2 {
		t.Errorf("the depth after compress is %d, want 2", got)
	}
	if tree.size() != ln(5) || tree.ptr.visibleChildCount != 5 {
		t.Errorf("compress changed the size to %v or the visible children to %d", tree.size(), tree.ptr.visibleChildCount)
	}

	// compress stops at a node that two trees hold
	shared := repetition(&pool, l, 5)
	shared.ptr.children[0].retain()
	before := shape(shared)
	shared.compress(1, l, &stack)
	if got := shape(shared); got != before {
		t.Errorf("compress changed a shared chain to %s", got)
	}
}

func TestCompare(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	a := expression(&pool, l)
	b := expression(&pool, l)
	if got := compare(a, b, &pool); got != 0 {
		t.Errorf("compare of equal trees = %d", got)
	}
	c := newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
	}, 1, l)
	if got := compare(a, c, &pool); got != 1 {
		t.Errorf("compare(a, c) = %d, want 1, because the last child of c has a smaller symbol", got)
	}
	if got := compare(c, a, &pool); got != -1 {
		t.Errorf("compare(c, a) = %d, want -1", got)
	}
	if len(pool.treeStack) != 0 {
		t.Error("compare left subtrees on the stack")
	}
	d := newNode(&pool, testSymExpression, subtreeArray{leaf(&pool, l, testSymIdentifier, 0, 1)}, 0, l)
	if compare(d, a, &pool) != -1 || compare(a, d, &pool) != 1 {
		t.Error("compare does not order by the number of children")
	}
}

func TestEdit(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	// "a + b"
	tree := expression(&pool, l)
	tree.retain()
	old := tree

	// insert two bytes after the "+"
	edited := tree.edit(InputEdit{
		StartByte: 3, OldEndByte: 3, NewEndByte: 5,
		StartPoint: Point{0, 3}, OldEndPoint: Point{0, 3}, NewEndPoint: Point{0, 5},
	}, &pool)
	if edited == old {
		t.Fatal("an edit of a shared tree changed it in place")
	}
	if old.size() != ln(5) || old.hasChanges() {
		t.Errorf("the edit changed the old tree to the size %v", old.size())
	}
	if edited.size() != ln(7) || !edited.hasChanges() {
		t.Errorf("the edited tree has the size %v and has changes %t", edited.size(), edited.hasChanges())
	}
	children := edited.ptr.children
	// The insertion goes to the first child that touches it, the "+". The
	// lookahead of the a ends at byte 2, before the edit, and the b starts at
	// the old end of the edit, so the edit stops before it.
	if children[0].hasChanges() || !children[1].hasChanges() || children[2].hasChanges() {
		t.Errorf("the children have changes %t, %t and %t", children[0].hasChanges(), children[1].hasChanges(), children[2].hasChanges())
	}
	if children[1].size() != ln(3) || children[2].padding() != ln(1) {
		t.Errorf("the + has the size %v and the b the padding %v", children[1].size(), children[2].padding())
	}

	// a deletion that starts in the padding shrinks the child
	edited = edited.edit(InputEdit{
		StartByte: 4, OldEndByte: 7, NewEndByte: 4,
		StartPoint: Point{0, 4}, OldEndPoint: Point{0, 7}, NewEndPoint: Point{0, 4},
	}, &pool)
	if edited.size() != ln(4) {
		t.Errorf("after the deletion the tree has the size %v, want 4 bytes", edited.size())
	}

	// an edit past the end of a subtree and its lookahead does nothing
	before := edited.ptr.heapFields
	same := edited.edit(InputEdit{StartByte: 50, OldEndByte: 51, NewEndByte: 52}, &pool)
	if same != edited || !reflect.DeepEqual(same.ptr.heapFields, before) {
		t.Error("an edit past the end changed the tree")
	}
}

func TestEditOfAnInlineLeaf(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	s := leaf(&pool, l, testSymIdentifier, 1, 2)
	// an insertion that keeps the leaf small keeps it inline
	small := s.edit(InputEdit{StartByte: 2, OldEndByte: 2, NewEndByte: 3, StartPoint: Point{0, 2}, OldEndPoint: Point{0, 2}, NewEndPoint: Point{0, 3}}, &pool)
	if !small.ptr.isInline || small.size() != ln(3) || !small.hasChanges() {
		t.Errorf("the small edit gave %+v", small.ptr.heapFields)
	}
	if s.size() != ln(2) || s.hasChanges() {
		t.Error("the edit changed the original inline leaf")
	}
	// a newline in the leaf makes it too large to be inline
	large := s.edit(InputEdit{StartByte: 2, OldEndByte: 2, NewEndByte: 3, StartPoint: Point{0, 2}, OldEndPoint: Point{0, 2}, NewEndPoint: Point{1, 0}}, &pool)
	if large.ptr.isInline || large.size() != (length{3, point{1, 1}}) || !large.hasChanges() || large.ptr.refCount.Load() != 1 {
		t.Errorf("the large edit gave %+v with the count %d", large.ptr.heapFields, large.ptr.refCount.Load())
	}
	// a leaf that leaves the inline form keeps dependsOnColumn
	column := newLeaf(&pool, testSymIdentifier, ln(1), ln(2), 1, 7, false, true, false, l)
	moved := column.edit(InputEdit{StartByte: 2, OldEndByte: 2, NewEndByte: 3, StartPoint: Point{0, 2}, OldEndPoint: Point{0, 2}, NewEndPoint: Point{1, 0}}, &pool)
	if moved.ptr.isInline || !moved.dependsOnColumn() {
		t.Errorf("the large edit of a leaf that depends on the column gave %+v", moved.ptr.heapFields)
	}
}

func TestLastExternalToken(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	if got := expression(&pool, l).lastExternalToken(); got.ptr != nil {
		t.Error("a tree with no external tokens has a last external token")
	}
	ext1 := newLeaf(&pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	ext2 := newLeaf(&pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	inner := newNode(&pool, testSymExpression, subtreeArray{ext1, ext2, leaf(&pool, l, testSymIdentifier, 0, 1)}, 0, l)
	tree := newNode(&pool, testSymExpression, subtreeArray{inner, leaf(&pool, l, testSymIdentifier, 0, 1)}, 0, l)
	if !tree.hasExternalTokens() {
		t.Fatal("the node over external tokens has none")
	}
	if got := tree.lastExternalToken(); got != ext2 {
		t.Error("lastExternalToken did not find the last external token")
	}
}

func TestExternalScannerState(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	a := newLeaf(&pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	b := newLeaf(&pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	data := []byte("state")
	a.ptr.externalScannerState.init(&pool, data)
	data[0] = 'X'
	if got := string(a.getExternalScannerState().data()); got != "state" {
		t.Errorf("the state is %q, want a copy of the data", got)
	}
	if a.externalScannerStateEq(b) {
		t.Error("a state and an empty state are equal")
	}
	b.ptr.externalScannerState.init(&pool, []byte("state"))
	if !a.externalScannerStateEq(b) {
		t.Error("two equal states are not equal")
	}
	// a node, an inline leaf and NULL have the empty state
	for _, s := range []subtree{expression(&pool, l), leaf(&pool, l, testSymIdentifier, 0, 1), {}} {
		if s.getExternalScannerState() != &emptyExternalScannerState {
			t.Error("a subtree with no external tokens has a state")
		}
	}
}

func TestSubtreeArray(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	extra := newLeaf(&pool, testSymPlus, ln(0), ln(300), 1, 0, false, false, false, l)
	extra.setExtra(true)
	a := newLeaf(&pool, testSymIdentifier, ln(0), ln(300), 1, 0, false, false, false, l)
	b := newLeaf(&pool, testSymIdentifier, ln(0), ln(301), 1, 0, false, false, false, l)
	array := subtreeArray{a, extra, b, extra}

	copied := array.copy(&pool)
	if !slices.Equal(copied, array) || a.ptr.refCount.Load() != 2 || extra.ptr.refCount.Load() != 3 {
		t.Errorf("copy gave %v with the counts %d and %d", copied, a.ptr.refCount.Load(), extra.ptr.refCount.Load())
	}
	if subtreeArray(nil).copy(&pool) != nil {
		t.Error("the copy of an empty array is not empty")
	}
	copied.clear(&pool)
	if len(copied) != 0 || a.ptr.refCount.Load() != 1 {
		t.Errorf("clear left %d subtrees and the count %d", len(copied), a.ptr.refCount.Load())
	}
	copied = array.copy(&pool)
	copied.delete(&pool)
	if copied != nil || a.ptr.refCount.Load() != 1 {
		t.Error("delete did not release the subtrees")
	}

	var trailing subtreeArray
	array = subtreeArray{a, extra, b, extra, extra}
	array.removeTrailingExtras(&trailing)
	if !slices.Equal(array, subtreeArray{a, extra, b}) || !slices.Equal(trailing, subtreeArray{extra, extra}) {
		t.Errorf("removeTrailingExtras left %v and %v", array, trailing)
	}
	array.reverse()
	if !slices.Equal(array, subtreeArray{b, extra, a}) {
		t.Errorf("reverse gave %v", array)
	}
}

func TestSubtreePoolChunks(t *testing.T) {
	pool := newSubtreePool(0)
	first := pool.allocate()
	if len(pool.nodes) != minSubtreeChunk-1 {
		t.Errorf("the first chunk holds %d nodes, want %d", len(pool.nodes)+1, minSubtreeChunk)
	}
	for range 10000 {
		pool.allocate()
	}
	if pool.nodeChunk != maxSubtreeChunk {
		t.Errorf("the chunk grew to %d nodes, want %d", pool.nodeChunk, maxSubtreeChunk)
	}
	if first.refCount.Load() != 0 {
		t.Error("a new node has a count")
	}
	children := pool.allocateChildren(3)
	if len(children) != 3 || cap(children) != 3 {
		t.Errorf("allocateChildren(3) has the length %d and the capacity %d", len(children), cap(children))
	}
	if big := pool.allocateChildren(1000); len(big) != 1000 {
		t.Errorf("allocateChildren(1000) has the length %d", len(big))
	}
}

func TestSubtreePoolFreeList(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(tsMaxTreePoolSize)
	// big returns a leaf that is not inline, because it has external tokens.
	big := func() subtree {
		return newLeaf(&pool, testSymIdentifier, ln(1), ln(1), 1, 7, true, false, false, l)
	}

	// a leaf whose count reaches 0 goes to the free list, cleared, and the
	// next leaf takes it
	s := big()
	s.ptr.externalScannerState.init(&pool, []byte("state"))
	ptr := s.ptr
	s.release(&pool)
	if len(pool.freeTrees) != 1 || pool.freeTrees[0].ptr != ptr {
		t.Fatalf("the free list holds %d nodes after a release", len(pool.freeTrees))
	}
	if !reflect.DeepEqual(ptr.heapFields, heapFields{}) {
		t.Errorf("a node of the free list holds %+v", ptr.heapFields)
	}
	if next := big(); next.ptr != ptr || len(pool.freeTrees) != 0 || next.ptr.refCount.Load() != 1 {
		t.Error("the next leaf does not take the node of the free list")
	}

	// the children of a node go to the free list, and the node does not, as
	// C frees it with its children
	node := newNode(&pool, testSymRepeat, subtreeArray{big(), big()}, 0, l)
	node.release(&pool)
	if len(pool.freeTrees) != 2 || slices.ContainsFunc(pool.freeTrees, func(f subtree) bool { return f.ptr == node.ptr }) {
		t.Errorf("the free list holds %d nodes after the release of a node of two leaves", len(pool.freeTrees))
	}

	// a node that a tree still holds does not go to the free list
	shared := big()
	shared.retain()
	n := len(pool.freeTrees)
	shared.release(&pool)
	if len(pool.freeTrees) != n {
		t.Errorf("a node with a count went to the free list")
	}

	// the free list holds at most TS_MAX_TREE_POOL_SIZE nodes
	leaves := make(subtreeArray, 2*tsMaxTreePoolSize)
	for i := range leaves {
		leaves[i] = big()
	}
	for _, leaf := range leaves {
		leaf.release(&pool)
	}
	if len(pool.freeTrees) != tsMaxTreePoolSize {
		t.Errorf("the free list holds %d nodes, want %d", len(pool.freeTrees), tsMaxTreePoolSize)
	}

	// a pool whose capacity is 0 keeps no free node
	empty := newSubtreePool(0)
	newLeaf(&empty, testSymIdentifier, ln(1), ln(1), 1, 7, true, false, false, l).release(&empty)
	if len(empty.freeTrees) != 0 {
		t.Error("a pool whose capacity is 0 kept a free node")
	}

	// a clone and a new node take a new node of the chunk, and not a node of
	// the free list, as C does
	n = len(pool.freeTrees)
	big().clone(&pool)
	newNode(&pool, testSymRepeat, subtreeArray{}, 0, l)
	if len(pool.freeTrees) != n-1 {
		t.Errorf("the free list holds %d nodes, want %d", len(pool.freeTrees), n-1)
	}
}

func TestSubtreeString(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	n := expression(&pool, l)
	if got, want := n.string(0, false, l, false), "(expression left: (identifier) (alias_name) right: (identifier))"; got != want {
		t.Errorf("string() = %s, want %s", got, want)
	}
	if got, want := n.string(0, false, l, true), `(expression left: (identifier) (alias_name) right: (identifier))`; got != want {
		t.Errorf("string() with all nodes = %s, want %s", got, want)
	}
	// an alias of the root
	if got, want := n.string(testSymAlias, true, l, false), "(alias_name left: (identifier) (alias_name) right: (identifier))"; got != want {
		t.Errorf("string() of an alias = %s, want %s", got, want)
	}

	// a missing leaf, an unexpected character and a node with no production
	e := newNode(&pool, testSymExpression, subtreeArray{
		newMissingLeaf(&pool, testSymIdentifier, 0, ln(0), 1, l),
		newMissingLeaf(&pool, testSymPlus, 0, ln(0), 1, l),
		newError(&pool, 'q', ln(0), ln(1), 1, 0, l),
		newError(&pool, '\n', ln(0), ln(1), 1, 0, l),
		newError(&pool, 0x1f600, ln(0), ln(1), 1, 0, l),
		newError(&pool, -1, ln(0), ln(1), 1, 0, l),
		leaf(&pool, l, testSymPlus, 0, 1),
	}, 0, l)
	want := `(expression (MISSING identifier) (MISSING "+") (UNEXPECTED 'q') (UNEXPECTED '\n') (UNEXPECTED 128512) (UNEXPECTED INVALID))`
	if got := e.string(0, false, l, false); got != want {
		t.Errorf("string() = %s, want %s", got, want)
	}
	// a leaf root that is not visible
	if got, want := leaf(&pool, l, testSymPlus, 0, 1).string(0, false, l, false), `("+")`; got != want {
		t.Errorf("string() of an anonymous leaf = %s, want %s", got, want)
	}
	if got, want := (subtree{}).string(0, false, l, false), "(NULL)"; got != want {
		t.Errorf("string() of NULL = %s, want %s", got, want)
	}
}

func TestWriteCharToString(t *testing.T) {
	for chr, want := range map[int32]string{
		-1:   "INVALID",
		0:    `'\0'`,
		'\n': `'\n'`,
		'\t': `'\t'`,
		'\r': `'\r'`,
		'a':  "'a'",
		' ':  "' '",
		0x7f: "127",
		0x1f: "31",
		0xe9: "233",
	} {
		var b strings.Builder
		writeCharToString(&b, chr)
		if got := b.String(); got != want {
			t.Errorf("writeCharToString(%d) = %s, want %s", chr, got, want)
		}
	}
}

func TestPrintDotGraph(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	n := newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		newError(&pool, 'q', ln(0), ln(1), 1, 0, l),
	}, 1, l)
	n.setHasChanges()
	n.ptr.children[0].setExtra(true)
	var b strings.Builder
	n.printDotGraph(l, &b)
	got := b.String()
	for _, want := range []string{
		"digraph tree {\nedge [arrowhead=none]\n",
		`[label="expression", color=green, penwidth=2, tooltip="range: 0 - 2`,
		`[label="identifier", shape=plaintext, fontcolor=gray, tooltip="range: 0 - 1`,
		// the error child is the first structural child, so it takes the
		// alias at index 0 of the production, which is none
		`[label="ERROR", shape=plaintext, tooltip="range: 1 - 2`,
		"\ncharacter: 'q'\"]\n",
		"[tooltip=1]\n",
		"}\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the graph has no %q:\n%s", want, got)
		}
	}
}

func TestIsRepetition(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	if !repetition(&pool, l, 3).isRepetition() {
		t.Error("a hidden node of a repetition is not a repetition")
	}
	if expression(&pool, l).isRepetition() {
		t.Error("a visible node is a repetition")
	}
	hidden := newLeaf(&pool, testSymRepeat, ln(0), ln(300), 1, 0, false, false, false, l)
	if hidden.isRepetition() || leaf(&pool, l, testSymRepeat, 0, 1).isRepetition() {
		t.Error("a hidden leaf is a repetition")
	}
}

func TestSetSymbol(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	s := leaf(&pool, l, testSymIdentifier, 0, 1)
	s.setSymbol(testSymPlus, l)
	if s.symbol() != testSymPlus || s.named() || !s.visible() {
		t.Errorf("setSymbol gave %+v", s.ptr.heapFields)
	}
	n := expression(&pool, l)
	n.setSymbol(testSymRepeat, l)
	if n.symbol() != testSymRepeat || n.named() || n.visible() {
		t.Errorf("setSymbol of a node gave %+v", n.ptr.heapFields)
	}
	// An inline leaf has a symbol of 8 bits, and C asserts that it is below
	// 255. The language wide has the metadata of 300 symbols.
	tables := testTables()
	tables.SymbolMetadata = append(tables.SymbolMetadata, make([]abi.SymbolMetadata, 300)...)
	wide := NewLanguage(tables)
	n.setSymbol(255, wide)
	defer func() {
		if r := recover(); r != "transit: assertion failed" {
			t.Errorf("setSymbol of an inline leaf to 255 panicked with %v, want the assertion", r)
		}
	}()
	s.setSymbol(255, wide)
}
