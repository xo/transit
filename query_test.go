package transit

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// queryNew compiles a query on the test language, and fails the test when
// the source has an error.
func queryNew(t *testing.T, source string) *query {
	t.Helper()
	q, offset, kind := newQuery(testLanguage(15), source)
	if q == nil || kind != QueryErrorNone || offset != 0 {
		t.Fatalf("newQuery(%q) = %t, %d, %v", source, q != nil, offset, kind)
	}
	return q
}

// queryCaptureString returns a capture as name=kind@start-end.
func queryCaptureString(q *query, capture queryCapture) string {
	return fmt.Sprintf("%s=%s@%d-%d",
		q.captureNameForID(capture.index),
		capture.node.Kind(),
		capture.node.StartByte(),
		capture.node.EndByte(),
	)
}

// queryMatchString returns a match as its pattern index and its captures.
func queryMatchString(q *query, m queryMatch) string {
	captures := make([]string, 0, len(m.captures))
	for _, capture := range m.captures {
		captures = append(captures, queryCaptureString(q, capture))
	}
	return fmt.Sprintf("%d: %s", m.patternIndex, strings.Join(captures, " "))
}

// queryMatches runs a query with a cursor on the root of treeSample, and
// returns each match as queryMatchString gives it.
func queryMatches(q *query, c *queryCursor) []string {
	tree := treeSample(q.language)
	c.exec(q, tree.RootNode())
	var out []string
	for {
		m, ok := c.nextMatch(context.Background())
		if !ok {
			return out
		}
		out = append(out, queryMatchString(q, m))
	}
}

// queryCaptures runs a query with a cursor on the root of treeSample, and
// returns each capture with the pattern index of its match.
func queryCaptures(q *query, c *queryCursor) []string {
	tree := treeSample(q.language)
	c.exec(q, tree.RootNode())
	var out []string
	for {
		m, index, ok := c.nextCapture(context.Background())
		if !ok {
			return out
		}
		out = append(out, fmt.Sprintf("%d: %s", m.patternIndex, queryCaptureString(q, m.captures[index])))
	}
}

// queryCheck compares the lines that a test got with the lines that it
// wants.
func queryCheck(t *testing.T, name string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\ngot:\n  %s\nwant:\n  %s", name, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestQueryEnumStrings(t *testing.T) {
	for q, want := range map[Quantifier]string{
		QuantifierZero:       "zero",
		QuantifierZeroOrOne:  "zero or one",
		QuantifierZeroOrMore: "zero or more",
		QuantifierOne:        "one",
		QuantifierOneOrMore:  "one or more",
		9:                    unknownName,
	} {
		if got := q.String(); got != want {
			t.Errorf("Quantifier(%d).String() = %q, want %q", q, got, want)
		}
	}
	for k, want := range map[QueryErrorKind]string{
		QueryErrorNone:      noneName,
		QueryErrorSyntax:    "syntax",
		QueryErrorNodeType:  "node type",
		QueryErrorField:     "field",
		QueryErrorCapture:   "capture",
		QueryErrorStructure: "structure",
		QueryErrorLanguage:  "language",
		QueryErrorPredicate: "predicate",
		parentDone:          unknownName,
	} {
		if got := k.String(); got != want {
			t.Errorf("QueryErrorKind(%d).String() = %q, want %q", k, got, want)
		}
	}
	for typ, want := range map[queryPredicateStepType]string{
		queryPredicateStepTypeDone:    "done",
		queryPredicateStepTypeCapture: "capture",
		queryPredicateStepTypeString:  "string",
		9:                             unknownName,
	} {
		if got := typ.String(); got != want {
			t.Errorf("queryPredicateStepType(%d).String() = %q, want %q", typ, got, want)
		}
	}
}

func TestQueryQuantifierArithmetic(t *testing.T) {
	const (
		z  = QuantifierZero
		zo = QuantifierZeroOrOne
		zm = QuantifierZeroOrMore
		o  = QuantifierOne
		om = QuantifierOneOrMore
	)
	all := []Quantifier{z, zo, zm, o, om}
	// Each row is the result for a left operand, and each column for a right
	// operand, in the order of all.
	tables := []struct {
		name string
		fn   func(Quantifier, Quantifier) Quantifier
		want [5][5]Quantifier
	}{
		{"mul", quantifierMul, [5][5]Quantifier{
			{z, z, z, z, z},
			{z, zo, zm, zo, zm},
			{z, zm, zm, zm, zm},
			{z, zo, zm, o, om},
			{z, zm, zm, om, om},
		}},
		{"join", quantifierJoin, [5][5]Quantifier{
			{z, zo, zm, zo, zm},
			{zo, zo, zm, zo, zm},
			{zm, zm, zm, zm, zm},
			{zo, zo, zm, o, om},
			{zm, zm, zm, om, om},
		}},
		{"add", quantifierAdd, [5][5]Quantifier{
			{z, zo, zm, o, om},
			{zo, zm, zm, om, om},
			{zm, zm, zm, om, om},
			{o, om, om, om, om},
			{om, om, om, om, om},
		}},
	}
	for _, table := range tables {
		for i, left := range all {
			for j, right := range all {
				if got := table.fn(left, right); got != table.want[i][j] {
					t.Errorf("%s(%v, %v) = %v, want %v", table.name, left, right, got, table.want[i][j])
				}
			}
		}
		// A value that is not a quantifier gives zero, as the end of the C
		// function does.
		if got := table.fn(9, o); got != z {
			t.Errorf("%s(9, one) = %v, want zero", table.name, got)
		}
	}
}

