package generate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLanguageVersionsAreInSync is test_language_versions_are_in_sync in
// generate.rs. It reads api.h of the upstream checkout, which git ignores, so
// it skips when the checkout is absent, as in CI. The test
// test_parser_header_in_sync comes with the command, which writes parser.h
// into the folder of a grammar.
func TestLanguageVersionsAreInSync(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "tree-sitter", "lib", "include", "tree_sitter", "api.h"))
	if os.IsNotExist(err) {
		t.Skip("the upstream checkout is absent")
	}
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(b)) {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "#define TREE_SITTER_LANGUAGE_VERSION ")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if n != LanguageVersion {
			t.Errorf("expected LanguageVersion %d, as api.h says, got: %d", n, LanguageVersion)
		}
		return
	}
	t.Fatal("Failed to find TREE_SITTER_LANGUAGE_VERSION definition in api.h")
}

// TestParserHeaderInSync is test_parser_header_in_sync in generate.rs, for
// the three headers that the port copies into generate/templates. It reads
// the upstream checkout, which git ignores, so it skips when the checkout is
// absent, as in CI.
func TestParserHeaderInSync(t *testing.T) {
	t.Parallel()
	upstream := filepath.Join("..", "tree-sitter")
	if _, err := os.Stat(upstream); os.IsNotExist(err) {
		t.Skip("the upstream checkout is absent")
	}
	for _, h := range []struct {
		path string
		copy string
	}{
		{filepath.Join(upstream, "lib", "src", "parser.h"), ParserHeader},
		{filepath.Join(upstream, "crates", "generate", "src", "parser.h.inc"), ParserHeader},
		{filepath.Join(upstream, "crates", "generate", "src", "templates", "alloc.h"), AllocHeader},
		{filepath.Join(upstream, "crates", "generate", "src", "templates", "array.h"), ArrayHeader},
	} {
		b, err := os.ReadFile(h.path)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != h.copy {
			t.Errorf("generate/templates is out of sync with %s. Copy the file again", h.path)
		}
	}
}

// TestParserInDirectoryWritesTheFilesOfUpstream generates a test grammar in a
// folder, and compares the files with the golden files and the headers.
func TestParserInDirectoryWritesTheFilesOfUpstream(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "aliased_rules")
	b, err := os.ReadFile(filepath.Join(golden, "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "grammar.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ParserInDirectory(dir, "", "", 14, true, OptLevelMergeStates, fakeBackend{}, new([]Diagnostic)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, f := range []struct {
		path     string
		expected string
	}{
		{filepath.Join("src", "parser.c"), "fake parser"},
		{filepath.Join("src", "tree_sitter", "parser.h"), ParserHeader},
		{filepath.Join("src", "tree_sitter", "alloc.h"), AllocHeader},
		{filepath.Join("src", "tree_sitter", "array.h"), ArrayHeader},
	} {
		b, err := os.ReadFile(filepath.Join(dir, f.path))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != f.expected {
			t.Errorf("%s: expected the text of the backend or of the header", f.path)
		}
	}
	nodeTypes, err := os.ReadFile(filepath.Join(dir, "src", "node-types.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join(golden, "abi14", "node-types.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(nodeTypes) != string(expected) {
		t.Error("expected the golden node-types.json")
	}

	// a grammar.js is an error, because the port runs no JavaScript (D17)
	if err := ParserInDirectory(dir, "", filepath.Join(dir, "grammar.js"), 15, true, OptLevelMergeStates, fakeBackend{}, new([]Diagnostic)); err == nil {
		t.Error("expected an error for grammar.js")
	}
}

// TestReadGrammarVersion makes sure that the version comes from the nearest
// tree-sitter.json, in the folder or a folder above it, and that no file
// gives no version.
func TestReadGrammarVersion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inner := filepath.Join(root, "grammars", "tsx")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if v, err := ReadGrammarVersion(inner); err != nil || v != nil {
		t.Errorf("expected no version, got: %v, %v", v, err)
	}
	if err := os.WriteFile(filepath.Join(root, "tree-sitter.json"), []byte(`{"metadata": {"version": "0.23.300-rc.1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := ReadGrammarVersion(inner)
	if err != nil || v == nil || *v != (SemanticVersion{Major: 0, Minor: 23, Patch: 300 % 256}) {
		t.Errorf("expected 0.23.44, the patch cut to 8 bits, got: %v, %v", v, err)
	}
	if err := os.WriteFile(filepath.Join(root, "tree-sitter.json"), []byte(`{"metadata": {"version": "1.02.3"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadGrammarVersion(inner); err == nil {
		t.Error("expected an error for a version with a leading zero")
	}
}

// fakeBackend writes a fixed text, so that a test of the files of a folder
// does not need a real backend.
type fakeBackend struct{}

// Render returns a fixed text.
func (fakeBackend) Render(*RenderInput) (string, error) {
	return "fake parser", nil
}
