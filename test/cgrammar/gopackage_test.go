package cgrammar

import (
	"context"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	golang "github.com/xo/transit/generate/backend/go"
	"github.com/xo/transit/grammars/json"
	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/grammartest"
)

// This file runs the tests of phase 3 on the grammar packages that the Go
// backend writes. Each test compares a Go package with the C grammar that
// the C backend writes from the same grammar.json, with the same version:
// every table field, every call of the lex functions, the tree of each
// corpus input and each edit of it.

// goPackage is a grammar package that the Go backend writes, and the
// fixture grammar that it comes from.
type goPackage struct {
	name     string
	language func() *transit.Language
}

// goPackages are the grammar packages in grammars/.
var goPackages = []goPackage{
	{"json", json.Language},
}

// loadGoPackage builds and loads the C grammar of a grammar package, with
// the version of its tree-sitter.json, and reads the corpus of the grammar
// and the error corpus of upstream.
func loadGoPackage(t *testing.T, gp goPackage) (*Grammar, []Example) {
	t.Helper()
	root, cache := setup(t)
	var f fixture
	for _, candidate := range fixtures(t, root) {
		if candidate.Name == gp.name {
			f = candidate
		}
	}
	if f.Name == "" {
		t.Fatalf("no fixture grammar %s in grammars/grammars.json", gp.name)
	}
	dir, corpusDir := grammarDirs(cache, f)
	so, err := BuildGrammarVersion(context.Background(), dir, cache)
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(so, f.Name)
	if err != nil {
		t.Fatal(err)
	}
	examples, err := ReadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	return g, append(examples, errorCorpus(t, root, f.Name)...)
}

// tablesOf returns the tables of a language. The field is not exported, so
// the test reads it with reflect and unsafe.
func tablesOf(l *transit.Language) *abi.Language {
	v := reflect.ValueOf(l).Elem().FieldByName("tables")
	return (*abi.Language)(unsafe.Pointer(v.UnsafeAddr()))
}

