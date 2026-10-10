package generate

import (
	"cmp"
	"math"
	"math/bits"
	"slices"
)

// This file ports crates/generate/src/build_tables/minimize_parse_table.rs:
// the steps that make the parse table smaller after it is built. They merge
// the states that are compatible, skip the states that only reduce a unit
// rule, remove the states that nothing uses, and order the states by size.
//
// Upstream writes log lines with debug! when it splits two states, and
// Minimizer::symbol_name and SymbolKey::symbol exist only for those lines.
// The generator has no logger yet, so the port leaves out the log lines and
// the two functions, as build_tables.go does. The functions that take the
// ids of the states only for the log lines do not take them in Go.
//
// The last part of the file ports the unstable sort of the Rust standard
// library, for the one sort of this file where two elements that compare
// equal can reach the output.

// minimizeSymbolKey is a Symbol packed into a uint64 for a fast comparison of
// the keys of a sort.
//
// Layout: the high 32 bits are the kind, and the low 32 bits are the index.
// This keeps the order of CompareSymbol (kind first, then index) as a single
// integer comparison, and halves each entry's size vs storing a full
// (Symbol, _) tuple.
//
// minimizeSymbolKey is SymbolKey.
type minimizeSymbolKey uint64

// The parts of a minimizeSymbolKey.
const (
	// minimizeKeyTagShift is KEY_TAG_SHIFT.
	minimizeKeyTagShift = 32
	// minimizeKeyIndexMask is KEY_INDEX_MASK.
	minimizeKeyIndexMask uint64 = math.MaxUint32
)

// newMinimizeSymbolKey returns the key of a symbol.
//
// newMinimizeSymbolKey is SymbolKey::new.
func newMinimizeSymbolKey(sym Symbol) minimizeSymbolKey {
	return minimizeSymbolKey(sym.packedKey())
}

// index returns the index of the symbol.
//
// index is SymbolKey::index.
func (k minimizeSymbolKey) index() uint32 {
	return uint32(uint64(k) & minimizeKeyIndexMask)
}

// tag returns the kind of the symbol.
//
// tag is SymbolKey::tag.
func (k minimizeSymbolKey) tag() uint64 {
	return uint64(k) >> minimizeKeyTagShift
}

// isTerminal reports whether the symbol is a terminal.
//
// isTerminal is SymbolKey::is_terminal.
func (k minimizeSymbolKey) isTerminal() bool {
	return k.tag() == uint64(SymbolTerminal)
}

// MinimizeParseTable makes the parse table smaller. When optimizations holds
// OptLevelMergeStates, it merges the states that are compatible. Then it
// skips the states that only reduce a unit rule, removes the states that no
// state refers to, and orders the states by descending size. tokenConflictMap
// and keywords are used only to merge states.
//
// MinimizeParseTable is minimize_parse_table.
func MinimizeParseTable(
	parseTable *ParseTable[ActionListID],
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	simpleAliases AliasMap,
	tokenConflictMap *TokenConflictMap,
	keywords *TokenSet,
	strPool *StrPool,
	optimizations OptLevel,
) {
	m := &minimizer{
		parseTable:       parseTable,
		syntaxGrammar:    syntaxGrammar,
		lexicalGrammar:   lexicalGrammar,
		tokenConflictMap: tokenConflictMap,
		keywords:         keywords,
		simpleAliases:    simpleAliases,
		strPool:          strPool,
	}
	if optimizations.Contains(OptLevelMergeStates) {
		m.mergeCompatibleStates()
	}
	m.removeUnitReductions()
	m.removeUnusedStates()
	m.reorderStatesByDescendingSize()
}

// minimizeConflictBits holds bit sets, one bit for each terminal, that
// tokenConflicts reads. stateTerminals and conflictRows are flat tables:
// row i spans [i*rowWords, (i+1)*rowWords).
//
// minimizeConflictBits is ConflictBits. hasWordToken is false for the None
// of wordToken.
type minimizeConflictBits struct {
	rowWords int
	// stateTerminals holds, for each state, the terminals that have entries.
	stateTerminals []uint64
	// conflictRows holds, for each token, the terminals that it lexically
	// conflicts with.
	conflictRows []uint64
	// keywords is the keyword set.
	keywords []uint64
	// internalExternal holds the tokens that are also external tokens.
	internalExternal []uint64
	// wordToken is the grammar's word token, packed with the same ordering key
	// as entries.
	wordToken    minimizeSymbolKey
	hasWordToken bool
}

// getConflictRow returns the terminals that a token conflicts with.
//
// getConflictRow is ConflictBits::get_conflict_row.
func (c *minimizeConflictBits) getConflictRow(token int) []uint64 {
	base := token * c.rowWords
	return c.conflictRows[base : base+c.rowWords]
}

// getStateRow returns the terminals that have entries in a state.
//
// getStateRow is ConflictBits::get_state_row.
func (c *minimizeConflictBits) getStateRow(state int) []uint64 {
	base := state * c.rowWords
	return c.stateTerminals[base : base+c.rowWords]
}

// minimizer holds the parse table and what the steps of MinimizeParseTable
// read.
//
// minimizer is Minimizer. strPool is used only by the log lines of
// upstream.
type minimizer struct {
	parseTable       *ParseTable[ActionListID]
	syntaxGrammar    *SyntaxGrammar
	lexicalGrammar   *LexicalGrammar
	tokenConflictMap *TokenConflictMap
	keywords         *TokenSet
	simpleAliases    AliasMap
	strPool          *StrPool
}

// minimizeEntry is the key of the symbol of a terminal entry, and its action
// list.
//
// minimizeEntry is (SymbolKey, ActionListId).
type minimizeEntry struct {
	key minimizeSymbolKey
	id  ActionListID
}

// minimizeShift is the key of the symbol of a shift, and its target state.
//
// minimizeShift is (SymbolKey, ParseStateId).
type minimizeShift struct {
	key   minimizeSymbolKey
	state ParseStateID
}

