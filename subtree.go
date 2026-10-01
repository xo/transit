package transit

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/subtree.c and lib/src/subtree.h, and the type
// TSInputEdit of lib/include/tree_sitter/api.h.
//
// A subtree is a pointer to a node, and the node holds a slice of its
// children. A parser takes the nodes and the slices from chunks (D62). The
// garbage collector frees the memory, so these parts have no Go form:
// ts_subtree_pool_delete, ts_subtree_alloc_size and the conversions
// ts_subtree_from_mut and ts_subtree_to_mut_unsafe. Subtree and
// MutableSubtree have the one Go form subtree, because Go has no const.
//
// The reference count stays, because upstream reads it to decide what to
// do. ts_subtree_make_mut changes a node in place only when one tree holds
// it, and ts_subtree_compress and the balancing of the parser stop at a node
// that two trees hold. So retain and release keep the count as C keeps it
// (D64). When the count of a leaf reaches 0, release gives the node to the
// free list of the pool, as C does, and the next leaf of the pool takes it
// (D91). The garbage collector frees any other node whose count reaches 0.
//
// C stores a small leaf inline, in the Subtree value itself. A Go node has
// no inline form (D62), but the inline form of C changes what a leaf keeps:
// the size of an inline leaf has no column of its own, and an inline leaf has
// no depends_on_column. So a Go node that C would store inline has isInline,
// and it keeps only what the inline form keeps. C copies an inline leaf as a
// value, and Go shares the node, so ts_subtree_make_mut copies an inline
// node before a change (D64).

// InputEdit describes one edit of the text.
//
// InputEdit is TSInputEdit.
type InputEdit struct {
	StartByte   int
	OldEndByte  int
	NewEndByte  int
	StartPoint  Point
	OldEndPoint Point
	NewEndPoint Point
}

// tsTreeStateNone is TS_TREE_STATE_NONE.
const tsTreeStateNone = StateID(math.MaxUint16)

// externalScannerState is ExternalScannerState, the serialized state of an
// external scanner.
//
// Every time an external token subtree is created after a call to an
// external scanner, the scanner's `serialize` function is called to
// retrieve a serialized copy of its state. The bytes are then copied
// onto the subtree itself so that the scanner's state can later be
// restored using its `deserialize` function.
//
// C stores a short state inline and a long state on the heap. Go keeps a
// slice for both, and its length is the member length. The parser takes the
// slice from the chunks of its subtreePool, so that a short state does not
// allocate, as in C.
type externalScannerState struct {
	buf []byte
}

// subtreeHeapData is SubtreeHeapData, a node of a tree.
type subtreeHeapData struct {
	heapFields

	// refCount is ref_count.
	refCount atomic.Uint32
}

// heapFields holds the members of SubtreeHeapData other than ref_count. They
// are a struct of their own so that a clone copies them without a race with
// an atomic change of the count.
type heapFields struct {
	padding        length
	size           length
	lookaheadBytes uint32
	errorCost      uint32
	symbol         Symbol
	parseState     StateID

	visible                       bool
	named                         bool
	extra                         bool
	fragileLeft                   bool
	fragileRight                  bool
	hasChanges                    bool
	hasExternalTokens             bool
	hasExternalScannerStateChange bool
	dependsOnColumn               bool
	isMissing                     bool
	isKeyword                     bool

	// isInline is is_inline of SubtreeInlineData: C stores this leaf in the
	// Subtree value.
	isInline bool

	// children holds the children, which C allocates just before the node.
	// Its length is child_count.
	children []subtree

	// The members of the union of SubtreeHeapData. C shares their memory,
	// and Go gives each its own field.

	// Non-terminal subtrees (`child_count > 0`)
	visibleChildCount      uint32
	namedChildCount        uint32
	visibleDescendantCount uint32
	dynamicPrecedence      int32
	repeatDepth            uint16
	productionID           uint16
	firstLeaf              firstLeaf

	// External terminal subtrees (`child_count == 0 && has_external_tokens`)
	externalScannerState externalScannerState

	// Error terminal subtrees (`child_count == 0 && symbol == ts_builtin_sym_error`)
	lookaheadChar int32
}

// firstLeaf is the member first_leaf of SubtreeHeapData.
type firstLeaf struct {
	symbol     Symbol
	parseState StateID
}

// subtree is Subtree and MutableSubtree, the fundamental building block of a
// syntax tree. The zero subtree is NULL_SUBTREE.
type subtree struct {
	ptr *subtreeHeapData
}

// subtreeArray is SubtreeArray and MutableSubtreeArray.
type subtreeArray []subtree

// subtreePool is SubtreePool. freeTrees is free_trees, and treeStack is
// tree_stack. The Go pool also gives out the nodes, the slices of children
// and the states of the external scanner from chunks (D62). A node of
// freeTrees is a slot of a chunk, and free clears it, so that it keeps no
// other memory alive.
type subtreePool struct {
	freeTrees subtreeArray
	treeStack subtreeArray

	nodes        []subtreeHeapData
	children     []subtree
	states       []byte
	nodeChunk    int
	childrenSize int
	statesSize   int
}

// The sizes of a chunk of a subtreePool. The first chunk is small, because
// an edit makes a pool of its own for a few nodes, and each next chunk is
// twice as large, up to the size of the benchmark of D62. A chunk of states
// holds stateChunkScale bytes for each node of a chunk of nodes.
const (
	minSubtreeChunk = 16
	maxSubtreeChunk = 512
	stateChunkScale = 16
)

// symbol is ts_subtree_symbol.
func (s subtree) symbol() Symbol { return s.ptr.symbol }

// visible is ts_subtree_visible.
func (s subtree) visible() bool { return s.ptr.visible }

// named is ts_subtree_named.
func (s subtree) named() bool { return s.ptr.named }

// extra is ts_subtree_extra.
func (s subtree) extra() bool { return s.ptr.extra }

// hasChanges is ts_subtree_has_changes.
func (s subtree) hasChanges() bool { return s.ptr.hasChanges }

