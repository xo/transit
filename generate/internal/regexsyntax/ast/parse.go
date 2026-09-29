package ast

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file ports src/ast/parse.rs: the parser of a pattern.
//
// The parser reads the pattern as UTF-8, as upstream reads a str. A position
// holds a byte offset, and a line and a column that count characters, as
// upstream counts them. A Go string can hold bytes that are not valid UTF-8,
// and a Rust str cannot. The parser reads such a byte as U+FFFD, one byte
// wide.
//
// Upstream keeps the state of Parser in a Cell or a RefCell, so that a method
// that takes &self can change it. The Go methods change it through a pointer.
// ParserI, which is generic over a Borrow<Parser>, becomes parserI with a
// *Parser, and ParserI::parser and ParserI::pattern are its fields. An Either
// of upstream becomes two results, and one of them is nil. A panic of
// upstream, such as a failed assert, is a panic here.
//
// The parser calls is_meta_character and is_escapeable_character of lib.rs.
// The port leaves out lib.rs, so this file holds them. It also holds
// isWhitespace, isAlphabetic and isAlphanumeric in place of the methods of
// the Rust type char. They use the tables of the Go package unicode, so they
// follow the Unicode version of Go, not the version of the Rust standard
// library. The two versions are the same, 17.0.0, for Go 1.27 and for Rust
// 1.97.1, which built the upstream tool of the golden files.
//
// The Default of ParserBuilder is NewParserBuilder, because the zero
// ParserBuilder has a nest limit of 0.

// primitive is an expression with no sub-expressions: a literal, an
// assertion or a class that is not a set. It is one of *Literal, *Assertion,
// *Dot, *ClassPerl and *ClassUnicode. The parser uses it for its state.
//
// A primitive is not an ASCII class, because an ASCII class can only occur
// in a set.
//
// primitive is Primitive. Primitive::span is GetSpan, and Primitive::into_ast
// is the primitive itself, because each variant is an Ast.
type primitive interface {
	Ast
	isPrimitive()
}

// isPrimitive marks *Literal as a primitive.
func (*Literal) isPrimitive() {}

// isPrimitive marks *Assertion as a primitive.
func (*Assertion) isPrimitive() {}

// isPrimitive marks *Dot as a primitive.
func (*Dot) isPrimitive() {}

// isPrimitive marks *ClassPerl as a primitive.
func (*ClassPerl) isPrimitive() {}

// isPrimitive marks *ClassUnicode as a primitive.
func (*ClassUnicode) isPrimitive() {}

// intoClassSetItem returns a primitive as an item of a class. An assertion or
// a dot is not a valid item, and for those intoClassSetItem returns an
// error.
//
// intoClassSetItem is Primitive::into_class_set_item.
func intoClassSetItem(prim primitive, p *parserI) (ClassSetItem, error) {
	switch x := prim.(type) {
	case *Literal:
		return x, nil
	case *ClassPerl:
		return x, nil
	case *ClassUnicode:
		return x, nil
	}
	return nil, p.error(prim.GetSpan(), ClassEscapeInvalid)
}

// intoClassLiteral returns a primitive as a literal of a class. Only a
// literal can be an end of a range, and for the others intoClassLiteral
// returns an error.
//
// intoClassLiteral is Primitive::into_class_literal.
func intoClassLiteral(prim primitive, p *parserI) (*Literal, error) {
	if lit, ok := prim.(*Literal); ok {
		return lit, nil
	}
	return nil, p.error(prim.GetSpan(), ClassRangeLiteral)
}

// isHex reports whether c is a hexadecimal digit.
//
// isHex is is_hex.
func isHex(c rune) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isCaptureChar reports whether a capture name can hold c. If first is true,
// c is the first character of the name, which must be a letter or an
// underscore.
//
// isCaptureChar is is_capture_char.
func isCaptureChar(c rune, first bool) bool {
	if first {
		return c == '_' || isAlphabetic(c)
	}
	return c == '_' || c == '.' || c == '[' || c == ']' || isAlphanumeric(c)
}

// isMetaCharacter reports whether c is a meta character of the syntax. An
// escape of a meta character matches the character itself.
//
// isMetaCharacter is is_meta_character in lib.rs.
func isMetaCharacter(c rune) bool {
	switch c {
	case '\\', '.', '+', '*', '?', '(', ')', '|', '[', ']', '{', '}', '^', '$', '#', '&', '-', '~':
		return true
	}
	return false
}

// isEscapeableCharacter reports whether a pattern can escape c. Each meta
// character is escapeable, and some other characters are too. For example, %
// is not a meta character, but \% matches %.
//
// isEscapeableCharacter is is_escapeable_character in lib.rs.
func isEscapeableCharacter(c rune) bool {
	// A meta character can always be escaped.
	if isMetaCharacter(c) {
		return true
	}
	// A character that is not ASCII cannot be escaped.
	if c >= utf8.RuneSelf {
		return false
	}
	// Every other ASCII character can be escaped, except a letter or a digit.
	// A letter is kept for new syntax, and \3 is octal or an error. \< and \>
	// are assertions, so < and > are not escapeable either.
	switch {
	case '0' <= c && c <= '9', 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z':
		return false
	case c == '<', c == '>':
		return false
	}
	return true
}

// isWhitespace reports whether c has the Unicode property White_Space.
//
// isWhitespace is char::is_whitespace.
func isWhitespace(c rune) bool {
	return unicode.Is(unicode.White_Space, c)
}

// isAlphabetic reports whether c has the Unicode property Alphabetic. The
// package unicode has no table for it, so isAlphabetic uses the tables that
// Unicode derives it from: Lu, Ll, Lt, Lm, Lo, Nl, Other_Alphabetic,
// Other_Lowercase and Other_Uppercase.
//
// isAlphabetic is char::is_alphabetic.
func isAlphabetic(c rune) bool {
	return unicode.In(c, unicode.L, unicode.Nl, unicode.Other_Alphabetic, unicode.Other_Lowercase, unicode.Other_Uppercase)
}

// isAlphanumeric reports whether c is alphabetic or numeric. A numeric
// character is in the general category Nd, Nl or No.
//
// isAlphanumeric is char::is_alphanumeric.
func isAlphanumeric(c rune) bool {
	return isAlphabetic(c) || unicode.IsNumber(c)
}

// ParserBuilder holds the configuration of a parser.
//
// ParserBuilder is ParserBuilder.
type ParserBuilder struct {
	ignoreWhitespace bool
	nestLimit        uint32
	octal            bool
	emptyMinRange    bool
}

// NewParserBuilder returns a builder with the default configuration.
//
// NewParserBuilder is ParserBuilder::new.
func NewParserBuilder() *ParserBuilder {
	return &ParserBuilder{
		ignoreWhitespace: false,
		nestLimit:        250,
		octal:            false,
		emptyMinRange:    false,
	}
}

// Build returns a parser with the configuration of b.
//
// Build is ParserBuilder::build.
func (b *ParserBuilder) Build() *Parser {
	return &Parser{
		pos:                     Position{Offset: 0, Line: 1, Column: 1},
		captureIndex:            0,
		nestLimit:               b.nestLimit,
		octal:                   b.octal,
		emptyMinRange:           b.emptyMinRange,
		initialIgnoreWhitespace: b.ignoreWhitespace,
		ignoreWhitespace:        b.ignoreWhitespace,
	}
}

// NestLimit sets the nest limit of the parser, and returns b.
//
// The nest limit is the depth that the syntax tree can have. If a tree is
// deeper, for example with too many nested groups, the parser returns an
// error. The limit protects a consumer that walks a tree with recursion from
// a stack overflow. This package walks a tree with a stack on the heap.
//
// The parser checks the limit after it parses the whole tree. To limit the
// memory that the parser uses, limit the length of the pattern, because the
// parser uses memory in proportion to that length.
//
// A limit of 0 gives an error for most patterns, but not for all. For
// example, it accepts a, but not ab, because ab is a concatenation, which has
// a depth of 1.
//
// NestLimit is ParserBuilder::nest_limit.
func (b *ParserBuilder) NestLimit(limit uint32) *ParserBuilder {
	b.nestLimit = limit
	return b
}

// Octal sets whether the parser accepts the octal syntax, such as \141 for
// a, and returns b. The octal syntax is off by default. When it is off, the
// parser reports an octal escape as a backreference, which it does not
// support. A PCRE engine reads \0 as a backreference, and users expect that.
//
// Octal is ParserBuilder::octal.
func (b *ParserBuilder) Octal(yes bool) *ParserBuilder {
	b.octal = yes
	return b
}

// IgnoreWhitespace sets the verbose mode, and returns b. In verbose mode, the
// parser ignores white space in many places, and a # starts a comment that
// runs to the end of the line. Verbose mode is off by default. The flag x of
// a pattern turns it on for part of the pattern.
//
// IgnoreWhitespace is ParserBuilder::ignore_whitespace.
func (b *ParserBuilder) IgnoreWhitespace(yes bool) *ParserBuilder {
	b.ignoreWhitespace = yes
	return b
}

// EmptyMinRange sets whether the parser accepts {,n} for {0,n}, and returns
// b. Most engines do not accept {,n}, but the module re of Python does. It
// is off by default.
//
// EmptyMinRange is ParserBuilder::empty_min_range.
func (b *ParserBuilder) EmptyMinRange(yes bool) *ParserBuilder {
	b.emptyMinRange = yes
	return b
}

