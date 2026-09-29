package generate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestStartingCharacters is test_starting_characters in token_conflicts.rs.
func TestStartingCharacters(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	t0 := pat(pool, "[a-f]1|0x\\d")
	t1 := pat(pool, "d*ef")
	vars := []LexicalToken{
		{Name: pool.Intern("token_0"), Kind: VariableNamed, Root: t0},
		{Name: pool.Intern("token_1"), Kind: VariableNamed, Root: t1},
	}
	grammar, err := expandTokens(pool, vars, nil)
	if err != nil {
		t.Fatal(err)
	}

	tokenMap := NewTokenConflictMap(grammar, nil)

	if expected := (CharacterSet{}).AddRange('a', 'f').AddChar('0'); !tokenMap.startingCharsByIndex[0].Equal(expected) {
		t.Errorf("expected the starting characters of token_0 to be %v, got %v", expected, tokenMap.startingCharsByIndex[0])
	}
	if expected := (CharacterSet{}).AddRange('d', 'e'); !tokenMap.startingCharsByIndex[1].Equal(expected) {
		t.Errorf("expected the starting characters of token_1 to be %v, got %v", expected, tokenMap.startingCharsByIndex[1])
	}
}

// TestTokenConflicts is test_token_conflicts in token_conflicts.rs.
func TestTokenConflicts(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	inTok := str(pool, "in")
	ident := pat(pool, "\\w+")
	instanceof := str(pool, "instanceof")
	vars := []LexicalToken{
		{Name: pool.Intern("in"), Kind: VariableNamed, Root: inTok},
		{Name: pool.Intern("identifier"), Kind: VariableNamed, Root: ident},
		{Name: pool.Intern("instanceof"), Kind: VariableNamed, Root: instanceof},
	}
	grammar, err := expandTokens(pool, vars, nil)
	if err != nil {
		t.Fatal(err)
	}

	v := func(name string) int { return indexOfVar(t, pool, grammar, name) }
	single := func(name string) TokenSet {
		return TokenSetFrom(slices.Values([]Symbol{TerminalSymbol(v(name))}))
	}

	tokenMap := NewTokenConflictMap(grammar, []TokenSet{
		single("identifier"),
		single("in"),
		single("identifier"),
	})

	// Given the string "in", the `in` token is preferrred over the
	// `identifier` token
	if !tokenMap.DoesMatchSameString(v("in"), v("identifier")) {
		t.Error("expected in to match the same string as identifier")
	}
	if tokenMap.DoesMatchSameString(v("identifier"), v("in")) {
		t.Error("expected identifier not to match the same string as in")
	}

	// Depending on what character follows, the string "in" may be treated as
	// part of an `identifier` token
	if !tokenMap.DoesConflict(v("identifier"), v("in")) {
		t.Error("expected identifier to conflict with in")
	}

	// Depending on what character follows, the string "instanceof" may be
	// treated as part of an `identifier` token
	if !tokenMap.DoesConflict(v("identifier"), v("instanceof")) {
		t.Error("expected identifier to conflict with instanceof")
	}
	if !tokenMap.DoesConflict(v("instanceof"), v("in")) {
		t.Error("expected instanceof to conflict with in")
	}
}

// TestTokenConflictsWithSeparators is test_token_conflicts_with_separators in
// token_conflicts.rs.
func TestTokenConflictsWithSeparators(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	sep := pat(pool, "\\s")
	x := str(pool, "x")
	newline := str(pool, "\n")
	vars := []LexicalToken{
		{Name: pool.Intern("x"), Kind: VariableNamed, Root: x},
		{Name: pool.Intern("newline"), Kind: VariableNamed, Root: newline},
	}
	grammar, err := expandTokens(pool, vars, []RuleID{sep})
	if err != nil {
		t.Fatal(err)
	}

	v := func(name string) int { return indexOfVar(t, pool, grammar, name) }

	tokenMap := NewTokenConflictMap(grammar, make([]TokenSet, 4))

	if !tokenMap.DoesConflict(v("newline"), v("x")) {
		t.Error("expected newline to conflict with x")
	}
	if tokenMap.DoesConflict(v("x"), v("newline")) {
		t.Error("expected x not to conflict with newline")
	}
}

