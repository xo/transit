package generate

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestBuildTablesOnEveryTestGrammar runs the generator up to BuildTables on
// each test grammar, with merging (abi15) and without (abi15-nomerge). A
// grammar that it rejects must have the same error in its golden file. For
// the others, it compares the counts in the golden parser.c that the tables
// decide: the parse states, the production ids, the longest aliased
// production, and the states of the main lexer and of the keyword lexer.
func TestBuildTablesOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	modes := []struct {
		dir  string
		opts OptLevel
	}{
		{"abi15", OptLevelMergeStates},
		{"abi15-nomerge", 0},
	}
	var rejected []string
	built := 0
	for _, f := range files {
		name := filepath.Base(filepath.Dir(f))
		for _, mode := range modes {
			tables, err := buildTablesForTest(t, f, mode.opts)
			dir := filepath.Join(filepath.Dir(f), mode.dir)
			if err != nil {
				if mode.dir == "abi15" {
					rejected = append(rejected, name)
				}
				golden, rerr := os.ReadFile(filepath.Join(dir, "error.txt"))
				if rerr != nil {
					t.Errorf("%s/%s: BuildTables gives %q, and the grammar has no error golden: %v", name, mode.dir, err, rerr)
					continue
				}
				if !strings.Contains(string(golden), "Caused by:\n"+indent(err.Error())+"\n") {
					t.Errorf("%s/%s: expected the error of the golden file, got: %q", name, mode.dir, err)
				}
				continue
			}
			built++
			parserC, rerr := os.ReadFile(filepath.Join(dir, "parser.c"))
			if rerr != nil {
				t.Errorf("%s/%s: BuildTables succeeds, and the grammar has no parser.c golden: %v", name, mode.dir, rerr)
				continue
			}
			c := string(parserC)
			pt := &tables.ParseTable
			for _, check := range []struct {
				what   string
				golden int
				actual int
			}{
				{"STATE_COUNT", cDefine(t, c, "STATE_COUNT"), len(pt.States)},
				{"PRODUCTION_ID_COUNT", cDefine(t, c, "PRODUCTION_ID_COUNT"), len(pt.ProductionInfos)},
				{"MAX_ALIAS_SEQUENCE_LENGTH", cDefine(t, c, "MAX_ALIAS_SEQUENCE_LENGTH"), pt.MaxAliasedProductionLength},
				{"the states of ts_lex", cLexStates(c, "ts_lex"), len(tables.MainLexTable.States)},
				{"the states of ts_lex_keywords", cLexStates(c, "ts_lex_keywords"), len(tables.KeywordLexTable.States)},
			} {
				if check.golden != check.actual {
					t.Errorf("%s/%s: %s: expected %d, got: %d", name, mode.dir, check.what, check.golden, check.actual)
				}
			}
		}
	}
	expected := []string{
		"associativity_missing",
		"conflict_in_repeat_rule",
		"conflict_in_repeat_rule_after_external_token",
		"conflicting_precedence",
		"eof_misplaced",
		"eof_repeat_via_nullable_rule",
		"epsilon_rules",
		"indirect_recursion_in_transitions",
		"invisible_start_rule",
		"partially_resolved_conflict",
		"precedence_on_single_child_missing",
		"terminal_supertype",
	}
	if !slices.Equal(rejected, expected) {
		t.Errorf("expected BuildTables to reject %v, it rejected %v", expected, rejected)
	}
	if built != 2*(len(files)-len(expected)) {
		t.Errorf("expected tables for %d grammars in 2 modes, got: %d", len(files)-len(expected), built)
	}
}

// buildTablesForTest runs the generator up to BuildTables on a grammar.json,
// in the order of generate.rs.
func buildTablesForTest(t *testing.T, path string, opts OptLevel) (*Tables, error) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []Diagnostic
	g, err := ParseGrammar(b, &diagnostics)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareGrammar(g, &diagnostics)
	if err != nil {
		return nil, err
	}
	variableInfo, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)
	if err != nil {
		return nil, err
	}
	return BuildTables(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, variableInfo, &prepared.Inlines, prepared.StrPool, opts, &diagnostics)
}

// cDefine returns the number of a #define of a golden parser.c.
func cDefine(t *testing.T, c, name string) int {
	t.Helper()
	m := regexp.MustCompile(`(?m)^#define ` + name + ` (\d+)$`).FindStringSubmatch(c)
	if m == nil {
		t.Fatalf("parser.c has no #define %s", name)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// cLexStates returns the number of states of a lexer function of a golden
// parser.c: the number of its case labels, or 0 when the file has no such
// function.
func cLexStates(c, function string) int {
	start := strings.Index(c, "static bool "+function+"(TSLexer *lexer, TSStateId state) {")
	if start < 0 {
		return 0
	}
	end := strings.Index(c[start:], "\n}\n")
	return len(regexp.MustCompile(`(?m)^    case \d+:`).FindAllString(c[start:start+end], -1))
}

// TestSymbolIndexerSymbolInvertsIndex checks that symbolIndexer.symbol gives
// back the symbol of each position, and that the positions follow the order
// of CompareSymbol. It is not an upstream test.
func TestSymbolIndexerSymbolInvertsIndex(t *testing.T) {
	t.Parallel()
	indexer := symbolIndexer{externalCount: 2, terminalCount: 3, nonTerminalCount: 2}
	symbols := []Symbol{
		ExternalSymbol(0),
		ExternalSymbol(1),
		SymbolEndValue,
		SymbolEndOfNonTerminalExtraValue,
		TerminalSymbol(0),
		TerminalSymbol(1),
		TerminalSymbol(2),
		NonTerminalSymbol(0),
		NonTerminalSymbol(1),
	}
	if !slices.IsSortedFunc(symbols, CompareSymbol) {
		t.Fatal("the symbols of the test are not in the order of CompareSymbol")
	}
	if n := int(indexer.symbolCount()); n != len(symbols) {
		t.Fatalf("symbolCount: got %d, want %d", n, len(symbols))
	}
	for i, symbol := range symbols {
		if index := indexer.index(symbol); index != i {
			t.Errorf("index(%v): got %d, want %d", symbol, index, i)
		}
		if got := indexer.symbol(i); got != symbol {
			t.Errorf("symbol(%d): got %v, want %v", i, got, symbol)
		}
	}
}
