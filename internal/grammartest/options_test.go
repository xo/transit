package grammartest

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/transit"
)

// This file tests the highlight tests that name a set of options (D137):
// how Highlight skips them, and how HighlightOptions runs them. It ports no
// upstream test.

// wordGrammar is a small grammar with comments: a source is a list of words
// and numbers. "1" is a number.
const wordGrammar = `{
  "name": "tiny",
  "rules": {
    "source": {"type": "REPEAT", "content": {"type": "CHOICE", "members": [
      {"type": "SYMBOL", "name": "word"},
      {"type": "SYMBOL", "name": "number"}
    ]}},
    "word": {"type": "PATTERN", "value": "[a-z]+"},
    "number": {"type": "PATTERN", "value": "[0-9]+"},
    "comment": {"type": "PATTERN", "value": "--[^\\n]*"}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}, {"type": "SYMBOL", "name": "comment"}]
}`

// wideGrammar is wordGrammar, but a word can hold digits. So "1" is a word.
// It stands for the language of a set of options.
const wideGrammar = `{
  "name": "tiny",
  "rules": {
    "source": {"type": "REPEAT", "content": {"type": "CHOICE", "members": [
      {"type": "SYMBOL", "name": "word"},
      {"type": "SYMBOL", "name": "number"}
    ]}},
    "word": {"type": "PATTERN", "value": "[a-z0-9]+"},
    "number": {"type": "PATTERN", "value": "#[0-9]+"},
    "comment": {"type": "PATTERN", "value": "--[^\\n]*"}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}, {"type": "SYMBOL", "name": "comment"}]
}`

// wordQueries are the query files of wordGrammar and wideGrammar.
var wordQueries = fstest.MapFS{
	"queries/highlights.scm": {Data: []byte("(word) @variable\n(number) @number\n(comment) @comment\n")},
}

// TestFileOptions makes sure that only a first line of the exact form
// "-- options: <name>" names a set of options.
func TestFileOptions(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		src  string
		name string
	}{
		{"-- options: wide\n1\n", "wide"},
		{"-- options: wide\r\n1\r\n", "wide"},
		{"-- options: wide", "wide"},
		{"-- options: \n1\n", ""},
		{"--options: wide\n", ""},
		{"-- Options: wide\n", ""},
		{"a\n-- options: wide\n", ""},
		{"", ""},
	} {
		name, ok := fileOptions([]byte(c.src))
		if name != c.name || ok != (c.name != "") {
			t.Errorf("fileOptions(%q) = %q, %t, want %q", c.src, name, ok, c.name)
		}
	}
}

// optionsHelperEnv names the variable that makes TestHighlightOptionsHelper
// run the highlight tests. It holds the folder of the package.
const optionsHelperEnv = "TRANSIT_TEST_OPTIONS_HELPER"

// TestHighlightOptionsHelper runs Highlight and HighlightOptions on the
// files of testdata/highlight in the folder that optionsHelperEnv names, in
// a process of its own for TestHighlightOptions. So a failure of a
// highlight test does not fail this test binary. HighlightOptions knows
// only the options wide. It does nothing without optionsHelperEnv.
func TestHighlightOptionsHelper(t *testing.T) {
	dir := os.Getenv(optionsHelperEnv)
	if dir == "" {
		t.Skip("the test runs only as the helper of TestHighlightOptions")
	}
	word, wide := grammarLanguage(t, wordGrammar), grammarLanguage(t, wideGrammar)
	t.Chdir(dir)
	t.Run("Highlight", func(t *testing.T) {
		Highlight(t, word, wordQueries, "testdata/highlight")
	})
	t.Run("HighlightOptions", func(t *testing.T) {
		HighlightOptions(t, func(name string) (*transit.Language, bool) {
			if name == "wide" {
				return wide, true
			}
			return nil, false
		}, wordQueries, "testdata/highlight")
	})
}

// TestHighlightOptions makes sure that Highlight skips a file that names a
// set of options, and that HighlightOptions runs it with the language of
// those options, skips the other files, and fails on a name that it does
// not know (D137).
func TestHighlightOptions(t *testing.T) {
	t.Parallel()
	// plain passes with wordGrammar, and wide passes only with wideGrammar
	const (
		plain = "a 1\n-- <- variable\n"
		wide  = "-- options: wide\n1\n-- <- variable\n"
	)
	for _, c := range []struct {
		name   string
		files  map[string]string
		pass   bool
		output []string
	}{
		{"a file of each kind", map[string]string{"plain.txt": plain, "sub/wide.txt": wide}, true, []string{
			"--- SKIP: TestHighlightOptionsHelper/Highlight/sub/wide.txt",
			"the file names the options wide, so HighlightOptions runs it",
			"--- PASS: TestHighlightOptionsHelper/Highlight/plain.txt",
			"--- SKIP: TestHighlightOptionsHelper/HighlightOptions/plain.txt",
			"--- PASS: TestHighlightOptionsHelper/HighlightOptions/sub/wide.txt",
		}},
		{"an assertion that fails", map[string]string{"wide.txt": "-- options: wide\n1\n-- <- number\n"}, false, []string{
			"--- FAIL: TestHighlightOptionsHelper/HighlightOptions/wide.txt",
			"expected highlight 'number', actual highlights: 'variable'",
		}},
		{"an unknown name", map[string]string{"wide.txt": wide, "other.txt": "-- options: other\n1\n"}, false, []string{
			"--- FAIL: TestHighlightOptionsHelper/HighlightOptions/other.txt",
			"no language for the options other",
			"--- PASS: TestHighlightOptionsHelper/HighlightOptions/wide.txt",
		}},
		{"no file names options", map[string]string{"plain.txt": plain}, false, []string{
			"--- PASS: TestHighlightOptionsHelper/Highlight/plain.txt",
			"no file of testdata/highlight names options",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := map[string]string{}
			for name, text := range c.files {
				files["testdata/highlight/"+name] = text
			}
			writeFiles(t, dir, files)
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHighlightOptionsHelper$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), optionsHelperEnv+"="+dir)
			out, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			switch {
			case err == nil && !c.pass:
				t.Errorf("the highlight tests pass, want a failure:\n%s", out)
			case err != nil && !errors.As(err, &exitErr):
				t.Fatal(err)
			case err != nil && c.pass:
				t.Errorf("the highlight tests fail, want a pass:\n%s", out)
			}
			for _, want := range c.output {
				if !strings.Contains(string(out), want) {
					t.Errorf("the output does not hold %q:\n%s", want, out)
				}
			}
		})
	}
}
