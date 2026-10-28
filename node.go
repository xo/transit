package transit

import (
	"iter"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/node.c, and the type TSNode of
// lib/include/tree_sitter/api.h.
//
// A method whose C function can return a null node returns the node and a
// bool, which is false for the null node (D56). The zero Node is the null
// node. ts_node_is_null has no exported form, and it is the method isNull,
// because a lookup returns a bool with the node.
//
// Two nodes compare with ==, which also compares the position of the node.
// Equal is ts_node_eq, which compares only the tree and the node, so a node
// that Node.Edit moved is Equal to the node before the edit, and not == to
// it (D68).
//
// The exported API counts with int (D25). A function of this file that
// needs the width of C converts the int back to a uint32.
//
// The methods at the end of the file come from the Rust binding, and they
// port no C function.

// Node is a node of a tree. It is a small value, and two nodes compare with
// ==, which also compares their positions. Equal compares them as C does
// (D68). The zero Node is no node.
//
// Node is TSNode. context holds the start byte, the start row, the start
// column and the alias of the node, as in C. id is the address of the
// subtree of the node: the root of the tree, or a child in the children of
// its parent.
type Node struct {
	context [4]uint32
	id      *subtree
	tree    *Tree
}

// nodeChildIterator is NodeChildIterator.
type nodeChildIterator struct {
	parent               subtree
	tree                 *Tree
	position             length
	childIndex           uint32
	structuralChildIndex uint32
	aliasSequence        []uint16
}

// tsNodeMaxRecursiveSearchDepth is TS_NODE_MAX_RECURSIVE_SEARCH_DEPTH.
const tsNodeMaxRecursiveSearchDepth uint32 = 64

// TSNode - constructors

// makeNode is ts_node_new. The name newNode is ts_subtree_new_node.
func makeNode(tree *Tree, subtree *subtree, position length, alias Symbol) Node {
	return Node{
		context: [4]uint32{position.bytes, position.extent.row, position.extent.column, uint32(alias)},
		id:      subtree,
		tree:    tree,
	}
}

// nullNode is ts_node__null.
func nullNode() Node {
	return makeNode(nil, nil, lengthZero(), 0)
}

// TSNode - accessors

// StartByte returns the byte offset where the node starts.
//
// StartByte is ts_node_start_byte.
func (n Node) StartByte() int {
	return int(n.context[0])
}

// StartPoint returns the point where the node starts.
//
// StartPoint is ts_node_start_point.
func (n Node) StartPoint() Point {
	return point{n.context[1], n.context[2]}.public()
}

// alias is ts_node__alias.
func (n Node) alias() Symbol {
	return Symbol(n.context[3])
}

// subtree is ts_node__subtree.
func (n Node) subtree() subtree {
	return *n.id
}

// NodeChildIterator

// iterateChildren is ts_node_iterate_children.
func (n Node) iterateChildren() nodeChildIterator {
	subtree := n.subtree()
	if subtree.childCount() == 0 {
		return nodeChildIterator{tree: n.tree, position: lengthZero()}
	}
	aliasSequence := n.tree.language.aliasSequence(uint32(subtree.ptr.productionID))
	return nodeChildIterator{
		tree:                 n.tree,
		parent:               subtree,
		position:             length{uint32(n.StartByte()), n.StartPoint().internal()},
		childIndex:           0,
		structuralChildIndex: 0,
		aliasSequence:        aliasSequence,
	}
}

// done is ts_node_child_iterator_done.
func (it *nodeChildIterator) done() bool {
	return it.childIndex == it.parent.childCount()
}

// next is ts_node_child_iterator_next. The C function writes the child to an
// out parameter, and the Go function returns it.
func (it *nodeChildIterator) next() (Node, bool) {
	if it.parent.ptr == nil || it.done() {
		return Node{}, false
	}
	child := &it.parent.ptr.children[it.childIndex]
	aliasSymbol := Symbol(0)
	if !child.extra() {
		if it.aliasSequence != nil {
			aliasSymbol = Symbol(it.aliasSequence[it.structuralChildIndex])
		}
		it.structuralChildIndex++
	}
	if it.childIndex > 0 {
		it.position = it.position.add(child.padding())
	}
	result := makeNode(
		it.tree,
		child,
		it.position,
		aliasSymbol,
	)
	it.position = it.position.add(child.size())
	it.childIndex++
	return result, true
}

// TSNode - private

// isRelevant is ts_node__is_relevant.
func (n Node) isRelevant(includeAnonymous bool) bool {
	tree := n.subtree()
	if includeAnonymous {
		return tree.visible() || n.alias() != 0
	}
	alias := n.alias()
	if alias != 0 {
		return n.tree.language.symbolMetadata(alias).Named
	}
	return tree.visible() && tree.named()
}

// relevantChildCount is ts_node__relevant_child_count.
func (n Node) relevantChildCount(includeAnonymous bool) uint32 {
	tree := n.subtree()
	if tree.childCount() > 0 {
		if includeAnonymous {
			return tree.ptr.visibleChildCount
		}
		return tree.ptr.namedChildCount
	}
	return 0
}

// child is ts_node__child.
func (n Node) child(childIndex uint32, includeAnonymous bool) Node {
	result := n
	didDescend := true

	for didDescend {
		didDescend = false

		index := uint32(0)
		iterator := result.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			if child.isRelevant(includeAnonymous) {
				if index == childIndex {
					return child
				}
				index++
			} else {
				grandchildIndex := childIndex - index
				grandchildCount := child.relevantChildCount(includeAnonymous)
				if grandchildIndex < grandchildCount {
					didDescend = true
					result = child
					childIndex = grandchildIndex
					break
				}
				index += grandchildCount
			}
		}
	}

	return nullNode()
}

