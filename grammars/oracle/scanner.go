package oracle

import "github.com/xo/transit/internal/abi"

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js.
const (
	comment = iota
	qString
	slash
)

// scanner is a port of src/scanner.c of the grammar. It reads three tokens.
// comment is -- to the end of the line, or /* ... */. qString is a string
// with the alternative quoting of Oracle, q'[...]' or nq'[...]', in any
// case. Its quote delimiter is any character but white space, and the
// brackets [, {, ( and < close with ], }, ) and >. slash is a / alone on
// its line, which ends a statement or a PL/SQL unit in a script. Only white
// space can come before it and after it on the line.
type scanner struct{}

// newScanner is tree_sitter_oracle_external_scanner_create.
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

// scanQString is scan_q_string. It reads a string with the alternative
// quoting to its end, or to the end of the input. The q' of the string is
// already read.
func scanQString(lexer *abi.Lexer) bool {
	open := lexer.Lookahead
	if lexer.EOF() || isSpace(open) {
		return false
	}
	closing := open
	switch open {
	case '[':
		closing = ']'
	case '{':
		closing = '}'
	case '(':
		closing = ')'
	case '<':
		closing = '>'
	}
	advance(lexer)
	for !lexer.EOF() {
		if lexer.Lookahead == closing {
			advance(lexer)
			if lexer.Lookahead == '\'' {
				advance(lexer)
				break
			}
		} else {
			advance(lexer)
		}
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = qString
	return true
}

// scanSlash is scan_slash. It reads a / alone on its line. The / is already
// read, and lineStart reports whether only white space comes before it on
// its line.
func scanSlash(lexer *abi.Lexer, lineStart bool) bool {
	if !lineStart {
		return false
	}
	lexer.MarkEnd()
	for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' || lexer.Lookahead == '\f' ||
		lexer.Lookahead == '\v' {
		advance(lexer)
	}
	if !lexer.EOF() && !isNewline(lexer.Lookahead) {
		return false
	}
	lexer.ResultSymbol = slash
	return true
}

// Serialize is tree_sitter_oracle_external_scanner_serialize. The scanner
// has no state.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_oracle_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// Scan is tree_sitter_oracle_external_scanner_scan.
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
	if c == '/' {
		// The / is the first character of its line when the white space
		// before it holds the end of a line, or when the scan starts at the
		// start of a line.
		lineStart := validSymbols[slash] && (newline || lexer.GetColumn() == skipped)
		advance(lexer)
		if lexer.Lookahead == '*' && validSymbols[comment] {
			advance(lexer)
			return scanBlockComment(lexer)
		}
		return scanSlash(lexer, lineStart)
	}
	if validSymbols[qString] {
		if c == 'n' || c == 'N' {
			advance(lexer)
			c = lexer.Lookahead
			if c != 'q' && c != 'Q' {
				return false
			}
		}
		if c == 'q' || c == 'Q' {
			advance(lexer)
			if lexer.Lookahead != '\'' {
				return false
			}
			advance(lexer)
			return scanQString(lexer)
		}
	}
	return false
}