// Parser parses a pattern into a syntax tree. The size of the tree follows
// the length of the pattern. A ParserBuilder sets the configuration of a
// parser.
//
// A Parser parses one pattern. As in upstream, a second call panics, unless
// the first call stopped at offset 0 of its pattern.
//
// Parser is Parser.
type Parser struct {
	// pos is the position of the parser.
	pos Position
	// captureIndex is the capture index of the last capturing group.
	captureIndex uint32
	// nestLimit is the depth that the syntax tree can have.
	nestLimit uint32
	// octal is true when the parser accepts the octal syntax. When it is
	// false, the parser reports an octal escape as a backreference, which it
	// does not support.
	octal bool
	// initialIgnoreWhitespace is the verbose mode that the builder set. reset
	// uses it.
	initialIgnoreWhitespace bool
	// emptyMinRange is true when the parser accepts {,n} for {0,n}.
	emptyMinRange bool
	// ignoreWhitespace is true in verbose mode, which also allows comments.
	ignoreWhitespace bool
	// comments are the comments of the pattern, in order.
	comments []Comment
	// stackGroup is a stack of the open groups and alternations.
	stackGroup []groupState
	// stackClass is a stack of the open classes. It is empty except while
	// the parser reads a class.
	stackClass []classState
	// captureNames are the capture names, sorted by name. The parser uses
	// them to find a name that occurs twice.
	captureNames []CaptureName
	// scratch is a buffer that the parser uses again and again, for example
	// to collect the characters of a part of the pattern.
	scratch []byte
}

// parserI is the parser at work on one pattern. The state of a Parser is not
// tied to a pattern, but a parserI is.
//
// parserI is ParserI.
type parserI struct {
	// parser is the state and the configuration of the parser.
	parser *Parser
	// pattern is the whole pattern.
	pattern string
}

// groupState is a frame of the stack of groups. It holds the state up to an
// opening parenthesis or a |.
//
// groupState is GroupState, an enum with data upstream. A groupState with an
// alternation that is not nil is GroupState::Alternation. Otherwise it is
// GroupState::Group, which uses concat, group and ignoreWhitespace.
type groupState struct {
	// concat is the concatenation before the opening parenthesis.
	concat *Concat
	// group is the open group. Its tree is always empty.
	group *Group
	// ignoreWhitespace is the verbose mode before the group.
	ignoreWhitespace bool
	// alternation is the alternation that the parser builds. When the parser
	// finds a new branch and an alternation is on top of the stack, it adds
	// the branch to that alternation.
	alternation *Alternation
}

// classState is a frame of the stack of classes. It holds the state up to an
// intersection, a difference, a symmetric difference or a nested class.
//
// classState is ClassState, an enum with data upstream. A classState with a
// set that is not nil is ClassState::Open, which uses union and set.
// Otherwise it is ClassState::Op, which uses kind and lhs.
type classState struct {
	// union is the union of the items before the class.
	union *ClassSetUnion
	// set is the open class. It holds the [, and the ^ of a negated class.
	set *ClassBracketed
	// kind is the operation: &&, -- or ~~.
	kind ClassSetBinaryOpKind
	// lhs is the left side of the operation. When the parser pops the
	// frame, lhs becomes the left side of a *ClassSetBinaryOp.
	lhs ClassSet
}

// NewParser returns a parser with the default configuration. Parse and
// ParseWithComments run it.
//
// NewParser is Parser::new.
func NewParser() *Parser {
	return NewParserBuilder().Build()
}

// Parse parses a pattern into a syntax tree. The error is an *Error.
//
// Parse is Parser::parse.
func (p *Parser) Parse(pattern string) (Ast, error) {
	return newParserI(p, pattern).parse()
}

// ParseWithComments parses a pattern into a syntax tree, and returns the tree
// with the comments of the pattern. The error is an *Error.
//
// ParseWithComments is Parser::parse_with_comments.
func (p *Parser) ParseWithComments(pattern string) (*WithComments, error) {
	return newParserI(p, pattern).parseWithComments()
}

// reset resets the state of the parser. Each parse calls it first, so that
// the parser does not run with the state of an earlier parse that failed.
//
// reset is Parser::reset.
func (p *Parser) reset() {
	// These values match the ones of ParserBuilder.Build.
	p.pos = Position{Offset: 0, Line: 1, Column: 1}
	p.ignoreWhitespace = p.initialIgnoreWhitespace
	p.comments = p.comments[:0]
	p.stackGroup = p.stackGroup[:0]
	p.stackClass = p.stackClass[:0]
}

// newParserI returns the parser at work on a pattern.
//
// newParserI is ParserI::new.
func newParserI(parser *Parser, pattern string) *parserI {
	return &parserI{parser: parser, pattern: pattern}
}

// error returns an error with a span and a kind.
//
// error is ParserI::error.
func (p *parserI) error(span Span, kind ErrorKind) *Error {
	return &Error{Kind: kind, Pattern: p.pattern, Span: span}
}

// offset returns the offset of the parser, from 0.
//
// offset is ParserI::offset.
func (p *parserI) offset() int {
	return p.parser.pos.Offset
}

// line returns the line of the parser, from 1.
//
// line is ParserI::line.
func (p *parserI) line() int {
	return p.parser.pos.Line
}

// column returns the column of the parser, from 1. The column starts again
// at 1 after each \n.
//
// column is ParserI::column.
func (p *parserI) column() int {
	return p.parser.pos.Column
}

// nextCaptureIndex returns the next capture index, and adds one to the index
// of the parser. The span is the span of the opening parenthesis. If there
// are too many capturing groups, nextCaptureIndex returns an error.
//
// nextCaptureIndex is ParserI::next_capture_index.
func (p *parserI) nextCaptureIndex(span Span) (uint32, error) {
	current := p.parser.captureIndex
	if current == math.MaxUint32 {
		return 0, p.error(span, CaptureLimitExceeded)
	}
	i := current + 1
	p.parser.captureIndex = i
	return i, nil
}

// addCaptureName adds a capture name to the parser. If the parser has the
// name already, addCaptureName returns an error.
//
// addCaptureName is ParserI::add_capture_name.
func (p *parserI) addCaptureName(capture CaptureName) error {
	names := p.parser.captureNames
	i, found := slices.BinarySearchFunc(names, capture.Name, func(c CaptureName, name string) int {
		return strings.Compare(c.Name, name)
	})
	if found {
		err := p.error(capture.Span, GroupNameDuplicate)
		err.Original = names[i].Span
		return err
	}
	p.parser.captureNames = slices.Insert(names, i, capture)
	return nil
}

// ignoreWhitespace reports whether the parser is in verbose mode.
//
// ignoreWhitespace is ParserI::ignore_whitespace.
func (p *parserI) ignoreWhitespace() bool {
	return p.parser.ignoreWhitespace
}

// char returns the character at the position of the parser. It panics if
// the parser is at the end of the pattern.
//
// char is ParserI::char.
func (p *parserI) char() rune {
	return p.charAt(p.offset())
}

// charLen returns the width in bytes of the character at the position of the
// parser. It is the size of the UTF-8 sequence that char decodes, so it is 1
// for a byte that is not valid UTF-8.
//
// charLen is self.char().len_utf8() of upstream.
func (p *parserI) charLen() int {
	_, n := utf8.DecodeRuneInString(p.pattern[p.offset():])
	return n
}

// charAt returns the character at the offset i. It panics if there is no
// character at i.
//
// charAt is ParserI::char_at.
func (p *parserI) charAt(i int) rune {
	c, n := utf8.DecodeRuneInString(p.pattern[i:])
	if n == 0 {
		panic(fmt.Sprintf("expected char at offset %d", i))
	}
	return c
}

// bump moves the parser to the next character. It returns false if the
// parser is at the end of the pattern after the move, or was there before.
//
// bump is ParserI::bump.
func (p *parserI) bump() bool {
	if p.isEOF() {
		return false
	}
	pos := p.pos()
	if p.char() == '\n' {
		pos.Line++
		pos.Column = 1
	} else {
		pos.Column++
	}
	pos.Offset += p.charLen()
	p.parser.pos = pos
	return p.offset() < len(p.pattern)
}

// bumpIf moves the parser past prefix and returns true, if the pattern at the
// position of the parser starts with prefix. Otherwise it does not move the
// parser, and returns false.
//
// bumpIf is ParserI::bump_if.
func (p *parserI) bumpIf(prefix string) bool {
	if !strings.HasPrefix(p.pattern[p.offset():], prefix) {
		return false
	}
	for range utf8.RuneCountInString(prefix) {
		p.bump()
	}
	return true
}

// isLookaroundPrefix moves the parser past a prefix of a look-around and
// returns true, if the parser is at one. Such a prefix is always a pattern
// that is not valid. Call it only after the opening of a group or a set of
// flags.
//
// isLookaroundPrefix is ParserI::is_lookaround_prefix.
func (p *parserI) isLookaroundPrefix() bool {
	return p.bumpIf("?=") || p.bumpIf("?!") || p.bumpIf("?<=") || p.bumpIf("?<!")
}

// bumpAndBumpSpace moves the parser to the next character, and in verbose
// mode past the white space after it. It returns true if the parser is not
// at the end of the pattern.
//
// bumpAndBumpSpace is ParserI::bump_and_bump_space.
func (p *parserI) bumpAndBumpSpace() bool {
	if !p.bump() {
		return false
	}
	p.bumpSpace()
	return !p.isEOF()
}

