package tsx

import (
	"github.com/xo/transit/grammars/typescript/internal/scan"
	"github.com/xo/transit/internal/abi"
)

// scanner is a port of tsx/src/scanner.c of tree-sitter-typescript at
// v0.23.2 (f975a621f4e7f532fe322e13c4f79495e0a7b2e7). The C file includes
// common/scanner.h, which the package scan ports. The scanner keeps no
// state. So create returns NULL, destroy does nothing, serialize writes no
// byte, and deserialize reads no byte.
type scanner struct{}

// newScanner returns the scanner.
//
// newScanner is tree_sitter_tsx_external_scanner_create.
func newScanner() *scanner { return &scanner{} }

// Serialize writes no byte, because the scanner keeps no state.
//
// Serialize is tree_sitter_tsx_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int { return 0 }

// Deserialize does nothing, because the scanner keeps no state.
//
// Deserialize is tree_sitter_tsx_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// Scan scans one external token.
//
// Scan is tree_sitter_tsx_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return scan.ExternalScannerScan(lexer, validSymbols)
}
