package generate

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/transit/generate/internal/fxhash"
)

// This file ports crates/generate/src/build_tables/build_parse_table.rs: the
// builder of the LR(1) parse table, with the resolution of conflicts and the
// errors that it reports.

// auxiliaryContextID is an index into auxiliarySymbolContexts.contexts, plus
// one.
//
// auxiliaryContextID is AuxiliaryContextId, whose NonZeroU32 is 1-based.
// The zero auxiliaryContextID is the None of Option<AuxiliaryContextId>.
type auxiliaryContextID uint32

// auxiliaryContextIDFromIndex is the inverse of index.
//
// auxiliaryContextIDFromIndex is AuxiliaryContextId::from_index.
func auxiliaryContextIDFromIndex(index int) auxiliaryContextID {
	return auxiliaryContextID(uint32(index) + 1)
}

// index returns the dense 0-based index of the id. Ids are 1-based.
//
// index is AuxiliaryContextId::index.
func (id auxiliaryContextID) index() int {
	return int(id) - 1
}

// auxiliaryParentSetID is an index into auxiliarySymbolContexts.parentSets.
//
// auxiliaryParentSetID is AuxiliaryParentSetId.
type auxiliaryParentSetID uint32

// auxiliarySymbolContexts holds, for conflict reporting, the auxiliary
// (repeat) symbols in progress along the path to each parse state, and their
// parents: the non-auxiliary rules that were using them. Auxiliary symbols
// can't be named in the grammar's `conflicts`, so a conflict inside a repeat
// rule is reported in terms of its parents.
//
// A state's context only holds the auxiliary symbols at a dot in that state,
// and links to its predecessor: the context of the state that first led to
// it. Successor states share a context, and a lookup follows the
// predecessors back to the most recent state that used the symbol.
//
// auxiliarySymbolContexts is AuxiliarySymbolContexts. The zero value is
// AuxiliarySymbolContexts::default. Upstream finds the id of a parent set
// in a HashTable of ids. The Go form maps the key of each parent set to its
// id, and only looks it up.
type auxiliarySymbolContexts struct {
	// contexts is indexed by auxiliaryContextID.index.
	contexts []auxiliaryContext
	// entries holds every context's (auxiliary symbol, parent set) pairs.
	entries []auxiliaryContextEntry
	// parentSymbols holds the symbols of every distinct parent set,
	// concatenated in intern order.
	parentSymbols []Symbol
	// parentSets holds each parent set's slice of parentSymbols, indexed by
	// auxiliaryParentSetID.
	parentSets []auxiliaryParentSet
	// parentSetIDs holds the auxiliaryParentSetIDs of every distinct parent
	// set, by the buildParseSymbolsKey of its symbols.
	parentSetIDs map[string]auxiliaryParentSetID
}

// auxiliaryContextEntry is an auxiliary symbol of a context, with its parent
// set.
//
// auxiliaryContextEntry is (NonTerminalIndex, AuxiliaryParentSetId).
type auxiliaryContextEntry struct {
	symbol    NonTerminalIndex
	parentSet auxiliaryParentSetID
}

// auxiliaryContext is one state's slice of auxiliarySymbolContexts.entries,
// and its predecessor.
//
// auxiliaryContext is AuxiliaryContext.
type auxiliaryContext struct {
	predecessor auxiliaryContextID
	start       uint32
	len         uint32
}

// auxiliaryParentSet is one parent set's slice of
// auxiliarySymbolContexts.parentSymbols.
//
// auxiliaryParentSet is AuxiliaryParentSet.
type auxiliaryParentSet struct {
	start uint32
	len   uint32
}

// auxiliaryUse pairs an auxiliary symbol at a dot in a state with the rule of
// an item that uses it.
//
// auxiliaryUse is (NonTerminalIndex, NonTerminalIndex).
type auxiliaryUse struct {
	symbol NonTerminalIndex
	parent NonTerminalIndex
}

// compareAuxiliaryUse orders two uses by the symbol and then by the parent.
//
// compareAuxiliaryUse is the Ord of (NonTerminalIndex, NonTerminalIndex).
func compareAuxiliaryUse(a, b auxiliaryUse) int {
	if c := cmp.Compare(a.symbol, b.symbol); c != 0 {
		return c
	}
	return cmp.Compare(a.parent, b.parent)
}

// push adds the context of a state whose predecessor is predecessor. uses
// pairs each auxiliary symbol at a dot in the state with the rule of an item
// using it. Auxiliary rules aren't parents, but a symbol used only by
// auxiliary rules still gets an (empty) entry, which hides any earlier one.
//
// push is AuxiliarySymbolContexts::push. Upstream sorts with sort_unstable.
// Two uses that compare equal are the same value, so the order is the same
// in Go.
func (c *auxiliarySymbolContexts) push(grammar *SyntaxGrammar, predecessor auxiliaryContextID, uses []auxiliaryUse) auxiliaryContextID {
	if len(uses) == 0 {
		return predecessor
	}
	slices.SortFunc(uses, compareAuxiliaryUse)
	uses = slices.Compact(uses)
	start := uint32(len(c.entries))
	var parents []Symbol
	for len(uses) > 0 {
		n := 1
		for n < len(uses) && uses[n].symbol == uses[0].symbol {
			n++
		}
		group := uses[:n]
		uses = uses[n:]
		parents = parents[:0]
		for _, use := range group {
			if !grammar.Variables[use.parent].IsAuxiliary() {
				parents = append(parents, use.parent.Symbol())
			}
		}
		parentSet := c.intern(parents)
		symbol := group[0].symbol
		c.entries = append(c.entries, auxiliaryContextEntry{symbol: symbol, parentSet: parentSet})
	}
	id := auxiliaryContextIDFromIndex(len(c.contexts))
	c.contexts = append(c.contexts, auxiliaryContext{
		predecessor: predecessor,
		start:       start,
		len:         uint32(len(c.entries)) - start,
	})
	return id
}

// parents returns the parents of symbol in the most recent state that had
// it at a dot, along the path to the state whose context is context. It
// returns false when no state along the path had it.
//
// parents is AuxiliarySymbolContexts::parents.
func (c *auxiliarySymbolContexts) parents(context auxiliaryContextID, symbol NonTerminalIndex) ([]Symbol, bool) {
	for context != 0 {
		ctx := c.contexts[context.index()]
		entries := c.entries[ctx.start : ctx.start+ctx.len]
		for _, entry := range entries {
			if entry.symbol == symbol {
				return c.parentSet(entry.parentSet), true
			}
		}
		context = ctx.predecessor
	}
	return nil, false
}

// intern returns the id of the parent set parents, interning it if it's new.
//
// intern is AuxiliarySymbolContexts::intern.
func (c *auxiliarySymbolContexts) intern(parents []Symbol) auxiliaryParentSetID {
	key := buildParseSymbolsKey(parents)
	if id, ok := c.parentSetIDs[key]; ok {
		return id
	}
	if c.parentSetIDs == nil {
		c.parentSetIDs = make(map[string]auxiliaryParentSetID)
	}
	id := auxiliaryParentSetID(len(c.parentSets))
	c.parentSets = append(c.parentSets, auxiliaryParentSet{
		start: uint32(len(c.parentSymbols)),
		len:   uint32(len(parents)),
	})
	c.parentSymbols = append(c.parentSymbols, parents...)
	c.parentSetIDs[key] = id
	return id
}

// parentSet returns the symbols of a parent set.
//
// parentSet is AuxiliarySymbolContexts::parent_set.
func (c *auxiliarySymbolContexts) parentSet(id auxiliaryParentSetID) []Symbol {
	set := c.parentSets[id]
	return c.parentSymbols[set.start : set.start+set.len : set.start+set.len]
}