func TestQueryCaptureQuantifierList(t *testing.T) {
	var q captureQuantifierList
	q.addForID(2, QuantifierOne)
	if len(q) != 3 || q[2] != QuantifierOne || q[0] != QuantifierZero {
		t.Fatalf("addForID(2, one) = %v", q)
	}
	if q.forID(7) != QuantifierZero {
		t.Error("forID of an id past the end is not zero")
	}
	q.addAll(captureQuantifierList{QuantifierOne, QuantifierZero, QuantifierOne, QuantifierZeroOrOne})
	want := captureQuantifierList{QuantifierOne, QuantifierZero, QuantifierOneOrMore, QuantifierZeroOrOne}
	if fmt.Sprint(q) != fmt.Sprint(want) {
		t.Errorf("addAll = %v, want %v", q, want)
	}
	q.joinAll(captureQuantifierList{QuantifierOne})
	want = captureQuantifierList{QuantifierOne, QuantifierZero, QuantifierZeroOrMore, QuantifierZeroOrOne}
	if fmt.Sprint(q) != fmt.Sprint(want) {
		t.Errorf("joinAll = %v, want %v", q, want)
	}
	q.mul(QuantifierZeroOrMore)
	want = captureQuantifierList{QuantifierZeroOrMore, QuantifierZero, QuantifierZeroOrMore, QuantifierZeroOrMore}
	if fmt.Sprint(q) != fmt.Sprint(want) {
		t.Errorf("mul = %v, want %v", q, want)
	}
	other := captureQuantifierList{QuantifierOne}
	q.replace(other)
	other[0] = QuantifierZero
	if len(q) != 1 || q[0] != QuantifierOne {
		t.Errorf("replace = %v, want a copy of [one]", q)
	}
	q.clear()
	if len(q) != 0 {
		t.Errorf("clear left %v", q)
	}
}

func TestQueryStream(t *testing.T) {
	for c := range int32(128) {
		wantSpace := c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
		if iswspace(c) != wantSpace {
			t.Errorf("iswspace(%q) = %t", c, !wantSpace)
		}
		wantAlnum := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if iswalnum(c) != wantAlnum {
			t.Errorf("iswalnum(%q) = %t", c, !wantAlnum)
		}
	}
	// The C locale counts no character past ASCII, and the decoder error is
	// WEOF.
	for _, c := range []int32{0xa0, 0x3000, 'é', 'ж', decodeError} {
		if iswspace(c) || iswalnum(c) {
			t.Errorf("iswspace or iswalnum is true for %#x", c)
		}
	}

	s := newStream([]byte("  ; a comment\n\t(é\xff"))
	s.skipWhitespace()
	if s.next != '(' || s.offset() != 15 {
		t.Fatalf("after the comment, the stream is at %q, %d", s.next, s.offset())
	}
	s.advance()
	if s.next != 'é' || s.nextSize != 2 {
		t.Errorf("the stream decodes %q in %d bytes", s.next, s.nextSize)
	}
	s.advance()
	if s.next != decodeError || s.nextSize != 1 {
		t.Errorf("the stream decodes %q in %d bytes for a bad byte", s.next, s.nextSize)
	}
	if s.advance() || s.next != 0 || s.nextSize != 0 {
		t.Error("the stream does not stop at its end")
	}
	s.reset(15)
	if s.next != '(' {
		t.Errorf("reset(15) is at %q", s.next)
	}

	// A comment that runs to the end of the source ends the stream.
	s = newStream([]byte(";x"))
	s.skipWhitespace()
	if s.next != 0 || s.offset() != 2 {
		t.Errorf("a comment at the end leaves the stream at %q, %d", s.next, s.offset())
	}

	s = newStream([]byte("a-b_c.d?"))
	if !s.isIdentStart() {
		t.Error("a is not the start of an identifier")
	}
	s.scanIdentifier()
	if s.offset() != 7 || s.next != '?' {
		t.Errorf("scanIdentifier stops at %d, %q", s.offset(), s.next)
	}
}

func TestQueryNewErrors(t *testing.T) {
	tests := []struct {
		source string
		offset uint32
		kind   QueryErrorKind
	}{
		{`(identifier`, 11, QueryErrorSyntax},
		{`(identifier))`, 12, QueryErrorSyntax},
		{`(foo)`, 1, QueryErrorNodeType},
		{`(program_repeat1)`, 1, QueryErrorNodeType},
		{`"foo"`, 1, QueryErrorNodeType},
		{`"+`, 0, QueryErrorSyntax},
		{"\"+\n\"", 0, QueryErrorSyntax},
		{`(_ foo: (identifier))`, 3, QueryErrorField},
		{`(identifier !foo)`, 13, QueryErrorField},
		{`(identifier !)`, 13, QueryErrorSyntax},
		{`((identifier) @a (#eq? @b "x"))`, 24, QueryErrorCapture},
		{`((identifier) @a (#eq @a))`, 21, QueryErrorSyntax},
		{`((identifier) @a (#eq? @))`, 24, QueryErrorSyntax},
		{`((identifier) @a (#eq? @a "x`, 26, QueryErrorSyntax},
		{`((identifier) @a (#eq? @a ,))`, 26, QueryErrorSyntax},
		{`(expression (identifier))`, 0, QueryErrorStructure},
		{"(identifier)\n(expression (identifier))", 13, QueryErrorStructure},
		{`(_statement/alias_name)`, 0, QueryErrorStructure},
		{`(identifier/identifier)`, 0, QueryErrorStructure},
		{`(_statement/foo)`, 12, QueryErrorNodeType},
		{`(_statement/"foo")`, 12, QueryErrorNodeType},
		{`(_statement/,)`, 12, QueryErrorSyntax},
		{`(identifier) @`, 14, QueryErrorSyntax},
		{`(identifier) foo`, 13, QueryErrorSyntax},
		{`[]`, 1, QueryErrorSyntax},
		{`[(identifier)`, 13, QueryErrorSyntax},
		{`(_ . )`, 5, QueryErrorSyntax},
		{`((identifier) . )`, 14, QueryErrorSyntax},
		{`((identifier)`, 13, QueryErrorSyntax},
		{`(MISSING foo)`, 9, QueryErrorNodeType},
		{`(MISSING "foo")`, 10, QueryErrorNodeType},
		{`(MISSING ,)`, 9, QueryErrorSyntax},
		{`field: (identifier)`, 0, QueryErrorField},
		{`field (identifier)`, 0, QueryErrorSyntax},
		{`left: )`, 6, QueryErrorSyntax},
		{`()`, 1, QueryErrorSyntax},
		{`)`, 0, QueryErrorSyntax},
		{"\xff", 0, QueryErrorSyntax},
		{`,`, 0, QueryErrorSyntax},
	}
	l := testLanguage(15)
	for _, test := range tests {
		q, offset, kind := newQuery(l, test.source)
		if q != nil || offset != test.offset || kind != test.kind {
			t.Errorf("newQuery(%q) = %t, %d, %v, want an error at %d of the kind %v",
				test.source, q != nil, offset, kind, test.offset, test.kind)
		}
	}

	for _, l := range []*Language{nil, testLanguage(12), testLanguage(16)} {
		q, offset, kind := newQuery(l, `(identifier)`)
		if q != nil || offset != 0 || kind != QueryErrorLanguage {
			t.Errorf("newQuery with the language %v = %t, %d, %v", l, q != nil, offset, kind)
		}
	}
}

