package sqlserver

import (
	"context"
	"strings"
	"testing"

	"github.com/xo/transit/internal/grammartest"
)

// TestHighlightNames highlights a text with queries/highlights.scm, and
// checks the capture name of each word that a case names. The grammar has no
// comment, so a file of testdata/highlight cannot hold the assertions of
// the highlight test. Each word of a case is found after the word before it.
func TestHighlightNames(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		src   string
		names [][2]string
	}{
		{
			"select foo = Type, [bar] as baz, COUNT(*) from dbo.PostTypes",
			[][2]string{
				{"select", "keyword"},
				{"foo", "variable"},
				{"=", "operator"},
				{"Type", "field"},
				{",", "punctuation.delimiter"},
				{"[bar]", "field"},
				{"as", "keyword"},
				{"baz", "variable"},
				{"COUNT", "function.builtin"},
				{"(", "punctuation.bracket"},
				{"*", "operator"},
				{"from", "keyword"},
				{"dbo", "namespace"},
				{".", "punctuation.delimiter"},
				{"PostTypes", "type"},
			},
		},
		{
			"select 'foo', 0x1F, 10, 1.23e+2, $+3.0, NULL, @total",
			[][2]string{
				{"'foo'", "string"},
				{"0x1F", "number"},
				{"10", "number"},
				{"1.23e+2", "number.float"},
				{"$+3.0", "number"},
				{"NULL", "constant.builtin"},
				{"@total", "variable"},
			},
		},
		{
			"select RANK() OVER (PARTITION BY a ORDER BY b ROWS UNBOUNDED PRECEDING)",
			[][2]string{
				{"RANK", "function.builtin"},
				{"OVER", "keyword"},
				{"PARTITION BY", "keyword"},
				{"a", "field"},
				{"ORDER BY", "keyword"},
				{"b", "field"},
				{"ROWS", "keyword"},
				{"UNBOUNDED PRECEDING", "keyword"},
				{")", "punctuation.bracket"},
			},
		},
		{
			"EXEC dbo.foo @foo='spt_monitor', @baz=@quux OUTPUT;\nGO 2",
			[][2]string{
				{"EXEC", "keyword"},
				{"dbo", "namespace"},
				{"foo", "function.call"},
				{"@foo", "variable.parameter"},
				{"'spt_monitor'", "string"},
				{"@quux", "variable"},
				{"OUTPUT", "keyword"},
				{";", "punctuation.delimiter"},
				{"GO", "keyword"},
				{"2", "number"},
			},
		},
		{
			"Select foo::bar(@baz) as quux",
			[][2]string{
				{"foo", "field"},
				{"::", "operator"},
				{"bar", "function.method.call"},
				{"@baz", "variable"},
			},
		},
	} {
		check(t, test.src, test.names)
	}
}

// check highlights src, and makes sure that the first byte of each word of
// names has its capture name.
func check(t *testing.T, src string, names [][2]string) {
	t.Helper()
	g := grammartest.Grammar{Language: Language(), Queries: Queries}
	spans, err := grammartest.Spans(context.Background(), ".", g, nil, []byte(src))
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	from := 0
	for _, n := range names {
		i := strings.Index(src[from:], n[0])
		if i < 0 {
			t.Fatalf("%q: no %q after byte %d", src, n[0], from)
		}
		i += from
		from = i + len(n[0])
		var got string
		for _, s := range spans {
			if s.Start <= i && i < s.End {
				got = s.Name
			}
		}
		if got != n[1] {
			t.Errorf("%q: %q at byte %d is %q, and want %q", src, n[0], i, got, n[1])
		}
	}
}
