package generate

import (
	"errors"
	"slices"
	"strconv"
	"testing"
)

// TestBasicInlining is test_basic_inlining in process_inlines.rs.
func TestBasicInlining(t *testing.T) {
	t.Parallel()
	// var0: [t10, nt1, t11], where nt1 is inlined
	// var1: [t12, t13] | [t14]
	var out ProductionStore
	addVariable(&out, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(NonTerminalSymbol(1)), plain(TerminalSymbol(11))}, 0},
	})
	addVariable(&out, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(12)), plain(TerminalSymbol(13))}, 0},
		{[]ProductionStep{plain(TerminalSymbol(14))}, -2},
	})

	g := &InputGrammar{Pool: NewRulePool()}
	meta := &extractedGrammarMeta{inline: []Symbol{NonTerminalSymbol(1)}}
	lexicalVariables := makeLexicalVariablesThrough(g.Pool, 14)
	m, err := processInlines(g, meta, lexicalVariables, &out)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	prod0 := out.VarProds[0][0]

	// nothing to inline at step 0
	if _, _, ok := inlined(&out, m, prod0, 0); ok {
		t.Error("expected nothing inlined at step 0")
	}

	// inlining variable 1 gives two productions
	_, prods, _ := inlined(&out, m, prod0, 1)
	expectInlined(t, prods, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(TerminalSymbol(12)), plain(TerminalSymbol(13)), plain(TerminalSymbol(11))}, 0},
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(TerminalSymbol(14)), plain(TerminalSymbol(11))}, -2},
	})
}

// TestNestedInlining is test_nested_inlining in process_inlines.rs.
func TestNestedInlining(t *testing.T) {
	t.Parallel()
	// var0: [t10, nt1, t11, nt2, t12], where nt1 and nt2 are inlined
	// var1: [t13] | [nt3, t14], where nt3 is inlined
	// var2: [t15]
	// var3: [t16]
	var out ProductionStore
	addVariable(&out, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(NonTerminalSymbol(1)), plain(TerminalSymbol(11)), plain(NonTerminalSymbol(2)), plain(TerminalSymbol(12))}, 0},
	})
	addVariable(&out, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(13))}, 0},
		{[]ProductionStep{plain(NonTerminalSymbol(3)), plain(TerminalSymbol(14))}, 0},
	})
	addVariable(&out, []inlinedProduction{{[]ProductionStep{plain(TerminalSymbol(15))}, 0}})
	addVariable(&out, []inlinedProduction{{[]ProductionStep{plain(TerminalSymbol(16))}, 0}})

	g := &InputGrammar{Pool: NewRulePool()}
	meta := &extractedGrammarMeta{inline: []Symbol{NonTerminalSymbol(1), NonTerminalSymbol(2), NonTerminalSymbol(3)}}
	lexicalVariables := makeLexicalVariablesThrough(g.Pool, 16)
	m, err := processInlines(g, meta, lexicalVariables, &out)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	prod0 := out.VarProds[0][0]

	ids, prods, _ := inlined(&out, m, prod0, 1)
	expectInlined(t, prods, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(TerminalSymbol(13)), plain(TerminalSymbol(11)), plain(NonTerminalSymbol(2)), plain(TerminalSymbol(12))}, 0},
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(TerminalSymbol(16)), plain(TerminalSymbol(14)), plain(TerminalSymbol(11)), plain(NonTerminalSymbol(2)), plain(TerminalSymbol(12))}, 0},
	})

	// nt2, now at step 3
	if len(ids) == 0 {
		t.Fatal("expected inlined productions")
	}
	_, prods, _ = inlined(&out, m, ids[0], 3)
	expectInlined(t, prods, []inlinedProduction{
		{[]ProductionStep{plain(TerminalSymbol(10)), plain(TerminalSymbol(13)), plain(TerminalSymbol(11)), plain(TerminalSymbol(15)), plain(TerminalSymbol(12))}, 0},
	})
}

