package generate

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

// This file ports crates/generate/src/build_tables/item_set_builder.rs: the
// FIRST and LAST sets of the symbols, and the transitive closure of an item
// set. The Debug text of ParseItemSetBuilder is a debugging aid that upstream
// does not use, and it is not ported.

// transitiveClosureAddition is an item that the closure adds when a
// non-terminal is the next symbol of an item, with its lookahead information.
//
// transitiveClosureAddition is TransitiveClosureAddition.
type transitiveClosureAddition struct {
	item ParseItem
	info additionInfo
	// rank is item's rank, which locates its entry in a closure.
	rank firstStepRank
}

// itemAddition is an addition before it has a rank: an item, with its
// lookahead information.
//
// itemAddition is (ParseItem, AdditionInfo).
type itemAddition struct {
	item ParseItem
	info additionInfo
}

// itemAdditionKey is a value that is the same for two additions exactly when
// they are equal, so it can be the key of a Go map.
//
// itemAdditionKey is what the Hash of (ParseItem, AdditionInfo) hashes.
type itemAdditionKey struct {
	item ParseItemKey
	info additionInfo
}

// key returns the key of the addition.
//
// key stands in for the Hash and the PartialEq of (ParseItem, AdditionInfo).
func (a *itemAddition) key() itemAdditionKey {
	return itemAdditionKey{item: a.item.Key(), info: a.info}
}

// additionInfo is a followSetInfo with an interned lookahead set, and with
// whether the set holds the word token.
//
// additionInfo is AdditionInfo.
type additionInfo struct {
	lookaheads           LookaheadSetID
	reservedLookaheads   ReservedWordSetID
	propagatesLookaheads bool
	containsWord         bool
}

// followSetInfo is the accumulator of the follow set of a non-terminal that
// the builder changes. lookaheads is a TokenSet that the traversal can join
// step by step, and it is interned into an additionInfo once it is complete.
//
// followSetInfo is FollowSetInfo.
type followSetInfo struct {
	lookaheads           TokenSet
	reservedLookaheads   ReservedWordSetID
	propagatesLookaheads bool
}

// ParseItemSetBuilder computes the transitive closure of item sets.
//
// ParseItemSetBuilder is ParseItemSetBuilder.
type ParseItemSetBuilder struct {
	KeyMap     *ItemKeyMap
	Lookaheads *LookaheadSetPool

	syntaxGrammar *SyntaxGrammar
	firstSets     map[Symbol]*TokenSet
	// firstSetIDs holds each FIRST set interned, for the propagation of the
	// closure by id.
	firstSetIDs                map[Symbol]LookaheadSetID
	reservedFirstSets          map[Symbol]ReservedWordSetID
	lastSets                   map[Symbol]*TokenSet
	inlines                    *InlinedProductionMap
	transitiveClosureAdditions [][]transitiveClosureAddition
	// closureScratch is scratch for TransitiveClosure.
	closureScratch closureEntries
}

// pushUnique pushes value unless vector already has it. indices holds the
// positions of vector's elements, by key.
//
// pushUnique is push_unique. Upstream finds a position in a HashTable of
// positions. The Go form maps the key of each element to its position, and
// only looks it up.
func pushUnique(vector *[]itemAddition, indices map[itemAdditionKey]uint32, value itemAddition) {
	key := value.key()
	if _, ok := indices[key]; ok {
		return
	}
	indices[key] = uint32(len(*vector))
	*vector = append(*vector, value)
}

// firstStepRank is an item's position among all the items that a closure
// can hold at their first step (StepIndex 0), in item order. See
// firstStepRanks.
//
// firstStepRank is FirstStepRank.
type firstStepRank uint32

// index returns the rank as an index.
//
// index is FirstStepRank::index.
func (r firstStepRank) index() int {
	return int(r)
}

