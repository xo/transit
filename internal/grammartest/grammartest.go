// Package grammartest holds the tests of a grammar package, which the
// grammar_test.go that the Go backend writes calls: the corpus test, the
// query test, the highlight test and the generator test of docs/GRAMMAR.md.
// The package is internal, so only the grammar packages of transit and its
// test module can import it. It uses no cgo, so a grammar module stays pure
// Go (D1).
//
// test.go ports the parts of crates/cli/src/test.rs that read a corpus,
// parse.go ports the output of a concrete syntax tree of
// crates/cli/src/parse.rs, query_testing.go ports
// crates/cli/src/query_testing.rs, test_highlight.go ports
// crates/cli/src/test_highlight.rs, and highlight.go ports the highlighter
// of crates/highlight/src/highlight.rs that the highlight test needs (D80).
// This file ports no upstream file.
package grammartest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
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
// Each file and each case is a subtest.
//
// The cases that fail upstream are the cases of testdata/failing.txt in the
// working folder, the folder of the package, and the cases of failing. A
// caller outside a grammar package, such as the test module, gives them in
// failing. The golden harness writes the file from the field failing of
// grammars/grammars.json (D79, D88), and failingText gives its form. A case
// is named by its path: the names of its groups and its own name, joined by
// "/", such as "expressions/Binary operators". The test expects each such
// case to fail, and it names the case and its failure in its log. A case
// that fails and is not in the list, and a case in the list that does not
// fail, fail the test.
//
// A repository can hold more than one grammar, and its grammars share one
// corpus, as tree-sitter-php does. Each package of the module holds the
// whole corpus (D83). A case runs in the package of the grammar that its
// attribute :language(x) names, or of the first grammar of the nearest
// tree-sitter.json when it names none, as upstream runs it. The other
// packages skip the case, and name the package that runs it. A case for a
// grammar that the module does not hold fails with "Language not found",
// as upstream does. A package of a grammar that tree-sitter.json does not
// list, such as plpgsql of tree-sitter-postgres, holds the corpus of its own
// folder, and it runs each case that names no grammar.
func Corpus(t *testing.T, language *transit.Language, dir string, failing ...string) {
	t.Helper()
	entry, err := parseTests(dir)
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	m, err := readModule(language.Name())
	if err != nil {
		t.Fatal(err)
	}
	fromFile, err := readFailing()
	if err != nil {
		t.Fatal(err)
	}
	failing = append(fromFile, failing...)
	parser := transit.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		t.Fatalf("setting the language: %v", err)
	}
	r := &corpusRun{
		parser:   parser,
		language: language,
		module:   m,
		expected: map[string]int{},
		failed:   map[string]int{},
	}
	for _, name := range failing {
		r.expected[name]++
	}
	r.runTests(t, entry, "")
	for _, name := range failing {
		if r.failed[name] < r.expected[name] {
			// a name that repeats is reported once
			r.failed[name] = r.expected[name]
			t.Errorf("the case %q does not fail, or the corpus does not hold it, and the list of the cases that fail upstream names it", name)
		}
	}
}

// corpusRun holds the state of one run of Corpus.
type corpusRun struct {
	parser   *transit.Parser
	language *transit.Language
	// module is the tree-sitter.json of the module of the package.
	module *module
	// expected counts the cases of each path that fail upstream, and failed
	// counts the cases of each path that failed in this run.
	expected map[string]int
	failed   map[string]int
}

// runTests runs the tests of a group, each as a subtest, and reports whether
// the run goes on. A failed test with the attribute :fail-fast stops it.
// prefix is the path of the group, which is empty for the root.
//
// runTests is run_tests, without the report and the update of a corpus
// file.
func (r *corpusRun) runTests(t *testing.T, entry testEntry, prefix string) bool {
	t.Helper()
	goOn := true
	for _, child := range entry.children {
		if !goOn {
			break
		}
		name := child.name
		if prefix != "" {
			name = prefix + "/" + child.name
		}
		if child.isGroup {
			if len(child.children) == 0 {
				continue
			}
			t.Run(child.name, func(t *testing.T) {
				goOn = r.runTests(t, child, name)
			})
			continue
		}
		t.Run(child.name, func(t *testing.T) {
			goOn = r.runCase(t, child, name)
		})
	}
	return goOn
}

