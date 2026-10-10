package generate

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestValidatePrecedencesWithUndeclaredPrecedences is
// test_validate_precedences_with_undeclared_precedences in prepare_grammar.rs.
func TestValidatePrecedencesWithUndeclaredPrecedences(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	// v1: seq(prec_left('b', "w"), prec('c', "x"))
	v1 := pool.Seq([]RuleID{
		precLeftNamed(pool, "b", leaf(pool, "w")),
		precNamed(pool, "c", leaf(pool, "x")),
	})
	// v2: repeat(choice(prec_left('omg', "y"), prec('c', "z")))
	v2 := pool.Repeat(pool.Choice([]RuleID{
		precLeftNamed(pool, "omg", leaf(pool, "y")),
		precNamed(pool, "c", leaf(pool, "z")),
	}))
	g := &InputGrammar{
		Pool:      pool,
		Variables: []Variable{{Name: pool.Intern("v1"), Root: v1}, {Name: pool.Intern("v2"), Root: v2}},
		PrecedenceOrderings: [][]PrecedenceEntry{
			{nameEntry(pool, "a"), nameEntry(pool, "b")},
			{nameEntry(pool, "b"), nameEntry(pool, "c"), nameEntry(pool, "d")},
		},
	}
	err := validatePrecedences(g)
	ue, ok := errors.AsType[*UndeclaredPrecedenceError](err)
	if !ok || *ue != (UndeclaredPrecedenceError{Precedence: "omg", Rule: "v2"}) {
		t.Errorf("expected the error that omg is not declared in v2, got: %v", err)
	}
}

// TestValidatePrecedencesWithConflictingOrder is
// test_validate_precedences_with_conflicting_order in prepare_grammar.rs.
func TestValidatePrecedencesWithConflictingOrder(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	g := &InputGrammar{
		Pool: pool,
		PrecedenceOrderings: [][]PrecedenceEntry{
			{nameEntry(pool, "a"), nameEntry(pool, "b")},
			{nameEntry(pool, "b"), nameEntry(pool, "c"), nameEntry(pool, "a")},
		},
	}
	err := validatePrecedences(g)
	ce, ok := errors.AsType[*ConflictingPrecedenceOrderingError](err)
	if !ok || *ce != (ConflictingPrecedenceOrderingError{Precedence1: "'a'", Precedence2: "'b'"}) {
		t.Errorf("expected the error that 'a' and 'b' conflict, got: %v", err)
	}
}

// TestValidatePrecedencesKeepsTheSwappedEntry makes sure of a fault that the
// port keeps. After upstream swaps two entries, the first entry stays
// swapped for the rest of its list, so later pairs are made with it. The
// upstream tool at the base commit gives the same results.
func TestValidatePrecedencesKeepsTheSwappedEntry(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	// The list [c, a, b] swaps c with a, and then pairs a with b instead of
	// c with b. So the list [b, c] finds no conflict, although c comes before
	// b in the first list.
	g := &InputGrammar{
		Pool: pool,
		PrecedenceOrderings: [][]PrecedenceEntry{
			{nameEntry(pool, "c"), nameEntry(pool, "a"), nameEntry(pool, "b")},
			{nameEntry(pool, "b"), nameEntry(pool, "c")},
		},
	}
	if err := validatePrecedences(g); err != nil {
		t.Errorf("expected no error, as upstream gives, got: %v", err)
	}

	// The list [b, a] conflicts with the pair of a and b, and upstream
	// reports it.
	g.PrecedenceOrderings[1] = []PrecedenceEntry{nameEntry(pool, "b"), nameEntry(pool, "a")}
	err := validatePrecedences(g)
	ce, ok := errors.AsType[*ConflictingPrecedenceOrderingError](err)
	if !ok || *ce != (ConflictingPrecedenceOrderingError{Precedence1: "'a'", Precedence2: "'b'"}) {
		t.Errorf("expected the error that 'a' and 'b' conflict, got: %v", err)
	}
}

