package generate

import (
	"errors"
	"slices"
	"testing"
)

// TestExtraction is test_extraction in extract_tokens.rs.
func TestExtraction(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	// rule_0: repeat(seq("a", /b/, choice(rule_1, rule_2, token(repeat(choice("c", "d"))))))
	r0 := pool.Repeat(pool.Seq([]RuleID{
		str(pool, "a"),
		pat(pool, "b"),
		pool.Choice([]RuleID{
			nSym(pool, "rule_1"),
			nSym(pool, "rule_2"),
			pool.Token(pool.Repeat(pool.Choice([]RuleID{str(pool, "c"), str(pool, "d")}))),
		}),
	}))
	r1 := pat(pool, "e")
	r2 := pat(pool, "b")
	r3 := pool.Seq([]RuleID{nSym(pool, "rule_2"), pool.Blank()})
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("rule_1"), Root: r1},
		{Name: pool.Intern("rule_2"), Root: r2},
		{Name: pool.Intern("rule_3"), Root: r3},
	})
	pending := extractPending(t, g)

	// the extraction keeps each original token body until the lexical
	// expansion runs
	p := pending.grammar.Pool
	expected := []RuleID{
		str(p, "a"),
		pat(p, "b"),
		p.Repeat(p.Choice([]RuleID{str(p, "c"), str(p, "d")})),
		pat(p, "e"),
	}
	if len(pending.lexicalVariables) != len(expected) {
		t.Fatalf("expected %d tokens, got: %d", len(expected), len(pending.lexicalVariables))
	}
	for i, e := range expected {
		if !p.SubtreeEqual(pending.lexicalVariables[i].Root, e) {
			t.Errorf("token %d: the body is not the original body", i)
		}
	}

	ext, lexicalGrammar := commitForTest(t, pending)

	// rule_1 became a token, and rule_0, rule_2 and rule_3 stay
	expectVariableNames(t, g, []string{"rule_0", "rule_2", "rule_3"})
	if expected := []VariableType{VariableNamed, VariableNamed, VariableNamed}; !slices.Equal(ext.kinds, expected) {
		t.Errorf("expected kinds %v, got: %v", expected, ext.kinds)
	}

	// rule_0: repeat(seq(terminal(0), terminal(1), choice(terminal(3), non_terminal(1), terminal(2))))
	//  - Its leaves became terminals: "a" is t0, "b" is t1, and token(...)
	//    is t2.
	//  - Its references changed: rule_1 became a token, t3, and the index of
	//    rule_2 went down to 1, because rule_1 left.
	expectSubtree(t, p, g.Variables[0].Root, p.Repeat(p.Seq([]RuleID{
		term(p, 0),
		term(p, 1),
		p.Choice([]RuleID{term(p, 3), nonTerm(p, 1), term(p, 2)}),
	})))

	// rule_2 is /b/, which is terminal(1). It does not become a token,
	// because /b/ is in two places, rule_0 and rule_2, so its terminal is
	// used more than once.
	if n := g.Pool.Node(g.Variables[1].Root); n != (Rule{Kind: RuleSym, Sym: TerminalSymbol(1)}) {
		t.Errorf("expected terminal 1, got: %v", n)
	}

	// rule_3: seq(non_terminal(1), blank), with the index of rule_2 down
	// after rule_1 left
	expectSubtree(t, p, g.Variables[2].Root, p.Seq([]RuleID{nonTerm(p, 1), p.Blank()}))

	// /e/ is used in one place only, the whole body of rule_1, so rule_1
	// became a token and gave its name to it
	expectLexicalVariables(t, g.Pool, lexicalGrammar, []tokenNameKind{
		{"a", VariableAnonymous},
		{"rule_0_token1", VariableAuxiliary},
		{"rule_0_token2", VariableAuxiliary},
		{"rule_1", VariableNamed},
	})
}

// TestStartRuleIsToken is test_start_rule_is_token in extract_tokens.rs.
func TestStartRuleIsToken(t *testing.T) {
	t.Parallel()
	// The start rule is a bare token. The token is extracted, but the start
	// rule never becomes a token.
	pool := NewRulePool()
	r0 := str(pool, "hello")
	g := poolGrammar(pool, []Variable{{Name: pool.Intern("rule_0"), Root: r0}})
	pending := extractPending(t, g)

	if !g.Pool.SubtreeEqual(pending.lexicalVariables[0].Root, str(g.Pool, "hello")) {
		t.Error("expected the token body to be the original body")
	}

	_, lexicalGrammar := commitForTest(t, pending)
	expectVariableNames(t, g, []string{"rule_0"})
	if n := g.Pool.Node(g.Variables[0].Root); n != (Rule{Kind: RuleSym, Sym: TerminalSymbol(0)}) {
		t.Errorf("expected terminal 0, got: %v", n)
	}
	expectLexicalVariables(t, g.Pool, lexicalGrammar, []tokenNameKind{{"hello", VariableAnonymous}})
}

