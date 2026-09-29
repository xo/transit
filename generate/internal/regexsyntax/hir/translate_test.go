package hir

import (
	"errors"
	"testing"

	"github.com/xo/transit/generate/internal/regexsyntax/ast"
)

// This file ports the tests of src/hir/translate.rs, with their helpers. The
// function t of upstream is tr here, because t is the *testing.T of a Go
// test. Each helper that can fail takes the *testing.T, and the test fails
// where upstream calls unwrap.
//
// Upstream compares an Hir with assert_eq. The port compares it with
// expectHir, which calls Hir.Equal.
//
// These tests are not here, because they run only when a feature is off,
// and the port follows the default features, which turn every feature on:
// class_perl_word_disabled, class_perl_space_disabled,
// class_perl_digit_disabled, class_unicode_gencat_disabled,
// class_unicode_script_disabled and class_unicode_age_disabled. The
// attributes cfg(feature = ...) inside a test are all true, so each
// assertion that they guard is here.

// testError is an error of the translator without its pattern, to compare
// with an *Error.
//
// testError is TestError.
type testError struct {
	span ast.Span
	kind ErrorKind
}

// sp returns the span from the position (o1, l1, c1) to the position (o2,
// l2, c2), each an offset, a line and a column.
//
// sp is Span::new(Position::new(o1, l1, c1), Position::new(o2, l2, c2)).
func sp(o1, l1, c1, o2, l2, c2 int) ast.Span {
	return ast.NewSpan(ast.NewPosition(o1, l1, c1), ast.NewPosition(o2, l2, c2))
}

// parse parses a pattern with the octal syntax on.
func parse(t *testing.T, pattern string) ast.Ast {
	t.Helper()
	a, err := ast.NewParserBuilder().Octal(true).Build().Parse(pattern)
	if err != nil {
		t.Fatalf("parsing %q: %v", pattern, err)
	}
	return a
}

// tr translates a pattern with utf8 on.
//
// tr is t.
func tr(t *testing.T, pattern string) *Hir {
	t.Helper()
	h, err := NewTranslatorBuilder().UTF8(true).Build().Translate(pattern, parse(t, pattern))
	if err != nil {
		t.Fatalf("translating %q: %v", pattern, err)
	}
	return h
}

// trErr translates a pattern with utf8 on, and returns its error.
//
// trErr is t_err.
func trErr(t *testing.T, pattern string) *Error {
	t.Helper()
	h, err := NewTranslatorBuilder().UTF8(true).Build().Translate(pattern, parse(t, pattern))
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("translating %q: got %s and error %v, want an *Error", pattern, dumpValue(h), err)
	}
	return e
}

// trBytes translates a pattern with utf8 off.
//
// trBytes is t_bytes.
func trBytes(t *testing.T, pattern string) *Hir {
	t.Helper()
	h, err := NewTranslatorBuilder().UTF8(false).Build().Translate(pattern, parse(t, pattern))
	if err != nil {
		t.Fatalf("translating %q: %v", pattern, err)
	}
	return h
}

// props returns the properties of a pattern that tr translates.
func props(t *testing.T, pattern string) *Properties {
	t.Helper()
	return tr(t, pattern).Properties()
}

// propsBytes returns the properties of a pattern that trBytes translates.
//
// propsBytes is props_bytes.
func propsBytes(t *testing.T, pattern string) *Properties {
	t.Helper()
	return trBytes(t, pattern).Properties()
}

// hirLit returns a literal of the bytes of s.
//
// hirLit is hir_lit.
func hirLit(s string) *Hir {
	return hirBlit([]byte(s))
}

// hirBlit returns a literal of s.
//
// hirBlit is hir_blit.
func hirBlit(s []byte) *Hir {
	return NewLiteral(s)
}

// hirCapture returns a capture group with no name.
//
// hirCapture is hir_capture.
func hirCapture(index uint32, expr *Hir) *Hir {
	return NewCapture(Capture{Index: index, Name: "", Sub: expr})
}

// hirCaptureName returns a capture group with a name.
//
// hirCaptureName is hir_capture_name.
func hirCaptureName(index uint32, name string, expr *Hir) *Hir {
	return NewCapture(Capture{Index: index, Name: name, Sub: expr})
}

// hirQuest returns expr?.
//
// hirQuest is hir_quest.
func hirQuest(greedy bool, expr *Hir) *Hir {
	return NewRepetition(Repetition{Min: 0, Max: new(uint32(1)), Greedy: greedy, Sub: expr})
}

// hirStar returns expr*.
//
// hirStar is hir_star.
func hirStar(greedy bool, expr *Hir) *Hir {
	return NewRepetition(Repetition{Min: 0, Max: nil, Greedy: greedy, Sub: expr})
}

// hirPlus returns expr+.
//
// hirPlus is hir_plus.
func hirPlus(greedy bool, expr *Hir) *Hir {
	return NewRepetition(Repetition{Min: 1, Max: nil, Greedy: greedy, Sub: expr})
}

// hirRange returns expr{min,max}.
//
// hirRange is hir_range.
func hirRange(greedy bool, minimum uint32, maximum *uint32, expr *Hir) *Hir {
	return NewRepetition(Repetition{Min: minimum, Max: maximum, Greedy: greedy, Sub: expr})
}

// hirAlt returns the alternation of alts.
//
// hirAlt is hir_alt.
func hirAlt(alts ...*Hir) *Hir {
	return NewAlternation(alts)
}

// hirCat returns the concatenation of exprs.
//
// hirCat is hir_cat.
func hirCat(exprs ...*Hir) *Hir {
	return NewConcat(exprs)
}

// qBinary returns a query for a binary property.
//
// qBinary is ClassQuery::Binary.
func qBinary(name string) classQuery {
	return classQuery{kind: queryBinary, name: name}
}

// qByValue returns a query for a property and a value.
//
// qByValue is ClassQuery::ByValue.
func qByValue(name, value string) classQuery {
	return classQuery{kind: queryByValue, propertyName: name, propertyValue: value}
}

// hirUclassQuery returns the class of a query. It panics if the lookup
// fails, as upstream unwraps it.
//
// hirUclassQuery is hir_uclass_query.
func hirUclassQuery(query classQuery) *Hir {
	cls, err := unicodeClass(query)
	if err != nil {
		panic(err)
	}
	return NewClass(cls)
}

// hirUclassPerlWord returns the class of \w.
//
// hirUclassPerlWord is hir_uclass_perl_word.
func hirUclassPerlWord() *Hir {
	return NewClass(perlWord())
}

// hirASCIIUclass returns the Unicode class of an ASCII class.
//
// hirASCIIUclass is hir_ascii_uclass.
func hirASCIIUclass(kind ast.ClassASCIIKind) *Hir {
	var rs []ClassUnicodeRange
	for _, r := range asciiClassAsChars(kind) {
		rs = append(rs, NewClassUnicodeRange(r[0], r[1]))
	}
	return NewClass(NewClassUnicode(rs))
}

// hirASCIIBclass returns the class of bytes of an ASCII class.
//
// hirASCIIBclass is hir_ascii_bclass.
func hirASCIIBclass(kind ast.ClassASCIIKind) *Hir {
	var rs []ClassBytesRange
	for _, r := range asciiClass(kind) {
		rs = append(rs, NewClassBytesRange(r[0], r[1]))
	}
	return NewClass(NewClassBytes(rs))
}

// hirUclass returns an expression of the Unicode class of the ranges.
//
// hirUclass is hir_uclass.
func hirUclass(ranges ...[2]rune) *Hir {
	return NewClass(uclass(ranges...))
}

// hirBclass returns an expression of the class of bytes of the ranges.
//
// hirBclass is hir_bclass.
func hirBclass(ranges ...[2]byte) *Hir {
	return NewClass(bclass(ranges...))
}

// hirCaseFold returns the class of expr, case folded. It panics if expr is
// not a class.
//
// hirCaseFold is hir_case_fold.
func hirCaseFold(expr *Hir) *Hir {
	cls, ok := expr.IntoKind().(Class)
	if !ok {
		panic("cannot case fold non-class Hir expr")
	}
	cls.CaseFoldSimple()
	return NewClass(cls)
}

// hirNegate returns the class of expr, negated. It panics if expr is not a
// class.
//
// hirNegate is hir_negate.
func hirNegate(expr *Hir) *Hir {
	cls, ok := expr.IntoKind().(Class)
	if !ok {
		panic("cannot negate non-class Hir expr")
	}
	cls.Negate()
	return NewClass(cls)
}

// classCaseFold returns an expression of cls, case folded.
//
// classCaseFold is class_case_fold.
func classCaseFold(cls Class) *Hir {
	cls.CaseFoldSimple()
	return NewClass(cls)
}

// classNegate returns an expression of cls, negated.
//
// classNegate is class_negate.
func classNegate(cls Class) *Hir {
	cls.Negate()
	return NewClass(cls)
}

// hirUnion returns the union of two classes of the same kind. It panics if
// they are not.
//
// hirUnion is hir_union.
func hirUnion(expr1, expr2 *Hir) *Hir {
	switch c1 := expr1.IntoKind().(type) {
	case *ClassUnicode:
		if c2, ok := expr2.IntoKind().(*ClassUnicode); ok {
			c1.Union(c2)
			return NewClass(c1)
		}
	case *ClassBytes:
		if c2, ok := expr2.IntoKind().(*ClassBytes); ok {
			c1.Union(c2)
			return NewClass(c1)
		}
	}
	panic("cannot union non-class Hir exprs")
}

// hirDifference returns the difference of two classes of the same kind. It
// panics if they are not.
//
// hirDifference is hir_difference.
func hirDifference(expr1, expr2 *Hir) *Hir {
	switch c1 := expr1.IntoKind().(type) {
	case *ClassUnicode:
		if c2, ok := expr2.IntoKind().(*ClassUnicode); ok {
			c1.Difference(c2)
			return NewClass(c1)
		}
	case *ClassBytes:
		if c2, ok := expr2.IntoKind().(*ClassBytes); ok {
			c1.Difference(c2)
			return NewClass(c1)
		}
	}
	panic("cannot difference non-class Hir exprs")
}

// hirLook returns an assertion.
//
// hirLook is hir_look.
func hirLook(look Look) *Hir {
	return NewLook(look)
}

// expectHir fails the test if got is not equal to want.
func expectHir(t *testing.T, got, want *Hir) {
	t.Helper()
	assertEq(t, got, want)
}

