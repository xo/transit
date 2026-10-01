package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/sqlserver"
)

func init() {
	goPackages = append(goPackages, goPackage{"TSQL", sqlserver.Language})
}

// TestSQLServerExamplesMatchC compares the trees and the edits of the files
// of test/examples of tree-sitter-tsql, which the corpus does not hold.
func TestSQLServerExamplesMatchC(t *testing.T) {
	t.Parallel()
	compareExampleFiles(t, goPackage{"TSQL", sqlserver.Language}, "test/examples/*.sql")
}
