package transit

import (
	"io"
	"math"
	"os"
	"slices"
	"strings"
)

// This file ports lib/src/stack.c and lib/src/stack.h.
//
// The stack is a graph of stack nodes. Each version of the stack is a head
// that points to a node, and each node links to the nodes before it. Two
// versions that merge share their nodes.
//
// The stack keeps the free list of stack nodes of C, node_pool, with up to
// MAX_NODE_POOL_SIZE nodes. When the reference count of a node reaches 0, C
// puts the node on the free list, and stack_node_new takes a node from it.
// So a parse after an edit does not allocate a node for each push. When the
// count of a node reaches 0, C also releases the subtrees of its links, and
// the counts of the subtrees decide what ts_subtree_make_mut,
// ts_subtree_compress and the balancing of the parser do (D64). So Go
// releases them at the same time. The garbage collector frees a node that
// does not fit on the free list, so ts_stack_delete and the macro
// forceinline have no Go form.

// maxLinkCount is MAX_LINK_COUNT.
const maxLinkCount = 8

// maxNodePoolSize is MAX_NODE_POOL_SIZE.
const maxNodePoolSize = 50

// maxIteratorCount is MAX_ITERATOR_COUNT.
const maxIteratorCount = 64

// stackVersion is StackVersion, the index of a version of the stack.
type stackVersion uint32

// stackVersionNone is STACK_VERSION_NONE.
const stackVersionNone = stackVersion(math.MaxUint32)

// stackSlice is StackSlice: the subtrees that a pop removed from a version,
// and the version that the pop revealed.
type stackSlice struct {
	subtrees subtreeArray
	version  stackVersion
}

// stackSliceArray is StackSliceArray.
type stackSliceArray []stackSlice

// stackSummaryEntry is StackSummaryEntry.
type stackSummaryEntry struct {
	position length
	depth    uint32
	state    StateID
}

// stackSummary is StackSummary.
type stackSummary []stackSummaryEntry

// stackLink is StackLink, a link from a node to the node before it.
type stackLink struct {
	node      *stackNode
	subtree   subtree
	isPending bool
}

// stackNode is StackNode.
type stackNode struct {
	state             StateID
	position          length
	links             [maxLinkCount]stackLink
	linkCount         uint16
	refCount          uint32
	errorCost         uint32
	nodeCount         uint32
	dynamicPrecedence int
}

// stackIterator is StackIterator.
type stackIterator struct {
	node         *stackNode
	subtrees     subtreeArray
	subtreeCount uint32
	isPending    bool
}

// stackNodeArray is StackNodeArray, the type of the free list.
type stackNodeArray []*stackNode

// stackStatus is StackStatus.
type stackStatus int

// The states of a version of the stack.
const (
	// stackStatusActive is StackStatusActive.
	stackStatusActive stackStatus = iota
	// stackStatusPaused is StackStatusPaused.
	stackStatusPaused
	// stackStatusHalted is StackStatusHalted.
	stackStatusHalted
)

// String returns the name of the status.
func (s stackStatus) String() string {
	switch s {
	case stackStatusActive:
		return "active"
	case stackStatusPaused:
		return "paused"
	case stackStatusHalted:
		return "halted"
	}
	return unknownName
}

// stackHead is StackHead, a version of the stack.
type stackHead struct {
	node                 *stackNode
	summary              *stackSummary
	nodeCountAtLastError uint32
	lastExternalToken    subtree
	lookaheadWhenPaused  subtree
	status               stackStatus
}

// stack is Stack. The arrays slices and iterators are kept between calls,
// as in C, so that a pop does not allocate them again.
type stack struct {
	heads       []stackHead
	slices      stackSliceArray
	iterators   []stackIterator
	nodePool    stackNodeArray
	baseNode    *stackNode
	subtreePool *subtreePool
}

// stackAction is StackAction, a set of flags that a stackCallback returns.
type stackAction uint32

// The flags of a stackAction.
const (
	// stackActionNone is StackActionNone.
	stackActionNone stackAction = 0
	// stackActionStop is StackActionStop.
	stackActionStop stackAction = 1
	// stackActionPop is StackActionPop.
	stackActionPop stackAction = 2
)

