package generate

import "testing"

// TestExtractSimpleAliases is test_extract_simple_aliases in
// extract_default_aliases.rs.
func TestExtractSimpleAliases(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	dummy := pool.Intern("_")
	root := pool.Blank()

	var lexicalVariables []LexicalVariable
	t0 := addLexicalVariable(pool, &lexicalVariables, "t0")
	t1 := addLexicalVariable(pool, &lexicalVariables, "t1")
	t2 := addLexicalVariable(pool, &lexicalVariables, "t2")
	t3 := addLexicalVariable(pool, &lexicalVariables, "t3")

	// v1: every token has an alias
	v1 := []ProductionStep{
		aliased(pool, t0, "a1"),
		aliased(pool, t1, "a2"),
		aliased(pool, t2, "a3"),
		aliased(pool, t3, "a4"),
	}
	// v2: t0 has the same alias, t1 has none, t2 has another one, and t3
	// has a6 twice
	v2 := []ProductionStep{
		aliased(pool, t0, "a1"),
		plain(t1),
		aliased(pool, t2, "a5"),
		aliased(pool, t3, "a6"),
		aliased(pool, t3, "a6"),
	}

	var out ProductionStore
	for _, steps := range [][]ProductionStep{v1, v2} {
		ps := uint32(len(out.Productions))
		stepsStart := uint32(len(out.Steps))
		out.Steps = append(out.Steps, steps...)
		out.Productions = append(out.Productions, Production{StepsStart: stepsStart, StepsLen: uint32(len(steps))})
		out.VarProds = append(out.VarProds, [2]uint32{ps, uint32(len(out.Productions))})
	}

	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: dummy, Root: root}, {Name: dummy, Root: root}}}
	result := extractDefaultAliases(g, &extractedGrammarMeta{}, lexicalVariables, &out)

	// t0 gets a1, t2 gets a3 because v1 wins the tie by coming first, and t3
	// gets a6 because it is used twice. t1 gets none, because it appears
	// without an alias in v2.
	if len(result) != 3 {
		t.Errorf("expected 3 default aliases, got: %d", len(result))
	}
	for _, test := range []struct {
		symbol   Symbol
		expected string
	}{
		{TerminalSymbol(0), "a1"},
		{TerminalSymbol(2), "a3"},
		{TerminalSymbol(3), "a6"},
		{TerminalSymbol(1), ""},
	} {
		a, ok := result[test.symbol]
		actual := ""
		if ok {
			actual = pool.Resolve(a.Value)
			if !a.IsNamed {
				t.Errorf("%v: expected a named alias", test.symbol)
			}
		}
		if actual != test.expected {
			t.Errorf("%v: expected the default alias %q, got: %q", test.symbol, test.expected, actual)
		}
	}

	// the steps with the default alias of their symbol lose it, and the
	// others keep their alias
	for i, expected := range []string{
		"",   // v1 t0(a1) is the default
		"a2", // v1 t1(a2), no default
		"",   // v1 t2(a3) is the default
		"a4", // v1 t3(a4) is not a6
		"",   // v2 t0(a1) is the default
		"",   // v2 t1 has no alias
		"a5", // v2 t2(a5) is not a3
		"",   // v2 t3(a6) is the default
		"",   // v2 t3(a6) is the default
	} {
		actual := ""
		if a, ok := out.Steps[i].GetAlias(); ok {
			actual = pool.Resolve(a.Value)
		}
		if actual != expected {
			t.Errorf("step %d: expected the alias %q, got: %q", i, expected, actual)
		}
	}
}

// addLexicalVariable adds an anonymous token with a name, and returns its
// symbol.
//
// addLexicalVariable is add_lexical_variable in extract_default_aliases.rs.
func addLexicalVariable(pool *RulePool, variables *[]LexicalVariable, name string) Symbol {
	symbol := TerminalSymbol(len(*variables))
	*variables = append(*variables, LexicalVariable{Name: pool.Intern(name), Kind: VariableAnonymous})
	return symbol
}

// aliased returns a step of a symbol with a named alias.
//
// aliased is aliased in extract_default_aliases.rs.
func aliased(pool *RulePool, symbol Symbol, name string) ProductionStep {
	return PackProductionStep(symbol, Precedence{}, AssociativityNone, Alias{Value: pool.Intern(name), IsNamed: true}, true, 0, 0)
}

// plain returns a step of a symbol with no metadata.
//
// plain is plain in extract_default_aliases.rs.
func plain(symbol Symbol) ProductionStep {
	return PackProductionStep(symbol, Precedence{}, AssociativityNone, Alias{}, false, 0, 0)
}
