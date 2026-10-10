package generate

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// This file tests build_parse_table.go. TestAuxiliaryContextLookup ports the
// one test of build_parse_table.rs. The other tests compare the port with
// the golden files.

// builtParseTableForTest is what buildParseTableForTest returns: the
// prepared grammar, the item set builder, and the results of
// BuildParseTable.
type builtParseTableForTest struct {
	prepared    *PreparedGrammar
	builder     *ParseItemSetBuilder
	table       ParseTable
	info        *ParseStateInfo
	diagnostics []Diagnostic
}

// buildParseTableForTest runs the generator on a test grammar up to
// BuildParseTable, with the calls and the arguments of generate.rs and
// build_tables.rs. It returns the error of the first step that fails, and
// then prepared is nil when the steps before BuildParseTable fail.
func buildParseTableForTest(t *testing.T, file string) (*builtParseTableForTest, error) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []Diagnostic
	g, err := ParseGrammar(b, &diagnostics)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	result := &builtParseTableForTest{}
	prepared, err := PrepareGrammar(g, &diagnostics)
	if err != nil {
		return result, err
	}
	variableInfo, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)
	if err != nil {
		return result, err
	}
	result.prepared = prepared
	keyMap := NewItemKeyMap(&prepared.SyntaxGrammar, prepared.StrPool)
	result.builder = NewParseItemSetBuilder(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, &prepared.Inlines, keyMap)
	result.table, result.info, err = BuildParseTable(
		&prepared.SyntaxGrammar,
		&prepared.LexicalGrammar,
		result.builder,
		variableInfo,
		prepared.StrPool,
		&result.diagnostics,
	)
	return result, err
}

// TestBuildParseTableOnEveryTestGrammar runs BuildParseTable on each test
// grammar. A grammar that it rejects must have the same error, byte for
// byte, in its golden file. Every other grammar must build a table.
func TestBuildParseTableOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rejected, prepareRejected []string
	for _, f := range files {
		dir := filepath.Dir(f)
		name := filepath.Base(dir)
		built, err := buildParseTableForTest(t, f)
		var buildErr *ParseTableBuilderError
		switch {
		case err == nil:
			if n := len(built.table.States); n < 2 || len(built.info.PrecedingSymbolsByID) != n {
				t.Errorf("%s: expected a table with the error state and the start state, got %d states", name, n)
			}
			for _, d := range built.diagnostics {
				t.Logf("%s: %s", name, d.String())
			}
			continue
		case !errors.As(err, &buildErr):
			if built.prepared == nil {
				prepareRejected = append(prepareRejected, name)
				continue
			}
			t.Errorf("%s: expected a *ParseTableBuilderError, got %T: %v", name, err, err)
			continue
		}
		rejected = append(rejected, name)
		golden, rerr := os.ReadFile(filepath.Join(dir, "abi15", "error.txt"))
		if rerr != nil {
			t.Errorf("%s: BuildParseTable gives %q, and the grammar has no error golden: %v", name, err, rerr)
			continue
		}
		expected := "Error: Error when generating parser\n\nCaused by:\n" + indent(err.Error()) + "\n"
		if string(golden) != expected {
			t.Errorf("%s: expected the error of the golden file:\n%s\ngot:\n%s", name, golden, expected)
		}
	}
	expected := []string{
		"associativity_missing",
		"conflict_in_repeat_rule",
		"conflict_in_repeat_rule_after_external_token",
		"conflicting_precedence",
		"partially_resolved_conflict",
		"precedence_on_single_child_missing",
	}
	if !slices.Equal(rejected, expected) {
		t.Errorf("expected BuildParseTable to reject %v, it rejected %v", expected, rejected)
	}
	expectedPrepare := []string{
		"eof_misplaced",
		"eof_repeat_via_nullable_rule",
		"epsilon_rules",
		"indirect_recursion_in_transitions",
		"invisible_start_rule",
		"terminal_supertype",
	}
	if !slices.Equal(prepareRejected, expectedPrepare) {
		t.Errorf("expected the steps before BuildParseTable to reject %v, they rejected %v", expectedPrepare, prepareRejected)
	}
}