// ParseStateInfo is what the builder of the parse table knows about each
// state, for the report of the states of a symbol.
//
// ParseStateInfo is ParseStateInfo. Upstream keeps the item sets in an
// IndexMap from the item set to its state id, and reads it only by index.
// Each state id is the index of its item set, so itemSetsByIDs is the keys
// of that map in order.
type ParseStateInfo struct {
	// PrecedingSymbolsByID holds, for each state, an example sequence of
	// symbols that leads to the state. The report of a conflict uses it.
	PrecedingSymbolsByID [][]Symbol
	// Lookaheads is the pool of the lookahead sets of the item sets.
	Lookaheads *LookaheadSetPool

	itemSetsByIDs []ParseItemSet
}

// ItemSet returns the item set of a state, before the closure.
//
// ItemSet is ParseStateInfo::item_set.
func (i *ParseStateInfo) ItemSet(id ParseStateID) *ParseItemSet {
	return &i.itemSetsByIDs[id]
}

// reductionInfo is what the builder knows about the reduce actions of a
// state for one lookahead: their precedence, their symbols in order, and
// their associativities.
//
// reductionInfo is ReductionInfo. The zero reductionInfo is
// ReductionInfo::default.
type reductionInfo struct {
	precedence    Precedence
	symbols       []Symbol
	hasLeftAssoc  bool
	hasRightAssoc bool
	hasNonAssoc   bool
}

// clear resets r to the zero reductionInfo, and keeps the buffer of
// symbols.
//
// clear is ReductionInfo::clear.
func (r *reductionInfo) clear() {
	*r = reductionInfo{symbols: r.symbols[:0]}
}

// reductionInfos holds the reductions on each lookahead of the state that
// addActions works on.
//
// reductionInfos is ReductionInfos.
type reductionInfos struct {
	// indexer gives each lookahead its slot in infos.
	indexer symbolIndexer
	// infos holds each lookahead's reductions, by symbolIndexer.index. Only
	// the lookaheads with a reduction in the current state are up to date:
	// addActions clears a lookahead's info at its first reduction in each
	// state.
	infos []reductionInfo
}

// newReductionInfos returns the reduction infos of a grammar.
//
// newReductionInfos is ReductionInfos::new.
func newReductionInfos(indexer symbolIndexer) reductionInfos {
	return reductionInfos{
		indexer: indexer,
		infos:   make([]reductionInfo, indexer.tokenCount()),
	}
}

// get returns the reductions on lookahead. The caller can update them.
//
// get is ReductionInfos::get and ReductionInfos::get_mut.
func (r *reductionInfos) get(lookahead Symbol) *reductionInfo {
	return &r.infos[r.indexer.index(lookahead)]
}

// successorSets holds the item sets of a state's successors, one for each
// symbol that follows a dot in the state.
//
// successorSets is SuccessorSets. A nil set in sets is the None of
// Option<ParseItemSet>.
type successorSets struct {
	// indexer gives each symbol its slot in sets.
	indexer symbolIndexer
	// sets holds each symbol's successor item set, by symbolIndexer.index.
	sets []*ParseItemSet
	// symbols holds the symbols that have a set in sets, in the order that
	// their sets were added.
	symbols []Symbol
	// pool holds empty item sets to build successors in, so that most
	// successors do not allocate.
	pool []*ParseItemSet
}

// successorSetsMaxPooledCapacity is the capacity of the largest item set
// that pool keeps.
//
// successorSetsMaxPooledCapacity is SuccessorSets::MAX_POOLED_CAPACITY.
const successorSetsMaxPooledCapacity = 256

// successorSet is a symbol and the item set of its successor.
//
// successorSet is (Symbol, ParseItemSet).
type successorSet struct {
	symbol  Symbol
	itemSet *ParseItemSet
}

// newSuccessorSets returns successor sets with no set.
//
// newSuccessorSets is SuccessorSets::new.
func newSuccessorSets(indexer symbolIndexer) successorSets {
	return successorSets{
		indexer: indexer,
		sets:    make([]*ParseItemSet, indexer.symbolCount()),
		symbols: nil,
		pool:    nil,
	}
}

// itemSet returns the item set of the successor after symbol, and adds it if
// it is new.
//
// itemSet is SuccessorSets::item_set.
func (s *successorSets) itemSet(symbol Symbol) *ParseItemSet {
	slot := &s.sets[s.indexer.index(symbol)]
	if *slot == nil {
		s.symbols = append(s.symbols, symbol)
		if n := len(s.pool); n > 0 {
			*slot = s.pool[n-1]
			s.pool = s.pool[:n-1]
		} else {
			*slot = &ParseItemSet{}
		}
	}
	return *slot
}

// takeAll takes the sets out in symbol order, and leaves s empty for the
// next state.
//
// takeAll is SuccessorSets::take_all. The symbols are distinct, so the sort
// gives the order of sort_unstable.
func (s *successorSets) takeAll() []successorSet {
	slices.SortFunc(s.symbols, CompareSymbol)
	result := make([]successorSet, 0, len(s.symbols))
	for _, symbol := range s.symbols {
		// INVARIANT: every symbol in symbols has a set
		slot := &s.sets[s.indexer.index(symbol)]
		result = append(result, successorSet{symbol: symbol, itemSet: *slot})
		*slot = nil
	}
	s.symbols = s.symbols[:0]
	return result
}

// recycle takes back the sets from takeAll, to build later successors in.
//
// recycle is SuccessorSets::recycle.
func (s *successorSets) recycle(sets []successorSet) {
	for _, set := range sets {
		if cap(set.itemSet.Entries) <= successorSetsMaxPooledCapacity {
			set.itemSet.Entries = set.itemSet.Entries[:0]
			s.pool = append(s.pool, set.itemSet)
		}
	}
}

// parseStateQueueEntry is a state that waits for its actions.
//
// parseStateQueueEntry is ParseStateQueueEntry.
type parseStateQueueEntry struct {
	stateID                   ParseStateID
	precedingAuxiliaryContext auxiliaryContextID
}

// buildParseNoProductionInfoID marks a production that has no production
// info yet. No id reaches it, because an id must fit in a uint16.
const buildParseNoProductionInfoID ProductionInfoID = math.MaxUint32

// parseTableBuilder builds the parse table.
//
// parseTableBuilder is ParseTableBuilder. Upstream keeps the state ids in an
// IndexMap from the item set to the id. Each id is the index of its item
// set, so the Go form is itemSets, the keys in order, and stateIDsByItemSet,
// which maps the Key of each item set to its id. Upstream only looks up the
// FxHashMap coreIDsByCore and the FxHashSet actualConflicts, with one
// exception that build explains.
type parseTableBuilder struct {
	itemSetBuilder            *ParseItemSetBuilder
	syntaxGrammar             *SyntaxGrammar
	lexicalGrammar            *LexicalGrammar
	variableInfo              []VariableInfo
	coreIDsByCore             map[string]uint32
	stateIDsByItemSet         map[string]ParseStateID
	itemSets                  []ParseItemSet
	precedingSymbolsByID      [][]Symbol
	productionInfoIDsByProdID []ProductionInfoID
	parseStateQueue           []parseStateQueueEntry
	auxiliaryContexts         auxiliarySymbolContexts
	nonTerminalExtraStates    []buildParseExtraState
	actualConflicts           map[string][]Symbol
	parseTable                ParseTable[ParseTableEntry]
	strPool                   *StrPool
	// successorSets is scratch for addActions: the successor item sets of
	// the state that it works on.
	successorSets successorSets
	// reductionInfos is scratch for addActions: the reductions on each
	// lookahead of the state that it works on.
	reductionInfos reductionInfos
	// parseStateQueueHead is the index of the front of parseStateQueue,
	// which is a VecDeque upstream.
	parseStateQueueHead int
}

// buildParseExtraState is a terminal that starts a non-terminal extra, and
// the state that the parser shifts to on it.
//
// buildParseExtraState is (Symbol, ParseStateId).
type buildParseExtraState struct {
	terminal Symbol
	stateID  ParseStateID
}

// ParseTableBuilderErrorKind is the kind of an error of the builder of the
// parse table.
type ParseTableBuilderErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ParseTableBuilderConflict ParseTableBuilderErrorKind = iota
	ParseTableBuilderAmbiguousExtra
	ParseTableBuilderImproperNonTerminalExtra
	ParseTableBuilderStateCount
)