// firstStepKey is everything that ParseItem's Equal and Compare compare
// between items at their first step, which have nothing before their dot.
// The fields are declared in the order that Compare compares them, so
// compareFirstStepKey orders keys the way Compare orders their items.
//
// firstStepKey is FirstStepKey.
type firstStepKey struct {
	variableIndex uint32
	// cmp is Keys[0].Cmp.
	cmp uint32
}

// newFirstStepKey returns the key of an item at its first step.
//
// newFirstStepKey is FirstStepKey::new.
func newFirstStepKey(item *ParseItem) firstStepKey {
	// INVARIANT: only items at their first step are ranked
	if item.StepIndex != 0 || item.HasPrecedingInheritedFields {
		panic("generate: a ranked item is past its first step")
	}
	return firstStepKey{
		variableIndex: item.VariableIndex,
		cmp:           item.Keys[0].Cmp,
	}
}

// compareFirstStepKey orders two keys by the variable index and then by cmp.
//
// compareFirstStepKey is the derived Ord of FirstStepKey.
func compareFirstStepKey(a, b firstStepKey) int {
	if c := cmp.Compare(a.variableIndex, b.variableIndex); c != 0 {
		return c
	}
	return cmp.Compare(a.cmp, b.cmp)
}

// firstStepRanks holds the firstStepRank of every item that a closure can
// hold at its first step, by key.
//
// firstStepRanks is FirstStepRanks. Upstream keeps the ranks in an
// FxHashMap, and only looks it up.
type firstStepRanks struct {
	ranks map[firstStepKey]firstStepRank
}

// newFirstStepRanks ranks items, which are all at their first step.
//
// newFirstStepRanks is FirstStepRanks::new. Upstream sorts with
// sort_unstable. Two keys that compare equal are the same value, so the
// order is the same in Go.
func newFirstStepRanks(items iter.Seq[ParseItem]) firstStepRanks {
	var keys []firstStepKey
	for item := range items {
		keys = append(keys, newFirstStepKey(&item))
	}
	slices.SortFunc(keys, compareFirstStepKey)
	keys = slices.Compact(keys)
	ranks := make(map[firstStepKey]firstStepRank, len(keys))
	for rank, key := range keys {
		ranks[key] = firstStepRank(rank)
	}
	return firstStepRanks{ranks: ranks}
}

// len returns the number of ranks.
//
// len is FirstStepRanks::len.
func (r *firstStepRanks) len() int {
	return len(r.ranks)
}

// rank returns the rank of item, which must be one of the items these were
// built from.
//
// rank is FirstStepRanks::rank.
func (r *firstStepRanks) rank(item *ParseItem) firstStepRank {
	// INVARIANT: every item at its first step has a rank
	rank, ok := r.ranks[newFirstStepKey(item)]
	if !ok {
		panic("generate: an item at its first step has no rank")
	}
	return rank
}

// closureEntries holds the entries of a closure while
// ParseItemSetBuilder.TransitiveClosure builds it: found by item as they're
// added, and taken out in item order once it's complete.
//
// Items are ordered by StepIndex first, so the items at their first step
// come before all the others. Most of a closure's items are at their first
// step, and each of those has a firstStepRank and a slot for its entry. The
// rest are kernel items past their first step, which are found by key and
// sorted.
//
// closureEntries is ClosureEntries. The zero value is
// ClosureEntries::default. Upstream holds each slot of firstStep as an
// Option, and the Go form holds the entry, which is in use exactly when its
// bit of occupied is set. Upstream finds a position of later in a HashTable
// of positions. The Go form maps the key of each item to its position, and
// only looks it up.
type closureEntries struct {
	// ranks holds the rank of every item that can be in firstStep.
	ranks firstStepRanks
	// firstStep holds the entry of each first-step item in the closure, by
	// rank.
	firstStep []ParseItemSetEntry
	// occupied holds the ranks that have an entry in firstStep, to find
	// those without scanning it.
	occupied BitVec
	// firstStepLen is the number of entries in firstStep.
	firstStepLen int
	// later holds the entries of the items past their first step, in the
	// order they were added.
	later []ParseItemSetEntry
	// laterIndices holds the positions of later's entries, by the key of
	// their items.
	laterIndices map[ParseItemKey]uint32
}

