package transit

import "slices"

// This file ports lib/src/tree_cursor.c and lib/src/tree_cursor.h.
//
// C has two types: TSTreeCursor of the API, and TreeCursor inside the
// runtime, which fits in TSTreeCursor, and the functions cast one to the
// other. Go has the one type TreeCursor. ts_tree_cursor_delete has no Go
// form, because the garbage collector frees a cursor (D24).

// treeCursorEntry is TreeCursorEntry. subtree is the address of the subtree
// in the children of its parent, or the root of the cursor.
type treeCursorEntry struct {
	subtree              *subtree
	position             length
	childIndex           uint32
	structuralChildIndex uint32
	descendantIndex      uint32
}

// TreeCursor walks a tree. It belongs to one goroutine at a time.
//
// TreeCursor is TSTreeCursor and TreeCursor.
type TreeCursor struct {
	tree            *Tree
	stack           []treeCursorEntry
	rootAliasSymbol Symbol
}

// treeCursorStep is TreeCursorStep.
type treeCursorStep int

// The steps of a move of a cursor.
const (
	// treeCursorStepNone is TreeCursorStepNone.
	treeCursorStepNone treeCursorStep = iota
	// treeCursorStepHidden is TreeCursorStepHidden.
	treeCursorStepHidden
	// treeCursorStepVisible is TreeCursorStepVisible.
	treeCursorStepVisible
)

// String returns the name of the step.
func (s treeCursorStep) String() string {
	switch s {
	case treeCursorStepNone:
		return noneName
	case treeCursorStepHidden:
		return "hidden"
	case treeCursorStepVisible:
		return "visible"
	}
	return unknownName
}

// currentSubtree is ts_tree_cursor_current_subtree of tree_cursor.h.
func (c *TreeCursor) currentSubtree() subtree {
	lastEntry := &c.stack[len(c.stack)-1]
	return *lastEntry.subtree
}

// cursorChildIterator is CursorChildIterator.
type cursorChildIterator struct {
	parent               subtree
	tree                 *Tree
	position             length
	childIndex           uint32
	structuralChildIndex uint32
	descendantIndex      uint32
	aliasSequence        []uint16
}

// CursorChildIterator

// isEntryVisible is ts_tree_cursor_is_entry_visible.
func (c *TreeCursor) isEntryVisible(index uint32) bool {
	entry := &c.stack[index]
	if index == 0 || entry.subtree.visible() {
		return true
	} else if !entry.subtree.extra() {
		parentEntry := &c.stack[index-1]
		return c.tree.language.aliasAt(
			uint32(parentEntry.subtree.ptr.productionID),
			entry.structuralChildIndex,
		) != 0
	}
	return false
}

// iterateChildren is ts_tree_cursor_iterate_children.
func (c *TreeCursor) iterateChildren() cursorChildIterator {
	lastEntry := &c.stack[len(c.stack)-1]
	if lastEntry.subtree.childCount() == 0 {
		return cursorChildIterator{tree: c.tree, position: lengthZero()}
	}
	aliasSequence := c.tree.language.aliasSequence(
		uint32(lastEntry.subtree.ptr.productionID),
	)

	descendantIndex := lastEntry.descendantIndex
	if c.isEntryVisible(uint32(len(c.stack) - 1)) {
		descendantIndex++
	}

	return cursorChildIterator{
		tree:                 c.tree,
		parent:               *lastEntry.subtree,
		position:             lastEntry.position,
		childIndex:           0,
		structuralChildIndex: 0,
		descendantIndex:      descendantIndex,
		aliasSequence:        aliasSequence,
	}
}

// next is ts_tree_cursor_child_iterator_next. The C function writes the
// entry and whether it is visible to out parameters, and the Go function
// returns them.
func (it *cursorChildIterator) next() (result treeCursorEntry, visible, ok bool) {
	if it.parent.ptr == nil || it.childIndex == it.parent.childCount() {
		return treeCursorEntry{}, false, false
	}
	child := &it.parent.ptr.children[it.childIndex]
	result = treeCursorEntry{
		subtree:              child,
		position:             it.position,
		childIndex:           it.childIndex,
		structuralChildIndex: it.structuralChildIndex,
		descendantIndex:      it.descendantIndex,
	}
	visible = child.visible()
	extra := child.extra()
	if !extra {
		if it.aliasSequence != nil {
			visible = visible || it.aliasSequence[it.structuralChildIndex] != 0
		}
		it.structuralChildIndex++
	}

	it.descendantIndex += child.visibleDescendantCount()
	if visible {
		it.descendantIndex++
	}

	it.position = it.position.add(child.size())
	it.childIndex++

	if it.childIndex < it.parent.childCount() {
		nextChild := it.parent.ptr.children[it.childIndex]
		it.position = it.position.add(nextChild.padding())
	}

	return result, visible, true
}

