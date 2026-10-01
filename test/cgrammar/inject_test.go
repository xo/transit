package cgrammar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/inject"
)

// injectRoots are the grammars whose corpus inputs the test of inject
// parses as the root layer. Each one has queries/injections.scm, and
// together they use combined injections, include-children and a language
// that a capture names. nix, bitbake and zig also have injections, but their
// repositories hold no corpus.
//
// doxygen and re2c set injection.parent, and they are not roots. As a root,
// each one injects itself: injection.parent names the root language (D72),
// and a layer of re2c over a host_lang node holds a host_lang node over the
// same range again. Upstream then recurses with no end, and so does inject.
var injectRoots = []string{
	"html", "javascript", "rust", "cpp", "haskell", "julia", "markdown",
	"markdown_inline", "svelte", "elixir", "heex", "twig", "kdl", "postgres",
	"plpgsql", "perl", "lua", "luau", "http", "gleam", "swift", "blade",
}

// injectedNames matches the language names that an injection query sets
// with #set! injection.language.
var injectedNames = regexp.MustCompile(`injection\.language\s+"([^"]+)"`)

// injectLanguages returns the root grammar and each grammar that its
// injection query names, and the grammars that their queries name, as far
// as grammars.json has them.
func injectLanguages(t *testing.T, root, cache, name string) []OracleLanguage {
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
	available := map[string]fixture{}
	for _, g := range rec.Grammars {
		if g.Status == "available" {
			available[g.Name] = g
		}
	}
	names := []string{name}
	for i := 0; i < len(names); i++ {
		dir, _ := grammarDirs(cache, available[names[i]])
		query, err := os.ReadFile(filepath.Join(dir, "queries", "injections.scm"))
		if err != nil {
			continue
		}
		for _, m := range injectedNames.FindAllStringSubmatch(string(query), -1) {
			if _, ok := available[m[1]]; ok && !slices.Contains(names, m[1]) {
				names = append(names, m[1])
			}
		}
	}
	return oracleLanguages(t, root, cache, names...)
}

// goInjectConfigs loads the languages into the Go runtime, and returns the
// inject configuration of each, by name. With packages, a language that
// has a grammar package in goPackages gets the grammar package, and each
// other language gets the tables of its C grammar.
func goInjectConfigs(t *testing.T, languages []OracleLanguage, packages bool) map[string]*inject.Config {
	t.Helper()
	configs := map[string]*inject.Config{}
	for _, l := range languages {
		language := findGoPackage(l.Name)
		if !packages || language == nil {
			g, err := Load(l.Library, strings.TrimPrefix(l.Symbol, "tree_sitter_"))
			if err != nil {
				t.Fatal(err)
			}
			language = g.Language
		}
		c, err := inject.NewConfig(language, l.Name, l.Injections)
		if err != nil {
			t.Fatalf("%s: %v", l.Name, err)
		}
		configs[l.Name] = c
	}
	return configs
}

// oracleValue maps a value of the oracle to the Go form. The root layer of
// upstream ends at the largest usize of Rust, which the parser cuts to the
// largest uint32. The root layer of inject ends there.
func oracleValue(v uint64) int {
	return int(min(v, math.MaxUint32))
}

// describeOracleLayers writes the layers of the oracle in the form of
// describeGoLayers.
func describeOracleLayers(layers []OracleLayer) string {
	var b strings.Builder
	for _, l := range layers {
		fmt.Fprintf(&b, "%s depth %d ranges", l.Name, l.Depth)
		for _, r := range l.Ranges {
			fmt.Fprintf(&b, " %d-%d(%d,%d)-(%d,%d)", oracleValue(r.StartByte), oracleValue(r.EndByte),
				oracleValue(r.StartPoint.Row), oracleValue(r.StartPoint.Column),
				oracleValue(r.EndPoint.Row), oracleValue(r.EndPoint.Column))
		}
		fmt.Fprintf(&b, "\n  %s\n", l.Tree)
	}
	return b.String()
}

// describeGoLayers writes the layers of inject: the name of the config, the
// depth, the ranges and the tree of each.
func describeGoLayers(layers []inject.Layer) string {
	var b strings.Builder
	for _, l := range layers {
		fmt.Fprintf(&b, "%s depth %d ranges", l.Config.Name(), l.Depth)
		for _, r := range l.Ranges {
			fmt.Fprintf(&b, " %d-%d(%d,%d)-(%d,%d)", r.StartByte, r.EndByte,
				r.StartPoint.Row, r.StartPoint.Column, r.EndPoint.Row, r.EndPoint.Column)
		}
		fmt.Fprintf(&b, "\n  %s\n", l.Tree.RootNode())
	}
	return b.String()
}

// injectVariant is a set of inject configurations: the tables of the C
// grammars, or the grammar packages where goPackages has them.
type injectVariant struct {
	source  string
	configs map[string]*inject.Config
}

// lookup finds the configuration of a language name.
func (v injectVariant) lookup(name string) (*inject.Config, bool) {
	c, ok := v.configs[name]
	return c, ok
}