// TestValidateIndirectRecursion is test_validate_indirect_recursion in
// prepare_grammar.rs.
func TestValidateIndirectRecursion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		build    func(p *RulePool) []Variable
		expected []string
	}{
		{
			"a -> b -> a",
			func(p *RulePool) []Variable {
				bRef := named(p, "b")
				x := leaf(p, "x")
				a := p.Choice([]RuleID{bRef, x})
				aRef := named(p, "a")
				b := p.Prec(Precedence{Kind: PrecedenceInteger, Integer: 1}, aRef)
				return []Variable{{Name: p.Intern("a"), Root: a}, {Name: p.Intern("b"), Root: b}}
			},
			[]string{"a", "b", "a"},
		},
		{
			"a direct reference to itself is allowed",
			func(p *RulePool) []Variable {
				a := named(p, "a")
				return []Variable{{Name: p.Intern("a"), Root: a}}
			},
			nil,
		},
		{
			"b -> c -> d -> b, from a start rule outside the cycle",
			func(p *RulePool) []Variable {
				x := leaf(p, "x")
				cRef := named(p, "c")
				dRef := named(p, "d")
				bRef := named(p, "b")
				return []Variable{
					{Name: p.Intern("a"), Root: x},
					{Name: p.Intern("b"), Root: cRef},
					{Name: p.Intern("c"), Root: dRef},
					{Name: p.Intern("d"), Root: bRef},
				}
			},
			[]string{"b", "c", "d", "b"},
		},
		{
			"b -> c -> b, through seq(optional('|'), $.b, repeat(seq('|', $.b)))",
			func(p *RulePool) []Variable {
				a := p.Seq([]RuleID{named(p, "b"), leaf(p, ":=")})
				b := p.Choice([]RuleID{leaf(p, "x"), named(p, "c")})
				leadingPipe := p.Choice([]RuleID{leaf(p, "|"), p.Blank()})
				bRef := named(p, "b")
				item := p.Seq([]RuleID{leaf(p, "|"), named(p, "b")})
				rest := p.Choice([]RuleID{p.Repeat(item), p.Blank()})
				c := p.Seq([]RuleID{leadingPipe, bRef, rest})
				return []Variable{
					{Name: p.Intern("a"), Root: a},
					{Name: p.Intern("b"), Root: b},
					{Name: p.Intern("c"), Root: c},
				}
			},
			[]string{"b", "c", "b"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := PrepareGrammar(buildGrammar(test.build), new([]Diagnostic))
			if test.expected == nil {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			ie, ok := errors.AsType[*IndirectRecursionError](err)
			if !ok || !slices.Equal(ie.Symbols, test.expected) {
				t.Errorf("expected the cycle %v, got: %v", test.expected, err)
			}
		})
	}
}

// TestTokenBodySharedWithSyntax is test_token_body_shared_with_syntax in
// prepare_grammar.rs.
func TestTokenBodySharedWithSyntax(t *testing.T) {
	t.Parallel()
	g := buildGrammar(func(p *RulePool) []Variable {
		shared := p.Seq([]RuleID{leaf(p, "a"), leaf(p, "b")})
		token := p.Token(shared)
		program := p.Seq([]RuleID{named(p, "t"), named(p, "x")})
		return []Variable{
			{Name: p.Intern("program"), Root: program},
			{Name: p.Intern("t"), Root: token},
			{Name: p.Intern("x"), Root: shared},
		}
	})
	prepared := prepareForTest(t, g)
	expectSyntaxVariables(t, prepared, []syntaxNameKind{{"program", VariableNamed}, {"x", VariableNamed}})
	expectLexicalStates(t, prepared, []lexicalState{
		{"t", VariableNamed, 0, 2},
		{"a", VariableAnonymous, 2, 4},
		{"b", VariableAnonymous, 2, 6},
	})
}