// minimizeGoto is the index of the non-terminal of a non-terminal entry,
// and its action.
//
// minimizeGoto is (NonterminalIndex, GotoAction). The index is an index
// into SyntaxGrammar.Variables. All non-terminal symbols have the same kind,
// so the index alone orders them.
type minimizeGoto struct {
	index  uint32
	action GotoAction
}

// removeUnitReductions finds each state whose only actions reduce the same
// hidden unit rule, and makes the shifts and the gotos to such a state go to
// where the reduction goes.
//
// removeUnitReductions is Minimizer::remove_unit_reductions. Upstream only
// looks up the FxHashSet aliasedSymbols and the FxHashMaps
// unitReductionSymbolsByState and actionListIDs.
func (m *minimizer) removeUnitReductions() {
	aliasedSymbols := make(map[Symbol]bool)
	for i := range m.syntaxGrammar.Variables {
		start, end := m.syntaxGrammar.VariableProdIDs(i)
		for prodID := start; prodID < end; prodID++ {
			for _, step := range m.syntaxGrammar.Production(prodID).Steps {
				if _, ok := step.GetAlias(); ok {
					aliasedSymbols[step.Symbol()] = true
				}
			}
		}
	}

	isUnitReductionSymbol := func(symbol Symbol) bool {
		if _, ok := m.simpleAliases[symbol]; ok {
			return false
		}
		if slices.Contains(m.syntaxGrammar.SupertypeSymbols, symbol) ||
			slices.Contains(m.syntaxGrammar.ExtraSymbols, symbol) ||
			aliasedSymbols[symbol] {
			return false
		}
		index, ok := symbol.NonTerminalIndex()
		return ok && m.syntaxGrammar.Variables[index].Kind != VariableNamed
	}

	unitReductionSymbolsByState := make(map[ParseStateID]Symbol)
	for i := range m.parseTable.States {
		state := &m.parseTable.States[i]
		if state.HasEOFGatedReduce {
			continue
		}
		onlyUnitReductions := true
		var unitReductionSymbol Symbol
		hasUnitReductionSymbol := false
		for id := range state.TerminalEntries.Values() {
			for _, action := range m.parseTable.ActionLists.Get(id) {
				if action.Kind == ParseActionShiftExtra {
					continue
				}
				if action.Kind == ParseActionReduce &&
					action.ChildCount == 1 &&
					action.ProductionID == 0 &&
					isUnitReductionSymbol(action.Symbol) &&
					(!hasUnitReductionSymbol || unitReductionSymbol == action.Symbol) {
					unitReductionSymbol, hasUnitReductionSymbol = action.Symbol, true
					continue
				}
				onlyUnitReductions = false
				break
			}

			if !onlyUnitReductions {
				break
			}
		}

		if hasUnitReductionSymbol && onlyUnitReductions {
			unitReductionSymbolsByState[ParseStateID(i)] = unitReductionSymbol
		}
	}

	if len(unitReductionSymbolsByState) == 0 {
		return
	}

	actionListIDs := make(map[string]uint32)
	actionLists := &m.parseTable.ActionLists
	for stateIndex := range m.parseTable.States {
		done := false
		for !done {
			done = true
			state := &m.parseTable.States[stateIndex]

			state.UpdateNonterminalReferences(func(otherStateID ParseStateID, state *ParseState[ActionListID]) ParseStateID {
				symbol, ok := unitReductionSymbolsByState[otherStateID]
				if !ok {
					return otherStateID
				}
				done = false
				if action, ok := state.NonterminalEntries.Get(symbol); ok && action.Kind == GotoActionGoto {
					return action.State
				}
				return otherStateID
			})

			for i := range state.TerminalEntries.Len() {
				_, oldID, _ := state.TerminalEntries.GetIndex(i)
				actions := ActionListFromSlice(actionLists.Get(oldID))
				changed := false
				for j := range actions {
					action := &actions[j]
					// A Shift onto a unit-reduction state (one whose only action reduces a
					// single `symbol`) can skip it. Shift then reduce then goto is equivalent
					// to shifting straight to the goto target for `symbol` in this case.
					if action.Kind != ParseActionShift {
						continue
					}
					symbol, ok := unitReductionSymbolsByState[action.State]
					if !ok {
						continue
					}
					if newTarget, ok := state.NonterminalEntries.Get(symbol); ok &&
						newTarget.Kind == GotoActionGoto &&
						newTarget.State != action.State {
						action.State = newTarget.State
						changed = true
						done = false
					}
				}
				if changed {
					index := actionLists.Intern(actionListIDs, actions)
					_, id, _ := state.TerminalEntries.GetIndexMut(i)
					*id = NewActionListID(index, oldID.Reusable())
				}
			}
		}
	}
}

