package cgrammar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/transit/grammars/json"
	"github.com/xo/transit/internal/grammartest"
)

func init() {
	goPackages = append(goPackages, goPackage{"json", json.Language})
}

// TestGoPackageHighlight runs the highlight test of internal/grammartest on
// a file of JSON with assertions in its comments, because the corpus of
// tree-sitter-json has no test/highlight. The key is a string, because the
// last of the two patterns that capture it counts, as tree-sitter test of
// the upstream tool says (D80).
func TestGoPackageHighlight(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "{\"key\": 12}\n//  ^ string\n//  ^ !string.special.key\n//      ^ number\n//   ^ !number\n"
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	grammartest.Highlight(t, json.Language(), json.Queries, dir)
}
