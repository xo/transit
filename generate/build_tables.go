package generate

import (
	"math"
	"slices"
	"unicode"
)

// This file ports crates/generate/src/build_tables.rs: the builder of the
// parse table and the lexer tables.
//
// Upstream writes log lines with debug! and info!, and report_state_info
// writes the parse states of one rule to the log, for the option
// --report-states-for-rule of the tool. The generator has no logger yet, so
// the port leaves out the log lines and report_state_info. The backlog has
// the item.

// Tables is the parse table and the lexer tables of a grammar.
//
// Tables is Tables.
type Tables struct {
	ParseTable         ParseTable[ActionListID]
	MainLexTable       LexTable
	KeywordLexTable    LexTable
	LargeCharacterSets []LargeCharacterSet
}

// symbolIndexer gives each of a grammar's symbols a position. Positions
// follow the order of CompareSymbol:
//   - external tokens
//   - SymbolEnd
//   - SymbolEndOfNonTerminalExtra
//   - terminals
//   - non-terminals
//
// symbolIndexer is SymbolIndexer.
type symbolIndexer struct {
	externalCount    uint32
	terminalCount    uint32
	nonTerminalCount uint32
}

// newSymbolIndexer returns the indexer of a grammar's symbols.
//
// newSymbolIndexer is SymbolIndexer::new.
func newSymbolIndexer(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar) symbolIndexer {
	return symbolIndexer{
		externalCount:    uint32(len(syntaxGrammar.ExternalTokens)),
		terminalCount:    uint32(len(lexicalGrammar.Variables)),
		nonTerminalCount: uint32(len(syntaxGrammar.Variables)),
	}
}

// index returns this symbol's position.
//
// index is SymbolIndexer::index.
func (s symbolIndexer) index(symbol Symbol) int {
	var position uint32
	switch symbol.kind {
	case SymbolExternal:
		position = symbol.index
	case SymbolEnd:
		position = s.externalCount
	case SymbolEndOfNonTerminalExtra:
		position = s.externalCount + 1
	case SymbolTerminal:
		position = s.externalCount + 2 + symbol.index
	case SymbolNonTerminal:
		position = s.tokenCount() + symbol.index
	}
	return int(position)
}

// tokenCount returns how many positions index gives to tokens only: one
// per external token, one each for SymbolEnd and
// SymbolEndOfNonTerminalExtra, and one per terminal.
//
// tokenCount is SymbolIndexer::token_count.
func (s symbolIndexer) tokenCount() uint32 {
	return s.externalCount + 2 + s.terminalCount
}

// symbolCount returns how many positions index gives out in total: the
// tokens, then one per non-terminal.
//
// symbolCount is SymbolIndexer::symbol_count.
func (s symbolIndexer) symbolCount() uint32 {
	return s.tokenCount() + s.nonTerminalCount
}

// BuildTables builds the parse table and the lexer tables of a prepared
// grammar.
//
// BuildTables is build_tables. The port leaves out its argument
// report_symbol_name, which only report_state_info reads.
func BuildTables(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, simpleAliases AliasMap, variableInfo []VariableInfo, inlines *InlinedProductionMap, strPool *StrPool, optimizations OptLevel, diagnostics *[]Diagnostic) (*Tables, error) {
	itemKeyMap := NewItemKeyMap(syntaxGrammar, strPool)
	itemSetBuilder := NewParseItemSetBuilder(syntaxGrammar, lexicalGrammar, inlines, itemKeyMap)
	followingTokens := getFollowingTokens(syntaxGrammar, lexicalGrammar, itemSetBuilder)
	parseTable, _, err := BuildParseTable(syntaxGrammar, lexicalGrammar, itemSetBuilder, variableInfo, strPool, diagnostics)
	if err != nil {
		return nil, err
	}
	tokenConflictMap := NewTokenConflictMap(lexicalGrammar, followingTokens)
	coincidentTokenIndex := NewCoincidentTokenIndex(&parseTable, lexicalGrammar, syntaxGrammar.WordToken, syntaxGrammar.HasWordToken)
	keywords := identifyKeywords(lexicalGrammar, syntaxGrammar.WordToken, syntaxGrammar.HasWordToken, tokenConflictMap, coincidentTokenIndex)
	populateErrorState(&parseTable, syntaxGrammar, lexicalGrammar, coincidentTokenIndex, tokenConflictMap, &keywords)
	populateUsedSymbols(&parseTable, syntaxGrammar, lexicalGrammar)
	MinimizeParseTable(&parseTable, syntaxGrammar, lexicalGrammar, simpleAliases, tokenConflictMap, &keywords, strPool, optimizations)
	lexTables := BuildLexTable(&parseTable, syntaxGrammar, lexicalGrammar, &keywords, coincidentTokenIndex, tokenConflictMap)
	populateExternalLexStates(&parseTable, syntaxGrammar)
	markFragileTokens(&parseTable, tokenConflictMap)
	parseTable.ActionLists.Canonicalize(parseTable.States)

	if len(parseTable.States) > math.MaxUint16 {
		return nil, &ParseTableBuilderError{Kind: ParseTableBuilderStateCount, StateCount: len(parseTable.States)}
	}

	return &Tables{
		ParseTable:         parseTable,
		MainLexTable:       lexTables.MainLexTable,
		KeywordLexTable:    lexTables.KeywordLexTable,
		LargeCharacterSets: lexTables.LargeCharacterSets,
	}, nil
}