// expectErr fails the test if the span or the kind of got is not the one of
// want.
func expectErr(t *testing.T, got *Error, want testError) {
	t.Helper()
	if got.Span != want.span || got.Kind != want.kind {
		t.Errorf("got error %v at %v, want %v at %v", got.Kind, got.Span, want.kind, want.span)
	}
}

// expectEq fails the test if left and right are not equal.
func expectEq[T comparable](t *testing.T, left, right T) {
	t.Helper()
	if left != right {
		t.Errorf("assertion `left == right` failed\n  left: %v\n right: %v", left, right)
	}
}

// opt is an int or no int, to compare an Option<usize> of upstream.
type opt struct {
	n  int
	ok bool
}

// some returns the int n.
func some(n int) opt {
	return opt{n: n, ok: true}
}

// optOf returns the result of a method such as Properties.MinimumLen as an
// opt.
func optOf(n int, ok bool) opt {
	if !ok {
		return opt{}
	}
	return some(n)
}

// TestEmpty is empty in translate.rs.
func TestEmpty(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, ""), NewEmpty())
	expectHir(t, tr(t, "(?i)"), NewEmpty())
	expectHir(t, tr(t, "()"), hirCapture(1, NewEmpty()))
	expectHir(t, tr(t, "(?:)"), NewEmpty())
	expectHir(t, tr(t, "(?P<wat>)"), hirCaptureName(1, "wat", NewEmpty()))
	expectHir(t, tr(t, "|"), hirAlt(NewEmpty(), NewEmpty()))
	expectHir(t,
		tr(t, "()|()"),
		hirAlt(
			hirCapture(1, NewEmpty()),
			hirCapture(2, NewEmpty())),
	)
	expectHir(t,
		tr(t, "(|b)"),
		hirCapture(1, hirAlt(NewEmpty(), hirLit("b"))),
	)
	expectHir(t,
		tr(t, "(a|)"),
		hirCapture(1, hirAlt(hirLit("a"), NewEmpty())),
	)
	expectHir(t,
		tr(t, "(a||c)"),
		hirCapture(
			1,
			hirAlt(hirLit("a"), NewEmpty(), hirLit("c")),
		),
	)
	expectHir(t,
		tr(t, "(||)"),
		hirCapture(
			1,
			hirAlt(NewEmpty(), NewEmpty(), NewEmpty()),
		),
	)
}

// TestLiteral is literal in translate.rs.
func TestLiteral(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "a"), hirLit("a"))
	expectHir(t, tr(t, "(?-u)a"), hirLit("a"))
	expectHir(t, tr(t, "☃"), hirLit("☃"))
	expectHir(t, tr(t, "abcd"), hirLit("abcd"))
	expectHir(t, trBytes(t, "(?-u)a"), hirLit("a"))
	expectHir(t, trBytes(t, "(?-u)\x61"), hirLit("a"))
	expectHir(t, trBytes(t, `(?-u)\x61`), hirLit("a"))
	expectHir(t, trBytes(t, `(?-u)\xFF`), hirBlit([]byte("\xFF")))
	expectHir(t, tr(t, "(?-u)☃"), hirLit("☃"))
	expectErr(t, trErr(t, `(?-u)\xFF`),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 9, 1, 10)},
	)
}

// TestLiteralCaseInsensitive is literal_case_insensitive in translate.rs.
func TestLiteralCaseInsensitive(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "(?i)a"), hirUclass(ur('A', 'A'), ur('a', 'a')))
	expectHir(t, tr(t, "(?i:a)"), hirUclass(ur('A', 'A'), ur('a', 'a')))
	expectHir(t,
		tr(t, "a(?i)a(?-i)a"),
		hirCat(
			hirLit("a"),
			hirUclass(ur('A', 'A'), ur('a', 'a')),
			hirLit("a")),
	)
	expectHir(t,
		tr(t, "(?i)ab@c"),
		hirCat(
			hirUclass(ur('A', 'A'), ur('a', 'a')),
			hirUclass(ur('B', 'B'), ur('b', 'b')),
			hirLit("@"),
			hirUclass(ur('C', 'C'), ur('c', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i)β"),
		hirUclass(ur('Β', 'Β'), ur('β', 'β'), ur('ϐ', 'ϐ')),
	)
	expectHir(t, tr(t, "(?i-u)a"), hirBclass(br('A', 'A'), br('a', 'a')))
	expectHir(t,
		tr(t, "(?-u)a(?i)a(?-i)a"),
		hirCat(
			hirLit("a"),
			hirBclass(br('A', 'A'), br('a', 'a')),
			hirLit("a")),
	)
	expectHir(t,
		tr(t, "(?i-u)ab@c"),
		hirCat(
			hirBclass(br('A', 'A'), br('a', 'a')),
			hirBclass(br('B', 'B'), br('b', 'b')),
			hirLit("@"),
			hirBclass(br('C', 'C'), br('c', 'c'))),
	)
	expectHir(t,
		trBytes(t, "(?i-u)a"),
		hirBclass(br('A', 'A'), br('a', 'a')),
	)
	expectHir(t,
		trBytes(t, "(?i-u)\x61"),
		hirBclass(br('A', 'A'), br('a', 'a')),
	)
	expectHir(t,
		trBytes(t, `(?i-u)\x61`),
		hirBclass(br('A', 'A'), br('a', 'a')),
	)
	expectHir(t, trBytes(t, `(?i-u)\xFF`), hirBlit([]byte("\xFF")))
	expectHir(t, tr(t, "(?i-u)β"), hirLit("β"))
}

// TestDot is dot in translate.rs.
func TestDot(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, "."),
		hirUclass(ur('\x00', '\t'), ur('\x0B', '\U0010FFFF')),
	)
	expectHir(t,
		tr(t, "(?R)."),
		hirUclass(
			ur('\x00', '\t'),
			ur('\x0B', '\x0C'),
			ur('\x0E', '\U0010FFFF')),
	)
	expectHir(t, tr(t, "(?s)."), hirUclass(ur('\x00', '\U0010FFFF')))
	expectHir(t, tr(t, "(?Rs)."), hirUclass(ur('\x00', '\U0010FFFF')))
	expectHir(t,
		trBytes(t, "(?-u)."),
		hirBclass(br('\x00', '\t'), br('\x0B', '\xFF')),
	)
	expectHir(t,
		trBytes(t, "(?R-u)."),
		hirBclass(
			br('\x00', '\t'),
			br('\x0B', '\x0C'),
			br('\x0E', '\xFF')),
	)
	expectHir(t, trBytes(t, "(?s-u)."), hirBclass(br('\x00', '\xFF')))
	expectHir(t, trBytes(t, "(?Rs-u)."), hirBclass(br('\x00', '\xFF')))
	// If invalid UTF-8 is not allowed, then a . outside Unicode mode is not
	// allowed.
	expectErr(t, trErr(t, "(?-u)."),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 6, 1, 7)},
	)
	expectErr(t, trErr(t, "(?R-u)."),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 7, 1, 8)},
	)
	expectErr(t, trErr(t, "(?s-u)."),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 7, 1, 8)},
	)
	expectErr(t, trErr(t, "(?Rs-u)."),
		testError{kind: InvalidUTF8, span: sp(7, 1, 8, 8, 1, 9)},
	)
}

// TestAssertions is assertions in translate.rs.
func TestAssertions(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "^"), hirLook(LookStart))
	expectHir(t, tr(t, "$"), hirLook(LookEnd))
	expectHir(t, tr(t, `\A`), hirLook(LookStart))
	expectHir(t, tr(t, `\z`), hirLook(LookEnd))
	expectHir(t, tr(t, "(?m)^"), hirLook(LookStartLF))
	expectHir(t, tr(t, "(?m)$"), hirLook(LookEndLF))
	expectHir(t, tr(t, `(?m)\A`), hirLook(LookStart))
	expectHir(t, tr(t, `(?m)\z`), hirLook(LookEnd))
	expectHir(t, tr(t, `\b`), hirLook(LookWordUnicode))
	expectHir(t, tr(t, `\B`), hirLook(LookWordUnicodeNegate))
	expectHir(t, tr(t, `(?-u)\b`), hirLook(LookWordASCII))
	expectHir(t, tr(t, `(?-u)\B`), hirLook(LookWordASCIINegate))
}

// TestGroup is group in translate.rs.
func TestGroup(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "(a)"), hirCapture(1, hirLit("a")))
	expectHir(t,
		tr(t, "(a)(b)"),
		hirCat(
			hirCapture(1, hirLit("a")),
			hirCapture(2, hirLit("b"))),
	)
	expectHir(t,
		tr(t, "(a)|(b)"),
		hirAlt(
			hirCapture(1, hirLit("a")),
			hirCapture(2, hirLit("b"))),
	)
	expectHir(t, tr(t, "(?P<foo>)"), hirCaptureName(1, "foo", NewEmpty()))
	expectHir(t, tr(t, "(?P<foo>a)"), hirCaptureName(1, "foo", hirLit("a")))
	expectHir(t,
		tr(t, "(?P<foo>a)(?P<bar>b)"),
		hirCat(
			hirCaptureName(1, "foo", hirLit("a")),
			hirCaptureName(2, "bar", hirLit("b"))),
	)
	expectHir(t, tr(t, "(?:)"), NewEmpty())
	expectHir(t, tr(t, "(?:a)"), hirLit("a"))
	expectHir(t,
		tr(t, "(?:a)(b)"),
		hirCat(hirLit("a"), hirCapture(1, hirLit("b"))),
	)
	expectHir(t,
		tr(t, "(a)(?:b)(c)"),
		hirCat(
			hirCapture(1, hirLit("a")),
			hirLit("b"),
			hirCapture(2, hirLit("c"))),
	)
	expectHir(t,
		tr(t, "(a)(?P<foo>b)(c)"),
		hirCat(
			hirCapture(1, hirLit("a")),
			hirCaptureName(2, "foo", hirLit("b")),
			hirCapture(3, hirLit("c"))),
	)
	expectHir(t, tr(t, "()"), hirCapture(1, NewEmpty()))
	expectHir(t, tr(t, "((?i))"), hirCapture(1, NewEmpty()))
	expectHir(t, tr(t, "((?x))"), hirCapture(1, NewEmpty()))
	expectHir(t,
		tr(t, "(((?x)))"),
		hirCapture(1, hirCapture(2, NewEmpty())),
	)
}