// TestTokenConflictsWithOpenEndedTokens is
// test_token_conflicts_with_open_ended_tokens in token_conflicts.rs.
func TestTokenConflictsWithOpenEndedTokens(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	sep := pat(pool, "\\s")
	x := str(pool, "x")
	anything := pool.Prec(precInt(-1), pat(pool, ".*"))
	vars := []LexicalToken{
		{Name: pool.Intern("x"), Kind: VariableNamed, Root: x},
		{Name: pool.Intern("anything"), Kind: VariableNamed, Root: anything},
	}
	grammar, err := expandTokens(pool, vars, []RuleID{sep})
	if err != nil {
		t.Fatal(err)
	}

	v := func(name string) int { return indexOfVar(t, pool, grammar, name) }

	tokenMap := NewTokenConflictMap(grammar, make([]TokenSet, 4))

	if !tokenMap.DoesMatchShorterOrLonger(v("anything"), v("x")) {
		t.Error("expected anything to match a shorter or a longer string than x")
	}
	if tokenMap.DoesMatchShorterOrLonger(v("x"), v("anything")) {
		t.Error("expected x not to match a shorter or a longer string than anything")
	}
}

// indexOfVar returns the index of the lexical variable with a name.
//
// indexOfVar is index_of_var in token_conflicts.rs.
func indexOfVar(t *testing.T, pool *RulePool, grammar *LexicalGrammar, name string) int {
	t.Helper()
	i := slices.IndexFunc(grammar.Variables, func(v LexicalVariable) bool { return pool.Resolve(v.Name) == name })
	if i < 0 {
		t.Fatalf("no lexical variable %q", name)
	}
	return i
}

// lexTablesDump returns the text of the lex tables and of the lex state of
// each parse state, so that two runs can be compared.
func lexTablesDump(tables *LexTables, parseTable *ParseTable[ActionListID]) string {
	var b strings.Builder
	dumpTable := func(name string, table *LexTable) {
		fmt.Fprintf(&b, "%s\n", name)
		for _, state := range table.States {
			fmt.Fprintf(&b, "%v %v %v %v", state.HasAcceptAction, state.AcceptAction, state.HasEOFAction, state.EOFAction)
			for _, advance := range state.AdvanceActions {
				fmt.Fprintf(&b, " %v:%v", advance.Chars.ranges, advance.Action)
			}
			b.WriteByte('\n')
		}
	}
	dumpTable("main", &tables.MainLexTable)
	dumpTable("keywords", &tables.KeywordLexTable)
	for _, set := range tables.LargeCharacterSets {
		fmt.Fprintf(&b, "large %v %v %v\n", set.HasSymbol, set.Symbol, set.Chars.ranges)
	}
	for i := range parseTable.States {
		fmt.Fprintf(&b, "%d ", parseTable.States[i].LexStateID)
	}
	return b.String()
}