func TestQueryEmpty(t *testing.T) {
	for _, source := range []string{"", "  \n", "; only a comment"} {
		q := queryNew(t, source)
		if q.patternCount() != 0 || q.captureCount() != 0 || q.stringCount() != 0 {
			t.Errorf("the query %q has patterns, captures or strings", source)
		}
		if got := queryMatches(q, newQueryCursor()); len(got) != 0 {
			t.Errorf("the query %q matches %v", source, got)
		}
	}
}

func TestQuerySteps(t *testing.T) {
	q := queryNew(t, `(identifier)+ @ids`)
	if len(q.steps) != 3 {
		t.Fatalf("the query has %d steps, want 3", len(q.steps))
	}
	first, repeat, done := q.steps[0], q.steps[1], q.steps[2]
	if first.symbol != testSymIdentifier || first.captureIDs != [3]uint16{0, none, none} || first.alternativeIndex != none {
		t.Errorf("the first step is %+v", first)
	}
	if !repeat.isPassThrough || repeat.alternativeIndex != 0 || repeat.symbol != wildcardSymbol {
		t.Errorf("the repeat step is %+v", repeat)
	}
	if done.depth != patternDoneMarker || !done.rootPatternGuaranteed || !done.parentPatternGuaranteed {
		t.Errorf("the last step is %+v", done)
	}

	q = queryNew(t, `(identifier)* @ids`)
	if q.steps[0].alternativeIndex != 2 || !q.steps[0].alternativeIsSkip || q.steps[1].alternativeIndex != 0 {
		t.Errorf("the steps of * are %+v", q.steps)
	}

	q = queryNew(t, `(identifier)? @id`)
	if q.steps[0].alternativeIndex != 1 || !q.steps[0].alternativeIsSkip || q.steps[1].depth != patternDoneMarker {
		t.Errorf("the steps of ? are %+v", q.steps)
	}

	// Each branch of an alternation but the last has the next branch as its
	// alternative, and a dead end that jumps past the alternation.
	q = queryNew(t, `[(identifier) "+" (alias_name)] @x`)
	steps := q.steps
	if len(steps) != 6 {
		t.Fatalf("the alternation has %d steps, want 6", len(steps))
	}
	if steps[0].alternativeIndex != 2 || steps[2].alternativeIndex != 4 || steps[4].alternativeIndex != none {
		t.Errorf("the branches have the alternatives %d, %d, %d", steps[0].alternativeIndex, steps[2].alternativeIndex, steps[4].alternativeIndex)
	}
	if !steps[1].isDeadEnd || steps[1].alternativeIndex != 5 || !steps[3].isDeadEnd || steps[3].alternativeIndex != 5 {
		t.Errorf("the dead ends are %+v and %+v", steps[1], steps[3])
	}
	for _, i := range []int{0, 2, 4} {
		if steps[i].captureIDs[0] != 0 {
			t.Errorf("the branch at %d does not capture x", i)
		}
	}
	if len(q.patternMap) != 3 {
		t.Errorf("the alternation has %d entries in the pattern map, want 3", len(q.patternMap))
	}

	// A wildcard, named or not, and the anchors.
	q = queryNew(t, `(_ . (_) @first _ @anon .)`)
	steps = q.steps
	if !steps[0].isNamed || steps[0].symbol != wildcardSymbol || steps[0].depth != 0 {
		t.Errorf("the root is %+v", steps[0])
	}
	if !steps[1].isImmediate || !steps[1].isNamed || steps[1].depth != 1 {
		t.Errorf("the first child is %+v", steps[1])
	}
	if steps[2].isNamed || steps[2].isImmediate || !steps[2].isLastChild {
		t.Errorf("the last child is %+v", steps[2])
	}

	// A field names each branch of the alternation that follows it.
	q = queryNew(t, `(_ left: [(identifier) (alias_name)])`)
	if q.steps[1].field != 1 || q.steps[3].field != 1 {
		t.Errorf("the fields of the branches are %d and %d", q.steps[1].field, q.steps[3].field)
	}

	// The negated fields of a step are a list that ends with 0, and two steps
	// with the same list share it.
	q = queryNew(t, "(expression !left !right)\n(identifier !left !right)\n(identifier !right)")
	if fmt.Sprint(q.negatedFields) != "[0 1 2 0 2 0]" {
		t.Errorf("the negated fields are %v", q.negatedFields)
	}
	if q.steps[0].negatedFieldListID != 1 || q.steps[2].negatedFieldListID != 1 || q.steps[4].negatedFieldListID != 4 {
		t.Errorf("the lists of negated fields are %d, %d and %d", q.steps[0].negatedFieldListID, q.steps[2].negatedFieldListID, q.steps[4].negatedFieldListID)
	}

	// MISSING, a supertype and a subtype.
	q = queryNew(t, "(MISSING identifier)\n(MISSING \"+\")\n(MISSING)\n(_statement)\n(_statement/identifier)")
	for i, want := range []Symbol{testSymIdentifier, testSymPlus, wildcardSymbol} {
		if step := q.steps[2*i]; !step.isMissing || step.symbol != want {
			t.Errorf("the MISSING step %d is %+v", i, step)
		}
	}
	if step := q.steps[6]; step.symbol != wildcardSymbol || step.supertypeSymbol != testSymStatement {
		t.Errorf("the supertype step is %+v", step)
	}
	if step := q.steps[8]; step.symbol != testSymIdentifier || step.supertypeSymbol != testSymStatement {
		t.Errorf("the subtype step is %+v", step)
	}

	// A step keeps at most three captures.
	q = queryNew(t, `(identifier) @a @b @c @d`)
	if q.steps[0].captureIDs != [3]uint16{0, 1, 2} || q.captureCount() != 4 {
		t.Errorf("the captures of the step are %v of %d", q.steps[0].captureIDs, q.captureCount())
	}
}

