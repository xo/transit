package transit

import "iter"

// This file ports lib/src/node.c, and the type TSNode of
// lib/include/tree_sitter/api.h.
//
// A method whose C function can return a null node returns the node and a
// bool, which is false for the null node (D56). The zero Node is the null
// node. These parts have no exported form. ts_node_eq is ==, because a Node
// holds what TSNode holds. ts_node_is_null is the method isNull, because a
// lookup returns a bool with the node.
//
// The exported API counts with int (D25). A function of this file that
// needs the width of C converts the int back to a uint32.
//
// The methods at the end of the file come from the Rust binding, and they
// port no C function.

// Node is a node of a tree. It is a small value, and two nodes compare with
// ==. The zero Node is no node.
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

// hasTrailingEmptyDescendant is ts_subtree_has_trailing_empty_descendant.
func (s subtree) hasTrailingEmptyDescendant(other subtree) bool {
	for i := s.childCount() - 1; i+1 > 0; i-- {
		child := s.ptr.children[i]
		if child.totalBytes() > 0 {
			break
		}
		if child.ptr == other.ptr || child.hasTrailingEmptyDescendant(other) {
			return true
		}
	}
	return false
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
					child.subtree().hasTrailingEmptyDescendant(selfSubtree)) {
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
//
// The C function compares the index of the iterator in the parent with the
// child count of the child, and not with the child count of the parent. The
// Go function does the same.
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
					if iterator.childIndex < child.subtree().childCount() {
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
	return n.subtree().errorCost() > 0
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

// ChildWithDescendant returns the child of the node that holds descendant.
// The result can be descendant itself.
//
// ChildWithDescendant is ts_node_child_with_descendant.
func (n Node) ChildWithDescendant(descendant Node) (Node, bool) {
	self := n
	startByte := uint32(descendant.StartByte())
	endByte := uint32(descendant.EndByte())
	isEmpty := startByte == endByte

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

			// If the descendant is empty, and the end byte is within `self`,
			// we check whether `self` contains it or not.
			if isEmpty && iterator.position.bytes >= endByte && self.ChildCount() > 0 {
				child, ok := self.ChildWithDescendant(descendant)
				// If the child is not null, return self if it's relevant, else return the child
				if ok {
					if self.isRelevant(true) {
						return self, true
					}
					return child, true
				}
			}

			var inRange bool
			if isEmpty {
				inRange = iterator.position.bytes <= endByte
			} else {
				inRange = iterator.position.bytes < endByte
			}
			if !inRange && self.ChildCount() != 0 {
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

// ChildByFieldID returns the first child of the node with a field.
//
// ChildByFieldID is ts_node_child_by_field_id. The goto of C is a labeled
// continue.
func (n Node) ChildByFieldID(fieldID FieldID) (Node, bool) {
	self := n
recur:
	for {
		if fieldID == 0 || self.ChildCount() == 0 {
			return nullNode(), false
		}

		fieldMap := self.tree.language.fieldMap(uint32(self.subtree().ptr.productionID))
		if len(fieldMap) == 0 {
			return nullNode(), false
		}

		// The field mappings are sorted by their field id. Scan all
		// the mappings to find the ones for the given field id.
		for FieldID(fieldMap[0].FieldID) < fieldID {
			fieldMap = fieldMap[1:]
			if len(fieldMap) == 0 {
				return nullNode(), false
			}
		}
		for FieldID(fieldMap[len(fieldMap)-1].FieldID) > fieldID {
			fieldMap = fieldMap[:len(fieldMap)-1]
			if len(fieldMap) == 0 {
				return nullNode(), false
			}
		}

		iterator := self.iterateChildren()
		for {
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
					// If this is the *last* possible child node for this field,
					// then perform a tail call to avoid recursion.
					if len(fieldMap) == 1 {
						self = child
						continue recur
					}

					// Otherwise, descend into this child, but if it doesn't contain
					// the field, continue searching subsequent children.
					result, ok := child.ChildByFieldID(fieldID)
					if ok {
						return result, true
					}
					fieldMap = fieldMap[1:]
					if len(fieldMap) == 0 {
						return nullNode(), false
					}

				case child.isRelevant(true):
					return child, true

				// If the field refers to a hidden node with visible children,
				// return the first visible child.
				case child.ChildCount() > 0:
					return child.Child(0)

				// Otherwise, continue searching subsequent children.
				default:
					fieldMap = fieldMap[1:]
					if len(fieldMap) == 0 {
						return nullNode(), false
					}
				}
			}
		}

		return nullNode(), false
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
					fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
					if fieldName != "" {
						inheritedFieldName = fieldName
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
					fieldName := result.fieldNameFromLanguage(iterator.structuralChildIndex - 1)
					if fieldName != "" {
						inheritedFieldName = fieldName
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
