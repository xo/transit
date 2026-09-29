package cgrammar

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/transit"
)

// leaves returns the leaves of a tree that are tokens and have a width, in
// the order of the text. A node with only hidden children has no child in
// the API, and it is not a token.
func leaves(n transit.Node, tokenCount int) []transit.Node {
	if n.ChildCount() == 0 {
		if n.EndByte() > n.StartByte() && int(n.GrammarID()) < tokenCount {
			return []transit.Node{n}
		}
		return nil
	}
	var out []transit.Node
	for c := range n.Children() {
		out = append(out, leaves(c, tokenCount)...)
	}
	return out
}

// accepts reports whether one of the states has an action for a symbol.
func accepts(language *transit.Language, states []transit.StateID, symbol transit.Symbol) bool {
	for _, state := range states {
		it, ok := language.LookaheadIterator(state)
		if !ok {
			continue
		}
		for s := range it.Symbols() {
			if s == symbol {
				return true
			}
		}
	}
	return false
}

// TestFixtureStatesAtAcceptNextToken checks StatesAt on the corpus of each
// fixture grammar. For each input with no error, and for the start of each
// leaf, one of the states that StatesAt gives must have an action for the
// symbol of the leaf. When StatesAt gives one state, an offset inside the
// leaf must give the same state. With two stack versions, each version
// lexes with its own lex mode, and one of them can find a shorter token that
// ends inside the leaf. With the tree of the input as the old tree, StatesAt must give
// some of the same states. The parser can then reuse a node where it split
// into two stack versions without the old tree, so it can give fewer.
func TestFixtureStatesAtAcceptNextToken(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadFixture(t, cache, f)
			p := transit.NewParser()
			if err := p.SetLanguage(g.Language); err != nil {
				t.Fatal(err)
			}
			checked, missed, differ := 0, 0, 0
			for _, e := range examples[:min(len(examples), 40)] {
				tree, err := goParse(p, e.Input)
				if err != nil {
					t.Fatal(err)
				}
				if tree.RootNode().HasError() {
					continue
				}
				ls := leaves(tree.RootNode(), g.TokenCount())
				for _, leaf := range ls[:min(len(ls), 60)] {
					states, err := p.StatesAt(context.Background(), e.Input, leaf.StartByte(), nil)
					if err != nil {
						t.Fatal(err)
					}
					checked++
					if !accepts(g.Language, states, leaf.GrammarID()) {
						missed++
						if missed <= 3 {
							t.Errorf("%s: no state of %v at byte %d accepts %q", e.Name, states, leaf.StartByte(), leaf.GrammarKind())
						}
					}
					reused, err := p.StatesAt(context.Background(), e.Input, leaf.StartByte(), tree)
					if err != nil {
						t.Fatal(err)
					}
					if !accepts(g.Language, reused, leaf.GrammarID()) {
						missed++
						if missed <= 3 {
							t.Errorf("%s: with the old tree, no state of %v at byte %d accepts %q", e.Name, reused, leaf.StartByte(), leaf.GrammarKind())
						}
					}
					if !slices.Equal(reused, states) {
						differ++
					}
					if len(reused) == 0 || slices.ContainsFunc(reused, func(s transit.StateID) bool { return !slices.Contains(states, s) }) {
						t.Errorf("%s: at byte %d, the states are %v with the old tree and %v without it", e.Name, leaf.StartByte(), reused, states)
					}
					if len(states) == 1 && leaf.EndByte()-leaf.StartByte() > 1 {
						inside, err := p.StatesAt(context.Background(), e.Input, leaf.StartByte()+1, nil)
						if err != nil {
							t.Fatal(err)
						}
						if !slices.Equal(inside, states) {
							t.Errorf("%s: at byte %d, inside %q, the states are %v, want %v", e.Name, leaf.StartByte()+1, leaf.GrammarKind(), inside, states)
						}
					}
				}
			}
			t.Logf("%d offsets, %d missed, %d with other states from the old tree", checked, missed, differ)
		})
	}
}