// ParseTableBuilderError is an error of the builder of the parse table. Its
// text is the text of upstream.
//
// ParseTableBuilderError is ParseTableBuilderError, an enum with data
// upstream. The fields that a kind uses are:
//
//   - ParseTableBuilderConflict: Conflict.
//   - ParseTableBuilderAmbiguousExtra: AmbiguousExtra.
//   - ParseTableBuilderImproperNonTerminalExtra: Name.
//   - ParseTableBuilderStateCount: StateCount.
//
// Upstream also derives Serialize and Deserialize for the error types of
// this file. The port does not.
type ParseTableBuilderError struct {
	Kind           ParseTableBuilderErrorKind
	Conflict       *ConflictError
	AmbiguousExtra *AmbiguousExtraError
	Name           string
	StateCount     int
}

// Error returns the text of the error.
func (e *ParseTableBuilderError) Error() string {
	switch e.Kind {
	case ParseTableBuilderConflict:
		return "Unresolved conflict for symbol sequence:\n\n" + e.Conflict.Error()
	case ParseTableBuilderAmbiguousExtra:
		return "Extra rules must have unambiguous endings. Conflicting rules: " + e.AmbiguousExtra.Error()
	case ParseTableBuilderImproperNonTerminalExtra:
		return "The non-terminal rule `" + e.Name + "` is used in a non-terminal `extra` rule, which is not allowed."
	case ParseTableBuilderStateCount:
		return "State count `" + strconv.Itoa(e.StateCount) + "` exceeds the max value " + strconv.Itoa(math.MaxUint16) + "."
	}
	return ""
}

// Unwrap returns the error in the error: the *ConflictError or the
// *AmbiguousExtraError, and nil for the other kinds.
func (e *ParseTableBuilderError) Unwrap() error {
	switch e.Kind {
	case ParseTableBuilderConflict:
		return e.Conflict
	case ParseTableBuilderAmbiguousExtra:
		return e.AmbiguousExtra
	}
	return nil
}

// ConflictError is a conflict that the grammar does not resolve: the
// sequence of symbols before the conflict, the lookahead, the ways in which
// the parser can read the input, and the changes to the grammar that
// resolve the conflict.
//
// ConflictError is ConflictError.
type ConflictError struct {
	SymbolSequence          []string
	ConflictingLookahead    string
	PossibleInterpretations []Interpretation
	PossibleResolutions     []Resolution
}

// Interpretation is one way in which the parser can read the input at a
// conflict. HasPrecedence and HasAssociativity are false for the None of
// upstream.
//
// Interpretation is Interpretation. Upstream also derives Error for it. The
// port gives it only its text.
type Interpretation struct {
	PrecedingSymbols      []string
	VariableName          string
	ProductionStepSymbols []string
	StepIndex             uint32
	Done                  bool
	ConflictingLookahead  string
	Precedence            string
	HasPrecedence         bool
	Associativity         string
	HasAssociativity      bool
	RequiresEOFLookahead  bool
}

// ResolutionKind is the kind of a resolution of a conflict.
type ResolutionKind uint8

// The kinds of resolution, in the order of upstream.
const (
	ResolutionPrecedence ResolutionKind = iota
	ResolutionAssociativity
	ResolutionAddConflict
)

// Resolution is a change to the grammar that resolves a conflict: the kind
// of change, and the rules that it changes.
//
// Resolution is Resolution, an enum with data upstream. Each variant has the
// field symbols.
type Resolution struct {
	Kind    ResolutionKind
	Symbols []string
}

// AmbiguousExtraError is a non-terminal extra whose end is ambiguous. It
// holds the names of the rules that conflict.
//
// AmbiguousExtraError is AmbiguousExtraError.
type AmbiguousExtraError struct {
	ParentSymbols []string
}

// buildParseInterpretationLine is the text of an interpretation, and its
// annotation of precedence, when it has one.
//
// buildParseInterpretationLine is (String, Option<String>).
type buildParseInterpretationLine struct {
	line          string
	precLine      string
	hasPrecLine   bool
	lineCharCount int
}

// Error returns the text of the conflict.
//
// Error is the Display of ConflictError.
func (e *ConflictError) Error() string {
	var b strings.Builder
	for _, symbol := range e.SymbolSequence {
		b.WriteString("  " + symbol)
	}
	b.WriteString("  •  " + e.ConflictingLookahead + "  …\n\n")

	b.WriteString("Possible interpretations:\n\n")
	interpretations := make([]buildParseInterpretationLine, 0, len(e.PossibleInterpretations))
	for i := range e.PossibleInterpretations {
		in := &e.PossibleInterpretations[i]
		line := in.String()
		var annotations []string
		if in.HasPrecedence && in.HasAssociativity {
			annotations = append(annotations, "(precedence: "+in.Precedence+", associativity: "+in.Associativity+")")
		} else if in.HasPrecedence {
			annotations = append(annotations, "(precedence: "+in.Precedence+")")
		}
		if in.RequiresEOFLookahead {
			annotations = append(annotations, "(reduces only at end of input)")
		}
		interpretations = append(interpretations, buildParseInterpretationLine{
			line:          line,
			precLine:      strings.Join(annotations, "  "),
			hasPrecLine:   len(annotations) > 0,
			lineCharCount: utf8.RuneCountInString(line),
		})
	}
	if len(interpretations) == 0 {
		// Upstream unwraps the maximum of an empty list here.
		panic("generate: a conflict has no interpretations")
	}
	maxInterpretationLength := 0
	for _, in := range interpretations {
		maxInterpretationLength = max(maxInterpretationLength, in.lineCharCount)
	}
	// Upstream sorts with sort_unstable. Two lines that compare equal have the
	// same text and the same annotation, so the order is the same in Go.
	slices.SortFunc(interpretations, func(a, b buildParseInterpretationLine) int {
		if c := strings.Compare(a.line, b.line); c != 0 {
			return c
		}
		if c := compareBool(a.hasPrecLine, b.hasPrecLine); c != 0 {
			return c
		}
		return strings.Compare(a.precLine, b.precLine)
	})
	for i, in := range interpretations {
		fmt.Fprintf(&b, "  %d:", i+1)
		b.WriteString(in.line)
		if in.hasPrecLine {
			b.WriteString(strings.Repeat(" ", max(maxInterpretationLength-in.lineCharCount, 0)+2))
			b.WriteString(in.precLine)
		}
		b.WriteString("\n")
	}

	b.WriteString("\nPossible resolutions:\n\n")
	for i, resolution := range e.PossibleResolutions {
		fmt.Fprintf(&b, "  %d:  %s\n", i+1, resolution.String())
	}
	return b.String()
}

// String returns the text of the interpretation.
//
// String is the Display of Interpretation.
func (i *Interpretation) String() string {
	var b strings.Builder
	for _, symbol := range i.PrecedingSymbols {
		b.WriteString("  " + symbol)
	}
	b.WriteString("  (" + i.VariableName)
	for j, symbol := range i.ProductionStepSymbols {
		if j == int(i.StepIndex) {
			b.WriteString("  •")
		}
		b.WriteString("  " + symbol)
	}
	b.WriteString(")")
	if i.Done {
		b.WriteString("  •  " + i.ConflictingLookahead + "  …")
	}
	return b.String()
}

// String returns the text of the resolution.
//
// String is the Display of Resolution.
func (r Resolution) String() string {
	var b strings.Builder
	switch r.Kind {
	case ResolutionPrecedence:
		b.WriteString("Specify a higher precedence in ")
		for i, symbol := range r.Symbols {
			if i > 0 {
				b.WriteString(" and ")
			}
			b.WriteString("`" + symbol + "`")
		}
		b.WriteString(" than in the other rules.")
	case ResolutionAssociativity:
		b.WriteString("Specify a left or right associativity in ")
		for i, symbol := range r.Symbols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("`" + symbol + "`")
		}
	case ResolutionAddConflict:
		b.WriteString("Add a conflict for these rules: ")
		for i, symbol := range r.Symbols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("`" + symbol + "`")
		}
	}
	return b.String()
}

