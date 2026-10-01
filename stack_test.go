package transit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// testStackNew returns a new stack, its subtree pool and the test language.
func testStackNew() (*stack, *subtreePool, *Language) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	return newStack(&pool), &pool, l
}

// stackTree returns a leaf of the test language that is not inline, so that
// it has a reference count. Its lookahead of 16 bytes is too long for an
// inline leaf.
func stackTree(pool *subtreePool, l *Language, symbol Symbol, padding, size uint32) subtree {
	return newLeaf(pool, symbol, ln(padding), ln(size), 16, 0, false, false, false, l)
}

// stackParent returns a node of the symbol expression over one leaf of a
// size of 1, with the dynamic precedence precedence.
func stackParent(pool *subtreePool, l *Language, precedence int32) subtree {
	n := newNode(pool, testSymExpression, subtreeArray{stackTree(pool, l, testSymIdentifier, 0, 1)}, 0, l)
	n.ptr.dynamicPrecedence = precedence
	return n
}

// stackCount returns the reference count of a subtree.
func stackCount(s subtree) uint32 {
	return s.ptr.refCount.Load()
}

// testStackCounts reports each subtree whose count is not the count in want.
func testStackCounts(t *testing.T, name string, trees []subtree, want []uint32) {
	t.Helper()
	for i, s := range trees {
		if got := stackCount(s); got != want[i] {
			t.Errorf("%s: the count of subtree %d is %d, want %d", name, i, got, want[i])
		}
	}
}

// testStackPanics reports an error when f does not fail an assertion.
func testStackPanics(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != "transit: assertion failed" {
			t.Errorf("%s panicked with %v, want the assertion", name, r)
		}
	}()
	f()
}

func TestStackNew(t *testing.T) {
	st, _, _ := testStackNew()
	if st.versionCount() != 1 || st.haltedVersionCount() != 0 {
		t.Fatalf("a new stack has %d versions and %d halted versions", st.versionCount(), st.haltedVersionCount())
	}
	if st.state(0) != 1 || st.position(0) != lengthZero() {
		t.Errorf("a new stack has the state %d and the position %v", st.state(0), st.position(0))
	}
	if !st.isActive(0) || st.isPaused(0) || st.isHalted(0) {
		t.Error("the version of a new stack is not active")
	}
	if st.errorCost(0) != 0 || st.nodeCountSinceError(0) != 0 || st.dynamicPrecedence(0) != 0 {
		t.Error("a new stack has an error cost, a node count or a precedence")
	}
	if st.lastExternalToken(0).ptr != nil || st.getSummary(0) != nil {
		t.Error("a new stack has an external token or a summary")
	}
	if !st.hasAdvancedSinceError(0) {
		t.Error("a new stack has not advanced since an error")
	}
	// newStack makes the base node, and clear retains it for the head
	if st.baseNode.refCount != 2 {
		t.Errorf("the base node has the count %d, want 2", st.baseNode.refCount)
	}
	if stackVersionNone != stackVersion(1<<32-1) {
		t.Error("stackVersionNone is not the largest version")
	}
}

func TestStackPushAndPopCount(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 1, 2)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	c := stackTree(pool, l, testSymIdentifier, 1, 1)
	st.push(0, a, false, 2)
	st.push(0, b, false, 3)
	st.push(0, c, false, 4)
	if st.state(0) != 4 || st.position(0) != ln(6) {
		t.Errorf("after three pushes, the state is %d and the position is %v", st.state(0), st.position(0))
	}
	// each of the three leaves is visible
	if got := st.nodeCountSinceError(0); got != 3 {
		t.Errorf("nodeCountSinceError() = %d, want 3", got)
	}

	pop := st.popCount(0, 2)
	if len(pop) != 1 || pop[0].version != 1 || !slices.Equal(pop[0].subtrees, subtreeArray{b, c}) {
		t.Fatalf("popCount(0, 2) = %v", pop)
	}
	if &pop[0] != &st.slices[0] {
		t.Error("the result of popCount is not the array slices of the stack")
	}
	if st.versionCount() != 2 || st.state(1) != 2 || st.position(1) != ln(3) {
		t.Errorf("after the pop, the new version has the state %d and the position %v", st.state(1), st.position(1))
	}
	// the pop retains each subtree of the slice
	testStackCounts(t, "after the pop", []subtree{a, b, c}, []uint32{1, 2, 2})

	// the removed version releases the two nodes above a and their subtrees
	st.removeVersion(0)
	testStackCounts(t, "after the remove", []subtree{a, b, c}, []uint32{1, 1, 1})
	if st.versionCount() != 1 || st.state(0) != 2 || st.heads[0].node.refCount != 1 {
		t.Errorf("after the remove, the stack has %d versions and the state %d", st.versionCount(), st.state(0))
	}
}