// TestLineAnchors is line_anchors in translate.rs.
func TestLineAnchors(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "^"), hirLook(LookStart))
	expectHir(t, tr(t, "$"), hirLook(LookEnd))
	expectHir(t, tr(t, `\A`), hirLook(LookStart))
	expectHir(t, tr(t, `\z`), hirLook(LookEnd))
	expectHir(t, tr(t, `(?m)\A`), hirLook(LookStart))
	expectHir(t, tr(t, `(?m)\z`), hirLook(LookEnd))
	expectHir(t, tr(t, "(?m)^"), hirLook(LookStartLF))
	expectHir(t, tr(t, "(?m)$"), hirLook(LookEndLF))
	expectHir(t, tr(t, `(?R)\A`), hirLook(LookStart))
	expectHir(t, tr(t, `(?R)\z`), hirLook(LookEnd))
	expectHir(t, tr(t, "(?R)^"), hirLook(LookStart))
	expectHir(t, tr(t, "(?R)$"), hirLook(LookEnd))
	expectHir(t, tr(t, `(?Rm)\A`), hirLook(LookStart))
	expectHir(t, tr(t, `(?Rm)\z`), hirLook(LookEnd))
	expectHir(t, tr(t, "(?Rm)^"), hirLook(LookStartCRLF))
	expectHir(t, tr(t, "(?Rm)$"), hirLook(LookEndCRLF))
}

// TestFlags is flags in translate.rs.
func TestFlags(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, "(?i:a)a"),
		hirCat(
			hirUclass(ur('A', 'A'), ur('a', 'a')), hirLit("a"),
		),
	)
	expectHir(t,
		tr(t, "(?i-u:a)β"),
		hirCat(
			hirBclass(br('A', 'A'), br('a', 'a')),
			hirLit("β")),
	)
	expectHir(t,
		tr(t, "(?:(?i-u)a)b"),
		hirCat(
			hirBclass(br('A', 'A'), br('a', 'a')),
			hirLit("b")),
	)
	expectHir(t,
		tr(t, "((?i-u)a)b"),
		hirCat(
			hirCapture(1, hirBclass(br('A', 'A'), br('a', 'a'))),
			hirLit("b")),
	)
	expectHir(t,
		tr(t, "(?i)(?-i:a)a"),
		hirCat(
			hirLit("a"), hirUclass(ur('A', 'A'), ur('a', 'a')),
		),
	)
	expectHir(t,
		tr(t, "(?im)a^"),
		hirCat(
			hirUclass(ur('A', 'A'), ur('a', 'a')),
			hirLook(LookStartLF)),
	)
	expectHir(t,
		tr(t, "(?im)a^(?i-m)a^"),
		hirCat(
			hirUclass(ur('A', 'A'), ur('a', 'a')),
			hirLook(LookStartLF),
			hirUclass(ur('A', 'A'), ur('a', 'a')),
			hirLook(LookStart)),
	)
	expectHir(t,
		tr(t, "(?U)a*a*?(?-U)a*a*?"),
		hirCat(
			hirStar(false, hirLit("a")),
			hirStar(true, hirLit("a")),
			hirStar(true, hirLit("a")),
			hirStar(false, hirLit("a"))),
	)
	expectHir(t,
		tr(t, "(?:a(?i)a)a"),
		hirCat(
			hirCat(
				hirLit("a"),
				hirUclass(ur('A', 'A'), ur('a', 'a'))),
			hirLit("a")),
	)
	expectHir(t,
		tr(t, "(?i)(?:a(?-i)a)a"),
		hirCat(
			hirCat(
				hirUclass(ur('A', 'A'), ur('a', 'a')),
				hirLit("a")),
			hirUclass(ur('A', 'A'), ur('a', 'a'))),
	)
}

// TestEscape is escape in translate.rs.
func TestEscape(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, `\\\.\+\*\?\(\)\|\[\]\{\}\^\$\#`),
		hirLit(`\.+*?()|[]{}^$#`),
	)
}

// TestRepetition is repetition in translate.rs.
func TestRepetition(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "a?"), hirQuest(true, hirLit("a")))
	expectHir(t, tr(t, "a*"), hirStar(true, hirLit("a")))
	expectHir(t, tr(t, "a+"), hirPlus(true, hirLit("a")))
	expectHir(t, tr(t, "a??"), hirQuest(false, hirLit("a")))
	expectHir(t, tr(t, "a*?"), hirStar(false, hirLit("a")))
	expectHir(t, tr(t, "a+?"), hirPlus(false, hirLit("a")))
	expectHir(t, tr(t, "a{1}"), hirRange(true, 1, new(uint32(1)), hirLit("a")))
	expectHir(t, tr(t, "a{1,}"), hirRange(true, 1, nil, hirLit("a")))
	expectHir(t, tr(t, "a{1,2}"), hirRange(true, 1, new(uint32(2)), hirLit("a")))
	expectHir(t, tr(t, "a{1}?"), hirRange(false, 1, new(uint32(1)), hirLit("a")))
	expectHir(t, tr(t, "a{1,}?"), hirRange(false, 1, nil, hirLit("a")))
	expectHir(t, tr(t, "a{1,2}?"), hirRange(false, 1, new(uint32(2)), hirLit("a")))
	expectHir(t,
		tr(t, "ab?"),
		hirCat(hirLit("a"), hirQuest(true, hirLit("b"))),
	)
	expectHir(t, tr(t, "(ab)?"), hirQuest(true, hirCapture(1, hirLit("ab"))))
	expectHir(t,
		tr(t, "a|b?"),
		hirAlt(hirLit("a"), hirQuest(true, hirLit("b"))),
	)
}

// TestCatAlt is cat_alt in translate.rs.
func TestCatAlt(t *testing.T) {
	t.Parallel()

	a := func() *Hir { return hirLook(LookStart) }
	b := func() *Hir { return hirLook(LookEnd) }
	c := func() *Hir { return hirLook(LookWordUnicode) }
	d := func() *Hir { return hirLook(LookWordUnicodeNegate) }

	expectHir(t, tr(t, "(^$)"), hirCapture(1, hirCat(a(), b())))
	expectHir(t, tr(t, "^|$"), hirAlt(a(), b()))
	expectHir(t, tr(t, `^|$|\b`), hirAlt(a(), b(), c()))
	expectHir(t,
		tr(t, `^$|$\b|\b\B`),
		hirAlt(
			hirCat(a(), b()),
			hirCat(b(), c()),
			hirCat(c(), d())),
	)
	expectHir(t, tr(t, "(^|$)"), hirCapture(1, hirAlt(a(), b())))
	expectHir(t,
		tr(t, `(^|$|\b)`),
		hirCapture(1, hirAlt(a(), b(), c())),
	)
	expectHir(t,
		tr(t, `(^$|$\b|\b\B)`),
		hirCapture(
			1,
			hirAlt(
				hirCat(a(), b()),
				hirCat(b(), c()),
				hirCat(c(), d())),
		),
	)
	expectHir(t,
		tr(t, `(^$|($\b|(\b\B)))`),
		hirCapture(
			1,
			hirAlt(
				hirCat(a(), b()),
				hirCapture(
					2,
					hirAlt(
						hirCat(b(), c()),
						hirCapture(3, hirCat(c(), d()))),
				)),
		),
	)
}

// TestCatClassFlattened is cat_class_flattened in translate.rs.
func TestCatClassFlattened(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `[a-z]|[A-Z]`), hirUclass(ur('A', 'Z'), ur('a', 'z')))
	// The union of all the letter properties is the one large letter
	// property.
	expectHir(t,
		tr(t, `(?x)
                \p{Lowercase_Letter}
                |\p{Uppercase_Letter}
                |\p{Titlecase_Letter}
                |\p{Modifier_Letter}
                |\p{Other_Letter}
            `),
		hirUclassQuery(qBinary("letter")),
	)
	// A class of bytes that can match invalid UTF-8 cannot join a Unicode
	// class.
	expectHir(t,
		trBytes(t, `[Δδ]|(?-u:[\x90-\xFF])|[Λλ]`),
		hirAlt(
			hirUclass(ur('Δ', 'Δ'), ur('δ', 'δ')),
			hirBclass(br('\x90', '\xFF')),
			hirUclass(ur('Λ', 'Λ'), ur('λ', 'λ'))),
	)
	// Classes of bytes alone can join, even if some are ASCII and others
	// are invalid UTF-8.
	expectHir(t,
		trBytes(t, `[a-z]|(?-u:[\x90-\xFF])|[A-Z]`),
		hirBclass(br('A', 'Z'), br('a', 'z'), br('\x90', '\xFF')),
	)
}

// TestClassASCII is class_ascii in translate.rs.
func TestClassASCII(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, "[[:alnum:]]"),
		hirASCIIUclass(ast.ClassASCIIAlnum),
	)
	expectHir(t,
		tr(t, "[[:alpha:]]"),
		hirASCIIUclass(ast.ClassASCIIAlpha),
	)
	expectHir(t,
		tr(t, "[[:ascii:]]"),
		hirASCIIUclass(ast.ClassASCIIASCII),
	)
	expectHir(t,
		tr(t, "[[:blank:]]"),
		hirASCIIUclass(ast.ClassASCIIBlank),
	)
	expectHir(t,
		tr(t, "[[:cntrl:]]"),
		hirASCIIUclass(ast.ClassASCIICntrl),
	)
	expectHir(t,
		tr(t, "[[:digit:]]"),
		hirASCIIUclass(ast.ClassASCIIDigit),
	)
	expectHir(t,
		tr(t, "[[:graph:]]"),
		hirASCIIUclass(ast.ClassASCIIGraph),
	)
	expectHir(t,
		tr(t, "[[:lower:]]"),
		hirASCIIUclass(ast.ClassASCIILower),
	)
	expectHir(t,
		tr(t, "[[:print:]]"),
		hirASCIIUclass(ast.ClassASCIIPrint),
	)
	expectHir(t,
		tr(t, "[[:punct:]]"),
		hirASCIIUclass(ast.ClassASCIIPunct),
	)
	expectHir(t,
		tr(t, "[[:space:]]"),
		hirASCIIUclass(ast.ClassASCIISpace),
	)
	expectHir(t,
		tr(t, "[[:upper:]]"),
		hirASCIIUclass(ast.ClassASCIIUpper),
	)
	expectHir(t,
		tr(t, "[[:word:]]"),
		hirASCIIUclass(ast.ClassASCIIWord),
	)
	expectHir(t,
		tr(t, "[[:xdigit:]]"),
		hirASCIIUclass(ast.ClassASCIIXdigit),
	)
	expectHir(t,
		tr(t, "[[:^lower:]]"),
		hirNegate(hirASCIIUclass(ast.ClassASCIILower)),
	)
	expectHir(t,
		tr(t, "(?i)[[:lower:]]"),
		hirUclass(
			ur('A', 'Z'),
			ur('a', 'z'),
			ur('\u017F', '\u017F'),
			ur('\u212A', '\u212A')),
	)
	expectHir(t,
		tr(t, "(?-u)[[:lower:]]"),
		hirASCIIBclass(ast.ClassASCIILower),
	)
	expectHir(t,
		tr(t, "(?i-u)[[:lower:]]"),
		hirCaseFold(hirASCIIBclass(ast.ClassASCIILower)),
	)
	expectErr(t, trErr(t, "(?-u)[[:^lower:]]"),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 16, 1, 17)},
	)
	expectErr(t, trErr(t, "(?i-u)[[:^lower:]]"),
		testError{kind: InvalidUTF8, span: sp(7, 1, 8, 17, 1, 18)},
	)
}

