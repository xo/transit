package golang

import (
	"errors"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/transit/generate"
)

// testBackend is a backend that keeps the output of the generator and the
// text of parser.go.
type testBackend struct {
	out  *output
	code string
}

// Render keeps the output and the text.
func (b *testBackend) Render(in *generate.RenderInput) (string, error) {
	out, err := render(in)
	if err != nil {
		return "", err
	}
	b.out = out
	b.code, err = writeParser(out, true)
	return b.code, err
}

// TestRenderOnEveryTestGrammar runs the whole generator with the Go backend
// on each test grammar, at ABI 14 and ABI 15, with and without the merge of
// parse states. It makes sure that gofmt leaves parser.go as it is, that the
// file has a constant for each symbol and field, and that a grammar that the
// C backend rejects is rejected too. The test module compares the tables
// with the tables of the C backend.
func TestRenderOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 68 {
		t.Fatalf("expected the grammar.json of the 68 test grammars in testdata, found %d", len(files))
	}
	for _, f := range files {
		name := filepath.Base(filepath.Dir(f))
		grammarJSON, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range []struct {
			dir  string
			abi  int
			opts generate.OptLevel
		}{
			{"abi14", 14, generate.OptLevelMergeStates},
			{"abi15", 15, generate.OptLevelMergeStates},
			{"abi15-nomerge", 15, 0},
		} {
			_, wantErr := os.Stat(filepath.Join(filepath.Dir(f), m.dir, "error.txt"))
			var diagnostics []generate.Diagnostic
			g, err := generate.ParseGrammar(grammarJSON, &diagnostics)
			b := &testBackend{}
			if err == nil {
				_, err = generate.ParserForGrammarWithOpts(g, m.abi, nil, m.opts, b, &diagnostics)
			}
			if wantErr == nil {
				if err == nil {
					t.Errorf("%s/%s: the C backend has an error, and the Go backend has none", name, m.dir)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s/%s: %v", name, m.dir, err)
				continue
			}
			formatted, err := format.Source([]byte(b.code))
			if err != nil || string(formatted) != b.code {
				t.Errorf("%s/%s: gofmt changes parser.go, or it is not valid Go: %v", name, m.dir, err)
			}
			tables := &b.out.tables
			if n, want := strings.Count(b.code, " transit.Symbol = "), int(tables.SymbolCount+tables.AliasCount); n != want {
				t.Errorf("%s/%s: %d symbol constants, want %d", name, m.dir, n, want)
			}
			if n, want := strings.Count(b.code, " transit.FieldID = "), int(tables.FieldCount); n != want {
				t.Errorf("%s/%s: %d field constants, want %d", name, m.dir, n, want)
			}
			if m.abi == 14 && (tables.Name != "" || tables.SupertypeCount != 0 || tables.ReservedWords != nil) {
				t.Errorf("%s/%s: the tables of ABI 14 have a part of ABI 15", name, m.dir)
			}
		}
	}
}

// TestRenderABIError checks the error for an ABI version that the backend
// does not write.
func TestRenderABIError(t *testing.T) {
	t.Parallel()
	for _, abi := range []int{13, 16} {
		_, err := Backend{}.Render(&generate.RenderInput{ABIVersion: abi})
		if _, ok := errors.AsType[*RenderError](err); !ok {
			t.Fatalf("ABI %d: expected a RenderError, got: %v", abi, err)
		}
		expected := "This version of Tree-sitter can only generate parsers with ABI version 14 - 15, not " + strconv.Itoa(abi)
		if err.Error() != expected {
			t.Errorf("ABI %d: expected %q, got: %q", abi, expected, err)
		}
	}
}