// Error returns the names of the rules, separated by commas.
//
// Error is the Display of AmbiguousExtraError.
func (e *AmbiguousExtraError) Error() string {
	return strings.Join(e.ParentSymbols, ", ")
}

// newParseTableBuilder returns a builder for a grammar.
//
// newParseTableBuilder is ParseTableBuilder::new.
func newParseTableBuilder(
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	itemSetBuilder *ParseItemSetBuilder,
	variableInfo []VariableInfo,
	strPool *StrPool,
) *parseTableBuilder {
	productionInfoIDs := make([]ProductionInfoID, len(syntaxGrammar.Productions))
	for i := range productionInfoIDs {
		productionInfoIDs[i] = buildParseNoProductionInfoID
	}
	actualConflicts := make(map[string][]Symbol, len(syntaxGrammar.ExpectedConflicts))
	for _, conflict := range syntaxGrammar.ExpectedConflicts {
		actualConflicts[buildParseSymbolsKey(conflict)] = slices.Clone(conflict)
	}
	symbolIndexer := newSymbolIndexer(syntaxGrammar, lexicalGrammar)
	return &parseTableBuilder{
		syntaxGrammar:             syntaxGrammar,
		lexicalGrammar:            lexicalGrammar,
		itemSetBuilder:            itemSetBuilder,
		variableInfo:              variableInfo,
		stateIDsByItemSet:         make(map[string]ParseStateID),
		coreIDsByCore:             make(map[string]uint32),
		productionInfoIDsByProdID: productionInfoIDs,
		actualConflicts:           actualConflicts,
		parseTable: ParseTable[ParseTableEntry]{
			MaxAliasedProductionLength: 1,
		},
		strPool:        strPool,
		successorSets:  newSuccessorSets(symbolIndexer),
		reductionInfos: newReductionInfos(symbolIndexer),
	}
}

// buildParseSymbolsKey returns a string that is the same for two sequences
// of symbols exactly when they are equal, for a map key. It stands in for
// the Hash of Vec<Symbol>.
func buildParseSymbolsKey(symbols []Symbol) string {
	buf := make([]byte, 0, 8*len(symbols))
	for _, s := range symbols {
		buf = binary.LittleEndian.AppendUint64(buf, s.packedKey())
	}
	return string(buf)
}

// build builds the parse table, and returns it with the information about
// its states. It adds a diagnostic for the expected conflicts that the
// grammar does not need.
//
// build is ParseTableBuilder::build.
func (b *parseTableBuilder) build(diagnostics *[]Diagnostic) (ParseTable[ParseTableEntry], *ParseStateInfo, error) {
	// Ensure that the empty alias sequence has index 0.
	b.parseTable.ProductionInfos = append(b.parseTable.ProductionInfos, ProductionInfo{})

	// Add the error state at index 0.
	b.addParseState(nil, 0, &ParseItemSet{})

	// Add the starting state at index 1.
	endLookaheads := b.itemSetBuilder.Lookaheads.Singleton(SymbolEndValue)
	b.addParseState(nil, 0, &ParseItemSet{
		Entries: []ParseItemSetEntry{{
			Item:                     StartParseItem(b.itemSetBuilder.KeyMap),
			Lookaheads:               endLookaheads,
			FollowingReservedWordSet: 0,
		}},
	})

	// Compute the possible item sets for non-terminal extras. Upstream keeps
	// them in a BTreeMap, which iterates in the order of the terminals.
	nonTerminalExtraItemSetsByFirstTerminal := make(map[Symbol]*ParseItemSet)
	for _, s := range b.syntaxGrammar.ExtraSymbols {
		extraIndex, ok := s.NonTerminalIndex()
		if !ok {
			continue
		}
		start, end := b.syntaxGrammar.VariableProdIDs(int(extraIndex))
		for prodID := start; prodID < end; prodID++ {
			production := b.syntaxGrammar.Production(prodID)
			firstSymbol, ok := production.FirstSymbol()
			if !ok {
				panic("generate: a production of a non-terminal extra is empty")
			}
			set, ok := nonTerminalExtraItemSetsByFirstTerminal[firstSymbol]
			if !ok {
				set = &ParseItemSet{}
				nonTerminalExtraItemSetsByFirstTerminal[firstSymbol] = set
			}
			entry := set.Insert(ParseItem{
				VariableIndex:               uint32(extraIndex),
				ProdID:                      prodID,
				StepIndex:                   1,
				Keys:                        b.itemSetBuilder.KeyMap.KeysFor(prodID),
				HasPrecedingInheritedFields: false,
			})
			entry.Lookaheads = b.itemSetBuilder.Lookaheads.Insert(entry.Lookaheads, SymbolEndOfNonTerminalExtraValue)
		}
	}

	b.nonTerminalExtraStates = slices.Grow(b.nonTerminalExtraStates, len(nonTerminalExtraItemSetsByFirstTerminal))
	// Add a state for each starting terminal of a non-terminal extra rule.
	for _, terminal := range slices.SortedFunc(maps.Keys(nonTerminalExtraItemSetsByFirstTerminal), CompareSymbol) {
		itemSet := nonTerminalExtraItemSetsByFirstTerminal[terminal]
		if _, ok := terminal.NonTerminalIndex(); ok {
			return ParseTable[ParseTableEntry]{}, nil, &ParseTableBuilderError{
				Kind: ParseTableBuilderImproperNonTerminalExtra,
				Name: b.symbolName(terminal),
			}
		}

		// Add the parse state, and *then* push the terminal and the state id into the
		// list of nonterminal extra states
		stateID := b.addParseState(nil, 0, itemSet)
		b.nonTerminalExtraStates = append(b.nonTerminalExtraStates, buildParseExtraState{terminal: terminal, stateID: stateID})
	}

	for b.parseStateQueueHead < len(b.parseStateQueue) {
		entry := b.parseStateQueue[b.parseStateQueueHead]
		b.parseStateQueue[b.parseStateQueueHead] = parseStateQueueEntry{}
		b.parseStateQueueHead++
		// The dedup-map key is each state's kernel (the GOTO result, pre closure).
		// Two states are identical iff their kernels match.
		kernel := &b.itemSets[entry.stateID]
		itemSet := b.itemSetBuilder.TransitiveClosure(kernel)

		if err := b.addActions(
			slices.Clone(b.precedingSymbolsByID[entry.stateID]),
			entry.precedingAuxiliaryContext,
			entry.stateID,
			&itemSet,
		); err != nil {
			return ParseTable[ParseTableEntry]{}, nil, err
		}
	}

	if len(b.actualConflicts) > 0 {
		// Upstream collects the names from an FxHashSet, in the order of the
		// set, and then sorts them with sort_unstable. Two lists of names that
		// compare equal are the same text, so the order of the set and of the
		// sort does not reach the output.
		conflicts := make([][]string, 0, len(b.actualConflicts))
		for _, conflict := range b.actualConflicts {
			names := make([]string, len(conflict))
			for i, s := range conflict {
				names[i] = b.symbolName(s)
			}
			conflicts = append(conflicts, names)
		}
		slices.SortFunc(conflicts, slices.Compare)
		*diagnostics = append(*diagnostics, Diagnostic{Kind: DiagnosticUnnecessaryConflicts, Conflicts: conflicts})
	}

	return b.parseTable, &ParseStateInfo{
		PrecedingSymbolsByID: b.precedingSymbolsByID,
		itemSetsByIDs:        b.itemSets,
		Lookaheads:           b.itemSetBuilder.Lookaheads,
	}, nil
}

