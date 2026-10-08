package grammartest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	golang "github.com/xo/transit/generate/backend/go"
	"github.com/xo/transit/internal/corpus"
)

// This file tests testdata/failing.txt of a grammar package (D88): how the
// corpus test reads it, and how the generator test compares it with the
// record. It ports no upstream test.

// tinyGrammar is a small grammar: a source is a list of words.
const tinyGrammar = `{
  "name": "tiny",
  "rules": {
    "source": {"type": "REPEAT", "content": {"type": "SYMBOL", "name": "word"}},
    "word": {"type": "PATTERN", "value": "[a-z]+"}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}]
}`

// tinyCorpus holds a case of tinyGrammar that passes and a case whose
// expected tree is wrong.
const tinyCorpus = `===
Passes
===
a b
---
(source (word) (word))

===
Fails
===
a
---
(source (number))
`

// languageBackend keeps the language of the tables that the Go backend
// writes.
type languageBackend struct {
	language *transit.Language
}

// Render keeps the language of the tables. It returns no code.
func (b *languageBackend) Render(in *generate.RenderInput) (string, error) {
	tables, err := golang.Tables(in)
	if err != nil {
		return "", err
	}
	b.language = transit.NewLanguage(tables)
	return "", nil
}

// tinyLanguage runs the generator on tinyGrammar, and returns its language.
func tinyLanguage(t *testing.T) *transit.Language {
	t.Helper()
	var diagnostics []generate.Diagnostic
	b := &languageBackend{}
	if _, _, err := generate.ParserForGrammar([]byte(tinyGrammar), nil, generate.OptLevelMergeStates, b, &diagnostics); err != nil {
		t.Fatal(err)
	}
	return b.language
}

// writeFiles writes files into dir, each name with its text.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// failingHelperEnv names the variable that makes TestFailingHelper run the
// corpus test. It holds the folder of the package.
const failingHelperEnv = "TRANSIT_TEST_FAILING_HELPER"

// TestFailingHelper runs Corpus on tinyGrammar in the folder that
// failingHelperEnv names, as grammar_test.go runs it, in a process of its
// own for TestCorpusReadsFailing. So a failure of the corpus test does not
// fail this test binary. It does nothing without failingHelperEnv.
func TestFailingHelper(t *testing.T) {
	dir := os.Getenv(failingHelperEnv)
	if dir == "" {
		t.Skip("the test runs only as the helper of TestCorpusReadsFailing")
	}
	language := tinyLanguage(t)
	t.Chdir(dir)
	Corpus(t, language, "testdata/corpus")
}

// TestCorpusReadsFailing makes sure that the corpus test reads the cases
// that fail upstream from testdata/failing.txt of the package, when
// grammar_test.go gives no list (D88).
func TestCorpusReadsFailing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		failing string
		pass    bool
		output  []string
	}{
		{"the file names the failure", "main/Fails\n", true, []string{
			"the case fails, as it fails upstream",
			"--- PASS: TestFailingHelper/main/Fails",
		}},
		{"no file", "", false, []string{
			"--- FAIL: TestFailingHelper/main/Fails",
			"expected: (source (number))",
		}},
		{"the file names a case that passes", "main/Fails\nmain/Passes\n", false, []string{
			`the case "main/Passes" does not fail`,
		}},
		{"no newline at the end", "main/Fails", false, []string{
			"testdata/failing.txt: the file does not end with a newline",
		}},
		{"a blank line", "main/Fails\n\n", false, []string{
			"testdata/failing.txt: the file holds a blank line",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := map[string]string{"testdata/corpus/main.txt": tinyCorpus}
			if c.failing != "" {
				files[corpus.FailingPath] = c.failing
			}
			writeFiles(t, dir, files)
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFailingHelper$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), failingHelperEnv+"="+dir)
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

// TestFailingText makes sure of the form of testdata/failing.txt, and that
// corpus.ReadFailing reads the names that failingText writes.
func TestFailingText(t *testing.T) {
	for _, names := range [][]string{
		{"main/B"},
		{"main/B", "sub/file/D / E", "main/B"},
	} {
		text := failingText(names)
		if want := strings.Join(names, "\n") + "\n"; text != want {
			t.Errorf("failingText(%q) = %q, want %q", names, text, want)
		}
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{corpus.FailingPath: text})
		t.Chdir(dir)
		got, err := corpus.ReadFailing(".")
		if err != nil || !slices.Equal(got, names) {
			t.Errorf("ReadFailing = %q, %v, want %q", got, err, names)
		}
	}
	if text := failingText(nil); text != "" {
		t.Errorf("failingText(nil) = %q, want no text", text)
	}
}

// TestCompareFailing makes sure that the generator test compares
// testdata/failing.txt with the record of the grammar in a checkout of
// transit (D88).
func TestCompareFailing(t *testing.T) {
	record := func(names ...string) *golang.Record {
		return &golang.Record{Corpus: &golang.CorpusResult{Failing: names}}
	}
	for _, c := range []struct {
		name    string
		rec     *golang.Record
		failing string
		err     string
	}{
		{"no corpus and no file", &golang.Record{}, "", ""},
		{"no failure and no file", record(), "", ""},
		{"the file names the failures", record("main/B", "main/C"), "main/B\nmain/C\n", ""},
		{"no file", record("main/B"), "", "testdata/failing.txt does not exist"},
		{"a file and no failure", record(), "main/B\n", "testdata/failing.txt exists"},
		{"another order", record("main/B", "main/C"), "main/C\nmain/B\n", "is not the file that the golden harness writes"},
		{"another name", record("main/B"), "main/C\n", "is not the file that the golden harness writes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if c.failing != "" {
				writeFiles(t, dir, map[string]string{corpus.FailingPath: c.failing})
			}
			t.Chdir(dir)
			err := compareFailing(c.rec)
			switch {
			case c.err == "" && err != nil:
				t.Errorf("compareFailing = %v, want no error", err)
			case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
				t.Errorf("compareFailing = %v, want an error that holds %q", err, c.err)
			}
		})
	}
}
