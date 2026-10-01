package transit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// queryBindingText is the text of treeSample, which the predicates read.
var queryBindingText = []byte(treeSampleText)

// queryBindingNew compiles a query with NewQuery on the test language, and
// fails the test on an error.
func queryBindingNew(t *testing.T, source string) *Query {
	t.Helper()
	q, err := NewQuery(testLanguage(15), source)
	if err != nil {
		t.Fatalf("NewQuery(%q) = %v", source, err)
	}
	return q
}

// queryBindingMatches returns each match of a query on treeSample, as the
// text of each capture.
func queryBindingMatches(q *Query, c *QueryCursor) []string {
	tree := treeSample(q.ptr.language)
	var out []string
	for m := range c.Matches(context.Background(), q, tree.RootNode(), queryBindingText) {
		var parts []string
		for _, capture := range m.Captures {
			parts = append(parts, fmt.Sprintf("%s=%s", q.CaptureNames()[capture.Index], capture.Node.Text(queryBindingText)))
		}
		out = append(out, fmt.Sprintf("%d: %s", m.PatternIndex, strings.Join(parts, " ")))
	}
	return out
}

// queryBindingError compiles a query that must fail, and returns its error.
func queryBindingError(t *testing.T, l *Language, source string) *QueryError {
	t.Helper()
	_, err := NewQuery(l, source)
	var qerr *QueryError
	if !errors.As(err, &qerr) {
		t.Fatalf("NewQuery(%q) = %v, want a *QueryError", source, err)
	}
	return qerr
}

func TestQueryBindingTextPredicates(t *testing.T) {
	tests := []struct {
		source string
		want   []string
	}{
		{`((identifier) @id (#eq? @id "a"))`, []string{"0: id=a"}},
		{`((identifier) @id (#not-eq? @id "a"))`, []string{
			"0: id=x", "0: id=y", "0: id=b", "0: id=c", "0: id=z",
		}},
		{`((identifier) @id (#match? @id "^[abc]$"))`, []string{
			"0: id=a", "0: id=b", "0: id=c",
		}},
		{`((identifier) @id (#not-match? @id "^[abc]$"))`, []string{
			"0: id=x", "0: id=y", "0: id=z",
		}},
		{`((identifier) @id (#any-of? @id "x" "z"))`, []string{"0: id=x", "0: id=z"}},
		{`((identifier) @id (#not-any-of? @id "x" "z" "c"))`, []string{
			"0: id=y", "0: id=a", "0: id=b",
		}},
		// the two captures of the inner expression have other texts
		{`(_ left: (identifier) @l right: (identifier) @r (#eq? @l @r))`, nil},
		{`(_ left: (identifier) @l right: (identifier) @r (#not-eq? @l @r))`, []string{"0: l=a r=b"}},
		// a predicate of Neovim is a general predicate, and it filters
		// nothing (D69)
		{`((identifier) @id (#lua-match? @id "^%a$") (#eq? @id "y"))`, []string{"0: id=y"}},
		// the patterns of a query each have their own predicates
		{`((identifier) @a (#eq? @a "x")) ((identifier) @b (#eq? @b "z"))`, []string{
			"0: a=x", "1: b=z",
		}},
	}
	for _, test := range tests {
		q := queryBindingNew(t, test.source)
		got := queryBindingMatches(q, NewQueryCursor())
		if !slices.Equal(got, test.want) {
			t.Errorf("%s:\ngot  %q\nwant %q", test.source, got, test.want)
		}
	}
}

