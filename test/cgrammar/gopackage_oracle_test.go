package cgrammar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/oracle"
)

func init() {
	goPackages = append(goPackages, goPackage{"plsql", oracle.Language})
}

// TestOracleExamplesMatchC compares the trees and the edits of the files of
// examples/ of tree-sitter-plsql. The repository has no test/corpus, so the
// tests of gopackage_test.go have no input for it.
func TestOracleExamplesMatchC(t *testing.T) {
	t.Parallel()
	compareExampleFiles(t, goPackage{"plsql", oracle.Language}, "examples/*.sql")
}

// compareExampleFiles parses each file of the checkout of the grammar of a
// grammar package whose path matches pattern, with the Go runtime and the Go
// package, and with the C runtime and the C grammar. It compares the trees
// node by node, and the trees after each edit of compareEdits.
func compareExampleFiles(t *testing.T, gp goPackage, pattern string) {
	t.Helper()
	g, _ := loadGoPackage(t, gp)
	root, cache := setup(t)
	dir, _ := grammarDirs(cache, goPackageEntry(t, root, gp.name))
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no file of %s matches %s", dir, pattern)
	}
	p := transit.NewParser()
	if err := p.SetLanguage(gp.language()); err != nil {
		t.Fatal(err)
	}
	goGrammar := *g
	goGrammar.Language = gp.language()
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		want, _, err := g.CParse(src)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := goParse(p, src)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
		if d := Diff(want, GoSnapshot(tree.RootNode(), "")); d != "" {
			t.Errorf("%s: the trees differ at %s", filepath.Base(file), d)
		}
		if msg := compareEdits(&goGrammar, src); msg != "" {
			t.Errorf("%s: %s", filepath.Base(file), msg)
		}
	}
	t.Logf("%d files", len(files))
}
