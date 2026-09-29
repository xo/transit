package transit

import "math"

// This file ports lib/src/reusable_node.h. reusable_node_delete has no Go
// form, because the garbage collector frees the stack.

// reusableNodeEntry is StackEntry of reusable_node.h. The name StackEntry
// would read as an entry of the parse stack of stack.go.
type reusableNodeEntry struct {
	tree       subtree
	childIndex uint32
	byteOffset uint32
}

// reusableNode is ReusableNode, a walk over the old tree that finds the
// subtrees that a parse can reuse.
type reusableNode struct {
	stack             []reusableNodeEntry
	lastExternalToken subtree
}

// newReusableNode is reusable_node_new.
func newReusableNode() reusableNode {
	return reusableNode{stack: nil, lastExternalToken: subtree{}}
}

// clear is reusable_node_clear.
func (r *reusableNode) clear() {
	r.stack = r.stack[:0]
	r.lastExternalToken = subtree{}
}

// tree is reusable_node_tree.
func (r *reusableNode) tree() subtree {
	if len(r.stack) > 0 {
		return r.stack[len(r.stack)-1].tree
	}
	return subtree{}
}

// byteOffset is reusable_node_byte_offset.
func (r *reusableNode) byteOffset() uint32 {
	if len(r.stack) > 0 {
		return r.stack[len(r.stack)-1].byteOffset
	}
	return math.MaxUint32
}

// advance is reusable_node_advance.
func (r *reusableNode) advance() {
	lastEntry := r.stack[len(r.stack)-1]
	byteOffset := lastEntry.byteOffset + lastEntry.tree.totalBytes()
	if lastEntry.tree.hasExternalTokens() {
		r.lastExternalToken = lastEntry.tree.lastExternalToken()
	}

	var tree subtree
	var nextIndex uint32
	for {
		poppedEntry := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		nextIndex = poppedEntry.childIndex + 1
		if len(r.stack) == 0 {
			return
		}
		tree = r.stack[len(r.stack)-1].tree
		if tree.childCount() > nextIndex {
			break
		}
	}

	r.stack = append(r.stack, reusableNodeEntry{
		tree:       tree.ptr.children[nextIndex],
		childIndex: nextIndex,
		byteOffset: byteOffset,
	})
}

// descend is reusable_node_descend.
func (r *reusableNode) descend() bool {
	lastEntry := r.stack[len(r.stack)-1]
	if lastEntry.tree.childCount() > 0 {
		r.stack = append(r.stack, reusableNodeEntry{
			tree:       lastEntry.tree.ptr.children[0],
			childIndex: 0,
			byteOffset: lastEntry.byteOffset,
		})
		return true
	}
	return false
}

// advancePastLeaf is reusable_node_advance_past_leaf.
func (r *reusableNode) advancePastLeaf() {
	for r.descend() {
	}
	r.advance()
}

// reset is reusable_node_reset.
func (r *reusableNode) reset(tree subtree) {
	r.clear()
	r.stack = append(r.stack, reusableNodeEntry{
		tree:       tree,
		childIndex: 0,
		byteOffset: 0,
	})

	// Never reuse the root node, because it has a non-standard internal structure
	// due to transformations that are applied when it is accepted: adding the EOF
	// child and any extra children.
	if !r.descend() {
		r.clear()
	}
}
