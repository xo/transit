package cgrammar

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oracleLanguages builds the grammars named in names, from the cache of the
// golden harness, and returns them as languages of the input of the oracle.
// A grammar can be in any set of grammars/grammars.json. It skips the test
// when a grammar is not in the cache.
func oracleLanguages(t *testing.T, root, cache string, names ...string) []OracleLanguage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "grammars", "grammars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Grammars []fixture `json:"grammars"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	var out []OracleLanguage
	for _, name := range names {
		i := -1
		for j, g := range rec.Grammars {
			if g.Name == name && g.Status == "available" {
				i = j
				break
			}
		}
		if i < 0 {
			t.Fatalf("grammars.json has no available grammar %s", name)
		}
		dir, _ := grammarDirs(cache, rec.Grammars[i])
		so, err := BuildGrammar(context.Background(), dir, cache)
		if errors.Is(err, ErrMissing) {
			t.Skipf("skipping: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		gb, err := os.ReadFile(filepath.Join(dir, "src", "grammar.json"))
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(gb, &head); err != nil {
			t.Fatal(err)
		}
		injections, err := os.ReadFile(filepath.Join(dir, "queries", "injections.scm"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		out = append(out, OracleLanguage{
			Name:       name,
			Library:    so,
			Symbol:     "tree_sitter_" + head.Name,
			Injections: string(injections),
		})
	}
	return out
}

// oracleSetup builds the oracle, and returns it with the root of the
// repository and the folder of the cache. It skips the test when cargo or
// the checkout of upstream is missing, or in short mode.
func oracleSetup(t *testing.T) (string, string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("the oracle test builds the oracle and grammars, which takes minutes")
	}
	root, err := Root()
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	cache, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := BuildOracle(context.Background(), root, cache)
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return oracle, root, cache
}

// TestOracleRuns runs the Rust oracle of D71 on an HTML text with a script
// and a style element, and on the inputs of the injection tests of
// crates/cli/src/tests/highlight_test.rs of upstream.
func TestOracleRuns(t *testing.T) {
	oracle, root, cache := oracleSetup(t)
	languages := oracleLanguages(t, root, cache, "html", "javascript", "css", "rust")

	t.Run("html", func(t *testing.T) {
		src := "<html>\n<script>let x = 1;</script>\n<style>p { color: red; }</style>\n</html>\n"
		layers, err := RunOracle(context.Background(), oracle, OracleInput{
			Languages: languages,
			Root:      "html",
			Source:    src,
		})
		if err != nil {
			t.Fatal(err)
		}
		logLayers(t, layers)
		want := []struct {
			name  string
			depth int
		}{{"html", 0}, {"javascript", 1}, {"css", 1}}
		if len(layers) != len(want) {
			t.Fatalf("got %d layers, want %d", len(layers), len(want))
		}
		for i, w := range want {
			if layers[i].Name != w.name || layers[i].Depth != w.depth {
				t.Errorf("layer %d is %s at depth %d, want %s at depth %d", i, layers[i].Name, layers[i].Depth, w.name, w.depth)
			}
		}
	})

	// The inputs of the tests of upstream that inject a language.
	for _, tt := range []struct {
		name string
		root string
		src  string
	}{
		{
			"test_highlighting_injected_html_in_javascript",
			"javascript",
			"const s = html `<div>${a < b}</div>`;",
		},
		{
			"test_highlighting_injected_javascript_in_html_mini",
			"html",
			"<script>const x = new Thing();</script>",
		},
		{
			"test_highlighting_injected_javascript_in_html",
			"html",
			strings.Join([]string{
				"<body>",
				"  <script>",
				"    const x = new Thing();",
				"  </script>",
				"</body>",
			}, "\n"),
		},
		{
			"test_highlighting_with_content_children_included",
			"rust",
			strings.Join([]string{"assert!(", "    a.b.c() < D::e::<F>()", ");"}, "\n"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			layers, err := RunOracle(context.Background(), oracle, OracleInput{
				Languages: languages,
				Root:      tt.root,
				Source:    tt.src,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(layers) == 0 || layers[0].Name != tt.root || layers[0].Depth != 0 {
				t.Errorf("the first layer is not the root layer %s", tt.root)
			}
			logLayers(t, layers)
		})
	}
}

// logLayers logs each layer of the oracle.
func logLayers(t *testing.T, layers []OracleLayer) {
	t.Helper()
	for _, l := range layers {
		t.Logf("%s depth %d ranges %v tree %s", l.Name, l.Depth, l.Ranges, l.Tree)
	}
}
