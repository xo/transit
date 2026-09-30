package javascript

import (
	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// The external tokens of the grammar, in the order of externals in
// grammar.json. They are enum TokenType.
const (
	automaticSemicolon = iota
	templateChars
	ternaryQmark
	htmlComment
	logicalOr
	escapeSequence
	regexPattern
	jsxText
)

// scanner is a port of src/scanner.c of tree-sitter-javascript at v0.25.0
// (44c892e0be055ac465d5eeddae6d3e194424e7de). It keeps no state, so it
// serializes no bytes.
//
// The C scanner calls iswspace, iswdigit and iswalpha of the C library. The
// Go scanner calls the functions of the same names in the package wctype
// (D39, D46).
type scanner struct{}

// newScanner is tree_sitter_javascript_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Serialize is tree_sitter_javascript_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_javascript_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// advance is advance.
func advance(lexer *abi.Lexer) {
	lexer.Advance(false)
}

// skip is skip.
func skip(lexer *abi.Lexer) {
	lexer.Advance(true)
}

// scanTemplateChars is scan_template_chars.
func scanTemplateChars(lexer *abi.Lexer) bool {
	lexer.ResultSymbol = templateChars
	for hasContent := false; ; hasContent = true {
		lexer.MarkEnd()
		switch lexer.Lookahead {
		case '`':
			return hasContent
		case 0:
			return false
		case '$':
			advance(lexer)
			if lexer.Lookahead == '{' {
				return hasContent
			}
		case '\\':
			return hasContent
		default:
			advance(lexer)
		}
	}
}

// whitespaceResult is WhitespaceResult.
type whitespaceResult int

// The results of scanWhitespaceAndComments.
const (
	reject    whitespaceResult = iota // Semicolon is illegal, ie a syntax error occurred
	noNewline                         // Unclear if semicolon will be legal, continue
	accept                            // Semicolon is legal, assuming a comment was encountered
)

// String returns the C name of the result.
func (r whitespaceResult) String() string {
	switch r {
	case reject:
		return "REJECT"
	case noNewline:
		return "NO_NEWLINE"
	case accept:
		return "ACCEPT"
	}
	return "WhitespaceResult(?)"
}

// scanWhitespaceAndComments is scan_whitespace_and_comments.
//
// If consume is false, only consume enough to check if comment indicates
// semicolon-legality.
func scanWhitespaceAndComments(lexer *abi.Lexer, scannedComment *bool, consume bool) whitespaceResult {
	sawBlockNewline := false

	for {
		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		if lexer.Lookahead == '/' {
			skip(lexer)

			switch lexer.Lookahead {
			case '/':
				skip(lexer)
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' && lexer.Lookahead != 0x2028 &&
					lexer.Lookahead != 0x2029 {
					skip(lexer)
				}
				*scannedComment = true
			case '*':
				skip(lexer)
			block:
				for lexer.Lookahead != 0 {
					switch lexer.Lookahead {
					case '*':
						skip(lexer)
						if lexer.Lookahead == '/' {
							skip(lexer)
							*scannedComment = true

							if lexer.Lookahead != '/' && !consume {
								if sawBlockNewline {
									return accept
								}
								return noNewline
							}

							break block
						}
					case '\n', 0x2028, 0x2029:
						sawBlockNewline = true
						skip(lexer)
					default:
						skip(lexer)
					}
				}
			default:
				return reject
			}
		} else {
			return accept
		}
	}
}

// scanAutomaticSemicolon is scan_automatic_semicolon.
func scanAutomaticSemicolon(lexer *abi.Lexer, commentCondition bool, scannedComment *bool) bool {
	lexer.ResultSymbol = automaticSemicolon
	lexer.MarkEnd()

	for {
		if lexer.Lookahead == 0 {
			return true
		}

		if lexer.Lookahead == '/' {
			result := scanWhitespaceAndComments(lexer, scannedComment, false)
			if result == reject {
				return false
			}

			if result == accept && commentCondition && lexer.Lookahead != ',' && lexer.Lookahead != '=' {
				return true
			}
		}

		if lexer.Lookahead == '}' {
			return true
		}

		if lexer.IsAtIncludedRangeStart() {
			return true
		}

		if lexer.Lookahead == '\n' || lexer.Lookahead == 0x2028 || lexer.Lookahead == 0x2029 {
			break
		}

		if !wctype.Iswspace(lexer.Lookahead) {
			return false
		}

		skip(lexer)
	}

	skip(lexer)

	if scanWhitespaceAndComments(lexer, scannedComment, true) == reject {
		return false
	}

	switch lexer.Lookahead {
	case '`', ',', ':', ';', '*', '%', '>', '<', '=', '[', '(', '?', '^', '|', '&', '/':
		return false

	// Insert a semicolon before decimals literals but not otherwise.
	case '.':
		skip(lexer)
		return wctype.Iswdigit(lexer.Lookahead)

	// Insert a semicolon before `--` and `++`, but not before binary `+` or `-`.
	case '+':
		skip(lexer)
		return lexer.Lookahead == '+'
	case '-':
		skip(lexer)
		return lexer.Lookahead == '-'

	// Don't insert a semicolon before `!=`, but do insert one before a unary `!`.
	case '!':
		skip(lexer)
		return lexer.Lookahead != '='

	// Don't insert a semicolon before `in` or `instanceof`, but do insert one
	// before an identifier.
	case 'i':
		skip(lexer)

		if lexer.Lookahead != 'n' {
			return true
		}
		skip(lexer)

		if !wctype.Iswalpha(lexer.Lookahead) {
			return false
		}

		for i := range 8 {
			if lexer.Lookahead != rune("stanceof"[i]) {
				return true
			}
			skip(lexer)
		}

		if !wctype.Iswalpha(lexer.Lookahead) {
			return false
		}
	}

	return true
}

// scanTernaryQmark is scan_ternary_qmark.
func scanTernaryQmark(lexer *abi.Lexer) bool {
	for wctype.Iswspace(lexer.Lookahead) {
		skip(lexer)
	}

	if lexer.Lookahead == '?' {
		advance(lexer)

		if lexer.Lookahead == '?' {
			return false
		}

		lexer.MarkEnd()
		lexer.ResultSymbol = ternaryQmark

		if lexer.Lookahead == '.' {
			advance(lexer)
			return wctype.Iswdigit(lexer.Lookahead)
		}
		return true
	}
	return false
}

// scanHTMLComment is scan_html_comment.
func scanHTMLComment(lexer *abi.Lexer) bool {
	for wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == 0x2028 || lexer.Lookahead == 0x2029 {
		skip(lexer)
	}

	const commentStart = "<!--"
	const commentEnd = "-->"

	switch lexer.Lookahead {
	case '<':
		for i := range 4 {
			if lexer.Lookahead != rune(commentStart[i]) {
				return false
			}
			advance(lexer)
		}
	case '-':
		for i := range 3 {
			if lexer.Lookahead != rune(commentEnd[i]) {
				return false
			}
			advance(lexer)
		}
	default:
		return false
	}

	for lexer.Lookahead != 0 && lexer.Lookahead != '\n' && lexer.Lookahead != 0x2028 &&
		lexer.Lookahead != 0x2029 {
		advance(lexer)
	}

	lexer.ResultSymbol = htmlComment
	lexer.MarkEnd()

	return true
}

// scanJSXText is scan_jsx_text.
func scanJSXText(lexer *abi.Lexer) bool {
	// saw_text will be true if we see any non-whitespace content, or any whitespace content that is not a newline and
	// does not immediately follow a newline.
	sawText := false
	// at_newline will be true if we are currently at a newline, or if we are at whitespace that is not a newline but
	// immediately follows a newline.
	atNewline := false

	for lexer.Lookahead != 0 && lexer.Lookahead != '<' && lexer.Lookahead != '>' && lexer.Lookahead != '{' &&
		lexer.Lookahead != '}' && lexer.Lookahead != '&' {
		isWspace := wctype.Iswspace(lexer.Lookahead)
		if lexer.Lookahead == '\n' {
			atNewline = true
		} else {
			// If at_newline is already true, and we see some whitespace, then it must stay true.
			// Otherwise, it should be false.
			//
			// See the table below to determine the logic for computing `saw_text`.
			//
			// |------------------------------------|
			// | at_newline | is_wspace | saw_text  |
			// |------------|-----------|-----------|
			// | false (0)  | false (0) | true  (1) |
			// | false (0)  | true  (1) | true  (1) |
			// | true  (1)  | false (0) | true  (1) |
			// | true  (1)  | true  (1) | false (0) |
			// |------------------------------------|

			atNewline = atNewline && isWspace
			if !atNewline {
				sawText = true
			}
		}

		advance(lexer)
	}

	lexer.ResultSymbol = jsxText
	return sawText
}

// Scan is tree_sitter_javascript_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[templateChars] {
		if validSymbols[automaticSemicolon] {
			return false
		}
		return scanTemplateChars(lexer)
	}

	if validSymbols[jsxText] && scanJSXText(lexer) {
		return true
	}

	if validSymbols[automaticSemicolon] {
		scannedComment := false
		ret := scanAutomaticSemicolon(lexer, !validSymbols[logicalOr], &scannedComment)
		if !ret && !scannedComment && validSymbols[ternaryQmark] && lexer.Lookahead == '?' {
			return scanTernaryQmark(lexer)
		}
		return ret
	}

	if validSymbols[ternaryQmark] {
		return scanTernaryQmark(lexer)
	}

	if validSymbols[htmlComment] && !validSymbols[logicalOr] && !validSymbols[escapeSequence] &&
		!validSymbols[regexPattern] {
		return scanHTMLComment(lexer)
	}

	return false
}
