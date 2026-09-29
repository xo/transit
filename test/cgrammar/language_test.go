package cgrammar

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/xo/transit"
)

// This file ports crates/cli/src/tests/language_test.rs of upstream (D35).
// test_lookahead_iterator_modifiable_only_by_mut tests a rule of the Rust
// borrow checker, so it has no Go form. The Go API does not export
// current_symbol, so test_lookahead_iterator_exhaustion checks the same
// states through Symbols and Names.

// fixtureGrammar builds and loads the fixture grammar with a name.
func fixtureGrammar(t *testing.T, name string) *Grammar {
	t.Helper()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		if f.Name == name {
			g, _ := loadFixture(t, cache, f)
			return g
		}
	}
	t.Fatalf("no fixture grammar %s in grammars/grammars.json", name)
	return nil
}

// cursorAtStructKeyword parses "struct Stuff {}" with the Rust grammar and
// returns a cursor at the keyword struct.
func cursorAtStructKeyword(t *testing.T, g *Grammar) *transit.TreeCursor {
	t.Helper()
	p := transit.NewParser()
	if err := p.SetLanguage(g.Language); err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse(context.Background(), []byte("struct Stuff {}"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cursor := tree.Walk()
	if !cursor.GotoFirstChild() { // struct
		t.Fatal("GotoFirstChild to struct_item returned false")
	}
	if !cursor.GotoFirstChild() { // struct keyword
		t.Fatal("GotoFirstChild to the keyword returned false")
	}
	return cursor
}

func TestLookaheadIterator(t *testing.T) {
	g := fixtureGrammar(t, "rust")
	language := g.Language
	cursor := cursorAtStructKeyword(t, g)

	nextState := cursor.Node().NextParseState()
	if nextState == 0 {
		t.Fatal("NextParseState of the keyword struct is 0")
	}
	if want := language.NextState(cursor.Node().ParseState(), cursor.Node().GrammarID()); nextState != want {
		t.Errorf("NextParseState() = %d, and NextState gives %d", nextState, want)
	}
	if int(nextState) >= language.StateCount() {
		t.Errorf("NextParseState() = %d, past the state count %d", nextState, language.StateCount())
	}
	if !cursor.GotoNextSibling() { // type_identifier
		t.Fatal("GotoNextSibling returned false")
	}
	if got := cursor.Node().ParseState(); got != nextState {
		t.Errorf("the parse state of the name is %d, want %d", got, nextState)
	}
	if got := cursor.Node().GrammarKind(); got != "identifier" {
		t.Errorf("GrammarKind() = %q, want identifier", got)
	}
	if cursor.Node().GrammarID() == cursor.Node().KindID() {
		t.Error("the grammar symbol of the name is its symbol, want an alias")
	}

	expectedSymbols := []string{"//", "/*", "identifier", "line_comment", "block_comment"}
	lookahead, ok := language.LookaheadIterator(nextState)
	if !ok {
		t.Fatal("LookaheadIterator returned false")
	}
	if lookahead.Language() != language {
		t.Error("the iterator has another language")
	}
	if got := slices.Collect(lookahead.Names()); !slices.Equal(got, expectedSymbols) {
		t.Errorf("Names() = %q, want %q", got, expectedSymbols)
	}
	if got := slices.Collect(lookahead.Names()); len(got) != 0 {
		t.Errorf("Names() of an exhausted iterator = %q", got)
	}

	if !lookahead.ResetState(nextState) {
		t.Fatal("ResetState returned false")
	}
	if got := slices.Collect(lookahead.Names()); !slices.Equal(got, expectedSymbols) {
		t.Errorf("Names() after ResetState = %q, want %q", got, expectedSymbols)
	}

	if !lookahead.Reset(language, nextState) {
		t.Fatal("Reset returned false")
	}
	var names []string
	for s := range lookahead.Symbols() {
		names = append(names, language.SymbolName(s))
	}
	if !slices.Equal(names, expectedSymbols) {
		t.Errorf("the names of Symbols() = %q, want %q", names, expectedSymbols)
	}
}

func TestLookaheadIteratorExhaustion(t *testing.T) {
	language := fixtureGrammar(t, "json").Language
	for state := range language.StateCount() {
		lookahead, ok := language.LookaheadIterator(transit.StateID(state))
		if !ok {
			t.Fatalf("LookaheadIterator(%d) returned false", state)
		}
		count := len(slices.Collect(lookahead.Symbols()))

		// An exhausted iterator stays exhausted.
		if n := len(slices.Collect(lookahead.Symbols())); n != 0 {
			t.Errorf("state %d: an exhausted iterator gave %d symbols", state, n)
		}
		if n := len(slices.Collect(lookahead.Names())); n != 0 {
			t.Errorf("state %d: an exhausted iterator gave %d names", state, n)
		}

		// Resetting restores it exactly.
		if !lookahead.ResetState(transit.StateID(state)) {
			t.Fatalf("ResetState(%d) returned false", state)
		}
		if n := len(slices.Collect(lookahead.Symbols())); n != count {
			t.Errorf("state %d: %d symbols after ResetState, want %d", state, n, count)
		}
	}
}

func TestSymbolMetadataChecks(t *testing.T) {
	language := fixtureGrammar(t, "rust").Language
	for i := range language.SymbolCount() {
		sym := transit.Symbol(i)
		typ := language.SymbolType(sym)
		switch name := language.SymbolName(sym); name {
		case "_type", "_expression", "_pattern", "_literal", "_literal_pattern", "_declaration_statement":
			if typ != transit.SymbolSupertype {
				t.Errorf("%s is %v, want a supertype", name, typ)
			}
		case "_raw_string_literal_start", "_raw_string_literal_end", "_line_doc_comment", "_error_sentinel":
			if typ == transit.SymbolSupertype {
				t.Errorf("%s is a supertype", name)
			}
		case "enum_item", "struct_item", "type_item":
			if typ != transit.SymbolRegular {
				t.Errorf("%s is %v, want named", name, typ)
			}
		case "=>", "[", "]", "(", ")", "{", "}":
			if typ != transit.SymbolRegular && typ != transit.SymbolAnonymous {
				t.Errorf("%s is %v, want visible", name, typ)
			}
		}
	}
}

func TestSupertypes(t *testing.T) {
	language := fixtureGrammar(t, "rust").Language
	supertypes := language.Supertypes()
	if language.ABIVersion() < 15 {
		return
	}

	var names []string
	for _, s := range supertypes {
		names = append(names, language.SymbolName(s))
	}
	want := []string{"_expression", "_literal", "_literal_pattern", "_pattern", "_type"}
	if !slices.Equal(names, want) {
		t.Fatalf("the supertypes are %q, want %q", names, want)
	}

	for _, supertype := range supertypes {
		var subtypes []string
		for _, s := range language.Subtypes(supertype) {
			subtypes = append(subtypes, language.SymbolName(s))
		}
		sort.Strings(subtypes)
		subtypes = slices.Compact(subtypes)

		var want []string
		switch language.SymbolName(supertype) {
		case "_literal":
			want = []string{
				"boolean_literal",
				"char_literal",
				"float_literal",
				"integer_literal",
				"raw_string_literal",
				"string_literal",
			}
		case "_pattern":
			want = []string{
				"_",
				"_literal_pattern",
				"captured_pattern",
				"const_block",
				"generic_pattern",
				"identifier",
				"macro_invocation",
				"mut_pattern",
				"or_pattern",
				"range_pattern",
				"ref_pattern",
				"reference_pattern",
				"remaining_field_pattern",
				"scoped_identifier",
				"slice_pattern",
				"struct_pattern",
				"tuple_pattern",
				"tuple_struct_pattern",
			}
		case "_type":
			want = []string{
				"abstract_type",
				"array_type",
				"bounded_type",
				"dynamic_type",
				"function_type",
				"generic_type",
				"macro_invocation",
				"metavariable",
				"never_type",
				"pointer_type",
				"primitive_type",
				"reference_type",
				"removed_trait_bound",
				"scoped_type_identifier",
				"tuple_type",
				"type_identifier",
				"unit_type",
			}
		default:
			continue
		}
		if !slices.Equal(subtypes, want) {
			t.Errorf("the subtypes of %s are %q, want %q", language.SymbolName(supertype), subtypes, want)
		}
	}
}
