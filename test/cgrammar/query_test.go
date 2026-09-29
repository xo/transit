package cgrammar

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// queryExtra holds queries that reach parts of the query engine that the
// queries of a grammar can miss: anchors, quantifiers, alternations, fields,
// negated fields, wildcards, supertypes and MISSING. A query that names a
// node or a field that a grammar does not have fails to compile, and the
// test compares that error too.
var queryExtra = []string{
	`(_ (_) @a . (_) @b)`,
	`(_ . (_) @first)`,
	`(_ (_) @last .)`,
	`(_ (_)+ @kids) @p`,
	`(_ (_)* @kids . (_) @x)`,
	`(_ (_)? @opt (_) @x)`,
	`(ERROR) @e`,
	`(MISSING) @m`,
	`(_ [(_) @n "("]) @p`,
	`((_) @a (_) @b)`,
	`((_)+ @a . (_) @b)`,
	`_ @any`,
	`(_) @named`,
	`(_ _ @anon . _ @anon2)`,
	`[(_ (_) @a) (_) @b]`,
	`(_ . _ @first _ @second .)`,
	`((_) @a . (_)? @b . (_) @c)`,
	`(_expression/identifier) @e`,
	`(_expression) @e`,
	`(expression/identifier) @e`,
	`(function_item name: (identifier) @n !return_type)`,
	`(function_declaration name: (identifier) @n !result)`,
	`(call_expression function: (_) @f arguments: (_ . (_) @first))`,
	`(binary_expression left: (_)* @l right: (_)? @r)`,
	`(call_expression [(identifier) (field_expression)] @f (arguments (_)+ @args))`,
	`((comment)+ @c . (function_item) @f)`,
	`((comment)* @c . (function_declaration) @f)`,
	`(_ (comment) @c . [(identifier) (number_literal)] @x)`,
	`(identifier) @a @b`,
	`(MISSING identifier) @m`,
	`(MISSING ";") @m`,
	`(block . (_) @first (_)* @rest)`,
}

// stripPredicates removes each predicate, a group that starts with "(#",
// from the text of a query. The C runtime does not evaluate predicates, and
// the Go runtime does, so the test compares the two on queries with no
// predicate. A string or a comment of the query is kept as it is.
func stripPredicates(source string) string {
	var b strings.Builder
	for i := 0; i < len(source); {
		switch c := source[i]; {
		case c == '"':
			j := skipString(source, i)
			b.WriteString(source[i:j])
			i = j
		case c == ';':
			j := strings.IndexByte(source[i:], '\n')
			if j < 0 {
				j = len(source) - i
			}
			b.WriteString(source[i : i+j])
			i += j
		case c == '(' && i+1 < len(source) && source[i+1] == '#':
			i = skipGroup(source, i)
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// skipString returns the offset after the string that starts at i.
func skipString(source string, i int) int {
	for j := i + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(source)
}

// skipGroup returns the offset after the group of parentheses that starts
// at i, with the strings in it.
func skipGroup(source string, i int) int {
	depth := 0
	for j := i; j < len(source); {
		switch source[j] {
		case '"':
			j = skipString(source, j)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j + 1
			}
		}
		j++
	}
	return len(source)
}

func TestStripPredicates(t *testing.T) {
	for in, want := range map[string]string{
		`((identifier) @x (#eq? @x "a)b"))`:              `((identifier) @x )`,
		`(a) @x (#match? @x "^(x|y)$") ; (#eq? @x)`:      `(a) @x  ; (#eq? @x)`,
		`(string "(#not a predicate)") @s`:               `(string "(#not a predicate)") @s`,
		`((a) @x (#set! "k" "v") (#any-of? @x "a" "b"))`: `((a) @x  )`,
	} {
		if got := stripPredicates(in); got != want {
			t.Errorf("stripPredicates(%q) = %q, want %q", in, got, want)
		}
	}
}

// fixtureQueries returns the queries of a fixture grammar with their
// predicates removed, and the queries of queryExtra.
func fixtureQueries(t *testing.T, cache string, f fixture) []string {
	t.Helper()
	dir, _ := grammarDirs(cache, f)
	repo := strings.TrimPrefix(f.Repository, "https://github.com/")
	checkout := filepath.Join(cache, "grammars", filepath.FromSlash(repo))
	var out []string
	seen := map[string]bool{}
	for _, d := range []string{filepath.Join(dir, "queries"), filepath.Join(checkout, "queries")} {
		files, err := filepath.Glob(filepath.Join(d, "*.scm"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			resolved, err := filepath.EvalSymlinks(file)
			if err != nil || seen[resolved] {
				continue
			}
			seen[resolved] = true
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, stripPredicates(string(b)))
		}
	}
	return append(out, queryExtra...)
}

// queryRuns are the settings of the runs of each query on each input: the
// matches and the captures, a byte range, a small match limit and a limit of
// the start depth. A byte range from 0 to 0 is the whole text.
func queryRuns(n uint32) []QueryRun {
	all := uint32(math.MaxUint32)
	return []QueryRun{
		{false, 0, 0, all, all},
		{true, 0, 0, all, all},
		{true, n / 3, 2 * n / 3, all, all},
		{false, n / 3, 2 * n / 3, all, all},
		{false, 0, 0, 3, all},
		{true, 0, 0, 3, all},
		{true, 0, 0, 1, all},
		{false, 0, 0, all, 2},
		{true, 0, 0, all, 1},
	}
}

// TestFixtureQueriesMatchC compiles the queries of each fixture grammar,
// with their predicates removed, with the C runtime and with the Go
// runtime, and compares the patterns. Then it runs each query on the first
// inputs of the corpus with the settings of queryRuns, and compares each
// match, each capture and the match limit.
func TestFixtureQueriesMatchC(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadFixture(t, cache, f)
			examples = examples[:min(len(examples), 60)]
			queries := fixtureQueries(t, cache, f)
			p := transit.NewParser()
			if err := p.SetLanguage(g.Language); err != nil {
				t.Fatal(err)
			}
			trees := make([]*transit.Tree, len(examples))
			sessions := make([]*CSession, len(examples))
			for i, e := range examples {
				tree, err := goParse(p, e.Input)
				if err != nil {
					t.Fatal(err)
				}
				trees[i] = tree
				if sessions[i], err = g.NewCSession(e.Input); err != nil {
					t.Fatal(err)
				}
				defer sessions[i].Close()
			}
			failures, runs := 0, 0
			for qi, source := range queries {
				cq, want, err := g.NewCQuery(source)
				if err != nil {
					t.Fatal(err)
				}
				gq, got := NewGoQuery(g.Language, source)
				if cq != nil {
					want = cq.Describe()
					defer cq.Close()
				}
				if gq != nil {
					got = DescribeGoQuery(gq, source)
				}
				if got != want {
					failures++
					t.Errorf("query %d compiles differently:\n  C:  %.500s\n  Go: %.500s", qi, want, got)
					continue
				}
				if cq == nil {
					continue
				}
				for ei, e := range examples {
					for _, run := range queryRuns(uint32(len(e.Input))) {
						runs++
						want := cq.Run(sessions[ei], run)
						if got := RunGoQuery(gq, trees[ei], e.Input, run); got != want {
							failures++
							if failures <= 3 {
								t.Errorf("query %d on %s with %+v differs:\nquery: %.200s\nC:\n%.2000s\nGo:\n%.2000s", qi, e.Name, run, source, want, got)
							}
						}
					}
				}
			}
			t.Logf("%d queries, %d runs, %d differ", len(queries), runs, failures)
		})
	}
}