// loadSQL builds and loads the SQL grammar of the C example.
func loadSQL(t *testing.T) *Grammar {
	t.Helper()
	_, cache := setup(t)
	dir := filepath.Join(cache, "grammars", "DerekStride", "tree-sitter-sql")
	so, err := BuildGrammar(context.Background(), dir, cache)
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(so, "sql")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// lookaheadNames returns the names of the symbols that the states accept,
// with no name twice.
func lookaheadNames(language *transit.Language, states []transit.StateID) []string {
	var out []string
	for _, state := range states {
		it, ok := language.LookaheadIterator(state)
		if !ok {
			continue
		}
		for s := range it.Symbols() {
			if name := language.SymbolName(s); !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	slices.Sort(out)
	return out
}

// selectFrom is the statement of the C example that is not finished.
const selectFrom = "SELECT * FROM "

// TestStatesAtSQL measures StatesAt against the cases of the working C
// example (D57). After "SELECT * FROM ", the parser recovers from an error,
// and the state at the end of the tree is 0, which accepts every symbol.
// StatesAt must give the state after FROM, which accepts the symbols of a
// table reference, as it does in "SELECT id FROM u".
func TestStatesAtSQL(t *testing.T) {
	t.Parallel()
	g := loadSQL(t)
	p := transit.NewParser()
	if err := p.SetLanguage(g.Language); err != nil {
		t.Fatal(err)
	}
	statesAt := func(src string, offset int) []transit.StateID {
		t.Helper()
		states, err := p.StatesAt(context.Background(), []byte(src), offset, nil)
		if err != nil {
			t.Fatal(err)
		}
		return states
	}

	all := lookaheadNames(g.Language, []transit.StateID{0})
	cut := statesAt(selectFrom, len(selectFrom))
	valid := statesAt("SELECT id FROM u", 15)
	cutNames := lookaheadNames(g.Language, cut)
	validNames := lookaheadNames(g.Language, valid)
	t.Logf("state 0 accepts %d symbols", len(all))
	t.Logf("after %q: states %v accept %d symbols: %v", selectFrom, cut, len(cutNames), cutNames)
	t.Logf("after %q: states %v accept %d symbols", "SELECT id FROM ", valid, len(validNames))
	if slices.Contains(cut, 0) || len(cutNames) == 0 || len(cutNames) >= len(all) {
		t.Errorf("after %q, the states %v accept %d of %d symbols", selectFrom, cut, len(cutNames), len(all))
	}
	if !slices.Equal(cutNames, validNames) {
		t.Errorf("after FROM, the symbols differ:\n  cut:   %v\n  valid: %v", cutNames, validNames)
	}
	if !slices.Contains(cutNames, "identifier") {
		t.Errorf("after FROM, the symbols do not hold identifier: %v", cutNames)
	}

	// The tree of the text is no help at the cursor: the parser recovered.
	tree, err := goParse(p, []byte(selectFrom))
	if err != nil {
		t.Fatal(err)
	}
	if !tree.RootNode().HasError() {
		t.Errorf("the tree of %q has no error: %s", selectFrom, tree.RootNode())
	}

	// The text after the cursor does not change the states at the cursor:
	// a word, the inside of a word, or another statement.
	for _, c := range []struct {
		src    string
		offset int
	}{
		{selectFrom + "users", 14},
		{selectFrom + "users", 16},
		{selectFrom + "users WHERE id = 1", 14},
		{selectFrom + "\nSELECT 1;", 14},
		{selectFrom + "-- a comment\n", 27},
	} {
		if got := statesAt(c.src, c.offset); !slices.Equal(got, cut) {
			t.Errorf("at byte %d of %q, the states are %v, want %v", c.offset, c.src, got, cut)
		}
	}
}

func TestStatesAtErrors(t *testing.T) {
	t.Parallel()
	g := loadSQL(t)
	p := transit.NewParser()
	if _, err := p.StatesAt(context.Background(), nil, 0, nil); !errors.Is(err, transit.ErrNoLanguage) {
		t.Errorf("with no language, the error is %v", err)
	}
	if err := p.SetLanguage(g.Language); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-1, 4} {
		if _, err := p.StatesAt(context.Background(), []byte("SEL"), offset, nil); !errors.Is(err, transit.ErrInvalidInput) {
			t.Errorf("at offset %d, the error is %v", offset, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := []byte("SELECT a FROM b; ")
	for range 8 {
		src = append(src, src...)
	}
	if _, err := p.StatesAt(ctx, src, len(src), nil); !errors.Is(err, context.Canceled) {
		t.Errorf("with a canceled context, the error is %v", err)
	}
	states, err := p.StatesAt(context.Background(), []byte("SELECT"), 0, nil)
	if err != nil || len(states) != 1 {
		t.Errorf("at the start, the states are %v, %v", states, err)
	}
}
