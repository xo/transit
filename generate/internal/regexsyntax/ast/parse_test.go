package ast

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// This file ports the tests of src/ast/parse.rs, with their helpers. The
// macro assert_eq of upstream becomes expect and expectErr, which compare a
// dump of the values. A dump reads a nil slice and an empty slice the same
// way, as the equality of a Rust Vec does.

// testError is an error without its pattern, to compare with an *Error.
//
// testError is TestError. The data of the kinds is in original and limit, as
// in Error.
type testError struct {
	span     Span
	kind     ErrorKind
	original Span
	limit    uint32
}

// result is the two results of a call, so that one argument can hold them.
type result[T any] struct {
	v   T
	err error
}

// res returns the results of a call as a result.
func res[T any](v T, err error) result[T] {
	return result[T]{v: v, err: err}
}

// expect fails the test if r holds an error, or if its value is not want.
func expect[T any](t *testing.T, r result[T], want T) {
	t.Helper()
	if r.err != nil {
		t.Errorf("unexpected error: %v", r.err)
		return
	}
	if got, w := dump(r.v), dump(want); got != w {
		t.Errorf("got:\n%s\nwant:\n%s", got, w)
	}
}

// expectErr fails the test if r holds no error, or if its error is not want.
func expectErr[T any](t *testing.T, r result[T], want testError) {
	t.Helper()
	var e *Error
	if !errors.As(r.err, &e) {
		t.Errorf("got %s and error %v, want an *Error", dump(r.v), r.err)
		return
	}
	got := testError{span: e.Span, kind: e.Kind, original: e.Original, limit: e.Limit}
	if got != want {
		t.Errorf("got error %+v, want %+v", got, want)
	}
}

// dump returns the text of a value, with each pointer and each interface
// followed, for the comparisons and the messages of the tests.
func dump(v any) string {
	var b strings.Builder
	dumpValue(&b, reflect.ValueOf(v))
	return b.String()
}

// dumpValue writes the text of v to b.
func dumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Invalid:
		b.WriteString("nil")
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		if v.Kind() == reflect.Pointer {
			b.WriteString("&")
		}
		dumpValue(b, v.Elem())
	case reflect.Struct:
		if sp, ok := reflect.TypeAssert[Span](v); ok {
			b.WriteString(sp.String())
			return
		}
		b.WriteString(v.Type().Name())
		b.WriteString("{")
		for i := range v.NumField() {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(v.Type().Field(i).Name)
			b.WriteString(": ")
			dumpValue(b, v.Field(i))
		}
		b.WriteString("}")
	case reflect.Slice:
		b.WriteString("[")
		for i := range v.Len() {
			if i > 0 {
				b.WriteString(", ")
			}
			dumpValue(b, v.Index(i))
		}
		b.WriteString("]")
	case reflect.String:
		b.WriteString(strconv.Quote(v.String()))
	case reflect.Int32:
		b.WriteString(strconv.QuoteRune(rune(v.Int())))
	default:
		fmt.Fprint(b, v)
	}
}

// s returns str.
//
// s is s.
func s(str string) string {
	return str
}

// parser returns a parser with the default configuration.
//
// parser is parser.
func parser(pattern string) *parserI {
	return newParserI(NewParser(), pattern)
}

// parserOctal returns a parser that accepts octal.
//
// parserOctal is parser_octal.
func parserOctal(pattern string) *parserI {
	parser := NewParserBuilder().Octal(true).Build()
	return newParserI(parser, pattern)
}

// parserEmptyMinRange returns a parser that accepts {,n}.
//
// parserEmptyMinRange is parser_empty_min_range.
func parserEmptyMinRange(pattern string) *parserI {
	parser := NewParserBuilder().EmptyMinRange(true).Build()
	return newParserI(parser, pattern)
}

// parserNestLimit returns a parser with a nest limit.
//
// parserNestLimit is parser_nest_limit.
func parserNestLimit(pattern string, nestLimit uint32) *parserI {
	p := NewParserBuilder().NestLimit(nestLimit).Build()
	return newParserI(p, pattern)
}

// parserIgnoreWhitespace returns a parser in verbose mode.
//
// parserIgnoreWhitespace is parser_ignore_whitespace.
func parserIgnoreWhitespace(pattern string) *parserI {
	p := NewParserBuilder().IgnoreWhitespace(true).Build()
	return newParserI(p, pattern)
}

// nspan returns a span.
//
// nspan is nspan.
func nspan(start, end Position) Span {
	return NewSpan(start, end)
}

// npos returns a position.
//
// npos is npos.
func npos(offset, line, column int) Position {
	return NewPosition(offset, line, column)
}

// span returns the span of the offsets from start to end. It assumes one line,
// and sets each column from its offset. So it works for ASCII only, which is
// enough for most tests.
//
// span is span.
func span(start, end int) Span {
	return NewSpan(NewPosition(start, 1, start+1), NewPosition(end, 1, end+1))
}

// spanRange returns the span of the bytes from start to end of subject.
//
// spanRange is span_range.
func spanRange(subject string, start, end int) Span {
	pos := func(offset int) Position {
		prefix := subject[:offset]
		column := utf8.RuneCountInString(prefix)
		if _, after, ok := strings.CutLast(prefix, "\n"); ok {
			column = utf8.RuneCountInString(after)
		}
		return Position{
			Offset: offset,
			Line:   1 + strings.Count(prefix, "\n"),
			Column: 1 + column,
		}
	}
	return NewSpan(pos(start), pos(end))
}

// lit returns a literal as it is, at the offset start.
//
// lit is lit.
func lit(c rune, start int) Ast {
	return litWith(c, span(start, start+utf8.RuneLen(c)))
}

// metaLit returns an escaped meta literal with a span.
//
// metaLit is meta_lit.
func metaLit(c rune, span Span) Ast {
	return &Literal{Span: span, Kind: LiteralMeta, C: c}
}

// litWith returns a literal as it is, with a span.
//
// litWith is lit_with.
func litWith(c rune, span Span) Ast {
	return &Literal{
		Span: span,
		Kind: LiteralVerbatim,
		C:    c,
	}
}

// concat returns a concatenation of the offsets from start to end.
//
// concat is concat.
func concat(start, end int, asts ...Ast) Ast {
	return concatWith(span(start, end), asts...)
}

// concatWith returns a concatenation with a span.
//
// concatWith is concat_with.
func concatWith(span Span, asts ...Ast) Ast {
	return &Concat{Span: span, Asts: asts}
}

// alt returns an alternation of the offsets from start to end.
//
// alt is alt.
func alt(start, end int, asts ...Ast) Ast {
	return &Alternation{Span: span(start, end), Asts: asts}
}

// group returns a capturing group of the offsets from start to end.
//
// group is group.
func group(start, end int, index uint32, ast Ast) Ast {
	return &Group{
		Span:  span(start, end),
		Kind:  GroupCaptureIndex,
		Index: index,
		Ast:   ast,
	}
}

// flagSet returns a set of one flag. The pattern is the whole pattern, and the
// flags are at the offsets from start to end. If negated is true, a negation
// comes before the flag.
//
// flagSet is flag_set.
func flagSet(pat string, start, end int, flag Flag, negated bool) Ast {
	items := []FlagsItem{{
		Span: spanRange(pat, end-2, end-1),
		Kind: FlagsItemFlag,
		Flag: flag,
	}}
	if negated {
		items = append([]FlagsItem{{
			Span: spanRange(pat, start+2, end-2),
			Kind: FlagsItemNegation,
		}}, items...)
	}
	return &SetFlags{
		Span: spanRange(pat, start, end),
		Flags: Flags{
			Span:  spanRange(pat, start+2, end-1),
			Items: items,
		},
	}
}

