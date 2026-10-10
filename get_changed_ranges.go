package transit

import "math"

// This file ports lib/src/get_changed_ranges.c and
// lib/src/get_changed_ranges.h.
//
// TSRangeArray is a slice of textRange. The function iterator_print_state
// and the output of DEBUG_GET_CHANGED_RANGES are off in upstream, and they
// have no Go form. The type Iterator of C is changedRangesIterator, and its
// functions are its methods. InputEdit.EditRange is InputEdit::edit_range
// of the Rust binding, which calls ts_range_edit (D119).

// rangeArrayAdd is ts_range_array_add.
func rangeArrayAdd(self *[]textRange, start, end length) {
	if len(*self) > 0 {
		lastRange := &(*self)[len(*self)-1]
		if start.bytes <= lastRange.endByte {
			lastRange.endByte = end.bytes
			lastRange.endPoint = end.extent
			return
		}
	}

	if start.bytes < end.bytes {
		r := textRange{start.extent, end.extent, start.bytes, end.bytes}
		*self = append(*self, r)
	}
}

// rangeArrayIntersects is ts_range_array_intersects.
func rangeArrayIntersects(self []textRange, startIndex uint32, startByte, endByte uint32) bool {
	for i := startIndex; i < uint32(len(self)); i++ {
		r := &self[i]
		if r.endByte > startByte {
			if r.startByte >= endByte {
				break
			}
			return true
		}
	}
	return false
}

// rangeArrayGetChangedRanges is ts_range_array_get_changed_ranges. The C
// function takes the address of the range at each index before it checks
// the index, and the Go function reads a range only at an index that it
// checked.
func rangeArrayGetChangedRanges(oldRanges, newRanges []textRange, differences *[]textRange) {
	oldRangeCount := uint32(len(oldRanges))
	newRangeCount := uint32(len(newRanges))
	newIndex := uint32(0)
	oldIndex := uint32(0)
	currentPosition := lengthZero()
	inOldRange := false
	inNewRange := false

	for oldIndex < oldRangeCount || newIndex < newRangeCount {
		var nextOldPosition length
		switch {
		case inOldRange:
			oldRange := &oldRanges[oldIndex]
			nextOldPosition = length{oldRange.endByte, oldRange.endPoint}
		case oldIndex < oldRangeCount:
			oldRange := &oldRanges[oldIndex]
			nextOldPosition = length{oldRange.startByte, oldRange.startPoint}
		default:
			nextOldPosition = lengthMax
		}

		var nextNewPosition length
		switch {
		case inNewRange:
			newRange := &newRanges[newIndex]
			nextNewPosition = length{newRange.endByte, newRange.endPoint}
		case newIndex < newRangeCount:
			newRange := &newRanges[newIndex]
			nextNewPosition = length{newRange.startByte, newRange.startPoint}
		default:
			nextNewPosition = lengthMax
		}

		switch {
		case nextOldPosition.bytes < nextNewPosition.bytes:
			if inOldRange != inNewRange {
				rangeArrayAdd(differences, currentPosition, nextOldPosition)
			}
			if inOldRange {
				oldIndex++
			}
			currentPosition = nextOldPosition
			inOldRange = !inOldRange
		case nextNewPosition.bytes < nextOldPosition.bytes:
			if inOldRange != inNewRange {
				rangeArrayAdd(differences, currentPosition, nextNewPosition)
			}
			if inNewRange {
				newIndex++
			}
			currentPosition = nextNewPosition
			inNewRange = !inNewRange
		default:
			if inOldRange != inNewRange {
				rangeArrayAdd(differences, currentPosition, nextNewPosition)
			}
			if inOldRange {
				oldIndex++
			}
			if inNewRange {
				newIndex++
			}
			inOldRange = !inOldRange
			inNewRange = !inNewRange
			currentPosition = nextNewPosition
		}
	}
}

// edit is ts_range_edit.
func (r *textRange) edit(edit InputEdit) {
	startByte := uint32(edit.StartByte)
	oldEndByte := uint32(edit.OldEndByte)
	newEndByte := uint32(edit.NewEndByte)
	startPoint := edit.StartPoint.internal()
	oldEndPoint := edit.OldEndPoint.internal()
	newEndPoint := edit.NewEndPoint.internal()

	if r.endByte >= oldEndByte {
		if r.endByte != math.MaxUint32 {
			r.endByte = newEndByte + (r.endByte - oldEndByte)
			r.endPoint = newEndPoint.add(r.endPoint.sub(oldEndPoint))
			if r.endByte < newEndByte {
				r.endByte = math.MaxUint32
				r.endPoint = pointMax
			}
		}
	} else if r.endByte > startByte {
		r.endByte = startByte
		r.endPoint = startPoint
	}

	if r.startByte >= oldEndByte {
		r.startByte = newEndByte + (r.startByte - oldEndByte)
		r.startPoint = newEndPoint.add(r.startPoint.sub(oldEndPoint))
		if r.startByte < newEndByte {
			r.startByte = math.MaxUint32
			r.startPoint = pointMax
		}
	} else if r.startByte > startByte {
		r.startByte = startByte
		r.startPoint = startPoint
	}
}