// newClosureEntries returns the entries of an empty closure, for items with
// the given ranks.
//
// newClosureEntries is ClosureEntries::new.
func newClosureEntries(ranks firstStepRanks) closureEntries {
	var occupied BitVec
	occupied.Resize(ranks.len(), false)
	return closureEntries{
		firstStep:    make([]ParseItemSetEntry, ranks.len()),
		ranks:        ranks,
		occupied:     occupied,
		firstStepLen: 0,
		later:        nil,
		laterIndices: make(map[ParseItemKey]uint32),
	}
}

// additionEntry returns the entry of addition's item, added if it's new.
//
// additionEntry is ClosureEntries::addition_entry.
func (c *closureEntries) additionEntry(addition *transitiveClosureAddition) *ParseItemSetEntry {
	return c.firstStepEntry(addition.rank, addition.item)
}

// kernelEntry returns the entry of item, from the closure's kernel, added if
// it's new. The pointer is valid until the next kernelEntry.
//
// kernelEntry is ClosureEntries::kernel_entry.
func (c *closureEntries) kernelEntry(item ParseItem) *ParseItemSetEntry {
	// A kernel's items are past their first step, except for the start item.
	if item.StepIndex == 0 {
		rank := c.ranks.rank(&item)
		return c.firstStepEntry(rank, item)
	}
	return c.laterEntry(item)
}

// firstStepEntry returns the entry of item, which is at its first step and
// has the given rank, added if it's new.
//
// firstStepEntry is ClosureEntries::first_step_entry.
func (c *closureEntries) firstStepEntry(rank firstStepRank, item ParseItem) *ParseItemSetEntry {
	slot := &c.firstStep[rank.index()]
	if occupied, _ := c.occupied.Get(rank.index()); !occupied {
		c.occupied.Set(rank.index(), true)
		c.firstStepLen++
		*slot = closureNewEntry(item)
	}
	return slot
}

// laterEntry returns the entry of item, which is past its first step, added
// if it's new. The pointer is valid until the next laterEntry.
//
// laterEntry is ClosureEntries::later_entry.
func (c *closureEntries) laterEntry(item ParseItem) *ParseItemSetEntry {
	key := item.Key()
	index, ok := c.laterIndices[key]
	if !ok {
		index = uint32(len(c.later))
		c.laterIndices[key] = index
		c.later = append(c.later, closureNewEntry(item))
	}
	return &c.later[index]
}

// closureNewEntry returns an entry for item, with no lookaheads yet.
//
// closureNewEntry is ClosureEntries::new_entry.
func closureNewEntry(item ParseItem) ParseItemSetEntry {
	return ParseItemSetEntry{
		Item:                     item,
		Lookaheads:               LookaheadSetPoolEmpty,
		FollowingReservedWordSet: 0,
	}
}

// takeItemSet takes the entries out in item order, leaving c empty for the
// next closure.
//
// takeItemSet is ClosureEntries::take_item_set. Upstream sorts with
// sort_unstable_by. No two items of a closure are equal, so the order is the
// same in Go.
func (c *closureEntries) takeItemSet() ParseItemSet {
	entries := make([]ParseItemSetEntry, 0, c.firstStepLen+len(c.later))
	for rank := range setBits(c.occupied.Words()) {
		// INVARIANT: occupied ranks have entries
		entries = append(entries, c.firstStep[rank])
		c.firstStep[rank] = ParseItemSetEntry{}
	}
	c.occupied.UnsetAll()
	c.firstStepLen = 0
	// Past their first step, these all come after the entries above.
	slices.SortFunc(c.later, func(a, b ParseItemSetEntry) int {
		return a.Item.Compare(b.Item)
	})
	entries = append(entries, c.later...)
	clear(c.later)
	c.later = c.later[:0]
	clear(c.laterIndices)
	return ParseItemSet{Entries: entries}
}