// missing is ts_subtree_missing.
func (s subtree) missing() bool { return s.ptr.isMissing }

// isKeyword is ts_subtree_is_keyword.
func (s subtree) isKeyword() bool { return s.ptr.isKeyword }

// parseState is ts_subtree_parse_state.
func (s subtree) parseState() StateID { return s.ptr.parseState }

// lookaheadBytes is ts_subtree_lookahead_bytes.
func (s subtree) lookaheadBytes() uint32 { return s.ptr.lookaheadBytes }

// setExtra is ts_subtree_set_extra.
func (s subtree) setExtra(isExtra bool) {
	s.ptr.extra = isExtra
}

// leafSymbol is ts_subtree_leaf_symbol.
func (s subtree) leafSymbol() Symbol {
	if len(s.ptr.children) == 0 {
		return s.ptr.symbol
	}
	return s.ptr.firstLeaf.symbol
}

// leafParseState is ts_subtree_leaf_parse_state.
func (s subtree) leafParseState() StateID {
	if len(s.ptr.children) == 0 {
		return s.ptr.parseState
	}
	return s.ptr.firstLeaf.parseState
}

// padding is ts_subtree_padding.
func (s subtree) padding() length { return s.ptr.padding }

// size is ts_subtree_size.
func (s subtree) size() length { return s.ptr.size }

// totalSize is ts_subtree_total_size.
func (s subtree) totalSize() length {
	return s.padding().add(s.size())
}

// totalBytes is ts_subtree_total_bytes.
func (s subtree) totalBytes() uint32 {
	return s.totalSize().bytes
}

// childCount is ts_subtree_child_count.
func (s subtree) childCount() uint32 {
	return uint32(len(s.ptr.children))
}

// repeatDepth is ts_subtree_repeat_depth. In C, repeat_depth of a leaf that
// is not inline shares its memory with the state of an external scanner, so
// C can read other bytes there. Only the balancing of a repetition reads the
// depth, and the children of a repetition are nodes.
func (s subtree) repeatDepth() uint32 {
	return uint32(s.ptr.repeatDepth)
}

// isRepetition is ts_subtree_is_repetition.
func (s subtree) isRepetition() bool {
	return !s.ptr.isInline && !s.ptr.named && !s.ptr.visible && len(s.ptr.children) != 0
}

// visibleDescendantCount is ts_subtree_visible_descendant_count.
func (s subtree) visibleDescendantCount() uint32 {
	if len(s.ptr.children) == 0 {
		return 0
	}
	return s.ptr.visibleDescendantCount
}

// visibleChildCount is ts_subtree_visible_child_count.
func (s subtree) visibleChildCount() uint32 {
	if s.childCount() > 0 {
		return s.ptr.visibleChildCount
	}
	return 0
}

// errorCost is ts_subtree_error_cost.
func (s subtree) errorCost() uint32 {
	if s.missing() {
		return errorCostPerMissingTree + errorCostPerRecovery
	}
	return s.ptr.errorCost
}

// dynamicPrecedence is ts_subtree_dynamic_precedence.
func (s subtree) dynamicPrecedence() int32 {
	if len(s.ptr.children) == 0 {
		return 0
	}
	return s.ptr.dynamicPrecedence
}

// productionID is ts_subtree_production_id.
func (s subtree) productionID() uint16 {
	if s.childCount() > 0 {
		return s.ptr.productionID
	}
	return 0
}

// fragileLeft is ts_subtree_fragile_left.
func (s subtree) fragileLeft() bool { return s.ptr.fragileLeft }

// fragileRight is ts_subtree_fragile_right.
func (s subtree) fragileRight() bool { return s.ptr.fragileRight }

// hasExternalTokens is ts_subtree_has_external_tokens.
func (s subtree) hasExternalTokens() bool { return s.ptr.hasExternalTokens }

// hasExternalScannerStateChange is
// ts_subtree_has_external_scanner_state_change.
func (s subtree) hasExternalScannerStateChange() bool {
	return s.ptr.hasExternalScannerStateChange
}

// dependsOnColumn is ts_subtree_depends_on_column.
func (s subtree) dependsOnColumn() bool { return s.ptr.dependsOnColumn }

// isFragile is ts_subtree_is_fragile.
func (s subtree) isFragile() bool {
	return s.ptr.fragileLeft || s.ptr.fragileRight
}

// isError is ts_subtree_is_error.
func (s subtree) isError() bool {
	return s.symbol() == builtinSymError
}

// isEOF is ts_subtree_is_eof.
func (s subtree) isEOF() bool {
	return s.symbol() == builtinSymEnd
}

// init is ts_external_scanner_state_init. It keeps a copy of data, in a
// slice from pool.
func (e *externalScannerState) init(pool *subtreePool, data []byte) {
	e.buf = pool.allocateState(len(data))
	copy(e.buf, data)
}

// delete is ts_external_scanner_state_delete. The garbage collector frees
// the bytes.
func (e *externalScannerState) delete() {
	e.buf = nil
}

// copy is ts_external_scanner_state_copy.
func (e *externalScannerState) copy() externalScannerState {
	return externalScannerState{buf: slices.Clone(e.buf)}
}

// data is ts_external_scanner_state_data.
func (e *externalScannerState) data() []byte {
	return e.buf
}

// eq is ts_external_scanner_state_eq.
func (e *externalScannerState) eq(buffer []byte) bool {
	return bytes.Equal(e.data(), buffer)
}

// copy is ts_subtree_array_copy. It returns a copy of the array and retains
// each subtree. C allocates the copy with malloc, and Go takes it from the
// chunks of pool.
func (a subtreeArray) copy(pool *subtreePool) subtreeArray {
	if cap(a) == 0 {
		return a
	}
	dest := pool.allocateChildren(cap(a))[:len(a)]
	copy(dest, a)
	for _, s := range dest {
		s.retain()
	}
	return dest
}

