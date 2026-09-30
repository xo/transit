// Package grammartest holds the tests of a grammar package, which the
// grammar_test.go that the Go backend writes calls: the corpus test, the
// query test, the highlight test and the generator test of docs/GRAMMAR.md.
// The package is internal, so only the grammar packages of transit and its
// test module can import it. It uses no cgo, so a grammar module stays pure
// Go (D1).
//
// test.go ports the parts of crates/cli/src/test.rs that read a corpus, and
// query_testing.go ports crates/cli/src/query_testing.rs. This file ports no
// upstream file.
package grammartest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
	golang "github.com/xo/transit/generate/backend/go"
)

// Corpus parses each case of the corpus in dir, such as testdata/corpus,
// and compares its tree with the expected tree, as `tree-sitter test` does.
// Each file and each case is a subtest. A case with the attribute :cst is
// skipped, because the output of a concrete syntax tree waits for the
// subcommand test of D41.
//
// A repository can hold more than one grammar, and its grammars share one
// corpus, as tree-sitter-typescript does. Upstream runs a case with the
// attribute :language(x) with the grammar x of the repository. A package
// has only its own language, so a case for another grammar of the nearest
// tree-sitter.json is skipped, and the package of that grammar runs it. A
// case for a grammar that the repository does not hold fails with
// "Language not found", as upstream does.
func Corpus(t *testing.T, language *transit.Language, dir string) {
	t.Helper()
	entry, err := parseTests(dir)
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	others, err := otherGrammars(".", language.Name())
	if err != nil {
		t.Fatalf("reading tree-sitter.json: %v", err)
	}
	parser := transit.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		t.Fatalf("setting the language: %v", err)
	}
	runTests(t, parser, language, others, entry)
}