// followSetStackEntry is an entry of the stack of NewParseItemSetBuilder.
type followSetStackEntry struct {
	symIx                int
	lookaheads           *TokenSet
	reservedWordSetID    ReservedWordSetID
	propagatesLookaheads bool
}

// NewParseItemSetBuilder returns a builder for a grammar, with the FIRST and
// LAST sets of its symbols and the additions of each non-terminal.
//
// NewParseItemSetBuilder is ParseItemSetBuilder::new.
func NewParseItemSetBuilder(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, inlines *InlinedProductionMap, keyMap *ItemKeyMap) *ParseItemSetBuilder {
	result := &ParseItemSetBuilder{
		syntaxGrammar:              syntaxGrammar,
		firstSets:                  make(map[Symbol]*TokenSet),
		firstSetIDs:                make(map[Symbol]LookaheadSetID),
		reservedFirstSets:          make(map[Symbol]ReservedWordSetID),
		lastSets:                   make(map[Symbol]*TokenSet),
		inlines:                    inlines,
		KeyMap:                     keyMap,
		Lookaheads:                 NewLookaheadSetPool(),
		transitiveClosureAdditions: nil,
		closureScratch:             closureEntries{},
	}

	// For each grammar symbol, populate the FIRST and LAST sets: the set of
	// terminals that appear at the beginning and end that symbol's productions,
	// respectively.
	// For a terminal symbol, the FIRST and LAST sets just consist of the
	// terminal itself.
	nTerminals := len(lexicalGrammar.Variables)
	nExternals := len(syntaxGrammar.ExternalTokens)

	for i := range nTerminals {
		symbol := TerminalSymbol(i)
		set := NewTokenSetWithCapacity(nTerminals, nExternals)
		set.Insert(symbol)
		first := set.Clone()
		result.firstSets[symbol] = &first
		result.lastSets[symbol] = &set
		result.reservedFirstSets[symbol] = 0
	}

	for i := range nExternals {
		symbol := ExternalSymbol(i)
		set := NewTokenSetWithCapacity(nTerminals, nExternals)
		set.Insert(symbol)
		first := set.Clone()
		result.firstSets[symbol] = &first
		result.lastSets[symbol] = &set
		result.reservedFirstSets[symbol] = 0
	}

	// The FIRST set of a non-terminal `i` is the union of the FIRST sets
	// of all the symbols that appear at the beginnings of i's productions. Some
	// of these symbols may themselves be non-terminals, so this is a recursive
	// definition.
	//
	// Rather than computing these sets using recursion, we use an explicit stack
	// called `symbols_to_process`.
	//
	// Upstream holds processed_non_terminals in an FxHashSet, and only looks
	// it up.
	var symbolsToProcess []NonTerminalIndex
	processedNonTerminals := make(map[NonTerminalIndex]bool)
	for i := range syntaxGrammar.Variables {
		rootIndex := NonTerminalIndex(i)
		symbol := NonTerminalSymbol(i)
		firstSet := result.firstSets[symbol]
		if firstSet == nil {
			firstSet = new(TokenSet)
			result.firstSets[symbol] = firstSet
		}
		reservedFirstSet := result.reservedFirstSets[symbol]

		clear(processedNonTerminals)
		symbolsToProcess = append(symbolsToProcess[:0], rootIndex)
		for len(symbolsToProcess) > 0 {
			index := symbolsToProcess[len(symbolsToProcess)-1]
			symbolsToProcess = symbolsToProcess[:len(symbolsToProcess)-1]
			start, end := syntaxGrammar.VariableProdIDs(int(index))
			for prodID := start; prodID < end; prodID++ {
				steps := syntaxGrammar.Production(prodID).Steps
				if len(steps) == 0 {
					continue
				}
				step := steps[0]
				symbol := step.Symbol()
				switch symbol.Kind() {
				case SymbolTerminal, SymbolExternal:
					firstSet.Insert(symbol)
				case SymbolNonTerminal:
					index, _ := symbol.NonTerminalIndex()
					if !processedNonTerminals[index] {
						processedNonTerminals[index] = true
						symbolsToProcess = append(symbolsToProcess, index)
					}
				default:
					panic("unreachable")
				}
				reservedFirstSet = max(reservedFirstSet, ReservedWordSetID(step.Reserved))
			}
		}
		result.reservedFirstSets[symbol] = reservedFirstSet

		// The LAST set is defined in a similar way to the FIRST set.
		lastSet := result.lastSets[symbol]
		if lastSet == nil {
			lastSet = new(TokenSet)
			result.lastSets[symbol] = lastSet
		}
		clear(processedNonTerminals)
		symbolsToProcess = append(symbolsToProcess[:0], rootIndex)
		for len(symbolsToProcess) > 0 {
			index := symbolsToProcess[len(symbolsToProcess)-1]
			symbolsToProcess = symbolsToProcess[:len(symbolsToProcess)-1]
			start, end := syntaxGrammar.VariableProdIDs(int(index))
			for prodID := start; prodID < end; prodID++ {
				steps := syntaxGrammar.Production(prodID).Steps
				if len(steps) == 0 {
					continue
				}
				symbol := steps[len(steps)-1].Symbol()
				switch symbol.Kind() {
				case SymbolTerminal, SymbolExternal:
					lastSet.Insert(symbol)
				case SymbolNonTerminal:
					index, _ := symbol.NonTerminalIndex()
					if !processedNonTerminals[index] {
						processedNonTerminals[index] = true
						symbolsToProcess = append(symbolsToProcess, index)
					}
				default:
					panic("unreachable")
				}
			}
		}
	}

	// Intern each FIRST set so closure propagation can union by id.
	//
	// Upstream walks the FxHashMap first_sets, so its order is the order of
	// the hash table. The order decides only which number each new set gets
	// in the pool. An id is canonical, and nothing orders ids or walks the
	// pool to make output, so the numbers cannot reach the output. Go walks
	// the symbols in order.
	for _, symbol := range slices.SortedFunc(maps.Keys(result.firstSets), CompareSymbol) {
		result.firstSetIDs[symbol] = result.Lookaheads.InternRef(result.firstSets[symbol])
	}

	// To compute an item set's transitive closure, we find each item in the set
	// whose next symbol is a non-terminal, and we add new items to the set for
	// each of that symbol's productions. These productions might themselves begin
	// with non-terminals, so the process continues recursively. In this process,
	// the total set of entries that get added depends only on two things:
	//
	//   * the non-terminal symbol that occurs next in each item
	//
	//   * the set of terminals that can follow that non-terminal symbol in the item
	//
	// So we can avoid a lot of duplicated recursive work by precomputing, for each
	// non-terminal symbol `i`, a final list of *additions* that must be made to an
	// item set when symbol `i` occurs as the next symbol in one if its core items.
	// The structure of a precomputed *addition* is as follows:
	//
	//   * `item` - the new item that must be added as part of the expansion of the symbol `i`.
	//
	//   * `lookaheads` - the set of possible lookahead tokens that can always come after `item`
	//     in an expansion of symbol `i`.
	//
	//   * `reserved_lookaheads` - the set of reserved lookahead tokens that can always come
	//     after `item` in the expansion of symbol `i`.
	//
	//   * `propagates_lookaheads` - a boolean indicating whether or not `item` can occur at the
	//     *end* of the expansion of symbol `i`, so that i's own current lookahead tokens can
	//     occur after `item`.
	//
	// Rather than computing these additions recursively, we use an explicit stack.
	emptyLookaheads := TokenSet{}
	var eofLookaheads TokenSet
	eofLookaheads.Insert(SymbolEndValue)
	var stack []followSetStackEntry
	followSetInfoByNonTerminal := make(map[int]*followSetInfo)
	additions := make([][]itemAddition, len(syntaxGrammar.Variables))
	additionIndices := make(map[itemAdditionKey]uint32)
	for i := range additions {
		additionsForNonTerminal := &additions[i]
		// First, build up a map whose keys are all of the non-terminals that can
		// appear at the beginning of non-terminal `i`, and whose values store
		// information about the tokens that can follow those non-terminals.
		stack = append(stack[:0], followSetStackEntry{
			symIx:                i,
			lookaheads:           &emptyLookaheads,
			reservedWordSetID:    0,
			propagatesLookaheads: true,
		})
		clear(followSetInfoByNonTerminal)
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			didAdd := false
			info := followSetInfoByNonTerminal[top.symIx]
			if info == nil {
				info = new(followSetInfo)
				followSetInfoByNonTerminal[top.symIx] = info
			}
			didAdd = info.lookaheads.InsertAll(top.lookaheads) || didAdd
			if top.reservedWordSetID > info.reservedLookaheads {
				info.reservedLookaheads = top.reservedWordSetID
				didAdd = true
			}
			didAdd = didAdd || (top.propagatesLookaheads && !info.propagatesLookaheads)
			info.propagatesLookaheads = info.propagatesLookaheads || top.propagatesLookaheads
			if !didAdd {
				continue
			}

			start, end := syntaxGrammar.VariableProdIDs(top.symIx)
			for prodID := start; prodID < end; prodID++ {
				production := syntaxGrammar.Production(prodID)
				first, ok := production.FirstSymbol()
				if !ok {
					continue
				}
				index, ok := first.NonTerminalIndex()
				if !ok {
					continue
				}
				switch {
				case len(production.Steps) > 1:
					nextSymbol := production.Steps[1].Symbol()
					stack = append(stack, followSetStackEntry{
						symIx:                int(index),
						lookaheads:           result.mustFirstSet(nextSymbol),
						reservedWordSetID:    result.mustReservedFirstSet(nextSymbol),
						propagatesLookaheads: false,
					})
				case production.RequiresEOFLookahead:
					stack = append(stack, followSetStackEntry{
						symIx:                int(index),
						lookaheads:           &eofLookaheads,
						reservedWordSetID:    0,
						propagatesLookaheads: false,
					})
				default:
					stack = append(stack, followSetStackEntry{
						symIx:                int(index),
						lookaheads:           top.lookaheads,
						reservedWordSetID:    top.reservedWordSetID,
						propagatesLookaheads: top.propagatesLookaheads,
					})
				}
			}
		}

		// Store all of those non-terminals' productions, along with their associated
		// lookahead info, as *additions* associated with non-terminal `i`.
		//
		// Upstream walks the FxHashMap follow_set_info_by_non_terminal, so its
		// order is the order of the hash table. The order decides the order of
		// the additions of i and which number each new lookahead set gets in
		// the pool. AddItem adds each addition to the closure once, and
		// TransitiveClosure sorts the closure by item at the end. AddItem
		// joins lookahead sets and takes the maximum of reserved word sets,
		// which do not depend on the order. So the order changes only the
		// numbers of the lookahead sets. An id is canonical, and nothing
		// orders ids or walks the pool to make output, so the numbers cannot
		// reach the output. Go walks the variables in order.
		clear(additionIndices)
		for _, variableIndex := range slices.Sorted(maps.Keys(followSetInfoByNonTerminal)) {
			followSetInfo := followSetInfoByNonTerminal[variableIndex]
			nonTerminal := NonTerminalSymbol(variableIndex)
			if slices.Contains(syntaxGrammar.VariablesToInline, nonTerminal) {
				continue
			}
			info := additionInfo{
				lookaheads:           result.Lookaheads.InternRef(&followSetInfo.lookaheads),
				reservedLookaheads:   followSetInfo.reservedLookaheads,
				propagatesLookaheads: followSetInfo.propagatesLookaheads,
				containsWord:         syntaxGrammar.HasWordToken && followSetInfo.lookaheads.Contains(syntaxGrammar.WordToken),
			}
			start, end := syntaxGrammar.VariableProdIDs(variableIndex)
			for prodID := start; prodID < end; prodID++ {
				item := ParseItem{
					VariableIndex:               uint32(variableIndex),
					ProdID:                      prodID,
					Keys:                        keyMap.KeysFor(prodID),
					StepIndex:                   0,
					HasPrecedingInheritedFields: false,
				}

				if ids, ok := inlines.InlinedProdIDs(item.ProdID, item.StepIndex); ok {
					for _, id := range ids {
						itemInfo := info
						if syntaxGrammar.Production(id).RequiresEOFLookahead {
							itemInfo.lookaheads = result.Lookaheads.InternRef(&eofLookaheads)
							itemInfo.reservedLookaheads = 0
							itemInfo.propagatesLookaheads = false
							itemInfo.containsWord = false
						}
						pushUnique(additionsForNonTerminal, additionIndices, itemAddition{
							item: item.SubstituteProduction(id, keyMap.KeysFor(id)),
							info: itemInfo,
						})
					}
				} else {
					itemInfo := info
					if syntaxGrammar.Production(prodID).RequiresEOFLookahead {
						itemInfo.lookaheads = result.Lookaheads.InternRef(&eofLookaheads)
						itemInfo.reservedLookaheads = 0
						itemInfo.propagatesLookaheads = false
						itemInfo.containsWord = false
					}
					pushUnique(additionsForNonTerminal, additionIndices, itemAddition{
						item: item,
						info: itemInfo,
					})
				}
			}
		}
	}

	// Besides these additions, the only item a closure holds at its first step is the start
	// item, in the start state's kernel.
	ranks := newFirstStepRanks(func(yield func(ParseItem) bool) {
		for _, additionsForNonTerminal := range additions {
			for _, addition := range additionsForNonTerminal {
				if !yield(addition.item) {
					return
				}
			}
		}
		yield(StartParseItem(keyMap))
	})
	result.transitiveClosureAdditions = make([][]transitiveClosureAddition, len(additions))
	for i, additionsForNonTerminal := range additions {
		result.transitiveClosureAdditions[i] = make([]transitiveClosureAddition, len(additionsForNonTerminal))
		for j, addition := range additionsForNonTerminal {
			result.transitiveClosureAdditions[i][j] = transitiveClosureAddition{
				item: addition.item,
				info: addition.info,
				rank: ranks.rank(&addition.item),
			}
		}
	}
	result.closureScratch = newClosureEntries(ranks)

	return result
}