// hasTrailingEmptyDescendantIterative is
// ts_subtree_has_trailing_empty_descendant_iterative. other is the slot of
// the target in the children of its parent, as the id of a node is.
func (s subtree) hasTrailingEmptyDescendantIterative(other *subtree) bool {
	var stack []subtree
	subtree := s

	for {
		for i := subtree.childCount(); i > 0; i-- {
			child := &subtree.ptr.children[i-1]
			if child.totalBytes() > 0 {
				break
			}
			// Distinct inline leaves can have identical contents; compare their slots.
			if child == other {
				return true
			}
			if child.childCount() > 0 {
				stack = append(stack, *child)
			}
		}

		if len(stack) == 0 {
			break
		}
		subtree = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
	}

	return false
}

// hasTrailingEmptyDescendantBounded is
// ts_subtree_has_trailing_empty_descendant_bounded. The C function writes
// too_deep to an out parameter, and the Go function returns it.
func (s subtree) hasTrailingEmptyDescendantBounded(other *subtree, depth uint32) (found, tooDeep bool) {
	for i := s.childCount(); i > 0; i-- {
		child := &s.ptr.children[i-1]
		if child.totalBytes() > 0 {
			break
		}
		if child == other {
			return true, false
		}
		// A nonmatching leaf cannot contain the target.
		if child.childCount() == 0 {
			continue
		}
		if depth == 0 {
			return false, true
		}

		found, tooDeep := child.hasTrailingEmptyDescendantBounded(other, depth-1)
		if found {
			return true, false
		}
		if tooDeep {
			return false, true
		}
	}
	return false, false
}

// hasTrailingEmptyDescendant is ts_subtree_has_trailing_empty_descendant.
func (s subtree) hasTrailingEmptyDescendant(other *subtree) bool {
	found, tooDeep := s.hasTrailingEmptyDescendantBounded(other, tsNodeMaxRecursiveSearchDepth)
	if tooDeep {
		return s.hasTrailingEmptyDescendantIterative(other)
	}
	return found
}

// prevSibling is ts_node__prev_sibling.
func (n Node) prevSibling(includeAnonymous bool) Node {
	self := n
	selfSubtree := self.subtree()
	selfIsEmpty := selfSubtree.totalBytes() == 0
	targetEndByte := uint32(self.EndByte())

	node, _ := self.Parent()
	earlierNode := nullNode()
	earlierNodeIsRelevant := false

	for !node.isNull() {
		earlierChild := nullNode()
		earlierChildIsRelevant := false
		foundChildContainingTarget := false

		var child Node
		iterator := node.iterateChildren()
		for {
			next, ok := iterator.next()
			if !ok {
				break
			}
			child = next
			if child.id == self.id {
				break
			}
			if iterator.position.bytes > targetEndByte {
				foundChildContainingTarget = true
				break
			}

			if iterator.position.bytes == targetEndByte &&
				(!selfIsEmpty ||
					child.subtree().hasTrailingEmptyDescendant(self.id)) {
				foundChildContainingTarget = true
				break
			}

			if child.isRelevant(includeAnonymous) {
				earlierChild = child
				earlierChildIsRelevant = true
			} else if child.relevantChildCount(includeAnonymous) > 0 {
				earlierChild = child
				earlierChildIsRelevant = false
			}
		}

		switch {
		case foundChildContainingTarget:
			if !earlierChild.isNull() {
				earlierNode = earlierChild
				earlierNodeIsRelevant = earlierChildIsRelevant
			}
			node = child
		case earlierChildIsRelevant:
			return earlierChild
		case !earlierChild.isNull():
			node = earlierChild
		case earlierNodeIsRelevant:
			return earlierNode
		default:
			node = earlierNode
			earlierNode = nullNode()
			earlierNodeIsRelevant = false
		}
	}

	return nullNode()
}