// lengthBacktrack is length_backtrack.
//
// Return a position that, when `b` is added to it, yields `a`. This
// can only be computed if `b` has zero rows. Otherwise, this function
// returns `LENGTH_UNDEFINED`, and the caller needs to recompute
// the position some other way.
func lengthBacktrack(a, b length) length {
	if a.isUndefined() || b.extent.row != 0 {
		return lengthUndefined
	}

	var result length
	result.bytes = a.bytes - b.bytes
	result.extent.row = a.extent.row
	result.extent.column = a.extent.column - b.extent.column
	return result
}

// previous is ts_tree_cursor_child_iterator_previous. The C function writes
// the entry and whether it is visible to out parameters, and the Go function
// returns them.
//
// The C function stops when the child index, cut to 8 bits, is -1. So it
// also stops at the child index 255, and the Go function does the same. The
// entry of C has no descendant index, so it is 0.
func (it *cursorChildIterator) previous() (result treeCursorEntry, visible, ok bool) {
	// this is mostly a reverse `ts_tree_cursor_child_iterator_next` taking into
	// account unsigned underflow
	if it.parent.ptr == nil || int8(it.childIndex) == -1 {
		return treeCursorEntry{}, false, false
	}
	child := &it.parent.ptr.children[it.childIndex]
	result = treeCursorEntry{
		subtree:              child,
		position:             it.position,
		childIndex:           it.childIndex,
		structuralChildIndex: it.structuralChildIndex,
	}
	visible = child.visible()
	extra := child.extra()

	it.position = lengthBacktrack(it.position, child.padding())
	it.childIndex--

	if !extra && it.aliasSequence != nil {
		visible = visible || it.aliasSequence[it.structuralChildIndex] != 0
		if it.structuralChildIndex > 0 {
			it.structuralChildIndex--
		}
	}

	// unsigned can underflow so compare it to child_count
	if it.childIndex < it.parent.childCount() {
		previousChild := it.parent.ptr.children[it.childIndex]
		size := previousChild.size()
		it.position = lengthBacktrack(it.position, size)
	}

	return result, visible, true
}

// TSTreeCursor - lifecycle

// Walk returns a cursor at the node. The node is the root of the cursor, and
// the cursor does not walk out of it.
//
// Walk is ts_tree_cursor_new.
func (n Node) Walk() *TreeCursor {
	var cursor TreeCursor
	cursor.init(n)
	return &cursor
}

// Reset moves the cursor to a node, which becomes the root of the cursor.
//
// Reset is ts_tree_cursor_reset.
func (c *TreeCursor) Reset(n Node) {
	c.init(n)
}

// init is ts_tree_cursor_init.
func (c *TreeCursor) init(node Node) {
	c.tree = node.tree
	c.rootAliasSymbol = Symbol(node.context[3])
	c.stack = append(c.stack[:0], treeCursorEntry{
		subtree: node.id,
		position: length{
			uint32(node.StartByte()),
			node.StartPoint().internal(),
		},
		childIndex:           0,
		structuralChildIndex: 0,
		descendantIndex:      0,
	})
}

// TSTreeCursor - walking the tree

// gotoFirstChildInternal is ts_tree_cursor_goto_first_child_internal.
func (c *TreeCursor) gotoFirstChildInternal() treeCursorStep {
	iterator := c.iterateChildren()
	for {
		entry, visible, ok := iterator.next()
		if !ok {
			break
		}
		if visible {
			c.stack = append(c.stack, entry)
			return treeCursorStepVisible
		}
		if entry.subtree.visibleChildCount() > 0 {
			c.stack = append(c.stack, entry)
			return treeCursorStepHidden
		}
	}
	return treeCursorStepNone
}

