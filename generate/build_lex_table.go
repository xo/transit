package generate

import (
	"encoding/binary"
	"slices"
)

// This file ports crates/generate/src/build_tables/build_lex_table.rs: the lex
// tables, which the lexer of a parser runs. Upstream writes a debug log line
// for each new entry state of a lex table. The port has no log, so it does
// not take the StrPool that upstream takes only for that line.

// LargeCharacterRangeCount is the number of ranges above which a set of
// characters is a large character set, which the parser holds in a table of
// its own.
//
// LargeCharacterRangeCount is LARGE_CHARACTER_RANGE_COUNT.
const LargeCharacterRangeCount = 8

// LargeCharacterSet is a large character set, and the token that it belongs
// to. HasSymbol is false for the None of upstream, which is a set that is not
// in the main token.
//
// LargeCharacterSet is (Option<Symbol>, CharacterSet).
type LargeCharacterSet struct {
	Symbol    Symbol
	HasSymbol bool
	Chars     CharacterSet
}

// LexTables holds the lex tables of a grammar and its large character sets.
//
// LexTables is LexTables.
type LexTables struct {
	MainLexTable       LexTable
	KeywordLexTable    LexTable
	LargeCharacterSets []LargeCharacterSet
}

// BuildLexTable builds the main lex table and the keyword lex table, and sets
// the lex state of each state of the parse table.
//
// BuildLexTable is build_lex_table.
func BuildLexTable(
	parseTable *ParseTable[ActionListID],
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	keywords *TokenSet,
	coincidentTokenIndex *CoincidentTokenIndex,
	tokenConflictMap *TokenConflictMap,
) LexTables {
	var keywordLexTable LexTable
	if syntaxGrammar.HasWordToken {
		builder := newLexTableBuilder(lexicalGrammar)
		builder.addStateForTokens(keywords)
		keywordLexTable = builder.table
	}

	type tokenSetStates struct {
		tokens        TokenSet
		parseStateIDs []ParseStateID
	}
	var parseStateIDsByTokenSet []tokenSetStates
	for i := range parseTable.States {
		state := &parseTable.States[i]
		var tokens TokenSet
		add := func(token Symbol) {
			switch token.Kind() {
			case SymbolTerminal:
				if keywords.Contains(token) {
					if syntaxGrammar.HasWordToken {
						tokens.Insert(syntaxGrammar.WordToken)
					}
				} else {
					tokens.Insert(token)
				}
			case SymbolEnd:
				tokens.Insert(token)
			case SymbolExternal, SymbolEndOfNonTerminalExtra, SymbolNonTerminal:
			}
		}
		for token := range state.TerminalEntries.Keys() {
			add(token)
		}
		for token := range state.ReservedWords.All() {
			add(token)
		}

		didMerge := false
		for e := range parseStateIDsByTokenSet {
			entry := &parseStateIDsByTokenSet[e]
			if mergeTokenSet(&entry.tokens, &tokens, tokenConflictMap, coincidentTokenIndex) {
				didMerge = true
				entry.parseStateIDs = append(entry.parseStateIDs, uint32(i))
				break
			}
		}

		if !didMerge {
			parseStateIDsByTokenSet = append(parseStateIDsByTokenSet, tokenSetStates{tokens, []ParseStateID{uint32(i)}})
		}
	}

	builder := newLexTableBuilder(lexicalGrammar)
	for e := range parseStateIDsByTokenSet {
		entry := &parseStateIDsByTokenSet[e]
		lexStateID := builder.addStateForTokens(&entry.tokens)
		for _, id := range entry.parseStateIDs {
			parseTable.States[id].LexStateID = lexStateID
		}
	}

	mainLexTable := builder.table
	builder.table = LexTable{}
	minimizeLexTable(&mainLexTable, parseTable)
	lexSortStates(&mainLexTable, parseTable)

	var largeCharacterSets []LargeCharacterSet
	for variableIx := range lexicalGrammar.Variables {
		symbol := TerminalSymbol(variableIx)
		builder.reset()
		symbolSet := TokenSetFrom(slices.Values([]Symbol{symbol}))
		builder.addStateForTokens(&symbolSet)
		for s := range builder.table.States {
			state := &builder.table.States[s]
			var characters CharacterSet
			for _, advance := range state.AdvanceActions {
				chars := advance.Chars
				if advance.Action.InMainToken {
					characters = characters.Add(chars)
					continue
				}

				if chars.RangeCount() > LargeCharacterRangeCount &&
					!slices.ContainsFunc(largeCharacterSets, func(set LargeCharacterSet) bool { return set.Chars.Equal(chars) }) {
					largeCharacterSets = append(largeCharacterSets, LargeCharacterSet{Chars: chars.Clone()})
				}
			}

			if characters.RangeCount() > LargeCharacterRangeCount &&
				!slices.ContainsFunc(largeCharacterSets, func(set LargeCharacterSet) bool { return set.Chars.Equal(characters) }) {
				largeCharacterSets = append(largeCharacterSets, LargeCharacterSet{Symbol: symbol, HasSymbol: true, Chars: characters})
			}
		}
	}

	return LexTables{
		MainLexTable:       mainLexTable,
		KeywordLexTable:    keywordLexTable,
		LargeCharacterSets: largeCharacterSets,
	}
}

