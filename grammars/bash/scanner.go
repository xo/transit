package bash

import (
	"bytes"
	"encoding/binary"

	"github.com/xo/transit/internal/abi"
	"github.com/xo/transit/internal/wctype"
)

// tokenType is enum TokenType, the external tokens of the grammar, in the
// order of externals in grammar.json.
type tokenType uint16

// The external tokens of the grammar.
const (
	heredocStart tokenType = iota
	simpleHeredocBody
	heredocBodyBeginning
	heredocContent
	heredocEnd
	fileDescriptor
	emptyValue
	concat
	variableName
	testOperator
	regex
	regexNoSlash
	regexNoSpace
	expansionWord
	extglobPattern
	bareDollar
	braceStart
	immediateDoubleHash
	externalExpansionSymHash
	externalExpansionSymBang
	externalExpansionSymEqual
	closingBrace
	closingBracket
	heredocArrow
	heredocArrowDash
	newline
	openingParen
	esac
	errorRecovery
)

// heredoc is Heredoc. A String of the C code, an Array(char), is a byte
// slice. The C code pushes a character of the lexer into it as a char, so
// the slice holds the low 8 bits of the character.
type heredoc struct {
	isRaw              bool
	started            bool
	allowsIndent       bool
	delimiter          []byte
	currentLeadingWord []byte
}

