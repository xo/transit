package inject_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	golang "github.com/xo/transit/generate/backend/go"
	"github.com/xo/transit/inject"
)

// hostGrammar is a small grammar like the usql grammar: statements of words
// and variables, such as :name.
const hostGrammar = `{
  "name": "host",
  "rules": {
    "source": {"type": "REPEAT", "content": {"type": "SYMBOL", "name": "statement"}},
    "statement": {"type": "SEQ", "members": [
      {"type": "REPEAT1", "content": {"type": "CHOICE", "members": [
        {"type": "SYMBOL", "name": "word"},
        {"type": "SYMBOL", "name": "variable"}
      ]}},
      {"type": "STRING", "value": ";"}
    ]},
    "word": {"type": "PATTERN", "value": "[a-z_*=]+"},
    "variable": {"type": "SEQ", "members": [
      {"type": "STRING", "value": ":"},
      {"type": "SYMBOL", "name": "name"}
    ]},
    "name": {"type": "PATTERN", "value": "[a-z]+"}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}]
}`

// hostInjections injects each statement of hostGrammar, with its children,
// as the language sql.
const hostInjections = `((statement) @injection.content
  (#set! injection.language "sql")
  (#set! injection.include-children))`

// sqlGrammar is a small grammar of words and signs, with no colon, so a
// variable of hostGrammar is an error in it.
const sqlGrammar = `{
  "name": "sql",
  "rules": {
    "source": {"type": "REPEAT", "content": {"type": "CHOICE", "members": [
      {"type": "PATTERN", "value": "[a-z_]+"},
      {"type": "STRING", "value": "*"},
      {"type": "STRING", "value": "="},
      {"type": "STRING", "value": ";"}
    ]}}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}]
}`

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

