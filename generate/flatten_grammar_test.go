package generate

import (
	"errors"
	"slices"
	"testing"
)

// TestFlattenGrammar is test_flatten_grammar in flatten_grammar.rs.
func TestFlattenGrammar(t *testing.T) {
	t.Parallel()
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		inner := p.PrecRight(precInt(102), p.Seq([]RuleID{nonTerm(p, 3), nonTerm(p, 4)}))
		choice := p.Choice([]RuleID{inner, nonTerm(p, 5)})
		pl := p.PrecLeft(precInt(101), p.Seq([]RuleID{nonTerm(p, 2), choice, nonTerm(p, 6)}))
		return p.Seq([]RuleID{nonTerm(p, 1), pl, nonTerm(p, 7)})
	})
	expectProds(t, g, pool, []prodView{
		{steps: []stepView{
			newStep(NonTerminalSymbol(1)),
			newStep(NonTerminalSymbol(2)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
			newStep(NonTerminalSymbol(3)).withPrec(precInt(102)).withAssoc(AssociativityRight),
			newStep(NonTerminalSymbol(4)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
			newStep(NonTerminalSymbol(6)),
			newStep(NonTerminalSymbol(7)),
		}},
		{steps: []stepView{
			newStep(NonTerminalSymbol(1)),
			newStep(NonTerminalSymbol(2)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
			newStep(NonTerminalSymbol(5)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
			newStep(NonTerminalSymbol(6)),
			newStep(NonTerminalSymbol(7)),
		}},
	})
}

// TestFlattenGrammarWithMaximumDynamicPrecedence is
// test_flatten_grammar_with_maximum_dynamic_precedence in flatten_grammar.rs.
func TestFlattenGrammarWithMaximumDynamicPrecedence(t *testing.T) {
	t.Parallel()
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		inner := p.PrecDynamic(102, p.Seq([]RuleID{nonTerm(p, 3), nonTerm(p, 4)}))
		choice := p.Choice([]RuleID{inner, nonTerm(p, 5)})
		pd := p.PrecDynamic(101, p.Seq([]RuleID{nonTerm(p, 2), choice, nonTerm(p, 6)}))
		return p.Seq([]RuleID{nonTerm(p, 1), pd, nonTerm(p, 7)})
	})
	expectProds(t, g, pool, []prodView{
		{dynPrec: 102, steps: []stepView{
			newStep(NonTerminalSymbol(1)),
			newStep(NonTerminalSymbol(2)),
			newStep(NonTerminalSymbol(3)),
			newStep(NonTerminalSymbol(4)),
			newStep(NonTerminalSymbol(6)),
			newStep(NonTerminalSymbol(7)),
		}},
		{dynPrec: 101, steps: []stepView{
			newStep(NonTerminalSymbol(1)),
			newStep(NonTerminalSymbol(2)),
			newStep(NonTerminalSymbol(5)),
			newStep(NonTerminalSymbol(6)),
			newStep(NonTerminalSymbol(7)),
		}},
	})
}

// TestFlattenGrammarWithFinalPrecedence is
// test_flatten_grammar_with_final_precedence in flatten_grammar.rs.
func TestFlattenGrammarWithFinalPrecedence(t *testing.T) {
	t.Parallel()
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		return p.PrecLeft(precInt(101), p.Seq([]RuleID{nonTerm(p, 1), nonTerm(p, 2)}))
	})
	expectProds(t, g, pool, []prodView{{steps: []stepView{
		newStep(NonTerminalSymbol(1)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
		newStep(NonTerminalSymbol(2)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
	}}})

	g, pool = flattenNamed(t, func(p *RulePool) RuleID {
		return p.PrecLeft(precInt(101), p.Seq([]RuleID{nonTerm(p, 1)}))
	})
	expectProds(t, g, pool, []prodView{{steps: []stepView{
		newStep(NonTerminalSymbol(1)).withPrec(precInt(101)).withAssoc(AssociativityLeft),
	}}})
}

// TestFlattenGrammarWithFieldNames is test_flatten_grammar_with_field_names
// in flatten_grammar.rs.
func TestFlattenGrammarWithFieldNames(t *testing.T) {
	t.Parallel()
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		f1 := p.Field(p.Intern("first-thing"), term(p, 1))
		t2 := term(p, 2)
		f2 := p.Field(p.Intern("second-thing"), term(p, 3))
		return p.Seq([]RuleID{f1, t2, p.Choice([]RuleID{p.Blank(), f2})})
	})
	expectProds(t, g, pool, []prodView{
		{steps: []stepView{
			newStep(TerminalSymbol(1)).withField("first-thing"),
			newStep(TerminalSymbol(2)),
		}},
		{steps: []stepView{
			newStep(TerminalSymbol(1)).withField("first-thing"),
			newStep(TerminalSymbol(2)),
			newStep(TerminalSymbol(3)).withField("second-thing"),
		}},
	})
}

