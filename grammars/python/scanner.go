package python

import (
	"math"
	"slices"

	"github.com/xo/transit/internal/abi"
)

// The external tokens of the scanner, in the order of externals in
// grammar.json. They are the C enum TokenType.
const (
	newline = iota
	indent
	dedent
	stringStart
	stringContent
	escapeInterpolation
	stringEnd
	comment
	closeParen
	closeBracket
	closeBrace
	except
)

// The flags of a delimiter. They are the C enum Flags.
const (
	singleQuote byte = 1 << 0
	doubleQuote byte = 1 << 1
	backQuote   byte = 1 << 2
	raw         byte = 1 << 3
	format      byte = 1 << 4
	triple      byte = 1 << 5
	bytes       byte = 1 << 6
)

// delimiter is Delimiter, the start of a string that the scanner is inside.
type delimiter struct {
	flags byte
}

// newDelimiter is new_delimiter.
func newDelimiter() delimiter { return delimiter{0} }

// isFormat is is_format.
func (d *delimiter) isFormat() bool { return d.flags&format != 0 }

// isRaw is is_raw.
func (d *delimiter) isRaw() bool { return d.flags&raw != 0 }

// isTriple is is_triple.
func (d *delimiter) isTriple() bool { return d.flags&triple != 0 }

// isBytes is is_bytes.
func (d *delimiter) isBytes() bool { return d.flags&bytes != 0 }

// endCharacter is end_character.
func (d *delimiter) endCharacter() int32 {
	if d.flags&singleQuote != 0 {
		return '\''
	}
	if d.flags&doubleQuote != 0 {
		return '"'
	}
	if d.flags&backQuote != 0 {
		return '`'
	}
	return 0
}

// setFormat is set_format.
func (d *delimiter) setFormat() { d.flags |= format }

// setRaw is set_raw.
func (d *delimiter) setRaw() { d.flags |= raw }

// setTriple is set_triple.
func (d *delimiter) setTriple() { d.flags |= triple }

// setBytes is set_bytes.
func (d *delimiter) setBytes() { d.flags |= bytes }

// setEndCharacter is set_end_character. The C function asserts that the
// character is a quote, and the Go function panics when it is not.
func (d *delimiter) setEndCharacter(character int32) {
	switch character {
	case '\'':
		d.flags |= singleQuote
	case '"':
		d.flags |= doubleQuote
	case '`':
		d.flags |= backQuote
	default:
		panic("python: set_end_character got a character that is not a quote")
	}
}