// String returns the names of the flags of the action, joined with "|".
func (a stackAction) String() string {
	if a == stackActionNone {
		return noneName
	}
	var names []string
	if a&stackActionStop != 0 {
		names = append(names, "stop")
	}
	if a&stackActionPop != 0 {
		names = append(names, "pop")
	}
	if a&^(stackActionStop|stackActionPop) != 0 {
		names = append(names, unknownName)
	}
	return strings.Join(names, "|")
}

// stackCallback is StackCallback. The payload of C is in the closure.
type stackCallback func(iterator *stackIterator) stackAction

// retain is stack_node_retain.
func (n *stackNode) retain() {
	if n == nil {
		return
	}
	assert(n.refCount > 0)
	n.refCount++
	assert(n.refCount != 0)
}

// release is stack_node_release. When the count of a node reaches 0, it
// releases the links of the node, and it puts the node on the free list
// when the list has room, as C does. C frees a node that does not fit, and
// Go leaves it to the garbage collector. Go also clears a node that goes on
// the free list, so that the list does not keep the subtrees and the nodes
// of its links alive. The goto of C is the loop.
func (n *stackNode) release(pool *stackNodeArray, subtreePool *subtreePool) {
	for node := n; node != nil; {
		assert(node.refCount != 0)
		node.refCount--
		if node.refCount > 0 {
			return
		}

		var firstPredecessor *stackNode
		if node.linkCount > 0 {
			for i := uint32(node.linkCount) - 1; i > 0; i-- {
				link := node.links[i]
				if link.subtree.ptr != nil {
					link.subtree.release(subtreePool)
				}
				link.node.release(pool, subtreePool)
			}
			link := node.links[0]
			if link.subtree.ptr != nil {
				link.subtree.release(subtreePool)
			}
			firstPredecessor = node.links[0].node
		}

		if len(*pool) < maxNodePoolSize {
			*node = stackNode{}
			*pool = append(*pool, node)
		}

		node = firstPredecessor
	}
}

// stackSubtreeNodeCount is stack__subtree_node_count.
//
// Get the number of nodes in the subtree, for the purpose of measuring
// how much progress has been made by a given version of the stack.
func stackSubtreeNodeCount(subtree subtree) uint32 {
	count := subtree.visibleDescendantCount()
	if subtree.visible() {
		count++
	}

	// Count intermediate error nodes even though they are not visible,
	// because a stack version's node count is used to check whether it
	// has made any progress since the last time it encountered an error.
	if subtree.symbol() == builtinSymErrorRepeat {
		count++
	}

	return count
}

// newStackNode is stack_node_new. It takes the node from the free list
// when the list is not empty, and it allocates the node when the list is
// empty.
func newStackNode(
	previousNode *stackNode,
	subtree subtree,
	isPending bool,
	state StateID,
	pool *stackNodeArray,
) *stackNode {
	var node *stackNode
	if n := len(*pool); n > 0 {
		node = (*pool)[n-1]
		*pool = (*pool)[:n-1]
	} else {
		node = new(stackNode)
	}
	*node = stackNode{
		refCount:  1,
		linkCount: 0,
		state:     state,
	}

	if previousNode != nil {
		node.linkCount = 1
		node.links[0] = stackLink{
			node:      previousNode,
			subtree:   subtree,
			isPending: isPending,
		}

		node.position = previousNode.position
		node.errorCost = previousNode.errorCost
		node.dynamicPrecedence = previousNode.dynamicPrecedence
		node.nodeCount = previousNode.nodeCount

		if subtree.ptr != nil {
			node.errorCost += subtree.errorCost()
			node.position = node.position.add(subtree.totalSize())
			node.nodeCount += stackSubtreeNodeCount(subtree)
			node.dynamicPrecedence += int(subtree.dynamicPrecedence())
		}
	} else {
		node.position = lengthZero()
		node.errorCost = 0
	}

	return node
}

// stackSubtreeIsEquivalent is stack__subtree_is_equivalent. C compares the
// bits of two inline leaves as one pointer, and Go compares two nodes. Two
// inline leaves with the same bits have the same fields, so the checks after
// the first one return true for them, as C does.
func stackSubtreeIsEquivalent(left, right subtree) bool {
	if left.ptr == right.ptr {
		return true
	}
	if left.ptr == nil || right.ptr == nil {
		return false
	}

	// Symbols must match
	if left.symbol() != right.symbol() {
		return false
	}

	// If both have errors, don't bother keeping both.
	if left.errorCost() > 0 && right.errorCost() > 0 {
		return true
	}

	return left.padding().bytes == right.padding().bytes &&
		left.size().bytes == right.size().bytes &&
		left.childCount() == right.childCount() &&
		left.extra() == right.extra() &&
		left.externalScannerStateEq(right)
}