// TestPrecedenceInheritedThroughInnerMetadata is
// test_precedence_inherited_through_inner_metadata in flatten_grammar.rs.
func TestPrecedenceInheritedThroughInnerMetadata(t *testing.T) {
	t.Parallel()
	// an inner symbol with other metadata, a field, takes the precedence of
	// the region around it
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		a := term(p, 1)
		bf := p.Field(p.Intern("f"), term(p, 2))
		return p.Prec(precInt(5), p.Seq([]RuleID{a, bf}))
	})
	expectProds(t, g, pool, []prodView{{steps: []stepView{
		newStep(TerminalSymbol(1)).withPrec(precInt(5)),
		newStep(TerminalSymbol(2)).withPrec(precInt(5)).withField("f"),
	}}})
}

// TestFlattenGrammarWithRecursiveInlineVariable is
// test_flatten_grammar_with_recursive_inline_variable in flatten_grammar.rs.
func TestFlattenGrammarWithRecursiveInlineVariable(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	root := pool.Seq([]RuleID{nonTerm(pool, 0), nonTerm(pool, 1), nonTerm(pool, 2)})
	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: pool.Intern("test"), Root: root}}}
	meta := &extractedGrammarMeta{kinds: []VariableType{VariableNamed}, inline: []Symbol{NonTerminalSymbol(0)}}
	_, _, err := runFlatten(g, meta)
	expectFlattenError(t, err, FlattenGrammarError{Kind: FlattenGrammarRecursiveInline, Name: "test"})
}

// TestFlattenGrammarWithUnknownReserved is
// test_flatten_grammar_with_unknown_reserved in flatten_grammar.rs.
func TestFlattenGrammarWithUnknownReserved(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	root := pool.Reserved(term(pool, 1), pool.Intern("nope"))
	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: pool.Intern("test"), Root: root}}}
	_, _, err := runFlatten(g, &extractedGrammarMeta{kinds: []VariableType{VariableNamed}})
	expectFlattenError(t, err, FlattenGrammarError{Kind: FlattenGrammarNoReservedWordSet, Name: "nope"})
}

// TestFlattenGrammarWithEmptyProduction is
// test_flatten_grammar_with_empty_production in flatten_grammar.rs.
func TestFlattenGrammarWithEmptyProduction(t *testing.T) {
	t.Parallel()
	// a used variable has an empty production: a refers to b, and b is
	// empty
	pool := NewRulePool()
	aRoot := nonTerm(pool, 1)
	bRoot := pool.Blank()
	g := &InputGrammar{Pool: pool, Variables: []Variable{
		{Name: pool.Intern("a"), Root: aRoot},
		{Name: pool.Intern("b"), Root: bRoot},
	}}
	_, _, err := runFlatten(g, &extractedGrammarMeta{kinds: []VariableType{VariableNamed, VariableNamed}})
	expectFlattenError(t, err, FlattenGrammarError{Kind: FlattenGrammarEmptyString, Name: "b"})
}

// TestFlattenGrammarWithEmptyChoice is test_flatten_grammar_with_empty_choice
// in flatten_grammar.rs.
func TestFlattenGrammarWithEmptyChoice(t *testing.T) {
	t.Parallel()
	g, pool := flattenNamed(t, func(p *RulePool) RuleID {
		prefix := term(p, 1)
		empty := p.Choice(nil)
		tail := p.Choice([]RuleID{term(p, 2), term(p, 3)})
		dead := p.Seq([]RuleID{prefix, empty, tail})
		return p.Choice([]RuleID{dead, term(p, 4)})
	})
	expectProds(t, g, pool, []prodView{{steps: []stepView{newStep(TerminalSymbol(4))}}})

	g, pool = flattenNamed(t, func(p *RulePool) RuleID {
		return p.Choice(nil)
	})
	expectProds(t, g, pool, nil)
}