// bumpSpace moves the parser past the white space and the comments at its
// position, in verbose mode. When verbose mode is off, it does nothing. The
// parser calls it where verbose mode allows white space. For example,
// {   5  , 6} is the same as {5,6}.
//
// bumpSpace is ParserI::bump_space.
func (p *parserI) bumpSpace() {
	if !p.ignoreWhitespace() {
		return
	}
	for !p.isEOF() {
		switch {
		case isWhitespace(p.char()):
			p.bump()
		case p.char() == '#':
			start := p.pos()
			var commentText strings.Builder
			p.bump()
			for !p.isEOF() {
				c := p.char()
				p.bump()
				if c == '\n' {
					break
				}
				commentText.WriteRune(c)
			}
			comment := Comment{
				Span:    NewSpan(start, p.pos()),
				Comment: commentText.String(),
			}
			p.parser.comments = append(p.parser.comments, comment)
		default:
			return
		}
	}
}

// peek returns the character after the one at the position of the parser,
// and true, without a move. At the end of the pattern, peek returns false.
//
// peek is ParserI::peek.
func (p *parserI) peek() (rune, bool) {
	if p.isEOF() {
		return 0, false
	}
	rest := p.pattern[p.offset()+p.charLen():]
	if rest == "" {
		return 0, false
	}
	c, _ := utf8.DecodeRuneInString(rest)
	return c, true
}

// peekSpace is peek, but in verbose mode it skips the white space and the
// comments.
//
// peekSpace is ParserI::peek_space. As upstream, it skips a \n as white
// space, so a comment does not end, and it returns the first character of a
// comment that is not white space.
func (p *parserI) peekSpace() (rune, bool) {
	if !p.ignoreWhitespace() {
		return p.peek()
	}
	if p.isEOF() {
		return 0, false
	}
	start := p.offset() + p.charLen()
	inComment := false
loop:
	for i, c := range p.pattern[start:] {
		switch {
		case isWhitespace(c):
			continue
		case !inComment && c == '#':
			inComment = true
		case inComment && c == '\n':
			inComment = false
		default:
			start += i
			break loop
		}
	}
	rest := p.pattern[start:]
	if rest == "" {
		return 0, false
	}
	c, _ := utf8.DecodeRuneInString(rest)
	return c, true
}

// isEOF reports whether the next call to bump returns false.
//
// isEOF is ParserI::is_eof.
func (p *parserI) isEOF() bool {
	return p.offset() == len(p.pattern)
}

// pos returns the position of the parser: the offset, the line and the
// column.
//
// pos is ParserI::pos.
func (p *parserI) pos() Position {
	return p.parser.pos
}

// span returns an empty span at the position of the parser.
//
// span is ParserI::span.
func (p *parserI) span() Span {
	return SplatSpan(p.pos())
}

// spanChar returns the span of the character at the position of the parser.
//
// spanChar is ParserI::span_char.
func (p *parserI) spanChar() Span {
	next := Position{
		Offset: p.offset() + p.charLen(),
		Line:   p.line(),
		Column: p.column() + 1,
	}
	if p.char() == '\n' {
		next.Line++
		next.Column = 1
	}
	return NewSpan(p.pos(), next)
}

// pushAlternate pushes a branch of an alternation on the stack of groups.
// If an alternation is on top of the stack, it adds the branch to it. The
// concatenation is the branch. The empty concatenation that pushAlternate
// returns starts the next branch.
//
// The parser must be at a |. pushAlternate moves it past the |.
//
// pushAlternate is ParserI::push_alternate. Upstream returns a Result, but
// never an error, so pushAlternate returns no error.
func (p *parserI) pushAlternate(concat *Concat) *Concat {
	if p.char() != '|' {
		panic("assertion failed: the parser is not at |")
	}
	concat.Span.End = p.pos()
	p.pushOrAddAlternation(concat)
	p.bump()
	return &Concat{Span: p.span()}
}

// pushOrAddAlternation pushes a branch of an alternation on the stack of
// groups, or adds it to the alternation on top of the stack.
//
// pushOrAddAlternation is ParserI::push_or_add_alternation.
func (p *parserI) pushOrAddAlternation(concat *Concat) {
	stack := p.parser.stackGroup
	if n := len(stack); n > 0 && stack[n-1].alternation != nil {
		alts := stack[n-1].alternation
		alts.Asts = append(alts.Asts, concat.IntoAst())
		return
	}
	p.parser.stackGroup = append(p.parser.stackGroup, groupState{alternation: &Alternation{
		Span: NewSpan(concat.Span.Start, p.pos()),
		Asts: []Ast{concat.IntoAst()},
	}})
}

// pushGroup parses the start of a group and pushes the group, with the
// concatenation before it, on the stack of groups. It returns a new
// concatenation for the tree of the group. For a set of flags with no group,
// it adds the flags to the concatenation and returns it.
//
// The parser must be at the opening parenthesis. pushGroup moves it to the
// start of the expression in the group, or of the expression after the
// flags. If the start of the group is not valid, pushGroup returns an error.
//
// pushGroup is ParserI::push_group.
func (p *parserI) pushGroup(concat *Concat) (*Concat, error) {
	if p.char() != '(' {
		panic("assertion failed: the parser is not at (")
	}
	set, group, err := p.parseGroup()
	if err != nil {
		return nil, err
	}
	if set != nil {
		if v, ok := set.Flags.FlagState(FlagIgnoreWhitespace); ok {
			p.parser.ignoreWhitespace = v
		}
		concat.Asts = append(concat.Asts, set)
		return concat, nil
	}
	oldIgnoreWhitespace := p.ignoreWhitespace()
	newIgnoreWhitespace := oldIgnoreWhitespace
	if f := group.GetFlags(); f != nil {
		if v, ok := f.FlagState(FlagIgnoreWhitespace); ok {
			newIgnoreWhitespace = v
		}
	}
	p.parser.stackGroup = append(p.parser.stackGroup, groupState{
		concat:           concat,
		group:            group,
		ignoreWhitespace: oldIgnoreWhitespace,
	})
	p.parser.ignoreWhitespace = newIgnoreWhitespace
	return &Concat{Span: p.span()}, nil
}

// popGroup pops a group from the stack of groups, and sets the tree of the
// group to the concatenation. It returns the concatenation that holds the
// group.
//
// The parser must be at the closing parenthesis. popGroup moves it past the
// parenthesis. If no group is open, popGroup returns an error.
//
// popGroup is ParserI::pop_group.
func (p *parserI) popGroup(groupConcat *Concat) (*Concat, error) {
	if p.char() != ')' {
		panic("assertion failed: the parser is not at )")
	}
	stack := p.parser.stackGroup
	if len(stack) == 0 {
		return nil, p.error(p.spanChar(), GroupUnopened)
	}
	top := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	var alt *Alternation
	if top.alternation != nil {
		alt = top.alternation
		if len(stack) == 0 {
			p.parser.stackGroup = stack
			return nil, p.error(p.spanChar(), GroupUnopened)
		}
		top = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if top.alternation != nil {
			p.parser.stackGroup = stack
			return nil, p.error(p.spanChar(), GroupUnopened)
		}
	}
	p.parser.stackGroup = stack
	priorConcat, group := top.concat, top.group
	p.parser.ignoreWhitespace = top.ignoreWhitespace
	groupConcat.Span.End = p.pos()
	p.bump()
	group.Span.End = p.pos()
	if alt != nil {
		alt.Span.End = groupConcat.Span.End
		alt.Asts = append(alt.Asts, groupConcat.IntoAst())
		group.Ast = alt.IntoAst()
	} else {
		group.Ast = groupConcat.IntoAst()
	}
	priorConcat.Asts = append(priorConcat.Asts, group)
	return priorConcat, nil
}

// popGroupEnd pops the last frame of the stack of groups, if there is one,
// and adds the concatenation to it. The stack must be empty or hold one
// alternation. Otherwise popGroupEnd returns an error.
//
// The parser must be at the end of the pattern.
//
// popGroupEnd is ParserI::pop_group_end.
func (p *parserI) popGroupEnd(concat *Concat) (Ast, error) {
	concat.Span.End = p.pos()
	stack := p.parser.stackGroup
	var ast Ast
	switch n := len(stack); {
	case n == 0:
		ast = concat.IntoAst()
	case stack[n-1].alternation != nil:
		alt := stack[n-1].alternation
		stack = stack[:n-1]
		alt.Span.End = p.pos()
		alt.Asts = append(alt.Asts, concat.IntoAst())
		ast = alt
	default:
		p.parser.stackGroup = stack[:n-1]
		return nil, p.error(stack[n-1].group.Span, GroupUnclosed)
	}
	// A second pop finds nothing, or an open group.
	n := len(stack)
	if n == 0 {
		p.parser.stackGroup = stack
		return ast, nil
	}
	top := stack[n-1]
	p.parser.stackGroup = stack[:n-1]
	if top.alternation != nil {
		// This cannot happen. The parser never pushes an alternation on top
		// of an alternation, so two alternations are never next to each
		// other on the stack.
		panic("internal error: entered unreachable code")
	}
	return nil, p.error(top.group.Span, GroupUnclosed)
}