// GotoFirstChild moves the cursor to the first child of its node. It
// returns false, and does not move, when the node has no children.
//
// GotoFirstChild is ts_tree_cursor_goto_first_child.
func (c *TreeCursor) GotoFirstChild() bool {
	for {
		switch c.gotoFirstChildInternal() {
		case treeCursorStepHidden:
			continue
		case treeCursorStepVisible:
			return true
		default:
			return false
		}
	}
}

// gotoLastChildInternal is ts_tree_cursor_goto_last_child_internal.
func (c *TreeCursor) gotoLastChildInternal() treeCursorStep {
	iterator := c.iterateChildren()
	if iterator.parent.ptr == nil || iterator.parent.childCount() == 0 {
		return treeCursorStepNone
	}

	var lastEntry treeCursorEntry
	lastStep := treeCursorStepNone
	for {
		entry, visible, ok := iterator.next()
		if !ok {
			break
		}
		switch {
		case visible:
			lastEntry = entry
			lastStep = treeCursorStepVisible
		case entry.subtree.visibleChildCount() > 0:
			lastEntry = entry
			lastStep = treeCursorStepHidden
		}
	}
	if lastEntry.subtree != nil {
		c.stack = append(c.stack, lastEntry)
		return lastStep
	}

	return treeCursorStepNone
}

// GotoLastChild moves the cursor to the last child of its node. It returns
// false, and does not move, when the node has no children.
//
// GotoLastChild is ts_tree_cursor_goto_last_child.
func (c *TreeCursor) GotoLastChild() bool {
	for {
		switch c.gotoLastChildInternal() {
		case treeCursorStepHidden:
			continue
		case treeCursorStepVisible:
			return true
		default:
			return false
		}
	}
}

// gotoFirstChildForByteAndPoint is
// ts_tree_cursor_goto_first_child_for_byte_and_point.
func (c *TreeCursor) gotoFirstChildForByteAndPoint(goalByte uint32, goalPoint point) int64 {
	initialSize := len(c.stack)
	visibleChildIndex := uint32(0)

	for {
		didDescend := false

		iterator := c.iterateChildren()
		for {
			entry, visible, ok := iterator.next()
			if !ok {
				break
			}
			entryEnd := entry.position.add(entry.subtree.size())
			atGoal := entryEnd.bytes > goalByte && entryEnd.extent.gt(goalPoint)
			visibleChildCount := entry.subtree.visibleChildCount()
			switch {
			case atGoal:
				if visible {
					c.stack = append(c.stack, entry)
					return int64(visibleChildIndex)
				}
				if visibleChildCount > 0 {
					c.stack = append(c.stack, entry)
					didDescend = true
				}
			case visible:
				visibleChildIndex++
			default:
				visibleChildIndex += visibleChildCount
			}
			if didDescend {
				break
			}
		}
		if !didDescend {
			break
		}
	}

	c.stack = c.stack[:initialSize]
	return -1
}

// GotoFirstChildForByte moves the cursor to the first child of its node
// that ends after a byte offset. It returns the index of the child, or
// false, and no move, when no child ends after the offset.
//
// GotoFirstChildForByte is ts_tree_cursor_goto_first_child_for_byte.
func (c *TreeCursor) GotoFirstChildForByte(offset int) (int, bool) {
	index := c.gotoFirstChildForByteAndPoint(uint32(offset), pointZero)
	return int(index), index >= 0
}

// GotoFirstChildForPoint moves the cursor to the first child of its node
// that ends after a point. It returns the index of the child, or false, and
// no move, when no child ends after the point.
//
// GotoFirstChildForPoint is ts_tree_cursor_goto_first_child_for_point.
func (c *TreeCursor) GotoFirstChildForPoint(at Point) (int, bool) {
	index := c.gotoFirstChildForByteAndPoint(0, at.internal())
	return int(index), index >= 0
}

// gotoSiblingInternal is ts_tree_cursor_goto_sibling_internal.
func (c *TreeCursor) gotoSiblingInternal(
	advance func(*cursorChildIterator) (treeCursorEntry, bool, bool),
) treeCursorStep {
	initialSize := len(c.stack)

	for len(c.stack) > 1 {
		entry := c.stack[len(c.stack)-1]
		c.stack = c.stack[:len(c.stack)-1]
		iterator := c.iterateChildren()
		iterator.childIndex = entry.childIndex
		iterator.structuralChildIndex = entry.structuralChildIndex
		iterator.position = entry.position
		iterator.descendantIndex = entry.descendantIndex

		_, visible, _ := advance(&iterator)
		if visible && len(c.stack)+1 < initialSize {
			break
		}

		for {
			next, nextVisible, ok := advance(&iterator)
			if !ok {
				break
			}
			entry, visible = next, nextVisible
			if visible {
				c.stack = append(c.stack, entry)
				return treeCursorStepVisible
			}

			if entry.subtree.visibleChildCount() != 0 {
				c.stack = append(c.stack, entry)
				return treeCursorStepHidden
			}
		}
	}

	c.stack = c.stack[:initialSize]
	return treeCursorStepNone
}