// lexQueueEntry is a state of the lex table that waits for its actions.
//
// lexQueueEntry is QueueEntry.
type lexQueueEntry struct {
	stateID   LexStateID
	nfaStates []uint32
	eofValid  bool
}

// lexTableBuilder builds a lex table from the NFA of a lexical grammar.
//
// lexTableBuilder is LexTableBuilder. The queue of upstream is a VecDeque,
// and the port uses a slice with the head at index 0. The map of upstream is
// an FxHashMap<(Vec<u32>, bool), LexStateId>, which it only looks up, so a Go
// map holds it, with the key from lexStateSetKey.
type lexTableBuilder struct {
	lexicalGrammar        *LexicalGrammar
	cursor                *NfaCursor
	table                 LexTable
	stateQueue            []lexQueueEntry
	stateIDsByNfaStateSet map[string]LexStateID
}

// newLexTableBuilder returns a builder with an empty table.
//
// newLexTableBuilder is LexTableBuilder::new.
func newLexTableBuilder(lexicalGrammar *LexicalGrammar) *lexTableBuilder {
	return &lexTableBuilder{
		lexicalGrammar:        lexicalGrammar,
		cursor:                NewNfaCursor(&lexicalGrammar.Nfa, nil),
		stateIDsByNfaStateSet: make(map[string]LexStateID),
	}
}

// reset empties the table, the queue and the map of the builder.
//
// reset is LexTableBuilder::reset.
func (b *lexTableBuilder) reset() {
	b.table = LexTable{}
	b.stateQueue = b.stateQueue[:0]
	clear(b.stateIDsByNfaStateSet)
}

// addStateForTokens adds the entry state for a set of tokens, and every state
// that it reaches, and returns the id of the entry state.
//
// addStateForTokens is LexTableBuilder::add_state_for_tokens.
func (b *lexTableBuilder) addStateForTokens(tokens *TokenSet) LexStateID {
	eofValid := false
	var nfaStates []uint32
	for token := range tokens.All() {
		switch token.Kind() {
		case SymbolTerminal:
			nfaStates = append(nfaStates, b.lexicalGrammar.Variables[token.index].StartState)
		case SymbolEnd:
			eofValid = true
		case SymbolExternal, SymbolEndOfNonTerminalExtra, SymbolNonTerminal:
			panic("generate: a lex table cannot hold an external or a non-terminal token")
		}
	}
	stateID := b.addState(nfaStates, eofValid)

	for len(b.stateQueue) > 0 {
		entry := b.stateQueue[0]
		b.stateQueue = b.stateQueue[1:]
		b.populateState(entry.stateID, entry.nfaStates, entry.eofValid)
	}
	return stateID
}