// TestParseNestLimit is parse_nest_limit in parse.rs.
func TestParseNestLimit(t *testing.T) {
	t.Parallel()
	// A nest limit of 0 still allows some kinds of regular expression.
	expect(t, res(parserNestLimit("", 0).parse()), Ast(&Empty{Span: span(0, 0)}))
	expect(t, res(parserNestLimit("a", 0).parse()), lit('a', 0))

	// A repetition needs one level of nesting.
	expectErr(t, res(parserNestLimit("a+", 0).parse()), testError{
		span:  span(0, 2),
		kind:  NestLimitExceeded,
		limit: 0,
	})
	expect(t, res(parserNestLimit("a+", 1).parse()), Ast(&Repetition{
		Span: span(0, 2),
		Op: RepetitionOp{
			Span: span(1, 2),
			Kind: RepetitionOneOrMore,
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expectErr(t, res(parserNestLimit("(a)+", 1).parse()), testError{
		span:  span(0, 3),
		kind:  NestLimitExceeded,
		limit: 1,
	})
	expectErr(t, res(parserNestLimit("a+*", 1).parse()), testError{
		span:  span(0, 2),
		kind:  NestLimitExceeded,
		limit: 1,
	})
	expect(t, res(parserNestLimit("a+*", 2).parse()), Ast(&Repetition{
		Span: span(0, 3),
		Op: RepetitionOp{
			Span: span(2, 3),
			Kind: RepetitionZeroOrMore,
		},
		Greedy: true,
		Ast: &Repetition{
			Span: span(0, 2),
			Op: RepetitionOp{
				Span: span(1, 2),
				Kind: RepetitionOneOrMore,
			},
			Greedy: true,
			Ast:    lit('a', 0),
		},
	}))

	// A concatenation needs one level of nesting.
	expectErr(t, res(parserNestLimit("ab", 0).parse()), testError{
		span:  span(0, 2),
		kind:  NestLimitExceeded,
		limit: 0,
	})
	expect(t, res(parserNestLimit("ab", 1).parse()), concat(0, 2, lit('a', 0), lit('b', 1)))
	expect(t, res(parserNestLimit("abc", 1).parse()), concat(0, 3, lit('a', 0), lit('b', 1), lit('c', 2)))

	// An alternation needs one level of nesting.
	expectErr(t, res(parserNestLimit("a|b", 0).parse()), testError{
		span:  span(0, 3),
		kind:  NestLimitExceeded,
		limit: 0,
	})
	expect(t, res(parserNestLimit("a|b", 1).parse()), alt(0, 3, lit('a', 0), lit('b', 2)))
	expect(t, res(parserNestLimit("a|b|c", 1).parse()), alt(0, 5, lit('a', 0), lit('b', 2), lit('c', 4)))

	// A class has a recursive syntax of its own.
	expectErr(t, res(parserNestLimit("[a]", 0).parse()), testError{
		span:  span(0, 3),
		kind:  NestLimitExceeded,
		limit: 0,
	})
	expect(t, res(parserNestLimit("[a]", 1).parse()), Ast(&ClassBracketed{
		Span:    span(0, 3),
		Negated: false,
		Kind: &Literal{
			Span: span(1, 2),
			Kind: LiteralVerbatim,
			C:    'a',
		},
	}))
	expectErr(t, res(parserNestLimit("[ab]", 1).parse()), testError{
		span:  span(1, 3),
		kind:  NestLimitExceeded,
		limit: 1,
	})
	expectErr(t, res(parserNestLimit("[ab[cd]]", 2).parse()), testError{
		span:  span(3, 7),
		kind:  NestLimitExceeded,
		limit: 2,
	})
	expectErr(t, res(parserNestLimit("[ab[cd]]", 3).parse()), testError{
		span:  span(4, 6),
		kind:  NestLimitExceeded,
		limit: 3,
	})
	expectErr(t, res(parserNestLimit("[a--b]", 1).parse()), testError{
		span:  span(1, 5),
		kind:  NestLimitExceeded,
		limit: 1,
	})
	expectErr(t, res(parserNestLimit("[a--bc]", 2).parse()), testError{
		span:  span(4, 6),
		kind:  NestLimitExceeded,
		limit: 2,
	})
}

// TestParseComments is parse_comments in parse.rs.
func TestParseComments(t *testing.T) {
	t.Parallel()
	pat := "(?x)\n# This is comment 1.\nfoo # This is comment 2.\n  # This is comment 3.\nbar\n# This is comment 4."
	astc, err := parser(pat).parseWithComments()
	if err != nil {
		t.Fatal(err)
	}
	expect(t, res(astc.Ast, nil), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		litWith('f', spanRange(pat, 26, 27)),
		litWith('o', spanRange(pat, 27, 28)),
		litWith('o', spanRange(pat, 28, 29)),
		litWith('b', spanRange(pat, 74, 75)),
		litWith('a', spanRange(pat, 75, 76)),
		litWith('r', spanRange(pat, 76, 77)),
	))
	expect(t, res(astc.Comments, nil), []Comment{
		{
			Span:    spanRange(pat, 5, 26),
			Comment: s(" This is comment 1."),
		},
		{
			Span:    spanRange(pat, 30, 51),
			Comment: s(" This is comment 2."),
		},
		{
			Span:    spanRange(pat, 53, 74),
			Comment: s(" This is comment 3."),
		},
		{
			Span:    spanRange(pat, 78, 98),
			Comment: s(" This is comment 4."),
		},
	})
}

// TestParseHolistic is parse_holistic in parse.rs.
func TestParseHolistic(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("]").parse()), lit(']', 0))
	expect(t, res(parser(`\\\.\+\*\?\(\)\|\[\]\{\}\^\$\#\&\-\~`).parse()), concat(
		0, 36,
		metaLit('\\', span(0, 2)),
		metaLit('.', span(2, 4)),
		metaLit('+', span(4, 6)),
		metaLit('*', span(6, 8)),
		metaLit('?', span(8, 10)),
		metaLit('(', span(10, 12)),
		metaLit(')', span(12, 14)),
		metaLit('|', span(14, 16)),
		metaLit('[', span(16, 18)),
		metaLit(']', span(18, 20)),
		metaLit('{', span(20, 22)),
		metaLit('}', span(22, 24)),
		metaLit('^', span(24, 26)),
		metaLit('$', span(26, 28)),
		metaLit('#', span(28, 30)),
		metaLit('&', span(30, 32)),
		metaLit('-', span(32, 34)),
		metaLit('~', span(34, 36)),
	))
}

// TestParseIgnoreWhitespace is parse_ignore_whitespace in parse.rs.
func TestParseIgnoreWhitespace(t *testing.T) {
	t.Parallel()
	// Verbose mode ignores white space.
	pat := "(?x)a b"
	expect(t, res(parser(pat).parse()), concatWith(
		nspan(npos(0, 1, 1), npos(7, 1, 8)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		litWith('a', nspan(npos(4, 1, 5), npos(5, 1, 6))),
		litWith('b', nspan(npos(6, 1, 7), npos(7, 1, 8))),
	))

	// Verbose mode can go on and off.
	pat = "(?x)a b(?-x)a b"
	expect(t, res(parser(pat).parse()), concatWith(
		nspan(npos(0, 1, 1), npos(15, 1, 16)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		litWith('a', nspan(npos(4, 1, 5), npos(5, 1, 6))),
		litWith('b', nspan(npos(6, 1, 7), npos(7, 1, 8))),
		flagSet(pat, 7, 12, FlagIgnoreWhitespace, true),
		litWith('a', nspan(npos(12, 1, 13), npos(13, 1, 14))),
		litWith(' ', nspan(npos(13, 1, 14), npos(14, 1, 15))),
		litWith('b', nspan(npos(14, 1, 15), npos(15, 1, 16))),
	))

	// The flag of verbose mode nests.
	pat = "a (?x:a )a "
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, 11),
		litWith('a', spanRange(pat, 0, 1)),
		litWith(' ', spanRange(pat, 1, 2)),
		&Group{
			Span: spanRange(pat, 2, 9),
			Kind: GroupNonCapturing,
			Flags: Flags{
				Span: spanRange(pat, 4, 5),
				Items: []FlagsItem{{
					Span: spanRange(pat, 4, 5),
					Kind: FlagsItemFlag,
					Flag: FlagIgnoreWhitespace,
				}},
			},
			Ast: litWith('a', spanRange(pat, 6, 7)),
		},
		litWith('a', spanRange(pat, 9, 10)),
		litWith(' ', spanRange(pat, 10, 11)),
	))

	// White space after an opening parenthesis does not count.
	pat = "(?x)( ?P<foo> a )"
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		&Group{
			Span:        spanRange(pat, 4, len(pat)),
			Kind:        GroupCaptureName,
			StartsWithP: true,
			Name: CaptureName{
				Span:  spanRange(pat, 9, 12),
				Name:  s("foo"),
				Index: 1,
			},
			Ast: litWith('a', spanRange(pat, 14, 15)),
		},
	))
	pat = "(?x)(  a )"
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		&Group{
			Span:  spanRange(pat, 4, len(pat)),
			Kind:  GroupCaptureIndex,
			Index: 1,
			Ast:   litWith('a', spanRange(pat, 7, 8)),
		},
	))
	pat = "(?x)(  ?:  a )"
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		&Group{
			Span: spanRange(pat, 4, len(pat)),
			Kind: GroupNonCapturing,
			Flags: Flags{
				Span:  spanRange(pat, 8, 8),
				Items: []FlagsItem{},
			},
			Ast: litWith('a', spanRange(pat, 11, 12)),
		},
	))
	pat = `(?x)\x { 53 }`
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		&Literal{
			Span: span(4, 13),
			Kind: LiteralHexBrace,
			Hex:  HexLiteralX,
			C:    'S',
		},
	))

	// White space after an escape is valid.
	pat = `(?x)\ `
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		flagSet(pat, 0, 4, FlagIgnoreWhitespace, false),
		&Literal{
			Span: spanRange(pat, 4, 6),
			Kind: LiteralSuperfluous,
			C:    ' ',
		},
	))
}

// TestParseNewlines is parse_newlines in parse.rs.
func TestParseNewlines(t *testing.T) {
	t.Parallel()
	pat := ".\n."
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, 3),
		&Dot{Span: spanRange(pat, 0, 1)},
		litWith('\n', spanRange(pat, 1, 2)),
		&Dot{Span: spanRange(pat, 2, 3)},
	))

	pat = "foobar\nbaz\nquux\n"
	expect(t, res(parser(pat).parse()), concatWith(
		spanRange(pat, 0, len(pat)),
		litWith('f', nspan(npos(0, 1, 1), npos(1, 1, 2))),
		litWith('o', nspan(npos(1, 1, 2), npos(2, 1, 3))),
		litWith('o', nspan(npos(2, 1, 3), npos(3, 1, 4))),
		litWith('b', nspan(npos(3, 1, 4), npos(4, 1, 5))),
		litWith('a', nspan(npos(4, 1, 5), npos(5, 1, 6))),
		litWith('r', nspan(npos(5, 1, 6), npos(6, 1, 7))),
		litWith('\n', nspan(npos(6, 1, 7), npos(7, 2, 1))),
		litWith('b', nspan(npos(7, 2, 1), npos(8, 2, 2))),
		litWith('a', nspan(npos(8, 2, 2), npos(9, 2, 3))),
		litWith('z', nspan(npos(9, 2, 3), npos(10, 2, 4))),
		litWith('\n', nspan(npos(10, 2, 4), npos(11, 3, 1))),
		litWith('q', nspan(npos(11, 3, 1), npos(12, 3, 2))),
		litWith('u', nspan(npos(12, 3, 2), npos(13, 3, 3))),
		litWith('u', nspan(npos(13, 3, 3), npos(14, 3, 4))),
		litWith('x', nspan(npos(14, 3, 4), npos(15, 3, 5))),
		litWith('\n', nspan(npos(15, 3, 5), npos(16, 4, 1))),
	))
}

// TestParseUncountedRepetition is parse_uncounted_repetition in parse.rs.
func TestParseUncountedRepetition(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`a*`).parse()), Ast(&Repetition{
		Span: span(0, 2),
		Op: RepetitionOp{
			Span: span(1, 2),
			Kind: RepetitionZeroOrMore,
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a+`).parse()), Ast(&Repetition{
		Span: span(0, 2),
		Op: RepetitionOp{
			Span: span(1, 2),
			Kind: RepetitionOneOrMore,
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))

	expect(t, res(parser(`a?`).parse()), Ast(&Repetition{
		Span: span(0, 2),
		Op: RepetitionOp{
			Span: span(1, 2),
			Kind: RepetitionZeroOrOne,
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a??`).parse()), Ast(&Repetition{
		Span: span(0, 3),
		Op: RepetitionOp{
			Span: span(1, 3),
			Kind: RepetitionZeroOrOne,
		},
		Greedy: false,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a?`).parse()), Ast(&Repetition{
		Span: span(0, 2),
		Op: RepetitionOp{
			Span: span(1, 2),
			Kind: RepetitionZeroOrOne,
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a?b`).parse()), concat(
		0, 3,
		&Repetition{
			Span: span(0, 2),
			Op: RepetitionOp{
				Span: span(1, 2),
				Kind: RepetitionZeroOrOne,
			},
			Greedy: true,
			Ast:    lit('a', 0),
		},
		lit('b', 2),
	))
	expect(t, res(parser(`a??b`).parse()), concat(
		0, 4,
		&Repetition{
			Span: span(0, 3),
			Op: RepetitionOp{
				Span: span(1, 3),
				Kind: RepetitionZeroOrOne,
			},
			Greedy: false,
			Ast:    lit('a', 0),
		},
		lit('b', 3),
	))
	expect(t, res(parser(`ab?`).parse()), concat(
		0, 3,
		lit('a', 0),
		&Repetition{
			Span: span(1, 3),
			Op: RepetitionOp{
				Span: span(2, 3),
				Kind: RepetitionZeroOrOne,
			},
			Greedy: true,
			Ast:    lit('b', 1),
		},
	))
	expect(t, res(parser(`(ab)?`).parse()), Ast(&Repetition{
		Span: span(0, 5),
		Op: RepetitionOp{
			Span: span(4, 5),
			Kind: RepetitionZeroOrOne,
		},
		Greedy: true,
		Ast: group(
			0, 4,
			1,
			concat(1, 3, lit('a', 1), lit('b', 2)),
		),
	}))
	expect(t, res(parser(`|a?`).parse()), alt(
		0, 3,
		&Empty{Span: span(0, 0)},
		&Repetition{
			Span: span(1, 3),
			Op: RepetitionOp{
				Span: span(2, 3),
				Kind: RepetitionZeroOrOne,
			},
			Greedy: true,
			Ast:    lit('a', 1),
		},
	))

	expectErr(t, res(parser(`*`).parse()), testError{
		span: span(0, 0),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`(?i)*`).parse()), testError{
		span: span(4, 4),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`(*)`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`(?:?)`).parse()), testError{
		span: span(3, 3),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`+`).parse()), testError{
		span: span(0, 0),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`?`).parse()), testError{
		span: span(0, 0),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`(?)`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`|*`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`|+`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`|?`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
}

