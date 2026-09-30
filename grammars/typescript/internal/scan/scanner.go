// Package scan is a port of common/scanner.h of tree-sitter-typescript at
// v0.23.2 (f975a621f4e7f532fe322e13c4f79495e0a7b2e7). The scanners of the
// grammars typescript and tsx include this header, and each of them calls
// ExternalScannerScan.
//
// The scanner keeps no state. The C scanners create no payload, so
// ExternalScannerScan takes no payload.
//
// The C code calls iswspace, iswalpha and iswdigit of <wctype.h>. The Go code
// calls Iswspace, Iswalpha and Iswdigit of the package wctype in their place
// (D39, D46). They answer as the package unicode does, and the C functions
// run in the C locale, where they answer as for ASCII. So the two can differ
// for a character that is not ASCII.
package scan

import (
	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// The external tokens of the two grammars, in the order of externals in
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
	functionSignatureAutomaticSemicolon
	errorRecovery
)

// advance moves the lexer to the next character, which is part of the
// token.
//
// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip moves the lexer to the next character, which is not part of the
// token.
//
// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// scanTemplateChars scans the characters of a template string up to a
// backquote, a substitution or an escape.
//
// scanTemplateChars is scan_template_chars.
func scanTemplateChars(lexer *abi.Lexer) bool {
	lexer.ResultSymbol = templateChars
	for hasContent := false; ; hasContent = true {
		lexer.MarkEnd()
		switch lexer.Lookahead {
		case '`':
			return hasContent
		case '\x00':
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

// scanWhitespaceAndComments skips whitespace and comments. It returns false
// when it finds a slash that does not start a comment. It sets
// scannedComment when it skips a line comment.
//
// scanWhitespaceAndComments is scan_whitespace_and_comments.
func scanWhitespaceAndComments(lexer *abi.Lexer, scannedComment *bool) bool {
	for {
		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		if lexer.Lookahead == '/' {
			skip(lexer)

			switch lexer.Lookahead {
			case '/':
				skip(lexer)
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					skip(lexer)
				}
				*scannedComment = true
			case '*':
				skip(lexer)
				for lexer.Lookahead != 0 {
					if lexer.Lookahead == '*' {
						skip(lexer)
						if lexer.Lookahead == '/' {
							skip(lexer)
							break
						}
					} else {
						skip(lexer)
					}
				}
			default:
				return false
			}
		} else {
			return true
		}
	}
}

// scanAutomaticSemicolon decides whether the parser inserts a semicolon at
// the end of a line.
//
// scanAutomaticSemicolon is scan_automatic_semicolon.
func scanAutomaticSemicolon(lexer *abi.Lexer, validSymbols []bool, scannedComment *bool) bool {
	lexer.ResultSymbol = automaticSemicolon
	lexer.MarkEnd()

	for {
		if lexer.Lookahead == 0 {
			return true
		}
		if lexer.Lookahead == '}' {
			// Automatic semicolon insertion breaks detection of object patterns
			// in a typed context:
			//   type F = ({a}: {a: number}) => number;
			// Therefore, disable automatic semicolons when followed by typing
			for {
				skip(lexer)
				if !wctype.Iswspace(lexer.Lookahead) {
					break
				}
			}
			if lexer.Lookahead == ':' {
				return validSymbols[logicalOr] // Don't return false if we're in a ternary by checking if || is valid
			}
			return true
		}
		if !wctype.Iswspace(lexer.Lookahead) {
			return false
		}
		if lexer.Lookahead == '\n' {
			break
		}
		skip(lexer)
	}

	skip(lexer)

	if !scanWhitespaceAndComments(lexer, scannedComment) {
		return false
	}

	switch lexer.Lookahead {
	case '`', ',', '.', ';', '*', '%', '>', '<', '=', '?', '^', '|', '&', '/', ':':
		return false

	case '{':
		if validSymbols[functionSignatureAutomaticSemicolon] {
			return false
		}

	// Don't insert a semicolon before a '[' or '(', unless we're parsing
	// a type. Detect whether we're parsing a type or an expression using
	// the validity of a binary operator token.
	case '(', '[':
		if validSymbols[logicalOr] {
			return false
		}

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

// scanTernaryQmark scans the question mark of a ternary expression, which is
// not the start of an optional chain or of an optional parameter.
//
// scanTernaryQmark is scan_ternary_qmark.
func scanTernaryQmark(lexer *abi.Lexer) bool {
	for wctype.Iswspace(lexer.Lookahead) {
		skip(lexer)
	}

	if lexer.Lookahead == '?' {
		advance(lexer)

		/* Optional chaining. */
		if lexer.Lookahead == '?' || lexer.Lookahead == '.' {
			return false
		}

		lexer.MarkEnd()
		lexer.ResultSymbol = ternaryQmark

		/* TypeScript optional arguments contain the ?: sequence, possibly
		   with whitespace. */
		for wctype.Iswspace(lexer.Lookahead) {
			advance(lexer)
		}

		if lexer.Lookahead == ':' || lexer.Lookahead == ')' || lexer.Lookahead == ',' {
			return false
		}

		if lexer.Lookahead == '.' {
			advance(lexer)
			return wctype.Iswdigit(lexer.Lookahead)
		}
		return true
	}
	return false
}

// scanClosingComment scans an HTML comment, which starts with <!-- or -->
// and ends at the end of the line.
//
// scanClosingComment is scan_closing_comment.
func scanClosingComment(lexer *abi.Lexer) bool {
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

// scanJSXText scans the text of a JSX element, up to a character that starts
// or ends a tag, an expression or an entity.
//
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

// ExternalScannerScan scans one external token of the grammars typescript and
// tsx. validSymbols holds one entry for each external token, in the order of
// externals in grammar.json.
//
// ExternalScannerScan is external_scanner_scan.
func ExternalScannerScan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[templateChars] {
		if validSymbols[automaticSemicolon] {
			return false
		}
		return scanTemplateChars(lexer)
	}

	if validSymbols[jsxText] && scanJSXText(lexer) {
		return true
	}

	if validSymbols[automaticSemicolon] || validSymbols[functionSignatureAutomaticSemicolon] {
		scannedComment := false
		ret := scanAutomaticSemicolon(lexer, validSymbols, &scannedComment)
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
		return scanClosingComment(lexer)
	}

	return false
}