// EditRange changes a range so that it stays at the same place in the text
// after the edit e. It needs no tree and no node.
//
// EditRange is InputEdit::edit_range of the Rust binding, with
// ts_range_edit.
func (e InputEdit) EditRange(r *Range) {
	tr := r.internal()
	tr.edit(e)
	*r = tr.public()
}

// changedRangesIterator is Iterator. It holds a copy of a TreeCursor, which
// shares the stack of the cursor, as the copy of C does.
type changedRangesIterator struct {
	cursor            TreeCursor
	language          *Language
	visibleDepth      uint32
	inPadding         bool
	prevExternalToken subtree
}

// newChangedRangesIterator is iterator_new.
func newChangedRangesIterator(
	cursor *TreeCursor,
	tree *subtree,
	language *Language,
) changedRangesIterator {
	cursor.stack = append(cursor.stack[:0], treeCursorEntry{
		subtree:              tree,
		position:             lengthZero(),
		childIndex:           0,
		structuralChildIndex: 0,
	})
	return changedRangesIterator{
		cursor:            *cursor,
		language:          language,
		visibleDepth:      1,
		inPadding:         false,
		prevExternalToken: subtree{},
	}
}

// done is iterator_done.
func (it *changedRangesIterator) done() bool {
	return len(it.cursor.stack) == 0
}

// startPosition is iterator_start_position.
func (it *changedRangesIterator) startPosition() length {
	entry := it.cursor.stack[len(it.cursor.stack)-1]
	if it.inPadding {
		return entry.position
	}
	return entry.position.add(entry.subtree.padding())
}

// endPosition is iterator_end_position.
func (it *changedRangesIterator) endPosition() length {
	entry := it.cursor.stack[len(it.cursor.stack)-1]
	result := entry.position.add(entry.subtree.padding())
	if it.inPadding {
		return result
	}
	return result.add(entry.subtree.size())
}

// treeIsVisible is iterator_tree_is_visible.
func (it *changedRangesIterator) treeIsVisible() bool {
	entry := it.cursor.stack[len(it.cursor.stack)-1]
	if entry.subtree.visible() {
		return true
	}
	if len(it.cursor.stack) > 1 {
		parent := *it.cursor.stack[len(it.cursor.stack)-2].subtree
		return it.language.aliasAt(
			uint32(parent.ptr.productionID),
			entry.structuralChildIndex,
		) != 0
	}
	return false
}

// getVisibleState is iterator_get_visible_state. The C function writes its
// results to out parameters, which the caller sets to NULL_SUBTREE, 0 and 0.
// The Go function returns them, and they start at those values.
func (it *changedRangesIterator) getVisibleState() (tree subtree, aliasSymbol Symbol, startByte uint32) {
	i := uint32(len(it.cursor.stack) - 1)

	if it.inPadding {
		if i == 0 {
			return tree, aliasSymbol, startByte
		}
		i--
	}

	for ; i+1 > 0; i-- {
		entry := it.cursor.stack[i]

		if i > 0 {
			parent := it.cursor.stack[i-1].subtree
			aliasSymbol = it.language.aliasAt(
				uint32(parent.ptr.productionID),
				entry.structuralChildIndex,
			)
		}

		if entry.subtree.visible() || aliasSymbol != 0 {
			tree = *entry.subtree
			startByte = entry.position.bytes
			break
		}
	}
	return tree, aliasSymbol, startByte
}

// ascend is iterator_ascend.
func (it *changedRangesIterator) ascend() {
	if it.done() {
		return
	}
	if it.treeIsVisible() && !it.inPadding {
		it.visibleDepth--
	}
	if it.cursor.stack[len(it.cursor.stack)-1].childIndex > 0 {
		it.inPadding = false
	}
	it.cursor.stack = it.cursor.stack[:len(it.cursor.stack)-1]
}

