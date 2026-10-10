package c_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
)

// mode is one way that the golden harness ran the upstream tool (D17, D19).
type mode struct {
	name string
	abi  int
	opts generate.OptLevel
}

// modes are the ways that the golden harness ran the upstream tool on the
// test grammars, in test/cmd/golden.
var modes = []mode{
	{"abi14", 14, generate.OptLevelMergeStates},
	{"abi15", 15, generate.OptLevelMergeStates},
	{"abi15-nomerge", 15, 0},
}

// TestRenderOnEveryTestGrammar runs the whole generator with the C backend on
// each test grammar, in each mode of the golden harness, and compares
// parser.c and node-types.json with the golden files byte for byte. A
// grammar that the generator rejects must have the same error in its golden
// file.
//
// The harness ran the upstream tool in a temporary folder with no
// tree-sitter.json, so the test grammars have no semantic version.
func TestRenderOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 70 {
		t.Fatalf("expected the grammar.json of the 70 test grammars in testdata, found %d", len(files))
	}
	matched, rejected := 0, 0
	for _, f := range files {
		name := filepath.Base(filepath.Dir(f))
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range modes {
			dir := filepath.Join(filepath.Dir(f), m.name)
			parser, err := generateForTest(b, m.abi, nil, m.opts)
			if err != nil {
				golden, rerr := os.ReadFile(filepath.Join(dir, "error.txt"))
				if rerr != nil {
					t.Errorf("%s/%s: the generator gives %q, and the grammar has no error golden: %v", name, m.name, err, rerr)
					continue
				}
				if !strings.Contains(string(golden), "Caused by:\n"+indent(err.Error())+"\n") {
					t.Errorf("%s/%s: expected the error of the golden file, got: %q", name, m.name, err)
					continue
				}
				rejected++
				continue
			}
			parserC, err := os.ReadFile(filepath.Join(dir, "parser.c"))
			if err != nil {
				t.Errorf("%s/%s: the generator succeeds, and the grammar has no parser.c golden: %v", name, m.name, err)
				continue
			}
			nodeTypes, err := os.ReadFile(filepath.Join(dir, "node-types.json"))
			if err != nil {
				t.Fatal(err)
			}
			ok := true
			if diff := firstDiff(string(parserC), parser.Code); diff != "" {
				t.Errorf("%s/%s: parser.c differs from the golden file:\n%s", name, m.name, diff)
				ok = false
			}
			if diff := firstDiff(string(nodeTypes), parser.NodeTypesJSON); diff != "" {
				t.Errorf("%s/%s: node-types.json differs from the golden file:\n%s", name, m.name, diff)
				ok = false
			}
			if ok {
				matched++
			}
		}
	}
	t.Logf("%d of %d parser.c and node-types.json pairs match, and %d runs are rejected with the golden error", matched, 3*len(files)-rejected, rejected)
	if matched != 3*58 || rejected != 3*12 {
		t.Errorf("expected 174 matches and 36 rejections, got: %d and %d", matched, rejected)
	}
}

// fixtureRecord is the part of grammars/grammars.json that the test reads.
type fixtureRecord struct {
	Grammars []struct {
		Name       string `json:"name"`
		Repository string `json:"repository"`
		Path       string `json:"path"`
		Set        string `json:"set"`
		Status     string `json:"status"`
		Golden     map[string]struct {
			ParserC   string `json:"parser_c"`
			NodeTypes string `json:"node_types"`
			Error     string `json:"error"`
		} `json:"golden"`
	} `json:"grammars"`
}

// allGrammars asks TestRenderOnEveryRecordedGrammar for every grammar of the
// record, and not only the fixture grammars. Pass it after -args:
//
//	go test ./generate/backend/c -run Recorded -args -all-grammars
var allGrammars = flag.Bool("all-grammars", false, "generate every grammar of grammars/grammars.json, not only the fixture grammars")