// TestParseCountedRepetition is parse_counted_repetition in parse.rs.
func TestParseCountedRepetition(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`a{5}`).parse()), Ast(&Repetition{
		Span: span(0, 4),
		Op: RepetitionOp{
			Span:  span(1, 4),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeExactly, M: 5},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a{5,}`).parse()), Ast(&Repetition{
		Span: span(0, 5),
		Op: RepetitionOp{
			Span:  span(1, 5),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeAtLeast, M: 5},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a{5,9}`).parse()), Ast(&Repetition{
		Span: span(0, 6),
		Op: RepetitionOp{
			Span:  span(1, 6),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeBounded, M: 5, N: 9},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a{5}?`).parse()), Ast(&Repetition{
		Span: span(0, 5),
		Op: RepetitionOp{
			Span:  span(1, 5),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeExactly, M: 5},
		},
		Greedy: false,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`ab{5}`).parse()), concat(
		0, 5,
		lit('a', 0),
		&Repetition{
			Span: span(1, 5),
			Op: RepetitionOp{
				Span:  span(2, 5),
				Kind:  RepetitionKindRange,
				Range: RepetitionRange{Kind: RepetitionRangeExactly, M: 5},
			},
			Greedy: true,
			Ast:    lit('b', 1),
		},
	))
	expect(t, res(parser(`ab{5}c`).parse()), concat(
		0, 6,
		lit('a', 0),
		&Repetition{
			Span: span(1, 5),
			Op: RepetitionOp{
				Span:  span(2, 5),
				Kind:  RepetitionKindRange,
				Range: RepetitionRange{Kind: RepetitionRangeExactly, M: 5},
			},
			Greedy: true,
			Ast:    lit('b', 1),
		},
		lit('c', 5),
	))

	expect(t, res(parser(`a{ 5 }`).parse()), Ast(&Repetition{
		Span: span(0, 6),
		Op: RepetitionOp{
			Span:  span(1, 6),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeExactly, M: 5},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`a{ 5 , 9 }`).parse()), Ast(&Repetition{
		Span: span(0, 10),
		Op: RepetitionOp{
			Span:  span(1, 10),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeBounded, M: 5, N: 9},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parserEmptyMinRange(`a{,9}`).parse()), Ast(&Repetition{
		Span: span(0, 5),
		Op: RepetitionOp{
			Span:  span(1, 5),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeBounded, M: 0, N: 9},
		},
		Greedy: true,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parserIgnoreWhitespace(`a{5,9} ?`).parse()), Ast(&Repetition{
		Span: span(0, 8),
		Op: RepetitionOp{
			Span:  span(1, 8),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeBounded, M: 5, N: 9},
		},
		Greedy: false,
		Ast:    lit('a', 0),
	}))
	expect(t, res(parser(`\b{5,9}`).parse()), Ast(&Repetition{
		Span: span(0, 7),
		Op: RepetitionOp{
			Span:  span(2, 7),
			Kind:  RepetitionKindRange,
			Range: RepetitionRange{Kind: RepetitionRangeBounded, M: 5, N: 9},
		},
		Greedy: true,
		Ast: &Assertion{
			Span: span(0, 2),
			Kind: AssertionWordBoundary,
		},
	}))

	expectErr(t, res(parser(`(?i){0}`).parse()), testError{
		span: span(4, 4),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`(?m){1,1}`).parse()), testError{
		span: span(4, 4),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`a{]}`).parse()), testError{
		span: span(2, 2),
		kind: RepetitionCountDecimalEmpty,
	})
	expectErr(t, res(parser(`a{1,]}`).parse()), testError{
		span: span(4, 4),
		kind: RepetitionCountDecimalEmpty,
	})
	expectErr(t, res(parser(`a{`).parse()), testError{
		span: span(1, 2),
		kind: RepetitionCountUnclosed,
	})
	expectErr(t, res(parser(`a{}`).parse()), testError{
		span: span(2, 2),
		kind: RepetitionCountDecimalEmpty,
	})
	expectErr(t, res(parser(`a{a`).parse()), testError{
		span: span(2, 2),
		kind: RepetitionCountDecimalEmpty,
	})
	expectErr(t, res(parser(`a{9999999999}`).parse()), testError{
		span: span(2, 12),
		kind: DecimalInvalid,
	})
	expectErr(t, res(parser(`a{9`).parse()), testError{
		span: span(1, 3),
		kind: RepetitionCountUnclosed,
	})
	expectErr(t, res(parser(`a{9,a`).parse()), testError{
		span: span(4, 4),
		kind: RepetitionCountDecimalEmpty,
	})
	expectErr(t, res(parser(`a{9,9999999999}`).parse()), testError{
		span: span(4, 14),
		kind: DecimalInvalid,
	})
	expectErr(t, res(parser(`a{9,`).parse()), testError{
		span: span(1, 4),
		kind: RepetitionCountUnclosed,
	})
	expectErr(t, res(parser(`a{9,11`).parse()), testError{
		span: span(1, 6),
		kind: RepetitionCountUnclosed,
	})
	expectErr(t, res(parser(`a{2,1}`).parse()), testError{
		span: span(1, 6),
		kind: RepetitionCountInvalid,
	})
	expectErr(t, res(parser(`{5}`).parse()), testError{
		span: span(0, 0),
		kind: RepetitionMissing,
	})
	expectErr(t, res(parser(`|{5}`).parse()), testError{
		span: span(1, 1),
		kind: RepetitionMissing,
	})
}

// TestParseAlternate is parse_alternate in parse.rs.
func TestParseAlternate(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`a|b`).parse()), Ast(&Alternation{
		Span: span(0, 3),
		Asts: []Ast{lit('a', 0), lit('b', 2)},
	}))
	expect(t, res(parser(`(a|b)`).parse()), group(
		0, 5,
		1,
		&Alternation{
			Span: span(1, 4),
			Asts: []Ast{lit('a', 1), lit('b', 3)},
		},
	))

	expect(t, res(parser(`a|b|c`).parse()), Ast(&Alternation{
		Span: span(0, 5),
		Asts: []Ast{lit('a', 0), lit('b', 2), lit('c', 4)},
	}))
	expect(t, res(parser(`ax|by|cz`).parse()), Ast(&Alternation{
		Span: span(0, 8),
		Asts: []Ast{
			concat(0, 2, lit('a', 0), lit('x', 1)),
			concat(3, 5, lit('b', 3), lit('y', 4)),
			concat(6, 8, lit('c', 6), lit('z', 7)),
		},
	}))
	expect(t, res(parser(`(ax|by|cz)`).parse()), group(
		0, 10,
		1,
		&Alternation{
			Span: span(1, 9),
			Asts: []Ast{
				concat(1, 3, lit('a', 1), lit('x', 2)),
				concat(4, 6, lit('b', 4), lit('y', 5)),
				concat(7, 9, lit('c', 7), lit('z', 8)),
			},
		},
	))
	expect(t, res(parser(`(ax|(by|(cz)))`).parse()), group(
		0, 14,
		1,
		alt(
			1, 13,
			concat(1, 3, lit('a', 1), lit('x', 2)),
			group(
				4, 13,
				2,
				alt(
					5, 12,
					concat(
						5, 7,
						lit('b', 5), lit('y', 6),
					),
					group(
						8, 12,
						3,
						concat(
							9, 11,
							lit('c', 9), lit('z', 10),
						),
					),
				),
			),
		),
	))

	expect(t, res(parser(`|`).parse()), alt(
		0, 1,
		&Empty{Span: span(0, 0)}, &Empty{Span: span(1, 1)},
	))
	expect(t, res(parser(`||`).parse()), alt(
		0, 2,
		&Empty{Span: span(0, 0)},
		&Empty{Span: span(1, 1)},
		&Empty{Span: span(2, 2)},
	))
	expect(t, res(parser(`a|`).parse()), alt(0, 2, lit('a', 0), &Empty{Span: span(2, 2)}))
	expect(t, res(parser(`|a`).parse()), alt(0, 2, &Empty{Span: span(0, 0)}, lit('a', 1)))

	expect(t, res(parser(`(|)`).parse()), group(
		0, 3,
		1,
		alt(
			1, 2,
			&Empty{Span: span(1, 1)}, &Empty{Span: span(2, 2)},
		),
	))
	expect(t, res(parser(`(a|)`).parse()), group(
		0, 4,
		1,
		alt(1, 3, lit('a', 1), &Empty{Span: span(3, 3)}),
	))
	expect(t, res(parser(`(|a)`).parse()), group(
		0, 4,
		1,
		alt(1, 3, &Empty{Span: span(1, 1)}, lit('a', 2)),
	))

	expectErr(t, res(parser(`a|b)`).parse()), testError{
		span: span(3, 4),
		kind: GroupUnopened,
	})
	expectErr(t, res(parser(`(a|b`).parse()), testError{
		span: span(0, 1),
		kind: GroupUnclosed,
	})
}

// TestParseUnsupportedLookaround is parse_unsupported_lookaround in
// parse.rs.
func TestParseUnsupportedLookaround(t *testing.T) {
	t.Parallel()
	expectErr(t, res(parser(`(?=a)`).parse()), testError{
		span: span(0, 3),
		kind: UnsupportedLookAround,
	})
	expectErr(t, res(parser(`(?!a)`).parse()), testError{
		span: span(0, 3),
		kind: UnsupportedLookAround,
	})
	expectErr(t, res(parser(`(?<=a)`).parse()), testError{
		span: span(0, 4),
		kind: UnsupportedLookAround,
	})
	expectErr(t, res(parser(`(?<!a)`).parse()), testError{
		span: span(0, 4),
		kind: UnsupportedLookAround,
	})
}

// TestParseGroup is parse_group in parse.rs.
func TestParseGroup(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("(?i)").parse()), Ast(&SetFlags{
		Span: span(0, 4),
		Flags: Flags{
			Span: span(2, 3),
			Items: []FlagsItem{{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			}},
		},
	}))
	expect(t, res(parser("(?iU)").parse()), Ast(&SetFlags{
		Span: span(0, 5),
		Flags: Flags{
			Span: span(2, 4),
			Items: []FlagsItem{
				{
					Span: span(2, 3),
					Kind: FlagsItemFlag,
					Flag: FlagCaseInsensitive,
				},
				{
					Span: span(3, 4),
					Kind: FlagsItemFlag,
					Flag: FlagSwapGreed,
				},
			},
		},
	}))
	expect(t, res(parser("(?i-U)").parse()), Ast(&SetFlags{
		Span: span(0, 6),
		Flags: Flags{
			Span: span(2, 5),
			Items: []FlagsItem{
				{
					Span: span(2, 3),
					Kind: FlagsItemFlag,
					Flag: FlagCaseInsensitive,
				},
				{
					Span: span(3, 4),
					Kind: FlagsItemNegation,
				},
				{
					Span: span(4, 5),
					Kind: FlagsItemFlag,
					Flag: FlagSwapGreed,
				},
			},
		},
	}))

	expect(t, res(parser("()").parse()), Ast(&Group{
		Span:  span(0, 2),
		Kind:  GroupCaptureIndex,
		Index: 1,
		Ast:   &Empty{Span: span(1, 1)},
	}))
	expect(t, res(parser("(a)").parse()), Ast(&Group{
		Span:  span(0, 3),
		Kind:  GroupCaptureIndex,
		Index: 1,
		Ast:   lit('a', 1),
	}))
	expect(t, res(parser("(())").parse()), Ast(&Group{
		Span:  span(0, 4),
		Kind:  GroupCaptureIndex,
		Index: 1,
		Ast: &Group{
			Span:  span(1, 3),
			Kind:  GroupCaptureIndex,
			Index: 2,
			Ast:   &Empty{Span: span(2, 2)},
		},
	}))

	expect(t, res(parser("(?:a)").parse()), Ast(&Group{
		Span: span(0, 5),
		Kind: GroupNonCapturing,
		Flags: Flags{
			Span:  span(2, 2),
			Items: []FlagsItem{},
		},
		Ast: lit('a', 3),
	}))

	expect(t, res(parser("(?i:a)").parse()), Ast(&Group{
		Span: span(0, 6),
		Kind: GroupNonCapturing,
		Flags: Flags{
			Span: span(2, 3),
			Items: []FlagsItem{{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			}},
		},
		Ast: lit('a', 4),
	}))
	expect(t, res(parser("(?i-U:a)").parse()), Ast(&Group{
		Span: span(0, 8),
		Kind: GroupNonCapturing,
		Flags: Flags{
			Span: span(2, 5),
			Items: []FlagsItem{
				{
					Span: span(2, 3),
					Kind: FlagsItemFlag,
					Flag: FlagCaseInsensitive,
				},
				{
					Span: span(3, 4),
					Kind: FlagsItemNegation,
				},
				{
					Span: span(4, 5),
					Kind: FlagsItemFlag,
					Flag: FlagSwapGreed,
				},
			},
		},
		Ast: lit('a', 6),
	}))

	expectErr(t, res(parser("(").parse()), testError{
		span: span(0, 1),
		kind: GroupUnclosed,
	})
	expectErr(t, res(parser("(?").parse()), testError{
		span: span(0, 1),
		kind: GroupUnclosed,
	})
	expectErr(t, res(parser("(?P").parse()), testError{
		span: span(2, 3),
		kind: FlagUnrecognized,
	})
	expectErr(t, res(parser("(?P<").parse()), testError{
		span: span(4, 4),
		kind: GroupNameUnexpectedEOF,
	})
	expectErr(t, res(parser("(a").parse()), testError{
		span: span(0, 1),
		kind: GroupUnclosed,
	})
	expectErr(t, res(parser("(()").parse()), testError{
		span: span(0, 1),
		kind: GroupUnclosed,
	})
	expectErr(t, res(parser(")").parse()), testError{
		span: span(0, 1),
		kind: GroupUnopened,
	})
	expectErr(t, res(parser("a)").parse()), testError{
		span: span(1, 2),
		kind: GroupUnopened,
	})
}

// captureName returns a group with a capture name, for the tests of
// parseCaptureName. Upstream writes each group in full.
func captureName(span Span, startsWithP bool, name CaptureName, ast Ast) Ast {
	return &Group{
		Span:        span,
		Kind:        GroupCaptureName,
		StartsWithP: startsWithP,
		Name:        name,
		Ast:         ast,
	}
}

// TestParseCaptureName is parse_capture_name in parse.rs.
func TestParseCaptureName(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("(?<a>z)").parse()), captureName(
		span(0, 7),
		false,
		CaptureName{
			Span:  span(3, 4),
			Name:  s("a"),
			Index: 1,
		},
		lit('z', 5),
	))
	expect(t, res(parser("(?P<a>z)").parse()), captureName(
		span(0, 8),
		true,
		CaptureName{
			Span:  span(4, 5),
			Name:  s("a"),
			Index: 1,
		},
		lit('z', 6),
	))
	expect(t, res(parser("(?P<abc>z)").parse()), captureName(
		span(0, 10),
		true,
		CaptureName{
			Span:  span(4, 7),
			Name:  s("abc"),
			Index: 1,
		},
		lit('z', 8),
	))

	expect(t, res(parser("(?P<a_1>z)").parse()), captureName(
		span(0, 10),
		true,
		CaptureName{
			Span:  span(4, 7),
			Name:  s("a_1"),
			Index: 1,
		},
		lit('z', 8),
	))

	expect(t, res(parser("(?P<a.1>z)").parse()), captureName(
		span(0, 10),
		true,
		CaptureName{
			Span:  span(4, 7),
			Name:  s("a.1"),
			Index: 1,
		},
		lit('z', 8),
	))

	expect(t, res(parser("(?P<a[1]>z)").parse()), captureName(
		span(0, 11),
		true,
		CaptureName{
			Span:  span(4, 8),
			Name:  s("a[1]"),
			Index: 1,
		},
		lit('z', 9),
	))

	expect(t, res(parser("(?P<a¾>)").parse()), captureName(
		NewSpan(
			NewPosition(0, 1, 1),
			NewPosition(9, 1, 9),
		),
		true,
		CaptureName{
			Span: NewSpan(
				NewPosition(4, 1, 5),
				NewPosition(7, 1, 7),
			),
			Name:  s("a¾"),
			Index: 1,
		},
		&Empty{Span: NewSpan(
			NewPosition(8, 1, 8),
			NewPosition(8, 1, 8),
		)},
	))
	expect(t, res(parser("(?P<名字>)").parse()), captureName(
		NewSpan(
			NewPosition(0, 1, 1),
			NewPosition(12, 1, 9),
		),
		true,
		CaptureName{
			Span: NewSpan(
				NewPosition(4, 1, 5),
				NewPosition(10, 1, 7),
			),
			Name:  s("名字"),
			Index: 1,
		},
		&Empty{Span: NewSpan(
			NewPosition(11, 1, 8),
			NewPosition(11, 1, 8),
		)},
	))

	expectErr(t, res(parser("(?P<").parse()), testError{
		span: span(4, 4),
		kind: GroupNameUnexpectedEOF,
	})
	expectErr(t, res(parser("(?P<>z)").parse()), testError{
		span: span(4, 4),
		kind: GroupNameEmpty,
	})
	expectErr(t, res(parser("(?P<a").parse()), testError{
		span: span(5, 5),
		kind: GroupNameUnexpectedEOF,
	})
	expectErr(t, res(parser("(?P<ab").parse()), testError{
		span: span(6, 6),
		kind: GroupNameUnexpectedEOF,
	})
	expectErr(t, res(parser("(?P<0a").parse()), testError{
		span: span(4, 5),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<~").parse()), testError{
		span: span(4, 5),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<abc~").parse()), testError{
		span: span(7, 8),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<a>y)(?P<a>z)").parse()), testError{
		span:     span(12, 13),
		kind:     GroupNameDuplicate,
		original: span(4, 5),
	})
	expectErr(t, res(parser("(?P<5>)").parse()), testError{
		span: span(4, 5),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<5a>)").parse()), testError{
		span: span(4, 5),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<¾>)").parse()), testError{
		span: NewSpan(
			NewPosition(4, 1, 5),
			NewPosition(6, 1, 6),
		),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<¾a>)").parse()), testError{
		span: NewSpan(
			NewPosition(4, 1, 5),
			NewPosition(6, 1, 6),
		),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<☃>)").parse()), testError{
		span: NewSpan(
			NewPosition(4, 1, 5),
			NewPosition(7, 1, 6),
		),
		kind: GroupNameInvalid,
	})
	expectErr(t, res(parser("(?P<a☃>)").parse()), testError{
		span: NewSpan(
			NewPosition(5, 1, 6),
			NewPosition(8, 1, 7),
		),
		kind: GroupNameInvalid,
	})
}

// TestParseFlags is parse_flags in parse.rs.
func TestParseFlags(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("i:").parseFlags()), Flags{
		Span: span(0, 1),
		Items: []FlagsItem{{
			Span: span(0, 1),
			Kind: FlagsItemFlag,
			Flag: FlagCaseInsensitive,
		}},
	})
	expect(t, res(parser("i)").parseFlags()), Flags{
		Span: span(0, 1),
		Items: []FlagsItem{{
			Span: span(0, 1),
			Kind: FlagsItemFlag,
			Flag: FlagCaseInsensitive,
		}},
	})

	expect(t, res(parser("isU:").parseFlags()), Flags{
		Span: span(0, 3),
		Items: []FlagsItem{
			{
				Span: span(0, 1),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			},
			{
				Span: span(1, 2),
				Kind: FlagsItemFlag,
				Flag: FlagDotMatchesNewLine,
			},
			{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagSwapGreed,
			},
		},
	})

	expect(t, res(parser("-isU:").parseFlags()), Flags{
		Span: span(0, 4),
		Items: []FlagsItem{
			{
				Span: span(0, 1),
				Kind: FlagsItemNegation,
			},
			{
				Span: span(1, 2),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			},
			{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagDotMatchesNewLine,
			},
			{
				Span: span(3, 4),
				Kind: FlagsItemFlag,
				Flag: FlagSwapGreed,
			},
		},
	})
	expect(t, res(parser("i-sU:").parseFlags()), Flags{
		Span: span(0, 4),
		Items: []FlagsItem{
			{
				Span: span(0, 1),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			},
			{
				Span: span(1, 2),
				Kind: FlagsItemNegation,
			},
			{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagDotMatchesNewLine,
			},
			{
				Span: span(3, 4),
				Kind: FlagsItemFlag,
				Flag: FlagSwapGreed,
			},
		},
	})
	expect(t, res(parser("i-sR:").parseFlags()), Flags{
		Span: span(0, 4),
		Items: []FlagsItem{
			{
				Span: span(0, 1),
				Kind: FlagsItemFlag,
				Flag: FlagCaseInsensitive,
			},
			{
				Span: span(1, 2),
				Kind: FlagsItemNegation,
			},
			{
				Span: span(2, 3),
				Kind: FlagsItemFlag,
				Flag: FlagDotMatchesNewLine,
			},
			{
				Span: span(3, 4),
				Kind: FlagsItemFlag,
				Flag: FlagCRLF,
			},
		},
	})

	expectErr(t, res(parser("isU").parseFlags()), testError{
		span: span(3, 3),
		kind: FlagUnexpectedEOF,
	})
	expectErr(t, res(parser("isUa:").parseFlags()), testError{
		span: span(3, 4),
		kind: FlagUnrecognized,
	})
	expectErr(t, res(parser("isUi:").parseFlags()), testError{
		span:     span(3, 4),
		kind:     FlagDuplicate,
		original: span(0, 1),
	})
	expectErr(t, res(parser("i-sU-i:").parseFlags()), testError{
		span:     span(4, 5),
		kind:     FlagRepeatedNegation,
		original: span(1, 2),
	})
	expectErr(t, res(parser("-)").parseFlags()), testError{
		span: span(0, 1),
		kind: FlagDanglingNegation,
	})
	expectErr(t, res(parser("i-)").parseFlags()), testError{
		span: span(1, 2),
		kind: FlagDanglingNegation,
	})
	expectErr(t, res(parser("iU-)").parseFlags()), testError{
		span: span(2, 3),
		kind: FlagDanglingNegation,
	})
}

// TestParseFlag is parse_flag in parse.rs.
func TestParseFlag(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("i").parseFlag()), FlagCaseInsensitive)
	expect(t, res(parser("m").parseFlag()), FlagMultiLine)
	expect(t, res(parser("s").parseFlag()), FlagDotMatchesNewLine)
	expect(t, res(parser("U").parseFlag()), FlagSwapGreed)
	expect(t, res(parser("u").parseFlag()), FlagUnicode)
	expect(t, res(parser("R").parseFlag()), FlagCRLF)
	expect(t, res(parser("x").parseFlag()), FlagIgnoreWhitespace)

	expectErr(t, res(parser("a").parseFlag()), testError{
		span: span(0, 1),
		kind: FlagUnrecognized,
	})
	expectErr(t, res(parser("☃").parseFlag()), testError{
		span: spanRange("☃", 0, 3),
		kind: FlagUnrecognized,
	})
}

// TestParsePrimitiveNonEscape is parse_primitive_non_escape in parse.rs.
func TestParsePrimitiveNonEscape(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`.`).parsePrimitive()), primitive(&Dot{Span: span(0, 1)}))
	expect(t, res(parser(`^`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 1),
		Kind: AssertionStartLine,
	}))
	expect(t, res(parser(`$`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 1),
		Kind: AssertionEndLine,
	}))

	expect(t, res(parser(`a`).parsePrimitive()), primitive(&Literal{
		Span: span(0, 1),
		Kind: LiteralVerbatim,
		C:    'a',
	}))
	expect(t, res(parser(`|`).parsePrimitive()), primitive(&Literal{
		Span: span(0, 1),
		Kind: LiteralVerbatim,
		C:    '|',
	}))
	expect(t, res(parser(`☃`).parsePrimitive()), primitive(&Literal{
		Span: spanRange("☃", 0, 3),
		Kind: LiteralVerbatim,
		C:    '☃',
	}))
}

// TestParseEscape is parse_escape in parse.rs.
func TestParseEscape(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`\|`).parsePrimitive()), primitive(&Literal{
		Span: span(0, 2),
		Kind: LiteralMeta,
		C:    '|',
	}))
	specials := []struct {
		pat  string
		c    rune
		kind SpecialLiteralKind
	}{
		{`\a`, '\x07', SpecialLiteralBell},
		{`\f`, '\x0C', SpecialLiteralFormFeed},
		{`\t`, '\t', SpecialLiteralTab},
		{`\n`, '\n', SpecialLiteralLineFeed},
		{`\r`, '\r', SpecialLiteralCarriageReturn},
		{`\v`, '\x0B', SpecialLiteralVerticalTab},
	}
	for _, special := range specials {
		expect(t, res(parser(special.pat).parsePrimitive()), primitive(&Literal{
			Span:    span(0, 2),
			Kind:    LiteralSpecial,
			Special: special.kind,
			C:       special.c,
		}))
	}
	expect(t, res(parser(`\A`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionStartText,
	}))
	expect(t, res(parser(`\z`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionEndText,
	}))
	expect(t, res(parser(`\b`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionWordBoundary,
	}))
	expect(t, res(parser(`\b{start}`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 9),
		Kind: AssertionWordBoundaryStart,
	}))
	expect(t, res(parser(`\b{end}`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 7),
		Kind: AssertionWordBoundaryEnd,
	}))
	expect(t, res(parser(`\b{start-half}`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 14),
		Kind: AssertionWordBoundaryStartHalf,
	}))
	expect(t, res(parser(`\b{end-half}`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 12),
		Kind: AssertionWordBoundaryEndHalf,
	}))
	expect(t, res(parser(`\<`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionWordBoundaryStartAngle,
	}))
	expect(t, res(parser(`\>`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionWordBoundaryEndAngle,
	}))
	expect(t, res(parser(`\B`).parsePrimitive()), primitive(&Assertion{
		Span: span(0, 2),
		Kind: AssertionNotWordBoundary,
	}))

	// Most superfluous escapes are valid.
	for _, c := range []rune{'!', '@', '%', '"', '\'', '/', ' '} {
		pat := `\` + string(c)
		expect(t, res(parser(pat).parsePrimitive()), primitive(&Literal{
			Span: span(0, 2),
			Kind: LiteralSuperfluous,
			C:    c,
		}))
	}

	// Some superfluous escapes, of [0-9A-Za-z], are not valid. They are kept
	// for new syntax.
	expectErr(t, res(parser(`\e`).parseEscape()), testError{
		span: span(0, 2),
		kind: EscapeUnrecognized,
	})
	expectErr(t, res(parser(`\y`).parseEscape()), testError{
		span: span(0, 2),
		kind: EscapeUnrecognized,
	})

	// A special word boundary with no character after the brace, other than
	// white space, is ambiguous. It can be a counted repetition or a special
	// word boundary.
	expectErr(t, res(parser(`\b{`).parseEscape()), testError{
		span: span(0, 3),
		kind: SpecialWordOrRepetitionUnexpectedEOF,
	})
	expectErr(t, res(parserIgnoreWhitespace(`\b{ `).parseEscape()), testError{
		span: span(0, 4),
		kind: SpecialWordOrRepetitionUnexpectedEOF,
	})
	// When the flag x is off, the space is not in [-A-Za-z], so the parser
	// reads a counted repetition.
	expectErr(t, res(parser(`\b{ `).parse()), testError{
		span: span(2, 4),
		kind: RepetitionCountUnclosed,
	})
	// The characters look like a special word boundary, but the brace does
	// not close.
	expectErr(t, res(parser(`\b{foo`).parseEscape()), testError{
		span: span(2, 6),
		kind: SpecialWordBoundaryUnclosed,
	})
	// The same error, but a character that is not valid comes before a
	// closing brace.
	expectErr(t, res(parser(`\b{foo!}`).parseEscape()), testError{
		span: span(2, 6),
		kind: SpecialWordBoundaryUnclosed,
	})
	// The syntax is valid, but the name is not the name of a word boundary.
	expectErr(t, res(parser(`\b{foo}`).parseEscape()), testError{
		span: span(3, 6),
		kind: SpecialWordBoundaryUnrecognized,
	})

	// An escape with no end is not valid.
	expectErr(t, res(parser(`\`).parseEscape()), testError{
		span: span(0, 1),
		kind: EscapeUnexpectedEOF,
	})
}

// TestParseUnsupportedBackreference is parse_unsupported_backreference in
// parse.rs.
func TestParseUnsupportedBackreference(t *testing.T) {
	t.Parallel()
	expectErr(t, res(parser(`\0`).parseEscape()), testError{
		span: span(0, 2),
		kind: UnsupportedBackreference,
	})
	expectErr(t, res(parser(`\9`).parseEscape()), testError{
		span: span(0, 2),
		kind: UnsupportedBackreference,
	})
}

// TestParseOctal is parse_octal in parse.rs.
func TestParseOctal(t *testing.T) {
	t.Parallel()
	for i := range 511 {
		pat := fmt.Sprintf(`\%o`, i)
		expect(t, res(parserOctal(pat).parseEscape()), primitive(&Literal{
			Span: span(0, len(pat)),
			Kind: LiteralOctal,
			C:    rune(i),
		}))
	}
	expect(t, res(parserOctal(`\778`).parseEscape()), primitive(&Literal{
		Span: span(0, 3),
		Kind: LiteralOctal,
		C:    '?',
	}))
	expect(t, res(parserOctal(`\7777`).parseEscape()), primitive(&Literal{
		Span: span(0, 4),
		Kind: LiteralOctal,
		C:    'ǿ',
	}))
	expect(t, res(parserOctal(`\778`).parse()), Ast(&Concat{
		Span: span(0, 4),
		Asts: []Ast{
			&Literal{
				Span: span(0, 3),
				Kind: LiteralOctal,
				C:    '?',
			},
			&Literal{
				Span: span(3, 4),
				Kind: LiteralVerbatim,
				C:    '8',
			},
		},
	}))
	expect(t, res(parserOctal(`\7777`).parse()), Ast(&Concat{
		Span: span(0, 5),
		Asts: []Ast{
			&Literal{
				Span: span(0, 4),
				Kind: LiteralOctal,
				C:    'ǿ',
			},
			&Literal{
				Span: span(4, 5),
				Kind: LiteralVerbatim,
				C:    '7',
			},
		},
	}))

	expectErr(t, res(parserOctal(`\8`).parseEscape()), testError{
		span: span(0, 2),
		kind: EscapeUnrecognized,
	})
}

// TestParseHexTwo is parse_hex_two in parse.rs.
func TestParseHexTwo(t *testing.T) {
	t.Parallel()
	for i := range 256 {
		pat := fmt.Sprintf(`\x%02x`, i)
		expect(t, res(parser(pat).parseEscape()), primitive(&Literal{
			Span: span(0, len(pat)),
			Kind: LiteralHexFixed,
			Hex:  HexLiteralX,
			C:    rune(i),
		}))
	}

	expectErr(t, res(parser(`\xF`).parseEscape()), testError{
		span: span(3, 3),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\xG`).parseEscape()), testError{
		span: span(2, 3),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\xFG`).parseEscape()), testError{
		span: span(3, 4),
		kind: EscapeHexInvalidDigit,
	})
}

// TestParseHexFour is parse_hex_four in parse.rs.
func TestParseHexFour(t *testing.T) {
	t.Parallel()
	for i := range rune(65536) {
		if !utf8.ValidRune(i) {
			continue
		}
		pat := fmt.Sprintf(`\u%04x`, i)
		expect(t, res(parser(pat).parseEscape()), primitive(&Literal{
			Span: span(0, len(pat)),
			Kind: LiteralHexFixed,
			Hex:  HexLiteralUnicodeShort,
			C:    i,
		}))
	}

	expectErr(t, res(parser(`\uF`).parseEscape()), testError{
		span: span(3, 3),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\uG`).parseEscape()), testError{
		span: span(2, 3),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\uFG`).parseEscape()), testError{
		span: span(3, 4),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\uFFG`).parseEscape()), testError{
		span: span(4, 5),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\uFFFG`).parseEscape()), testError{
		span: span(5, 6),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\uD800`).parseEscape()), testError{
		span: span(2, 6),
		kind: EscapeHexInvalid,
	})
}

// TestParseHexEight is parse_hex_eight in parse.rs.
func TestParseHexEight(t *testing.T) {
	t.Parallel()
	for i := range rune(65536) {
		if !utf8.ValidRune(i) {
			continue
		}
		pat := fmt.Sprintf(`\U%08x`, i)
		expect(t, res(parser(pat).parseEscape()), primitive(&Literal{
			Span: span(0, len(pat)),
			Kind: LiteralHexFixed,
			Hex:  HexLiteralUnicodeLong,
			C:    i,
		}))
	}

	expectErr(t, res(parser(`\UF`).parseEscape()), testError{
		span: span(3, 3),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\UG`).parseEscape()), testError{
		span: span(2, 3),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFG`).parseEscape()), testError{
		span: span(3, 4),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFG`).parseEscape()), testError{
		span: span(4, 5),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFFG`).parseEscape()), testError{
		span: span(5, 6),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFFFG`).parseEscape()), testError{
		span: span(6, 7),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFFFFG`).parseEscape()), testError{
		span: span(7, 8),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFFFFFG`).parseEscape()), testError{
		span: span(8, 9),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\UFFFFFFFG`).parseEscape()), testError{
		span: span(9, 10),
		kind: EscapeHexInvalidDigit,
	})
}