func TestStackPopCountSkipsExtras(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	// a leaf of the end is an extra
	e := stackTree(pool, l, testSymEnd, 0, 1)
	b := stackTree(pool, l, testSymIdentifier, 0, 1)
	st.copyVersion(0)
	st.push(1, a, false, 2)
	st.push(1, e, false, 2)
	st.push(1, b, false, 3)

	// the pop of one tree stops at the node after b
	pop := st.popCount(1, 1)
	if len(pop) != 1 || pop[0].version != 2 || !slices.Equal(pop[0].subtrees, subtreeArray{b}) || st.state(2) != 2 {
		t.Errorf("popCount(1, 1) = %v", pop)
	}
	st.removeVersion(pop[0].version)
	pop[0].subtrees.delete(pool)

	// the extra does not count
	pop = st.popCount(1, 2)
	if len(pop) != 1 || pop[0].version != 2 || !slices.Equal(pop[0].subtrees, subtreeArray{a, e, b}) || st.state(2) != 1 {
		t.Errorf("popCount(1, 2) = %v", pop)
	}
}

func TestStackPopCountRevealsMergedVersions(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 2)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	c := stackTree(pool, l, testSymExpression, 0, 1)
	d := stackTree(pool, l, testSymPlus, 0, 2)
	if v := st.copyVersion(0); v != 1 {
		t.Fatalf("copyVersion(0) = %d, want 1", v)
	}
	if st.baseNode.refCount != 3 {
		t.Errorf("after the copy, the base node has the count %d, want 3", st.baseNode.refCount)
	}
	st.push(0, a, false, 2)
	st.push(0, b, false, 5)
	st.push(1, c, false, 3)
	st.push(1, d, false, 5)
	nodeA := st.heads[0].node.links[0].node
	nodeC := st.heads[1].node.links[0].node

	if !st.canMerge(0, 1) || !st.merge(0, 1) {
		t.Fatal("the two versions do not merge")
	}
	if st.versionCount() != 1 || st.heads[0].node.linkCount != 2 {
		t.Fatalf("after the merge, the stack has %d versions and %d links", st.versionCount(), st.heads[0].node.linkCount)
	}
	// the merge retains d and nodeC, and the remove releases them
	testStackCounts(t, "after the merge", []subtree{a, b, c, d}, []uint32{1, 1, 1, 1})
	if nodeA.refCount != 1 || nodeC.refCount != 1 {
		t.Errorf("after the merge, the nodes have the counts %d and %d", nodeA.refCount, nodeC.refCount)
	}

	pop := st.popCount(0, 1)
	if len(pop) != 2 {
		t.Fatalf("popCount(0, 1) gave %d slices, want 2", len(pop))
	}
	if pop[0].version != 1 || !slices.Equal(pop[0].subtrees, subtreeArray{b}) ||
		pop[1].version != 2 || !slices.Equal(pop[1].subtrees, subtreeArray{d}) {
		t.Errorf("popCount(0, 1) = %v", pop)
	}
	if st.versionCount() != 3 || st.state(1) != 2 || st.state(2) != 3 {
		t.Errorf("the pop revealed the states %d and %d", st.state(1), st.state(2))
	}
	testStackCounts(t, "after the pop", []subtree{a, b, c, d}, []uint32{1, 2, 1, 2})

	st.removeVersion(0)
	testStackCounts(t, "after the remove", []subtree{a, b, c, d}, []uint32{1, 1, 1, 1})
	if nodeA.refCount != 1 || nodeC.refCount != 1 || st.versionCount() != 2 {
		t.Errorf("after the remove, the nodes have the counts %d and %d", nodeA.refCount, nodeC.refCount)
	}
}