// nextSibling is ts_node__next_sibling.
func (n Node) nextSibling(includeAnonymous bool) Node {
	self := n
	targetEndByte := uint32(self.EndByte())

	node, _ := self.Parent()
	laterNode := nullNode()
	laterNodeIsRelevant := false

	for !node.isNull() {
		laterChild := nullNode()
		laterChildIsRelevant := false
		childContainingTarget := nullNode()

		iterator := node.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			if iterator.position.bytes <= targetEndByte {
				continue
			}
			startByte := uint32(self.StartByte())
			childStartByte := uint32(child.StartByte())

			isEmpty := startByte == targetEndByte
			var containsTarget bool
			if isEmpty {
				containsTarget = childStartByte < startByte
			} else {
				containsTarget = childStartByte <= startByte
			}

			switch {
			case containsTarget:
				if child.subtree().ptr != self.subtree().ptr {
					childContainingTarget = child
				}
				continue
			case child.isRelevant(includeAnonymous):
				laterChild = child
				laterChildIsRelevant = true
			case child.relevantChildCount(includeAnonymous) > 0:
				laterChild = child
				laterChildIsRelevant = false
			default:
				continue
			}
			break
		}

		switch {
		case !childContainingTarget.isNull():
			if !laterChild.isNull() {
				laterNode = laterChild
				laterNodeIsRelevant = laterChildIsRelevant
			}
			node = childContainingTarget
		case laterChildIsRelevant:
			return laterChild
		case !laterChild.isNull():
			node = laterChild
		case laterNodeIsRelevant:
			return laterNode
		default:
			node = laterNode
		}
	}

	return nullNode()
}

// firstChildForByte is ts_node__first_child_for_byte. The goto of C is a
// loop.
func (n Node) firstChildForByte(goal uint32, includeAnonymous bool) Node {
	node := n
	didDescend := true

	var lastIterator nodeChildIterator
	hasLastIterator := false

	for didDescend {
		didDescend = false

		iterator := node.iterateChildren()
		for {
			for {
				child, ok := iterator.next()
				if !ok {
					break
				}
				if uint32(child.EndByte()) <= goal {
					continue
				}
				if child.isRelevant(includeAnonymous) {
					return child
				}
				if child.ChildCount() > 0 {
					if !iterator.done() {
						lastIterator = iterator
						hasLastIterator = true
					}
					didDescend = true
					node = child
					break
				}
			}

			if !didDescend && hasLastIterator {
				iterator = lastIterator
				hasLastIterator = false
				continue
			}
			break
		}
	}

	return nullNode()
}

// descendantForByteRange is ts_node__descendant_for_byte_range.
func (n Node) descendantForByteRange(rangeStart, rangeEnd uint32, includeAnonymous bool) Node {
	if rangeStart > rangeEnd {
		return nullNode()
	}
	node := n
	lastVisibleNode := n

	didDescend := true
	for didDescend {
		didDescend = false

		iterator := node.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			nodeEnd := iterator.position.bytes

			// The end of this node must extend far enough forward to touch
			// the end of the range
			if nodeEnd < rangeEnd {
				continue
			}

			// ...and exceed the start of the range, unless the node itself is
			// empty, in which case it must at least be equal to the start of the range.
			isEmpty := uint32(child.StartByte()) == nodeEnd
			if isEmpty && nodeEnd < rangeStart || !isEmpty && nodeEnd <= rangeStart {
				continue
			}

			// The start of this node must extend far enough backward to
			// touch the start of the range.
			if rangeStart < uint32(child.StartByte()) {
				break
			}

			node = child
			if node.isRelevant(includeAnonymous) {
				lastVisibleNode = node
			}
			didDescend = true
			break
		}
	}

	return lastVisibleNode
}

// descendantForPointRange is ts_node__descendant_for_point_range.
func (n Node) descendantForPointRange(rangeStart, rangeEnd point, includeAnonymous bool) Node {
	if rangeStart.gt(rangeEnd) {
		return nullNode()
	}
	node := n
	lastVisibleNode := n

	didDescend := true
	for didDescend {
		didDescend = false

		iterator := node.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			nodeEnd := iterator.position.extent

			// The end of this node must extend far enough forward to touch
			// the end of the range
			if nodeEnd.lt(rangeEnd) {
				continue
			}

			// ...and exceed the start of the range, unless the node itself is
			// empty, in which case it must at least be equal to the start of the range.
			isEmpty := child.StartPoint().internal().eq(nodeEnd)
			if isEmpty && nodeEnd.lt(rangeStart) || !isEmpty && nodeEnd.lte(rangeStart) {
				continue
			}

			// The start of this node must extend far enough backward to
			// touch the start of the range.
			if rangeStart.lt(child.StartPoint().internal()) {
				break
			}

			node = child
			if node.isRelevant(includeAnonymous) {
				lastVisibleNode = node
			}
			didDescend = true
			break
		}
	}

	return lastVisibleNode
}

// TSNode - public

// EndByte returns the byte offset where the node ends.
//
// EndByte is ts_node_end_byte.
func (n Node) EndByte() int {
	return int(uint32(n.StartByte()) + n.subtree().size().bytes)
}