// addLink is stack_node_add_link.
func (n *stackNode) addLink(link stackLink, subtreePool *subtreePool) {
	if link.node == n {
		return
	}

	for i := range int(n.linkCount) {
		existingLink := &n.links[i]
		if stackSubtreeIsEquivalent(existingLink.subtree, link.subtree) {
			// In general, we preserve ambiguities until they are removed from the stack
			// during a pop operation where multiple paths lead to the same node. But in
			// the special case where two links directly connect the same pair of nodes,
			// we can safely remove the ambiguity ahead of time without changing behavior.
			if existingLink.node == link.node {
				if link.subtree.dynamicPrecedence() >
					existingLink.subtree.dynamicPrecedence() {
					link.subtree.retain()
					existingLink.subtree.release(subtreePool)
					existingLink.subtree = link.subtree
					n.dynamicPrecedence =
						link.node.dynamicPrecedence + int(link.subtree.dynamicPrecedence())
				}
				return
			}

			// If the previous nodes are mergeable, merge them recursively.
			if existingLink.node.state == link.node.state &&
				existingLink.node.position.bytes == link.node.position.bytes &&
				existingLink.node.errorCost == link.node.errorCost {
				for j := range int(link.node.linkCount) {
					existingLink.node.addLink(link.node.links[j], subtreePool)
				}
				dynamicPrecedence := int32(link.node.dynamicPrecedence)
				if link.subtree.ptr != nil {
					dynamicPrecedence += link.subtree.dynamicPrecedence()
				}
				if int(dynamicPrecedence) > n.dynamicPrecedence {
					n.dynamicPrecedence = int(dynamicPrecedence)
				}
				return
			}
		}
	}

	if n.linkCount == maxLinkCount {
		return
	}

	link.node.retain()
	nodeCount := link.node.nodeCount
	dynamicPrecedence := link.node.dynamicPrecedence
	n.links[n.linkCount] = link
	n.linkCount++

	if link.subtree.ptr != nil {
		link.subtree.retain()
		nodeCount += stackSubtreeNodeCount(link.subtree)
		dynamicPrecedence += int(link.subtree.dynamicPrecedence())
	}

	if nodeCount > n.nodeCount {
		n.nodeCount = nodeCount
	}
	if dynamicPrecedence > n.dynamicPrecedence {
		n.dynamicPrecedence = dynamicPrecedence
	}
}

// delete is stack_head_delete. It releases what the head holds. C also
// frees the summary, and Go leaves it to the garbage collector.
func (h *stackHead) delete(pool *stackNodeArray, subtreePool *subtreePool) {
	if h.node != nil {
		if h.lastExternalToken.ptr != nil {
			h.lastExternalToken.release(subtreePool)
		}
		if h.lookaheadWhenPaused.ptr != nil {
			h.lookaheadWhenPaused.release(subtreePool)
		}
		h.node.release(pool, subtreePool)
	}
}

// addVersion is ts_stack__add_version.
func (s *stack) addVersion(
	originalVersion stackVersion,
	node *stackNode,
) stackVersion {
	head := stackHead{
		node:                 node,
		nodeCountAtLastError: s.heads[originalVersion].nodeCountAtLastError,
		lastExternalToken:    s.heads[originalVersion].lastExternalToken,
		status:               stackStatusActive,
		lookaheadWhenPaused:  subtree{},
	}
	s.heads = append(s.heads, head)
	node.retain()
	if head.lastExternalToken.ptr != nil {
		head.lastExternalToken.retain()
	}
	return stackVersion(len(s.heads) - 1)
}

// addSlice is ts_stack__add_slice.
func (s *stack) addSlice(
	originalVersion stackVersion,
	node *stackNode,
	subtrees subtreeArray,
) {
	for i := uint32(len(s.slices)) - 1; i+1 > 0; i-- {
		version := s.slices[i].version
		if s.heads[version].node == node {
			slice := stackSlice{subtrees, version}
			s.slices = slices.Insert(s.slices, int(i)+1, slice)
			return
		}
	}

	version := s.addVersion(originalVersion, node)
	slice := stackSlice{subtrees, version}
	s.slices = append(s.slices, slice)
}