func TestStackMergeOfLinksToTheSameNode(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	st.push(0, a, false, 2)
	st.copyVersion(0)
	x := stackParent(pool, l, 1)
	y := stackParent(pool, l, 4)
	child := x.ptr.children[0]
	st.push(0, x, false, 5)
	st.push(1, y, false, 5)
	if st.dynamicPrecedence(1) != 4 {
		t.Errorf("dynamicPrecedence(1) = %d, want 4", st.dynamicPrecedence(1))
	}
	if !stackSubtreeIsEquivalent(x, y) {
		t.Fatal("x and y are not equivalent")
	}
	if !st.merge(0, 1) {
		t.Fatal("the two versions do not merge")
	}
	// y has the larger precedence, so it takes the place of x
	head := st.heads[0].node
	if head.linkCount != 1 || head.links[0].subtree != y || st.dynamicPrecedence(0) != 4 {
		t.Errorf("after the merge, the node has %d links and the precedence %d", head.linkCount, st.dynamicPrecedence(0))
	}
	testStackCounts(t, "after the merge", []subtree{a, x, y, child}, []uint32{1, 0, 1, 0})
	if head.links[0].node.refCount != 1 {
		t.Errorf("the node of a has the count %d, want 1", head.links[0].node.refCount)
	}
}

func TestStackMergeOfMergeableNodes(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 2)
	c := stackTree(pool, l, testSymPlus, 0, 2)
	x := stackParent(pool, l, 1)
	y := stackParent(pool, l, 4)
	st.copyVersion(0)
	st.push(0, a, false, 2)
	st.push(0, x, false, 5)
	st.push(1, c, false, 2)
	st.push(1, y, false, 5)
	if !st.merge(0, 1) {
		t.Fatal("the two versions do not merge")
	}
	// the node of c merges into the node of a, and y is dropped
	head := st.heads[0].node
	nodeA := head.links[0].node
	if head.linkCount != 1 || nodeA.linkCount != 2 || nodeA.links[1].subtree != c {
		t.Fatalf("after the merge, the nodes have %d and %d links", head.linkCount, nodeA.linkCount)
	}
	// the precedence of the path through y stays
	if st.dynamicPrecedence(0) != 4 {
		t.Errorf("dynamicPrecedence() = %d, want 4", st.dynamicPrecedence(0))
	}
	testStackCounts(t, "after the merge", []subtree{a, c, x, y}, []uint32{1, 1, 1, 0})

	// two paths reach the base node, so the two slices share a version
	pop := st.popCount(0, 2)
	if len(pop) != 2 || pop[0].version != 1 || pop[1].version != 1 ||
		!slices.Equal(pop[0].subtrees, subtreeArray{a, x}) ||
		!slices.Equal(pop[1].subtrees, subtreeArray{c, x}) {
		t.Errorf("popCount(0, 2) = %v", pop)
	}
	if st.versionCount() != 2 || st.state(1) != 1 {
		t.Errorf("the pop gave %d versions", st.versionCount())
	}
	testStackCounts(t, "after the pop", []subtree{a, c, x}, []uint32{2, 2, 3})
}

func TestStackMergeKeepsEightLinks(t *testing.T) {
	st, pool, l := testStackNew()
	var trees []subtree
	for range maxLinkCount {
		st.copyVersion(0)
	}
	for i := range uint32(maxLinkCount + 1) {
		// the paddings differ, so no two trees are equivalent
		tree := stackTree(pool, l, testSymIdentifier, i, 20-i)
		trees = append(trees, tree)
		st.push(stackVersion(i), tree, false, 5)
	}
	for range maxLinkCount {
		if !st.merge(0, 1) {
			t.Fatal("the versions do not merge")
		}
	}
	if st.versionCount() != 1 || st.heads[0].node.linkCount != maxLinkCount {
		t.Fatalf("after the merges, the stack has %d versions and %d links", st.versionCount(), st.heads[0].node.linkCount)
	}
	// the node has no room for the last link, so the remove frees its tree
	want := slices.Repeat([]uint32{1}, maxLinkCount)
	testStackCounts(t, "after the merges", trees, append(want, 0))
}