// TestExtractingExtraSymbols is test_extracting_extra_symbols in
// extract_tokens.rs.
func TestExtractingExtraSymbols(t *testing.T) {
	t.Parallel()
	// The extras go two ways. The reference to comment, which becomes a
	// token, becomes an extra terminal, and the bare " " becomes a
	// separator.
	pool := NewRulePool()
	r0 := str(pool, "x")
	comment := pat(pool, "//.*")
	sep := str(pool, " ")
	extraRef := nSym(pool, "comment")
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("comment"), Root: comment},
	})
	g.ExtraRoots = []RuleID{sep, extraRef}
	pending := extractPending(t, g)

	// The comment rule //.* is used once and becomes terminal 1, so its
	// extra becomes extra_symbols[0]. The string " " is a separator.
	if len(pending.separatorRoots) != 1 {
		t.Fatalf("expected 1 separator, got: %d", len(pending.separatorRoots))
	}
	if !g.Pool.SubtreeEqual(pending.separatorRoots[0], str(g.Pool, " ")) {
		t.Error(`expected the separator to be " "`)
	}
	ext, _ := commitForTest(t, pending)
	if !slices.Equal(ext.extraSymbols, []Symbol{TerminalSymbol(1)}) {
		t.Errorf("expected the extra symbols [t1], got: %v", ext.extraSymbols)
	}
}

// TestExtractExternals is test_extract_externals in extract_tokens.rs.
func TestExtractExternals(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	// rule_0: seq(external_0, "a", rule_1, rule_2)
	r0 := pool.Seq([]RuleID{nSym(pool, "external_0"), str(pool, "a"), nSym(pool, "rule_1"), nSym(pool, "rule_2")})
	r1 := str(pool, "b")
	r2 := str(pool, "c")
	// externals: [external_0, "a", rule_2]
	e0 := nSym(pool, "external_0")
	ea := str(pool, "a")
	er2 := nSym(pool, "rule_2")
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("rule_1"), Root: r1},
		{Name: pool.Intern("rule_2"), Root: r2},
	})
	external0, a, rule2 := pool.Intern("external_0"), pool.Intern("a"), pool.Intern("rule_2")
	g.ExternalRoots = []RuleID{e0, ea, er2}
	ext, _ := commitForTest(t, extractPending(t, g))

	expected := []ExternalToken{
		// a real external token, with no internal token
		{Name: external0, Kind: VariableNamed},
		// "a" is also an internal token, terminal 0, so it links to it
		{Name: a, Kind: VariableAnonymous, CorrespondingInternalToken: TerminalSymbol(0), HasCorrespondingInternalToken: true},
		// rule_2 shadows a variable that became terminal 2
		{Name: rule2, Kind: VariableNamed, CorrespondingInternalToken: TerminalSymbol(2), HasCorrespondingInternalToken: true},
	}
	if !slices.Equal(ext.externalTokens, expected) {
		t.Errorf("expected %v, got: %v", expected, ext.externalTokens)
	}
}

// TestErrorOnExternalWithSameNameAsNonTerminal is
// test_error_on_external_with_same_name_as_non_terminal in extract_tokens.rs.
func TestErrorOnExternalWithSameNameAsNonTerminal(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	r0 := pool.Seq([]RuleID{nSym(pool, "rule_1"), nSym(pool, "rule_2")})
	r1 := pool.Seq([]RuleID{nSym(pool, "rule_2"), nSym(pool, "rule_2")})
	r2 := str(pool, "a")
	extRef := nSym(pool, "rule_1")
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("rule_1"), Root: r1},
		{Name: pool.Intern("rule_2"), Root: r2},
	})
	g.ExternalRoots = []RuleID{extRef}

	// rule_1 is a seq, so it stays a non-terminal, and a non-terminal cannot
	// also be an external token
	_, err := extractTokens(g, internForTest(t, g))
	ee, ok := errors.AsType[*ExtractTokensError](err)
	if !ok || *ee != (ExtractTokensError{Kind: ExtractTokensExternalTokenNonTerminal, Name: "rule_1"}) {
		t.Errorf("expected the error that rule_1 is both, got: %v", err)
	}
}