// iter is stack__iter. It returns the array slices of the stack, which the
// next call reuses.
//
// C reserves room in the first array of subtrees for goal_subtree_count
// subtrees and a node of ts_subtree_new_node, so the array becomes the
// memory of the new node. The Go node comes from the pool, so Go reserves one
// more slot than the goal. Go takes the array from the chunks of the pool,
// as it takes the children of a node (D62), so that a pop does not allocate
// it. The capacity is above 0, as in C, so ts_subtree_array_copy copies the
// array.
func (s *stack) iter(
	version stackVersion,
	callback stackCallback,
	goalSubtreeCount int,
) stackSliceArray {
	s.slices = s.slices[:0]
	s.iterators = s.iterators[:0]

	head := &s.heads[version]
	newIterator := stackIterator{
		node:         head.node,
		subtrees:     nil,
		subtreeCount: 0,
		isPending:    true,
	}

	includeSubtrees := false
	if goalSubtreeCount >= 0 {
		includeSubtrees = true
		newIterator.subtrees = s.subtreePool.allocateChildren(goalSubtreeCount + 1)[:0]
	}

	s.iterators = append(s.iterators, newIterator)

	for len(s.iterators) > 0 {
		for i, size := uint32(0), uint32(len(s.iterators)); i < size; i++ {
			iterator := &s.iterators[i]
			node := iterator.node

			action := callback(iterator)
			shouldPop := action&stackActionPop != 0
			shouldStop := action&stackActionStop != 0 || node.linkCount == 0

			if shouldPop {
				subtrees := iterator.subtrees
				if !shouldStop {
					subtrees = subtrees.copy(s.subtreePool)
				}
				subtrees.reverse()
				s.addSlice(
					version,
					node,
					subtrees,
				)
			}

			if shouldStop {
				if !shouldPop {
					iterator.subtrees.delete(s.subtreePool)
				}
				s.iterators = slices.Delete(s.iterators, int(i), int(i)+1)
				i--
				size--
				continue
			}

			for j := uint32(1); j <= uint32(node.linkCount); j++ {
				var nextIterator *stackIterator
				var link stackLink
				if j == uint32(node.linkCount) {
					link = node.links[0]
					nextIterator = &s.iterators[i]
				} else {
					if len(s.iterators) >= maxIteratorCount {
						continue
					}
					link = node.links[j]
					currentIterator := s.iterators[i]
					s.iterators = append(s.iterators, currentIterator)
					nextIterator = &s.iterators[len(s.iterators)-1]
					nextIterator.subtrees = nextIterator.subtrees.copy(s.subtreePool)
				}

				nextIterator.node = link.node
				if link.subtree.ptr != nil {
					if includeSubtrees {
						nextIterator.subtrees = append(nextIterator.subtrees, link.subtree)
						link.subtree.retain()
					}

					if !link.subtree.extra() {
						nextIterator.subtreeCount++
						if !link.isPending {
							nextIterator.isPending = false
						}
					}
				} else {
					nextIterator.subtreeCount++
					nextIterator.isPending = false
				}
			}
		}
	}

	return s.slices
}

// newStack is ts_stack_new. C reserves room in the arrays heads, slices
// and iterators, and Go lets append grow them. It reserves room for the
// free list, as C does.
//
// Create a stack.
func newStack(subtreePool *subtreePool) *stack {
	s := &stack{}
	s.nodePool = make(stackNodeArray, 0, maxNodePoolSize)

	s.subtreePool = subtreePool
	s.baseNode = newStackNode(nil, subtree{}, false, 1, &s.nodePool)
	s.clear()

	return s
}

// versionCount is ts_stack_version_count.
//
// Get the stack's current number of versions.
func (s *stack) versionCount() uint32 {
	return uint32(len(s.heads))
}

// haltedVersionCount is ts_stack_halted_version_count.
//
// Get the stack's current number of halted versions.
func (s *stack) haltedVersionCount() uint32 {
	count := uint32(0)
	for i := range s.heads {
		head := &s.heads[i]
		if head.status == stackStatusHalted {
			count++
		}
	}
	return count
}