// addState returns the id of the state for a set of NFA states. It adds a
// new state to the queue. It takes nfaStates, and the caller must not use it
// after.
//
// addState is LexTableBuilder::add_state. Upstream also returns whether the
// state is new, only for its debug log, so the port does not.
func (b *lexTableBuilder) addState(nfaStates []uint32, eofValid bool) LexStateID {
	b.cursor.Reset(nfaStates)
	key := lexStateSetKey(b.cursor.stateIDs, eofValid)
	if id, ok := b.stateIDsByNfaStateSet[key]; ok {
		return id
	}
	stateID := uint32(len(b.table.States))
	b.table.States = append(b.table.States, LexState{})
	b.stateQueue = append(b.stateQueue, lexQueueEntry{
		stateID:   stateID,
		nfaStates: slices.Clone(b.cursor.stateIDs),
		eofValid:  eofValid,
	})
	b.stateIDsByNfaStateSet[key] = stateID
	return stateID
}

// lexStateSetKey returns a string that is the same for two keys (states,
// eofValid) exactly when they are equal, for a map key.
func lexStateSetKey(states []uint32, eofValid bool) string {
	buf := make([]byte, 0, 4*len(states)+1)
	for _, s := range states {
		buf = binary.LittleEndian.AppendUint32(buf, s)
	}
	buf = append(buf, byte(boolWord(eofValid)))
	return string(buf)
}

// populateState sets the actions of a state of the lex table. It takes
// nfaStates, and the caller must not use it after.
//
// populateState is LexTableBuilder::populate_state.
func (b *lexTableBuilder) populateState(stateID LexStateID, nfaStates []uint32, eofValid bool) {
	b.cursor.ForceReset(nfaStates)

	// The EOF state is represented as an empty list of NFA states.
	completionID, completionPrecedence, hasCompletion := 0, int32(0), false
	for id, prec := range b.cursor.Completions() {
		if hasCompletion && PreferToken(b.lexicalGrammar, completionPrecedence, completionID, prec, id) {
			continue
		}
		completionID, completionPrecedence, hasCompletion = id, prec, true
	}

	transitions, hasSep := b.cursor.TransitionsAndAnySep()

	// If EOF is a valid lookahead token, add a transition predicated on the
	// null character that leads to the empty set of NFA states.
	if eofValid {
		nextStateID := b.addState(nil, false)
		state := &b.table.States[stateID]
		state.EOFAction = AdvanceAction{State: nextStateID, InMainToken: true}
		state.HasEOFAction = true
	}

	for t := range transitions {
		transition := &transitions[t]
		if hasCompletion && !PreferTransition(b.lexicalGrammar, transition, completionID, completionPrecedence, hasSep) {
			continue
		}

		nextStateID := b.addState(transition.States, eofValid && transition.IsSeparator)
		state := &b.table.States[stateID]
		state.AdvanceActions = append(state.AdvanceActions, LexAdvance{
			Chars: transition.Characters,
			Action: AdvanceAction{
				State:       nextStateID,
				InMainToken: !transition.IsSeparator,
			},
		})
	}

	state := &b.table.States[stateID]
	if hasCompletion {
		state.AcceptAction = TerminalSymbol(completionID)
		state.HasAcceptAction = true
	} else if len(b.cursor.stateIDs) == 0 {
		state.AcceptAction = SymbolEndValue
		state.HasAcceptAction = true
	}
}

