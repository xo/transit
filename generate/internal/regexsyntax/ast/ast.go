// Package ast is the syntax tree of a regular expression, its parser and a
// walk of the tree. It ports the module ast of the Rust crate regex-syntax
// 0.8.11, which the generator of tree-sitter uses to read the pattern of a
// token (D59).
package ast

import (
	"fmt"
	"strconv"
)

// This file ports src/ast/mod.rs: the types of the syntax tree, and the error
// of the parser.
//
// A Rust enum with data becomes one of two Go forms:
//
//   - Ast, ClassSet and ClassSetItem become interfaces. Each variant is a
//     pointer to a struct. A caller can change a node in place, and can
//     replace a child through a pointer to the field or the slice element
//     that holds it. A struct that is a variant of more than one enum, such as
//     Literal, implements each interface. A ClassSetItem is a ClassSet, which
//     stands for ClassSet::Item.
//   - A small enum, such as ErrorKind, LiteralKind or GroupKind, becomes a
//     typed constant. The data of a variant is a field of the struct that
//     holds the kind, and the field is zero for the other variants.
//
// The port leaves out these items of upstream:
//
//   - The Display of Error, which calls the Formatter of error.rs. tree-sitter
//     writes the text of an error itself, from the kind, the span and the
//     auxiliary span. Error returns the Display of the kind only.
//   - The Display of Ast, which calls print.rs.
//   - The Drop of Ast and of ClassSet, and Ast::has_subexprs and
//     ClassSet::is_empty, which only Drop uses. The garbage collector frees a
//     tree.
//   - The Arbitrary of ClassUnicodeKind and CaptureName, which only the fuzz
//     tests of upstream use.
//   - The constructors Ast::empty to Ast::concat, and ClassSet::union. A
//     composite literal, such as &Empty{Span: span}, does the same work.
//
// A method of upstream that reads a field of the same name, such as
// Ast::span, becomes a method with the prefix Get, because a Go struct cannot
// have a field and a method with the same name.

// Error is an error of the parser. The text of the error is the Display of
// its kind.
//
// Not every syntax tree is a valid regular expression. For example, the
// parser accepts \p{Quux}, but Quux is not the name of a Unicode property.
// The translator to the HIR reports that error.
//
// Error is Error. The data of the variants of upstream ErrorKind is in the
// fields Original and Limit.
type Error struct {
	// Kind is the kind of the error.
	Kind ErrorKind
	// Pattern is the pattern that the parser read. Every span of the error is
	// a valid range of this string.
	Pattern string
	// Span is the span of the error.
	Span Span
	// Original is the span of the first occurrence, for FlagDuplicate,
	// FlagRepeatedNegation and GroupNameDuplicate. Span points to the second
	// occurrence.
	Original Span
	// Limit is the limit of the parser, for NestLimitExceeded.
	Limit uint32
}

// AuxiliarySpan returns a second span of the error, and true, for an error
// that points to two places in the pattern. A "duplicate" error has its span
// on the second occurrence, and its auxiliary span on the first. For other
// errors, AuxiliarySpan returns false.
//
// AuxiliarySpan is Error::auxiliary_span.
func (e *Error) AuxiliarySpan() (Span, bool) {
	switch e.Kind {
	case FlagDuplicate, FlagRepeatedNegation, GroupNameDuplicate:
		return e.Original, true
	}
	return Span{}, false
}

// ErrorKind is the kind of an error of the parser.
//
// ErrorKind is ErrorKind.
type ErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	// CaptureLimitExceeded is an error for too many capturing groups. The
	// limit is on the number of all capturing groups, not on their depth.
	CaptureLimitExceeded ErrorKind = iota
	// ClassEscapeInvalid is an escape sequence that a class cannot hold.
	ClassEscapeInvalid
	// ClassRangeInvalid is a range of a class with a start after its end.
	ClassRangeInvalid
	// ClassRangeLiteral is an end of a range that is not a literal, such as
	// a nested class.
	ClassRangeLiteral
	// ClassUnclosed is a [ with no ].
	ClassUnclosed
	// DecimalEmpty is a decimal number with no digits. The parser no longer
	// returns it, because it returns RepetitionCountDecimalEmpty.
	DecimalEmpty
	// DecimalInvalid is a decimal number that is not valid.
	DecimalInvalid
	// EscapeHexEmpty is a hexadecimal literal in braces with no digits.
	EscapeHexEmpty
	// EscapeHexInvalid is a hexadecimal literal in braces that is not a
	// Unicode scalar value.
	EscapeHexInvalid
	// EscapeHexInvalidDigit is a digit that is not hexadecimal.
	EscapeHexInvalidDigit
	// EscapeUnexpectedEOF is the end of the pattern in an escape sequence.
	EscapeUnexpectedEOF
	// EscapeUnrecognized is an escape sequence that the parser does not know.
	EscapeUnrecognized
	// FlagDanglingNegation is a negation with no flag after it, such as i-.
	FlagDanglingNegation
	// FlagDuplicate is a flag that occurs twice, such as i-i. Original holds
	// the span of the first flag.
	FlagDuplicate
	// FlagRepeatedNegation is a negation that occurs twice, such as -i-s.
	// Original holds the span of the first negation.
	FlagRepeatedNegation
	// FlagUnexpectedEOF is the end of the pattern where a flag must be, such
	// as (?.
	FlagUnexpectedEOF
	// FlagUnrecognized is a flag that the parser does not know, such as a.
	FlagUnrecognized
	// GroupNameDuplicate is a capture name that occurs twice. Original holds
	// the span of the first name.
	GroupNameDuplicate
	// GroupNameEmpty is an empty capture name, such as (?P<>abc).
	GroupNameEmpty
	// GroupNameInvalid is a character that a capture name cannot hold. A name
	// cannot start with a digit.
	GroupNameInvalid
	// GroupNameUnexpectedEOF is a capture name with no closing >.
	GroupNameUnexpectedEOF
	// GroupUnclosed is a group with no closing parenthesis, such as (ab. The
	// span is on the opening parenthesis.
	GroupUnclosed
	// GroupUnopened is a closing parenthesis with no group, such as ab).
	GroupUnopened
	// NestLimitExceeded is a tree that is deeper than the limit of the
	// parser. Limit holds that limit.
	NestLimitExceeded
	// RepetitionCountInvalid is a counted repetition with a start after its
	// end.
	RepetitionCountInvalid
	// RepetitionCountDecimalEmpty is a { with no decimal number after it,
	// such as x{} or x{]}.
	RepetitionCountDecimalEmpty
	// RepetitionCountUnclosed is a { with no }.
	RepetitionCountUnclosed
	// RepetitionMissing is a repetition with no expression before it, such as
	// * or (?i)*. A repetition of an empty group, such as ()*, is valid.
	RepetitionMissing
	// SpecialWordBoundaryUnclosed is a \b{ with no }, or with a character
	// that the name of a word boundary cannot hold.
	SpecialWordBoundaryUnclosed
	// SpecialWordBoundaryUnrecognized is a \b{name} with a name that the
	// parser does not know.
	SpecialWordBoundaryUnrecognized
	// SpecialWordOrRepetitionUnexpectedEOF is the end of the pattern after
	// \b{, where the parser cannot tell a repetition from a word boundary.
	SpecialWordOrRepetitionUnexpectedEOF
	// UnicodeClassInvalid is a Unicode class that is not valid, such as a \p
	// with no { after it.
	UnicodeClassInvalid
	// UnsupportedBackreference is an octal escape when the parser does not
	// accept octal. The parser takes it for a backreference.
	UnsupportedBackreference
	// UnsupportedLookAround is a look-around of PCRE, such as (?=re), (?!re),
	// (?<=re) or (?<!re).
	UnsupportedLookAround
)