// lexTablesForTest builds the lex tables of a grammar for a parse table made
// for the test: state 0 holds every terminal and the end of the input, and
// state i+1 holds terminal i and the end of the input.
func lexTablesForTest(prepared *PreparedGrammar) (*LexTables, *ParseTable[ActionListID], *TokenConflictMap) {
	syntaxGrammar, lexicalGrammar := &prepared.SyntaxGrammar, &prepared.LexicalGrammar
	keyMap := NewItemKeyMap(syntaxGrammar, prepared.StrPool)
	builder := NewParseItemSetBuilder(syntaxGrammar, lexicalGrammar, &prepared.Inlines, keyMap)
	tokenConflictMap := NewTokenConflictMap(lexicalGrammar, getFollowingTokens(syntaxGrammar, lexicalGrammar, builder))

	n := len(lexicalGrammar.Variables)
	parseTable := &ParseTable[ActionListID]{States: make([]ParseState[ActionListID], n+1)}
	for i := range n {
		parseTable.States[0].TerminalEntries.Insert(TerminalSymbol(i), 0)
		parseTable.States[i+1].TerminalEntries.Insert(TerminalSymbol(i), 0)
	}
	for i := range parseTable.States {
		parseTable.States[i].TerminalEntries.Insert(SymbolEndValue, 0)
	}
	coincidentTokenIndex := NewCoincidentTokenIndex(parseTable, lexicalGrammar, syntaxGrammar.WordToken, syntaxGrammar.HasWordToken)
	var keywords TokenSet
	tables := BuildLexTable(parseTable, syntaxGrammar, lexicalGrammar, &keywords, coincidentTokenIndex, tokenConflictMap)
	return &tables, parseTable, tokenConflictMap
}

// formatLexCharForTest returns the text of a character in parser.c, as
// Generator::add_character in render.rs writes it.
func formatLexCharForTest(c rune) string {
	switch c {
	case '\'':
		return `'\''`
	case '\\':
		return `'\\'`
	case '\f':
		return `'\f'`
	case '\n':
		return `'\n'`
	case '\t':
		return `'\t'`
	case '\r':
		return `'\r'`
	case 0:
		return "0"
	}
	if c == ' ' || (c > ' ' && c < 0x7f) {
		return "'" + string(c) + "'"
	}
	return fmt.Sprintf("0x%02x", c)
}

// largeCharacterSetsForTest returns the text of each large character set of
// a named terminal, as Generator::add_character_set in render.rs writes it,
// by the name of its constant.
func largeCharacterSetsForTest(sets []LargeCharacterSet, lexicalGrammar *LexicalGrammar, strPool *StrPool) map[string]string {
	result := make(map[string]string)
	for ix, set := range sets {
		count := 1
		for _, prev := range sets[:ix] {
			if prev.HasSymbol == set.HasSymbol && prev.Symbol == set.Symbol {
				count++
			}
		}
		name := fmt.Sprintf("extras_character_set_%d", count)
		if set.HasSymbol {
			name = fmt.Sprintf("sym_%s_character_set_%d", strPool.Resolve(lexicalGrammar.Variables[set.Symbol.index].Name), count)
		}
		var b strings.Builder
		ix := 0
		for first, last := range set.Chars.Ranges() {
			switch {
			case ix%8 != 0:
				b.WriteByte(' ')
			case ix > 0:
				b.WriteString("\n  ")
			default:
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "{%s, %s},", formatLexCharForTest(first), formatLexCharForTest(last))
			ix++
		}
		result[name] = b.String()
	}
	return result
}

