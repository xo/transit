package cgrammar

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xo/transit"
)

// This file holds the helpers of upstream_query_test.go. It ports
// crates/cli/src/tests/helpers/query_helpers.rs of upstream (D35), and the
// parts of the other helpers of upstream that query_test.rs uses:
// get_language, get_test_language, get_test_fixture_language,
// generate_parser, and the functions indoc! and unindent of the Rust crates
// indoc and unindent.

// uqCaptures is a list of captures. Each capture is a capture name and the
// text of its node. It is the Vec<(&str, &str)> of the Rust tests.
type uqCaptures = [][2]string

// uqMatch is a match as collect_matches of upstream gives it: the index of
// its pattern and its captures.
type uqMatch struct {
	Pattern  int
	Captures uqCaptures
}

// uqLanguages holds the language of each fixture grammar that uqLanguage
// loaded. A Language is safe to share between goroutines (D52).
var uqLanguages sync.Map

// uqLanguage returns the language of a fixture grammar. It is get_language
// of upstream. It skips the test when the cache or the checkout of upstream
// is missing.
func uqLanguage(t *testing.T, name string) *transit.Language {
	t.Helper()
	if l, ok := uqLanguages.Load(name); ok {
		language, _ := l.(*transit.Language)
		return language
	}
	language := fixtureGrammar(t, name).Language
	uqLanguages.Store(name, language)
	return language
}

// uqTestLanguage generates the parser of a grammar from the text of its
// grammar.json, builds it and returns its language. It is generate_parser
// and get_test_language of upstream.
func uqTestLanguage(t *testing.T, grammarJSON string) *transit.Language {
	t.Helper()
	_, cache := setup(t)
	var g struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(grammarJSON), &g); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), g.Name)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "grammar.json"), []byte(grammarJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	so, err := BuildGrammar(t.Context(), dir, cache)
	if err != nil {
		t.Fatal(err)
	}
	grammar, err := Load(so, g.Name)
	if err != nil {
		t.Fatal(err)
	}
	return grammar.Language
}

