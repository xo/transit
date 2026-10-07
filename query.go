package transit

import (
	"context"
	"math"
	"slices"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/query.c, and the types of
// lib/include/tree_sitter/api.h that a query uses.
//
// TSQuery is the type query, and TSQueryCursor is the type queryCursor. The
// exported types Query and QueryCursor of docs/API.md wrap them, as the Rust
// binding wraps the C types.
//
// These parts have no Go form:
//
//  1. ts_query_delete, ts_query_cursor_delete, and each _delete function of
//     the types of this file free memory, and the garbage collector frees it
//     (D24). capture_quantifiers_new, symbol_table_new and
//     query_analysis__new return empty values, and the zero value of the Go
//     type is the same value.
//  2. ts_query_copy copies a query so that two threads can each own one. A Go
//     query is safe to share between goroutines (D52).
//  3. The debug output of DEBUG_ANALYZE_QUERY, DEBUG_EXECUTE_QUERY and
//     DEBUG_QUERY_STEPS, which upstream compiles out, and
//     ts_query__dump_steps.
//  4. ts_query_cursor_exec_with_options. A context.Context replaces the
//     progress callback. The methods that advance the cursor take the context,
//     and they read ctx.Err() where C calls the callback. Go does not record
//     current_byte_offset, because only the callback reads it.
//
// C keeps pointers into the arrays of states. Go keeps indices where an
// insert can move the states, and takes the pointer again after the insert,
// as C takes it again from the index.
//
// The C stream reads the source with iswspace and iswalnum of the C library.
// A program that does not call setlocale runs in the C locale, where glibc
// counts only ASCII characters. The Go functions iswspace and iswalnum give
// the same results.

// Quantifier is how many times a capture can appear in one match of a
// pattern.
//
// Quantifier is TSQuantifier.
type Quantifier int

// The quantifiers of a capture.
const (
	// QuantifierZero is TSQuantifierZero. The capture does not appear.
	QuantifierZero Quantifier = iota
	// QuantifierZeroOrOne is TSQuantifierZeroOrOne. The capture appears at
	// most once.
	QuantifierZeroOrOne
	// QuantifierZeroOrMore is TSQuantifierZeroOrMore. The capture can appear
	// any number of times.
	QuantifierZeroOrMore
	// QuantifierOne is TSQuantifierOne. The capture appears once.
	QuantifierOne
	// QuantifierOneOrMore is TSQuantifierOneOrMore. The capture appears at
	// least once.
	QuantifierOneOrMore
)

// String returns the name of the quantifier.
func (q Quantifier) String() string {
	switch q {
	case QuantifierZero:
		return "zero"
	case QuantifierZeroOrOne:
		return "zero or one"
	case QuantifierZeroOrMore:
		return "zero or more"
	case QuantifierOne:
		return "one"
	case QuantifierOneOrMore:
		return "one or more"
	}
	return unknownName
}

// QueryErrorKind is the kind of fault in the source of a query.
//
// QueryErrorKind is TSQueryError.
type QueryErrorKind int

// The kinds of fault in the source of a query.
const (
	// QueryErrorNone is TSQueryErrorNone. The query has no fault.
	QueryErrorNone QueryErrorKind = iota
	// QueryErrorSyntax is TSQueryErrorSyntax. The source is not valid.
	QueryErrorSyntax
	// QueryErrorNodeType is TSQueryErrorNodeType. The language has no node
	// with the name.
	QueryErrorNodeType
	// QueryErrorField is TSQueryErrorField. The language has no field with
	// the name.
	QueryErrorField
	// QueryErrorCapture is TSQueryErrorCapture. A predicate names a capture
	// that the pattern does not have.
	QueryErrorCapture
	// QueryErrorStructure is TSQueryErrorStructure. The pattern cannot match
	// any tree of the language.
	QueryErrorStructure
	// QueryErrorLanguage is TSQueryErrorLanguage. The ABI version of the
	// language is not one that the runtime accepts.
	QueryErrorLanguage
	// QueryErrorPredicate is QueryErrorKind::Predicate of the Rust binding,
	// which TSQueryError does not have. A predicate of the query does not
	// parse. The query API returns it, and newQuery never returns it.
	QueryErrorPredicate
)

// String returns the name of the kind.
func (k QueryErrorKind) String() string {
	switch k {
	case QueryErrorNone:
		return noneName
	case QueryErrorSyntax:
		return "syntax"
	case QueryErrorNodeType:
		return "node type"
	case QueryErrorField:
		return "field"
	case QueryErrorCapture:
		return "capture"
	case QueryErrorStructure:
		return "structure"
	case QueryErrorLanguage:
		return "language"
	case QueryErrorPredicate:
		return "predicate"
	}
	return unknownName
}

// queryCapture is TSQueryCapture, a node that a pattern captures. index is
// the id of the name of the capture.
type queryCapture struct {
	node  Node
	index uint32
}

// queryMatch is TSQueryMatch. C gives a pointer to the captures and a count,
// and Go gives a slice.
//
// captures is the capture list of the cursor, as in C. The cursor uses the
// list again for a later match, and then changes the captures that the slice
// holds. The slice has no room past its length, so an append to it copies
// it.
type queryMatch struct {
	id           uint32
	patternIndex uint16
	captures     []queryCapture
}

// queryPredicateStepType is TSQueryPredicateStepType.
type queryPredicateStepType int

// The kinds of step of a predicate.
const (
	// queryPredicateStepTypeDone is TSQueryPredicateStepTypeDone, the end of
	// a predicate.
	queryPredicateStepTypeDone queryPredicateStepType = iota
	// queryPredicateStepTypeCapture is TSQueryPredicateStepTypeCapture. The
	// value is the id of a capture.
	queryPredicateStepTypeCapture
	// queryPredicateStepTypeString is TSQueryPredicateStepTypeString. The
	// value is the id of a string.
	queryPredicateStepTypeString
)

// String returns the name of the kind.
func (t queryPredicateStepType) String() string {
	switch t {
	case queryPredicateStepTypeDone:
		return "done"
	case queryPredicateStepTypeCapture:
		return "capture"
	case queryPredicateStepTypeString:
		return "string"
	}
	return unknownName
}

// queryPredicateStep is TSQueryPredicateStep.
type queryPredicateStep struct {
	typ     queryPredicateStepType
	valueID uint32
}

// The limits of a query.
const (
	// maxStepCaptureCount is MAX_STEP_CAPTURE_COUNT.
	maxStepCaptureCount = 3
	// maxNegatedFieldCount is MAX_NEGATED_FIELD_COUNT.
	maxNegatedFieldCount = 8
	// maxStatePredecessorCount is MAX_STATE_PREDECESSOR_COUNT.
	maxStatePredecessorCount = 256
	// maxAnalysisStateDepth is MAX_ANALYSIS_STATE_DEPTH.
	maxAnalysisStateDepth = 8
	// maxAnalysisIterationCount is MAX_ANALYSIS_ITERATION_COUNT.
	maxAnalysisIterationCount = 256
)

// stream is Stream. C keeps the pointers input, start and end into the
// source. Go keeps the source, from start to end, and input is the offset of
// input from start.
//
// Stream - A sequence of unicode characters derived from a UTF8 string.
// This struct is used in parsing queries from S-expressions.
type stream struct {
	source   []byte
	input    uint32
	next     int32
	nextSize uint8
}

// queryStep is QueryStep. The bit fields of C are bool fields.
//
// QueryStep - A step in the process of matching a query. Each node within
// a query S-expression corresponds to one of these steps. An entire pattern
// is represented as a sequence of these steps. The basic properties of a
// node are represented by these fields:
//   - `symbol` - The grammar symbol to match. A zero value represents the
//     wildcard symbol, '_'.
//   - `field` - The field name to match. A zero value means that a field name
//     was not specified.
//   - `capture_ids` - An array of integers representing the names of captures
//     associated with this node in the pattern, terminated by a `NONE` value.
//   - `depth` - The depth where this node occurs in the pattern. The root node
//     of the pattern has depth zero.
//   - `negated_field_list_id` - An id representing a set of fields that must
//     not be present on a node matching this step.
//
// Steps have some additional fields in order to handle the `.` (or "anchor") operator,
// which forbids additional child nodes:
//   - `is_immediate` - Indicates that the node matching this step cannot be preceded
//     by other sibling nodes that weren't specified in the pattern.
//   - `is_last_child` - Indicates that the node matching this step cannot have any
//     subsequent named siblings.
//
// For simple patterns, steps are matched in sequential order. But in order to
// handle alternative/repeated/optional sub-patterns, query steps are not always
// structured as a linear sequence; they sometimes need to split and merge. This
// is done using the following fields:
//   - `alternative_index` - The index of a different query step that serves as
//     an alternative to this step. A `NONE` value represents no alternative.
//     When a query state reaches a step with an alternative index, the state
//     is duplicated, with one copy remaining at the original step, and one copy
//     moving to the alternative step. The alternative may have its own alternative
//     step, so this splitting is an iterative process.
//   - `is_dead_end` - Indicates that this state cannot be passed directly, and
//     exists only in order to redirect to an alternative index, with no splitting.
//   - `is_pass_through` - Indicates that state has no matching logic of its own,
//     and exists only to split a state. One copy of the state advances immediately
//     to the next step, and one moves to the alternative step.
//   - `alternative_is_skip` - Indicates that this step's `alternative_index` is the
//     forward skip introduced by a `?` or `*` quantifier (the branch taken when the
//     quantifier matches zero occurrences). For a state that follows it, an
//     immediately-following anchor is vacuous.
//   - `is_inside_alternation` - Indicates that state is inside an alternation.
//     Currently only written to quantifier steps, read by logic that maintains
//     correctness for quantifiers inside alternations.
//
// Steps also store some derived state that summarizes how they relate to other
// steps within the same pattern. This is used to optimize the matching process:
//   - `contains_captures` - Indicates that this step or one of its child steps
//     has a non-empty `capture_ids` list.
//   - `parent_pattern_guaranteed` - Indicates that if this step is reached, then
//     it and all of its subsequent sibling steps within the same parent pattern
//     are guaranteed to match.
//   - `root_pattern_guaranteed` - Similar to `parent_pattern_guaranteed`, but
//     for the entire top-level pattern. When iterating through a query's
//     captures using `ts_query_cursor_next_capture`, this field is used to
//     detect that a capture can safely be returned from a match that has not
//     even completed yet.
type queryStep struct {
	symbol             Symbol
	supertypeSymbol    Symbol
	field              FieldID
	captureIDs         [maxStepCaptureCount]uint16
	depth              uint16
	alternativeIndex   uint16
	negatedFieldListID uint16

	isNamed                 bool
	isImmediate             bool
	isLastChild             bool
	isPassThrough           bool
	isDeadEnd               bool
	isInsideAlternation     bool
	containsCaptures        bool
	rootPatternGuaranteed   bool
	parentPatternGuaranteed bool
	isMissing               bool
	alternativeIsSkip       bool
}

// slice is Slice.
//
// Slice - A slice of an external array. Within a query, capture names,
// literal string values, and predicate step information are stored in three
// contiguous arrays. Individual captures, string values, and predicates are
// represented as slices of these three arrays.
type slice struct {
	offset uint32
	length uint32
}

// symbolTable is SymbolTable. characters holds each name with a NUL byte
// after it, as in C.
//
// SymbolTable - a two-way mapping of strings to ids.
type symbolTable struct {
	characters []byte
	slices     []slice
}

// captureQuantifierList is CaptureQuantifiers. C keeps each quantifier in
// a uint8_t. The name captureQuantifiers is the name of the variables of
// this type, as in C.
//
// CaptureQuantifiers - a data structure holding the quantifiers of pattern captures.
type captureQuantifierList []Quantifier

// patternEntry is PatternEntry.
//
// PatternEntry - Information about the starting point for matching a particular
// pattern. These entries are stored in a 'pattern map' - a sorted array that
// makes it possible to efficiently lookup patterns based on the symbol for their
// first step. The entry consists of the following fields:
//   - `pattern_index` - the index of the pattern within the query
//   - `step_index` - the index of the pattern's first step in the shared `steps` array
//   - `is_rooted` - whether or not the pattern has a single root node. This property
//     affects decisions about whether or not to start the pattern for nodes outside
//     of a QueryCursor's range restriction.
type patternEntry struct {
	stepIndex    uint16
	patternIndex uint16
	isRooted     bool
}

// queryPattern is QueryPattern.
type queryPattern struct {
	steps          slice
	predicateSteps slice
	startByte      uint32
	endByte        uint32
	isNonLocal     bool
}

// stepOffset is StepOffset.
type stepOffset struct {
	byteOffset uint32
	stepIndex  uint16
}

// consumedCaptureCountMask is the mask of the bit field
// consumed_capture_count of QueryState, which has 12 bits. Go keeps the
// field in a uint16, and an increment wraps at 4096 as in C.
const consumedCaptureCountMask = 1<<12 - 1

// queryState is QueryState. The bit fields of C are bool fields, and
// consumedCaptureCount keeps 12 bits.
//
// QueryState - The state of an in-progress match of a particular pattern
// in a query. While executing, a `TSQueryCursor` must keep track of a number
// of possible in-progress matches. Each of those possible matches is
// represented as one of these states. Fields:
//   - `id` - A numeric id that is exposed to the public API. This allows the
//     caller to remove a given match, preventing any more of its captures
//     from being returned.
//   - `start_depth` - The depth in the tree where the first step of the state's
//     pattern was matched.
//   - `pattern_index` - The pattern that the state is matching.
//   - `consumed_capture_count` - The number of captures from this match that
//     have already been returned.
//   - `capture_list_id` - A numeric id that can be used to retrieve the state's
//     list of captures from the `CaptureListPool`.
//   - `heap_insert_order` - A sequence number used to preserve discovery order
//     among finished states with the same capture position and pattern.
//   - `seeking_immediate_match` - A flag that indicates that the state's next
//     step must be matched by the very next sibling. This is used when
//     processing repetitions, or when processing a wildcard node followed by
//     an anchor.
//   - `has_in_progress_alternatives` - A flag that indicates that there are
//     other states that have the same captures as this state, but are at
//     different steps in their pattern. This means that in order to obey the
//     'longest-match' rule, this state should not be returned as a match until
//     it is clear that there can be no other alternative match with more captures.
type queryState struct {
	id                   uint32
	captureListID        uint32
	heapInsertOrder      uint32
	startDepth           uint16
	stepIndex            uint16
	patternIndex         uint16
	consumedCaptureCount uint16

	seekingImmediateMatch     bool
	hasInProgressAlternatives bool
	dead                      bool
	needsParent               bool
	skippedQuantifier         bool
}

// captureList is CaptureList. C marks a list that no state uses with the
// size UINT32_MAX, and keeps its contents. Go keeps the contents and sets
// released.
type captureList struct {
	contents []queryCapture
	released bool
}

// captureListPool is CaptureListPool. emptyList is empty_list, which is
// nil.
//
// CaptureListPool - A collection of *lists* of captures. Each query state needs
// to maintain its own list of captures. To avoid repeated allocations, this struct
// maintains a fixed set of capture lists, and keeps track of which ones are
// currently in use by a query state.
type captureListPool struct {
	list      []captureList
	emptyList []queryCapture
	// The maximum number of capture lists that we are allowed to allocate. We
	// never allow `list` to allocate more entries than this, dropping pending
	// matches if needed to stay under the limit.
	maxCaptureListCount uint32
	// The number of capture lists allocated in `list` that are not currently in
	// use. We reuse those existing-but-unused capture lists before trying to
	// allocate any new ones. We use an invalid value (UINT32_MAX) for a capture
	// list's length to indicate that it's not in use.
	freeCaptureListCount uint32
}

// analysisStateEntry is AnalysisStateEntry. fieldID keeps the 15 bits of the
// bit field field_id.
type analysisStateEntry struct {
	parseState   StateID
	parentSymbol Symbol
	childIndex   uint16
	fieldID      FieldID
	done         bool
}

// analysisState is AnalysisState.
//
// AnalysisState - The state needed for walking the parse table when analyzing
// a query pattern, to determine at which steps the pattern might fail to match.
type analysisState struct {
	stack      [maxAnalysisStateDepth]analysisStateEntry
	depth      uint16
	stepIndex  uint16
	rootSymbol Symbol
}

// analysisStateSet is AnalysisStateSet.
type analysisStateSet []*analysisState

// queryAnalysis is QueryAnalysis.
type queryAnalysis struct {
	states                analysisStateSet
	nextStates            analysisStateSet
	deeperStates          analysisStateSet
	statePool             analysisStateSet
	finalStepIndices      []uint16
	finishedParentSymbols []Symbol
	didAbort              bool
}

// analysisSubgraphNode is AnalysisSubgraphNode. childIndex keeps the 7 bits
// of the bit field child_index, and a change of it wraps at 128 as in C.
//
// AnalysisSubgraph - A subset of the states in the parse table that are used
// in constructing nodes with a certain symbol. Each state is accompanied by
// some information about the possible node that could be produced in
// downstream states.
type analysisSubgraphNode struct {
	state        StateID
	productionID uint16
	childIndex   uint8
	done         bool
}

// analysisSubgraph is AnalysisSubgraph.
type analysisSubgraph struct {
	symbol      Symbol
	startStates []StateID
	nodes       []analysisSubgraphNode
}

// statePredecessorMap is StatePredecessorMap.
//
// StatePredecessorMap - A map that stores the predecessors of each parse state.
// This is used during query analysis to determine which parse states can lead
// to which reduce actions.
type statePredecessorMap struct {
	contents []StateID
}

// query is TSQuery. It is safe to share between goroutines, after any call
// of disableCapture or disablePattern (D52).
//
// TSQuery - A tree query, compiled from a string of S-expressions. The query
// itself is immutable. The mutable state used in the process of executing the
// query is stored in a `TSQueryCursor`.
type query struct {
	captures                          symbolTable
	predicateValues                   symbolTable
	captureQuantifiers                []captureQuantifierList
	steps                             []queryStep
	patternMap                        []patternEntry
	predicateSteps                    []queryPredicateStep
	patterns                          []queryPattern
	stepOffsets                       []stepOffset
	negatedFields                     []FieldID
	stringBuffer                      []byte
	repeatSymbolsWithRootlessPatterns []Symbol
	language                          *Language
	wildcardRootPatternCount          uint16
}

// queryCursor is TSQueryCursor. It belongs to one goroutine at a time (D52).
// C keeps query_options and query_state for the progress callback, and the
// Go methods that advance the cursor take a context in their place.
//
// TSQueryCursor - A stateful struct used to execute a query on a tree.
type queryCursor struct {
	query          *query
	cursor         TreeCursor
	states         []queryState
	finishedStates []queryState
	// Tracks how much of finished_states is in heap order. Elements at indices
	// < this value satisfy the min-heap property; elements >= this value are
	// newly pushed and need to be sifted into place. Only used by `next_capture`.
	finishedStatesHeapSize uint32
	captureListPool        captureListPool
	depth                  uint32
	maxStartDepth          uint32
	includedRange          textRange
	containingRange        textRange
	nextStateID            uint32
	nextFinishedStateID    uint32
	operationCount         uint32
	onVisibleNode          bool
	ascending              bool
	halted                 bool
	// exceededMatchLimit is did_exceed_match_limit. The method
	// didExceedMatchLimit has the name of the C function.
	exceededMatchLimit bool
}

// The constants of a query.
const (
	// parentDone is PARENT_DONE, which a parse of a pattern returns at the
	// end of its parent.
	parentDone QueryErrorKind = -1
	// patternDoneMarker is PATTERN_DONE_MARKER, the depth of the step that
	// ends a pattern.
	patternDoneMarker uint16 = math.MaxUint16
	// none is NONE.
	none uint16 = math.MaxUint16
	// captureListNone is CAPTURE_LIST_NONE.
	captureListNone uint32 = math.MaxUint32
	// wildcardSymbol is WILDCARD_SYMBOL.
	wildcardSymbol Symbol = 0
	// opCountPerQueryCallbackCheck is OP_COUNT_PER_QUERY_CALLBACK_CHECK.
	opCountPerQueryCallbackCheck = 100
)

// Stream

// advance is stream_advance.
//
// Advance to the next unicode code point in the stream.
func (s *stream) advance() bool {
	s.input += uint32(s.nextSize)
	if s.input < uint32(len(s.source)) {
		size, next := decodeUTF8(s.source[s.input:])
		s.next = next
		if size > 0 {
			s.nextSize = uint8(size)
			return true
		}
	} else {
		s.nextSize = 0
		s.next = 0
	}
	return false
}

// reset is stream_reset.
//
// Reset the stream to the given input position, represented as a pointer
// into the input string.
func (s *stream) reset(input uint32) {
	s.input = input
	s.nextSize = 0
	s.advance()
}

// newStream is stream_new.
func newStream(source []byte) stream {
	s := stream{
		next:   0,
		input:  0,
		source: source,
	}
	s.advance()
	return s
}

// skipWhitespace is stream_skip_whitespace.
func (s *stream) skipWhitespace() {
	for {
		switch {
		case iswspace(s.next):
			s.advance()
		case s.next == ';':
			// skip over comments
			s.advance()
			for s.next != 0 && s.next != '\n' {
				if !s.advance() {
					break
				}
			}
		default:
			return
		}
	}
}

// isIdentStart is stream_is_ident_start.
func (s *stream) isIdentStart() bool {
	return iswalnum(s.next) || s.next == '_' || s.next == '-'
}

// scanIdentifier is stream_scan_identifier.
func (s *stream) scanIdentifier() {
	for {
		s.advance()
		if !(iswalnum(s.next) ||
			s.next == '_' ||
			s.next == '-' ||
			s.next == '.') {
			break
		}
	}
}

// offset is stream_offset.
func (s *stream) offset() uint32 {
	return s.input
}

// CaptureListPool

// newCaptureListPool is capture_list_pool_new.
func newCaptureListPool() captureListPool {
	return captureListPool{
		list:                 nil,
		emptyList:            nil,
		maxCaptureListCount:  math.MaxUint32,
		freeCaptureListCount: 0,
	}
}

// reset is capture_list_pool_reset.
func (p *captureListPool) reset() {
	for i := range p.list {
		// This invalid size means that the list is not in use.
		p.list[i].released = true
	}
	p.freeCaptureListCount = uint32(len(p.list))
}

// get is capture_list_pool_get. C returns the size UINT32_MAX for a list
// that no state uses, and Go returns its contents. No caller reads such a
// list.
func (p *captureListPool) get(id uint32) []queryCapture {
	if id >= uint32(len(p.list)) {
		return p.emptyList
	}
	return p.list[id].contents
}

// getMut is capture_list_pool_get_mut.
func (p *captureListPool) getMut(id uint32) *captureList {
	assert(id < uint32(len(p.list)))
	return &p.list[id]
}

// isEmpty is capture_list_pool_is_empty.
func (p *captureListPool) isEmpty() bool {
	// The capture list pool is empty if all allocated lists are in use, and we
	// have reached the maximum allowed number of allocated lists.
	return p.freeCaptureListCount == 0 && uint32(len(p.list)) >= p.maxCaptureListCount
}

// acquire is capture_list_pool_acquire.
func (p *captureListPool) acquire() uint32 {
	// First see if any already allocated capture list is currently unused.
	if p.freeCaptureListCount > 0 {
		for i := range p.list {
			if p.list[i].released {
				p.list[i].contents = p.list[i].contents[:0]
				p.list[i].released = false
				p.freeCaptureListCount--
				return uint32(i)
			}
		}
	}

	// Otherwise allocate and initialize a new capture list, as long as that
	// doesn't put us over the requested maximum.
	i := uint32(len(p.list))
	if i >= p.maxCaptureListCount {
		return captureListNone
	}
	p.list = append(p.list, captureList{})
	return i
}

// release is capture_list_pool_release.
func (p *captureListPool) release(id uint32) {
	if id >= uint32(len(p.list)) {
		return
	}
	p.list[id].released = true
	p.freeCaptureListCount++
}

// FinishedStateHeap
//
// A min-heap of finished query states, ordered by (byte offset of next
// unconsumed capture, pattern_index, insertion order). This allows
// ts_query_cursor_next_capture to find the earliest capture in O(1) instead
// of scanning all finished states. The heap is maintained lazily -
// ts_query_cursor__advance uses plain array_push, and next_capture sifts
// new elements into place via a tracked heap_size boundary.

// finishedStateSwap is finished_state_swap.
func finishedStateSwap(states []queryState, a, b uint32) {
	states[a], states[b] = states[b], states[a]
}

// finishedStatePrecedes is finished_state_precedes.
//
// Compare two finished states by (byte offset of next unconsumed capture,
// pattern_index, insertion order).
func finishedStatePrecedes(a, b *queryState, pool *captureListPool) bool {
	aCaps := pool.get(a.captureListID)
	bCaps := pool.get(b.captureListID)
	if uint32(a.consumedCaptureCount) >= uint32(len(aCaps)) {
		return false
	}
	if uint32(b.consumedCaptureCount) >= uint32(len(bCaps)) {
		return true
	}
	aByte := uint32(aCaps[a.consumedCaptureCount].node.StartByte())
	bByte := uint32(bCaps[b.consumedCaptureCount].node.StartByte())
	if aByte != bByte {
		return aByte < bByte
	}
	if a.patternIndex != b.patternIndex {
		return a.patternIndex < b.patternIndex
	}
	return a.heapInsertOrder < b.heapInsertOrder
}

// finishedStateSiftDown is finished_state_sift_down.
func finishedStateSiftDown(states []queryState, index uint32, pool *captureListPool) {
	size := uint32(len(states))
	for {
		smallest := index
		left := 2*index + 1
		right := 2*index + 2
		if left < size && finishedStatePrecedes(
			&states[left],
			&states[smallest],
			pool,
		) {
			smallest = left
		}
		if right < size && finishedStatePrecedes(
			&states[right],
			&states[smallest],
			pool,
		) {
			smallest = right
		}
		if smallest == index {
			break
		}
		finishedStateSwap(states, index, smallest)
		index = smallest
	}
}

// finishedStateSiftUp is finished_state_sift_up.
func finishedStateSiftUp(states []queryState, index uint32, pool *captureListPool) {
	for index > 0 {
		parent := (index - 1) / 2
		if finishedStatePrecedes(
			&states[index],
			&states[parent],
			pool,
		) {
			finishedStateSwap(states, index, parent)
			index = parent
		} else {
			break
		}
	}
}

// finishedStatePop is finished_state_pop.
func finishedStatePop(states *[]queryState, pool *captureListPool) {
	s := *states
	if len(s) > 1 {
		s[0] = s[len(s)-1]
	}
	s = s[:len(s)-1]
	*states = s
	if len(s) > 0 {
		finishedStateSiftDown(s, 0, pool)
	}
}

// finishedStateErase is finished_state_erase.
//
// Remove an element at an arbitrary index and restore heap order.
func finishedStateErase(states *[]queryState, index uint32, pool *captureListPool) {
	s := *states
	if index == uint32(len(s))-1 {
		*states = s[:len(s)-1]
		return
	}
	s[index] = s[len(s)-1]
	s = s[:len(s)-1]
	*states = s
	// The replacement element may need to go up or down.
	if index > 0 && finishedStatePrecedes(
		&s[index],
		&s[(index-1)/2],
		pool,
	) {
		finishedStateSiftUp(s, index, pool)
	} else {
		finishedStateSiftDown(s, index, pool)
	}
}

// pushFinishedState is ts_query_cursor__push_finished_state.
func (c *queryCursor) pushFinishedState(state *queryState) {
	state.heapInsertOrder = c.nextFinishedStateID
	c.nextFinishedStateID++
	c.finishedStates = append(c.finishedStates, *state)
}

// heapifyFinishedStates is ts_query_cursor__heapify_finished_states.
func (c *queryCursor) heapifyFinishedStates() {
	for c.finishedStatesHeapSize < uint32(len(c.finishedStates)) {
		finishedStateSiftUp(
			c.finishedStates,
			c.finishedStatesHeapSize,
			&c.captureListPool,
		)
		c.finishedStatesHeapSize++
	}
}

// Quantifiers

// quantifierMul is quantifier_mul.
func quantifierMul(left, right Quantifier) Quantifier {
	switch left {
	case QuantifierZero:
		return QuantifierZero
	case QuantifierZeroOrOne:
		switch right {
		case QuantifierZero:
			return QuantifierZero
		case QuantifierZeroOrOne, QuantifierOne:
			return QuantifierZeroOrOne
		case QuantifierZeroOrMore, QuantifierOneOrMore:
			return QuantifierZeroOrMore
		}
	case QuantifierZeroOrMore:
		switch right {
		case QuantifierZero:
			return QuantifierZero
		case QuantifierZeroOrOne, QuantifierZeroOrMore, QuantifierOne, QuantifierOneOrMore:
			return QuantifierZeroOrMore
		}
	case QuantifierOne:
		return right
	case QuantifierOneOrMore:
		switch right {
		case QuantifierZero:
			return QuantifierZero
		case QuantifierZeroOrOne, QuantifierZeroOrMore:
			return QuantifierZeroOrMore
		case QuantifierOne, QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	}
	return QuantifierZero // to make compiler happy, but all cases should be covered above!
}

// quantifierJoin is quantifier_join.
func quantifierJoin(left, right Quantifier) Quantifier {
	switch left {
	case QuantifierZero:
		switch right {
		case QuantifierZero:
			return QuantifierZero
		case QuantifierZeroOrOne, QuantifierOne:
			return QuantifierZeroOrOne
		case QuantifierZeroOrMore, QuantifierOneOrMore:
			return QuantifierZeroOrMore
		}
	case QuantifierZeroOrOne:
		switch right {
		case QuantifierZero, QuantifierZeroOrOne, QuantifierOne:
			return QuantifierZeroOrOne
		case QuantifierZeroOrMore, QuantifierOneOrMore:
			return QuantifierZeroOrMore
		}
	case QuantifierZeroOrMore:
		return QuantifierZeroOrMore
	case QuantifierOne:
		switch right {
		case QuantifierZero, QuantifierZeroOrOne:
			return QuantifierZeroOrOne
		case QuantifierZeroOrMore:
			return QuantifierZeroOrMore
		case QuantifierOne:
			return QuantifierOne
		case QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	case QuantifierOneOrMore:
		switch right {
		case QuantifierZero, QuantifierZeroOrOne, QuantifierZeroOrMore:
			return QuantifierZeroOrMore
		case QuantifierOne, QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	}
	return QuantifierZero // to make compiler happy, but all cases should be covered above!
}

// quantifierAdd is quantifier_add.
func quantifierAdd(left, right Quantifier) Quantifier {
	switch left {
	case QuantifierZero:
		return right
	case QuantifierZeroOrOne:
		switch right {
		case QuantifierZero:
			return QuantifierZeroOrOne
		case QuantifierZeroOrOne, QuantifierZeroOrMore:
			return QuantifierZeroOrMore
		case QuantifierOne, QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	case QuantifierZeroOrMore:
		switch right {
		case QuantifierZero:
			return QuantifierZeroOrMore
		case QuantifierZeroOrOne, QuantifierZeroOrMore:
			return QuantifierZeroOrMore
		case QuantifierOne, QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	case QuantifierOne:
		switch right {
		case QuantifierZero:
			return QuantifierOne
		case QuantifierZeroOrOne, QuantifierZeroOrMore, QuantifierOne, QuantifierOneOrMore:
			return QuantifierOneOrMore
		}
	case QuantifierOneOrMore:
		return QuantifierOneOrMore
	}
	return QuantifierZero // to make compiler happy, but all cases should be covered above!
}

// clear is capture_quantifiers_clear.
//
// Clear capture quantifiers structure.
func (q *captureQuantifierList) clear() {
	*q = (*q)[:0]
}

// replace is capture_quantifiers_replace.
//
// Replace capture quantifiers with the given quantifiers.
func (q *captureQuantifierList) replace(quantifiers captureQuantifierList) {
	*q = append((*q)[:0], quantifiers...)
}

// forID is capture_quantifier_for_id.
//
// Return capture quantifier for the given capture id.
func (q captureQuantifierList) forID(id uint16) Quantifier {
	if uint32(len(q)) <= uint32(id) {
		return QuantifierZero
	}
	return q[id]
}

// addForID is capture_quantifiers_add_for_id.
//
// Add the given quantifier to the current value for id.
func (q *captureQuantifierList) addForID(id uint16, quantifier Quantifier) {
	if uint32(len(*q)) <= uint32(id) {
		*q = append(*q, make(captureQuantifierList, uint32(id)+1-uint32(len(*q)))...)
	}
	ownQuantifier := &(*q)[id]
	*ownQuantifier = quantifierAdd(*ownQuantifier, quantifier)
}

// addAll is capture_quantifiers_add_all.
//
// Point-wise add the given quantifiers to the current values.
func (q *captureQuantifierList) addAll(quantifiers captureQuantifierList) {
	if len(*q) < len(quantifiers) {
		*q = append(*q, make(captureQuantifierList, len(quantifiers)-len(*q))...)
	}
	for id := range uint16(len(quantifiers)) {
		quantifier := quantifiers[id]
		ownQuantifier := &(*q)[id]
		*ownQuantifier = quantifierAdd(*ownQuantifier, quantifier)
	}
}

// mul is capture_quantifiers_mul.
//
// Join the given quantifier with the current values.
func (q captureQuantifierList) mul(quantifier Quantifier) {
	for id := range uint16(len(q)) {
		ownQuantifier := &q[id]
		*ownQuantifier = quantifierMul(*ownQuantifier, quantifier)
	}
}

// joinAll is capture_quantifiers_join_all.
//
// Point-wise join the quantifiers from a list of alternatives with the current values.
func (q *captureQuantifierList) joinAll(quantifiers captureQuantifierList) {
	if len(*q) < len(quantifiers) {
		*q = append(*q, make(captureQuantifierList, len(quantifiers)-len(*q))...)
	}
	own := *q
	for id := range quantifiers {
		quantifier := quantifiers[id]
		ownQuantifier := &own[id]
		*ownQuantifier = quantifierJoin(*ownQuantifier, quantifier)
	}
	for id := len(quantifiers); id < len(own); id++ {
		ownQuantifier := &own[id]
		*ownQuantifier = quantifierJoin(*ownQuantifier, QuantifierZero)
	}
}

// SymbolTable

// idForName is symbol_table_id_for_name.
func (t *symbolTable) idForName(name []byte) int {
	length := uint32(len(name))
	for i := range t.slices {
		slice := t.slices[i]
		if slice.length == length &&
			strncmpBytes(t.characters[slice.offset:], name, int(length)) == 0 {
			return i
		}
	}
	return -1
}

// nameForID is symbol_table_name_for_id. The C function returns a pointer
// and writes the length, and the Go function returns a copy of the name.
func (t *symbolTable) nameForID(id uint16) string {
	slice := t.slices[id]
	return string(t.characters[slice.offset : slice.offset+slice.length])
}

// insertName is symbol_table_insert_name.
func (t *symbolTable) insertName(name []byte) uint16 {
	id := t.idForName(name)
	if id >= 0 {
		return uint16(id)
	}
	slice := slice{
		offset: uint32(len(t.characters)),
		length: uint32(len(name)),
	}
	t.characters = append(t.characters, name...)
	t.characters = append(t.characters, 0)
	t.slices = append(t.slices, slice)
	return uint16(len(t.slices) - 1)
}

// QueryStep

// newQueryStep is query_step__new.
func newQueryStep(symbol Symbol, depth uint16, isImmediate bool) queryStep {
	step := queryStep{
		symbol:           symbol,
		depth:            depth,
		alternativeIndex: none,
		isImmediate:      isImmediate,
	}
	for i := range maxStepCaptureCount {
		step.captureIDs[i] = none
	}
	return step
}

// addCapture is query_step__add_capture.
func (s *queryStep) addCapture(captureID uint16) {
	for i := range maxStepCaptureCount {
		if s.captureIDs[i] == none {
			s.captureIDs[i] = captureID
			break
		}
	}
}

// removeCapture is query_step__remove_capture.
func (s *queryStep) removeCapture(captureID uint16) {
	for i := 0; i < maxStepCaptureCount; i++ {
		if s.captureIDs[i] == captureID {
			s.captureIDs[i] = none
			for i+1 < maxStepCaptureCount {
				if s.captureIDs[i+1] == none {
					break
				}
				s.captureIDs[i] = s.captureIDs[i+1]
				s.captureIDs[i+1] = none
				i++
			}
			break
		}
	}
}

// StatePredecessorMap

// newStatePredecessorMap is state_predecessor_map_new.
func newStatePredecessorMap(language *Language) statePredecessorMap {
	return statePredecessorMap{
		contents: make([]StateID, int(language.tables.StateCount)*(maxStatePredecessorCount+1)),
	}
}

// add is state_predecessor_map_add.
func (m *statePredecessorMap) add(state, predecessor StateID) {
	index := int(state) * (maxStatePredecessorCount + 1)
	count := &m.contents[index]
	if *count == 0 ||
		(*count < maxStatePredecessorCount && m.contents[index+int(*count)] != predecessor) {
		(*count)++
		m.contents[index+int(*count)] = predecessor
	}
}

// get is state_predecessor_map_get. The C function returns a pointer and
// writes the count, and the Go function returns the slice.
func (m *statePredecessorMap) get(state StateID) []StateID {
	index := int(state) * (maxStatePredecessorCount + 1)
	count := int(m.contents[index])
	return m.contents[index+1 : index+1+count]
}

// AnalysisState

// recursionDepth is analysis_state__recursion_depth.
func (s *analysisState) recursionDepth() uint32 {
	result := uint32(0)
	for i := range s.depth {
		symbol := s.stack[i].parentSymbol
		for j := range i {
			if s.stack[j].parentSymbol == symbol {
				result++
				break
			}
		}
	}
	return result
}

// analysisStateCompare is analysis_state__compare. The C function takes
// pointers to the pointers of the states.
func analysisStateCompare(self, other *analysisState) int {
	if self.depth < other.depth {
		return 1
	}
	for i := range self.depth {
		if i >= other.depth {
			return -1
		}
		s1 := self.stack[i]
		s2 := other.stack[i]
		if s1.childIndex < s2.childIndex {
			return -1
		}
		if s1.childIndex > s2.childIndex {
			return 1
		}
		if s1.parentSymbol < s2.parentSymbol {
			return -1
		}
		if s1.parentSymbol > s2.parentSymbol {
			return 1
		}
		if s1.parseState < s2.parseState {
			return -1
		}
		if s1.parseState > s2.parseState {
			return 1
		}
		if s1.fieldID < s2.fieldID {
			return -1
		}
		if s1.fieldID > s2.fieldID {
			return 1
		}
	}
	if self.stepIndex < other.stepIndex {
		return -1
	}
	if self.stepIndex > other.stepIndex {
		return 1
	}
	return 0
}

// top is analysis_state__top.
func (s *analysisState) top() *analysisStateEntry {
	if s.depth == 0 {
		return &s.stack[0]
	}
	return &s.stack[s.depth-1]
}

// hasSupertype is analysis_state__has_supertype.
func (s *analysisState) hasSupertype(symbol Symbol) bool {
	for i := range s.depth {
		if s.stack[i].parentSymbol == symbol {
			return true
		}
	}
	return false
}

// AnalysisStateSet

// cloneOrReuse is analysis_state_pool__clone_or_reuse.
//
// Obtains an `AnalysisState` instance, either by consuming one from this set's object pool, or by
// cloning one from scratch.
func (s *analysisStateSet) cloneOrReuse(borrowedItem *analysisState) *analysisState {
	var newItem *analysisState
	if len(*s) > 0 {
		newItem = (*s)[len(*s)-1]
		*s = (*s)[:len(*s)-1]
	} else {
		newItem = new(analysisState)
	}
	*newItem = *borrowedItem
	return newItem
}

// insertSorted is analysis_state_set__insert_sorted.
//
// Inserts a clone of the passed-in item at the appropriate position to maintain ordering in this
// set. The set does not contain duplicates, so if the item is already present, it will not be
// inserted, and no clone will be made.
//
// The caller retains ownership of the passed-in memory. However, the clone that is created by this
// function will be managed by the state set.
func (s *analysisStateSet) insertSorted(pool *analysisStateSet, borrowedItem *analysisState) {
	index, exists := arraySearchSorted(*s, func(item **analysisState) int {
		return analysisStateCompare(*item, borrowedItem)
	})
	if !exists {
		newItem := pool.cloneOrReuse(borrowedItem)
		*s = slices.Insert(*s, int(index), newItem)
	}
}

// push is analysis_state_set__push.
//
// Inserts a clone of the passed-in item at the end position of this list.
//
// IMPORTANT: The caller MUST ENSURE that this item is larger (by the comparison function
// `analysis_state__compare`) than largest item already in this set. If items are inserted in the
// wrong order, the set will not function properly for future use.
//
// The caller retains ownership of the passed-in memory. However, the clone that is created by this
// function will be managed by the state set.
func (s *analysisStateSet) push(pool *analysisStateSet, borrowedItem *analysisState) {
	newItem := pool.cloneOrReuse(borrowedItem)
	*s = append(*s, newItem)
}

// clear is analysis_state_set__clear.
//
// Removes all items from this set, returning it to an empty state.
func (s *analysisStateSet) clear(pool *analysisStateSet) {
	*pool = append(*pool, *s...)
	*s = (*s)[:0]
}

// AnalysisSubgraphNode

// analysisSubgraphNodeCompare is analysis_subgraph_node__compare.
func analysisSubgraphNodeCompare(self, other *analysisSubgraphNode) int {
	if self.state < other.state {
		return -1
	}
	if self.state > other.state {
		return 1
	}
	if self.childIndex < other.childIndex {
		return -1
	}
	if self.childIndex > other.childIndex {
		return 1
	}
	if !self.done && other.done {
		return -1
	}
	if self.done && !other.done {
		return 1
	}
	if self.productionID < other.productionID {
		return -1
	}
	if self.productionID > other.productionID {
		return 1
	}
	return 0
}

// Query

// patternMapSearch is ts_query__pattern_map_search. The C function writes
// the index to an out parameter, and the Go function returns it.
//
// The `pattern_map` contains a mapping from TSSymbol values to indices in the
// `steps` array. For a given syntax node, the `pattern_map` makes it possible
// to quickly find the starting steps of all of the patterns whose root matches
// that node. Each entry has two fields: a `pattern_index`, which identifies one
// of the patterns in the query, and a `step_index`, which indicates the start
// offset of that pattern's steps within the `steps` array.
//
// The entries are sorted by the patterns' root symbols, and lookups use a
// binary search. This ensures that the cost of this initial lookup step
// scales logarithmically with the number of patterns in the query.
//
// This returns `true` if the symbol is present and `false` otherwise.
// If the symbol is not present `*result` is set to the index where the
// symbol should be inserted.
func (q *query) patternMapSearch(needle Symbol) (uint32, bool) {
	baseIndex := uint32(q.wildcardRootPatternCount)
	size := uint32(len(q.patternMap)) - baseIndex
	if size == 0 {
		return baseIndex, false
	}
	for size > 1 {
		halfSize := size / 2
		midIndex := baseIndex + halfSize
		midSymbol := q.steps[q.patternMap[midIndex].stepIndex].symbol
		if needle > midSymbol {
			baseIndex = midIndex
		}
		size -= halfSize
	}

	symbol := q.steps[q.patternMap[baseIndex].stepIndex].symbol

	if needle > symbol {
		baseIndex++
		if baseIndex < uint32(len(q.patternMap)) {
			symbol = q.steps[q.patternMap[baseIndex].stepIndex].symbol
		}
	}

	return baseIndex, needle == symbol
}

// patternMapInsert is ts_query__pattern_map_insert.
//
// Insert a new pattern's start index into the pattern map, maintaining
// the pattern map's ordering invariant.
func (q *query) patternMapInsert(symbol Symbol, newEntry patternEntry) {
	index, _ := q.patternMapSearch(symbol)

	// Ensure that the entries are sorted not only by symbol, but also
	// by pattern_index. This way, states for earlier patterns will be
	// initiated first, which allows the ordering of the states array
	// to be maintained more efficiently.
	for index < uint32(len(q.patternMap)) {
		entry := &q.patternMap[index]
		if q.steps[entry.stepIndex].symbol == symbol &&
			entry.patternIndex < newEntry.patternIndex {
			index++
		} else {
			break
		}
	}

	q.patternMap = slices.Insert(q.patternMap, int(index), newEntry)
}

// performAnalysis is ts_query__perform_analysis.
//
// Walk the subgraph for this non-terminal, tracking all of the possible
// sequences of progress within the pattern.
func (q *query) performAnalysis(subgraphs []analysisSubgraph, analysis *queryAnalysis) {
	recursionDepthLimit := uint32(0)
	prevFinalStepCount := uint32(0)
	analysis.finalStepIndices = analysis.finalStepIndices[:0]
	analysis.finishedParentSymbols = analysis.finishedParentSymbols[:0]

	for iteration := uint32(0); ; iteration++ {
		if iteration == maxAnalysisIterationCount {
			analysis.didAbort = true
			break
		}

		// If no further progress can be made within the current recursion depth limit, then
		// bump the depth limit by one, and continue to process the states the exceeded the
		// limit. But only allow this if progress has been made since the last time the depth
		// limit was increased.
		if len(analysis.states) == 0 {
			if len(analysis.deeperStates) > 0 &&
				uint32(len(analysis.finalStepIndices)) > prevFinalStepCount {
				prevFinalStepCount = uint32(len(analysis.finalStepIndices))
				recursionDepthLimit++
				analysis.states, analysis.deeperStates = analysis.deeperStates, analysis.states
				continue
			}

			break
		}

		analysis.nextStates.clear(&analysis.statePool)
		for j := 0; j < len(analysis.states); j++ {
			state := analysis.states[j]

			// For efficiency, it's important to avoid processing the same analysis state more
			// than once. To achieve this, keep the states in order of ascending position within
			// their hypothetical syntax trees. In each iteration of this loop, start by advancing
			// the states that have made the least progress. Avoid advancing states that have already
			// made more progress.
			if len(analysis.nextStates) > 0 {
				comparison := analysisStateCompare(
					state,
					analysis.nextStates[len(analysis.nextStates)-1],
				)
				if comparison == 0 {
					analysis.nextStates.insertSorted(&analysis.statePool, state)
					continue
				} else if comparison > 0 {
					for j < len(analysis.states) {
						analysis.nextStates.push(
							&analysis.statePool,
							analysis.states[j],
						)
						j++
					}
					break
				}
			}

			parseState := state.top().parseState
			parentSymbol := state.top().parentSymbol
			parentFieldID := state.top().fieldID
			childIndex := uint32(state.top().childIndex)
			step := &q.steps[state.stepIndex]

			subgraphIndex, exists := searchSubgraphs(subgraphs, parentSymbol)
			if !exists {
				continue
			}
			subgraph := &subgraphs[subgraphIndex]

			// Follow every possible path in the parse table, but only visit states that
			// are part of the subgraph for the current symbol.
			lookaheadIterator := q.language.lookaheads(parseState)
			for lookaheadIterator.next() {
				sym := lookaheadIterator.symbol

				successor := analysisSubgraphNode{
					state:      parseState,
					childIndex: bits7(childIndex),
				}
				if actionCount := len(lookaheadIterator.actions); actionCount > 0 {
					action := &lookaheadIterator.actions[actionCount-1].Action
					if action.Type == abi.ParseActionTypeShift {
						if !action.Shift.Extra {
							successor.state = StateID(action.Shift.State)
							successor.childIndex = bits7(uint32(successor.childIndex) + 1)
						}
					} else {
						continue
					}
				} else if lookaheadIterator.nextState != 0 {
					successor.state = lookaheadIterator.nextState
					successor.childIndex = bits7(uint32(successor.childIndex) + 1)
				} else {
					continue
				}

				nodeIndex, _ := searchSubgraphNodes(subgraph.nodes, &successor)
				for nodeIndex < uint32(len(subgraph.nodes)) {
					node := &subgraph.nodes[nodeIndex]
					nodeIndex++
					if node.state != successor.state || node.childIndex != successor.childIndex {
						break
					}

					// Use the subgraph to determine what alias and field will eventually be applied
					// to this child node.
					alias := q.language.aliasAt(uint32(node.productionID), childIndex)
					visibleSymbol := alias
					if alias == 0 {
						if q.language.tables.SymbolMetadata[sym].Visible {
							visibleSymbol = Symbol(q.language.tables.PublicSymbolMap[sym])
						} else {
							visibleSymbol = 0
						}
					}
					fieldID := parentFieldID
					if fieldID == 0 {
						for _, fieldMap := range q.language.fieldMap(uint32(node.productionID)) {
							if !fieldMap.Inherited && uint32(fieldMap.ChildIndex) == childIndex {
								fieldID = FieldID(fieldMap.FieldID)
								break
							}
						}
					}

					// Create a new state that has advanced past this hypothetical subtree.
					nextState := *state
					nextStateTop := nextState.top()
					nextStateTop.childIndex = uint16(successor.childIndex)
					nextStateTop.parseState = successor.state
					if node.done {
						nextStateTop.done = true
					}

					// Determine if this hypothetical child node would match the current step
					// of the query pattern.
					doesMatch := false

					// ERROR nodes can appear anywhere, so if the step is
					// looking for an ERROR node, consider it potentially matchable.
					switch {
					case step.symbol == builtinSymError:
						doesMatch = true
					case visibleSymbol != 0:
						doesMatch = true
						if step.symbol == wildcardSymbol {
							if step.isNamed &&
								!q.language.tables.SymbolMetadata[visibleSymbol].Named {
								doesMatch = false
							}
						} else if step.symbol != visibleSymbol {
							doesMatch = false
						}
						if step.field != 0 && step.field != fieldID {
							doesMatch = false
						}
						if step.supertypeSymbol != 0 &&
							!state.hasSupertype(step.supertypeSymbol) {
							doesMatch = false
						}

					// If this child is hidden, then descend into it and walk through its children.
					// If the top entry of the stack is at the end of its rule, then that entry can
					// be replaced. Otherwise, push a new entry onto the stack.
					case uint32(sym) >= q.language.tables.TokenCount:
						if !nextStateTop.done {
							if nextState.depth+1 >= maxAnalysisStateDepth {
								analysis.didAbort = true
								continue
							}

							nextState.depth++
							nextStateTop = nextState.top()
						}

						*nextStateTop = analysisStateEntry{
							parseState:   parseState,
							parentSymbol: sym,
							childIndex:   0,
							fieldID:      fieldID & fieldIDMask,
							done:         false,
						}

						if nextState.recursionDepth() > recursionDepthLimit {
							analysis.deeperStates.insertSorted(
								&analysis.statePool,
								&nextState,
							)
							continue
						}
					}

					// Pop from the stack when this state reached the end of its current syntax node.
					for nextState.depth > 0 && nextStateTop.done {
						nextState.depth--
						nextStateTop = nextState.top()
					}

					// If this hypothetical child did match the current step of the query pattern,
					// then advance to the next step at the current depth. This involves skipping
					// over any descendant steps of the current child.
					nextStep := step
					if doesMatch {
						for {
							nextState.stepIndex++
							nextStep = &q.steps[nextState.stepIndex]
							if nextStep.depth == patternDoneMarker ||
								nextStep.depth <= step.depth {
								break
							}
						}
					} else if successor.state == parseState {
						continue
					}

					for {
						// Skip pass-through states. Although these states have alternatives, they are only
						// used to implement repetitions, and query analysis does not need to process
						// repetitions in order to determine whether steps are possible and definite.
						if nextStep.isPassThrough {
							nextState.stepIndex++
							nextStep = &q.steps[nextState.stepIndex]
							continue
						}

						// If the pattern is finished or hypothetical parent node is complete, then
						// record that matching can terminate at this step of the pattern. Otherwise,
						// add this state to the list of states to process on the next iteration.
						if !nextStep.isDeadEnd {
							didFinishPattern := q.steps[nextState.stepIndex].depth != step.depth
							switch {
							case didFinishPattern:
								arrayInsertSortedBy(&analysis.finishedParentSymbols, state.rootSymbol)
							case nextState.depth == 0:
								arrayInsertSortedBy(&analysis.finalStepIndices, nextState.stepIndex)
							default:
								analysis.nextStates.insertSorted(&analysis.statePool, &nextState)
							}
						}

						// If the state has advanced to a step with an alternative step, then add another state
						// at that alternative step. This process is simpler than the process of actually matching a
						// pattern during query execution, because for the purposes of query analysis, there is no
						// need to process repetitions.
						if doesMatch &&
							nextStep.alternativeIndex != none &&
							nextStep.alternativeIndex > nextState.stepIndex {
							nextState.stepIndex = nextStep.alternativeIndex
							nextStep = &q.steps[nextState.stepIndex]
						} else {
							break
						}
					}
				}
			}
		}

		analysis.states, analysis.nextStates = analysis.nextStates, analysis.states
	}
}

// analyzePatterns is ts_query__analyze_patterns. The C function writes the
// error offset to an out parameter, and the Go function returns it. The goto
// of C is a return.
func (q *query) analyzePatterns() (errorOffset uint32, ok bool) {
	var nonRootedPatternStartSteps []uint16
	for i := range q.patternMap {
		pattern := &q.patternMap[i]
		if !pattern.isRooted {
			step := &q.steps[pattern.stepIndex]
			if step.symbol != wildcardSymbol {
				nonRootedPatternStartSteps = append(nonRootedPatternStartSteps, uint16(i))
			}
		}
	}

	// Walk forward through all of the steps in the query, computing some
	// basic information about each step. Mark all of the steps that contain
	// captures, and record the indices of all of the steps that have child steps.
	var parentStepIndices []uint32
	allPatternsAreValid := true
	for i := range uint32(len(q.steps)) {
		step := &q.steps[i]
		if step.depth == patternDoneMarker {
			step.parentPatternGuaranteed = true
			step.rootPatternGuaranteed = true
			continue
		}

		hasChildren := false
		isWildcard := step.symbol == wildcardSymbol
		step.containsCaptures = step.captureIDs[0] != none
		for j := i + 1; j < uint32(len(q.steps)); j++ {
			nextStep := &q.steps[j]
			if nextStep.depth == patternDoneMarker ||
				nextStep.depth <= step.depth {
				break
			}
			if nextStep.captureIDs[0] != none {
				step.containsCaptures = true
			}
			if !isWildcard {
				nextStep.rootPatternGuaranteed = true
				nextStep.parentPatternGuaranteed = true
			}
			hasChildren = true
		}

		if hasChildren {
			if !isWildcard {
				parentStepIndices = append(parentStepIndices, i)
			} else if step.supertypeSymbol != 0 && q.language.tables.ABIVersion >= languageVersionWithReservedWords {
				// Look at the child steps to see if any aren't valid subtypes for this supertype.
				subtypes := q.language.Subtypes(step.supertypeSymbol)

				for j := i + 1; j < uint32(len(q.steps)); j++ {
					childStep := &q.steps[j]
					if childStep.depth == patternDoneMarker || childStep.depth <= step.depth {
						break
					}
					if childStep.depth == step.depth+1 && childStep.symbol != wildcardSymbol {
						isValidSubtype := slices.Contains(subtypes, childStep.symbol)

						if !isValidSubtype {
							for offsetIdx := range q.stepOffsets {
								stepOffset := &q.stepOffsets[offsetIdx]
								if uint32(stepOffset.stepIndex) >= j {
									return stepOffset.byteOffset, false
								}
							}
						}
					}
				}
			}
		}
	}

	// For every parent symbol in the query, initialize an 'analysis subgraph'.
	// This subgraph lists all of the states in the parse table that are directly
	// involved in building subtrees for this symbol.
	//
	// In addition to the parent symbols in the query, construct subgraphs for all
	// of the hidden symbols in the grammar, because these might occur within
	// one of the parent nodes, such that their children appear to belong to the
	// parent.
	var subgraphs []analysisSubgraph
	insertSubgraph := func(subgraph analysisSubgraph) {
		index, exists := searchSubgraphs(subgraphs, subgraph.symbol)
		if !exists {
			subgraphs = slices.Insert(subgraphs, int(index), subgraph)
		}
	}
	for _, parentStepIndex := range parentStepIndices {
		parentSymbol := q.steps[parentStepIndex].symbol
		insertSubgraph(analysisSubgraph{symbol: parentSymbol})
	}
	for sym := Symbol(uint16(q.language.tables.TokenCount)); sym < Symbol(uint16(q.language.tables.SymbolCount)); sym++ {
		if !q.language.symbolMetadata(sym).Visible {
			insertSubgraph(analysisSubgraph{symbol: sym})
		}
	}

	// Scan the parse table to find the data needed to populate these subgraphs.
	// Collect three things during this scan:
	//   1) All of the parse states where one of these symbols can start.
	//   2) All of the parse states where one of these symbols can end, along
	//      with information about the node that would be created.
	//   3) A list of predecessor states for each state.
	predecessorMap := newStatePredecessorMap(q.language)
	for state := StateID(1); state < StateID(uint16(q.language.tables.StateCount)); state++ {
		lookaheadIterator := q.language.lookaheads(state)
		for lookaheadIterator.next() {
			if len(lookaheadIterator.actions) > 0 {
				for i := range lookaheadIterator.actions {
					action := &lookaheadIterator.actions[i].Action
					if action.Type == abi.ParseActionTypeReduce {
						aliases := q.language.aliasesForSymbol(Symbol(action.Reduce.Symbol))
						for _, symbol := range aliases {
							subgraphIndex, exists := searchSubgraphs(subgraphs, Symbol(symbol))
							if exists {
								subgraph := &subgraphs[subgraphIndex]
								if len(subgraph.nodes) == 0 || subgraph.nodes[len(subgraph.nodes)-1].state != state {
									subgraph.nodes = append(subgraph.nodes, analysisSubgraphNode{
										state:        state,
										productionID: action.Reduce.ProductionID,
										childIndex:   bits7(uint32(action.Reduce.ChildCount)),
										done:         true,
									})
								}
							}
						}
					} else if action.Type == abi.ParseActionTypeShift && !action.Shift.Extra {
						nextState := StateID(action.Shift.State)
						predecessorMap.add(nextState, state)
					}
				}
			} else if lookaheadIterator.nextState != 0 {
				if lookaheadIterator.nextState != state {
					predecessorMap.add(lookaheadIterator.nextState, state)
				}
				if q.language.stateIsPrimary(state) {
					aliases := q.language.aliasesForSymbol(lookaheadIterator.symbol)
					for _, symbol := range aliases {
						subgraphIndex, exists := searchSubgraphs(subgraphs, Symbol(symbol))
						if exists {
							subgraph := &subgraphs[subgraphIndex]
							if len(subgraph.startStates) == 0 ||
								subgraph.startStates[len(subgraph.startStates)-1] != state {
								subgraph.startStates = append(subgraph.startStates, state)
							}
						}
					}
				}
			}
		}
	}

	// For each subgraph, compute the preceding states by walking backward
	// from the end states using the predecessor map.
	var nextNodes []analysisSubgraphNode
	for i := 0; i < len(subgraphs); i++ {
		subgraph := &subgraphs[i]
		if len(subgraph.nodes) == 0 {
			subgraphs = slices.Delete(subgraphs, i, i+1)
			i--
			continue
		}
		nextNodes = append(nextNodes[:0], subgraph.nodes...)
		for len(nextNodes) > 0 {
			node := nextNodes[len(nextNodes)-1]
			nextNodes = nextNodes[:len(nextNodes)-1]
			if node.childIndex > 1 {
				predecessors := predecessorMap.get(node.state)
				for _, predecessor := range predecessors {
					predecessorNode := analysisSubgraphNode{
						state:        predecessor,
						childIndex:   bits7(uint32(node.childIndex) - 1),
						productionID: node.productionID,
						done:         false,
					}
					index, exists := searchSubgraphNodes(subgraph.nodes, &predecessorNode)
					if !exists {
						subgraph.nodes = slices.Insert(subgraph.nodes, int(index), predecessorNode)
						nextNodes = append(nextNodes, predecessorNode)
					}
				}
			}
		}
	}

	// For each non-terminal pattern, determine if the pattern can successfully match,
	// and identify all of the possible children within the pattern where matching could fail.
	var analysis queryAnalysis
	for _, index := range parentStepIndices {
		parentStepIndex := uint16(index)
		parentDepth := q.steps[parentStepIndex].depth
		parentSymbol := q.steps[parentStepIndex].symbol
		if parentSymbol == builtinSymError {
			continue
		}

		// Find the subgraph that corresponds to this pattern's root symbol. If the pattern's
		// root symbol is a terminal, then return an error.
		subgraphIndex, exists := searchSubgraphs(subgraphs, parentSymbol)
		if !exists {
			firstChildStepIndex := uint32(parentStepIndex) + 1
			j, childExists := arraySearchSorted(q.stepOffsets, func(s *stepOffset) int {
				return int(s.stepIndex) - int(firstChildStepIndex)
			})
			assert(childExists)
			errorOffset = q.stepOffsets[j].byteOffset
			allPatternsAreValid = false
			break
		}

		// Initialize an analysis state at every parse state in the table where
		// this parent symbol can occur.
		subgraph := &subgraphs[subgraphIndex]
		analysis.states.clear(&analysis.statePool)
		analysis.deeperStates.clear(&analysis.statePool)
		for _, parseState := range subgraph.startStates {
			analysis.states.push(&analysis.statePool, &analysisState{
				stepIndex: parentStepIndex + 1,
				stack: [maxAnalysisStateDepth]analysisStateEntry{
					0: {
						parseState:   parseState,
						parentSymbol: parentSymbol,
						childIndex:   0,
						fieldID:      0,
						done:         false,
					},
				},
				depth:      1,
				rootSymbol: parentSymbol,
			})
		}

		analysis.didAbort = false
		q.performAnalysis(subgraphs, &analysis)

		// If this pattern could not be fully analyzed, then every step should
		// be considered fallible.
		if analysis.didAbort {
			for j := uint32(parentStepIndex) + 1; j < uint32(len(q.steps)); j++ {
				step := &q.steps[j]
				if step.depth <= parentDepth ||
					step.depth == patternDoneMarker {
					break
				}
				if !step.isDeadEnd {
					step.parentPatternGuaranteed = false
					step.rootPatternGuaranteed = false
				}
			}
			continue
		}

		// If this pattern cannot match, store the pattern index so that it can be
		// returned to the caller.
		if len(analysis.finishedParentSymbols) == 0 {
			var impossibleStepIndex uint16
			if len(analysis.finalStepIndices) > 0 {
				impossibleStepIndex = analysis.finalStepIndices[len(analysis.finalStepIndices)-1]
			} else {
				// If there isn't a final step, then that means the parent step itself is unreachable.
				impossibleStepIndex = parentStepIndex
			}
			j, _ := arraySearchSorted(q.stepOffsets, func(s *stepOffset) int {
				return int(s.stepIndex) - int(impossibleStepIndex)
			})
			if j >= uint32(len(q.stepOffsets)) {
				j = uint32(len(q.stepOffsets)) - 1
			}
			errorOffset = q.stepOffsets[j].byteOffset
			allPatternsAreValid = false
			break
		}

		// Mark as fallible any step where a match terminated.
		// Later, this property will be propagated to all of the step's predecessors.
		for _, finalStepIndex := range analysis.finalStepIndices {
			step := &q.steps[finalStepIndex]
			if step.depth != patternDoneMarker &&
				step.depth > parentDepth &&
				!step.isDeadEnd {
				step.parentPatternGuaranteed = false
				step.rootPatternGuaranteed = false
			}
		}
	}

	// Mark as indefinite any step with captures that are used in predicates.
	var predicateCaptureIDs []uint16
	for i := range q.patterns {
		pattern := &q.patterns[i]

		// Gather all of the captures that are used in predicates for this pattern.
		predicateCaptureIDs = predicateCaptureIDs[:0]
		start := pattern.predicateSteps.offset
		end := start + pattern.predicateSteps.length
		for j := start; j < end; j++ {
			step := &q.predicateSteps[j]
			if step.typ == queryPredicateStepTypeCapture {
				valueID := uint16(step.valueID)
				arrayInsertSortedBy(&predicateCaptureIDs, valueID)
			}
		}

		// Find all of the steps that have these captures.
		start = pattern.steps.offset
		end = start + pattern.steps.length
		for j := start; j < end; j++ {
			step := &q.steps[j]
			for k := range maxStepCaptureCount {
				captureID := step.captureIDs[k]
				if captureID == none {
					break
				}
				_, exists := arraySearchSortedBy(predicateCaptureIDs, captureID)
				if exists {
					step.rootPatternGuaranteed = false
					break
				}
			}
		}
	}

	// Propagate fallibility. If a pattern is fallible at a given step, then it is
	// fallible at all of its preceding steps.
	done := len(q.steps) == 0
	for !done {
		done = true
		for i := uint32(len(q.steps)) - 1; i > 0; i-- {
			step := &q.steps[i]
			if step.depth == patternDoneMarker {
				continue
			}

			// Determine if this step is definite or has definite alternatives.
			parentPatternGuaranteed := false
			for {
				if step.rootPatternGuaranteed {
					parentPatternGuaranteed = true
					break
				}
				if step.alternativeIndex == none || uint32(step.alternativeIndex) < i {
					break
				}
				step = &q.steps[step.alternativeIndex]
			}

			// If not, mark its predecessor as indefinite.
			if !parentPatternGuaranteed {
				prevStep := &q.steps[i-1]
				if !prevStep.isDeadEnd &&
					prevStep.depth != patternDoneMarker &&
					prevStep.rootPatternGuaranteed {
					prevStep.rootPatternGuaranteed = false
					done = false
				}
			}
		}
	}

	// Determine which repetition symbols in this language have the possibility
	// of matching non-rooted patterns in this query. These repetition symbols
	// prevent certain optimizations with range restrictions.
	analysis.didAbort = false
	for _, patternEntryIndex := range nonRootedPatternStartSteps {
		patternEntry := &q.patternMap[patternEntryIndex]

		analysis.states.clear(&analysis.statePool)
		analysis.deeperStates.clear(&analysis.statePool)
		for j := range subgraphs {
			subgraph := &subgraphs[j]
			metadata := q.language.symbolMetadata(subgraph.symbol)
			if metadata.Visible || metadata.Named {
				continue
			}

			for _, parseState := range subgraph.startStates {
				analysis.states.push(&analysis.statePool, &analysisState{
					stepIndex: patternEntry.stepIndex,
					stack: [maxAnalysisStateDepth]analysisStateEntry{
						0: {
							parseState:   parseState,
							parentSymbol: subgraph.symbol,
							childIndex:   0,
							fieldID:      0,
							done:         false,
						},
					},
					rootSymbol: subgraph.symbol,
					depth:      1,
				})
			}
		}

		q.performAnalysis(
			subgraphs,
			&analysis,
		)

		if len(analysis.finishedParentSymbols) > 0 {
			q.patterns[patternEntry.patternIndex].isNonLocal = true
		}

		for _, symbol := range analysis.finishedParentSymbols {
			arrayInsertSortedBy(&q.repeatSymbolsWithRootlessPatterns, symbol)
		}
	}

	return errorOffset, allPatternsAreValid
}

// addNegatedFields is ts_query__add_negated_fields. The C function takes a
// pointer and a count, and the Go function takes a slice.
func (q *query) addNegatedFields(stepIndex uint16, fieldIDs []FieldID) {
	step := &q.steps[stepIndex]
	fieldCount := uint32(uint16(len(fieldIDs)))

	// The negated field array stores a list of field lists, separated by zeros.
	// Try to find the start index of an existing list that matches this new list.
	failedMatch := false
	matchCount := uint32(0)
	startI := uint32(0)
	for i := range uint32(len(q.negatedFields)) {
		existingFieldID := q.negatedFields[i]

		// At each zero value, terminate the match attempt. If we've exactly
		// matched the new field list, then reuse this index. Otherwise,
		// start over the matching process.
		switch {
		case existingFieldID == 0:
			if matchCount == fieldCount {
				step.negatedFieldListID = uint16(startI)
				return
			}
			startI = i + 1
			matchCount = 0
			failedMatch = false

		// If the existing list matches our new list so far, then advance
		// to the next element of the new list.
		case matchCount < fieldCount &&
			existingFieldID == fieldIDs[matchCount] &&
			!failedMatch:
			matchCount++

		// Otherwise, this existing list has failed to match.
		default:
			matchCount = 0
			failedMatch = true
		}
	}

	step.negatedFieldListID = uint16(len(q.negatedFields))
	q.negatedFields = append(q.negatedFields, fieldIDs[:fieldCount]...)
	q.negatedFields = append(q.negatedFields, 0)
}

// parseStringLiteral is ts_query__parse_string_literal.
func (q *query) parseStringLiteral(stream *stream) QueryErrorKind {
	stringStart := stream.input
	if stream.next != '"' {
		return QueryErrorSyntax
	}
	stream.advance()
	prevPosition := stream.input

	isEscaped := false
	q.stringBuffer = q.stringBuffer[:0]
	for {
		if isEscaped {
			isEscaped = false
			switch stream.next {
			case 'n':
				q.stringBuffer = append(q.stringBuffer, '\n')
			case 'r':
				q.stringBuffer = append(q.stringBuffer, '\r')
			case 't':
				q.stringBuffer = append(q.stringBuffer, '\t')
			case '0':
				q.stringBuffer = append(q.stringBuffer, 0)
			default:
				q.stringBuffer = append(q.stringBuffer, stream.source[stream.input:stream.input+uint32(stream.nextSize)]...)
			}
			prevPosition = stream.input + uint32(stream.nextSize)
		} else {
			switch stream.next {
			case '\\':
				q.stringBuffer = append(q.stringBuffer, stream.source[prevPosition:stream.input]...)
				prevPosition = stream.input + 1
				isEscaped = true
			case '"':
				q.stringBuffer = append(q.stringBuffer, stream.source[prevPosition:stream.input]...)
				stream.advance()
				return QueryErrorNone
			case '\n':
				stream.reset(stringStart)
				return QueryErrorSyntax
			}
		}
		if !stream.advance() {
			stream.reset(stringStart)
			return QueryErrorSyntax
		}
	}
}

// parsePredicate is ts_query__parse_predicate.
//
// Parse a single predicate associated with a pattern, adding it to the
// query's internal `predicate_steps` array. Predicates are arbitrary
// S-expressions associated with a pattern which are meant to be handled at
// a higher level of abstraction, such as the Rust/JavaScript bindings. They
// can contain '@'-prefixed capture names, double-quoted strings, and bare
// symbols, which also represent strings.
func (q *query) parsePredicate(stream *stream) QueryErrorKind {
	if !stream.isIdentStart() {
		return QueryErrorSyntax
	}
	predicateName := stream.input
	stream.scanIdentifier()
	if stream.next != '?' && stream.next != '!' {
		return QueryErrorSyntax
	}
	stream.advance()
	id := q.predicateValues.insertName(stream.source[predicateName:stream.input])
	q.predicateSteps = append(q.predicateSteps, queryPredicateStep{
		typ:     queryPredicateStepTypeString,
		valueID: uint32(id),
	})
	stream.skipWhitespace()

	for {
		switch {
		case stream.next == ')':
			stream.advance()
			stream.skipWhitespace()
			q.predicateSteps = append(q.predicateSteps, queryPredicateStep{
				typ:     queryPredicateStepTypeDone,
				valueID: 0,
			})
			return QueryErrorNone

		// Parse an '@'-prefixed capture name
		case stream.next == '@':
			stream.advance()

			// Parse the capture name
			if !stream.isIdentStart() {
				return QueryErrorSyntax
			}
			captureName := stream.input
			stream.scanIdentifier()

			// Add the capture id to the first step of the pattern
			captureID := q.captures.idForName(stream.source[captureName:stream.input])
			if captureID == -1 {
				stream.reset(captureName)
				return QueryErrorCapture
			}

			q.predicateSteps = append(q.predicateSteps, queryPredicateStep{
				typ:     queryPredicateStepTypeCapture,
				valueID: uint32(captureID),
			})

		// Parse a string literal
		case stream.next == '"':
			e := q.parseStringLiteral(stream)
			if e != QueryErrorNone {
				return e
			}
			queryID := q.predicateValues.insertName(q.stringBuffer)
			q.predicateSteps = append(q.predicateSteps, queryPredicateStep{
				typ:     queryPredicateStepTypeString,
				valueID: uint32(queryID),
			})

		// Parse a bare symbol
		case stream.isIdentStart():
			symbolStart := stream.input
			stream.scanIdentifier()
			queryID := q.predicateValues.insertName(stream.source[symbolStart:stream.input])
			q.predicateSteps = append(q.predicateSteps, queryPredicateStep{
				typ:     queryPredicateStepTypeString,
				valueID: uint32(queryID),
			})

		default:
			return QueryErrorSyntax
		}

		stream.skipWhitespace()
	}
}

// parsePattern is ts_query__parse_pattern.
//
// Read one S-expression pattern from the stream, and incorporate it into
// the query's internal state machine representation. For nested patterns,
// this function calls itself recursively.
//
// The caller is responsible for passing in a dedicated CaptureQuantifiers.
// These should not be shared between different calls to ts_query__parse_pattern!
func (q *query) parsePattern(
	stream *stream,
	depth uint32,
	isImmediate bool,
	isInsideAlternation bool,
	captureQuantifiers *captureQuantifierList,
) QueryErrorKind {
	if stream.next == 0 {
		return QueryErrorSyntax
	}
	if stream.next == ')' || stream.next == ']' {
		return parentDone
	}

	startingStepIndex := uint32(len(q.steps))

	// Store the byte offset of each step in the query.
	if len(q.stepOffsets) == 0 ||
		uint32(q.stepOffsets[len(q.stepOffsets)-1].stepIndex) != startingStepIndex {
		q.stepOffsets = append(q.stepOffsets, stepOffset{
			stepIndex:  uint16(startingStepIndex),
			byteOffset: stream.offset(),
		})
	}

	switch {
	// An open bracket is the start of an alternation.
	case stream.next == '[':
		stream.advance()
		stream.skipWhitespace()

		// Parse each branch, and add a placeholder step in between the branches.
		var branchStepIndices []uint32
		var branchCaptureQuantifiers captureQuantifierList
		for {
			startIndex := uint32(len(q.steps))
			e := q.parsePattern(
				stream,
				depth,
				isImmediate,
				true,
				&branchCaptureQuantifiers,
			)

			if e == parentDone {
				if stream.next == ']' && len(branchStepIndices) > 0 {
					stream.advance()
					break
				}
				e = QueryErrorSyntax
			}
			if e != QueryErrorNone {
				return e
			}

			if startIndex == startingStepIndex {
				captureQuantifiers.replace(branchCaptureQuantifiers)
			} else {
				captureQuantifiers.joinAll(branchCaptureQuantifiers)
			}

			branchStepIndices = append(branchStepIndices, startIndex)
			q.steps = append(q.steps, newQueryStep(0, uint16(depth), false))
			branchCaptureQuantifiers.clear()
		}
		q.steps = q.steps[:len(q.steps)-1]

		// For all of the branches except for the last one, add the subsequent branch as an
		// alternative, and link the end of the branch to the current end of the steps.
		for i := range len(branchStepIndices) - 1 {
			stepIndex := branchStepIndices[i]
			nextStepIndex := branchStepIndices[i+1]
			startStep := &q.steps[stepIndex]
			endStep := &q.steps[nextStepIndex-1]
			startStep.alternativeIndex = uint16(nextStepIndex)
			endStep.alternativeIndex = uint16(len(q.steps))
			endStep.isDeadEnd = true
		}

	// An open parenthesis can be the start of three possible constructs:
	// * A grouped sequence
	// * A predicate
	// * A named node
	case stream.next == '(':
		stream.advance()
		stream.skipWhitespace()

		switch stream.next {
		// If this parenthesis is followed by a node, then it represents a grouped sequence.
		case '(', '"', '[':
			childIsImmediate := isImmediate
			var childCaptureQuantifiers captureQuantifierList
			for {
				if stream.next == '.' {
					anchorStart := stream.input
					childIsImmediate = true
					stream.advance()
					stream.skipWhitespace()
					// A `.` at a group's end has no sibling to anchor, and a group is not a
					// node, so there is no last child to anchor against.
					if stream.next == ')' {
						stream.reset(anchorStart)
						return QueryErrorSyntax
					}
				}
				e := q.parsePattern(
					stream,
					depth,
					childIsImmediate,
					isInsideAlternation,
					&childCaptureQuantifiers,
				)
				if e == parentDone {
					if stream.next == ')' {
						stream.advance()
						break
					}
					e = QueryErrorSyntax
				}
				if e != QueryErrorNone {
					return e
				}

				captureQuantifiers.addAll(childCaptureQuantifiers)
				childCaptureQuantifiers.clear()
				childIsImmediate = false
			}

		// A dot/pound character indicates the start of a predicate.
		case '.', '#':
			stream.advance()
			return q.parsePredicate(stream)

		// Otherwise, this parenthesis is the start of a named node.
		default:
			var symbol Symbol
			isMissing := false
			nodeName := stream.input

			// Parse a normal node name
			if !stream.isIdentStart() {
				return QueryErrorSyntax
			}
			stream.scanIdentifier()
			length := stream.input - nodeName

			// Parse the wildcard symbol
			switch {
			case length == 1 && stream.source[nodeName] == '_':
				symbol = wildcardSymbol
			case length == 7 && string(stream.source[nodeName:nodeName+length]) == "MISSING":
				isMissing = true
				stream.skipWhitespace()

				switch {
				case stream.isIdentStart():
					missingNodeName := stream.input
					stream.scanIdentifier()
					symbol, _ = q.language.SymbolForName(
						string(stream.source[missingNodeName:stream.input]),
						true,
					)
					if symbol == 0 {
						stream.reset(missingNodeName)
						return QueryErrorNodeType
					}

				case stream.next == '"':
					stringStart := stream.input
					e := q.parseStringLiteral(stream)
					if e != QueryErrorNone {
						return e
					}

					symbol, _ = q.language.SymbolForName(
						string(q.stringBuffer),
						false,
					)
					if symbol == 0 {
						stream.reset(stringStart + 1)
						return QueryErrorNodeType
					}

				case stream.next == ')':
					symbol = wildcardSymbol

				default:
					stream.reset(stream.input)
					return QueryErrorSyntax
				}

			default:
				symbol, _ = q.language.SymbolForName(
					string(stream.source[nodeName:nodeName+length]),
					true,
				)
				if symbol == 0 {
					stream.reset(nodeName)
					return QueryErrorNodeType
				}
			}

			// Add a step for the node.
			q.steps = append(q.steps, newQueryStep(symbol, uint16(depth), isImmediate))
			step := &q.steps[len(q.steps)-1]
			if q.language.symbolMetadata(symbol).Supertype {
				step.supertypeSymbol = step.symbol
				step.symbol = wildcardSymbol
			}
			if isMissing {
				step.isMissing = true
			}
			if symbol == wildcardSymbol {
				step.isNamed = true
			}

			// Parse a supertype symbol
			if stream.next == '/' {
				if step.supertypeSymbol == 0 {
					stream.reset(nodeName - 1) // reset to the start of the node
					return QueryErrorStructure
				}

				stream.advance()

				subtypeNodeName := stream.input

				switch {
				case stream.isIdentStart(): // Named node
					stream.scanIdentifier()
					length := stream.input - subtypeNodeName
					step.symbol, _ = q.language.SymbolForName(
						string(stream.source[subtypeNodeName:subtypeNodeName+length]),
						true,
					)
				case stream.next == '"': // Anonymous leaf node
					e := q.parseStringLiteral(stream)
					if e != QueryErrorNone {
						return e
					}
					step.symbol, _ = q.language.SymbolForName(
						string(q.stringBuffer),
						false,
					)
				default:
					return QueryErrorSyntax
				}

				if step.symbol == 0 {
					stream.reset(subtypeNodeName)
					return QueryErrorNodeType
				}

				// Get all the possible subtypes for the given supertype,
				// and check if the given subtype is valid.
				if q.language.tables.ABIVersion >= languageVersionWithReservedWords {
					subtypes := q.language.Subtypes(step.supertypeSymbol)

					subtypeIsValid := slices.Contains(subtypes, step.symbol)

					// This subtype is not valid for the given supertype.
					if !subtypeIsValid {
						stream.reset(nodeName - 1) // reset to the start of the node
						return QueryErrorStructure
					}
				}
			}

			stream.skipWhitespace()

			// Parse the child patterns
			childIsImmediate := false
			lastChildStepIndex := uint16(0)
			negatedFieldCount := uint16(0)
			var negatedFieldIDs [maxNegatedFieldCount]FieldID
			var childCaptureQuantifiers captureQuantifierList
			for {
				// Parse a negated field assertion
				if stream.next == '!' {
					stream.advance()
					stream.skipWhitespace()
					if !stream.isIdentStart() {
						return QueryErrorSyntax
					}
					fieldName := stream.input
					stream.scanIdentifier()
					length := stream.input - fieldName
					stream.skipWhitespace()

					fieldID, _ := q.language.FieldForName(
						string(stream.source[fieldName : fieldName+length]),
					)
					if fieldID == 0 {
						stream.input = fieldName
						return QueryErrorField
					}

					// Keep the field ids sorted.
					if negatedFieldCount < maxNegatedFieldCount {
						negatedFieldIDs[negatedFieldCount] = fieldID
						negatedFieldCount++
					}

					continue
				}

				// Parse a sibling anchor
				if stream.next == '.' {
					childIsImmediate = true
					stream.advance()
					stream.skipWhitespace()
				}

				stepIndex := uint16(len(q.steps))
				e := q.parsePattern(
					stream,
					depth+1,
					childIsImmediate,
					isInsideAlternation,
					&childCaptureQuantifiers,
				)
				// In the event we only parsed a predicate, meaning no new steps were added,
				// then subtract one so we're not indexing past the end of the array
				if stepIndex == uint16(len(q.steps)) {
					stepIndex--
				}
				if e == parentDone {
					if stream.next == ')' {
						if childIsImmediate {
							if lastChildStepIndex == 0 {
								return QueryErrorSyntax
							}
							// Mark this step *and* its alternatives as the last child of the parent.
							lastChildStep := &q.steps[lastChildStepIndex]
							lastChildStep.isLastChild = true
							if lastChildStep.alternativeIndex != none &&
								uint32(lastChildStep.alternativeIndex) < uint32(len(q.steps)) {
								alternativeStep := &q.steps[lastChildStep.alternativeIndex]
								alternativeStep.isLastChild = true
								for alternativeStep.alternativeIndex != none &&
									uint32(alternativeStep.alternativeIndex) < uint32(len(q.steps)) {
									alternativeStep = &q.steps[alternativeStep.alternativeIndex]
									alternativeStep.isLastChild = true
								}
							}
						}

						if negatedFieldCount != 0 {
							q.addNegatedFields(
								uint16(startingStepIndex),
								negatedFieldIDs[:negatedFieldCount],
							)
						}

						stream.advance()
						break
					}
					e = QueryErrorSyntax
				}
				if e != QueryErrorNone {
					return e
				}

				captureQuantifiers.addAll(childCaptureQuantifiers)

				lastChildStepIndex = stepIndex
				childIsImmediate = false
				childCaptureQuantifiers.clear()
			}
		}

	// Parse a wildcard pattern
	case stream.next == '_':
		stream.advance()
		stream.skipWhitespace()

		// Add a step that matches any kind of node
		q.steps = append(q.steps, newQueryStep(wildcardSymbol, uint16(depth), isImmediate))

	// Parse a double-quoted anonymous leaf node expression
	case stream.next == '"':
		stringStart := stream.input
		e := q.parseStringLiteral(stream)
		if e != QueryErrorNone {
			return e
		}

		// Add a step for the node
		symbol, _ := q.language.SymbolForName(
			string(q.stringBuffer),
			false,
		)
		if symbol == 0 {
			stream.reset(stringStart + 1)
			return QueryErrorNodeType
		}
		q.steps = append(q.steps, newQueryStep(symbol, uint16(depth), isImmediate))

	// Parse a field-prefixed pattern
	case stream.isIdentStart():
		// Parse the field name
		fieldName := stream.input
		stream.scanIdentifier()
		length := stream.input - fieldName
		stream.skipWhitespace()

		if stream.next != ':' {
			stream.reset(fieldName)
			return QueryErrorSyntax
		}
		stream.advance()
		stream.skipWhitespace()

		// Parse the pattern
		var fieldCaptureQuantifiers captureQuantifierList
		e := q.parsePattern(
			stream,
			depth,
			isImmediate,
			isInsideAlternation,
			&fieldCaptureQuantifiers,
		)
		if e != QueryErrorNone {
			if e == parentDone {
				e = QueryErrorSyntax
			}
			return e
		}

		// Add the field name to the first step of the pattern
		fieldID, _ := q.language.FieldForName(
			string(stream.source[fieldName : fieldName+length]),
		)
		if fieldID == 0 {
			stream.input = fieldName
			return QueryErrorField
		}

		stepIndex := startingStepIndex
		step := &q.steps[stepIndex]
		for {
			step.field = fieldID
			if step.alternativeIndex != none &&
				uint32(step.alternativeIndex) > stepIndex &&
				uint32(step.alternativeIndex) < uint32(len(q.steps)) {
				stepIndex = uint32(step.alternativeIndex)
				step = &q.steps[stepIndex]
			} else {
				break
			}
		}

		captureQuantifiers.addAll(fieldCaptureQuantifiers)

	default:
		return QueryErrorSyntax
	}

	stream.skipWhitespace()

	// Parse suffixes modifiers for this pattern
	quantifier := QuantifierOne
suffixes:
	for {
		switch stream.next {
		// Parse the one-or-more operator.
		case '+':
			quantifier = quantifierJoin(QuantifierOneOrMore, quantifier)

			stream.advance()
			stream.skipWhitespace()

		// Parse the zero-or-more repetition operator.
		case '*':
			quantifier = quantifierJoin(QuantifierZeroOrMore, quantifier)

			stream.advance()
			stream.skipWhitespace()

		// Parse the optional operator.
		case '?':
			quantifier = quantifierJoin(QuantifierZeroOrOne, quantifier)

			stream.advance()
			stream.skipWhitespace()

		// Parse an '@'-prefixed capture pattern
		case '@':
			stream.advance()
			if !stream.isIdentStart() {
				return QueryErrorSyntax
			}
			captureName := stream.input
			stream.scanIdentifier()
			length := stream.input - captureName
			stream.skipWhitespace()

			// Add the capture id to the first step of the pattern
			captureID := q.captures.insertName(stream.source[captureName : captureName+length])

			// Add the capture quantifier
			captureQuantifiers.addForID(captureID, QuantifierOne)

			stepIndex := startingStepIndex
			for {
				step := &q.steps[stepIndex]
				step.addCapture(captureID)
				if step.alternativeIndex != none &&
					uint32(step.alternativeIndex) > stepIndex &&
					uint32(step.alternativeIndex) < uint32(len(q.steps)) {
					stepIndex = uint32(step.alternativeIndex)
				} else {
					break
				}
			}

		// No more suffix modifiers
		default:
			break suffixes
		}
	}

	switch quantifier {
	case QuantifierOneOrMore:
		repeatStep := newQueryStep(wildcardSymbol, uint16(depth), false)
		repeatStep.isInsideAlternation = isInsideAlternation
		repeatStep.alternativeIndex = uint16(startingStepIndex)
		repeatStep.isPassThrough = true
		q.steps = append(q.steps, repeatStep)
	case QuantifierZeroOrMore:
		repeatStep := newQueryStep(wildcardSymbol, uint16(depth), false)
		repeatStep.isInsideAlternation = isInsideAlternation
		repeatStep.alternativeIndex = uint16(startingStepIndex)
		repeatStep.isPassThrough = true
		q.steps = append(q.steps, repeatStep)

		// Stop when `step->alternative_index` is `NONE` or it points to
		// `repeat_step` or beyond. Note that having just been pushed,
		// `repeat_step` occupies slot `self->steps.size - 1`.
		step := &q.steps[startingStepIndex]
		for step.alternativeIndex != none && uint32(step.alternativeIndex) < uint32(len(q.steps))-1 {
			step = &q.steps[step.alternativeIndex]
		}
		step.alternativeIndex = uint16(len(q.steps))
		step.alternativeIsSkip = true
	case QuantifierZeroOrOne:
		step := &q.steps[startingStepIndex]
		for step.alternativeIndex != none && uint32(step.alternativeIndex) < uint32(len(q.steps)) {
			step = &q.steps[step.alternativeIndex]
		}
		step.alternativeIndex = uint16(len(q.steps))
		step.alternativeIsSkip = true
	default:
	}

	captureQuantifiers.mul(quantifier)

	return QueryErrorNone
}

// newQuery is ts_query_new. The C function writes the error offset and the
// kind of error to out parameters, and the Go function returns them. It
// returns a nil query when the kind is not QueryErrorNone. The error offset
// is 0 for a query with no error, and for a language that the runtime does
// not accept.
func newQuery(language *Language, source string) (q *query, errorOffset uint32, errorType QueryErrorKind) {
	if language == nil ||
		language.tables.ABIVersion > languageVersion ||
		language.tables.ABIVersion < minCompatibleLanguageVersion {
		return nil, 0, QueryErrorLanguage
	}

	q = &query{
		wildcardRootPatternCount: 0,
		language:                 language,
	}

	q.negatedFields = append(q.negatedFields, 0)

	// Parse all of the S-expressions in the given string.
	stream := newStream([]byte(source))
	stream.skipWhitespace()
	for stream.input < uint32(len(stream.source)) {
		patternIndex := uint32(len(q.patterns))
		startStepIndex := uint32(len(q.steps))
		startPredicateStepIndex := uint32(len(q.predicateSteps))
		q.patterns = append(q.patterns, queryPattern{
			steps:          slice{offset: startStepIndex},
			predicateSteps: slice{offset: startPredicateStepIndex},
			startByte:      stream.offset(),
			isNonLocal:     false,
		})
		var captureQuantifiers captureQuantifierList
		errorType = q.parsePattern(&stream, 0, false, false, &captureQuantifiers)
		q.steps = append(q.steps, newQueryStep(0, patternDoneMarker, false))

		pattern := &q.patterns[len(q.patterns)-1]
		pattern.steps.length = uint32(len(q.steps)) - startStepIndex
		pattern.predicateSteps.length = uint32(len(q.predicateSteps)) - startPredicateStepIndex
		pattern.endByte = stream.offset()

		// If any pattern could not be parsed, then report the error information
		// and terminate.
		if errorType != QueryErrorNone {
			if errorType == parentDone {
				errorType = QueryErrorSyntax
			}
			errorOffset = stream.offset()
			return nil, errorOffset, errorType
		}

		// Maintain a list of capture quantifiers for each pattern
		q.captureQuantifiers = append(q.captureQuantifiers, captureQuantifiers)

		// Maintain a map that can look up patterns for a given root symbol.
		wildcardRootAlternativeIndex := none
	rootSteps:
		for {
			step := &q.steps[startStepIndex]

			// If a pattern has a wildcard at its root, but it has a non-wildcard child,
			// then optimize the matching process by skipping matching the wildcard.
			// Later, during the matching process, the query cursor will check that
			// there is a parent node, and capture it if necessary.
			if step.symbol == wildcardSymbol && step.depth == 0 && step.field == 0 {
				secondStep := &q.steps[startStepIndex+1]
				if secondStep.symbol != wildcardSymbol && secondStep.depth == 1 && !secondStep.isImmediate {
					wildcardRootAlternativeIndex = step.alternativeIndex
					startStepIndex++
					step = secondStep
				}
			}

			// Determine whether the pattern has a single root node. This affects
			// decisions about whether or not to start matching the pattern when
			// a query cursor has a range restriction or when immediately within an
			// error node.
			startDepth := uint32(step.depth)
			isRooted := startDepth == 0
			for stepIndex := startStepIndex + 1; stepIndex < uint32(len(q.steps)); stepIndex++ {
				childStep := &q.steps[stepIndex]
				if childStep.isDeadEnd {
					break
				}
				if uint32(childStep.depth) == startDepth {
					isRooted = false
					break
				}
			}

			q.patternMapInsert(step.symbol, patternEntry{
				stepIndex:    uint16(startStepIndex),
				patternIndex: uint16(patternIndex),
				isRooted:     isRooted,
			})
			if step.symbol == wildcardSymbol {
				q.wildcardRootPatternCount++
			}

			// If there are alternatives or options at the root of the pattern,
			// then add multiple entries to the pattern map.
			switch {
			case step.alternativeIndex != none:
				startStepIndex = uint32(step.alternativeIndex)
			case wildcardRootAlternativeIndex != none:
				startStepIndex = uint32(wildcardRootAlternativeIndex)
				wildcardRootAlternativeIndex = none
			default:
				break rootSteps
			}
		}

		// Fix up quantifier loop-backs within alternations. When a branch of an
		// alternation has a + or * quantifier, the quantifier's pass_through step
		// loops back to the branch's first step. However, the alternation linking
		// assigns that same step's `alternative_index` to point to the _next_ branch.
		// This causes the quantifier loop to incorrectly explore other alternation branches,
		// when a quantified branch matches, loops back, and then fails to match. To correct
		// this, we create "clean" copies of the branches' first steps without the link to the
		// next branch. After a quantified branch matches, it loops back to the cleaned copy.
		patStart := pattern.steps.offset
		patEnd := patStart + pattern.steps.length - 1 // exclude DONE

		for i := patStart; i < patEnd; i++ {
			s := &q.steps[i]
			// Ensure this step is a pass_through with a _backward_ alternative (a quantifier loop-back)
			if !s.isPassThrough || !s.isInsideAlternation ||
				s.alternativeIndex == none || uint32(s.alternativeIndex) >= i {
				continue
			}

			targetIdx := uint32(s.alternativeIndex)
			target := &q.steps[targetIdx]

			// Check if the target has a forward alternative from alternation linking
			targetAltIndex := target.alternativeIndex
			if targetAltIndex == none ||
				uint32(targetAltIndex) <= targetIdx || uint32(targetAltIndex) >= patEnd {
				continue
			}

			// Create a clean copy of the target step without the alternation alternative.
			copyIdx := uint32(len(q.steps))
			copied := *target
			copied.alternativeIndex = none
			targetDepth := target.depth
			q.steps = append(q.steps, copied)

			// Add a dead_end that redirects to the pass through step after the target,
			// so the pattern continues correctly after the cleaned copy matches.
			redirect := newQueryStep(0, targetDepth, false)
			redirect.isDeadEnd = true
			redirect.alternativeIndex = uint16(targetIdx + 1)
			q.steps = append(q.steps, redirect)

			// Update the pass_through to loop back to the copy. Reacquire `s` since
			// `self->steps` may have been reallocated.
			s = &q.steps[i]
			s.alternativeIndex = uint16(copyIdx)
		}
	}

	if errorOffset, ok := q.analyzePatterns(); !ok {
		return nil, errorOffset, QueryErrorStructure
	}

	q.stringBuffer = nil
	return q, 0, QueryErrorNone
}

// patternCount is ts_query_pattern_count.
func (q *query) patternCount() uint32 {
	return uint32(len(q.patterns))
}

// captureCount is ts_query_capture_count.
func (q *query) captureCount() uint32 {
	return uint32(len(q.captures.slices))
}

// stringCount is ts_query_string_count.
func (q *query) stringCount() uint32 {
	return uint32(len(q.predicateValues.slices))
}

// captureNameForID is ts_query_capture_name_for_id. The C function returns
// a pointer and writes the length, and the Go function returns the name.
func (q *query) captureNameForID(index uint32) string {
	return q.captures.nameForID(uint16(index))
}

// captureQuantifierForID is ts_query_capture_quantifier_for_id.
func (q *query) captureQuantifierForID(patternIndex, captureIndex uint32) Quantifier {
	captureQuantifiers := q.captureQuantifiers[patternIndex]
	return captureQuantifiers.forID(uint16(captureIndex))
}

// stringValueForID is ts_query_string_value_for_id. The C function returns
// a pointer and writes the length, and the Go function returns the string.
func (q *query) stringValueForID(index uint32) string {
	return q.predicateValues.nameForID(uint16(index))
}

// predicatesForPattern is ts_query_predicates_for_pattern. The C function
// returns a pointer and writes the count, and the Go function returns a
// slice of the steps of the query. The slice has no room past its length.
func (q *query) predicatesForPattern(patternIndex uint32) []queryPredicateStep {
	slice := q.patterns[patternIndex].predicateSteps
	if slice.length == 0 {
		return nil
	}
	end := slice.offset + slice.length
	return q.predicateSteps[slice.offset:end:end]
}

// startByteForPattern is ts_query_start_byte_for_pattern.
func (q *query) startByteForPattern(patternIndex uint32) uint32 {
	return q.patterns[patternIndex].startByte
}

// endByteForPattern is ts_query_end_byte_for_pattern.
func (q *query) endByteForPattern(patternIndex uint32) uint32 {
	return q.patterns[patternIndex].endByte
}

// isPatternRooted is ts_query_is_pattern_rooted.
func (q *query) isPatternRooted(patternIndex uint32) bool {
	for i := range q.patternMap {
		entry := &q.patternMap[i]
		if uint32(entry.patternIndex) == patternIndex {
			if !entry.isRooted {
				return false
			}
		}
	}
	return true
}

// isPatternNonLocal is ts_query_is_pattern_non_local.
func (q *query) isPatternNonLocal(patternIndex uint32) bool {
	if patternIndex < uint32(len(q.patterns)) {
		return q.patterns[patternIndex].isNonLocal
	}
	return false
}

// isPatternGuaranteedAtStep is ts_query_is_pattern_guaranteed_at_step.
func (q *query) isPatternGuaranteedAtStep(byteOffset uint32) bool {
	stepIndex := uint32(math.MaxUint32)
	for i := range q.stepOffsets {
		stepOffset := &q.stepOffsets[i]
		if stepOffset.byteOffset > byteOffset {
			break
		}
		stepIndex = uint32(stepOffset.stepIndex)
	}
	if stepIndex < uint32(len(q.steps)) {
		return q.steps[stepIndex].rootPatternGuaranteed
	}
	return false
}

// stepIsFallible is ts_query__step_is_fallible.
func (q *query) stepIsFallible(stepIndex uint16) bool {
	i := uint32(1)
	step := &q.steps[stepIndex]
	var nextStep *queryStep
	for {
		assert(uint32(stepIndex)+i < uint32(len(q.steps)))
		nextStep = &q.steps[uint32(stepIndex)+i]
		i++
		if !nextStep.isPassThrough {
			break
		}
	}
	return nextStep.depth != patternDoneMarker &&
		(nextStep.depth > step.depth ||
			(nextStep.depth == step.depth && nextStep.isImmediate)) &&
		(!nextStep.parentPatternGuaranteed || step.symbol == wildcardSymbol)
}

// disableCapture is ts_query_disable_capture.
func (q *query) disableCapture(name string) {
	// Remove capture information for any pattern step that previously
	// captured with the given name.
	id := q.captures.idForName([]byte(name))
	if id != -1 {
		for i := range q.steps {
			step := &q.steps[i]
			step.removeCapture(uint16(id))
		}
	}
}

// disablePattern is ts_query_disable_pattern.
func (q *query) disablePattern(patternIndex uint32) {
	// Remove the given pattern from the pattern map. Its steps will still
	// be in the `steps` array, but they will never be read.
	for i := 0; i < len(q.patternMap); i++ {
		pattern := &q.patternMap[i]
		if uint32(pattern.patternIndex) == patternIndex {
			q.patternMap = slices.Delete(q.patternMap, i, i+1)
			i--
		}
	}
}

// QueryCursor

// newQueryCursor is ts_query_cursor_new.
func newQueryCursor() *queryCursor {
	return &queryCursor{
		exceededMatchLimit: false,
		ascending:          false,
		halted:             false,
		states:             make([]queryState, 0, 8),
		finishedStates:     make([]queryState, 0, 8),
		captureListPool:    newCaptureListPool(),
		includedRange: textRange{
			startPoint: point{0, 0},
			endPoint:   pointMax,
			startByte:  0,
			endByte:    math.MaxUint32,
		},
		containingRange: textRange{
			startPoint: point{0, 0},
			endPoint:   pointMax,
			startByte:  0,
			endByte:    math.MaxUint32,
		},
		maxStartDepth:  math.MaxUint32,
		operationCount: 0,
	}
}

// didExceedMatchLimit is ts_query_cursor_did_exceed_match_limit.
func (c *queryCursor) didExceedMatchLimit() bool {
	return c.exceededMatchLimit
}

// matchLimit is ts_query_cursor_match_limit.
func (c *queryCursor) matchLimit() uint32 {
	return c.captureListPool.maxCaptureListCount
}

// setMatchLimit is ts_query_cursor_set_match_limit.
func (c *queryCursor) setMatchLimit(limit uint32) {
	c.captureListPool.maxCaptureListCount = limit
}

// exec is ts_query_cursor_exec.
func (c *queryCursor) exec(query *query, node Node) {
	c.states = c.states[:0]
	c.finishedStates = c.finishedStates[:0]
	c.finishedStatesHeapSize = 0
	c.cursor.Reset(node)
	c.captureListPool.reset()
	c.onVisibleNode = true
	c.nextStateID = 0
	c.nextFinishedStateID = 0
	c.depth = 0
	c.ascending = false
	c.halted = false
	c.query = query
	c.exceededMatchLimit = false
	c.operationCount = 0
}

// setByteRange is ts_query_cursor_set_byte_range.
func (c *queryCursor) setByteRange(startByte, endByte uint32) bool {
	if endByte == 0 {
		endByte = math.MaxUint32
	}
	if startByte > endByte {
		return false
	}
	c.includedRange.startByte = startByte
	c.includedRange.endByte = endByte
	return true
}

// setPointRange is ts_query_cursor_set_point_range.
func (c *queryCursor) setPointRange(startPoint, endPoint point) bool {
	if endPoint.row == 0 && endPoint.column == 0 {
		endPoint = pointMax
	}
	if startPoint.gt(endPoint) {
		return false
	}
	c.includedRange.startPoint = startPoint
	c.includedRange.endPoint = endPoint
	return true
}

// setContainingByteRange is ts_query_cursor_set_containing_byte_range.
func (c *queryCursor) setContainingByteRange(startByte, endByte uint32) bool {
	if endByte == 0 {
		endByte = math.MaxUint32
	}
	if startByte > endByte {
		return false
	}
	c.containingRange.startByte = startByte
	c.containingRange.endByte = endByte
	return true
}

// setContainingPointRange is ts_query_cursor_set_containing_point_range.
func (c *queryCursor) setContainingPointRange(startPoint, endPoint point) bool {
	if endPoint.row == 0 && endPoint.column == 0 {
		endPoint = pointMax
	}
	if startPoint.gt(endPoint) {
		return false
	}
	c.containingRange.startPoint = startPoint
	c.containingRange.endPoint = endPoint
	return true
}

// firstInProgressCapture is ts_query_cursor__first_in_progress_capture.
// The C function writes its results to out parameters, and the Go function
// returns them. C reads is_definite only when the pointer is not NULL, and
// Go reads wantDefinite in its place. isDefinite is false when wantDefinite
// is false.
//
// Search through all of the in-progress states, and find the captured
// node that occurs earliest in the document.
func (c *queryCursor) firstInProgressCapture(wantDefinite bool) (
	stateIndex uint32,
	byteOffset uint32,
	patternIndex uint32,
	isDefinite bool,
	result bool,
) {
	result = false
	stateIndex = math.MaxUint32
	byteOffset = math.MaxUint32
	patternIndex = math.MaxUint32
	for i := uint32(0); i < uint32(len(c.states)); i++ {
		state := &c.states[i]
		if state.dead {
			continue
		}

		captures := c.captureListPool.get(state.captureListID)
		if uint32(state.consumedCaptureCount) >= uint32(len(captures)) {
			continue
		}

		node := captures[state.consumedCaptureCount].node
		if uint32(node.EndByte()) <= c.includedRange.startByte ||
			node.EndPoint().internal().lte(c.includedRange.startPoint) {
			state.consumedCaptureCount = (state.consumedCaptureCount + 1) & consumedCaptureCountMask
			i--
			continue
		}

		nodeStartByte := uint32(node.StartByte())
		if !result ||
			nodeStartByte < byteOffset ||
			(nodeStartByte == byteOffset && uint32(state.patternIndex) < patternIndex) {
			step := &c.query.steps[state.stepIndex]
			if wantDefinite {
				// We're being a bit conservative here by asserting that the following step
				// is not immediate, because this capture might end up being discarded if the
				// following symbol in the tree isn't the required symbol for this step.
				isDefinite = step.rootPatternGuaranteed && !step.isImmediate
			} else if step.rootPatternGuaranteed {
				continue
			}

			result = true
			stateIndex = i
			byteOffset = nodeStartByte
			patternIndex = uint32(state.patternIndex)
		}
	}
	return stateIndex, byteOffset, patternIndex, isDefinite, result
}

// compareNodes is ts_query_cursor__compare_nodes.
//
// Determine which node is first in a depth-first traversal.
func compareNodes(left, right Node) int {
	if left.id != right.id {
		leftStart := uint32(left.StartByte())
		rightStart := uint32(right.StartByte())
		if leftStart < rightStart {
			return -1
		}
		if leftStart > rightStart {
			return 1
		}
		leftNodeCount := uint32(left.EndByte())
		rightNodeCount := uint32(right.EndByte())
		if leftNodeCount > rightNodeCount {
			return -1
		}
		if leftNodeCount < rightNodeCount {
			return 1
		}
	}
	return 0
}

// compareCaptures is ts_query_cursor__compare_captures. The C function
// writes its results to out parameters, and the Go function returns them.
//
// Determine if either state contains a superset of the other state's captures.
func (c *queryCursor) compareCaptures(
	leftState *queryState,
	rightState *queryState,
) (leftContainsRight, rightContainsLeft bool) {
	leftCaptures := c.captureListPool.get(leftState.captureListID)
	rightCaptures := c.captureListPool.get(rightState.captureListID)
	leftContainsRight = true
	rightContainsLeft = true
	i, j := 0, 0
	for {
		if i < len(leftCaptures) {
			if j < len(rightCaptures) {
				left := &leftCaptures[i]
				right := &rightCaptures[j]
				if left.node.id == right.node.id && left.index == right.index {
					i++
					j++
				} else {
					switch compareNodes(left.node, right.node) {
					case -1:
						rightContainsLeft = false
						i++
					case 1:
						leftContainsRight = false
						j++
					default:
						rightContainsLeft = false
						leftContainsRight = false
						i++
						j++
					}
				}
			} else {
				rightContainsLeft = false
				break
			}
		} else {
			if j < len(rightCaptures) {
				leftContainsRight = false
			}
			break
		}
	}
	return leftContainsRight, rightContainsLeft
}

// statePrecedes is ts_query_cursor__state_precedes.
//
// Order two in-progress states for the longest-match dedup pass. Within a
// (start_depth, pattern_index) group, states with no captures sort first, as they are a
// subset of every other state (so the dedup pass must always compare them). The rest
// sort by the start byte of their first capture.
func (c *queryCursor) statePrecedes(a, b *queryState) bool {
	if a.startDepth != b.startDepth {
		return a.startDepth < b.startDepth
	}
	if a.patternIndex != b.patternIndex {
		return a.patternIndex < b.patternIndex
	}
	aCaps := c.captureListPool.get(a.captureListID)
	bCaps := c.captureListPool.get(b.captureListID)
	if (len(aCaps) == 0) != (len(bCaps) == 0) {
		return len(aCaps) == 0
	}
	if len(aCaps) == 0 {
		return false
	}
	return uint32(aCaps[0].node.StartByte()) <
		uint32(bCaps[0].node.StartByte())
}

// sortStatesByCapture is ts_query_cursor__sort_states_by_capture.
//
// Stable-sort the in-progress states with the order dictated by `ts_query_cursor__state_precedes`.
// This runs once per node, right before the dedup pass.
func (c *queryCursor) sortStatesByCapture() {
	states := c.states
	for i := 1; i < len(states); i++ {
		// Fast+common path: this state is already ordered after its predecessor, so it does not need
		// to move.
		if !c.statePrecedes(&states[i], &states[i-1]) {
			continue
		}
		key := states[i]
		j := i
		for {
			states[j] = states[j-1]
			j--
			if !(j > 0 && c.statePrecedes(&key, &states[j-1])) {
				break
			}
		}
		states[j] = key
	}
}

// addState is ts_query_cursor__add_state.
func (c *queryCursor) addState(pattern *patternEntry) {
	step := &c.query.steps[pattern.stepIndex]
	startDepth := c.depth - uint32(step.depth)

	// Keep the states array in ascending order of start_depth and pattern_index,
	// so that it can be processed more efficiently elsewhere. Usually, there is
	// no work to do here because of two facts:
	// * States with lower start_depth are naturally added first due to the
	//   order in which nodes are visited.
	// * Earlier patterns are naturally added first because of the ordering of the
	//   pattern_map data structure that's used to initiate matches.
	//
	// This loop is only needed in cases where two conditions hold:
	// * A pattern consists of more than one sibling node, so that its states
	//   remain in progress after exiting the node that started the match.
	// * The first node in the pattern matches against multiple nodes at the
	//   same depth.
	//
	// An example of this is the pattern '((comment)* (function))'. If multiple
	// `comment` nodes appear in a row, then we may initiate a new state for this
	// pattern while another state for the same pattern is already in progress.
	// If there are multiple patterns like this in a query, then this loop will
	// need to execute in order to keep the states ordered by pattern_index.
	index := uint32(len(c.states))
	for index > 0 {
		prevState := &c.states[index-1]
		if uint32(prevState.startDepth) < startDepth {
			break
		}
		if uint32(prevState.startDepth) == startDepth {
			// Avoid inserting an unnecessary duplicate state, which would be
			// immediately pruned by the longest-match criteria.
			if prevState.patternIndex == pattern.patternIndex &&
				prevState.stepIndex == pattern.stepIndex {
				return
			}
			if prevState.patternIndex <= pattern.patternIndex {
				break
			}
		}
		index--
	}

	c.states = slices.Insert(c.states, int(index), queryState{
		id:                        math.MaxUint32,
		captureListID:             captureListNone,
		heapInsertOrder:           math.MaxUint32,
		stepIndex:                 pattern.stepIndex,
		patternIndex:              pattern.patternIndex,
		startDepth:                uint16(startDepth),
		consumedCaptureCount:      0,
		seekingImmediateMatch:     true,
		hasInProgressAlternatives: false,
		needsParent:               step.depth == 1,
		dead:                      false,
		skippedQuantifier:         false,
	})
}

// prepareToCapture is ts_query_cursor__prepare_to_capture. It returns nil
// where C returns NULL.
//
// Acquire a capture list for this state. If there are no capture lists left in the
// pool, this will steal the capture list from another existing state, and mark that
// other state as 'dead'.
func (c *queryCursor) prepareToCapture(state *queryState, stateIndexToPreserve uint32) *captureList {
	if state.captureListID == captureListNone {
		state.captureListID = c.captureListPool.acquire()

		// If there are no capture lists left in the pool, then terminate whichever
		// state has captured the earliest node in the document, and steal its
		// capture list.
		if state.captureListID == captureListNone {
			c.exceededMatchLimit = true
			stateIndex, _, _, _, ok := c.firstInProgressCapture(false)
			if ok && stateIndex != stateIndexToPreserve {
				otherState := &c.states[stateIndex]
				state.captureListID = otherState.captureListID
				otherState.captureListID = captureListNone
				otherState.dead = true
				list := c.captureListPool.getMut(state.captureListID)
				list.contents = list.contents[:0]
				return list
			}
			return nil
		}
	}
	return c.captureListPool.getMut(state.captureListID)
}

// capture is ts_query_cursor__capture.
func (c *queryCursor) capture(state *queryState, step *queryStep, node Node) {
	if state.dead {
		return
	}
	captureList := c.prepareToCapture(state, math.MaxUint32)
	if captureList == nil {
		state.dead = true
		return
	}

	for j := range maxStepCaptureCount {
		captureID := step.captureIDs[j]
		if step.captureIDs[j] == none {
			break
		}
		captureList.contents = append(captureList.contents, queryCapture{node, uint32(captureID)})
	}
}

// copyState is ts_query_cursor__copy_state. The C function takes a pointer
// to the pointer of the state, and it updates the pointer after the insert.
// The Go function takes the index of the state, and the caller takes the
// state again from its index. It returns nil where C returns NULL.
//
// Duplicate the given state and insert the newly-created state immediately after
// the given state in the `states` array. Ensures that the given state reference is
// still valid, even if the states array is reallocated.
func (c *queryCursor) copyState(stateIndex uint32) *queryState {
	state := &c.states[stateIndex]
	copied := *state
	copied.captureListID = captureListNone

	// If the state has captures, copy its capture list.
	if state.captureListID != captureListNone {
		newCaptures := c.prepareToCapture(&copied, stateIndex)
		if newCaptures == nil {
			return nil
		}
		oldCaptures := c.captureListPool.get(state.captureListID)
		newCaptures.contents = append(newCaptures.contents, oldCaptures...)
	}

	c.states = slices.Insert(c.states, int(stateIndex)+1, copied)
	return &c.states[stateIndex+1]
}

// shouldDescend is ts_query_cursor__should_descend.
func (c *queryCursor) shouldDescend(nodeIntersectsRange bool) bool {
	if nodeIntersectsRange && c.depth < c.maxStartDepth {
		return true
	}

	// If there are in-progress matches whose remaining steps occur
	// deeper in the tree, then descend.
	for i := range c.states {
		state := &c.states[i]
		nextStep := &c.query.steps[state.stepIndex]
		if nextStep.depth != patternDoneMarker &&
			uint32(state.startDepth)+uint32(nextStep.depth) > c.depth {
			return true
		}
	}

	if c.depth >= c.maxStartDepth {
		return false
	}

	// If the current node is hidden, then a non-rooted pattern might match
	// one if its roots inside of this node, and match another of its roots
	// as part of a sibling node, so we may need to descend.
	if !c.onVisibleNode {
		// Descending into a repetition node outside of the range can be
		// expensive, because these nodes can have many visible children.
		// Avoid descending into repetition nodes unless we have already
		// determined that this query can match rootless patterns inside
		// of this type of repetition node.
		subtree := c.cursor.currentSubtree()
		if subtree.isRepetition() {
			_, exists := arraySearchSortedBy(
				c.query.repeatSymbolsWithRootlessPatterns,
				subtree.symbol(),
			)
			return exists
		}

		return true
	}

	return false
}

// rangeIntersects is range_intersects.
func rangeIntersects(a, b *textRange) bool {
	isEmpty := a.startByte == a.endByte
	return (a.endByte > b.startByte ||
		(isEmpty && a.endByte == b.startByte)) &&
		(a.endPoint.gt(b.startPoint) ||
			(isEmpty && a.endPoint.eq(b.startPoint))) &&
		a.startByte < b.endByte &&
		a.startPoint.lt(b.endPoint)
}

// rangeWithin is range_within.
func rangeWithin(a, b *textRange) bool {
	return a.startByte >= b.startByte &&
		a.startPoint.gte(b.startPoint) &&
		a.endByte <= b.endByte &&
		a.endPoint.lte(b.endPoint)
}

// advance is ts_query_cursor__advance. It reads ctx.Err() where C calls the
// progress callback, and it stops as C stops when the callback returns true.
//
// Walk the tree, processing patterns until at least one pattern finishes,
// If one or more patterns finish, return `true` and store their states in the
// `finished_states` array. Multiple patterns can finish on the same node. If
// there are no more matches, return `false`.
func (c *queryCursor) advance(ctx context.Context, stopOnDefiniteStep bool) bool {
	didMatch := false
	for {
		if c.halted {
			for len(c.states) > 0 {
				state := c.states[len(c.states)-1]
				c.states = c.states[:len(c.states)-1]
				c.captureListPool.release(
					state.captureListID,
				)
			}
		}

		c.operationCount++
		if c.operationCount == opCountPerQueryCallbackCheck {
			c.operationCount = 0
		}

		if didMatch ||
			c.halted ||
			(c.operationCount == 0 && ctx.Err() != nil) {
			return didMatch
		}

		// Exit the current node.
		if c.ascending {
			if c.onVisibleNode {
				// After leaving a node, remove any states that cannot make further progress.
				deletedCount := uint32(0)
				for i, n := uint32(0), uint32(len(c.states)); i < n; i++ {
					state := &c.states[i]
					step := &c.query.steps[state.stepIndex]

					switch {
					// If a state completed its pattern inside of this node, but was deferred from finishing
					// in order to search for longer matches, mark it as finished.
					case step.depth == patternDoneMarker &&
						(uint32(state.startDepth) > c.depth || c.depth == 0):
						c.pushFinishedState(state)
						didMatch = true
						deletedCount++

					// If a state needed to match something within this node, then remove that state
					// as it has failed to match.
					case step.depth != patternDoneMarker &&
						uint32(state.startDepth)+uint32(step.depth) > c.depth:
						c.captureListPool.release(
							state.captureListID,
						)
						deletedCount++

					case deletedCount > 0:
						c.states[i-deletedCount] = *state
					}
				}
				c.states = c.states[:uint32(len(c.states))-deletedCount]
			}

			// Leave this node by stepping to its next sibling or to its parent.
			switch c.cursor.gotoNextSiblingInternal() {
			case treeCursorStepVisible:
				if !c.onVisibleNode {
					c.depth++
					c.onVisibleNode = true
				}
				c.ascending = false
			case treeCursorStepHidden:
				if c.onVisibleNode {
					c.depth--
					c.onVisibleNode = false
				}
				c.ascending = false
			default:
				if c.cursor.GotoParent() {
					c.depth--
				} else {
					c.halted = true
				}
			}
		} else {
			// Enter a new node.
			node := c.cursor.Node()
			parentNode := c.cursor.parentNode()

			parentIntersectsRange := parentNode.isNull()
			if !parentIntersectsRange {
				parentRange := parentNode.Range().internal()
				parentIntersectsRange = rangeIntersects(&parentRange, &c.includedRange)
			}
			nodeRange := node.Range().internal()
			nodeIntersectsRange := parentIntersectsRange && rangeIntersects(&nodeRange, &c.includedRange)
			nodeIntersectsContainingRange := rangeIntersects(&nodeRange, &c.containingRange)
			nodeWithinContainingRange := rangeWithin(&nodeRange, &c.containingRange)

			if nodeWithinContainingRange && c.onVisibleNode {
				symbol := node.KindID()
				isNamed := node.IsNamed()
				isMissing := node.IsMissing()
				var supertypes [8]Symbol
				fieldID,
					hasLaterSiblings,
					hasLaterNamedSiblings,
					canHaveLaterSiblingsWithThisField,
					supertypeCount := c.cursor.currentStatus(supertypes[:])

				nodeIsError := symbol == builtinSymError
				parentIsError := !parentNode.isNull() &&
					parentNode.KindID() == builtinSymError

				// Add new states for any patterns whose root node is a wildcard.
				if !nodeIsError {
					for i := range uint32(c.query.wildcardRootPatternCount) {
						pattern := &c.query.patternMap[i]

						// If this node matches the first step of the pattern, then add a new
						// state at the start of this pattern.
						step := &c.query.steps[pattern.stepIndex]
						startDepth := c.depth - uint32(step.depth)
						intersects := parentIntersectsRange && !parentIsError
						if pattern.isRooted {
							intersects = nodeIntersectsRange
						}
						if intersects &&
							(step.field == 0 || fieldID == step.field) &&
							(step.supertypeSymbol == 0 || supertypeCount > 0) &&
							startDepth <= c.maxStartDepth {
							c.addState(pattern)
						}
					}
				}

				// Add new states for any patterns whose root node matches this node.
				if i, ok := c.query.patternMapSearch(symbol); ok {
					pattern := &c.query.patternMap[i]

					step := &c.query.steps[pattern.stepIndex]
					startDepth := c.depth - uint32(step.depth)
					for {
						// If this node matches the first step of the pattern, then add a new
						// state at the start of this pattern.
						intersects := parentIntersectsRange && !parentIsError
						if pattern.isRooted {
							intersects = nodeIntersectsRange
						}
						if intersects &&
							(step.field == 0 || fieldID == step.field) &&
							startDepth <= c.maxStartDepth {
							c.addState(pattern)
						}

						// Advance to the next pattern whose root node matches this node.
						i++
						if i == uint32(len(c.query.patternMap)) {
							break
						}
						pattern = &c.query.patternMap[i]
						step = &c.query.steps[pattern.stepIndex]
						if step.symbol != symbol {
							break
						}
					}
				}

				// Update all of the in-progress states with current node.
				var copyCount uint32
				for j := uint32(0); j < uint32(len(c.states)); j += 1 + copyCount {
					state := &c.states[j]
					step := &c.query.steps[state.stepIndex]
					state.hasInProgressAlternatives = false
					copyCount = 0

					// Check that the node matches all of the criteria for the next
					// step of the pattern.
					if uint32(state.startDepth)+uint32(step.depth) != c.depth {
						continue
					}

					// Determine if this node matches this step of the pattern, and also
					// if this node can have later siblings that match this step of the
					// pattern.
					var nodeDoesMatch bool
					if step.symbol == wildcardSymbol {
						if step.isMissing {
							nodeDoesMatch = isMissing
						} else {
							nodeDoesMatch = !nodeIsError && (isNamed || !step.isNamed)
						}
					} else {
						nodeDoesMatch = symbol == step.symbol && (!step.isMissing || isMissing)
					}
					laterSiblingCanMatch := hasLaterSiblings
					if (step.isImmediate && isNamed && !state.skippedQuantifier) || state.seekingImmediateMatch {
						laterSiblingCanMatch = false
					}
					if step.isLastChild && hasLaterNamedSiblings {
						nodeDoesMatch = false
					}
					if step.supertypeSymbol != 0 {
						hasSupertype := false
						for k := range supertypeCount {
							if supertypes[k] == step.supertypeSymbol {
								hasSupertype = true
								break
							}
						}
						if !hasSupertype {
							nodeDoesMatch = false
						}
					}
					if step.field != 0 {
						if step.field == fieldID {
							if !canHaveLaterSiblingsWithThisField {
								laterSiblingCanMatch = false
							}
						} else {
							nodeDoesMatch = false
						}
					}

					if step.negatedFieldListID != 0 {
						negatedFieldIDs := c.query.negatedFields[step.negatedFieldListID:]
						for {
							negatedFieldID := negatedFieldIDs[0]
							if negatedFieldID == 0 {
								break
							}
							negatedFieldIDs = negatedFieldIDs[1:]
							if _, ok := node.ChildByFieldID(negatedFieldID); ok {
								nodeDoesMatch = false
								break
							}
						}
					}

					// Remove states immediately if it is ever clear that they cannot match.
					if !nodeDoesMatch {
						if !laterSiblingCanMatch {
							c.captureListPool.release(
								state.captureListID,
							)
							c.states = slices.Delete(c.states, int(j), int(j)+1)
							j--
						}
						continue
					}

					// Some patterns can match their root node in multiple ways, capturing different
					// children. If this pattern step could match later children within the same
					// parent, then this query state cannot simply be updated in place. It must be
					// split into two states: one that matches this node, and one which skips over
					// this node, to preserve the possibility of matching later siblings.
					if laterSiblingCanMatch && (step.containsCaptures ||
						c.query.stepIsFallible(state.stepIndex)) {
						if c.copyState(j) != nil {
							copyCount++
						}
						state = &c.states[j]
					}

					// If this pattern started with a wildcard, such that the pattern map
					// actually points to the *second* step of the pattern, then check
					// that the node has a parent, and capture the parent node if necessary.
					if state.needsParent {
						parent := c.cursor.parentNode()
						if parent.isNull() {
							state.dead = true
						} else {
							state.needsParent = false
							skippedWildcardStepIndex := state.stepIndex
							for {
								skippedWildcardStepIndex--
								skippedWildcardStep := &c.query.steps[skippedWildcardStepIndex]
								if !(skippedWildcardStep.isDeadEnd ||
									skippedWildcardStep.isPassThrough ||
									skippedWildcardStep.depth > 0) {
									break
								}
							}
							skippedWildcardStep := &c.query.steps[skippedWildcardStepIndex]
							if skippedWildcardStep.captureIDs[0] != none {
								c.capture(
									state,
									skippedWildcardStep,
									parent,
								)
							}
						}
					}

					// If the current node is captured in this pattern, add it to the capture list.
					if step.captureIDs[0] != none {
						c.capture(state, step, node)
					}

					if state.dead {
						c.states = slices.Delete(c.states, int(j), int(j)+1)
						j--
						continue
					}

					// Advance this state to the next step of its pattern.
					state.stepIndex++

					nextStep := &c.query.steps[state.stepIndex]

					// For a given step, if the current symbol is the wildcard symbol, `_`, and it is **not**
					// named, meaning it should capture anonymous nodes, **and** the next step is immediate,
					// we reuse the `seeking_immediate_match` flag to indicate that we are looking for an
					// immediate match due to an unnamed wildcard symbol.
					//
					// The reason for this is that typically, anchors will not consider anonymous nodes,
					// but we're special casing the wildcard symbol to allow for any immediate matches,
					// regardless of whether they are named or not.
					if step.symbol == wildcardSymbol && !step.isNamed && nextStep.isImmediate {
						state.seekingImmediateMatch = true
					} else {
						state.seekingImmediateMatch = false
					}
					// The zero-skip's vacuous-anchor exemption only covers the immediate
					// step it lands on. Once the state advances, a later anchor is normal.
					state.skippedQuantifier = false

					if stopOnDefiniteStep && nextStep.rootPatternGuaranteed {
						didMatch = true
					}

					// If this state's next step has an alternative step, then copy the state in order
					// to pursue both alternatives. The alternative step itself may have an alternative,
					// so this is an interactive process.
					endIndex := j + 1
					for k := j; k < endIndex; k++ {
						// childStateIndex is the index of child_state, because
						// the C code changes k before it copies the state.
						childStateIndex := k
						childState := &c.states[childStateIndex]
						childStep := &c.query.steps[childState.stepIndex]
						if childStep.alternativeIndex != none {
							// A "dead-end" step exists only to add a non-sequential jump into the step sequence,
							// via its alternative index. When a state reaches a dead-end step, it jumps straight
							// to the step's alternative.
							if childStep.isDeadEnd {
								childState.stepIndex = childStep.alternativeIndex
								k--
								continue
							}

							// A "pass-through" step exists only to add a branch into the step sequence,
							// via its alternative_index. When a state reaches a pass-through step, it splits
							// in order to process the alternative step, and then it advances to the next step.
							if childStep.isPassThrough {
								childState.stepIndex++
								k--
							}

							// A `?`/`*` zero-skip past a step that carries a trailing last-child
							// anchor transfers that requirement to the last matched node. The
							// skip is only valid if that node really is the last named child.
							if childStep.alternativeIsSkip &&
								childStep.isLastChild &&
								hasLaterNamedSiblings {
								continue
							}

							copied := c.copyState(childStateIndex)
							childState = &c.states[childStateIndex]
							if copied != nil {
								endIndex++
								copyCount++
								copied.stepIndex = childStep.alternativeIndex
								if childStep.isPassThrough {
									copied.seekingImmediateMatch = true
								}
								// Taking a `?`/`*` zero-skip means the quantified subpattern matched
								// nothing. How an adjacent anchor behaves then depends on where it sat:
								if childStep.alternativeIsSkip {
									if !childStep.isImmediate {
										skipTarget := &c.query.steps[childStep.alternativeIndex]
										// No leading anchor on the skipped step, so an immediately-following
										// anchor on the skip target is vacuous (`Q* . B` with zero `Q` lets
										// `B` match anywhere).
										copied.skippedQuantifier = skipTarget.depth == childStep.depth
									} else if c.query.steps[childState.stepIndex-1].depth <
										childStep.depth {
										// The skipped step was the parent's first child pattern and carried a
										// leading *boundary* anchor (`(P . Q* Y)`). Transfer the first-child
										// requirement to the skip target so it survives the empty run: `Y`
										// must still be the parent's first named child.
										copied.seekingImmediateMatch = true
									}
									// Otherwise the skipped step carried a leading *between* anchor
									// (`A . Q* ...`): with zero `Q` that adjacency vanishes, while the skip
									// target's own anchor, if any, still applies (`A . Q* . B` stays adjacent).
								}
							}
						}
					}
				}

				// Order states by capture position so the dedup pass below can stop scanning a
				// group once the remaining states are disjoint from the current one.
				c.sortStatesByCapture()

				for j := uint32(0); j < uint32(len(c.states)); j++ {
					state := &c.states[j]
					if state.dead {
						c.states = slices.Delete(c.states, int(j), int(j)+1)
						j--
						continue
					}

					// Enforce the longest-match criteria. When a query pattern contains optional or
					// repeated nodes, this is necessary to avoid multiple redundant states, where
					// one state has a strict subset of another state's captures.
					didRemove := false
					for k := j + 1; k < uint32(len(c.states)); k++ {
						otherState := &c.states[k]

						// Query states are kept in ascending order of start_depth and pattern_index, and
						// (via the above call to `ts_query_cursor__sort_states_by_capture`) in ascending
						// order of first-capture position within each such group.
						//
						// Since the longest-match criteria is only used for deduping matches of the same
						// pattern and root node, we only need to perform pairwise comparisons within a
						// small slice of the states array.
						if otherState.startDepth != state.startDepth ||
							otherState.patternIndex != state.patternIndex {
							break
						}

						// States in a group acquire their first capture in tree-traversal order, so the
						// group is ordered by first-capture position. Once `other_state`'s captures begin
						// at or after where `state`'s captures end, `other_state` (and every state after
						// it in the group) is disjoint from `state`: neither can be a capture-subset of
						// the other, so there is nothing to drop and no longest-match alternative to
						// record. Stop scanning `state` against the rest of the group.
						stateCaptures := c.captureListPool.get(state.captureListID)
						otherCaptures := c.captureListPool.get(otherState.captureListID)
						if len(stateCaptures) > 0 &&
							len(otherCaptures) > 0 &&
							uint32(otherCaptures[0].node.StartByte()) >=
								uint32(stateCaptures[len(stateCaptures)-1].node.EndByte()) {
							break
						}

						leftContainsRight, rightContainsLeft := c.compareCaptures(
							state,
							otherState,
						)
						if leftContainsRight {
							if state.stepIndex == otherState.stepIndex &&
								(otherState.seekingImmediateMatch || !state.seekingImmediateMatch) {
								c.captureListPool.release(otherState.captureListID)
								c.states = slices.Delete(c.states, int(k), int(k)+1)
								k--
								continue
							}
							otherState.hasInProgressAlternatives = true
						}
						if rightContainsLeft {
							if state.stepIndex == otherState.stepIndex &&
								(state.seekingImmediateMatch || !otherState.seekingImmediateMatch) {
								c.captureListPool.release(state.captureListID)
								c.states = slices.Delete(c.states, int(j), int(j)+1)
								j--
								didRemove = true
								break
							}
							state.hasInProgressAlternatives = true
						}
					}

					// If the state is at the end of its pattern, remove it from the list
					// of in-progress states and add it to the list of finished states.
					if !didRemove {
						nextStep := &c.query.steps[state.stepIndex]
						if nextStep.depth == patternDoneMarker {
							if !state.hasInProgressAlternatives {
								c.pushFinishedState(state)
								c.states = slices.Delete(c.states, int(j), int(j)+1)
								didMatch = true
								j--
							}
						}
					}
				}
			}

			if nodeIntersectsContainingRange && c.shouldDescend(nodeIntersectsRange) {
				switch c.cursor.gotoFirstChildInternal() {
				case treeCursorStepVisible:
					c.depth++
					c.onVisibleNode = true
					continue
				case treeCursorStepHidden:
					c.onVisibleNode = false
					continue
				default:
				}
			}

			c.ascending = true
		}
	}
}

// nextMatch is ts_query_cursor_next_match. The C function writes the match
// to an out parameter, and the Go function returns it. The captures of the
// match alias the capture list of the cursor, as queryMatch says.
func (c *queryCursor) nextMatch(ctx context.Context) (queryMatch, bool) {
	if len(c.finishedStates) == 0 {
		if !c.advance(ctx, false) {
			return queryMatch{}, false
		}
	}
	if c.finishedStatesHeapSize > 0 {
		c.heapifyFinishedStates()
	}

	stateIndex := uint32(0)
	if c.finishedStatesHeapSize > 0 {
		for i := uint32(1); i < uint32(len(c.finishedStates)); i++ {
			state := &c.finishedStates[i]
			earliestState := &c.finishedStates[stateIndex]
			if state.heapInsertOrder < earliestState.heapInsertOrder {
				stateIndex = i
			}
		}
	}

	state := &c.finishedStates[stateIndex]
	if state.id == math.MaxUint32 {
		state.id = c.nextStateID
		c.nextStateID++
	}
	captures := c.captureListPool.get(state.captureListID)
	match := queryMatch{
		id:           state.id,
		patternIndex: state.patternIndex,
		captures:     slices.Clip(captures),
	}
	c.captureListPool.release(state.captureListID)
	if c.finishedStatesHeapSize > 0 {
		finishedStateErase(&c.finishedStates, stateIndex, &c.captureListPool)
		c.finishedStatesHeapSize = uint32(len(c.finishedStates))
	} else {
		c.finishedStates = slices.Delete(c.finishedStates, int(stateIndex), int(stateIndex)+1)
	}
	return match, true
}

// removeMatch is ts_query_cursor_remove_match.
func (c *queryCursor) removeMatch(matchID uint32) {
	if c.finishedStatesHeapSize > 0 {
		c.heapifyFinishedStates()
	}

	for i := range uint32(len(c.finishedStates)) {
		state := &c.finishedStates[i]
		if state.id == matchID {
			c.captureListPool.release(
				state.captureListID,
			)
			if c.finishedStatesHeapSize > 0 {
				finishedStateErase(&c.finishedStates, i, &c.captureListPool)
				c.finishedStatesHeapSize = uint32(len(c.finishedStates))
			} else {
				c.finishedStates = slices.Delete(c.finishedStates, int(i), int(i)+1)
			}
			return
		}
	}

	// Remove unfinished query states as well to prevent future
	// captures for a match being removed.
	for i := range c.states {
		state := &c.states[i]
		if state.id == matchID {
			c.captureListPool.release(
				state.captureListID,
			)
			c.states = slices.Delete(c.states, i, i+1)
			return
		}
	}
}

// nextCapture is ts_query_cursor_next_capture. The C function writes the
// match and the index of the capture to out parameters, and the Go function
// returns them. The captures of the match alias the capture list of the
// cursor, as queryMatch says.
func (c *queryCursor) nextCapture(ctx context.Context) (queryMatch, uint32, bool) {
	// The goal here is to return captures in order, even though they may not
	// be discovered in order, because patterns can overlap. Search for matches
	// until there is a finished capture that is before any unfinished capture.
	for {
		// Sift any newly pushed finished states into the heap.
		c.heapifyFinishedStates()

		// First, find the earliest capture in an unfinished match.
		firstUnfinishedStateIndex,
			firstUnfinishedCaptureByte,
			firstUnfinishedPatternIndex,
			firstUnfinishedStateIsDefinite,
			foundUnfinishedState := c.firstInProgressCapture(true)

		// Then find the earliest capture in a finished match. The finished_states
		// array is maintained as a min-heap, so the earliest is always at index 0.
		// Clean up fully-consumed and out-of-range states from the heap root first.
		var firstFinishedState *queryState
		firstFinishedCaptureByte := firstUnfinishedCaptureByte
		firstFinishedPatternIndex := firstUnfinishedPatternIndex
		for len(c.finishedStates) > 0 {
			state := &c.finishedStates[0]
			captures := c.captureListPool.get(state.captureListID)

			// Remove states whose captures are all consumed.
			if uint32(state.consumedCaptureCount) >= uint32(len(captures)) {
				c.captureListPool.release(
					state.captureListID,
				)
				finishedStatePop(&c.finishedStates, &c.captureListPool)
				c.finishedStatesHeapSize = uint32(len(c.finishedStates))
				continue
			}

			node := captures[state.consumedCaptureCount].node

			nodePrecedesRange := uint32(node.EndByte()) <= c.includedRange.startByte ||
				node.EndPoint().internal().lte(c.includedRange.startPoint)
			nodeFollowsRange := uint32(node.StartByte()) >= c.includedRange.endByte ||
				node.StartPoint().internal().gte(c.includedRange.endPoint)
			nodeOutsideOfRange := nodePrecedesRange || nodeFollowsRange

			// Skip captures that are outside of the cursor's range.
			if nodeOutsideOfRange {
				state.consumedCaptureCount = (state.consumedCaptureCount + 1) & consumedCaptureCountMask
				finishedStateSiftDown(c.finishedStates, 0, &c.captureListPool)
				continue
			}

			nodeStartByte := uint32(node.StartByte())
			if nodeStartByte < firstFinishedCaptureByte ||
				(nodeStartByte == firstFinishedCaptureByte &&
					uint32(state.patternIndex) < firstFinishedPatternIndex) {
				// C also sets first_finished_capture_byte and
				// first_finished_pattern_index here, and nothing reads them
				// after the break.
				firstFinishedState = state
			}
			break
		}

		// If there is finished capture that is clearly before any unfinished
		// capture, then return its match, and its capture index. Internally
		// record the fact that the capture has been 'consumed'.
		var state *queryState
		switch {
		case firstFinishedState != nil:
			state = firstFinishedState
		case firstUnfinishedStateIsDefinite:
			state = &c.states[firstUnfinishedStateIndex]
		default:
			state = nil
		}

		if state != nil {
			if state.id == math.MaxUint32 {
				state.id = c.nextStateID
				c.nextStateID++
			}
			captures := c.captureListPool.get(state.captureListID)
			match := queryMatch{
				id:           state.id,
				patternIndex: state.patternIndex,
				captures:     slices.Clip(captures),
			}
			captureIndex := uint32(state.consumedCaptureCount)
			state.consumedCaptureCount = (state.consumedCaptureCount + 1) & consumedCaptureCountMask
			// If this state is in the finished_states heap, its sort key has changed
			// (next capture is now later in the document). Restore heap order.
			if state == firstFinishedState {
				finishedStateSiftDown(c.finishedStates, 0, &c.captureListPool)
			}
			return match, captureIndex, true
		}

		if c.captureListPool.isEmpty() && foundUnfinishedState {
			c.captureListPool.release(
				c.states[firstUnfinishedStateIndex].captureListID,
			)
			c.states = slices.Delete(c.states, int(firstUnfinishedStateIndex), int(firstUnfinishedStateIndex)+1)
		}

		// If there are no finished matches that are ready to be returned, then
		// continue finding more matches.
		if !c.advance(ctx, true) &&
			len(c.finishedStates) == 0 {
			return queryMatch{}, 0, false
		}
	}
}

// setMaxStartDepth is ts_query_cursor_set_max_start_depth.
func (c *queryCursor) setMaxStartDepth(maxStartDepth uint32) {
	c.maxStartDepth = maxStartDepth
}

// iswspace is iswspace of the C library, in the C locale of glibc: the
// space and the characters from '\t' to '\r'. A code point of -1, which the
// decoder returns for bytes that are not valid, is WEOF, and it is not a
// space.
func iswspace(c int32) bool {
	return c == ' ' || (c >= '\t' && c <= '\r')
}

// iswalnum is iswalnum of the C library, in the C locale of glibc: the ASCII
// letters and digits.
func iswalnum(c int32) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// strncmpBytes is strncmp of the C library for byte slices, as strncmp is
// for strings. A slice ends at its length or at its first NUL byte.
func strncmpBytes(a, b []byte, n int) int {
	for i := range n {
		var ca, cb byte
		if i < len(a) {
			ca = a[i]
		}
		if i < len(b) {
			cb = b[i]
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		if ca == 0 {
			return 0
		}
	}
	return 0
}

// bits7 returns the low 7 bits of v, as C stores v in the bit field
// child_index of AnalysisSubgraphNode.
func bits7(v uint32) uint8 {
	return uint8(v & 0x7f)
}

// fieldIDMask is the mask of the bit field field_id of AnalysisStateEntry,
// which has 15 bits.
const fieldIDMask FieldID = 1<<15 - 1

// arraySearchSorted is _array__search_sorted of array.h, for the macros
// array_search_sorted_with and array_search_sorted_by. compare compares an
// element with the needle. The search returns the index of the last element
// that compares equal, or the index where the needle goes, as the macro
// does.
func arraySearchSorted[T any](self []T, compare func(*T) int) (index uint32, exists bool) {
	index = 0
	exists = false
	size := uint32(len(self)) - index
	if size == 0 {
		return index, exists
	}
	var comparison int
	for size > 1 {
		halfSize := size / 2
		midIndex := index + halfSize
		comparison = compare(&self[midIndex])
		if comparison <= 0 {
			index = midIndex
		}
		size -= halfSize
	}
	comparison = compare(&self[index])
	if comparison == 0 {
		exists = true
	} else if comparison < 0 {
		index++
	}
	return index, exists
}

// searchSubgraphs is array_search_sorted_by of array.h for subgraphs, by
// their symbol. The macro of upstream expands at each call, and this form
// keeps the comparison inline, as arraySearchSorted cannot.
func searchSubgraphs(self []analysisSubgraph, symbol Symbol) (index uint32, exists bool) {
	size := uint32(len(self))
	if size == 0 {
		return 0, false
	}
	for size > 1 {
		halfSize := size / 2
		midIndex := index + halfSize
		if int(self[midIndex].symbol)-int(symbol) <= 0 {
			index = midIndex
		}
		size -= halfSize
	}
	switch comparison := int(self[index].symbol) - int(symbol); {
	case comparison == 0:
		exists = true
	case comparison < 0:
		index++
	}
	return index, exists
}

// searchSubgraphNodes is array_search_sorted_with of array.h for the nodes
// of a subgraph, with analysis_subgraph_node__compare. The macro of upstream
// expands at each call, and this form keeps the comparison inline, as
// arraySearchSorted cannot.
func searchSubgraphNodes(self []analysisSubgraphNode, needle *analysisSubgraphNode) (index uint32, exists bool) {
	size := uint32(len(self))
	if size == 0 {
		return 0, false
	}
	for size > 1 {
		halfSize := size / 2
		midIndex := index + halfSize
		if analysisSubgraphNodeCompare(&self[midIndex], needle) <= 0 {
			index = midIndex
		}
		size -= halfSize
	}
	switch comparison := analysisSubgraphNodeCompare(&self[index], needle); {
	case comparison == 0:
		exists = true
	case comparison < 0:
		index++
	}
	return index, exists
}

// arraySearchSortedBy is array_search_sorted_by of array.h for an array of
// integers, which compares with _compare_int.
func arraySearchSortedBy[T ~uint16 | ~uint32](self []T, needle T) (uint32, bool) {
	return arraySearchSorted(self, func(a *T) int {
		return int(*a) - int(needle)
	})
}

// arrayInsertSortedBy is array_insert_sorted_by of array.h for an array of
// integers.
func arrayInsertSortedBy[T ~uint16 | ~uint32](self *[]T, value T) {
	index, exists := arraySearchSortedBy(*self, value)
	if !exists {
		*self = slices.Insert(*self, int(index), value)
	}
}