// addParseState returns the state of an item set. When the builder does not
// have the item set yet, addParseState adds a new state for it and puts the
// state in the queue.
//
// addParseState is ParseTableBuilder::add_parse_state.
func (b *parseTableBuilder) addParseState(
	precedingSymbols []Symbol,
	precedingAuxiliaryContext auxiliaryContextID,
	itemSet *ParseItemSet,
) ParseStateID {
	// Compute the key of the item set once, for both the lookup and a
	// possible insert.
	key := itemSet.Key()
	// If an equivalent item set has already been processed, then return
	// the existing parse state index.
	if id, ok := b.stateIDsByItemSet[key]; ok {
		return id
	}

	// Otherwise, insert a new parse state and add it to the queue of
	// parse states to populate.
	core := itemSet.Core()
	coreKey := core.Key()
	coreID, ok := b.coreIDsByCore[coreKey]
	if !ok {
		coreID = uint32(len(b.coreIDsByCore))
		b.coreIDsByCore[coreKey] = coreID
	}

	stateID := ParseStateID(len(b.parseTable.States))
	b.precedingSymbolsByID = append(b.precedingSymbolsByID, slices.Clone(precedingSymbols))

	b.parseTable.States = append(b.parseTable.States, ParseState[ParseTableEntry]{
		ID:                 stateID,
		LexStateID:         0,
		ExternalLexStateID: 0,
		CoreID:             coreID,
		HasEOFGatedReduce:  false,
	})
	b.parseStateQueue = append(b.parseStateQueue, parseStateQueueEntry{
		stateID:                   stateID,
		precedingAuxiliaryContext: precedingAuxiliaryContext,
	})
	b.stateIDsByItemSet[key] = stateID
	b.itemSets = append(b.itemSets, ParseItemSet{Entries: slices.Clone(itemSet.Entries)})
	return stateID
}

// addActions adds the actions of a state: a shift for each symbol after a
// dot, a reduce for each item that is done, the actions of the extras, and
// the reserved words. It resolves the conflicts that it finds, and returns
// an error for a conflict that the grammar does not resolve.
//
// addActions is ParseTableBuilder::add_actions.
func (b *parseTableBuilder) addActions(
	precedingSymbols []Symbol,
	precedingAuxiliaryContext auxiliaryContextID,
	stateID ParseStateID,
	itemSet *ParseItemSet,
) error {
	var lookaheadsWithConflicts TokenSet
	var auxiliaryUses []auxiliaryUse

	// Each item in the item set contributes to either or a Shift action or a Reduce
	// action in this state.
	for i := range itemSet.Entries {
		entry := &itemSet.Entries[i]
		item := &entry.Item
		// If the item is unfinished, then this state has a transition for the item's
		// next symbol. Advance the item to its next step and insert the resulting
		// item into the successor item set.
		if nextSymbol, ok := item.Symbol(b.syntaxGrammar); ok {
			successor := item.Successor()
			if nonTerminalIndex, ok := nextSymbol.NonTerminalIndex(); ok {
				index := int(nonTerminalIndex)
				variable := b.syntaxGrammar.Variables[index]

				// Keep track of where auxiliary non-terminals (repeat symbols) are
				// used within visible symbols. This information may be needed later
				// for conflict resolution.
				if variable.IsAuxiliary() {
					parent := NonTerminalIndex(item.VariableIndex)
					auxiliaryUses = append(auxiliaryUses, auxiliaryUse{symbol: nonTerminalIndex, parent: parent})
				}

				// For most parse items, the symbols associated with the preceding children
				// don't matter: they have no effect on the REDUCE action that would be
				// performed at the end of the item. But the symbols *do* matter for
				// children that are hidden and have fields, because those fields are
				// "inherited" by the parent node.
				//
				// If this item has consumed a hidden child with fields, then the symbols
				// of its preceding children need to be taken into account when comparing
				// it with other items.
				if variable.IsHidden() && len(b.variableInfo[index].Fields) > 0 {
					successor.HasPrecedingInheritedFields = true
				}
			}
			successorSet := b.successorSets.itemSet(nextSymbol)
			successorEntry := successorSet.Insert(successor)
			successorEntry.Lookaheads = b.itemSetBuilder.Lookaheads.Union(successorEntry.Lookaheads, entry.Lookaheads)
			successorEntry.FollowingReservedWordSet = max(successorEntry.FollowingReservedWordSet, entry.FollowingReservedWordSet)
			continue
		}

		// If the item is finished, then add a Reduce action to this state based
		// on this item.
		symbol := NonTerminalSymbol(int(item.VariableIndex))
		var action ParseAction
		if item.IsAugmented() {
			action = ParseAction{Kind: ParseActionAccept}
		} else {
			// These values are narrowed to u16 to reduce the size of
			// ParseAction. No real-world grammar approaches these limits.
			// Upstream checks them only with debug_assert.
			productionID := b.getProductionID(item)
			action = ParseAction{
				Kind:              ParseActionReduce,
				Symbol:            symbol,
				ChildCount:        uint16(item.StepIndex),
				DynamicPrecedence: item.Production(b.syntaxGrammar).DynamicPrecedence,
				ProductionID:      uint16(productionID),
			}
		}

		precedence := item.Precedence(b.syntaxGrammar)
		associativity := item.Associativity(b.syntaxGrammar)
		requiresEOFLookahead := item.Production(b.syntaxGrammar).RequiresEOFLookahead
		if requiresEOFLookahead {
			b.parseTable.States[stateID].HasEOFGatedReduce = true
		}
		for lookahead := range b.itemSetBuilder.Lookaheads.Get(entry.Lookaheads).All() {
			if requiresEOFLookahead && lookahead != SymbolEndValue {
				continue
			}
			tableEntry := b.parseTable.States[stateID].TerminalEntries.GetOrInsertFunc(lookahead, NewParseTableEntry)
			info := b.reductionInfos.get(lookahead)

			// While inserting Reduce actions, eagerly resolve conflicts related
			// to precedence: avoid inserting lower-precedence reductions, and
			// clear the action list when inserting higher-precedence reductions.
			if len(tableEntry.Actions) == 0 {
				// This is the lookahead's first reduction in this state, so its info is
				// still from an earlier state.
				info.clear()
				tableEntry.Actions.Push(action)
			} else {
				switch buildParseComparePrecedence(b.syntaxGrammar, precedence, []Symbol{symbol}, info.precedence, info.symbols) {
				case 1:
					tableEntry.Actions.Clear()
					tableEntry.Actions.Push(action)
					lookaheadsWithConflicts.Remove(lookahead)
					info.clear()
				// Two items that reduce identically build the same tree, so
				// there is nothing for the user to resolve. Precedence is
				// still compared first, because an item that only repeats an
				// existing action can still outrank it and clear the entry.
				case 0:
					if !slices.Contains(tableEntry.Actions, action) {
						tableEntry.Actions.Push(action)
						lookaheadsWithConflicts.Insert(lookahead)
					}
				case -1:
					continue
				}
			}

			info.precedence = precedence
			if i, found := slices.BinarySearchFunc(info.symbols, symbol, CompareSymbol); !found {
				info.symbols = slices.Insert(info.symbols, i, symbol)
			}
			switch associativity {
			case AssociativityLeft:
				info.hasLeftAssoc = true
			case AssociativityRight:
				info.hasRightAssoc = true
			case AssociativityNone:
				info.hasNonAssoc = true
			}
		}
	}

	auxiliaryContext := b.auxiliaryContexts.push(
		b.syntaxGrammar,
		precedingAuxiliaryContext,
		auxiliaryUses,
	)

	// Having computed the successor item sets for each symbol, add a new
	// parse state for each of these item sets, and add a corresponding Shift
	// action to this state.
	successors := b.successorSets.takeAll()
	// Non-terminals come last in symbol order.
	split := sort.Search(len(successors), func(i int) bool {
		_, ok := successors[i].symbol.NonTerminalIndex()
		return ok
	})
	terminalSuccessors, nonTerminalSuccessors := successors[:split], successors[split:]
	for _, successor := range terminalSuccessors {
		symbol, nextItemSet := successor.symbol, successor.itemSet
		precedingSymbols = append(precedingSymbols, symbol)
		nextStateID := b.addParseState(precedingSymbols, auxiliaryContext, nextItemSet)
		precedingSymbols = precedingSymbols[:len(precedingSymbols)-1]

		entries := &b.parseTable.States[stateID].TerminalEntries
		if e, ok := entries.Get(symbol); ok && len(e.Actions) > 0 {
			lookaheadsWithConflicts.Insert(symbol)
		}

		entries.GetOrInsertFunc(symbol, NewParseTableEntry).Actions.Push(ParseAction{
			Kind:         ParseActionShift,
			State:        nextStateID,
			IsRepetition: false,
		})
	}

	for _, successor := range nonTerminalSuccessors {
		symbol, nextItemSet := successor.symbol, successor.itemSet
		precedingSymbols = append(precedingSymbols, symbol)
		nextStateID := b.addParseState(precedingSymbols, auxiliaryContext, nextItemSet)
		precedingSymbols = precedingSymbols[:len(precedingSymbols)-1]
		b.parseTable.States[stateID].NonterminalEntries.Insert(symbol, GotoAction{Kind: GotoActionGoto, State: nextStateID})
	}
	b.successorSets.recycle(successors)

	// For any symbol with multiple actions, perform conflict resolution.
	// This will either
	// * choose one action over the others using precedence or associativity
	// * keep multiple actions if this conflict has been whitelisted in the grammar
	// * fail, terminating the parser generation process
	if !lookaheadsWithConflicts.IsEmpty() {
		// Only fnished items and items past their first step can take part in a
		// conflict. Most of a closure is neither, so find those items once per state.
		var candidates []*ParseItemSetEntry
		for i := range itemSet.Entries {
			if entry := &itemSet.Entries[i]; entry.Item.StepIndex > 0 || entry.Item.IsDone() {
				candidates = append(candidates, entry)
			}
		}
		for symbol := range lookaheadsWithConflicts.All() {
			if err := b.handleConflict(candidates, stateID, precedingSymbols, auxiliaryContext, symbol); err != nil {
				return err
			}
		}
	}

	// Add actions for the grammar's `extra` symbols.
	state := &b.parseTable.States[stateID]
	isEndOfNonTerminalExtra := state.IsEndOfNonTerminalExtra()

	// If this state represents the end of a non-terminal extra rule, then make sure that
	// it doesn't have other successor states. Non-terminal extra rules must have
	// unambiguous endings.
	if isEndOfNonTerminalExtra {
		if state.TerminalEntries.Len() > 1 {
			// Upstream collects the variable indices into an FxHashSet and
			// writes the names in the order of the set. fxhash.SetOrderU32
			// gives that order.
			var variableIndices []uint32
			for i := range itemSet.Entries {
				item := &itemSet.Entries[i].Item
				if !item.IsAugmented() && item.StepIndex > 0 {
					variableIndices = append(variableIndices, item.VariableIndex)
				}
			}
			parentSymbols := fxhash.SetOrderU32(variableIndices)
			parentSymbolNames := make([]string, len(parentSymbols))
			for i, variableIndex := range parentSymbols {
				parentSymbolNames[i] = b.strPool.Resolve(b.syntaxGrammar.Variables[variableIndex].Name)
			}

			return &ParseTableBuilderError{
				Kind:           ParseTableBuilderAmbiguousExtra,
				AmbiguousExtra: &AmbiguousExtraError{ParentSymbols: parentSymbolNames},
			}
		}
	} else {
		// Add actions for the start tokens of each non-terminal extra rule.
		for _, extra := range b.nonTerminalExtraStates {
			state.TerminalEntries.GetOrInsert(extra.terminal, ParseTableEntry{
				Reusable: true,
				Actions: ActionList{{
					Kind:         ParseActionShift,
					State:        extra.stateID,
					IsRepetition: false,
				}},
			})
		}

		// Add ShiftExtra actions for the terminal extra tokens. These actions
		// are added to every state except for those at the ends of non-terminal
		// extras.
		for _, extraToken := range b.syntaxGrammar.ExtraSymbols {
			switch extraToken.Kind() {
			case SymbolNonTerminal:
				state.NonterminalEntries.Insert(extraToken, GotoAction{Kind: GotoActionShiftExtra})
			case SymbolTerminal, SymbolExternal:
				state.TerminalEntries.GetOrInsert(extraToken, ParseTableEntry{
					Reusable: true,
					Actions:  ActionList{{Kind: ParseActionShiftExtra}},
				})
			case SymbolEnd, SymbolEndOfNonTerminalExtra:
				panic("generate: an extra symbol is the end of the input")
			}
		}
	}

	if b.syntaxGrammar.HasWordToken {
		keywordCaptureToken := b.syntaxGrammar.WordToken
		var reservedWordSetID ReservedWordSetID
		found := false
		for i := range itemSet.Entries {
			entry := &itemSet.Entries[i]
			var id ReservedWordSetID
			if nextStep, ok := entry.Item.Step(b.syntaxGrammar); ok {
				if nextStep.Symbol() != keywordCaptureToken {
					continue
				}
				id = ReservedWordSetID(nextStep.Reserved)
			} else if b.itemSetBuilder.Lookaheads.Get(entry.Lookaheads).Contains(keywordCaptureToken) {
				id = entry.FollowingReservedWordSet
			} else {
				continue
			}
			if !found || id > reservedWordSetID {
				reservedWordSetID = id
			}
			found = true
		}
		if found {
			state.ReservedWords = b.syntaxGrammar.ReservedWordSets[reservedWordSetID].Clone()
		}
	}

	return nil
}