// mergeCompatibleStates groups the states by their core, splits each group
// until the states of a group are compatible, and makes one state of each
// group.
//
// mergeCompatibleStates is Minimizer::merge_compatible_states.
func (m *minimizer) mergeCompatibleStates() {
	states := m.parseTable.States
	if len(states) == 0 {
		// Upstream unwraps the maximum of the core ids here.
		panic("generate: the parse table has no states")
	}
	var maxCoreID uint32
	for i := range states {
		maxCoreID = max(maxCoreID, states[i].CoreID)
	}
	coreCount := 1 + maxCoreID

	// Initially group the states by their parse item set core.
	groupIDsByStateID := make([]ParseStateID, 0, len(states))
	// Pre-allocate for the maximum possible number of groups (one per state) to
	// avoid reallocs as split_state_id_groups pushes new groups.
	stateIDsByGroupID := make([][]ParseStateID, coreCount, max(len(states), int(coreCount)))
	for i := range states {
		coreID := states[i].CoreID
		stateIDsByGroupID[coreID] = append(stateIDsByGroupID[coreID], ParseStateID(i))
		groupIDsByStateID = append(groupIDsByStateID, coreID)
	}

	// Precompute sorted terminal entry references for merge-join in states_conflict.
	// entry_maps[state_id][i] = (symbol_key, action_list_id). Keys are packed u64s
	// (symbol_key) for easy comparison. Upstream sorts with sort_unstable_by_key.
	// The keys of the entries of a state are different, so the order is the
	// same in Go.
	entryMaps := make([][]minimizeEntry, len(states))
	for s := range states {
		entries := make([]minimizeEntry, 0, states[s].TerminalEntries.Len())
		for sym, id := range states[s].TerminalEntries.All() {
			entries = append(entries, minimizeEntry{key: newMinimizeSymbolKey(sym), id: id})
		}
		slices.SortFunc(entries, func(a, b minimizeEntry) int { return cmp.Compare(a.key, b.key) })
		entryMaps[s] = entries
	}

	// Precompute word-aligned bitsets so `token_conflicts` can test a candidate
	// token against a whole state's terminals.
	//   - per state: which terminal indices have entries
	//   - per token: which terminal indices it lexically conflicts with
	//   - the keyword set as bits
	nTerminals := len(m.lexicalGrammar.Variables)
	rowWords := (nTerminals + 63) / 64
	set := func(bits []uint64, index int) { bits[index/64] |= 1 << (index % 64) }

	stateTerminals := make([]uint64, len(states)*rowWords)
	for s := range states {
		base := s * rowWords
		row := stateTerminals[base : base+rowWords]
		for symbol := range states[s].TerminalEntries.Keys() {
			if index, ok := symbol.TerminalIndex(); ok {
				set(row, int(index))
			}
		}
	}

	conflictRows := make([]uint64, nTerminals*rowWords)
	for i := range nTerminals {
		base := i * rowWords
		row := conflictRows[base : base+rowWords]
		for j := range nTerminals {
			if m.tokenConflictMap.DoesConflict(i, j) {
				set(row, j)
			}
		}
	}

	keywords := make([]uint64, rowWords)
	for symbol := range m.keywords.All() {
		if index, ok := symbol.TerminalIndex(); ok {
			set(keywords, int(index))
		}
	}

	internalExternal := make([]uint64, rowWords)
	for _, external := range m.syntaxGrammar.ExternalTokens {
		if !external.HasCorrespondingInternalToken {
			continue
		}
		if index, ok := external.CorrespondingInternalToken.TerminalIndex(); ok {
			set(internalExternal, int(index))
		}
	}

	conflictBits := &minimizeConflictBits{
		rowWords:         rowWords,
		stateTerminals:   stateTerminals,
		conflictRows:     conflictRows,
		keywords:         keywords,
		internalExternal: internalExternal,
		hasWordToken:     m.syntaxGrammar.HasWordToken,
	}
	if m.syntaxGrammar.HasWordToken {
		conflictBits.wordToken = newMinimizeSymbolKey(m.syntaxGrammar.WordToken)
	}

	SplitStateIDGroups(
		states,
		&stateIDsByGroupID,
		groupIDsByStateID,
		0,
		func(left, right *ParseState[ActionListID], groups []ParseStateID) bool {
			return m.statesConflict(left, right, groups, entryMaps, conflictBits)
		},
	)

	// Precompute per-state sorted shift actions and nonterminal goto actions.
	// State actions are stable across loop iterations; only group assignments change.
	// Keys are packed u64s (symbol_key) for single-instruction comparison.
	// Upstream sorts with sort_unstable_by_key. The keys of the entries of a
	// state are different, so the order is the same in Go.
	shiftMaps := make([][]minimizeShift, len(states))
	for s := range states {
		var shifts []minimizeShift
		for sym, id := range states[s].TerminalEntries.All() {
			actions := m.parseTable.ActionLists.Get(id)
			if len(actions) == 0 {
				continue
			}
			if action := actions[len(actions)-1]; action.Kind == ParseActionShift {
				shifts = append(shifts, minimizeShift{key: newMinimizeSymbolKey(sym), state: action.State})
			}
		}
		slices.SortFunc(shifts, func(a, b minimizeShift) int { return cmp.Compare(a.key, b.key) })
		shiftMaps[s] = shifts
	}

	// Store only the symbol index: all nonterminal entries share the same kind,
	// so index alone is sufficient for sorting and comparison.
	nonterminalMaps := make([][]minimizeGoto, len(states))
	for s := range states {
		entries := make([]minimizeGoto, 0, states[s].NonterminalEntries.Len())
		for sym, action := range states[s].NonterminalEntries.All() {
			index, ok := sym.NonTerminalIndex()
			if !ok {
				panic("generate: a non-terminal entry has a symbol that is not a non-terminal")
			}
			entries = append(entries, minimizeGoto{index: uint32(index), action: action})
		}
		slices.SortFunc(entries, func(a, b minimizeGoto) int { return cmp.Compare(a.index, b.index) })
		nonterminalMaps[s] = entries
	}

	for SplitStateIDGroups(
		states,
		&stateIDsByGroupID,
		groupIDsByStateID,
		0,
		func(left, right *ParseState[ActionListID], groups []ParseStateID) bool {
			return m.stateSuccessorsDiffer(left, right, groups, shiftMaps, nonterminalMaps)
		},
	) {
	}

	errorGroupIndex := slices.IndexFunc(stateIDsByGroupID, func(g []ParseStateID) bool { return slices.Contains(g, 0) })
	startGroupIndex := slices.IndexFunc(stateIDsByGroupID, func(g []ParseStateID) bool { return slices.Contains(g, 1) })
	if errorGroupIndex < 0 || startGroupIndex < 0 {
		panic("generate: no group holds the error state or the start state")
	}
	stateIDsByGroupID[errorGroupIndex], stateIDsByGroupID[0] = stateIDsByGroupID[0], stateIDsByGroupID[errorGroupIndex]
	stateIDsByGroupID[startGroupIndex], stateIDsByGroupID[1] = stateIDsByGroupID[1], stateIDsByGroupID[startGroupIndex]

	// Create a list of new parse states: one state for each group of old states.
	newStates := make([]ParseState[ActionListID], 0, len(stateIDsByGroupID))
	for _, stateIDs := range stateIDsByGroupID {
		// Initialize the new state based on the first old state in the group.
		parseState := states[stateIDs[0]]
		states[stateIDs[0]] = ParseState[ActionListID]{}

		// Extend the new state with all of the actions from the other old states
		// in the group.
		for _, stateID := range stateIDs[1:] {
			otherParseState := states[stateID]
			states[stateID] = ParseState[ActionListID]{}

			parseState.HasEOFGatedReduce = parseState.HasEOFGatedReduce || otherParseState.HasEOFGatedReduce
			parseState.TerminalEntries.Extend(otherParseState.TerminalEntries.All())
			parseState.NonterminalEntries.Extend(otherParseState.NonterminalEntries.All())
			parseState.ReservedWords.InsertAll(&otherParseState.ReservedWords)
			for symbol := range parseState.TerminalEntries.Keys() {
				parseState.ReservedWords.Remove(symbol)
			}
		}

		// Update the new state's outgoing references using the new grouping.
		parseState.UpdateNonterminalReferences(func(stateID ParseStateID, _ *ParseState[ActionListID]) ParseStateID {
			return groupIDsByStateID[stateID]
		})
		newStates = append(newStates, parseState)
	}

	m.parseTable.States = newStates
	RemapTerminalReferences(m.parseTable, func(stateID ParseStateID) ParseStateID {
		return groupIDsByStateID[stateID]
	})
}

