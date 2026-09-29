package hir

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// This file ports the tests of src/hir/mod.rs, with their helpers. The tests
// of interval.rs are here too, because upstream tests interval sets through
// the exported API of the HIR, and interval.rs has no tests of its own.
//
// The macro assert_eq of upstream becomes assertEq, which compares two
// values of the same type. A pair of chars or bytes of upstream, such as
// ('a', 'c'), is a [2]rune or a [2]byte, which ur and br build.
//
// The tests class_case_fold_unicode_disabled and
// class_case_fold_unicode_disabled_panics are not here. They run only when
// the feature unicode-case is off, and the port follows the default
// features, which turn it on.

// ur returns a pair of characters, the start and the end of a range.
func ur(start, end rune) [2]rune {
	return [2]rune{start, end}
}

// br returns a pair of bytes, the start and the end of a range.
func br(start, end byte) [2]byte {
	return [2]byte{start, end}
}

// uclass returns a Unicode class of the ranges.
//
// uclass is uclass in mod.rs and in translate.rs. The second one returns a
// hir::Class, which a *ClassUnicode is.
func uclass(ranges ...[2]rune) *ClassUnicode {
	rs := make([]ClassUnicodeRange, 0, len(ranges))
	for _, r := range ranges {
		rs = append(rs, NewClassUnicodeRange(r[0], r[1]))
	}
	return NewClassUnicode(rs)
}

// bclass returns a class of bytes of the ranges.
//
// bclass is bclass in mod.rs and in translate.rs. The second one returns a
// hir::Class, which a *ClassBytes is.
func bclass(ranges ...[2]byte) *ClassBytes {
	rs := make([]ClassBytesRange, 0, len(ranges))
	for _, r := range ranges {
		rs = append(rs, NewClassBytesRange(r[0], r[1]))
	}
	return NewClassBytes(rs)
}

// uranges returns the ranges of a Unicode class as pairs.
func uranges(cls *ClassUnicode) [][2]rune {
	var rs [][2]rune
	for r := range cls.Iter() {
		rs = append(rs, ur(r.Start(), r.End()))
	}
	return rs
}

// ucasefold returns a copy of the class, case folded.
func ucasefold(cls *ClassUnicode) *ClassUnicode {
	c := cls.Clone()
	c.CaseFoldSimple()
	return c
}

// uunion returns the union of two classes.
func uunion(cls1, cls2 *ClassUnicode) *ClassUnicode {
	c := cls1.Clone()
	c.Union(cls2)
	return c
}

// uintersect returns the intersection of two classes.
func uintersect(cls1, cls2 *ClassUnicode) *ClassUnicode {
	c := cls1.Clone()
	c.Intersect(cls2)
	return c
}

// udifference returns the difference of two classes.
func udifference(cls1, cls2 *ClassUnicode) *ClassUnicode {
	c := cls1.Clone()
	c.Difference(cls2)
	return c
}

// usymdifference returns the symmetric difference of two classes.
func usymdifference(cls1, cls2 *ClassUnicode) *ClassUnicode {
	c := cls1.Clone()
	c.SymmetricDifference(cls2)
	return c
}

// unegate returns the negation of a class.
func unegate(cls *ClassUnicode) *ClassUnicode {
	c := cls.Clone()
	c.Negate()
	return c
}

// branges returns the ranges of a class of bytes as pairs.
func branges(cls *ClassBytes) [][2]byte {
	var rs [][2]byte
	for r := range cls.Iter() {
		rs = append(rs, br(r.Start(), r.End()))
	}
	return rs
}

// bcasefold returns a copy of the class, case folded.
func bcasefold(cls *ClassBytes) *ClassBytes {
	c := cls.Clone()
	c.CaseFoldSimple()
	return c
}

// bunion returns the union of two classes.
func bunion(cls1, cls2 *ClassBytes) *ClassBytes {
	c := cls1.Clone()
	c.Union(cls2)
	return c
}

// bintersect returns the intersection of two classes.
func bintersect(cls1, cls2 *ClassBytes) *ClassBytes {
	c := cls1.Clone()
	c.Intersect(cls2)
	return c
}