// TestTokenBodySharedWithSyntaxReversed is
// test_token_body_shared_with_syntax_reversed in prepare_grammar.rs.
func TestTokenBodySharedWithSyntaxReversed(t *testing.T) {
	t.Parallel()
	g := buildGrammar(func(p *RulePool) []Variable {
		shared := p.Seq([]RuleID{leaf(p, "a"), leaf(p, "b")})
		token := p.Token(shared)
		program := p.Seq([]RuleID{named(p, "x"), named(p, "t")})
		return []Variable{
			{Name: p.Intern("program"), Root: program},
			{Name: p.Intern("x"), Root: shared},
			{Name: p.Intern("t"), Root: token},
		}
	})
	prepared := prepareForTest(t, g)
	expectSyntaxVariables(t, prepared, []syntaxNameKind{{"program", VariableNamed}, {"x", VariableNamed}})
	expectLexicalStates(t, prepared, []lexicalState{
		{"a", VariableAnonymous, 2, 1},
		{"b", VariableAnonymous, 2, 3},
		{"t", VariableNamed, 0, 6},
	})
}

// TestSeparatorBodySharedWithSyntax is test_separator_body_shared_with_syntax
// in prepare_grammar.rs.
func TestSeparatorBodySharedWithSyntax(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	shared := pool.Seq([]RuleID{leaf(pool, "a"), leaf(pool, "b")})
	program := pool.Intern("program")
	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: program, Root: shared}}, ExtraRoots: []RuleID{shared}}
	prepared := prepareForTest(t, g)
	expectSyntaxVariables(t, prepared, []syntaxNameKind{{"program", VariableNamed}})
	expectLexicalStates(t, prepared, []lexicalState{
		{"a", VariableAnonymous, 2, 5},
		{"b", VariableAnonymous, 2, 11},
	})
	if len(prepared.SyntaxGrammar.ExtraSymbols) != 0 {
		t.Errorf("expected no extra symbols, got: %v", prepared.SyntaxGrammar.ExtraSymbols)
	}
}

// TestSharedExtraSymbolIsRenumberedOnce is
// test_shared_extra_symbol_is_renumbered_once in prepare_grammar.rs.
func TestSharedExtraSymbolIsRenumberedOnce(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	targetRef := named(pool, "target")
	program := pool.Seq([]RuleID{named(pool, "kw"), named(pool, "keep"), targetRef})
	kw := leaf(pool, "keyword")
	keep := pool.Seq([]RuleID{leaf(pool, "k"), leaf(pool, "e")})
	target := pool.Seq([]RuleID{leaf(pool, "t"), leaf(pool, "g")})
	g := &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("program"), Root: program},
			{Name: pool.Intern("kw"), Root: kw},
			{Name: pool.Intern("keep"), Root: keep},
			{Name: pool.Intern("target"), Root: target},
		},
		ExtraRoots: []RuleID{targetRef},
	}
	prepared := prepareForTest(t, g)
	expectSyntaxVariables(t, prepared, []syntaxNameKind{
		{"program", VariableNamed},
		{"keep", VariableNamed},
		{"target", VariableNamed},
	})
	expectLexicalStates(t, prepared, []lexicalState{
		{"kw", VariableNamed, 2, 7},
		{"k", VariableAnonymous, 2, 9},
		{"e", VariableAnonymous, 2, 11},
		{"t", VariableAnonymous, 2, 13},
		{"g", VariableAnonymous, 2, 15},
	})
	if !slices.Equal(prepared.SyntaxGrammar.ExtraSymbols, []Symbol{NonTerminalSymbol(2)}) {
		t.Errorf("expected the extra symbols [nt2], got: %v", prepared.SyntaxGrammar.ExtraSymbols)
	}
}