// scanner is a port of src/scanner.c of tree-sitter-bash at v0.25.0
// (56b54c61fb48bce0c63e3dfa2240b5d274384763). It is the C struct Scanner.
//
// A char of the C code is signed on the platforms that CI tests, and the Go
// code follows that where the C code widens a char to an int32. On a
// platform where a char is unsigned, the C code gives other results for a
// delimiter of a heredoc that is not ASCII.
type scanner struct {
	lastGlobParenDepth  uint8
	extWasInDoubleQuote bool
	extSawOutsideQuote  bool
	heredocs            []heredoc
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// inErrorRecovery is in_error_recovery.
func inErrorRecovery(validSymbols []bool) bool { return validSymbols[errorRecovery] }

// resetString is reset_string.
func resetString(s *[]byte) {
	if len(*s) > 0 {
		clear(*s)
		*s = (*s)[:0]
	}
}

// resetHeredoc is reset_heredoc.
func resetHeredoc(h *heredoc) {
	h.isRaw = false
	h.started = false
	h.allowsIndent = false
	resetString(&h.delimiter)
}

// reset is reset.
func (s *scanner) reset() {
	for i := range s.heredocs {
		resetHeredoc(&s.heredocs[i])
	}
}

// boolByte is the char of a bool: 1 for true and 0 for false.
func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// serialize is serialize. It writes last_glob_paren_depth,
// ext_was_in_double_quote, ext_saw_outside_quote and the number of heredocs
// as one byte each. For each heredoc, it writes is_raw, started and
// allows_indent as one byte each, the size of the delimiter as a uint32 in
// little-endian order, and the bytes of the delimiter.
func (s *scanner) serialize(buffer []byte) uint32 {
	size := uint32(0)

	buffer[size] = s.lastGlobParenDepth
	size++
	buffer[size] = boolByte(s.extWasInDoubleQuote)
	size++
	buffer[size] = boolByte(s.extSawOutsideQuote)
	size++
	buffer[size] = byte(len(s.heredocs))
	size++

	for i := range s.heredocs {
		h := &s.heredocs[i]
		if int(size)+3+4+len(h.delimiter) >= abi.SerializationBufferSize {
			return 0
		}

		buffer[size] = boolByte(h.isRaw)
		size++
		buffer[size] = boolByte(h.started)
		size++
		buffer[size] = boolByte(h.allowsIndent)
		size++

		binary.LittleEndian.PutUint32(buffer[size:], uint32(len(h.delimiter)))
		size += 4
		if len(h.delimiter) > 0 {
			copy(buffer[size:], h.delimiter)
			size += uint32(len(h.delimiter))
		}
	}
	return size
}

// deserialize is deserialize.
//
// The C function does not check the length of the buffer. It reads past the
// end of a buffer that is too short, and it asserts at the end that it read
// the whole buffer. The runtime gives it only the bytes that serialize wrote,
// so it reads each byte of the buffer and no more. For a buffer that is too
// short, the Go function stops at the first field that is not in the
// buffer. It does not check that it read the whole buffer.
func (s *scanner) deserialize(buffer []byte) {
	if len(buffer) == 0 {
		s.reset()
	} else {
		size := 0
		if len(buffer) < 4 {
			return
		}
		s.lastGlobParenDepth = buffer[size]
		size++
		s.extWasInDoubleQuote = buffer[size] != 0
		size++
		s.extSawOutsideQuote = buffer[size] != 0
		size++
		heredocCount := int(buffer[size])
		size++
		for i := range heredocCount {
			var h *heredoc
			if i < len(s.heredocs) {
				h = &s.heredocs[i]
			} else {
				s.heredocs = append(s.heredocs, heredoc{})
				h = &s.heredocs[len(s.heredocs)-1]
			}

			if len(buffer)-size < 3+4 {
				return
			}
			h.isRaw = buffer[size] != 0
			size++
			h.started = buffer[size] != 0
			size++
			h.allowsIndent = buffer[size] != 0
			size++

			delimiterSize := int(binary.LittleEndian.Uint32(buffer[size:]))
			size += 4
			if delimiterSize > len(buffer)-size {
				return
			}
			h.delimiter = h.delimiter[:0]

			if delimiterSize > 0 {
				h.delimiter = append(h.delimiter, buffer[size:size+delimiterSize]...)
				size += delimiterSize
			}
		}
	}
}

// advanceWord is advance_word.
//
// Consume a "word" in POSIX parlance, and returns it unquoted.
//
// This is an approximate implementation that doesn't deal with any
// POSIX-mandated substitution, and assumes the default value for
// IFS.
func advanceWord(lexer *abi.Lexer, unquotedWord *[]byte) bool {
	empty := true

	quote := int32(0)
	if lexer.Lookahead == '\'' || lexer.Lookahead == '"' {
		quote = lexer.Lookahead
		advance(lexer)
	}

	for lexer.Lookahead != 0 &&
		!(quote != 0 && (lexer.Lookahead == quote || lexer.Lookahead == '\r' || lexer.Lookahead == '\n') ||
			quote == 0 && wctype.Iswspace(lexer.Lookahead)) {
		if lexer.Lookahead == '\\' {
			advance(lexer)
			if lexer.Lookahead == 0 {
				return false
			}
		}
		empty = false
		*unquotedWord = append(*unquotedWord, byte(lexer.Lookahead))
		advance(lexer)
	}
	*unquotedWord = append(*unquotedWord, 0)

	if quote != 0 && lexer.Lookahead == quote {
		advance(lexer)
	}

	return !empty
}

// scanBareDollar is scan_bare_dollar.
func scanBareDollar(lexer *abi.Lexer) bool {
	for wctype.Iswspace(lexer.Lookahead) && lexer.Lookahead != '\n' && !lexer.EOF() {
		skip(lexer)
	}

	if lexer.Lookahead == '$' {
		advance(lexer)
		lexer.ResultSymbol = uint16(bareDollar)
		lexer.MarkEnd()
		return wctype.Iswspace(lexer.Lookahead) || lexer.EOF() || lexer.Lookahead == '"'
	}

	return false
}

// scanHeredocStart is scan_heredoc_start.
func scanHeredocStart(h *heredoc, lexer *abi.Lexer) bool {
	for wctype.Iswspace(lexer.Lookahead) {
		skip(lexer)
	}

	lexer.ResultSymbol = uint16(heredocStart)
	h.isRaw = lexer.Lookahead == '\'' || lexer.Lookahead == '"' || lexer.Lookahead == '\\'

	foundDelimiter := advanceWord(lexer, &h.delimiter)
	if !foundDelimiter {
		resetString(&h.delimiter)
		return false
	}
	return foundDelimiter
}

// cString returns the bytes of a C string, up to its first zero byte.
func cString(s []byte) []byte {
	before, _, _ := bytes.Cut(s, []byte{0})
	return before
}

// scanHeredocEndIdentifier is scan_heredoc_end_identifier.
//
// The C loop reads the character of the delimiter at size before it tests
// that the leading word is shorter than the delimiter. The size is the
// length of the leading word, so the Go loop tests the length first, and it
// never reads past the delimiter. The C function reads past it only when
// the delimiter has no zero byte, which the runtime never gives it.
func scanHeredocEndIdentifier(h *heredoc, lexer *abi.Lexer) bool {
	resetString(&h.currentLeadingWord)
	// Scan the first 'n' characters on this line, to see if they match the
	// heredoc delimiter
	size := int32(0)
	if len(h.delimiter) > 0 {
		for lexer.Lookahead != 0 && lexer.Lookahead != '\n' &&
			len(h.currentLeadingWord) < len(h.delimiter) &&
			int32(int8(h.delimiter[size])) == lexer.Lookahead {
			h.currentLeadingWord = append(h.currentLeadingWord, byte(lexer.Lookahead))
			advance(lexer)
			size++
		}
	}
	h.currentLeadingWord = append(h.currentLeadingWord, 0)
	if len(h.delimiter) == 0 {
		return false
	}
	return bytes.Equal(cString(h.currentLeadingWord), cString(h.delimiter))
}

// scanHeredocContent is scan_heredoc_content.
func (s *scanner) scanHeredocContent(lexer *abi.Lexer, middleType, endType tokenType) bool {
	didAdvance := false
	h := &s.heredocs[len(s.heredocs)-1]

	for {
		switch lexer.Lookahead {
		case 0:
			if lexer.EOF() && didAdvance {
				resetHeredoc(h)
				lexer.ResultSymbol = uint16(endType)
				return true
			}
			return false

		case '\\':
			didAdvance = true
			advance(lexer)
			advance(lexer)

		case '$':
			if h.isRaw {
				didAdvance = true
				advance(lexer)
				break
			}
			if didAdvance {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(middleType)
				h.started = true
				advance(lexer)
				if wctype.Iswalpha(lexer.Lookahead) || lexer.Lookahead == '{' || lexer.Lookahead == '(' {
					return true
				}
				break
			}
			if middleType == heredocBodyBeginning && lexer.GetColumn() == 0 {
				lexer.ResultSymbol = uint16(middleType)
				h.started = true
				return true
			}
			return false

		case '\n':
			if !didAdvance {
				skip(lexer)
			} else {
				advance(lexer)
			}
			didAdvance = true
			if h.allowsIndent {
				for wctype.Iswspace(lexer.Lookahead) {
					advance(lexer)
				}
			}
			if h.started {
				lexer.ResultSymbol = uint16(middleType)
			} else {
				lexer.ResultSymbol = uint16(endType)
			}
			lexer.MarkEnd()
			if scanHeredocEndIdentifier(h, lexer) {
				if lexer.ResultSymbol == uint16(heredocEnd) {
					s.heredocs = s.heredocs[:len(s.heredocs)-1]
				}
				return true
			}

		default:
			if lexer.GetColumn() == 0 {
				// an alternative is to check the starting column of the
				// heredoc body and track that statefully
				for wctype.Iswspace(lexer.Lookahead) {
					if didAdvance {
						advance(lexer)
					} else {
						skip(lexer)
					}
				}
				if endType != simpleHeredocBody {
					lexer.ResultSymbol = uint16(middleType)
					if scanHeredocEndIdentifier(h, lexer) {
						return true
					}
				}
				if endType == simpleHeredocBody {
					lexer.ResultSymbol = uint16(endType)
					lexer.MarkEnd()
					if scanHeredocEndIdentifier(h, lexer) {
						return true
					}
				}
			}
			didAdvance = true
			advance(lexer)
		}
	}
}

// scan is scan.
//
// The C function jumps forward with goto to four labels: regex,
// extglob_pattern, expansion_word and brace_start. Each label starts a
// part of the function that runs to its end. In Go, each part is a method
// of its own, and each part calls the next part at its end, as the C code
// runs on past the next label. A goto becomes a return of the call of the
// part of its label.
func (s *scanner) scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[concat] && !inErrorRecovery(validSymbols) {
		if !(lexer.Lookahead == 0 || wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == '>' ||
			lexer.Lookahead == '<' || lexer.Lookahead == ')' || lexer.Lookahead == '(' ||
			lexer.Lookahead == ';' || lexer.Lookahead == '&' || lexer.Lookahead == '|' ||
			(lexer.Lookahead == '}' && validSymbols[closingBrace]) ||
			(lexer.Lookahead == ']' && validSymbols[closingBracket])) {
			lexer.ResultSymbol = uint16(concat)
			// So for a`b`, we want to return a concat. We check if the
			// 2nd backtick has whitespace after it, and if it does we
			// return concat.
			if lexer.Lookahead == '`' {
				lexer.MarkEnd()
				advance(lexer)
				for lexer.Lookahead != '`' && !lexer.EOF() {
					advance(lexer)
				}
				if lexer.EOF() {
					return false
				}
				if lexer.Lookahead == '`' {
					advance(lexer)
				}
				return wctype.Iswspace(lexer.Lookahead) || lexer.EOF()
			}
			// strings w/ expansions that contains escaped quotes or
			// backslashes need this to return a concat
			if lexer.Lookahead == '\\' {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '"' || lexer.Lookahead == '\'' || lexer.Lookahead == '\\' {
					return true
				}
				if lexer.EOF() {
					return false
				}
			} else {
				return true
			}
		}
		if wctype.Iswspace(lexer.Lookahead) && validSymbols[closingBrace] && !validSymbols[expansionWord] {
			lexer.ResultSymbol = uint16(concat)
			return true
		}
	}

	if validSymbols[immediateDoubleHash] && !inErrorRecovery(validSymbols) {
		// advance two # and ensure not } after
		if lexer.Lookahead == '#' {
			lexer.MarkEnd()
			advance(lexer)
			if lexer.Lookahead == '#' {
				advance(lexer)
				if lexer.Lookahead != '}' {
					lexer.ResultSymbol = uint16(immediateDoubleHash)
					lexer.MarkEnd()
					return true
				}
			}
		}
	}

	if validSymbols[externalExpansionSymHash] && !inErrorRecovery(validSymbols) {
		if lexer.Lookahead == '#' || lexer.Lookahead == '=' || lexer.Lookahead == '!' {
			switch lexer.Lookahead {
			case '#':
				lexer.ResultSymbol = uint16(externalExpansionSymHash)
			case '!':
				lexer.ResultSymbol = uint16(externalExpansionSymBang)
			default:
				lexer.ResultSymbol = uint16(externalExpansionSymEqual)
			}
			advance(lexer)
			lexer.MarkEnd()
			for lexer.Lookahead == '#' || lexer.Lookahead == '=' || lexer.Lookahead == '!' {
				advance(lexer)
			}
			for wctype.Iswspace(lexer.Lookahead) {
				skip(lexer)
			}
			return lexer.Lookahead == '}'
		}
	}

	if validSymbols[emptyValue] {
		if wctype.Iswspace(lexer.Lookahead) || lexer.EOF() || lexer.Lookahead == ';' || lexer.Lookahead == '&' {
			lexer.ResultSymbol = uint16(emptyValue)
			return true
		}
	}

	if (validSymbols[heredocBodyBeginning] || validSymbols[simpleHeredocBody]) && len(s.heredocs) > 0 &&
		!s.heredocs[len(s.heredocs)-1].started && !inErrorRecovery(validSymbols) {
		return s.scanHeredocContent(lexer, heredocBodyBeginning, simpleHeredocBody)
	}

	if validSymbols[heredocEnd] && len(s.heredocs) > 0 {
		h := &s.heredocs[len(s.heredocs)-1]
		if scanHeredocEndIdentifier(h, lexer) {
			h.currentLeadingWord = nil
			h.delimiter = nil
			s.heredocs = s.heredocs[:len(s.heredocs)-1]
			lexer.ResultSymbol = uint16(heredocEnd)
			return true
		}
	}

	if validSymbols[heredocContent] && len(s.heredocs) > 0 && s.heredocs[len(s.heredocs)-1].started &&
		!inErrorRecovery(validSymbols) {
		return s.scanHeredocContent(lexer, heredocContent, heredocEnd)
	}

	if validSymbols[heredocStart] && !inErrorRecovery(validSymbols) && len(s.heredocs) > 0 {
		return scanHeredocStart(&s.heredocs[len(s.heredocs)-1], lexer)
	}

	if validSymbols[testOperator] && !validSymbols[expansionWord] {
		for wctype.Iswspace(lexer.Lookahead) && lexer.Lookahead != '\n' {
			skip(lexer)
		}

		if lexer.Lookahead == '\\' {
			if validSymbols[extglobPattern] {
				return s.scanExtglobPattern(lexer, validSymbols)
			}
			if validSymbols[regexNoSpace] {
				return s.scanRegex(lexer, validSymbols)
			}
			skip(lexer)

			if lexer.EOF() {
				return false
			}

			switch lexer.Lookahead {
			case '\r':
				skip(lexer)
				if lexer.Lookahead == '\n' {
					skip(lexer)
				}
			case '\n':
				skip(lexer)
			default:
				return false
			}

			for wctype.Iswspace(lexer.Lookahead) {
				skip(lexer)
			}
		}

		if lexer.Lookahead == '\n' && !validSymbols[newline] {
			skip(lexer)

			for wctype.Iswspace(lexer.Lookahead) {
				skip(lexer)
			}
		}

		if lexer.Lookahead == '-' {
			advance(lexer)

			advancedOnce := false
			for wctype.Iswalpha(lexer.Lookahead) {
				advancedOnce = true
				advance(lexer)
			}

			if wctype.Iswspace(lexer.Lookahead) && advancedOnce {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '}' && validSymbols[closingBrace] {
					if validSymbols[expansionWord] {
						lexer.MarkEnd()
						lexer.ResultSymbol = uint16(expansionWord)
						return true
					}
					return false
				}
				lexer.ResultSymbol = uint16(testOperator)
				return true
			}
			if wctype.Iswspace(lexer.Lookahead) && validSymbols[extglobPattern] {
				lexer.ResultSymbol = uint16(extglobPattern)
				return true
			}
		}

		if validSymbols[bareDollar] && !inErrorRecovery(validSymbols) && scanBareDollar(lexer) {
			return true
		}
	}

	if (validSymbols[variableName] || validSymbols[fileDescriptor] || validSymbols[heredocArrow]) &&
		!validSymbols[regexNoSlash] && !inErrorRecovery(validSymbols) {
	skipBlanks:
		for {
			switch {
			case (lexer.Lookahead == ' ' || lexer.Lookahead == '\t' || lexer.Lookahead == '\r' ||
				(lexer.Lookahead == '\n' && !validSymbols[newline])) &&
				!validSymbols[expansionWord]:
				skip(lexer)
			case lexer.Lookahead == '\\':
				skip(lexer)

				if lexer.EOF() {
					lexer.MarkEnd()
					lexer.ResultSymbol = uint16(variableName)
					return true
				}

				if lexer.Lookahead == '\r' {
					skip(lexer)
				}
				if lexer.Lookahead == '\n' {
					skip(lexer)
				} else {
					if lexer.Lookahead == '\\' && validSymbols[expansionWord] {
						return s.scanExpansionWord(lexer, validSymbols)
					}
					return false
				}
			default:
				break skipBlanks
			}
		}

		// no '*', '@', '?', '-', '$', '0', '_'
		if !validSymbols[expansionWord] &&
			(lexer.Lookahead == '*' || lexer.Lookahead == '@' || lexer.Lookahead == '?' || lexer.Lookahead == '-' ||
				lexer.Lookahead == '0' || lexer.Lookahead == '_') {
			lexer.MarkEnd()
			advance(lexer)
			if lexer.Lookahead == '=' || lexer.Lookahead == '[' || lexer.Lookahead == ':' ||
				lexer.Lookahead == '-' || lexer.Lookahead == '%' || lexer.Lookahead == '#' ||
				lexer.Lookahead == '/' {
				return false
			}
			if validSymbols[extglobPattern] && wctype.Iswspace(lexer.Lookahead) {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(extglobPattern)
				return true
			}
		}

		if validSymbols[heredocArrow] && lexer.Lookahead == '<' {
			advance(lexer)
			if lexer.Lookahead == '<' {
				advance(lexer)
				switch lexer.Lookahead {
				case '-':
					advance(lexer)
					h := heredoc{}
					h.allowsIndent = true
					s.heredocs = append(s.heredocs, h)
					lexer.ResultSymbol = uint16(heredocArrowDash)
				case '<', '=':
					return false
				default:
					h := heredoc{}
					s.heredocs = append(s.heredocs, h)
					lexer.ResultSymbol = uint16(heredocArrow)
				}
				return true
			}
			return false
		}

		isNumber := true
		switch {
		case wctype.Iswdigit(lexer.Lookahead):
			advance(lexer)
		case wctype.Iswalpha(lexer.Lookahead) || lexer.Lookahead == '_':
			isNumber = false
			advance(lexer)
		default:
			if lexer.Lookahead == '{' {
				return s.scanBraceStart(lexer, validSymbols)
			}
			if validSymbols[expansionWord] {
				return s.scanExpansionWord(lexer, validSymbols)
			}
			if validSymbols[extglobPattern] {
				return s.scanExtglobPattern(lexer, validSymbols)
			}
			return false
		}

	word:
		for {
			switch {
			case wctype.Iswdigit(lexer.Lookahead):
				advance(lexer)
			case wctype.Iswalpha(lexer.Lookahead) || lexer.Lookahead == '_':
				isNumber = false
				advance(lexer)
			default:
				break word
			}
		}

		if isNumber && validSymbols[fileDescriptor] && (lexer.Lookahead == '>' || lexer.Lookahead == '<') {
			lexer.ResultSymbol = uint16(fileDescriptor)
			return true
		}

		if validSymbols[variableName] {
			if lexer.Lookahead == '+' {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '=' || lexer.Lookahead == ':' || validSymbols[closingBrace] {
					lexer.ResultSymbol = uint16(variableName)
					return true
				}
				return false
			}
			if lexer.Lookahead == '/' {
				return false
			}
			if lexer.Lookahead == '=' || lexer.Lookahead == '[' ||
				(lexer.Lookahead == ':' && !validSymbols[closingBrace] &&
					!validSymbols[openingParen]) || // TODO(amaanq): more cases for regular word chars but not variable
				// names for function words, only handling : for now? #235
				lexer.Lookahead == '%' ||
				(lexer.Lookahead == '#' && !isNumber) || lexer.Lookahead == '@' ||
				(lexer.Lookahead == '-' && validSymbols[closingBrace]) {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(variableName)
				return true
			}

			if lexer.Lookahead == '?' {
				lexer.MarkEnd()
				advance(lexer)
				lexer.ResultSymbol = uint16(variableName)
				return wctype.Iswalpha(lexer.Lookahead)
			}
		}

		return false
	}

	if validSymbols[bareDollar] && !inErrorRecovery(validSymbols) && scanBareDollar(lexer) {
		return true
	}

	return s.scanRegex(lexer, validSymbols)
}

