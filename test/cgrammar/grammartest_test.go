package cgrammar

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/json"
	"github.com/xo/transit/internal/grammartest"
)

// This file tests the corpus test of internal/grammartest, which the
// grammar_test.go of each grammar package calls: the cases with the
// attribute :cst, and the cases that fail upstream (D79).

// cstCorpus holds corpus cases of the json grammar with the attribute :cst.
// The expected output of each case is the output of tree-sitter test of the
// upstream tool at the base commit, for the same input, with
// tree-sitter-json at the tag of grammars/grammars.json.
const cstCorpus = "================================================================================\n" +
	"Object with a string\n" +
	":cst\n" +
	"================================================================================\n" +
	"\n" +
	`{"a": [1, "b\tc"], "d": null}` + "\n" +
	"\n" +
	"--------------------------------------------------------------------------------\n" +
	"\n" +
	"1:0  - 2:0    document\n" +
	"1:0  - 1:29     object\n" +
	"1:0  - 1:1        \"{\"\n" +
	"1:1  - 1:17       pair\n" +
	"1:1  - 1:4          key: string\n" +
	"1:1  - 1:2            \"\\\"\"\n" +
	"1:2  - 1:3            string_content `a`\n" +
	"1:3  - 1:4            \"\\\"\"\n" +
	"1:4  - 1:5          \":\"\n" +
	"1:6  - 1:17         value: array\n" +
	"1:6  - 1:7            \"[\"\n" +
	"1:7  - 1:8            number `1`\n" +
	"1:8  - 1:9            \",\"\n" +
	"1:10 - 1:16           string\n" +
	"1:10 - 1:11             \"\\\"\"\n" +
	"1:11 - 1:12             string_content `b`\n" +
	"1:12 - 1:14             escape_sequence `\\\\t`\n" +
	"1:14 - 1:15             string_content `c`\n" +
	"1:15 - 1:16             \"\\\"\"\n" +
	"1:16 - 1:17           \"]\"\n" +
	"1:17 - 1:18       \",\"\n" +
	"1:19 - 1:28       pair\n" +
	"1:19 - 1:22         key: string\n" +
	"1:19 - 1:20           \"\\\"\"\n" +
	"1:20 - 1:21           string_content `d`\n" +
	"1:21 - 1:22           \"\\\"\"\n" +
	"1:22 - 1:23         \":\"\n" +
	"1:24 - 1:28         value: null `null`\n" +
	"1:28 - 1:29       \"}\"\n" +
	"\n" +
	"================================================================================\n" +
	"Missing value\n" +
	":cst\n" +
	"================================================================================\n" +
	"\n" +
	"[1,\n" +
	" 2\n" +
	"\n" +
	"--------------------------------------------------------------------------------\n" +
	"\n" +
	"1:0 - 3:0   •document\n" +
	"1:0 - 2:2     •array\n" +
	"1:0 - 1:1        \"[\"\n" +
	"1:1 - 1:2        number `1`\n" +
	"1:2 - 1:3        \",\"\n" +
	"2:1 - 2:2        number `2`\n" +
	"2:2 - 2:2       MISSING: \"]\"\n" +
	"\n" +
	"================================================================================\n" +
	"An error with :cst\n" +
	":cst\n" +
	":error\n" +
	"================================================================================\n" +
	"\n" +
	"[1,,\n" +
	"\n" +
	"--------------------------------------------------------------------------------\n" +
	"\n" +
	"================================================================================\n" +
	"Error in an array\n" +
	":cst\n" +
	"================================================================================\n" +
	"\n" +
	"[1, @, 2]\n" +
	"\n" +
	"--------------------------------------------------------------------------------\n" +
	"\n" +
	"1:0 - 2:0   •document\n" +
	"1:0 - 1:9     •array\n" +
	"1:0 - 1:1        \"[\"\n" +
	"1:1 - 1:2        number `1`\n" +
	"1:2 - 1:3        \",\"\n" +
	"1:4 - 1:6       •ERROR\n" +
	"1:4 - 1:5          •ERROR `@`\n" +
	"1:5 - 1:6          \",\"\n" +
	"1:7 - 1:8        number `2`\n" +
	"1:8 - 1:9        \"]\"\n" +
	"\n" +
	"==================\n" +
	"A comment of two lines\n" +
	":cst\n" +
	"==================\n" +
	"/* a\n" +
	"b */ 1\n" +
	"---\n" +
	"0:0 - 1:6   document\n" +
	"0:0 - 1:4     comment\n" +
	"0:0 - 0:5       `/* a\\n`\n" +
	"1:0 - 1:4       `b */`\n" +
	"1:5 - 1:6     number `1`\n"

