package ruby

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// This file ports src/scanner.c of tree-sitter-ruby. The C type char is
// signed on the platforms that CI tests, so where the C code keeps a
// character in a char, the Go code keeps a byte, and it compares the byte
// as an int8 where C compares the char with an int.

// tokenType is TokenType, the external tokens of the grammar, in the order of
// externals in grammar.json.
type tokenType uint8

const (
	lineBreak tokenType = iota
	noLineBreak

	// Delimited literals.
	simpleSymbol
	stringStart
	symbolStart
	subshellStart
	regexStart
	stringArrayStart
	symbolArrayStart
	heredocBodyStart
	stringContent
	heredocContent
	stringEnd
	heredocBodyEnd
	heredocStart

	// Whitespace-sensitive tokens.
	forwardSlash
	blockAmpersand
	splatStar
	unaryMinus
	unaryMinusNum
	binaryMinus
	binaryStar
	singletonClassLeftAngleLeftAngle
	hashKeySymbol
	identifierSuffix
	constantSuffix
	hashSplatStarStar
	binaryStarStar
	elementReferenceBracket
	shortInterpolation

	none
)

// tokenTypeNames are the C names of the token types.
var tokenTypeNames = [...]string{
	"LINE_BREAK",
	"NO_LINE_BREAK",
	"SIMPLE_SYMBOL",
	"STRING_START",
	"SYMBOL_START",
	"SUBSHELL_START",
	"REGEX_START",
	"STRING_ARRAY_START",
	"SYMBOL_ARRAY_START",
	"HEREDOC_BODY_START",
	"STRING_CONTENT",
	"HEREDOC_CONTENT",
	"STRING_END",
	"HEREDOC_BODY_END",
	"HEREDOC_START",
	"FORWARD_SLASH",
	"BLOCK_AMPERSAND",
	"SPLAT_STAR",
	"UNARY_MINUS",
	"UNARY_MINUS_NUM",
	"BINARY_MINUS",
	"BINARY_STAR",
	"SINGLETON_CLASS_LEFT_ANGLE_LEFT_ANGLE",
	"HASH_KEY_SYMBOL",
	"IDENTIFIER_SUFFIX",
	"CONSTANT_SUFFIX",
	"HASH_SPLAT_STAR_STAR",
	"BINARY_STAR_STAR",
	"ELEMENT_REFERENCE_BRACKET",
	"SHORT_INTERPOLATION",
	"NONE",
}

// String returns the C name of the token type.
func (t tokenType) String() string {
	if int(t) < len(tokenTypeNames) {
		return tokenTypeNames[t]
	}
	return "TokenType(" + strconv.Itoa(int(t)) + ")"
}

// literal is Literal, a delimited literal that is open.
type literal struct {
	typ                 tokenType
	openDelimiter       int32
	closeDelimiter      int32
	nestingDepth        int32
	allowsInterpolation bool
}

// heredoc is Heredoc, a heredoc that is open. word is the C String, an
// Array(char).
type heredoc struct {
	word                      []byte
	endWordIndentationAllowed bool
	allowsInterpolation       bool
	started                   bool
}

// scanner is a port of src/scanner.c of tree-sitter-ruby at v0.23.1
// (71bd32fb7607035768799732addba884a37a6210). It is the C type Scanner.
type scanner struct {
	hasLeadingWhitespace bool
	literalStack         []literal
	openHeredocs         []heredoc
}

// nonIdentifierChars is NON_IDENTIFIER_CHARS.
var nonIdentifierChars = [...]byte{
	'\x00', '\n', '\r', '\t', ' ', ':', ';', '`', '"', '\'', '@', '$', '#', '.', ',', '|', '^', '&',
	'<', '=', '>', '+', '-', '*', '/', '\\', '%', '?', '!', '~', '(', ')', '[', ']', '{', '}',
}