func TestQueryPredicates(t *testing.T) {
	q := queryNew(t, `((identifier) @a (#eq? @a "x\n\t\\\"") (#set! key value) (.not-eq? @a "x\n\t\\\""))`+"\n"+`(identifier) @b`)
	type step struct {
		typ   queryPredicateStepType
		value string
	}
	var got []step
	for _, s := range q.predicatesForPattern(0) {
		value := ""
		switch s.typ {
		case queryPredicateStepTypeCapture:
			value = q.captureNameForID(s.valueID)
		case queryPredicateStepTypeString:
			value = q.stringValueForID(s.valueID)
		}
		got = append(got, step{s.typ, value})
	}
	want := []step{
		{queryPredicateStepTypeString, "eq?"},
		{queryPredicateStepTypeCapture, "a"},
		{queryPredicateStepTypeString, "x\n\t\\\""},
		{queryPredicateStepTypeDone, ""},
		{queryPredicateStepTypeString, "set!"},
		{queryPredicateStepTypeString, "key"},
		{queryPredicateStepTypeString, "value"},
		{queryPredicateStepTypeDone, ""},
		{queryPredicateStepTypeString, "not-eq?"},
		{queryPredicateStepTypeCapture, "a"},
		{queryPredicateStepTypeString, "x\n\t\\\""},
		{queryPredicateStepTypeDone, ""},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the predicate steps are\n%v\nwant\n%v", got, want)
	}
	// The strings are kept once each.
	if q.stringCount() != 6 {
		t.Errorf("the query has %d strings, want 6", q.stringCount())
	}
	if steps := q.predicatesForPattern(1); steps != nil {
		t.Errorf("the second pattern has the predicate steps %v", steps)
	}
	if steps := q.predicatesForPattern(0); cap(steps) != len(steps) {
		t.Error("the predicate steps have room past their length")
	}

	// The escape \0 is a NUL byte, and C compares the names of the table with
	// strncmp, which stops at the NUL. So two strings that differ only after a
	// NUL byte are one string.
	q = queryNew(t, `((identifier) @a (#eq? @a "p\0q" "p\0r" "\z"))`)
	steps := q.predicatesForPattern(0)
	if steps[2].valueID != steps[3].valueID || q.stringValueForID(steps[2].valueID) != "p\x00q" {
		t.Errorf("the strings with a NUL byte are %q and %q", q.stringValueForID(steps[2].valueID), q.stringValueForID(steps[3].valueID))
	}
	if got := q.stringValueForID(steps[4].valueID); got != "z" {
		t.Errorf("the escape \\z is %q, want z", got)
	}

	// A predicate adds no step, and its captures make the steps indefinite.
	q = queryNew(t, `((identifier) @a (#eq? @a "x"))`)
	if len(q.steps) != 2 {
		t.Errorf("the pattern with a predicate has %d steps, want 2", len(q.steps))
	}
}

func TestQueryPatternMetadata(t *testing.T) {
	source := "; comment\n(identifier) @a\n  ((identifier) (identifier)) @b\n[(identifier) \"+\"]\n(_ (identifier) @c)"
	q := queryNew(t, source)
	if q.patternCount() != 4 {
		t.Fatalf("the query has %d patterns, want 4", q.patternCount())
	}
	type pattern struct {
		start, end       uint32
		rooted, nonLocal bool
	}
	// A pattern ends after the space that follows it. The last pattern has
	// a wildcard parent, so its first step in the pattern map is at the
	// depth 1, and it is not rooted.
	want := []pattern{
		{10, 28, true, false},
		{28, 59, false, false},
		{59, 78, true, false},
		{78, 97, false, false},
	}
	texts := []string{
		`(identifier) @a`,
		`((identifier) (identifier)) @b`,
		`[(identifier) "+"]`,
		`(_ (identifier) @c)`,
	}
	for i, w := range want {
		index := uint32(i)
		got := pattern{
			q.startByteForPattern(index),
			q.endByteForPattern(index),
			q.isPatternRooted(index),
			q.isPatternNonLocal(index),
		}
		if got != w {
			t.Errorf("the pattern %d is %+v, want %+v", i, got, w)
		}
		if text := strings.TrimSpace(source[got.start:got.end]); text != texts[i] {
			t.Errorf("the text of the pattern %d is %q, want %q", i, text, texts[i])
		}
	}
	if q.isPatternNonLocal(9) {
		t.Error("a pattern that does not exist is not local")
	}

	// The test language fails the analysis of every pattern whose parent is
	// not a wildcard, so no step of the query is guaranteed, and the steps
	// that end the patterns are past the offsets of the steps.
	for offset := range uint32(len(source) + 2) {
		if q.isPatternGuaranteedAtStep(offset) {
			t.Errorf("isPatternGuaranteedAtStep(%d) = true", offset)
		}
	}

	if got, want := q.captureCount(), uint32(3); got != want {
		t.Errorf("captureCount() = %d, want %d", got, want)
	}
	for i, name := range []string{"a", "b", "c"} {
		if got := q.captureNameForID(uint32(i)); got != name {
			t.Errorf("captureNameForID(%d) = %q, want %q", i, got, name)
		}
	}
}

