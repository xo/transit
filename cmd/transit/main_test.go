package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateOnEveryTestGrammar runs transit generate on each test grammar,
// with the flags that the golden harness gives the upstream tool, and
// compares parser.c and node-types.json with the golden files, or the text
// on stderr with the golden error.txt.
func TestGenerateOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "generate", "testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 68 {
		t.Fatalf("expected the 68 test grammars, found %d", len(files))
	}
	variants := []struct {
		dir  string
		args []string
	}{
		{"abi14", []string{"--abi", "14"}},
		{"abi15", []string{"--abi", "15"}},
		{"abi15-nomerge", []string{"--abi", "15", "--disable-optimizations"}},
	}
	for _, f := range files {
		golden := filepath.Dir(f)
		for _, v := range variants {
			out := t.TempDir()
			var stderr bytes.Buffer
			// the path comes before the flags here, as clap allows
			code := run(append([]string{"generate", f, "-o", out}, v.args...), &stderr)
			want := filepath.Join(golden, v.dir)
			if expected, err := os.ReadFile(filepath.Join(want, "error.txt")); err == nil {
				if code != 1 || strings.TrimSpace(stderr.String()) != strings.TrimSpace(string(expected)) {
					t.Errorf("%s/%s: expected the golden error, got code %d and:\n%s", filepath.Base(golden), v.dir, code, stderr.String())
				}
				continue
			}
			if code != 0 {
				t.Errorf("%s/%s: expected no error, got code %d and:\n%s", filepath.Base(golden), v.dir, code, stderr.String())
				continue
			}
			for _, name := range []string{"parser.c", "node-types.json"} {
				actual, err := os.ReadFile(filepath.Join(out, name))
				if err != nil {
					t.Fatal(err)
				}
				expected, err := os.ReadFile(filepath.Join(want, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(actual, expected) {
					t.Errorf("%s/%s: %s differs from the golden file", filepath.Base(golden), v.dir, name)
				}
			}
		}
	}
}

// TestGenerateFlags makes sure of the errors of the flags.
func TestGenerateFlags(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args []string
		code int
		text string
	}{
		{nil, 2, "usage"},
		{[]string{"parse"}, 2, "usage"},
		{[]string{"generate", "--abi", "x"}, 1, "invalid abi version flag"},
		{[]string{"generate", "--backend", "rust"}, 1, `the backend "rust" does not exist. The backends are: c, go`},
		{[]string{"generate", "--backend", "go", t.TempDir()}, 1, "Failed to load grammar.json"},
		{[]string{"generate", "a.json", "b.json"}, 1, `unexpected argument "b.json"`},
		{[]string{"generate", filepath.Join(t.TempDir(), "missing.json")}, 1, "not found"},
	} {
		var stderr bytes.Buffer
		if code := run(test.args, &stderr); code != test.code || !strings.Contains(stderr.String(), test.text) {
			t.Errorf("%q: expected code %d and %q, got code %d and %q", test.args, test.code, test.text, code, stderr.String())
		}
	}
}

// TestGenerateGoPackage runs transit generate --backend go on a copy of the
// inputs of grammars/json, once with the folder and once with its
// grammar.json, and compares the files that it writes with the files of
// grammars/json.
func TestGenerateGoPackage(t *testing.T) {
	t.Parallel()
	src := filepath.Join("..", "..", "grammars", "json")
	for _, arg := range []string{"", "grammar.json"} {
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
		var stderr bytes.Buffer
		if code := run([]string{"generate", "--backend", "go", filepath.Join(dir, arg)}, &stderr); code != 0 {
			t.Fatalf("expected no error, got code %d and:\n%s", code, stderr.String())
		}
		for _, name := range []string{"parser.go", "node-types.json", "grammar_test.go", "example_test.go"} {
			actual, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join(src, name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, expected) {
				t.Errorf("%s differs from grammars/json/%s", name, name)
			}
		}
	}
}