// runCase runs one corpus test with the path name, and reports whether the
// run goes on. A failure of a case in the list of the cases that fail
// upstream goes to the log, and any other failure fails the test. The log
// names testdata/failing.txt, the file that the list comes from (D93).
func (r *corpusRun) runCase(t *testing.T, e testEntry, name string) bool {
	t.Helper()
	a := e.attributes
	switch {
	case a.expectation == expectSkip:
		t.Skip("the test has the attribute :skip")
	case !a.platform:
		t.Skip("the test is for another platform")
	}
	if owner := r.owner(a); owner != r.module.own {
		t.Skipf("the package in the folder %s of the module runs the case (D83)", owner)
	}
	failure := r.runExample(e)
	switch {
	case failure == "":
		return true
	case r.failed[name] < r.expected[name]:
		r.failed[name]++
		t.Logf("the case fails, as it fails upstream (testdata/failing.txt): %s", failure)
	default:
		t.Error(failure)
	}
	return !a.failFast
}

// owner returns the folder of the package that runs a case: the package of
// the grammar of its first :language, or of the first grammar of the module
// when it names none. A case for a grammar that the module does not hold
// runs in the package, and fails with "Language not found". A case that
// names no grammar runs in the package when no entry of tree-sitter.json
// has the folder of the package, such as plpgsql of tree-sitter-postgres.
// Upstream runs the corpus of such a grammar in its own folder, where the
// grammar of the folder is the only language.
func (r *corpusRun) owner(a testAttributes) string {
	name := ""
	if len(a.languages) > 0 {
		name = a.languages[0]
	}
	if name == "" {
		if len(r.module.entries) == 0 || !r.module.hasFolder(r.module.own) {
			return r.module.own
		}
		return r.module.entries[0].folder()
	}
	if e, ok := r.module.entry(name); ok {
		return e.folder()
	}
	return r.module.own
}

// runExample runs one corpus test, and returns the text of its failure, or
// an empty text when it passes.
func (r *corpusRun) runExample(e testEntry) string {
	a := e.attributes
	for _, name := range a.languages {
		// A package has one language. A grammar of the module in the folder
		// of the package, such as flow of tree-sitter-typescript, is that
		// language.
		if entry, ok := r.module.entry(name); name != "" && name != r.language.Name() && (!ok || entry.folder() != r.module.own) {
			return "Language not found: " + name
		}
		tree, err := r.parser.Parse(context.Background(), e.input, nil)
		if err != nil {
			return fmt.Sprintf("parsing the input: %v", err)
		}
		if a.expectation == expectError {
			if !tree.RootNode().HasError() {
				return fmt.Sprintf("the tree has no error:\n  actual:   %s\n  expected: NO ERROR", renderTestOutput(e.input, tree, a.cst, true))
			}
			continue
		}
		if actual := renderTestOutput(e.input, tree, a.cst, e.hasFields); actual != e.output {
			return fmt.Sprintf("the trees differ:\n  actual:   %s\n  expected: %s", actual, e.output)
		}
	}
	return ""
}