// TransitiveClosure returns the closure of an item set: its items, with each
// symbol that is inlined replaced by its productions, and the items that
// each non-terminal after a dot adds.
//
// TransitiveClosure is ParseItemSetBuilder::transitive_closure.
func (b *ParseItemSetBuilder) TransitiveClosure(itemSet *ParseItemSet) ParseItemSet {
	for i := range itemSet.Entries {
		entry := &itemSet.Entries[i]
		if ids, ok := b.inlines.InlinedProdIDs(entry.Item.ProdID, entry.Item.StepIndex); ok {
			for _, id := range ids {
				b.addItem(&ParseItemSetEntry{
					Item:                     entry.Item.SubstituteProduction(id, b.KeyMap.KeysFor(id)),
					Lookaheads:               entry.Lookaheads,
					FollowingReservedWordSet: entry.FollowingReservedWordSet,
				})
			}
		} else {
			b.addItem(entry)
		}
	}

	return b.closureScratch.takeItemSet()
}

// FirstSet returns the FIRST set of a symbol. It panics when the builder has
// no FIRST set for the symbol, as upstream does.
//
// FirstSet is ParseItemSetBuilder::first_set.
func (b *ParseItemSetBuilder) FirstSet(symbol Symbol) *TokenSet {
	return b.mustFirstSet(symbol)
}