// TestInliningWithPrecedenceAndAlias is
// test_inlining_with_precedence_and_alias in process_inlines.rs.
func TestInliningWithPrecedenceAndAlias(t *testing.T) {
	t.Parallel()
	// var0: [nt1{prec 1, left}, t10, nt2{alias outer}], where nt1 and nt2
	//       are inlined
	// var1: [t11{prec 2, alias inner}, t12]
	// var2: [t13]
	pool := NewRulePool()
	var out ProductionStore
	addVariable(&out, []inlinedProduction{{[]ProductionStep{
		decorated(pool, NonTerminalSymbol(1), precInt(1), AssociativityLeft, ""),
		plain(TerminalSymbol(10)),
		decorated(pool, NonTerminalSymbol(2), Precedence{}, AssociativityNone, "outer_alias"),
	}, 0}})
	addVariable(&out, []inlinedProduction{{[]ProductionStep{
		decorated(pool, TerminalSymbol(11), precInt(2), AssociativityNone, "inner_alias"),
		plain(TerminalSymbol(12)),
	}, 0}})
	addVariable(&out, []inlinedProduction{{[]ProductionStep{plain(TerminalSymbol(13))}, 0}})

	g := &InputGrammar{Pool: NewRulePool()}
	meta := &extractedGrammarMeta{inline: []Symbol{NonTerminalSymbol(1), NonTerminalSymbol(2)}}
	lexicalVariables := makeLexicalVariablesThrough(g.Pool, 13)
	m, err := processInlines(g, meta, lexicalVariables, &out)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	prod0 := out.VarProds[0][0]

	ids, prods, _ := inlined(&out, m, prod0, 0)
	expectInlined(t, prods, []inlinedProduction{{[]ProductionStep{
		// the first inlined step keeps its own precedence and alias
		decorated(pool, TerminalSymbol(11), precInt(2), AssociativityNone, "inner_alias"),
		// the last inlined step takes the precedence of the inlined step
		decorated(pool, TerminalSymbol(12), precInt(1), AssociativityLeft, ""),
		plain(TerminalSymbol(10)),
		decorated(pool, NonTerminalSymbol(2), Precedence{}, AssociativityNone, "outer_alias"),
	}, 0}})

	if len(ids) == 0 {
		t.Fatal("expected inlined productions")
	}
	_, prods, _ = inlined(&out, m, ids[0], 3)
	expectInlined(t, prods, []inlinedProduction{{[]ProductionStep{
		decorated(pool, TerminalSymbol(11), precInt(2), AssociativityNone, "inner_alias"),
		decorated(pool, TerminalSymbol(12), precInt(1), AssociativityLeft, ""),
		plain(TerminalSymbol(10)),
		// every inlined step takes the alias of the inlined step
		decorated(pool, TerminalSymbol(13), Precedence{}, AssociativityNone, "outer_alias"),
	}, 0}})
}

// TestErrorWhenInliningTokens is test_error_when_inlining_tokens in
// process_inlines.rs.
func TestErrorWhenInliningTokens(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	name := pool.Intern("something")
	g := &InputGrammar{Pool: pool}
	meta := &extractedGrammarMeta{inline: []Symbol{TerminalSymbol(0)}}
	lexicalVariables := []LexicalVariable{{Name: name, Kind: VariableNamed}}
	var out ProductionStore

	_, err := processInlines(g, meta, lexicalVariables, &out)
	pe, ok := errors.AsType[*ProcessInlinesError](err)
	if !ok || *pe != (ProcessInlinesError{Kind: ProcessInlinesToken, Name: "something"}) {
		t.Errorf("expected the error that the token cannot be inlined, got: %v", err)
	}
}

