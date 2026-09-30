package cpp

import (
	"encoding/binary"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// tokenType is enum TokenType, the external tokens of the grammar, in the
// order of externals in grammar.json.
type tokenType uint16

// The external tokens of the grammar.
const (
	rawStringDelimiter tokenType = iota
	rawStringContent
)

// maxDelimiterLength is MAX_DELIMITER_LENGTH.
//
// The spec limits delimiters to 16 chars.
const maxDelimiterLength = 16

// wcharSize is sizeof(wchar_t), which is 4 on each platform that CI tests.
const wcharSize = 4

// The C function serialize has a static_assert that the delimiter fits in
// the buffer. This constant is the Go form: it is negative, and so it does
// not compile, when the delimiter does not fit.
//
// Serialized delimiter is too long!
const _ uint = abi.SerializationBufferSize - maxDelimiterLength*wcharSize - 1

// scanner is a port of src/scanner.c of tree-sitter-cpp at v0.23.4
// (f41e1a044c8a84ea9fa8577fdd2eab92ec96de02). It is the C struct Scanner.
type scanner struct {
	delimiterLength uint8
	delimiter       [maxDelimiterLength]int32
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// reset is reset.
func (s *scanner) reset() {
	s.delimiterLength = 0
	s.delimiter = [maxDelimiterLength]int32{}
}

// scanRawStringDelimiter is scan_raw_string_delimiter.
//
// Scan the raw string delimiter in R"delimiter(content)delimiter".
func (s *scanner) scanRawStringDelimiter(lexer *abi.Lexer) bool {
	if s.delimiterLength > 0 {
		// Closing delimiter: must exactly match the opening delimiter.
		// We already checked this when scanning content, but this is how we
		// know when to stop. We can't stop at ", because R"""hello""" is valid.
		for i := range int(s.delimiterLength) {
			if lexer.Lookahead != s.delimiter[i] {
				return false
			}
			advance(lexer)
		}
		s.reset()
		return true
	}

	// Opening delimiter: record the d-char-sequence up to (.
	// d-char is any basic character except parens, backslashes, and spaces.
	for {
		if s.delimiterLength >= maxDelimiterLength || lexer.EOF() || lexer.Lookahead == '\\' ||
			wctype.Iswspace(lexer.Lookahead) {
			return false
		}
		if lexer.Lookahead == '(' {
			// Rather than create a token for an empty delimiter, we fail and
			// let the grammar fall back to a delimiter-less rule.
			return s.delimiterLength > 0
		}
		s.delimiter[s.delimiterLength] = lexer.Lookahead
		s.delimiterLength++
		advance(lexer)
	}
}

// scanRawStringContent is scan_raw_string_content.
//
// Scan the raw string content in R"delimiter(content)delimiter".
func (s *scanner) scanRawStringContent(lexer *abi.Lexer) bool {
	// The progress made through the delimiter since the last ')'.
	// The delimiter may not contain ')' so a single counter suffices.
	for delimiterIndex := -1; ; {
		// If we hit EOF, consider the content to terminate there.
		// This forms an incomplete raw_string_literal, and models the code
		// well.
		if lexer.EOF() {
			lexer.MarkEnd()
			return true
		}

		if delimiterIndex >= 0 {
			if delimiterIndex == int(s.delimiterLength) {
				if lexer.Lookahead == '"' {
					return true
				}
				delimiterIndex = -1
			} else {
				if lexer.Lookahead == s.delimiter[delimiterIndex] {
					delimiterIndex++
				} else {
					delimiterIndex = -1
				}
			}
		}

		if delimiterIndex == -1 && lexer.Lookahead == ')' {
			// The content doesn't include the )delimiter" part.
			// We must still scan through it, but exclude it from the token.
			lexer.MarkEnd()
			delimiterIndex = 0
		}

		advance(lexer)
	}
}

// newScanner is tree_sitter_cpp_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Scan is tree_sitter_cpp_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[rawStringDelimiter] && validSymbols[rawStringContent] {
		// we're in error recovery
		return false
	}

	// No skipping leading whitespace: raw-string grammar is space-sensitive.
	if validSymbols[rawStringDelimiter] {
		lexer.ResultSymbol = uint16(rawStringDelimiter)
		return s.scanRawStringDelimiter(lexer)
	}

	if validSymbols[rawStringContent] {
		lexer.ResultSymbol = uint16(rawStringContent)
		return s.scanRawStringContent(lexer)
	}

	return false
}

// Serialize is tree_sitter_cpp_external_scanner_serialize. It writes each
// character of the delimiter as a wchar_t of 4 bytes, in little-endian
// order.
func (s *scanner) Serialize(buffer []byte) int {
	size := int(s.delimiterLength) * wcharSize
	for i := range int(s.delimiterLength) {
		binary.LittleEndian.PutUint32(buffer[i*wcharSize:], uint32(s.delimiter[i]))
	}
	return size
}

// Deserialize is tree_sitter_cpp_external_scanner_deserialize.
//
// The C function asserts that the length is a multiple of 4, and it copies
// the whole buffer into the delimiter, which holds 64 bytes. The runtime
// gives it only the bytes that serialize wrote, so both hold. For a buffer
// that breaks them, the C function stops the program or writes past the
// delimiter. The Go function reads only the whole characters that fit in
// the delimiter.
func (s *scanner) Deserialize(buffer []byte) {
	// Can't decode serialized delimiter!
	s.delimiterLength = uint8(min(len(buffer)/wcharSize, maxDelimiterLength))
	if len(buffer) > 0 {
		for i := range int(s.delimiterLength) {
			s.delimiter[i] = int32(binary.LittleEndian.Uint32(buffer[i*wcharSize:]))
		}
	}
}
