package jsdoc

import "github.com/xo/transit/internal/abi"

// The external tokens of the grammar, in the order of externals in
// grammar.json. They are enum TokenType.
const (
	typeToken = iota
)

// scanner is a port of src/scanner.c of tree-sitter-jsdoc at v0.23.2
// (b253abf68a73217b7a52c0ec254f4b6a7bb86665). It keeps no state, so it
// serializes no bytes.
type scanner struct{}

// newScanner is tree_sitter_jsdoc_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Serialize is tree_sitter_jsdoc_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_jsdoc_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// scanForType is scan_for_type.
//
// Scan to the next balanced `}` character.
func scanForType(lexer *abi.Lexer) bool {
	stack := int32(0)
	for {
		if lexer.EOF() {
			return false
		}
		switch lexer.Lookahead {
		case '{':
			stack++
		case '}':
			stack--
			if stack == -1 {
				return true
			}
		case '\n', 0:
			// Something's gone wrong.
			return false
		}
		lexer.Advance(false)
	}
}

// Scan is tree_sitter_jsdoc_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[typeToken] && scanForType(lexer) {
		lexer.ResultSymbol = typeToken
		lexer.MarkEnd()
		return true
	}

	return false
}