// checkTokenConflicts reports whether terminal i conflicts with a terminal of
// a set that does not hold it, so that the two cannot share a lex state.
//
// checkTokenConflicts is check_token_conflicts.
func checkTokenConflicts(
	i int,
	setWithoutTerminal *TokenSet,
	tokenConflictMap *TokenConflictMap,
	coincidentTokenIndex *CoincidentTokenIndex,
) bool {
	wpr := tokenConflictMap.rowWords
	rowStart := i * wpr
	setBits := setWithoutTerminal.TerminalWords()

	// Does terminal i conflict with or match-prefix any terminal in the set?
	conflictRow := tokenConflictMap.conflictOrPrefixBits[rowStart : rowStart+wpr]
	for k := range min(len(conflictRow), len(setBits)) {
		if conflictRow[k]&setBits[k] != 0 {
			return true
		}
	}

	// Does terminal i overlap (in either direction) with any non-coincident
	// terminal in the set?
	overlapRow := tokenConflictMap.overlapEitherBits[rowStart : rowStart+wpr]
	coincidentRow := coincidentTokenIndex.rowBits[rowStart : rowStart+wpr]
	for k := range min(len(overlapRow), len(setBits), len(coincidentRow)) {
		if overlapRow[k]&setBits[k]&^coincidentRow[k] != 0 {
			return true
		}
	}

	return false
}

// mergeTokenSet adds the tokens of other to tokens and returns true, unless a
// token of one set conflicts with the other set. Then it changes nothing and
// returns false.
//
// mergeTokenSet is merge_token_set.
func mergeTokenSet(
	tokens *TokenSet,
	other *TokenSet,
	tokenConflictMap *TokenConflictMap,
	coincidentTokenIndex *CoincidentTokenIndex,
) bool {
	for index := range tokens.Terminals() {
		if !other.ContainsTerminal(int(index)) &&
			checkTokenConflicts(int(index), other, tokenConflictMap, coincidentTokenIndex) {
			return false
		}
	}

	for index := range other.Terminals() {
		if !tokens.ContainsTerminal(int(index)) &&
			checkTokenConflicts(int(index), tokens, tokenConflictMap, coincidentTokenIndex) {
			return false
		}
	}

	tokens.InsertAll(other)
	return true
}

// minimizeLexTable merges the states of a lex table that behave the same,
// and updates the lex states of the parse table.
//
// minimizeLexTable is minimize_lex_table. Upstream groups the states in an
// FxHashMap and then sorts the groups, so the order of the map does not reach
// the result. Each group is a list of state ids in increasing order, and no
// id is in two groups, so the first ids of the groups differ. The sort is
// therefore a total order, and the port gets the same order from a Go map.
func minimizeLexTable(table *LexTable, parseTable *ParseTable[ActionListID]) {
	// Initially group the states by their accept action and their valid
	// lookahead characters.
	stateIDsBySignature := make(map[string][]uint32)
	for i := range table.States {
		key := lexStateSignature(i == 0, &table.States[i])
		stateIDsBySignature[key] = append(stateIDsBySignature[key], uint32(i))
	}
	stateIDsByGroupID := make([][]uint32, 0, len(stateIDsBySignature))
	for _, ids := range stateIDsBySignature {
		stateIDsByGroupID = append(stateIDsByGroupID, ids)
	}
	slices.SortFunc(stateIDsByGroupID, slices.Compare)
	errorGroupIndex := slices.IndexFunc(stateIDsByGroupID, func(g []uint32) bool { return slices.Contains(g, 0) })
	if errorGroupIndex < 0 {
		panic("generate: the lex table has no state 0")
	}
	stateIDsByGroupID[errorGroupIndex], stateIDsByGroupID[0] = stateIDsByGroupID[0], stateIDsByGroupID[errorGroupIndex]

	groupIDsByStateID := make([]uint32, len(table.States))
	for groupID, stateIDs := range stateIDsByGroupID {
		for _, stateID := range stateIDs {
			groupIDsByStateID[stateID] = uint32(groupID)
		}
	}

	for SplitStateIDGroups(table.States, &stateIDsByGroupID, groupIDsByStateID, 1, lexStatesDiffer) {
	}

	newStates := make([]LexState, 0, len(stateIDsByGroupID))
	for _, stateIDs := range stateIDsByGroupID {
		newState := table.States[stateIDs[0]]
		table.States[stateIDs[0]] = LexState{}

		for a := range newState.AdvanceActions {
			action := &newState.AdvanceActions[a].Action
			action.State = groupIDsByStateID[action.State]
		}
		if newState.HasEOFAction {
			newState.EOFAction.State = groupIDsByStateID[newState.EOFAction.State]
		}
		newStates = append(newStates, newState)
	}

	for i := range parseTable.States {
		state := &parseTable.States[i]
		state.LexStateID = groupIDsByStateID[state.LexStateID]
	}

	table.States = newStates
}

