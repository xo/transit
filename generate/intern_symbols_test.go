package generate

import (
	"errors"
	"slices"
	"testing"
)

// TestBasicInterning is test_basic_interning in intern_symbols.rs.
func TestBasicInterning(t *testing.T) {
	t.Parallel()
	// x: choice(y, _z)
	// y: _z
	// _z: "a"
	pool := NewRulePool()
	yStr, zStr := pool.Intern("y"), pool.Intern("_z")
	y := pool.NamedSymbol(yStr)
	z := pool.NamedSymbol(zStr)
	a := pool.Intern("a")
	xRoot := pool.Choice([]RuleID{y, z})
	yRoot := pool.NamedSymbol(zStr)
	zRoot := pool.String(a)
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("x"), Root: xRoot},
		{Name: pool.Intern("y"), Root: yRoot},
		{Name: pool.Intern("_z"), Root: zRoot},
	})
	meta, err := internSymbols(g, new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// x's body was choice(y, _z), and is choice(nt1, nt2)
	expectChildren(t, g.Pool, g.Variables[0].Root, RuleChoice, []Rule{nt(1), nt(2)})

	// y's body was _z, and is nt2
	if n := g.Pool.Node(g.Variables[1].Root); n != nt(2) {
		t.Errorf("expected %v, got: %v", nt(2), n)
	}

	// z's body, a string, stays as it is, and z stays hidden
	if n := g.Pool.Node(g.Variables[2].Root); n.Kind != RuleString {
		t.Errorf("expected a string, got: %v", n)
	}
	if expected := []VariableType{VariableNamed, VariableNamed, VariableHidden}; !slices.Equal(meta.kinds, expected) {
		t.Errorf("expected kinds %v, got: %v", expected, meta.kinds)
	}
}

// TestInterningExternalTokenNames is test_interning_external_token_names in
// intern_symbols.rs.
func TestInterningExternalTokenNames(t *testing.T) {
	t.Parallel()
	// w: choice(x, y, z)
	// x: "a"
	// y: "b"
	// externals: [y, z]
	pool := NewRulePool()
	xStr, yStr, zStr := pool.Intern("x"), pool.Intern("y"), pool.Intern("z")
	xNS := pool.NamedSymbol(xStr)
	yNS := pool.NamedSymbol(yStr)
	zNS := pool.NamedSymbol(zStr)
	wRoot := pool.Choice([]RuleID{xNS, yNS, zNS})
	a, b := pool.Intern("a"), pool.Intern("b")
	xRoot := pool.String(a)
	yRoot := pool.String(b)
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("w"), Root: wRoot},
		{Name: pool.Intern("x"), Root: xRoot},
		{Name: pool.Intern("y"), Root: yRoot},
	})
	g.ExternalRoots = []RuleID{g.Pool.NamedSymbol(yStr), g.Pool.NamedSymbol(zStr)}

	meta, err := internSymbols(g, new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// w: x is nt1, y is nt2 because the variable shadows the external, and
	// z is ext1
	expectChildren(t, g.Pool, g.Variables[0].Root, RuleChoice, []Rule{nt(1), nt(2), ext(1)})

	// the bodies of x and y, bare strings, stay as they are
	for i := 1; i <= 2; i++ {
		if n := g.Pool.Node(g.Variables[i].Root); n.Kind != RuleString {
			t.Errorf("variable %d: expected a string, got: %v", i, n)
		}
	}

	// the external roots resolve the same way: y is nt2, and z is ext1
	if n := g.Pool.Node(g.ExternalRoots[0]); n != nt(2) {
		t.Errorf("expected %v, got: %v", nt(2), n)
	}
	if n := g.Pool.Node(g.ExternalRoots[1]); n != ext(1) {
		t.Errorf("expected %v, got: %v", ext(1), n)
	}

	// the metadata of the external tokens holds their names and kinds
	var names []string
	for _, e := range meta.externalTokens {
		if e.kind != VariableNamed {
			t.Errorf("expected a named external token, got: %v", e.kind)
		}
		names = append(names, g.Pool.Resolve(e.name))
	}
	if !slices.Equal(names, []string{"y", "z"}) {
		t.Errorf("expected [y z], got: %v", names)
	}
}

// TestGrammarWithUndefinedSymbols is test_grammar_with_undefined_symbols in
// intern_symbols.rs.
func TestGrammarWithUndefinedSymbols(t *testing.T) {
	t.Parallel()
	// x refers to y, which is not defined
	pool := NewRulePool()
	yStr := pool.Intern("y")
	xRoot := pool.NamedSymbol(yStr)
	g := poolGrammar(pool, []Variable{{Name: pool.Intern("x"), Root: xRoot}})

	_, err := internSymbols(g, new([]Diagnostic))
	ie, ok := errors.AsType[*InternSymbolsError](err)
	if !ok || *ie != (InternSymbolsError{Kind: InternSymbolsUndefined, Name: "y"}) {
		t.Fatalf("expected the error that y is not defined, got: %v", err)
	}
}