func TestQueryCaptureQuantifiers(t *testing.T) {
	q := queryNew(t, "((identifier) @a (identifier)? @b [(identifier) @c \"+\"] (identifier)* @d ((identifier) @e)+ (identifier) @a)\n"+
		"[(identifier) @a (alias_name) @a]\n"+
		"(_ left: (identifier)? @f)")
	tests := []struct {
		pattern uint32
		name    string
		want    Quantifier
	}{
		{0, "a", QuantifierOneOrMore},
		{0, "b", QuantifierZeroOrOne},
		{0, "c", QuantifierZeroOrOne},
		{0, "d", QuantifierZeroOrMore},
		{0, "e", QuantifierOneOrMore},
		{0, "f", QuantifierZero},
		{1, "a", QuantifierOne},
		{1, "b", QuantifierZero},
		{2, "f", QuantifierZeroOrOne},
	}
	ids := map[string]uint32{}
	for i := range q.captureCount() {
		ids[q.captureNameForID(i)] = i
	}
	for _, test := range tests {
		if got := q.captureQuantifierForID(test.pattern, ids[test.name]); got != test.want {
			t.Errorf("the quantifier of @%s in the pattern %d is %v, want %v", test.name, test.pattern, got, test.want)
		}
	}
}

func TestQueryMatches(t *testing.T) {
	tests := []struct {
		source string
		want   []string
	}{
		{`(identifier) @id`, []string{
			"0: id=identifier@0-1",
			"0: id=identifier@2-3",
			"0: id=identifier@6-7",
			"0: id=identifier@10-11",
			"0: id=identifier@12-13",
			"0: id=identifier@14-15",
		}},
		// The "+" of the inner expression is an alias.
		{`"+" @plus`, []string{"0: plus=+@4-5"}},
		{`(alias_name) @a`, []string{"0: a=alias_name@8-9"}},
		// The wildcard parent is skipped at the start, and the cursor
		// captures the visible parent when the child matches.
		{`(_ (identifier) @child) @parent`, []string{
			"0: parent=expression@0-15 child=identifier@0-1",
			"0: parent=expression@0-15 child=identifier@2-3",
			"0: parent=expression@6-11 child=identifier@6-7",
			"0: parent=expression@6-11 child=identifier@10-11",
			"0: parent=expression@0-15 child=identifier@12-13",
			"0: parent=expression@0-15 child=identifier@14-15",
		}},
		{`(_ left: (identifier) @l)`, []string{"0: l=identifier@6-7"}},
		{`(_ right: (identifier) @r)`, []string{"0: r=identifier@10-11"}},
		{`(expression !left) @e`, []string{"0: e=expression@0-15"}},
		{`(expression !right) @e`, []string{"0: e=expression@0-15"}},
		{`[(identifier) "+"] @x`, []string{
			"0: x=identifier@0-1",
			"0: x=identifier@2-3",
			"0: x=+@4-5",
			"0: x=identifier@6-7",
			"0: x=identifier@10-11",
			"0: x=identifier@12-13",
			"0: x=identifier@14-15",
		}},
		{`(_ . (identifier) @first) @p`, []string{
			"0: p=expression@0-15 first=identifier@0-1",
			"0: p=expression@6-11 first=identifier@6-7",
		}},
		{`(_ (identifier) @last .) @p`, []string{
			"0: p=expression@6-11 last=identifier@10-11",
			"0: p=expression@0-15 last=identifier@14-15",
		}},
		{`(_) @n`, []string{
			"0: n=expression@0-15",
			"0: n=identifier@0-1",
			"0: n=identifier@2-3",
			"0: n=expression@6-11",
			"0: n=identifier@6-7",
			"0: n=alias_name@8-9",
			"0: n=identifier@10-11",
			"0: n=identifier@12-13",
			"0: n=identifier@14-15",
		}},
		{`_ @n`, []string{
			"0: n=expression@0-15",
			"0: n=identifier@0-1",
			"0: n=identifier@2-3",
			"0: n=+@4-5",
			"0: n=expression@6-11",
			"0: n=identifier@6-7",
			"0: n=alias_name@8-9",
			"0: n=identifier@10-11",
			"0: n=identifier@12-13",
			"0: n=identifier@14-15",
		}},
		// Only "z" is inside the hidden _statement.
		{`(_statement) @s`, []string{"0: s=identifier@14-15"}},
		{`(_statement/identifier) @s`, []string{"0: s=identifier@14-15"}},
		{`((identifier) @a . (identifier) @b)`, []string{
			"0: a=identifier@0-1 b=identifier@2-3",
			"0: a=identifier@12-13 b=identifier@14-15",
		}},
		{`((identifier) @a (identifier) @b)`, []string{
			"0: a=identifier@0-1 b=identifier@2-3",
			"0: a=identifier@6-7 b=identifier@10-11",
			"0: a=identifier@0-1 b=identifier@12-13",
			"0: a=identifier@2-3 b=identifier@12-13",
			"0: a=identifier@0-1 b=identifier@14-15",
			"0: a=identifier@2-3 b=identifier@14-15",
			"0: a=identifier@12-13 b=identifier@14-15",
		}},
		{`(identifier)+ @ids`, []string{
			"0: ids=identifier@0-1 ids=identifier@2-3",
			"0: ids=identifier@6-7",
			"0: ids=identifier@10-11",
			"0: ids=identifier@12-13 ids=identifier@14-15",
		}},
		// The C runtime does not evaluate predicates.
		{`((identifier) @a (#eq? @a "x"))`, []string{
			"0: a=identifier@0-1",
			"0: a=identifier@2-3",
			"0: a=identifier@6-7",
			"0: a=identifier@10-11",
			"0: a=identifier@12-13",
			"0: a=identifier@14-15",
		}},
		{`(MISSING) @m`, nil},
		{"(identifier) @id\n\"+\" @plus", []string{
			"0: id=identifier@0-1",
			"0: id=identifier@2-3",
			"1: plus=+@4-5",
			"0: id=identifier@6-7",
			"0: id=identifier@10-11",
			"0: id=identifier@12-13",
			"0: id=identifier@14-15",
		}},
	}
	for _, test := range tests {
		q := queryNew(t, test.source)
		queryCheck(t, test.source, queryMatches(q, newQueryCursor()), test.want)
	}
}