// TestInjectLayersMatchOracle parses the corpus inputs of each grammar of
// injectRoots with inject, and with crates/highlight of upstream through the
// Rust oracle (D71), and compares the layers: the name, the depth, the
// ranges and the tree of each, in the order of D72. inject runs with the
// tables of the C grammars, and again with the grammar packages when one of
// the languages has one. The oracle runs once for both, with the C
// grammars.
func TestInjectLayersMatchOracle(t *testing.T) {
	t.Parallel()
	oracle, root, cache := oracleSetup(t)
	for _, name := range injectRoots {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			languages := injectLanguages(t, root, cache, name)
			variants := []injectVariant{{"C", goInjectConfigs(t, languages, false)}}
			if slices.ContainsFunc(languages, func(l OracleLanguage) bool { return findGoPackage(l.Name) != nil }) {
				variants = append(variants, injectVariant{"Go", goInjectConfigs(t, languages, true)})
			}
			_, corpus := grammarDirs(cache, fixtureByName(t, root, name))
			examples, err := ReadCorpus(corpus)
			if err != nil {
				t.Fatal(err)
			}
			examples = examples[:min(len(examples), 40)]
			p := transit.NewParser()
			layerCount := 0
			differ := make([]int, len(variants))
			for _, e := range examples {
				oracleLayers, err := RunOracle(context.Background(), oracle, OracleInput{
					Languages: languages,
					Root:      name,
					Source:    string(e.Input),
				})
				if err != nil {
					t.Fatalf("%s: %v", e.Name, err)
				}
				want := describeOracleLayers(oracleLayers)
				layerCount += len(oracleLayers)
				for vi, v := range variants {
					layers, err := v.configs[name].Layers(context.Background(), p, e.Input, v.lookup)
					if err != nil {
						t.Fatalf("%s: %s: %v", v.source, e.Name, err)
					}
					if got := describeGoLayers(layers); got != want {
						differ[vi]++
						if differ[vi] <= 3 {
							t.Errorf("%s: %s: the layers differ:\nupstream:\n%s\ninject:\n%s", v.source, e.Name, want, got)
						}
					}
				}
			}
			for vi, v := range variants {
				t.Logf("%s: %d inputs, %d layers, %d differ", v.source, len(examples), layerCount, differ[vi])
			}
		})
	}
}

// fixtureByName returns the record of an available grammar of
// grammars.json.
func fixtureByName(t *testing.T, root, name string) fixture {
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
	for _, g := range rec.Grammars {
		if g.Name == name && g.Status == "available" {
			return g
		}
	}
	t.Fatalf("grammars.json has no available grammar %s", name)
	return fixture{}
}

// htmlInject returns the inject configurations of html, javascript and css,
// and a lookup of them. With packages, html and javascript are the grammar
// packages.
func htmlInject(t *testing.T, packages bool) (*inject.Config, func(string) (*inject.Config, bool)) {
	t.Helper()
	_, root, cache := oracleSetup(t)
	v := injectVariant{configs: goInjectConfigs(t, oracleLanguages(t, root, cache, "html", "javascript", "css"), packages)}
	return v.configs["html"], v.lookup
}

// htmlSample is an HTML text with a script and a style element.
const htmlSample = "<html>\n<script>let x = 1;</script>\n<style>p { color: red; }</style>\n</html>\n"

// TestInjectLayersRules checks the rules of Layers in D72 that the oracle
// does not see: the parser has no included ranges after the call, a name
// that the lookup does not find is skipped, and an ended context gives its
// error and no layers. It runs with the tables of the C grammars, and with
// the grammar packages of html and javascript.
func TestInjectLayersRules(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"C", "Go"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			html, lookup := htmlInject(t, source == "Go")
			p := transit.NewParser()

			layers, err := html.Layers(context.Background(), p, []byte(htmlSample), lookup)
			if err != nil {
				t.Fatal(err)
			}
			if got := describeGoLayers(layers); !strings.HasPrefix(got, "html depth 0") || strings.Count(got, "depth 1") != 2 {
				t.Errorf("the layers are:\n%s", got)
			}
			if got := p.IncludedRanges(); len(got) != 1 || got[0].StartByte != 0 || got[0].EndByte != math.MaxUint32 {
				t.Errorf("after Layers, the included ranges are %v", got)
			}

			onlyCSS := func(name string) (*inject.Config, bool) {
				if name == "css" {
					return lookup(name)
				}
				return nil, false
			}
			layers, err = html.Layers(context.Background(), p, []byte(htmlSample), onlyCSS)
			if err != nil {
				t.Fatal(err)
			}
			if len(layers) != 2 || layers[1].Config.Name() != "css" || layers[1].Name != "css" {
				t.Errorf("with a lookup of css only, the layers are:\n%s", describeGoLayers(layers))
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			big := []byte(strings.Repeat(htmlSample, 200))
			layers, err = html.Layers(ctx, p, big, lookup)
			if !errors.Is(err, context.Canceled) || layers != nil {
				t.Errorf("with an ended context, Layers gives %d layers and %v", len(layers), err)
			}
		})
	}
}

// TestInjectNewConfigError checks that a query that does not compile gives
// a *transit.QueryError, wrapped, with the C tables of html and with its
// grammar package.
func TestInjectNewConfigError(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"C", "Go"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			html, _ := htmlInject(t, source == "Go")
			_, err := inject.NewConfig(html.Language(), "html", "(no_such_node) @injection.content")
			var qerr *transit.QueryError
			if !errors.As(err, &qerr) || qerr.Kind != transit.QueryErrorNodeType {
				t.Errorf("the error is %v", err)
			}
		})
	}
}