// TestParseHexBrace is parse_hex_brace in parse.rs.
func TestParseHexBrace(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`\u{26c4}`).parseEscape()), primitive(&Literal{
		Span: span(0, 8),
		Kind: LiteralHexBrace,
		Hex:  HexLiteralUnicodeShort,
		C:    '⛄',
	}))
	expect(t, res(parser(`\U{26c4}`).parseEscape()), primitive(&Literal{
		Span: span(0, 8),
		Kind: LiteralHexBrace,
		Hex:  HexLiteralUnicodeLong,
		C:    '⛄',
	}))
	expect(t, res(parser(`\x{26c4}`).parseEscape()), primitive(&Literal{
		Span: span(0, 8),
		Kind: LiteralHexBrace,
		Hex:  HexLiteralX,
		C:    '⛄',
	}))
	expect(t, res(parser(`\x{26C4}`).parseEscape()), primitive(&Literal{
		Span: span(0, 8),
		Kind: LiteralHexBrace,
		Hex:  HexLiteralX,
		C:    '⛄',
	}))
	expect(t, res(parser(`\x{10fFfF}`).parseEscape()), primitive(&Literal{
		Span: span(0, 10),
		Kind: LiteralHexBrace,
		Hex:  HexLiteralX,
		C:    '\U0010FFFF',
	}))

	expectErr(t, res(parser(`\x`).parseEscape()), testError{
		span: span(2, 2),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\x{`).parseEscape()), testError{
		span: span(2, 3),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\x{FF`).parseEscape()), testError{
		span: span(2, 5),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\x{}`).parseEscape()), testError{
		span: span(2, 4),
		kind: EscapeHexEmpty,
	})
	expectErr(t, res(parser(`\x{FGF}`).parseEscape()), testError{
		span: span(4, 5),
		kind: EscapeHexInvalidDigit,
	})
	expectErr(t, res(parser(`\x{FFFFFF}`).parseEscape()), testError{
		span: span(3, 9),
		kind: EscapeHexInvalid,
	})
	expectErr(t, res(parser(`\x{D800}`).parseEscape()), testError{
		span: span(3, 7),
		kind: EscapeHexInvalid,
	})
	expectErr(t, res(parser(`\x{FFFFFFFFF}`).parseEscape()), testError{
		span: span(3, 12),
		kind: EscapeHexInvalid,
	})
}

// TestParseDecimal is parse_decimal in parse.rs.
func TestParseDecimal(t *testing.T) {
	t.Parallel()
	expect(t, res(parser("123").parseDecimal()), uint32(123))
	expect(t, res(parser("0").parseDecimal()), uint32(0))
	expect(t, res(parser("01").parseDecimal()), uint32(1))

	expectErr(t, res(parser("-1").parseDecimal()), testError{span: span(0, 0), kind: DecimalEmpty})
	expectErr(t, res(parser("").parseDecimal()), testError{span: span(0, 0), kind: DecimalEmpty})
	expectErr(t, res(parser("9999999999").parseDecimal()), testError{
		span: span(0, 10),
		kind: DecimalInvalid,
	})
}

// TestParseSetClass is parse_set_class in parse.rs.
func TestParseSetClass(t *testing.T) {
	t.Parallel()
	union := func(span Span, items ...ClassSetItem) ClassSet {
		return &ClassSetUnion{Span: span, Items: items}
	}

	intersection := func(span Span, lhs, rhs ClassSet) ClassSet {
		return &ClassSetBinaryOp{
			Span: span,
			Kind: ClassSetBinaryOpIntersection,
			LHS:  lhs,
			RHS:  rhs,
		}
	}

	difference := func(span Span, lhs, rhs ClassSet) ClassSet {
		return &ClassSetBinaryOp{
			Span: span,
			Kind: ClassSetBinaryOpDifference,
			LHS:  lhs,
			RHS:  rhs,
		}
	}

	symdifference := func(span Span, lhs, rhs ClassSet) ClassSet {
		return &ClassSetBinaryOp{
			Span: span,
			Kind: ClassSetBinaryOpSymmetricDifference,
			LHS:  lhs,
			RHS:  rhs,
		}
	}

	itemset := func(item ClassSetItem) ClassSet {
		return item
	}

	itemASCII := func(cls *ClassASCII) ClassSetItem {
		return cls
	}

	itemUnicode := func(cls *ClassUnicode) ClassSetItem {
		return cls
	}

	itemPerl := func(cls *ClassPerl) ClassSetItem {
		return cls
	}

	itemBracket := func(cls *ClassBracketed) ClassSetItem {
		return cls
	}

	lit := func(span Span, c rune) ClassSetItem {
		return &Literal{
			Span: span,
			Kind: LiteralVerbatim,
			C:    c,
		}
	}

	empty := func(span Span) ClassSetItem {
		return &Empty{Span: span}
	}

	rng := func(span Span, start, end rune) ClassSetItem {
		pos1 := span.Start
		pos1.Offset = span.Start.Offset + utf8.RuneLen(start)
		pos1.Column = span.Start.Column + 1
		pos2 := span.End
		pos2.Offset = span.End.Offset - utf8.RuneLen(end)
		pos2.Column = span.End.Column - 1
		return &ClassSetRange{
			Span: span,
			Start: Literal{
				Span: span.WithEnd(pos1),
				Kind: LiteralVerbatim,
				C:    start,
			},
			End: Literal{
				Span: span.WithStart(pos2),
				Kind: LiteralVerbatim,
				C:    end,
			},
		}
	}

	alnum := func(span Span, negated bool) *ClassASCII {
		return &ClassASCII{Span: span, Kind: ClassASCIIAlnum, Negated: negated}
	}

	lower := func(span Span, negated bool) *ClassASCII {
		return &ClassASCII{Span: span, Kind: ClassASCIILower, Negated: negated}
	}

	expect(t, res(parser("[[:alnum:]]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 11),
		Negated: false,
		Kind:    itemset(itemASCII(alnum(span(1, 10), false))),
	}))
	expect(t, res(parser("[[[:alnum:]]]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 13),
		Negated: false,
		Kind: itemset(itemBracket(&ClassBracketed{
			Span:    span(1, 12),
			Negated: false,
			Kind:    itemset(itemASCII(alnum(span(2, 11), false))),
		})),
	}))
	expect(t, res(parser("[[:alnum:]&&[:lower:]]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 22),
		Negated: false,
		Kind: intersection(
			span(1, 21),
			itemset(itemASCII(alnum(span(1, 10), false))),
			itemset(itemASCII(lower(span(12, 21), false))),
		),
	}))
	expect(t, res(parser("[[:alnum:]--[:lower:]]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 22),
		Negated: false,
		Kind: difference(
			span(1, 21),
			itemset(itemASCII(alnum(span(1, 10), false))),
			itemset(itemASCII(lower(span(12, 21), false))),
		),
	}))
	expect(t, res(parser("[[:alnum:]~~[:lower:]]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 22),
		Negated: false,
		Kind: symdifference(
			span(1, 21),
			itemset(itemASCII(alnum(span(1, 10), false))),
			itemset(itemASCII(lower(span(12, 21), false))),
		),
	}))

	expect(t, res(parser("[a]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 3),
		Negated: false,
		Kind:    itemset(lit(span(1, 2), 'a')),
	}))
	expect(t, res(parser(`[a\]]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 5),
		Negated: false,
		Kind: union(
			span(1, 4),
			lit(span(1, 2), 'a'),
			&Literal{
				Span: span(2, 4),
				Kind: LiteralMeta,
				C:    ']',
			},
		),
	}))
	expect(t, res(parser(`[a\-z]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 6),
		Negated: false,
		Kind: union(
			span(1, 5),
			lit(span(1, 2), 'a'),
			&Literal{
				Span: span(2, 4),
				Kind: LiteralMeta,
				C:    '-',
			},
			lit(span(4, 5), 'z'),
		),
	}))
	expect(t, res(parser("[ab]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 4),
		Negated: false,
		Kind: union(
			span(1, 3),
			lit(span(1, 2), 'a'), lit(span(2, 3), 'b'),
		),
	}))
	expect(t, res(parser("[a-]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 4),
		Negated: false,
		Kind: union(
			span(1, 3),
			lit(span(1, 2), 'a'), lit(span(2, 3), '-'),
		),
	}))
	expect(t, res(parser("[-a]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 4),
		Negated: false,
		Kind: union(
			span(1, 3),
			lit(span(1, 2), '-'), lit(span(2, 3), 'a'),
		),
	}))
	expect(t, res(parser(`[\pL]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 5),
		Negated: false,
		Kind: itemset(itemUnicode(&ClassUnicode{
			Span:    span(1, 4),
			Negated: false,
			Kind:    ClassUnicodeOneLetter,
			Letter:  'L',
		})),
	}))
	expect(t, res(parser(`[\w]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 4),
		Negated: false,
		Kind: itemset(itemPerl(&ClassPerl{
			Span:    span(1, 3),
			Kind:    ClassPerlWord,
			Negated: false,
		})),
	}))
	expect(t, res(parser(`[a\wz]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 6),
		Negated: false,
		Kind: union(
			span(1, 5),
			lit(span(1, 2), 'a'),
			itemPerl(&ClassPerl{
				Span:    span(2, 4),
				Kind:    ClassPerlWord,
				Negated: false,
			}),
			lit(span(4, 5), 'z'),
		),
	}))

	expect(t, res(parser("[a-z]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 5),
		Negated: false,
		Kind:    itemset(rng(span(1, 4), 'a', 'z')),
	}))
	expect(t, res(parser("[a-cx-z]").parse()), Ast(&ClassBracketed{
		Span:    span(0, 8),
		Negated: false,
		Kind: union(
			span(1, 7),
			rng(span(1, 4), 'a', 'c'),
			rng(span(4, 7), 'x', 'z'),
		),
	}))
	expect(t, res(parser(`[\w&&a-cx-z]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 12),
		Negated: false,
		Kind: intersection(
			span(1, 11),
			itemset(itemPerl(&ClassPerl{
				Span:    span(1, 3),
				Kind:    ClassPerlWord,
				Negated: false,
			})),
			union(
				span(5, 11),
				rng(span(5, 8), 'a', 'c'),
				rng(span(8, 11), 'x', 'z'),
			),
		),
	}))
	expect(t, res(parser(`[a-cx-z&&\w]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 12),
		Negated: false,
		Kind: intersection(
			span(1, 11),
			union(
				span(1, 7),
				rng(span(1, 4), 'a', 'c'),
				rng(span(4, 7), 'x', 'z'),
			),
			itemset(itemPerl(&ClassPerl{
				Span:    span(9, 11),
				Kind:    ClassPerlWord,
				Negated: false,
			})),
		),
	}))
	expect(t, res(parser(`[a--b--c]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 9),
		Negated: false,
		Kind: difference(
			span(1, 8),
			difference(
				span(1, 5),
				itemset(lit(span(1, 2), 'a')),
				itemset(lit(span(4, 5), 'b')),
			),
			itemset(lit(span(7, 8), 'c')),
		),
	}))
	expect(t, res(parser(`[a~~b~~c]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 9),
		Negated: false,
		Kind: symdifference(
			span(1, 8),
			symdifference(
				span(1, 5),
				itemset(lit(span(1, 2), 'a')),
				itemset(lit(span(4, 5), 'b')),
			),
			itemset(lit(span(7, 8), 'c')),
		),
	}))
	expect(t, res(parser(`[\^&&^]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 7),
		Negated: false,
		Kind: intersection(
			span(1, 6),
			itemset(&Literal{
				Span: span(1, 3),
				Kind: LiteralMeta,
				C:    '^',
			}),
			itemset(lit(span(5, 6), '^')),
		),
	}))
	expect(t, res(parser(`[\&&&&]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 7),
		Negated: false,
		Kind: intersection(
			span(1, 6),
			itemset(&Literal{
				Span: span(1, 3),
				Kind: LiteralMeta,
				C:    '&',
			}),
			itemset(lit(span(5, 6), '&')),
		),
	}))
	expect(t, res(parser(`[&&&&]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 6),
		Negated: false,
		Kind: intersection(
			span(1, 5),
			intersection(
				span(1, 3),
				itemset(empty(span(1, 1))),
				itemset(empty(span(3, 3))),
			),
			itemset(empty(span(5, 5))),
		),
	}))

	pat := "[☃-⛄]"
	expect(t, res(parser(pat).parse()), Ast(&ClassBracketed{
		Span:    spanRange(pat, 0, 9),
		Negated: false,
		Kind: itemset(&ClassSetRange{
			Span: spanRange(pat, 1, 8),
			Start: Literal{
				Span: spanRange(pat, 1, 4),
				Kind: LiteralVerbatim,
				C:    '☃',
			},
			End: Literal{
				Span: spanRange(pat, 5, 8),
				Kind: LiteralVerbatim,
				C:    '⛄',
			},
		}),
	}))

	expect(t, res(parser(`[]]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 3),
		Negated: false,
		Kind:    itemset(lit(span(1, 2), ']')),
	}))
	expect(t, res(parser(`[]\[]`).parse()), Ast(&ClassBracketed{
		Span:    span(0, 5),
		Negated: false,
		Kind: union(
			span(1, 4),
			lit(span(1, 2), ']'),
			&Literal{
				Span: span(2, 4),
				Kind: LiteralMeta,
				C:    '[',
			},
		),
	}))
	expect(t, res(parser(`[\[]]`).parse()), concat(
		0, 5,
		&ClassBracketed{
			Span:    span(0, 4),
			Negated: false,
			Kind: itemset(&Literal{
				Span: span(1, 3),
				Kind: LiteralMeta,
				C:    '[',
			}),
		},
		&Literal{
			Span: span(4, 5),
			Kind: LiteralVerbatim,
			C:    ']',
		},
	))

	expectErr(t, res(parser("[").parse()), testError{
		span: span(0, 1),
		kind: ClassUnclosed,
	})
	expectErr(t, res(parser("[[").parse()), testError{
		span: span(1, 2),
		kind: ClassUnclosed,
	})
	expectErr(t, res(parser("[[-]").parse()), testError{
		span: span(0, 1),
		kind: ClassUnclosed,
	})
	expectErr(t, res(parser("[[[:alnum:]").parse()), testError{
		span: span(1, 2),
		kind: ClassUnclosed,
	})
	expectErr(t, res(parser(`[\b]`).parse()), testError{
		span: span(1, 3),
		kind: ClassEscapeInvalid,
	})
	expectErr(t, res(parser(`[\w-a]`).parse()), testError{
		span: span(1, 3),
		kind: ClassRangeLiteral,
	})
	expectErr(t, res(parser(`[a-\w]`).parse()), testError{
		span: span(3, 5),
		kind: ClassRangeLiteral,
	})
	expectErr(t, res(parser(`[z-a]`).parse()), testError{
		span: span(1, 4),
		kind: ClassRangeInvalid,
	})

	expectErr(t, res(parserIgnoreWhitespace("[a ").parse()), testError{
		span: span(0, 1),
		kind: ClassUnclosed,
	})
	expectErr(t, res(parserIgnoreWhitespace("[a- ").parse()), testError{
		span: span(0, 1),
		kind: ClassUnclosed,
	})
}