// mustFirstSet returns the FIRST set of a symbol, and panics when there is
// none, as the index of an FxHashMap does upstream.
func (b *ParseItemSetBuilder) mustFirstSet(symbol Symbol) *TokenSet {
	set, ok := b.firstSets[symbol]
	if !ok {
		panic("generate: no FIRST set for a symbol")
	}
	return set
}

// mustReservedFirstSet returns the reserved word set at the start of a
// symbol, and panics when there is none, as the index of an FxHashMap does
// upstream.
func (b *ParseItemSetBuilder) mustReservedFirstSet(symbol Symbol) ReservedWordSetID {
	id, ok := b.reservedFirstSets[symbol]
	if !ok {
		panic("generate: no reserved FIRST set for a symbol")
	}
	return id
}

// ReservedFirstSet returns the reserved words that can start a symbol, and
// false when the builder has none for the symbol.
//
// ReservedFirstSet is ParseItemSetBuilder::reserved_first_set.
func (b *ParseItemSetBuilder) ReservedFirstSet(symbol Symbol) (*TokenSet, bool) {
	id, ok := b.reservedFirstSets[symbol]
	if !ok {
		return nil, false
	}
	return &b.syntaxGrammar.ReservedWordSets[id], true
}

// LastSet returns the LAST set of a symbol. It panics when the builder has no
// LAST set for the symbol, as upstream does.
//
// LastSet is ParseItemSetBuilder::last_set.
func (b *ParseItemSetBuilder) LastSet(symbol Symbol) *TokenSet {
	set, ok := b.lastSets[symbol]
	if !ok {
		panic("generate: no LAST set for a symbol")
	}
	return set
}

