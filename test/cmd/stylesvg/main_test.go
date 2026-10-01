package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestSamples highlights the sample of each grammar, and expects no ERROR
// or MISSING node and at least one highlight in each.
func TestSamples(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range grammars {
		t.Run(g.name, func(t *testing.T) {
			src, err := samples.ReadFile("samples/" + g.name + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			s, err := highlight(root, g, src)
			if err != nil {
				t.Fatal(err)
			}
			if i := slices.Index(s.errs, true); i >= 0 {
				line := strings.Count(string(src[:i]), "\n") + 1
				t.Errorf("expected no ERROR or MISSING node, got one on line %d", line)
			}
			if !slices.ContainsFunc(s.names, func(name string) bool { return name != "" }) {
				t.Error("expected a highlight, got none")
			}
		})
	}
}

// TestRun writes the files for two grammars and two styles.
func TestRun(t *testing.T) {
	out := t.TempDir()
	if err := run(io.Discard, "go,json", "monokai,metro-lamps", "", out, 2); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lang-go.svg", "lang-json.svg", "style-monokai.svg", "style-metro-lamps.svg", "index.html"} {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".svg") && !strings.HasPrefix(string(b), "<svg ") {
			t.Errorf("%s: expected an SVG file, got %.20q", name, b)
		}
	}
}