func TestStackPopPending(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	st.push(0, a, false, 2)
	st.push(0, b, true, 3)

	pop := st.popPending(0)
	if len(pop) != 1 || pop[0].version != 0 || !slices.Equal(pop[0].subtrees, subtreeArray{b}) {
		t.Fatalf("popPending(0) = %v", pop)
	}
	if st.versionCount() != 1 || st.state(0) != 2 {
		t.Errorf("after the pop, the stack has %d versions and the state %d", st.versionCount(), st.state(0))
	}
	testStackCounts(t, "after the pop", []subtree{a, b}, []uint32{1, 1})

	// a is not pending, so nothing pops, and the pop releases what it took
	if pop := st.popPending(0); len(pop) != 0 {
		t.Errorf("popPending(0) of a tree that is not pending = %v", pop)
	}
	if st.versionCount() != 1 || st.state(0) != 2 {
		t.Errorf("after the empty pop, the stack has %d versions and the state %d", st.versionCount(), st.state(0))
	}
	testStackCounts(t, "after the empty pop", []subtree{a}, []uint32{1})
}

func TestStackPopError(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	e := newError(pool, 'x', ln(0), ln(300), 1, 0, l)
	st.push(0, a, false, 2)
	if got := st.popError(0); got != nil {
		t.Errorf("popError(0) with no error = %v", got)
	}
	st.push(0, e, false, errorState)

	got := st.popError(0)
	if !slices.Equal(got, subtreeArray{e}) {
		t.Fatalf("popError(0) = %v", got)
	}
	if st.versionCount() != 1 || st.state(0) != 2 {
		t.Errorf("after the pop, the stack has %d versions and the state %d", st.versionCount(), st.state(0))
	}
	testStackCounts(t, "after the pop", []subtree{a, e}, []uint32{1, 1})
}

func TestStackPopAll(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	st.push(0, a, false, 2)
	st.push(0, b, true, 3)
	pop := st.popAll(0)
	if len(pop) != 1 || pop[0].version != 1 || !slices.Equal(pop[0].subtrees, subtreeArray{a, b}) {
		t.Fatalf("popAll(0) = %v", pop)
	}
	if st.versionCount() != 2 || st.state(1) != 1 || st.heads[1].node != st.baseNode {
		t.Error("popAll did not reveal the base node")
	}
	testStackCounts(t, "after the pop", []subtree{a, b}, []uint32{2, 2})
}

func TestStackPauseResumeHalt(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	look := stackTree(pool, l, testSymPlus, 0, 1)
	st.push(0, a, false, 2)
	st.copyVersion(0)

	st.pause(1, look)
	if !st.isPaused(1) || st.isActive(1) || st.errorCost(1) != errorCostPerRecovery {
		t.Errorf("the paused version has the status %v and the error cost %d", st.heads[1].status, st.errorCost(1))
	}
	if st.nodeCountSinceError(1) != 0 || st.nodeCountSinceError(0) != 1 {
		t.Error("pause did not record the node count")
	}
	if st.canMerge(0, 1) || st.merge(0, 1) {
		t.Error("a paused version merges")
	}
	if got := st.resume(1); got != look || !st.isActive(1) || st.heads[1].lookaheadWhenPaused.ptr != nil {
		t.Error("resume did not give back the lookahead")
	}
	testStackPanics(t, "resume of an active version", func() { st.resume(1) })

	// the head takes the reference of the lookahead
	st.pause(1, look)
	st.removeVersion(1)
	testStackCounts(t, "after the remove", []subtree{look}, []uint32{0})

	st.halt(0)
	if !st.isHalted(0) || st.haltedVersionCount() != 1 {
		t.Error("halt did not halt the version")
	}
}

func TestStackCopyVersionDoesNotRetainTheLookahead(t *testing.T) {
	st, pool, l := testStackNew()
	look := stackTree(pool, l, testSymPlus, 0, 1)
	st.pause(0, look)
	v := st.copyVersion(0)
	// C copies the head with its lookahead and does not retain it
	if st.heads[v].lookaheadWhenPaused != look || !st.isPaused(v) {
		t.Error("the copy does not have the lookahead")
	}
	testStackCounts(t, "after the copy", []subtree{look}, []uint32{1})
}

