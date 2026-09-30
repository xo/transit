package grammartest

import (
	"slices"
	"testing"

	"github.com/xo/transit"
)

// This file tests the helpers of parse.go that need no language. The test
// module runs RenderCST on the corpus cases with the attribute :cst of the
// json grammar and of the test grammar basic_cst of upstream.

// TestFromUTF8Lossy makes sure that each maximal part of an invalid
// sequence becomes one U+FFFD, as in String::from_utf8_lossy.
func TestFromUTF8Lossy(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"abc":              "abc",
		"a\xffb":           "a\uFFFDb",
		"\xff\xfe":         "\uFFFD\uFFFD",
		"\xe2\x82":         "\uFFFD",
		"\xe2\x82a":        "\uFFFDa",
		"\xe2\x82\xac":     "\u20AC",
		"\xed\xa0\x80":     "\uFFFD\uFFFD\uFFFD",
		"\xf0\x9f\x98":     "\uFFFD",
		"\xf0\x9f\x98\x80": "\U0001F600",
		"\xf4\x90\x80\x80": "\uFFFD\uFFFD\uFFFD\uFFFD",
		"\xc0\xaf":         "\uFFFD\uFFFD",
		"\uFFFD":           "\uFFFD",
	} {
		if got := fromUTF8Lossy([]byte(in)); got != want {
			t.Errorf("fromUTF8Lossy(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRustLines makes sure that rustLines splits a text as str::lines does.
func TestRustLines(t *testing.T) {
	t.Parallel()
	for in, want := range map[string][]string{
		"":          nil,
		"a":         {"a"},
		"a\n":       {"a"},
		"a\r\nb":    {"a", "b"},
		"a\n\nb\n":  {"a", "", "b"},
		"a\rb\r":    {"a\rb\r"},
		"a\r\n\r\n": {"a", ""},
	} {
		got := slices.Collect(rustLines(in))
		if !slices.Equal(got, want) {
			t.Errorf("rustLines(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCSTNodeRange makes sure that the range of a node is padded to the
// width of the widest range, and with one space at least.
func TestCSTNodeRange(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		width int
		r     transit.Range
		want  string
	}{
		{1, transit.Range{EndPoint: transit.Point{Column: 5}}, "0:0 - 0:5 "},
		{3, transit.Range{StartPoint: transit.Point{Column: 7}, EndPoint: transit.Point{Row: 1, Column: 12}}, "0:7   - 1:12  "},
		{1, transit.Range{StartPoint: transit.Point{Row: 10, Column: 100}, EndPoint: transit.Point{Row: 10, Column: 100}}, "10:100 - 10:100 "},
	} {
		if got := cstNodeRange(c.width, c.r); got != c.want {
			t.Errorf("cstNodeRange(%d, %+v) = %q, want %q", c.width, c.r, got, c.want)
		}
	}
}

// TestCSTNodeText makes sure that the invisible characters and the quotes
// are escaped.
func TestCSTNodeText(t *testing.T) {
	t.Parallel()
	if got, want := cstNodeText("a\n\r\t\x00\\\x0b\x0c`\"b"), "a\\n\\r\\t\\0\\\\\\v\\f\\`\\\"b"; got != want {
		t.Errorf("cstNodeText = %q, want %q", got, want)
	}
	if got, want := cstLineFeed("a\"\n"), "a\\\"\\n"; got != want {
		t.Errorf("cstLineFeed = %q, want %q", got, want)
	}
}