// TestErrorWhenInliningRemovesAllProductions is
// test_error_when_inlining_removes_all_productions in process_inlines.rs.
func TestErrorWhenInliningRemovesAllProductions(t *testing.T) {
	t.Parallel()
	// var0: [nt1]
	// var1: [nt2, t10], where nt2 is inlined
	// var2: one empty production that needs the end of the input, as the
	//       flattened _eof: _ => eof() has
	// Inlining var2 into var1 drops the only production of var1.
	var out ProductionStore
	addVariable(&out, []inlinedProduction{{[]ProductionStep{plain(NonTerminalSymbol(1))}, 0}})
	addVariable(&out, []inlinedProduction{{[]ProductionStep{plain(NonTerminalSymbol(2)), plain(TerminalSymbol(10))}, 0}})
	start := uint32(len(out.Productions))
	out.Productions = append(out.Productions, Production{StepsStart: uint32(len(out.Steps)), RequiresEOFLookahead: true})
	out.VarProds = append(out.VarProds, [2]uint32{start, start + 1})

	pool := NewRulePool()
	var variables []Variable
	for _, name := range []string{"rule0", "rule1", "rule2"} {
		variables = append(variables, Variable{Name: pool.Intern(name), Root: pool.PushNode(Rule{Kind: RuleBlank})})
	}
	lexicalVariables := makeLexicalVariablesThrough(pool, 10)
	g := &InputGrammar{Pool: pool, Variables: variables}
	meta := &extractedGrammarMeta{inline: []Symbol{NonTerminalSymbol(2)}}
	_, err := processInlines(g, meta, lexicalVariables, &out)
	pe, ok := errors.AsType[*ProcessInlinesError](err)
	if !ok || *pe != (ProcessInlinesError{Kind: ProcessInlinesNoReachableProductions, Name: "rule1"}) {
		t.Errorf("expected the error that rule1 has no productions, got: %v", err)
	}
}

// makeLexicalVariablesThrough returns the anonymous tokens t0 to t<last>.
//
// makeLexicalVariablesThrough is make_lexical_variables_through in
// process_inlines.rs.
func makeLexicalVariablesThrough(pool *RulePool, last int) []LexicalVariable {
	variables := make([]LexicalVariable, 0, last+1)
	for i := range last + 1 {
		variables = append(variables, LexicalVariable{Name: pool.Intern("t" + strconv.Itoa(i)), Kind: VariableAnonymous})
	}
	return variables
}

// inlinedProduction is the steps and the dynamic precedence of a production.
//
// inlinedProduction is InlinedProduction in process_inlines.rs.
type inlinedProduction struct {
	steps   []ProductionStep
	dynPrec int32
}

// addVariable adds the productions of one variable to out, and records the
// range of their ids.
//
// addVariable is add_variable in process_inlines.rs.
func addVariable(out *ProductionStore, prods []inlinedProduction) {
	start := uint32(len(out.Productions))
	for _, p := range prods {
		stepsStart := uint32(len(out.Steps))
		out.Steps = append(out.Steps, p.steps...)
		out.Productions = append(out.Productions, Production{StepsStart: stepsStart, StepsLen: uint32(len(p.steps)), DynamicPrecedence: p.dynPrec})
	}
	out.VarProds = append(out.VarProds, [2]uint32{start, uint32(len(out.Productions))})
}

// inlined returns the ids of the productions that replace a step of a
// production, with the steps and the dynamic precedence of each.
//
// inlined is inlined in process_inlines.rs.
func inlined(out *ProductionStore, m InlinedProductionMap, prodID, step uint32) ([]uint32, []inlinedProduction, bool) {
	ids, ok := m.InlinedProdIDs(prodID, step)
	if !ok {
		return nil, nil, false
	}
	prods := make([]inlinedProduction, 0, len(ids))
	for _, id := range ids {
		p := out.Productions[id]
		start, end := p.StepRange()
		prods = append(prods, inlinedProduction{steps: slices.Clone(out.Steps[start:end]), dynPrec: p.DynamicPrecedence})
	}
	return slices.Clone(ids), prods, true
}

// decorated returns a step with a precedence, an associativity and a named
// alias. An empty alias stands for none.
//
// decorated is decorated in process_inlines.rs.
func decorated(pool *RulePool, symbol Symbol, prec Precedence, assoc Associativity, alias string) ProductionStep {
	var a Alias
	if alias != "" {
		a = Alias{Value: pool.Intern(alias), IsNamed: true}
	}
	return PackProductionStep(symbol, prec, assoc, a, alias != "", 0, 0)
}

// expectInlined fails the test unless the productions are the expected ones.
func expectInlined(t *testing.T, actual, expected []inlinedProduction) {
	t.Helper()
	if !slices.EqualFunc(actual, expected, func(a, b inlinedProduction) bool {
		return a.dynPrec == b.dynPrec && slices.Equal(a.steps, b.steps)
	}) {
		t.Errorf("expected %v, got: %v", expected, actual)
	}
}
