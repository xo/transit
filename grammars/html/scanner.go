package html

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// tokenType is enum TokenType, the external tokens of the grammar, in the
// order of externals in grammar.json.
type tokenType uint16

const (
	startTagName tokenType = iota
	scriptStartTagName
	styleStartTagName
	endTagName
	erroneousEndTagName
	selfClosingTagDelimiter
	implicitEndTag
	rawText
	comment
)

// scanner is a port of src/scanner.c of tree-sitter-html at v0.23.2
// (5a5ca8551a179998360b4a4ca2c0f366a35acc03). It is the struct Scanner. The
// macro MAX of C has no use, so it has no port.
type scanner struct {
	tags []tag
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// serialize is serialize. It writes these fields:
//
//   - at offset 0, the number of tags that it wrote, as a little-endian
//     uint16
//   - at offset 2, the number of tags, as a little-endian uint16, which is at
//     most math.MaxUint16
//   - from offset 4, one byte for the type of each tag, and for a custom tag
//     one byte for the length of its name, at most math.MaxUint8, and the
//     bytes of the name as strncpy copies them
//
// It stops at the first tag that does not fit.
func (s *scanner) serialize(buffer []byte) uint32 {
	tagCount := uint16(min(len(s.tags), math.MaxUint16))
	var serializedTagCount uint16

	size := uint32(2)
	binary.LittleEndian.PutUint16(buffer[size:], tagCount)
	size += 2

	for ; serializedTagCount < tagCount; serializedTagCount++ {
		t := s.tags[serializedTagCount]
		if t.typ == tagCustom {
			nameLength := min(uint32(len(t.customTagName)), math.MaxUint8)
			if size+2+nameLength >= abi.SerializationBufferSize {
				break
			}
			buffer[size] = byte(t.typ)
			size++
			buffer[size] = byte(nameLength)
			size++
			strncpy(buffer[size:size+nameLength], t.customTagName)
			size += nameLength
		} else {
			if size+1 >= abi.SerializationBufferSize {
				break
			}
			buffer[size] = byte(t.typ)
			size++
		}
	}

	binary.LittleEndian.PutUint16(buffer[0:], serializedTagCount)
	return size
}

// strncpy is strncpy of the C library, which serialize calls. It copies the
// bytes of src to dst up to the first zero byte, and it fills the rest of dst
// with zeros. src holds at least len(dst) bytes.
func strncpy(dst, src []byte) {
	for i := range dst {
		if src[i] == 0 {
			clear(dst[i:])
			return
		}
		dst[i] = src[i]
	}
}

// deserialize is deserialize. The type of a tag is a C char, which is signed
// on the platforms that CI tests, so a byte from 0x80 up gives a type with
// the high bits set, as in C. No result depends on the sign.
//
// The C function reads the buffer with no test of its length. The Go form
// stops where the buffer ends, so that a short buffer never makes Go panic.
func (s *scanner) deserialize(buffer []byte) {
	s.tags = s.tags[:0]

	length := uint32(len(buffer))
	if length > 0 {
		var size uint32

		if length < 4 {
			return
		}
		serializedTagCount := binary.LittleEndian.Uint16(buffer[size:])
		size += 2

		tagCount := binary.LittleEndian.Uint16(buffer[size:])
		size += 2

		s.tags = slices.Grow(s.tags, int(tagCount))
		if tagCount > 0 {
			for range serializedTagCount {
				if size >= length {
					return
				}
				t := tagNew()
				t.typ = tagType(int8(buffer[size]))
				size++
				if t.typ == tagCustom {
					if size >= length {
						return
					}
					nameLength := uint32(buffer[size])
					size++
					if nameLength > length-size {
						return
					}
					t.customTagName = slices.Clone(buffer[size : size+nameLength])
					size += nameLength
				}
				s.tags = append(s.tags, t)
			}
			// add zero tags if we didn't read enough, this is because the
			// buffer had no more room but we held more tags.
			for iter := serializedTagCount; iter < tagCount; iter++ {
				s.tags = append(s.tags, tagNew())
			}
		}
	}
}

// scanTagName is scan_tag_name. As in C, each character of the name is cut
// to one byte after towupper.
func scanTagName(lexer *abi.Lexer) []byte {
	var tagName []byte
	for wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '-' || lexer.Lookahead == ':' {
		tagName = append(tagName, byte(wctype.Towupper(lexer.Lookahead)))
		advance(lexer)
	}
	return tagName
}

// scanComment is scan_comment. As in C, the case '>' falls through to the
// default case when fewer than two dashes come before it.
func scanComment(lexer *abi.Lexer) bool {
	if lexer.Lookahead != '-' {
		return false
	}
	advance(lexer)
	if lexer.Lookahead != '-' {
		return false
	}
	advance(lexer)

	var dashes uint32
	for lexer.Lookahead != 0 {
		switch lexer.Lookahead {
		case '-':
			dashes++
		case '>':
			if dashes >= 2 {
				lexer.ResultSymbol = uint16(comment)
				advance(lexer)
				lexer.MarkEnd()
				return true
			}
			fallthrough
		default:
			dashes = 0
		}
		advance(lexer)
	}
	return false
}

// scanRawText is scan_raw_text.
func (s *scanner) scanRawText(lexer *abi.Lexer) bool {
	if len(s.tags) == 0 {
		return false
	}

	lexer.MarkEnd()

	endDelimiter := "</STYLE"
	if s.tags[len(s.tags)-1].typ == tagScript {
		endDelimiter = "</SCRIPT"
	}

	delimiterIndex := 0
	for lexer.Lookahead != 0 {
		if wctype.Towupper(lexer.Lookahead) == int32(endDelimiter[delimiterIndex]) {
			delimiterIndex++
			if delimiterIndex == len(endDelimiter) {
				break
			}
			advance(lexer)
		} else {
			delimiterIndex = 0
			advance(lexer)
			lexer.MarkEnd()
		}
	}

	lexer.ResultSymbol = uint16(rawText)
	return true
}

// popTag is pop_tag.
func (s *scanner) popTag() {
	s.tags = s.tags[:len(s.tags)-1]
}

// scanImplicitEndTag is scan_implicit_end_tag.
func (s *scanner) scanImplicitEndTag(lexer *abi.Lexer) bool {
	var parent *tag
	if len(s.tags) != 0 {
		parent = &s.tags[len(s.tags)-1]
	}

	isClosingTag := false
	if lexer.Lookahead == '/' {
		isClosingTag = true
		advance(lexer)
	} else if parent != nil && tagIsVoid(parent) {
		s.popTag()
		lexer.ResultSymbol = uint16(implicitEndTag)
		return true
	}

	tagName := scanTagName(lexer)
	if len(tagName) == 0 && !lexer.EOF() {
		return false
	}

	nextTag := tagForName(tagName)

	if isClosingTag {
		// The tag correctly closes the topmost element on the stack
		if len(s.tags) > 0 && tagEq(&s.tags[len(s.tags)-1], &nextTag) {
			return false
		}

		// Otherwise, dig deeper and queue implicit end tags (to be nice in
		// the case of malformed HTML)
		for i := len(s.tags); i > 0; i-- {
			if s.tags[i-1].typ == nextTag.typ {
				s.popTag()
				lexer.ResultSymbol = uint16(implicitEndTag)
				return true
			}
		}
	} else if parent != nil &&
		(!tagCanContain(parent, &nextTag) ||
			((parent.typ == tagHTML || parent.typ == tagHead || parent.typ == tagBody) && lexer.EOF())) {
		s.popTag()
		lexer.ResultSymbol = uint16(implicitEndTag)
		return true
	}

	return false
}

// scanStartTagName is scan_start_tag_name.
func (s *scanner) scanStartTagName(lexer *abi.Lexer) bool {
	tagName := scanTagName(lexer)
	if len(tagName) == 0 {
		return false
	}

	t := tagForName(tagName)
	s.tags = append(s.tags, t)
	switch t.typ {
	case tagScript:
		lexer.ResultSymbol = uint16(scriptStartTagName)
	case tagStyle:
		lexer.ResultSymbol = uint16(styleStartTagName)
	default:
		lexer.ResultSymbol = uint16(startTagName)
	}
	return true
}

// scanEndTagName is scan_end_tag_name.
func (s *scanner) scanEndTagName(lexer *abi.Lexer) bool {
	tagName := scanTagName(lexer)

	if len(tagName) == 0 {
		return false
	}

	t := tagForName(tagName)
	if len(s.tags) > 0 && tagEq(&s.tags[len(s.tags)-1], &t) {
		s.popTag()
		lexer.ResultSymbol = uint16(endTagName)
	} else {
		lexer.ResultSymbol = uint16(erroneousEndTagName)
	}

	return true
}

// scanSelfClosingTagDelimiter is scan_self_closing_tag_delimiter.
func (s *scanner) scanSelfClosingTagDelimiter(lexer *abi.Lexer) bool {
	advance(lexer)
	if lexer.Lookahead == '>' {
		advance(lexer)
		if len(s.tags) > 0 {
			s.popTag()
			lexer.ResultSymbol = uint16(selfClosingTagDelimiter)
		}
		return true
	}
	return false
}

// scan is scan.
func (s *scanner) scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[rawText] && !validSymbols[startTagName] && !validSymbols[endTagName] {
		return s.scanRawText(lexer)
	}

	for wctype.Iswspace(lexer.Lookahead) {
		skip(lexer)
	}

	switch lexer.Lookahead {
	case '<':
		lexer.MarkEnd()
		advance(lexer)

		if lexer.Lookahead == '!' {
			advance(lexer)
			return scanComment(lexer)
		}

		if validSymbols[implicitEndTag] {
			return s.scanImplicitEndTag(lexer)
		}

	case 0:
		if validSymbols[implicitEndTag] {
			return s.scanImplicitEndTag(lexer)
		}

	case '/':
		if validSymbols[selfClosingTagDelimiter] {
			return s.scanSelfClosingTagDelimiter(lexer)
		}

	default:
		if (validSymbols[startTagName] || validSymbols[endTagName]) && !validSymbols[rawText] {
			if validSymbols[startTagName] {
				return s.scanStartTagName(lexer)
			}
			return s.scanEndTagName(lexer)
		}
	}

	return false
}

// newScanner is tree_sitter_html_external_scanner_create.
// tree_sitter_html_external_scanner_destroy has no port, because it only
// frees memory.
func newScanner() *scanner {
	return &scanner{}
}

// Scan is tree_sitter_html_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return s.scan(lexer, validSymbols)
}

// Serialize is tree_sitter_html_external_scanner_serialize.
func (s *scanner) Serialize(buffer []byte) int {
	return int(s.serialize(buffer))
}

// Deserialize is tree_sitter_html_external_scanner_deserialize.
func (s *scanner) Deserialize(buffer []byte) {
	s.deserialize(buffer)
}