// compareTables returns the names of the table fields of two languages that
// differ, but for the function fields. A slice that is nil and a slice that
// is empty are equal. A parse action is compared in the member of the C
// union that its type uses: Shift for a shift, Reduce for a reduce, and the
// type only for an accept and a recover, because the C union also holds the
// bytes of the other member.
func compareTables(want, got *abi.Language) []string {
	var diffs []string
	eq := func(name string, ok bool) {
		if !ok {
			diffs = append(diffs, name)
		}
	}
	eq("ABIVersion", want.ABIVersion == got.ABIVersion)
	eq("SymbolCount", want.SymbolCount == got.SymbolCount)
	eq("AliasCount", want.AliasCount == got.AliasCount)
	eq("TokenCount", want.TokenCount == got.TokenCount)
	eq("ExternalTokenCount", want.ExternalTokenCount == got.ExternalTokenCount)
	eq("StateCount", want.StateCount == got.StateCount)
	eq("LargeStateCount", want.LargeStateCount == got.LargeStateCount)
	eq("ProductionIDCount", want.ProductionIDCount == got.ProductionIDCount)
	eq("FieldCount", want.FieldCount == got.FieldCount)
	eq("MaxAliasSequenceLength", want.MaxAliasSequenceLength == got.MaxAliasSequenceLength)
	eq("ParseTable", slices.Equal(want.ParseTable, got.ParseTable))
	eq("SmallParseTable", slices.Equal(want.SmallParseTable, got.SmallParseTable))
	eq("SmallParseTableMap", slices.Equal(want.SmallParseTableMap, got.SmallParseTableMap))
	eq("ParseActions", sameParseActions(want.ParseActions, got.ParseActions))
	eq("SymbolNames", slices.Equal(want.SymbolNames, got.SymbolNames))
	eq("FieldNames", slices.Equal(want.FieldNames, got.FieldNames))
	eq("FieldMapSlices", slices.Equal(want.FieldMapSlices, got.FieldMapSlices))
	eq("FieldMapEntries", slices.Equal(want.FieldMapEntries, got.FieldMapEntries))
	eq("SymbolMetadata", slices.Equal(want.SymbolMetadata, got.SymbolMetadata))
	eq("PublicSymbolMap", slices.Equal(want.PublicSymbolMap, got.PublicSymbolMap))
	eq("AliasMap", slices.Equal(want.AliasMap, got.AliasMap))
	eq("AliasSequences", slices.Equal(want.AliasSequences, got.AliasSequences))
	eq("LexModes", slices.Equal(want.LexModes, got.LexModes))
	eq("LexFn", (want.LexFn == nil) == (got.LexFn == nil))
	eq("KeywordLexFn", (want.KeywordLexFn == nil) == (got.KeywordLexFn == nil))
	eq("KeywordCaptureToken", want.KeywordCaptureToken == got.KeywordCaptureToken)
	eq("ExternalScanner.States", slices.Equal(want.ExternalScanner.States, got.ExternalScanner.States))
	eq("ExternalScanner.SymbolMap", slices.Equal(want.ExternalScanner.SymbolMap, got.ExternalScanner.SymbolMap))
	eq("ExternalScanner.Create", (want.ExternalScanner.Create == nil) == (got.ExternalScanner.Create == nil))
	eq("PrimaryStateIDs", slices.Equal(want.PrimaryStateIDs, got.PrimaryStateIDs))
	eq("Name", want.Name == got.Name)
	eq("ReservedWords", slices.Equal(want.ReservedWords, got.ReservedWords))
	eq("MaxReservedWordSetSize", want.MaxReservedWordSetSize == got.MaxReservedWordSetSize)
	eq("SupertypeCount", want.SupertypeCount == got.SupertypeCount)
	eq("SupertypeSymbols", slices.Equal(want.SupertypeSymbols, got.SupertypeSymbols))
	eq("SupertypeMapSlices", slices.Equal(want.SupertypeMapSlices, got.SupertypeMapSlices))
	eq("SupertypeMapEntries", slices.Equal(want.SupertypeMapEntries, got.SupertypeMapEntries))
	eq("Metadata", want.Metadata == got.Metadata)
	return diffs
}

// sameParseActions compares two tables of parse actions, group by group.
func sameParseActions(want, got []abi.ParseActionEntry) bool {
	if len(want) != len(got) {
		return false
	}
	for i := 0; i < len(want); {
		if want[i].Entry != got[i].Entry {
			return false
		}
		count := int(want[i].Entry.Count)
		for k := i + 1; k <= i+count; k++ {
			a, b := want[k].Action, got[k].Action
			if a.Type != b.Type ||
				a.Type == abi.ParseActionTypeShift && a.Shift != b.Shift ||
				a.Type == abi.ParseActionTypeReduce && a.Reduce != b.Reduce {
				return false
			}
		}
		i += 1 + count
	}
	return true
}

// TestGoPackageTablesMatchC compares every table field of each grammar
// package with the tables of its C grammar.
func TestGoPackageTablesMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range goPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			if diffs := compareTables(tablesOf(g.Language), tablesOf(gp.language())); len(diffs) > 0 {
				t.Errorf("the tables differ: %v", diffs)
			}
		})
	}
}

// lexLog is a lexer over a text that records each call of a lex function:
// 1 for Advance, 2 for Advance with skip, and -1-p for MarkEnd at the
// position p. It decodes UTF-8, with the lookahead -1 for an invalid byte,
// and the lookahead 0 at the end.
type lexLog struct {
	abi.Lexer

	text []byte
	pos  int
	size int
	log  []int
}

// reset moves the lexer to pos, for a new token.
func (l *lexLog) reset(pos int) {
	l.pos, l.log = pos, nil
	l.ResultSymbol = 0
	l.decode()
}

// decode sets the lookahead at the position.
func (l *lexLog) decode() {
	if l.pos >= len(l.text) {
		l.Lookahead, l.size = 0, 0
		return
	}
	r, size := utf8.DecodeRune(l.text[l.pos:])
	if r == utf8.RuneError && size <= 1 {
		r = -1
	}
	l.Lookahead, l.size = r, size
}