// state is ts_stack_state.
//
// Get the state at the top of the given version of the stack. If the stack is
// empty, this returns the initial state, 1.
func (s *stack) state(version stackVersion) StateID {
	return s.heads[version].node.state
}

// position is ts_stack_position.
//
// Get the position of the given version of the stack within the document.
func (s *stack) position(version stackVersion) length {
	return s.heads[version].node.position
}

// lastExternalToken is ts_stack_last_external_token.
//
// Get the last external token associated with a given version of the stack.
func (s *stack) lastExternalToken(version stackVersion) subtree {
	return s.heads[version].lastExternalToken
}

// setLastExternalToken is ts_stack_set_last_external_token.
//
// Set the last external token associated with a given version of the stack.
func (s *stack) setLastExternalToken(version stackVersion, token subtree) {
	head := &s.heads[version]
	if token.ptr != nil {
		token.retain()
	}
	if head.lastExternalToken.ptr != nil {
		head.lastExternalToken.release(s.subtreePool)
	}
	head.lastExternalToken = token
}

// errorCost is ts_stack_error_cost.
//
// Get the total cost of all errors on the given version of the stack.
func (s *stack) errorCost(version stackVersion) uint32 {
	head := &s.heads[version]
	result := head.node.errorCost
	if head.status == stackStatusPaused ||
		(head.node.state == errorState && head.node.links[0].subtree.ptr == nil) {
		result += errorCostPerRecovery
	}
	return result
}

// nodeCountSinceError is ts_stack_node_count_since_error.
//
// Get the maximum number of tree nodes reachable from this version of the stack
// since the last error was detected.
func (s *stack) nodeCountSinceError(version stackVersion) uint32 {
	head := &s.heads[version]
	if head.node.nodeCount < head.nodeCountAtLastError {
		head.nodeCountAtLastError = head.node.nodeCount
	}
	return head.node.nodeCount - head.nodeCountAtLastError
}

// push is ts_stack_push.
//
// Push a tree and state onto the given version of the stack.
//
// This transfers ownership of the tree to the Stack. Callers that
// need to retain ownership of the tree for their own purposes should
// first retain the tree.
func (s *stack) push(
	version stackVersion,
	subtree subtree,
	pending bool,
	state StateID,
) {
	head := &s.heads[version]
	newNode := newStackNode(head.node, subtree, pending, state, &s.nodePool)
	if subtree.ptr == nil {
		head.nodeCountAtLastError = newNode.nodeCount
	}
	head.node = newNode
}

// popCountCallback is pop_count_callback.
func popCountCallback(goalSubtreeCount *uint32, iterator *stackIterator) stackAction {
	if iterator.subtreeCount == *goalSubtreeCount {
		return stackActionPop | stackActionStop
	}
	return stackActionNone
}

// popCount is ts_stack_pop_count. It returns the array slices of the stack,
// which the next pop reuses. C converts count to an int, so a count above
// math.MaxInt32 is negative, and Go converts it the same way.
//
// Pop the given number of entries from the given version of the stack. This
// operation can increase the number of stack versions by revealing multiple
// versions which had previously been merged. It returns an array that
// specifies the index of each revealed version and the trees that were
// removed from that version.
func (s *stack) popCount(version stackVersion, count uint32) stackSliceArray {
	return s.iter(version, func(iterator *stackIterator) stackAction {
		return popCountCallback(&count, iterator)
	}, int(int32(count)))
}

// popPendingCallback is pop_pending_callback.
func popPendingCallback(iterator *stackIterator) stackAction {
	if iterator.subtreeCount >= 1 {
		if iterator.isPending {
			return stackActionPop | stackActionStop
		}
		return stackActionStop
	}
	return stackActionNone
}

// popPending is ts_stack_pop_pending. It returns the array slices of the
// stack, which the next pop reuses.
//
// Remove any pending trees from the top of the given version of the stack.
func (s *stack) popPending(version stackVersion) stackSliceArray {
	pop := s.iter(version, popPendingCallback, 0)
	if len(pop) > 0 {
		s.renumberVersion(pop[0].version, version)
		pop[0].version = version
	}
	return pop
}

