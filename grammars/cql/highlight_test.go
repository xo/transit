package cql

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
			"SELECT DISTINCT JSON col1, count(*) AS n FROM ks.users WHERE id = ? AND age >= 18 LIMIT 10 ALLOW FILTERING;",
			[][2]string{
				{"SELECT", "keyword"},
				{"DISTINCT", "keyword"},
				{"col1", "field"},
				{"count", "function.call"},
				{"(", "punctuation.bracket"},
				{"*", "operator"},
				{"AS", "keyword"},
				{"n", "variable"},
				{"FROM", "keyword"},
				{"ks", "namespace"},
				{".", "punctuation.delimiter"},
				{"users", "type"},
				{"id", "field"},
				{"=", "operator"},
				{"?", "variable.parameter"},
				{"AND", "keyword.operator"},
				{">=", "operator"},
				{"18", "number"},
				{"LIMIT", "keyword"},
				{"10", "number"},
				{"FILTERING", "keyword"},
				{";", "punctuation.delimiter"},
			},
		},
		{
			"CREATE TABLE IF NOT EXISTS t (id uuid PRIMARY KEY, tags set<text>, m map<int, text>, u frozen<my_type>);",
			[][2]string{
				{"CREATE", "keyword"},
				{"NOT", "keyword.operator"},
				{"t", "type"},
				{"id", "field"},
				{"uuid", "type.builtin"},
				{"PRIMARY", "keyword"},
				{"set", "type.builtin"},
				{"<", "punctuation.bracket"},
				{"text", "type.builtin"},
				{"map", "type.builtin"},
				{"frozen", "type.builtin"},
				{"my_type", "type"},
				{">", "punctuation.bracket"},
			},
		},
		{
			"UPDATE t USING TTL 60 SET a = 'x', b = true, c = null WHERE id = 5b6962dd-3f90-4c93-8f61-eabfa4a803e2;",
			[][2]string{
				{"UPDATE", "keyword"},
				{"USING", "keyword"},
				{"TTL", "keyword"},
				{"60", "number"},
				{"SET", "keyword"},
				{"a", "field"},
				{"'x'", "string"},
				{"true", "boolean"},
				{"null", "constant.builtin"},
				{"5b6962dd", "number"},
			},
		},
		{
			"CREATE KEYSPACE ks WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 3} AND durable_writes = false;",
			[][2]string{
				{"KEYSPACE", "keyword"},
				{"ks", "namespace"},
				{"WITH", "keyword"},
				{"{", "punctuation.bracket"},
				{"'class'", "string.special.key"},
				{":", "punctuation.delimiter"},
				{"'SimpleStrategy'", "string"},
				{"3", "number"},
				{"false", "boolean"},
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