// TestCorpusRendersCST runs the corpus test on cases of the json grammar
// with the attribute :cst, and on the corpus of the test grammar basic_cst
// of upstream.
func TestCorpusRendersCST(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cst.txt"), []byte(cstCorpus), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("json", func(t *testing.T) {
		grammartest.Corpus(t, json.Language(), dir)
	})
	t.Run("basic_cst", func(t *testing.T) {
		root, _ := setup(t)
		_, grammarDir := urTestGrammarDirs(root, "basic_cst")
		grammartest.Corpus(t, urTestFixtureLanguage(t, "basic_cst"), filepath.Join(grammarDir, "corpus.txt"))
	})
}

// failingCorpus holds a case of the json grammar that passes and a case
// whose expected tree is wrong.
const failingCorpus = `===
Passes
===
1
---
(document (number))

===
Fails
===
1
---
(document (string))
`

// corpusHelperEnv names the variable that makes TestCorpusHelper run the
// corpus test. It holds the folder of the corpus, and then the failing
// cases, one on each line.
const corpusHelperEnv = "TRANSIT_TEST_CORPUS_HELPER"

// TestCorpusHelper runs grammartest.Corpus in a process of its own for
// TestCorpusExpectsTheRecordedFailures, so that a failure of the corpus test
// does not fail this test binary. It does nothing without corpusHelperEnv.
func TestCorpusHelper(t *testing.T) {
	v := os.Getenv(corpusHelperEnv)
	if v == "" {
		t.Skip("the test runs only as the helper of TestCorpusExpectsTheRecordedFailures")
	}
	lines := strings.Split(v, "\n")
	grammartest.Corpus(t, json.Language(), lines[0], lines[1:]...)
}

// TestCorpusExpectsTheRecordedFailures makes sure that the corpus test
// expects the cases that grammars/grammars.json records as failing upstream
// to fail, and no other case, and that it names each one in its output.
func TestCorpusExpectsTheRecordedFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.txt"), []byte(failingCorpus), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		failing []string
		pass    bool
		output  []string
	}{
		{"the recorded failure", []string{"main/Fails"}, true, []string{
			"the case fails, as it fails upstream (testdata/failing.txt): the trees differ",
			"--- PASS: TestCorpusHelper/main/Fails",
		}},
		{"no recorded failure", nil, false, []string{
			"--- FAIL: TestCorpusHelper/main/Fails",
			"expected: (document (string))",
		}},
		{"a recorded failure that passes", []string{"main/Fails", "main/Passes"}, false, []string{
			`the case "main/Passes" does not fail`,
			"--- PASS: TestCorpusHelper/main/Passes",
		}},
		{"a recorded failure that the corpus does not hold", []string{"main/Fails", "main/Missing"}, false, []string{
			`the case "main/Missing" does not fail, or the corpus does not hold it`,
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCorpusHelper$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), corpusHelperEnv+"="+strings.Join(append([]string{dir}, c.failing...), "\n"))
			out, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			switch {
			case err == nil && !c.pass:
				t.Errorf("the corpus test passes, want a failure:\n%s", out)
			case err != nil && !errors.As(err, &exitErr):
				t.Fatal(err)
			case err != nil && c.pass:
				t.Errorf("the corpus test fails, want a pass:\n%s", out)
			}
			for _, want := range c.output {
				if !strings.Contains(string(out), want) {
					t.Errorf("the output does not hold %q:\n%s", want, out)
				}
			}
		})
	}
}

