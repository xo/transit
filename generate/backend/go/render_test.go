package golang

import (
	"errors"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	b.code = writeParser(out, "grammar", true)
	return b.code, nil
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
// package, and the error of a folder whose name no Go package can have.
func TestPackageName(t *testing.T) {
	t.Parallel()
	for dir, want := range map[string]string{
		"grammars/json":                                   "json",
		"grammars/csharp":                                 "csharp",
		"grammars/embeddedtemplate":                       "embeddedtemplate",
		"grammars/go":                                     "golang",
		"grammars/sqlserver":                              "sqlserver",
		"grammars/php/phponly":                            "phponly",
		"grammars/typescript/typescript/":                 "typescript",
		"/mod/github.com/xo/transit/grammars/json@v0.1.0": "json",
		"grammars/Upper":                                  "upper",
	} {
		if got, err := PackageName(dir); err != nil || got != want {
			t.Errorf("PackageName(%q) = %q, %v, want %q", dir, got, err, want)
		}
	}
	for _, dir := range []string{"grammars/func", "Func", "1c", "embedded-template", "go-mod@v1.0.0"} {
		if _, err := PackageName(dir); !errors.Is(err, errPackageName) {
			t.Errorf("PackageName(%q) gives the error %v, want errPackageName", dir, err)
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
// is, that the corpus test and the highlight test are there only with their
// folders, and that the corpus test names no case, because the cases that
// fail upstream are in testdata/failing.txt (D88).
func TestTests(t *testing.T) {
	t.Parallel()
	for _, opts := range []Options{
		{},
		{Queries: true, Corpus: true},
		{Queries: true, Corpus: true, Highlight: true},
		{Highlight: true, Others: []Other{{"tsx", "github.com/xo/transit/grammars/typescript/tsx"}, {"a", "github.com/xo/transit/grammars/typescript/a"}}},
		{Others: []Other{{"tsx", "github.com/xo/transit/grammars/typescript/tsx"}}},
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
		if opts.Corpus && !strings.Contains(text, "\tgrammartest.Corpus(t, Language(), \"testdata/corpus\")\n") {
			t.Errorf("%+v: the corpus test gives more than the folder of the corpus", opts)
		}
		for _, o := range opts.Others {
			if got := strings.Contains(text, "grammartest.Grammar{Language: "+o.Package+".Language(), Queries: "+o.Package+".Queries}"); got != opts.Highlight {
				t.Errorf("%+v: the grammar %s is there: %t", opts, o.Package, got)
			}
			if got := strings.Contains(text, strconv.Quote(o.ImportPath)); got != opts.Highlight {
				t.Errorf("%+v: the import of %s is there: %t", opts, o.Package, got)
			}
		}
	}
}

// TestReadGrammarOptions makes sure that the options of a package of a
// module with two grammars name the other grammar when it has an
// injection-regex, and that a module with one grammar has no other.
func TestReadGrammarOptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, text := range map[string]string{
		"two/go.mod": "module example.com/two\n\ngo 1.27\n",
		"two/tree-sitter.json": `{"grammars": [
			{"name": "typescript", "path": "typescript", "injection-regex": "^(ts|typescript)$"},
			{"name": "tsx", "path": "tsx", "injection-regex": "^tsx$"},
			{"name": "flow", "path": "tsx", "injection-regex": "^flow$"},
			{"name": "php_only", "path": "php_only", "injection-regex": "^php$"},
			{"name": "missing", "path": "missing", "injection-regex": "^missing$"},
			{"name": "no_regex", "path": "noregex"}]}`,
		"two/typescript/grammar.json": `{"name": "typescript"}`,
		"two/tsx/grammar.json":        `{"name": "tsx"}`,
		"two/phponly/grammar.json":    `{"name": "php_only"}`,
		"two/noregex/grammar.json":    `{"name": "no_regex"}`,
		"one/tree-sitter.json":        `{"grammars": [{"name": "json", "path": ".", "injection-regex": "^json$"}]}`,
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opts, err := ReadGrammarOptions(filepath.Join(root, "two", "typescript"))
	if err != nil {
		t.Fatalf("ReadGrammarOptions = %+v, %v", opts, err)
	}
	if want := []Other{{"tsx", "example.com/two/tsx"}, {"phponly", "example.com/two/phponly"}}; !slices.Equal(opts.Others, want) {
		t.Errorf("Others = %+v, want %+v", opts.Others, want)
	}
	opts, err = ReadGrammarOptions(filepath.Join(root, "one"))
	if err != nil || len(opts.Others) != 0 {
		t.Errorf("ReadGrammarOptions of one grammar = %+v, %v", opts, err)
	}
}

// TestGrammarFolder makes sure that the path of a grammar in
// tree-sitter.json gives the folder of its package, as docs/GRAMMAR.md lays
// it out.
func TestGrammarFolder(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]string{
		"":                  ".",
		".":                 ".",
		"./":                ".",
		"php_only":          "phponly",
		"tsx":               "tsx",
		"grammars/php_only": "phponly",
		"embedded-template": "embeddedtemplate",
	} {
		if got := GrammarFolder(p); got != want {
			t.Errorf("GrammarFolder(%q) = %q, want %q", p, got, want)
		}
	}
}

// TestFindRecord finds the entries of grammar packages in a record. Two
// entries give the module folder sql, and the tree-sitter.json of the
// module names the repository of one of them (D106). A package outside a
// checkout of transit has no record.
func TestFindRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, text := range map[string]string{
		"grammars/grammars.json": `{"grammars": [
			{"name": "json", "repository": "https://github.com/tree-sitter/tree-sitter-json"},
			{"name": "sql", "repository": "https://github.com/DerekStride/tree-sitter-sql"},
			{"name": "sql", "repository": "https://github.com/m-novikov/tree-sitter-sql"}]}`,
		"grammars/json/go.mod":          "module example.com/json\n",
		"grammars/sql/go.mod":           "module example.com/sql\n",
		"grammars/sql/tree-sitter.json": `{"metadata": {"links": {"repository": "git+https://github.com/derekstride/tree-sitter-sql.git"}}}`,
		"grammars/nosql/go.mod":         "module example.com/nosql\n",
		"grammars/other/sql/go.mod":     "module example.com/other\n",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		dir, name, want string
	}{
		{"grammars/json", "json", "https://github.com/tree-sitter/tree-sitter-json"},
		{"grammars/sql", "sql", "https://github.com/DerekStride/tree-sitter-sql"},
	} {
		rec, err := FindRecord(filepath.Join(root, c.dir), c.name)
		if err != nil || rec == nil || rec.Repository != c.want {
			t.Errorf("FindRecord(%s, %s) = %+v, %v, want the entry of %s", c.dir, c.name, rec, err, c.want)
		}
	}
	// the folder of the module has no tree-sitter.json, so both entries stay
	if rec, err := FindRecord(filepath.Join(root, "grammars", "other", "sql"), "sql"); err == nil {
		t.Errorf("FindRecord of a module with no tree-sitter.json = %+v, want an error", rec)
	}
	if rec, err := FindRecord(filepath.Join(root, "grammars", "nosql"), "nosql"); err == nil {
		t.Errorf("FindRecord of a grammar with no entry = %+v, want an error", rec)
	}
	if rec, err := FindRecord(t.TempDir(), "json"); rec != nil || err != nil {
		t.Errorf("FindRecord outside a checkout = %+v, %v, want nil and no error", rec, err)
	}
}

// TestSameRepository compares the forms of the URL of a repository.
func TestSameRepository(t *testing.T) {
	t.Parallel()
	const repo = "https://github.com/DerekStride/tree-sitter-sql"
	for u, want := range map[string]bool{
		repo: true,
		"git+https://github.com/derekstride/tree-sitter-sql.git": true,
		"https://github.com/derekstride/tree-sitter-sql/":        true,
		"https://github.com/m-novikov/tree-sitter-sql":           false,
		"": false,
	} {
		if got := sameRepository(repo, u); got != want {
			t.Errorf("sameRepository(%q, %q) = %t, want %t", repo, u, got, want)
		}
	}
	if sameRepository("", "") {
		t.Error("two empty URLs name the same repository")
	}
}

func TestModuleFolderName(t *testing.T) {
	t.Parallel()
	for repo, want := range map[string]string{
		"https://github.com/tree-sitter/tree-sitter-json":              "json",
		"https://github.com/tree-sitter/tree-sitter-embedded-template": "embeddedtemplate",
		"https://github.com/DerekStride/tree-sitter-sql":               "sql",
	} {
		if got := ModuleFolderName(repo); got != want {
			t.Errorf("ModuleFolderName(%q) = %q, want %q", repo, got, want)
		}
	}
}

// TestPackageInDirectory generates the package of grammars/json again, in a
// new folder, and compares the files with the files of grammars/json. With
// generateParser false, it writes node-types.json only.
func TestPackageInDirectory(t *testing.T) {
	t.Parallel()
	src := filepath.Join("..", "..", "..", "grammars", "json")
	// the folder gives the name of the package
	dir := filepath.Join(t.TempDir(), "json")
	for _, name := range []string{"go.mod", "grammar.json", "tree-sitter.json", "queries/highlights.scm", "testdata/corpus/main.txt"} {
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
	if err != nil || !reflect.DeepEqual(opts, Options{Queries: true, Corpus: true}) {
		t.Fatalf("ReadOptions = %+v, %v", opts, err)
	}
	var diagnostics []generate.Diagnostic
	if err := PackageInDirectory(dir, "", generate.LanguageVersion, true, generate.OptLevelMergeStates, &diagnostics); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"parser.go", "node-types.json", "grammar_test.go", "example_test.go"} {
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

// TestGoString makes sure that a text becomes a raw string when it can, and
// an interpreted string when it holds a backquote, a carriage return, a
// character that does not print or bytes that are not UTF-8.
func TestGoString(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"a\n\tb \u00e9": "`a\n\tb é`",
		"":              "``",
		"a`b":           `"a` + "`" + `b"`,
		"a\r\nb":        `"a\r\nb"`,
		"\ufeffa":       `"\ufeffa"`,
		"\xffa":         `"\xffa"`,
		"a\u00a0b":      `"a\u00a0b"`,
	} {
		if got := goString([]byte(in)); got != want {
			t.Errorf("goString(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestHoldsKind makes sure that the kinds of the nodes of a tree come from
// the tree as Node.String prints it.
func TestHoldsKind(t *testing.T) {
	t.Parallel()
	kinds := map[string]bool{"pair": true, "ERROR": true}
	for sexp, want := range map[string]bool{
		"(document (object (pair (string) (number))))": true,
		"(document (array (number) (pair)))":           true,
		"(document (array (number) (string)))":         false,
		"(document (ERROR (number)))":                  true,
		"(document (MISSING pair))":                    false,
	} {
		if got := holdsKind(sexp, kinds); got != want {
			t.Errorf("holdsKind(%q) = %t, want %t", sexp, got, want)
		}
	}
}