// Advance implements abi.LexerFuncs.
func (l *lexLog) Advance(skip bool) {
	if skip {
		l.log = append(l.log, 2)
	} else {
		l.log = append(l.log, 1)
	}
	l.pos += l.size
	l.decode()
}

// MarkEnd implements abi.LexerFuncs.
func (l *lexLog) MarkEnd() {
	l.log = append(l.log, -1-l.pos)
}

// GetColumn implements abi.LexerFuncs.
func (l *lexLog) GetColumn() uint32 {
	return 0
}

// IsAtIncludedRangeStart implements abi.LexerFuncs.
func (l *lexLog) IsAtIncludedRangeStart() bool {
	return false
}

// EOF implements abi.LexerFuncs.
func (l *lexLog) EOF() bool {
	return l.pos >= len(l.text)
}

// Logf implements abi.LexerFuncs.
func (l *lexLog) Logf(string, ...any) {}

// lexCall runs a lex function in a state from a position, and returns what
// it found and its calls.
func lexCall(l *lexLog, fn abi.LexFunc, pos int, state uint16) string {
	l.reset(pos)
	found := fn(&l.Lexer, state)
	return fmt.Sprint(found, l.ResultSymbol, l.log)
}

// compareLexFuncs runs two lex functions in each state from each position
// of each text, and returns the number of runs and the first difference. It
// stops after limit runs.
func compareLexFuncs(want, got abi.LexFunc, states int, texts [][]byte, limit int) (int, string) {
	runs := 0
	for ti, text := range texts {
		l := &lexLog{text: text}
		l.Funcs = l
		for pos := 0; pos <= len(text); pos++ {
			for state := range states {
				if runs == limit {
					return runs, ""
				}
				runs++
				w := lexCall(l, want, pos, uint16(state))
				g := lexCall(l, got, pos, uint16(state))
				if w != g {
					return runs, fmt.Sprintf("text %d at %d in state %d: C %s, Go %s", ti, pos, state, w, g)
				}
			}
		}
	}
	return runs, ""
}

// lexTexts returns the texts of the comparison of two lexers: some bytes
// that are not UTF-8, some characters outside ASCII, and the corpus inputs.
func lexTexts(examples []Example) [][]byte {
	texts := [][]byte{{0xff, 'a', 0xc3, '"', '\\', 0}, []byte("\u00e9 \U0001F600\t\r\n")}
	for _, e := range examples {
		texts = append(texts, e.Input)
	}
	return texts
}

// compareLexers runs the main lex function and the keyword lex function of
// two tables in each of their states and in one more state that the tables
// do not have, from each position of each text. It stops each comparison
// after limit runs.
func compareLexers(t *testing.T, want, got *abi.Language, counts lexCounts, texts [][]byte, limit int) {
	t.Helper()
	for _, fn := range []struct {
		name      string
		want, got abi.LexFunc
		states    int
	}{
		{"main", want.LexFn, got.LexFn, counts.main + 1},
		{"keyword", want.KeywordLexFn, got.KeywordLexFn, counts.keyword + 1},
	} {
		if fn.want == nil || fn.got == nil {
			if (fn.want == nil) != (fn.got == nil) {
				t.Errorf("%s: only one of the grammars has the lex function", fn.name)
			}
			continue
		}
		runs, d := compareLexFuncs(fn.want, fn.got, fn.states, texts, limit)
		if d != "" {
			t.Errorf("%s: %s", fn.name, d)
		}
		t.Logf("%s: %d runs", fn.name, runs)
	}
}

// TestGoPackageLexersMatchC runs the lex functions of each grammar package
// and of its C grammar in each lex state, and in one more state that the
// tables do not have, from each position of each corpus input and of some
// bytes that are not UTF-8. It compares what they find and each call of the
// lexer.
func TestGoPackageLexersMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range goPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			root, cache := setup(t)
			var dir string
			for _, f := range fixtures(t, root) {
				if f.Name == gp.name {
					dir, _ = grammarDirs(cache, f)
				}
			}
			b := generateGo(t, dir)
			compareLexers(t, tablesOf(g.Language), tablesOf(gp.language()), b.counts, lexTexts(examples), -1)
		})
	}
}