// TestParseTableBuilderErrorText checks the text of each kind of error, and
// the diagnostic of the conflicts that a grammar does not need.
func TestParseTableBuilderErrorText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      *ParseTableBuilderError
		expected string
	}{
		{
			&ParseTableBuilderError{Kind: ParseTableBuilderAmbiguousExtra, AmbiguousExtra: &AmbiguousExtraError{ParentSymbols: []string{"a", "b"}}},
			"Extra rules must have unambiguous endings. Conflicting rules: a, b",
		},
		{
			&ParseTableBuilderError{Kind: ParseTableBuilderImproperNonTerminalExtra, Name: "x"},
			"The non-terminal rule `x` is used in a non-terminal `extra` rule, which is not allowed.",
		},
		{
			&ParseTableBuilderError{Kind: ParseTableBuilderStateCount, StateCount: 70000},
			"State count `70000` exceeds the max value 65535.",
		},
		{
			&ParseTableBuilderError{Kind: ParseTableBuilderConflict, Conflict: &ConflictError{
				SymbolSequence:       []string{"a"},
				ConflictingLookahead: "'b'",
				PossibleInterpretations: []Interpretation{
					{VariableName: "y", ProductionStepSymbols: []string{"a"}, StepIndex: 1, Done: true, ConflictingLookahead: "'b'", RequiresEOFLookahead: true},
					{VariableName: "x", ProductionStepSymbols: []string{"a", "'b'"}, StepIndex: 1, Precedence: "'p'", HasPrecedence: true},
				},
				PossibleResolutions: []Resolution{
					{Kind: ResolutionPrecedence, Symbols: []string{"x", "y"}},
					{Kind: ResolutionAssociativity, Symbols: []string{"x", "y"}},
					{Kind: ResolutionAddConflict, Symbols: []string{"x", "y"}},
				},
			}},
			"Unresolved conflict for symbol sequence:\n\n" +
				"  a  •  'b'  …\n\n" +
				"Possible interpretations:\n\n" +
				"  1:  (x  a  •  'b')     (precedence: 'p')\n" +
				"  2:  (y  a)  •  'b'  …  (reduces only at end of input)\n" +
				"\nPossible resolutions:\n\n" +
				"  1:  Specify a higher precedence in `x` and `y` than in the other rules.\n" +
				"  2:  Specify a left or right associativity in `x`, `y`\n" +
				"  3:  Add a conflict for these rules: `x`, `y`\n",
		},
	}
	for _, test := range tests {
		if actual := test.err.Error(); actual != test.expected {
			t.Errorf("expected %q, got %q", test.expected, actual)
		}
	}
	d := Diagnostic{Kind: DiagnosticUnnecessaryConflicts, Conflicts: [][]string{{"a", "b"}, {"c"}}}
	if expected, actual := "unnecessary conflicts:\n  `a`, `b`\n  `c`", d.String(); actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

// TestAuxiliaryContextLookup is test_auxiliary_context_lookup.
func TestAuxiliaryContextLookup(t *testing.T) {
	//   source_file: $ => seq(repeat('x'), $.block),
	//   block: $ => seq('{', repeat('y'), '}', repeat('x')),
	variable := func(kind VariableType) SyntaxVariable {
		return SyntaxVariable{Name: EmptyStrID, Kind: kind}
	}
	grammar := &SyntaxGrammar{
		Variables: []SyntaxVariable{
			variable(VariableNamed),
			variable(VariableNamed),
			variable(VariableAuxiliary),
			variable(VariableAuxiliary),
		},
	}
	sourceFile, block, xRepeat, yRepeat := NonTerminalIndex(0), NonTerminalIndex(1), NonTerminalIndex(2), NonTerminalIndex(3)

	// The contexts of three states along one path through the grammar:
	//
	//   source_file: • repeat('x') block
	//   block: '{' • repeat('y') '}' repeat('x')
	//   block: '{' repeat('y') '}' • repeat('x')
	var contexts auxiliarySymbolContexts
	first := contexts.push(grammar, 0, []auxiliaryUse{{xRepeat, sourceFile}})
	second := contexts.push(grammar, first, []auxiliaryUse{{yRepeat, block}})
	third := contexts.push(grammar, second, []auxiliaryUse{{xRepeat, block}})

	tests := []struct {
		name     string
		context  auxiliaryContextID
		symbol   NonTerminalIndex
		expected []Symbol
		ok       bool
	}{
		// The nearest context has the symbol.
		{"nearest", second, yRepeat, []Symbol{block.Symbol()}, true},
		// Only a predecessor has it.
		{"predecessor", second, xRepeat, []Symbol{sourceFile.Symbol()}, true},
		// A more recent entry hides an earlier one.
		{"hidden", third, xRepeat, []Symbol{block.Symbol()}, true},
		// Nothing along the path recorded it.
		{"none", first, yRepeat, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, ok := contexts.parents(test.context, test.symbol)
			if ok != test.ok || !slices.Equal(actual, test.expected) {
				t.Errorf("expected %v, %t, got %v, %t", test.expected, test.ok, actual, ok)
			}
		})
	}
}
