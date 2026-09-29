package generate

import (
	"errors"
	"slices"
	"testing"
)

// TestBasicRepeatExpansion is test_basic_repeat_expansion in
// expand_repeats.rs.
func TestBasicRepeatExpansion(t *testing.T) {
	t.Parallel()
	// the repeats in seqs and choices expand
	pool := NewRulePool()
	r0 := pool.Seq([]RuleID{
		term(pool, 10),
		pool.Choice([]RuleID{pool.Repeat(term(pool, 11)), pool.Repeat(term(pool, 12))}),
		term(pool, 13),
	})
	g, meta := expand(t, pool, []Variable{{Name: pool.Intern("rule0"), Root: r0}}, []VariableType{VariableNamed})

	expectVariableNames(t, g, []string{"rule0", "rule0_repeat1", "rule0_repeat2"})
	expectKinds(t, meta, []VariableType{VariableNamed, VariableAuxiliary, VariableAuxiliary})

	p := g.Pool
	// rule0: seq(terminal(10), choice(non_terminal(1), non_terminal(2)), terminal(13))
	expectSubtree(t, p, g.Variables[0].Root, p.Seq([]RuleID{
		term(p, 10),
		p.Choice([]RuleID{nonTerm(p, 1), nonTerm(p, 2)}),
		term(p, 13),
	}))
	// rule0_repeat1: choice(seq(nt1, nt1), terminal(11))
	expectSubtree(t, p, g.Variables[1].Root, p.Choice([]RuleID{p.Seq([]RuleID{nonTerm(p, 1), nonTerm(p, 1)}), term(p, 11)}))
	// rule0_repeat2: choice(seq(nt2, nt2), terminal(12))
	expectSubtree(t, p, g.Variables[2].Root, p.Choice([]RuleID{p.Seq([]RuleID{nonTerm(p, 2), nonTerm(p, 2)}), term(p, 12)}))
}

// TestRepeatDeduplication is test_repeat_deduplication in expand_repeats.rs.
func TestRepeatDeduplication(t *testing.T) {
	t.Parallel()
	// repeat(terminal(4)) is in three places, and only one auxiliary rule is
	// made
	pool := NewRulePool()
	r0 := pool.Choice([]RuleID{
		pool.Seq([]RuleID{term(pool, 1), pool.Repeat(term(pool, 4))}),
		pool.Seq([]RuleID{term(pool, 2), pool.Repeat(term(pool, 4))}),
	})
	r1 := pool.Seq([]RuleID{term(pool, 3), pool.Repeat(term(pool, 4))})
	g, meta := expand(t, pool, []Variable{
		{Name: pool.Intern("rule0"), Root: r0},
		{Name: pool.Intern("rule1"), Root: r1},
	}, []VariableType{VariableNamed, VariableNamed})

	expectVariableNames(t, g, []string{"rule0", "rule1", "rule0_repeat1"})
	expectKinds(t, meta, []VariableType{VariableNamed, VariableNamed, VariableAuxiliary})

	p := g.Pool
	// rule0: choice(seq(t1, nt2), seq(t2, nt2))
	expectSubtree(t, p, g.Variables[0].Root, p.Choice([]RuleID{
		p.Seq([]RuleID{term(p, 1), nonTerm(p, 2)}),
		p.Seq([]RuleID{term(p, 2), nonTerm(p, 2)}),
	}))
	// rule1: seq(t3, nt2)
	expectSubtree(t, p, g.Variables[1].Root, p.Seq([]RuleID{term(p, 3), nonTerm(p, 2)}))
	// rule0_repeat1: choice(seq(nt2, nt2), terminal(4))
	expectSubtree(t, p, g.Variables[2].Root, p.Choice([]RuleID{p.Seq([]RuleID{nonTerm(p, 2), nonTerm(p, 2)}), term(p, 4)}))
}

// TestExpansionOfNestedRepeats is test_expansion_of_nested_repeats in
// expand_repeats.rs.
func TestExpansionOfNestedRepeats(t *testing.T) {
	t.Parallel()
	// Nested repeats expand from the inside. The inner one becomes
	// rule0_repeat1, nt1, and then the outer one, which refers to it,
	// becomes rule0_repeat2.
	pool := NewRulePool()
	r0 := pool.Seq([]RuleID{
		term(pool, 10),
		pool.Repeat(pool.Seq([]RuleID{term(pool, 11), pool.Repeat(term(pool, 12))})),
	})
	g, meta := expand(t, pool, []Variable{{Name: pool.Intern("rule0"), Root: r0}}, []VariableType{VariableNamed})

	expectVariableNames(t, g, []string{"rule0", "rule0_repeat1", "rule0_repeat2"})
	expectKinds(t, meta, []VariableType{VariableNamed, VariableAuxiliary, VariableAuxiliary})

	p := g.Pool
	// rule0: seq(terminal(10), non_terminal(2))
	expectSubtree(t, p, g.Variables[0].Root, p.Seq([]RuleID{term(p, 10), nonTerm(p, 2)}))
	// rule0_repeat2, the outer one: choice(seq(nt2, nt2), seq(terminal(11), nt1))
	expectSubtree(t, p, g.Variables[2].Root, p.Choice([]RuleID{
		p.Seq([]RuleID{nonTerm(p, 2), nonTerm(p, 2)}),
		p.Seq([]RuleID{term(p, 11), nonTerm(p, 1)}),
	}))
}