// skip is skip.
func (s *scanner) skip(lexer *abi.Lexer) {
	s.hasLeadingWhitespace = true
	lexer.Advance(true)
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// reset is reset.
func (s *scanner) reset() {
	s.literalStack = s.literalStack[:0]
	s.openHeredocs = s.openHeredocs[:0]
}

// serialize is serialize. Each literal takes 5 bytes: its type, its open
// delimiter, its close delimiter and its nesting depth, each cut to a byte,
// and allows_interpolation. Each heredoc takes 4 bytes, the three booleans
// and the length of its word cut to a byte, and then the bytes of its word.
//
// The test of the room for a heredoc counts 2 bytes for the 4 that it writes.
// For a state that fills the buffer, such as one heredoc with a word of 1019
// bytes, C writes 1 byte past the end of the buffer, and Go panics. A word of
// more than 255 bytes gets its length cut to a byte, so deserialize reads a
// shorter word.
func (s *scanner) serialize(buffer []byte) int {
	size := 0

	if len(s.literalStack)*5+2 >= abi.SerializationBufferSize {
		return 0
	}

	buffer[size] = byte(len(s.literalStack))
	size++
	for i := range s.literalStack {
		literal := &s.literalStack[i]
		buffer[size] = byte(literal.typ)
		buffer[size+1] = byte(literal.openDelimiter)
		buffer[size+2] = byte(literal.closeDelimiter)
		buffer[size+3] = byte(literal.nestingDepth)
		buffer[size+4] = boolByte(literal.allowsInterpolation)
		size += 5
	}

	buffer[size] = byte(len(s.openHeredocs))
	size++
	for i := range s.openHeredocs {
		heredoc := &s.openHeredocs[i]
		if size+2+len(heredoc.word) >= abi.SerializationBufferSize {
			return 0
		}
		buffer[size] = boolByte(heredoc.endWordIndentationAllowed)
		buffer[size+1] = boolByte(heredoc.allowsInterpolation)
		buffer[size+2] = boolByte(heredoc.started)
		buffer[size+3] = byte(len(heredoc.word))
		size += 4
		copy(buffer[size:size+len(heredoc.word)], heredoc.word)
		size += len(heredoc.word)
	}

	return size
}

// boolByte is the C conversion of a bool to a char.
func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// deserialize is deserialize.
//
// The C function tests no length, and it ends with assert(size == length).
// The Go function stops when the next byte is past the end of the buffer,
// and it ignores the bytes after the state, so that no buffer makes it panic.
// For the bytes that serialize writes, the two functions do the same, but
// for a heredoc word of more than 255 bytes. There the assertion of C fails,
// and Go keeps the part of the word that the length gives, as C does
// without assertions.
func (s *scanner) deserialize(buffer []byte) {
	size := 0
	s.hasLeadingWhitespace = false
	s.reset()

	if len(buffer) == 0 {
		return
	}

	literalDepth := buffer[size]
	size++
	for range literalDepth {
		if size+5 > len(buffer) {
			return
		}
		literal := literal{
			// The C code converts the char to TokenType, and it compares and
			// serializes only the low byte.
			typ:                 tokenType(buffer[size]),
			openDelimiter:       int32(buffer[size+1]),
			closeDelimiter:      int32(buffer[size+2]),
			nestingDepth:        int32(buffer[size+3]),
			allowsInterpolation: buffer[size+4] != 0,
		}
		size += 5
		s.literalStack = append(s.literalStack, literal)
	}

	if size >= len(buffer) {
		return
	}
	openHeredocCount := buffer[size]
	size++
	for range openHeredocCount {
		if size+4 > len(buffer) {
			return
		}
		heredoc := heredoc{
			endWordIndentationAllowed: buffer[size] != 0,
			allowsInterpolation:       buffer[size+1] != 0,
			started:                   buffer[size+2] != 0,
		}
		size += 3

		wordLength := int(buffer[size])
		size++
		if size+wordLength > len(buffer) {
			return
		}
		heredoc.word = slices.Clone(buffer[size : size+wordLength])
		size += wordLength
		s.openHeredocs = append(s.openHeredocs, heredoc)
	}
}

// scanWhitespace is scan_whitespace.
func (s *scanner) scanWhitespace(lexer *abi.Lexer, validSymbols []bool) bool {
	heredocBodyStartIsValid := len(s.openHeredocs) > 0 && !s.openHeredocs[0].started &&
		validSymbols[heredocBodyStart]
	crossedNewline := false

	for {
		if !validSymbols[noLineBreak] && validSymbols[lineBreak] && lexer.IsAtIncludedRangeStart() {
			lexer.MarkEnd()
			lexer.ResultSymbol = uint16(lineBreak)
			return true
		}

		switch lexer.Lookahead {
		case ' ', '\t':
			s.skip(lexer)
		case '\r':
			if heredocBodyStartIsValid {
				lexer.ResultSymbol = uint16(heredocBodyStart)
				s.openHeredocs[0].started = true
				return true
			}
			s.skip(lexer)
		case '\n':
			switch {
			case heredocBodyStartIsValid:
				lexer.ResultSymbol = uint16(heredocBodyStart)
				s.openHeredocs[0].started = true
				return true
			case !validSymbols[noLineBreak] && validSymbols[lineBreak] && !crossedNewline:
				lexer.MarkEnd()
				advance(lexer)
				crossedNewline = true
			default:
				s.skip(lexer)
			}
		case '\\':
			advance(lexer)
			if lexer.Lookahead == '\r' {
				s.skip(lexer)
			}
			if !wctype.Iswspace(lexer.Lookahead) {
				return false
			}
			s.skip(lexer)
		default:
			if crossedNewline {
				if lexer.Lookahead != '.' && lexer.Lookahead != '&' && lexer.Lookahead != '#' {
					lexer.ResultSymbol = uint16(lineBreak)
				} else if lexer.Lookahead == '.' {
					// Don't return LINE_BREAK for the call operator (`.`) but do return one for range
					// operators
					// (`..` and `...`)
					advance(lexer)
					if !lexer.EOF() && lexer.Lookahead == '.' {
						lexer.ResultSymbol = uint16(lineBreak)
					} else {
						return false
					}
				}
			}
			return true
		}
	}
}

// scanOperator is scan_operator.
func scanOperator(lexer *abi.Lexer) bool {
	switch lexer.Lookahead {
	// <, <=, <<, <=>
	case '<':
		advance(lexer)
		switch lexer.Lookahead {
		case '<':
			advance(lexer)
		case '=':
			advance(lexer)
			if lexer.Lookahead == '>' {
				advance(lexer)
			}
		}
		return true

	// >, >=, >>
	case '>':
		advance(lexer)
		if lexer.Lookahead == '>' || lexer.Lookahead == '=' {
			advance(lexer)
		}
		return true

	// ==, ===, =~
	case '=':
		advance(lexer)
		if lexer.Lookahead == '~' {
			advance(lexer)
			return true
		}
		if lexer.Lookahead == '=' {
			advance(lexer)
			if lexer.Lookahead == '=' {
				advance(lexer)
			}
			return true
		}
		return false

	// +, -, ~, +@, -@, ~@
	case '+', '-', '~':
		advance(lexer)
		if lexer.Lookahead == '@' {
			advance(lexer)
		}
		return true

	// ..
	case '.':
		advance(lexer)
		if lexer.Lookahead == '.' {
			advance(lexer)
			return true
		}
		return false

	// &, ^, |, /, %`
	case '&', '^', '|', '/', '%', '`':
		advance(lexer)
		return true

	// !, !=, !~
	case '!':
		advance(lexer)
		if lexer.Lookahead == '=' || lexer.Lookahead == '~' {
			advance(lexer)
		}
		return true

	// *, **
	case '*':
		advance(lexer)
		if lexer.Lookahead == '*' {
			advance(lexer)
		}
		return true

	// [], []=
	case '[':
		advance(lexer)
		if lexer.Lookahead != ']' {
			return false
		}
		advance(lexer)
		if lexer.Lookahead == '=' {
			advance(lexer)
		}
		return true

	default:
		return false
	}
}

// isIdenChar is is_iden_char. The caller cuts the lookahead to a byte, as the
// C code casts it to a char.
func isIdenChar(c byte) bool {
	return bytes.IndexByte(nonIdentifierChars[:], c) < 0
}

// scanSymbolIdentifier is scan_symbol_identifier.
func scanSymbolIdentifier(lexer *abi.Lexer) bool {
	switch lexer.Lookahead {
	case '@':
		advance(lexer)
		if lexer.Lookahead == '@' {
			advance(lexer)
		}
	case '$':
		advance(lexer)
	}

	if isIdenChar(byte(lexer.Lookahead)) {
		advance(lexer)
	} else if !scanOperator(lexer) {
		return false
	}

	for isIdenChar(byte(lexer.Lookahead)) {
		advance(lexer)
	}

	if lexer.Lookahead == '?' || lexer.Lookahead == '!' {
		advance(lexer)
	}

	if lexer.Lookahead == '=' {
		lexer.MarkEnd()
		advance(lexer)
		if lexer.Lookahead != '>' {
			lexer.MarkEnd()
		}
	}

	return true
}

// scanOpenDelimiter is scan_open_delimiter.
func (s *scanner) scanOpenDelimiter(lexer *abi.Lexer, literal *literal, validSymbols []bool) bool {
	switch lexer.Lookahead {
	case '"':
		literal.typ = stringStart
		literal.openDelimiter = lexer.Lookahead
		literal.closeDelimiter = lexer.Lookahead
		literal.allowsInterpolation = true
		advance(lexer)
		return true

	case '\'':
		literal.typ = stringStart
		literal.openDelimiter = lexer.Lookahead
		literal.closeDelimiter = lexer.Lookahead
		literal.allowsInterpolation = false
		advance(lexer)
		return true

	case '`':
		if !validSymbols[subshellStart] {
			return false
		}
		literal.typ = subshellStart
		literal.openDelimiter = lexer.Lookahead
		literal.closeDelimiter = lexer.Lookahead
		literal.allowsInterpolation = true
		advance(lexer)
		return true

	case '/':
		if !validSymbols[regexStart] {
			return false
		}
		literal.typ = regexStart
		literal.openDelimiter = lexer.Lookahead
		literal.closeDelimiter = lexer.Lookahead
		literal.allowsInterpolation = true
		advance(lexer)
		if validSymbols[forwardSlash] {
			if !s.hasLeadingWhitespace {
				return false
			}
			if lexer.Lookahead == ' ' || lexer.Lookahead == '\t' || lexer.Lookahead == '\n' ||
				lexer.Lookahead == '\r' {
				return false
			}
			if lexer.Lookahead == '=' {
				return false
			}
		}
		return true

	case '%':
		advance(lexer)

		switch lexer.Lookahead {
		case 's':
			if !validSymbols[simpleSymbol] {
				return false
			}
			literal.typ = symbolStart
			literal.allowsInterpolation = false
			advance(lexer)

		case 'r':
			if !validSymbols[regexStart] {
				return false
			}
			literal.typ = regexStart
			literal.allowsInterpolation = true
			advance(lexer)

		case 'x':
			if !validSymbols[subshellStart] {
				return false
			}
			literal.typ = subshellStart
			literal.allowsInterpolation = true
			advance(lexer)

		case 'q':
			if !validSymbols[stringStart] {
				return false
			}
			literal.typ = stringStart
			literal.allowsInterpolation = false
			advance(lexer)

		case 'Q':
			if !validSymbols[stringStart] {
				return false
			}
			literal.typ = stringStart
			literal.allowsInterpolation = true
			advance(lexer)

		case 'w':
			if !validSymbols[stringArrayStart] {
				return false
			}
			literal.typ = stringArrayStart
			literal.allowsInterpolation = false
			advance(lexer)

		case 'i':
			if !validSymbols[symbolArrayStart] {
				return false
			}
			literal.typ = symbolArrayStart
			literal.allowsInterpolation = false
			advance(lexer)

		case 'W':
			if !validSymbols[stringArrayStart] {
				return false
			}
			literal.typ = stringArrayStart
			literal.allowsInterpolation = true
			advance(lexer)

		case 'I':
			if !validSymbols[symbolArrayStart] {
				return false
			}
			literal.typ = symbolArrayStart
			literal.allowsInterpolation = true
			advance(lexer)

		default:
			if !validSymbols[stringStart] {
				return false
			}
			literal.typ = stringStart
			literal.allowsInterpolation = true
		}

		switch lexer.Lookahead {
		case '(':
			literal.openDelimiter = '('
			literal.closeDelimiter = ')'

		case '[':
			literal.openDelimiter = '['
			literal.closeDelimiter = ']'

		case '{':
			literal.openDelimiter = '{'
			literal.closeDelimiter = '}'

		case '<':
			literal.openDelimiter = '<'
			literal.closeDelimiter = '>'

		case '\r', '\n', ' ', '\t':
			// If the `/` operator is valid, then so is the `%` operator, which means
			// that a `%` followed by whitespace should be considered an operator,
			// not a percent string.
			if validSymbols[forwardSlash] {
				return false
			}

		case '|', '!', '#', '/', '\\', '@', '$', '%', '^', '&', '*', ')', ']', '}', '>',
			// TODO: Implement %= as external rule and re-enable = as a valid
			// unbalanced delimiter. That will be necessary due to ambiguity
			// between &= assignment operator and %=...= as string
			// content delimiter.
			// case '=':
			'+', '-', '~', '`', ',', '.', '?', ':', ';', '_', '"', '\'':
			literal.openDelimiter = lexer.Lookahead
			literal.closeDelimiter = lexer.Lookahead
		default:
			return false
		}

		advance(lexer)
		return true

	default:
		return false
	}
}

// scanHeredocWord is scan_heredoc_word. The word keeps each character cut to
// a byte, as the C code pushes it to an Array(char).
func scanHeredocWord(lexer *abi.Lexer, heredoc *heredoc) {
	var word []byte
	quote := int32(0)

	switch lexer.Lookahead {
	case '\'', '"', '`':
		quote = lexer.Lookahead
		advance(lexer)
		for lexer.Lookahead != quote && !lexer.EOF() {
			word = append(word, byte(lexer.Lookahead))
			advance(lexer)
		}
		advance(lexer)

	default:
		if wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '_' {
			word = append(word, byte(lexer.Lookahead))
			advance(lexer)
			for wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '_' {
				word = append(word, byte(lexer.Lookahead))
				advance(lexer)
			}
		}
	}

	heredoc.word = word
	heredoc.allowsInterpolation = quote != '\''
}

// shortInterpolationChars is the string that scan_short_interpolation passes
// to strchr.
const shortInterpolationChars = "!@&`'+~=/\\,;.<>*$?:\""

// scanShortInterpolation is scan_short_interpolation. The C code cuts the
// lookahead to a char for start and for strchr. strchr also finds the 0
// byte at the end of its string, so a lookahead whose low byte is 0, such
// as the 0 at the end of the input, counts as a short interpolation.
func scanShortInterpolation(lexer *abi.Lexer, hasContent bool, contentSymbol tokenType) bool {
	start := byte(lexer.Lookahead)
	if start == '@' || start == '$' {
		if hasContent {
			lexer.ResultSymbol = uint16(contentSymbol)
			return true
		}
		lexer.MarkEnd()
		advance(lexer)
		isShortInterpolation := false
		if start == '$' {
			if c := byte(lexer.Lookahead); c == 0 || strings.IndexByte(shortInterpolationChars, c) >= 0 {
				isShortInterpolation = true
			} else {
				if lexer.Lookahead == '-' {
					advance(lexer)
					isShortInterpolation = wctype.Iswalpha(lexer.Lookahead) || lexer.Lookahead == '_'
				} else {
					isShortInterpolation = wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '_'
				}
			}
		}
		if start == '@' {
			if lexer.Lookahead == '@' {
				advance(lexer)
			}
			isShortInterpolation = isIdenChar(byte(lexer.Lookahead)) && !wctype.Iswdigit(lexer.Lookahead)
		}

		if isShortInterpolation {
			lexer.ResultSymbol = uint16(shortInterpolation)
			return true
		}
	}
	return false
}

// scanHeredocContent is scan_heredoc_content.
func (s *scanner) scanHeredocContent(lexer *abi.Lexer) bool {
	heredoc := &s.openHeredocs[0]
	positionInWord := 0
	lookForHeredocEnd := true
	hasContent := false

	for {
		if positionInWord == len(heredoc.word) {
			if !hasContent {
				lexer.MarkEnd()
			}
			for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' {
				advance(lexer)
			}
			if lexer.Lookahead == '\n' || lexer.Lookahead == '\r' {
				if hasContent {
					lexer.ResultSymbol = uint16(heredocContent)
				} else {
					s.openHeredocs = slices.Delete(s.openHeredocs, 0, 1)
					lexer.ResultSymbol = uint16(heredocBodyEnd)
				}
				return true
			}
			hasContent = true
			positionInWord = 0
		}

		if lexer.EOF() {
			lexer.MarkEnd()
			if hasContent {
				lexer.ResultSymbol = uint16(heredocContent)
			} else {
				s.openHeredocs = slices.Delete(s.openHeredocs, 0, 1)
				lexer.ResultSymbol = uint16(heredocBodyEnd)
			}
			return true
		}

		// The C code compares the lookahead with a char, which is signed.
		if lexer.Lookahead == int32(int8(heredoc.word[positionInWord])) && lookForHeredocEnd {
			advance(lexer)
			positionInWord++
		} else {
			positionInWord = 0
			lookForHeredocEnd = false

			if heredoc.allowsInterpolation && lexer.Lookahead == '\\' {
				if hasContent {
					lexer.ResultSymbol = uint16(heredocContent)
					return true
				}
				return false
			}

			switch {
			case heredoc.allowsInterpolation && lexer.Lookahead == '#':
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '{' {
					if hasContent {
						lexer.ResultSymbol = uint16(heredocContent)
						return true
					}
					return false
				}
				if scanShortInterpolation(lexer, hasContent, heredocContent) {
					return true
				}
			case lexer.Lookahead == '\r' || lexer.Lookahead == '\n':
				if lexer.Lookahead == '\r' {
					advance(lexer)
					if lexer.Lookahead == '\n' {
						advance(lexer)
					}
				} else {
					advance(lexer)
				}
				hasContent = true
				lookForHeredocEnd = true
				for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' {
					advance(lexer)
					if !heredoc.endWordIndentationAllowed {
						lookForHeredocEnd = false
					}
				}
				lexer.MarkEnd()
			default:
				hasContent = true
				advance(lexer)
				lexer.MarkEnd()
			}
		}
	}
}