// clear is ts_subtree_array_clear.
func (a *subtreeArray) clear(pool *subtreePool) {
	for _, s := range *a {
		s.release(pool)
	}
	*a = (*a)[:0]
}

// delete is ts_subtree_array_delete.
func (a *subtreeArray) delete(pool *subtreePool) {
	a.clear(pool)
	*a = nil
}

// removeTrailingExtras is ts_subtree_array_remove_trailing_extras.
func (a *subtreeArray) removeTrailingExtras(destination *subtreeArray) {
	*destination = (*destination)[:0]
	for len(*a) > 0 {
		last := (*a)[len(*a)-1]
		if last.extra() {
			*a = (*a)[:len(*a)-1]
			*destination = append(*destination, last)
		} else {
			break
		}
	}
	destination.reverse()
}

// reverse is ts_subtree_array_reverse.
func (a subtreeArray) reverse() {
	slices.Reverse(a)
}

// newSubtreePool is ts_subtree_pool_new. A pool whose capacity is 0 keeps no
// free node.
func newSubtreePool(capacity int) subtreePool {
	return subtreePool{freeTrees: make(subtreeArray, 0, capacity)}
}

// allocate is ts_subtree_pool_allocate. It returns a node of the free list,
// or else a new node from the chunk of the pool. The node holds only zero
// values.
func (p *subtreePool) allocate() *subtreeHeapData {
	if n := len(p.freeTrees); n > 0 {
		tree := p.freeTrees[n-1]
		p.freeTrees = p.freeTrees[:n-1]
		return tree.ptr
	}
	return p.allocateNode()
}

// free is ts_subtree_pool_free. It gives the node, whose count is 0, to the
// free list, when the list has room. C frees any other node, and the Go
// function leaves it to the garbage collector. It clears the node, so that
// a node of the list keeps no other memory alive.
func (p *subtreePool) free(tree *subtreeHeapData) {
	if cap(p.freeTrees) > 0 && len(p.freeTrees)+1 <= tsMaxTreePoolSize {
		tree.heapFields = heapFields{}
		p.freeTrees = append(p.freeTrees, subtree{tree})
	}
}

// allocateNode returns a new node from the chunk of the pool, with a count of
// 0. It is the Go form of the ts_malloc of a node, which does not take a node
// of the free list.
func (p *subtreePool) allocateNode() *subtreeHeapData {
	if len(p.nodes) == 0 {
		p.nodeChunk = nextSubtreeChunk(p.nodeChunk)
		p.nodes = make([]subtreeHeapData, p.nodeChunk)
	}
	data := &p.nodes[0]
	p.nodes = p.nodes[1:]
	return data
}

// allocateChildren returns a slice of n children from the chunk of the pool,
// whose capacity is n.
func (p *subtreePool) allocateChildren(n int) subtreeArray {
	if n > len(p.children) {
		p.childrenSize = nextSubtreeChunk(p.childrenSize)
		p.children = make([]subtree, max(p.childrenSize, n))
	}
	children := p.children[:n:n]
	p.children = p.children[n:]
	return children
}

// allocateState returns a slice of n bytes from the chunk of the pool, for
// the state of an external scanner, whose capacity is n.
func (p *subtreePool) allocateState(n int) []byte {
	if n > len(p.states) {
		p.statesSize = nextSubtreeChunk(p.statesSize)
		p.states = make([]byte, max(stateChunkScale*p.statesSize, n))
	}
	state := p.states[:n:n]
	p.states = p.states[n:]
	return state
}

// nextSubtreeChunk returns the size of the chunk after a chunk of the size
// last, which is 0 for the first chunk.
func nextSubtreeChunk(last int) int {
	return min(max(2*last, minSubtreeChunk), maxSubtreeChunk)
}

// canInline is ts_subtree_can_inline.
func canInline(padding, size length, lookaheadBytes uint32) bool {
	return padding.bytes < tsMaxInlineTreeLength &&
		padding.extent.row < 16 &&
		padding.extent.column < tsMaxInlineTreeLength &&
		size.bytes < tsMaxInlineTreeLength &&
		size.extent.row == 0 &&
		size.extent.column < tsMaxInlineTreeLength &&
		lookaheadBytes < 16
}

const (
	// tsMaxInlineTreeLength is TS_MAX_INLINE_TREE_LENGTH.
	tsMaxInlineTreeLength = math.MaxUint8
	// tsMaxTreePoolSize is TS_MAX_TREE_POOL_SIZE, the most nodes that the free
	// list of a pool holds.
	tsMaxTreePoolSize = 32
)

// newLeaf is ts_subtree_new_leaf. A leaf that C stores inline keeps only
// what SubtreeInlineData keeps: the size has the column size.bytes, and
// dependsOnColumn is false. C allocates nothing for an inline leaf, and Go
// takes a node from the pool for both kinds of leaf.
func newLeaf(
	pool *subtreePool, symbol Symbol, padding, size length,
	lookaheadBytes uint32, parseState StateID,
	hasExternalTokens, dependsOnColumn,
	isKeyword bool, language *Language,
) subtree {
	metadata := language.symbolMetadata(symbol)
	extra := symbol == builtinSymEnd

	isInline := symbol <= math.MaxUint8 &&
		!hasExternalTokens &&
		canInline(padding, size, lookaheadBytes)

	data := pool.allocate()
	data.refCount.Store(1)
	if isInline {
		data.heapFields = heapFields{
			parseState:     parseState,
			symbol:         symbol,
			padding:        padding,
			size:           length{size.bytes, point{0, size.bytes}},
			lookaheadBytes: lookaheadBytes,
			visible:        metadata.Visible,
			named:          metadata.Named,
			extra:          extra,
			hasChanges:     false,
			isMissing:      false,
			isKeyword:      isKeyword,
			isInline:       true,
		}
		return subtree{data}
	}
	data.heapFields = heapFields{
		padding:                       padding,
		size:                          size,
		lookaheadBytes:                lookaheadBytes,
		errorCost:                     0,
		symbol:                        symbol,
		parseState:                    parseState,
		visible:                       metadata.Visible,
		named:                         metadata.Named,
		extra:                         extra,
		fragileLeft:                   false,
		fragileRight:                  false,
		hasChanges:                    false,
		hasExternalTokens:             hasExternalTokens,
		hasExternalScannerStateChange: false,
		dependsOnColumn:               dependsOnColumn,
		isMissing:                     false,
		isKeyword:                     isKeyword,
		firstLeaf:                     firstLeaf{symbol: 0, parseState: 0},
	}
	return subtree{data}
}