// uqTestFixtureLanguage returns the language of a test grammar of
// test/fixtures/test_grammars of upstream. It is get_test_fixture_language
// of upstream. It reads the grammar.json of the grammar from
// generate/testdata, which the golden harness writes from the grammar.js of
// upstream.
func uqTestFixtureLanguage(t *testing.T, name string) *transit.Language {
	t.Helper()
	root, _ := setup(t)
	b, err := os.ReadFile(filepath.Join(root, "generate", "testdata", name, "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	return uqTestLanguage(t, string(b))
}

// uqNewQuery compiles a query, and stops the test when the query does not
// compile. It is Query::new(...).unwrap().
func uqNewQuery(t *testing.T, language *transit.Language, source string) *transit.Query {
	t.Helper()
	q, err := transit.NewQuery(language, source)
	if err != nil {
		t.Fatalf("NewQuery(%q): %v", source, err)
	}
	return q
}

// uqQueryError compiles a query that must fail, and returns its error. It
// is Query::new(...).unwrap_err().
func uqQueryError(t *testing.T, language *transit.Language, source string) transit.QueryError {
	t.Helper()
	_, err := transit.NewQuery(language, source)
	qe, ok := errors.AsType[*transit.QueryError](err)
	if !ok {
		t.Fatalf("NewQuery(%q) returned the error %v, want a *QueryError", source, err)
	}
	return *qe
}

// uqCheckQueryError compiles a query that must fail, and compares its error
// with want.
func uqCheckQueryError(t *testing.T, language *transit.Language, source string, want transit.QueryError) {
	t.Helper()
	if got := uqQueryError(t, language, source); got != want {
		t.Errorf("NewQuery(%q) returned\n%#v\nwant\n%#v", source, got, want)
	}
}

// uqCheckQueryErrorMessage compiles a query that must fail, and compares the
// message of its error with want.
func uqCheckQueryErrorMessage(t *testing.T, language *transit.Language, source, want string) {
	t.Helper()
	if got := uqQueryError(t, language, source).Message; got != want {
		t.Errorf("NewQuery(%q) returned the message\n%s\nwant\n%s", source, got, want)
	}
}

// uqLines joins lines with newlines. It is [...].join("\n").
func uqLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

// uqParser returns a parser of a language.
func uqParser(t *testing.T, language *transit.Language) *transit.Parser {
	t.Helper()
	p := transit.NewParser()
	if err := p.SetLanguage(language); err != nil {
		t.Fatal(err)
	}
	return p
}

// uqParse parses source with a new parser of a language.
func uqParse(t *testing.T, language *transit.Language, source string) *transit.Tree {
	t.Helper()
	tree, err := uqParser(t, language).Parse(t.Context(), []byte(source), nil)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// uqAssertQueryMatches parses source, runs the query on the tree, and
// compares the matches with want. It is assert_query_matches of upstream.
func uqAssertQueryMatches(t *testing.T, language *transit.Language, q *transit.Query, source string, want []uqMatch) {
	t.Helper()
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), q, tree.RootNode(), []byte(source))
	uqCheckMatches(t, uqCollectMatches(matches, q, source), want)
	if cursor.DidExceedMatchLimit() {
		t.Error("DidExceedMatchLimit() = true, want false")
	}
}

// uqCollectMatches runs a sequence of matches, and returns each match with
// the text of its captures. It is collect_matches of upstream.
func uqCollectMatches(matches iter.Seq[transit.QueryMatch], q *transit.Query, source string) []uqMatch {
	var result []uqMatch
	for m := range matches {
		result = append(result, uqMatch{m.PatternIndex, uqFormatCaptures(m.Captures, q, source)})
	}
	return result
}

// uqCollectCaptures runs a sequence of captures, and returns the name and
// the text of each capture. It is collect_captures of upstream.
func uqCollectCaptures(captures iter.Seq2[transit.QueryMatch, int], q *transit.Query, source string) uqCaptures {
	var result uqCaptures
	for m, i := range captures {
		result = append(result, uqFormatCaptures(m.Captures[i:i+1], q, source)...)
	}
	return result
}

// uqFormatCaptures returns the name and the text of each capture. It is
// format_captures of upstream.
func uqFormatCaptures(captures []transit.QueryCapture, q *transit.Query, source string) uqCaptures {
	result := uqCaptures{}
	for _, c := range captures {
		result = append(result, [2]string{q.CaptureNames()[c.Index], c.Node.Text([]byte(source))})
	}
	return result
}

// uqEqualMatches reports whether two lists of matches are the same. An
// empty list and nil are the same.
func uqEqualMatches(a, b []uqMatch) bool {
	return slices.EqualFunc(a, b, func(x, y uqMatch) bool {
		return x.Pattern == y.Pattern && slices.Equal(x.Captures, y.Captures)
	})
}

// uqCheckMatches compares a list of matches with want.
func uqCheckMatches(t *testing.T, got, want []uqMatch) {
	t.Helper()
	if !uqEqualMatches(got, want) {
		t.Errorf("the matches are\n%s\nwant\n%s", uqFormatMatches(got), uqFormatMatches(want))
	}
}

// uqCheckCaptures compares a list of captures with want. An empty list and
// nil are the same.
func uqCheckCaptures(t *testing.T, got, want uqCaptures) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("the captures are\n%q\nwant\n%q", got, want)
	}
}

// uqFormatMatches writes a list of matches, one match on each line.
func uqFormatMatches(matches []uqMatch) string {
	var b strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&b, "  (%d, %q)\n", m.Pattern, m.Captures)
	}
	if len(matches) == 0 {
		b.WriteString("  (none)\n")
	}
	return b.String()
}