// gotoNextSiblingInternal is ts_tree_cursor_goto_next_sibling_internal.
func (c *TreeCursor) gotoNextSiblingInternal() treeCursorStep {
	return c.gotoSiblingInternal((*cursorChildIterator).next)
}

// GotoNextSibling moves the cursor to the next sibling of its node. It
// returns false, and does not move, when the node has no next sibling.
//
// GotoNextSibling is ts_tree_cursor_goto_next_sibling.
func (c *TreeCursor) GotoNextSibling() bool {
	switch c.gotoNextSiblingInternal() {
	case treeCursorStepHidden:
		c.GotoFirstChild()
		return true
	case treeCursorStepVisible:
		return true
	default:
		return false
	}
}

// gotoPreviousSiblingInternal is
// ts_tree_cursor_goto_previous_sibling_internal.
func (c *TreeCursor) gotoPreviousSiblingInternal() treeCursorStep {
	// since subtracting across row loses column information, we may have to
	// restore it

	// for that, save current position before traversing
	step := c.gotoSiblingInternal((*cursorChildIterator).previous)
	if step == treeCursorStepNone {
		return step
	}

	// if length is already valid, there's no need to recompute it
	if !c.stack[len(c.stack)-1].position.isUndefined() {
		return step
	}

	// restore position from the parent node
	parent := &c.stack[len(c.stack)-2]
	position := parent.position
	childIndex := c.stack[len(c.stack)-1].childIndex
	children := parent.subtree.ptr.children

	if childIndex > 0 {
		// skip first child padding since its position should match the position of the parent
		position = position.add(children[0].size())
		for i := uint32(1); i < childIndex; i++ {
			position = position.add(children[i].totalSize())
		}
		position = position.add(children[childIndex].padding())
	}

	c.stack[len(c.stack)-1].position = position

	return step
}

// GotoPreviousSibling moves the cursor to the previous sibling of its node.
// It returns false, and does not move, when the node has no previous
// sibling.
//
// GotoPreviousSibling is ts_tree_cursor_goto_previous_sibling.
func (c *TreeCursor) GotoPreviousSibling() bool {
	switch c.gotoPreviousSiblingInternal() {
	case treeCursorStepHidden:
		c.GotoLastChild()
		return true
	case treeCursorStepVisible:
		return true
	default:
		return false
	}
}

// GotoParent moves the cursor to the parent of its node. It returns false,
// and does not move, when the node is the root of the cursor.
//
// GotoParent is ts_tree_cursor_goto_parent.
func (c *TreeCursor) GotoParent() bool {
	for i := uint32(len(c.stack)) - 2; i+1 > 0; i-- {
		if c.isEntryVisible(i) {
			c.stack = c.stack[:i+1]
			return true
		}
	}
	return false
}

// GotoDescendant moves the cursor to the descendant of the root of the
// cursor with an index, in the order of a walk from the root, where the root
// has the index 0.
//
// GotoDescendant is ts_tree_cursor_goto_descendant.
func (c *TreeCursor) GotoDescendant(index int) {
	goalDescendantIndex := uint32(index)

	// Ascend to the lowest ancestor that contains the goal node.
	for {
		i := uint32(len(c.stack) - 1)
		entry := &c.stack[i]
		nextDescendantIndex := entry.descendantIndex +
			uint32(boolInt(c.isEntryVisible(i))) +
			entry.subtree.visibleDescendantCount()
		if entry.descendantIndex <= goalDescendantIndex &&
			nextDescendantIndex > goalDescendantIndex {
			break
		} else if len(c.stack) <= 1 {
			return
		}
		c.stack = c.stack[:len(c.stack)-1]
	}

	// Descend to the goal node.
	for {
		didDescend := false
		iterator := c.iterateChildren()
		if iterator.descendantIndex > goalDescendantIndex {
			return
		}

		for {
			entry, visible, ok := iterator.next()
			if !ok {
				break
			}
			if iterator.descendantIndex > goalDescendantIndex {
				c.stack = append(c.stack, entry)
				if visible && entry.descendantIndex == goalDescendantIndex {
					return
				}
				didDescend = true
				break
			}
		}
		if !didDescend {
			break
		}
	}
}

