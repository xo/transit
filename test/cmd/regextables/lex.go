package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// tokenKind is the kind of a Rust token.
type tokenKind int

// The kinds of Rust token that the table files use.
const (
	tokEOF      tokenKind = iota
	tokIdent              // a name, such as pub or CASED_LETTER
	tokLifetime           // a lifetime, such as 'static, without the quote
	tokChar               // a char literal, such as 'a' or '\u{10ffff}'
	tokString             // a string literal, such as "Cased_Letter"
	tokPunct              // one character of punctuation, such as & or [
	tokComment            // a line comment, without the two slashes
)

// String returns the name of the kind, for an error message.
func (k tokenKind) String() string {
	switch k {
	case tokEOF:
		return "the end of the file"
	case tokIdent:
		return "a name"
	case tokLifetime:
		return "a lifetime"
	case tokChar:
		return "a char literal"
	case tokString:
		return "a string literal"
	case tokPunct:
		return "punctuation"
	case tokComment:
		return "a comment"
	}
	return "token kind " + strconv.Itoa(int(k))
}

// token is one Rust token.
type token struct {
	kind tokenKind
	text string // the name, the lifetime, the punctuation, the comment or the value of the string
	r    rune   // the value of a char literal
	line int
	col  int
}

// String returns the token as an error message shows it.
func (t token) String() string {
	switch t.kind {
	case tokEOF:
		return t.kind.String()
	case tokChar:
		return fmt.Sprintf("the char %U", t.r)
	case tokString:
		return "the string " + strconv.Quote(t.text)
	case tokLifetime:
		return "the lifetime '" + t.text
	}
	return fmt.Sprintf("%s %q", t.kind, t.text)
}

// puncts holds the punctuation that the table files use.
const puncts = "&[](),:;=#"

// lexer splits the text of a Rust file into tokens.
type lexer struct {
	src  string
	pos  int
	line int
	col  int
}

// lex returns the tokens of the Rust text src. The last token has the kind
// tokEOF. It fails on any form that the table files do not use.
func lex(src string) ([]token, error) {
	if !utf8.ValidString(src) {
		return nil, errors.New("reading text that is not UTF-8")
	}
	l := &lexer{src: src, line: 1, col: 1}
	var toks []token
	for {
		t, err := l.next()
		if err != nil {
			return nil, fmt.Errorf("%d:%d: %w", l.line, l.col, err)
		}
		toks = append(toks, t)
		if t.kind == tokEOF {
			return toks, nil
		}
	}
}