// Error returns the text of the kind of the error, as the Display of upstream
// ErrorKind writes it. The text does not hold the pattern or the span.
//
// Error is the Display of ErrorKind.
func (e *Error) Error() string {
	switch e.Kind {
	case CaptureLimitExceeded:
		return "exceeded the maximum number of capturing groups (4294967295)"
	case ClassEscapeInvalid:
		return "invalid escape sequence found in character class"
	case ClassRangeInvalid:
		return "invalid character class range, the start must be <= the end"
	case ClassRangeLiteral:
		return "invalid range boundary, must be a literal"
	case ClassUnclosed:
		return "unclosed character class"
	case DecimalEmpty:
		return "decimal literal empty"
	case DecimalInvalid:
		return "decimal literal invalid"
	case EscapeHexEmpty:
		return "hexadecimal literal empty"
	case EscapeHexInvalid:
		return "hexadecimal literal is not a Unicode scalar value"
	case EscapeHexInvalidDigit:
		return "invalid hexadecimal digit"
	case EscapeUnexpectedEOF:
		return "incomplete escape sequence, reached end of pattern prematurely"
	case EscapeUnrecognized:
		return "unrecognized escape sequence"
	case FlagDanglingNegation:
		return "dangling flag negation operator"
	case FlagDuplicate:
		return "duplicate flag"
	case FlagRepeatedNegation:
		return "flag negation operator repeated"
	case FlagUnexpectedEOF:
		return "expected flag but got end of regex"
	case FlagUnrecognized:
		return "unrecognized flag"
	case GroupNameDuplicate:
		return "duplicate capture group name"
	case GroupNameEmpty:
		return "empty capture group name"
	case GroupNameInvalid:
		return "invalid capture group character"
	case GroupNameUnexpectedEOF:
		return "unclosed capture group name"
	case GroupUnclosed:
		return "unclosed group"
	case GroupUnopened:
		return "unopened group"
	case NestLimitExceeded:
		return "exceed the maximum number of nested parentheses/brackets (" + strconv.FormatUint(uint64(e.Limit), 10) + ")"
	case RepetitionCountInvalid:
		return "invalid repetition count range, the start must be <= the end"
	case RepetitionCountDecimalEmpty:
		return "repetition quantifier expects a valid decimal"
	case RepetitionCountUnclosed:
		return "unclosed counted repetition"
	case RepetitionMissing:
		return "repetition operator missing expression"
	case SpecialWordBoundaryUnclosed:
		return "special word boundary assertion is either unclosed or contains an invalid character"
	case SpecialWordBoundaryUnrecognized:
		return "unrecognized special word boundary assertion, valid choices are: start, end, start-half or end-half"
	case SpecialWordOrRepetitionUnexpectedEOF:
		return "found either the beginning of a special word boundary or a bounded repetition on a \\b with an opening brace, but no closing brace"
	case UnicodeClassInvalid:
		return "invalid Unicode character class"
	case UnsupportedBackreference:
		return "backreferences are not supported"
	case UnsupportedLookAround:
		return "look-around, including look-ahead and look-behind, is not supported"
	}
	return ""
}

// Span is the place of one item of the syntax tree in the pattern. Each
// position holds an absolute byte offset in the pattern.
//
// Span is Span.
type Span struct {
	// Start is the position of the start.
	Start Position
	// End is the position of the end.
	End Position
}

// String returns the span in the form Span(start, end).
//
// String is the Debug of Span.
func (s Span) String() string {
	return fmt.Sprintf("Span(%s, %s)", s.Start, s.End)
}