// statesConflict reports whether two states cannot be merged: an entry that
// both have differs, or a token that only one has conflicts with the tokens
// of the other.
//
// statesConflict is Minimizer::states_conflict.
func (m *minimizer) statesConflict(
	state1, state2 *ParseState[ActionListID],
	groupIDsByStateID []ParseStateID,
	entryMaps [][]minimizeEntry,
	conflictBits *minimizeConflictBits,
) bool {
	entries1 := entryMaps[state1.ID]
	entries2 := entryMaps[state2.ID]
	len1 := len(entries1)
	len2 := len(entries2)
	i := 0
	j := 0
	for i < len1 || j < len2 {
		var ord int
		switch {
		case i < len1 && j < len2:
			ord = cmp.Compare(entries1[i].key, entries2[j].key)
		case i < len1:
			ord = -1
		default:
			ord = 1
		}
		switch ord {
		case 0:
			if m.entriesConflict(entries1[i].id, entries2[j].id, groupIDsByStateID) {
				return true
			}
			i++
			j++
		case -1:
			if m.tokenConflicts(state2, conflictBits, entries1[i].key) {
				return true
			}
			i++
		case 1:
			if m.tokenConflicts(state1, conflictBits, entries2[j].key) {
				return true
			}
			j++
		}
	}
	return false
}

// stateSuccessorsDiffer reports whether two states shift or go to states of
// different groups for the same symbol.
//
// stateSuccessorsDiffer is Minimizer::state_successors_differ.
func (m *minimizer) stateSuccessorsDiffer(
	state1, state2 *ParseState[ActionListID],
	groupIDsByStateID []ParseStateID,
	shiftMaps [][]minimizeShift,
	nonterminalMaps [][]minimizeGoto,
) bool {
	shifts1 := shiftMaps[state1.ID]
	shifts2 := shiftMaps[state2.ID]
	i := 0
	j := 0
	for i < len(shifts1) && j < len(shifts2) {
		k1, s1 := shifts1[i].key, shifts1[i].state
		k2, s2 := shifts2[j].key, shifts2[j].state
		switch cmp.Compare(k1, k2) {
		case -1:
			i++
		case 1:
			j++
		case 0:
			if groupIDsByStateID[s1] != groupIDsByStateID[s2] {
				return true
			}
			i++
			j++
		}
	}

	nonterms1 := nonterminalMaps[state1.ID]
	nonterms2 := nonterminalMaps[state2.ID]
	i = 0
	j = 0
	for i < len(nonterms1) && j < len(nonterms2) {
		idx1, s1 := nonterms1[i].index, nonterms1[i].action
		idx2, s2 := nonterms2[j].index, nonterms2[j].action
		switch cmp.Compare(idx1, idx2) {
		case -1:
			i++
		case 1:
			j++
		case 0:
			switch {
			case s1.Kind == GotoActionShiftExtra && s2.Kind == GotoActionShiftExtra:
			case s1.Kind == GotoActionGoto && s2.Kind == GotoActionGoto:
				if groupIDsByStateID[s1.State] != groupIDsByStateID[s2.State] {
					return true
				}
			default:
				return true
			}
			i++
			j++
		}
	}

	return false
}

// entriesConflict reports whether two action lists of the same token cannot
// be merged. Two shifts are compatible when their targets are in the same
// group, and other actions must be equal.
//
// entriesConflict is Minimizer::entries_conflict.
func (m *minimizer) entriesConflict(id1, id2 ActionListID, groupIDsByStateID []ParseStateID) bool {
	// To be compatible, entries need to have the same actions.
	if id1.Index() == id2.Index() {
		return false
	}
	actions1 := m.parseTable.ActionLists.Get(id1)
	actions2 := m.parseTable.ActionLists.Get(id2)
	if len(actions1) != len(actions2) {
		return true
	}

	for i := range actions1 {
		action1, action2 := actions1[i], actions2[i]
		// Two shift actions are equivalent if their destinations are in the same group.
		if action1.Kind == ParseActionShift && action2.Kind == ParseActionShift {
			group1 := groupIDsByStateID[action1.State]
			group2 := groupIDsByStateID[action2.State]
			if group1 == group2 && action1.IsRepetition == action2.IsRepetition {
				continue
			}
			return true
		} else if action1 != action2 {
			return true
		}
	}

	return false
}

