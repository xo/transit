package transit

import (
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// The symbols of the test language.
const (
	testSymEnd        Symbol = 0
	testSymPlus       Symbol = 1
	testSymIdentifier Symbol = 2
	testSymExpression Symbol = 3
	testSymStatement  Symbol = 4
	testSymRepeat     Symbol = 5
	testSymAlias      Symbol = 6
)

// shift returns a shift action.
func shift(state uint16, extra bool) abi.ParseActionEntry {
	return abi.ParseActionEntry{Action: abi.ParseAction{
		Type:  abi.ParseActionTypeShift,
		Shift: abi.ShiftAction{State: state, Extra: extra},
	}}
}

// group returns the first entry of a group of parse actions.
func group(count uint8, reusable bool) abi.ParseActionEntry {
	return abi.ParseActionEntry{Entry: abi.EntryHeader{Count: count, Reusable: reusable}}
}

// testTables returns the tables of a small language, written by hand. It has
// three tokens, three nonterminals and one alias, two large states and two
// small states, two fields, a supertype, a set of reserved words and three
// external lex states.
func testTables() *abi.Language {
	return &abi.Language{
		ABIVersion:             15,
		SymbolCount:            6,
		AliasCount:             1,
		TokenCount:             3,
		ExternalTokenCount:     2,
		StateCount:             4,
		LargeStateCount:        2,
		ProductionIDCount:      2,
		FieldCount:             2,
		MaxAliasSequenceLength: 3,
		ParseTable: []uint16{
			// state 0: "+" is an extra, identifier shifts to 2, and
			// expression goes to 1
			0, 3, 1, 1, 0, 0,
			// state 1: the end accepts, and "+" reduces or shifts to 3
			8, 5, 0, 0, 0, 0,
		},
		SmallParseTable: []uint16{
			// state 2: two groups
			2,
			5, 2, uint16(testSymEnd), uint16(testSymPlus),
			3, 1, uint16(testSymExpression),
			// state 3: one group
			1,
			10, 1, uint16(testSymIdentifier),
		},
		SmallParseTableMap: []uint32{0, 8},
		ParseActions: []abi.ParseActionEntry{
			group(0, false),
			group(1, true),
			shift(2, false),
			group(1, true),
			shift(0, true),
			group(2, false),
			{Action: abi.ParseAction{
				Type:   abi.ParseActionTypeReduce,
				Reduce: abi.ReduceAction{ChildCount: 3, Symbol: uint16(testSymExpression), ProductionID: 1},
			}},
			shift(3, false),
			group(1, true),
			{Action: abi.ParseAction{Type: abi.ParseActionTypeAccept}},
			group(1, true),
			{Action: abi.ParseAction{Type: abi.ParseActionTypeRecover}},
		},
		SymbolNames:    []string{"end", "+", "identifier", "expression", "_statement", "program_repeat1", "alias_name"},
		FieldNames:     []string{"", "left", "right"},
		FieldMapSlices: []abi.MapSlice{{Index: 0, Length: 0}, {Index: 0, Length: 2}},
		FieldMapEntries: []abi.FieldMapEntry{
			{FieldID: 1, ChildIndex: 0},
			{FieldID: 2, ChildIndex: 2},
		},
		SymbolMetadata: []abi.SymbolMetadata{
			{Visible: false, Named: true},
			{Visible: true, Named: false},
			{Visible: true, Named: true},
			{Visible: true, Named: true},
			{Visible: false, Named: true, Supertype: true},
			{Visible: false, Named: false},
			{Visible: true, Named: true},
		},
		PublicSymbolMap: []uint16{0, 1, 2, 3, 4, 5, 6},
		AliasMap:        []uint16{uint16(testSymExpression), 2, uint16(testSymExpression), uint16(testSymAlias), 0},
		AliasSequences:  []uint16{0, 0, 0, 0, uint16(testSymAlias), 0},
		LexModes: []abi.LexerMode{
			{LexState: 1},
			{LexState: 2, ExternalLexState: 1},
			{LexState: 3, ReservedWordSetID: 1},
			{LexState: 4, ExternalLexState: 2},
		},
		ExternalScanner: abi.ExternalScanner{
			States:    []bool{false, false, true, false, true, true},
			SymbolMap: []uint16{uint16(testSymPlus), uint16(testSymIdentifier)},
		},
		PrimaryStateIDs:        []uint16{0, 1, 2, 2},
		Name:                   "test",
		ReservedWords:          []uint16{0, 0, uint16(testSymIdentifier), 0},
		MaxReservedWordSetSize: 2,
		SupertypeCount:         1,
		SupertypeSymbols:       []uint16{uint16(testSymStatement)},
		SupertypeMapSlices:     []abi.MapSlice{{}, {}, {}, {}, {Index: 0, Length: 2}, {}, {}},
		SupertypeMapEntries:    []uint16{uint16(testSymIdentifier), uint16(testSymExpression)},
		Metadata:               abi.LanguageMetadata{MajorVersion: 1, MinorVersion: 2, PatchVersion: 3},
	}
}

// testLanguage returns the language of testTables, with the ABI version abi.
func testLanguage(abiVersion uint32) *Language {
	tables := testTables()
	tables.ABIVersion = abiVersion
	return NewLanguage(tables)
}

func TestNewLanguageCopiesTheStruct(t *testing.T) {
	tables := testTables()
	l := NewLanguage(tables)
	tables.Name = "changed"
	if got := l.Name(); got != "test" {
		t.Errorf("Name() = %q after a change of the tables, want %q", got, "test")
	}
}

func TestLanguageLookup(t *testing.T) {
	l := testLanguage(15)
	tests := []struct {
		state  StateID
		symbol Symbol
		want   uint16
	}{
		{0, testSymEnd, 0},
		{0, testSymPlus, 3},
		{0, testSymIdentifier, 1},
		{0, testSymExpression, 1},
		{1, testSymEnd, 8},
		{1, testSymPlus, 5},
		{2, testSymEnd, 5},
		{2, testSymPlus, 5},
		{2, testSymIdentifier, 0},
		{2, testSymExpression, 3},
		{3, testSymIdentifier, 10},
		{3, testSymEnd, 0},
		{3, testSymStatement, 0},
	}
	for _, test := range tests {
		if got := l.lookup(test.state, test.symbol); got != test.want {
			t.Errorf("lookup(%d, %d) = %d, want %d", test.state, test.symbol, got, test.want)
		}
		if got := l.hasActions(test.state, test.symbol); got != (test.want != 0) {
			t.Errorf("hasActions(%d, %d) = %t, want %t", test.state, test.symbol, got, test.want != 0)
		}
	}
}

func TestLanguageTableEntry(t *testing.T) {
	l := testLanguage(15)
	for _, symbol := range []Symbol{builtinSymError, builtinSymErrorRepeat} {
		if entry := l.tableEntry(1, symbol); entry.actions != nil || entry.isReusable {
			t.Errorf("tableEntry(1, %d) = %+v, want no actions", symbol, entry)
		}
	}
	entry := l.tableEntry(1, testSymPlus)
	if len(entry.actions) != 2 || entry.isReusable {
		t.Fatalf("tableEntry(1, +) has %d actions and reusable %t, want 2 and false", len(entry.actions), entry.isReusable)
	}
	if a := entry.actions[0].Action; a.Type != abi.ParseActionTypeReduce || Symbol(a.Reduce.Symbol) != testSymExpression {
		t.Errorf("the first action is %+v, want a reduce to expression", a)
	}
	if a := entry.actions[1].Action; a.Type != abi.ParseActionTypeShift || a.Shift.State != 3 {
		t.Errorf("the second action is %+v, want a shift to 3", a)
	}
	if entry := l.tableEntry(0, testSymEnd); len(entry.actions) != 0 || entry.isReusable {
		t.Errorf("tableEntry(0, end) = %+v, want no actions", entry)
	}
	if got := len(l.actions(0, testSymIdentifier)); got != 1 {
		t.Errorf("actions(0, identifier) has %d actions, want 1", got)
	}
	if !l.hasReduceAction(1, testSymPlus) {
		t.Error("hasReduceAction(1, +) = false, want true")
	}
	if l.hasReduceAction(0, testSymIdentifier) || l.hasReduceAction(0, testSymEnd) {
		t.Error("hasReduceAction is true for a state and a symbol with no reduce")
	}
}

func TestLanguageTableEntryAssertsATerminal(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("tableEntry of a nonterminal did not panic")
		}
	}()
	testLanguage(15).tableEntry(0, testSymExpression)
}

