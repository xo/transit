package transit

import (
	"io"
	"slices"
)

// This file ports lib/src/tree.c and lib/src/tree.h.
//
// These parts have no Go form. The members of TSTree for WebAssembly and
// the WebAssembly branch of ts_tree_language are for WebAssembly (D1).
// Nothing in the runtime uses ParentCacheEntry of tree.h. _ts_dup exists
// only for the file descriptor of ts_tree_print_dot_graph, and the Go form
// of that function takes an io.Writer.

// Tree is a syntax tree. It is safe for reads from many goroutines. Edit
// changes it, so call Copy first to keep a version.
//
// Tree is TSTree.
type Tree struct {
	root           subtree
	language       *Language
	includedRanges []textRange
}

// newTree is ts_tree_new. It keeps a copy of includedRanges.
func newTree(root subtree, language *Language, includedRanges []textRange) *Tree {
	return &Tree{
		root:           root,
		language:       language,
		includedRanges: slices.Clone(includedRanges),
	}
}

// Copy returns a copy of the tree. The copy shares the nodes of the tree,
// so it costs little. Edit changes only the tree that it is called on.
//
// Copy is ts_tree_copy.
func (t *Tree) Copy() *Tree {
	t.root.retain()
	return newTree(t.root, t.language, t.includedRanges)
}

// closeTreeStackSize is the capacity of the tree stack that Close starts
// with.
const closeTreeStackSize = 64

// Close releases the nodes of the tree. After Close, the program must not
// use the tree, or a node or a cursor of it, and the runtime does not check
// this. A node that another tree shares, after Copy or after a parse with
// the tree as the old tree, stays in the other tree. Close is optional. The
// garbage collector frees a tree that the program does not close (D91).
//
// Close is ts_tree_delete. The C function releases the root into a pool of
// its own, whose capacity is 0, so no parser takes the freed nodes again.
// The Go function does the same, and then clears the tree, so that it keeps
// no node alive.
func (t *Tree) Close() {
	if t == nil {
		return
	}

	// The tree stack starts at a size that a release seldom passes, so that
	// it does not grow in several steps.
	pool := newSubtreePool(0)
	pool.treeStack = make(subtreeArray, 0, closeTreeStackSize)
	t.root.release(&pool)
	*t = Tree{}
}

// RootNode returns the root node of the tree.
//
// RootNode is ts_tree_root_node.
func (t *Tree) RootNode() Node {
	return makeNode(t, &t.root, t.root.padding(), 0)
}

// RootNodeWithOffset returns the root node of the tree, moved forward by a
// number of bytes and by a point.
//
// RootNodeWithOffset is ts_tree_root_node_with_offset.
func (t *Tree) RootNodeWithOffset(offset int, at Point) Node {
	offsetLength := length{uint32(offset), at.internal()}
	return makeNode(t, &t.root, offsetLength.add(t.root.padding()), 0)
}

// Language returns the language of the tree.
//
// Language is ts_tree_language.
func (t *Tree) Language() *Language {
	return t.language
}

// Edit changes the tree to match an edit of the text. Give the edit both in
// bytes and in points.
//
// Edit is ts_tree_edit.
func (t *Tree) Edit(e InputEdit) {
	for i := range t.includedRanges {
		t.includedRanges[i].edit(e)
	}

	pool := newSubtreePool(0)
	t.root = t.root.edit(e, &pool)
}

// IncludedRanges returns a copy of the included ranges of the parse that
// made the tree.
//
// IncludedRanges is ts_tree_included_ranges.
func (t *Tree) IncludedRanges() []Range {
	ranges := make([]Range, len(t.includedRanges))
	for i, r := range t.includedRanges {
		ranges[i] = r.public()
	}
	return ranges
}

// ChangedRanges compares the tree, after an edit, with a new tree of the same
// text. It returns the ranges whose structure of nodes changed. Call it on
// the old tree that the parse used, with the new tree that the parse
// returned.
//
// ChangedRanges is ts_tree_get_changed_ranges.
func (t *Tree) ChangedRanges(newTree *Tree) []Range {
	oldTree := t
	var cursor1, cursor2 TreeCursor
	cursor1.init(oldTree.RootNode())
	cursor2.init(newTree.RootNode())

	var includedRangeDifferences []textRange
	rangeArrayGetChangedRanges(
		oldTree.includedRanges,
		newTree.includedRanges,
		&includedRangeDifferences,
	)

	result := subtreeGetChangedRanges(
		&oldTree.root, &newTree.root, &cursor1, &cursor2,
		oldTree.language, includedRangeDifferences,
	)

	ranges := make([]Range, len(result))
	for i, r := range result {
		ranges[i] = r.public()
	}
	return ranges
}

// PrintDotGraph writes a graph of the tree to w, in the DOT language. The
// program dot of Graphviz draws it.
//
// PrintDotGraph is ts_tree_print_dot_graph. The C function writes to a file
// descriptor, and the Go function writes to w.
func (t *Tree) PrintDotGraph(w io.Writer) {
	t.root.printDotGraph(t.language, w)
}

// Walk returns a cursor at the root node of the tree.
//
// Walk is walk of Tree in the Rust binding.
func (t *Tree) Walk() *TreeCursor {
	return t.RootNode().Walk()
}