// uqUnindent removes the largest indent that every line after the first
// has, where a line of only spaces and tabs does not count. It removes the
// first line when it is empty. It is unindent of the Rust crate unindent,
// and indoc! of the Rust crate indoc does the same.
func uqUnindent(s string) string {
	ignoreFirstLine := strings.HasPrefix(s, "\n") || strings.HasPrefix(s, "\r\n")
	body := s
	if strings.HasPrefix(body, "\r\n") {
		body = body[1:]
	}
	lines := strings.Split(body, "\n")
	spaces := -1
	for _, line := range lines[1:] {
		if n := strings.IndexFunc(line, func(r rune) bool { return r != ' ' && r != '\t' }); n >= 0 && (spaces < 0 || n < spaces) {
			spaces = n
		}
	}
	spaces = max(spaces, 0)
	var b strings.Builder
	for i, line := range lines {
		if i > 1 || (i == 1 && !ignoreFirstLine) {
			b.WriteByte('\n')
		}
		switch {
		case i == 0:
			// Do not un-indent anything on same line as opening quote
			b.WriteString(line)
		case len(line) > spaces:
			// Whitespace-only lines may have fewer than the number of spaces
			// being removed
			b.WriteString(line[spaces:])
		}
	}
	return b.String()
}

// uqPattern is a pattern that test_query_random builds from a tree, and
// matches by walking the tree. It is Pattern of query_helpers.rs.
type uqPattern struct {
	// kind is the kind of the node, "_" for a wildcard, or "" for a group
	// of sibling patterns, which is None in Rust.
	kind  string
	named bool
	// field and capture are "" for none.
	field    string
	capture  string
	children []*uqPattern
}

// uqNodeCapture is a capture of a uqNodeMatch.
type uqNodeCapture struct {
	name string
	node transit.Node
}

// uqNodeMatch is a match of a uqPattern, or a match that
// QueryCursor.Matches gives. It is Match of query_helpers.rs.
type uqNodeMatch struct {
	captures []uqNodeCapture
	lastNode transit.Node
	hasLast  bool
}

// uqCaptureNames are CAPTURE_NAMES of query_helpers.rs.
var uqCaptureNames = []string{
	"one", "two", "three", "four", "five", "six", "seven", "eight",
}

// uqRandomBool returns true with the probability p. It is random_bool of
// the Rust crate rand.
func uqRandomBool(rng *rand.Rand, p float64) bool {
	return rng.Float64() < p
}

// uqRandomPatternInTree builds a random pattern that matches a node of a
// tree, and returns it with the range of points that it covers. It is
// Pattern::random_pattern_in_tree.
func uqRandomPatternInTree(tree *transit.Tree, rng *rand.Rand) (*uqPattern, transit.Point, transit.Point) {
	cursor := tree.Walk()

	// Descend to the node at a random byte offset and depth.
	maxDepth := 0
	byteOffset := rng.IntN(cursor.Node().EndByte())
	for {
		if _, ok := cursor.GotoFirstChildForByte(byteOffset); !ok {
			break
		}
		maxDepth++
	}
	depth := rng.IntN(maxDepth + 1)
	for range depth {
		cursor.GotoParent()
	}

	// Build a pattern that matches that node.
	// Sometimes include subsequent siblings of the node.
	patternStart := cursor.Node().StartPoint()
	roots := []*uqPattern{uqRandomPatternForNode(cursor, rng)}
	for len(roots) < 5 && cursor.GotoNextSibling() {
		if uqRandomBool(rng, 0.2) {
			roots = append(roots, uqRandomPatternForNode(cursor, rng))
		}
	}
	patternEnd := cursor.Node().EndPoint()

	pattern := &uqPattern{named: true, children: roots}

	if len(pattern.children) == 1 ||
		// In a parenthesized list of sibling patterns, the first
		// sibling can't be an anonymous `_` wildcard.
		(pattern.children[0].kind == "_" && !pattern.children[0].named) {
		pattern = pattern.children[len(pattern.children)-1]
	} else {
		// In a parenthesized list of sibling patterns, the first
		// sibling can't have a field name.
		pattern.children[0].field = ""
	}

	return pattern, patternStart, patternEnd
}