// scanner is Scanner, the external scanner of the grammar python. It is a
// port of src/scanner.c of tree-sitter-python at v0.23.6
// (bffb65a8cfe4e46290331dfef0dbf0ef3679de11).
type scanner struct {
	indents       []uint16
	delimiters    []delimiter
	insideFString bool
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// Scan is tree_sitter_python_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	errorRecoveryMode := validSymbols[stringContent] && validSymbols[indent]
	withinBrackets := validSymbols[closeBrace] || validSymbols[closeParen] || validSymbols[closeBracket]

	advancedOnce := false
	if validSymbols[escapeInterpolation] && len(s.delimiters) > 0 &&
		(lexer.Lookahead == '{' || lexer.Lookahead == '}') && !errorRecoveryMode {
		delimiter := &s.delimiters[len(s.delimiters)-1]
		if delimiter.isFormat() {
			lexer.MarkEnd()
			isLeftBrace := lexer.Lookahead == '{'
			advance(lexer)
			// The C function sets advanced_once to true here, but each path
			// after it returns, so advanced_once is false below.
			if (lexer.Lookahead == '{' && isLeftBrace) || (lexer.Lookahead == '}' && !isLeftBrace) {
				advance(lexer)
				lexer.MarkEnd()
				lexer.ResultSymbol = escapeInterpolation
				return true
			}
			return false
		}
	}

	if validSymbols[stringContent] && len(s.delimiters) > 0 && !errorRecoveryMode {
		delimiter := &s.delimiters[len(s.delimiters)-1]
		endChar := delimiter.endCharacter()
		hasContent := advancedOnce
		for lexer.Lookahead != 0 {
			if (advancedOnce || lexer.Lookahead == '{' || lexer.Lookahead == '}') && delimiter.isFormat() {
				lexer.MarkEnd()
				lexer.ResultSymbol = stringContent
				return hasContent
			}
			switch {
			case lexer.Lookahead == '\\':
				if delimiter.isRaw() {
					// Step over the backslash.
					advance(lexer)
					// Step over any escaped quotes.
					if lexer.Lookahead == delimiter.endCharacter() || lexer.Lookahead == '\\' {
						advance(lexer)
					}
					// Step over newlines
					switch lexer.Lookahead {
					case '\r':
						advance(lexer)
						if lexer.Lookahead == '\n' {
							advance(lexer)
						}
					case '\n':
						advance(lexer)
					}
					continue
				}
				if delimiter.isBytes() {
					lexer.MarkEnd()
					advance(lexer)
					if lexer.Lookahead == 'N' || lexer.Lookahead == 'u' || lexer.Lookahead == 'U' {
						// In bytes string, \N{...}, \uXXXX and \UXXXXXXXX are
						// not escape sequences
						// https://docs.python.org/3/reference/lexical_analysis.html#string-and-bytes-literals
						advance(lexer)
					} else {
						lexer.ResultSymbol = stringContent
						return hasContent
					}
				} else {
					lexer.MarkEnd()
					lexer.ResultSymbol = stringContent
					return hasContent
				}
			case lexer.Lookahead == endChar:
				if delimiter.isTriple() {
					lexer.MarkEnd()
					advance(lexer)
					if lexer.Lookahead == endChar {
						advance(lexer)
						if lexer.Lookahead == endChar {
							if hasContent {
								lexer.ResultSymbol = stringContent
							} else {
								advance(lexer)
								lexer.MarkEnd()
								s.delimiters = s.delimiters[:len(s.delimiters)-1]
								lexer.ResultSymbol = stringEnd
								s.insideFString = false
							}
							return true
						}
						lexer.MarkEnd()
						lexer.ResultSymbol = stringContent
						return true
					}
					lexer.MarkEnd()
					lexer.ResultSymbol = stringContent
					return true
				}
				if hasContent {
					lexer.ResultSymbol = stringContent
				} else {
					advance(lexer)
					s.delimiters = s.delimiters[:len(s.delimiters)-1]
					lexer.ResultSymbol = stringEnd
					s.insideFString = false
				}
				lexer.MarkEnd()
				return true
			case lexer.Lookahead == '\n' && hasContent && !delimiter.isTriple():
				return false
			}
			advance(lexer)
			hasContent = true
		}
	}

	lexer.MarkEnd()

	foundEndOfLine := false
	var indentLength uint32
	firstCommentIndentLength := int32(-1)
loop:
	for {
		switch {
		case lexer.Lookahead == '\n':
			foundEndOfLine = true
			indentLength = 0
			skip(lexer)
		case lexer.Lookahead == ' ':
			indentLength++
			skip(lexer)
		case lexer.Lookahead == '\r' || lexer.Lookahead == '\f':
			indentLength = 0
			skip(lexer)
		case lexer.Lookahead == '\t':
			indentLength += 8
			skip(lexer)
		case lexer.Lookahead == '#' && (validSymbols[indent] || validSymbols[dedent] ||
			validSymbols[newline] || validSymbols[except]):
			// If we haven't found an EOL yet,
			// then this is a comment after an expression:
			//   foo = bar # comment
			// Just return, since we don't want to generate an indent/dedent
			// token.
			if !foundEndOfLine {
				return false
			}
			if firstCommentIndentLength == -1 {
				firstCommentIndentLength = int32(indentLength)
			}
			for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
				skip(lexer)
			}
			skip(lexer)
			indentLength = 0
		case lexer.Lookahead == '\\':
			skip(lexer)
			if lexer.Lookahead == '\r' {
				skip(lexer)
			}
			if lexer.Lookahead == '\n' || lexer.EOF() {
				skip(lexer)
			} else {
				return false
			}
		case lexer.EOF():
			indentLength = 0
			foundEndOfLine = true
			break loop
		default:
			break loop
		}
	}

	if foundEndOfLine {
		if len(s.indents) > 0 {
			currentIndentLength := s.indents[len(s.indents)-1]

			if validSymbols[indent] && indentLength > uint32(currentIndentLength) {
				s.indents = append(s.indents, uint16(indentLength))
				lexer.ResultSymbol = indent
				return true
			}

			nextTokIsStringStart := lexer.Lookahead == '"' || lexer.Lookahead == '\'' || lexer.Lookahead == '`'

			if (validSymbols[dedent] ||
				(!validSymbols[newline] && !(validSymbols[stringStart] && nextTokIsStringStart) &&
					!withinBrackets)) &&
				indentLength < uint32(currentIndentLength) && !s.insideFString &&

				// Wait to create a dedent token until we've consumed any
				// comments
				// whose indentation matches the current block.
				firstCommentIndentLength < int32(currentIndentLength) {
				s.indents = s.indents[:len(s.indents)-1]
				lexer.ResultSymbol = dedent
				return true
			}
		}

		if validSymbols[newline] && !errorRecoveryMode {
			lexer.ResultSymbol = newline
			return true
		}
	}

	if firstCommentIndentLength == -1 && validSymbols[stringStart] {
		delimiter := newDelimiter()

		hasFlags := false
	flags:
		for lexer.Lookahead != 0 {
			switch {
			case lexer.Lookahead == 'f' || lexer.Lookahead == 'F':
				delimiter.setFormat()
			case lexer.Lookahead == 'r' || lexer.Lookahead == 'R':
				delimiter.setRaw()
			case lexer.Lookahead == 'b' || lexer.Lookahead == 'B':
				delimiter.setBytes()
			case lexer.Lookahead != 'u' && lexer.Lookahead != 'U':
				break flags
			}
			hasFlags = true
			advance(lexer)
		}

		switch lexer.Lookahead {
		case '`':
			delimiter.setEndCharacter('`')
			advance(lexer)
			lexer.MarkEnd()
		case '\'':
			delimiter.setEndCharacter('\'')
			advance(lexer)
			lexer.MarkEnd()
			if lexer.Lookahead == '\'' {
				advance(lexer)
				if lexer.Lookahead == '\'' {
					advance(lexer)
					lexer.MarkEnd()
					delimiter.setTriple()
				}
			}
		case '"':
			delimiter.setEndCharacter('"')
			advance(lexer)
			lexer.MarkEnd()
			if lexer.Lookahead == '"' {
				advance(lexer)
				if lexer.Lookahead == '"' {
					advance(lexer)
					lexer.MarkEnd()
					delimiter.setTriple()
				}
			}
		}

		if delimiter.endCharacter() != 0 {
			s.delimiters = append(s.delimiters, delimiter)
			lexer.ResultSymbol = stringStart
			s.insideFString = delimiter.isFormat()
			return true
		}
		if hasFlags {
			return false
		}
	}

	return false
}