// scanRegex is the part of scan that starts at the label regex.
func (s *scanner) scanRegex(lexer *abi.Lexer, validSymbols []bool) bool {
	if (validSymbols[regex] || validSymbols[regexNoSlash] || validSymbols[regexNoSpace]) &&
		!inErrorRecovery(validSymbols) {
		if validSymbols[regex] || validSymbols[regexNoSpace] {
			for wctype.Iswspace(lexer.Lookahead) {
				skip(lexer)
			}
		}

		if (lexer.Lookahead != '"' && lexer.Lookahead != '\'') ||
			((lexer.Lookahead == '$' || lexer.Lookahead == '\'') && validSymbols[regexNoSlash]) ||
			(lexer.Lookahead == '\'' && validSymbols[regexNoSpace]) {
			type regexState struct {
				done                         bool
				advancedOnce                 bool
				foundNonAlnumdollarunderdash bool
				lastWasEscape                bool
				inSingleQuote                bool
				parenDepth                   uint32
				bracketDepth                 uint32
				braceDepth                   uint32
			}

			if lexer.Lookahead == '$' && validSymbols[regexNoSlash] {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '(' {
					return false
				}
			}

			lexer.MarkEnd()

			state := regexState{}
			for !state.done {
				if state.inSingleQuote {
					if lexer.Lookahead == '\'' {
						state.inSingleQuote = false
						advance(lexer)
						lexer.MarkEnd()
					}
				}
				switch lexer.Lookahead {
				case '\\':
					state.lastWasEscape = true
				case 0:
					return false
				case '(':
					state.parenDepth++
					state.lastWasEscape = false
				case '[':
					state.bracketDepth++
					state.lastWasEscape = false
				case '{':
					if !state.lastWasEscape {
						state.braceDepth++
					}
					state.lastWasEscape = false
				case ')':
					if state.parenDepth == 0 {
						state.done = true
					}
					state.parenDepth--
					state.lastWasEscape = false
				case ']':
					if state.bracketDepth == 0 {
						state.done = true
					}
					state.bracketDepth--
					state.lastWasEscape = false
				case '}':
					if state.braceDepth == 0 {
						state.done = true
					}
					state.braceDepth--
					state.lastWasEscape = false
				case '\'':
					// Enter or exit a single-quoted string.
					state.inSingleQuote = !state.inSingleQuote
					advance(lexer)
					state.advancedOnce = true
					state.lastWasEscape = false
					continue
				default:
					state.lastWasEscape = false
				}

				if !state.done {
					switch {
					case validSymbols[regex]:
						wasSpace := !state.inSingleQuote && wctype.Iswspace(lexer.Lookahead)
						advance(lexer)
						state.advancedOnce = true
						if !wasSpace || state.parenDepth > 0 {
							lexer.MarkEnd()
						}
					case validSymbols[regexNoSlash]:
						if lexer.Lookahead == '/' {
							lexer.MarkEnd()
							lexer.ResultSymbol = uint16(regexNoSlash)
							return state.advancedOnce
						}
						if lexer.Lookahead == '\\' {
							advance(lexer)
							state.advancedOnce = true
							if !lexer.EOF() && lexer.Lookahead != '[' && lexer.Lookahead != '/' {
								advance(lexer)
								lexer.MarkEnd()
							}
						} else {
							wasSpace := !state.inSingleQuote && wctype.Iswspace(lexer.Lookahead)
							advance(lexer)
							state.advancedOnce = true
							if !wasSpace {
								lexer.MarkEnd()
							}
						}
					case validSymbols[regexNoSpace]:
						switch lexer.Lookahead {
						case '\\':
							state.foundNonAlnumdollarunderdash = true
							advance(lexer)
							if !lexer.EOF() {
								advance(lexer)
							}
						case '$':
							lexer.MarkEnd()
							advance(lexer)
							// do not parse a command
							// substitution
							if lexer.Lookahead == '(' {
								return false
							}
							// end $ always means regex, e.g.
							// 99999999$
							if wctype.Iswspace(lexer.Lookahead) {
								lexer.ResultSymbol = uint16(regexNoSpace)
								lexer.MarkEnd()
								return true
							}
						default:
							wasSpace := !state.inSingleQuote && wctype.Iswspace(lexer.Lookahead)
							if wasSpace && state.parenDepth == 0 {
								lexer.MarkEnd()
								lexer.ResultSymbol = uint16(regexNoSpace)
								return state.foundNonAlnumdollarunderdash
							}
							if !wctype.Iswalnum(lexer.Lookahead) && lexer.Lookahead != '$' && lexer.Lookahead != '-' &&
								lexer.Lookahead != '_' {
								state.foundNonAlnumdollarunderdash = true
							}
							advance(lexer)
						}
					}
				}
			}

			switch {
			case validSymbols[regexNoSlash]:
				lexer.ResultSymbol = uint16(regexNoSlash)
			case validSymbols[regexNoSpace]:
				lexer.ResultSymbol = uint16(regexNoSpace)
			default:
				lexer.ResultSymbol = uint16(regex)
			}
			if validSymbols[regex] && !state.advancedOnce {
				return false
			}
			return true
		}
	}

	return s.scanExtglobPattern(lexer, validSymbols)
}