// getFollowingTokens returns, for each terminal, the terminals that can
// follow it.
//
// getFollowingTokens is get_following_tokens.
func getFollowingTokens(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, builder *ParseItemSetBuilder) []TokenSet {
	nTerminals := len(lexicalGrammar.Variables)
	nExternals := len(syntaxGrammar.ExternalTokens)
	result := make([]TokenSet, nTerminals)
	for i := range result {
		result[i] = NewTokenSetWithCapacity(nTerminals, nExternals)
	}
	var allTokens TokenSet
	for i := range result {
		allTokens.Insert(TerminalSymbol(i))
	}
	for _, production := range syntaxGrammar.Productions {
		start, end := production.StepRange()
		steps := syntaxGrammar.Steps[start:end]
		for i := 1; i < len(steps); i++ {
			leftTokens := builder.LastSet(steps[i-1].Symbol())
			rightTokens := builder.FirstSet(steps[i].Symbol())
			rightReservedTokens, hasReserved := builder.ReservedFirstSet(steps[i].Symbol())
			for leftToken := range leftTokens.All() {
				if index, ok := leftToken.TerminalIndex(); ok {
					result[index].InsertAllTerminals(rightTokens)
					if hasReserved {
						result[index].InsertAllTerminals(rightReservedTokens)
					}
				}
			}
		}
	}
	for _, extra := range syntaxGrammar.ExtraSymbols {
		if index, ok := extra.TerminalIndex(); ok {
			for i := range result {
				result[i].Insert(extra)
			}
			result[index] = allTokens.Clone()
		}
	}
	return result
}