// EndPoint returns the point where the node ends.
//
// EndPoint is ts_node_end_point.
func (n Node) EndPoint() Point {
	return n.StartPoint().internal().add(n.subtree().size().extent).public()
}

// KindID returns the symbol of the node, or of its alias.
//
// KindID is ts_node_symbol.
func (n Node) KindID() Symbol {
	symbol := n.alias()
	if symbol == 0 {
		symbol = n.subtree().symbol()
	}
	return n.tree.language.publicSymbol(symbol)
}

// Kind returns the name of the symbol of the node, or of its alias.
//
// Kind is ts_node_type.
func (n Node) Kind() string {
	symbol := n.alias()
	if symbol == 0 {
		symbol = n.subtree().symbol()
	}
	return n.tree.language.SymbolName(symbol)
}

// Language returns the language of the tree of the node.
//
// Language is ts_node_language.
func (n Node) Language() *Language {
	return n.tree.Language()
}

// GrammarID returns the symbol of the node in the grammar, before any alias.
//
// GrammarID is ts_node_grammar_symbol.
func (n Node) GrammarID() Symbol {
	return n.subtree().symbol()
}

// GrammarKind returns the name of the symbol of the node in the grammar,
// before any alias.
//
// GrammarKind is ts_node_grammar_type.
func (n Node) GrammarKind() string {
	symbol := n.subtree().symbol()
	return n.tree.language.SymbolName(symbol)
}

// String returns the node and its named descendants as an S-expression.
//
// String is ts_node_string.
func (n Node) String() string {
	aliasSymbol := n.alias()
	return n.subtree().string(
		aliasSymbol,
		n.tree.language.symbolMetadata(aliasSymbol).Visible,
		n.tree.language,
		false,
	)
}

// Equal reports whether n and other are the same node of the same tree. It
// does not compare the positions of the nodes, so a node that Edit moved is
// Equal to the node before the edit, as in C, and == tells them apart.
//
// Equal is ts_node_eq.
func (n Node) Equal(other Node) bool {
	return n.tree == other.tree && n.id == other.id
}

// isNull is ts_node_is_null.
func (n Node) isNull() bool {
	return n.id == nil
}

// IsExtra reports whether the node is an extra, such as a comment.
//
// IsExtra is ts_node_is_extra.
func (n Node) IsExtra() bool {
	return n.subtree().extra()
}

// IsNamed reports whether the node is named, or has a named alias.
//
// IsNamed is ts_node_is_named.
func (n Node) IsNamed() bool {
	alias := n.alias()
	if alias != 0 {
		return n.tree.language.symbolMetadata(alias).Named
	}
	return n.subtree().named()
}

// IsMissing reports whether the parser inserted the node to recover from an
// error.
//
// IsMissing is ts_node_is_missing.
func (n Node) IsMissing() bool {
	return n.subtree().missing()
}

// HasChanges reports whether an edit changed the node.
//
// HasChanges is ts_node_has_changes.
func (n Node) HasChanges() bool {
	return n.subtree().hasChanges()
}

// HasError reports whether the node is an error or holds one.
//
// HasError is ts_node_has_error.
func (n Node) HasError() bool {
	return n.IsError() ||
		n.subtree().errorCost() > 0
}

// IsError reports whether the node is an ERROR node.
//
// IsError is ts_node_is_error.
func (n Node) IsError() bool {
	symbol := n.KindID()
	return symbol == builtinSymError
}

// DescendantCount returns the number of the node and its descendants.
//
// DescendantCount is ts_node_descendant_count.
func (n Node) DescendantCount() int {
	return int(n.subtree().visibleDescendantCount() + 1)
}

// ParseState returns the parse state of the node.
//
// ParseState is ts_node_parse_state.
func (n Node) ParseState() StateID {
	return n.subtree().parseState()
}

// NextParseState returns the parse state after the node.
//
// NextParseState is ts_node_next_parse_state.
func (n Node) NextParseState() StateID {
	language := n.tree.language
	state := n.ParseState()
	if state == tsTreeStateNone {
		return tsTreeStateNone
	}
	symbol := n.GrammarID()
	return language.NextState(state, symbol)
}

// Parent returns the parent of the node. It returns false for the root.
//
// Parent is ts_node_parent.
func (n Node) Parent() (Node, bool) {
	self := n
	node := self.tree.RootNode()
	if node.id == self.id {
		return nullNode(), false
	}

	for {
		nextNode, _ := node.ChildWithDescendant(self)
		if nextNode.id == self.id || nextNode.isNull() {
			break
		}
		node = nextNode
	}

	return node, !node.isNull()
}

// descendantSearchFrame is DescendantSearchFrame of
// ts_node_child_with_empty_descendant_iterative.
type descendantSearchFrame struct {
	child    Node
	iterator nodeChildIterator
}