// uqRandomPatternForNode builds a random pattern that matches the node of
// the cursor. It is Pattern::random_pattern_for_node.
func uqRandomPatternForNode(cursor *transit.TreeCursor, rng *rand.Rand) *uqPattern {
	node := cursor.Node()

	// Sometimes specify the node's type, sometimes use a wildcard.
	var kind string
	var named bool
	if uqRandomBool(rng, 0.9) {
		kind, named = node.Kind(), node.IsNamed()
	} else {
		kind, named = "_", node.IsNamed() && uqRandomBool(rng, 0.8)
	}

	// Sometimes specify the node's field.
	var field string
	if uqRandomBool(rng, 0.75) {
		field = cursor.FieldName()
	}

	// Sometimes capture the node.
	var capture string
	if uqRandomBool(rng, 0.7) {
		capture = uqCaptureNames[rng.IntN(len(uqCaptureNames))]
	}

	// Walk the children and include child patterns for some of them.
	var children []*uqPattern
	if named && cursor.GotoFirstChild() {
		maxChildren := rng.IntN(4)
		for cursor.GotoNextSibling() {
			if uqRandomBool(rng, 0.6) {
				childAST := uqRandomPatternForNode(cursor, rng)
				children = append(children, childAST)
				if len(children) >= maxChildren {
					break
				}
			}
		}
		cursor.GotoParent()
	}

	return &uqPattern{
		kind:     kind,
		named:    named,
		field:    field,
		capture:  capture,
		children: children,
	}
}

// writeTo writes the pattern as the text of a query. It is
// Pattern::write_to_string.
func (p *uqPattern) writeTo(b *strings.Builder, indent int) {
	if p.field != "" {
		fmt.Fprintf(b, "%s: ", p.field)
	}

	switch {
	case p.named:
		b.WriteByte('(')
		hasContents := false
		if p.kind != "" {
			b.WriteString(p.kind)
			hasContents = true
		}
		for _, child := range p.children {
			indent := indent + 2
			if hasContents {
				b.WriteByte('\n')
				b.WriteString(strings.Repeat(" ", indent))
			}
			child.writeTo(b, indent)
			hasContents = true
		}
		b.WriteByte(')')
	case p.kind == "_":
		b.WriteByte('_')
	default:
		fmt.Fprintf(b, "\"%s\"", strings.ReplaceAll(p.kind, "\"", "\\\""))
	}

	if p.capture != "" {
		fmt.Fprintf(b, " @%s", p.capture)
	}
}

// String returns the pattern as the text of a query. It is the Display of
// Pattern.
func (p *uqPattern) String() string {
	var b strings.Builder
	p.writeTo(&b, 0)
	return b.String()
}

// matchesInTree returns the matches of the pattern in a tree. It is
// Pattern::matches_in_tree.
func (p *uqPattern) matchesInTree(tree *transit.Tree) []uqNodeMatch {
	var matches []uqNodeMatch

	// Compute the matches naively: walk the tree and
	// retry the entire pattern for each node.
	cursor := tree.Walk()
	ascending := false
	for {
		if ascending {
			if cursor.GotoNextSibling() {
				ascending = false
			} else if !cursor.GotoParent() {
				break
			}
		} else {
			matchesHere := p.matchNode(cursor)
			matches = append(matches, matchesHere...)
			if !cursor.GotoFirstChild() {
				ascending = true
			}
		}
	}

	slices.SortFunc(matches, uqCompareMatches)
	for i := range matches {
		matches[i].lastNode, matches[i].hasLast = transit.Node{}, false
	}
	return slices.CompactFunc(matches, uqEqualNodeMatches)
}