func TestQueryBindingQuantifiedPredicates(t *testing.T) {
	// The pattern matches the identifiers x and y, a, b, and c and z, as
	// four matches: the children of the root and of the inner expression.
	p := "x y + a + b c z"
	tests := []struct {
		source string
		want   []string
	}{
		// eq?, not-eq?, match? and any-of? hold only when every node of the
		// capture holds them
		{`(_ (identifier)+ @ids (#eq? @ids "x")) @p`, nil},
		{`(_ (identifier)+ @ids (#not-eq? @ids "a")) @p`, []string{
			"0: p=" + p + " ids=x ids=y", "0: p=a + b ids=b", "0: p=" + p + " ids=c ids=z",
		}},
		{`(_ (identifier)+ @ids (#match? @ids "^[xyz]$")) @p`, []string{"0: p=" + p + " ids=x ids=y"}},
		{`(_ (identifier)+ @ids (#any-of? @ids "x" "y")) @p`, []string{"0: p=" + p + " ids=x ids=y"}},
		{`(_ (identifier)+ @ids (#not-any-of? @ids "a" "b")) @p`, []string{
			"0: p=" + p + " ids=x ids=y", "0: p=" + p + " ids=c ids=z",
		}},
		// A fault of the Rust binding, which the port keeps: an any-
		// predicate of eq? and match? returns true after its loop, so it
		// holds when no node of the capture holds it, and it keeps every
		// match.
		{`(_ (identifier)+ @ids (#any-eq? @ids "y")) @p`, []string{
			"0: p=" + p + " ids=x ids=y", "0: p=a + b ids=a", "0: p=a + b ids=b", "0: p=" + p + " ids=c ids=z",
		}},
		{`(_ (identifier)+ @ids (#any-match? @ids "^b$")) @p`, []string{
			"0: p=" + p + " ids=x ids=y", "0: p=a + b ids=a", "0: p=a + b ids=b", "0: p=" + p + " ids=c ids=z",
		}},
	}
	for _, test := range tests {
		q := queryBindingNew(t, test.source)
		got := queryBindingMatches(q, NewQueryCursor())
		if !slices.Equal(got, test.want) {
			t.Errorf("%s:\ngot  %q\nwant %q", test.source, got, test.want)
		}
	}
}

func TestQueryBindingCaptures(t *testing.T) {
	q := queryBindingNew(t, `((identifier) @id (#match? @id "^[xz]$")) "+" @plus`)
	tree := treeSample(q.ptr.language)
	var got []string
	for m, i := range NewQueryCursor().Captures(context.Background(), q, tree.RootNode(), queryBindingText) {
		c := m.Captures[i]
		got = append(got, fmt.Sprintf("%d %s=%s", m.PatternIndex, q.CaptureNames()[c.Index], c.Node.Text(queryBindingText)))
	}
	want := []string{"0 id=x", "1 plus=+", "0 id=z"}
	if !slices.Equal(got, want) {
		t.Errorf("Captures = %q, want %q", got, want)
	}

	// a break stops the sequence, and a second range runs the query again
	c := NewQueryCursor()
	for range c.Captures(context.Background(), q, tree.RootNode(), queryBindingText) {
		break
	}
	n := 0
	for range c.Captures(context.Background(), q, tree.RootNode(), queryBindingText) {
		n++
	}
	if n != 3 {
		t.Errorf("a second range gave %d captures, want 3", n)
	}
}

func TestQueryBindingRemove(t *testing.T) {
	// Pattern 0 matches a + b, with the captures l=a and r=b. Pattern 1
	// matches each identifier.
	q := queryBindingNew(t, `(_ left: (identifier) @l right: (identifier) @r) (identifier) @id`)
	tree := treeSample(q.ptr.language)
	captures := func(remove func(QueryMatch, QueryCapture) bool) []string {
		var got []string
		for m, i := range NewQueryCursor().Captures(context.Background(), q, tree.RootNode(), queryBindingText) {
			c := m.Captures[i]
			if remove(m, c) {
				m.Remove()
				continue
			}
			got = append(got, fmt.Sprintf("%d %s=%s", m.PatternIndex, q.CaptureNames()[c.Index], c.Node.Text(queryBindingText)))
		}
		return got
	}
	all := captures(func(QueryMatch, QueryCapture) bool { return false })
	want := []string{"1 id=x", "1 id=y", "0 l=a", "1 id=a", "0 r=b", "1 id=b", "1 id=c", "1 id=z"}
	if !slices.Equal(all, want) {
		t.Fatalf("Captures = %q, want %q", all, want)
	}
	// a removal while Captures runs takes out the rest of that match, and
	// no other match
	got := captures(func(m QueryMatch, c QueryCapture) bool {
		return m.PatternIndex == 0 && c.Node.Text(queryBindingText) == "a"
	})
	want = []string{"1 id=x", "1 id=y", "1 id=a", "1 id=b", "1 id=c", "1 id=z"}
	if !slices.Equal(got, want) {
		t.Errorf("Captures with a removal = %q, want %q", got, want)
	}

	// a match that Matches gives is finished, so its removal changes no
	// other match
	n := 0
	for m := range NewQueryCursor().Matches(context.Background(), q, tree.RootNode(), queryBindingText) {
		m.Remove()
		n++
	}
	if n != 7 {
		t.Errorf("Matches with a removal of each match gave %d matches, want 7", n)
	}
}