// otherGrammars returns the names of the grammars of the nearest
// tree-sitter.json, in dir or in a folder above it, but for the grammar
// name. It returns none when no folder holds a tree-sitter.json.
func otherGrammars(dir, name string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("finding the folder %s: %w", dir, err)
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "tree-sitter.json"))
		switch {
		case errors.Is(err, os.ErrNotExist):
			parent := filepath.Dir(dir)
			if parent == dir {
				return nil, nil
			}
			dir = parent
			continue
		case err != nil:
			return nil, fmt.Errorf("reading tree-sitter.json: %w", err)
		}
		var cfg struct {
			Grammars []struct {
				Name string `json:"name"`
			} `json:"grammars"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Join(dir, "tree-sitter.json"), err)
		}
		var names []string
		for _, g := range cfg.Grammars {
			if g.Name != name && !slices.Contains(names, g.Name) {
				names = append(names, g.Name)
			}
		}
		return names, nil
	}
}

// runTests runs the tests of a group, each as a subtest, and reports whether
// the run goes on. A failed test with the attribute :fail-fast stops it.
//
// runTests is run_tests, without the report and the update of a corpus
// file.
func runTests(t *testing.T, parser *transit.Parser, language *transit.Language, others []string, entry testEntry) bool {
	t.Helper()
	goOn := true
	for _, child := range entry.children {
		if !goOn {
			break
		}
		if child.isGroup {
			if len(child.children) == 0 {
				continue
			}
			t.Run(child.name, func(t *testing.T) {
				goOn = runTests(t, parser, language, others, child)
			})
			continue
		}
		t.Run(child.name, func(t *testing.T) {
			goOn = runExample(t, parser, language, others, child)
		})
	}
	return goOn
}

// runExample runs one corpus test, and reports whether the run goes on.
// others holds the names of the other grammars of the repository.
func runExample(t *testing.T, parser *transit.Parser, language *transit.Language, others []string, e testEntry) bool {
	t.Helper()
	a := e.attributes
	switch {
	case a.expectation == expectSkip:
		t.Skip("the test has the attribute :skip")
	case !a.platform:
		t.Skip("the test is for another platform")
	case a.cst:
		t.Skip("the output of a concrete syntax tree waits for the subcommand test of D41")
	}
	for _, name := range a.languages {
		if name != "" && name != language.Name() && slices.Contains(others, name) {
			t.Skipf("the case is for the grammar %s of the same repository, and the package of that grammar runs it", name)
		}
		if name != "" && name != language.Name() {
			t.Errorf("Language not found: %s", name)
			return !a.failFast
		}
		tree, err := parser.Parse(context.Background(), e.input, nil)
		if err != nil {
			t.Errorf("parsing the input: %v", err)
			return !a.failFast
		}
		root := tree.RootNode()
		if a.expectation == expectError {
			if !root.HasError() {
				t.Errorf("the tree has no error:\n  actual:   %s\n  expected: NO ERROR", renderTestOutput(root, true))
				return !a.failFast
			}
			continue
		}
		if actual := renderTestOutput(root, e.hasFields); actual != e.output {
			t.Errorf("the trees differ:\n  actual:   %s\n  expected: %s", actual, e.output)
			return !a.failFast
		}
	}
	return true
}

// renderTestOutput returns a tree in the form of the output of a corpus
// test: its s-expression, with the field names only when includeFields is
// true.
//
// renderTestOutput is render_test_output, without the concrete syntax tree.
func renderTestOutput(root transit.Node, includeFields bool) string {
	out := root.String()
	if includeFields {
		return out
	}
	return stripSexpFields(out)
}

// Queries compiles each file queries/*.scm of fsys, such as the Queries of a
// grammar package, and reports each one that fails.
//
// Queries is check_queries_at_path of crates/cli/src/test.rs.
func Queries(t *testing.T, language *transit.Language, fsys fs.FS) {
	t.Helper()
	err := fs.WalkDir(fsys, "queries", func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && name == "queries":
			// the grammar has no queries
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir() || path.Ext(name) != ".scm":
			return nil
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("reading query file %q: %w", path.Base(name), err)
		}
		if _, err := transit.NewQuery(language, string(content)); err != nil {
			t.Errorf("Error in query file %q: %v", path.Base(name), err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Highlight runs queries/highlights.scm of fsys on each file of dir, such as
// testdata/highlight, and checks the assertions in the comments of the
// file. transit does not port the highlighter of upstream (D7), so the test
// checks each assertion against the captures of the query: the innermost
// capture that holds the position of the assertion, and of two captures of
// one node, the first. A negative assertion passes when that capture has
// another name. An assertion with no capture fails, as iterate_assertions of
// crates/cli/src/test_highlight.rs fails it.
func Highlight(t *testing.T, language *transit.Language, fsys fs.FS, dir string) {
	t.Helper()
	source, err := fs.ReadFile(fsys, "queries/highlights.scm")
	if err != nil {
		t.Fatalf("reading the highlight query: %v", err)
	}
	query, err := transit.NewQuery(language, string(source))
	if err != nil {
		t.Fatalf("compiling queries/highlights.scm: %v", err)
	}
	err = filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		t.Run(filepath.Base(name), func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := testHighlight(language, query, src); err != nil {
				t.Error(err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// testHighlight checks the assertions of one file, and returns their
// number.
func testHighlight(language *transit.Language, query *transit.Query, src []byte) (int, error) {
	parser := transit.NewParser()
	assertions, err := parsePositionComments(parser, language, src)
	if err != nil {
		return 0, err
	}
	tree, err := parser.Parse(context.Background(), src, nil)
	if err != nil {
		return 0, fmt.Errorf("parsing the source: %w", err)
	}
	var infos []captureInfo
	names := query.CaptureNames()
	for m, i := range transit.NewQueryCursor().Captures(context.Background(), query, tree.RootNode(), src) {
		capture := m.Captures[i]
		infos = append(infos, captureInfo{
			name:  names[capture.Index],
			start: toUTF8Point(capture.Node.StartPoint(), src),
			end:   toUTF8Point(capture.Node.EndPoint(), src),
		})
	}
	return checkAssertions(infos, assertions)
}

// checkAssertions checks each assertion against the innermost capture that
// holds it, and returns the number of assertions. Of two captures with the
// same range, the first one counts, as in the highlighter of upstream.
func checkAssertions(infos []captureInfo, assertions []assertion) (int, error) {
	for _, a := range assertions {
		end := utf8Point{row: a.position.row, column: a.position.column + a.length - 1}
		best := -1
		for k, p := range infos {
			if a.position.compare(p.start) < 0 || end.compare(p.end) >= 0 {
				continue
			}
			if best < 0 || p.start.compare(infos[best].start) > 0 ||
				p.start == infos[best].start && p.end.compare(infos[best].end) < 0 {
				best = k
			}
		}
		expected := a.expectedCaptureName
		if a.negative {
			expected = "!" + expected
		}
		if best < 0 {
			// a negative assertion fails too, as in iterate_assertions
			return 0, fmt.Errorf("%w: row %d, column %d, expected highlight %q, actual highlights: none",
				errAssertion, a.position.row, end.column, expected)
		}
		if (infos[best].name == a.expectedCaptureName) == a.negative {
			return 0, fmt.Errorf("%w: row %d, column %d, expected highlight %q, actual highlight %q",
				errAssertion, a.position.row, end.column, expected, infos[best].name)
		}
	}
	return len(assertions), nil
}

// NodeTypes makes sure that each node type, and each type that it names,
// is a symbol of the language.
func NodeTypes(t *testing.T, language *transit.Language, types []transit.NodeType) {
	t.Helper()
	if len(types) == 0 {
		t.Fatal("the grammar has no node types")
	}
	check := func(kind string, named bool) {
		if _, ok := language.SymbolForName(kind, named); !ok {
			t.Errorf("the node type %q (named %t) is not a symbol of the language", kind, named)
		}
	}
	for _, nt := range types {
		check(nt.Kind, nt.Named)
		for _, st := range nt.Subtypes {
			check(st.Kind, st.Named)
		}
		for _, f := range nt.Fields {
			for _, ft := range f.Types {
				check(ft.Kind, ft.Named)
			}
		}
		if nt.Children != nil {
			for _, ct := range nt.Children.Types {
				check(ct.Kind, ct.Named)
			}
		}
	}
}

// Keywords makes sure that each keyword is the name of a symbol of the
// language, and that the list is sorted with no repeats.
func Keywords(t *testing.T, language *transit.Language, keywords []string) {
	t.Helper()
	if !slices.IsSorted(keywords) || len(slices.Compact(slices.Clone(keywords))) != len(keywords) {
		t.Errorf("the keywords are not sorted with no repeats: %q", keywords)
	}
	for _, k := range keywords {
		_, anonymous := language.SymbolForName(k, false)
		_, named := language.SymbolForName(k, true)
		if !anonymous && !named {
			t.Errorf("the keyword %q is not a symbol of the language", k)
		}
	}
}

// record is an entry of grammars/grammars.json, with the fields that the
// generator test reads.
type record struct {
	Name       string            `json:"name"`
	Repository string            `json:"repository"`
	Golden     map[string]golden `json:"golden"`
}

// golden is a golden file of a record.
type golden struct {
	ParserC   string `json:"parser_c"`
	NodeTypes string `json:"node_types"`
	Error     string `json:"error"`
}

// Generator runs the generator on grammar.json of the package in the
// working folder, with the version of the nearest tree-sitter.json. It
// makes sure of two facts:
//
//  1. The C backend writes the parser.c and the node-types.json whose
//     SHA-256 grammars/grammars.json records, at ABI 14 and ABI 15 (D40).
//  2. The Go backend writes the parser.go, the node-types.json and the
//     grammar_test.go of the package, at the ABI version of the language.
//
// The test skips the first check when the package is not in a checkout of
// transit, which holds grammars/grammars.json, and the whole test in short
// mode, because the generator takes minutes for a large grammar.
func Generator(t *testing.T, language *transit.Language) {
	t.Helper()
	if testing.Short() {
		t.Skip("the generator test runs the whole generator")
	}
	grammarJSON, err := os.ReadFile("grammar.json")
	if err != nil {
		t.Fatal(err)
	}
	version, err := generate.ReadGrammarVersion(".")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := golang.ReadOptions(".")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []generate.Diagnostic
	inputGrammar, err := generate.ParseGrammar(grammarJSON, &diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	name := inputGrammar.Pool.Resolve(inputGrammar.Name)
	pkg, err := golang.PackageName(name)
	if err != nil {
		t.Fatal(err)
	}

	backend := &multiBackend{targets: []target{
		{abi: 14, backend: c.Backend{}},
		{abi: 15, backend: c.Backend{}},
		{abi: language.ABIVersion(), backend: golang.Backend{Queries: opts.Queries}},
	}}
	parser, err := generate.ParserForGrammarWithOpts(inputGrammar, generate.LanguageVersion, version, generate.OptLevelMergeStates, backend, &diagnostics)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range []struct{ name, want string }{
		{"parser.go", backend.outputs[2]},
		{"node-types.json", parser.NodeTypesJSON},
		{"grammar_test.go", golang.Tests(pkg, opts)},
	} {
		got, err := os.ReadFile(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != f.want {
			t.Errorf("%s is not the file that transit generate writes. Generate the package again", f.name)
		}
	}

	rec, ok := findRecord(t, name)
	if !ok {
		return
	}
	for i, abi := range []string{"abi14", "abi15"} {
		want, ok := rec.Golden[abi]
		switch {
		case !ok:
			t.Errorf("grammars/grammars.json has no golden file %s for %s", abi, name)
			continue
		case want.Error != "":
			t.Errorf("%s: grammars/grammars.json records the error %q, and the generator writes the grammar", abi, want.Error)
			continue
		}
		if got := sum(backend.outputs[i]); got != want.ParserC {
			t.Errorf("%s: parser.c has the SHA-256 %s, and the record has %s", abi, got, want.ParserC)
		}
		if got := sum(parser.NodeTypesJSON); got != want.NodeTypes {
			t.Errorf("%s: node-types.json has the SHA-256 %s, and the record has %s", abi, got, want.NodeTypes)
		}
	}
}

// target is a backend and the ABI version that it writes.
type target struct {
	abi     int
	backend generate.Backend
}

// multiBackend writes the output of several backends from the tables of one
// run of the generator, and keeps them.
type multiBackend struct {
	targets []target
	outputs []string
}

// Render writes the output of each target. It returns no code of its own.
func (m *multiBackend) Render(in *generate.RenderInput) (string, error) {
	for _, tg := range m.targets {
		// a backend does not change the RenderInput
		copied := *in
		copied.ABIVersion = tg.abi
		out, err := tg.backend.Render(&copied)
		if err != nil {
			return "", fmt.Errorf("rendering at ABI %d: %w", tg.abi, err)
		}
		m.outputs = append(m.outputs, out)
	}
	return "", nil
}

// sum returns the SHA-256 of a text in hex.
func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// findRecord returns the entry of grammars/grammars.json for the grammar
// name, and false when the working folder is not in a checkout of transit.
// When several entries have the name, the entry is the one whose repository
// gives the name of the folder of the module, as docs/GRAMMAR.md says.
func findRecord(t *testing.T, name string) (record, bool) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var moduleDir, recordFile string
	for {
		if moduleDir == "" {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				moduleDir = dir
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "grammars", "grammars.json")); err == nil {
			recordFile = filepath.Join(dir, "grammars", "grammars.json")
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Log("the package is not in a checkout of transit, so the test does not read grammars/grammars.json")
			return record{}, false
		}
		dir = parent
	}
	b, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Grammars []record `json:"grammars"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	var found []record
	for _, r := range file.Grammars {
		if r.Name == name {
			found = append(found, r)
		}
	}
	if len(found) > 1 {
		found = slices.DeleteFunc(found, func(r record) bool {
			return moduleFolderName(r.Repository) != filepath.Base(moduleDir)
		})
	}
	if len(found) != 1 {
		t.Fatalf("grammars/grammars.json has %d entries for the grammar %s in the module folder %s", len(found), name, filepath.Base(moduleDir))
	}
	return found[0], true
}

// moduleFolderName returns the name of the folder of the module of a
// repository: its name without the prefix tree-sitter- and with each "-"
// removed, as docs/GRAMMAR.md says.
func moduleFolderName(repository string) string {
	name := strings.TrimPrefix(path.Base(repository), "tree-sitter-")
	return strings.ReplaceAll(name, "-", "")
}