// TestClassASCIIMultiple is class_ascii_multiple in translate.rs.
func TestClassASCIIMultiple(t *testing.T) {
	t.Parallel()

	// See https://github.com/rust-lang/regex/issues/680.
	expectHir(t,
		tr(t, "[[:alnum:][:^ascii:]]"),
		hirUnion(
			hirASCIIUclass(ast.ClassASCIIAlnum),
			hirUclass(ur('\u0080', '\U0010FFFF')),
		),
	)
	expectHir(t,
		trBytes(t, "(?-u)[[:alnum:][:^ascii:]]"),
		hirUnion(
			hirASCIIBclass(ast.ClassASCIIAlnum),
			hirBclass(br(0x80, 0xFF)),
		),
	)
}

// TestClassPerlUnicode is class_perl_unicode in translate.rs.
func TestClassPerlUnicode(t *testing.T) {
	t.Parallel()

	// Unicode
	expectHir(t, tr(t, `\d`), hirUclassQuery(qBinary("digit")))
	expectHir(t, tr(t, `\s`), hirUclassQuery(qBinary("space")))
	expectHir(t, tr(t, `\w`), hirUclassPerlWord())
	expectHir(t,
		tr(t, `(?i)\d`),
		hirUclassQuery(qBinary("digit")),
	)
	expectHir(t,
		tr(t, `(?i)\s`),
		hirUclassQuery(qBinary("space")),
	)
	expectHir(t, tr(t, `(?i)\w`), hirUclassPerlWord())
	// Unicode, negated
	expectHir(t,
		tr(t, `\D`),
		hirNegate(hirUclassQuery(qBinary("digit"))),
	)
	expectHir(t,
		tr(t, `\S`),
		hirNegate(hirUclassQuery(qBinary("space"))),
	)
	expectHir(t, tr(t, `\W`), hirNegate(hirUclassPerlWord()))
	expectHir(t,
		tr(t, `(?i)\D`),
		hirNegate(hirUclassQuery(qBinary("digit"))),
	)
	expectHir(t,
		tr(t, `(?i)\S`),
		hirNegate(hirUclassQuery(qBinary("space"))),
	)
	expectHir(t, tr(t, `(?i)\W`), hirNegate(hirUclassPerlWord()))
}

// TestClassPerlASCII is class_perl_ascii in translate.rs.
func TestClassPerlASCII(t *testing.T) {
	t.Parallel()

	// ASCII only
	expectHir(t,
		tr(t, `(?-u)\d`),
		hirASCIIBclass(ast.ClassASCIIDigit),
	)
	expectHir(t,
		tr(t, `(?-u)\s`),
		hirASCIIBclass(ast.ClassASCIISpace),
	)
	expectHir(t,
		tr(t, `(?-u)\w`),
		hirASCIIBclass(ast.ClassASCIIWord),
	)
	expectHir(t,
		tr(t, `(?i-u)\d`),
		hirASCIIBclass(ast.ClassASCIIDigit),
	)
	expectHir(t,
		tr(t, `(?i-u)\s`),
		hirASCIIBclass(ast.ClassASCIISpace),
	)
	expectHir(t,
		tr(t, `(?i-u)\w`),
		hirASCIIBclass(ast.ClassASCIIWord),
	)
	// ASCII only, negated
	expectHir(t,
		trBytes(t, `(?-u)\D`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIDigit)),
	)
	expectHir(t,
		trBytes(t, `(?-u)\S`),
		hirNegate(hirASCIIBclass(ast.ClassASCIISpace)),
	)
	expectHir(t,
		trBytes(t, `(?-u)\W`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIWord)),
	)
	expectHir(t,
		trBytes(t, `(?i-u)\D`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIDigit)),
	)
	expectHir(t,
		trBytes(t, `(?i-u)\S`),
		hirNegate(hirASCIIBclass(ast.ClassASCIISpace)),
	)
	expectHir(t,
		trBytes(t, `(?i-u)\W`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIWord)),
	)
	// ASCII only, negated, with UTF-8 mode on. Here the negation of any
	// Perl class is an error, because each such class can match invalid
	// UTF-8.
	expectErr(t, trErr(t, `(?-u)\D`),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 7, 1, 8)},
	)
	expectErr(t, trErr(t, `(?-u)\S`),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 7, 1, 8)},
	)
	expectErr(t, trErr(t, `(?-u)\W`),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 7, 1, 8)},
	)
	expectErr(t, trErr(t, `(?i-u)\D`),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 8, 1, 9)},
	)
	expectErr(t, trErr(t, `(?i-u)\S`),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 8, 1, 9)},
	)
	expectErr(t, trErr(t, `(?i-u)\W`),
		testError{kind: InvalidUTF8, span: sp(6, 1, 7, 8, 1, 9)},
	)
}

// TestClassUnicodeGencat is class_unicode_gencat in translate.rs.
func TestClassUnicodeGencat(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `\pZ`), hirUclassQuery(qBinary("Z")))
	expectHir(t, tr(t, `\pz`), hirUclassQuery(qBinary("Z")))
	expectHir(t,
		tr(t, `\p{Separator}`),
		hirUclassQuery(qBinary("Z")),
	)
	expectHir(t,
		tr(t, `\p{se      PaRa ToR}`),
		hirUclassQuery(qBinary("Z")),
	)
	expectHir(t,
		tr(t, `\p{gc:Separator}`),
		hirUclassQuery(qBinary("Z")),
	)
	expectHir(t,
		tr(t, `\p{gc=Separator}`),
		hirUclassQuery(qBinary("Z")),
	)
	expectHir(t,
		tr(t, `\p{gc!=Separator}`),
		hirNegate(hirUclassQuery(qBinary("Z"))),
	)
	expectHir(t,
		tr(t, `\p{Other}`),
		hirUclassQuery(qBinary("Other")),
	)
	expectHir(t, tr(t, `\pC`), hirUclassQuery(qBinary("Other")))
	expectHir(t,
		tr(t, `\PZ`),
		hirNegate(hirUclassQuery(qBinary("Z"))),
	)
	expectHir(t,
		tr(t, `\P{separator}`),
		hirNegate(hirUclassQuery(qBinary("Z"))),
	)
	expectHir(t,
		tr(t, `\P{gc!=separator}`),
		hirUclassQuery(qBinary("Z")),
	)
	expectHir(t, tr(t, `\p{any}`), hirUclassQuery(qBinary("Any")))
	expectHir(t,
		tr(t, `\p{assigned}`),
		hirUclassQuery(qBinary("Assigned")),
	)
	expectHir(t,
		tr(t, `\p{ascii}`),
		hirUclassQuery(qBinary("ASCII")),
	)
	expectHir(t,
		tr(t, `\p{gc:any}`),
		hirUclassQuery(qBinary("Any")),
	)
	expectHir(t,
		tr(t, `\p{gc:assigned}`),
		hirUclassQuery(qBinary("Assigned")),
	)
	expectHir(t,
		tr(t, `\p{gc:ascii}`),
		hirUclassQuery(qBinary("ASCII")),
	)
	expectErr(t, trErr(t, `(?-u)\pZ`),
		testError{kind: UnicodeNotAllowed, span: sp(5, 1, 6, 8, 1, 9)},
	)
	expectErr(t, trErr(t, `(?-u)\p{Separator}`),
		testError{kind: UnicodeNotAllowed, span: sp(5, 1, 6, 18, 1, 19)},
	)
	expectErr(t, trErr(t, `\pE`),
		testError{kind: UnicodePropertyNotFound, span: sp(0, 1, 1, 3, 1, 4)},
	)
	expectErr(t, trErr(t, `\p{Foo}`),
		testError{kind: UnicodePropertyNotFound, span: sp(0, 1, 1, 7, 1, 8)},
	)
	expectErr(t, trErr(t, `\p{gc:Foo}`),
		testError{kind: UnicodePropertyValueNotFound, span: sp(0, 1, 1, 10, 1, 11)},
	)
}

// TestClassUnicodeScript is class_unicode_script in translate.rs.
func TestClassUnicodeScript(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, `\p{Greek}`),
		hirUclassQuery(qBinary("Greek")),
	)
	expectHir(t,
		tr(t, `(?i)\p{Greek}`),
		hirCaseFold(hirUclassQuery(qBinary("Greek"))),
	)
	expectHir(t,
		tr(t, `(?i)\P{Greek}`),
		hirNegate(hirCaseFold(hirUclassQuery(qBinary(
			"Greek",
		)))),
	)
	expectErr(t, trErr(t, `\p{sc:Foo}`),
		testError{kind: UnicodePropertyValueNotFound, span: sp(0, 1, 1, 10, 1, 11)},
	)
	expectErr(t, trErr(t, `\p{scx:Foo}`),
		testError{kind: UnicodePropertyValueNotFound, span: sp(0, 1, 1, 11, 1, 12)},
	)
}

// TestClassUnicodeAge is class_unicode_age in translate.rs.
func TestClassUnicodeAge(t *testing.T) {
	t.Parallel()

	expectErr(t, trErr(t, `\p{age:Foo}`),
		testError{kind: UnicodePropertyValueNotFound, span: sp(0, 1, 1, 11, 1, 12)},
	)
}

// TestClassUnicodeAnyEmpty is class_unicode_any_empty in translate.rs.
func TestClassUnicodeAnyEmpty(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `\P{any}`), hirUclass())
}