// scanLiteralContent is scan_literal_content.
func (s *scanner) scanLiteralContent(lexer *abi.Lexer) bool {
	literal := &s.literalStack[len(s.literalStack)-1]
	hasContent := false
	stopOnSpace := literal.typ == symbolArrayStart || literal.typ == stringArrayStart

	for {
		if stopOnSpace && wctype.Iswspace(lexer.Lookahead) {
			if hasContent {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(stringContent)
				return true
			}
			return false
		}
		switch {
		case lexer.Lookahead == literal.closeDelimiter:
			lexer.MarkEnd()
			if literal.nestingDepth == 1 {
				if hasContent {
					lexer.ResultSymbol = uint16(stringContent)
				} else {
					advance(lexer)
					if literal.typ == regexStart {
						for wctype.Iswlower(lexer.Lookahead) {
							advance(lexer)
						}
					}
					s.literalStack = s.literalStack[:len(s.literalStack)-1]
					lexer.ResultSymbol = uint16(stringEnd)
					lexer.MarkEnd()
				}
				return true
			}
			literal.nestingDepth--
			advance(lexer)

		case lexer.Lookahead == literal.openDelimiter:
			literal.nestingDepth++
			advance(lexer)
		case literal.allowsInterpolation && lexer.Lookahead == '#':
			lexer.MarkEnd()
			advance(lexer)
			if lexer.Lookahead == '{' {
				if hasContent {
					lexer.ResultSymbol = uint16(stringContent)
					return true
				}
				return false
			}
			if scanShortInterpolation(lexer, hasContent, stringContent) {
				return true
			}
		case lexer.Lookahead == '\\':
			if literal.allowsInterpolation {
				if hasContent {
					lexer.MarkEnd()
					lexer.ResultSymbol = uint16(stringContent)
					return true
				}
				return false
			}
			advance(lexer)
			advance(lexer)

		case lexer.EOF():
			advance(lexer)
			lexer.MarkEnd()
			return false
		default:
			advance(lexer)
		}

		hasContent = true
	}
}