// TestSharedSyntaxSubtreeIsRenumberedOnce is
// test_shared_syntax_subtree_is_renumbered_once in prepare_grammar.rs.
func TestSharedSyntaxSubtreeIsRenumberedOnce(t *testing.T) {
	t.Parallel()
	g := buildGrammar(func(p *RulePool) []Variable {
		pair := p.Seq([]RuleID{named(p, "thing"), leaf(p, "-")})
		itemB := p.Seq([]RuleID{pair, leaf(p, ";")})
		program := p.Seq([]RuleID{named(p, "kw"), named(p, "item_a"), named(p, "item_b")})
		kw := leaf(p, "keyword")
		thing := p.Seq([]RuleID{leaf(p, "t"), leaf(p, "u")})
		return []Variable{
			{Name: p.Intern("program"), Root: program},
			{Name: p.Intern("kw"), Root: kw},
			{Name: p.Intern("item_a"), Root: pair},
			{Name: p.Intern("item_b"), Root: itemB},
			{Name: p.Intern("thing"), Root: thing},
		}
	})
	prepared := prepareForTest(t, g)

	expected := []ProductionStep{
		PackProductionStep(NonTerminalSymbol(3), Precedence{}, AssociativityNone, Alias{}, false, 0, 0),
		PackProductionStep(TerminalSymbol(1), Precedence{}, AssociativityNone, Alias{}, false, 0, 0),
	}
	start, end := prepared.SyntaxGrammar.VariableProdIDs(1)
	if end-start != 1 {
		t.Fatalf("expected 1 production, got: %d", end-start)
	}
	production := prepared.SyntaxGrammar.Production(start)
	if !slices.Equal(production.Steps, expected) {
		t.Errorf("expected the steps %v, got: %v", expected, production.Steps)
	}
	if production.DynamicPrecedence != 0 {
		t.Errorf("expected no dynamic precedence, got: %d", production.DynamicPrecedence)
	}
}

// TestPrepareGrammarRegexErrors makes sure of the whole text of an error in
// the pattern of a token. The expected texts are what the upstream tool at
// the base commit writes under "Caused by:" for each grammar, without the
// indent.
func TestPrepareGrammarRegexErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern  string
		expected string
	}{
		{`a(`, "Error processing rule source_token1: regex parse error:\n    a(\n     ^\nerror: unclosed group\n"},
		{`(?ii)`, "Error processing rule source_token1: regex parse error:\n    (?ii)\n      ^^\nerror: duplicate flag\n"},
		{`é\\p{Nope}`, "Error processing rule source_token1: regex parse error:\n    é\\p{Nope}\n     ^^^^^^^^\nerror: Unicode property not found\n"},
	}
	for _, test := range tests {
		g, err := ParseGrammar([]byte(`{"name":"x","rules":{"source":{"type":"PATTERN","value":"`+test.pattern+`"}}}`), new([]Diagnostic))
		if err != nil {
			t.Fatalf("%q: %v", test.pattern, err)
		}
		_, err = PrepareGrammar(g, new([]Diagnostic))
		if err == nil || err.Error() != test.expected {
			t.Errorf("%q: expected %q, got: %v", test.pattern, test.expected, err)
		}
	}
}

// TestPrepareGrammarOnEveryTestGrammar runs PrepareGrammar on each test
// grammar. A grammar that it rejects must have the same error in its golden
// file.
func TestPrepareGrammarOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rejected []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := ParseGrammar(b, new([]Diagnostic))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		_, err = PrepareGrammar(g, new([]Diagnostic))
		if err == nil {
			continue
		}
		dir := filepath.Dir(f)
		rejected = append(rejected, filepath.Base(dir))
		golden, rerr := os.ReadFile(filepath.Join(dir, "abi15", "error.txt"))
		if rerr != nil {
			t.Errorf("%s: PrepareGrammar gives %q, and the grammar has no error golden: %v", f, err, rerr)
			continue
		}
		if !strings.Contains(string(golden), "Caused by:\n"+indent(err.Error())+"\n") {
			t.Errorf("%s: expected the error of the golden file, got: %q", f, err)
		}
	}
	expected := []string{
		"eof_misplaced",
		"eof_repeat_via_nullable_rule",
		"epsilon_rules",
		"indirect_recursion_in_transitions",
		"invisible_start_rule",
		"terminal_supertype",
	}
	if !slices.Equal(rejected, expected) {
		t.Errorf("expected PrepareGrammar to reject %v, it rejected %v", expected, rejected)
	}
}