func TestQueryCaptures(t *testing.T) {
	q := queryNew(t, "((identifier) @a (identifier) @b)")
	queryCheck(t, "the captures of two siblings", queryCaptures(q, newQueryCursor()), []string{
		"0: a=identifier@0-1",
		"0: a=identifier@0-1",
		"0: a=identifier@0-1",
		"0: b=identifier@2-3",
		"0: a=identifier@2-3",
		"0: a=identifier@2-3",
		"0: a=identifier@6-7",
		"0: b=identifier@10-11",
		"0: b=identifier@12-13",
		"0: b=identifier@12-13",
		"0: a=identifier@12-13",
		"0: b=identifier@14-15",
		"0: b=identifier@14-15",
		"0: b=identifier@14-15",
	})

	q = queryNew(t, "(identifier) @id\n\"+\" @plus\n(_ (identifier) @child) @parent")
	// The cursor gives a capture when no match in progress has an earlier
	// capture, so the parent of a later match comes after the captures of
	// an earlier match.
	queryCheck(t, "the captures of three patterns", queryCaptures(q, newQueryCursor()), []string{
		"0: id=identifier@0-1",
		"2: parent=expression@0-15",
		"2: child=identifier@0-1",
		"2: parent=expression@0-15",
		"0: id=identifier@2-3",
		"2: child=identifier@2-3",
		"1: plus=+@4-5",
		"0: id=identifier@6-7",
		"2: parent=expression@6-11",
		"2: child=identifier@6-7",
		"2: parent=expression@6-11",
		"0: id=identifier@10-11",
		"2: child=identifier@10-11",
		"2: parent=expression@0-15",
		"0: id=identifier@12-13",
		"2: child=identifier@12-13",
		"2: parent=expression@0-15",
		"0: id=identifier@14-15",
		"2: child=identifier@14-15",
	})
}

func TestQueryCursorRanges(t *testing.T) {
	c := newQueryCursor()
	if c.setByteRange(5, 4) || c.setPointRange(point{0, 5}, point{0, 4}) ||
		c.setContainingByteRange(5, 4) || c.setContainingPointRange(point{1, 0}, point{0, 9}) {
		t.Error("a range that ends before its start is valid")
	}
	if !c.setByteRange(5, 0) || c.includedRange.endByte != 1<<32-1 ||
		!c.setPointRange(point{0, 5}, point{0, 0}) || c.includedRange.endPoint != pointMax ||
		!c.setContainingByteRange(5, 0) || c.containingRange.endByte != 1<<32-1 ||
		!c.setContainingPointRange(point{0, 5}, point{0, 0}) || c.containingRange.endPoint != pointMax {
		t.Error("the end 0 is not the end of the text")
	}

	tests := []struct {
		name   string
		source string
		set    func(c *queryCursor)
		want   []string
	}{
		{"bytes", `(identifier) @a`, func(c *queryCursor) { c.setByteRange(6, 11) },
			[]string{"0: a=identifier@6-7", "0: a=identifier@10-11"}},
		{"points", `(identifier) @a`, func(c *queryCursor) { c.setPointRange(point{0, 6}, point{0, 11}) },
			[]string{"0: a=identifier@6-7", "0: a=identifier@10-11"}},
		// The range does not hold the root, but the root intersects it.
		{"bytes of every node", `(_) @n`, func(c *queryCursor) { c.setByteRange(6, 11) }, []string{
			"0: n=expression@0-15",
			"0: n=expression@6-11",
			"0: n=identifier@6-7",
			"0: n=alias_name@8-9",
			"0: n=identifier@10-11",
		}},
		// A containing range holds each node of a match.
		{"containing bytes", `(_) @n`, func(c *queryCursor) { c.setContainingByteRange(6, 11) }, []string{
			"0: n=expression@6-11",
			"0: n=identifier@6-7",
			"0: n=alias_name@8-9",
			"0: n=identifier@10-11",
		}},
		{"containing points", `(_) @n`, func(c *queryCursor) { c.setContainingPointRange(point{0, 6}, point{0, 11}) }, []string{
			"0: n=expression@6-11",
			"0: n=identifier@6-7",
			"0: n=alias_name@8-9",
			"0: n=identifier@10-11",
		}},
		// A pattern with two roots starts outside of the range.
		{"siblings in a range", `((identifier) @a (identifier) @b)`, func(c *queryCursor) { c.setByteRange(6, 11) }, []string{
			"0: a=identifier@6-7 b=identifier@10-11",
			"0: a=identifier@12-13 b=identifier@14-15",
		}},
		{"siblings in a containing range", `((identifier) @a (identifier) @b)`, func(c *queryCursor) { c.setContainingByteRange(6, 11) }, []string{
			"0: a=identifier@6-7 b=identifier@10-11",
		}},
	}
	for _, test := range tests {
		q := queryNew(t, test.source)
		c := newQueryCursor()
		test.set(c)
		queryCheck(t, test.name, queryMatches(q, c), test.want)
	}

	// The captures outside of the range are skipped.
	q := queryNew(t, `((identifier) @a (identifier) @b)`)
	c = newQueryCursor()
	c.setByteRange(6, 11)
	queryCheck(t, "the captures in a range", queryCaptures(q, c), []string{"0: a=identifier@6-7", "0: b=identifier@10-11"})
}