// TestSupertypeAndInlineConflict is test_supertype_and_inline_conflict in
// intern_symbols.rs.
func TestSupertypeAndInlineConflict(t *testing.T) {
	t.Parallel()
	// v1: _v2
	// _v2: choice("a", "b")
	// supertypes: [_v2]
	// inline: [_v2]
	pool := NewRulePool()
	v2 := pool.Intern("_v2")
	v1Root := pool.NamedSymbol(v2)
	a, b := pool.Intern("a"), pool.Intern("b")
	v2Root := pool.Choice([]RuleID{pool.String(a), pool.String(b)})
	g := poolGrammar(pool, []Variable{
		{Name: pool.Intern("v1"), Root: v1Root},
		{Name: v2, Root: v2Root},
	})
	g.SupertypeNames = []StrID{v2}
	g.InlineNames = []StrID{v2}

	var diagnostics []Diagnostic
	meta, err := internSymbols(g, &diagnostics)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// the supertype entry is dropped with a warning, and the rule stays
	// inlined
	if len(meta.supertypes) != 0 {
		t.Errorf("expected no supertypes, got: %v", meta.supertypes)
	}
	if !slices.Equal(meta.inline, []Symbol{NonTerminalSymbol(1)}) {
		t.Errorf("expected inline [nt1], got: %v", meta.inline)
	}
	expected := []Diagnostic{{Kind: DiagnosticSupertypeInlined, Name: "_v2"}}
	if !slices.EqualFunc(diagnostics, expected, func(a, b Diagnostic) bool { return a.String() == b.String() }) {
		t.Errorf("expected %v, got: %v", expected, diagnostics)
	}
}

// TestMetaPaths is test_meta_paths in intern_symbols.rs.
func TestMetaPaths(t *testing.T) {
	t.Parallel()
	// x: "a"
	// y: "b"
	// supertypes: [y]
	// word: y
	// conflicts: [x, y]
	// inline: [x, nonexistent]
	pool := NewRulePool()
	a, b := pool.Intern("a"), pool.Intern("b")
	x, y := pool.Intern("x"), pool.Intern("y")
	xRoot := pool.String(a)
	yRoot := pool.String(b)
	g := poolGrammar(pool, []Variable{{Name: x, Root: xRoot}, {Name: y, Root: yRoot}})
	nonexistent := g.Pool.Intern("nonexistent")
	g.SupertypeNames = []StrID{y}
	g.WordName = y
	g.ConflictNames = [][]StrID{{x, y}}
	g.InlineNames = []StrID{x, nonexistent}

	meta, err := internSymbols(g, new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// y is a supertype, so it becomes hidden
	if expected := []VariableType{VariableNamed, VariableHidden}; !slices.Equal(meta.kinds, expected) {
		t.Errorf("expected kinds %v, got: %v", expected, meta.kinds)
	}
	if !slices.Equal(meta.supertypes, []Symbol{NonTerminalSymbol(1)}) {
		t.Errorf("expected supertypes [nt1], got: %v", meta.supertypes)
	}
	if !meta.hasWord || meta.word != NonTerminalSymbol(1) {
		t.Errorf("expected the word nt1, got: %v, %t", meta.word, meta.hasWord)
	}
	if len(meta.conflicts) != 1 || !slices.Equal(meta.conflicts[0], []Symbol{NonTerminalSymbol(0), NonTerminalSymbol(1)}) {
		t.Errorf("expected conflicts [[nt0 nt1]], got: %v", meta.conflicts)
	}
	// an inline name that is not defined is skipped, and no error says so
	if !slices.Equal(meta.inline, []Symbol{NonTerminalSymbol(0)}) {
		t.Errorf("expected inline [nt0], got: %v", meta.inline)
	}
}

// TestInternSymbolsWarnsOnUnaryRules makes sure that a seq or a choice of one
// string or pattern gives a warning with the name of its variable.
func TestInternSymbolsWarnsOnUnaryRules(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	a := pool.Intern("a")
	xRoot := pool.PushNode(Rule{Kind: RuleChoice, Children: pool.PushChildren([]RuleID{pool.String(a)})})
	g := poolGrammar(pool, []Variable{{Name: pool.Intern("x"), Root: xRoot}})
	g.ExtraRoots = []RuleID{pool.PushNode(Rule{Kind: RuleSeq, Children: pool.PushChildren([]RuleID{pool.String(a)})})}

	var diagnostics []Diagnostic
	if _, err := internSymbols(g, &diagnostics); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	var texts []string
	for _, d := range diagnostics {
		texts = append(texts, d.String())
	}
	expected := []string{
		"rule x contains a `choice` rule with a single element. this is unnecessary.",
		"rule <ANONYMOUS> contains a `seq` rule with a single element. this is unnecessary.",
	}
	if !slices.Equal(texts, expected) {
		t.Errorf("expected %q, got: %q", expected, texts)
	}
}

// poolGrammar returns a grammar with a pool and its variables.
//
// poolGrammar is pool_grammar in intern_symbols.rs.
func poolGrammar(pool *RulePool, variables []Variable) *InputGrammar {
	return &InputGrammar{Pool: pool, Variables: variables}
}

// nt returns the node of the non-terminal with an index.
func nt(index int) Rule {
	return Rule{Kind: RuleSym, Sym: NonTerminalSymbol(index)}
}

// ext returns the node of the external token with an index.
func ext(index int) Rule {
	return Rule{Kind: RuleSym, Sym: ExternalSymbol(index)}
}

// expectChildren fails the test unless a node has a kind and its children are
// the expected nodes.
func expectChildren(t *testing.T, pool *RulePool, id RuleID, kind RuleKind, expected []Rule) {
	t.Helper()
	n := pool.Node(id)
	if n.Kind != kind {
		t.Fatalf("expected kind %d, got: %v", kind, n)
	}
	var children []Rule
	for _, c := range pool.ChildSlice(n.Children) {
		children = append(children, pool.Node(c))
	}
	if !slices.Equal(children, expected) {
		t.Errorf("expected %v, got: %v", expected, children)
	}
}