// expectSetClassOpen fails the test if parseSetClassOpen of p does not return
// set and union. Upstream compares the pair of the results with assert_eq.
func expectSetClassOpen(t *testing.T, p *parserI, set *ClassBracketed, union *ClassSetUnion) {
	t.Helper()
	gotSet, gotUnion, err := p.parseSetClassOpen()
	expect(t, res(gotSet, err), set)
	expect(t, res(gotUnion, err), union)
}

// TestParseSetClassOpen is parse_set_class_open in parse.rs.
func TestParseSetClassOpen(t *testing.T) {
	t.Parallel()
	expectSetClassOpen(t, parser("[a]"),
		&ClassBracketed{
			Span:    span(0, 1),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(1, 1),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{Span: span(1, 1), Items: []ClassSetItem{}},
	)
	expectSetClassOpen(t, parserIgnoreWhitespace("[   a]"),
		&ClassBracketed{
			Span:    span(0, 4),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(4, 4),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{Span: span(4, 4), Items: []ClassSetItem{}},
	)
	expectSetClassOpen(t, parser("[^a]"),
		&ClassBracketed{
			Span:    span(0, 2),
			Negated: true,
			Kind: &ClassSetUnion{
				Span:  span(2, 2),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{Span: span(2, 2), Items: []ClassSetItem{}},
	)
	expectSetClassOpen(t, parserIgnoreWhitespace("[ ^ a]"),
		&ClassBracketed{
			Span:    span(0, 4),
			Negated: true,
			Kind: &ClassSetUnion{
				Span:  span(4, 4),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{Span: span(4, 4), Items: []ClassSetItem{}},
	)
	expectSetClassOpen(t, parser("[-a]"),
		&ClassBracketed{
			Span:    span(0, 2),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(1, 1),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(1, 2),
			Items: []ClassSetItem{&Literal{
				Span: span(1, 2),
				Kind: LiteralVerbatim,
				C:    '-',
			}},
		},
	)
	expectSetClassOpen(t, parserIgnoreWhitespace("[ - a]"),
		&ClassBracketed{
			Span:    span(0, 4),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(2, 2),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(2, 3),
			Items: []ClassSetItem{&Literal{
				Span: span(2, 3),
				Kind: LiteralVerbatim,
				C:    '-',
			}},
		},
	)
	expectSetClassOpen(t, parser("[^-a]"),
		&ClassBracketed{
			Span:    span(0, 3),
			Negated: true,
			Kind: &ClassSetUnion{
				Span:  span(2, 2),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(2, 3),
			Items: []ClassSetItem{&Literal{
				Span: span(2, 3),
				Kind: LiteralVerbatim,
				C:    '-',
			}},
		},
	)
	expectSetClassOpen(t, parser("[--a]"),
		&ClassBracketed{
			Span:    span(0, 3),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(1, 1),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(1, 3),
			Items: []ClassSetItem{
				&Literal{
					Span: span(1, 2),
					Kind: LiteralVerbatim,
					C:    '-',
				},
				&Literal{
					Span: span(2, 3),
					Kind: LiteralVerbatim,
					C:    '-',
				},
			},
		},
	)
	expectSetClassOpen(t, parser("[]a]"),
		&ClassBracketed{
			Span:    span(0, 2),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(1, 1),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(1, 2),
			Items: []ClassSetItem{&Literal{
				Span: span(1, 2),
				Kind: LiteralVerbatim,
				C:    ']',
			}},
		},
	)
	expectSetClassOpen(t, parserIgnoreWhitespace("[ ] a]"),
		&ClassBracketed{
			Span:    span(0, 4),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(2, 2),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(2, 3),
			Items: []ClassSetItem{&Literal{
				Span: span(2, 3),
				Kind: LiteralVerbatim,
				C:    ']',
			}},
		},
	)
	expectSetClassOpen(t, parser("[^]a]"),
		&ClassBracketed{
			Span:    span(0, 3),
			Negated: true,
			Kind: &ClassSetUnion{
				Span:  span(2, 2),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(2, 3),
			Items: []ClassSetItem{&Literal{
				Span: span(2, 3),
				Kind: LiteralVerbatim,
				C:    ']',
			}},
		},
	)
	expectSetClassOpen(t, parser("[-]a]"),
		&ClassBracketed{
			Span:    span(0, 2),
			Negated: false,
			Kind: &ClassSetUnion{
				Span:  span(1, 1),
				Items: []ClassSetItem{},
			},
		},
		&ClassSetUnion{
			Span: span(1, 2),
			Items: []ClassSetItem{&Literal{
				Span: span(1, 2),
				Kind: LiteralVerbatim,
				C:    '-',
			}},
		},
	)

	setClassOpen := func(p *parserI) result[*ClassBracketed] {
		set, _, err := p.parseSetClassOpen()
		return res(set, err)
	}
	expectErr(t, setClassOpen(parser("[")), testError{
		span: span(0, 1),
		kind: ClassUnclosed,
	})
	expectErr(t, setClassOpen(parserIgnoreWhitespace("[    ")), testError{
		span: span(0, 5),
		kind: ClassUnclosed,
	})
	expectErr(t, setClassOpen(parser("[^")), testError{
		span: span(0, 2),
		kind: ClassUnclosed,
	})
	expectErr(t, setClassOpen(parser("[]")), testError{
		span: span(0, 2),
		kind: ClassUnclosed,
	})
	expectErr(t, setClassOpen(parser("[-")), testError{
		span: span(0, 0),
		kind: ClassUnclosed,
	})
	expectErr(t, setClassOpen(parser("[--")), testError{
		span: span(0, 0),
		kind: ClassUnclosed,
	})

	// See https://github.com/rust-lang/regex/issues/792.
	expectErr(t, res(parser("(?x)[-#]").parseWithComments()), testError{
		span: span(4, 4),
		kind: ClassUnclosed,
	})
}

// TestMaybeParseASCIIClass is maybe_parse_ascii_class in parse.rs.
func TestMaybeParseASCIIClass(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`[:alnum:]`).maybeParseASCIIClass(), nil), &ClassASCII{
		Span:    span(0, 9),
		Kind:    ClassASCIIAlnum,
		Negated: false,
	})
	expect(t, res(parser(`[:alnum:]A`).maybeParseASCIIClass(), nil), &ClassASCII{
		Span:    span(0, 9),
		Kind:    ClassASCIIAlnum,
		Negated: false,
	})
	expect(t, res(parser(`[:^alnum:]`).maybeParseASCIIClass(), nil), &ClassASCII{
		Span:    span(0, 10),
		Kind:    ClassASCIIAlnum,
		Negated: true,
	})

	for _, pat := range []string{`[:`, `[:^`, `[^:alnum:]`, `[:alnnum:]`, `[:alnum]`, `[:alnum:`} {
		p := parser(pat)
		if cls := p.maybeParseASCIIClass(); cls != nil {
			t.Errorf("maybeParseASCIIClass of %q = %s, want nil", pat, dump(cls))
		}
		if p.offset() != 0 {
			t.Errorf("offset after %q = %d, want 0", pat, p.offset())
		}
	}
}

// TestParseUnicodeClass is parse_unicode_class in parse.rs.
func TestParseUnicodeClass(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`\pN`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 3),
		Negated: false,
		Kind:    ClassUnicodeOneLetter,
		Letter:  'N',
	}))
	expect(t, res(parser(`\PN`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 3),
		Negated: true,
		Kind:    ClassUnicodeOneLetter,
		Letter:  'N',
	}))
	expect(t, res(parser(`\p{N}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 5),
		Negated: false,
		Kind:    ClassUnicodeNamed,
		Name:    s("N"),
	}))
	expect(t, res(parser(`\P{N}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 5),
		Negated: true,
		Kind:    ClassUnicodeNamed,
		Name:    s("N"),
	}))
	expect(t, res(parser(`\p{Greek}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 9),
		Negated: false,
		Kind:    ClassUnicodeNamed,
		Name:    s("Greek"),
	}))

	expect(t, res(parser(`\p{scx:Katakana}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 16),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpColon,
		Name:    s("scx"),
		Value:   s("Katakana"),
	}))
	expect(t, res(parser(`\p{scx=Katakana}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 16),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpEqual,
		Name:    s("scx"),
		Value:   s("Katakana"),
	}))
	expect(t, res(parser(`\p{scx!=Katakana}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 17),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpNotEqual,
		Name:    s("scx"),
		Value:   s("Katakana"),
	}))

	expect(t, res(parser(`\p{:}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 5),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpColon,
		Name:    s(""),
		Value:   s(""),
	}))
	expect(t, res(parser(`\p{=}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 5),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpEqual,
		Name:    s(""),
		Value:   s(""),
	}))
	expect(t, res(parser(`\p{!=}`).parseEscape()), primitive(&ClassUnicode{
		Span:    span(0, 6),
		Negated: false,
		Kind:    ClassUnicodeNamedValue,
		Op:      ClassUnicodeOpNotEqual,
		Name:    s(""),
		Value:   s(""),
	}))

	expectErr(t, res(parser(`\p`).parseEscape()), testError{
		span: span(2, 2),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\p{`).parseEscape()), testError{
		span: span(3, 3),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\p{N`).parseEscape()), testError{
		span: span(4, 4),
		kind: EscapeUnexpectedEOF,
	})
	expectErr(t, res(parser(`\p{Greek`).parseEscape()), testError{
		span: span(8, 8),
		kind: EscapeUnexpectedEOF,
	})

	expect(t, res(parser(`\pNz`).parse()), Ast(&Concat{
		Span: span(0, 4),
		Asts: []Ast{
			&ClassUnicode{
				Span:    span(0, 3),
				Negated: false,
				Kind:    ClassUnicodeOneLetter,
				Letter:  'N',
			},
			&Literal{
				Span: span(3, 4),
				Kind: LiteralVerbatim,
				C:    'z',
			},
		},
	}))
	expect(t, res(parser(`\p{Greek}z`).parse()), Ast(&Concat{
		Span: span(0, 10),
		Asts: []Ast{
			&ClassUnicode{
				Span:    span(0, 9),
				Negated: false,
				Kind:    ClassUnicodeNamed,
				Name:    s("Greek"),
			},
			&Literal{
				Span: span(9, 10),
				Kind: LiteralVerbatim,
				C:    'z',
			},
		},
	}))
	expectErr(t, res(parser(`\p\{`).parse()), testError{
		span: span(2, 3),
		kind: UnicodeClassInvalid,
	})
	expectErr(t, res(parser(`\P\{`).parse()), testError{
		span: span(2, 3),
		kind: UnicodeClassInvalid,
	})
}

