package postgres

import "github.com/xo/transit/internal/abi"

// The external tokens of the grammar, in the order of externals in
// grammar.json. They are enum TokenType.
const (
	dollarQuotedString = iota
)

// scanner is a port of postgres/src/scanner.c of tree-sitter-postgres at
// v1.2.4 (9b27ba5c8700f9bf808221a0f6d17fe6515da787). It provides the token
// dollar_quoted_string. A dollar-quoted string of PostgreSQL has the form
// $tag$...$tag$, where the opening tag and the closing tag must be the same.
// The scanner remembers the opening tag, and scans until the same tag comes
// again, as the lexer of PostgreSQL does. It keeps no state, so it
// serializes no bytes.
type scanner struct{}

// newScanner is tree_sitter_postgres_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Serialize is tree_sitter_postgres_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_postgres_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// The C scanner has its own functions for ASCII, so that a Wasm build of the
// grammar imports no function of the C library.
//
// A tag of a dollar quote follows the rules of an identifier of PostgreSQL.
// The first character is a letter, which is an ASCII letter or a character
// whose first byte in UTF-8 is 0x80 or more, or an underscore. The other
// characters can also be digits. A tag holds no dollar sign.

// isTagStartChar is is_tag_start_char.
func isTagStartChar(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		c == '_' || c >= 0x80
}

// isTagChar is is_tag_char.
func isTagChar(c int32) bool {
	return isTagStartChar(c) || (c >= '0' && c <= '9')
}

// skipWhitespace is skip_whitespace.
func skipWhitespace(lexer *abi.Lexer) {
	for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' ||
		lexer.Lookahead == '\n' || lexer.Lookahead == '\r' {
		lexer.Advance(true)
	}
}

// Scan is tree_sitter_postgres_external_scanner_scan.
//
// The C function keeps each character of the tag as a char, and it compares
// the char as an unsigned char with the lookahead. So the tag keeps the low
// byte of each character, and a character above 0xff never matches its
// byte.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if !validSymbols[dollarQuotedString] {
		return false
	}

	skipWhitespace(lexer)

	if lexer.Lookahead != '$' {
		return false
	}

	lexer.Advance(false)

	// PostgreSQL limits the length of an identifier to NAMEDATALEN-1 = 63.
	// An empty tag ($$...$$) is valid. A tag that is not empty starts with
	// a letter or an underscore, and not with a digit.
	var tag [64]byte
	tagLen := 0
	if isTagStartChar(lexer.Lookahead) {
		for {
			if tagLen >= 63 {
				return false
			}
			tag[tagLen] = byte(lexer.Lookahead)
			tagLen++
			lexer.Advance(false)
			if !isTagChar(lexer.Lookahead) {
				break
			}
		}
	}

	if lexer.Lookahead != '$' {
		return false
	}
	lexer.Advance(false)

	for lexer.Lookahead != 0 {
		if lexer.Lookahead == '$' {
			lexer.Advance(false)
			i := 0
			for i < tagLen && lexer.Lookahead == int32(tag[i]) {
				lexer.Advance(false)
				i++
			}
			if i == tagLen && lexer.Lookahead == '$' {
				lexer.Advance(false)
				lexer.ResultSymbol = dollarQuotedString
				return true
			}
			// A part of the tag matches. The scan of the body goes on.
			continue
		}
		lexer.Advance(false)
	}

	return false
}