// populateErrorState fills the state of error recovery, state 0, with a
// recover action for each token that can be recovered to.
//
// populateErrorState is populate_error_state.
func populateErrorState(parseTable *ParseTable[ActionListID], syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, coincidentTokenIndex *CoincidentTokenIndex, tokenConflictMap *TokenConflictMap, keywords *TokenSet) {
	n := len(lexicalGrammar.Variables)

	// First find the tokens that are free of conflicts: the tokens that
	// overlap no other token in any way, other than to match exactly the
	// same string.
	var conflictFreeTokens TokenSet
	for i := range n {
		a := TerminalIndex(i)
		conflictsWithOtherTokens := false
		for j := range n {
			if j != i && !coincidentTokenIndex.Contains(a, TerminalIndex(j)) && tokenConflictMap.DoesMatchShorterOrLonger(i, j) {
				conflictsWithOtherTokens = true
				break
			}
		}
		if !conflictsWithOtherTokens {
			conflictFreeTokens.Insert(TerminalSymbol(i))
		}
	}

	recoverEntry := NewActionListID(parseTable.ActionLists.Push([]ParseAction{{Kind: ParseActionRecover}}), false)
	state := &parseTable.States[0]

	// Leave out of the state of error recovery each token that conflicts
	// with one of the tokens that are free of conflicts.
	for i := range n {
		symbol := TerminalSymbol(i)
		if !conflictFreeTokens.Contains(symbol) &&
			!keywords.Contains(symbol) &&
			(!syntaxGrammar.HasWordToken || syntaxGrammar.WordToken != symbol) {
			excluded := false
			for other := range conflictFreeTokens.Terminals() {
				if !coincidentTokenIndex.Contains(TerminalIndex(i), other) && tokenConflictMap.DoesConflict(i, int(other)) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}
		state.TerminalEntries.GetOrInsert(symbol, recoverEntry)
	}

	for i, externalToken := range syntaxGrammar.ExternalTokens {
		if !externalToken.HasCorrespondingInternalToken {
			state.TerminalEntries.GetOrInsert(ExternalSymbol(i), recoverEntry)
		}
	}

	state.TerminalEntries.Insert(SymbolEndValue, recoverEntry)
}

// populateUsedSymbols lists the symbols that the parse table uses, in the
// order of the symbol numbers of the parser: the end, the terminals with
// the word token first, the external tokens and the non-terminals.
//
// populateUsedSymbols is populate_used_symbols.
func populateUsedSymbols(parseTable *ParseTable[ActionListID], syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar) {
	terminalUsages := make([]bool, len(lexicalGrammar.Variables))
	nonTerminalUsages := make([]bool, len(syntaxGrammar.Variables))
	externalUsages := make([]bool, len(syntaxGrammar.ExternalTokens))
	for i := range parseTable.States {
		state := &parseTable.States[i]
		for symbol := range state.TerminalEntries.Keys() {
			switch symbol.Kind() {
			case SymbolTerminal:
				index, _ := symbol.TerminalIndex()
				terminalUsages[index] = true
			case SymbolExternal:
				index, _ := symbol.ExternalIndex()
				externalUsages[index] = true
			case SymbolNonTerminal:
				panic("internal error: entered unreachable code")
			}
		}
		for symbol := range state.NonterminalEntries.Keys() {
			index, ok := symbol.NonTerminalIndex()
			if !ok {
				panic("internal error: entered unreachable code")
			}
			nonTerminalUsages[index] = true
		}
	}
	parseTable.Symbols = append(parseTable.Symbols, SymbolEndValue)
	for i, used := range terminalUsages {
		if !used {
			continue
		}
		// The word token gets a low number, so that a subtree can hold it
		// with no memory on the heap, even for a grammar with very many
		// tokens. It is an optimization, and it also makes sure that the
		// symbol of a subtree can change to the word token without moving
		// the subtree to the heap. See
		// https://github.com/tree-sitter/tree-sitter/issues/258.
		if syntaxGrammar.HasWordToken && syntaxGrammar.WordToken == TerminalSymbol(i) {
			parseTable.Symbols = slices.Insert(parseTable.Symbols, 1, TerminalSymbol(i))
		} else {
			parseTable.Symbols = append(parseTable.Symbols, TerminalSymbol(i))
		}
	}
	for i, used := range externalUsages {
		if used {
			parseTable.Symbols = append(parseTable.Symbols, ExternalSymbol(i))
		}
	}
	for i, used := range nonTerminalUsages {
		if used {
			parseTable.Symbols = append(parseTable.Symbols, NonTerminalSymbol(i))
		}
	}
}

// populateExternalLexStates gives each parse state the number of its set of
// external tokens, and lists the sets. Set 0 is the empty set.
//
// populateExternalLexStates is populate_external_lex_states.
func populateExternalLexStates(parseTable *ParseTable[ActionListID], syntaxGrammar *SyntaxGrammar) {
	externalTokensByCorrespondingInternalToken := map[TerminalIndex]int{}
	for i, externalToken := range syntaxGrammar.ExternalTokens {
		if !externalToken.HasCorrespondingInternalToken {
			continue
		}
		if index, ok := externalToken.CorrespondingInternalToken.TerminalIndex(); ok {
			externalTokensByCorrespondingInternalToken[index] = i
		}
	}

	// external lex state 0 stands for no external tokens
	parseTable.ExternalLexStates = append(parseTable.ExternalLexStates, TokenSet{})

	for i := range parseTable.States {
		var externalTokens TokenSet
		for token := range parseTable.States[i].TerminalEntries.Keys() {
			switch token.Kind() {
			case SymbolExternal:
				externalTokens.Insert(token)
			case SymbolTerminal:
				tokenIndex, _ := token.TerminalIndex()
				if index, ok := externalTokensByCorrespondingInternalToken[tokenIndex]; ok {
					externalTokens.Insert(ExternalSymbol(index))
				}
			case SymbolNonTerminal:
				panic("internal error: entered unreachable code")
			}
		}

		id := slices.IndexFunc(parseTable.ExternalLexStates, func(tokens TokenSet) bool { return tokens.Equal(&externalTokens) })
		if id < 0 {
			parseTable.ExternalLexStates = append(parseTable.ExternalLexStates, externalTokens)
			id = len(parseTable.ExternalLexStates) - 1
		}
		parseTable.States[i].ExternalLexStateID = uint32(id)
	}
}

// identifyKeywords returns the tokens that the lexer can read as the word
// token and then match against the keyword lexer.
//
// identifyKeywords is identify_keywords.
func identifyKeywords(lexicalGrammar *LexicalGrammar, wordToken Symbol, hasWordToken bool, tokenConflictMap *TokenConflictMap, coincidentTokenIndex *CoincidentTokenIndex) TokenSet {
	var wordTokenIndex int
	switch {
	case hasWordToken && wordToken.Kind() == SymbolTerminal:
		wordTokenIndex = int(wordToken.index)
	// An external token has no lexical rule to compare with keywords.
	case !hasWordToken || wordToken.Kind() == SymbolExternal:
		return TokenSet{}
	// INVARIANT: Token extraction rejects a non-terminal word token.
	default:
		panic("internal error: entered unreachable code")
	}
	cursor := NewNfaCursor(&lexicalGrammar.Nfa, nil)

	// First find the candidates: the tokens that start with a letter or an
	// underscore and can match the same string as the word token.
	var keywordCandidates TokenSet
	for i, variable := range lexicalGrammar.Variables {
		cursor.Reset([]uint32{variable.StartState})
		if allCharsAreAlphabetical(cursor) &&
			tokenConflictMap.DoesMatchSameString(i, wordTokenIndex) &&
			!tokenConflictMap.DoesMatchDifferentString(i, wordTokenIndex) {
			keywordCandidates.Insert(TerminalSymbol(i))
		}
	}

	// Leave out a candidate that shadows another candidate.
	var keywords TokenSet
	for token := range keywordCandidates.Terminals() {
		shadows := false
		for other := range keywordCandidates.Terminals() {
			if other != token && tokenConflictMap.DoesMatchSameString(int(other), int(token)) {
				shadows = true
				break
			}
		}
		if !shadows {
			keywords.Insert(token.Symbol())
		}
	}

	// Leave out a candidate for which the use of the word token in its place
	// would add new conflicts with other tokens.
	var result TokenSet
	for token := range keywords.Terminals() {
		tokenIndex := int(token)
		keep := true
		for otherIndex := range lexicalGrammar.Variables {
			if keywordCandidates.Contains(TerminalSymbol(otherIndex)) {
				continue
			}

			// When the word token was valid in every state that holds this
			// candidate already, the word token in its place adds no
			// conflict.
			if coincidentTokenIndex.AllCoincidentStatesHaveWord(token, TerminalIndex(otherIndex)) {
				continue
			}

			if !tokenConflictMap.HasSameConflictStatus(tokenIndex, wordTokenIndex, otherIndex) {
				keep = false
				break
			}
		}
		if keep {
			result.Insert(token.Symbol())
		}
	}
	return result
}

// markFragileTokens marks as not reusable each token of a state that can
// overlap another token of that state.
//
// markFragileTokens is mark_fragile_tokens.
func markFragileTokens(parseTable *ParseTable[ActionListID], tokenConflictMap *TokenConflictMap) {
	var validTerminalIndices []TerminalIndex
	for s := range parseTable.States {
		state := &parseTable.States[s]
		validTerminalIndices = validTerminalIndices[:0]
		for token := range state.TerminalEntries.Keys() {
			if index, ok := token.TerminalIndex(); ok {
				validTerminalIndices = append(validTerminalIndices, index)
			}
		}
		for token, id := range state.TerminalEntries.AllMut() {
			index, ok := token.TerminalIndex()
			if !ok {
				continue
			}
			for _, i := range validTerminalIndices {
				if tokenConflictMap.DoesOverlap(int(i), int(index)) {
					id.SetReusable(false)
					break
				}
			}
		}
	}
}

// allCharsAreAlphabetical reports whether each move from the states of the
// cursor that is not a separator is on letters or underscores only.
//
// allCharsAreAlphabetical is all_chars_are_alphabetical.
func allCharsAreAlphabetical(cursor *NfaCursor) bool {
	for chars, isSep := range cursor.TransitionChars() {
		if isSep {
			continue
		}
		for c := range chars.Chars() {
			if !isAlphabetic(c) && c != '_' {
				return false
			}
		}
	}
	return true
}

// isAlphabetic reports whether a character has the Unicode property
// Alphabetic, as char::is_alphabetic of Rust does. The property is the
// letters, the letter numbers, and the characters of Other_Alphabetic,
// Other_Lowercase and Other_Uppercase. The Go package unicode has each of
// these tables. On 2026-09-29 Go 1.27 and Rust 1.97.1, which built the
// upstream tool of the golden files, are both at Unicode 17.0.0.
func isAlphabetic(c rune) bool {
	return unicode.IsLetter(c) ||
		unicode.Is(unicode.Nl, c) ||
		unicode.Is(unicode.Other_Alphabetic, c) ||
		unicode.Is(unicode.Other_Lowercase, c) ||
		unicode.Is(unicode.Other_Uppercase, c)
}