func TestQueryBindingCancel(t *testing.T) {
	l := testLanguage(15)
	pool := newSubtreePool(0)
	tree := newTree(repetition(&pool, l, 300), l, nil)
	src := []byte(strings.Repeat("x", 300))
	q := queryBindingNew(t, `(identifier) @id`)
	all := 0
	for range NewQueryCursor().Matches(context.Background(), q, tree.RootNode(), src) {
		all++
	}
	if all != 300 {
		t.Fatalf("the query gave %d matches, want 300", all)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := 0
	for range NewQueryCursor().Matches(ctx, q, tree.RootNode(), src) {
		n++
	}
	// the cursor reads the context once in each 100 operations, so it can
	// give a few matches before it stops
	if n >= all {
		t.Errorf("a canceled context gave all %d matches", n)
	}
}

func TestQueryBindingProperties(t *testing.T) {
	q := queryBindingNew(t, `((identifier) @id
  (#set! "k" "v")
  (#set! @id "flag")
  (#is? "local")
  (#is-not? @id "global" "x")
  (#other? @id "arg"))`)
	settings := q.PropertySettings(0)
	wantSettings := []QueryProperty{
		{Key: "k", Value: "v", HasValue: true, CaptureID: -1},
		{Key: "flag", CaptureID: 0},
	}
	if !slices.Equal(settings, wantSettings) {
		t.Errorf("PropertySettings = %+v, want %+v", settings, wantSettings)
	}
	predicates := q.PropertyPredicates(0)
	wantPredicates := []QueryPropertyPredicate{
		{Property: QueryProperty{Key: "local", CaptureID: -1}, Positive: true},
		{Property: QueryProperty{Key: "global", Value: "x", HasValue: true, CaptureID: 0}, Positive: false},
	}
	if !slices.Equal(predicates, wantPredicates) {
		t.Errorf("PropertyPredicates = %+v, want %+v", predicates, wantPredicates)
	}
	general := q.GeneralPredicates(0)
	if len(general) != 1 || general[0].Operator != "other?" ||
		!slices.Equal(general[0].Args, []QueryPredicateArg{{IsCapture: true, Capture: 0}, {Value: "arg"}}) {
		t.Errorf("GeneralPredicates = %+v", general)
	}
}

func TestQueryBindingPredicateErrors(t *testing.T) {
	l := testLanguage(15)
	tests := []struct {
		source  string
		row     int
		message string
	}{
		{`((identifier) @x (#eq? @x))`, 0, "Wrong number of arguments to #eq? predicate. Expected 2, got 1."},
		{`((identifier) @x (#eq? "a" @x))`, 0, `First argument to #eq? predicate must be a capture name. Got literal "a".`},
		{`((identifier) @x (#match? @x @x))`, 0, "Second argument to #match? predicate must be a literal. Got capture @x."},
		{`((identifier) @x (#match? @x "("))`, 0, "Invalid regex '('"},
		{`((identifier) @x (#any-of? @x @x))`, 0, "Arguments to #any-of? predicate must be literals. Got capture @x."},
		{`((identifier) @x (#set!))`, 0, "Wrong number of arguments to set! predicate. Expected 1 to 3, got 0."},
		{`((identifier) @x (#set! @x @x))`, 0, "Invalid arguments to set! predicate. Unexpected second capture name @x"},
		{`((identifier) @x (#set! @x))`, 0, "Invalid arguments to set! predicate. Missing key argument"},
		// the row is the row of the start of the pattern
		{"(identifier) @a\n\n((identifier) @x (#eq? @x))", 2, "Wrong number of arguments to #eq? predicate. Expected 2, got 1."},
	}
	for _, test := range tests {
		qerr := queryBindingError(t, l, test.source)
		if qerr.Kind != QueryErrorPredicate || qerr.Row != test.row || qerr.Message != test.message {
			t.Errorf("NewQuery(%q) = %+v, want the row %d and %q", test.source, qerr, test.row, test.message)
		}
		if want := fmt.Sprintf("Query error at %d:1. Invalid predicate: %s", test.row+1, test.message); qerr.Error() != want {
			t.Errorf("Error() = %q, want %q", qerr.Error(), want)
		}
	}
}

func TestQueryBindingCompileErrors(t *testing.T) {
	l := testLanguage(15)

	qerr := queryBindingError(t, l, "(identifier) @a\n(nope) @x")
	if qerr.Kind != QueryErrorNodeType || qerr.Message != `"nope"` || qerr.Row != 1 || qerr.Column != 1 {
		t.Errorf("an unknown node type gave %+v", qerr)
	}
	if want := `Query error at 2:2. Invalid node type "nope"`; qerr.Error() != want {
		t.Errorf("Error() = %q, want %q", qerr.Error(), want)
	}

	qerr = queryBindingError(t, l, `(identifier nope: (identifier))`)
	if qerr.Kind != QueryErrorField || qerr.Message != `"nope"` {
		t.Errorf("an unknown field gave %+v", qerr)
	}

	qerr = queryBindingError(t, l, `((identifier) @x (#eq? @y "a"))`)
	if qerr.Kind != QueryErrorCapture || qerr.Message != `"y"` {
		t.Errorf("an unknown capture gave %+v", qerr)
	}

	// a name in quotes ends at its closing quote
	qerr = queryBindingError(t, l, `("no such" @x)`)
	if qerr.Kind != QueryErrorNodeType || qerr.Message != `"no such"` {
		t.Errorf("an unknown anonymous node gave %+v", qerr)
	}

	qerr = queryBindingError(t, l, "(identifier) @a\n((identifier) @x")
	if qerr.Kind != QueryErrorSyntax {
		t.Fatalf("an unclosed pattern gave %+v", qerr)
	}
	if !strings.HasPrefix(qerr.Error(), "Query error at ") || !strings.Contains(qerr.Error(), "Invalid syntax:\n") {
		t.Errorf("Error() = %q", qerr.Error())
	}

	qerr = queryBindingError(t, l, "(identifier) @a )")
	if qerr.Kind != QueryErrorSyntax || qerr.Message != "(identifier) @a )\n                ^" {
		t.Errorf("a stray parenthesis gave %+v", qerr)
	}

	// a language that the runtime does not accept
	qerr = queryBindingError(t, testLanguage(12), `(identifier) @a`)
	if qerr.Kind != QueryErrorLanguage || qerr.Error() != "Incompatible language version 12. Expected minimum 13, maximum 15" {
		t.Errorf("an old language gave %+v, %q", qerr, qerr.Error())
	}
}

func TestQueryBindingMetadata(t *testing.T) {
	source := "(identifier) @a\n\"+\" @b @c"
	q := queryBindingNew(t, source)
	if got := q.PatternCount(); got != 2 {
		t.Errorf("PatternCount() = %d, want 2", got)
	}
	if got := q.CaptureNames(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("CaptureNames() = %q", got)
	}
	if i, ok := q.CaptureIndexForName("c"); !ok || i != 2 {
		t.Errorf("CaptureIndexForName(c) = %d, %t", i, ok)
	}
	if _, ok := q.CaptureIndexForName("d"); ok {
		t.Error("CaptureIndexForName(d) returned true")
	}
	if got := q.CaptureQuantifiers(1); !slices.Equal(got, []Quantifier{QuantifierZero, QuantifierOne, QuantifierOne}) {
		t.Errorf("CaptureQuantifiers(1) = %v", got)
	}
	if got := q.StartByteForPattern(1); got != strings.Index(source, `"+"`) {
		t.Errorf("StartByteForPattern(1) = %d", got)
	}
	if got := q.EndByteForPattern(0); got <= 0 || got > len(source) {
		t.Errorf("EndByteForPattern(0) = %d", got)
	}
	if !q.IsPatternRooted(0) || q.IsPatternNonLocal(0) {
		t.Error("the pattern 0 is not rooted and local")
	}
	_ = q.IsPatternGuaranteedAtStep(0)

	defer func() {
		if r := recover(); r != "Pattern index is 2 but the pattern count is 2" {
			t.Errorf("StartByteForPattern(2) panicked with %v", r)
		}
	}()
	q.StartByteForPattern(2)
}

func TestQueryBindingDisable(t *testing.T) {
	q := queryBindingNew(t, `(identifier) @a "+" @b`)
	q.DisablePattern(1)
	q.DisableCapture("a")
	got := queryBindingMatches(q, NewQueryCursor())
	if len(got) != 6 || got[0] != "0: " {
		t.Errorf("after DisablePattern(1) and DisableCapture(a), the matches are %q", got)
	}
}

func TestQueryBindingCursorSettings(t *testing.T) {
	q := queryBindingNew(t, `(identifier) @id`)
	c := NewQueryCursor()
	c.SetByteRange(5, 11)
	if got := queryBindingMatches(q, c); !slices.Equal(got, []string{"0: id=a", "0: id=b"}) {
		t.Errorf("SetByteRange(5, 11) gave %q", got)
	}
	c = NewQueryCursor()
	c.SetPointRange(Point{0, 12}, Point{0, 15})
	if got := queryBindingMatches(q, c); !slices.Equal(got, []string{"0: id=c", "0: id=z"}) {
		t.Errorf("SetPointRange gave %q", got)
	}
	c = NewQueryCursor()
	c.SetContainingByteRange(0, 3)
	if got := queryBindingMatches(q, c); !slices.Equal(got, []string{"0: id=x", "0: id=y"}) {
		t.Errorf("SetContainingByteRange(0, 3) gave %q", got)
	}
	c = NewQueryCursor()
	c.SetContainingPointRange(Point{0, 0}, Point{0, 1})
	if got := queryBindingMatches(q, c); !slices.Equal(got, []string{"0: id=x"}) {
		t.Errorf("SetContainingPointRange gave %q", got)
	}

	c = NewQueryCursor()
	c.SetMaxStartDepth(1)
	if got := len(queryBindingMatches(q, c)); got != 4 {
		t.Errorf("SetMaxStartDepth(1) gave %d matches, want the 4 children of the root", got)
	}
	c.SetMaxStartDepth(-1)
	if got := len(queryBindingMatches(q, c)); got != 6 {
		t.Errorf("SetMaxStartDepth(-1) gave %d matches, want 6", got)
	}

	c = NewQueryCursor()
	c.SetMatchLimit(1)
	if got := c.MatchLimit(); got != 1 {
		t.Errorf("MatchLimit() = %d", got)
	}
	queryBindingMatches(queryBindingNew(t, `(_ (identifier) @a (identifier) @b)`), c)
	if !c.DidExceedMatchLimit() {
		t.Error("a limit of 1 was not exceeded")
	}
}

func TestQueryBindingHelpers(t *testing.T) {
	for in, want := range map[string][]string{
		"":           nil,
		"a":          {"a"},
		"a\n":        {"a"},
		"a\r\nb\n\n": {"a", "b", ""},
		"a\rb":       {"a\rb"},
	} {
		if got := rustLines(in); !slices.Equal(got, want) {
			t.Errorf("rustLines(%q) = %q, want %q", in, got, want)
		}
	}
	steps := []queryPredicateStep{
		{typ: queryPredicateStepTypeString}, {typ: queryPredicateStepTypeDone},
		{typ: queryPredicateStepTypeDone},
		{typ: queryPredicateStepTypeCapture}, {typ: queryPredicateStepTypeDone},
	}
	var lens []int
	for p := range splitPredicateSteps(steps) {
		lens = append(lens, len(p))
	}
	if !slices.Equal(lens, []int{1, 0, 1, 0}) {
		t.Errorf("splitPredicateSteps gave slices of the lengths %v, want 1, 0, 1 and 0 as Rust does", lens)
	}
	for k, want := range map[textPredicateKind]string{
		textPredicateEqString: "eq string", textPredicateEqCapture: "eq capture",
		textPredicateMatchString: "match string", textPredicateAnyString: "any string",
		9: unknownName,
	} {
		if got := k.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
	if !isAlphanumeric('é') || !isAlphanumeric('7') || isAlphanumeric('-') {
		t.Error("isAlphanumeric is wrong")
	}
}

// TestQueryBindingRemoveZero checks that Remove on a zero QueryMatch does
// nothing (D94).
func TestQueryBindingRemoveZero(t *testing.T) {
	var m QueryMatch
	m.Remove()
}