// pushClassOpen parses the opening of a class, and pushes the state of the
// class on the stack of classes. The union is the union of the items before
// the [. The parser must be at the [.
//
// If the opening of the class is not valid, pushClassOpen returns an error.
// Otherwise it returns a new union for the items of the class, which can
// hold a ] or a - already.
//
// pushClassOpen is ParserI::push_class_open.
func (p *parserI) pushClassOpen(parentUnion *ClassSetUnion) (*ClassSetUnion, error) {
	if p.char() != '[' {
		panic("assertion failed: the parser is not at [")
	}
	nestedSet, nestedUnion, err := p.parseSetClassOpen()
	if err != nil {
		return nil, err
	}
	p.parser.stackClass = append(p.parser.stackClass, classState{union: parentUnion, set: nestedSet})
	return nestedUnion, nil
}

// popClass parses the end of a class, and pops the stack of classes. The
// union is the last union before the ]. popClass returns the union of the
// parent class, with the nested class added to it. If the stack is empty
// after the pop, popClass returns the class at the top level instead, which
// is in no other class.
//
// The parser must be at a ]. popClass moves it past the ]. If no class is
// open, popClass returns an error.
//
// popClass is ParserI::pop_class. Upstream returns a Result, but never an
// error, so popClass returns no error.
func (p *parserI) popClass(nestedUnion *ClassSetUnion) (*ClassSetUnion, *ClassBracketed) {
	if p.char() != ']' {
		panic("assertion failed: the parser is not at ]")
	}
	item := nestedUnion.IntoItem()
	prevset := p.popClassOp(item)
	stack := p.parser.stackClass
	if len(stack) == 0 {
		// The stack is never empty here. The parser reads a class only after
		// it sees a [, so the stack starts with a frame. When the stack is
		// empty after the pop of a ], the parser stops reading the class.
		panic("unexpected empty character class stack")
	}
	top := stack[len(stack)-1]
	p.parser.stackClass = stack[:len(stack)-1]
	if top.set == nil {
		// This cannot happen, because popClassOp popped the Op frame. The
		// parser changes an Op frame on top of the stack, and never pushes
		// a second one on it.
		panic("unexpected ClassState::Op")
	}
	p.bump()
	top.set.Span.End = p.pos()
	top.set.Kind = prevset
	if len(p.parser.stackClass) == 0 {
		return nil, top.set
	}
	top.union.Push(top.set)
	return top.union, nil
}

// unclosedClassError returns an error for an unclosed class, with the span
// of the class that the parser opened last. Call it only while the parser
// reads a class.
//
// unclosedClassError is ParserI::unclosed_class_error.
func (p *parserI) unclosedClassError() *Error {
	for _, state := range slices.Backward(p.parser.stackClass) {
		if state.set != nil {
			return p.error(state.set.Span, ClassUnclosed)
		}
	}
	// The stack always holds an open class here.
	panic("no open character class found")
}

// pushClassOp pushes the items of the union on the stack of classes, as the
// left side of the operation. It returns a new union for the right side.
//
// pushClassOp is ParserI::push_class_op.
func (p *parserI) pushClassOp(nextKind ClassSetBinaryOpKind, nextUnion *ClassSetUnion) *ClassSetUnion {
	item := nextUnion.IntoItem()
	newLHS := p.popClassOp(item)
	p.parser.stackClass = append(p.parser.stackClass, classState{kind: nextKind, lhs: newLHS})
	return &ClassSetUnion{Span: p.span()}
}

// popClassOp pops a set from the stack of classes. If an operation is on top
// of the stack, popClassOp returns the operation with rhs as its right side.
// Otherwise it returns rhs.
//
// popClassOp is ParserI::pop_class_op.
func (p *parserI) popClassOp(rhs ClassSet) ClassSet {
	stack := p.parser.stackClass
	if len(stack) == 0 {
		panic("internal error: entered unreachable code")
	}
	top := stack[len(stack)-1]
	if top.set != nil {
		return rhs
	}
	p.parser.stackClass = stack[:len(stack)-1]
	span := NewSpan(top.lhs.GetSpan().Start, rhs.GetSpan().End)
	return &ClassSetBinaryOp{
		Span: span,
		Kind: top.kind,
		LHS:  top.lhs,
		RHS:  rhs,
	}
}

// parse parses the pattern into a syntax tree.
//
// parse is ParserI::parse.
func (p *parserI) parse() (Ast, error) {
	astc, err := p.parseWithComments()
	if err != nil {
		return nil, err
	}
	return astc.Ast, nil
}