// tokenConflicts reports whether a state cannot take a new token: the token
// ends a non-terminal extra, is external, is both internal and external, or
// conflicts with a token of the state.
//
// tokenConflicts is Minimizer::token_conflicts.
func (m *minimizer) tokenConflicts(rightState *ParseState[ActionListID], conflictBits *minimizeConflictBits, newToken minimizeSymbolKey) bool {
	newTokenIsTerminal := newToken.isTerminal()
	var newTokenIndex int
	switch newToken.tag() {
	case uint64(SymbolEndOfNonTerminalExtra):
		return true
	// Do not add external tokens, as they could conflict lexically with
	// any of the state's existing lookahead tokens.
	case uint64(SymbolExternal):
		return true
	case uint64(SymbolEnd):
		newTokenIndex = 0
	case uint64(SymbolTerminal):
		newTokenIndex = int(newToken.index())
	default:
		panic("generate: a terminal entry has a non-terminal symbol")
	}

	var isReserved bool
	if newTokenIsTerminal {
		isReserved = rightState.ReservedWords.ContainsTerminal(newTokenIndex)
	} else {
		isReserved = rightState.ReservedWords.Contains(SymbolEndValue)
	}
	if isReserved {
		return false
	}

	// Do not add tokens which are both internal and external. Their validity could
	// influence the behavior of the external scanner. `bits.internal_external` is
	// indexed by terminal index only.
	if newTokenIsTerminal && conflictBits.internalExternal[newTokenIndex/64]&(1<<(newTokenIndex%64)) != 0 {
		return true
	}

	newTokenIsWord := conflictBits.hasWordToken && conflictBits.wordToken == newToken
	newTokenIsKeyword := false
	if conflictBits.hasWordToken {
		if newTokenIsTerminal {
			newTokenIsKeyword = conflictBits.keywords[newTokenIndex/64]&(1<<(newTokenIndex%64)) != 0
		} else {
			newTokenIsKeyword = m.keywords.Contains(SymbolEndValue)
		}
	}
	// Do not add a token if it conflicts with an existing token. Test the candidate's
	// conflict row against the state's terminal bits, masking out the word/keyword
	// exemptions.
	row := conflictBits.getConflictRow(newTokenIndex)
	rightTerminalBits := conflictBits.getStateRow(int(rightState.ID))
	for w, rowWord := range row {
		candidates := rightTerminalBits[w] & rowWord
		if newTokenIsKeyword && conflictBits.hasWordToken {
			word := conflictBits.wordToken
			if word.isTerminal() && int(word.index())/64 == w {
				candidates &^= 1 << (word.index() % 64)
			}
		}
		if newTokenIsWord {
			candidates &^= conflictBits.keywords[w]
		}
		if candidates != 0 {
			return true
		}
	}

	return false
}

// removeUnusedStates removes each state that no state refers to, except the
// error state and the start state, and renumbers the references.
//
// removeUnusedStates is Minimizer::remove_unused_states.
func (m *minimizer) removeUnusedStates() {
	states := m.parseTable.States
	stateUsageMap := make([]bool, len(states))

	stateUsageMap[0] = true
	stateUsageMap[1] = true

	for i := range states {
		for referencedState := range ReferencedStates(&states[i], &m.parseTable.ActionLists) {
			stateUsageMap[referencedState] = true
		}
	}
	removedPredecessorCount := 0
	stateReplacementMap := make([]ParseStateID, len(states))
	for stateID := range states {
		stateReplacementMap[stateID] = ParseStateID(stateID - removedPredecessorCount)
		if !stateUsageMap[stateID] {
			removedPredecessorCount++
		}
	}
	stateID := 0
	originalStateID := 0
	for stateID < len(states) {
		if stateUsageMap[originalStateID] {
			states[stateID].UpdateNonterminalReferences(func(otherStateID ParseStateID, _ *ParseState[ActionListID]) ParseStateID {
				return stateReplacementMap[otherStateID]
			})
			stateID++
		} else {
			states = slices.Delete(states, stateID, stateID+1)
		}
		originalStateID++
	}
	m.parseTable.States = states
	RemapTerminalReferences(m.parseTable, func(stateID ParseStateID) ParseStateID {
		return stateReplacementMap[stateID]
	})
}

// reorderStatesByDescendingSize orders the states by descending number of
// entries, after the error state and the start state, and renumbers the
// references.
//
// reorderStatesByDescendingSize is
// Minimizer::reorder_states_by_descending_size. Upstream sorts with
// sort_unstable_by_key, and many states have the same number of entries. The
// order of such states reaches the parser, so the port sorts with
// minimizeSortUnstableByKey, a port of that sort.
func (m *minimizer) reorderStatesByDescendingSize() {
	states := m.parseTable.States
	// Get a mapping of old state index -> new_state_index
	oldIDsByNewID := make([]int, len(states))
	for i := range oldIDsByNewID {
		oldIDsByNewID[i] = i
	}
	minimizeSortUnstableByKey(oldIDsByNewID, func(i int) int64 {
		// Don't change states 0 (the error state) or 1 (the start state).
		if i <= 1 {
			return int64(i) - 1_000_000
		}

		// Reorder all the other states by descending symbol count.
		state := &states[i]
		return -int64(state.TerminalEntries.Len() + state.NonterminalEntries.Len())
	})

	// Get the inverse mapping
	newIDsByOldID := make([]ParseStateID, len(oldIDsByNewID))
	for id, oldID := range oldIDsByNewID {
		newIDsByOldID[oldID] = ParseStateID(id)
	}

	// Reorder the parse states and update their references to reflect
	// the new ordering.
	newStates := make([]ParseState[ActionListID], 0, len(oldIDsByNewID))
	for _, oldID := range oldIDsByNewID {
		state := states[oldID]
		states[oldID] = ParseState[ActionListID]{}
		state.UpdateNonterminalReferences(func(id ParseStateID, _ *ParseState[ActionListID]) ParseStateID {
			return newIDsByOldID[id]
		})
		newStates = append(newStates, state)
	}
	m.parseTable.States = newStates
	RemapTerminalReferences(m.parseTable, func(id ParseStateID) ParseStateID {
		return newIDsByOldID[id]
	})
}