// TestBuildLexTableOnEveryTestGrammar builds the token conflict map and the
// lex tables of each test grammar that PrepareGrammar accepts, for a parse
// table made for the test. The parse table builder is not ported yet. The
// test makes sure that the tables are the same on each run, so that no order
// of a Go map reaches them, and that each state id is in range. The large
// character sets need no parse table, and the test compares each one that a
// golden parser.c holds with the set that BuildLexTable gives.
func TestBuildLexTableOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	constantRE := regexp.MustCompile(`(?s)static const TSCharacterRange (\w+)\[\] = \{\n(.*?)\n\};`)
	goldenSets := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		prepare := func() *PreparedGrammar {
			g, err := ParseGrammar(b, new([]Diagnostic))
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			prepared, err := PrepareGrammar(g, new([]Diagnostic))
			if err != nil {
				return nil
			}
			return prepared
		}
		prepared := prepare()
		if prepared == nil {
			continue
		}
		tables, parseTable, _ := lexTablesForTest(prepared)
		dump := lexTablesDump(tables, parseTable)
		for range 3 {
			again := prepare()
			tables2, parseTable2, _ := lexTablesForTest(again)
			if dump2 := lexTablesDump(tables2, parseTable2); dump2 != dump {
				t.Errorf("%s: the lex tables differ between two runs", f)
				break
			}
		}

		for _, table := range []*LexTable{&tables.MainLexTable, &tables.KeywordLexTable} {
			for _, state := range table.States {
				if state.HasEOFAction && int(state.EOFAction.State) >= len(table.States) {
					t.Errorf("%s: the action at the end of the input goes to state %d of %d", f, state.EOFAction.State, len(table.States))
				}
				for _, advance := range state.AdvanceActions {
					if int(advance.Action.State) >= len(table.States) {
						t.Errorf("%s: an advance action goes to state %d of %d", f, advance.Action.State, len(table.States))
					}
				}
			}
		}
		for i := range parseTable.States {
			if id := parseTable.States[i].LexStateID; int(id) >= len(tables.MainLexTable.States) {
				t.Errorf("%s: parse state %d has lex state %d of %d", f, i, id, len(tables.MainLexTable.States))
			}
		}

		golden, err := os.ReadFile(filepath.Join(filepath.Dir(f), "abi15", "parser.c"))
		if errors.Is(err, fs.ErrNotExist) {
			// The upstream tool rejects the grammar in a later step, so it
			// has an error golden and no parser.c.
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		sets := largeCharacterSetsForTest(tables.LargeCharacterSets, &prepared.LexicalGrammar, prepared.StrPool)
		for _, m := range constantRE.FindAllStringSubmatch(string(golden), -1) {
			goldenSets++
			actual, ok := sets[m[1]]
			if !ok {
				t.Errorf("%s: BuildLexTable gives no large character set %s", f, m[1])
				continue
			}
			if actual != m[2] {
				t.Errorf("%s: expected the large character set %s to be\n%s\ngot\n%s", f, m[1], m[2], actual)
			}
		}
	}
	if goldenSets == 0 {
		t.Error("expected a large character set in a golden parser.c")
	}
}

// TestTokenConflictsGiveTheKeywordCandidates finds the keyword candidates of
// the test grammar reserved_words with the token conflict map, as the first
// two steps of identifyKeywords do. The expected candidates are the ones that
// `tree-sitter generate --log` of upstream prints for the grammar, which
// logs the candidates and not only the keywords that are left.
func TestTokenConflictsGiveTheKeywordCandidates(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("testdata", "reserved_words", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGrammar(b, new([]Diagnostic))
	if err != nil {
		t.Fatal(err)
	}
	prepared := prepareForTest(t, g)
	_, _, tokenConflictMap := lexTablesForTest(prepared)
	lexicalGrammar := &prepared.LexicalGrammar
	if !prepared.SyntaxGrammar.HasWordToken {
		t.Fatal("expected the grammar to have a word token")
	}
	wordTokenIndex := int(prepared.SyntaxGrammar.WordToken.index)

	cursor := NewNfaCursor(&lexicalGrammar.Nfa, nil)
	var candidates []int
	for i, variable := range lexicalGrammar.Variables {
		cursor.Reset([]uint32{variable.StartState})
		if allCharsAreAlphabetical(cursor) &&
			tokenConflictMap.DoesMatchSameString(i, wordTokenIndex) &&
			!tokenConflictMap.DoesMatchDifferentString(i, wordTokenIndex) {
			candidates = append(candidates, i)
		}
	}
	var keywords []string
	for _, token := range candidates {
		if !slices.ContainsFunc(candidates, func(other int) bool {
			return other != token && tokenConflictMap.DoesMatchSameString(other, token)
		}) {
			keywords = append(keywords, prepared.StrPool.Resolve(lexicalGrammar.Variables[token].Name))
		}
	}
	if expected := []string{"var", "if", "while", "get"}; !slices.Equal(keywords, expected) {
		t.Errorf("expected the keyword candidates %v, got %v", expected, keywords)
	}
}