// parseWithComments parses the pattern into a syntax tree, and returns the
// tree with the comments of the pattern.
//
// parseWithComments is ParserI::parse_with_comments.
func (p *parserI) parseWithComments() (*WithComments, error) {
	if p.offset() != 0 {
		panic("parser can only be used once")
	}
	p.parser.reset()
	concat := &Concat{Span: p.span()}
	for {
		p.bumpSpace()
		if p.isEOF() {
			break
		}
		var err error
		switch p.char() {
		case '(':
			concat, err = p.pushGroup(concat)
		case ')':
			concat, err = p.popGroup(concat)
		case '|':
			concat = p.pushAlternate(concat)
		case '[':
			var class *ClassBracketed
			class, err = p.parseSetClass()
			if err == nil {
				concat.Asts = append(concat.Asts, class)
			}
		case '?':
			concat, err = p.parseUncountedRepetition(concat, RepetitionZeroOrOne)
		case '*':
			concat, err = p.parseUncountedRepetition(concat, RepetitionZeroOrMore)
		case '+':
			concat, err = p.parseUncountedRepetition(concat, RepetitionOneOrMore)
		case '{':
			concat, err = p.parseCountedRepetition(concat)
		default:
			var prim primitive
			prim, err = p.parsePrimitive()
			if err == nil {
				concat.Asts = append(concat.Asts, prim)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	ast, err := p.popGroupEnd(concat)
	if err != nil {
		return nil, err
	}
	if err := newNestLimiter(p).check(ast); err != nil {
		return nil, err
	}
	comments := p.parser.comments
	p.parser.comments = nil
	return &WithComments{Ast: ast, Comments: comments}, nil
}

// parseUncountedRepetition parses a repetition operator: ?, * or +, but not
// {m,n}. The kind is the operator that the caller saw.
//
// The parser must be at the operator. parseUncountedRepetition moves it past
// the operator, which can have one more ? that makes it lazy.
//
// The concatenation is the one that the parser builds. The returned
// concatenation has the operator applied to its last expression.
//
// parseUncountedRepetition is ParserI::parse_uncounted_repetition.
func (p *parserI) parseUncountedRepetition(concat *Concat, kind RepetitionKind) (*Concat, error) {
	if c := p.char(); c != '?' && c != '*' && c != '+' {
		panic("assertion failed: the parser is not at ?, * or +")
	}
	opStart := p.pos()
	if len(concat.Asts) == 0 {
		return nil, p.error(p.span(), RepetitionMissing)
	}
	ast := concat.Asts[len(concat.Asts)-1]
	concat.Asts = concat.Asts[:len(concat.Asts)-1]
	switch ast.(type) {
	case *Empty, *SetFlags:
		return nil, p.error(p.span(), RepetitionMissing)
	}
	greedy := true
	if p.bump() && p.char() == '?' {
		greedy = false
		p.bump()
	}
	concat.Asts = append(concat.Asts, &Repetition{
		Span: ast.GetSpan().WithEnd(p.pos()),
		Op: RepetitionOp{
			Span: NewSpan(opStart, p.pos()),
			Kind: kind,
		},
		Greedy: greedy,
		Ast:    ast,
	})
	return concat, nil
}

// parseCountedRepetition parses a counted repetition operator, {m,n}. It
// does not parse ?, * or +.
//
// The parser must be at the {. parseCountedRepetition moves it past the
// operator, which can have one more ? that makes it lazy.
//
// The concatenation is the one that the parser builds. The returned
// concatenation has the operator applied to its last expression.
//
// parseCountedRepetition is ParserI::parse_counted_repetition.
func (p *parserI) parseCountedRepetition(concat *Concat) (*Concat, error) {
	if p.char() != '{' {
		panic("assertion failed: the parser is not at {")
	}
	start := p.pos()
	if len(concat.Asts) == 0 {
		return nil, p.error(p.span(), RepetitionMissing)
	}
	ast := concat.Asts[len(concat.Asts)-1]
	concat.Asts = concat.Asts[:len(concat.Asts)-1]
	switch ast.(type) {
	case *Empty, *SetFlags:
		return nil, p.error(p.span(), RepetitionMissing)
	}
	if !p.bumpAndBumpSpace() {
		return nil, p.error(NewSpan(start, p.pos()), RepetitionCountUnclosed)
	}
	countStart, countStartErr := p.parseDecimal()
	countStartErr = specializeErr(countStartErr, DecimalEmpty, RepetitionCountDecimalEmpty)
	if p.isEOF() {
		return nil, p.error(NewSpan(start, p.pos()), RepetitionCountUnclosed)
	}
	var rng RepetitionRange
	if p.char() == ',' {
		if !p.bumpAndBumpSpace() {
			return nil, p.error(NewSpan(start, p.pos()), RepetitionCountUnclosed)
		}
		if p.char() != '}' {
			if countStartErr != nil {
				var e *Error
				if !errors.As(countStartErr, &e) || e.Kind != RepetitionCountDecimalEmpty || !p.parser.emptyMinRange {
					return nil, countStartErr
				}
				countStart = 0
			}
			countEnd, err := p.parseDecimal()
			if err = specializeErr(err, DecimalEmpty, RepetitionCountDecimalEmpty); err != nil {
				return nil, err
			}
			rng = RepetitionRange{Kind: RepetitionRangeBounded, M: countStart, N: countEnd}
		} else {
			if countStartErr != nil {
				return nil, countStartErr
			}
			rng = RepetitionRange{Kind: RepetitionRangeAtLeast, M: countStart}
		}
	} else {
		if countStartErr != nil {
			return nil, countStartErr
		}
		rng = RepetitionRange{Kind: RepetitionRangeExactly, M: countStart}
	}

	if p.isEOF() || p.char() != '}' {
		return nil, p.error(NewSpan(start, p.pos()), RepetitionCountUnclosed)
	}

	greedy := true
	if p.bumpAndBumpSpace() && p.char() == '?' {
		greedy = false
		p.bump()
	}

	opSpan := NewSpan(start, p.pos())
	if !rng.IsValid() {
		return nil, p.error(opSpan, RepetitionCountInvalid)
	}
	concat.Asts = append(concat.Asts, &Repetition{
		Span: ast.GetSpan().WithEnd(p.pos()),
		Op: RepetitionOp{
			Span:  opSpan,
			Kind:  RepetitionKindRange,
			Range: rng,
		},
		Greedy: greedy,
		Ast:    ast,
	})
	return concat, nil
}

// parseGroup parses a group, which holds an expression, or a set of flags.
// For a group, it returns the group with an empty tree. For a set of flags,
// it returns the set.
//
// The parser must be at the opening parenthesis. parseGroup moves it to the
// character before the start of the expression of a group, or to the closing
// parenthesis after a set of flags.
//
// If the flags or the capture name are not valid, parseGroup returns an
// error.
//
// parseGroup is ParserI::parse_group.
func (p *parserI) parseGroup() (*SetFlags, *Group, error) {
	if p.char() != '(' {
		panic("assertion failed: the parser is not at (")
	}
	openSpan := p.spanChar()
	p.bump()
	p.bumpSpace()
	if p.isLookaroundPrefix() {
		return nil, nil, p.error(NewSpan(openSpan.Start, p.span().End), UnsupportedLookAround)
	}
	innerSpan := p.span()
	startsWithP := true
	isName := p.bumpIf("?P<")
	if !isName {
		startsWithP = false
		isName = p.bumpIf("?<")
	}
	switch {
	case isName:
		captureIndex, err := p.nextCaptureIndex(openSpan)
		if err != nil {
			return nil, nil, err
		}
		name, err := p.parseCaptureName(captureIndex)
		if err != nil {
			return nil, nil, err
		}
		return nil, &Group{
			Span:        openSpan,
			Kind:        GroupCaptureName,
			StartsWithP: startsWithP,
			Name:        name,
			Ast:         &Empty{Span: p.span()},
		}, nil
	case p.bumpIf("?"):
		if p.isEOF() {
			return nil, nil, p.error(openSpan, GroupUnclosed)
		}
		flags, err := p.parseFlags()
		if err != nil {
			return nil, nil, err
		}
		charEnd := p.char()
		p.bump()
		if charEnd == ')' {
			// Empty flags, such as (?), are not allowed. The parser reads
			// them as a repetition operator with no expression.
			if len(flags.Items) == 0 {
				return nil, nil, p.error(innerSpan, RepetitionMissing)
			}
			return &SetFlags{
				Span:  Span{Start: openSpan.Start, End: p.pos()},
				Flags: flags,
			}, nil, nil
		}
		if charEnd != ':' {
			panic("assertion failed: the flags do not end at :")
		}
		return nil, &Group{
			Span:  openSpan,
			Kind:  GroupNonCapturing,
			Flags: flags,
			Ast:   &Empty{Span: p.span()},
		}, nil
	}
	captureIndex, err := p.nextCaptureIndex(openSpan)
	if err != nil {
		return nil, nil, err
	}
	return nil, &Group{
		Span:  openSpan,
		Kind:  GroupCaptureIndex,
		Index: captureIndex,
		Ast:   &Empty{Span: p.span()},
	}, nil
}

// parseCaptureName parses a capture name. The parser must be at the first
// character of the name, after the <, or at the end of the pattern.
// parseCaptureName moves the parser past the closing >. The caller gives the
// capture index of the group of the name.
//
// parseCaptureName is ParserI::parse_capture_name.
func (p *parserI) parseCaptureName(captureIndex uint32) (CaptureName, error) {
	if p.isEOF() {
		return CaptureName{}, p.error(p.span(), GroupNameUnexpectedEOF)
	}
	start := p.pos()
	for p.char() != '>' {
		if !isCaptureChar(p.char(), p.pos() == start) {
			return CaptureName{}, p.error(p.spanChar(), GroupNameInvalid)
		}
		if !p.bump() {
			break
		}
	}
	end := p.pos()
	if p.isEOF() {
		return CaptureName{}, p.error(p.span(), GroupNameUnexpectedEOF)
	}
	if p.char() != '>' {
		panic("assertion failed: the parser is not at >")
	}
	p.bump()
	name := p.pattern[start.Offset:end.Offset]
	if name == "" {
		return CaptureName{}, p.error(NewSpan(start, start), GroupNameEmpty)
	}
	capname := CaptureName{
		Span:  NewSpan(start, end),
		Name:  name,
		Index: captureIndex,
	}
	if err := p.addCaptureName(capname); err != nil {
		return CaptureName{}, err
	}
	return capname, nil
}

// parseFlags parses a sequence of flags at the position of the parser. It
// moves the parser to the character after the flags, which is a : or a ).
//
// parseFlags returns an error if a flag occurs twice, if the negation occurs
// twice, or if no flag follows a negation.
//
// parseFlags is ParserI::parse_flags.
func (p *parserI) parseFlags() (Flags, error) {
	flags := Flags{Span: p.span()}
	var lastWasNegation Span
	hasLastWasNegation := false
	for p.char() != ':' && p.char() != ')' {
		if p.char() == '-' {
			lastWasNegation, hasLastWasNegation = p.spanChar(), true
			item := FlagsItem{
				Span: p.spanChar(),
				Kind: FlagsItemNegation,
			}
			if i, dup := flags.AddItem(item); dup {
				err := p.error(p.spanChar(), FlagRepeatedNegation)
				err.Original = flags.Items[i].Span
				return Flags{}, err
			}
		} else {
			hasLastWasNegation = false
			span := p.spanChar()
			flag, err := p.parseFlag()
			if err != nil {
				return Flags{}, err
			}
			item := FlagsItem{
				Span: span,
				Kind: FlagsItemFlag,
				Flag: flag,
			}
			if i, dup := flags.AddItem(item); dup {
				err := p.error(p.spanChar(), FlagDuplicate)
				err.Original = flags.Items[i].Span
				return Flags{}, err
			}
		}
		if !p.bump() {
			return Flags{}, p.error(p.span(), FlagUnexpectedEOF)
		}
	}
	if hasLastWasNegation {
		return Flags{}, p.error(lastWasNegation, FlagDanglingNegation)
	}
	flags.Span.End = p.pos()
	return flags, nil
}

// parseFlag parses the character at the position of the parser as a flag,
// and does not move the parser. If the flag is not known, parseFlag returns
// an error.
//
// parseFlag is ParserI::parse_flag.
func (p *parserI) parseFlag() (Flag, error) {
	switch p.char() {
	case 'i':
		return FlagCaseInsensitive, nil
	case 'm':
		return FlagMultiLine, nil
	case 's':
		return FlagDotMatchesNewLine, nil
	case 'U':
		return FlagSwapGreed, nil
	case 'u':
		return FlagUnicode, nil
	case 'R':
		return FlagCRLF, nil
	case 'x':
		return FlagIgnoreWhitespace, nil
	}
	return 0, p.error(p.spanChar(), FlagUnrecognized)
}

// parsePrimitive parses a primitive: a literal, a class that is not a set,
// or an assertion. The caller handles every other case first. For example, a
// | at the position of the parser becomes a literal, as in a class.
//
// parsePrimitive moves the parser past the primitive.
//
// parsePrimitive is ParserI::parse_primitive.
func (p *parserI) parsePrimitive() (primitive, error) {
	switch c := p.char(); c {
	case '\\':
		return p.parseEscape()
	case '.':
		ast := &Dot{Span: p.spanChar()}
		p.bump()
		return ast, nil
	case '^':
		ast := &Assertion{
			Span: p.spanChar(),
			Kind: AssertionStartLine,
		}
		p.bump()
		return ast, nil
	case '$':
		ast := &Assertion{
			Span: p.spanChar(),
			Kind: AssertionEndLine,
		}
		p.bump()
		return ast, nil
	default:
		ast := &Literal{
			Span: p.spanChar(),
			Kind: LiteralVerbatim,
			C:    c,
		}
		p.bump()
		return ast, nil
	}
}

// parseEscape parses an escape sequence as a primitive. The parser must be
// at the \. parseEscape moves it past the escape sequence.
//
// parseEscape is ParserI::parse_escape.
func (p *parserI) parseEscape() (primitive, error) {
	if p.char() != '\\' {
		panic("assertion failed: the parser is not at \\")
	}
	start := p.pos()
	if !p.bump() {
		return nil, p.error(NewSpan(start, p.pos()), EscapeUnexpectedEOF)
	}
	c := p.char()
	// The longer sequences have functions of their own.
	switch {
	case '0' <= c && c <= '7':
		if !p.parser.octal {
			return nil, p.error(NewSpan(start, p.spanChar().End), UnsupportedBackreference)
		}
		lit := p.parseOctal()
		lit.Span.Start = start
		return lit, nil
	case ('8' <= c && c <= '9') && !p.parser.octal:
		return nil, p.error(NewSpan(start, p.spanChar().End), UnsupportedBackreference)
	case c == 'x' || c == 'u' || c == 'U':
		lit, err := p.parseHex()
		if err != nil {
			return nil, err
		}
		lit.Span.Start = start
		return lit, nil
	case c == 'p' || c == 'P':
		cls, err := p.parseUnicodeClass()
		if err != nil {
			return nil, err
		}
		cls.Span.Start = start
		return cls, nil
	case c == 'd' || c == 's' || c == 'w' || c == 'D' || c == 'S' || c == 'W':
		cls := p.parsePerlClass()
		cls.Span.Start = start
		return cls, nil
	}

	// The sequences of one letter follow.
	p.bump()
	span := NewSpan(start, p.pos())
	if isMetaCharacter(c) {
		return &Literal{
			Span: span,
			Kind: LiteralMeta,
			C:    c,
		}, nil
	}
	if isEscapeableCharacter(c) {
		return &Literal{
			Span: span,
			Kind: LiteralSuperfluous,
			C:    c,
		}, nil
	}
	special := func(kind SpecialLiteralKind, c rune) (primitive, error) {
		return &Literal{
			Span:    span,
			Kind:    LiteralSpecial,
			Special: kind,
			C:       c,
		}, nil
	}
	switch c {
	case 'a':
		return special(SpecialLiteralBell, '\x07')
	case 'f':
		return special(SpecialLiteralFormFeed, '\x0C')
	case 't':
		return special(SpecialLiteralTab, '\t')
	case 'n':
		return special(SpecialLiteralLineFeed, '\n')
	case 'r':
		return special(SpecialLiteralCarriageReturn, '\r')
	case 'v':
		return special(SpecialLiteralVerticalTab, '\x0B')
	case 'A':
		return &Assertion{
			Span: span,
			Kind: AssertionStartText,
		}, nil
	case 'z':
		return &Assertion{
			Span: span,
			Kind: AssertionEndText,
		}, nil
	case 'b':
		wb := &Assertion{
			Span: span,
			Kind: AssertionWordBoundary,
		}
		// After a \b, the parser tries to read a special word boundary,
		// such as \b{start}.
		if !p.isEOF() && p.char() == '{' {
			kind, ok, err := p.maybeParseSpecialWordBoundary(start)
			if err != nil {
				return nil, err
			}
			if ok {
				wb.Kind = kind
				wb.Span.End = p.pos()
			}
		}
		return wb, nil
	case 'B':
		return &Assertion{
			Span: span,
			Kind: AssertionNotWordBoundary,
		}, nil
	case '<':
		return &Assertion{
			Span: span,
			Kind: AssertionWordBoundaryStartAngle,
		}, nil
	case '>':
		return &Assertion{
			Span: span,
			Kind: AssertionWordBoundaryEndAngle,
		}, nil
	}
	return nil, p.error(span, EscapeUnrecognized)
}

// maybeParseSpecialWordBoundary tries to parse a special word boundary:
// \b{start}, \b{end}, \b{start-half} or \b{end-half}. It returns the kind
// and true.
//
// In most cases, when it cannot, it returns false and no error, as
// maybeParseASCIIClass does. \b{5} is a valid counted repetition, and the
// parser of counted repetitions reads it. But when the braces cannot be a
// counted repetition, maybeParseSpecialWordBoundary returns an error for a
// special word boundary.
//
// The parser must be at a { after a \b. When maybeParseSpecialWordBoundary
// returns false, the parser is back at the {. The position wbStart is the
// start of the \b.
//
// maybeParseSpecialWordBoundary is
// ParserI::maybe_parse_special_word_boundary.
func (p *parserI) maybeParseSpecialWordBoundary(wbStart Position) (AssertionKind, bool, error) {
	if p.char() != '{' {
		panic("assertion failed: the parser is not at {")
	}

	isValidChar := func(c rune) bool {
		return ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || c == '-'
	}
	start := p.pos()
	if !p.bumpAndBumpSpace() {
		return 0, false, p.error(NewSpan(wbStart, p.pos()), SpecialWordOrRepetitionUnexpectedEOF)
	}
	startContents := p.pos()
	// If the first character that is not white space cannot be in a special
	// word boundary, give up and let the parser of counted repetitions read
	// the braces.
	if !isValidChar(p.char()) {
		p.parser.pos = start
		return 0, false, nil
	}

	// Collect the characters until a }.
	scratch := p.parser.scratch[:0]
	for !p.isEOF() && isValidChar(p.char()) {
		scratch = utf8.AppendRune(scratch, p.char())
		p.bumpAndBumpSpace()
	}
	p.parser.scratch = scratch
	if p.isEOF() || p.char() != '}' {
		return 0, false, p.error(NewSpan(start, p.pos()), SpecialWordBoundaryUnclosed)
	}
	end := p.pos()
	p.bump()
	switch string(scratch) {
	case "start":
		return AssertionWordBoundaryStart, true, nil
	case "end":
		return AssertionWordBoundaryEnd, true, nil
	case "start-half":
		return AssertionWordBoundaryStartHalf, true, nil
	case "end-half":
		return AssertionWordBoundaryEndHalf, true, nil
	}
	return 0, false, p.error(NewSpan(startContents, end), SpecialWordBoundaryUnrecognized)
}

// parseOctal parses an octal code point of up to three digits. The parser
// must be at the first digit, and the parser must accept octal. parseOctal
// moves the parser past the number. It cannot fail when those conditions
// hold.
//
// parseOctal is ParserI::parse_octal.
func (p *parserI) parseOctal() *Literal {
	if !p.parser.octal {
		panic("assertion failed: octal is off")
	}
	if c := p.char(); c < '0' || c > '7' {
		panic("assertion failed: the parser is not at an octal digit")
	}
	start := p.pos()
	// Parse up to two more digits.
	for p.bump() && '0' <= p.char() && p.char() <= '7' && p.pos().Offset-start.Offset <= 2 {
		continue
	}
	end := p.pos()
	octal := p.pattern[start.Offset:end.Offset]
	// The loop above makes a valid number, so the parse does not fail.
	codepoint, err := strconv.ParseUint(octal, 8, 32)
	if err != nil {
		panic("valid octal number")
	}
	// The largest octal number of three digits is 0777, which is 511. Each
	// value from 0 to 511 is a Unicode scalar value.
	return &Literal{
		Span: NewSpan(start, end),
		Kind: LiteralOctal,
		C:    rune(codepoint),
	}
}

// parseHex parses a hexadecimal code point, as \xFF or as \x{FFFF}. The
// parser must be at the prefix: x, u or U. parseHex moves it past the
// literal.
//
// parseHex is ParserI::parse_hex.
func (p *parserI) parseHex() (*Literal, error) {
	if c := p.char(); c != 'x' && c != 'u' && c != 'U' {
		panic("assertion failed: the parser is not at x, u or U")
	}

	var hexKind HexLiteralKind
	switch p.char() {
	case 'x':
		hexKind = HexLiteralX
	case 'u':
		hexKind = HexLiteralUnicodeShort
	default:
		hexKind = HexLiteralUnicodeLong
	}
	if !p.bumpAndBumpSpace() {
		return nil, p.error(p.span(), EscapeUnexpectedEOF)
	}
	if p.char() == '{' {
		return p.parseHexBrace(hexKind)
	}
	return p.parseHexDigits(hexKind)
}

// parseHexDigits parses a hexadecimal code point with a fixed number of
// digits: 2 for \xNN, 4 for \uNNNN and 8 for \UNNNNNNNN. The parser must be
// at the first digit. parseHexDigits moves it past the escape sequence.
//
// parseHexDigits is ParserI::parse_hex_digits.
func (p *parserI) parseHexDigits(kind HexLiteralKind) (*Literal, error) {
	scratch := p.parser.scratch[:0]

	start := p.pos()
	for i := range kind.Digits() {
		if i > 0 && !p.bumpAndBumpSpace() {
			return nil, p.error(p.span(), EscapeUnexpectedEOF)
		}
		if !isHex(p.char()) {
			return nil, p.error(p.spanChar(), EscapeHexInvalidDigit)
		}
		scratch = utf8.AppendRune(scratch, p.char())
	}
	p.parser.scratch = scratch
	// The last bump moves the parser past the literal, which can be the end
	// of the pattern.
	p.bumpAndBumpSpace()
	end := p.pos()
	c, ok := charFromHex(string(scratch))
	if !ok {
		return nil, p.error(NewSpan(start, end), EscapeHexInvalid)
	}
	return &Literal{
		Span: NewSpan(start, end),
		Kind: LiteralHexFixed,
		Hex:  kind,
		C:    c,
	}, nil
}

// parseHexBrace parses a hexadecimal Unicode scalar value in braces. The
// parser must be at the {. parseHexBrace moves it past the }.
//
// parseHexBrace is ParserI::parse_hex_brace.
func (p *parserI) parseHexBrace(kind HexLiteralKind) (*Literal, error) {
	scratch := p.parser.scratch[:0]

	bracePos := p.pos()
	start := p.spanChar().End
	for p.bumpAndBumpSpace() && p.char() != '}' {
		if !isHex(p.char()) {
			return nil, p.error(p.spanChar(), EscapeHexInvalidDigit)
		}
		scratch = utf8.AppendRune(scratch, p.char())
	}
	p.parser.scratch = scratch
	if p.isEOF() {
		return nil, p.error(NewSpan(bracePos, p.pos()), EscapeUnexpectedEOF)
	}
	end := p.pos()
	hex := string(scratch)
	if p.char() != '}' {
		panic("assertion failed: the parser is not at }")
	}
	p.bumpAndBumpSpace()

	if hex == "" {
		return nil, p.error(NewSpan(bracePos, p.pos()), EscapeHexEmpty)
	}
	c, ok := charFromHex(hex)
	if !ok {
		return nil, p.error(NewSpan(start, end), EscapeHexInvalid)
	}
	return &Literal{
		Span: NewSpan(start, p.pos()),
		Kind: LiteralHexBrace,
		Hex:  kind,
		C:    c,
	}, nil
}

// charFromHex returns the Unicode scalar value of a hexadecimal number, and
// true. If the number does not fit in 32 bits, or is not a Unicode scalar
// value, charFromHex returns false.
//
// charFromHex is u32::from_str_radix(hex, 16).ok().and_then(char::from_u32)
// of upstream.
func charFromHex(hex string) (rune, bool) {
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil || !utf8.ValidRune(rune(n)) {
		return 0, false
	}
	return rune(n), true
}

// parseDecimal parses a decimal number into a uint32, and skips the white
// space before and after it.
//
// The parser must be at the first place where a digit can be. parseDecimal
// moves it past the last digit of the number. If there is no digit, or the
// number does not fit in a uint32, parseDecimal returns an error.
//
// parseDecimal is ParserI::parse_decimal.
func (p *parserI) parseDecimal() (uint32, error) {
	scratch := p.parser.scratch[:0]

	for !p.isEOF() && isWhitespace(p.char()) {
		p.bump()
	}
	start := p.pos()
	for !p.isEOF() && '0' <= p.char() && p.char() <= '9' {
		scratch = utf8.AppendRune(scratch, p.char())
		p.bumpAndBumpSpace()
	}
	span := NewSpan(start, p.pos())
	for !p.isEOF() && isWhitespace(p.char()) {
		p.bumpAndBumpSpace()
	}
	p.parser.scratch = scratch
	if len(scratch) == 0 {
		return 0, p.error(span, DecimalEmpty)
	}
	n, err := strconv.ParseUint(string(scratch), 10, 32)
	if err != nil {
		return 0, p.error(span, DecimalInvalid)
	}
	return uint32(n), nil
}

// parseSetClass parses a class in brackets. It holds mostly characters and
// ranges, and it can hold nested classes of any kind but the dot.
//
// The parser must be at the [. When parseSetClass succeeds, the parser is
// past the ].
//
// parseSetClass is ParserI::parse_set_class.
func (p *parserI) parseSetClass() (*ClassBracketed, error) {
	if p.char() != '[' {
		panic("assertion failed: the parser is not at [")
	}

	union := &ClassSetUnion{Span: p.span()}
	for {
		p.bumpSpace()
		if p.isEOF() {
			return nil, p.unclosedClassError()
		}
		next, hasNext := p.peek()
		switch c := p.char(); {
		case c == '[':
			// Inside a class, the [ can start an ASCII class. If it does
			// not, the parser is back at the [.
			if len(p.parser.stackClass) != 0 {
				if cls := p.maybeParseASCIIClass(); cls != nil {
					union.Push(cls)
					continue
				}
			}
			var err error
			union, err = p.pushClassOpen(union)
			if err != nil {
				return nil, err
			}
		case c == ']':
			nestedUnion, class := p.popClass(union)
			if class != nil {
				return class, nil
			}
			union = nestedUnion
		case c == '&' && hasNext && next == '&':
			if !p.bumpIf("&&") {
				panic("assertion failed: the parser is not at &&")
			}
			union = p.pushClassOp(ClassSetBinaryOpIntersection, union)
		case c == '-' && hasNext && next == '-':
			if !p.bumpIf("--") {
				panic("assertion failed: the parser is not at --")
			}
			union = p.pushClassOp(ClassSetBinaryOpDifference, union)
		case c == '~' && hasNext && next == '~':
			if !p.bumpIf("~~") {
				panic("assertion failed: the parser is not at ~~")
			}
			union = p.pushClassOp(ClassSetBinaryOpSymmetricDifference, union)
		default:
			item, err := p.parseSetClassRange()
			if err != nil {
				return nil, err
			}
			union.Push(item)
		}
	}
}

// parseSetClassRange parses one item of a class: a literal, a range of two
// literals, or a class that is not a set, such as \w or \p{Greek}.
//
// If an escape is not valid, or a class is where a literal must be, as in a
// range, parseSetClassRange returns an error.
//
// parseSetClassRange is ParserI::parse_set_class_range.
func (p *parserI) parseSetClassRange() (ClassSetItem, error) {
	prim1, err := p.parseSetClassItem()
	if err != nil {
		return nil, err
	}
	p.bumpSpace()
	if p.isEOF() {
		return nil, p.unclosedClassError()
	}
	// If the next character is not a -, this is not a range. There are two
	// more cases. A - before a ] is a literal -. A - before a - starts a
	// difference.
	if p.char() != '-' {
		return intoClassSetItem(prim1, p)
	}
	if c, ok := p.peekSpace(); ok && (c == ']' || c == '-') {
		return intoClassSetItem(prim1, p)
	}
	// This is a range, so move past the - and parse the end of the range.
	if !p.bumpAndBumpSpace() {
		return nil, p.unclosedClassError()
	}
	prim2, err := p.parseSetClassItem()
	if err != nil {
		return nil, err
	}
	span := NewSpan(prim1.GetSpan().Start, prim2.GetSpan().End)
	start, err := intoClassLiteral(prim1, p)
	if err != nil {
		return nil, err
	}
	end, err := intoClassLiteral(prim2, p)
	if err != nil {
		return nil, err
	}
	rng := &ClassSetRange{
		Span:  span,
		Start: *start,
		End:   *end,
	}
	if !rng.IsValid() {
		return nil, p.error(rng.Span, ClassRangeInvalid)
	}
	return rng, nil
}

// parseSetClassItem parses one item of a class as a primitive: a literal
// as it is, or one escape sequence. The parser must be at the start of the
// primitive. When parseSetClassItem succeeds, the parser is past the
// primitive.
//
// The caller reports an error for a primitive that is not valid.
//
// parseSetClassItem is ParserI::parse_set_class_item.
func (p *parserI) parseSetClassItem() (primitive, error) {
	if p.char() == '\\' {
		return p.parseEscape()
	}
	x := &Literal{
		Span: p.spanChar(),
		Kind: LiteralVerbatim,
		C:    p.char(),
	}
	p.bump()
	return x, nil
}

// parseSetClassOpen parses the opening of a class: the [, and the ^ of a
// negated class. It also parses the first items of the union, because some
// characters have special rules at the opening. For example, [^]] is the
// class of each character but ], and ] needs an escape in any other place.
// The - has a rule too.
//
// The set of the returned class is always an empty union. The parser puts
// the real set in its place when it pops the class from its stack.
//
// The parser must be at the [. parseSetClassOpen moves it to the first
// character of the class that is not special. At the end of the pattern,
// parseSetClassOpen returns an error.
//
// parseSetClassOpen is ParserI::parse_set_class_open.
func (p *parserI) parseSetClassOpen() (*ClassBracketed, *ClassSetUnion, error) {
	if p.char() != '[' {
		panic("assertion failed: the parser is not at [")
	}
	start := p.pos()
	if !p.bumpAndBumpSpace() {
		return nil, nil, p.error(NewSpan(start, p.pos()), ClassUnclosed)
	}

	negated := false
	if p.char() == '^' {
		if !p.bumpAndBumpSpace() {
			return nil, nil, p.error(NewSpan(start, p.pos()), ClassUnclosed)
		}
		negated = true
	}
	// Each - at the start is a literal -.
	union := &ClassSetUnion{Span: p.span()}
	for p.char() == '-' {
		union.Push(&Literal{
			Span: p.spanChar(),
			Kind: LiteralVerbatim,
			C:    '-',
		})
		if !p.bumpAndBumpSpace() {
			return nil, nil, p.error(NewSpan(start, start), ClassUnclosed)
		}
	}
	// A ] as the first character of a set is a literal ]. So no pattern can
	// spell an empty class.
	if len(union.Items) == 0 && p.char() == ']' {
		union.Push(&Literal{
			Span: p.spanChar(),
			Kind: LiteralVerbatim,
			C:    ']',
		})
		if !p.bumpAndBumpSpace() {
			return nil, nil, p.error(NewSpan(start, p.pos()), ClassUnclosed)
		}
	}
	set := &ClassBracketed{
		Span:    NewSpan(start, p.pos()),
		Negated: negated,
		Kind: &ClassSetUnion{
			Span: NewSpan(union.Span.Start, union.Span.Start),
		},
	}
	return set, union, nil
}

// maybeParseASCIIClass tries to parse an ASCII class, such as [:alnum:]. The
// parser must be at the [.
//
// If there is no valid ASCII class, maybeParseASCIIClass does not move the
// parser, and returns nil. Otherwise it moves the parser past the ], and
// returns the class.
//
// maybeParseASCIIClass is ParserI::maybe_parse_ascii_class.
func (p *parserI) maybeParseASCIIClass() *ClassASCII {
	// A parse of an ASCII class cannot fail with an error. An ASCII class
	// has the syntax [:NAME:], and it can occur only in a class, as in
	// [[:alnum:]] or [[:lower:]A].
	//
	// If a pattern spells a wrong ASCII class, such as [[:loower:]], the
	// parser reads a nested class with the characters :elorw. The colons show
	// that the user wanted an ASCII class, but [[:lower]] can also be a
	// nested class, and the parser cannot tell. So upstream chose a syntax
	// that never fails, at the cost of worse errors.
	if p.char() != '[' {
		panic("assertion failed: the parser is not at [")
	}
	// If the parse fails, the parser goes back to this position.
	start := p.pos()
	negated := false
	if !p.bump() || p.char() != ':' {
		p.parser.pos = start
		return nil
	}
	if !p.bump() {
		p.parser.pos = start
		return nil
	}
	if p.char() == '^' {
		negated = true
		if !p.bump() {
			p.parser.pos = start
			return nil
		}
	}
	nameStart := p.offset()
	for p.char() != ':' && p.bump() {
		continue
	}
	if p.isEOF() {
		p.parser.pos = start
		return nil
	}
	name := p.pattern[nameStart:p.offset()]
	if !p.bumpIf(":]") {
		p.parser.pos = start
		return nil
	}
	kind, ok := ClassASCIIKindFromName(name)
	if !ok {
		p.parser.pos = start
		return nil
	}
	return &ClassASCII{
		Span:    NewSpan(start, p.pos()),
		Kind:    kind,
		Negated: negated,
	}
}

// parseUnicodeClass parses a Unicode class, as \pN with one letter or as
// \p{Greek} with a name in braces. The parser must be at the p, or at the P
// of a negated class. parseUnicodeClass moves it past the class.
//
// parseUnicodeClass does not make sure that the name of the class is valid.
//
// parseUnicodeClass is ParserI::parse_unicode_class.
func (p *parserI) parseUnicodeClass() (*ClassUnicode, error) {
	if c := p.char(); c != 'p' && c != 'P' {
		panic("assertion failed: the parser is not at p or P")
	}

	scratch := p.parser.scratch[:0]

	negated := p.char() == 'P'
	if !p.bumpAndBumpSpace() {
		return nil, p.error(p.span(), EscapeUnexpectedEOF)
	}
	class := &ClassUnicode{Negated: negated}
	var start Position
	if p.char() == '{' {
		start = p.spanChar().End
		for p.bumpAndBumpSpace() && p.char() != '}' {
			scratch = utf8.AppendRune(scratch, p.char())
		}
		p.parser.scratch = scratch
		if p.isEOF() {
			return nil, p.error(p.span(), EscapeUnexpectedEOF)
		}
		if p.char() != '}' {
			panic("assertion failed: the parser is not at }")
		}
		p.bump()

		name := string(scratch)
		if before, after, ok := strings.Cut(name, "!="); ok {
			class.Kind = ClassUnicodeNamedValue
			class.Op = ClassUnicodeOpNotEqual
			class.Name = before
			class.Value = after
		} else if before, after, ok := strings.Cut(name, ":"); ok {
			class.Kind = ClassUnicodeNamedValue
			class.Op = ClassUnicodeOpColon
			class.Name = before
			class.Value = after
		} else if before, after, ok := strings.Cut(name, "="); ok {
			class.Kind = ClassUnicodeNamedValue
			class.Op = ClassUnicodeOpEqual
			class.Name = before
			class.Value = after
		} else {
			class.Kind = ClassUnicodeNamed
			class.Name = name
		}
	} else {
		start = p.pos()
		c := p.char()
		if c == '\\' {
			return nil, p.error(p.spanChar(), UnicodeClassInvalid)
		}
		p.bumpAndBumpSpace()
		class.Kind = ClassUnicodeOneLetter
		class.Letter = c
	}
	class.Span = NewSpan(start, p.pos())
	return class, nil
}

// parsePerlClass parses a Perl class, such as \d or \W. The parser must be
// at the name of a valid class. parsePerlClass moves it past the class.
//
// parsePerlClass is ParserI::parse_perl_class.
func (p *parserI) parsePerlClass() *ClassPerl {
	c := p.char()
	span := p.spanChar()
	p.bump()
	var negated bool
	var kind ClassPerlKind
	switch c {
	case 'd':
		negated, kind = false, ClassPerlDigit
	case 'D':
		negated, kind = true, ClassPerlDigit
	case 's':
		negated, kind = false, ClassPerlSpace
	case 'S':
		negated, kind = true, ClassPerlSpace
	case 'w':
		negated, kind = false, ClassPerlWord
	case 'W':
		negated, kind = true, ClassPerlWord
	default:
		panic(fmt.Sprintf("expected valid Perl class but got '%c'", c))
	}
	return &ClassPerl{Span: span, Kind: kind, Negated: negated}
}

// nestLimiter walks a whole syntax tree, and returns an error if the depth
// of the tree is more than the nest limit.
//
// nestLimiter is NestLimiter.
type nestLimiter struct {
	BaseVisitor

	// p is the parser that checks the nest limit.
	p *parserI
	// depth is the depth of the walk.
	depth uint32
}

// newNestLimiter returns a nestLimiter for the parser, at depth 0.
//
// newNestLimiter is NestLimiter::new.
func newNestLimiter(p *parserI) *nestLimiter {
	return &nestLimiter{p: p, depth: 0}
}

// check walks the tree, and returns an error if the tree is too deep.
//
// check is NestLimiter::check.
func (n *nestLimiter) check(ast Ast) error {
	_, err := Visit[struct{}](ast, n)
	return err
}

// incrementDepth adds one to the depth, or returns an error with the span if
// the depth is more than the limit.
//
// incrementDepth is NestLimiter::increment_depth.
func (n *nestLimiter) incrementDepth(span Span) error {
	if n.depth == math.MaxUint32 {
		err := n.p.error(span, NestLimitExceeded)
		err.Limit = math.MaxUint32
		return err
	}
	depth := n.depth + 1
	limit := n.p.parser.nestLimit
	if depth > limit {
		err := n.p.error(span, NestLimitExceeded)
		err.Limit = limit
		return err
	}
	n.depth = depth
	return nil
}

// decrementDepth takes one from the depth. The depth is never 0 here when
// the visitor is right.
//
// decrementDepth is NestLimiter::decrement_depth.
func (n *nestLimiter) decrementDepth() {
	if n.depth == 0 {
		panic("called `Option::unwrap()` on a `None` value")
	}
	n.depth--
}

// Finish returns no output and no error.
//
// Finish is NestLimiter::finish.
func (n *nestLimiter) Finish() (struct{}, error) {
	return struct{}{}, nil
}

// VisitPre adds one to the depth for a tree with children.
//
// VisitPre is NestLimiter::visit_pre.
func (n *nestLimiter) VisitPre(ast Ast) error {
	switch ast.(type) {
	case *Empty, *SetFlags, *Literal, *Dot, *Assertion, *ClassUnicode, *ClassPerl:
		// These are base cases, so the depth stays the same.
		return nil
	}
	return n.incrementDepth(ast.GetSpan())
}

// VisitPost takes one from the depth for a tree with children.
//
// VisitPost is NestLimiter::visit_post.
func (n *nestLimiter) VisitPost(ast Ast) error {
	switch ast.(type) {
	case *Empty, *SetFlags, *Literal, *Dot, *Assertion, *ClassUnicode, *ClassPerl:
		// These are base cases, so the depth stays the same.
		return nil
	}
	n.decrementDepth()
	return nil
}

// VisitClassSetItemPre adds one to the depth for an item with children.
//
// VisitClassSetItemPre is NestLimiter::visit_class_set_item_pre.
func (n *nestLimiter) VisitClassSetItemPre(ast ClassSetItem) error {
	switch ast.(type) {
	case *Empty, *Literal, *ClassSetRange, *ClassASCII, *ClassUnicode, *ClassPerl:
		// These are base cases, so the depth stays the same.
		return nil
	}
	return n.incrementDepth(ast.GetSpan())
}

// VisitClassSetItemPost takes one from the depth for an item with children.
//
// VisitClassSetItemPost is NestLimiter::visit_class_set_item_post.
func (n *nestLimiter) VisitClassSetItemPost(ast ClassSetItem) error {
	switch ast.(type) {
	case *Empty, *Literal, *ClassSetRange, *ClassASCII, *ClassUnicode, *ClassPerl:
		// These are base cases, so the depth stays the same.
		return nil
	}
	n.decrementDepth()
	return nil
}

// VisitClassSetBinaryOpPre adds one to the depth.
//
// VisitClassSetBinaryOpPre is NestLimiter::visit_class_set_binary_op_pre.
func (n *nestLimiter) VisitClassSetBinaryOpPre(ast *ClassSetBinaryOp) error {
	return n.incrementDepth(ast.Span)
}

// VisitClassSetBinaryOpPost takes one from the depth.
//
// VisitClassSetBinaryOpPost is NestLimiter::visit_class_set_binary_op_post.
func (n *nestLimiter) VisitClassSetBinaryOpPost(*ClassSetBinaryOp) error {
	n.decrementDepth()
	return nil
}

// specializeErr changes the kind of an error from one kind to another, for a
// clearer error. It returns any other error as it is.
//
// specializeErr is specialize_err.
func specializeErr(err error, from, to ErrorKind) error {
	var e *Error
	if errors.As(err, &e) && e.Kind == from {
		return &Error{Kind: to, Pattern: e.Pattern, Span: e.Span}
	}
	return err
}