// Compare compares two spans by their starts, and then by their ends. It
// returns -1, 0 or +1, as cmp.Compare does.
//
// Compare is the Ord of Span.
func (s Span) Compare(other Span) int {
	if c := s.Start.Compare(other.Start); c != 0 {
		return c
	}
	return s.End.Compare(other.End)
}

// Position is one end of a span: a byte offset, a line and a column.
//
// Position is Position.
type Position struct {
	// Offset is the byte offset in the pattern, from 0.
	Offset int
	// Line is the line number, from 1.
	Line int
	// Column is the approximate column number, from 1.
	Column int
}

// String returns the position in the form Position(o: 0, l: 1, c: 1).
//
// String is the Debug of Position.
func (p Position) String() string {
	return fmt.Sprintf("Position(o: %d, l: %d, c: %d)", p.Offset, p.Line, p.Column)
}

// Compare compares two positions by their offsets only. It returns -1, 0 or
// +1, as cmp.Compare does.
//
// Compare is the Ord of Position.
func (p Position) Compare(other Position) int {
	switch {
	case p.Offset < other.Offset:
		return -1
	case p.Offset > other.Offset:
		return 1
	}
	return 0
}

// NewSpan returns a span from start to end.
//
// NewSpan is Span::new.
func NewSpan(start, end Position) Span {
	return Span{Start: start, End: end}
}

// SplatSpan returns a span that starts and ends at pos.
//
// SplatSpan is Span::splat.
func SplatSpan(pos Position) Span {
	return NewSpan(pos, pos)
}

// WithStart returns the span with its start replaced by pos.
//
// WithStart is Span::with_start.
func (s Span) WithStart(pos Position) Span {
	s.Start = pos
	return s
}

// WithEnd returns the span with its end replaced by pos.
//
// WithEnd is Span::with_end.
func (s Span) WithEnd(pos Position) Span {
	s.End = pos
	return s
}

// IsOneLine reports whether the span is on one line.
//
// IsOneLine is Span::is_one_line.
func (s Span) IsOneLine() bool {
	return s.Start.Line == s.End.Line
}

// IsEmpty reports whether the span is empty, so that it points to one
// position in the pattern.
//
// IsEmpty is Span::is_empty.
func (s Span) IsEmpty() bool {
	return s.Start.Offset == s.End.Offset
}

// NewPosition returns a position. The offset starts at 0, and the line and the
// column start at 1.
//
// NewPosition is Position::new.
func NewPosition(offset, line, column int) Position {
	return Position{Offset: offset, Line: line, Column: column}
}

// WithComments is a syntax tree with the comments of its pattern. The tree
// does not hold the comments. Each comment has the span of its place in the
// pattern.
//
// WithComments is WithComments.
type WithComments struct {
	// Ast is the syntax tree.
	Ast Ast
	// Comments are the comments of the pattern, in order.
	Comments []Comment
}

// Comment is a comment of a pattern, with its span. A pattern can hold a
// comment only when the flag x is on.
//
// Comment is Comment.
type Comment struct {
	// Span is the span of the comment, from the # to the \n.
	Span Span
	// Comment is the text of the comment, after the # and before the \n.
	Comment string
}

// Ast is the syntax tree of one regular expression. It is one of *Empty,
// *SetFlags, *Literal, *Dot, *Assertion, *ClassUnicode, *ClassPerl,
// *ClassBracketed, *Repetition, *Group, *Alternation and *Concat.
//
// Ast is Ast.
type Ast interface {
	// GetSpan returns the span of the tree.
	GetSpan() Span
	isAst()
}

// Empty is an empty regular expression, which matches everything. As an item
// of a class, it is an empty item. A class cannot hold one empty item alone,
// but an operator can have one on a side, as in [&&].
//
// Empty is Ast::Empty and ClassSetItem::Empty.
type Empty struct {
	Span Span
}

// Dot is the class of any character.
//
// Dot is Ast::Dot.
type Dot struct {
	Span Span
}