// DescendantIndex returns the index of the node of the cursor among the
// descendants of the root of the cursor, as GotoDescendant counts them.
//
// DescendantIndex is ts_tree_cursor_current_descendant_index.
func (c *TreeCursor) DescendantIndex() int {
	lastEntry := &c.stack[len(c.stack)-1]
	return int(lastEntry.descendantIndex)
}

// Node returns the node of the cursor.
//
// Node is ts_tree_cursor_current_node.
func (c *TreeCursor) Node() Node {
	lastEntry := &c.stack[len(c.stack)-1]
	isExtra := lastEntry.subtree.extra()
	aliasSymbol := c.rootAliasSymbol
	if isExtra {
		aliasSymbol = 0
	}
	if len(c.stack) > 1 && !isExtra {
		parentEntry := &c.stack[len(c.stack)-2]
		aliasSymbol = c.tree.language.aliasAt(
			uint32(parentEntry.subtree.ptr.productionID),
			lastEntry.structuralChildIndex,
		)
	}
	return makeNode(
		c.tree,
		lastEntry.subtree,
		lastEntry.position,
		aliasSymbol,
	)
}

// currentStatus is ts_tree_cursor_current_status. The C function writes
// its results to out parameters, and the Go function returns them. It
// writes supertypes into the slice supertypes, up to its length, and
// supertypeCount is the number that it wrote.
//
// Private - Get various facts about the current node that are needed
// when executing tree queries.
func (c *TreeCursor) currentStatus(supertypes []Symbol) (
	fieldID FieldID,
	hasLaterSiblings bool,
	hasLaterNamedSiblings bool,
	canHaveLaterSiblingsWithThisField bool,
	supertypeCount uint32,
) {
	maxSupertypes := uint32(len(supertypes))

	// Walk up the tree, visiting the current node and its invisible ancestors,
	// because fields can refer to nodes through invisible *wrapper* nodes,
	for i := len(c.stack) - 1; i > 0; i-- {
		entry := &c.stack[i]
		parentEntry := &c.stack[i-1]

		aliasSequence := c.tree.language.aliasSequence(
			uint32(parentEntry.subtree.ptr.productionID),
		)

		// subtreeSymbol is the macro subtree_symbol.
		subtreeSymbol := func(subtree subtree, structuralChildIndex uint32) Symbol {
			if !subtree.extra() &&
				aliasSequence != nil &&
				aliasSequence[structuralChildIndex] != 0 {
				return Symbol(aliasSequence[structuralChildIndex])
			}
			return subtree.symbol()
		}

		// Stop walking up when a visible ancestor is found.
		entrySymbol := subtreeSymbol(
			*entry.subtree,
			entry.structuralChildIndex,
		)
		entryMetadata := c.tree.language.symbolMetadata(entrySymbol)
		if i != len(c.stack)-1 && entryMetadata.Visible {
			break
		}

		// Record any supertypes
		if entryMetadata.Supertype && supertypeCount < maxSupertypes {
			supertypes[supertypeCount] = entrySymbol
			supertypeCount++
		}

		// Determine if the current node has later siblings. A later *anonymous*
		// sibling settles `has_later_siblings` but says nothing about later *named*
		// siblings.
		if !hasLaterNamedSiblings {
			siblingCount := parentEntry.subtree.childCount()
			structuralChildIndex := entry.structuralChildIndex
			if !entry.subtree.extra() {
				structuralChildIndex++
			}
			for j := entry.childIndex + 1; j < siblingCount; j++ {
				sibling := parentEntry.subtree.ptr.children[j]
				siblingMetadata := c.tree.language.symbolMetadata(
					subtreeSymbol(sibling, structuralChildIndex),
				)
				if siblingMetadata.Visible {
					hasLaterSiblings = true
					if siblingMetadata.Named {
						hasLaterNamedSiblings = true
						break
					}
				} else if sibling.visibleChildCount() > 0 {
					hasLaterSiblings = true
					if sibling.ptr.namedChildCount > 0 {
						hasLaterNamedSiblings = true
						break
					}
				}
				if !sibling.extra() {
					structuralChildIndex++
				}
			}
		}

		if !entry.subtree.extra() {
			fieldMap := c.tree.language.fieldMap(
				uint32(parentEntry.subtree.ptr.productionID),
			)

			// Look for a field name associated with the current node.
			if fieldID == 0 {
				for _, m := range fieldMap {
					if !m.Inherited && uint32(m.ChildIndex) == entry.structuralChildIndex {
						fieldID = FieldID(m.FieldID)
						break
					}
				}
			}

			// Determine if the current node can have later siblings with the same field name.
			if fieldID != 0 {
				for _, m := range fieldMap {
					if FieldID(m.FieldID) == fieldID &&
						uint32(m.ChildIndex) > entry.structuralChildIndex {
						canHaveLaterSiblingsWithThisField = true
						break
					}
				}
			}
		}
	}
	return fieldID, hasLaterSiblings, hasLaterNamedSiblings, canHaveLaterSiblingsWithThisField, supertypeCount
}

