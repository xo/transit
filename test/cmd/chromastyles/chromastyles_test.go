package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCommittedStyles runs the converter on the styles of chroma with the
// committed list of capture names, and makes sure that it writes the files
// of the module styles, byte for byte. It makes sure too that styles/chroma
// holds no other file. It skips when the Go module cache does not hold
// chroma at chromaVersion.
func TestCommittedStyles(t *testing.T) {
	src, err := findSource(context.Background(), chromaVersion)
	if err != nil {
		t.Skipf("chroma is not in the Go module cache: %v", err)
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "styles")
	b, err := os.ReadFile(filepath.Join(dir, "captures.txt"))
	if err != nil {
		t.Fatal(err)
	}
	captures := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	files, err := convert(src, chromaVersion, captures)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range sortedKeys(files) {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("expected the committed file styles/%s, got: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, files[rel]) {
			t.Errorf("styles/%s: expected the output of the converter, got other bytes. Run: cd test && go run ./cmd/chromastyles", rel)
		}
	}
	committed, err := os.ReadDir(filepath.Join(dir, "chroma"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range committed {
		if files["chroma/"+e.Name()] == nil {
			t.Errorf("styles/chroma/%s: expected only the files of the converter", e.Name())
		}
	}
	for name := range fromPygments {
		if files["chroma/"+name+".json"] == nil {
			t.Errorf("fromPygments names %s, which is not a style of chroma", name)
		}
	}
}

// TestCommittedCaptures measures the capture names of the highlight queries
// of the grammars, and makes sure that styles/captures.txt holds them. It
// skips when the cache of the golden harness does not exist.
func TestCommittedCaptures(t *testing.T) {
	cache, err := grammarCache()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Skipf("the cache of the golden harness does not exist: %v", err)
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	captures, err := measure(context.Background(), root, cache)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "styles", "captures.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Join(captures, "\n") + "\n"; string(got) != want {
		t.Errorf("styles/captures.txt: expected the capture names of the highlight queries, got other names. Run: cd test && go run ./cmd/chromastyles")
	}
}

// TestScanCaptures makes sure that the scan finds each capture name, and
// skips the strings and the comments of a query.
func TestScanCaptures(t *testing.T) {
	src := `; a comment with @comment.in.a.comment
(identifier) @variable
((identifier) @_name @constant.builtin
  (#match? @_name "^@[A-Z]+;$"))
"@" @punctuation.special
(string "\"@escaped\"") @string
[(a) (b)]@keyword.function-x ; @after
(x) @@
`
	got := scanCaptures(src)
	want := []string{"variable", "_name", "constant.builtin", "_name", "punctuation.special", "string", "keyword.function-x"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %q, got: %q", want, got)
	}
}

// TestCaptureTypes makes sure that each type of captureTypes is a token type
// of chroma, and that captureType takes the longest prefix.
func TestCaptureTypes(t *testing.T) {
	lines := 0
	for line := range strings.Lines(captureTable) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines++
		if len(strings.Fields(line)) != 2 {
			t.Errorf("captureTable: expected a name and a type, got: %q", line)
		}
	}
	if lines != len(captureTypes) {
		t.Errorf("captureTable: expected each name once, got %d lines for %d names", lines, len(captureTypes))
	}
	for name, typ := range captureTypes {
		if _, ok := tokenTypes[typ]; !ok {
			t.Errorf("%s: %s is not a token type of chroma", name, typ)
		}
	}
	for _, c := range []struct {
		capture string
		want    string
	}{
		{"keyword", "Keyword"},
		{"keyword.import", "KeywordNamespace"},
		{"keyword.import.c", "KeywordNamespace"},
		{"keyword.return", "Keyword"},
		{"string.special.symbol.ruby", "LiteralStringSymbol"},
	} {
		got, ok := captureType(c.capture)
		if !ok || got != tokenTypes[c.want] {
			t.Errorf("%s: expected %s, got: %d, %v", c.capture, c.want, got, ok)
		}
	}
	if _, ok := captureType("nosuchcapture"); ok {
		t.Errorf("expected no type for nosuchcapture")
	}
}

// TestResolve makes sure that an entry resolves with the rules of chroma:
// the inheritance from the subcategory, the category, Text and Background,
// noinherit, and the background, which withoutBackground takes out.
func TestResolve(t *testing.T) {
	s, err := parseStyle(strings.NewReader(`<style name="test">
  <entry type="Background" style="bg:#000000"/>
  <entry type="Text" style="#ffffff"/>
  <entry type="Keyword" style="bold #ff0000"/>
  <entry type="KeywordType" style="italic"/>
  <entry type="NameVariable" style="noinherit #00ff00"/>
  <entry type="Name" style="underline bg:#111111"/>
  <entry type="Error" style="#ff0000 bg:#330000"/>
</style>`))
	if err != nil {
		t.Fatal(err)
	}
	cleared := withoutBackground(s)
	for _, c := range []struct {
		typ, full, clear string
	}{
		{"Background", "#ffffff bg:#000000", "#ffffff"},
		{"Text", "#ffffff bg:#000000", "#ffffff"},
		{"Keyword", "bold #ff0000 bg:#000000", "bold #ff0000"},
		{"KeywordType", "bold italic #ff0000 bg:#000000", "bold italic #ff0000"},
		{"NameVariable", "#00ff00", "#00ff00"},
		{"NameVariableGlobal", "underline #00ff00 bg:#111111", "underline #00ff00 bg:#111111"},
		{"NameFunction", "underline #ffffff bg:#111111", "underline #ffffff bg:#111111"},
		{"Error", "#ff0000 bg:#330000", "#ff0000 bg:#330000"},
		{"Comment", "#ffffff bg:#000000", "#ffffff"},
	} {
		typ := tokenTypes[c.typ]
		if got := toEntry(s.resolve(typ)).String(); got != c.full {
			t.Errorf("%s: expected %q, got: %q", c.typ, c.full, got)
		}
		if got := toEntry(cleared.resolve(typ)).String(); got != c.clear {
			t.Errorf("%s without the background: expected %q, got: %q", c.typ, c.clear, got)
		}
	}
}