// The rest of the file ports the unstable sort of the Rust standard library
// of rustc 1.97.1, in library/core/src/slice/sort, for a slice of usize on
// x86_64. That sort is ipnsort, and it does not keep the order of equal
// elements. The port takes the branches that the standard library takes for
// an element of 8 bytes that is Copy: the small sort is small_sort_network,
// and the partition is partition_lomuto_branchless_cyclic. The Rust code
// moves the elements with pointers and a gap, and the Go code moves the same
// elements with indices, in the same order.

// minimizeSortUnstableByKey sorts v by the key of each element, as
// slice::sort_unstable_by_key does.
//
// minimizeSortUnstableByKey is slice::sort_unstable_by_key.
func minimizeSortUnstableByKey(v []int, key func(int) int64) {
	minimizeSortUnstable(v, func(a, b int) bool { return key(a) < key(b) })
}

// minimizeSortUnstable sorts v with isLess.
//
// minimizeSortUnstable is sort::unstable::sort.
func minimizeSortUnstable(v []int, isLess func(a, b int) bool) {
	// Instrumenting the standard library showed that 90+% of the calls to sort
	// by rustc are either of size 0 or 1.
	n := len(v)
	if n < 2 {
		return
	}

	// More advanced sorting methods than insertion sort are faster if called in
	// a hot loop for small inputs, but for general-purpose code the small
	// binary size of insertion sort is more important.
	const maxLenAlwaysInsertionSort = 20
	if n <= maxLenAlwaysInsertionSort {
		minimizeInsertionSortShiftLeft(v, 1, isLess)
		return
	}

	minimizeIPNSort(v, isLess)
}

// minimizeIPNSort sorts v, which has more than 20 elements.
//
// minimizeIPNSort is sort::unstable::ipnsort.
func minimizeIPNSort(v []int, isLess func(a, b int) bool) {
	n := len(v)
	runLen, wasReversed := minimizeFindExistingRun(v, isLess)

	if runLen == n {
		if wasReversed {
			slices.Reverse(v)
		}
		return
	}

	// Limit the number of imbalanced partitions to `2 * floor(log2(len))`.
	// The binary OR by one is used to eliminate the zero-check in the logarithm.
	limit := uint32(2 * (bits.Len(uint(n|1)) - 1))
	minimizeQuicksort(v, 0, false, limit, isLess)
}

// minimizeFindExistingRun returns the length of the run of sorted elements
// at the start of v, and true when the run is strictly descending.
//
// minimizeFindExistingRun is sort::shared::find_existing_run.
func minimizeFindExistingRun(v []int, isLess func(a, b int) bool) (int, bool) {
	n := len(v)
	if n < 2 {
		return n, false
	}

	runLen := 2
	strictlyDescending := isLess(v[1], v[0])
	if strictlyDescending {
		for runLen < n && isLess(v[runLen], v[runLen-1]) {
			runLen++
		}
	} else {
		for runLen < n && !isLess(v[runLen], v[runLen-1]) {
			runLen++
		}
	}
	return runLen, strictlyDescending
}

// minimizeSmallSortThreshold is the length up to which the quicksort uses
// the small sort: SMALL_SORT_NETWORK_THRESHOLD, which the standard library
// picks for a Copy type of at most 8 bytes.
const minimizeSmallSortThreshold = 32

// minimizeQuicksort sorts v. hasAncestorPivot tells whether v has a
// predecessor in the original slice, ancestorPivot. limit is the number of
// imbalanced partitions that are allowed before the sort switches to
// heapsort.
//
// minimizeQuicksort is sort::unstable::quicksort::quicksort. Upstream holds
// the ancestor pivot by reference. The element does not move while the
// reference lives, so a copy of its value is the same.
func minimizeQuicksort(v []int, ancestorPivot int, hasAncestorPivot bool, limit uint32, isLess func(a, b int) bool) {
	for {
		if len(v) <= minimizeSmallSortThreshold {
			minimizeSmallSortNetwork(v, isLess)
			return
		}

		// If too many bad pivot choices were made, simply fall back to heapsort in order to
		// guarantee `O(N x log(N))` worst-case.
		if limit == 0 {
			minimizeHeapsort(v, isLess)
			return
		}

		limit--

		// Choose a pivot and try guessing whether the slice is already sorted.
		pivotPos := minimizeChoosePivot(v, isLess)

		// If the chosen pivot is equal to the predecessor, then it's the smallest element in the
		// slice. Partition the slice into elements equal to and elements greater than the pivot.
		// This case is usually hit when the slice contains many duplicate elements.
		if hasAncestorPivot && !isLess(ancestorPivot, v[pivotPos]) {
			numLt := minimizePartition(v, pivotPos, func(a, b int) bool { return !isLess(b, a) })

			// Continue sorting elements greater than the pivot. We know that `num_lt` contains
			// the pivot. So we can continue after `num_lt`.
			v = v[numLt+1:]
			hasAncestorPivot = false
			continue
		}

		// Partition the slice.
		numLt := minimizePartition(v, pivotPos, isLess)

		// Split the slice into `left`, `pivot`, and `right`.
		left, pivot, right := v[:numLt], v[numLt], v[numLt+1:]

		// Recurse into the left side. We have a fixed recursion limit, testing shows no real
		// benefit for recursing into the shorter side.
		minimizeQuicksort(left, ancestorPivot, hasAncestorPivot, limit, isLess)

		// Continue with the right side.
		v = right
		ancestorPivot, hasAncestorPivot = pivot, true
	}
}

