package sqlserver

import "github.com/xo/transit/internal/abi"

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js. goLine is GO of C, because go is a keyword of Go.
const (
	comment = iota
	goLine
)

// scanner is a port of src/scanner.c of the grammar. It reads two tokens.
// comment is -- to the end of the line, or /* ... */, where a block comment
// can hold another block comment, as in /* a /* b */ c */. goLine is the
// word GO, in any case, alone on its line with an optional count, as in GO
// 5. It ends a batch in sqlcmd and in SQL Server Management Studio. Only
// white space can come before it on the line, and only white space, the
// count and a comment after it. The token holds the word only, and the
// grammar reads the count.
type scanner struct{}

// newScanner is tree_sitter_sqlserver_external_scanner_create.
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

// isBlank is is_blank. It reports whether c is white space that does not
// end a line.
func isBlank(c int32) bool { return c == ' ' || c == '\t' || c == '\f' || c == '\v' }

// isNewline is is_newline. It reports whether c ends a line.
func isNewline(c int32) bool { return c == '\n' || c == '\r' }

// scanLineComment is scan_line_comment. It reads a comment to the end of
// the line. The -- of the comment is already read.
func scanLineComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() && !isNewline(lexer.Lookahead) {
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanBlockComment is scan_block_comment. It reads a block comment to its
// end, or to the end of the input. The /* of the comment is already read. A
// /* in the comment opens another comment, which its own */ closes.
func scanBlockComment(lexer *abi.Lexer) bool {
	depth := uint32(1)
	for !lexer.EOF() {
		switch lexer.Lookahead {
		case '*':
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				depth--
			}
		case '/':
			advance(lexer)
			if lexer.Lookahead == '*' {
				advance(lexer)
				depth++
			}
		default:
			advance(lexer)
		}
		if depth == 0 {
			break
		}
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanGo is scan_go. It reads the rest of a line with GO. The G is the
// lookahead, and only white space comes before it on its line.
func scanGo(lexer *abi.Lexer) bool {
	advance(lexer)
	if lexer.Lookahead != 'o' && lexer.Lookahead != 'O' {
		return false
	}
	advance(lexer)
	lexer.MarkEnd()
	for isBlank(lexer.Lookahead) {
		advance(lexer)
	}
	for lexer.Lookahead >= '0' && lexer.Lookahead <= '9' {
		advance(lexer)
	}
	for isBlank(lexer.Lookahead) {
		advance(lexer)
	}
	if lexer.Lookahead == '-' {
		advance(lexer)
		if lexer.Lookahead != '-' {
			return false
		}
	} else if !lexer.EOF() && !isNewline(lexer.Lookahead) {
		return false
	}
	lexer.ResultSymbol = goLine
	return true
}

// Serialize is tree_sitter_sqlserver_external_scanner_serialize. The
// scanner has no state.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_sqlserver_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// Scan is tree_sitter_sqlserver_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	// newline reports whether the white space before the token holds the
	// end of a line, and skipped counts its characters.
	newline := false
	var skipped uint32
	for isSpace(lexer.Lookahead) {
		if isNewline(lexer.Lookahead) {
			newline = true
		}
		skip(lexer)
		skipped++
	}
	c := lexer.Lookahead
	if c == '-' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '-' {
			return false
		}
		advance(lexer)
		return scanLineComment(lexer)
	}
	if c == '/' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '*' {
			return false
		}
		advance(lexer)
		return scanBlockComment(lexer)
	}
	if (c == 'g' || c == 'G') && validSymbols[goLine] {
		// GO is the first word of its line when the white space before it
		// holds the end of a line, or when the scan starts at the start of a
		// line.
		if !newline && lexer.GetColumn() != skipped {
			return false
		}
		return scanGo(lexer)
	}
	return false
}
