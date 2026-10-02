// Package abi holds the tables of a grammar in the shape of TSLanguage of
// lib/src/parser.h of upstream. A grammar package fills a Language, and
// transit.NewLanguage builds the language of the runtime from it (D63).
//
// The package is internal, so only code under github.com/xo/transit/ can fill
// a Language: the grammar packages that the Go backend writes, and the test
// module, which copies the tables of a C grammar through cgo (D12). The form of
// the tables can change in phase 4 (D31) without a change of the exported API.
package abi

// This file ports the types and the constants of lib/src/parser.h: the
// language, the lexer, the parse actions and the external scanner. The Go
// backend writes a lex table as data, and not as a C lex function. So the
// macros of a lex function (START_LEXER, ADVANCE, SKIP, ADVANCE_MAP,
// ACCEPT_TOKEN and END_STATE) and set_contains become one interpreter,
// LexTable.Lex, with the same calls of the lexer. TSCharacterRange has no Go
// form, because LexRange holds the ranges. The macros of the parse table,
// such as STATE and ACTIONS, have no Go form, because the Go backend writes
// the numbers. TSLexMode, the lex mode of ABI 14, has no Go form, because
// LexModes holds the modes of both versions.

// The symbols that every language has.
const (
	// BuiltinSymError is ts_builtin_sym_error, the symbol of an ERROR node.
	BuiltinSymError uint16 = 0xFFFF
	// BuiltinSymEnd is ts_builtin_sym_end, the symbol of the end of the input.
	BuiltinSymEnd uint16 = 0
)

// SerializationBufferSize is TREE_SITTER_SERIALIZATION_BUFFER_SIZE, the size
// of the buffer that an external scanner writes its state to.
const SerializationBufferSize = 1024

// LanguageMetadata is TSLanguageMetadata, the version of a grammar.
type LanguageMetadata struct {
	MajorVersion uint8
	MinorVersion uint8
	PatchVersion uint8
}

// FieldMapEntry is TSFieldMapEntry. It gives the field of one child of a
// production.
type FieldMapEntry struct {
	FieldID    uint16
	ChildIndex uint8
	Inherited  bool
}

// MapSlice is TSMapSlice. It indexes the field and supertype maps.
type MapSlice struct {
	Index  uint16
	Length uint16
}

// SymbolMetadata is TSSymbolMetadata.
type SymbolMetadata struct {
	Visible   bool
	Named     bool
	Supertype bool
}

// Lexer is TSLexer, the lexer that a lex function and an external scanner
// read the input with. The runtime sets Funcs, and the methods of Lexer are
// the function members of TSLexer.
type Lexer struct {
	// Lookahead is lookahead, the character at the position of the lexer.
	Lookahead int32
	// ResultSymbol is result_symbol, the symbol of the token that the lex
	// function or the scanner found.
	ResultSymbol uint16

	// Funcs holds the functions of the lexer of the runtime.
	Funcs LexerFuncs
}

// LexerFuncs is the function members of TSLexer, which the lexer of the
// runtime implements.
type LexerFuncs interface {
	Advance(skip bool)
	MarkEnd()
	GetColumn() uint32
	IsAtIncludedRangeStart() bool
	EOF() bool
	Logf(format string, args ...any)
}

// Advance is advance. It moves the lexer to the next character. When skip is
// true, the character is whitespace, and it is not part of the token.
func (l *Lexer) Advance(skip bool) {
	l.Funcs.Advance(skip)
}

// MarkEnd is mark_end. It marks the end of the token at the position of the
// lexer.
func (l *Lexer) MarkEnd() {
	l.Funcs.MarkEnd()
}

// GetColumn is get_column. It returns the column of the position of the
// lexer, in bytes.
func (l *Lexer) GetColumn() uint32 {
	return l.Funcs.GetColumn()
}

// IsAtIncludedRangeStart is is_at_included_range_start. It reports whether
// the lexer is at the start of an included range.
func (l *Lexer) IsAtIncludedRangeStart() bool {
	return l.Funcs.IsAtIncludedRangeStart()
}

// EOF is eof. It reports whether the lexer is at the end of the input.
func (l *Lexer) EOF() bool {
	return l.Funcs.EOF()
}