// minimizePartition moves the elements of v that are less than v[pivot] to
// the left of it, and the other elements to the right of it. It returns the
// number of elements that are less.
//
// minimizePartition is sort::unstable::quicksort::partition.
func minimizePartition(v []int, pivot int, isLess func(a, b int) bool) int {
	if len(v) == 0 {
		return 0
	}

	// Place the pivot at the beginning of slice.
	v[0], v[pivot] = v[pivot], v[0]
	numLt := minimizePartitionLomutoBranchlessCyclic(v[1:], v[0], isLess)

	// Place the pivot between the two partitions.
	v[0], v[numLt] = v[numLt], v[0]
	return numLt
}

// minimizePartitionLomutoBranchlessCyclic partitions v around pivot, and
// returns the number of elements that are less than pivot.
//
// minimizePartitionLomutoBranchlessCyclic is
// sort::unstable::quicksort::partition_lomuto_branchless_cyclic. Upstream
// unrolls the loop twice, which does not change the order of the steps. The
// last step takes the value that the gap saved at the start.
func minimizePartitionLomutoBranchlessCyclic(v []int, pivot int, isLess func(a, b int) bool) int {
	n := len(v)
	if n == 0 {
		return 0
	}

	gapValue := v[0]
	gapPos := 0
	numLt := 0
	for right := 1; ; right++ {
		isDone := right == n
		var rightValue int
		if isDone {
			rightValue = gapValue
		} else {
			rightValue = v[right]
		}

		rightIsLt := isLess(rightValue, pivot)
		v[gapPos] = v[numLt]
		v[numLt] = rightValue
		gapPos = right
		if rightIsLt {
			numLt++
		}

		if isDone {
			break
		}
	}
	return numLt
}

// minimizePseudoMedianRecThreshold is the length from which the choice of
// the pivot takes a pseudomedian recursively.
//
// minimizePseudoMedianRecThreshold is PSEUDO_MEDIAN_REC_THRESHOLD.
const minimizePseudoMedianRecThreshold = 64

// minimizeChoosePivot returns the index of the pivot of v, which has at
// least 8 elements.
//
// minimizeChoosePivot is sort::shared::pivot::choose_pivot.
func minimizeChoosePivot(v []int, isLess func(a, b int) bool) int {
	n := len(v)
	if n < 8 {
		panic("generate: choose a pivot of fewer than 8 elements")
	}

	lenDiv8 := n / 8
	a := 0           // [0, floor(n/8))
	b := lenDiv8 * 4 // [4*floor(n/8), 5*floor(n/8))
	c := lenDiv8 * 7 // [7*floor(n/8), 8*floor(n/8))

	if n < minimizePseudoMedianRecThreshold {
		return minimizeMedian3(v, a, b, c, isLess)
	}
	return minimizeMedian3Rec(v, a, b, c, lenDiv8, isLess)
}

// minimizeMedian3Rec returns the index of an approximate median of 3
// elements from the sections of n elements at a, b and c, or recursively
// from an approximation of each, if they're large enough.
//
// minimizeMedian3Rec is sort::shared::pivot::median3_rec.
func minimizeMedian3Rec(v []int, a, b, c, n int, isLess func(a, b int) bool) int {
	if n*8 >= minimizePseudoMedianRecThreshold {
		n8 := n / 8
		a = minimizeMedian3Rec(v, a, a+n8*4, a+n8*7, n8, isLess)
		b = minimizeMedian3Rec(v, b, b+n8*4, b+n8*7, n8, isLess)
		c = minimizeMedian3Rec(v, c, c+n8*4, c+n8*7, n8, isLess)
	}
	return minimizeMedian3(v, a, b, c, isLess)
}

// minimizeMedian3 returns the index of the median of the elements at a, b
// and c.
//
// minimizeMedian3 is sort::shared::pivot::median3.
func minimizeMedian3(v []int, a, b, c int, isLess func(a, b int) bool) int {
	x := isLess(v[a], v[b])
	y := isLess(v[a], v[c])
	if x == y {
		// If x=y=0 then b, c <= a. In this case we want to return max(b, c).
		// If x=y=1 then a < b, c. In this case we want to return min(b, c).
		// By toggling the outcome of b < c using XOR x we get this behavior.
		if z := isLess(v[b], v[c]); z != x {
			return c
		}
		return b
	}
	// Either c <= a < b or b <= a < c, thus a is our median.
	return a
}

// minimizeHeapsort sorts v with heapsort.
//
// minimizeHeapsort is sort::unstable::heapsort::heapsort.
func minimizeHeapsort(v []int, isLess func(a, b int) bool) {
	n := len(v)
	for i := n + n/2 - 1; i >= 0; i-- {
		siftIdx := 0
		if i >= n {
			siftIdx = i - n
		} else {
			v[0], v[i] = v[i], v[0]
		}
		minimizeSiftDown(v[:min(i, n)], siftIdx, isLess)
	}
}

// minimizeSiftDown moves the element at node down the heap v. The heap
// keeps the invariant parent >= child.
//
// minimizeSiftDown is sort::unstable::heapsort::sift_down.
func minimizeSiftDown(v []int, node int, isLess func(a, b int) bool) {
	n := len(v)
	for {
		// Children of `node`.
		child := 2*node + 1
		if child >= n {
			break
		}

		// Choose the greater child.
		if child+1 < n && isLess(v[child], v[child+1]) {
			child++
		}

		// Stop if the invariant holds at `node`.
		if !isLess(v[node], v[child]) {
			break
		}

		v[node], v[child] = v[child], v[node]
		node = child
	}
}