// lexCounts is the number of the states of the main lex table and of the
// keyword lex table of a grammar.
type lexCounts struct {
	main, keyword int
}

// goBackend is a backend that keeps the tables that the Go backend writes,
// its parser.go and the number of the lex states.
type goBackend struct {
	tables *abi.Language
	code   string
	counts lexCounts
}

// Render keeps the tables and parser.go, and returns parser.go. A grammar
// whose name is not the name of a Go package, such as go, gets no
// parser.go.
func (b *goBackend) Render(in *generate.RenderInput) (string, error) {
	b.counts = lexCounts{len(in.Tables.MainLexTable.States), len(in.Tables.KeywordLexTable.States)}
	tables, err := golang.Tables(in)
	if err != nil {
		return "", err
	}
	b.tables = tables
	if _, nameErr := golang.PackageName(in.StrPool.Resolve(in.Name)); nameErr == nil {
		if b.code, err = (golang.Backend{Queries: true}).Render(in); err != nil {
			return "", err
		}
	}
	return b.code, nil
}

// generateGo runs the generator with the Go backend on the grammar in dir,
// at ABI 15 and with the version 0.0.0, as BuildGrammar does.
func generateGo(t *testing.T, dir string) *goBackend {
	t.Helper()
	grammarJSON, err := os.ReadFile(filepath.Join(dir, "src", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	return generateGoJSON(t, grammarJSON)
}

// generateGoJSON runs the generator with the Go backend on a grammar.json,
// at ABI 15 and with the version 0.0.0.
func generateGoJSON(t *testing.T, grammarJSON []byte) *goBackend {
	t.Helper()
	var diagnostics []generate.Diagnostic
	b := &goBackend{}
	if _, _, err := generate.ParserForGrammar(grammarJSON, nil, generate.OptLevelMergeStates, b, &diagnostics); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestGoBackendMatchesC runs the Go backend on each fixture grammar, and
// compares its tables with the tables of the C grammar: every table field,
// the calls of the lex functions, and the tree of each corpus input. The
// external scanner of the Go tables is the C scanner, through cgo, because
// the Go ports of the scanners come later in phase 4. It also makes sure
// that gofmt leaves the parser.go of the grammar as it is.
func TestGoBackendMatchesC(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadFixture(t, cache, f)
			examples = append(examples, errorCorpus(t, root, f.Name)...)
			dir, _ := grammarDirs(cache, f)
			b := generateGo(t, dir)
			want := tablesOf(g.Language)
			got := b.tables
			got.ExternalScanner.Create = want.ExternalScanner.Create
			if diffs := compareTables(want, got); len(diffs) > 0 {
				t.Errorf("the tables differ: %v", diffs)
			}
			if b.code == "" {
				t.Logf("the grammar %s has no Go package name, so the test does not format its parser.go", f.Name)
			} else if formatted, err := format.Source([]byte(b.code)); err != nil || string(formatted) != b.code {
				t.Errorf("gofmt changes parser.go, or it is not valid Go: %v", err)
			}
			compareLexers(t, want, got, b.counts, lexTexts(examples), 200000)

			language := transit.NewLanguage(got)
			p := transit.NewParser()
			if err := p.SetLanguage(language); err != nil {
				t.Fatal(err)
			}
			failures := 0
			for _, e := range examples {
				wantTree, wantString, err := g.CParse(e.Input)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := goParse(p, e.Input)
				if err != nil {
					failures++
					t.Errorf("%s: %s: %v", filepath.Base(e.File), e.Name, err)
					p = transit.NewParser()
					if err := p.SetLanguage(language); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if d := Diff(wantTree, GoSnapshot(tree.RootNode(), "")); d != "" {
					failures++
					t.Errorf("%s: %s: the trees differ at %s", filepath.Base(e.File), e.Name, d)
					continue
				}
				if gotString := tree.RootNode().String(); gotString != wantString {
					failures++
					t.Errorf("%s: %s: the text of the trees differs", filepath.Base(e.File), e.Name)
				}
			}
			t.Logf("%d inputs, %d differ", len(examples), failures)
		})
	}
}

// TestGoBackendMatchesCOnTestGrammars runs the Go backend on each test
// grammar of generate/testdata that the generator writes, and compares its
// tables and the calls of its lex functions with those of the C grammar.
func TestGoBackendMatchesCOnTestGrammars(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	files, err := filepath.Glob(filepath.Join(root, "generate", "testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	texts := append(lexTexts(nil), []byte("abc 123 (x) {y} [z] \"s\" 'c' + - * / = ; , . # @ \\ \n\t"))
	for _, file := range files {
		name := filepath.Base(filepath.Dir(file))
		if _, err := os.Stat(filepath.Join(filepath.Dir(file), "abi15", "error.txt")); err == nil {
			// the generator rejects the grammar
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			grammarJSON, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			_, scannerDir := urTestGrammarDirs(root, name)
			so, grammarName, err := BuildGrammarJSON(context.Background(), grammarJSON, scannerDir, cache)
			if err != nil {
				t.Fatal(err)
			}
			g, err := Load(so, grammarName)
			if err != nil {
				t.Fatal(err)
			}
			b := generateGoJSON(t, grammarJSON)
			want := tablesOf(g.Language)
			b.tables.ExternalScanner.Create = want.ExternalScanner.Create
			if diffs := compareTables(want, b.tables); len(diffs) > 0 {
				t.Errorf("the tables differ: %v", diffs)
			}
			compareLexers(t, want, b.tables, b.counts, texts, 200000)
		})
	}
}