// GetSpan returns the span.
//
// GetSpan is Ast::span and ClassSetItem::span.
func (e *Empty) GetSpan() Span { return e.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (f *SetFlags) GetSpan() Span { return f.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span and ClassSetItem::span.
func (l *Literal) GetSpan() Span { return l.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (d *Dot) GetSpan() Span { return d.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (a *Assertion) GetSpan() Span { return a.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span and ClassSetItem::span.
func (c *ClassUnicode) GetSpan() Span { return c.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span and ClassSetItem::span.
func (c *ClassPerl) GetSpan() Span { return c.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span and ClassSetItem::span.
func (c *ClassBracketed) GetSpan() Span { return c.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (r *Repetition) GetSpan() Span { return r.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (g *Group) GetSpan() Span { return g.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (a *Alternation) GetSpan() Span { return a.Span }

// GetSpan returns the span.
//
// GetSpan is Ast::span.
func (c *Concat) GetSpan() Span { return c.Span }

// isAst marks *Empty as an Ast.
func (*Empty) isAst() {}

// isAst marks *SetFlags as an Ast.
func (*SetFlags) isAst() {}

// isAst marks *Literal as an Ast.
func (*Literal) isAst() {}

// isAst marks *Dot as an Ast.
func (*Dot) isAst() {}

// isAst marks *Assertion as an Ast.
func (*Assertion) isAst() {}

// isAst marks *ClassUnicode as an Ast.
func (*ClassUnicode) isAst() {}

// isAst marks *ClassPerl as an Ast.
func (*ClassPerl) isAst() {}

// isAst marks *ClassBracketed as an Ast.
func (*ClassBracketed) isAst() {}

// isAst marks *Repetition as an Ast.
func (*Repetition) isAst() {}

// isAst marks *Group as an Ast.
func (*Group) isAst() {}

// isAst marks *Alternation as an Ast.
func (*Alternation) isAst() {}

// isAst marks *Concat as an Ast.
func (*Concat) isAst() {}

// IsEmpty reports whether ast is an *Empty.
//
// IsEmpty is Ast::is_empty.
func IsEmpty(ast Ast) bool {
	_, ok := ast.(*Empty)
	return ok
}

// Alternation is an alternation of regular expressions.
//
// Alternation is Alternation.
type Alternation struct {
	// Span is the span of the alternation.
	Span Span
	// Asts are the branches.
	Asts []Ast
}

// IntoAst returns the alternation as a tree. With no branches, it returns an
// *Empty. With one branch, it returns that branch. Otherwise it returns a.
//
// IntoAst is Alternation::into_ast.
func (a *Alternation) IntoAst() Ast {
	switch len(a.Asts) {
	case 0:
		return &Empty{Span: a.Span}
	case 1:
		return a.Asts[0]
	}
	return a
}

// Concat is a concatenation of regular expressions.
//
// Concat is Concat.
type Concat struct {
	// Span is the span of the concatenation.
	Span Span
	// Asts are the expressions, in order.
	Asts []Ast
}

// IntoAst returns the concatenation as a tree. With no expressions, it
// returns an *Empty. With one expression, it returns that expression.
// Otherwise it returns c.
//
// IntoAst is Concat::into_ast.
func (c *Concat) IntoAst() Ast {
	switch len(c.Asts) {
	case 0:
		return &Empty{Span: c.Span}
	case 1:
		return c.Asts[0]
	}
	return c
}

// Literal is one literal character, which is one Unicode scalar value. The
// pattern can spell it as it is, such as a, or as an escape, such as \x61.
//
// Literal is Literal. The data of the variants of upstream LiteralKind is in
// the fields Hex and Special.
type Literal struct {
	// Span is the span of the literal.
	Span Span
	// Kind is the way that the pattern spells the literal.
	Kind LiteralKind
	// Hex is the prefix of a hexadecimal literal, for LiteralHexFixed and
	// LiteralHexBrace.
	Hex HexLiteralKind
	// Special is the special escape, for LiteralSpecial.
	Special SpecialLiteralKind
	// C is the Unicode scalar value of the literal.
	C rune
}

// Byte returns the byte of the literal, and true, if the pattern spells it as
// a \x escape and its value fits in a byte. Otherwise, Byte returns false.
//
// Byte is Literal::byte.
func (l *Literal) Byte() (byte, bool) {
	if l.Kind == LiteralHexFixed && l.Hex == HexLiteralX && l.C <= 0xFF {
		return byte(l.C), true
	}
	return 0, false
}

// LiteralKind is the way that a pattern spells a literal.
//
// LiteralKind is LiteralKind.
type LiteralKind uint8

// The kinds of literal.
const (
	// LiteralVerbatim is a literal as it is, such as a or ☃.
	LiteralVerbatim LiteralKind = iota
	// LiteralMeta is an escape of a meta character, such as \* or \[.
	LiteralMeta
	// LiteralSuperfluous is an escape that the literal does not need, such
	// as \% or \/.
	LiteralSuperfluous
	// LiteralOctal is an octal escape, such as \141.
	LiteralOctal
	// LiteralHexFixed is a hexadecimal escape with a fixed number of digits,
	// such as \x61, a or \U00000061.
	LiteralHexFixed
	// LiteralHexBrace is a hexadecimal escape with its digits in braces. The
	// value must be a Unicode scalar value.
	LiteralHexBrace
	// LiteralSpecial is a special escape, such as \f or \n.
	LiteralSpecial
)

// SpecialLiteralKind is the kind of a special escape, such as \f or \n.
//
// SpecialLiteralKind is SpecialLiteralKind.
type SpecialLiteralKind uint8

// The kinds of special escape.
const (
	// SpecialLiteralBell is \a (\x07).
	SpecialLiteralBell SpecialLiteralKind = iota
	// SpecialLiteralFormFeed is \f (\x0C).
	SpecialLiteralFormFeed
	// SpecialLiteralTab is \t (\x09).
	SpecialLiteralTab
	// SpecialLiteralLineFeed is \n (\x0A).
	SpecialLiteralLineFeed
	// SpecialLiteralCarriageReturn is \r (\x0D).
	SpecialLiteralCarriageReturn
	// SpecialLiteralVerticalTab is \v (\x0B).
	SpecialLiteralVerticalTab
	// SpecialLiteralSpace is an escaped space (\x20). It occurs only in
	// verbose mode.
	SpecialLiteralSpace
)

// HexLiteralKind is the prefix of a hexadecimal literal. With braces, all the
// prefixes do the same work. Without braces, the prefix sets the number of
// digits.
//
// HexLiteralKind is HexLiteralKind.
type HexLiteralKind uint8

// The prefixes of a hexadecimal literal.
const (
	// HexLiteralX is \x, with two digits without braces.
	HexLiteralX HexLiteralKind = iota
	// HexLiteralUnicodeShort is \u, with four digits without braces.
	HexLiteralUnicodeShort
	// HexLiteralUnicodeLong is \U, with eight digits without braces.
	HexLiteralUnicodeLong
)

// Digits returns the number of digits that the prefix takes without braces.
// With braces, the number of digits is free.
//
// Digits is HexLiteralKind::digits.
func (k HexLiteralKind) Digits() uint32 {
	switch k {
	case HexLiteralX:
		return 2
	case HexLiteralUnicodeShort:
		return 4
	}
	return 8
}

// ClassPerl is a Perl class, such as \d or \W.
//
// ClassPerl is ClassPerl.
type ClassPerl struct {
	// Span is the span of the class.
	Span Span
	// Kind is the kind of the class.
	Kind ClassPerlKind
	// Negated is true for the negated form, such as \D.
	Negated bool
}

// ClassPerlKind is the kind of a Perl class.
//
// ClassPerlKind is ClassPerlKind.
type ClassPerlKind uint8

// The kinds of Perl class.
const (
	// ClassPerlDigit is the decimal digits.
	ClassPerlDigit ClassPerlKind = iota
	// ClassPerlSpace is the white space.
	ClassPerlSpace
	// ClassPerlWord is the word characters.
	ClassPerlWord
)

// ClassASCII is an ASCII class, such as [:alnum:].
//
// ClassASCII is ClassAscii.
type ClassASCII struct {
	// Span is the span of the class.
	Span Span
	// Kind is the kind of the class.
	Kind ClassASCIIKind
	// Negated is true for the negated form, such as [:^alpha:].
	Negated bool
}

// ClassASCIIKind is the kind of an ASCII class.
//
// ClassASCIIKind is ClassAsciiKind.
type ClassASCIIKind uint8

// The kinds of ASCII class.
const (
	// ClassASCIIAlnum is [0-9A-Za-z].
	ClassASCIIAlnum ClassASCIIKind = iota
	// ClassASCIIAlpha is [A-Za-z].
	ClassASCIIAlpha
	// ClassASCIIASCII is [\x00-\x7F].
	ClassASCIIASCII
	// ClassASCIIBlank is [ \t].
	ClassASCIIBlank
	// ClassASCIICntrl is [\x00-\x1F\x7F].
	ClassASCIICntrl
	// ClassASCIIDigit is [0-9].
	ClassASCIIDigit
	// ClassASCIIGraph is [!-~].
	ClassASCIIGraph
	// ClassASCIILower is [a-z].
	ClassASCIILower
	// ClassASCIIPrint is [ -~].
	ClassASCIIPrint
	// ClassASCIIPunct is [!-/:-@\[-`{-~].
	ClassASCIIPunct
	// ClassASCIISpace is [\t\n\v\f\r ].
	ClassASCIISpace
	// ClassASCIIUpper is [A-Z].
	ClassASCIIUpper
	// ClassASCIIWord is [0-9A-Za-z_].
	ClassASCIIWord
	// ClassASCIIXdigit is [0-9A-Fa-f].
	ClassASCIIXdigit
)

// ClassASCIIKindFromName returns the kind of ASCII class with the name, and
// true. The name is the lower case name of the kind, such as cntrl. If no
// kind has the name, ClassASCIIKindFromName returns false.
//
// ClassASCIIKindFromName is ClassAsciiKind::from_name.
func ClassASCIIKindFromName(name string) (ClassASCIIKind, bool) {
	switch name {
	case "alnum":
		return ClassASCIIAlnum, true
	case "alpha":
		return ClassASCIIAlpha, true
	case "ascii":
		return ClassASCIIASCII, true
	case "blank":
		return ClassASCIIBlank, true
	case "cntrl":
		return ClassASCIICntrl, true
	case "digit":
		return ClassASCIIDigit, true
	case "graph":
		return ClassASCIIGraph, true
	case "lower":
		return ClassASCIILower, true
	case "print":
		return ClassASCIIPrint, true
	case "punct":
		return ClassASCIIPunct, true
	case "space":
		return ClassASCIISpace, true
	case "upper":
		return ClassASCIIUpper, true
	case "word":
		return ClassASCIIWord, true
	case "xdigit":
		return ClassASCIIXdigit, true
	}
	return 0, false
}

// ClassUnicode is a Unicode class, such as \pL or \p{Greek}.
//
// ClassUnicode is ClassUnicode. The data of the variants of upstream
// ClassUnicodeKind is in the fields Letter, Op, Name and Value.
type ClassUnicode struct {
	// Span is the span of the class.
	Span Span
	// Negated is true when the pattern spells the class as \P. It does not
	// tell if the class is negated: \P{scx!=Katakana} is the same class as
	// \p{scx=Katakana}. IsNegated tells that.
	Negated bool
	// Kind is the form of the class.
	Kind ClassUnicodeKind
	// Letter is the letter, for ClassUnicodeOneLetter.
	Letter rune
	// Op is the operator, for ClassUnicodeNamedValue.
	Op ClassUnicodeOpKind
	// Name is the name, for ClassUnicodeNamed, or the name of the property,
	// for ClassUnicodeNamedValue. It can be empty.
	Name string
	// Value is the value of the property, for ClassUnicodeNamedValue. It can
	// be empty.
	Value string
}

// IsNegated reports whether the class is negated. It takes the operator into
// account, so it is false for \P{scx!=Katakana}.
//
// IsNegated is ClassUnicode::is_negated.
func (c *ClassUnicode) IsNegated() bool {
	if c.Kind == ClassUnicodeNamedValue && c.Op == ClassUnicodeOpNotEqual {
		return !c.Negated
	}
	return c.Negated
}

// ClassUnicodeKind is the form of a Unicode class.
//
// ClassUnicodeKind is ClassUnicodeKind.
type ClassUnicodeKind uint8

// The forms of Unicode class.
const (
	// ClassUnicodeOneLetter is a class with a name of one letter, such as
	// \pN.
	ClassUnicodeOneLetter ClassUnicodeKind = iota
	// ClassUnicodeNamed is a binary property, a general category or a
	// script, such as \p{Greek}.
	ClassUnicodeNamed
	// ClassUnicodeNamedValue is a property with a value, such as
	// \p{scx=Katakana}.
	ClassUnicodeNamedValue
)

// ClassUnicodeOpKind is the operator of a Unicode class with a property and
// a value.
//
// ClassUnicodeOpKind is ClassUnicodeOpKind.
type ClassUnicodeOpKind uint8

// The operators of a Unicode class.
const (
	// ClassUnicodeOpEqual is =, as in \p{scx=Katakana}.
	ClassUnicodeOpEqual ClassUnicodeOpKind = iota
	// ClassUnicodeOpColon is :, as in \p{scx:Katakana}.
	ClassUnicodeOpColon
	// ClassUnicodeOpNotEqual is !=, as in \p{scx!=Katakana}.
	ClassUnicodeOpNotEqual
)

// IsEqual reports whether the operator is = or :.
//
// IsEqual is ClassUnicodeOpKind::is_equal.
func (k ClassUnicodeOpKind) IsEqual() bool {
	return k == ClassUnicodeOpEqual || k == ClassUnicodeOpColon
}

// ClassBracketed is a class in brackets, such as [a-z0-9]. It can hold
// ranges and nested classes.
//
// ClassBracketed is ClassBracketed.
type ClassBracketed struct {
	// Span is the span of the class.
	Span Span
	// Negated is true for a negated class, such as [^a].
	Negated bool
	// Kind is the set of the class: a union of items, such as [abc], or a set
	// operation, such as [\pL--c].
	Kind ClassSet
}

// ClassSet is the set of a class in brackets. It is a ClassSetItem, which is
// a union of items or one item, or a *ClassSetBinaryOp, which is a tree of
// set operations.
//
// ClassSet is ClassSet. A ClassSetItem value is ClassSet::Item.
type ClassSet interface {
	// GetSpan returns the span of the set.
	GetSpan() Span
	isClassSet()
}

// ClassSetItem is one part of the set of a class. It is one of *Empty,
// *Literal, *ClassSetRange, *ClassASCII, *ClassUnicode, *ClassPerl,
// *ClassBracketed and *ClassSetUnion.
//
// ClassSetItem is ClassSetItem.
type ClassSetItem interface {
	ClassSet
	isClassSetItem()
}

// GetSpan returns the span.
//
// GetSpan is ClassSetItem::span.
func (r *ClassSetRange) GetSpan() Span { return r.Span }

// GetSpan returns the span.
//
// GetSpan is ClassSetItem::span.
func (c *ClassASCII) GetSpan() Span { return c.Span }

// GetSpan returns the span.
//
// GetSpan is ClassSetItem::span.
func (u *ClassSetUnion) GetSpan() Span { return u.Span }

// GetSpan returns the span.
//
// GetSpan is ClassSet::span.
func (o *ClassSetBinaryOp) GetSpan() Span { return o.Span }

// isClassSet marks *Empty as a ClassSet.
func (*Empty) isClassSet() {}

// isClassSet marks *Literal as a ClassSet.
func (*Literal) isClassSet() {}

// isClassSet marks *ClassSetRange as a ClassSet.
func (*ClassSetRange) isClassSet() {}

// isClassSet marks *ClassASCII as a ClassSet.
func (*ClassASCII) isClassSet() {}

// isClassSet marks *ClassUnicode as a ClassSet.
func (*ClassUnicode) isClassSet() {}

// isClassSet marks *ClassPerl as a ClassSet.
func (*ClassPerl) isClassSet() {}

// isClassSet marks *ClassBracketed as a ClassSet.
func (*ClassBracketed) isClassSet() {}

// isClassSet marks *ClassSetUnion as a ClassSet.
func (*ClassSetUnion) isClassSet() {}

// isClassSet marks *ClassSetBinaryOp as a ClassSet.
func (*ClassSetBinaryOp) isClassSet() {}

// isClassSetItem marks *Empty as a ClassSetItem.
func (*Empty) isClassSetItem() {}

// isClassSetItem marks *Literal as a ClassSetItem.
func (*Literal) isClassSetItem() {}

// isClassSetItem marks *ClassSetRange as a ClassSetItem.
func (*ClassSetRange) isClassSetItem() {}

// isClassSetItem marks *ClassASCII as a ClassSetItem.
func (*ClassASCII) isClassSetItem() {}

// isClassSetItem marks *ClassUnicode as a ClassSetItem.
func (*ClassUnicode) isClassSetItem() {}

// isClassSetItem marks *ClassPerl as a ClassSetItem.
func (*ClassPerl) isClassSetItem() {}

// isClassSetItem marks *ClassBracketed as a ClassSetItem.
func (*ClassBracketed) isClassSetItem() {}

// isClassSetItem marks *ClassSetUnion as a ClassSetItem.
func (*ClassSetUnion) isClassSetItem() {}

// ClassSetRange is a range of two literals in a set.
//
// ClassSetRange is ClassSetRange.
type ClassSetRange struct {
	// Span is the span of the range.
	Span Span
	// Start is the start of the range.
	Start Literal
	// End is the end of the range.
	End Literal
}

// IsValid reports whether the range is valid. A range is not valid only when
// its start is after its end.
//
// IsValid is ClassSetRange::is_valid.
func (r *ClassSetRange) IsValid() bool {
	return r.Start.C <= r.End.C
}

// ClassSetUnion is a union of items in a set.
//
// ClassSetUnion is ClassSetUnion.
type ClassSetUnion struct {
	// Span is the span of the items, such as the a-z0-9 of [^a-z0-9].
	Span Span
	// Items are the items of the union.
	Items []ClassSetItem
}

// Push adds an item to the union. The end of the span of the union moves to
// the end of the span of the item. If the union is empty, the start of its
// span moves to the start of the span of the item too. So if every item is
// added with Push, the span of the union is right.
//
// Push is ClassSetUnion::push.
func (u *ClassSetUnion) Push(item ClassSetItem) {
	if len(u.Items) == 0 {
		u.Span.Start = item.GetSpan().Start
	}
	u.Span.End = item.GetSpan().End
	u.Items = append(u.Items, item)
}

// IntoItem returns the union as an item. With no items, it returns an
// *Empty. With one item, it returns that item. Otherwise it returns u.
//
// IntoItem is ClassSetUnion::into_item.
func (u *ClassSetUnion) IntoItem() ClassSetItem {
	switch len(u.Items) {
	case 0:
		return &Empty{Span: u.Span}
	case 1:
		return u.Items[0]
	}
	return u
}

// ClassSetBinaryOp is a set operation of a class.
//
// ClassSetBinaryOp is ClassSetBinaryOp.
type ClassSetBinaryOp struct {
	// Span is the span of the operation, such as the a-z--h-p of [a-z--h-p].
	Span Span
	// Kind is the operation.
	Kind ClassSetBinaryOpKind
	// LHS is the set on the left side.
	LHS ClassSet
	// RHS is the set on the right side.
	RHS ClassSet
}

// ClassSetBinaryOpKind is the kind of a set operation. No kind is the union,
// because the union has no operator: a sequence of items in a class is their
// union.
//
// ClassSetBinaryOpKind is ClassSetBinaryOpKind.
type ClassSetBinaryOpKind uint8

// The set operations.
const (
	// ClassSetBinaryOpIntersection is the intersection, such as \pN&&[a-z].
	ClassSetBinaryOpIntersection ClassSetBinaryOpKind = iota
	// ClassSetBinaryOpDifference is the difference, such as \pN--[0-9].
	ClassSetBinaryOpDifference
	// ClassSetBinaryOpSymmetricDifference is the set of the characters that
	// are in one of the sets and not in both, such as [\pL~~[:ascii:]].
	ClassSetBinaryOpSymmetricDifference
)

// Assertion is one assertion of zero width.
//
// Assertion is Assertion.
type Assertion struct {
	// Span is the span of the assertion.
	Span Span
	// Kind is the kind of the assertion, such as \b or ^.
	Kind AssertionKind
}

// AssertionKind is the kind of an assertion.
//
// AssertionKind is AssertionKind.
type AssertionKind uint8

// The kinds of assertion.
const (
	// AssertionStartLine is ^.
	AssertionStartLine AssertionKind = iota
	// AssertionEndLine is $.
	AssertionEndLine
	// AssertionStartText is \A.
	AssertionStartText
	// AssertionEndText is \z.
	AssertionEndText
	// AssertionWordBoundary is \b.
	AssertionWordBoundary
	// AssertionNotWordBoundary is \B.
	AssertionNotWordBoundary
	// AssertionWordBoundaryStart is \b{start}.
	AssertionWordBoundaryStart
	// AssertionWordBoundaryEnd is \b{end}.
	AssertionWordBoundaryEnd
	// AssertionWordBoundaryStartAngle is \<, the same as \b{start}.
	AssertionWordBoundaryStartAngle
	// AssertionWordBoundaryEndAngle is \>, the same as \b{end}.
	AssertionWordBoundaryEndAngle
	// AssertionWordBoundaryStartHalf is \b{start-half}.
	AssertionWordBoundaryStartHalf
	// AssertionWordBoundaryEndHalf is \b{end-half}.
	AssertionWordBoundaryEndHalf
)

// Repetition is a repetition of a regular expression.
//
// Repetition is Repetition.
type Repetition struct {
	// Span is the span of the repetition.
	Span Span
	// Op is the operator.
	Op RepetitionOp
	// Greedy is true for a greedy repetition.
	Greedy bool
	// Ast is the expression that repeats.
	Ast Ast
}

// RepetitionOp is the operator of a repetition.
//
// RepetitionOp is RepetitionOp. The data of the variant
// RepetitionKind::Range of upstream is in the field Range.
type RepetitionOp struct {
	// Span is the span of the operator, such as +, *? or {m,n}.
	Span Span
	// Kind is the kind of the operator.
	Kind RepetitionKind
	// Range is the range, for RepetitionKindRange.
	Range RepetitionRange
}

// RepetitionKind is the kind of a repetition operator.
//
// RepetitionKind is RepetitionKind.
type RepetitionKind uint8

// The kinds of repetition operator.
const (
	// RepetitionZeroOrOne is ?.
	RepetitionZeroOrOne RepetitionKind = iota
	// RepetitionZeroOrMore is *.
	RepetitionZeroOrMore
	// RepetitionOneOrMore is +.
	RepetitionOneOrMore
	// RepetitionKindRange is {m,n}. The field Range of RepetitionOp holds
	// the range. The name has Kind in it, because the type RepetitionRange
	// has the name without it.
	RepetitionKindRange
)

// RepetitionRange is the range of a counted repetition.
//
// RepetitionRange is RepetitionRange, an enum with data upstream. The fields
// that a kind uses are:
//
//   - RepetitionRangeExactly: M, for {m}.
//   - RepetitionRangeAtLeast: M, for {m,}.
//   - RepetitionRangeBounded: M and N, for {m,n}.
type RepetitionRange struct {
	Kind RepetitionRangeKind
	M    uint32
	N    uint32
}

// RepetitionRangeKind is the kind of the range of a counted repetition.
type RepetitionRangeKind uint8

// The kinds of range, in the order of the enum RepetitionRange.
const (
	RepetitionRangeExactly RepetitionRangeKind = iota
	RepetitionRangeAtLeast
	RepetitionRangeBounded
)

// IsValid reports whether the range is valid. A range is not valid only when
// it is bounded and its start is after its end.
//
// IsValid is RepetitionRange::is_valid.
func (r RepetitionRange) IsValid() bool {
	return r.Kind != RepetitionRangeBounded || r.M <= r.N
}

// Group is a regular expression in a group. It is a capturing group or a
// group that does not capture, such as (a), (?P<name>a), (?:a) and (?is:a).
// A group of flags alone, such as (?is), is a *SetFlags, not a Group.
//
// Group is Group. The data of the variants of upstream GroupKind is in the
// fields Index, StartsWithP, Name and Flags.
type Group struct {
	// Span is the span of the group.
	Span Span
	// Kind is the kind of the group.
	Kind GroupKind
	// Index is the capture index, for GroupCaptureIndex.
	Index uint32
	// StartsWithP is true when the pattern spells the name with ?P<, and
	// false for ?<, for GroupCaptureName.
	StartsWithP bool
	// Name is the capture name, for GroupCaptureName.
	Name CaptureName
	// Flags are the flags, for GroupNonCapturing.
	Flags Flags
	// Ast is the regular expression in the group.
	Ast Ast
}

// GetFlags returns the flags of a group that does not capture. The flags can
// be empty. For a capturing group, GetFlags returns nil.
//
// GetFlags is Group::flags.
func (g *Group) GetFlags() *Flags {
	if g.Kind == GroupNonCapturing {
		return &g.Flags
	}
	return nil
}

// IsCapturing reports whether the group captures.
//
// IsCapturing is Group::is_capturing.
func (g *Group) IsCapturing() bool {
	return g.Kind != GroupNonCapturing
}

// CaptureIndex returns the capture index of a capturing group, and true. For
// a group that does not capture, CaptureIndex returns false.
//
// CaptureIndex is Group::capture_index.
func (g *Group) CaptureIndex() (uint32, bool) {
	switch g.Kind {
	case GroupCaptureIndex:
		return g.Index, true
	case GroupCaptureName:
		return g.Name.Index, true
	}
	return 0, false
}

// GroupKind is the kind of a group.
//
// GroupKind is GroupKind.
type GroupKind uint8

// The kinds of group.
const (
	// GroupCaptureIndex is (a). The field Index of Group holds the index.
	GroupCaptureIndex GroupKind = iota
	// GroupCaptureName is (?<name>a) or (?P<name>a). The fields StartsWithP
	// and Name of Group hold the data.
	GroupCaptureName
	// GroupNonCapturing is (?:a) or (?i:a). The field Flags of Group holds
	// the flags.
	GroupNonCapturing
)

// CaptureName is a capture name, such as the foo of (?P<foo>expr).
//
// CaptureName is CaptureName.
type CaptureName struct {
	// Span is the span of the name.
	Span Span
	// Name is the name.
	Name string
	// Index is the capture index.
	Index uint32
}

// SetFlags is a group of flags that applies to no expression, such as (?is).
//
// SetFlags is SetFlags, and Ast::Flags.
type SetFlags struct {
	// Span is the span of the flags, with the parentheses.
	Span Span
	// Flags are the flags.
	Flags Flags
}

// Flags is a sequence of flags, such as is-u.
//
// Flags is Flags.
type Flags struct {
	// Span is the span of the flags.
	Span Span
	// Items are the flags and the negations, in order.
	Items []FlagsItem
}

// AddItem adds an item to the flags, and returns false. If an item of the
// same kind is in the flags already, AddItem does not add the item. It
// returns the index of that item and true.
//
// AddItem is Flags::add_item.
func (f *Flags) AddItem(item FlagsItem) (int, bool) {
	for i, x := range f.Items {
		if x.Kind == item.Kind && x.Flag == item.Flag {
			return i, true
		}
	}
	f.Items = append(f.Items, item)
	return 0, false
}

// FlagState returns the state of a flag, and true, if the flag is in the
// set. The state is false when a negation comes before the flag, and true
// otherwise. If the flag is not in the set, FlagState returns false as its
// second value.
//
// FlagState is Flags::flag_state.
func (f *Flags) FlagState(flag Flag) (bool, bool) {
	negated := false
	for _, x := range f.Items {
		switch {
		case x.Kind == FlagsItemNegation:
			negated = true
		case x.Flag == flag:
			return !negated, true
		}
	}
	return false, false
}

// FlagsItem is one item of a group of flags.
//
// FlagsItem is FlagsItem. The data of the variant FlagsItemKind::Flag of
// upstream is in the field Flag.
type FlagsItem struct {
	// Span is the span of the item.
	Span Span
	// Kind is the kind of the item.
	Kind FlagsItemKind
	// Flag is the flag, for FlagsItemFlag. It is zero for FlagsItemNegation.
	Flag Flag
}

// FlagsItemKind is the kind of an item of a group of flags.
//
// FlagsItemKind is FlagsItemKind.
type FlagsItemKind uint8

// The kinds of item of a group of flags.
const (
	// FlagsItemNegation is a negation, which applies to every flag after it
	// in the group.
	FlagsItemNegation FlagsItemKind = iota
	// FlagsItemFlag is one flag. The field Flag of FlagsItem holds it.
	FlagsItemFlag
)

// IsNegation reports whether the kind is a negation.
//
// IsNegation is FlagsItemKind::is_negation.
func (k FlagsItemKind) IsNegation() bool {
	return k == FlagsItemNegation
}

// Flag is one flag.
//
// Flag is Flag.
type Flag uint8

// The flags.
const (
	// FlagCaseInsensitive is i.
	FlagCaseInsensitive Flag = iota
	// FlagMultiLine is m.
	FlagMultiLine
	// FlagDotMatchesNewLine is s.
	FlagDotMatchesNewLine
	// FlagSwapGreed is U.
	FlagSwapGreed
	// FlagUnicode is u.
	FlagUnicode
	// FlagCRLF is R.
	FlagCRLF
	// FlagIgnoreWhitespace is x.
	FlagIgnoreWhitespace
)
