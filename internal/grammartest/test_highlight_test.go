package grammartest

import (
	"errors"
	"testing"

	"github.com/xo/transit"
)

// This file holds the tests of the highlight test and of its assertions.

// TestToUTF8Point makes sure that a point of the runtime, in bytes, becomes
// a point in characters.
func TestToUTF8Point(t *testing.T) {
	src := []byte("ab\né x\n")
	for _, c := range []struct {
		point transit.Point
		want  utf8Point
	}{
		{transit.Point{Row: 0, Column: 0}, utf8Point{0, 0}},
		{transit.Point{Row: 0, Column: 1}, utf8Point{0, 1}},
		{transit.Point{Row: 1, Column: 2}, utf8Point{1, 1}},
		{transit.Point{Row: 1, Column: 3}, utf8Point{1, 2}},
	} {
		if got := toUTF8Point(c.point, src); got != c.want {
			t.Errorf("toUTF8Point(%v) = %v, want %v", c.point, got, c.want)
		}
	}
}

// TestIterateAssertions checks assertions against highlights in the order
// of the text, as get_highlight_positions gives them: the text of an escape
// inside a string, and a number.
func TestIterateAssertions(t *testing.T) {
	names := []string{"string", "escape", "number"}
	highlights := []highlightPosition{
		{start: utf8Point{0, 0}, end: utf8Point{0, 2}, highlight: 0},
		{start: utf8Point{0, 2}, end: utf8Point{0, 4}, highlight: 1},
		{start: utf8Point{0, 4}, end: utf8Point{0, 10}, highlight: 0},
		{start: utf8Point{1, 0}, end: utf8Point{1, 3}, highlight: 2},
	}
	for _, c := range []struct {
		name string
		a    assertion
		want string
	}{
		{"the innermost highlight", assertion{position: utf8Point{0, 2}, length: 1, expectedCaptureName: "escape"}, ""},
		{"the outer highlight", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "string"}, ""},
		{"a wrong name", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "number"},
			"Failure - row: 0, column: 6, expected highlight 'number', actual highlights: 'string'"},
		{"a negative assertion", assertion{position: utf8Point{1, 0}, length: 2, negative: true, expectedCaptureName: "string"}, ""},
		{"a negative assertion that fails", assertion{position: utf8Point{1, 0}, length: 1, negative: true, expectedCaptureName: "number"},
			"Failure - row: 1, column: 0, expected highlight '!number', actual highlights: 'number'"},
		{"no highlight", assertion{position: utf8Point{2, 0}, length: 1, negative: true, expectedCaptureName: "x"},
			"Failure - row: 2, column: 0, expected highlight '!x', actual highlights: none."},
		{"arrows over two highlights", assertion{position: utf8Point{0, 1}, length: 3, expectedCaptureName: "escape"}, ""},
	} {
		n, err := iterateAssertions([]assertion{c.a}, highlights, names)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: got the error %v", c.name, err)
		case c.want == "" && n != 1:
			t.Errorf("%s: counted %d assertions", c.name, n)
		case c.want != "" && (err == nil || err.Error() != c.want):
			t.Errorf("%s: got the error %v, want %s", c.name, err, c.want)
		case c.want != "" && !errors.Is(err, errAssertion):
			t.Errorf("%s: the error %v is not errAssertion", c.name, err)
		}
	}
}

// TestPackageQueryPath makes sure that a query file of another grammar that
// tree-sitter.json lists is in the folder of that grammar under queries/
// (D84), and that any other path stays the same.
func TestPackageQueryPath(t *testing.T) {
	for p, want := range map[string]string{
		"queries/highlights.scm": "queries/highlights.scm",
		"./queries/locals.scm":   "queries/locals.scm",
		"node_modules/tree-sitter-javascript/queries/highlights.scm": "queries/javascript/highlights.scm",
		"node_modules/tree-sitter-c/queries/highlights.scm":          "queries/c/highlights.scm",
		"node_modules/tree-sitter-c-sharp/queries/tags.scm":          "queries/c_sharp/tags.scm",
		"node_modules/other/queries/highlights.scm":                  "node_modules/other/queries/highlights.scm",
	} {
		if got := packageQueryPath(p); got != want {
			t.Errorf("packageQueryPath(%q) = %q, want %q", p, got, want)
		}
	}
}