// scanExtglobPattern is the part of scan that starts at the label
// extglob_pattern.
func (s *scanner) scanExtglobPattern(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[extglobPattern] && !inErrorRecovery(validSymbols) {
		// first skip ws, then check for ? * + @ !
		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		if lexer.Lookahead == '?' || lexer.Lookahead == '*' || lexer.Lookahead == '+' || lexer.Lookahead == '@' ||
			lexer.Lookahead == '!' || lexer.Lookahead == '-' || lexer.Lookahead == ')' || lexer.Lookahead == '\\' ||
			lexer.Lookahead == '.' || lexer.Lookahead == '[' || (wctype.Iswalpha(lexer.Lookahead)) {
			if lexer.Lookahead == '\\' {
				advance(lexer)
				if (wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == '"') && lexer.Lookahead != '\r' &&
					lexer.Lookahead != '\n' {
					advance(lexer)
				} else {
					return false
				}
			}

			if lexer.Lookahead == ')' && s.lastGlobParenDepth == 0 {
				lexer.MarkEnd()
				advance(lexer)

				if wctype.Iswspace(lexer.Lookahead) {
					return false
				}
			}

			lexer.MarkEnd()
			wasNonAlpha := !wctype.Iswalpha(lexer.Lookahead)
			if lexer.Lookahead != '[' {
				// no esac
				if lexer.Lookahead == 'e' {
					lexer.MarkEnd()
					advance(lexer)
					if lexer.Lookahead == 's' {
						advance(lexer)
						if lexer.Lookahead == 'a' {
							advance(lexer)
							if lexer.Lookahead == 'c' {
								advance(lexer)
								if wctype.Iswspace(lexer.Lookahead) {
									return false
								}
							}
						}
					}
				} else {
					advance(lexer)
				}
			}

			// -\w is just a word, find something else special
			if lexer.Lookahead == '-' {
				lexer.MarkEnd()
				advance(lexer)
				for wctype.Iswalnum(lexer.Lookahead) {
					advance(lexer)
				}

				if lexer.Lookahead == ')' || lexer.Lookahead == '\\' || lexer.Lookahead == '.' {
					return false
				}
				lexer.MarkEnd()
			}

			// case item -) or *)
			if lexer.Lookahead == ')' && s.lastGlobParenDepth == 0 {
				lexer.MarkEnd()
				advance(lexer)
				if wctype.Iswspace(lexer.Lookahead) {
					lexer.ResultSymbol = uint16(extglobPattern)
					return wasNonAlpha
				}
			}

			if wctype.Iswspace(lexer.Lookahead) {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(extglobPattern)
				s.lastGlobParenDepth = 0
				return true
			}

			if lexer.Lookahead == '$' {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '{' || lexer.Lookahead == '(' {
					lexer.ResultSymbol = uint16(extglobPattern)
					return true
				}
			}

			if lexer.Lookahead == '|' {
				lexer.MarkEnd()
				advance(lexer)
				lexer.ResultSymbol = uint16(extglobPattern)
				return true
			}

			if !wctype.Iswalnum(lexer.Lookahead) && lexer.Lookahead != '(' && lexer.Lookahead != '"' &&
				lexer.Lookahead != '[' && lexer.Lookahead != '?' && lexer.Lookahead != '/' &&
				lexer.Lookahead != '\\' && lexer.Lookahead != '_' && lexer.Lookahead != '*' {
				return false
			}

			type extglobState struct {
				done           bool
				sawNonAlphadot bool
				parenDepth     uint32
				bracketDepth   uint32
				braceDepth     uint32
			}

			state := extglobState{false, wasNonAlpha, uint32(s.lastGlobParenDepth), 0, 0}
			for !state.done {
				switch lexer.Lookahead {
				case 0:
					return false
				case '(':
					state.parenDepth++
				case '[':
					state.bracketDepth++
				case '{':
					state.braceDepth++
				case ')':
					if state.parenDepth == 0 {
						state.done = true
					}
					state.parenDepth--
				case ']':
					if state.bracketDepth == 0 {
						state.done = true
					}
					state.bracketDepth--
				case '}':
					if state.braceDepth == 0 {
						state.done = true
					}
					state.braceDepth--
				}

				if lexer.Lookahead == '|' {
					lexer.MarkEnd()
					advance(lexer)
					if state.parenDepth == 0 && state.bracketDepth == 0 && state.braceDepth == 0 {
						lexer.ResultSymbol = uint16(extglobPattern)
						return true
					}
				}

				if !state.done {
					wasSpace := wctype.Iswspace(lexer.Lookahead)
					if lexer.Lookahead == '$' {
						lexer.MarkEnd()
						if !wctype.Iswalpha(lexer.Lookahead) && lexer.Lookahead != '.' && lexer.Lookahead != '\\' {
							state.sawNonAlphadot = true
						}
						advance(lexer)
						if lexer.Lookahead == '(' || lexer.Lookahead == '{' {
							lexer.ResultSymbol = uint16(extglobPattern)
							s.lastGlobParenDepth = uint8(state.parenDepth)
							return state.sawNonAlphadot
						}
					}
					if wasSpace {
						lexer.MarkEnd()
						lexer.ResultSymbol = uint16(extglobPattern)
						s.lastGlobParenDepth = 0
						return state.sawNonAlphadot
					}
					if lexer.Lookahead == '"' {
						lexer.MarkEnd()
						lexer.ResultSymbol = uint16(extglobPattern)
						s.lastGlobParenDepth = 0
						return state.sawNonAlphadot
					}
					if lexer.Lookahead == '\\' {
						if !wctype.Iswalpha(lexer.Lookahead) && lexer.Lookahead != '.' && lexer.Lookahead != '\\' {
							state.sawNonAlphadot = true
						}
						advance(lexer)
						if wctype.Iswspace(lexer.Lookahead) || lexer.Lookahead == '"' {
							advance(lexer)
						}
					} else {
						if !wctype.Iswalpha(lexer.Lookahead) && lexer.Lookahead != '.' && lexer.Lookahead != '\\' {
							state.sawNonAlphadot = true
						}
						advance(lexer)
					}
					if !wasSpace {
						lexer.MarkEnd()
					}
				}
			}

			lexer.ResultSymbol = uint16(extglobPattern)
			s.lastGlobParenDepth = 0
			return state.sawNonAlphadot
		}
		s.lastGlobParenDepth = 0

		return false
	}

	return s.scanExpansionWord(lexer, validSymbols)
}

