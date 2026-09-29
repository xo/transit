package transit

import (
	"io"
	"iter"
	"math"
	"strings"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/language.c and lib/src/language.h, and the types of
// lib/include/tree_sitter/api.h that name the parts of a language. The tables
// themselves are in the shape of TSLanguage of lib/src/parser.h, in the
// package internal/abi (D63).
//
// These functions of language.c have no Go form. ts_language_copy and
// ts_language_delete count references, and the garbage collector frees a
// language (D24). ts_language_is_parseable is true for each language that Go
// can build, and ts_language_copy_without_callbacks and
// ts_language_current_context_id are for WebAssembly (D1).
// ts_lookahead_iterator_delete frees memory.

// Symbol is the number of a symbol in the tables of a language.
//
// Symbol is TSSymbol.
type Symbol uint16

// FieldID is the number of a field in the tables of a language. The field 0
// is no field.
//
// FieldID is TSFieldId.
type FieldID uint16

// StateID is the number of a parse state in the tables of a language.
//
// StateID is TSStateId.
type StateID uint16

// unknownName is the name that String gives a value of an enum that has no
// name.
const unknownName = "unknown"

// noneName is the name that String gives the value of an enum that means
// none.
const noneName = "none"

// SymbolType is the kind of a symbol.
//
// SymbolType is TSSymbolType.
type SymbolType int

// The kinds of a symbol.
const (
	// SymbolRegular is a named symbol that is visible in the tree, such as
	// identifier. It is TSSymbolTypeRegular.
	SymbolRegular SymbolType = iota
	// SymbolAnonymous is a symbol that is visible in the tree and has no name,
	// such as a keyword or a punctuation mark. It is TSSymbolTypeAnonymous.
	SymbolAnonymous
	// SymbolSupertype is a hidden symbol that stands for a set of symbols,
	// such as expression. It is TSSymbolTypeSupertype.
	SymbolSupertype
	// SymbolAuxiliary is a hidden symbol, such as a rule whose name starts
	// with an underscore. It is TSSymbolTypeAuxiliary.
	SymbolAuxiliary
)

// String returns the name of the kind.
func (t SymbolType) String() string {
	switch t {
	case SymbolRegular:
		return "regular"
	case SymbolAnonymous:
		return "anonymous"
	case SymbolSupertype:
		return "supertype"
	case SymbolAuxiliary:
		return "auxiliary"
	}
	return unknownName
}

// LanguageMetadata is the version of a grammar.
//
// LanguageMetadata is TSLanguageMetadata.
type LanguageMetadata struct {
	Major, Minor, Patch int
}

// Language is the tables of one grammar. A grammar package returns it. It is
// safe to share between goroutines.
//
// Language is TSLanguage.
type Language struct {
	tables abi.Language
}

// NewLanguage builds a language from its tables. Only code under
// github.com/xo/transit/, such as a grammar package, can build an
// *abi.Language, because the package abi is internal (D63). The language
// keeps its own copy of the struct, and it shares the tables.
func NewLanguage(tables *abi.Language) *Language {
	return &Language{tables: *tables}
}

// The symbols that every language has.
const (
	// builtinSymError is ts_builtin_sym_error.
	builtinSymError = Symbol(abi.BuiltinSymError)
	// builtinSymErrorRepeat is ts_builtin_sym_error_repeat.
	builtinSymErrorRepeat = builtinSymError - 1
	// builtinSymEnd is ts_builtin_sym_end.
	builtinSymEnd = Symbol(abi.BuiltinSymEnd)
)

// The first ABI versions that have a part of the tables.
const (
	// languageVersionWithReservedWords is
	// LANGUAGE_VERSION_WITH_RESERVED_WORDS.
	languageVersionWithReservedWords = 15
	// languageVersionWithPrimaryStates is
	// LANGUAGE_VERSION_WITH_PRIMARY_STATES.
	languageVersionWithPrimaryStates = 14
)

// tableEntry is TableEntry, the parse actions of a state and a terminal
// symbol.
type tableEntry struct {
	actions    []abi.ParseActionEntry
	isReusable bool
}

// lookaheadPhase is LookaheadPhase.
type lookaheadPhase uint8

// The phases of a lookahead iterator.
const (
	// lookaheadFresh is LookaheadFresh: no `next()` yet.
	lookaheadFresh lookaheadPhase = iota
	// lookaheadPositioned is LookaheadPositioned: the last `next()` returned
	// true.
	lookaheadPositioned
	// lookaheadDone is LookaheadDone: the last `next()` returned false.
	lookaheadDone
)

// String returns the name of the phase.
func (p lookaheadPhase) String() string {
	switch p {
	case lookaheadFresh:
		return "fresh"
	case lookaheadPositioned:
		return "positioned"
	case lookaheadDone:
		return "done"
	}
	return unknownName
}

// LookaheadIterator lists the symbols that the parser can accept in a state.
// It belongs to one goroutine at a time.
//
// LookaheadIterator is LookaheadIterator and TSLookaheadIterator. In C, data
// and group_end point into a parse table. In Go, data is the table from the
// start of the state, and pos and groupEnd are indices into it.
type LookaheadIterator struct {
	language     *Language
	data         []uint16
	pos          int
	groupEnd     int
	tableValue   uint16
	groupCount   uint16
	isSmallState bool
	phase        lookaheadPhase
	actions      []abi.ParseActionEntry
	symbol       Symbol
	nextState    StateID
}

// actions returns the parse actions of a state and a terminal symbol.
//
// actions is ts_language_actions.
func (l *Language) actions(state StateID, symbol Symbol) []abi.ParseActionEntry {
	entry := l.tableEntry(state, symbol)
	return entry.actions
}

// hasReduceAction reports whether the first parse action of a state and a
// terminal symbol is a reduce.
//
// hasReduceAction is ts_language_has_reduce_action.
func (l *Language) hasReduceAction(state StateID, symbol Symbol) bool {
	entry := l.tableEntry(state, symbol)
	return len(entry.actions) > 0 && entry.actions[0].Action.Type == abi.ParseActionTypeReduce
}

// lookup is ts_language_lookup.
//
// Lookup the table value for a given symbol and state.
//
// For non-terminal symbols, the table value represents a successor state.
// For terminal symbols, it represents an index in the actions table.
// For 'large' parse states, this is a direct lookup. For 'small' parse
// states, this requires searching through the symbol groups to find
// the given symbol.
func (l *Language) lookup(state StateID, symbol Symbol) uint16 {
	if uint32(state) >= l.tables.LargeStateCount {
		index := l.tables.SmallParseTableMap[uint32(state)-l.tables.LargeStateCount]
		data := l.tables.SmallParseTable[index:]
		pos := 0
		groupCount := data[pos]
		pos++
		for range groupCount {
			sectionValue := data[pos]
			pos++
			symbolCount := data[pos]
			pos++
			for range symbolCount {
				s := Symbol(data[pos])
				pos++
				if s == symbol {
					return sectionValue
				}
			}
		}
		return 0
	}
	return l.tables.ParseTable[uint32(state)*l.tables.SymbolCount+uint32(symbol)]
}

// hasActions is ts_language_has_actions.
func (l *Language) hasActions(state StateID, symbol Symbol) bool {
	return l.lookup(state, symbol) != 0
}

// lookaheads is ts_language_lookaheads.
//
// Iterate over all of the symbols that are valid in the given state.
//
// For 'large' parse states, this just requires iterating through
// all possible symbols and checking the parse table for each one.
// For 'small' parse states, this exploits the structure of the
// table to only visit the valid symbols.
func (l *Language) lookaheads(state StateID) LookaheadIterator {
	isSmallState := uint32(state) >= l.tables.LargeStateCount
	var data []uint16
	groupEnd := 0
	var groupCount uint16
	if isSmallState {
		index := l.tables.SmallParseTableMap[uint32(state)-l.tables.LargeStateCount]
		data = l.tables.SmallParseTable[index:]
		groupEnd = 1
		groupCount = data[0]
	} else {
		data = l.tables.ParseTable[uint32(state)*l.tables.SymbolCount:]
	}
	return LookaheadIterator{
		language:     l,
		data:         data,
		groupEnd:     groupEnd,
		groupCount:   groupCount,
		isSmallState: isSmallState,
		phase:        lookaheadFresh,
		symbol:       math.MaxUint16,
		nextState:    0,
	}
}

// next moves the iterator to the next valid symbol, and reports whether
// there is one.
//
// next is ts_lookahead_iterator__next.
func (it *LookaheadIterator) next() bool {
	if it.phase == lookaheadDone {
		return false
	}

	// For small parse states, valid symbols are listed explicitly,
	// grouped by their value. There's no need to look up the actions
	// again until moving to the next group.
	if it.isSmallState {
		it.pos++
		if it.pos == it.groupEnd {
			if it.groupCount == 0 {
				it.phase = lookaheadDone
				return false
			}
			it.groupCount--
			it.tableValue = it.data[it.pos]
			it.pos++
			symbolCount := int(it.data[it.pos])
			it.pos++
			it.groupEnd = it.pos + symbolCount
			it.symbol = Symbol(it.data[it.pos])
		} else {
			it.symbol = Symbol(it.data[it.pos])
			it.phase = lookaheadPositioned
			return true
		}
	} else {
		// For large parse states, iterate through every symbol until one
		// is found that has valid actions.
		row := it.data
		symbol := it.symbol + 1
		if it.phase == lookaheadFresh {
			symbol = 0
		}
		for uint32(symbol) < it.language.tables.SymbolCount && row[symbol] == 0 {
			symbol++
		}
		if uint32(symbol) >= it.language.tables.SymbolCount {
			it.phase = lookaheadDone
			return false
		}
		it.symbol = symbol
		it.tableValue = row[symbol]
	}

	// Depending on if the symbol is terminal or non-terminal, the table value either
	// represents a list of actions or a successor state.
	if uint32(it.symbol) < it.language.tables.TokenCount {
		index := uint32(it.tableValue)
		count := uint32(it.language.tables.ParseActions[index].Entry.Count)
		it.actions = it.language.tables.ParseActions[index+1 : index+1+count]
		it.nextState = 0
	} else {
		it.actions = nil
		it.nextState = StateID(it.tableValue)
	}
	it.phase = lookaheadPositioned
	return true
}

// stateIsPrimary is ts_language_state_is_primary.
//
// Whether the state is a "primary state". If this returns false, it indicates that there exists
// another state that behaves identically to this one with respect to query analysis.
func (l *Language) stateIsPrimary(state StateID) bool {
	if l.tables.ABIVersion >= languageVersionWithPrimaryStates {
		return state == StateID(l.tables.PrimaryStateIDs[state])
	}
	return true
}

// enabledExternalTokens returns the external tokens that are valid in an
// external lex state, one entry for each external token, or nil for the
// state 0.
//
// enabledExternalTokens is ts_language_enabled_external_tokens.
func (l *Language) enabledExternalTokens(externalScannerState uint32) []bool {
	if externalScannerState == 0 {
		return nil
	}
	start := l.tables.ExternalTokenCount * externalScannerState
	return l.tables.ExternalScanner.States[start : start+l.tables.ExternalTokenCount]
}

// aliasSequence returns the aliases of the children of a production, or nil
// for the production 0. The C function returns a pointer into the table, and
// a caller can read past the production into the next one, so the Go
// function returns the table from the start of the production.
//
// aliasSequence is ts_language_alias_sequence.
func (l *Language) aliasSequence(productionID uint32) []uint16 {
	if productionID == 0 {
		return nil
	}
	return l.tables.AliasSequences[productionID*uint32(l.tables.MaxAliasSequenceLength):]
}

// aliasAt is ts_language_alias_at.
func (l *Language) aliasAt(productionID, childIndex uint32) Symbol {
	if productionID == 0 {
		return 0
	}
	return Symbol(l.tables.AliasSequences[productionID*uint32(l.tables.MaxAliasSequenceLength)+childIndex])
}

// fieldMap returns the field map entries of a production. The C function
// writes a start and an end pointer, and the Go function returns the slice
// between them.
//
// fieldMap is ts_language_field_map.
func (l *Language) fieldMap(productionID uint32) []abi.FieldMapEntry {
	if l.tables.FieldCount == 0 {
		return nil
	}
	slice := l.tables.FieldMapSlices[productionID]
	return l.tables.FieldMapEntries[slice.Index : slice.Index+slice.Length]
}

// aliasesForSymbol returns the symbols that a symbol can appear as: the
// aliases of the symbol, or its public symbol when it has no alias. The C
// function writes a start and an end pointer, and the Go function returns
// the slice between them.
//
// aliasesForSymbol is ts_language_aliases_for_symbol.
func (l *Language) aliasesForSymbol(originalSymbol Symbol) []uint16 {
	result := l.tables.PublicSymbolMap[originalSymbol : originalSymbol+1]

	idx := 0
	for {
		symbol := Symbol(l.tables.AliasMap[idx])
		idx++
		if symbol == 0 || symbol > originalSymbol {
			break
		}
		count := int(l.tables.AliasMap[idx])
		idx++
		if symbol == originalSymbol {
			result = l.tables.AliasMap[idx : idx+count]
			break
		}
		idx += count
	}
	return result
}

// writeSymbolAsDotString writes the name of a symbol as the text of a string
// of the DOT language. A write error is dropped, as upstream drops the error
// of fputc.
//
// writeSymbolAsDotString is ts_language_write_symbol_as_dot_string.
func (l *Language) writeSymbolAsDotString(w io.Writer, symbol Symbol) {
	name := l.SymbolName(symbol)
	var b strings.Builder
	for i := 0; i < len(name) && name[i] != 0; i++ {
		switch chr := name[i]; chr {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(chr)
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteByte(chr)
		}
	}
	_, _ = io.WriteString(w, b.String())
}

// SymbolCount returns the number of symbols of the language, with the
// aliases.
//
// SymbolCount is ts_language_symbol_count.
func (l *Language) SymbolCount() int {
	return int(l.tables.SymbolCount + l.tables.AliasCount)
}

// StateCount returns the number of parse states of the language.
//
// StateCount is ts_language_state_count.
func (l *Language) StateCount() int {
	return int(l.tables.StateCount)
}

// Supertypes returns the supertype symbols of the language. A language of
// ABI 14 has none. The slice is a copy.
//
// Supertypes is ts_language_supertypes.
func (l *Language) Supertypes() []Symbol {
	if l.tables.ABIVersion >= languageVersionWithReservedWords {
		return symbols(l.tables.SupertypeSymbols[:l.tables.SupertypeCount])
	}
	return nil
}

// Subtypes returns the symbols that a supertype stands for. It returns nil
// for a symbol that is not a supertype, and for a language of ABI 14. The
// slice is a copy.
//
// Subtypes is ts_language_subtypes.
func (l *Language) Subtypes(supertype Symbol) []Symbol {
	if l.tables.ABIVersion < languageVersionWithReservedWords ||
		int(supertype) >= l.SymbolCount() ||
		!l.symbolMetadata(supertype).Supertype {
		return nil
	}

	slice := l.tables.SupertypeMapSlices[supertype]
	return symbols(l.tables.SupertypeMapEntries[slice.Index : slice.Index+slice.Length])
}

// ABIVersion returns the ABI version of the tables of the language.
//
// ABIVersion is ts_language_abi_version.
func (l *Language) ABIVersion() int {
	return int(l.tables.ABIVersion)
}

// Metadata returns the version of the grammar. A language of ABI 14 has no
// version, and then Metadata returns false.
//
// Metadata is ts_language_metadata.
func (l *Language) Metadata() (LanguageMetadata, bool) {
	if l.tables.ABIVersion >= languageVersionWithReservedWords {
		m := l.tables.Metadata
		return LanguageMetadata{Major: int(m.MajorVersion), Minor: int(m.MinorVersion), Patch: int(m.PatchVersion)}, true
	}
	return LanguageMetadata{}, false
}

// Name returns the name of the grammar. A language of ABI 14 has no name,
// and then Name returns "".
//
// Name is ts_language_name.
func (l *Language) Name() string {
	if l.tables.ABIVersion >= languageVersionWithReservedWords {
		return l.tables.Name
	}
	return ""
}

// FieldCount returns the number of fields of the language.
//
// FieldCount is ts_language_field_count.
func (l *Language) FieldCount() int {
	return int(l.tables.FieldCount)
}

// tableEntry returns the parse actions of a state and a terminal symbol.
//
// tableEntry is ts_language_table_entry.
func (l *Language) tableEntry(state StateID, symbol Symbol) tableEntry {
	if symbol == builtinSymError || symbol == builtinSymErrorRepeat {
		return tableEntry{actions: nil, isReusable: false}
	}
	assert(uint32(symbol) < l.tables.TokenCount)
	actionIndex := uint32(l.lookup(state, symbol))
	entry := &l.tables.ParseActions[actionIndex]
	count := uint32(entry.Entry.Count)
	return tableEntry{
		actions:    l.tables.ParseActions[actionIndex+1 : actionIndex+1+count],
		isReusable: entry.Entry.Reusable,
	}
}

// lexModeForState is ts_language_lex_mode_for_state.
func (l *Language) lexModeForState(state StateID) abi.LexerMode {
	if l.tables.ABIVersion < 15 {
		mode := l.tables.LexModes[state]
		return abi.LexerMode{
			LexState:          mode.LexState,
			ExternalLexState:  mode.ExternalLexState,
			ReservedWordSetID: 0,
		}
	}
	return l.tables.LexModes[state]
}

// isReservedWord is ts_language_is_reserved_word.
func (l *Language) isReservedWord(state StateID, symbol Symbol) bool {
	lexMode := l.lexModeForState(state)
	if lexMode.ReservedWordSetID > 0 {
		start := uint32(lexMode.ReservedWordSetID) * uint32(l.tables.MaxReservedWordSetSize)
		end := start + uint32(l.tables.MaxReservedWordSetSize)
		for i := start; i < end; i++ {
			if Symbol(l.tables.ReservedWords[i]) == symbol {
				return true
			}
			if l.tables.ReservedWords[i] == 0 {
				break
			}
		}
	}
	return false
}

// symbolMetadata is ts_language_symbol_metadata.
func (l *Language) symbolMetadata(symbol Symbol) abi.SymbolMetadata {
	switch symbol {
	case builtinSymError:
		return abi.SymbolMetadata{Visible: true, Named: true}
	case builtinSymErrorRepeat:
		return abi.SymbolMetadata{Visible: false, Named: false}
	}
	return l.tables.SymbolMetadata[symbol]
}

// publicSymbol is ts_language_public_symbol.
func (l *Language) publicSymbol(symbol Symbol) Symbol {
	if symbol == builtinSymError {
		return symbol
	}
	return Symbol(l.tables.PublicSymbolMap[symbol])
}

// NextState returns the parse state that follows a state after a symbol. It
// returns 0 when the symbol is not valid in the state.
//
// NextState is ts_language_next_state.
func (l *Language) NextState(state StateID, symbol Symbol) StateID {
	switch {
	case symbol == builtinSymError ||
		symbol == builtinSymErrorRepeat ||
		uint32(symbol) >= l.tables.SymbolCount ||
		uint32(state) >= l.tables.StateCount:
		return 0
	case uint32(symbol) < l.tables.TokenCount:
		actions := l.actions(state, symbol)
		if count := len(actions); count > 0 {
			action := actions[count-1].Action
			if action.Type == abi.ParseActionTypeShift {
				if action.Shift.Extra {
					return state
				}
				return StateID(action.Shift.State)
			}
		}
		return 0
	}
	return StateID(l.lookup(state, symbol))
}

// SymbolName returns the name of a symbol. It returns "" for a symbol that
// the language does not have.
//
// SymbolName is ts_language_symbol_name.
func (l *Language) SymbolName(symbol Symbol) string {
	switch {
	case symbol == builtinSymError:
		return "ERROR"
	case symbol == builtinSymErrorRepeat:
		return "_ERROR"
	case int(symbol) < l.SymbolCount():
		return l.tables.SymbolNames[symbol]
	}
	return ""
}

// SymbolForName returns the symbol with a name, named or anonymous, and
// reports whether the language has it.
//
// SymbolForName is ts_language_symbol_for_name. The C function compares the
// names with strncmp, so the Go function does too. A named name that is a
// prefix of "ERROR", such as "ERR", finds the symbol of ERROR, as it does in
// C.
func (l *Language) SymbolForName(name string, named bool) (Symbol, bool) {
	length := len(name)
	if named && strncmp(name, "ERROR", length) == 0 {
		return builtinSymError, true
	}
	count := uint16(l.SymbolCount())
	for i := Symbol(0); uint16(i) < count; i++ {
		metadata := l.symbolMetadata(i)
		if (!metadata.Visible && !metadata.Supertype) || metadata.Named != named {
			continue
		}
		symbolName := l.tables.SymbolNames[i]
		if strncmp(symbolName, name, length) == 0 && cByte(symbolName, length) == 0 {
			symbol := Symbol(l.tables.PublicSymbolMap[i])
			return symbol, symbol != 0
		}
	}
	return 0, false
}

// SymbolType returns the kind of a symbol.
//
// SymbolType is ts_language_symbol_type.
func (l *Language) SymbolType(symbol Symbol) SymbolType {
	metadata := l.symbolMetadata(symbol)
	switch {
	case metadata.Named && metadata.Visible:
		return SymbolRegular
	case metadata.Visible:
		return SymbolAnonymous
	case metadata.Supertype:
		return SymbolSupertype
	}
	return SymbolAuxiliary
}

// FieldName returns the name of a field. It returns "" for the field 0 and
// for a field that the language does not have.
//
// FieldName is ts_language_field_name_for_id.
func (l *Language) FieldName(id FieldID) string {
	count := l.FieldCount()
	if count != 0 && int(id) <= count {
		return l.tables.FieldNames[id]
	}
	return ""
}

// FieldForName returns the field with a name, and reports whether the
// language has it.
//
// FieldForName is ts_language_field_id_for_name. The field names of a
// language are sorted, and the C function stops when strncmp returns -1.
// glibc returns the difference of the first bytes that differ, and so does
// strncmp here, so the loop stops at the same field as in C.
func (l *Language) FieldForName(name string) (FieldID, bool) {
	nameLength := len(name)
	count := uint16(l.FieldCount())
	for i := uint16(1); int(i) < int(count)+1; i++ {
		switch strncmp(name, l.tables.FieldNames[i], nameLength) {
		case 0:
			if cByte(l.tables.FieldNames[i], nameLength) == 0 {
				return FieldID(i), true
			}
		case -1:
			return 0, false
		}
	}
	return 0, false
}

// LookaheadIterator returns an iterator of the symbols that are valid in a
// state. It returns false for a state that the language does not have.
//
// LookaheadIterator is ts_lookahead_iterator_new.
func (l *Language) LookaheadIterator(state StateID) (*LookaheadIterator, bool) {
	if uint32(state) >= l.tables.StateCount {
		return nil, false
	}
	it := l.lookaheads(state)
	return &it, true
}

// ResetState moves the iterator to the start of another state of the same
// language. It returns false, and leaves the iterator as it is, for a state
// that the language does not have.
//
// ResetState is ts_lookahead_iterator_reset_state.
func (it *LookaheadIterator) ResetState(state StateID) bool {
	if uint32(state) >= it.language.tables.StateCount {
		return false
	}
	*it = it.language.lookaheads(state)
	return true
}

// Language returns the language of the iterator.
//
// Language is ts_lookahead_iterator_language.
func (it *LookaheadIterator) Language() *Language {
	return it.language
}

// Reset moves the iterator to the start of a state of a language. It returns
// false, and leaves the iterator as it is, for a state that the language
// does not have.
//
// Reset is ts_lookahead_iterator_reset.
func (it *LookaheadIterator) Reset(language *Language, state StateID) bool {
	if uint32(state) >= language.tables.StateCount {
		return false
	}
	*it = language.lookaheads(state)
	return true
}

// Symbols returns the valid symbols that the iterator has not yet given. The
// iterator moves as the sequence runs, so a second range over it gives only
// the rest. Reset and ResetState start it again.
//
// Symbols calls ts_lookahead_iterator_next and
// ts_lookahead_iterator_current_symbol, as the Iterator of the Rust binding
// does.
func (it *LookaheadIterator) Symbols() iter.Seq[Symbol] {
	return func(yield func(Symbol) bool) {
		for it.next() {
			if !yield(it.currentSymbol()) {
				return
			}
		}
	}
}

// Names returns the names of the valid symbols that the iterator has not yet
// given. It moves the iterator as Symbols does.
//
// Names calls ts_lookahead_iterator_next and
// ts_lookahead_iterator_current_symbol_name, as iter_names of the Rust
// binding does.
func (it *LookaheadIterator) Names() iter.Seq[string] {
	return func(yield func(string) bool) {
		for it.next() {
			name, _ := it.currentSymbolName()
			if !yield(name) {
				return
			}
		}
	}
}

// currentSymbol is ts_lookahead_iterator_current_symbol.
func (it *LookaheadIterator) currentSymbol() Symbol {
	return it.symbol
}

// currentSymbolName returns the name of the current symbol, and false when
// the iterator is not at a symbol.
//
// currentSymbolName is ts_lookahead_iterator_current_symbol_name.
func (it *LookaheadIterator) currentSymbolName() (string, bool) {
	if it.phase != lookaheadPositioned {
		return "", false
	}
	return it.language.SymbolName(it.symbol), true
}

// symbols returns a copy of a table of symbols, as Symbol values.
func symbols(table []uint16) []Symbol {
	out := make([]Symbol, len(table))
	for i, s := range table {
		out[i] = Symbol(s)
	}
	return out
}

// strncmp is strncmp of the C library, for the C strings of upstream. A C
// string ends at its first NUL byte, and a Go string ends at its length, so
// strncmp reads each byte through cByte. It returns the difference of the
// first bytes that differ, as glibc does.
func strncmp(a, b string, n int) int {
	for i := range n {
		ca, cb := cByte(a, i), cByte(b, i)
		if ca != cb {
			return int(ca) - int(cb)
		}
		if ca == 0 {
			return 0
		}
	}
	return 0
}

// cByte returns the byte at index i of a string as C reads it: the byte, or
// the NUL byte that ends the string when i is at its end or past it.
func cByte(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}