// Logf is log. It writes a message to the logger of the parser, if it has
// one.
func (l *Lexer) Logf(format string, args ...any) {
	l.Funcs.Logf(format, args...)
}

// ParseActionType is TSParseActionType.
type ParseActionType uint8

// The kinds of a parse action.
const (
	// ParseActionTypeShift is TSParseActionTypeShift.
	ParseActionTypeShift ParseActionType = iota
	// ParseActionTypeReduce is TSParseActionTypeReduce.
	ParseActionTypeReduce
	// ParseActionTypeAccept is TSParseActionTypeAccept.
	ParseActionTypeAccept
	// ParseActionTypeRecover is TSParseActionTypeRecover.
	ParseActionTypeRecover
)

// String returns the name of the kind.
func (t ParseActionType) String() string {
	switch t {
	case ParseActionTypeShift:
		return "shift"
	case ParseActionTypeReduce:
		return "reduce"
	case ParseActionTypeAccept:
		return "accept"
	case ParseActionTypeRecover:
		return "recover"
	}
	return "unknown"
}

// ParseAction is TSParseAction, a union in C. Type says which of Shift and
// Reduce holds the action. An accept and a recover action have only a Type.
type ParseAction struct {
	Type   ParseActionType
	Shift  ShiftAction
	Reduce ReduceAction
}

// ShiftAction is the member shift of TSParseAction.
type ShiftAction struct {
	State      uint16
	Extra      bool
	Repetition bool
}

// ReduceAction is the member reduce of TSParseAction.
type ReduceAction struct {
	ChildCount        uint8
	Symbol            uint16
	DynamicPrecedence int16
	ProductionID      uint16
}

// LexerMode is TSLexerMode, the lex mode of a parse state.
type LexerMode struct {
	LexState          uint16
	ExternalLexState  uint16
	ReservedWordSetID uint16
}

// ParseActionEntry is TSParseActionEntry, a union in C. The parse actions of
// a state and a symbol are a group: one entry that holds Entry, and then
// Entry.Count entries that each hold an Action.
type ParseActionEntry struct {
	Action ParseAction
	Entry  EntryHeader
}

// EntryHeader is the member entry of TSParseActionEntry, the first entry of a
// group of parse actions.
type EntryHeader struct {
	Count    uint8
	Reusable bool
}

// LexFunc is the type of lex_fn and keyword_lex_fn. It lexes one token from
// the lexer in a lex state, and reports whether it found one.
type LexFunc func(lexer *Lexer, state uint16) bool

// LexTable is a lex table as data: the states of a lex function of the C
// backend, with the transitions of each state as sorted ranges of
// characters. The Go backend writes it, and its lex function calls Lex.
type LexTable struct {
	States []LexState
	Ranges []LexRange
}

// LexState is one state of a LexTable.
type LexState struct {
	// Start and Count give the transitions of the state in Ranges: Count
	// ranges from Start. The ranges are sorted, and they do not overlap.
	Start uint32
	Count uint32
	// Accept is the symbol that the state accepts, when HasAccept is true.
	// It is ACCEPT_TOKEN of the C lex function.
	Accept    uint16
	HasAccept bool
	// EOFState is the state that the state goes to at the end of the input,
	// when HasEOF is true. EOFSkip is true when it skips, as SKIP does.
	EOFState uint16
	HasEOF   bool
	EOFSkip  bool
}

// LexRange is a transition of a LexState: each character from Lo to Hi goes
// to State. Skip is true when the character is not part of the token, as
// SKIP does.
type LexRange struct {
	Lo    int32
	Hi    int32
	State uint16
	Skip  bool
}