// childWithEmptyDescendantIterative is
// ts_node_child_with_empty_descendant_iterative. The labels of the C
// function become the loop and its two ends: scan is the next turn of the
// loop, resume is the code after the if and the else, found is the code
// after the loop, and no_descendant is the else.
func (n Node) childWithEmptyDescendantIterative(descendant Node) Node {
	targetByte := uint32(descendant.StartByte())

	var stack []descendantSearchFrame
	iterator := n.iterateChildren()
	result := nullNode()

	for {
		child, ok := iterator.next()
		if ok && uint32(child.StartByte()) <= targetByte {
			if child.id == descendant.id {
				result = child
				break
			}

			if iterator.position.bytes >= targetByte && child.ChildCount() > 0 {
				stack = append(stack, descendantSearchFrame{child, iterator})
				iterator = child.iterateChildren()
				continue
			}
		} else {
			if len(stack) == 0 {
				return result
			}
			frame := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			child = frame.child
			iterator = frame.iterator
		}

		if iterator.position.bytes <= targetByte ||
			child.ChildCount() == 0 {
			continue
		}

		if child.isRelevant(true) {
			result = child
			break
		}
		iterator = child.iterateChildren()
	}

	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.child.isRelevant(true) {
			result = frame.child
		}
	}

	return result
}

// childWithEmptyDescendantBounded is
// ts_node_child_with_empty_descendant_bounded. The C function writes
// too_deep to an out parameter, and the Go function returns it.
func (n Node) childWithEmptyDescendantBounded(descendant Node, depth uint32) (result Node, tooDeep bool) {
	self := n
	targetByte := uint32(descendant.StartByte())

	for {
		iterator := self.iterateChildren()
		for {
			next, ok := iterator.next()
			if !ok || uint32(next.StartByte()) > targetByte {
				return nullNode(), false
			}
			self = next
			if self.id == descendant.id {
				return self, false
			}

			if iterator.position.bytes >= targetByte && self.ChildCount() > 0 {
				if depth == 0 {
					return nullNode(), true
				}

				child, tooDeep := self.childWithEmptyDescendantBounded(
					descendant,
					depth-1,
				)
				if tooDeep {
					return nullNode(), true
				}
				if !child.isNull() {
					if self.isRelevant(true) {
						return self, false
					}
					return child, false
				}
			}
			if iterator.position.bytes > targetByte && self.ChildCount() != 0 {
				break
			}
		}
		if self.isRelevant(true) {
			break
		}
	}

	return self, false
}

// ChildWithDescendant returns the child of the node that holds descendant.
// The result can be descendant itself.
//
// ChildWithDescendant is ts_node_child_with_descendant.
func (n Node) ChildWithDescendant(descendant Node) (Node, bool) {
	self := n
	startByte := uint32(descendant.StartByte())
	endByte := uint32(descendant.EndByte())
	if startByte == endByte {
		child, tooDeep := self.childWithEmptyDescendantBounded(
			descendant,
			tsNodeMaxRecursiveSearchDepth,
		)
		if tooDeep {
			child = self.childWithEmptyDescendantIterative(descendant)
		}
		return child, !child.isNull()
	}

	for {
		iterator := self.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok || uint32(child.StartByte()) > startByte {
				return nullNode(), false
			}
			self = child
			if self.id == descendant.id {
				return self, true
			}
			if iterator.position.bytes >= endByte && self.ChildCount() != 0 {
				break
			}
		}
		if self.isRelevant(true) {
			break
		}
	}

	return self, true
}

// Child returns the child of the node at an index, where the index counts
// the named and the anonymous children.
//
// Child is ts_node_child.
func (n Node) Child(i int) (Node, bool) {
	result := n.child(uint32(i), true)
	return result, !result.isNull()
}

// NamedChild returns the named child of the node at an index, where the
// index counts only the named children.
//
// NamedChild is ts_node_named_child.
func (n Node) NamedChild(i int) (Node, bool) {
	result := n.child(uint32(i), false)
	return result, !result.isNull()
}

// firstChildWithInheritedField follows the first matching child of a hidden
// node without saving a traversal stack. A different field blocks this field
// unless the child can contain it explicitly. It returns false if resolving
// that case, a hidden extra, or backtracking needs a cursor. When it returns
// true, the node is the null node if no child has the field.
//
// firstChildWithInheritedField is ts_node__first_child_with_inherited_field.
// The C function writes the node to an out parameter, and the Go function
// returns it.
func (n Node) firstChildWithInheritedField(fieldID FieldID) (Node, bool) {
	self := n
	needsBacktracking := false
	for {
		iterator := self.iterateChildren()
		fieldMap := self.tree.language.fieldMap(uint32(self.subtree().ptr.productionID))
		didDescend := false
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			visible := child.isRelevant(true)
			if !visible && child.ChildCount() == 0 {
				continue
			}
			// An extra has no field of its own, but a hidden one can hold fields.
			if child.subtree().extra() {
				if visible {
					continue
				}
				return Node{}, false
			}

			var ownFieldID FieldID
			for _, m := range fieldMap {
				if !m.Inherited && uint32(m.ChildIndex) == iterator.structuralChildIndex-1 {
					ownFieldID = FieldID(m.FieldID)
					break
				}
			}

			if ownFieldID != 0 && ownFieldID != fieldID {
				if visible {
					continue
				}
				childFieldMap := self.tree.language.fieldMap(uint32(child.subtree().ptr.productionID))
				for _, m := range childFieldMap {
					if FieldID(m.FieldID) == fieldID {
						return Node{}, false
					}
				}
				continue
			}
			if visible {
				return child, true
			}
			needsBacktracking = needsBacktracking || !iterator.done()
			self = child
			didDescend = true
			break
		}
		if !didDescend {
			return nullNode(), !needsBacktracking
		}
	}
}