func TestQueryMaxStartDepth(t *testing.T) {
	q := queryNew(t, `(_) @n`)
	tests := []struct {
		depth uint32
		want  []string
	}{
		{0, []string{"0: n=expression@0-15"}},
		{1, []string{
			"0: n=expression@0-15",
			"0: n=identifier@0-1",
			"0: n=identifier@2-3",
			"0: n=expression@6-11",
			"0: n=identifier@12-13",
			"0: n=identifier@14-15",
		}},
	}
	for _, test := range tests {
		c := newQueryCursor()
		c.setMaxStartDepth(test.depth)
		queryCheck(t, fmt.Sprintf("the depth %d", test.depth), queryMatches(q, c), test.want)
	}

	// The first step of this pattern in the pattern map is the child, at the
	// depth 1. The cursor does not descend into a node at the limit when no
	// match in progress needs it, so it does not reach the children of the
	// inner expression.
	q = queryNew(t, `(_ (identifier) @child) @parent`)
	c := newQueryCursor()
	c.setMaxStartDepth(1)
	queryCheck(t, "the parents at the depth 1", queryMatches(q, c), []string{
		"0: parent=expression@0-15 child=identifier@0-1",
		"0: parent=expression@0-15 child=identifier@2-3",
		"0: parent=expression@0-15 child=identifier@12-13",
		"0: parent=expression@0-15 child=identifier@14-15",
	})
}

func TestQueryMatchLimit(t *testing.T) {
	c := newQueryCursor()
	if c.matchLimit() != 1<<32-1 {
		t.Errorf("the match limit is %d", c.matchLimit())
	}
	q := queryNew(t, `((identifier) @a (identifier) @b)`)
	queryMatches(q, c)
	if c.didExceedMatchLimit() {
		t.Error("the cursor exceeded no limit")
	}

	// The cursor keeps the capture lists of the earlier run, and it uses
	// each of them again before it looks at the limit, as in C.
	c.setMatchLimit(1)
	if c.matchLimit() != 1 {
		t.Errorf("the match limit is %d, want 1", c.matchLimit())
	}
	if got := queryMatches(q, c); len(got) != 7 || c.didExceedMatchLimit() {
		t.Errorf("a cursor with lists gives %d matches and exceeded %t", len(got), c.didExceedMatchLimit())
	}

	// With one list, the cursor drops the match that captured the earliest
	// node.
	c = newQueryCursor()
	c.setMatchLimit(1)
	queryCheck(t, "the matches with one capture list", queryMatches(q, c), []string{
		"0: a=identifier@0-1 b=identifier@2-3",
		"0: a=identifier@6-7 b=identifier@10-11",
		"0: a=identifier@12-13 b=identifier@14-15",
	})
	if !c.didExceedMatchLimit() {
		t.Error("the cursor did not exceed the limit of 1")
	}

	// exec clears the flag.
	c.exec(q, treeSample(q.language).RootNode())
	if c.didExceedMatchLimit() {
		t.Error("exec did not clear the flag")
	}

	// A match of one node releases its list before the next match.
	q = queryNew(t, `(identifier) @a`)
	c = newQueryCursor()
	c.setMatchLimit(1)
	if got := queryMatches(q, c); len(got) != 6 || c.didExceedMatchLimit() {
		t.Errorf("one list gives %d matches and exceeded %t", len(got), c.didExceedMatchLimit())
	}
}

func TestQueryMatchCapturesAlias(t *testing.T) {
	q := queryNew(t, `(identifier) @a`)
	c := newQueryCursor()
	tree := treeSample(q.language)
	c.exec(q, tree.RootNode())
	first, _ := c.nextMatch(context.Background())
	if len(first.captures) != 1 || cap(first.captures) != 1 {
		t.Fatalf("the first match has %d captures in %d", len(first.captures), cap(first.captures))
	}
	// The next match takes the same list again, as in C.
	second, _ := c.nextMatch(context.Background())
	if &first.captures[0] != &second.captures[0] || first.captures[0].node.StartByte() != 2 {
		t.Error("the matches do not share the capture list of the cursor")
	}
}

func TestQueryRemoveMatch(t *testing.T) {
	q := queryNew(t, `(_ (identifier) @child) @parent`)
	c := newQueryCursor()
	tree := treeSample(q.language)
	c.exec(q, tree.RootNode())
	ctx := context.Background()
	m, index, ok := c.nextCapture(ctx)
	if !ok || m.id != 0 || index != 0 {
		t.Fatalf("the first capture is %v %d %t", m, index, ok)
	}
	// A match that the cursor does not know changes nothing.
	c.removeMatch(99)
	c.removeMatch(m.id)
	m, index, ok = c.nextCapture(ctx)
	if !ok || m.id != 1 || index != 0 || queryCaptureString(q, m.captures[index]) != "parent=expression@0-15" {
		t.Fatalf("after the removal, the capture is %v %d %t", m, index, ok)
	}
	m, index, ok = c.nextCapture(ctx)
	if !ok || m.id != 1 || index != 1 || queryCaptureString(q, m.captures[index]) != "child=identifier@2-3" {
		t.Fatalf("the next capture is %v %d %t", m, index, ok)
	}

	// removeMatch also removes a state that is in progress.
	q = queryNew(t, `((identifier) @a . (identifier) @b)`)
	c.exec(q, tree.RootNode())
	c.states = append(c.states, queryState{id: 7, captureListID: captureListNone})
	c.removeMatch(7)
	if len(c.states) != 0 {
		t.Errorf("removeMatch left %d states", len(c.states))
	}

	// removeMatch finds a finished state in the heap of nextCapture.
	q = queryNew(t, "(identifier) @a\n(identifier) @b")
	c.exec(q, tree.RootNode())
	m, _, _ = c.nextCapture(ctx)
	if m.patternIndex != 0 || len(c.finishedStates) != 2 {
		t.Fatalf("the first capture is of the pattern %d, with %d finished states", m.patternIndex, len(c.finishedStates))
	}
	c.removeMatch(m.id)
	m, _, _ = c.nextCapture(ctx)
	if m.patternIndex != 1 || queryCaptureString(q, m.captures[0]) != "b=identifier@0-1" {
		t.Errorf("after the removal, the capture is of the pattern %d: %s", m.patternIndex, queryCaptureString(q, m.captures[0]))
	}
}