// matchNode returns the matches of the pattern at the node of the cursor.
// It is Pattern::match_node.
func (p *uqPattern) matchNode(cursor *transit.TreeCursor) []uqNodeMatch {
	node := cursor.Node()

	// If a kind is specified, check that it matches the node.
	if p.kind != "" {
		if p.kind == "_" {
			if p.named && !node.IsNamed() {
				return nil
			}
		} else if p.kind != node.Kind() || p.named != node.IsNamed() {
			return nil
		}
	}

	// If a field is specified, check that it matches the node.
	if p.field != "" && cursor.FieldName() != p.field {
		return nil
	}

	// Create a match for the current node.
	mat := uqNodeMatch{lastNode: node, hasLast: true}
	if p.capture != "" {
		mat.captures = []uqNodeCapture{{p.capture, node}}
	}

	// If there are no child patterns to match, then return this single match.
	if len(p.children) == 0 {
		return []uqNodeMatch{mat}
	}

	// Find every matching combination of child patterns and child nodes.
	type matchState struct {
		patternIndex int
		mat          uqNodeMatch
	}
	var finishedMatches []uqNodeMatch
	if cursor.GotoFirstChild() {
		matchStates := []matchState{{0, mat}}
		for {
			var newMatchStates []matchState
			for _, state := range matchStates {
				childPattern := p.children[state.patternIndex]
				childMatches := childPattern.matchNode(cursor)
				for _, childMatch := range childMatches {
					combinedMatch := uqNodeMatch{
						captures: append(slices.Clone(state.mat.captures), childMatch.captures...),
						lastNode: childMatch.lastNode,
						hasLast:  childMatch.hasLast,
					}
					if state.patternIndex+1 < len(p.children) {
						newMatchStates = append(newMatchStates, matchState{state.patternIndex + 1, combinedMatch})
					} else {
						existing := false
						for i := range finishedMatches {
							if slices.EqualFunc(finishedMatches[i].captures, combinedMatch.captures, uqEqualNodeCaptures) {
								if childPattern.capture != "" {
									finishedMatches[i].lastNode = combinedMatch.lastNode
									finishedMatches[i].hasLast = combinedMatch.hasLast
								}
								existing = true
							}
						}
						if !existing {
							finishedMatches = append(finishedMatches, combinedMatch)
						}
					}
				}
			}
			matchStates = append(matchStates, newMatchStates...)
			if !cursor.GotoNextSibling() {
				break
			}
		}
		cursor.GotoParent()
	}
	return finishedMatches
}

// uqEqualNodeCaptures reports whether two captures have the same name and
// the same node. Node in Rust compares as ts_node_eq does.
func uqEqualNodeCaptures(a, b uqNodeCapture) bool {
	return a.name == b.name && a.node.Equal(b.node)
}

// uqEqualNodeMatches reports whether two matches are the same. It is the
// PartialEq of Match.
func uqEqualNodeMatches(a, b uqNodeMatch) bool {
	if a.hasLast != b.hasLast || (a.hasLast && !a.lastNode.Equal(b.lastNode)) {
		return false
	}
	return slices.EqualFunc(a.captures, b.captures, uqEqualNodeCaptures)
}

// uqCompareMatches orders two matches. It is the Ord of Match.
//
// Tree-sitter returns matches in the order that they terminate
// during a depth-first walk of the tree. If multiple matches
// terminate on the same node, those matches are produced in the
// order that their captures were discovered.
func uqCompareMatches(a, b uqNodeMatch) int {
	if a.hasLast && b.hasLast {
		if c := uqCompareDepthFirst(a.lastNode, b.lastNode); c != 0 {
			return c
		}
	}

	for i := range min(len(a.captures), len(b.captures)) {
		if c := uqCompareDepthFirst(a.captures[i].node, b.captures[i].node); c != 0 {
			return c
		}
	}

	return cmp.Compare(len(a.captures), len(b.captures))
}

// uqCompareDepthFirst orders two nodes as a depth-first walk meets them. It
// is compare_depth_first of query_helpers.rs.
func uqCompareDepthFirst(a, b transit.Node) int {
	return cmp.Or(cmp.Compare(a.StartByte(), b.StartByte()), cmp.Compare(b.EndByte(), a.EndByte()))
}

// uqFormatNodeMatches writes a list of matches, one match on each line.
func uqFormatNodeMatches(matches []uqNodeMatch) string {
	var b strings.Builder
	for _, m := range matches {
		b.WriteString("  [")
		for i, c := range m.captures {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s %s %d..%d", c.name, c.node.Kind(), c.node.StartByte(), c.node.EndByte())
		}
		b.WriteString("]\n")
	}
	return b.String()
}