// renderTestOutput returns a tree in the form of the output of a corpus
// test: the concrete syntax tree when cst is true, and else its
// s-expression, with the field names only when includeFields is true.
//
// renderTestOutput is render_test_output.
func renderTestOutput(input []byte, tree *transit.Tree, cst, includeFields bool) string {
	if cst {
		return RenderCST(input, tree)
	}
	out := tree.RootNode().String()
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

// Generator runs the generator on grammar.json of the package in the
// working folder, with the version of the nearest tree-sitter.json. It
// makes sure of three facts:
//
//  1. The Go backend writes the parser.go, the node-types.json and the
//     grammar_test.go of the package, at the ABI version of the language.
//  2. The C backend writes the parser.c and the node-types.json whose
//     SHA-256 grammars/grammars.json records, at ABI 14 and ABI 15 (D40).
//  3. testdata/failing.txt names the corpus cases that grammars/grammars.json
//     records as failing upstream, in the form of failingText. The file does
//     not exist when the record names no case (D88).
//
// The test skips the last two checks when the package is not in a checkout
// of transit, which holds grammars/grammars.json, and the whole test in
// short mode, because the generator takes minutes for a large grammar.
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
	var diagnostics []generate.Diagnostic
	inputGrammar, err := generate.ParseGrammar(grammarJSON, &diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	name := inputGrammar.Pool.Resolve(inputGrammar.Name)
	pkg, err := golang.PackageName(".")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := golang.ReadGrammarOptions(".")
	if err != nil {
		t.Fatal(err)
	}
	rec, err := golang.FindRecord(".", name)
	if err != nil {
		t.Fatal(err)
	}

	backend := &multiBackend{targets: []target{
		{abi: 14, backend: c.Backend{}},
		{abi: 15, backend: c.Backend{}},
		{abi: language.ABIVersion(), backend: golang.Backend{Package: pkg, Queries: opts.Queries}},
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

	if rec == nil {
		t.Log("the package is not in a checkout of transit, so the test does not read grammars/grammars.json. " +
			"It does not compare the hashes and testdata/failing.txt with that file")
		return
	}
	if err := compareFailing(rec); err != nil {
		t.Error(err)
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

// failingPath is the file of a grammar package that names the corpus cases
// that fail upstream (D88).
const failingPath = "testdata/failing.txt"

// failingText returns the text of testdata/failing.txt for the names of the
// corpus cases that fail upstream, in the order of grammars/grammars.json:
// each name on a line of its own, and each line ends with a newline. A name
// that the record holds twice is on two lines. The text is empty when there
// is no name, and then the file does not exist.
func failingText(names []string) string {
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name + "\n")
	}
	return b.String()
}

// readFailing returns the names of testdata/failing.txt in the working
// folder, or no names when the file does not exist. A file with no name, a
// blank line or no newline at its end is an error.
func readFailing() ([]string, error) {
	b, err := os.ReadFile(failingPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", failingPath, err)
	}
	text, ok := strings.CutSuffix(string(b), "\n")
	if !ok {
		return nil, fmt.Errorf("reading %s: the file does not end with a newline", failingPath)
	}
	names := strings.Split(text, "\n")
	if slices.Contains(names, "") {
		return nil, fmt.Errorf("reading %s: the file holds a blank line", failingPath)
	}
	return names, nil
}

// compareFailing makes sure that testdata/failing.txt in the working
// folder is the file that the golden harness writes from the record of the
// grammar, and returns an error when it is not.
func compareFailing(rec *golang.Record) error {
	var names []string
	if rec.Corpus != nil {
		names = rec.Corpus.Failing
	}
	want := failingText(names)
	got, err := os.ReadFile(failingPath)
	switch {
	case errors.Is(err, fs.ErrNotExist) && want == "":
		// no case fails upstream, and the file does not exist
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s does not exist, and grammars/grammars.json records the cases %q as failing upstream. Run the golden harness again", failingPath, names)
	case err != nil:
		return fmt.Errorf("reading %s: %w", failingPath, err)
	case want == "":
		return fmt.Errorf("%s exists, and grammars/grammars.json records no case as failing upstream. Run the golden harness again", failingPath)
	case string(got) != want:
		return fmt.Errorf("%s is not the file that the golden harness writes from grammars/grammars.json, which records the cases %q as failing upstream. Run the golden harness again", failingPath, names)
	}
	return nil
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
