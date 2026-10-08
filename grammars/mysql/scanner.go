package mysql

import "github.com/xo/transit/internal/abi"

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js.
const (
	comment = iota
	versionCommentStart
	versionCommentEnd
)

// scanner is a port of src/scanner.c of the grammar. It reads the comments:
// # and -- to the end of the line, and /* ... */. The -- starts a comment
// only before white space, a control character or the end of the input. It
// reads the start of a version comment, /*! or /*M! with the digits of a
// version after it, and its end, */.
type scanner struct {
	// version is 1 inside a version comment, from its start to its end.
	version uint8
}

// newScanner is tree_sitter_mysql_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// isSpace is is_space. It reports whether c is white space.
func isSpace(c int32) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isNewline is is_newline. It reports whether c ends a line.
func isNewline(c int32) bool { return c == '\n' || c == '\r' }

// scanLineComment is scan_line_comment. It reads a comment to the end of
// the line. The start of the comment is already read.
func scanLineComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() && !isNewline(lexer.Lookahead) {
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanBlockComment is scan_block_comment. It reads a block comment to its
// end, or to the end of the input. The /* of the comment is already read.
func scanBlockComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() {
		if lexer.Lookahead == '*' {
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				break
			}
		} else {
			advance(lexer)
		}
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanVersionStart is scan_version_start. It reads the digits of the
// version of a version comment. The /*! or /*M! of the comment is already
// read.
func (s *scanner) scanVersionStart(lexer *abi.Lexer) bool {
	for lexer.Lookahead >= '0' && lexer.Lookahead <= '9' {
		advance(lexer)
	}
	lexer.MarkEnd()
	s.version = 1
	lexer.ResultSymbol = versionCommentStart
	return true
}

// scanComment is scan_comment. It reads a block comment or the start of a
// version comment. The /* is already read.
func (s *scanner) scanComment(lexer *abi.Lexer, validSymbols []bool) bool {
	if lexer.Lookahead == '!' && validSymbols[versionCommentStart] {
		advance(lexer)
		return s.scanVersionStart(lexer)
	}
	if lexer.Lookahead == 'M' && validSymbols[versionCommentStart] {
		advance(lexer)
		if lexer.Lookahead == '!' {
			advance(lexer)
			return s.scanVersionStart(lexer)
		}
	}
	return scanBlockComment(lexer)
}

// Serialize is tree_sitter_mysql_external_scanner_serialize. The state is
// version, in one byte.
func (s *scanner) Serialize(buf []byte) int {
	buf[0] = s.version
	return 1
}

// Deserialize is tree_sitter_mysql_external_scanner_deserialize.
func (s *scanner) Deserialize(buf []byte) {
	s.version = 0
	if len(buf) > 0 && buf[0] != 0 {
		s.version = 1
	}
}

// Scan is tree_sitter_mysql_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	for isSpace(lexer.Lookahead) {
		skip(lexer)
	}
	c := lexer.Lookahead
	if c == '#' && validSymbols[comment] {
		advance(lexer)
		return scanLineComment(lexer)
	}
	if c == '-' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '-' {
			return false
		}
		advance(lexer)
		d := lexer.Lookahead
		if lexer.EOF() || d <= ' ' || d == 0x7f {
			return scanLineComment(lexer)
		}
		return false
	}
	if c == '/' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '*' {
			return false
		}
		advance(lexer)
		return s.scanComment(lexer, validSymbols)
	}
	if c == '*' && s.version != 0 && validSymbols[versionCommentEnd] {
		advance(lexer)
		if lexer.Lookahead != '/' {
			return false
		}
		advance(lexer)
		lexer.MarkEnd()
		s.version = 0
		lexer.ResultSymbol = versionCommentEnd
		return true
	}
	return false
}