// setSymbol is ts_subtree_set_symbol.
func (s subtree) setSymbol(symbol Symbol, language *Language) {
	metadata := language.symbolMetadata(symbol)
	if s.ptr.isInline {
		assert(symbol < math.MaxUint8)
	}
	s.ptr.symbol = symbol
	s.ptr.named = metadata.Named
	s.ptr.visible = metadata.Visible
}

// newError is ts_subtree_new_error.
func newError(
	pool *subtreePool, lookaheadChar int32, padding, size length,
	bytesScanned uint32, parseState StateID, language *Language,
) subtree {
	result := newLeaf(
		pool, builtinSymError, padding, size, bytesScanned,
		parseState, false, false, false, language,
	)
	data := result.ptr
	data.fragileLeft = true
	data.fragileRight = true
	data.lookaheadChar = lookaheadChar
	return result
}

// clone is ts_subtree_clone. The C function allocates with malloc, and the
// Go function takes the node and the children from the chunks of pool.
//
// Clone a subtree.
func (s subtree) clone(pool *subtreePool) subtree {
	result := pool.allocateNode()
	result.heapFields = s.ptr.heapFields
	if len(s.ptr.children) > 0 {
		result.children = pool.allocateChildren(len(s.ptr.children))
		copy(result.children, s.ptr.children)
		for _, child := range result.children {
			child.retain()
		}
	} else if s.ptr.hasExternalTokens {
		result.externalScannerState = s.ptr.externalScannerState.copy()
	}
	result.refCount.Store(1)
	return subtree{result}
}

// makeMut is ts_subtree_make_mut. C returns a copy of an inline subtree,
// because it is a value, so Go copies an inline node. C needs no memory for
// the copy, and Go takes a node from the pool as newLeaf does.
//
// Get mutable version of a subtree.
//
// This takes ownership of the subtree. If the subtree has only one owner,
// this will directly convert it into a mutable version. Otherwise, it will
// perform a copy.
func (s subtree) makeMut(pool *subtreePool) subtree {
	if s.ptr.isInline {
		data := pool.allocate()
		data.heapFields = s.ptr.heapFields
		data.refCount.Store(1)
		return subtree{data}
	}
	if s.ptr.refCount.Load() == 1 {
		return s
	}
	result := s.clone(pool)
	s.release(pool)
	return result
}

