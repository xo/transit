package clickhouse

import (
	"encoding/binary"

	"github.com/xo/transit/internal/abi"
)

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js.
const (
	comment = iota
	heredocStart
	stringContent
	heredocEnd
	insertData
)

// maxTag is MAX_TAG, the longest tag that the scanner keeps, in
// characters. With the byte of the length, the state fits in the
// serialization buffer.
const maxTag = 255

// scanner is a port of src/scanner.c of the grammar. It reads the comments
// and the heredoc strings. A comment is -- or // to the end of the line, #
// followed by a space or by ! to the end of the line, or /* ... */, which
// can hold other block comments. A heredoc string is $tag$ ... $tag$, where
// the tag is empty or has the form of an identifier without a dollar sign.
// The scanner gives the opening tag, the text and the closing tag as three
// tokens, so that an injection can take the text. It keeps the tag from the
// opening tag to the closing tag. The scanner also reads the data of INSERT
// after FORMAT and the name of the format, to the end of the line.
type scanner struct {
	// open is 1 from the opening tag of a heredoc string to its closing
	// tag.
	open uint8
	// length is the number of the characters of the tag.
	length uint8
	// tag holds the characters of the tag.
	tag [maxTag]int32
}

// newScanner is tree_sitter_clickhouse_external_scanner_create.
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
func isBlank(c int32) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v'
}

// isTagStart is is_tag_start. It reports whether c can start a tag: a
// letter, an underscore or a character outside ASCII.
func isTagStart(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80
}

// isTagChar is is_tag_char. It reports whether c can be in a tag after its
// first character.
func isTagChar(c int32) bool { return isTagStart(c) || (c >= '0' && c <= '9') }

// scanLineComment is scan_line_comment. It reads a comment to the end of
// the line. The start of the comment, --, // or # and a space or !, is
// already read.
func scanLineComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() && lexer.Lookahead != '\n' && lexer.Lookahead != '\r' {
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanBlockComment is scan_block_comment. It reads a block comment to its
// end, or to the end of the input. The /* of the comment is already read. A
// block comment inside it must end before it ends.
func scanBlockComment(lexer *abi.Lexer) bool {
	depth := uint(1)
loop:
	for !lexer.EOF() {
		switch lexer.Lookahead {
		case '*':
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				depth--
				if depth == 0 {
					break loop
				}
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
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanStart is scan_start. It reads the opening tag of a heredoc string,
// and keeps its tag. The $ of the tag is already read.
func (s *scanner) scanStart(lexer *abi.Lexer) bool {
	var length uint8
	if isTagStart(lexer.Lookahead) {
		for isTagChar(lexer.Lookahead) {
			if length == maxTag {
				return false
			}
			s.tag[length] = lexer.Lookahead
			length++
			advance(lexer)
		}
	}
	if lexer.Lookahead != '$' {
		return false
	}
	advance(lexer)
	lexer.MarkEnd()
	s.open = 1
	s.length = length
	lexer.ResultSymbol = heredocStart
	return true
}

// scanEnd is scan_end. It reports whether the closing tag comes next, and
// reads as much of it as matches. The $ before the tag is already read.
func (s *scanner) scanEnd(lexer *abi.Lexer) bool {
	for i := range s.length {
		if lexer.Lookahead != s.tag[i] {
			return false
		}
		advance(lexer)
	}
	if lexer.Lookahead != '$' {
		return false
	}
	advance(lexer)
	return true
}

// scanContent is scan_content. It reads the text of a heredoc string
// up to its closing tag, or the closing tag when no text comes before it.
// The text goes to the end of the input when the closing tag does not
// come.
func (s *scanner) scanContent(lexer *abi.Lexer, validSymbols []bool) bool {
	text := false
	for !lexer.EOF() {
		if lexer.Lookahead == '$' {
			lexer.MarkEnd()
			advance(lexer)
			if s.scanEnd(lexer) {
				if text {
					lexer.ResultSymbol = stringContent
					return validSymbols[stringContent]
				}
				if !validSymbols[heredocEnd] {
					return false
				}
				lexer.MarkEnd()
				s.open = 0
				s.length = 0
				lexer.ResultSymbol = heredocEnd
				return true
			}
		} else {
			advance(lexer)
		}
		text = true
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = stringContent
	return text && validSymbols[stringContent]
}

// scanInsertData is scan_insert_data. It reads the data of INSERT to the
// end of the line. It skips the spaces and one line feed before the data.
// It returns false when no data comes, and then it has read only white
// space, so that the caller can go on to read a comment.
func scanInsertData(lexer *abi.Lexer) bool {
	for isBlank(lexer.Lookahead) {
		skip(lexer)
	}
	if lexer.Lookahead == '\n' {
		skip(lexer)
		for isBlank(lexer.Lookahead) {
			skip(lexer)
		}
	}
	if lexer.Lookahead == ';' {
		return false
	}
	data := false
	for !lexer.EOF() && lexer.Lookahead != '\n' {
		if lexer.Lookahead == ';' {
			advance(lexer)
			for isBlank(lexer.Lookahead) {
				advance(lexer)
			}
			if lexer.EOF() || lexer.Lookahead == '\n' {
				break
			}
			continue
		}
		blank := isBlank(lexer.Lookahead)
		advance(lexer)
		if !blank {
			lexer.MarkEnd()
			data = true
		}
	}
	lexer.ResultSymbol = insertData
	return data
}

// Serialize is tree_sitter_clickhouse_external_scanner_serialize. The state
// is open and the length of the tag in two bytes, and then each character
// of the tag in four bytes, in little-endian order.
func (s *scanner) Serialize(buf []byte) int {
	buf[0] = s.open
	buf[1] = s.length
	for i := range int(s.length) {
		binary.LittleEndian.PutUint32(buf[2+4*i:], uint32(s.tag[i]))
	}
	return 2 + 4*int(s.length)
}

// Deserialize is tree_sitter_clickhouse_external_scanner_deserialize.
func (s *scanner) Deserialize(buf []byte) {
	s.open = 0
	s.length = 0
	if len(buf) < 2 {
		return
	}
	n := buf[1]
	if len(buf) < 2+4*int(n) {
		return
	}
	if buf[0] != 0 {
		s.open = 1
	}
	s.length = n
	for i := range int(n) {
		s.tag[i] = int32(binary.LittleEndian.Uint32(buf[2+4*i:]))
	}
}

// Scan is tree_sitter_clickhouse_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if s.open != 0 && (validSymbols[stringContent] || validSymbols[heredocEnd]) {
		return s.scanContent(lexer, validSymbols)
	}
	// In error recovery, each token is valid, and the scanner does not read
	// the data of INSERT.
	if validSymbols[insertData] && !validSymbols[heredocEnd] && scanInsertData(lexer) {
		return true
	}
	for isSpace(lexer.Lookahead) {
		skip(lexer)
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
		if lexer.Lookahead == '/' {
			advance(lexer)
			return scanLineComment(lexer)
		}
		if lexer.Lookahead != '*' {
			return false
		}
		advance(lexer)
		return scanBlockComment(lexer)
	}
	if c == '#' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != ' ' && lexer.Lookahead != '!' {
			return false
		}
		advance(lexer)
		return scanLineComment(lexer)
	}
	if c == '$' && validSymbols[heredocStart] {
		advance(lexer)
		return s.scanStart(lexer)
	}
	return false
}