// TestGoNames checks the Go name of each form of C identifier that
// render.rs writes, and the numbers that make a repeated name unique.
func TestGoNames(t *testing.T) {
	t.Parallel()
	names := newGoNames()
	for _, c := range []struct{ c, want string }{
		{"ts_builtin_sym_end", "BuiltinSymEnd"},
		{"sym_document", "SymDocument"},
		{"sym__value", "SymValue"},
		{"sym_value", "SymValue2"},
		{"anon_sym_LBRACE", "AnonSymLBRACE"},
		{"anon_sym_DASH_DASH", "AnonSymDASHDASH"},
		{"anon_sym_DASHDASH", "AnonSymDASHDASH2"},
		{"aux_sym_document_repeat1", "AuxSymDocumentRepeat1"},
		{"alias_sym_type_identifier", "AliasSymTypeIdentifier"},
		{"anon_alias_sym_x", "AnonAliasSymX"},
		{"anon_sym_u00e9", "AnonSymU00e9"},
		{"sym_keyword_select", "SymKeywordSelect"},
		{"field_name", "FieldName"},
	} {
		if got := names.name(c.c); got != c.want {
			t.Errorf("the Go name of %s is %s, want %s", c.c, got, c.want)
		}
	}
}

// TestPackageName checks the rule of docs/GRAMMAR.md for the name of a
// package, and the error of a name that no Go package can have.
func TestPackageName(t *testing.T) {
	t.Parallel()
	for grammar, want := range map[string]string{
		"json":              "json",
		"c_sharp":           "csharp",
		"embedded_template": "embeddedtemplate",
		"go":                "golang",
	} {
		if got, err := PackageName(grammar); err != nil || got != want {
			t.Errorf("PackageName(%q) = %q, %v, want %q", grammar, got, err, want)
		}
	}
	for _, grammar := range []string{"func", "1c"} {
		if _, err := PackageName(grammar); !errors.Is(err, errPackageName) {
			t.Errorf("PackageName(%q) gives the error %v, want errPackageName", grammar, err)
		}
	}
}

// TestCharacter checks the Go form of a character of a lex range.
func TestCharacter(t *testing.T) {
	t.Parallel()
	for c, want := range map[int32]string{
		'a': "'a'", ' ': "' '", '\'': `'\''`, '\\': `'\\'`, '\n': `'\n'`,
		0: "0x00", 0x7f: "0x7f", 0xe9: "0xe9", -1: "-1", 0x10ffff: "0x10ffff",
	} {
		if got := character(c); got != want {
			t.Errorf("character(%d) = %s, want %s", c, got, want)
		}
	}
}

// TestTests makes sure that gofmt leaves each form of grammar_test.go as it
// is, and that the corpus test and the highlight test are there only with
// their folders.
func TestTests(t *testing.T) {
	t.Parallel()
	for _, opts := range []Options{
		{},
		{Queries: true, Corpus: true},
		{Queries: true, Corpus: true, Highlight: true},
	} {
		text := Tests("json", opts)
		formatted, err := format.Source([]byte(text))
		if err != nil || string(formatted) != text {
			t.Errorf("%+v: gofmt changes grammar_test.go, or it is not valid Go: %v", opts, err)
		}
		if got := strings.Contains(text, "func TestCorpus("); got != opts.Corpus {
			t.Errorf("%+v: TestCorpus is there: %t", opts, got)
		}
		if got := strings.Contains(text, "func TestHighlight("); got != opts.Highlight {
			t.Errorf("%+v: TestHighlight is there: %t", opts, got)
		}
	}
}

// TestPackageInDirectory generates the package of grammars/json again, in a
// new folder, and compares the files with the files of grammars/json. With
// generateParser false, it writes node-types.json only.
func TestPackageInDirectory(t *testing.T) {
	t.Parallel()
	src := filepath.Join("..", "..", "..", "grammars", "json")
	dir := t.TempDir()
	for _, name := range []string{"grammar.json", "tree-sitter.json", "queries/highlights.scm", "testdata/corpus/main.txt"} {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opts, err := ReadOptions(dir)
	if err != nil || opts != (Options{Queries: true, Corpus: true}) {
		t.Fatalf("ReadOptions = %+v, %v", opts, err)
	}
	var diagnostics []generate.Diagnostic
	if err := PackageInDirectory(dir, "", generate.LanguageVersion, true, generate.OptLevelMergeStates, &diagnostics); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"parser.go", "node-types.json", "grammar_test.go"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from grammars/json/%s", name, name)
		}
	}

	out := t.TempDir()
	if err := PackageInDirectory(dir, out, generate.LanguageVersion, false, generate.OptLevelMergeStates, &diagnostics); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "node-types.json" {
		t.Errorf("without the parser, the folder holds %v, want node-types.json only", entries)
	}
}