// TestCorpusFailuresMatchTheRecord runs the corpus test of each grammar
// whose corpus record in grammars/grammars.json names cases that fail
// upstream, with its C grammar in the Go runtime. The cases that fail in
// Go must be the cases of the record, by the same names (D79).
func TestCorpusFailuresMatchTheRecord(t *testing.T) {
	root, cache := setup(t)
	b, err := os.ReadFile(filepath.Join(root, "grammars", "grammars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Grammars []struct {
			Name       string `json:"name"`
			Repository string `json:"repository"`
			Path       string `json:"path"`
			Corpus     *struct {
				Failing []string `json:"failing"`
			} `json:"corpus"`
		} `json:"grammars"`
	}
	if err := stdjson.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	for _, g := range rec.Grammars {
		if g.Corpus == nil || len(g.Corpus.Failing) == 0 {
			continue
		}
		t.Run(g.Name, func(t *testing.T) {
			dir, corpus := grammarDirs(cache, fixture{Name: g.Name, Repository: g.Repository, Path: g.Path})
			so, err := BuildGrammar(context.Background(), dir, cache)
			if errors.Is(err, ErrMissing) {
				t.Skipf("skipping: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			gr, err := Load(so, g.Name)
			if err != nil {
				t.Fatal(err)
			}
			grammartest.Corpus(t, gr.Language, corpus, g.Corpus.Failing...)
		})
	}
}

// TestHighlightJavaScript runs the highlight test on test/highlight of the
// fixture grammar javascript, with its C grammar in the Go runtime, in the
// folder of its tree-sitter.json. tree-sitter test of the upstream tool
// passes each file at the tag of grammars/grammars.json. The files need the
// highlight queries that tree-sitter.json lists, queries/locals.scm, the
// last of two patterns that capture one node, and the injection of
// JavaScript into a template string (D80).
func TestHighlightJavaScript(t *testing.T) {
	// the helper finds the repository from the working folder
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	language, dir := loadJavaScript(t)
	grammartest.Highlight(t, language, os.DirFS(dir), filepath.Join(dir, "test", "highlight"))

	// Two assertions that the upstream tool fails with these messages.
	testDir := t.TempDir()
	for name, change := range map[string][2]string{
		"variables.js": {"^ variable.parameter", "^ variable"},
		"injection.js": {"//            ^ variable", "//            ^ string"},
	} {
		b, err := os.ReadFile(filepath.Join(dir, "test", "highlight", name))
		if err != nil {
			t.Fatal(err)
		}
		src := strings.Replace(string(b), change[0], change[1], 1)
		if err := os.WriteFile(filepath.Join(testDir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHighlightHelper$", "-test.v", "-test.count=1")
	cmd.Dir = wd
	cmd.Env = append(os.Environ(), highlightHelperEnv+"="+testDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("the highlight test passes, want a failure:\n%s", out)
	}
	for _, want := range []string{
		"Failure - row: 0, column: 14, expected highlight 'string', actual highlights: 'variable'",
		"Failure - row: 9, column: 26, expected highlight 'variable', actual highlights: 'variable.parameter'",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the output does not hold %q:\n%s", want, out)
		}
	}
}

// TestHighlightMatchesUpstream runs the highlight test on test/highlight of
// grammars in the cache of the golden harness, with their C grammars in the
// Go runtime. tree-sitter test of the upstream tool passes each file of
// each grammar at the tag of grammars/grammars.json, on 2026-09-30 (D80).
// The working folder is the folder of tree-sitter.json, because upstream
// reads the paths of the query files from that folder.
func TestHighlightMatchesUpstream(t *testing.T) {
	_, cache := setup(t)
	for _, c := range []struct{ repo, path, name string }{
		{"tree-sitter/tree-sitter-c", ".", "c"},
		{"tree-sitter/tree-sitter-c-sharp", ".", "c_sharp"},
		{"tree-sitter/tree-sitter-css", ".", "css"},
		{"tree-sitter/tree-sitter-html", ".", "html"},
		{"tree-sitter/tree-sitter-java", ".", "java"},
		{"tree-sitter/tree-sitter-php", "php", "php"},
		{"tree-sitter/tree-sitter-python", ".", "python"},
		{"tree-sitter/tree-sitter-ruby", ".", "ruby"},
		{"tree-sitter-grammars/tree-sitter-lua", ".", "lua"},
		{"tree-sitter-grammars/tree-sitter-toml", ".", "toml"},
		{"tree-sitter-grammars/tree-sitter-yaml", ".", "yaml"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(cache, "grammars", filepath.FromSlash(c.repo))
			so, err := BuildGrammar(context.Background(), filepath.Join(root, c.path), cache)
			if errors.Is(err, ErrMissing) {
				t.Skipf("skipping: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			g, err := Load(so, c.name)
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			grammartest.Highlight(t, g.Language, os.DirFS(root), filepath.Join(root, "test", "highlight"))
		})
	}
}

// highlightHelperEnv names the variable that makes TestHighlightHelper run
// the highlight test of javascript on the files of the folder that it
// holds.
const highlightHelperEnv = "TRANSIT_TEST_HIGHLIGHT_HELPER"

// TestHighlightHelper runs the highlight test in a process of its own for
// TestHighlightJavaScript. It does nothing without highlightHelperEnv.
func TestHighlightHelper(t *testing.T) {
	testDir := os.Getenv(highlightHelperEnv)
	if testDir == "" {
		t.Skip("the test runs only as the helper of TestHighlightJavaScript")
	}
	language, dir := loadJavaScript(t)
	grammartest.Highlight(t, language, os.DirFS(dir), testDir)
}

// loadJavaScript loads the C grammar of the fixture grammar javascript, and
// makes the folder of the grammar in the cache the working folder of the
// test. It returns the language and the folder.
func loadJavaScript(t *testing.T) (*transit.Language, string) {
	t.Helper()
	root, cache := setup(t)
	var f fixture
	for _, candidate := range fixtures(t, root) {
		if candidate.Name == "javascript" {
			f = candidate
		}
	}
	if f.Name == "" {
		t.Fatal("no fixture grammar javascript in grammars/grammars.json")
	}
	g, _ := loadFixture(t, cache, f)
	dir, _ := grammarDirs(cache, f)
	t.Chdir(dir)
	return g.Language, dir
}
