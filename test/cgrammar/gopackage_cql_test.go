package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/cql"
)

func init() {
	goPackages = append(goPackages, goPackage{"cql", cql.Language})
}

// TestCQLExamplesMatchC compares the trees and the edits of test/file.cql of
// tree-sitter-cql, which the corpus does not hold.
func TestCQLExamplesMatchC(t *testing.T) {
	t.Parallel()
	compareExampleFiles(t, goPackage{"cql", cql.Language}, "test/*.cql")
}