// descend is iterator_descend.
func (it *changedRangesIterator) descend(goalPosition uint32) bool {
	if it.inPadding {
		return false
	}

	for {
		didDescend := false
		entry := it.cursor.stack[len(it.cursor.stack)-1]
		position := entry.position
		structuralChildIndex := uint32(0)
		for i, n := uint32(0), entry.subtree.childCount(); i < n; i++ {
			child := &entry.subtree.ptr.children[i]
			childLeft := position.add(child.padding())
			childRight := childLeft.add(child.size())

			if childRight.bytes > goalPosition {
				it.cursor.stack = append(it.cursor.stack, treeCursorEntry{
					subtree:              child,
					position:             position,
					childIndex:           i,
					structuralChildIndex: structuralChildIndex,
				})

				if it.treeIsVisible() {
					if childLeft.bytes > goalPosition {
						it.inPadding = true
					} else {
						it.visibleDepth++
					}
					return true
				}

				didDescend = true
				break
			}

			position = childRight
			if !child.extra() {
				structuralChildIndex++
			}
			lastExternalToken := child.lastExternalToken()
			if lastExternalToken.ptr != nil {
				it.prevExternalToken = lastExternalToken
			}
		}
		if !didDescend {
			break
		}
	}

	return false
}

// advance is iterator_advance.
func (it *changedRangesIterator) advance() {
	if it.inPadding {
		it.inPadding = false
		if it.treeIsVisible() {
			it.visibleDepth++
		} else {
			it.descend(0)
		}
		return
	}

	for {
		if it.treeIsVisible() {
			it.visibleDepth--
		}
		entry := it.cursor.stack[len(it.cursor.stack)-1]
		it.cursor.stack = it.cursor.stack[:len(it.cursor.stack)-1]
		if it.done() {
			return
		}

		parent := it.cursor.stack[len(it.cursor.stack)-1].subtree
		childIndex := entry.childIndex + 1
		lastExternalToken := entry.subtree.lastExternalToken()
		if lastExternalToken.ptr != nil {
			it.prevExternalToken = lastExternalToken
		}
		if parent.childCount() > childIndex {
			position := entry.position.add(entry.subtree.totalSize())
			structuralChildIndex := entry.structuralChildIndex
			if !entry.subtree.extra() {
				structuralChildIndex++
			}
			nextChild := &parent.ptr.children[childIndex]

			it.cursor.stack = append(it.cursor.stack, treeCursorEntry{
				subtree:              nextChild,
				position:             position,
				childIndex:           childIndex,
				structuralChildIndex: structuralChildIndex,
			})

			if it.treeIsVisible() {
				if nextChild.padding().bytes > 0 {
					it.inPadding = true
				} else {
					it.visibleDepth++
				}
			} else {
				it.descend(0)
			}
			break
		}
	}
}

// iteratorComparison is IteratorComparison.
type iteratorComparison int

// The results of iteratorCompare.
const (
	// iteratorDiffers is IteratorDiffers.
	iteratorDiffers iteratorComparison = iota
	// iteratorMayDiffer is IteratorMayDiffer.
	iteratorMayDiffer
	// iteratorMatches is IteratorMatches.
	iteratorMatches
)

// String returns the name of the comparison.
func (c iteratorComparison) String() string {
	switch c {
	case iteratorDiffers:
		return "differs"
	case iteratorMayDiffer:
		return "may differ"
	case iteratorMatches:
		return "matches"
	}
	return unknownName
}

// iteratorCompare is iterator_compare.
//
// The C function reads the symbols of the two subtrees before it checks
// that they are not NULL_SUBTREE, and C reads a NULL subtree through a NULL
// pointer. A Go program stops there, so the Go function reads the symbols
// after the check. The result is the same, because the function uses the
// symbols only after the check.
func iteratorCompare(oldIter, newIter *changedRangesIterator) iteratorComparison {
	oldTree, oldAliasSymbol, oldStart := oldIter.getVisibleState()
	newTree, newAliasSymbol, newStart := newIter.getVisibleState()

	if oldTree.ptr == nil && newTree.ptr == nil {
		return iteratorMatches
	}
	if oldTree.ptr == nil || newTree.ptr == nil {
		return iteratorDiffers
	}
	oldSymbol := oldTree.symbol()
	newSymbol := newTree.symbol()
	if oldAliasSymbol != newAliasSymbol || oldSymbol != newSymbol {
		return iteratorDiffers
	}

	oldSize := oldTree.size().bytes
	newSize := newTree.size().bytes
	oldState := oldTree.parseState()
	newState := newTree.parseState()
	oldHasExternalTokens := oldTree.hasExternalTokens()
	newHasExternalTokens := newTree.hasExternalTokens()
	oldErrorCost := oldTree.errorCost()
	newErrorCost := newTree.errorCost()

	if oldStart != newStart ||
		oldSymbol == builtinSymError ||
		oldSize != newSize ||
		oldState == tsTreeStateNone ||
		newState == tsTreeStateNone ||
		((oldState == errorState) != (newState == errorState)) ||
		oldErrorCost != newErrorCost ||
		oldHasExternalTokens != newHasExternalTokens ||
		oldTree.hasChanges() ||
		(oldHasExternalTokens &&
			!oldIter.prevExternalToken.externalScannerStateEq(newIter.prevExternalToken)) {
		return iteratorMayDiffer
	}

	return iteratorMatches
}