// buildParseComparePrecedenceSymbol orders two pairs of a precedence and a
// symbol, by the precedence and then by the symbol.
//
// buildParseComparePrecedenceSymbol is the Ord of (Precedence, Symbol). The
// Ord of Precedence orders None, then Integer by its value, then Name by
// the id of the name.
func buildParseComparePrecedenceSymbol(a, b buildParsePrecedenceSymbol) int {
	if c := cmp.Compare(a.precedence.Kind, b.precedence.Kind); c != 0 {
		return c
	}
	switch a.precedence.Kind {
	case PrecedenceInteger:
		if c := cmp.Compare(a.precedence.Integer, b.precedence.Integer); c != 0 {
			return c
		}
	case PrecedenceName:
		if c := cmp.Compare(a.precedence.Name, b.precedence.Name); c != 0 {
			return c
		}
	}
	return CompareSymbol(a.symbol, b.symbol)
}

// buildParsePrecedenceSymbol is the precedence of a shift, and the symbol of
// the item that shifts.
//
// buildParsePrecedenceSymbol is (Precedence, Symbol).
type buildParsePrecedenceSymbol struct {
	precedence Precedence
	symbol     Symbol
}

// handleConflict resolves the conflict of a state for a lookahead, with the
// precedences and the associativities of the items, or with the conflicts
// that the grammar expects. It returns an error when it cannot resolve the
// conflict. candidates holds the entries of the state whose items are done
// or past their first step, in order.
//
// handleConflict is ParseTableBuilder::handle_conflict. Upstream keeps the
// conflicting items in a BTreeSet, which the Go form keeps as a slice
// sorted by the Ord of ParseItem, with no two items that compare equal.
func (b *parseTableBuilder) handleConflict(
	candidates []*ParseItemSetEntry,
	stateID ParseStateID,
	precedingSymbols []Symbol,
	auxiliaryContext auxiliaryContextID,
	conflictingLookahead Symbol,
) error {
	entry := b.parseTable.States[stateID].TerminalEntries.GetMut(conflictingLookahead)
	reductionInfo := b.reductionInfos.get(conflictingLookahead)

	// Determine which items in the set conflict with each other, and the
	// precedences associated with SHIFT vs REDUCE actions. There won't
	// be multiple REDUCE actions with different precedences; that is
	// sorted out ahead of time in `add_actions`. But there can still be
	// REDUCE-REDUCE conflicts where all actions have the *same*
	// precedence, and there can still be SHIFT/REDUCE conflicts.
	consideredAssociativity := false
	var shiftPrecedence []buildParsePrecedenceSymbol
	var conflictingItems []*ParseItem
	insertConflictingItem := func(item *ParseItem) {
		i, found := slices.BinarySearchFunc(conflictingItems, item, func(a, b *ParseItem) int {
			return a.Compare(*b)
		})
		if !found {
			conflictingItems = slices.Insert(conflictingItems, i, item)
		}
	}
	for _, e := range candidates {
		item := &e.Item
		if step, ok := item.Step(b.syntaxGrammar); ok {
			if item.StepIndex > 0 && b.itemSetBuilder.FirstSet(step.Symbol()).Contains(conflictingLookahead) {
				if item.VariableIndex != math.MaxUint32 {
					insertConflictingItem(item)
				}

				p := buildParsePrecedenceSymbol{
					precedence: item.Precedence(b.syntaxGrammar),
					symbol:     NonTerminalSymbol(int(item.VariableIndex)),
				}
				if i, found := slices.BinarySearchFunc(shiftPrecedence, p, buildParseComparePrecedenceSymbol); !found {
					shiftPrecedence = slices.Insert(shiftPrecedence, i, p)
				}
			}
		} else if b.itemSetBuilder.Lookaheads.Get(e.Lookaheads).Contains(conflictingLookahead) &&
			item.VariableIndex != math.MaxUint32 {
			insertConflictingItem(item)
		}
	}

	if last := &entry.Actions[len(entry.Actions)-1]; last.Kind == ParseActionShift {
		// If all of the items in the conflict have the same parent symbol,
		// and that parent symbols is auxiliary, then this is just the intentional
		// ambiguity associated with a repeat rule. Resolve that class of ambiguity
		// by leaving it in the parse table, but marking the SHIFT action with
		// an `is_repetition` flag.
		conflictingVariableIndex := conflictingItems[0].VariableIndex
		if b.syntaxGrammar.Variables[conflictingVariableIndex].IsAuxiliary() &&
			!slices.ContainsFunc(conflictingItems, func(item *ParseItem) bool {
				return item.VariableIndex != conflictingVariableIndex
			}) {
			last.IsRepetition = true
			return nil
		}

		// If the SHIFT action has higher precedence, remove all the REDUCE actions.
		shiftIsLess := false
		shiftIsEqual := false
		shiftIsMore := false
		for _, p := range shiftPrecedence {
			switch buildParseComparePrecedence(b.syntaxGrammar, p.precedence, []Symbol{p.symbol}, reductionInfo.precedence, reductionInfo.symbols) {
			case 1:
				shiftIsMore = true
			case -1:
				shiftIsLess = true
			case 0:
				shiftIsEqual = true
			}
		}

		isNotDone := func(item *ParseItem) bool { return !item.IsDone() }
		onlyRightAssoc := !reductionInfo.hasLeftAssoc && !reductionInfo.hasNonAssoc && reductionInfo.hasRightAssoc
		switch {
		case shiftIsMore && !shiftIsLess:
			entry.Actions.KeepLast()
		// If the REDUCE actions have higher precedence, remove the SHIFT action.
		case shiftIsLess && !shiftIsMore:
			// Exception: if one SHIFT interpretation ties the REDUCE actions in
			// precedence while another has lower precedence, and the REDUCE
			// actions are purely right associative, honor that right
			// associativity by shifting rather than reducing. The
			// lower-precedence interpretation coexists with the tying one, so on
			// its own it must not force a REDUCE that would flip the tie to left
			// associative.
			if shiftIsEqual && onlyRightAssoc {
				entry.Actions.KeepLast()
			} else {
				entry.Actions.Pop()
				conflictingItems = slices.DeleteFunc(conflictingItems, isNotDone)
			}
		// If the SHIFT and REDUCE actions have the same precedence, consider
		// the REDUCE actions' associativity.
		case !shiftIsLess && !shiftIsMore:
			consideredAssociativity = true

			// If all Reduce actions are left associative, remove the SHIFT action.
			// If all Reduce actions are right associative, remove the REDUCE actions.
			switch {
			case reductionInfo.hasLeftAssoc && !reductionInfo.hasNonAssoc && !reductionInfo.hasRightAssoc:
				entry.Actions.Pop()
				conflictingItems = slices.DeleteFunc(conflictingItems, isNotDone)
			case onlyRightAssoc:
				entry.Actions.KeepLast()
			}
		}
	}

	// If all of the actions but one have been eliminated, then there's no problem.
	entry = b.parseTable.States[stateID].TerminalEntries.GetMut(conflictingLookahead)
	if len(entry.Actions) == 1 {
		return nil
	}

	// Determine the set of parent symbols involved in this conflict.
	var actualConflict []Symbol
	for _, item := range conflictingItems {
		symbol := NonTerminalSymbol(int(item.VariableIndex))
		if b.syntaxGrammar.Variables[item.VariableIndex].IsAuxiliary() {
			parents, ok := b.auxiliaryContexts.parents(
				auxiliaryContext,
				NonTerminalIndex(item.VariableIndex),
			)
			if !ok {
				panic("generate: an auxiliary symbol of a conflict has no parent symbols")
			}
			actualConflict = append(actualConflict, parents...)
		} else {
			actualConflict = append(actualConflict, symbol)
		}
	}
	// Upstream sorts with sort_unstable. Two symbols that compare equal are
	// the same value, so the order is the same in Go.
	slices.SortFunc(actualConflict, CompareSymbol)
	actualConflict = slices.Compact(actualConflict)

	// If this set of symbols has been whitelisted, then there's no error.
	if slices.ContainsFunc(b.syntaxGrammar.ExpectedConflicts, func(c []Symbol) bool {
		return slices.Equal(c, actualConflict)
	}) {
		delete(b.actualConflicts, buildParseSymbolsKey(actualConflict))
		return nil
	}

	symbolSequence := make([]string, len(precedingSymbols))
	for i, symbol := range precedingSymbols {
		symbolSequence[i] = b.symbolName(symbol)
	}
	conflictError := &ConflictError{
		SymbolSequence:       symbolSequence,
		ConflictingLookahead: b.symbolName(conflictingLookahead),
	}

	interpretations := make([]Interpretation, 0, len(conflictingItems))
	for _, item := range conflictingItems {
		preceding := precedingSymbols[:len(precedingSymbols)-int(item.StepIndex)]
		precedingNames := make([]string, len(preceding))
		for i, symbol := range preceding {
			precedingNames[i] = b.symbolName(symbol)
		}

		variableName := b.strPool.Resolve(b.syntaxGrammar.Variables[item.VariableIndex].Name)

		steps := item.Production(b.syntaxGrammar).Steps
		productionStepSymbols := make([]string, len(steps))
		for i, step := range steps {
			productionStepSymbols[i] = b.symbolName(step.Symbol())
		}

		interpretation := Interpretation{
			PrecedingSymbols:      precedingNames,
			VariableName:          variableName,
			ProductionStepSymbols: productionStepSymbols,
			StepIndex:             item.StepIndex,
			Done:                  item.IsDone(),
			ConflictingLookahead:  b.symbolName(conflictingLookahead),
			RequiresEOFLookahead:  item.Production(b.syntaxGrammar).RequiresEOFLookahead,
		}
		if precedence := item.Precedence(b.syntaxGrammar); precedence.Kind != PrecedenceNone {
			interpretation.Precedence = PrecDisplay(precedence, b.strPool)
			interpretation.HasPrecedence = true
		}
		if assoc := item.Associativity(b.syntaxGrammar); assoc != AssociativityNone {
			interpretation.Associativity = associativityDebug(assoc)
			interpretation.HasAssociativity = true
		}
		interpretations = append(interpretations, interpretation)
	}
	conflictError.PossibleInterpretations = interpretations

	var shiftItems, reduceItems []*ParseItem
	for _, item := range conflictingItems {
		if item.IsDone() {
			reduceItems = append(reduceItems, item)
		} else {
			shiftItems = append(shiftItems, item)
		}
	}
	// Upstream sorts with sort_unstable. The items come from the BTreeSet in
	// order, and no two of them compare equal, so the sort does not change
	// the order in either language.
	compareItems := func(a, b *ParseItem) int { return a.Compare(*b) }
	slices.SortFunc(shiftItems, compareItems)
	slices.SortFunc(reduceItems, compareItems)

	getRuleNames := func(items []*ParseItem) []string {
		var lastRuleID uint32
		hasLastRuleID := false
		result := make([]string, 0, len(items))
		for _, item := range items {
			if hasLastRuleID && lastRuleID == item.VariableIndex {
				continue
			}
			lastRuleID, hasLastRuleID = item.VariableIndex, true
			result = append(result, b.symbolName(NonTerminalSymbol(int(item.VariableIndex))))
		}
		return result
	}

	if len(actualConflict) > 1 {
		if len(shiftItems) > 0 {
			names := getRuleNames(shiftItems)
			conflictError.PossibleResolutions = append(conflictError.PossibleResolutions, Resolution{
				Kind:    ResolutionPrecedence,
				Symbols: names,
			})
		}

		for _, item := range reduceItems {
			name := b.symbolName(NonTerminalSymbol(int(item.VariableIndex)))
			conflictError.PossibleResolutions = append(conflictError.PossibleResolutions, Resolution{
				Kind:    ResolutionPrecedence,
				Symbols: []string{name},
			})
		}
	}

	if consideredAssociativity {
		names := getRuleNames(reduceItems)
		conflictError.PossibleResolutions = append(conflictError.PossibleResolutions, Resolution{
			Kind:    ResolutionAssociativity,
			Symbols: names,
		})
	}

	conflictNames := make([]string, len(actualConflict))
	for i, s := range actualConflict {
		conflictNames[i] = b.symbolName(s)
	}
	conflictError.PossibleResolutions = append(conflictError.PossibleResolutions, Resolution{
		Kind:    ResolutionAddConflict,
		Symbols: conflictNames,
	})

	b.actualConflicts[buildParseSymbolsKey(actualConflict)] = actualConflict

	return &ParseTableBuilderError{Kind: ParseTableBuilderConflict, Conflict: conflictError}
}