// lexStateSignature returns a string that is the same for two states exactly
// when their signatures are equal, for a map key. The signature is whether
// the state is state 0, the accept action, whether the state has an action
// at the end of the input, and the characters of each advance action with
// whether the action is in the main token.
//
// lexStateSignature is the tuple that minimize_lex_table groups the states
// by.
func lexStateSignature(isFirst bool, state *LexState) string {
	buf := []byte{byte(boolWord(isFirst)), byte(boolWord(state.HasAcceptAction))}
	if state.HasAcceptAction {
		buf = binary.LittleEndian.AppendUint64(buf, state.AcceptAction.packedKey())
	}
	buf = append(buf, byte(boolWord(state.HasEOFAction)))
	for _, advance := range state.AdvanceActions {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(advance.Chars.ranges)))
		for _, r := range advance.Chars.ranges {
			buf = binary.LittleEndian.AppendUint32(buf, r.start)
			buf = binary.LittleEndian.AppendUint32(buf, r.end)
		}
		buf = append(buf, byte(boolWord(advance.Action.InMainToken)))
	}
	return string(buf)
}

// lexStatesDiffer reports whether two states of a group move to different
// groups on the same advance action.
//
// lexStatesDiffer is lex_states_differ.
func lexStatesDiffer(left, right *LexState, groupIDsByStateID []LexStateID) bool {
	for k := range min(len(left.AdvanceActions), len(right.AdvanceActions)) {
		if groupIDsByStateID[left.AdvanceActions[k].Action.State] !=
			groupIDsByStateID[right.AdvanceActions[k].Action.State] {
			return true
		}
	}
	return false
}

// lexSortStates sorts the states of a lex table after state 0, and updates
// the references to them.
//
// lexSortStates is sort_states. Upstream sorts with a stable sort, as the
// port does.
func lexSortStates(table *LexTable, parseTable *ParseTable[ActionListID]) {
	// Get a mapping of old state index -> new_state_index
	oldIDsByNewID := make([]int, len(table.States))
	for i := range oldIDsByNewID {
		oldIDsByNewID[i] = i
	}
	if len(oldIDsByNewID) > 1 {
		slices.SortStableFunc(oldIDsByNewID[1:], func(a, b int) int {
			return CompareLexState(&table.States[a], &table.States[b])
		})
	}

	// Get the inverse mapping
	newIDsByOldID := make([]uint32, len(oldIDsByNewID))
	for id, oldID := range oldIDsByNewID {
		newIDsByOldID[oldID] = uint32(id)
	}

	// Reorder the parse states and update their references to reflect the
	// new ordering.
	newStates := make([]LexState, 0, len(oldIDsByNewID))
	for _, oldID := range oldIDsByNewID {
		state := table.States[oldID]
		table.States[oldID] = LexState{}
		for a := range state.AdvanceActions {
			action := &state.AdvanceActions[a].Action
			action.State = newIDsByOldID[action.State]
		}
		if state.HasEOFAction {
			state.EOFAction.State = newIDsByOldID[state.EOFAction.State]
		}
		newStates = append(newStates, state)
	}
	table.States = newStates

	// Update the parse table's lex state references
	for i := range parseTable.States {
		state := &parseTable.States[i]
		state.LexStateID = newIDsByOldID[state.LexStateID]
	}
}
