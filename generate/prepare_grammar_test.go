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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateIndirectRecursion(buildGrammar(test.build))
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

// TestTheFirstPassesOnEveryTestGrammar runs the passes that are ported, in the
// order of prepare_grammar, on each test grammar. A grammar that they reject
// must have the same error in its golden file.
func TestTheFirstPassesOnEveryTestGrammar(t *testing.T) {
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
		err = validatePrecedences(g)
		if err == nil {
			err = validateIndirectRecursion(g)
		}
		var interned *internedGrammarMeta
		if err == nil {
			interned, err = internSymbols(g, new([]Diagnostic))
		}
		if err == nil {
			_, err = extractTokens(g, interned)
		}
		if err == nil {
			continue
		}
		dir := filepath.Dir(f)
		rejected = append(rejected, filepath.Base(dir))
		golden, rerr := os.ReadFile(filepath.Join(dir, "abi15", "error.txt"))
		if rerr != nil {
			t.Errorf("%s: the passes give %q, and the grammar has no error golden: %v", f, err, rerr)
			continue
		}
		if !strings.Contains(string(golden), "Caused by:\n    "+err.Error()+"\n") {
			t.Errorf("%s: expected the error of the golden file, got: %q", f, err)
		}
	}
	expected := []string{"indirect_recursion_in_transitions", "invisible_start_rule", "terminal_supertype"}
	if !slices.Equal(rejected, expected) {
		t.Errorf("expected the passes to reject %v, they rejected %v", expected, rejected)
	}
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