// TestGoPackageTreesMatchC parses each corpus input of each grammar package
// with the Go runtime and the Go package, and with the C runtime and the C
// grammar, and compares the two trees node by node, and their text.
func TestGoPackageTreesMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range goPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			p := transit.NewParser()
			if err := p.SetLanguage(gp.language()); err != nil {
				t.Fatal(err)
			}
			failures := 0
			for _, e := range examples {
				want, wantString, err := g.CParse(e.Input)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := goParse(p, e.Input)
				if err != nil {
					failures++
					t.Errorf("%s: %s: %v", filepath.Base(e.File), e.Name, err)
					p = transit.NewParser()
					if err := p.SetLanguage(gp.language()); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if d := Diff(want, GoSnapshot(tree.RootNode(), "")); d != "" {
					failures++
					t.Errorf("%s: %s: the trees differ at %s", filepath.Base(e.File), e.Name, d)
					continue
				}
				if gotString := tree.RootNode().String(); gotString != wantString {
					failures++
					t.Errorf("%s: %s: the text of the trees differs:\n  C:  %s\n  Go: %s", filepath.Base(e.File), e.Name, wantString, gotString)
				}
			}
			t.Logf("%d inputs, %d differ", len(examples), failures)
		})
	}
}

// TestGoPackageEditsMatchC does the steps of TestFixtureEditsMatchC with
// each grammar package: the Go runtime parses with the Go package, and the C
// runtime with the C grammar.
func TestGoPackageEditsMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range goPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			goGrammar := *g
			goGrammar.Language = gp.language()
			failures := 0
			for _, e := range examples {
				if len(e.Input) == 0 {
					continue
				}
				if msg := compareEdits(&goGrammar, e.Input); msg != "" {
					failures++
					t.Errorf("%s: %s: %s", filepath.Base(e.File), e.Name, msg)
				}
			}
			t.Logf("%d inputs, %d differ", len(examples), failures)
		})
	}
}

// TestGoPackageHighlight runs the highlight test of internal/grammartest on
// a file of JSON with assertions in its comments, because the corpus of
// tree-sitter-json has no test/highlight.
func TestGoPackageHighlight(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "{\"key\": 12}\n//  ^ string.special.key\n//      ^ number\n//   ^ !number\n"
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	grammartest.Highlight(t, json.Language(), json.Queries, dir)
}