// popErrorCallback is pop_error_callback.
func popErrorCallback(foundError *bool, iterator *stackIterator) stackAction {
	if len(iterator.subtrees) > 0 {
		if !*foundError && iterator.subtrees[0].isError() {
			*foundError = true
			return stackActionPop | stackActionStop
		}
		return stackActionStop
	}
	return stackActionNone
}

// popError is ts_stack_pop_error. It returns nil when there is no error.
//
// Remove an error at the top of the given version of the stack.
func (s *stack) popError(version stackVersion) subtreeArray {
	node := s.heads[version].node
	for i := range uint32(node.linkCount) {
		if node.links[i].subtree.ptr != nil && node.links[i].subtree.isError() {
			foundError := false
			pop := s.iter(version, func(iterator *stackIterator) stackAction {
				return popErrorCallback(&foundError, iterator)
			}, 1)
			if len(pop) > 0 {
				assert(len(pop) == 1)
				s.renumberVersion(pop[0].version, version)
				return pop[0].subtrees
			}
			break
		}
	}
	return nil
}

// popAllCallback is pop_all_callback.
func popAllCallback(iterator *stackIterator) stackAction {
	if iterator.node.linkCount == 0 {
		return stackActionPop
	}
	return stackActionNone
}

// popAll is ts_stack_pop_all. It returns the array slices of the stack,
// which the next pop reuses.
//
// Remove all trees from the given version of the stack.
func (s *stack) popAll(version stackVersion) stackSliceArray {
	return s.iter(version, popAllCallback, 0)
}

// summarizeStackSession is SummarizeStackSession.
type summarizeStackSession struct {
	summary  *stackSummary
	maxDepth uint32
}

// summarizeStackCallback is summarize_stack_callback.
func summarizeStackCallback(session *summarizeStackSession, iterator *stackIterator) stackAction {
	state := iterator.node.state
	depth := iterator.subtreeCount
	if depth > session.maxDepth {
		return stackActionStop
	}
	for i := uint32(len(*session.summary)) - 1; i+1 > 0; i-- {
		entry := (*session.summary)[i]
		if entry.depth < depth {
			break
		}
		if entry.depth == depth && entry.state == state {
			return stackActionNone
		}
	}
	*session.summary = append(*session.summary, stackSummaryEntry{
		position: iterator.node.position,
		depth:    depth,
		state:    state,
	})
	return stackActionNone
}

// recordSummary is ts_stack_record_summary.
//
// Compute a summary of all the parse states near the top of the given
// version of the stack and store the summary for later retrieval.
func (s *stack) recordSummary(version stackVersion, maxDepth uint32) {
	session := summarizeStackSession{
		summary:  &stackSummary{},
		maxDepth: maxDepth,
	}
	s.iter(version, func(iterator *stackIterator) stackAction {
		return summarizeStackCallback(&session, iterator)
	}, -1)
	head := &s.heads[version]
	head.summary = session.summary
}

// getSummary is ts_stack_get_summary. It returns nil when the version has
// no summary.
//
// Retrieve a summary of all the parse states near the top of the
// given version of the stack.
func (s *stack) getSummary(version stackVersion) *stackSummary {
	return s.heads[version].summary
}

// dynamicPrecedence is ts_stack_dynamic_precedence.
func (s *stack) dynamicPrecedence(version stackVersion) int {
	return s.heads[version].node.dynamicPrecedence
}

// hasAdvancedSinceError is ts_stack_has_advanced_since_error.
func (s *stack) hasAdvancedSinceError(version stackVersion) bool {
	head := &s.heads[version]
	node := head.node
	if node.errorCost == 0 {
		return true
	}
	for node != nil {
		if node.linkCount > 0 {
			subtree := node.links[0].subtree
			if subtree.ptr != nil {
				if subtree.totalBytes() > 0 {
					return true
				} else if node.nodeCount > head.nodeCountAtLastError &&
					subtree.errorCost() == 0 {
					node = node.links[0].node
					continue
				}
			}
		}
		break
	}
	return false
}

// removeVersion is ts_stack_remove_version.
//
// Remove the given version from the stack.
func (s *stack) removeVersion(version stackVersion) {
	s.heads[version].delete(&s.nodePool, s.subtreePool)
	s.heads = slices.Delete(s.heads, int(version), int(version)+1)
}