// scanExpansionWord is the part of scan that starts at the label
// expansion_word.
func (s *scanner) scanExpansionWord(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[expansionWord] {
		advancedOnce := false
		advanceOnceSpace := false
		for {
			if lexer.Lookahead == '"' {
				return false
			}
			if lexer.Lookahead == '$' {
				lexer.MarkEnd()
				advance(lexer)
				if lexer.Lookahead == '{' || lexer.Lookahead == '(' || lexer.Lookahead == '\'' ||
					wctype.Iswalnum(lexer.Lookahead) {
					lexer.ResultSymbol = uint16(expansionWord)
					return advancedOnce
				}
				advancedOnce = true
			}

			if lexer.Lookahead == '}' {
				lexer.MarkEnd()
				lexer.ResultSymbol = uint16(expansionWord)
				return advancedOnce || advanceOnceSpace
			}

			if lexer.Lookahead == '(' && !(advancedOnce || advanceOnceSpace) {
				lexer.MarkEnd()
				advance(lexer)
				for lexer.Lookahead != ')' && !lexer.EOF() {
					// if we find a $( or ${ assume this is valid and is
					// a garbage concatenation of some weird word + an
					// expansion
					// I wonder where this can fail
					if lexer.Lookahead == '$' {
						lexer.MarkEnd()
						advance(lexer)
						if lexer.Lookahead == '{' || lexer.Lookahead == '(' || lexer.Lookahead == '\'' ||
							wctype.Iswalnum(lexer.Lookahead) {
							lexer.ResultSymbol = uint16(expansionWord)
							return advancedOnce
						}
						advancedOnce = true
					} else {
						advancedOnce = advancedOnce || !wctype.Iswspace(lexer.Lookahead)
						advanceOnceSpace = advanceOnceSpace || wctype.Iswspace(lexer.Lookahead)
						advance(lexer)
					}
				}
				lexer.MarkEnd()
				if lexer.Lookahead == ')' {
					advancedOnce = true
					advance(lexer)
					lexer.MarkEnd()
					if lexer.Lookahead == '}' {
						return false
					}
				} else {
					return false
				}
			}

			if lexer.Lookahead == '\'' {
				return false
			}

			if lexer.EOF() {
				return false
			}
			advancedOnce = advancedOnce || !wctype.Iswspace(lexer.Lookahead)
			advanceOnceSpace = advanceOnceSpace || wctype.Iswspace(lexer.Lookahead)
			advance(lexer)
		}
	}

	return s.scanBraceStart(lexer, validSymbols)
}

