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
