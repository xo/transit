package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/graphql"
)

func init() {
	goPackages = append(goPackages, goPackage{"graphql", graphql.Language})
}

// TestGraphqlExamplesMatchC compares the trees and the edits of the files of
// examples/ of tree-sitter-graphql. The corpus of the repository is in
// corpus/ and not in test/corpus, so upstream does not run it, and the tests
// of gopackage_test.go have no input for it.
func TestGraphqlExamplesMatchC(t *testing.T) {
	t.Parallel()
	compareExampleFiles(t, goPackage{"graphql", graphql.Language}, "examples/*.graphql")
}