// ChildByFieldID returns the first child of the node with a field.
//
// ChildByFieldID is ts_node_child_by_field_id. The labels of C become
// loops: descend is the outer loop, scan is the inner loop, and not_found
// is the code after the loop over the children. notFound is true where C
// jumps from descend to not_found.
func (n Node) ChildByFieldID(fieldID FieldID) (Node, bool) {
	type fieldSearchFrame struct {
		node     Node
		iterator nodeChildIterator
		fieldMap []abi.FieldMapEntry
	}

	if fieldID == 0 {
		return nullNode(), false
	}

	self := n
	var stack []fieldSearchFrame
	var iterator nodeChildIterator
	var fieldMap []abi.FieldMapEntry

descend:
	for {
		notFound := self.ChildCount() == 0

		if !notFound {
			fieldMap = self.tree.language.fieldMap(uint32(self.subtree().ptr.productionID))
			notFound = len(fieldMap) == 0
		}

		// The field mappings are sorted by their field id. Scan all
		// the mappings to find the ones for the given field id.
		for !notFound && FieldID(fieldMap[0].FieldID) < fieldID {
			fieldMap = fieldMap[1:]
			notFound = len(fieldMap) == 0
		}
		for !notFound && FieldID(fieldMap[len(fieldMap)-1].FieldID) > fieldID {
			fieldMap = fieldMap[:len(fieldMap)-1]
			notFound = len(fieldMap) == 0
		}

		if !notFound {
			iterator = self.iterateChildren()
		}

		for {
		scan:
			for !notFound {
				child, ok := iterator.next()
				if !ok {
					break
				}
				if !child.subtree().extra() {
					index := iterator.structuralChildIndex - 1
					if index < uint32(fieldMap[0].ChildIndex) {
						continue
					}

					// Hidden nodes' fields are "inherited" by their visible parent.
					switch {
					case fieldMap[0].Inherited:
						// An alias can make this child visible, keeping its fields inside it.
						if child.isRelevant(true) {
							fieldMap = fieldMap[1:]
							if len(fieldMap) == 0 {
								break scan
							}
							continue
						}

						// If this is the *last* possible child node for this field,
						// then perform a tail call to avoid recursion.
						if len(fieldMap) == 1 {
							self = child
							continue descend
						}

						// Otherwise, descend into this child, but if it doesn't contain
						// the field, continue searching subsequent children.
						stack = append(stack, fieldSearchFrame{
							node:     self,
							iterator: iterator,
							fieldMap: fieldMap,
						})
						self = child
						continue descend

					case child.isRelevant(true):
						return child, true

					// A field on a hidden child can be overridden by fields inside it.
					// Use the cursor's field resolution to find the first matching child.
					case child.ChildCount() > 0:
						if first, ok := child.firstChildWithInheritedField(fieldID); ok {
							if !first.isNull() {
								return first, true
							}
							for {
								fieldMap = fieldMap[1:]
								if len(fieldMap) == 0 || uint32(fieldMap[0].ChildIndex) != index {
									break
								}
							}
							if len(fieldMap) == 0 {
								break scan
							}
							continue
						}
						cursor := self.Walk()
						result := nullNode()
						if cursor.GotoFirstChild() {
							for {
								if cursor.FieldID() == fieldID {
									result = cursor.Node()
									break
								}
								if !cursor.GotoNextSibling() {
									break
								}
							}
						}
						if !result.isNull() {
							return result, true
						}
						break scan

					// Otherwise, continue searching subsequent children.
					default:
						fieldMap = fieldMap[1:]
						if len(fieldMap) == 0 {
							break scan
						}
					}
				}
			}

			// not_found
			for {
				if len(stack) == 0 {
					return nullNode(), false
				}
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				self = frame.node
				iterator = frame.iterator
				fieldMap = frame.fieldMap[1:]
				if len(fieldMap) != 0 {
					break
				}
			}
			notFound = false
		}
	}
}