// scanBraceStart is the part of scan that starts at the label brace_start.
func (s *scanner) scanBraceStart(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[braceStart] && !inErrorRecovery(validSymbols) {
		for wctype.Iswspace(lexer.Lookahead) {
			skip(lexer)
		}

		if lexer.Lookahead != '{' {
			return false
		}

		advance(lexer)
		lexer.MarkEnd()

		for wctype.Isdigit(lexer.Lookahead) {
			advance(lexer)
		}

		if lexer.Lookahead != '.' {
			return false
		}
		advance(lexer)

		if lexer.Lookahead != '.' {
			return false
		}
		advance(lexer)

		for wctype.Isdigit(lexer.Lookahead) {
			advance(lexer)
		}

		if lexer.Lookahead != '}' {
			return false
		}

		lexer.ResultSymbol = uint16(braceStart)
		return true
	}

	return false
}

// newScanner is tree_sitter_bash_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Scan is tree_sitter_bash_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	return s.scan(lexer, validSymbols)
}

// Serialize is tree_sitter_bash_external_scanner_serialize.
func (s *scanner) Serialize(state []byte) int {
	return int(s.serialize(state))
}

// Deserialize is tree_sitter_bash_external_scanner_deserialize.
func (s *scanner) Deserialize(state []byte) {
	s.deserialize(state)
}