// buildParseComparePrecedence orders two precedences, each with the symbols
// of its items. Two numbers compare as numbers, where none is zero. Two
// names, or a name and the name of a symbol, compare by the first list of
// the precedence orderings of the grammar that holds both. Precedences that
// no rule orders are equal.
//
// buildParseComparePrecedence is ParseTableBuilder::compare_precedence.
func buildParseComparePrecedence(
	grammar *SyntaxGrammar,
	left Precedence,
	leftSymbols []Symbol,
	right Precedence,
	rightSymbols []Symbol,
) int {
	precedenceEntryMatches := func(entry PrecedenceEntry, precedence Precedence, symbols []Symbol) bool {
		switch entry.Kind {
		case PrecedenceEntryName:
			return precedence.Kind == PrecedenceName && entry.Value == precedence.Name
		case PrecedenceEntrySymbol:
			return slices.ContainsFunc(symbols, func(s Symbol) bool {
				index, ok := s.NonTerminalIndex()
				return ok && grammar.Variables[index].Name == entry.Value
			})
		}
		return false
	}

	switch {
	// Integer precedences can be compared to other integer precedences,
	// and to the default precedence, which is zero.
	case left.Kind == PrecedenceInteger && right.Kind == PrecedenceInteger && (left.Integer != 0 || right.Integer != 0):
		return cmp.Compare(left.Integer, right.Integer)
	case left.Kind == PrecedenceInteger && right.Kind == PrecedenceNone && left.Integer != 0:
		return cmp.Compare(left.Integer, 0)
	case left.Kind == PrecedenceNone && right.Kind == PrecedenceInteger && right.Integer != 0:
		return cmp.Compare(0, right.Integer)
	}

	// Named precedences can be compared to other named precedences.
	for _, list := range grammar.PrecedenceOrderings {
		sawLeft := false
		sawRight := false
		for _, entry := range list {
			matchesLeft := precedenceEntryMatches(entry, left, leftSymbols)
			matchesRight := precedenceEntryMatches(entry, right, rightSymbols)
			if matchesLeft {
				sawLeft = true
				if sawRight {
					return -1
				}
			} else if matchesRight {
				sawRight = true
				if sawLeft {
					return 1
				}
			}
		}
	}
	return 0
}

