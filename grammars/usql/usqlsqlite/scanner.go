package usqlsqlite

import (
	"github.com/xo/transit/grammars/usql/internal/scan"
	"github.com/xo/transit/internal/abi"
)

// family is USQL_FAMILY of src/scanner.c, the family of dialects of the
// grammar: block comments and backticks.
const family = scan.Block | scan.Backtick

// scanner is a port of usqlsqlite/src/scanner.c of the usql grammars of transit,
// in grammars/usql. The C file includes common/scanner.h, which the
// package scan ports, and gives it the family of the grammar.
type scanner struct {
	s *scan.Scanner
}

// newScanner returns the scanner.
//
// newScanner is tree_sitter_usql_sqlite_external_scanner_create.
func newScanner() *scanner { return &scanner{s: scan.New()} }

// Serialize writes the state of the scanner.
//
// Serialize is tree_sitter_usql_sqlite_external_scanner_serialize.
func (s *scanner) Serialize(buffer []byte) int { return s.s.Serialize(buffer) }

// Deserialize reads the state that Serialize writes.
//
// Deserialize is tree_sitter_usql_sqlite_external_scanner_deserialize.
func (s *scanner) Deserialize(buffer []byte) { s.s.Deserialize(buffer) }

// Scan scans one external token.
//
// Scan is tree_sitter_usql_sqlite_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return s.s.Scan(lexer, validSymbols, family)
}