// Lex lexes one token from the lexer in a state of the table, and reports
// whether it found one. It calls the lexer as the C lex function of the same
// table does, in the same order: Lookahead and EOF when it enters a state,
// MarkEnd when the state accepts a token, and Advance for each transition.
//
// Lex is the lex function of parser.c, with the macros START_LEXER, ADVANCE,
// SKIP, ADVANCE_MAP, ACCEPT_TOKEN and END_STATE, and set_contains.
func (t *LexTable) Lex(lexer *Lexer, state uint16) bool {
	result := false
	for {
		lookahead := lexer.Lookahead
		eof := lexer.EOF()
		if int(state) >= len(t.States) {
			// the default case of the switch of the C lex function
			return false
		}
		s := &t.States[state]
		if s.HasAccept {
			result = true
			lexer.ResultSymbol = s.Accept
			lexer.MarkEnd()
		}
		var skip bool
		if eof {
			if !s.HasEOF {
				return result
			}
			state, skip = s.EOFState, s.EOFSkip
		} else {
			ranges := t.Ranges[s.Start : s.Start+s.Count]
			// the first range that ends at or after the lookahead
			lo, hi := 0, len(ranges)
			for lo < hi {
				mid := int(uint(lo+hi) >> 1)
				if ranges[mid].Hi < lookahead {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			if lo == len(ranges) || ranges[lo].Lo > lookahead {
				return result
			}
			state, skip = ranges[lo].State, ranges[lo].Skip
		}
		lexer.Advance(skip)
	}
}

// Scanner is the payload of an external scanner, with the functions scan,
// serialize and deserialize of the member external_scanner of TSLanguage.
// The garbage collector frees it, so destroy has no Go form.
type Scanner interface {
	// Scan is scan. validSymbols holds one entry for each external token,
	// and it is true for each token that the parser can accept.
	Scan(lexer *Lexer, validSymbols []bool) bool
	// Serialize is serialize. It writes the state of the scanner to buf,
	// which holds SerializationBufferSize bytes, and returns the number of
	// bytes that it wrote.
	Serialize(buf []byte) int
	// Deserialize is deserialize. It restores the state that Serialize wrote
	// to buf. An empty buf is the state of a new scanner.
	Deserialize(buf []byte)
}

// ExternalScanner is the member external_scanner of TSLanguage.
type ExternalScanner struct {
	// States is states: for each external lex state, the external tokens
	// that are valid in it, ExternalTokenCount entries each.
	States []bool
	// SymbolMap is symbol_map. It maps each external token to its symbol.
	SymbolMap []uint16
	// Create is create. It returns a new scanner. It is nil when the grammar
	// has no external scanner.
	Create func() Scanner
}

// Language is TSLanguage, the tables of a grammar. A slice holds each table
// that TSLanguage points to, and a count keeps the width of its C member.
type Language struct {
	ABIVersion             uint32
	SymbolCount            uint32
	AliasCount             uint32
	TokenCount             uint32
	ExternalTokenCount     uint32
	StateCount             uint32
	LargeStateCount        uint32
	ProductionIDCount      uint32
	FieldCount             uint32
	MaxAliasSequenceLength uint16
	ParseTable             []uint16
	SmallParseTable        []uint16
	SmallParseTableMap     []uint32
	ParseActions           []ParseActionEntry
	SymbolNames            []string
	// FieldNames holds FieldCount+1 names. The name at index 0 is empty, as
	// field_names[0] is NULL in C.
	FieldNames      []string
	FieldMapSlices  []MapSlice
	FieldMapEntries []FieldMapEntry
	SymbolMetadata  []SymbolMetadata
	PublicSymbolMap []uint16
	AliasMap        []uint16
	AliasSequences  []uint16
	// LexModes holds the lex mode of each state. A language of ABI 14 has no
	// reserved words, and its modes have a ReservedWordSetID of 0.
	LexModes               []LexerMode
	LexFn                  LexFunc
	KeywordLexFn           LexFunc
	KeywordCaptureToken    uint16
	ExternalScanner        ExternalScanner
	PrimaryStateIDs        []uint16
	Name                   string
	ReservedWords          []uint16
	MaxReservedWordSetSize uint16
	SupertypeCount         uint32
	SupertypeSymbols       []uint16
	SupertypeMapSlices     []MapSlice
	SupertypeMapEntries    []uint16
	Metadata               LanguageMetadata
}

// TablesOf returns a copy of the tables of a *transit.Language. The root
// package sets it when the program starts, because this package cannot
// import the root package. A grammar package calls it to build a language
// with its own tables and another external scanner, as the usql grammar
// does for the options of a dialect (D108). TablesOf ports nothing of
// upstream.
var TablesOf func(language any) Language
