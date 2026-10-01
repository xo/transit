package sql

import (
	"bytes"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// The external tokens of the scanner, in the order of externals in
// grammar.json. They are the C enum TokenType.
const (
	dollarQuotedStringStartTag = iota
	dollarQuotedStringEndTag
	dollarQuotedString
)

// scanner is LexerState, the external scanner of the grammar sql. It is a
// port of src/scanner.c of tree-sitter-sql at v0.3.11
// (7b51ecda191d36b92f5a90a8d1bc3faef1c7b8b8).
//
// startTag is start_tag, the tag of the dollar quoted string that the
// scanner is in. It is nil where the C pointer is NULL. The C tag is a
// string that ends at its first 0 byte, so the Go scanner compares and
// writes the bytes of a tag up to its first 0 byte, as cString gives them.
//
// The C scanner calls iswspace, and the Go scanner calls the function of
// the same name in internal/wctype. It answers as the package unicode does,
// so it can answer differently from C for a character that is not ASCII
// (D39, D46).
type scanner struct {
	startTag []byte
}

// newScanner is tree_sitter_sql_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// cString returns the bytes of a C string up to its first 0 byte. A string
// with no 0 byte ends at the end of the slice, as a byte past the end reads
// as 0 (D85).
func cString(text []byte) []byte {
	before, _, _ := bytes.Cut(text, []byte{0})
	return before
}

// addChar is add_char. The C function grows a buffer of MALLOC_STRING_SIZE
// bytes, and a Go slice grows by itself. The C char is signed, so a
// character above 0xff keeps its low byte, as the conversion to byte does.
func addChar(text []byte, c byte) []byte {
	// will break when indexes advances more than MALLOC_STRING_SIZE
	return append(text, c)
}

// scanDollarStringTag is scan_dollar_string_tag. It returns nil where the C
// function returns NULL.
func scanDollarStringTag(lexer *abi.Lexer) []byte {
	var tag []byte
	if lexer.Lookahead == '$' {
		tag = addChar(tag, '$')
		lexer.Advance(false)
	} else {
		return nil
	}

	for lexer.Lookahead != '$' && !wctype.Iswspace(lexer.Lookahead) && !lexer.EOF() {
		tag = addChar(tag, byte(lexer.Lookahead))
		lexer.Advance(false)
	}

	if lexer.Lookahead == '$' {
		tag = addChar(tag, byte(lexer.Lookahead))
		lexer.Advance(false)
		return tag
	}
	return nil
}

// Scan is tree_sitter_sql_external_scanner_scan. strcmp becomes a
// comparison of the two tags up to their first 0 byte.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[dollarQuotedStringStartTag] && s.startTag == nil {
		for wctype.Iswspace(lexer.Lookahead) {
			lexer.Advance(true)
		}

		startTag := scanDollarStringTag(lexer)
		if startTag == nil {
			return false
		}
		s.startTag = startTag
		lexer.ResultSymbol = dollarQuotedStringStartTag
		return true
	}

	if validSymbols[dollarQuotedStringEndTag] && s.startTag != nil {
		for wctype.Iswspace(lexer.Lookahead) {
			lexer.Advance(true)
		}

		endTag := scanDollarStringTag(lexer)
		if endTag != nil && bytes.Equal(cString(endTag), cString(s.startTag)) {
			s.startTag = nil
			lexer.ResultSymbol = dollarQuotedStringEndTag
			return true
		}
		return false
	}

	if validSymbols[dollarQuotedString] {
		lexer.MarkEnd()
		for wctype.Iswspace(lexer.Lookahead) {
			lexer.Advance(true)
		}

		startTag := scanDollarStringTag(lexer)
		if startTag == nil {
			return false
		}

		if s.startTag != nil && bytes.Equal(cString(s.startTag), cString(startTag)) {
			return false
		}

		for {
			if lexer.EOF() {
				return false
			}

			endTag := scanDollarStringTag(lexer)
			if endTag == nil {
				lexer.Advance(false)
				continue
			}

			if bytes.Equal(cString(endTag), cString(startTag)) {
				lexer.MarkEnd()
				lexer.ResultSymbol = dollarQuotedString
				return true
			}
		}
	}

	return false
}

// Serialize is tree_sitter_sql_external_scanner_serialize. It writes the
// start tag and a 0 byte after it. Like the C function, it also clears the
// start tag, so the scanner after it is in no dollar quoted string until
// Deserialize gives it the state again.
func (s *scanner) Serialize(buffer []byte) int {
	if s.startTag == nil {
		return 0
	}
	tag := cString(s.startTag)
	// + 1 for the '\0'
	tagLength := len(tag) + 1
	if tagLength >= abi.SerializationBufferSize {
		return 0
	}

	copy(buffer, tag)
	buffer[len(tag)] = 0
	s.startTag = nil
	return tagLength
}

// Deserialize is tree_sitter_sql_external_scanner_deserialize. A buffer of
// two bytes or more gives a start tag, which can be empty when the buffer
// starts with a 0 byte. The tag is not nil then, as the C pointer is not
// NULL.
func (s *scanner) Deserialize(buffer []byte) {
	s.startTag = nil
	// A length of 1 can't exists.
	if len(buffer) > 1 {
		s.startTag = append([]byte{}, cString(buffer)...)
	}
}