// testConfig generates a grammar and returns its configuration.
func testConfig(t *testing.T, grammarJSON, name, injections string) *inject.Config {
	t.Helper()
	var diagnostics []generate.Diagnostic
	b := &languageBackend{}
	if _, _, err := generate.ParserForGrammar([]byte(grammarJSON), nil, generate.OptLevelMergeStates, b, &diagnostics); err != nil {
		t.Fatal(err)
	}
	c, err := inject.NewConfig(b.language, name, injections)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// placeholder replaces a variable :name with _name.
func placeholder(_ string, n transit.Node, src []byte) ([]byte, bool) {
	if n.Kind() != "variable" {
		return nil, false
	}
	return []byte("_" + string(src[n.StartByte()+1:n.EndByte()])), true
}

// TestWithReplacer parses the layers of a text with variables, with and
// without the replacer. With it, the sql layer parses the placeholders and
// has no error, and its offsets are the offsets of the text.
func TestWithReplacer(t *testing.T) {
	t.Parallel()
	host := testConfig(t, hostGrammar, "host", hostInjections)
	sql := testConfig(t, sqlGrammar, "sql", "")
	lookup := func(name string) (*inject.Config, bool) { return sql, name == "sql" }
	src := []byte("select * from :tbl where id = :id;") //nolint:unqueryvet // the text is an input to parse
	for _, test := range []struct {
		name     string
		opts     []inject.Option
		hasError bool
	}{
		{"without the replacer", nil, true},
		{"with the replacer", []inject.Option{inject.WithReplacer(placeholder)}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			layers, err := host.Layers(context.Background(), transit.NewParser(), src, lookup, test.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if len(layers) != 2 || layers[1].Name != "sql" {
				t.Fatalf("expected the root layer and a sql layer, got %d layers", len(layers))
			}
			root, layer := layers[0].Tree.RootNode(), layers[1].Tree.RootNode()
			if root.HasError() {
				t.Errorf("the root layer has an error: %s", root)
			}
			if layer.HasError() != test.hasError {
				t.Errorf("the sql layer has an error: %t, and want %t: %s", layer.HasError(), test.hasError, layer)
			}
			if layer.StartByte() != 0 || layer.EndByte() != len(src) {
				t.Errorf("the sql layer runs from %d to %d, and want 0 to %d", layer.StartByte(), layer.EndByte(), len(src))
			}
		})
	}
}

// TestWithReplacerLength makes sure that a replacement with another length
// than its node is an error.
func TestWithReplacerLength(t *testing.T) {
	t.Parallel()
	host := testConfig(t, hostGrammar, "host", hostInjections)
	sql := testConfig(t, sqlGrammar, "sql", "")
	lookup := func(name string) (*inject.Config, bool) { return sql, name == "sql" }
	long := func(name string, n transit.Node, src []byte) ([]byte, bool) {
		if n.Kind() != "variable" {
			return nil, false
		}
		return []byte("_long_" + string(src[n.StartByte():n.EndByte()])), true
	}
	_, err := host.Layers(context.Background(), transit.NewParser(), []byte("select :a;"), lookup, inject.WithReplacer(long))
	if err == nil || !strings.Contains(err.Error(), "the text has 8 bytes, and the node has 2") {
		t.Errorf("expected an error about the length, got: %v", err)
	}
}

// TestWithReplacerNames makes sure that the replacer gets the name of the
// injected layer, and that it is not asked about the nodes of the root
// layer outside the injected layer, nor about the children of a node that
// it replaces.
func TestWithReplacerNames(t *testing.T) {
	t.Parallel()
	host := testConfig(t, hostGrammar, "host", hostInjections)
	sql := testConfig(t, sqlGrammar, "sql", "")
	lookup := func(name string) (*inject.Config, bool) { return sql, name == "sql" }
	var asked []string
	r := func(name string, n transit.Node, src []byte) ([]byte, bool) {
		asked = append(asked, name+" "+n.Kind())
		if n.Kind() == "variable" {
			return []byte("_" + string(src[n.StartByte()+1:n.EndByte()])), true
		}
		return nil, false
	}
	if _, err := host.Layers(context.Background(), transit.NewParser(), []byte("a :b; c;"), lookup, inject.WithReplacer(r)); err != nil {
		t.Fatal(err)
	}
	want := "sql statement,sql word,sql variable,sql ;,sql statement,sql word,sql ;"
	if got := strings.Join(asked, ","); got != want {
		t.Errorf("the replacer was asked about %s, and want %s", got, want)
	}
}

// TestLayerText makes sure that the text of a layer holds the replacements
// of its layer only, with the offsets of the source. The root layer has no
// parent, so its text is the source. It also makes sure that the states of
// Layer.StatesAt are the states of a plain parse of the text of the layer
// with spaces in place of the bytes outside its ranges.
func TestLayerText(t *testing.T) {
	t.Parallel()
	host := testConfig(t, hostGrammar, "host", hostInjections)
	sql := testConfig(t, sqlGrammar, "sql", "")
	lookup := func(name string) (*inject.Config, bool) { return sql, name == "sql" }
	src := []byte("select :a; b :cd;")
	p := transit.NewParser()
	layers, err := host.Layers(context.Background(), p, src, lookup, inject.WithReplacer(placeholder))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"select :a; b :cd;",
		"select _a; b :cd;",
		"select :a; b _cd;",
	}
	if len(layers) != len(want) {
		t.Fatalf("expected %d layers, got %d", len(want), len(layers))
	}
	if &layers[0].Text[0] != &src[0] {
		t.Errorf("the text of the root layer is a copy of the source")
	}
	for i, l := range layers {
		if got := string(l.Text); got != want[i] {
			t.Errorf("layer %d: the text is %q, and want %q", i, got, want[i])
		}
	}
	for _, l := range layers[1:] {
		blank := []byte(strings.Repeat(" ", len(src)))
		for _, r := range l.Ranges {
			copy(blank[r.StartByte:r.EndByte], l.Text[r.StartByte:r.EndByte])
		}
		for offset := l.Ranges[0].StartByte; offset <= l.Ranges[len(l.Ranges)-1].EndByte; offset++ {
			got, err := l.StatesAt(context.Background(), p, offset)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.IncludedRanges()) != 1 {
				t.Errorf("the parser keeps the ranges of the layer: %v", p.IncludedRanges())
			}
			plain, err := p.StatesAt(context.Background(), blank, offset, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, plain) {
				t.Errorf("%q at %d: the states of the layer are %v, and the states of %q are %v", l.Text, offset, got, blank, plain)
			}
		}
	}
}