// renumberVersion is ts_stack_renumber_version.
func (s *stack) renumberVersion(v1, v2 stackVersion) {
	if v1 == v2 {
		return
	}
	assert(v2 < v1)
	assert(uint32(v1) < uint32(len(s.heads)))
	sourceHead := &s.heads[v1]
	targetHead := &s.heads[v2]
	if targetHead.summary != nil && sourceHead.summary == nil {
		sourceHead.summary = targetHead.summary
		targetHead.summary = nil
	}
	targetHead.delete(&s.nodePool, s.subtreePool)
	*targetHead = *sourceHead
	s.heads = slices.Delete(s.heads, int(v1), int(v1)+1)
}

// swapVersions is ts_stack_swap_versions.
func (s *stack) swapVersions(v1, v2 stackVersion) {
	s.heads[v1], s.heads[v2] = s.heads[v2], s.heads[v1]
}

// copyVersion is ts_stack_copy_version.
func (s *stack) copyVersion(version stackVersion) stackVersion {
	assert(uint32(version) < uint32(len(s.heads)))
	versionHead := s.heads[version]
	s.heads = append(s.heads, versionHead)
	head := &s.heads[len(s.heads)-1]
	head.node.retain()
	if head.lastExternalToken.ptr != nil {
		head.lastExternalToken.retain()
	}
	head.summary = nil
	return stackVersion(len(s.heads) - 1)
}

// merge is ts_stack_merge.
//
// Merge the given two stack versions if possible, returning true
// if they were successfully merged and false otherwise.
func (s *stack) merge(version1, version2 stackVersion) bool {
	if !s.canMerge(version1, version2) {
		return false
	}
	head1 := &s.heads[version1]
	head2 := &s.heads[version2]
	for i := range uint32(head2.node.linkCount) {
		head1.node.addLink(head2.node.links[i], s.subtreePool)
	}
	if head1.node.state == errorState {
		head1.nodeCountAtLastError = head1.node.nodeCount
	}
	s.removeVersion(version2)
	return true
}

// canMerge is ts_stack_can_merge.
//
// Determine whether the given two stack versions can be merged.
func (s *stack) canMerge(version1, version2 stackVersion) bool {
	head1 := &s.heads[version1]
	head2 := &s.heads[version2]
	return head1.status == stackStatusActive &&
		head2.status == stackStatusActive &&
		head1.node.state == head2.node.state &&
		head1.node.position.bytes == head2.node.position.bytes &&
		head1.node.errorCost == head2.node.errorCost &&
		head1.lastExternalToken.externalScannerStateEq(head2.lastExternalToken)
}

// halt is ts_stack_halt.
func (s *stack) halt(version stackVersion) {
	s.heads[version].status = stackStatusHalted
}

// pause is ts_stack_pause. The head takes the reference of lookahead.
func (s *stack) pause(version stackVersion, lookahead subtree) {
	head := &s.heads[version]
	head.status = stackStatusPaused
	head.lookaheadWhenPaused = lookahead
	head.nodeCountAtLastError = head.node.nodeCount
}

// isActive is ts_stack_is_active.
func (s *stack) isActive(version stackVersion) bool {
	return s.heads[version].status == stackStatusActive
}

// isHalted is ts_stack_is_halted.
func (s *stack) isHalted(version stackVersion) bool {
	return s.heads[version].status == stackStatusHalted
}

// isPaused is ts_stack_is_paused.
func (s *stack) isPaused(version stackVersion) bool {
	return s.heads[version].status == stackStatusPaused
}

// resume is ts_stack_resume. The caller takes the reference of the
// lookahead.
func (s *stack) resume(version stackVersion) subtree {
	head := &s.heads[version]
	assert(head.status == stackStatusPaused)
	result := head.lookaheadWhenPaused
	head.status = stackStatusActive
	head.lookaheadWhenPaused = subtree{}
	return result
}

// clear is ts_stack_clear.
func (s *stack) clear() {
	s.baseNode.retain()
	for i := range s.heads {
		s.heads[i].delete(&s.nodePool, s.subtreePool)
	}
	s.heads = s.heads[:0]
	s.heads = append(s.heads, stackHead{
		node:                s.baseNode,
		status:              stackStatusActive,
		lastExternalToken:   subtree{},
		lookaheadWhenPaused: subtree{},
	})
}