// Serialize is tree_sitter_python_external_scanner_serialize. It writes
// inside_f_string, the number of delimiters up to 255, the flags of each of
// those delimiters, and the low byte of each indent after the first, until
// the buffer is full.
func (s *scanner) Serialize(buffer []byte) int {
	size := 0

	buffer[size] = boolByte(s.insideFString)
	size++

	delimiterCount := min(len(s.delimiters), math.MaxUint8)
	buffer[size] = byte(delimiterCount)
	size++

	for i := range delimiterCount {
		buffer[size+i] = s.delimiters[i].flags
	}
	size += delimiterCount

	iter := 1
	for ; iter < len(s.indents) && size < abi.SerializationBufferSize; iter++ {
		buffer[size] = byte(s.indents[iter])
		size++
	}

	return size
}

// Deserialize is tree_sitter_python_external_scanner_deserialize.
//
// The C function has no test of the length after the first byte. It reads
// the number of delimiters and the delimiters even when the buffer ends
// before them, and so it reads memory past the end of the state. The Go
// function reads a byte past the end of the buffer as 0.
//
// The C function frees the arrays of the delimiters and the indents, and it
// allocates them again. The Go function keeps the memory of the slices and
// sets their length to 0, so that a parse does not allocate them for each
// token of the scanner.
func (s *scanner) Deserialize(buffer []byte) {
	s.delimiters = s.delimiters[:0]
	s.indents = s.indents[:0]
	s.indents = append(s.indents, 0)

	if len(buffer) > 0 {
		size := 0

		s.insideFString = buffer[size] != 0
		size++

		delimiterCount := int(byteAt(buffer, size))
		size++
		if delimiterCount > 0 {
			s.delimiters = slices.Grow(s.delimiters, delimiterCount)[:delimiterCount]
			for i := range delimiterCount {
				s.delimiters[i].flags = byteAt(buffer, size+i)
			}
			size += delimiterCount
		}

		for ; size < len(buffer); size++ {
			s.indents = append(s.indents, uint16(buffer[size]))
		}
	}
}

// newScanner is tree_sitter_python_external_scanner_create.
func newScanner() *scanner {
	s := &scanner{}
	s.Deserialize(nil)
	return s
}

// boolByte is the C conversion of a bool to a char.
func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// byteAt returns the byte of buffer at i, or 0 when i is past its end.
func byteAt(buffer []byte, i int) byte {
	if i < len(buffer) {
		return buffer[i]
	}
	return 0
}