// peek returns the rune at the position, or -1 at the end of the text.
func (l *lexer) peek() rune {
	if l.pos >= len(l.src) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

// advance moves past one rune and returns it.
func (l *lexer) advance() rune {
	r, n := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += n
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// next returns the next token.
func (l *lexer) next() (token, error) {
	for {
		r := l.peek()
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		l.advance()
	}
	t := token{line: l.line, col: l.col}
	r := l.peek()
	switch {
	case r < 0:
		t.kind = tokEOF
		return t, nil
	case strings.HasPrefix(l.src[l.pos:], "//"):
		end := strings.IndexByte(l.src[l.pos:], '\n')
		if end < 0 {
			end = len(l.src) - l.pos
		}
		t.kind = tokComment
		t.text = l.src[l.pos+2 : l.pos+end]
		for range utf8.RuneCountInString(l.src[l.pos : l.pos+end]) {
			l.advance()
		}
		return t, nil
	case strings.HasPrefix(l.src[l.pos:], "/*"):
		return t, errors.New("reading a block comment, which the converter does not read")
	case isIdentStart(r):
		t.kind = tokIdent
		t.text = l.ident()
		return t, nil
	case r == '\'':
		return l.quote(t)
	case r == '"':
		l.advance()
		s, err := l.stringBody()
		if err != nil {
			return t, err
		}
		t.kind = tokString
		t.text = s
		return t, nil
	case strings.ContainsRune(puncts, r):
		l.advance()
		t.kind = tokPunct
		t.text = string(r)
		return t, nil
	}
	return t, fmt.Errorf("reading the character %q, which the converter does not read", r)
}

// isIdentStart reports whether r can start a Rust name.
func isIdentStart(r rune) bool {
	return r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'
}

// isIdent reports whether r can be in a Rust name.
func isIdent(r rune) bool {
	return isIdentStart(r) || '0' <= r && r <= '9'
}

// ident reads a name.
func (l *lexer) ident() string {
	start := l.pos
	for isIdent(l.peek()) {
		l.advance()
	}
	return l.src[start:l.pos]
}

// quote reads what starts with a single quote: a char literal or a lifetime.
func (l *lexer) quote(t token) (token, error) {
	l.advance()
	r := l.peek()
	switch {
	case r < 0:
		return t, errors.New("reading a char literal that does not end")
	case r == '\n' || r == '\r' || r == '\t':
		return t, fmt.Errorf("reading the character %q in a char literal without an escape", r)
	case r == '\\':
		l.advance()
		v, err := l.escape('\'')
		if err != nil {
			return t, err
		}
		if l.peek() != '\'' {
			return t, errors.New("reading a char literal that holds more than one character")
		}
		l.advance()
		t.kind = tokChar
		t.r = v
		return t, nil
	case r == '\'':
		return t, errors.New("reading an empty char literal")
	}
	l.advance()
	if l.peek() == '\'' {
		l.advance()
		t.kind = tokChar
		t.r = r
		return t, nil
	}
	if !isIdentStart(r) {
		return t, errors.New("reading a char literal that holds more than one character")
	}
	// a lifetime, such as 'static
	start := l.pos - utf8.RuneLen(r)
	for isIdent(l.peek()) {
		l.advance()
	}
	if l.peek() == '\'' {
		return t, errors.New("reading a char literal that holds more than one character")
	}
	t.kind = tokLifetime
	t.text = l.src[start:l.pos]
	return t, nil
}

// stringBody reads a string literal after its opening quote, and returns its
// value.
func (l *lexer) stringBody() (string, error) {
	var b strings.Builder
	for {
		r := l.peek()
		switch r {
		case -1:
			return "", errors.New("reading a string literal that does not end")
		case '"':
			l.advance()
			return b.String(), nil
		case '\\':
			l.advance()
			if l.peek() == '\n' {
				return "", errors.New("reading a line break after a backslash in a string, which the converter does not read")
			}
			v, err := l.escape('"')
			if err != nil {
				return "", err
			}
			b.WriteRune(v)
		case '\r':
			return "", errors.New("reading a carriage return in a string literal")
		default:
			l.advance()
			b.WriteRune(r)
		}
	}
}

// escape reads an escape after its backslash, and returns the character that
// it names. The quote is the quote of the literal that holds it.
func (l *lexer) escape(quote rune) (rune, error) {
	r := l.peek()
	if r < 0 {
		return 0, errors.New("reading an escape that does not end")
	}
	l.advance()
	switch r {
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case '\\':
		return '\\', nil
	case '0':
		return 0, nil
	case '\'', '"':
		return r, nil
	case 'x':
		return l.hexEscape()
	case 'u':
		return l.unicodeEscape()
	}
	return 0, fmt.Errorf("reading the escape \\%c in a literal with the quote %c, which Rust does not have", r, quote)
}

// hexEscape reads the two digits of \xNN, which Rust allows up to 7F.
func (l *lexer) hexEscape() (rune, error) {
	if l.pos+2 > len(l.src) {
		return 0, errors.New("reading a \\x escape that does not end")
	}
	v, err := strconv.ParseUint(l.src[l.pos:l.pos+2], 16, 8)
	if err != nil || v > 0x7f {
		return 0, fmt.Errorf("reading the escape \\x%s, which is not from \\x00 to \\x7f", l.src[l.pos:l.pos+2])
	}
	l.advance()
	l.advance()
	return rune(v), nil
}

// unicodeEscape reads the rest of \u{NNNN}.
func (l *lexer) unicodeEscape() (rune, error) {
	if l.peek() != '{' {
		return 0, errors.New("reading a \\u escape without a brace")
	}
	l.advance()
	end := strings.IndexByte(l.src[l.pos:], '}')
	if end < 0 {
		return 0, errors.New("reading a \\u escape that does not end")
	}
	digits := l.src[l.pos : l.pos+end]
	clean := strings.ReplaceAll(digits, "_", "")
	if clean == "" || len(clean) > 6 || strings.HasPrefix(digits, "_") {
		return 0, fmt.Errorf("reading the escape \\u{%s}, which does not hold one to six hex digits", digits)
	}
	v, err := strconv.ParseUint(clean, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("reading the escape \\u{%s}, which does not hold one to six hex digits", digits)
	}
	if !utf8.ValidRune(rune(v)) {
		return 0, fmt.Errorf("reading the escape \\u{%s}, which is not a Unicode scalar value", digits)
	}
	for range end + 1 {
		l.advance()
	}
	return rune(v), nil
}
