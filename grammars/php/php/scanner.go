package php

import "github.com/xo/transit/grammars/php/internal/scanner"

// This file ports php/src/scanner.c of tree-sitter-php at v0.24.2, on
// the branch upstream_test_fixture
// (e8074c9943298d30a1af9ff84c2b61a6dc600d86). The C file includes
// common/scanner.h, which the package internal/scanner ports, and each of
// its five functions calls the function of that file with the same name.

// newScanner is tree_sitter_php_external_scanner_create. The methods
// Serialize, Deserialize and Scan of scanner.Scanner are the other functions
// of the file. tree_sitter_php_external_scanner_destroy has no port,
// because it only frees memory.
func newScanner() *scanner.Scanner {
	return scanner.New()
}