func TestLanguageNextState(t *testing.T) {
	l := testLanguage(15)
	tests := []struct {
		state  StateID
		symbol Symbol
		want   StateID
	}{
		// an extra keeps the state
		{0, testSymPlus, 0},
		{0, testSymIdentifier, 2},
		{0, testSymExpression, 1},
		{0, testSymEnd, 0},
		// the last action is the shift
		{1, testSymPlus, 3},
		// an accept is no shift
		{1, testSymEnd, 0},
		{2, testSymExpression, 3},
		// a recover is no shift
		{3, testSymIdentifier, 0},
		{0, builtinSymError, 0},
		{0, builtinSymErrorRepeat, 0},
		// an alias is past the symbol count
		{0, testSymAlias, 0},
		{4, testSymIdentifier, 0},
	}
	for _, test := range tests {
		if got := l.NextState(test.state, test.symbol); got != test.want {
			t.Errorf("NextState(%d, %d) = %d, want %d", test.state, test.symbol, got, test.want)
		}
	}
}

func TestLookaheadIterator(t *testing.T) {
	l := testLanguage(15)
	want := [][]Symbol{
		{testSymPlus, testSymIdentifier, testSymExpression},
		{testSymEnd, testSymPlus},
		{testSymEnd, testSymPlus, testSymExpression},
		{testSymIdentifier},
	}
	for state := range StateID(l.StateCount()) {
		it, ok := l.LookaheadIterator(state)
		if !ok {
			t.Fatalf("LookaheadIterator(%d) returned false", state)
		}
		if it.Language() != l {
			t.Errorf("Language() of the iterator of state %d is another language", state)
		}
		got := slices.Collect(it.Symbols())
		if !slices.Equal(got, want[state]) {
			t.Errorf("state %d: Symbols() = %v, want %v", state, got, want[state])
		}
		// the iterator lists each symbol that has a table value
		var valid []Symbol
		for s := range Symbol(l.tables.SymbolCount) {
			if l.hasActions(state, s) {
				valid = append(valid, s)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, valid) {
			t.Errorf("state %d: the iterator lists %v, and the table has values for %v", state, got, valid)
		}
	}
	if _, ok := l.LookaheadIterator(4); ok {
		t.Error("LookaheadIterator(4) returned true for a state past the state count")
	}
}

func TestLookaheadIteratorActions(t *testing.T) {
	l := testLanguage(15)
	it := l.lookaheads(2)
	type step struct {
		symbol    Symbol
		actions   int
		nextState StateID
	}
	var got []step
	for it.next() {
		got = append(got, step{it.symbol, len(it.actions), it.nextState})
	}
	want := []step{
		{testSymEnd, 2, 0},
		{testSymPlus, 2, 0},
		{testSymExpression, 0, 3},
	}
	if !slices.Equal(got, want) {
		t.Errorf("the steps of state 2 are %v, want %v", got, want)
	}
	if it.phase != lookaheadDone || it.next() {
		t.Errorf("after the last symbol, the phase is %v, want %v", it.phase, lookaheadDone)
	}
}

func TestLookaheadIteratorExhaustion(t *testing.T) {
	l := testLanguage(15)
	it, _ := l.LookaheadIterator(0)
	if name, ok := it.currentSymbolName(); ok || name != "" {
		t.Errorf("a fresh iterator has the name %q, %t, want none", name, ok)
	}
	for range it.Symbols() {
		break
	}
	if name, ok := it.currentSymbolName(); !ok || name != "+" {
		t.Errorf("after one symbol, the name is %q, %t, want %q", name, ok, "+")
	}
	if got := slices.Collect(it.Names()); !slices.Equal(got, []string{"identifier", "expression"}) {
		t.Errorf("Names() after one symbol = %q, want the other two", got)
	}
	if got := slices.Collect(it.Symbols()); len(got) != 0 {
		t.Errorf("Symbols() of a finished iterator = %v, want none", got)
	}
	if _, ok := it.currentSymbolName(); ok {
		t.Error("a finished iterator has a current name")
	}

	if it.ResetState(4) {
		t.Error("ResetState(4) returned true")
	}
	if !it.ResetState(3) {
		t.Fatal("ResetState(3) returned false")
	}
	if got := slices.Collect(it.Names()); !slices.Equal(got, []string{"identifier"}) {
		t.Errorf("Names() of state 3 = %q, want identifier", got)
	}

	other := testLanguage(15)
	if it.Reset(other, 4) {
		t.Error("Reset(other, 4) returned true")
	}
	if it.Language() != l {
		t.Error("a failed Reset changed the language")
	}
	if !it.Reset(other, 1) || it.Language() != other {
		t.Fatal("Reset(other, 1) did not move the iterator to the other language")
	}
	if got := slices.Collect(it.Symbols()); !slices.Equal(got, []Symbol{testSymEnd, testSymPlus}) {
		t.Errorf("Symbols() of state 1 = %v", got)
	}
}

func TestLanguageLexModes(t *testing.T) {
	l := testLanguage(15)
	if got := l.lexModeForState(2); got != (abi.LexerMode{LexState: 3, ReservedWordSetID: 1}) {
		t.Errorf("lexModeForState(2) = %+v", got)
	}
	if !l.isReservedWord(2, testSymIdentifier) {
		t.Error("isReservedWord(2, identifier) = false, want true")
	}
	if l.isReservedWord(2, testSymPlus) || l.isReservedWord(0, testSymIdentifier) {
		t.Error("isReservedWord is true for a word that is not reserved")
	}

	// ABI 14 has no reserved words
	l = testLanguage(14)
	if got := l.lexModeForState(2); got != (abi.LexerMode{LexState: 3}) {
		t.Errorf("lexModeForState(2) of ABI 14 = %+v", got)
	}
	if l.isReservedWord(2, testSymIdentifier) {
		t.Error("isReservedWord(2, identifier) of ABI 14 = true, want false")
	}
}

func TestLanguageSymbols(t *testing.T) {
	l := testLanguage(15)
	if got := l.SymbolCount(); got != 7 {
		t.Errorf("SymbolCount() = %d, want 7", got)
	}
	if got := l.StateCount(); got != 4 {
		t.Errorf("StateCount() = %d, want 4", got)
	}
	names := map[Symbol]string{
		testSymEnd:            "end",
		testSymIdentifier:     "identifier",
		testSymAlias:          "alias_name",
		builtinSymError:       "ERROR",
		builtinSymErrorRepeat: "_ERROR",
		7:                     "",
	}
	for s, want := range names {
		if got := l.SymbolName(s); got != want {
			t.Errorf("SymbolName(%d) = %q, want %q", s, got, want)
		}
	}
	types := []SymbolType{
		SymbolAuxiliary, SymbolAnonymous, SymbolRegular, SymbolRegular,
		SymbolSupertype, SymbolAuxiliary, SymbolRegular,
	}
	for s, want := range types {
		if got := l.SymbolType(Symbol(s)); got != want {
			t.Errorf("SymbolType(%d) = %v, want %v", s, got, want)
		}
	}
	if got := l.SymbolType(builtinSymError); got != SymbolRegular {
		t.Errorf("SymbolType(ERROR) = %v, want %v", got, SymbolRegular)
	}
	if got := l.SymbolType(builtinSymErrorRepeat); got != SymbolAuxiliary {
		t.Errorf("SymbolType(_ERROR) = %v, want %v", got, SymbolAuxiliary)
	}
	if got := l.publicSymbol(builtinSymError); got != builtinSymError {
		t.Errorf("publicSymbol(ERROR) = %d", got)
	}
	if got := l.publicSymbol(testSymIdentifier); got != testSymIdentifier {
		t.Errorf("publicSymbol(identifier) = %d", got)
	}
}

func TestLanguageSymbolForName(t *testing.T) {
	l := testLanguage(15)
	tests := []struct {
		name   string
		named  bool
		want   Symbol
		wantOK bool
	}{
		{"identifier", true, testSymIdentifier, true},
		{"identifier", false, 0, false},
		{"+", false, testSymPlus, true},
		{"+", true, 0, false},
		{"ERROR", true, builtinSymError, true},
		{"ERROR", false, 0, false},
		// strncmp compares only the length of the name, as in C
		{"ERR", true, builtinSymError, true},
		{"", true, builtinSymError, true},
		// a hidden supertype is found, and another hidden symbol is not
		{"_statement", true, testSymStatement, true},
		{"program_repeat1", false, 0, false},
		{"end", true, 0, false},
		{"identifie", true, 0, false},
		{"identifierx", true, 0, false},
		{"alias_name", true, testSymAlias, true},
		// a NUL ends a C string
		{"identifier\x00x", true, testSymIdentifier, true},
	}
	for _, test := range tests {
		got, ok := l.SymbolForName(test.name, test.named)
		if got != test.want || ok != test.wantOK {
			t.Errorf("SymbolForName(%q, %t) = %d, %t, want %d, %t", test.name, test.named, got, ok, test.want, test.wantOK)
		}
	}
}

func TestLanguageFields(t *testing.T) {
	l := testLanguage(15)
	if got := l.FieldCount(); got != 2 {
		t.Errorf("FieldCount() = %d, want 2", got)
	}
	for id, want := range []string{"", "left", "right", ""} {
		if got := l.FieldName(FieldID(id)); got != want {
			t.Errorf("FieldName(%d) = %q, want %q", id, got, want)
		}
	}
	tests := []struct {
		name   string
		want   FieldID
		wantOK bool
	}{
		{"left", 1, true},
		{"right", 2, true},
		{"lef", 0, false},
		{"leftx", 0, false},
		{"", 0, false},
		// strncmp returns -1, so the loop stops at the first field
		{"k", 0, false},
		{"z", 0, false},
	}
	for _, test := range tests {
		got, ok := l.FieldForName(test.name)
		if got != test.want || ok != test.wantOK {
			t.Errorf("FieldForName(%q) = %d, %t, want %d, %t", test.name, got, ok, test.want, test.wantOK)
		}
	}

	tables := testTables()
	tables.FieldCount = 0
	l = NewLanguage(tables)
	if got := l.FieldName(0); got != "" {
		t.Errorf("FieldName(0) with no fields = %q", got)
	}
	if got := l.fieldMap(1); got != nil {
		t.Errorf("fieldMap(1) with no fields = %v, want nil", got)
	}
}

func TestLanguageProductions(t *testing.T) {
	l := testLanguage(15)
	if got := l.fieldMap(0); len(got) != 0 {
		t.Errorf("fieldMap(0) = %v, want none", got)
	}
	if got := l.fieldMap(1); !slices.Equal(got, l.tables.FieldMapEntries) {
		t.Errorf("fieldMap(1) = %v", got)
	}
	if got := l.aliasSequence(0); got != nil {
		t.Errorf("aliasSequence(0) = %v, want nil", got)
	}
	// production 1 is the last, so its sequence runs to the end of the table
	if got := l.aliasSequence(1); !slices.Equal(got, []uint16{0, uint16(testSymAlias), 0}) {
		t.Errorf("aliasSequence(1) = %v", got)
	}
	if got := l.aliasAt(0, 1); got != 0 {
		t.Errorf("aliasAt(0, 1) = %d, want 0", got)
	}
	if got := l.aliasAt(1, 1); got != testSymAlias {
		t.Errorf("aliasAt(1, 1) = %d, want %d", got, testSymAlias)
	}
	aliases := map[Symbol][]uint16{
		testSymIdentifier: {uint16(testSymIdentifier)},
		testSymExpression: {uint16(testSymExpression), uint16(testSymAlias)},
		testSymRepeat:     {uint16(testSymRepeat)},
	}
	for s, want := range aliases {
		if got := l.aliasesForSymbol(s); !slices.Equal(got, want) {
			t.Errorf("aliasesForSymbol(%d) = %v, want %v", s, got, want)
		}
	}
}

func TestLanguageStates(t *testing.T) {
	l := testLanguage(15)
	for state, want := range []bool{true, true, true, false} {
		if got := l.stateIsPrimary(StateID(state)); got != want {
			t.Errorf("stateIsPrimary(%d) = %t, want %t", state, got, want)
		}
	}
	// ABI 13 has no primary states
	if !testLanguage(13).stateIsPrimary(3) {
		t.Error("stateIsPrimary(3) of ABI 13 = false, want true")
	}
	if got := l.enabledExternalTokens(0); got != nil {
		t.Errorf("enabledExternalTokens(0) = %v, want nil", got)
	}
	if got := l.enabledExternalTokens(1); !slices.Equal(got, []bool{true, false}) {
		t.Errorf("enabledExternalTokens(1) = %v", got)
	}
	if got := l.enabledExternalTokens(2); !slices.Equal(got, []bool{true, true}) {
		t.Errorf("enabledExternalTokens(2) = %v", got)
	}
}

func TestLanguageVersions(t *testing.T) {
	l := testLanguage(15)
	if got := l.ABIVersion(); got != 15 {
		t.Errorf("ABIVersion() = %d, want 15", got)
	}
	if got := l.Name(); got != "test" {
		t.Errorf("Name() = %q, want test", got)
	}
	if got, ok := l.Metadata(); !ok || got != (LanguageMetadata{1, 2, 3}) {
		t.Errorf("Metadata() = %v, %t", got, ok)
	}
	if got := l.Supertypes(); !slices.Equal(got, []Symbol{testSymStatement}) {
		t.Errorf("Supertypes() = %v", got)
	}
	if got := l.Subtypes(testSymStatement); !slices.Equal(got, []Symbol{testSymIdentifier, testSymExpression}) {
		t.Errorf("Subtypes(_statement) = %v", got)
	}
	for _, s := range []Symbol{testSymExpression, 7, builtinSymError} {
		if got := l.Subtypes(s); got != nil {
			t.Errorf("Subtypes(%d) = %v, want nil", s, got)
		}
	}
	// a change of the result does not change the language
	l.Supertypes()[0] = 0
	if got := l.Supertypes(); got[0] != testSymStatement {
		t.Error("a change of the result of Supertypes changed the language")
	}

	// ABI 14 has no name, no metadata and no supertypes
	l = testLanguage(14)
	if got := l.Name(); got != "" {
		t.Errorf("Name() of ABI 14 = %q, want none", got)
	}
	if _, ok := l.Metadata(); ok {
		t.Error("Metadata() of ABI 14 returned true")
	}
	if got := l.Supertypes(); got != nil {
		t.Errorf("Supertypes() of ABI 14 = %v", got)
	}
	if got := l.Subtypes(testSymStatement); got != nil {
		t.Errorf("Subtypes() of ABI 14 = %v", got)
	}
}

func TestWriteSymbolAsDotString(t *testing.T) {
	tables := testTables()
	tables.SymbolNames[testSymAlias] = "a\"b\\c\nd\te\x00f"
	l := NewLanguage(tables)
	var b strings.Builder
	l.writeSymbolAsDotString(&b, testSymAlias)
	if got, want := b.String(), `a\"b\\c\nd\te`; got != want {
		t.Errorf("writeSymbolAsDotString wrote %q, want %q", got, want)
	}
}

func TestStrncmp(t *testing.T) {
	tests := []struct {
		a, b string
		n    int
		want int
	}{
		{"abc", "abc", 3, 0},
		{"abc", "abd", 3, 'c' - 'd'},
		{"abc", "abd", 2, 0},
		{"ab", "abc", 3, -'c'},
		{"abc", "ab", 3, 'c'},
		{"ab\x00x", "ab\x00y", 4, 0},
		{"", "", 5, 0},
		{"x", "y", 0, 0},
	}
	for _, test := range tests {
		if got := strncmp(test.a, test.b, test.n); got != test.want {
			t.Errorf("strncmp(%q, %q, %d) = %d, want %d", test.a, test.b, test.n, got, test.want)
		}
	}
}

func TestEnumStrings(t *testing.T) {
	for want, v := range map[string]interface{ String() string }{
		"regular":    SymbolRegular,
		"anonymous":  SymbolAnonymous,
		"supertype":  SymbolSupertype,
		"auxiliary":  SymbolAuxiliary,
		"fresh":      lookaheadFresh,
		"positioned": lookaheadPositioned,
		"done":       lookaheadDone,
	} {
		if got := v.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
	if got := SymbolType(9).String(); got != "unknown" {
		t.Errorf("SymbolType(9).String() = %q", got)
	}
	if got := lookaheadPhase(9).String(); got != "unknown" {
		t.Errorf("lookaheadPhase(9).String() = %q", got)
	}
}