func TestStackVersions(t *testing.T) {
	st, pool, l := testStackNew()
	token := newLeaf(pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	token.ptr.externalScannerState.init(pool, []byte{1, 2})
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	st.setLastExternalToken(0, token)
	st.push(0, a, false, 2)
	st.recordSummary(0, 5)

	v := st.copyVersion(0)
	if v != 1 || st.heads[1].node != st.heads[0].node || st.heads[0].node.refCount != 2 {
		t.Fatal("copyVersion did not share the node")
	}
	if st.lastExternalToken(1) != token || st.getSummary(1) != nil || st.getSummary(0) == nil {
		t.Error("the copy does not have the token, or it has a summary")
	}
	testStackCounts(t, "after the copy", []subtree{token}, []uint32{3})

	st.push(1, b, false, 3)
	st.swapVersions(0, 1)
	if st.state(0) != 3 || st.state(1) != 2 || st.getSummary(1) == nil {
		t.Error("swapVersions did not swap the versions")
	}
	st.swapVersions(0, 1)

	// the source takes the summary of the target, and the target is released
	nodeA := st.heads[0].node
	st.renumberVersion(1, 0)
	if st.versionCount() != 1 || st.state(0) != 3 || st.getSummary(0) == nil {
		t.Errorf("after renumberVersion, the stack has %d versions and the state %d", st.versionCount(), st.state(0))
	}
	if nodeA.refCount != 1 {
		t.Errorf("after renumberVersion, the node of a has the count %d, want 1", nodeA.refCount)
	}
	testStackCounts(t, "after renumberVersion", []subtree{token, a, b}, []uint32{2, 1, 1})

	st.renumberVersion(0, 0)
	if st.versionCount() != 1 {
		t.Error("renumberVersion of a version to itself changed the stack")
	}
	st.copyVersion(0)
	testStackPanics(t, "renumberVersion to a later version", func() { st.renumberVersion(0, 1) })
}

func TestStackLastExternalToken(t *testing.T) {
	st, pool, l := testStackNew()
	token1 := newLeaf(pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	token1.ptr.externalScannerState.init(pool, []byte{1})
	token2 := newLeaf(pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	token2.ptr.externalScannerState.init(pool, []byte{2})

	st.setLastExternalToken(0, token1)
	st.copyVersion(0)
	st.setLastExternalToken(1, token2)
	if st.lastExternalToken(0) != token1 || st.lastExternalToken(1) != token2 {
		t.Error("the versions do not have their tokens")
	}
	testStackCounts(t, "after the sets", []subtree{token1, token2}, []uint32{2, 2})
	// the states of the scanner differ, so the versions do not merge
	if st.canMerge(0, 1) {
		t.Error("two versions with different states of the scanner can merge")
	}
	st.setLastExternalToken(1, subtree{})
	testStackCounts(t, "after the set to NULL", []subtree{token1, token2}, []uint32{2, 1})
	if st.lastExternalToken(1).ptr != nil {
		t.Error("the token of version 1 is not NULL")
	}
}

func TestStackErrorCostAndNodeCount(t *testing.T) {
	st, pool, l := testStackNew()
	st.push(0, stackTree(pool, l, testSymIdentifier, 0, 1), false, 2)
	if got := st.nodeCountSinceError(0); got != 1 {
		t.Errorf("nodeCountSinceError() = %d, want 1", got)
	}

	// a push of NULL in the error state records an error
	st.push(0, subtree{}, false, errorState)
	if got := st.errorCost(0); got != errorCostPerRecovery {
		t.Errorf("errorCost() = %d, want %d", got, errorCostPerRecovery)
	}
	if got := st.nodeCountSinceError(0); got != 0 {
		t.Errorf("nodeCountSinceError() after the error = %d, want 0", got)
	}

	// _ERROR is not visible, and it counts as a node
	repeat := newNode(pool, builtinSymErrorRepeat, subtreeArray{stackTree(pool, l, testSymIdentifier, 0, 3)}, 0, l)
	st.push(0, repeat, false, errorState)
	if got := st.nodeCountSinceError(0); got != 2 {
		t.Errorf("nodeCountSinceError() after _ERROR = %d, want 2", got)
	}
	if got := st.errorCost(0); got != repeat.errorCost() {
		t.Errorf("errorCost() after _ERROR = %d, want %d", got, repeat.errorCost())
	}

	// a count below the count of the last error resets it
	st.heads[0].nodeCountAtLastError = 100
	if got := st.nodeCountSinceError(0); got != 0 || st.heads[0].nodeCountAtLastError != 3 {
		t.Errorf("nodeCountSinceError() = %d with the count at the error %d", got, st.heads[0].nodeCountAtLastError)
	}
}

func TestStackHasAdvancedSinceError(t *testing.T) {
	st, pool, l := testStackNew()
	if !st.hasAdvancedSinceError(st.copyVersion(0)) {
		t.Error("a new version has not advanced since an error")
	}
	st.push(0, newMissingLeaf(pool, testSymIdentifier, 0, ln(0), 0, l), false, 2)
	if st.hasAdvancedSinceError(0) {
		t.Error("a version that holds only a missing leaf has advanced")
	}
	st.push(0, stackTree(pool, l, testSymPlus, 0, 0), false, 3)
	if st.hasAdvancedSinceError(0) {
		t.Error("a version that holds an empty leaf after a missing leaf has advanced")
	}
	st.push(0, stackTree(pool, l, testSymIdentifier, 0, 1), false, 4)
	if !st.hasAdvancedSinceError(0) {
		t.Error("a version with a leaf of one byte has not advanced")
	}
}

func TestStackDynamicPrecedence(t *testing.T) {
	st, pool, l := testStackNew()
	st.push(0, stackParent(pool, l, 3), false, 2)
	if got := st.dynamicPrecedence(0); got != 3 {
		t.Errorf("dynamicPrecedence() = %d, want 3", got)
	}
	st.push(0, stackParent(pool, l, -1), false, 3)
	if got := st.dynamicPrecedence(0); got != 2 {
		t.Errorf("dynamicPrecedence() = %d, want 2", got)
	}
}

func TestStackRecordSummary(t *testing.T) {
	st, pool, l := testStackNew()
	trees := []subtree{
		stackTree(pool, l, testSymIdentifier, 0, 2),
		stackTree(pool, l, testSymPlus, 0, 1),
		stackTree(pool, l, testSymExpression, 0, 1),
		stackTree(pool, l, testSymPlus, 0, 2),
	}
	st.copyVersion(0)
	st.push(0, trees[0], false, 2)
	st.push(0, trees[1], false, 5)
	st.push(1, trees[2], false, 3)
	st.push(1, trees[3], false, 5)
	// the node of version 1 takes the link of version 0, and version 1
	// becomes version 0
	if !st.merge(1, 0) || st.copyVersion(0) != 1 {
		t.Fatal("the versions do not merge")
	}

	// the second path to the base node at the depth 2 adds no entry
	st.recordSummary(1, 5)
	want := stackSummary{{ln(3), 0, 5}, {ln(1), 1, 3}, {ln(2), 1, 2}, {ln(0), 2, 1}}
	if got := st.getSummary(1); got == nil || !slices.Equal(*got, want) {
		t.Errorf("the summary is %v, want %v", got, want)
	}
	if st.getSummary(0) != nil {
		t.Error("the summary of version 1 is on version 0")
	}
	st.recordSummary(1, 1)
	if got := st.getSummary(1); got == nil || !slices.Equal(*got, want[:3]) {
		t.Errorf("the summary of the depth 1 is %v, want %v", got, want[:3])
	}
	// a summary does not retain the subtrees
	testStackCounts(t, "after the summary", trees, []uint32{1, 1, 1, 1})
}

func TestStackClear(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 0, 1)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	st.push(0, a, false, 2)
	st.push(0, b, false, 3)
	st.copyVersion(0)
	st.clear()
	if st.versionCount() != 1 || st.state(0) != 1 || st.heads[0].node != st.baseNode {
		t.Error("clear did not reset the stack")
	}
	if st.baseNode.refCount != 2 {
		t.Errorf("after clear, the base node has the count %d, want 2", st.baseNode.refCount)
	}
	testStackCounts(t, "after clear", []subtree{a, b}, []uint32{0, 0})
}

func TestStackNodePool(t *testing.T) {
	st, pool, l := testStackNew()
	a := stackTree(pool, l, testSymIdentifier, 1, 2)
	b := stackTree(pool, l, testSymPlus, 0, 1)
	c := stackTree(pool, l, testSymIdentifier, 1, 1)
	st.push(0, a, false, 2)
	st.push(0, b, false, 3)
	st.push(0, c, false, 4)
	top := st.heads[0].node
	below := top.links[0].node
	st.popCount(0, 2)

	// the removed version releases the two nodes above a, from the top down
	st.removeVersion(0)
	if !slices.Equal(st.nodePool, stackNodeArray{top, below}) {
		t.Fatalf("after the remove, the free list is %v, want the two nodes above a", st.nodePool)
	}
	if *top != (stackNode{}) {
		t.Errorf("a node on the free list is not clear: %+v", *top)
	}

	// a push takes the last node of the free list
	d := stackTree(pool, l, testSymPlus, 0, 1)
	st.push(0, d, true, 5)
	node := st.heads[0].node
	if node != below || len(st.nodePool) != 1 {
		t.Fatalf("the push did not take the last node of the free list")
	}
	if node.refCount != 1 || node.state != 5 || node.linkCount != 1 || node.links[0].subtree != d || !node.links[0].isPending || node.position != ln(4) {
		t.Errorf("the node of the push is %+v", *node)
	}

	// the free list keeps up to maxNodePoolSize nodes
	for range 2 * maxNodePoolSize {
		st.push(0, stackTree(pool, l, testSymPlus, 0, 1), false, 3)
	}
	st.clear()
	if len(st.nodePool) != maxNodePoolSize {
		t.Errorf("after clear, the free list has %d nodes, want %d", len(st.nodePool), maxNodePoolSize)
	}
}

func TestStackPrintDotGraph(t *testing.T) {
	st, pool, l := testStackNew()
	token := newLeaf(pool, testSymPlus, ln(0), ln(1), 1, 0, true, false, false, l)
	token.ptr.externalScannerState.init(pool, []byte{0x01, 0xAB})
	extra := stackTree(pool, l, testSymEnd, 0, 1)
	st.push(0, stackTree(pool, l, testSymIdentifier, 0, 1), false, 2)
	st.push(0, stackTree(pool, l, testSymPlus, 0, 1), true, 3)
	st.push(0, subtree{}, false, errorState)
	st.push(0, extra, false, 4)
	st.setLastExternalToken(0, token)
	st.recordSummary(0, 1)
	st.pause(st.copyVersion(0), stackTree(pool, l, testSymPlus, 0, 1))
	st.halt(st.copyVersion(1))

	var b strings.Builder
	if !st.printDotGraph(l, &b) {
		t.Fatal("printDotGraph returned false")
	}
	got := b.String()
	top := fmt.Sprintf("node_%p", st.heads[0].node)
	for _, want := range []string{
		"digraph stack {\nrankdir=\"RL\";\nedge [arrowhead=none]\n",
		"node_head_0 [shape=none, label=\"\"]\nnode_head_0 -> " + top + " [label=0, fontcolor=blue, weight=10000, labeltooltip=\"node_count: 0\nerror_cost: 0",
		"\nsummary: 4 0 3",
		"\nexternal_scanner_state:  1 FFFFFFAB\"]\n",
		"node_head_1 -> " + top + " [color=red label=1, ",
		top + " [shape=point margin=0 label=\"\" tooltip=\"position: 1,3\nnode_count:2\nerror_cost: 0\ndynamic_precedence: 0\"];\n",
		"[fontcolor=gray label=\"end\"labeltooltip=\"error_cost: 0\ndynamic_precedence: 0\"];\n",
		"[label=\"?\" tooltip=",
		"[color=red];\n",
		"[style=dashed label=\"'+'\"labeltooltip=",
		"[label=\"identifier\"labeltooltip=",
		fmt.Sprintf("node_%p [label=\"1\" tooltip=\"position: 1,0\n", st.baseNode),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the graph has no %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "node_head_2") {
		t.Errorf("the graph has the halted version:\n%s", got)
	}
	if !strings.HasSuffix(got, "}\n") {
		t.Errorf("the graph does not end with a brace:\n%s", got)
	}
	// each node is written once
	if n := strings.Count(got, fmt.Sprintf("\nnode_%p [", st.baseNode)); n != 1 {
		t.Errorf("the base node is written %d times", n)
	}
}

func TestStackEnumStrings(t *testing.T) {
	for _, test := range []struct {
		got, want string
	}{
		{stackStatusActive.String(), "active"},
		{stackStatusPaused.String(), "paused"},
		{stackStatusHalted.String(), "halted"},
		{stackStatus(9).String(), unknownName},
		{stackActionNone.String(), "none"},
		{stackActionStop.String(), "stop"},
		{stackActionPop.String(), "pop"},
		{(stackActionPop | stackActionStop).String(), "stop|pop"},
		{stackAction(4).String(), unknownName},
	} {
		if test.got != test.want {
			t.Errorf("got %q, want %q", test.got, test.want)
		}
	}
}