// minimizeSmallSortNetwork sorts v, which has at most 32 elements, with
// sorting networks, insertion sort and a merge.
//
// minimizeSmallSortNetwork is sort::shared::smallsort::small_sort_network.
func minimizeSmallSortNetwork(v []int, isLess func(a, b int) bool) {
	n := len(v)
	if n < 2 {
		return
	}

	lenDiv2 := n / 2
	noMerge := n < 18

	initialRegionLen := lenDiv2
	if noMerge {
		initialRegionLen = n
	}
	region := v[:initialRegionLen]
	first := true
	for {
		presortedLen := 1
		if len(region) >= 13 {
			minimizeSortNetwork(region, minimizeSort13Optimal[:], isLess)
			presortedLen = 13
		} else if len(region) >= 9 {
			minimizeSortNetwork(region, minimizeSort9Optimal[:], isLess)
			presortedLen = 9
		}

		minimizeInsertionSortShiftLeft(region, presortedLen, isLess)

		if noMerge {
			return
		}

		if !first {
			break
		}
		first = false
		region = v[lenDiv2:]
	}

	scratch := make([]int, n)
	minimizeBidirectionalMerge(v, scratch, isLess)
	copy(v, scratch)
}

// minimizeSort9Optimal is the optimal sorting network of 9 elements, from
// https://bertdobbelaere.github.io/sorting_networks.html. Each pair is a
// call of swap_if_less.
//
// minimizeSort9Optimal is the network of sort::shared::smallsort::sort9_optimal.
var minimizeSort9Optimal = [...][2]int{
	{0, 3}, {1, 7}, {2, 5}, {4, 8}, {0, 7}, {2, 4}, {3, 8}, {5, 6}, {0, 2},
	{1, 3}, {4, 5}, {7, 8}, {1, 4}, {3, 6}, {5, 7}, {0, 1}, {2, 4}, {3, 5},
	{6, 8}, {2, 3}, {4, 5}, {6, 7}, {1, 2}, {3, 4}, {5, 6},
}

// minimizeSort13Optimal is the optimal sorting network of 13 elements, from
// https://bertdobbelaere.github.io/sorting_networks.html. Each pair is a
// call of swap_if_less.
//
// minimizeSort13Optimal is the network of
// sort::shared::smallsort::sort13_optimal.
var minimizeSort13Optimal = [...][2]int{
	{0, 12}, {1, 10}, {2, 9}, {3, 7}, {5, 11}, {6, 8}, {1, 6}, {2, 3}, {4, 11},
	{7, 9}, {8, 10}, {0, 4}, {1, 2}, {3, 6}, {7, 8}, {9, 10}, {11, 12}, {4, 6},
	{5, 9}, {8, 11}, {10, 12}, {0, 5}, {3, 8}, {4, 7}, {6, 11}, {9, 10}, {0, 1},
	{2, 5}, {6, 9}, {7, 8}, {10, 11}, {1, 3}, {2, 4}, {5, 6}, {9, 10}, {1, 2},
	{3, 4}, {5, 7}, {6, 8}, {2, 3}, {4, 5}, {6, 7}, {8, 9}, {3, 4}, {5, 6},
}

// minimizeSortNetwork applies a sorting network to the first elements of v.
// For each pair, it swaps the two elements when the second is less than the
// first. It does not swap equal elements.
//
// minimizeSortNetwork is sort9_optimal and sort13_optimal, with
// swap_if_less, of sort::shared::smallsort.
func minimizeSortNetwork(v []int, network [][2]int, isLess func(a, b int) bool) {
	for _, pair := range network {
		a, b := pair[0], pair[1]
		if isLess(v[b], v[a]) {
			v[a], v[b] = v[b], v[a]
		}
	}
}

// minimizeInsertTail sorts v[:tail+1], when v[:tail] is already sorted.
//
// minimizeInsertTail is sort::shared::smallsort::insert_tail.
func minimizeInsertTail(v []int, tail int, isLess func(a, b int) bool) {
	sift := tail - 1
	if !isLess(v[tail], v[sift]) {
		return
	}

	tmp := v[tail]
	gap := tail
	for {
		v[gap] = v[sift]
		gap = sift

		if sift == 0 {
			break
		}

		sift--
		if !isLess(tmp, v[sift]) {
			break
		}
	}
	v[gap] = tmp
}

// minimizeInsertionSortShiftLeft sorts v, when v[:offset] is already sorted.
//
// minimizeInsertionSortShiftLeft is
// sort::shared::smallsort::insertion_sort_shift_left.
func minimizeInsertionSortShiftLeft(v []int, offset int, isLess func(a, b int) bool) {
	if offset == 0 || offset > len(v) {
		panic("generate: the offset of an insertion sort is out of range")
	}
	for tail := offset; tail < len(v); tail++ {
		minimizeInsertTail(v, tail, isLess)
	}
}

// minimizeBidirectionalMerge merges the sorted halves v[:len(v)/2] and
// v[len(v)/2:] into dst. It merges from the front and from the back at the
// same time.
//
// minimizeBidirectionalMerge is sort::shared::smallsort::bidirectional_merge,
// with merge_up and merge_down.
func minimizeBidirectionalMerge(v, dst []int, isLess func(a, b int) bool) {
	n := len(v)
	lenDiv2 := n / 2

	left := 0
	right := lenDiv2
	out := 0

	leftRev := lenDiv2 - 1
	rightRev := n - 1
	outRev := n - 1

	for range lenDiv2 {
		// merge_up
		if !isLess(v[right], v[left]) {
			dst[out] = v[left]
			left++
		} else {
			dst[out] = v[right]
			right++
		}
		out++

		// merge_down
		if !isLess(v[rightRev], v[leftRev]) {
			dst[outRev] = v[rightRev]
			rightRev--
		} else {
			dst[outRev] = v[leftRev]
			leftRev--
		}
		outRev--
	}

	leftEnd := leftRev + 1
	rightEnd := rightRev + 1

	// Odd length, so one element is left unconsumed in the input.
	if n%2 != 0 {
		if left < leftEnd {
			dst[out] = v[left]
			left++
		} else {
			dst[out] = v[right]
			right++
		}
	}

	// We now should have consumed the full input exactly once. This can only fail if the
	// user-provided comparison function fails to implement a strict weak ordering.
	if left != leftEnd || right != rightEnd {
		panic("user-provided comparison function does not correctly implement a total order")
	}
}