// getProductionID returns the id of the production info of an item: the
// aliases and the fields of its production. It adds the info to the table
// when no production has the same info yet.
//
// getProductionID is ParseTableBuilder::get_production_id. Upstream loops
// over the keys of the FxHashMap of the fields of a hidden child. Each key
// adds one location to its own list, in the order of the steps, so the order
// of the keys does not reach the output.
func (b *parseTableBuilder) getProductionID(item *ParseItem) ProductionInfoID {
	if id := b.productionInfoIDsByProdID[item.ProdID]; id != buildParseNoProductionInfoID {
		return id
	}
	productionInfo := ProductionInfo{
		AliasSequence: nil,
		FieldMap:      FieldMap{},
	}

	steps := item.Production(b.syntaxGrammar).Steps
	for i, step := range steps {
		alias, _ := step.GetAlias()
		productionInfo.AliasSequence = append(productionInfo.AliasSequence, alias)
		if fieldName, ok := step.GetField(); ok {
			productionInfo.FieldMap[fieldName] = append(productionInfo.FieldMap[fieldName], FieldLocation{
				Index:     uint32(i),
				Inherited: false,
			})
		}

		if index, ok := step.NonTerminalIndex(); ok && !b.syntaxGrammar.Variables[index].Kind.IsVisible() {
			info := &b.variableInfo[index]
			for fieldName := range info.Fields {
				productionInfo.FieldMap[fieldName] = append(productionInfo.FieldMap[fieldName], FieldLocation{
					Index:     uint32(i),
					Inherited: true,
				})
			}
		}
	}

	for n := len(productionInfo.AliasSequence); n > 0 && productionInfo.AliasSequence[n-1] == (Alias{}); n-- {
		productionInfo.AliasSequence = productionInfo.AliasSequence[:n-1]
	}

	if len(steps) > b.parseTable.MaxAliasedProductionLength {
		b.parseTable.MaxAliasedProductionLength = len(steps)
	}

	index := slices.IndexFunc(b.parseTable.ProductionInfos, func(seq ProductionInfo) bool {
		return seq.Equal(&productionInfo)
	})
	if index < 0 {
		b.parseTable.ProductionInfos = append(b.parseTable.ProductionInfos, productionInfo)
		index = len(b.parseTable.ProductionInfos) - 1
	}
	id := ProductionInfoID(index)
	b.productionInfoIDsByProdID[item.ProdID] = id
	return id
}

// symbolName returns the name of a symbol for the text of a conflict. An
// anonymous terminal has quotes around its name, and the end of the input is
// EOF.
//
// symbolName is ParseTableBuilder::symbol_name.
func (b *parseTableBuilder) symbolName(symbol Symbol) string {
	switch symbol.Kind() {
	case SymbolEnd, SymbolEndOfNonTerminalExtra:
		return "EOF"
	case SymbolExternal:
		return b.strPool.Resolve(b.syntaxGrammar.ExternalTokens[symbol.index].Name)
	case SymbolNonTerminal:
		return b.strPool.Resolve(b.syntaxGrammar.Variables[symbol.index].Name)
	case SymbolTerminal:
		variable := &b.lexicalGrammar.Variables[symbol.index]
		if variable.Kind == VariableNamed {
			return b.strPool.Resolve(variable.Name)
		}
		return "'" + b.strPool.Resolve(variable.Name) + "'"
	}
	return ""
}

// BuildParseTable builds the parse table of a grammar, and returns it with
// the information about its states. It adds a diagnostic for the expected
// conflicts that the grammar does not need. The error is a
// *ParseTableBuilderError.
//
// BuildParseTable is build_parse_table.
func BuildParseTable(
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	itemSetBuilder *ParseItemSetBuilder,
	variableInfo []VariableInfo,
	strPool *StrPool,
	diagnostics *[]Diagnostic,
) (ParseTable[ParseTableEntry], *ParseStateInfo, error) {
	return newParseTableBuilder(
		syntaxGrammar,
		lexicalGrammar,
		itemSetBuilder,
		variableInfo,
		strPool,
	).build(diagnostics)
}