// subtreeGetChangedRanges is ts_subtree_get_changed_ranges. The C function
// writes the ranges to an out parameter and returns their number, and the Go
// function returns the ranges.
func subtreeGetChangedRanges(
	oldTree, newTree *subtree,
	cursor1, cursor2 *TreeCursor,
	language *Language,
	includedRangeDifferences []textRange,
) []textRange {
	var results []textRange

	oldIter := newChangedRangesIterator(cursor1, oldTree, language)
	newIter := newChangedRangesIterator(cursor2, newTree, language)

	includedRangeDifferenceIndex := uint32(0)

	position := oldIter.startPosition()
	nextPosition := newIter.startPosition()
	if position.bytes < nextPosition.bytes {
		rangeArrayAdd(&results, position, nextPosition)
		position = nextPosition
	} else if position.bytes > nextPosition.bytes {
		rangeArrayAdd(&results, nextPosition, position)
		nextPosition = position
	}

	for {
		// Compare the old and new subtrees.
		comparison := iteratorCompare(&oldIter, &newIter)

		// Even if the two subtrees appear to be identical, they could differ
		// internally if they contain a range of text that was previously
		// excluded from the parse, and is now included, or vice-versa.
		if comparison == iteratorMatches && rangeArrayIntersects(
			includedRangeDifferences,
			includedRangeDifferenceIndex,
			position.bytes,
			oldIter.endPosition().bytes,
		) {
			comparison = iteratorMayDiffer
		}

		isChanged := false
		switch comparison {
		// If the subtrees are definitely identical, move to the end
		// of both subtrees.
		case iteratorMatches:
			nextPosition = oldIter.endPosition()

		// If the subtrees might differ internally, descend into both
		// subtrees, finding the first child that spans the current position.
		case iteratorMayDiffer:
			switch {
			case oldIter.descend(position.bytes):
				if !newIter.descend(position.bytes) {
					isChanged = true
					nextPosition = oldIter.endPosition()
				}
			case newIter.descend(position.bytes):
				isChanged = true
				nextPosition = newIter.endPosition()
			default:
				nextPosition = lengthMin(
					oldIter.endPosition(),
					newIter.endPosition(),
				)
			}

		// If the subtrees are different, record a change and then move
		// to the end of both subtrees.
		case iteratorDiffers:
			isChanged = true
			nextPosition = lengthMin(
				oldIter.endPosition(),
				newIter.endPosition(),
			)
		}

		// Ensure that both iterators are caught up to the current position.
		for !oldIter.done() &&
			oldIter.endPosition().bytes <= nextPosition.bytes {
			oldIter.advance()
		}
		for !newIter.done() &&
			newIter.endPosition().bytes <= nextPosition.bytes {
			newIter.advance()
		}

		// Ensure that both iterators are at the same depth in the tree.
		for oldIter.visibleDepth > newIter.visibleDepth {
			oldIter.ascend()
		}
		for newIter.visibleDepth > oldIter.visibleDepth {
			newIter.ascend()
		}

		if isChanged {
			rangeArrayAdd(&results, position, nextPosition)
		}

		position = nextPosition

		// Keep track of the current position in the included range differences
		// array in order to avoid scanning the entire array on each iteration.
		for includedRangeDifferenceIndex < uint32(len(includedRangeDifferences)) {
			r := &includedRangeDifferences[includedRangeDifferenceIndex]
			if r.endByte <= position.bytes {
				includedRangeDifferenceIndex++
			} else {
				break
			}
		}

		if oldIter.done() || newIter.done() {
			break
		}
	}

	oldSize := oldTree.totalSize()
	newSize := newTree.totalSize()
	if oldSize.bytes < newSize.bytes {
		rangeArrayAdd(&results, oldSize, newSize)
	} else if newSize.bytes < oldSize.bytes {
		rangeArrayAdd(&results, newSize, oldSize)
	}

	*cursor1 = oldIter.cursor
	*cursor2 = newIter.cursor
	return results
}