// compress is ts_subtree_compress.
func (s subtree) compress(count uint32, language *Language, stack *subtreeArray) {
	initialStackSize := len(*stack)

	tree := s
	symbol := tree.ptr.symbol
	for range count {
		if tree.ptr.refCount.Load() > 1 || len(tree.ptr.children) < 2 {
			break
		}

		child := tree.ptr.children[0]
		if child.ptr.isInline ||
			len(child.ptr.children) < 2 ||
			child.ptr.refCount.Load() > 1 ||
			child.ptr.symbol != symbol {
			break
		}

		grandchild := child.ptr.children[0]
		if grandchild.ptr.isInline ||
			len(grandchild.ptr.children) < 2 ||
			grandchild.ptr.refCount.Load() > 1 ||
			grandchild.ptr.symbol != symbol {
			break
		}

		tree.ptr.children[0] = grandchild
		child.ptr.children[0] = grandchild.ptr.children[len(grandchild.ptr.children)-1]
		grandchild.ptr.children[len(grandchild.ptr.children)-1] = child
		*stack = append(*stack, tree)
		tree = grandchild
	}

	for len(*stack) > initialStackSize {
		tree = (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		child := tree.ptr.children[0]
		grandchild := child.ptr.children[len(child.ptr.children)-1]
		grandchild.summarizeChildren(language)
		child.summarizeChildren(language)
		tree.summarizeChildren(language)
	}
}

// errorExtentCost is ts_subtree__error_extent_cost.
//
// The part of an error node's cost that penalizes the extent it spans, as
// opposed to the cost of its contents.
func errorExtentCost(size length) uint32 {
	return errorCostPerRecovery +
		errorCostPerSkippedChar*size.bytes +
		errorCostPerSkippedLine*size.extent.row
}

// summarizeChildren is ts_subtree_summarize_children.
//
// Assign all of the node's properties that depend on its children.
func (s subtree) summarizeChildren(language *Language) {
	assert(!s.ptr.isInline)

	s.ptr.namedChildCount = 0
	s.ptr.visibleChildCount = 0
	s.ptr.errorCost = 0
	s.ptr.repeatDepth = 0
	s.ptr.visibleDescendantCount = 0
	s.ptr.hasExternalTokens = false
	s.ptr.dependsOnColumn = false
	s.ptr.hasExternalScannerStateChange = false
	s.ptr.dynamicPrecedence = 0

	structuralIndex := uint32(0)
	aliasSequence := language.aliasSequence(uint32(s.ptr.productionID))
	lookaheadEndByte := uint32(0)

	children := s.ptr.children
	for i, child := range children {
		if s.ptr.size.extent.row == 0 &&
			child.dependsOnColumn() {
			s.ptr.dependsOnColumn = true
		}

		if child.hasExternalScannerStateChange() {
			s.ptr.hasExternalScannerStateChange = true
		}

		if i == 0 {
			s.ptr.padding = child.padding()
			s.ptr.size = child.size()
		} else {
			s.ptr.size = s.ptr.size.add(child.totalSize())
		}

		childLookaheadEndByte := s.ptr.padding.bytes +
			s.ptr.size.bytes +
			child.lookaheadBytes()
		if childLookaheadEndByte > lookaheadEndByte {
			lookaheadEndByte = childLookaheadEndByte
		}

		grandchildCount := child.childCount()
		if child.symbol() == builtinSymErrorRepeat {
			// Refund an `_ERROR` child's extent penalty, which this node re-charges
			// as part of its own extent below, so that the grouping is cost-neutral.
			extentCost := errorExtentCost(child.size())
			assert(child.errorCost() >= extentCost)
			s.ptr.errorCost += child.errorCost() - extentCost
		} else {
			s.ptr.errorCost += child.errorCost()
			if s.ptr.symbol == builtinSymError ||
				s.ptr.symbol == builtinSymErrorRepeat {
				if !child.extra() && !(child.isError() && grandchildCount == 0) {
					if child.visible() {
						s.ptr.errorCost += errorCostPerSkippedTree
					} else if grandchildCount > 0 {
						s.ptr.errorCost += errorCostPerSkippedTree * child.ptr.visibleChildCount
					}
				}
			}
		}

		s.ptr.dynamicPrecedence += child.dynamicPrecedence()
		s.ptr.visibleDescendantCount += child.visibleDescendantCount()

		switch {
		case !child.extra() &&
			child.symbol() != 0 &&
			aliasSequence != nil &&
			aliasSequence[structuralIndex] != 0:
			s.ptr.visibleDescendantCount++
			s.ptr.visibleChildCount++
			if language.symbolMetadata(Symbol(aliasSequence[structuralIndex])).Named {
				s.ptr.namedChildCount++
			}
		case child.visible():
			s.ptr.visibleDescendantCount++
			s.ptr.visibleChildCount++
			if child.named() {
				s.ptr.namedChildCount++
			}
		case grandchildCount > 0:
			s.ptr.visibleChildCount += child.ptr.visibleChildCount
			s.ptr.namedChildCount += child.ptr.namedChildCount
		}

		if child.hasExternalTokens() {
			s.ptr.hasExternalTokens = true
		}

		if child.isError() {
			s.ptr.fragileLeft = true
			s.ptr.fragileRight = true
			s.ptr.parseState = tsTreeStateNone
		}

		if !child.extra() {
			structuralIndex++
		}
	}

	s.ptr.lookaheadBytes = lookaheadEndByte - s.ptr.size.bytes - s.ptr.padding.bytes

	if s.ptr.symbol == builtinSymError ||
		s.ptr.symbol == builtinSymErrorRepeat {
		s.ptr.errorCost += errorExtentCost(s.ptr.size)
	}

	if len(children) > 0 {
		firstChild := children[0]
		lastChild := children[len(children)-1]

		s.ptr.firstLeaf.symbol = firstChild.leafSymbol()
		s.ptr.firstLeaf.parseState = firstChild.leafParseState()

		if firstChild.fragileLeft() {
			s.ptr.fragileLeft = true
		}
		if lastChild.fragileRight() {
			s.ptr.fragileRight = true
		}

		if len(children) >= 2 &&
			!s.ptr.visible &&
			!s.ptr.named &&
			firstChild.symbol() == s.ptr.symbol {
			if firstChild.repeatDepth() > lastChild.repeatDepth() {
				s.ptr.repeatDepth = uint16(firstChild.repeatDepth() + 1)
			} else {
				s.ptr.repeatDepth = uint16(lastChild.repeatDepth() + 1)
			}
		}
	}
}

// newNode is ts_subtree_new_node. The C function puts the node at the end of
// the memory of the array of children, and the Go function takes the node
// from pool.
//
// Create a new parent node with the given children.
//
// This takes ownership of the children array.
func newNode(
	pool *subtreePool,
	symbol Symbol,
	children subtreeArray,
	productionID uint32,
	language *Language,
) subtree {
	metadata := language.symbolMetadata(symbol)
	fragile := symbol == builtinSymError || symbol == builtinSymErrorRepeat

	data := pool.allocateNode()
	data.refCount.Store(1)
	data.heapFields = heapFields{
		symbol:                        symbol,
		children:                      slices.Clip(children),
		visible:                       metadata.Visible,
		named:                         metadata.Named,
		hasChanges:                    false,
		hasExternalScannerStateChange: false,
		fragileLeft:                   fragile,
		fragileRight:                  fragile,
		isKeyword:                     false,
		visibleDescendantCount:        0,
		productionID:                  uint16(productionID),
		firstLeaf:                     firstLeaf{symbol: 0, parseState: 0},
	}
	result := subtree{data}
	result.summarizeChildren(language)
	return result
}

// newErrorNode is ts_subtree_new_error_node.
//
// Create a new error node containing the given children.
//
// This node is treated as 'extra'. Its children are prevented from having
// any effect on the parse state.
func newErrorNode(
	pool *subtreePool,
	children subtreeArray,
	extra bool,
	language *Language,
) subtree {
	result := newNode(
		pool, builtinSymError, children, 0, language,
	)
	result.ptr.extra = extra
	return result
}

// newMissingLeaf is ts_subtree_new_missing_leaf.
//
// Create a new 'missing leaf' node.
//
// This node is treated as 'extra'. Its children are prevented from having
// any effect on the parse state.
func newMissingLeaf(
	pool *subtreePool,
	symbol Symbol,
	state StateID,
	padding length,
	lookaheadBytes uint32,
	language *Language,
) subtree {
	result := newLeaf(
		pool, symbol, padding, lengthZero(), lookaheadBytes,
		state, false, false, false, language,
	)
	result.ptr.isMissing = true
	return result
}

// retain is ts_subtree_retain.
func (s subtree) retain() {
	if s.ptr.isInline {
		return
	}
	assert(s.ptr.refCount.Load() > 0)
	s.ptr.refCount.Add(1)
	assert(s.ptr.refCount.Load() != 0)
}

// release is ts_subtree_release. When the count of a node reaches 0, it
// releases the children of the node. C then frees the node together with
// the array of its children, and Go leaves them to the garbage collector. A
// leaf whose count reaches 0 goes to the free list of pool, as in C.
func (s subtree) release(pool *subtreePool) {
	if s.ptr.isInline {
		return
	}
	pool.treeStack = pool.treeStack[:0]

	assert(s.ptr.refCount.Load() > 0)
	if s.ptr.refCount.Add(^uint32(0)) == 0 {
		pool.treeStack = append(pool.treeStack, s)
	}

	for len(pool.treeStack) > 0 {
		tree := pool.treeStack[len(pool.treeStack)-1]
		pool.treeStack = pool.treeStack[:len(pool.treeStack)-1]
		if len(tree.ptr.children) > 0 {
			for _, child := range tree.ptr.children {
				if child.ptr.isInline {
					continue
				}
				assert(child.ptr.refCount.Load() > 0)
				if child.ptr.refCount.Add(^uint32(0)) == 0 {
					pool.treeStack = append(pool.treeStack, child)
				}
			}
		} else {
			if tree.ptr.hasExternalTokens {
				tree.ptr.externalScannerState.delete()
			}
			pool.free(tree.ptr)
		}
	}
}

// compare is ts_subtree_compare.
func compare(left, right subtree, pool *subtreePool) int {
	pool.treeStack = append(pool.treeStack, left, right)

	for len(pool.treeStack) > 0 {
		right = pool.treeStack[len(pool.treeStack)-1]
		left = pool.treeStack[len(pool.treeStack)-2]
		pool.treeStack = pool.treeStack[:len(pool.treeStack)-2]

		result := 0
		switch {
		case left.symbol() < right.symbol():
			result = -1
		case right.symbol() < left.symbol():
			result = 1
		case left.childCount() < right.childCount():
			result = -1
		case right.childCount() < left.childCount():
			result = 1
		}
		if result != 0 {
			pool.treeStack = pool.treeStack[:0]
			return result
		}

		for i := left.childCount(); i > 0; i-- {
			leftChild := left.ptr.children[i-1]
			rightChild := right.ptr.children[i-1]
			pool.treeStack = append(pool.treeStack, leftChild, rightChild)
		}
	}

	return 0
}

// setHasChanges is ts_subtree_set_has_changes.
func (s subtree) setHasChanges() {
	s.ptr.hasChanges = true
}

// subtreeEdit is Edit, an edit in the coordinates of one subtree.
type subtreeEdit struct {
	start  length
	oldEnd length
	newEnd length
}

// editEntry is EditEntry of ts_subtree_edit.
type editEntry struct {
	tree *subtree
	edit subtreeEdit
}

// edit is ts_subtree_edit.
func (s subtree) edit(inputEdit InputEdit, pool *subtreePool) subtree {
	self := s
	stack := []editEntry{{
		tree: &self,
		edit: subtreeEdit{
			start:  length{uint32(inputEdit.StartByte), inputEdit.StartPoint.internal()},
			oldEnd: length{uint32(inputEdit.OldEndByte), inputEdit.OldEndPoint.internal()},
			newEnd: length{uint32(inputEdit.NewEndByte), inputEdit.NewEndPoint.internal()},
		},
	}}

	for len(stack) > 0 {
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		edit := entry.edit
		isNoop := edit.oldEnd.bytes == edit.start.bytes && edit.newEnd.bytes == edit.start.bytes
		isPureInsertion := edit.oldEnd.bytes == edit.start.bytes
		parentDependsOnColumn := entry.tree.dependsOnColumn()
		columnShifted := edit.newEnd.extent.column != edit.oldEnd.extent.column

		size := entry.tree.size()
		padding := entry.tree.padding()
		totalSize := padding.add(size)
		lookaheadBytes := entry.tree.lookaheadBytes()
		endByte := totalSize.bytes + lookaheadBytes
		if edit.start.bytes > endByte || (isNoop && edit.start.bytes == endByte) {
			continue
		}

		switch {
		// If the edit is entirely within the space before this subtree, then shift this
		// subtree over according to the edit without changing its size.
		case edit.oldEnd.bytes <= padding.bytes:
			padding = edit.newEnd.add(padding.sub(edit.oldEnd))

		// If the edit starts in the space before this subtree and extends into this subtree,
		// shrink the subtree's content to compensate for the change in the space before it.
		case edit.start.bytes < padding.bytes:
			size = size.saturatingSub(edit.oldEnd.sub(padding))
			padding = edit.newEnd

		// If the edit is within this subtree, resize the subtree to reflect the edit.
		case edit.start.bytes < totalSize.bytes ||
			(edit.start.bytes == totalSize.bytes && isPureInsertion):
			size = edit.newEnd.sub(padding).add(totalSize.saturatingSub(edit.oldEnd))
		}

		result := entry.tree.makeMut(pool)

		if result.ptr.isInline {
			if canInline(padding, size, lookaheadBytes) {
				result.ptr.padding = padding
				result.ptr.size = length{size.bytes, point{0, size.bytes}}
			} else {
				// The node that makeMut returned is a copy, so it becomes
				// the node that C allocates here.
				data := result.ptr
				data.refCount.Store(1)
				data.heapFields = heapFields{
					padding:           padding,
					size:              size,
					lookaheadBytes:    lookaheadBytes,
					errorCost:         0,
					symbol:            data.symbol,
					parseState:        data.parseState,
					visible:           data.visible,
					named:             data.named,
					extra:             data.extra,
					fragileLeft:       false,
					fragileRight:      false,
					hasChanges:        false,
					hasExternalTokens: false,
					dependsOnColumn:   false,
					isMissing:         data.isMissing,
					isKeyword:         data.isKeyword,
				}
			}
		} else {
			result.ptr.padding = padding
			result.ptr.size = size
		}

		result.setHasChanges()
		*entry.tree = result

		var childLeft length
		childRight := lengthZero()
		for i, n := uint32(0), entry.tree.childCount(); i < n; i++ {
			child := &entry.tree.ptr.children[i]
			childSize := child.totalSize()
			childLeft = childRight
			childRight = childLeft.add(childSize)

			// If this child ends before the edit, it is not affected.
			if childRight.bytes+child.lookaheadBytes() < edit.start.bytes {
				continue
			}

			// Keep editing child nodes until a node is reached that starts after the edit.
			// Also, if this node's validity depends on its column position, then continue
			// invalidating child nodes until reaching a line break.
			if ((childLeft.bytes > edit.oldEnd.bytes) ||
				(childLeft.bytes == edit.oldEnd.bytes && childSize.bytes > 0 && i > 0)) &&
				(!parentDependsOnColumn ||
					childLeft.extent.row > padding.extent.row) &&
				(!child.dependsOnColumn() ||
					!columnShifted ||
					childLeft.extent.row > edit.oldEnd.extent.row) {
				break
			}

			// Transform edit into the child's coordinate space.
			childEdit := subtreeEdit{
				start:  edit.start.saturatingSub(childLeft),
				oldEnd: edit.oldEnd.saturatingSub(childLeft),
				newEnd: edit.newEnd.saturatingSub(childLeft),
			}

			// Interpret all inserted text as applying to the *first* child that touches the edit.
			// Subsequent children never have any text inserted into them; they are only
			// shrunk to compensate for the edit.
			if childRight.bytes > edit.start.bytes ||
				(childRight.bytes == edit.start.bytes && isPureInsertion) {
				edit.newEnd = edit.start
			} else {
				// Children that occur before the edit are not reshaped by the edit.
				childEdit.oldEnd = childEdit.start
				childEdit.newEnd = childEdit.start
			}

			// Queue processing of this child's subtree.
			stack = append(stack, editEntry{
				tree: child,
				edit: childEdit,
			})
		}
	}

	return self
}

// lastExternalToken is ts_subtree_last_external_token.
func (s subtree) lastExternalToken() subtree {
	tree := s
	if !tree.hasExternalTokens() {
		return subtree{}
	}
	for len(tree.ptr.children) > 0 {
		for _, child := range slices.Backward(tree.ptr.children) {
			if child.hasExternalTokens() {
				tree = child
				break
			}
		}
	}
	return tree
}

// writeCharToString is ts_subtree__write_char_to_string.
func writeCharToString(b *strings.Builder, chr int32) {
	switch {
	case chr == -1:
		b.WriteString("INVALID")
	case chr == '\x00':
		b.WriteString(`'\0'`)
	case chr == '\n':
		b.WriteString(`'\n'`)
	case chr == '\t':
		b.WriteString(`'\t'`)
	case chr == '\r':
		b.WriteString(`'\r'`)
	case 0 < chr && chr < 128 && isprint(chr):
		fmt.Fprintf(b, "'%c'", chr)
	default:
		fmt.Fprintf(b, "%d", chr)
	}
}

// isprint is isprint of the C library in the C locale.
func isprint(chr int32) bool {
	return 0x20 <= chr && chr < 0x7f
}

// rootField is ROOT_FIELD.
const rootField = "__ROOT__"

// writeToStringFrame is WriteToStringFrame. An empty fieldName is NULL.
type writeToStringFrame struct {
	subtree              subtree
	aliasSymbol          Symbol
	aliasIsNamed         bool
	fieldName            string
	isRoot               bool
	preWritten           bool
	isVisible            bool
	childIndex           uint32
	structuralChildIndex uint32
	aliasSequence        []uint16
	fieldMap             []abi.FieldMapEntry
}

// writeToString is ts_subtree__write_to_string. C writes the string twice,
// once to find its length and once into a buffer of that length, and Go
// writes it once to a strings.Builder.
func (s subtree) writeToString(
	b *strings.Builder,
	language *Language, includeAll bool,
	rootAliasSymbol Symbol, rootAliasIsNamed bool, rootFieldName string,
) {
	stack := []writeToStringFrame{{
		subtree:      s,
		aliasSymbol:  rootAliasSymbol,
		aliasIsNamed: rootAliasIsNamed,
		fieldName:    rootFieldName,
		isRoot:       rootFieldName == rootField,
	}}

	for len(stack) > 0 {
		frame := &stack[len(stack)-1]
		node := frame.subtree

		if node.ptr == nil {
			if !frame.isRoot {
				b.WriteString(" ")
				if frame.fieldName != "" {
					fmt.Fprintf(b, "%s: ", frame.fieldName)
				}
			}
			b.WriteString("(NULL)")
			stack = stack[:len(stack)-1]
			continue
		}

		if !frame.preWritten {
			var isVisible bool
			switch {
			case includeAll || node.missing():
				isVisible = true
			case frame.aliasSymbol != 0:
				isVisible = frame.aliasIsNamed
			default:
				isVisible = node.visible() && node.named()
			}

			if isVisible {
				if !frame.isRoot {
					b.WriteString(" ")
					if frame.fieldName != "" {
						fmt.Fprintf(b, "%s: ", frame.fieldName)
					}
				}

				if node.isError() && node.childCount() == 0 && node.ptr.size.bytes > 0 {
					b.WriteString("(UNEXPECTED ")
					writeCharToString(b, node.ptr.lookaheadChar)
				} else {
					symbol := frame.aliasSymbol
					if symbol == 0 {
						symbol = node.symbol()
					}
					symbolName := language.SymbolName(symbol)
					if node.missing() {
						b.WriteString("(MISSING ")
						if frame.aliasIsNamed || node.named() {
							b.WriteString(symbolName)
						} else {
							fmt.Fprintf(b, "\"%s\"", symbolName)
						}
					} else {
						fmt.Fprintf(b, "(%s", symbolName)
					}
				}
			} else if frame.isRoot {
				symbol := frame.aliasSymbol
				if symbol == 0 {
					symbol = node.symbol()
				}
				symbolName := language.SymbolName(symbol)
				switch {
				case node.childCount() > 0:
					fmt.Fprintf(b, "(%s", symbolName)
				case node.named():
					fmt.Fprintf(b, "(%s)", symbolName)
				default:
					fmt.Fprintf(b, "(\"%s\")", symbolName)
				}
			}

			if node.childCount() != 0 {
				frame.aliasSequence = language.aliasSequence(uint32(node.ptr.productionID))
				frame.fieldMap = language.fieldMap(uint32(node.ptr.productionID))
			}

			frame.isVisible = isVisible
			frame.preWritten = true
		}

		if frame.childIndex < node.childCount() {
			child := node.ptr.children[frame.childIndex]
			childFrame := writeToStringFrame{
				subtree: child,
				isRoot:  false,
			}

			if child.extra() {
				// Extra children carry no alias/field info.
			} else {
				var subtreeAliasSymbol Symbol
				if frame.aliasSequence != nil {
					subtreeAliasSymbol = Symbol(frame.aliasSequence[frame.structuralChildIndex])
				}
				subtreeAliasIsNamed := false
				if subtreeAliasSymbol != 0 {
					subtreeAliasIsNamed = language.symbolMetadata(subtreeAliasSymbol).Named
				}

				childFieldName := frame.fieldName
				if frame.isVisible {
					childFieldName = ""
				}
				for _, m := range frame.fieldMap {
					if !m.Inherited && uint32(m.ChildIndex) == frame.structuralChildIndex {
						childFieldName = language.tables.FieldNames[m.FieldID]
						break
					}
				}

				childFrame.aliasSymbol = subtreeAliasSymbol
				childFrame.aliasIsNamed = subtreeAliasIsNamed
				childFrame.fieldName = childFieldName
				frame.structuralChildIndex++
			}

			frame.childIndex++
			// After this push, `frame` may be invalidated by a realloc.
			stack = append(stack, childFrame)
			continue
		}

		if frame.isVisible {
			b.WriteString(")")
		}
		stack = stack[:len(stack)-1]
	}
}

// string is ts_subtree_string.
func (s subtree) string(
	aliasSymbol Symbol,
	aliasIsNamed bool,
	language *Language,
	includeAll bool,
) string {
	var b strings.Builder
	s.writeToString(&b, language, includeAll, aliasSymbol, aliasIsNamed, rootField)
	return b.String()
}

// printDotGraphNode is ts_subtree__print_dot_graph. The name of a node in
// the graph is the address of the subtree, as in C.
func (s *subtree) printDotGraphNode(startOffset uint32, language *Language, aliasSymbol Symbol, w io.Writer) {
	subtreeSymbol := s.symbol()
	symbol := aliasSymbol
	if symbol == 0 {
		symbol = subtreeSymbol
	}
	endOffset := startOffset + s.totalBytes()
	fprintf(w, "tree_%p [label=\"", s)
	language.writeSymbolAsDotString(w, symbol)
	fprintf(w, "\"")

	if s.childCount() == 0 {
		fprintf(w, ", shape=plaintext")
	}
	if s.extra() {
		fprintf(w, ", fontcolor=gray")
	}
	if s.hasChanges() {
		fprintf(w, ", color=green, penwidth=2")
	}

	fprintf(w, ", tooltip=\""+
		"range: %d - %d\n"+
		"state: %d\n"+
		"error-cost: %d\n"+
		"has-changes: %d\n"+
		"depends-on-column: %d\n"+
		"descendant-count: %d\n"+
		"repeat-depth: %d\n"+
		"lookahead-bytes: %d",
		startOffset, endOffset,
		s.parseState(),
		s.errorCost(),
		boolInt(s.hasChanges()),
		boolInt(s.dependsOnColumn()),
		s.visibleDescendantCount(),
		s.repeatDepth(),
		s.lookaheadBytes(),
	)

	if s.isError() && s.childCount() == 0 && s.ptr.lookaheadChar != 0 {
		fprintf(w, "\ncharacter: '%s'", []byte{byte(s.ptr.lookaheadChar)})
	}

	fprintf(w, "\"]\n")

	childStartOffset := startOffset
	childInfoOffset := uint32(language.tables.MaxAliasSequenceLength) *
		uint32(s.productionID())
	for i, n := uint32(0), s.childCount(); i < n; i++ {
		child := &s.ptr.children[i]
		subtreeAliasSymbol := Symbol(0)
		if !child.extra() && childInfoOffset != 0 {
			subtreeAliasSymbol = Symbol(language.tables.AliasSequences[childInfoOffset])
			childInfoOffset++
		}
		child.printDotGraphNode(childStartOffset, language, subtreeAliasSymbol, w)
		fprintf(w, "tree_%p -> tree_%p [tooltip=%d]\n", s, child, i)
		childStartOffset += child.totalBytes()
	}
}

// printDotGraph is ts_subtree_print_dot_graph.
func (s subtree) printDotGraph(language *Language, w io.Writer) {
	fprintf(w, "digraph tree {\n")
	fprintf(w, "edge [arrowhead=none]\n")
	s.printDotGraphNode(0, language, 0, w)
	fprintf(w, "}\n")
}

// emptyExternalScannerState is empty_state of
// ts_subtree_external_scanner_state. Nothing changes it.
var emptyExternalScannerState externalScannerState

// getExternalScannerState is ts_subtree_external_scanner_state.
func (s subtree) getExternalScannerState() *externalScannerState {
	if s.ptr != nil &&
		!s.ptr.isInline &&
		s.ptr.hasExternalTokens &&
		len(s.ptr.children) == 0 {
		return &s.ptr.externalScannerState
	}
	return &emptyExternalScannerState
}

// externalScannerStateEq is ts_subtree_external_scanner_state_eq.
func (s subtree) externalScannerStateEq(other subtree) bool {
	stateSelf := s.getExternalScannerState()
	stateOther := other.getExternalScannerState()
	return stateSelf.eq(stateOther.data())
}

// boolInt returns 1 for true and 0 for false, as C prints a bool with %u.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// fprintf writes to w as fprintf of C does. A write error is dropped, as
// upstream drops the error of fprintf.
func fprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