// TestExtractionOnHiddenTerminal is test_extraction_on_hidden_terminal in
// extract_tokens.rs.
func TestExtractionOnHiddenTerminal(t *testing.T) {
	t.Parallel()
	// _rule_1 is hidden, and its token "a" is anonymous, so _rule_1 does not
	// become a token. Both variables stay, and the token keeps its own name.
	pool := NewRulePool()
	r0 := nSym(pool, "_rule_1")
	r1 := str(pool, "a")
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("_rule_1"), Root: r1},
	})
	pending := extractPending(t, g)

	if !g.Pool.SubtreeEqual(pending.lexicalVariables[0].Root, str(g.Pool, "a")) {
		t.Error("expected the token body to be the original body")
	}
	ext, lexicalGrammar := commitForTest(t, pending)
	expectVariableNames(t, g, []string{"rule_0", "_rule_1"})
	if expected := []VariableType{VariableNamed, VariableHidden}; !slices.Equal(ext.kinds, expected) {
		t.Errorf("expected kinds %v, got: %v", expected, ext.kinds)
	}
	if n := g.Pool.Node(g.Variables[0].Root); n != nt(1) {
		t.Errorf("expected %v, got: %v", nt(1), n)
	}
	if n := g.Pool.Node(g.Variables[1].Root); n != (Rule{Kind: RuleSym, Sym: TerminalSymbol(0)}) {
		t.Errorf("expected terminal 0, got: %v", n)
	}
	expectLexicalVariables(t, g.Pool, lexicalGrammar, []tokenNameKind{{"a", VariableAnonymous}})
}

// TestExtractionWithEmptyString is test_extraction_with_empty_string in
// extract_tokens.rs.
func TestExtractionWithEmptyString(t *testing.T) {
	t.Parallel()
	// an empty string outside the start rule is an error
	pool := NewRulePool()
	r0 := nSym(pool, "_rule_1")
	r1 := str(pool, "")
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("rule_0"), Root: r0},
		{Name: pool.Intern("_rule_1"), Root: r1},
	})
	_, err := extractTokens(g, internForTest(t, g))
	ee, ok := errors.AsType[*ExtractTokensError](err)
	if !ok || *ee != (ExtractTokensError{Kind: ExtractTokensEmptyString, Name: "_rule_1"}) {
		t.Errorf("expected the error of the empty string in _rule_1, got: %v", err)
	}
}

// str returns a string.
//
// str is str in extract_tokens.rs.
func str(p *RulePool, s string) RuleID {
	return p.String(p.Intern(s))
}

// pat returns a pattern with no flags.
//
// pat is pat in extract_tokens.rs.
func pat(p *RulePool, s string) RuleID {
	return p.Pattern(p.Intern(s), p.Intern(""))
}

// nSym returns a named symbol.
//
// nSym is n_sym in extract_tokens.rs.
func nSym(p *RulePool, name string) RuleID {
	return p.NamedSymbol(p.Intern(name))
}

// internForTest interns the symbols of a grammar, and fails the test on an
// error.
func internForTest(t *testing.T, g *InputGrammar) *internedGrammarMeta {
	t.Helper()
	meta, err := internSymbols(g, new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return meta
}

// extractPending interns the symbols of a grammar and extracts its tokens,
// and fails the test on an error.
//
// extractPending is extract_pending in extract_tokens.rs.
func extractPending(t *testing.T, g *InputGrammar) *pendingTokenExtraction {
	t.Helper()
	pending, err := extractTokens(g, internForTest(t, g))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return pending
}

// commitForTest expands and commits a pending extraction, and fails the test
// on an error.
func commitForTest(t *testing.T, pending *pendingTokenExtraction) (*extractedGrammarMeta, *LexicalGrammar) {
	t.Helper()
	ext, lexicalGrammar, err := pending.expandAndCommit()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return ext, lexicalGrammar
}

// expectLexicalVariables fails the test unless the tokens of a lexical
// grammar have the expected names and kinds.
func expectLexicalVariables(t *testing.T, pool *RulePool, g *LexicalGrammar, expected []tokenNameKind) {
	t.Helper()
	actual := make([]tokenNameKind, 0, len(g.Variables))
	for _, v := range g.Variables {
		actual = append(actual, tokenNameKind{pool.Resolve(v.Name), v.Kind})
	}
	if !slices.Equal(actual, expected) {
		t.Errorf("expected tokens %v, got: %v", expected, actual)
	}
}

// tokenNameKind is the name and the kind of a token.
type tokenNameKind struct {
	name string
	kind VariableType
}

// expectVariableNames fails the test unless the variables of a grammar have
// the expected names.
func expectVariableNames(t *testing.T, g *InputGrammar, expected []string) {
	t.Helper()
	names := make([]string, 0, len(g.Variables))
	for _, v := range g.Variables {
		names = append(names, g.Pool.Resolve(v.Name))
	}
	if !slices.Equal(names, expected) {
		t.Errorf("expected variables %v, got: %v", expected, names)
	}
}