// bdifference returns the difference of two classes.
func bdifference(cls1, cls2 *ClassBytes) *ClassBytes {
	c := cls1.Clone()
	c.Difference(cls2)
	return c
}

// bsymdifference returns the symmetric difference of two classes.
func bsymdifference(cls1, cls2 *ClassBytes) *ClassBytes {
	c := cls1.Clone()
	c.SymmetricDifference(cls2)
	return c
}

// bnegate returns the negation of a class.
func bnegate(cls *ClassBytes) *ClassBytes {
	c := cls.Clone()
	c.Negate()
	return c
}

// assertEq fails the test if left and right are not equal. It compares an
// Hir, a class and a slice by their contents, and any other value with ==.
//
// assertEq is the macro assert_eq.
func assertEq[T any](t *testing.T, left, right T) {
	t.Helper()
	var ok bool
	switch l := any(left).(type) {
	case *Hir:
		r, isHir := any(right).(*Hir)
		ok = isHir && l.Equal(r)
	case *ClassUnicode:
		r, isClass := any(right).(*ClassUnicode)
		ok = isClass && l.Equal(r)
	case *ClassBytes:
		r, isClass := any(right).(*ClassBytes)
		ok = isClass && l.Equal(r)
	case [][2]rune:
		r, isSlice := any(right).([][2]rune)
		ok = isSlice && slices.Equal(l, r)
	case [][2]byte:
		r, isSlice := any(right).([][2]byte)
		ok = isSlice && slices.Equal(l, r)
	default:
		ok = any(left) == any(right)
	}
	if !ok {
		t.Errorf("assertion `left == right` failed\n  left: %s\n right: %s", dumpValue(left), dumpValue(right))
	}
}

// assert fails the test if ok is false.
//
// assert is the macro assert.
func assert(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Error("assertion failed")
	}
}

// dumpValue returns the text of a value for the message of a test.
func dumpValue(v any) string {
	switch x := v.(type) {
	case *Hir:
		return dumpHir(x)
	case *ClassUnicode:
		return fmt.Sprintf("%q", uranges(x))
	case *ClassBytes:
		return fmt.Sprintf("%q", branges(x))
	}
	return fmt.Sprintf("%#v", v)
}

// dumpHir returns the text of an expression, with its kinds, for the
// message of a test.
func dumpHir(h *Hir) string {
	switch k := h.Kind().(type) {
	case *Empty:
		return "Empty"
	case *Literal:
		return fmt.Sprintf("Literal(%q)", []byte(*k))
	case *ClassUnicode:
		return fmt.Sprintf("ClassUnicode(%q)", uranges(k))
	case *ClassBytes:
		return fmt.Sprintf("ClassBytes(%q)", branges(k))
	case *Look:
		return fmt.Sprintf("Look(%#x)", uint32(*k))
	case *Repetition:
		m := "None"
		if k.Max != nil {
			m = strconv.FormatUint(uint64(*k.Max), 10)
		}
		return fmt.Sprintf("Repetition{min: %d, max: %s, greedy: %t, sub: %s}", k.Min, m, k.Greedy, dumpHir(k.Sub))
	case *Capture:
		return fmt.Sprintf("Capture{index: %d, name: %q, sub: %s}", k.Index, k.Name, dumpHir(k.Sub))
	case *Concat:
		return "Concat[" + dumpHirs(*k) + "]"
	case *Alternation:
		return "Alternation[" + dumpHirs(*k) + "]"
	}
	return "?"
}

// dumpHirs returns the text of a list of expressions.
func dumpHirs(hs []*Hir) string {
	parts := make([]string, 0, len(hs))
	for _, h := range hs {
		parts = append(parts, dumpHir(h))
	}
	return strings.Join(parts, ", ")
}

// TestClassRangeCanonicalUnicode is class_range_canonical_unicode in mod.rs.
func TestClassRangeCanonicalUnicode(t *testing.T) {
	t.Parallel()
	r := NewClassUnicodeRange('\u00FF', '\x00')
	assertEq(t, '\x00', r.Start())
	assertEq(t, '\u00FF', r.End())
}