// Depth returns the depth of the node of the cursor below the root of the
// cursor, where the root has the depth 0.
//
// Depth is ts_tree_cursor_current_depth.
func (c *TreeCursor) Depth() int {
	depth := uint32(0)
	for i := uint32(1); i < uint32(len(c.stack)); i++ {
		if c.isEntryVisible(i) {
			depth++
		}
	}
	return int(depth)
}

// parentNode is ts_tree_cursor_parent_node. It returns the null node when no
// entry of the stack below the top is visible.
func (c *TreeCursor) parentNode() Node {
	for i := len(c.stack) - 2; i >= 0; i-- {
		entry := &c.stack[i]
		isVisible := true
		aliasSymbol := Symbol(0)
		if i > 0 {
			parentEntry := &c.stack[i-1]
			aliasSymbol = c.tree.language.aliasAt(
				uint32(parentEntry.subtree.ptr.productionID),
				entry.structuralChildIndex,
			)
			isVisible = (aliasSymbol != 0) || entry.subtree.visible()
		}
		if isVisible {
			return makeNode(
				c.tree,
				entry.subtree,
				entry.position,
				aliasSymbol,
			)
		}
	}
	return makeNode(nil, nil, lengthZero(), 0)
}

// FieldID returns the field of the node of the cursor, or 0 when it has no
// field.
//
// FieldID is ts_tree_cursor_current_field_id.
func (c *TreeCursor) FieldID() FieldID {
	// Walk up the tree, visiting the current node and its invisible ancestors.
	for i := uint32(len(c.stack) - 1); i > 0; i-- {
		entry := &c.stack[i]
		parentEntry := &c.stack[i-1]

		// Stop walking up when another visible node is found.
		if i != uint32(len(c.stack)-1) &&
			c.isEntryVisible(i) {
			break
		}

		if entry.subtree.extra() {
			break
		}

		fieldMap := c.tree.language.fieldMap(
			uint32(parentEntry.subtree.ptr.productionID),
		)
		for _, m := range fieldMap {
			if !m.Inherited && uint32(m.ChildIndex) == entry.structuralChildIndex {
				return FieldID(m.FieldID)
			}
		}
	}
	return 0
}

// FieldName returns the name of the field of the node of the cursor, or ""
// when it has no field.
//
// FieldName is ts_tree_cursor_current_field_name.
func (c *TreeCursor) FieldName() string {
	id := c.FieldID()
	if id != 0 {
		return c.tree.language.tables.FieldNames[id]
	}
	return ""
}

// Copy returns a copy of the cursor, at the same node.
//
// Copy is ts_tree_cursor_copy.
func (c *TreeCursor) Copy() *TreeCursor {
	cursor := c
	copied := TreeCursor{
		tree:            cursor.tree,
		rootAliasSymbol: cursor.rootAliasSymbol,
		stack:           slices.Clone(cursor.stack),
	}
	return &copied
}

// ResetTo moves the cursor to the node of another cursor, with the root of
// that cursor.
//
// ResetTo is ts_tree_cursor_reset_to.
func (c *TreeCursor) ResetTo(other *TreeCursor) {
	cursor := other
	copied := c
	copied.tree = cursor.tree
	copied.rootAliasSymbol = cursor.rootAliasSymbol
	copied.stack = append(copied.stack[:0], cursor.stack...)
}