// TestExpansionOfRepeatsAtTopOfHiddenRules is
// test_expansion_of_repeats_at_top_of_hidden_rules in expand_repeats.rs.
func TestExpansionOfRepeatsAtTopOfHiddenRules(t *testing.T) {
	t.Parallel()
	// A hidden rule whose whole body is a repeat becomes its own binary
	// tree, with its own symbol, instead of gaining an auxiliary rule. Its
	// kind becomes auxiliary.
	pool := NewRulePool()
	r0 := nonTerm(pool, 1)
	r1 := pool.Repeat(pool.Choice([]RuleID{term(pool, 11), term(pool, 12)}))
	g, meta := expand(t, pool, []Variable{
		{Name: pool.Intern("rule0"), Root: r0},
		{Name: pool.Intern("_rule1"), Root: r1},
	}, []VariableType{VariableNamed, VariableHidden})

	// no auxiliary rule
	expectVariableNames(t, g, []string{"rule0", "_rule1"})
	expectKinds(t, meta, []VariableType{VariableNamed, VariableAuxiliary})

	// rule0 is non_terminal(1), as before
	if n := g.Pool.Node(g.Variables[0].Root); n != nt(1) {
		t.Errorf("expected %v, got: %v", nt(1), n)
	}
	p := g.Pool
	// _rule1: choice(seq(nt1, nt1), terminal(11), terminal(12)), with the
	// inner choice flattened
	expectSubtree(t, p, g.Variables[1].Root, p.Choice([]RuleID{
		p.Seq([]RuleID{nonTerm(p, 1), nonTerm(p, 1)}),
		term(p, 11),
		term(p, 12),
	}))
}

// TestRejectsRepeatOfEOFHelperRule is test_rejects_repeat_of_eof_helper_rule
// in expand_repeats.rs.
func TestRejectsRepeatOfEOFHelperRule(t *testing.T) {
	t.Parallel()
	// rule0: repeat(non_terminal(1))
	// rule1: eof()
	pool := NewRulePool()
	r0 := pool.Repeat(nonTerm(pool, 1))
	r1 := pool.PushNode(Rule{Kind: RuleEOF})
	g := &InputGrammar{Pool: pool, Variables: []Variable{
		{Name: pool.Intern("rule0"), Root: r0},
		{Name: pool.Intern("rule1"), Root: r1},
	}}
	meta := &extractedGrammarMeta{kinds: []VariableType{VariableNamed, VariableNamed}}
	err := expandRepeats(g, meta)
	ee, ok := errors.AsType[*ExpandRepeatsError](err)
	if !ok || ee.Rule != "rule0" {
		t.Errorf("expected the error of the repeat in rule0, got: %v", err)
	}
}

// term returns a new node of the terminal with an index.
//
// term is term in expand_repeats.rs.
func term(p *RulePool, i int) RuleID {
	return p.PushNode(Rule{Kind: RuleSym, Sym: TerminalSymbol(i)})
}

// nonTerm returns a new node of the non-terminal with an index.
//
// nonTerm is non_term in expand_repeats.rs.
func nonTerm(p *RulePool, i int) RuleID {
	return p.PushNode(Rule{Kind: RuleSym, Sym: NonTerminalSymbol(i)})
}

// expand expands the repeats of a grammar with the kinds of its variables,
// and fails the test on an error.
//
// expand is expand in expand_repeats.rs.
func expand(t *testing.T, pool *RulePool, variables []Variable, kinds []VariableType) (*InputGrammar, *extractedGrammarMeta) {
	t.Helper()
	g := &InputGrammar{Pool: pool, Variables: variables}
	meta := &extractedGrammarMeta{kinds: kinds}
	if err := expandRepeats(g, meta); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return g, meta
}

// expectKinds fails the test unless the kinds of the variables are the
// expected kinds.
func expectKinds(t *testing.T, meta *extractedGrammarMeta, expected []VariableType) {
	t.Helper()
	if !slices.Equal(meta.kinds, expected) {
		t.Errorf("expected kinds %v, got: %v", expected, meta.kinds)
	}
}

// expectSubtree fails the test unless two subtrees are equal.
func expectSubtree(t *testing.T, pool *RulePool, actual, expected RuleID) {
	t.Helper()
	if !pool.SubtreeEqual(actual, expected) {
		t.Error("expected the subtrees to be equal")
	}
}