// TestFlattenGrammarWithNoReachableProductions makes sure that a variable
// whose every path has eof() before its end has no production, and that the
// pass reports it.
func TestFlattenGrammarWithNoReachableProductions(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	root := pool.Seq([]RuleID{pool.EOF(), term(pool, 1)})
	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: pool.Intern("source_file"), Root: root}}}
	_, _, err := runFlatten(g, &extractedGrammarMeta{kinds: []VariableType{VariableNamed}})
	expectFlattenError(t, err, FlattenGrammarError{Kind: FlattenGrammarNoReachableProductions, Name: "source_file"})
	if err.Error() != "Rule `source_file` has no reachable productions." {
		t.Errorf("expected the text of eof_misplaced, got: %q", err)
	}
}

// stepView is a step of a production, in a form that a test compares.
//
// stepView is StepView in flatten_grammar.rs.
type stepView struct {
	symbol   Symbol
	prec     Precedence
	assoc    Associativity
	alias    Alias
	hasAlias bool
	field    string
	reserved uint16
}

// newStep returns the view of a step of a symbol with no metadata.
//
// newStep is StepView::new in flatten_grammar.rs.
func newStep(symbol Symbol) stepView {
	return stepView{symbol: symbol}
}

// withPrec returns v with a precedence.
func (v stepView) withPrec(prec Precedence) stepView {
	v.prec = prec
	return v
}

// withAssoc returns v with an associativity.
func (v stepView) withAssoc(assoc Associativity) stepView {
	v.assoc = assoc
	return v
}

// withField returns v with a field.
func (v stepView) withField(name string) stepView {
	v.field = name
	return v
}

// prodView is a production, in a form that a test compares.
//
// prodView is ProdView in flatten_grammar.rs.
type prodView struct {
	dynPrec int32
	steps   []stepView
}

// precInt returns a precedence of a number.
func precInt(n int32) Precedence {
	return Precedence{Kind: PrecedenceInteger, Integer: n}
}

// runFlatten flattens a grammar and builds its syntax grammar.
//
// runFlatten is run in flatten_grammar.rs.
func runFlatten(g *InputGrammar, meta *extractedGrammarMeta) (*SyntaxGrammar, *StrPool, error) {
	var st flattenState
	var out ProductionStore
	if err := flattenGrammar(g, meta, &st, &out); err != nil {
		return nil, nil, err
	}
	sg, pool := assembleSyntaxGrammar(g, meta, &out)
	return sg, pool, nil
}

// flattenNamed flattens a grammar of one named variable, test, whose rule
// build makes, and fails the test on an error.
//
// flattenNamed is flatten_named in flatten_grammar.rs.
func flattenNamed(t *testing.T, build func(p *RulePool) RuleID) (*SyntaxGrammar, *StrPool) {
	t.Helper()
	pool := NewRulePool()
	root := build(pool)
	g := &InputGrammar{Pool: pool, Variables: []Variable{{Name: pool.Intern("test"), Root: root}}}
	sg, sp, err := runFlatten(g, &extractedGrammarMeta{kinds: []VariableType{VariableNamed}})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return sg, sp
}

// expectProds fails the test unless the productions of variable 0 are the
// expected ones.
//
// expectProds does what prods in flatten_grammar.rs does, and compares.
func expectProds(t *testing.T, g *SyntaxGrammar, pool *StrPool, expected []prodView) {
	t.Helper()
	var actual []prodView
	for _, p := range g.Productions[g.VarProds[0][0]:g.VarProds[0][1]] {
		start, end := p.StepRange()
		view := prodView{dynPrec: p.DynamicPrecedence}
		for _, s := range g.Steps[start:end] {
			sv := stepView{symbol: s.Symbol(), prec: s.Precedence(), assoc: s.Associativity(), reserved: s.Reserved}
			sv.alias, sv.hasAlias = s.GetAlias()
			if f, ok := s.GetField(); ok {
				sv.field = pool.Resolve(f)
			}
			view.steps = append(view.steps, sv)
		}
		actual = append(actual, view)
	}
	if !slices.EqualFunc(actual, expected, func(a, b prodView) bool {
		return a.dynPrec == b.dynPrec && slices.Equal(a.steps, b.steps)
	}) {
		t.Errorf("expected %v, got: %v", expected, actual)
	}
}

// expectFlattenError fails the test unless err is the expected error.
func expectFlattenError(t *testing.T, err error, expected FlattenGrammarError) {
	t.Helper()
	fe, ok := errors.AsType[*FlattenGrammarError](err)
	if !ok || *fe != expected {
		t.Errorf("expected %v, got: %v", &expected, err)
	}
}