func TestQueryDisable(t *testing.T) {
	q := queryNew(t, "(identifier) @a @b\n\"+\" @a")
	q.disableCapture("a")
	q.disableCapture("nothing")
	queryCheck(t, "the matches without @a", queryMatches(q, newQueryCursor()), []string{
		"0: b=identifier@0-1",
		"0: b=identifier@2-3",
		"1: ",
		"0: b=identifier@6-7",
		"0: b=identifier@10-11",
		"0: b=identifier@12-13",
		"0: b=identifier@14-15",
	})
	if q.steps[0].captureIDs != [3]uint16{1, none, none} {
		t.Errorf("the captures of the first step are %v", q.steps[0].captureIDs)
	}

	q = queryNew(t, "(identifier) @a\n[\"+\" (alias_name)] @b")
	q.disablePattern(1)
	q.disablePattern(9)
	if len(q.patternMap) != 1 {
		t.Errorf("the pattern map has %d entries, want 1", len(q.patternMap))
	}
	if got := queryMatches(q, newQueryCursor()); len(got) != 6 {
		t.Errorf("the query without the pattern 1 has %d matches, want 6", len(got))
	}
}

func TestQueryCancel(t *testing.T) {
	// A tree of 300 identifiers and a "+" at the end.
	l := testLanguage(15)
	pool := newSubtreePool()
	var children subtreeArray
	for range 300 {
		children = append(children, leaf(&pool, l, testSymIdentifier, 1, 1))
	}
	children = append(children, leaf(&pool, l, testSymPlus, 1, 1))
	tree := newTree(newNode(&pool, testSymExpression, children, 0, l), l, nil)

	q := queryNew(t, `"+" @plus`)
	c := newQueryCursor()
	c.exec(q, tree.RootNode())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, ok := c.nextMatch(ctx); ok {
		t.Fatalf("a canceled context gives the match %v", m)
	}
	if _, _, ok := c.nextCapture(ctx); ok {
		t.Fatal("a canceled context gives a capture")
	}
	// The cursor keeps its place, as C keeps it when the callback stops it.
	m, ok := c.nextMatch(context.Background())
	if !ok || queryMatchString(q, m) != "0: plus=+@601-602" {
		t.Errorf("after the cancel, the match is %v %t", m, ok)
	}
	if _, ok := c.nextMatch(context.Background()); ok {
		t.Error("the cursor gives a second match")
	}

	// The cursor reads the context once in 100 operations, so a small tree
	// gives its match before the cursor reads it.
	q = queryNew(t, `(identifier) @a`)
	c.exec(q, treeSample(l).RootNode())
	if _, ok := c.nextMatch(ctx); !ok {
		t.Error("the first match of a small tree needs the context")
	}
}

func TestQueryPatternMapSearch(t *testing.T) {
	q := queryNew(t, "(expression) @e\n(_) @w\n(identifier) @i\n\"+\" @p\n(identifier) @j")
	if q.wildcardRootPatternCount != 1 {
		t.Fatalf("the query has %d wildcard roots, want 1", q.wildcardRootPatternCount)
	}
	var got []string
	for _, entry := range q.patternMap {
		got = append(got, fmt.Sprintf("%d:%d", q.steps[entry.stepIndex].symbol, entry.patternIndex))
	}
	if want := "0:1 1:3 2:2 2:4 3:0"; strings.Join(got, " ") != want {
		t.Errorf("the pattern map is %s, want %s", strings.Join(got, " "), want)
	}
	for _, test := range []struct {
		symbol Symbol
		index  uint32
		found  bool
	}{
		{testSymPlus, 1, true},
		{testSymIdentifier, 2, true},
		{testSymExpression, 4, true},
		{testSymAlias, 5, false},
		{testSymEnd, 1, false},
	} {
		index, found := q.patternMapSearch(test.symbol)
		if index != test.index || found != test.found {
			t.Errorf("patternMapSearch(%d) = %d, %t, want %d, %t", test.symbol, index, found, test.index, test.found)
		}
	}
}

func TestQueryArraySearchSorted(t *testing.T) {
	values := []uint16{1, 3, 3, 3, 7}
	for _, test := range []struct {
		needle uint16
		index  uint32
		exists bool
	}{
		{0, 0, false},
		{1, 0, true},
		{2, 1, false},
		{3, 3, true},
		{5, 4, false},
		{7, 4, true},
		{9, 5, false},
	} {
		index, exists := arraySearchSortedBy(values, test.needle)
		if index != test.index || exists != test.exists {
			t.Errorf("arraySearchSortedBy(%d) = %d, %t, want %d, %t", test.needle, index, exists, test.index, test.exists)
		}
	}
	if index, exists := arraySearchSortedBy([]uint16(nil), 3); index != 0 || exists {
		t.Error("the search of an empty array finds the needle")
	}
	var set []uint16
	for _, v := range []uint16{5, 1, 5, 3, 1} {
		arrayInsertSortedBy(&set, v)
	}
	if fmt.Sprint(set) != "[1 3 5]" {
		t.Errorf("the sorted set is %v", set)
	}
}