// TestClassBracketed is class_bracketed in translate.rs.
func TestClassBracketed(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "[a]"), hirLit("a"))
	expectHir(t, tr(t, "[ab]"), hirUclass(ur('a', 'b')))
	expectHir(t, tr(t, "[^[a]]"), classNegate(uclass(ur('a', 'a'))))
	expectHir(t, tr(t, "[a-z]"), hirUclass(ur('a', 'z')))
	expectHir(t, tr(t, "[a-fd-h]"), hirUclass(ur('a', 'h')))
	expectHir(t, tr(t, "[a-fg-m]"), hirUclass(ur('a', 'm')))
	expectHir(t, tr(t, `[\x00]`), hirUclass(ur('\x00', '\x00')))
	expectHir(t, tr(t, `[\n]`), hirUclass(ur('\n', '\n')))
	expectHir(t, tr(t, "[\n]"), hirUclass(ur('\n', '\n')))
	expectHir(t, tr(t, `[\d]`), hirUclassQuery(qBinary("digit")))
	expectHir(t,
		tr(t, `[\pZ]`),
		hirUclassQuery(qBinary("separator")),
	)
	expectHir(t,
		tr(t, `[\p{separator}]`),
		hirUclassQuery(qBinary("separator")),
	)
	expectHir(t, tr(t, `[^\D]`), hirUclassQuery(qBinary("digit")))
	expectHir(t,
		tr(t, `[^\PZ]`),
		hirUclassQuery(qBinary("separator")),
	)
	expectHir(t,
		tr(t, `[^\P{separator}]`),
		hirUclassQuery(qBinary("separator")),
	)
	expectHir(t,
		tr(t, `(?i)[^\D]`),
		hirUclassQuery(qBinary("digit")),
	)
	expectHir(t,
		tr(t, `(?i)[^\P{greek}]`),
		hirCaseFold(hirUclassQuery(qBinary("greek"))),
	)
	expectHir(t, tr(t, "(?-u)[a]"), hirBclass(br('a', 'a')))
	expectHir(t, tr(t, `(?-u)[\x00]`), hirBclass(br('\x00', '\x00')))
	expectHir(t, trBytes(t, `(?-u)[\xFF]`), hirBclass(br('\xFF', '\xFF')))
	expectHir(t, tr(t, "(?i)[a]"), hirUclass(ur('A', 'A'), ur('a', 'a')))
	expectHir(t,
		tr(t, "(?i)[k]"),
		hirUclass(ur('K', 'K'), ur('k', 'k'), ur('\u212A', '\u212A')),
	)
	expectHir(t,
		tr(t, "(?i)[β]"),
		hirUclass(ur('Β', 'Β'), ur('β', 'β'), ur('ϐ', 'ϐ')),
	)
	expectHir(t, tr(t, "(?i-u)[k]"), hirBclass(br('K', 'K'), br('k', 'k')))
	expectHir(t, tr(t, "[^a]"), classNegate(uclass(ur('a', 'a'))))
	expectHir(t, tr(t, `[^\x00]`), classNegate(uclass(ur('\x00', '\x00'))))
	expectHir(t,
		trBytes(t, "(?-u)[^a]"),
		classNegate(bclass(br('a', 'a'))),
	)
	expectHir(t,
		tr(t, `[^\d]`),
		hirNegate(hirUclassQuery(qBinary("digit"))),
	)
	expectHir(t,
		tr(t, `[^\pZ]`),
		hirNegate(hirUclassQuery(qBinary("separator"))),
	)
	expectHir(t,
		tr(t, `[^\p{separator}]`),
		hirNegate(hirUclassQuery(qBinary("separator"))),
	)
	expectHir(t,
		tr(t, `(?i)[^\p{greek}]`),
		hirNegate(hirCaseFold(hirUclassQuery(qBinary(
			"greek",
		)))),
	)
	expectHir(t,
		tr(t, `(?i)[\P{greek}]`),
		hirNegate(hirCaseFold(hirUclassQuery(qBinary(
			"greek",
		)))),
	)
	// Test some weird cases.
	expectHir(t, tr(t, `[\[]`), hirUclass(ur('[', '[')))
	expectHir(t, tr(t, `[&]`), hirUclass(ur('&', '&')))
	expectHir(t, tr(t, `[\&]`), hirUclass(ur('&', '&')))
	expectHir(t, tr(t, `[\&\&]`), hirUclass(ur('&', '&')))
	expectHir(t, tr(t, `[\x00-&]`), hirUclass(ur('\x00', '&')))
	expectHir(t, tr(t, `[&-\xFF]`), hirUclass(ur('&', '\u00FF')))
	expectHir(t, tr(t, `[~]`), hirUclass(ur('~', '~')))
	expectHir(t, tr(t, `[\~]`), hirUclass(ur('~', '~')))
	expectHir(t, tr(t, `[\~\~]`), hirUclass(ur('~', '~')))
	expectHir(t, tr(t, `[\x00-~]`), hirUclass(ur('\x00', '~')))
	expectHir(t, tr(t, `[~-\xFF]`), hirUclass(ur('~', '\u00FF')))
	expectHir(t, tr(t, `[-]`), hirUclass(ur('-', '-')))
	expectHir(t, tr(t, `[\-]`), hirUclass(ur('-', '-')))
	expectHir(t, tr(t, `[\-\-]`), hirUclass(ur('-', '-')))
	expectHir(t, tr(t, `[\x00-\-]`), hirUclass(ur('\x00', '-')))
	expectHir(t, tr(t, `[\--\xFF]`), hirUclass(ur('-', '\u00FF')))
	expectErr(t, trErr(t, "(?-u)[^a]"),
		testError{kind: InvalidUTF8, span: sp(5, 1, 6, 9, 1, 10)},
	)
	expectHir(t, tr(t, `[^\s\S]`), hirUclass())
	expectHir(t, trBytes(t, `(?-u)[^\s\S]`), hirBclass())
}

// TestClassBracketedUnion is class_bracketed_union in translate.rs.
func TestClassBracketedUnion(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "[a-zA-Z]"), hirUclass(ur('A', 'Z'), ur('a', 'z')))
	expectHir(t,
		tr(t, `[a\pZb]`),
		hirUnion(
			hirUclass(ur('a', 'b')),
			hirUclassQuery(qBinary("separator")),
		),
	)
	expectHir(t,
		tr(t, `[\pZ\p{Greek}]`),
		hirUnion(
			hirUclassQuery(qBinary("greek")),
			hirUclassQuery(qBinary("separator")),
		),
	)
	expectHir(t,
		tr(t, `[\p{age:3.0}\pZ\p{Greek}]`),
		hirUnion(
			hirUclassQuery(qByValue("age", "3.0")),
			hirUnion(
				hirUclassQuery(qBinary("greek")),
				hirUclassQuery(qBinary("separator")),
			),
		),
	)
	expectHir(t,
		tr(t, `[[[\p{age:3.0}\pZ]\p{Greek}][\p{Cyrillic}]]`),
		hirUnion(
			hirUclassQuery(qByValue("age", "3.0")),
			hirUnion(
				hirUclassQuery(qBinary("cyrillic")),
				hirUnion(
					hirUclassQuery(qBinary("greek")),
					hirUclassQuery(qBinary("separator")),
				),
			),
		),
	)
	expectHir(t,
		tr(t, `(?i)[\p{age:3.0}\pZ\p{Greek}]`),
		hirCaseFold(hirUnion(
			hirUclassQuery(qByValue("age", "3.0")),
			hirUnion(
				hirUclassQuery(qBinary("greek")),
				hirUclassQuery(qBinary("separator")),
			),
		)),
	)
	expectHir(t,
		tr(t, `[^\p{age:3.0}\pZ\p{Greek}]`),
		hirNegate(hirUnion(
			hirUclassQuery(qByValue("age", "3.0")),
			hirUnion(
				hirUclassQuery(qBinary("greek")),
				hirUclassQuery(qBinary("separator")),
			),
		)),
	)
	expectHir(t,
		tr(t, `(?i)[^\p{age:3.0}\pZ\p{Greek}]`),
		hirNegate(hirCaseFold(hirUnion(
			hirUclassQuery(qByValue("age", "3.0")),
			hirUnion(
				hirUclassQuery(qBinary("greek")),
				hirUclassQuery(qBinary("separator")),
			),
		))),
	)
}

// TestClassBracketedNested is class_bracketed_nested in translate.rs.
func TestClassBracketedNested(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `[a[^c]]`), classNegate(uclass(ur('c', 'c'))))
	expectHir(t, tr(t, `[a-b[^c]]`), classNegate(uclass(ur('c', 'c'))))
	expectHir(t, tr(t, `[a-c[^c]]`), classNegate(uclass()))
	expectHir(t, tr(t, `[^a[^c]]`), hirUclass(ur('c', 'c')))
	expectHir(t, tr(t, `[^a-b[^c]]`), hirUclass(ur('c', 'c')))
	expectHir(t,
		tr(t, `(?i)[a[^c]]`),
		hirNegate(classCaseFold(uclass(ur('c', 'c')))),
	)
	expectHir(t,
		tr(t, `(?i)[a-b[^c]]`),
		hirNegate(classCaseFold(uclass(ur('c', 'c')))),
	)
	expectHir(t, tr(t, `(?i)[^a[^c]]`), hirUclass(ur('C', 'C'), ur('c', 'c')))
	expectHir(t,
		tr(t, `(?i)[^a-b[^c]]`),
		hirUclass(ur('C', 'C'), ur('c', 'c')),
	)
	expectHir(t, tr(t, `[^a-c[^c]]`), hirUclass())
	expectHir(t, tr(t, `(?i)[^a-c[^c]]`), hirUclass())
}