// TestParsePerlClass is parse_perl_class in parse.rs.
func TestParsePerlClass(t *testing.T) {
	t.Parallel()
	expect(t, res(parser(`\d`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlDigit,
		Negated: false,
	}))
	expect(t, res(parser(`\D`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlDigit,
		Negated: true,
	}))
	expect(t, res(parser(`\s`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlSpace,
		Negated: false,
	}))
	expect(t, res(parser(`\S`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlSpace,
		Negated: true,
	}))
	expect(t, res(parser(`\w`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlWord,
		Negated: false,
	}))
	expect(t, res(parser(`\W`).parseEscape()), primitive(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlWord,
		Negated: true,
	}))

	expect(t, res(parser(`\d`).parse()), Ast(&ClassPerl{
		Span:    span(0, 2),
		Kind:    ClassPerlDigit,
		Negated: false,
	}))
	expect(t, res(parser(`\dz`).parse()), Ast(&Concat{
		Span: span(0, 3),
		Asts: []Ast{
			&ClassPerl{
				Span:    span(0, 2),
				Kind:    ClassPerlDigit,
				Negated: false,
			},
			&Literal{
				Span: span(2, 3),
				Kind: LiteralVerbatim,
				C:    'z',
			},
		},
	}))
}

// TestRegression454NestTooBig is regression_454_nest_too_big in parse.rs. It
// tests a fix: the nest limiter did not take one from the depth in its post
// visit, so a long pattern went over the default limit.
func TestRegression454NestTooBig(t *testing.T) {
	t.Parallel()
	pattern := `
        2(?:
          [45]\d{3}|
          7(?:
            1[0-267]|
            2[0-289]|
            3[0-29]|
            4[01]|
            5[1-3]|
            6[013]|
            7[0178]|
            91
          )|
          8(?:
            0[125]|
            [139][1-6]|
            2[0157-9]|
            41|
            6[1-35]|
            7[1-5]|
            8[1-8]|
            90
          )|
          9(?:
            0[0-2]|
            1[0-4]|
            2[568]|
            3[3-6]|
            5[5-7]|
            6[0167]|
            7[15]|
            8[0146-9]
          )
        )\d{4}
        `
	if _, err := parserNestLimit(pattern, 50).parse(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRegression455TrailingDashIgnoreWhitespace is
// regression_455_trailing_dash_ignore_whitespace in parse.rs. It tests that a
// - at the end of a class is a literal -, also in verbose mode with white
// space after the -.
func TestRegression455TrailingDashIgnoreWhitespace(t *testing.T) {
	t.Parallel()
	for _, pat := range []string{
		"(?x)[ / - ]",
		"(?x)[ a - ]",
		"(?x)[\n            a\n            - ]\n        ",
		"(?x)[\n            a # wat\n            - ]\n        ",
	} {
		if _, err := parser(pat).parse(); err != nil {
			t.Errorf("parse of %q: unexpected error: %v", pat, err)
		}
	}

	for _, pat := range []string{
		"(?x)[ / -",
		"(?x)[ / - ",
		"(?x)[\n            / -\n        ",
		"(?x)[\n            / - # wat\n        ",
	} {
		if _, err := parser(pat).parse(); err == nil {
			t.Errorf("parse of %q: no error", pat)
		}
	}
}