// addItem adds an entry to a set, with the additions of the non-terminal
// after its dot.
//
// addItem is ParseItemSetBuilder::add_item.
func (b *ParseItemSetBuilder) addItem(entry *ParseItemSetEntry) {
	if step, ok := entry.Item.Step(b.syntaxGrammar); ok {
		if index, ok := step.NonTerminalIndex(); ok {
			successor := entry.Item.Successor()

			// Determine which tokens can follow this non-terminal.
			var followingTokens LookaheadSetID
			var followingReservedTokens ReservedWordSetID
			if nextStep, ok := successor.Step(b.syntaxGrammar); ok {
				key := nextStep.Symbol()
				id, ok := b.firstSetIDs[key]
				if !ok {
					panic("generate: no FIRST set id for a symbol")
				}
				followingTokens, followingReservedTokens = id, b.mustReservedFirstSet(key)
			} else {
				followingTokens, followingReservedTokens = entry.Lookaheads, entry.FollowingReservedWordSet
			}

			// Use the pre-computed *additions* to expand the non-terminal.
			for i := range b.transitiveClosureAdditions[index] {
				addition := &b.transitiveClosureAdditions[index][i]
				e := b.closureScratch.additionEntry(addition)
				e.Lookaheads = b.Lookaheads.Union(e.Lookaheads, addition.info.lookaheads)

				if addition.info.containsWord {
					e.FollowingReservedWordSet = max(e.FollowingReservedWordSet, addition.info.reservedLookaheads)
				}

				if addition.info.propagatesLookaheads {
					e.Lookaheads = b.Lookaheads.Union(e.Lookaheads, followingTokens)

					if b.syntaxGrammar.HasWordToken && b.Lookaheads.Get(followingTokens).Contains(b.syntaxGrammar.WordToken) {
						e.FollowingReservedWordSet = max(e.FollowingReservedWordSet, followingReservedTokens)
					}
				}
			}
		}
	}

	e := b.closureScratch.kernelEntry(entry.Item)
	e.Lookaheads = b.Lookaheads.Union(e.Lookaheads, entry.Lookaheads)
	e.FollowingReservedWordSet = max(e.FollowingReservedWordSet, entry.FollowingReservedWordSet)
}
