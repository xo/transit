package rust

import (
	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// The external tokens of the scanner, in the order of externals in
// grammar.json. They are the C enum TokenType.
const (
	stringContent = iota
	rawStringLiteralStart
	rawStringLiteralContent
	rawStringLiteralEnd
	floatLiteral
	blockOuterDocMarker
	blockInnerDocMarker
	blockCommentContent
	lineDocContent
	errorSentinel
)

// scanner is Scanner, the external scanner of the grammar rust. It is a port
// of src/scanner.c of tree-sitter-rust at v0.24.0
// (18b0515fca567f5a10aee9978c6d2640e878671a).
//
// The C scanner calls iswdigit, iswalpha and iswspace, and the Go scanner
// calls the functions of the same name in internal/wctype. They answer as
// the package unicode does, so they can answer differently from C for a
// character that is not ASCII (D39, D46).
type scanner struct {
	openingHashCount uint8
}

// newScanner is tree_sitter_rust_external_scanner_create.
func newScanner() *scanner { return &scanner{} }

// Serialize is tree_sitter_rust_external_scanner_serialize. It writes
// opening_hash_count.
func (s *scanner) Serialize(buffer []byte) int {
	buffer[0] = s.openingHashCount
	return 1
}

// Deserialize is tree_sitter_rust_external_scanner_deserialize.
func (s *scanner) Deserialize(buffer []byte) {
	s.openingHashCount = 0
	if len(buffer) == 1 {
		s.openingHashCount = buffer[0]
	}
}

// isNumChar is is_num_char.
func isNumChar(c int32) bool { return c == '_' || wctype.Iswdigit(c) }

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// processString is process_string.
func processString(lexer *abi.Lexer) bool {
	hasContent := false
	for !(lexer.Lookahead == '"' || lexer.Lookahead == '\\') {
		if lexer.EOF() {
			return false
		}
		hasContent = true
		advance(lexer)
	}
	lexer.ResultSymbol = stringContent
	lexer.MarkEnd()
	return hasContent
}

// scanRawStringStart is scan_raw_string_start.
func (s *scanner) scanRawStringStart(lexer *abi.Lexer) bool {
	if lexer.Lookahead == 'b' || lexer.Lookahead == 'c' {
		advance(lexer)
	}
	if lexer.Lookahead != 'r' {
		return false
	}
	advance(lexer)

	var openingHashCount uint8
	for lexer.Lookahead == '#' {
		advance(lexer)
		openingHashCount++
	}

	if lexer.Lookahead != '"' {
		return false
	}
	advance(lexer)
	s.openingHashCount = openingHashCount

	lexer.ResultSymbol = rawStringLiteralStart
	return true
}

// scanRawStringContent is scan_raw_string_content.
func (s *scanner) scanRawStringContent(lexer *abi.Lexer) bool {
	for {
		if lexer.EOF() {
			return false
		}
		if lexer.Lookahead == '"' {
			lexer.MarkEnd()
			advance(lexer)
			var hashCount uint32
			for lexer.Lookahead == '#' && hashCount < uint32(s.openingHashCount) {
				advance(lexer)
				hashCount++
			}
			if hashCount == uint32(s.openingHashCount) {
				lexer.ResultSymbol = rawStringLiteralContent
				return true
			}
		} else {
			advance(lexer)
		}
	}
}

// scanRawStringEnd is scan_raw_string_end.
func (s *scanner) scanRawStringEnd(lexer *abi.Lexer) bool {
	advance(lexer)
	for range s.openingHashCount {
		advance(lexer)
	}
	lexer.ResultSymbol = rawStringLiteralEnd
	return true
}

// processFloatLiteral is process_float_literal.
func processFloatLiteral(lexer *abi.Lexer) bool {
	lexer.ResultSymbol = floatLiteral

	advance(lexer)
	for isNumChar(lexer.Lookahead) {
		advance(lexer)
	}

	hasFraction, hasExponent := false, false

	if lexer.Lookahead == '.' {
		hasFraction = true
		advance(lexer)
		if wctype.Iswalpha(lexer.Lookahead) {
			// The dot is followed by a letter: 1.max(2) => not a float but an integer
			return false
		}

		if lexer.Lookahead == '.' {
			return false
		}
		for isNumChar(lexer.Lookahead) {
			advance(lexer)
		}
	}

	lexer.MarkEnd()

	if lexer.Lookahead == 'e' || lexer.Lookahead == 'E' {
		hasExponent = true
		advance(lexer)
		if lexer.Lookahead == '+' || lexer.Lookahead == '-' {
			advance(lexer)
		}
		if !isNumChar(lexer.Lookahead) {
			return true
		}
		advance(lexer)
		for isNumChar(lexer.Lookahead) {
			advance(lexer)
		}

		lexer.MarkEnd()
	}

	if !hasExponent && !hasFraction {
		return false
	}

	if lexer.Lookahead != 'u' && lexer.Lookahead != 'i' && lexer.Lookahead != 'f' {
		return true
	}
	advance(lexer)
	if !wctype.Iswdigit(lexer.Lookahead) {
		return true
	}

	for wctype.Iswdigit(lexer.Lookahead) {
		advance(lexer)
	}

	lexer.MarkEnd()
	return true
}

// processLineDocContent is process_line_doc_content.
func processLineDocContent(lexer *abi.Lexer) bool {
	lexer.ResultSymbol = lineDocContent
	for {
		if lexer.EOF() {
			return true
		}
		if lexer.Lookahead == '\n' {
			// Include the newline in the doc content node.
			// Line endings are useful for markdown injection.
			advance(lexer)
			return true
		}
		advance(lexer)
	}
}

// blockCommentState is BlockCommentState.
type blockCommentState int

// The states of the scan of a block comment.
const (
	leftForwardSlash blockCommentState = iota
	leftAsterisk
	continuing
)

// blockCommentProcessing is BlockCommentProcessing.
type blockCommentProcessing struct {
	state        blockCommentState
	nestingDepth uint32
}

// processLeftForwardSlash is process_left_forward_slash.
func processLeftForwardSlash(processing *blockCommentProcessing, current byte) {
	if current == '*' {
		processing.nestingDepth++
	}
	processing.state = continuing
}

// processLeftAsterisk is process_left_asterisk.
func processLeftAsterisk(processing *blockCommentProcessing, current byte, lexer *abi.Lexer) {
	if current == '*' {
		lexer.MarkEnd()
		processing.state = leftAsterisk
		return
	}

	if current == '/' {
		processing.nestingDepth--
	}

	processing.state = continuing
}

// processContinuing is process_continuing.
func processContinuing(processing *blockCommentProcessing, current byte) {
	switch current {
	case '/':
		processing.state = leftForwardSlash
	case '*':
		processing.state = leftAsterisk
	}
}

// processBlockComment is process_block_comment. The C function keeps the
// lookahead in a char, so it keeps only the low byte of a character, and the
// Go function does the same.
func processBlockComment(lexer *abi.Lexer, validSymbols []bool) bool {
	first := byte(lexer.Lookahead)
	// The first character is stored so we can safely advance inside
	// these if blocks. However, because we only store one, we can only
	// safely advance 1 time. Since there's a chance that an advance could
	// happen in one state, we must advance in all states to ensure that
	// the program ends up in a sane state prior to processing the block
	// comment if need be.
	if validSymbols[blockInnerDocMarker] && first == '!' {
		lexer.ResultSymbol = blockInnerDocMarker
		advance(lexer)
		return true
	}
	if validSymbols[blockOuterDocMarker] && first == '*' {
		advance(lexer)
		lexer.MarkEnd()
		// If the next token is a / that means that it's an empty block comment.
		if lexer.Lookahead == '/' {
			return false
		}
		// If the next token is a * that means that this isn't a BLOCK_OUTER_DOC_MARKER
		// as BLOCK_OUTER_DOC_MARKER's only have 2 * not 3 or more.
		if lexer.Lookahead != '*' {
			lexer.ResultSymbol = blockOuterDocMarker
			return true
		}
	} else {
		advance(lexer)
	}

	if validSymbols[blockCommentContent] {
		processing := blockCommentProcessing{continuing, 1}
		// Manually set the current state based on the first character
		switch first {
		case '*':
			processing.state = leftAsterisk
			if lexer.Lookahead == '/' {
				// This case can happen in an empty doc block comment
				// like /*!*/. The comment has no contents, so bail.
				return false
			}
		case '/':
			processing.state = leftForwardSlash
		default:
			processing.state = continuing
		}

		// For the purposes of actually parsing rust code, this
		// is incorrect as it considers an unterminated block comment
		// to be an error. However, for the purposes of syntax highlighting
		// this should be considered successful as otherwise you are not able
		// to syntax highlight a block of code prior to closing the
		// block comment
		for !lexer.EOF() && processing.nestingDepth != 0 {
			// Set first to the current lookahead as that is the second character
			// as we force an advance in the above code when we are checking if we
			// need to handle a block comment inner or outer doc comment signifier
			// node
			first = byte(lexer.Lookahead)
			switch processing.state {
			case leftForwardSlash:
				processLeftForwardSlash(&processing, first)
			case leftAsterisk:
				processLeftAsterisk(&processing, first, lexer)
			case continuing:
				lexer.MarkEnd()
				processContinuing(&processing, first)
			default:
			}
			advance(lexer)
			if first == '/' && processing.nestingDepth != 0 {
				lexer.MarkEnd()
			}
		}
		lexer.ResultSymbol = blockCommentContent
		return true
	}

	return false
}

// Scan is tree_sitter_rust_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	// The documentation states that if the lexical analysis fails for some reason
	// they will mark every state as valid and pass it to the external scanner
	// However, we can't do anything to help them recover in that case so we
	// should just fail.
	/*
	  link: https://tree-sitter.github.io/tree-sitter/creating-parsers#external-scanners
	  If a syntax error is encountered during regular parsing, Tree-sitter’s
	  first action during error recovery will be to call the external scanner’s
	  scan function with all tokens marked valid. The scanner should detect this
	  case and handle it appropriately. One simple method of detection is to add
	  an unused token to the end of the externals array, for example

	  externals: $ => [$.token1, $.token2, $.error_sentinel],

	  then check whether that token is marked valid to determine whether
	  Tree-sitter is in error correction mode.
	*/
	if validSymbols[errorSentinel] {
		return false
	}

	if validSymbols[blockCommentContent] || validSymbols[blockInnerDocMarker] ||
		validSymbols[blockOuterDocMarker] {
		return processBlockComment(lexer, validSymbols)
	}

	if validSymbols[stringContent] && !validSymbols[floatLiteral] {
		return processString(lexer)
	}

	if validSymbols[lineDocContent] {
		return processLineDocContent(lexer)
	}

	for wctype.Iswspace(lexer.Lookahead) {
		skip(lexer)
	}

	if validSymbols[rawStringLiteralStart] &&
		(lexer.Lookahead == 'r' || lexer.Lookahead == 'b' || lexer.Lookahead == 'c') {
		return s.scanRawStringStart(lexer)
	}

	if validSymbols[rawStringLiteralContent] {
		return s.scanRawStringContent(lexer)
	}

	if validSymbols[rawStringLiteralEnd] && lexer.Lookahead == '"' {
		return s.scanRawStringEnd(lexer)
	}

	if validSymbols[floatLiteral] && wctype.Iswdigit(lexer.Lookahead) {
		return processFloatLiteral(lexer)
	}

	return false
}
