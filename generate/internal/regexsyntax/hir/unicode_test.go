package hir

import (
	"bytes"
	"slices"
	"testing"
)

// This file ports the tests of src/unicode.rs.
//
// The test simple_fold_disabled is not here. It runs only when the feature
// unicode-case is off, and the port follows the default features, which turn
// it on.

// simpleFoldOK returns the characters that c case folds with.
//
// simpleFoldOK is simple_fold_ok.
func simpleFoldOK(c rune) []rune {
	return slices.Clone(newSimpleCaseFolder().mapping(c))
}

// containsCaseMap reports whether a character from start to end has an
// entry in the table of case folding.
//
// containsCaseMap is contains_case_map.
func containsCaseMap(start, end rune) bool {
	return newSimpleCaseFolder().overlaps(start, end)
}

// TestSimpleFoldK is simple_fold_k in unicode.rs.
func TestSimpleFoldK(t *testing.T) {
	t.Parallel()
	xs := simpleFoldOK('k')
	expectRunes(t, xs, []rune{'K', '\u212A'})

	xs = simpleFoldOK('K')
	expectRunes(t, xs, []rune{'k', '\u212A'})

	xs = simpleFoldOK('\u212A')
	expectRunes(t, xs, []rune{'K', 'k'})
}

// TestSimpleFoldA is simple_fold_a in unicode.rs.
func TestSimpleFoldA(t *testing.T) {
	t.Parallel()
	xs := simpleFoldOK('a')
	expectRunes(t, xs, []rune{'A'})

	xs = simpleFoldOK('A')
	expectRunes(t, xs, []rune{'a'})
}

// expectRunes fails the test if got and want do not hold the same
// characters in the same order.
func expectRunes(t *testing.T, got, want []rune) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRangeContains is range_contains in unicode.rs.
func TestRangeContains(t *testing.T) {
	t.Parallel()
	assert(t, containsCaseMap('A', 'A'))
	assert(t, containsCaseMap('Z', 'Z'))
	assert(t, containsCaseMap('A', 'Z'))
	assert(t, containsCaseMap('@', 'A'))
	assert(t, containsCaseMap('Z', '['))
	assert(t, containsCaseMap('\u2603', '\u2C00'))

	assert(t, !containsCaseMap('[', '['))
	assert(t, !containsCaseMap('[', '`'))

	assert(t, !containsCaseMap('\u2603', '\u2603'))
}

// TestRegression466 is regression_466 in unicode.rs.
func TestRegression466(t *testing.T) {
	t.Parallel()
	q := classQuery{kind: queryOneLetter, letter: 'C'}
	got, err := q.canonicalize()
	if err != nil {
		t.Fatal(err)
	}
	want := canonicalClassQuery{kind: canonicalGeneralCategory, name: "Other"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestSymNormalize is sym_normalize in unicode.rs.
func TestSymNormalize(t *testing.T) {
	t.Parallel()
	symNorm := symbolicNameNormalize

	expectEq(t, symNorm("Line_Break"), "linebreak")
	expectEq(t, symNorm("Line-break"), "linebreak")
	expectEq(t, symNorm("linebreak"), "linebreak")
	expectEq(t, symNorm("BA"), "ba")
	expectEq(t, symNorm("ba"), "ba")
	expectEq(t, symNorm("Greek"), "greek")
	expectEq(t, symNorm("isGreek"), "greek")
	expectEq(t, symNorm("IS_Greek"), "greek")
	expectEq(t, symNorm("isc"), "isc")
	expectEq(t, symNorm("is c"), "isc")
	expectEq(t, symNorm("is_c"), "isc")
}

// TestValidUTF8Symbolic is valid_utf8_symbolic in unicode.rs.
func TestValidUTF8Symbolic(t *testing.T) {
	t.Parallel()
	x := []byte("abc\xFFxyz")
	y := symbolicNameNormalizeBytes(x)
	if !bytes.Equal(y, []byte("abcxyz")) {
		t.Errorf("got %q, want %q", y, "abcxyz")
	}
}