// fieldNameFromLanguage is ts_node__field_name_from_language. It returns ""
// where the C function returns NULL.
func (n Node) fieldNameFromLanguage(structuralChildIndex uint32) string {
	fieldMap := n.tree.language.fieldMap(uint32(n.subtree().ptr.productionID))
	for _, m := range fieldMap {
		if !m.Inherited && uint32(m.ChildIndex) == structuralChildIndex {
			return n.tree.language.tables.FieldNames[m.FieldID]
		}
	}
	return ""
}

// FieldNameForChild returns the field name of the child of the node at an
// index, or "" when the child has no field.
//
// FieldNameForChild is ts_node_field_name_for_child.
func (n Node) FieldNameForChild(i int) string {
	result := n
	childIndex := uint32(i)
	didDescend := true
	inheritedFieldName := ""

	for didDescend {
		didDescend = false

		index := uint32(0)
		iterator := result.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			if child.isRelevant(true) {
				if index == childIndex {
					if child.IsExtra() {
						return ""
					}
					fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
					if fieldName != "" {
						return fieldName
					}
					return inheritedFieldName
				}
				index++
			} else {
				grandchildIndex := childIndex - index
				grandchildCount := child.relevantChildCount(true)
				if grandchildIndex < grandchildCount {
					// An extra takes no structural index, and the cursor gives the nodes
					// inside one no field from outside it.
					if child.IsExtra() {
						inheritedFieldName = ""
					} else {
						fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
						if fieldName != "" {
							inheritedFieldName = fieldName
						}
					}

					didDescend = true
					result = child
					childIndex = grandchildIndex
					break
				}
				index += grandchildCount
			}
		}
	}

	return ""
}

// FieldNameForNamedChild returns the field name of the named child of the
// node at an index, or "" when the child has no field.
//
// FieldNameForNamedChild is ts_node_field_name_for_named_child.
func (n Node) FieldNameForNamedChild(i int) string {
	result := n
	namedChildIndex := uint32(i)
	didDescend := true
	inheritedFieldName := ""

	for didDescend {
		didDescend = false

		index := uint32(0)
		iterator := result.iterateChildren()
		for {
			child, ok := iterator.next()
			if !ok {
				break
			}
			if child.isRelevant(false) {
				if index == namedChildIndex {
					if child.IsExtra() {
						return ""
					}
					fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
					if fieldName != "" {
						return fieldName
					}
					return inheritedFieldName
				}
				index++
			} else {
				namedGrandchildIndex := namedChildIndex - index
				grandchildCount := child.relevantChildCount(false)
				if namedGrandchildIndex < grandchildCount {
					// An extra takes no structural index, and the cursor gives the nodes
					// inside one no field from outside it.
					if child.IsExtra() {
						inheritedFieldName = ""
					} else {
						fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
						if fieldName != "" {
							inheritedFieldName = fieldName
						}
					}

					didDescend = true
					result = child
					namedChildIndex = namedGrandchildIndex
					break
				}
				index += grandchildCount
			}
		}
	}

	return ""
}

// ChildByFieldName returns the first child of the node with a field.
//
// ChildByFieldName is ts_node_child_by_field_name. The C function looks up
// the field with ts_language_field_id_for_name, which returns 0 for a name
// that the language does not have. The field 0 finds no child.
func (n Node) ChildByFieldName(name string) (Node, bool) {
	fieldID, _ := n.tree.language.FieldForName(name)
	return n.ChildByFieldID(fieldID)
}

// ChildCount returns the number of the named and the anonymous children of
// the node.
//
// ChildCount is ts_node_child_count.
func (n Node) ChildCount() int {
	tree := n.subtree()
	if tree.childCount() > 0 {
		return int(tree.ptr.visibleChildCount)
	}
	return 0
}

// NamedChildCount returns the number of the named children of the node.
//
// NamedChildCount is ts_node_named_child_count.
func (n Node) NamedChildCount() int {
	tree := n.subtree()
	if tree.childCount() > 0 {
		return int(tree.ptr.namedChildCount)
	}
	return 0
}

// NextSibling returns the next sibling of the node.
//
// NextSibling is ts_node_next_sibling.
func (n Node) NextSibling() (Node, bool) {
	result := n.nextSibling(true)
	return result, !result.isNull()
}

// NextNamedSibling returns the next named sibling of the node.
//
// NextNamedSibling is ts_node_next_named_sibling.
func (n Node) NextNamedSibling() (Node, bool) {
	result := n.nextSibling(false)
	return result, !result.isNull()
}

// PrevSibling returns the previous sibling of the node.
//
// PrevSibling is ts_node_prev_sibling.
func (n Node) PrevSibling() (Node, bool) {
	result := n.prevSibling(true)
	return result, !result.isNull()
}

// PrevNamedSibling returns the previous named sibling of the node.
//
// PrevNamedSibling is ts_node_prev_named_sibling.
func (n Node) PrevNamedSibling() (Node, bool) {
	result := n.prevSibling(false)
	return result, !result.isNull()
}