// TestClassRangeCanonicalBytes is class_range_canonical_bytes in mod.rs.
func TestClassRangeCanonicalBytes(t *testing.T) {
	t.Parallel()
	r := NewClassBytesRange('\xFF', '\x00')
	assertEq(t, '\x00', r.Start())
	assertEq(t, '\xFF', r.End())
}

// TestClassCanonicalizeUnicode is class_canonicalize_unicode in mod.rs.
func TestClassCanonicalizeUnicode(t *testing.T) {
	t.Parallel()

	cls := uclass(ur('a', 'c'), ur('x', 'z'))
	expected := [][2]rune{ur('a', 'c'), ur('x', 'z')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(ur('x', 'z'), ur('a', 'c'))
	expected = [][2]rune{ur('a', 'c'), ur('x', 'z')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(ur('x', 'z'), ur('w', 'y'))
	expected = [][2]rune{ur('w', 'z')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(
		ur('c', 'f'),
		ur('a', 'g'),
		ur('d', 'j'),
		ur('a', 'c'),
		ur('m', 'p'),
		ur('l', 's'))
	expected = [][2]rune{ur('a', 'j'), ur('l', 's')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(ur('x', 'z'), ur('u', 'w'))
	expected = [][2]rune{ur('u', 'z')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(ur('\x00', '\U0010FFFF'), ur('\x00', '\U0010FFFF'))
	expected = [][2]rune{ur('\x00', '\U0010FFFF')}
	assertEq(t, expected, uranges(cls))
	cls = uclass(ur('a', 'a'), ur('b', 'b'))
	expected = [][2]rune{ur('a', 'b')}
	assertEq(t, expected, uranges(cls))
}

// TestClassCanonicalizeBytes is class_canonicalize_bytes in mod.rs.
func TestClassCanonicalizeBytes(t *testing.T) {
	t.Parallel()

	cls := bclass(br('a', 'c'), br('x', 'z'))
	expected := [][2]byte{br('a', 'c'), br('x', 'z')}
	assertEq(t, expected, branges(cls))
	cls = bclass(br('x', 'z'), br('a', 'c'))
	expected = [][2]byte{br('a', 'c'), br('x', 'z')}
	assertEq(t, expected, branges(cls))
	cls = bclass(br('x', 'z'), br('w', 'y'))
	expected = [][2]byte{br('w', 'z')}
	assertEq(t, expected, branges(cls))
	cls = bclass(
		br('c', 'f'),
		br('a', 'g'),
		br('d', 'j'),
		br('a', 'c'),
		br('m', 'p'),
		br('l', 's'))
	expected = [][2]byte{br('a', 'j'), br('l', 's')}
	assertEq(t, expected, branges(cls))
	cls = bclass(br('x', 'z'), br('u', 'w'))
	expected = [][2]byte{br('u', 'z')}
	assertEq(t, expected, branges(cls))
	cls = bclass(br('\x00', '\xFF'), br('\x00', '\xFF'))
	expected = [][2]byte{br('\x00', '\xFF')}
	assertEq(t, expected, branges(cls))
	cls = bclass(br('a', 'a'), br('b', 'b'))
	expected = [][2]byte{br('a', 'b')}
	assertEq(t, expected, branges(cls))
}

// TestClassCaseFoldUnicode is class_case_fold_unicode in mod.rs.
func TestClassCaseFoldUnicode(t *testing.T) {
	t.Parallel()

	cls := uclass(
		ur('C', 'F'),
		ur('A', 'G'),
		ur('D', 'J'),
		ur('A', 'C'),
		ur('M', 'P'),
		ur('L', 'S'),
		ur('c', 'f'))
	expected := uclass(
		ur('A', 'J'),
		ur('L', 'S'),
		ur('a', 'j'),
		ur('l', 's'),
		ur('\u017F', '\u017F'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('A', 'Z'))
	expected = uclass(
		ur('A', 'Z'),
		ur('a', 'z'),
		ur('\u017F', '\u017F'),
		ur('\u212A', '\u212A'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('a', 'z'))
	expected = uclass(
		ur('A', 'Z'),
		ur('a', 'z'),
		ur('\u017F', '\u017F'),
		ur('\u212A', '\u212A'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('A', 'A'), ur('_', '_'))
	expected = uclass(ur('A', 'A'), ur('_', '_'), ur('a', 'a'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('A', 'A'), ur('=', '='))
	expected = uclass(ur('=', '='), ur('A', 'A'), ur('a', 'a'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('\x00', '\x10'))
	assertEq(t, cls, ucasefold(cls))
	cls = uclass(ur('k', 'k'))
	expected = uclass(ur('K', 'K'), ur('k', 'k'), ur('\u212A', '\u212A'))
	assertEq(t, expected, ucasefold(cls))
	cls = uclass(ur('@', '@'))
	assertEq(t, cls, ucasefold(cls))
}

// TestClassCaseFoldBytes is class_case_fold_bytes in mod.rs.
func TestClassCaseFoldBytes(t *testing.T) {
	t.Parallel()

	cls := bclass(
		br('C', 'F'),
		br('A', 'G'),
		br('D', 'J'),
		br('A', 'C'),
		br('M', 'P'),
		br('L', 'S'),
		br('c', 'f'))
	expected := bclass(br('A', 'J'), br('L', 'S'), br('a', 'j'), br('l', 's'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('A', 'Z'))
	expected = bclass(br('A', 'Z'), br('a', 'z'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('a', 'z'))
	expected = bclass(br('A', 'Z'), br('a', 'z'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('A', 'A'), br('_', '_'))
	expected = bclass(br('A', 'A'), br('_', '_'), br('a', 'a'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('A', 'A'), br('=', '='))
	expected = bclass(br('=', '='), br('A', 'A'), br('a', 'a'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('\x00', '\x10'))
	assertEq(t, cls, bcasefold(cls))
	cls = bclass(br('k', 'k'))
	expected = bclass(br('K', 'K'), br('k', 'k'))
	assertEq(t, expected, bcasefold(cls))
	cls = bclass(br('@', '@'))
	assertEq(t, cls, bcasefold(cls))
}

// TestClassNegateUnicode is class_negate_unicode in mod.rs.
func TestClassNegateUnicode(t *testing.T) {
	t.Parallel()

	cls := uclass(ur('a', 'a'))
	expected := uclass(ur('\x00', '\x60'), ur('\x62', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('a', 'a'), ur('b', 'b'))
	expected = uclass(ur('\x00', '\x60'), ur('\x63', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('a', 'c'), ur('x', 'z'))
	expected = uclass(
		ur('\x00', '\x60'),
		ur('\x64', '\x77'),
		ur('\x7B', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\x00', 'a'))
	expected = uclass(ur('\x62', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('a', '\U0010FFFF'))
	expected = uclass(ur('\x00', '\x60'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\x00', '\U0010FFFF'))
	expected = uclass()
	assertEq(t, expected, unegate(cls))
	cls = uclass()
	expected = uclass(ur('\x00', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\x00', '\U0010FFFD'), ur('\U0010FFFF', '\U0010FFFF'))
	expected = uclass(ur('\U0010FFFE', '\U0010FFFE'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\x00', '\uD7FF'))
	expected = uclass(ur('\uE000', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\x00', '\uD7FE'))
	expected = uclass(ur('\uD7FF', '\U0010FFFF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\uE000', '\U0010FFFF'))
	expected = uclass(ur('\x00', '\uD7FF'))
	assertEq(t, expected, unegate(cls))
	cls = uclass(ur('\uE001', '\U0010FFFF'))
	expected = uclass(ur('\x00', '\uE000'))
	assertEq(t, expected, unegate(cls))
}

// TestClassNegateBytes is class_negate_bytes in mod.rs.
func TestClassNegateBytes(t *testing.T) {
	t.Parallel()

	cls := bclass(br('a', 'a'))
	expected := bclass(br('\x00', '\x60'), br('\x62', '\xFF'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('a', 'a'), br('b', 'b'))
	expected = bclass(br('\x00', '\x60'), br('\x63', '\xFF'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('a', 'c'), br('x', 'z'))
	expected = bclass(
		br('\x00', '\x60'),
		br('\x64', '\x77'),
		br('\x7B', '\xFF'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('\x00', 'a'))
	expected = bclass(br('\x62', '\xFF'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('a', '\xFF'))
	expected = bclass(br('\x00', '\x60'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('\x00', '\xFF'))
	expected = bclass()
	assertEq(t, expected, bnegate(cls))
	cls = bclass()
	expected = bclass(br('\x00', '\xFF'))
	assertEq(t, expected, bnegate(cls))
	cls = bclass(br('\x00', '\xFD'), br('\xFF', '\xFF'))
	expected = bclass(br('\xFE', '\xFE'))
	assertEq(t, expected, bnegate(cls))
}

// TestClassUnionUnicode is class_union_unicode in mod.rs.
func TestClassUnionUnicode(t *testing.T) {
	t.Parallel()

	cls1 := uclass(ur('a', 'g'), ur('m', 't'), ur('A', 'C'))
	cls2 := uclass(ur('a', 'z'))
	expected := uclass(ur('a', 'z'), ur('A', 'C'))
	assertEq(t, expected, uunion(cls1, cls2))
}

// TestClassUnionBytes is class_union_bytes in mod.rs.
func TestClassUnionBytes(t *testing.T) {
	t.Parallel()

	cls1 := bclass(br('a', 'g'), br('m', 't'), br('A', 'C'))
	cls2 := bclass(br('a', 'z'))
	expected := bclass(br('a', 'z'), br('A', 'C'))
	assertEq(t, expected, bunion(cls1, cls2))
}

// TestClassIntersectUnicode is class_intersect_unicode in mod.rs.
func TestClassIntersectUnicode(t *testing.T) {
	t.Parallel()

	cls1 := uclass()
	cls2 := uclass(ur('a', 'a'))
	expected := uclass()
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'a'))
	cls2 = uclass(ur('a', 'a'))
	expected = uclass(ur('a', 'a'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'a'))
	cls2 = uclass(ur('b', 'b'))
	expected = uclass()
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'a'))
	cls2 = uclass(ur('a', 'c'))
	expected = uclass(ur('a', 'a'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'))
	cls2 = uclass(ur('a', 'c'))
	expected = uclass(ur('a', 'b'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'))
	cls2 = uclass(ur('b', 'c'))
	expected = uclass(ur('b', 'b'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'))
	cls2 = uclass(ur('c', 'd'))
	expected = uclass()
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('b', 'c'))
	cls2 = uclass(ur('a', 'd'))
	expected = uclass(ur('b', 'c'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	cls2 = uclass(ur('a', 'h'))
	expected = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	cls2 = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	expected = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('g', 'h'))
	cls2 = uclass(ur('d', 'e'), ur('k', 'l'))
	expected = uclass()
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('d', 'e'), ur('g', 'h'))
	cls2 = uclass(ur('h', 'h'))
	expected = uclass(ur('h', 'h'))
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('e', 'f'), ur('i', 'j'))
	cls2 = uclass(ur('c', 'd'), ur('g', 'h'), ur('k', 'l'))
	expected = uclass()
	assertEq(t, expected, uintersect(cls1, cls2))
	cls1 = uclass(ur('a', 'b'), ur('c', 'd'), ur('e', 'f'))
	cls2 = uclass(ur('b', 'c'), ur('d', 'e'), ur('f', 'g'))
	expected = uclass(ur('b', 'f'))
	assertEq(t, expected, uintersect(cls1, cls2))
}

// TestClassIntersectBytes is class_intersect_bytes in mod.rs.
func TestClassIntersectBytes(t *testing.T) {
	t.Parallel()

	cls1 := bclass()
	cls2 := bclass(br('a', 'a'))
	expected := bclass()
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'a'))
	cls2 = bclass(br('a', 'a'))
	expected = bclass(br('a', 'a'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'a'))
	cls2 = bclass(br('b', 'b'))
	expected = bclass()
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'a'))
	cls2 = bclass(br('a', 'c'))
	expected = bclass(br('a', 'a'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'))
	cls2 = bclass(br('a', 'c'))
	expected = bclass(br('a', 'b'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'))
	cls2 = bclass(br('b', 'c'))
	expected = bclass(br('b', 'b'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'))
	cls2 = bclass(br('c', 'd'))
	expected = bclass()
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('b', 'c'))
	cls2 = bclass(br('a', 'd'))
	expected = bclass(br('b', 'c'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	cls2 = bclass(br('a', 'h'))
	expected = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	cls2 = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	expected = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('g', 'h'))
	cls2 = bclass(br('d', 'e'), br('k', 'l'))
	expected = bclass()
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('d', 'e'), br('g', 'h'))
	cls2 = bclass(br('h', 'h'))
	expected = bclass(br('h', 'h'))
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('e', 'f'), br('i', 'j'))
	cls2 = bclass(br('c', 'd'), br('g', 'h'), br('k', 'l'))
	expected = bclass()
	assertEq(t, expected, bintersect(cls1, cls2))
	cls1 = bclass(br('a', 'b'), br('c', 'd'), br('e', 'f'))
	cls2 = bclass(br('b', 'c'), br('d', 'e'), br('f', 'g'))
	expected = bclass(br('b', 'f'))
	assertEq(t, expected, bintersect(cls1, cls2))
}

// TestClassDifferenceUnicode is class_difference_unicode in mod.rs.
func TestClassDifferenceUnicode(t *testing.T) {
	t.Parallel()

	cls1 := uclass(ur('a', 'a'))
	cls2 := uclass(ur('a', 'a'))
	expected := uclass()
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'a'))
	cls2 = uclass()
	expected = uclass(ur('a', 'a'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass()
	cls2 = uclass(ur('a', 'a'))
	expected = uclass()
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'z'))
	cls2 = uclass(ur('a', 'a'))
	expected = uclass(ur('b', 'z'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'z'))
	cls2 = uclass(ur('z', 'z'))
	expected = uclass(ur('a', 'y'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'z'))
	cls2 = uclass(ur('m', 'm'))
	expected = uclass(ur('a', 'l'), ur('n', 'z'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'c'), ur('g', 'i'), ur('r', 't'))
	cls2 = uclass(ur('a', 'z'))
	expected = uclass()
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'c'), ur('g', 'i'), ur('r', 't'))
	cls2 = uclass(ur('d', 'v'))
	expected = uclass(ur('a', 'c'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'c'), ur('g', 'i'), ur('r', 't'))
	cls2 = uclass(ur('b', 'g'), ur('s', 'u'))
	expected = uclass(ur('a', 'a'), ur('h', 'i'), ur('r', 'r'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'c'), ur('g', 'i'), ur('r', 't'))
	cls2 = uclass(ur('b', 'd'), ur('e', 'g'), ur('s', 'u'))
	expected = uclass(ur('a', 'a'), ur('h', 'i'), ur('r', 'r'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('x', 'z'))
	cls2 = uclass(ur('a', 'c'), ur('e', 'g'), ur('s', 'u'))
	expected = uclass(ur('x', 'z'))
	assertEq(t, expected, udifference(cls1, cls2))
	cls1 = uclass(ur('a', 'z'))
	cls2 = uclass(ur('a', 'c'), ur('e', 'g'), ur('s', 'u'))
	expected = uclass(ur('d', 'd'), ur('h', 'r'), ur('v', 'z'))
	assertEq(t, expected, udifference(cls1, cls2))
}

// TestClassDifferenceBytes is class_difference_bytes in mod.rs.
func TestClassDifferenceBytes(t *testing.T) {
	t.Parallel()

	cls1 := bclass(br('a', 'a'))
	cls2 := bclass(br('a', 'a'))
	expected := bclass()
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'a'))
	cls2 = bclass()
	expected = bclass(br('a', 'a'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass()
	cls2 = bclass(br('a', 'a'))
	expected = bclass()
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'z'))
	cls2 = bclass(br('a', 'a'))
	expected = bclass(br('b', 'z'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'z'))
	cls2 = bclass(br('z', 'z'))
	expected = bclass(br('a', 'y'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'z'))
	cls2 = bclass(br('m', 'm'))
	expected = bclass(br('a', 'l'), br('n', 'z'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'c'), br('g', 'i'), br('r', 't'))
	cls2 = bclass(br('a', 'z'))
	expected = bclass()
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'c'), br('g', 'i'), br('r', 't'))
	cls2 = bclass(br('d', 'v'))
	expected = bclass(br('a', 'c'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'c'), br('g', 'i'), br('r', 't'))
	cls2 = bclass(br('b', 'g'), br('s', 'u'))
	expected = bclass(br('a', 'a'), br('h', 'i'), br('r', 'r'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'c'), br('g', 'i'), br('r', 't'))
	cls2 = bclass(br('b', 'd'), br('e', 'g'), br('s', 'u'))
	expected = bclass(br('a', 'a'), br('h', 'i'), br('r', 'r'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('x', 'z'))
	cls2 = bclass(br('a', 'c'), br('e', 'g'), br('s', 'u'))
	expected = bclass(br('x', 'z'))
	assertEq(t, expected, bdifference(cls1, cls2))
	cls1 = bclass(br('a', 'z'))
	cls2 = bclass(br('a', 'c'), br('e', 'g'), br('s', 'u'))
	expected = bclass(br('d', 'd'), br('h', 'r'), br('v', 'z'))
	assertEq(t, expected, bdifference(cls1, cls2))
}

// TestClassSymmetricDifferenceUnicode is class_symmetric_difference_unicode in mod.rs.
func TestClassSymmetricDifferenceUnicode(t *testing.T) {
	t.Parallel()

	cls1 := uclass(ur('a', 'm'))
	cls2 := uclass(ur('g', 't'))
	expected := uclass(ur('a', 'f'), ur('n', 't'))
	assertEq(t, expected, usymdifference(cls1, cls2))
}

// TestClassSymmetricDifferenceBytes is class_symmetric_difference_bytes in mod.rs.
func TestClassSymmetricDifferenceBytes(t *testing.T) {
	t.Parallel()

	cls1 := bclass(br('a', 'm'))
	cls2 := bclass(br('g', 't'))
	expected := bclass(br('a', 'f'), br('n', 't'))
	assertEq(t, expected, bsymdifference(cls1, cls2))
}

// TestNoStackOverflowOnDrop is no_stack_overflow_on_drop in mod.rs.
//
// Upstream builds a deep Hir on a thread with a small stack, to test that
// the Drop of Hir uses a stack of the same depth for any Hir. Go frees an
// Hir with the garbage collector and grows the stack of a goroutine, so the
// test builds the same Hir and checks it.
func TestNoStackOverflowOnDrop(t *testing.T) {
	t.Parallel()
	expr := NewEmpty()
	for range 100 {
		expr = NewCapture(Capture{
			Index: 1,
			Name:  "",
			Sub:   expr,
		})
		expr = NewRepetition(Repetition{
			Min:    0,
			Max:    new(uint32(1)),
			Greedy: true,
			Sub:    expr,
		})

		c := Concat{expr}
		expr = &Hir{kind: &c, props: emptyProperties()}
		a := Alternation{expr}
		expr = &Hir{kind: &a, props: emptyProperties()}
	}
	if _, ok := expr.Kind().(*Empty); ok {
		t.Error("the expression is empty")
	}
}

// count returns the number of values of seq.
func count[T any](seq func(func(T) bool)) int {
	n := 0
	for range seq {
		n++
	}
	return n
}

// TestLookSetIter is look_set_iter in mod.rs.
func TestLookSetIter(t *testing.T) {
	t.Parallel()
	set := EmptyLookSet()
	assertEq(t, 0, count(set.Iter()))

	set = FullLookSet()
	assertEq(t, 18, count(set.Iter()))

	set = EmptyLookSet().Insert(LookStartLF).Insert(LookWordUnicode)
	assertEq(t, 2, count(set.Iter()))

	set = EmptyLookSet().Insert(LookStartLF)
	assertEq(t, 1, count(set.Iter()))

	set = EmptyLookSet().Insert(LookWordASCIINegate)
	assertEq(t, 1, count(set.Iter()))
}

// TestLookSetDebug is look_set_debug in mod.rs.
func TestLookSetDebug(t *testing.T) {
	t.Parallel()
	res := EmptyLookSet().String()
	assertEq(t, "∅", res)
	res = FullLookSet().String()
	assertEq(t, "Az^$rRbB𝛃𝚩<>〈〉◁▷◀▶", res)
}