// TestClassBracketedIntersect is class_bracketed_intersect in translate.rs.
func TestClassBracketedIntersect(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, "[abc&&b-c]"), hirUclass(ur('b', 'c')))
	expectHir(t, tr(t, "[abc&&[b-c]]"), hirUclass(ur('b', 'c')))
	expectHir(t, tr(t, "[[abc]&&[b-c]]"), hirUclass(ur('b', 'c')))
	expectHir(t, tr(t, "[a-z&&b-y&&c-x]"), hirUclass(ur('c', 'x')))
	expectHir(t, tr(t, "[c-da-b&&a-d]"), hirUclass(ur('a', 'd')))
	expectHir(t, tr(t, "[a-d&&c-da-b]"), hirUclass(ur('a', 'd')))
	expectHir(t, tr(t, `[a-z&&a-c]`), hirUclass(ur('a', 'c')))
	expectHir(t, tr(t, `[[a-z&&a-c]]`), hirUclass(ur('a', 'c')))
	expectHir(t, tr(t, `[^[a-z&&a-c]]`), hirNegate(hirUclass(ur('a', 'c'))))
	expectHir(t, tr(t, "(?-u)[abc&&b-c]"), hirBclass(br('b', 'c')))
	expectHir(t, tr(t, "(?-u)[abc&&[b-c]]"), hirBclass(br('b', 'c')))
	expectHir(t, tr(t, "(?-u)[[abc]&&[b-c]]"), hirBclass(br('b', 'c')))
	expectHir(t, tr(t, "(?-u)[a-z&&b-y&&c-x]"), hirBclass(br('c', 'x')))
	expectHir(t, tr(t, "(?-u)[c-da-b&&a-d]"), hirBclass(br('a', 'd')))
	expectHir(t, tr(t, "(?-u)[a-d&&c-da-b]"), hirBclass(br('a', 'd')))
	expectHir(t,
		tr(t, "(?i)[abc&&b-c]"),
		hirCaseFold(hirUclass(ur('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i)[abc&&[b-c]]"),
		hirCaseFold(hirUclass(ur('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i)[[abc]&&[b-c]]"),
		hirCaseFold(hirUclass(ur('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i)[a-z&&b-y&&c-x]"),
		hirCaseFold(hirUclass(ur('c', 'x'))),
	)
	expectHir(t,
		tr(t, "(?i)[c-da-b&&a-d]"),
		hirCaseFold(hirUclass(ur('a', 'd'))),
	)
	expectHir(t,
		tr(t, "(?i)[a-d&&c-da-b]"),
		hirCaseFold(hirUclass(ur('a', 'd'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[abc&&b-c]"),
		hirCaseFold(hirBclass(br('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[abc&&[b-c]]"),
		hirCaseFold(hirBclass(br('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[[abc]&&[b-c]]"),
		hirCaseFold(hirBclass(br('b', 'c'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[a-z&&b-y&&c-x]"),
		hirCaseFold(hirBclass(br('c', 'x'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[c-da-b&&a-d]"),
		hirCaseFold(hirBclass(br('a', 'd'))),
	)
	expectHir(t,
		tr(t, "(?i-u)[a-d&&c-da-b]"),
		hirCaseFold(hirBclass(br('a', 'd'))),
	)
	// In [a^], the ^ needs no escape, so the ^ needs no escape after &&
	// either.
	expectHir(t, tr(t, `[\^&&^]`), hirUclass(ur('^', '^')))
	// A ] needs an escape after &&, because it is not at the start of the
	// class.
	expectHir(t, tr(t, `[]&&\]]`), hirUclass(ur(']', ']')))
	expectHir(t, tr(t, `[-&&-]`), hirUclass(ur('-', '-')))
	expectHir(t, tr(t, `[\&&&&]`), hirUclass(ur('&', '&')))
	expectHir(t, tr(t, `[\&&&\&]`), hirUclass(ur('&', '&')))
	// Test precedence.
	expectHir(t,
		tr(t, `[a-w&&[^c-g]z]`),
		hirUclass(ur('a', 'b'), ur('h', 'w')),
	)
}

// TestClassBracketedIntersectNegate is class_bracketed_intersect_negate in translate.rs.
func TestClassBracketedIntersectNegate(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, `[^\w&&\d]`),
		hirNegate(hirUclassQuery(qBinary("digit"))),
	)
	expectHir(t, tr(t, `[^[a-z&&a-c]]`), hirNegate(hirUclass(ur('a', 'c'))))
	expectHir(t,
		tr(t, `[^[\w&&\d]]`),
		hirNegate(hirUclassQuery(qBinary("digit"))),
	)
	expectHir(t,
		tr(t, `[^[^\w&&\d]]`),
		hirUclassQuery(qBinary("digit")),
	)
	expectHir(t, tr(t, `[[[^\w]&&[^\d]]]`), hirNegate(hirUclassPerlWord()))
	expectHir(t,
		trBytes(t, `(?-u)[^\w&&\d]`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIDigit)),
	)
	expectHir(t,
		trBytes(t, `(?-u)[^[a-z&&a-c]]`),
		hirNegate(hirBclass(br('a', 'c'))),
	)
	expectHir(t,
		trBytes(t, `(?-u)[^[\w&&\d]]`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIDigit)),
	)
	expectHir(t,
		trBytes(t, `(?-u)[^[^\w&&\d]]`),
		hirASCIIBclass(ast.ClassASCIIDigit),
	)
	expectHir(t,
		trBytes(t, `(?-u)[[[^\w]&&[^\d]]]`),
		hirNegate(hirASCIIBclass(ast.ClassASCIIWord)),
	)
}

// TestClassBracketedDifference is class_bracketed_difference in translate.rs.
func TestClassBracketedDifference(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, `[\pL--[:ascii:]]`),
		hirDifference(
			hirUclassQuery(qBinary("letter")),
			hirUclass(ur('\x00', '\x7F')),
		),
	)
	expectHir(t,
		tr(t, `(?-u)[[:alpha:]--[:lower:]]`),
		hirBclass(br('A', 'Z')),
	)
}

// TestClassBracketedSymmetricDifference is class_bracketed_symmetric_difference in translate.rs.
func TestClassBracketedSymmetricDifference(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, `[\p{sc:Greek}~~\p{scx:Greek}]`),
		// Class({
		//     '·'..='·',
		//     '\u0300'..='\u0301',
		//     '\u0304'..='\u0304',
		//     '\u0306'..='\u0306',
		//     '\u0308'..='\u0308',
		//     '\u0313'..='\u0313',
		//     '\u0342'..='\u0342',
		//     '\u0345'..='\u0345',
		//     'ʹ'..='ʹ',
		//     '\u1DC0'..='\u1DC1',
		//     '⁝'..='⁝',
		// })
		hirUclass(
			ur('·', '·'),
			ur('\u0300', '\u0301'),
			ur('\u0304', '\u0304'),
			ur('\u0306', '\u0306'),
			ur('\u0308', '\u0308'),
			ur('\u0313', '\u0313'),
			ur('\u0342', '\u0342'),
			ur('\u0345', '\u0345'),
			ur('ʹ', 'ʹ'),
			ur('\u1DC0', '\u1DC1'),
			ur('⁝', '⁝')),
	)
	expectHir(t, tr(t, `[a-g~~c-j]`), hirUclass(ur('a', 'b'), ur('h', 'j')))
	expectHir(t,
		tr(t, `(?-u)[a-g~~c-j]`),
		hirBclass(br('a', 'b'), br('h', 'j')),
	)
}

// TestIgnoreWhitespace is ignore_whitespace in translate.rs.
func TestIgnoreWhitespace(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `(?x)\12 3`), hirLit("\n3"))
	expectHir(t, tr(t, `(?x)\x { 53 }`), hirLit("S"))
	expectHir(t,
		tr(t, `(?x)\x # comment
{ # comment
    53 # comment
} #comment`),
		hirLit("S"),
	)
	expectHir(t, tr(t, `(?x)\x 53`), hirLit("S"))
	expectHir(t,
		tr(t, `(?x)\x # comment
        53 # comment`),
		hirLit("S"),
	)
	expectHir(t, tr(t, `(?x)\x5 3`), hirLit("S"))
	expectHir(t,
		tr(t, `(?x)\p # comment
{ # comment
    Separator # comment
} # comment`),
		hirUclassQuery(qBinary("separator")),
	)
	expectHir(t,
		tr(t, `(?x)a # comment
{ # comment
    5 # comment
    , # comment
    10 # comment
} # comment`),
		hirRange(true, 5, new(uint32(10)), hirLit("a")),
	)
	expectHir(t, tr(t, `(?x)a\  # hi there`), hirLit("a "))
}

// TestAnalysisIsUTF8 is analysis_is_utf8 in translate.rs.
func TestAnalysisIsUTF8(t *testing.T) {
	t.Parallel()

	// Positive examples.
	assert(t, propsBytes(t, `a`).IsUTF8())
	assert(t, propsBytes(t, `ab`).IsUTF8())
	assert(t, propsBytes(t, `(?-u)a`).IsUTF8())
	assert(t, propsBytes(t, `(?-u)ab`).IsUTF8())
	assert(t, propsBytes(t, `\xFF`).IsUTF8())
	assert(t, propsBytes(t, `\xFF\xFF`).IsUTF8())
	assert(t, propsBytes(t, `[^a]`).IsUTF8())
	assert(t, propsBytes(t, `[^a][^a]`).IsUTF8())
	assert(t, propsBytes(t, `\b`).IsUTF8())
	assert(t, propsBytes(t, `\B`).IsUTF8())
	assert(t, propsBytes(t, `(?-u)\b`).IsUTF8())
	assert(t, propsBytes(t, `(?-u)\B`).IsUTF8())
	// Negative examples.
	assert(t, !propsBytes(t, `(?-u)\xFF`).IsUTF8())
	assert(t, !propsBytes(t, `(?-u)\xFF\xFF`).IsUTF8())
	assert(t, !propsBytes(t, `(?-u)[^a]`).IsUTF8())
	assert(t, !propsBytes(t, `(?-u)[^a][^a]`).IsUTF8())
}

// TestAnalysisCapturesLen is analysis_captures_len in translate.rs.
func TestAnalysisCapturesLen(t *testing.T) {
	t.Parallel()

	expectEq(t, 0, props(t, `a`).ExplicitCapturesLen())
	expectEq(t, 0, props(t, `(?:a)`).ExplicitCapturesLen())
	expectEq(t, 0, props(t, `(?i-u:a)`).ExplicitCapturesLen())
	expectEq(t, 0, props(t, `(?i-u)a`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `(a)`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `(?P<foo>a)`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `()`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `()a`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `(a)+`).ExplicitCapturesLen())
	expectEq(t, 2, props(t, `(a)(b)`).ExplicitCapturesLen())
	expectEq(t, 2, props(t, `(a)|(b)`).ExplicitCapturesLen())
	expectEq(t, 2, props(t, `((a))`).ExplicitCapturesLen())
	expectEq(t, 1, props(t, `([a&&b])`).ExplicitCapturesLen())
}

// TestAnalysisStaticCapturesLen is analysis_static_captures_len in translate.rs.
func TestAnalysisStaticCapturesLen(t *testing.T) {
	t.Parallel()

	staticLen := func(pattern string) opt {
		return optOf(props(t, pattern).StaticExplicitCapturesLen())
	}
	expectEq(t, some(0), staticLen(``))
	expectEq(t, some(0), staticLen(`foo|bar`))
	expectEq(t, opt{}, staticLen(`(foo)|bar`))
	expectEq(t, opt{}, staticLen(`foo|(bar)`))
	expectEq(t, some(1), staticLen(`(foo|bar)`))
	expectEq(t, some(1), staticLen(`(a|b|c|d|e|f)`))
	expectEq(t, some(1), staticLen(`(a)|(b)|(c)|(d)|(e)|(f)`))
	expectEq(t, some(2), staticLen(`(a)(b)|(c)(d)|(e)(f)`))
	expectEq(t, some(6), staticLen(`(a)(b)(c)(d)(e)(f)`))
	expectEq(t, some(3), staticLen(`(a)(b)(extra)|(a)(b)()`))
	expectEq(t, some(3), staticLen(`(a)(b)((?:extra)?)`))
	expectEq(t, opt{}, staticLen(`(a)(b)(extra)?`))
	expectEq(t, some(1), staticLen(`(foo)|(bar)`))
	expectEq(t, some(2), staticLen(`(foo)(bar)`))
	expectEq(t, some(2), staticLen(`(foo)+(bar)`))
	expectEq(t, opt{}, staticLen(`(foo)*(bar)`))
	expectEq(t, some(0), staticLen(`(foo)?{0}`))
	expectEq(t, opt{}, staticLen(`(foo)?{1}`))
	expectEq(t, some(1), staticLen(`(foo){1}`))
	expectEq(t, some(1), staticLen(`(foo){1,}`))
	expectEq(t, some(1), staticLen(`(foo){1,}?`))
	expectEq(t, opt{}, staticLen(`(foo){1,}??`))
	expectEq(t, opt{}, staticLen(`(foo){0,}`))
	expectEq(t, some(1), staticLen(`(foo)(?:bar)`))
	expectEq(t, some(2), staticLen(`(foo(?:bar)+)(?:baz(boo))`))
	expectEq(t, some(2), staticLen(`(?P<bar>foo)(?:bar)(bal|loon)`))
	expectEq(t,
		some(2),
		staticLen(`<(a)[^>]+href="([^"]+)"|<(img)[^>]+src="([^"]+)"`),
	)
}

// TestAnalysisIsAllAssertions is analysis_is_all_assertions in translate.rs.
func TestAnalysisIsAllAssertions(t *testing.T) {
	t.Parallel()

	// Positive examples.
	p := props(t, `\b`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `\B`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `^`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `$`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `\A`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `\z`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `$^\z\A\b\B`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `$|^|\z|\A|\b|\B`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `^$|$^`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	p = props(t, `((\b)+())*^`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(0))
	// Negative examples.
	p = props(t, `^a`)
	assert(t, !p.LookSet().IsEmpty())
	expectEq(t, optOf(p.MinimumLen()), some(1))
}

// TestAnalysisLookSetPrefixAny is analysis_look_set_prefix_any in translate.rs.
func TestAnalysisLookSetPrefixAny(t *testing.T) {
	t.Parallel()

	p := props(t, `(?-u)(?i:(?:\b|_)win(?:32|64|dows)?(?:\b|_))`)
	assert(t, p.LookSetPrefixAny().Contains(LookWordASCII))
}

// TestAnalysisIsAnchored is analysis_is_anchored in translate.rs.
func TestAnalysisIsAnchored(t *testing.T) {
	t.Parallel()

	isStart := func(p string) bool { return props(t, p).LookSetPrefix().Contains(LookStart) }
	isEnd := func(p string) bool { return props(t, p).LookSetSuffix().Contains(LookEnd) }
	// Positive examples.
	assert(t, isStart(`^`))
	assert(t, isEnd(`$`))
	assert(t, isStart(`^^`))
	assert(t, props(t, `$$`).LookSetSuffix().Contains(LookEnd))
	assert(t, isStart(`^$`))
	assert(t, isEnd(`^$`))
	assert(t, isStart(`^foo`))
	assert(t, isEnd(`foo$`))
	assert(t, isStart(`^foo|^bar`))
	assert(t, isEnd(`foo$|bar$`))
	assert(t, isStart(`^(foo|bar)`))
	assert(t, isEnd(`(foo|bar)$`))
	assert(t, isStart(`^+`))
	assert(t, isEnd(`$+`))
	assert(t, isStart(`^++`))
	assert(t, isEnd(`$++`))
	assert(t, isStart(`(^)+`))
	assert(t, isEnd(`($)+`))
	assert(t, isStart(`$^`))
	assert(t, isStart(`$^`))
	assert(t, isStart(`$^|^$`))
	assert(t, isEnd(`$^|^$`))
	assert(t, isStart(`\b^`))
	assert(t, isEnd(`$\b`))
	assert(t, isStart(`^(?m:^)`))
	assert(t, isEnd(`(?m:$)$`))
	assert(t, isStart(`(?m:^)^`))
	assert(t, isEnd(`$(?m:$)`))
	// Negative examples.
	assert(t, !isStart(`(?m)^`))
	assert(t, !isEnd(`(?m)$`))
	assert(t, !isStart(`(?m:^$)|$^`))
	assert(t, !isEnd(`(?m:^$)|$^`))
	assert(t, !isStart(`$^|(?m:^$)`))
	assert(t, !isEnd(`$^|(?m:^$)`))
	assert(t, !isStart(`a^`))
	assert(t, !isStart(`$a`))
	assert(t, !isEnd(`a^`))
	assert(t, !isEnd(`$a`))
	assert(t, !isStart(`^foo|bar`))
	assert(t, !isEnd(`foo|bar$`))
	assert(t, !isStart(`^*`))
	assert(t, !isEnd(`$*`))
	assert(t, !isStart(`^*+`))
	assert(t, !isEnd(`$*+`))
	assert(t, !isStart(`^+*`))
	assert(t, !isEnd(`$+*`))
	assert(t, !isStart(`(^)*`))
	assert(t, !isEnd(`($)*`))
}

// TestAnalysisIsAnyAnchored is analysis_is_any_anchored in translate.rs.
func TestAnalysisIsAnyAnchored(t *testing.T) {
	t.Parallel()

	isStart := func(p string) bool { return props(t, p).LookSet().Contains(LookStart) }
	isEnd := func(p string) bool { return props(t, p).LookSet().Contains(LookEnd) }
	// Positive examples.
	assert(t, isStart(`^`))
	assert(t, isEnd(`$`))
	assert(t, isStart(`\A`))
	assert(t, isEnd(`\z`))
	// Negative examples.
	assert(t, !isStart(`(?m)^`))
	assert(t, !isEnd(`(?m)$`))
	assert(t, !isStart(`$`))
	assert(t, !isEnd(`^`))
}

// TestAnalysisCanEmpty is analysis_can_empty in translate.rs.
func TestAnalysisCanEmpty(t *testing.T) {
	t.Parallel()

	// Positive examples.
	assertEmpty := func(p string) {
		t.Helper()
		expectEq(t, some(0), optOf(propsBytes(t, p).MinimumLen()))
	}
	assertEmpty(``)
	assertEmpty(`()`)
	assertEmpty(`()*`)
	assertEmpty(`()+`)
	assertEmpty(`()?`)
	assertEmpty(`a*`)
	assertEmpty(`a?`)
	assertEmpty(`a{0}`)
	assertEmpty(`a{0,}`)
	assertEmpty(`a{0,1}`)
	assertEmpty(`a{0,10}`)
	assertEmpty(`\pL*`)
	assertEmpty(`a*|b`)
	assertEmpty(`b|a*`)
	assertEmpty(`a|`)
	assertEmpty(`|a`)
	assertEmpty(`a||b`)
	assertEmpty(`a*a?(abcd)*`)
	assertEmpty(`^`)
	assertEmpty(`$`)
	assertEmpty(`(?m)^`)
	assertEmpty(`(?m)$`)
	assertEmpty(`\A`)
	assertEmpty(`\z`)
	assertEmpty(`\B`)
	assertEmpty(`(?-u)\B`)
	assertEmpty(`\b`)
	assertEmpty(`(?-u)\b`)
	// Negative examples.
	assertNonEmpty := func(p string) {
		t.Helper()
		if got := optOf(propsBytes(t, p).MinimumLen()); got == some(0) {
			t.Errorf("minimum length of %q is 0", p)
		}
	}
	assertNonEmpty(`a+`)
	assertNonEmpty(`a{1}`)
	assertNonEmpty(`a{1,}`)
	assertNonEmpty(`a{1,2}`)
	assertNonEmpty(`a{1,10}`)
	assertNonEmpty(`b|a`)
	assertNonEmpty(`a*a+(abcd)*`)
	assertNonEmpty(`\P{any}`)
	assertNonEmpty(`[a--a]`)
	assertNonEmpty(`[a&&b]`)
}

// TestAnalysisIsLiteral is analysis_is_literal in translate.rs.
func TestAnalysisIsLiteral(t *testing.T) {
	t.Parallel()

	// Positive examples.
	assert(t, props(t, `a`).IsLiteral())
	assert(t, props(t, `ab`).IsLiteral())
	assert(t, props(t, `abc`).IsLiteral())
	assert(t, props(t, `(?m)abc`).IsLiteral())
	assert(t, props(t, `(?:a)`).IsLiteral())
	assert(t, props(t, `foo(?:a)`).IsLiteral())
	assert(t, props(t, `(?:a)foo`).IsLiteral())
	assert(t, props(t, `[a]`).IsLiteral())
	// Negative examples.
	assert(t, !props(t, ``).IsLiteral())
	assert(t, !props(t, `^`).IsLiteral())
	assert(t, !props(t, `a|b`).IsLiteral())
	assert(t, !props(t, `(a)`).IsLiteral())
	assert(t, !props(t, `a+`).IsLiteral())
	assert(t, !props(t, `foo(a)`).IsLiteral())
	assert(t, !props(t, `(a)foo`).IsLiteral())
	assert(t, !props(t, `[ab]`).IsLiteral())
}

// TestAnalysisIsAlternationLiteral is analysis_is_alternation_literal in translate.rs.
func TestAnalysisIsAlternationLiteral(t *testing.T) {
	t.Parallel()

	// Positive examples.
	assert(t, props(t, `a`).IsAlternationLiteral())
	assert(t, props(t, `ab`).IsAlternationLiteral())
	assert(t, props(t, `abc`).IsAlternationLiteral())
	assert(t, props(t, `(?m)abc`).IsAlternationLiteral())
	assert(t, props(t, `foo|bar`).IsAlternationLiteral())
	assert(t, props(t, `foo|bar|baz`).IsAlternationLiteral())
	assert(t, props(t, `[a]`).IsAlternationLiteral())
	assert(t, props(t, `(?:ab)|cd`).IsAlternationLiteral())
	assert(t, props(t, `ab|(?:cd)`).IsAlternationLiteral())
	// Negative examples.
	assert(t, !props(t, ``).IsAlternationLiteral())
	assert(t, !props(t, `^`).IsAlternationLiteral())
	assert(t, !props(t, `(a)`).IsAlternationLiteral())
	assert(t, !props(t, `a+`).IsAlternationLiteral())
	assert(t, !props(t, `foo(a)`).IsAlternationLiteral())
	assert(t, !props(t, `(a)foo`).IsAlternationLiteral())
	assert(t, !props(t, `[ab]`).IsAlternationLiteral())
	assert(t, !props(t, `[ab]|b`).IsAlternationLiteral())
	assert(t, !props(t, `a|[ab]`).IsAlternationLiteral())
	assert(t, !props(t, `(a)|b`).IsAlternationLiteral())
	assert(t, !props(t, `a|(b)`).IsAlternationLiteral())
	assert(t, !props(t, `a|b`).IsAlternationLiteral())
	assert(t, !props(t, `a|b|c`).IsAlternationLiteral())
	assert(t, !props(t, `[a]|b`).IsAlternationLiteral())
	assert(t, !props(t, `a|[b]`).IsAlternationLiteral())
	assert(t, !props(t, `(?:a)|b`).IsAlternationLiteral())
	assert(t, !props(t, `a|(?:b)`).IsAlternationLiteral())
	assert(t, !props(t, `(?:z|xx)@|xx`).IsAlternationLiteral())
}

// TestSmartRepetition is smart_repetition in translate.rs.
func TestSmartRepetition(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, `a{0}`), NewEmpty())
	expectHir(t, tr(t, `a{1}`), hirLit("a"))
	expectHir(t, tr(t, `\B{32111}`), hirLook(LookWordUnicodeNegate))
}

// TestSmartConcat is smart_concat in translate.rs.
func TestSmartConcat(t *testing.T) {
	t.Parallel()

	expectHir(t, tr(t, ""), NewEmpty())
	expectHir(t, tr(t, "(?:)"), NewEmpty())
	expectHir(t, tr(t, "abc"), hirLit("abc"))
	expectHir(t, tr(t, "(?:foo)(?:bar)"), hirLit("foobar"))
	expectHir(t, tr(t, "quux(?:foo)(?:bar)baz"), hirLit("quuxfoobarbaz"))
	expectHir(t,
		tr(t, "foo(?:bar^baz)quux"),
		hirCat(
			hirLit("foobar"),
			hirLook(LookStart),
			hirLit("bazquux")),
	)
	expectHir(t,
		tr(t, "foo(?:ba(?:r^b)az)quux"),
		hirCat(
			hirLit("foobar"),
			hirLook(LookStart),
			hirLit("bazquux")),
	)
}

// TestSmartAlternation is smart_alternation in translate.rs.
func TestSmartAlternation(t *testing.T) {
	t.Parallel()

	expectHir(t,
		tr(t, "(?:foo)|(?:bar)"),
		hirAlt(hirLit("foo"), hirLit("bar")),
	)
	expectHir(t,
		tr(t, "quux|(?:abc|def|xyz)|baz"),
		hirAlt(
			hirLit("quux"),
			hirLit("abc"),
			hirLit("def"),
			hirLit("xyz"),
			hirLit("baz")),
	)
	expectHir(t,
		tr(t, "quux|(?:abc|(?:def|mno)|xyz)|baz"),
		hirAlt(
			hirLit("quux"),
			hirLit("abc"),
			hirLit("def"),
			hirLit("mno"),
			hirLit("xyz"),
			hirLit("baz")),
	)
	expectHir(t,
		tr(t, "a|b|c|d|e|f|x|y|z"),
		hirUclass(ur('a', 'f'), ur('x', 'z')),
	)
	// Tests that we lift common prefixes out of an alternation.
	expectHir(t,
		tr(t, "[A-Z]foo|[A-Z]quux"),
		hirCat(
			hirUclass(ur('A', 'Z')),
			hirAlt(hirLit("foo"), hirLit("quux"))),
	)
	expectHir(t,
		tr(t, "[A-Z][A-Z]|[A-Z]quux"),
		hirCat(
			hirUclass(ur('A', 'Z')),
			hirAlt(hirUclass(ur('A', 'Z')), hirLit("quux"))),
	)
	expectHir(t,
		tr(t, "[A-Z][A-Z]|[A-Z][A-Z]quux"),
		hirCat(
			hirUclass(ur('A', 'Z')),
			hirUclass(ur('A', 'Z')),
			hirAlt(NewEmpty(), hirLit("quux"))),
	)
	expectHir(t,
		tr(t, "[A-Z]foo|[A-Z]foobar"),
		hirCat(
			hirUclass(ur('A', 'Z')),
			hirAlt(hirLit("foo"), hirLit("foobar"))),
	)
}

// TestRegressionAltEmptyConcat is regression_alt_empty_concat in
// translate.rs.
func TestRegressionAltEmptyConcat(t *testing.T) {
	t.Parallel()
	span := ast.SplatSpan(ast.NewPosition(0, 0, 0))
	a := &ast.Alternation{
		Span: span,
		Asts: []ast.Ast{&ast.Concat{Span: span, Asts: nil}},
	}

	tl := NewTranslator()
	got, err := tl.Translate("", a)
	if err != nil {
		t.Fatal(err)
	}
	expectHir(t, got, NewEmpty())
}

// TestRegressionEmptyAlt is regression_empty_alt in translate.rs.
func TestRegressionEmptyAlt(t *testing.T) {
	t.Parallel()
	span := ast.SplatSpan(ast.NewPosition(0, 0, 0))
	a := &ast.Concat{
		Span: span,
		Asts: []ast.Ast{&ast.Alternation{
			Span: span,
			Asts: nil,
		}},
	}

	tl := NewTranslator()
	got, err := tl.Translate("", a)
	if err != nil {
		t.Fatal(err)
	}
	expectHir(t, got, NewFail())
}

// TestRegressionSingletonAlt is regression_singleton_alt in translate.rs.
func TestRegressionSingletonAlt(t *testing.T) {
	t.Parallel()
	span := ast.SplatSpan(ast.NewPosition(0, 0, 0))
	a := &ast.Concat{
		Span: span,
		Asts: []ast.Ast{&ast.Alternation{
			Span: span,
			Asts: []ast.Ast{&ast.Dot{Span: span}},
		}},
	}

	tl := NewTranslator()
	got, err := tl.Translate("", a)
	if err != nil {
		t.Fatal(err)
	}
	expectHir(t, got, NewDot(Dot{Kind: DotAnyCharExceptLF}))
}

// TestRegressionFuzzMatch is regression_fuzz_match in translate.rs. See
// https://bugs.chromium.org/p/oss-fuzz/issues/detail?id=63168.
func TestRegressionFuzzMatch(t *testing.T) {
	t.Parallel()
	pat := "[(\u0006 \x00-\U000AFDF5]  \x00 "
	a, err := ast.NewParserBuilder().
		Octal(false).
		IgnoreWhitespace(true).
		Build().
		Parse(pat)
	if err != nil {
		t.Fatal(err)
	}
	hir, err := NewTranslatorBuilder().
		UTF8(true).
		CaseInsensitive(false).
		MultiLine(false).
		DotMatchesNewLine(false).
		SwapGreed(true).
		Unicode(true).
		Build().
		Translate(pat, a)
	if err != nil {
		t.Fatal(err)
	}
	expectHir(t,
		hir,
		NewConcat([]*Hir{
			hirUclass(ur('\x00', '\U000AFDF5')),
			hirLit("\x00"),
		}),
	)
}

// TestRegressionFuzzDifference1 is regression_fuzz_difference1 in
// translate.rs. See
// https://bugs.chromium.org/p/oss-fuzz/issues/detail?id=63155.
func TestRegressionFuzzDifference1(t *testing.T) {
	t.Parallel()
	pat := `\W\W|\W[^\v--\W\W\P{Script_Extensions:Pau_Cin_Hau}\u10A1A1-\U{3E3E3}--~~~~--~~~~~~~~------~~~~~~--~~~~~~]*`
	_ = tr(t, pat) // This must not panic.
}

// TestRegressionFuzzCharDecrement1 is regression_fuzz_char_decrement1 in
// translate.rs. See
// https://bugs.chromium.org/p/oss-fuzz/issues/detail?id=63153.
func TestRegressionFuzzCharDecrement1(t *testing.T) {
	t.Parallel()

	pat := "w[w[^w?\rw\rw[^w?\rw[^w?\rw[^w?\rw[^w?\rw[^w?\rw[^w?\r\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00w?\rw[^w?\rw[^w?\rw[^w\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\u0001\x00]\x00\x00\x00\x00\x00\x00\x00\x00\x00*\x00\x00\u0001\x00]\x00\x00-*\x00][^w?\rw[^w?\rw[^w?\rw[^w?\rw[^w?\rw[^w?\rw[^w\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\u0001\x00]\x00\x00\x00\x00\x00\x00\x00\x00\x00x\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\x00\x00\x00\x00*??\x00\u007F{2}\u0010??\x00\x00\x00\x00\x00\x00\x00\x00\x00\u0003\x00\x00\x00}\x00-*\x00]\x00\x00\x00\x00\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\u0001\x00]\x00\x00-*\x00]\x00\x00\x00\x00\x00\x00\x00\u0001\x00]\x00\u0001\u0001H-i]-]\x00\x00\x00\x00\u0001\x00]\x00\x00\x00\u0001\x00]\x00\x00-*\x00\x00\x00\x00\u00019-\u007F]\x00'|-\u007F]\x00'|(?i-ux)[-\u007F]\x00'\u0003\x00\x00\x00}\x00-*\x00]<D\x00\x00\x00\x00\x00\x00\u0001]\x00\x00\x00\x00]\x00\x00-*\x00]\x00\x00 "
	_ = tr(t, pat) // This must not panic.
}