// indent indents each line of an error as the upstream tool writes it under
// "Caused by:". An empty line gets the four spaces too.
func indent(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

// named returns a named symbol.
//
// named is named in prepare_grammar.rs.
func named(pool *RulePool, name string) RuleID {
	return pool.NamedSymbol(pool.Intern(name))
}

// leaf returns a string.
//
// leaf is leaf in prepare_grammar.rs.
func leaf(pool *RulePool, s string) RuleID {
	return pool.String(pool.Intern(s))
}

// precNamed returns content with a named precedence.
//
// precNamed is prec in prepare_grammar.rs.
func precNamed(pool *RulePool, name string, content RuleID) RuleID {
	return pool.Prec(Precedence{Kind: PrecedenceName, Name: pool.Intern(name)}, content)
}

// precLeftNamed returns content with a named precedence that associates to
// the left.
//
// precLeftNamed is prec_left in prepare_grammar.rs.
func precLeftNamed(pool *RulePool, name string, content RuleID) RuleID {
	return pool.PrecLeft(Precedence{Kind: PrecedenceName, Name: pool.Intern(name)}, content)
}

// nameEntry returns the entry of a named precedence.
//
// nameEntry is name_entry in prepare_grammar.rs.
func nameEntry(pool *RulePool, name string) PrecedenceEntry {
	return PrecedenceEntry{Kind: PrecedenceEntryName, Value: pool.Intern(name)}
}

// buildGrammar returns a grammar with the variables that build adds to a new
// pool.
//
// buildGrammar is build_grammar in prepare_grammar.rs.
func buildGrammar(build func(p *RulePool) []Variable) *InputGrammar {
	pool := NewRulePool()
	return &InputGrammar{Pool: pool, Variables: build(pool)}
}

// prepareForTest prepares a grammar, and fails the test on an error.
func prepareForTest(t *testing.T, g *InputGrammar) *PreparedGrammar {
	t.Helper()
	prepared, err := PrepareGrammar(g, new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return prepared
}

// syntaxNameKind is the name and the kind of a syntax variable.
type syntaxNameKind struct {
	name string
	kind VariableType
}

// expectSyntaxVariables fails the test unless the syntax variables have the
// expected names and kinds.
func expectSyntaxVariables(t *testing.T, prepared *PreparedGrammar, expected []syntaxNameKind) {
	t.Helper()
	var actual []syntaxNameKind
	for _, v := range prepared.SyntaxGrammar.Variables {
		actual = append(actual, syntaxNameKind{prepared.StrPool.Resolve(v.Name), v.Kind})
	}
	if !slices.Equal(actual, expected) {
		t.Errorf("expected the syntax variables %v, got: %v", expected, actual)
	}
}

// lexicalState is the name, the kind, the implicit precedence and the start
// state of a token.
type lexicalState struct {
	name               string
	kind               VariableType
	implicitPrecedence int32
	startState         uint32
}

// expectLexicalStates fails the test unless the tokens are the expected
// ones.
func expectLexicalStates(t *testing.T, prepared *PreparedGrammar, expected []lexicalState) {
	t.Helper()
	var actual []lexicalState
	for _, v := range prepared.LexicalGrammar.Variables {
		actual = append(actual, lexicalState{prepared.StrPool.Resolve(v.Name), v.Kind, v.ImplicitPrecedence, v.StartState})
	}
	if !slices.Equal(actual, expected) {
		t.Errorf("expected the tokens %v, got: %v", expected, actual)
	}
}