// scan is scan.
func (s *scanner) scan(lexer *abi.Lexer, validSymbols []bool) bool {
	s.hasLeadingWhitespace = false

	// Contents of literals, which match any character except for some close delimiter
	if !validSymbols[stringStart] {
		if (validSymbols[stringContent] || validSymbols[stringEnd]) && len(s.literalStack) > 0 {
			return s.scanLiteralContent(lexer)
		}
		if (validSymbols[heredocContent] || validSymbols[heredocBodyEnd]) && len(s.openHeredocs) > 0 {
			return s.scanHeredocContent(lexer)
		}
	}

	// Whitespace
	lexer.ResultSymbol = uint16(none)
	if !s.scanWhitespace(lexer, validSymbols) {
		return false
	}
	if lexer.ResultSymbol != uint16(none) {
		return true
	}

	switch lexer.Lookahead {
	case '&':
		if validSymbols[blockAmpersand] {
			advance(lexer)
			if lexer.Lookahead != '&' && lexer.Lookahead != '.' && lexer.Lookahead != '=' &&
				!wctype.Iswspace(lexer.Lookahead) {
				lexer.ResultSymbol = uint16(blockAmpersand)
				return true
			}
			return false
		}

	case '<':
		if validSymbols[singletonClassLeftAngleLeftAngle] {
			advance(lexer)
			if lexer.Lookahead == '<' {
				advance(lexer)
				lexer.ResultSymbol = uint16(singletonClassLeftAngleLeftAngle)
				return true
			}
			return false
		}

	case '*':
		if validSymbols[splatStar] || validSymbols[binaryStar] || validSymbols[hashSplatStarStar] ||
			validSymbols[binaryStarStar] {
			advance(lexer)
			if lexer.Lookahead == '=' {
				return false
			}
			if lexer.Lookahead == '*' {
				if validSymbols[hashSplatStarStar] || validSymbols[binaryStarStar] {
					advance(lexer)
					if lexer.Lookahead == '=' {
						return false
					}
					if validSymbols[binaryStarStar] && !s.hasLeadingWhitespace {
						lexer.ResultSymbol = uint16(binaryStarStar)
						return true
					}
					if validSymbols[hashSplatStarStar] && !wctype.Iswspace(lexer.Lookahead) {
						lexer.ResultSymbol = uint16(hashSplatStarStar)
						return true
					}
					if validSymbols[binaryStarStar] {
						lexer.ResultSymbol = uint16(binaryStarStar)
						return true
					}
					if validSymbols[hashSplatStarStar] {
						lexer.ResultSymbol = uint16(hashSplatStarStar)
						return true
					}
					return false
				}
				return false
			}
			if validSymbols[binaryStar] && !s.hasLeadingWhitespace {
				lexer.ResultSymbol = uint16(binaryStar)
				return true
			}
			if validSymbols[splatStar] && !wctype.Iswspace(lexer.Lookahead) {
				lexer.ResultSymbol = uint16(splatStar)
				return true
			}
			if validSymbols[binaryStar] {
				lexer.ResultSymbol = uint16(binaryStar)
				return true
			}
			if validSymbols[splatStar] {
				lexer.ResultSymbol = uint16(splatStar)
				return true
			}
			return false
		}

	case '-':
		if validSymbols[unaryMinus] || validSymbols[unaryMinusNum] || validSymbols[binaryMinus] {
			advance(lexer)
			if lexer.Lookahead != '=' && lexer.Lookahead != '>' {
				if validSymbols[unaryMinusNum] &&
					(!validSymbols[binaryStar] || s.hasLeadingWhitespace) &&
					wctype.Iswdigit(lexer.Lookahead) {
					lexer.ResultSymbol = uint16(unaryMinusNum)
					return true
				}
				switch {
				case validSymbols[unaryMinus] && s.hasLeadingWhitespace && !wctype.Iswspace(lexer.Lookahead):
					lexer.ResultSymbol = uint16(unaryMinus)
				case validSymbols[binaryMinus]:
					lexer.ResultSymbol = uint16(binaryMinus)
				default:
					lexer.ResultSymbol = uint16(unaryMinus)
				}
				return true
			}
			return false
		}

	case ':':
		if validSymbols[symbolStart] {
			literal := literal{typ: symbolStart, nestingDepth: 1}
			advance(lexer)

			switch lexer.Lookahead {
			case '"':
				advance(lexer)
				literal.openDelimiter = '"'
				literal.closeDelimiter = '"'
				literal.allowsInterpolation = true
				s.literalStack = append(s.literalStack, literal)
				lexer.ResultSymbol = uint16(symbolStart)
				return true

			case '\'':
				advance(lexer)
				literal.openDelimiter = '\''
				literal.closeDelimiter = '\''
				literal.allowsInterpolation = false
				s.literalStack = append(s.literalStack, literal)
				lexer.ResultSymbol = uint16(symbolStart)
				return true

			default:
				if scanSymbolIdentifier(lexer) {
					lexer.ResultSymbol = uint16(simpleSymbol)
					return true
				}
			}

			return false
		}

	case '[':
		// Treat a square bracket as an element reference if either:
		// * the bracket is not preceded by any whitespace
		// * an arbitrary expression is not valid at the current position.
		if validSymbols[elementReferenceBracket] &&
			(!s.hasLeadingWhitespace || !validSymbols[stringStart]) {
			advance(lexer)
			lexer.ResultSymbol = uint16(elementReferenceBracket)
			return true
		}
	}

	// Open delimiters for literals
	if ((validSymbols[hashKeySymbol] || validSymbols[identifierSuffix]) &&
		(wctype.Iswalpha(lexer.Lookahead) || lexer.Lookahead == '_')) ||
		(validSymbols[constantSuffix] && wctype.Iswupper(lexer.Lookahead)) {
		validIdentifierSymbol := identifierSuffix
		if wctype.Iswupper(lexer.Lookahead) {
			validIdentifierSymbol = constantSuffix
		}
		for wctype.Iswalnum(lexer.Lookahead) || lexer.Lookahead == '_' {
			advance(lexer)
		}

		if validSymbols[hashKeySymbol] && lexer.Lookahead == ':' {
			lexer.MarkEnd()
			advance(lexer)
			if lexer.Lookahead != ':' {
				lexer.ResultSymbol = uint16(hashKeySymbol)
				return true
			}
		} else if validSymbols[validIdentifierSymbol] && lexer.Lookahead == '!' {
			advance(lexer)
			if lexer.Lookahead != '=' {
				lexer.ResultSymbol = uint16(validIdentifierSymbol)
				return true
			}
		}

		return false
	}

	// Open delimiters for literals
	if validSymbols[stringStart] {
		literal := literal{nestingDepth: 1}

		if lexer.Lookahead == '<' {
			advance(lexer)
			if lexer.Lookahead != '<' {
				return false
			}
			advance(lexer)

			var heredoc heredoc
			if lexer.Lookahead == '-' || lexer.Lookahead == '~' {
				advance(lexer)
				heredoc.endWordIndentationAllowed = true
			}

			scanHeredocWord(lexer, &heredoc)
			if len(heredoc.word) == 0 {
				return false
			}
			s.openHeredocs = append(s.openHeredocs, heredoc)
			lexer.ResultSymbol = uint16(heredocStart)
			return true
		}

		if s.scanOpenDelimiter(lexer, &literal, validSymbols) {
			s.literalStack = append(s.literalStack, literal)
			lexer.ResultSymbol = uint16(literal.typ)
			return true
		}
		return false
	}

	return false
}

// newScanner is tree_sitter_ruby_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Scan is tree_sitter_ruby_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return s.scan(lexer, validSymbols)
}

// Serialize is tree_sitter_ruby_external_scanner_serialize.
func (s *scanner) Serialize(buf []byte) int {
	return s.serialize(buf)
}

// Deserialize is tree_sitter_ruby_external_scanner_deserialize.
func (s *scanner) Deserialize(buf []byte) {
	s.deserialize(buf)
}