// FirstChildForByte returns the first child of the node that ends after a
// byte offset.
//
// FirstChildForByte is ts_node_first_child_for_byte.
func (n Node) FirstChildForByte(offset int) (Node, bool) {
	result := n.firstChildForByte(uint32(offset), true)
	return result, !result.isNull()
}

// FirstNamedChildForByte returns the first named child of the node that
// ends after a byte offset.
//
// FirstNamedChildForByte is ts_node_first_named_child_for_byte.
func (n Node) FirstNamedChildForByte(offset int) (Node, bool) {
	result := n.firstChildForByte(uint32(offset), false)
	return result, !result.isNull()
}

// DescendantForByteRange returns the smallest node in the node that spans a
// range of bytes. It returns false when start is after end.
//
// DescendantForByteRange is ts_node_descendant_for_byte_range.
func (n Node) DescendantForByteRange(start, end int) (Node, bool) {
	result := n.descendantForByteRange(uint32(start), uint32(end), true)
	return result, !result.isNull()
}

// NamedDescendantForByteRange returns the smallest named node in the node
// that spans a range of bytes. It returns false when start is after end.
//
// NamedDescendantForByteRange is ts_node_named_descendant_for_byte_range.
func (n Node) NamedDescendantForByteRange(start, end int) (Node, bool) {
	result := n.descendantForByteRange(uint32(start), uint32(end), false)
	return result, !result.isNull()
}

// DescendantForPointRange returns the smallest node in the node that spans a
// range of points. It returns false when start is after end.
//
// DescendantForPointRange is ts_node_descendant_for_point_range.
func (n Node) DescendantForPointRange(start, end Point) (Node, bool) {
	result := n.descendantForPointRange(start.internal(), end.internal(), true)
	return result, !result.isNull()
}

// NamedDescendantForPointRange returns the smallest named node in the node
// that spans a range of points. It returns false when start is after end.
//
// NamedDescendantForPointRange is ts_node_named_descendant_for_point_range.
func (n Node) NamedDescendantForPointRange(start, end Point) (Node, bool) {
	result := n.descendantForPointRange(start.internal(), end.internal(), false)
	return result, !result.isNull()
}

// Edit moves the start of the node to match an edit of the text. A node
// that comes from a tree after Tree.Edit already matches the edit, so Edit
// is only for a node that the caller kept from before the edit.
//
// Edit is ts_node_edit.
func (n *Node) Edit(e InputEdit) {
	startByte := uint32(n.StartByte())
	startPoint := n.StartPoint().internal()

	startPoint, startByte = pointEdit(startPoint, startByte, e)

	n.context[0] = startByte
	n.context[1] = startPoint.row
	n.context[2] = startPoint.column
}

// Range returns the range of the node, in bytes and in points.
//
// Range is range of Node in the Rust binding.
func (n Node) Range() Range {
	return Range{
		StartByte:  n.StartByte(),
		EndByte:    n.EndByte(),
		StartPoint: n.StartPoint(),
		EndPoint:   n.EndPoint(),
	}
}

// Text returns the text of the node in src, the text that the parse read.
//
// Text is utf8_text of Node in the Rust binding. It does not check that
// the text is valid UTF-8.
func (n Node) Text(src []byte) string {
	return string(src[n.StartByte():n.EndByte()])
}

// Children returns the named and the anonymous children of the node. It
// walks them with a tree cursor.
//
// Children is children of Node in the Rust binding.
func (n Node) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		cursor := n.Walk()
		cursor.GotoFirstChild()
		for range n.ChildCount() {
			result := cursor.Node()
			cursor.GotoNextSibling()
			if !yield(result) {
				return
			}
		}
	}
}

// NamedChildren returns the named children of the node. It walks them with a
// tree cursor.
//
// NamedChildren is named_children of Node in the Rust binding.
func (n Node) NamedChildren() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		cursor := n.Walk()
		cursor.GotoFirstChild()
		for range n.NamedChildCount() {
			for !cursor.Node().IsNamed() {
				if !cursor.GotoNextSibling() {
					break
				}
			}
			result := cursor.Node()
			cursor.GotoNextSibling()
			if !yield(result) {
				return
			}
		}
	}
}

// ChildrenByFieldName returns the children of the node with a field. It
// walks them with a tree cursor.
//
// ChildrenByFieldName is children_by_field_name of Node in the Rust
// binding.
func (n Node) ChildrenByFieldName(name string) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		fieldID, ok := n.Language().FieldForName(name)
		if !ok {
			return
		}
		cursor := n.Walk()
		cursor.GotoFirstChild()
		for {
			for cursor.FieldID() != fieldID {
				if !cursor.GotoNextSibling() {
					return
				}
			}
			result := cursor.Node()
			done := !cursor.GotoNextSibling()
			if !yield(result) || done {
				return
			}
		}
	}
}
