// Package scan is the external scanner that the grammars php and php_only
// share. It ports common/scanner.h of tree-sitter-php, which the file
// src/scanner.c of each grammar includes.
package scan

import (
	"encoding/binary"
	"slices"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// tokenType is enum TokenType, the external tokens of the grammar, in the
// order of externals in grammar.json.
type tokenType uint16

const (
	automaticSemicolon tokenType = iota
	encapsedStringChars
	encapsedStringCharsAfterVariable
	executionStringChars
	executionStringCharsAfterVariable
	encapsedStringCharsHeredoc
	encapsedStringCharsAfterVariableHeredoc
	eofToken
	heredocStart
	heredocEnd
	nowdocString
	sentinelError // Unused token used to indicate error recovery mode
)

// stringEq is string_eq. A String of C is an Array(int32_t), and a Go slice
// of int32 here.
func stringEq(self, other []int32) bool {
	return slices.Equal(self, other)
}

// heredoc is the struct Heredoc.
type heredoc struct {
	endWordIndentationAllowed bool
	word                      []int32
}

// The enum ScanContentResult of C has no use, so it has no port.

// heredocNew is the macro heredoc_new.
func heredocNew() heredoc {
	return heredoc{
		endWordIndentationAllowed: false,
		word:                      nil,
	}
}

// Scanner is a port of common/scanner.h of tree-sitter-php at v0.24.2, on
// the branch upstream_test_fixture
// (e8074c9943298d30a1af9ff84c2b61a6dc600d86). It is the struct Scanner, and
// its methods are the functions of the file.
type Scanner struct {
	hasLeadingWhitespace bool
	heredocs             []heredoc
}

// resetHeredoc is reset_heredoc. It empties the word, and it keeps the
// heredoc in the list.
func resetHeredoc(h *heredoc) {
	h.word = nil
	h.endWordIndentationAllowed = false
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// serialize is serialize. It writes the number of heredocs in one byte, cut
// to 8 bits. Then for each heredoc it writes a byte for
// endWordIndentationAllowed, the length of the word as a little-endian
// uint32, and each character of the word as a little-endian int32. It
// writes nothing, and returns 0, when a heredoc does not fit.
func (s *Scanner) serialize(buffer []byte) uint32 {
	var size uint32

	buffer[size] = byte(len(s.heredocs))
	size++
	for j := range s.heredocs {
		h := &s.heredocs[j]
		wordSize := uint32(len(h.word)) * 4
		if size+5+wordSize >= abi.SerializationBufferSize {
			return 0
		}
		buffer[size] = 0
		if h.endWordIndentationAllowed {
			buffer[size] = 1
		}
		size++
		binary.LittleEndian.PutUint32(buffer[size:], uint32(len(h.word)))
		size += 4
		if len(h.word) > 0 {
			for i, c := range h.word {
				binary.LittleEndian.PutUint32(buffer[size+uint32(i)*4:], uint32(c))
			}
			size += wordSize
		}
	}

	return size
}

// deserialize is deserialize. As in C, it empties the words of the heredocs
// that the scanner holds, and it keeps them in the list. So a scanner that
// held more heredocs than the buffer names keeps the others, with no word.
//
// The C function reads the buffer with no test of its length, and it ends
// with assert(size == length). The Go form stops where the buffer ends, so
// that a short buffer never makes Go panic, and it has no assert.
func (s *Scanner) deserialize(buffer []byte) {
	var size uint32
	length := uint32(len(buffer))
	s.hasLeadingWhitespace = false

	for i := range s.heredocs {
		resetHeredoc(&s.heredocs[i])
	}

	if length == 0 {
		return
	}

	openHeredocCount := buffer[size]
	size++
	for i := range int(openHeredocCount) {
		var h *heredoc
		if i < len(s.heredocs) {
			h = &s.heredocs[i]
		} else {
			s.heredocs = append(s.heredocs, heredocNew())
			h = &s.heredocs[len(s.heredocs)-1]
		}

		if length-size < 5 {
			return
		}
		h.endWordIndentationAllowed = buffer[size] != 0
		size++
		wordLength := binary.LittleEndian.Uint32(buffer[size:])
		size += 4
		wordSize := wordLength * 4
		if wordSize > 0 {
			if uint64(wordLength)*4 > uint64(length-size) {
				return
			}
			h.word = make([]int32, wordLength)
			for k := range h.word {
				h.word[k] = int32(binary.LittleEndian.Uint32(buffer[size+uint32(k)*4:]))
			}
			size += wordSize
		}
	}
}

// scanWhitespace is scan_whitespace.
func scanWhitespace(lexer *abi.Lexer) bool {
	for {
		for wctype.Iswspace(lexer.Lookahead) {
			advance(lexer)
		}

		if lexer.Lookahead == '/' {
			advance(lexer)

			if lexer.Lookahead == '/' {
				advance(lexer)
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					advance(lexer)
				}
			} else {
				return false
			}
		} else {
			return true
		}
	}
}

// isValidNameChar is is_valid_name_char.
func isValidNameChar(lexer *abi.Lexer) bool {
	return wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '_' || lexer.Lookahead >= 0x80
}

// isEscapableSequence is is_escapable_sequence.
func isEscapableSequence(lexer *abi.Lexer) bool {
	// Note: remember to also update the escape_sequence rule in the
	// main grammar whenever changing this method
	letter := lexer.Lookahead

	if letter == 'n' || letter == 'r' || letter == 't' || letter == 'v' || letter == 'e' || letter == 'f' ||
		letter == '\\' || letter == '$' || letter == '"' {
		return true
	}

	// Hex
	if letter == 'x' {
		advance(lexer)
		return wctype.Iswxdigit(lexer.Lookahead)
	}

	// Unicode
	if letter == 'u' {
		return true // We handle the case where this is not really an escape
		// sequence in grammar.js - this is needed to support the
		// edge case "\u{$a}" in which case "\u" is to be
		// interpreted as characters and {$a} as a variable
	}

	// Octal
	return wctype.Iswdigit(lexer.Lookahead) && lexer.Lookahead >= '0' && lexer.Lookahead <= '7'
}

// scanHeredocWord is scan_heredoc_word.
func scanHeredocWord(lexer *abi.Lexer) []int32 {
	var result []int32

	for isValidNameChar(lexer) {
		result = append(result, lexer.Lookahead)
		advance(lexer)
	}

	return result
}

// scanNowdocString is scan_nowdoc_string.
func (s *Scanner) scanNowdocString(lexer *abi.Lexer) bool {
	hasConsumedContent := false
	if len(s.heredocs) == 0 {
		return false
	}

	// While PHP requires the nowdoc end tag to be the very first on a new line,
	// there may be an arbitrary amount of whitespace before the closing token
	for wctype.Iswspace(lexer.Lookahead) {
		advance(lexer)
		hasConsumedContent = true
	}

	endTagMatched := false
	heredocTag := s.heredocs[len(s.heredocs)-1].word

	for i := range heredocTag {
		if lexer.Lookahead != heredocTag[i] {
			break
		}
		advance(lexer)
		hasConsumedContent = true

		endTagMatched = i == len(heredocTag)-1 && (wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == ';' ||
			lexer.Lookahead == ',' || lexer.Lookahead == ')')
	}

	if endTagMatched {
		// There may be an arbitrary amount of white space after the end tag
		for wctype.Iswspace(lexer.Lookahead) && lexer.Lookahead != '\r' && lexer.Lookahead != '\n' {
			advance(lexer)
			hasConsumedContent = true
		}

		// Return to allow the end tag parsing if we've encountered an end tag
		// at a valid position
		if lexer.Lookahead == ';' || lexer.Lookahead == ',' || lexer.Lookahead == ')' || lexer.Lookahead == '\n' ||
			lexer.Lookahead == '\r' {
			// , and ) is needed to support heredoc in function arguments
			return false
		}
	}

	for hasContent := hasConsumedContent; ; hasContent = true {
		lexer.MarkEnd()

		switch lexer.Lookahead {
		case '\n', '\r':
			return hasContent
		default:
			if lexer.EOF() {
				return false
			}
			advance(lexer)
		}
	}
}

// scanEncapsedPartString is scan_encapsed_part_string. As in C, the case
// '-' falls through to the case '[' when isAfterVariable is false.
func (s *Scanner) scanEncapsedPartString(lexer *abi.Lexer, isAfterVariable, isHeredoc, isExecutionString bool) bool {
	hasConsumedContent := false

	if isHeredoc && len(s.heredocs) > 0 {
		// While PHP requires the heredoc end tag to be the very first on a new
		// line, there may be an arbitrary amount of whitespace before the
		// closing token However, we should not consume \r or \n
		for wctype.Iswspace(lexer.Lookahead) && lexer.Lookahead != '\r' && lexer.Lookahead != '\n' {
			advance(lexer)
			hasConsumedContent = true
		}

		heredocTag := s.heredocs[len(s.heredocs)-1].word

		endTagMatched := false

		for i := range heredocTag {
			if lexer.Lookahead != heredocTag[i] {
				break
			}
			hasConsumedContent = true
			advance(lexer)

			endTagMatched = i == len(heredocTag)-1 && (wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == ';' ||
				lexer.Lookahead == ',' || lexer.Lookahead == ')')
		}

		if endTagMatched {
			// There may be an arbitrary amount of white space after the end tag
			// However, we should not consume \r or \n
			for wctype.Iswspace(lexer.Lookahead) && lexer.Lookahead != '\r' && lexer.Lookahead != '\n' {
				advance(lexer)
				hasConsumedContent = true
			}

			// Return to allow the end tag parsing if we've encountered an end
			// tag at a valid position
			if lexer.Lookahead == ';' || lexer.Lookahead == ',' || lexer.Lookahead == ')' ||
				lexer.Lookahead == '\n' || lexer.Lookahead == '\r' {
				// , and ) is needed to support heredoc in function arguments
				return false
			}
		}
	}

	for hasContent := hasConsumedContent; ; hasContent = true {
		lexer.MarkEnd()

		switch lexer.Lookahead {
		case '"':
			if !isHeredoc && !isExecutionString {
				return hasContent
			}
			advance(lexer)
		case '`':
			if isExecutionString {
				return hasContent
			}
			advance(lexer)
		case '\n', '\r':
			if isHeredoc {
				return hasContent
			}
			advance(lexer)
		case '\\':
			advance(lexer)

			// \{ should not be interpreted as an escape sequence, but both
			// should be consumed as normal characters
			if lexer.Lookahead == '{' {
				advance(lexer)
				break
			}

			if isExecutionString && lexer.Lookahead == '`' {
				return hasContent
			}

			if isHeredoc && lexer.Lookahead == '\\' {
				advance(lexer)
				break
			}

			if isEscapableSequence(lexer) {
				return hasContent
			}
		case '$':
			advance(lexer)

			if (isValidNameChar(lexer) && !wctype.Iswdigit(lexer.Lookahead)) || lexer.Lookahead == '{' {
				return hasContent
			}
		case '-':
			if isAfterVariable {
				advance(lexer)
				if lexer.Lookahead == '>' {
					advance(lexer)
					if isValidNameChar(lexer) {
						return hasContent
					}
					break
				}
				break
			}
			fallthrough
		case '[':
			if isAfterVariable {
				return hasContent
			}
			advance(lexer)
		case '{':
			advance(lexer)
			if lexer.Lookahead == '$' {
				return hasContent
			}
		default:
			if lexer.EOF() {
				return false
			}
			advance(lexer)
		}

		isAfterVariable = false
	}
}

// scan is scan.
func (s *Scanner) scan(lexer *abi.Lexer, validSymbols []bool) bool {
	isErrorRecovery := validSymbols[sentinelError]

	if isErrorRecovery {
		return false
	}

	s.hasLeadingWhitespace = false

	lexer.MarkEnd()

	if validSymbols[encapsedStringCharsAfterVariable] {
		lexer.ResultSymbol = uint16(encapsedStringCharsAfterVariable)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ true,
			/* is_heredoc */ false,
			/* is_execution_string */ false)
	}

	if validSymbols[encapsedStringChars] {
		lexer.ResultSymbol = uint16(encapsedStringChars)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ false,
			/* is_heredoc */ false,
			/* is_execution_string */ false)
	}

	if validSymbols[executionStringCharsAfterVariable] {
		lexer.ResultSymbol = uint16(executionStringCharsAfterVariable)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ true,
			/* is_heredoc */ false,
			/* is_execution_string */ true)
	}

	if validSymbols[executionStringChars] {
		lexer.ResultSymbol = uint16(executionStringChars)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ false,
			/* is_heredoc */ false,
			/* is_execution_string */ true)
	}

	if validSymbols[encapsedStringCharsAfterVariableHeredoc] {
		lexer.ResultSymbol = uint16(encapsedStringCharsAfterVariableHeredoc)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ true,
			/* is_heredoc */ true,
			/* is_execution_string */ false)
	}

	if validSymbols[encapsedStringCharsHeredoc] {
		lexer.ResultSymbol = uint16(encapsedStringCharsHeredoc)
		return s.scanEncapsedPartString(lexer,
			/* is_after_variable */ false,
			/* is_heredoc */ true,
			/* is_execution_string */ false)
	}

	if validSymbols[nowdocString] {
		lexer.ResultSymbol = uint16(nowdocString)
		return s.scanNowdocString(lexer)
	}

	if validSymbols[heredocEnd] {
		lexer.ResultSymbol = uint16(heredocEnd)
		if len(s.heredocs) == 0 {
			return false
		}

		h := s.heredocs[len(s.heredocs)-1]

		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		word := scanHeredocWord(lexer)
		if !stringEq(word, h.word) {
			return false
		}

		lexer.MarkEnd()
		s.heredocs = s.heredocs[:len(s.heredocs)-1]
		return true
	}

	if !scanWhitespace(lexer) {
		return false
	}

	if validSymbols[eofToken] && lexer.EOF() {
		lexer.ResultSymbol = uint16(eofToken)
		return true
	}

	if validSymbols[heredocStart] {
		lexer.ResultSymbol = uint16(heredocStart)
		h := heredocNew()

		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		h.word = scanHeredocWord(lexer)
		if len(h.word) == 0 {
			return false
		}
		lexer.MarkEnd()

		s.heredocs = append(s.heredocs, h)
		return true
	}

	if validSymbols[automaticSemicolon] {
		lexer.ResultSymbol = uint16(automaticSemicolon)

		if lexer.Lookahead != '?' {
			return false
		}

		advance(lexer)

		return lexer.Lookahead == '>'
	}

	return false
}

// New is external_scanner_create. external_scanner_destroy has no port,
// because it only frees memory.
func New() *Scanner {
	return &Scanner{}
}

// Serialize is external_scanner_serialize.
func (s *Scanner) Serialize(buffer []byte) int {
	return int(s.serialize(buffer))
}

// Deserialize is external_scanner_deserialize.
func (s *Scanner) Deserialize(buffer []byte) {
	s.deserialize(buffer)
}

// Scan is external_scanner_scan.
func (s *Scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return s.scan(lexer, validSymbols)
}