// printDotGraph is ts_stack_print_dot_graph. A nil w is stderr, as a NULL
// file is in C. The name of a node in the graph is the address of the node,
// as in C. C prints each byte of the state of the external scanner as a
// char, which is signed on the platforms that transit supports, so a byte
// above 0x7F prints as a negative number with %X. Go prints it the same way.
func (s *stack) printDotGraph(language *Language, w io.Writer) bool {
	s.iterators = slices.Grow(s.iterators, 32)
	if w == nil {
		w = os.Stderr
	}

	fprintf(w, "digraph stack {\n")
	fprintf(w, "rankdir=\"RL\";\n")
	fprintf(w, "edge [arrowhead=none]\n")

	var visitedNodes []*stackNode

	s.iterators = s.iterators[:0]
	for i := range uint32(len(s.heads)) {
		head := &s.heads[i]
		if head.status == stackStatusHalted {
			continue
		}

		fprintf(w, "node_head_%d [shape=none, label=\"\"]\n", i)
		fprintf(w, "node_head_%d -> node_%p [", i, head.node)

		if head.status == stackStatusPaused {
			fprintf(w, "color=red ")
		}
		fprintf(w,
			"label=%d, fontcolor=blue, weight=10000, labeltooltip=\"node_count: %d\nerror_cost: %d",
			i,
			s.nodeCountSinceError(stackVersion(i)),
			s.errorCost(stackVersion(i)),
		)

		if head.summary != nil {
			fprintf(w, "\nsummary:")
			for _, entry := range *head.summary {
				fprintf(w, " %d", entry.state)
			}
		}

		if head.lastExternalToken.ptr != nil {
			state := &head.lastExternalToken.ptr.externalScannerState
			data := state.data()
			fprintf(w, "\nexternal_scanner_state:")
			for _, b := range data {
				fprintf(w, " %2X", uint32(int8(b)))
			}
		}

		fprintf(w, "\"]\n")
		s.iterators = append(s.iterators, stackIterator{
			node: head.node,
		})
	}

	allIteratorsDone := false
	for !allIteratorsDone {
		allIteratorsDone = true

		for i := 0; i < len(s.iterators); i++ {
			iterator := s.iterators[i]
			node := iterator.node

			if slices.Contains(visitedNodes, node) {
				continue
			}
			allIteratorsDone = false

			fprintf(w, "node_%p [", node)
			switch {
			case node.state == errorState:
				fprintf(w, "label=\"?\"")
			case node.linkCount == 1 &&
				node.links[0].subtree.ptr != nil &&
				node.links[0].subtree.extra():
				fprintf(w, "shape=point margin=0 label=\"\"")
			default:
				fprintf(w, "label=\"%d\"", node.state)
			}

			fprintf(
				w,
				" tooltip=\"position: %d,%d\nnode_count:%d\nerror_cost: %d\ndynamic_precedence: %d\"];\n",
				node.position.extent.row+1,
				node.position.extent.column,
				node.nodeCount,
				node.errorCost,
				int32(node.dynamicPrecedence),
			)

			for j := range int(node.linkCount) {
				link := node.links[j]
				fprintf(w, "node_%p -> node_%p [", node, link.node)
				if link.isPending {
					fprintf(w, "style=dashed ")
				}
				if link.subtree.ptr != nil && link.subtree.extra() {
					fprintf(w, "fontcolor=gray ")
				}

				if link.subtree.ptr == nil {
					fprintf(w, "color=red")
				} else {
					fprintf(w, "label=\"")
					quoted := link.subtree.visible() && !link.subtree.named()
					if quoted {
						fprintf(w, "'")
					}
					language.writeSymbolAsDotString(w, link.subtree.symbol())
					if quoted {
						fprintf(w, "'")
					}
					fprintf(w, "\"")
					fprintf(
						w,
						"labeltooltip=\"error_cost: %d\ndynamic_precedence: %d\"",
						link.subtree.errorCost(),
						link.subtree.dynamicPrecedence(),
					)
				}

				fprintf(w, "];\n")

				var nextIterator *stackIterator
				if j == 0 {
					nextIterator = &s.iterators[i]
				} else {
					s.iterators = append(s.iterators, iterator)
					nextIterator = &s.iterators[len(s.iterators)-1]
				}
				nextIterator.node = link.node
			}

			visitedNodes = append(visitedNodes, node)
		}
	}

	fprintf(w, "}\n")

	return true
}