// TestRenderOnEveryRecordedGrammar runs the whole generator with the C
// backend on each grammar of grammars/grammars.json, at ABI 14 and ABI 15,
// and compares the SHA-256 of parser.c and node-types.json with the record.
// It reads the grammars from the cache of the golden harness, and it skips
// when the cache is absent, or in short mode. It takes the fixture grammars,
// and every other available grammar with the flag -all-grammars, because the
// whole set takes much longer.
//
// The harness ran the upstream tool in the folder of the grammar, so the
// semantic version comes from the nearest tree-sitter.json, as
// read_grammar_version in generate.rs finds it.
func TestRenderOnEveryRecordedGrammar(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the recorded grammars are slow to generate")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder:", err)
	}
	cache := filepath.Join(home, ".cache", "transit", "grammars")
	if _, err := os.Stat(cache); err != nil {
		t.Skip("no cache of the fixture grammars:", err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "grammars", "grammars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec fixtureRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	total, matched := 0, 0
	for _, g := range rec.Grammars {
		if g.Status != "available" || (g.Set != "fixture" && !*allGrammars) {
			continue
		}
		dir := filepath.Join(cache, filepath.Base(filepath.Dir(g.Repository)), filepath.Base(g.Repository), g.Path)
		grammarJSON, err := os.ReadFile(filepath.Join(dir, "src", "grammar.json"))
		if err != nil {
			t.Logf("%s: skipped, the cache has no grammar.json: %v", g.Name, err)
			continue
		}
		version, err := generate.ReadGrammarVersion(dir)
		if err != nil {
			t.Errorf("%s: %v", g.Name, err)
			continue
		}
		for _, m := range modes[:2] {
			want, ok := g.Golden[m.name]
			if !ok {
				continue
			}
			total++
			if fixtureMatches(t, t.Errorf, g.Name, m, grammarJSON, version, want.ParserC, want.NodeTypes, want.Error) {
				matched++
			}
		}
	}
	t.Logf("%d of %d grammar runs match the record", matched, total)
}

// fixtureMatches generates a fixture grammar in one mode, and reports whether
// the output matches the record. It calls report for each difference.
func fixtureMatches(t *testing.T, report func(string, ...any), name string, m mode, grammarJSON []byte, version *generate.SemanticVersion, parserC, nodeTypes, wantErr string) bool {
	t.Helper()
	parser, err := generateForTest(grammarJSON, m.abi, version, m.opts)
	if err != nil {
		if wantErr == "" || !strings.Contains(wantErr, "Caused by:\n"+indent(err.Error())) {
			report("%s/%s: the generator gives %q, and the record has %q", name, m.name, err, wantErr)
			return false
		}
		return true
	}
	ok := true
	if got := sum(parser.Code); got != parserC {
		report("%s/%s: parser.c has the SHA-256 %s, and the record has %s", name, m.name, got, parserC)
		ok = false
	}
	if got := sum(parser.NodeTypesJSON); got != nodeTypes {
		report("%s/%s: node-types.json has the SHA-256 %s, and the record has %s", name, m.name, got, nodeTypes)
		ok = false
	}
	return ok
}

// TestRenderABIError checks the error for an ABI version that the backend
// does not write.
func TestRenderABIError(t *testing.T) {
	t.Parallel()
	for _, abi := range []int{13, 16} {
		_, err := c.Backend{}.Render(&generate.RenderInput{ABIVersion: abi})
		if _, ok := errors.AsType[*c.RenderError](err); !ok {
			t.Fatalf("ABI %d: expected a RenderError, got: %v", abi, err)
		}
		expected := "This version of Tree-sitter can only generate parsers with ABI version 14 - 15, not " + strconv.Itoa(abi)
		if err.Error() != expected {
			t.Errorf("ABI %d: expected %q, got: %q", abi, expected, err)
		}
	}
}

// generateForTest runs the whole generator with the C backend on a
// grammar.json.
func generateForTest(grammarJSON []byte, abi int, version *generate.SemanticVersion, opts generate.OptLevel) (*generate.GeneratedParser, error) {
	var diagnostics []generate.Diagnostic
	g, err := generate.ParseGrammar(grammarJSON, &diagnostics)
	if err != nil {
		return nil, err
	}
	return generate.ParserForGrammarWithOpts(g, abi, version, opts, c.Backend{}, &diagnostics)
}

// indent indents each line of an error as the upstream tool writes it under
// "Caused by:". An empty line gets the four spaces too.
//
// indent is a copy of indent in generate/prepare_grammar_test.go.
func indent(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

// sum returns the SHA-256 of a text, in hex.
func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// firstDiff returns the first line where got differs from want, with its
// number and the lines around it, and an empty string when they are equal.
func firstDiff(want, got string) string {
	if want == got {
		return ""
	}
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	i := 0
	for i < len(wantLines) && i < len(gotLines) && wantLines[i] == gotLines[i] {
		i++
	}
	var b bytes.Buffer
	b.WriteString("line " + strconv.Itoa(i+1) + ":\n")
	for j := max(i-3, 0); j < i+3; j++ {
		if j < len(wantLines) {
			b.WriteString("  want: " + wantLines[j] + "\n")
		}
		if j < len(gotLines) {
			b.WriteString("  got:  " + gotLines[j] + "\n")
		}
	}
	return b.String()
}
