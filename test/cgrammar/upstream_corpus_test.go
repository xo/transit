package cgrammar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
	"github.com/xo/transit/internal/grammartest"
)

// This file ports crates/cli/src/tests/corpus_test.rs of upstream (D35),
// with the parts of fuzz/corpus_test.rs, fuzz/scope_sequence.rs, fuzz.rs,
// test.rs and parse.rs that it uses: the reader of the corpus files, the
// rendering of a tree as the corpus expects it, and the checks of a tree
// after an edit.
//
// The attribute test_with_seed is urRetrySeeds. The start seed comes from
// TREE_SITTER_SEED, or it is random, as in upstream. urRand uses the PCG of
// math/rand/v2 in place of StdRng, so a seed gives other edits than in
// upstream. A failed check of the sizes of a tree panics in upstream, and
// here it fails the trial, with the other failures. allocations::record is
// dropped, and the environment variables for logs and for the filters of
// the examples are not read.

// urEditCount is DEFAULT_EDIT_COUNT of fuzz.rs.
const urEditCount = 3

// urIterationCount is DEFAULT_ITERATION_COUNT of fuzz.rs.
const urIterationCount = 10

// urTestEntry is TestEntry of test.rs: a group of tests, or one example.
type urTestEntry struct {
	name     string
	children []urTestEntry
	example  bool

	input     []byte
	output    string
	hasFields bool
	cst       bool
	languages []string
}

// urFlattenedTest is FlattenedTest of fuzz.rs.
type urFlattenedTest struct {
	name               string
	input              []byte
	output             string
	languages          []string
	hasFields          bool
	cst                bool
	templateDelimiters *[2]string
}

// urParseTests is parse_tests of test.rs.
func urParseTests(path string) (urTestEntry, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	info, err := os.Stat(path)
	if err != nil {
		return urTestEntry{}, fmt.Errorf("reading the tests in %s: %w", path, err)
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return urTestEntry{}, fmt.Errorf("reading the tests in %s: %w", path, err)
		}
		var children []urTestEntry
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			child, err := urParseTests(filepath.Join(path, e.Name()))
			if err != nil {
				return urTestEntry{}, err
			}
			children = append(children, child)
		}
		return urTestEntry{name: name, children: children}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return urTestEntry{}, fmt.Errorf("reading the tests in %s: %w", path, err)
	}
	return urParseTestContent(name, string(b)), nil
}

// urParseDelimiterLine is parse_delimiter_line.
func urParseDelimiterLine(line string, c byte) (int, string, bool) {
	n := len(line) - len(strings.TrimLeft(line, string(c)))
	if n < 3 {
		return 0, "", false
	}
	return n, strings.TrimRight(line[n:], "\r\n"), true
}

// urSuffixMatches is suffix_matches.
func urSuffixMatches(firstSuffix *string, suffix string) bool {
	if firstSuffix == nil {
		return suffix == ""
	}
	return suffix != "" && *firstSuffix == suffix
}

// urTrimEnd is str::trim_end.
func urTrimEnd(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

// urPendingTest is PendingTest.
type urPendingTest struct {
	entry         urTestEntry
	bodyStartLine int
}

// urParseHeader is parse_header, without the warnings. The corpus tests
// do not read the expectation, the platform or fail-fast of a test, so it
// does not keep them.
func urParseHeader(lines []string, firstSuffix *string, startLine int) (urPendingTest, int, bool) {
	_, suffix, ok := urParseDelimiterLine(lines[startLine], '=')
	if !ok || !urSuffixMatches(firstSuffix, suffix) {
		return urPendingTest{}, 0, false
	}

	// Collect name and attribute lines until the closing `===` line.
	testName := ""
	seenMarker, cst := false, false
	var languages []string

	lineNum := startLine + 1 // start past opening === line
	for ; lineNum < len(lines); lineNum++ {
		if _, closingSuffix, ok := urParseDelimiterLine(lines[lineNum], '='); ok && urSuffixMatches(firstSuffix, closingSuffix) {
			break
		}
		trimmed := strings.TrimSpace(lines[lineNum])
		// Reject a blank line in the name region so a literal `===` inside a
		// test body can't be mistaken for an opening delimiter. Blank lines
		// between markers are allowed as visual separators.
		if trimmed == "" && !seenMarker {
			return urPendingTest{}, 0, false
		}
		head, _, _ := strings.Cut(trimmed, "(")
		switch {
		case head == ":skip":
			seenMarker = true
		case head == ":platform":
			if strings.HasPrefix(trimmed, ":platform(") && strings.HasSuffix(trimmed, ")") {
				seenMarker = true
			}
		case head == ":fail-fast":
			seenMarker = true
		case head == ":error":
			seenMarker = true
		case head == ":language":
			if lang, ok := strings.CutPrefix(trimmed, ":language("); ok && strings.HasSuffix(lang, ")") {
				seenMarker = true
				languages = append(languages, strings.TrimSuffix(lang, ")"))
			}
		case head == ":cst":
			seenMarker, cst = true, true
		case !seenMarker:
			testName += lines[lineNum]
		}
	}

	if lineNum >= len(lines) {
		return urPendingTest{}, 0, false // No closing `===` line found.
	}

	if len(languages) == 0 {
		languages = append(languages, "")
	}

	pending := urPendingTest{
		entry: urTestEntry{
			name:      urTrimEnd(testName),
			example:   true,
			cst:       cst,
			languages: languages,
		},
		bodyStartLine: lineNum + 1,
	}
	return pending, lineNum + 1, true // +1 to consume the closing `===` line
}

// urParseTestContent is parse_test_content.
func urParseTestContent(name, content string) urTestEntry {
	var children []urTestEntry
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Determine the suffix from the first `===` line in the file.
	var firstSuffix *string
	for _, line := range lines {
		if _, suffix, ok := urParseDelimiterLine(line, '='); ok && suffix != "" {
			firstSuffix = &suffix
			break
		}
	}

	// Scan for header blocks and build test entries from the bodies between them.
	lineNum := 0
	var prevTest *urPendingTest

	for lineNum < len(lines) {
		pending, bodyStartLine, ok := urParseHeader(lines, firstSuffix, lineNum)
		if !ok {
			lineNum++
			continue
		}

		openingLine := lineNum
		lineNum = bodyStartLine

		// Process the PREVIOUS test's body now that we know where it ends.
		if prevTest != nil {
			if entry, ok := urBuildTestEntry(lines[prevTest.bodyStartLine:openingLine], firstSuffix, *prevTest); ok {
				children = append(children, entry)
			}
		}

		prevTest = &pending
	}

	// Process the last test's body (terminated by end of content).
	if prevTest != nil {
		if entry, ok := urBuildTestEntry(lines[prevTest.bodyStartLine:], firstSuffix, *prevTest); ok {
			children = append(children, entry)
		}
	}

	return urTestEntry{name: name, children: children}
}

// urBuildTestEntry is build_test_entry.
func urBuildTestEntry(bodyLines []string, firstSuffix *string, pending urPendingTest) (urTestEntry, bool) {
	// Find the longest `---` divider line in the body whose suffix matches.
	dividerLine := -1
	bestTotalLen := 0
	for j, line := range bodyLines {
		if n, suffix, ok := urParseDelimiterLine(line, '-'); ok && urSuffixMatches(firstSuffix, suffix) {
			totalLen := n + len(suffix)
			// For ties prefer the later candidate, as an earlier same-length
			// `---` is a literal in the input.
			if totalLen >= bestTotalLen {
				dividerLine = j
				bestTotalLen = totalLen
			}
		}
	}
	if dividerLine < 0 {
		return urTestEntry{}, false
	}

	// Input: lines before the divider (as bytes), with trailing newline stripped.
	input := []byte(strings.Join(bodyLines[:dividerLine], ""))
	// Remove trailing newline.
	input = bytes.TrimSuffix(input, []byte("\n"))
	input = bytes.TrimSuffix(input, []byte("\r"))

	// Output: lines after the divider.
	outputStr := strings.Join(bodyLines[dividerLine+1:], "")

	entry := pending.entry
	entry.input = input
	if entry.cst {
		entry.output = strings.TrimSpace(outputStr)
	} else {
		entry.output, entry.hasFields = urNormalizeSexpOutput(outputStr)
	}
	return entry, true
}

// urNormalizeSexpOutput is normalize_sexp_output: it removes comment
// lines (lines starting with `;`), collapses whitespace, and removes spaces
// before closing parens.
func urNormalizeSexpOutput(raw string) (string, bool) {
	var result []rune
	prevWasSpace := false

	for line := range strings.Lines(raw) {
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		// Skip comment lines: lines whose first non-whitespace character is `;`
		if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), ";") {
			continue
		}
		for _, ch := range line {
			if unicode.IsSpace(ch) {
				if !prevWasSpace && len(result) > 0 {
					result = append(result, ' ')
					prevWasSpace = true
				}
			} else {
				if ch == ')' && prevWasSpace {
					result = result[:len(result)-1] // remove trailing space before `)`
				}
				result = append(result, ch)
				prevWasSpace = false
			}
		}
		// Line boundary counts as whitespace
		if len(result) > 0 && !prevWasSpace {
			result = append(result, ' ')
			prevWasSpace = true
		}
	}

	// No leading whitespace
	out := urTrimEnd(string(result))
	hasFields := strings.Contains(out, ": (")

	return out, hasFields
}

// urFlattenTests is flatten_tests, with no filters.
func urFlattenTests(test urTestEntry) []urFlattenedTest {
	var result []urFlattenedTest
	var helper func(test urTestEntry, isRoot bool, prefix string)
	helper = func(test urTestEntry, isRoot bool, prefix string) {
		name := test.name
		if test.example {
			if prefix != "" {
				name = prefix + " - " + name
			}
			result = append(result, urFlattenedTest{
				name:      name,
				input:     test.input,
				output:    test.output,
				languages: test.languages,
				hasFields: test.hasFields,
				cst:       test.cst,
			})
			return
		}
		if !isRoot && prefix != "" {
			name = prefix + " - " + name
		}
		for _, child := range test.children {
			helper(child, false, name)
		}
	}
	helper(test, true, "")
	return result
}

// urStripSexpFields is strip_sexp_fields: it replaces ` word: (` with
// ` (` throughout the string.
func urStripSexpFields(sexp string) string {
	var result strings.Builder
	remaining := sexp
	for {
		pos := strings.Index(remaining, ": (")
		if pos < 0 {
			break
		}
		// Walk backwards from the `:` to find the field name and preceding space.
		if spacePos := strings.LastIndexByte(remaining[:pos], ' '); spacePos >= 0 {
			word := remaining[spacePos+1 : pos]
			if word != "" && strings.IndexFunc(word, func(r rune) bool {
				return r >= 0x80 || !(r == '_' || '0' <= r && r <= '9' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z')
			}) < 0 {
				// Emit everything up to and including the space, then `(`
				result.WriteString(remaining[:spacePos+1])
				result.WriteByte('(')
				remaining = remaining[pos+3:]
				continue
			}
		}
		// Not a field pattern — emit through `: (` and keep going
		result.WriteString(remaining[:pos+3])
		remaining = remaining[pos+3:]
	}
	result.WriteString(remaining)
	return result.String()
}

// urRenderTestOutput is render_test_output: it renders a parsed tree in
// the output format expected by a corpus test.
func urRenderTestOutput(input []byte, tree *transit.Tree, cst, includeFields bool) string {
	if cst {
		// render_cst of parse.rs is in the package grammartest, which the
		// corpus test of a grammar package runs with it
		return grammartest.RenderCST(input, tree)
	}
	out := tree.RootNode().String()
	if includeFields {
		return out
	}
	return urStripSexpFields(out)
}

// urSetIncludedRanges is set_included_ranges of fuzz/corpus_test.rs.
func urSetIncludedRanges(parser *transit.Parser, input []byte, delimiters *[2]string) error {
	if delimiters == nil {
		return parser.SetIncludedRanges(nil)
	}
	start, end := delimiters[0], delimiters[1]
	var ranges []transit.Range
	ix := 0
	for ix < len(input) {
		startIx := bytes.Index(input[ix:], []byte(start))
		if startIx < 0 {
			break
		}
		startIx += ix + len(start)
		endIx := len(input)
		if i := bytes.Index(input[startIx:], []byte(end)); i >= 0 {
			endIx = startIx + i
		}
		ix = endIx
		ranges = append(ranges, transit.Range{
			StartByte:  startIx,
			EndByte:    endIx,
			StartPoint: pointAt(input, startIx),
			EndPoint:   pointAt(input, endIx),
		})
	}
	return parser.SetIncludedRanges(ranges)
}

// urPointLess reports whether a point is before another.
func urPointLess(a, b transit.Point) bool {
	return a.Row < b.Row || a.Row == b.Row && a.Column < b.Column
}

// urSizeCheckFrame is SizeCheckFrame of fuzz/corpus_test.rs.
type urSizeCheckFrame struct {
	node                  transit.Node
	endByte               int
	endPoint              transit.Point
	childCount            int
	childIndex            int
	lastChildEndByte      int
	lastChildEndPoint     transit.Point
	someChildHasChanges   bool
	actualNamedChildCount int
}

// urNewSizeCheckFrame is SizeCheckFrame::new.
func urNewSizeCheckFrame(node transit.Node, lineOffsets []int) (*urSizeCheckFrame, error) {
	startByte := node.StartByte()
	endByte := node.EndByte()
	startPoint := node.StartPoint()
	endPoint := node.EndPoint()

	if startByte > endByte {
		return nil, fmt.Errorf("%s: start_byte %d > end_byte %d", node.Kind(), startByte, endByte)
	}
	if urPointLess(endPoint, startPoint) {
		return nil, fmt.Errorf("%s: start_point %v > end_point %v", node.Kind(), startPoint, endPoint)
	}
	if startPoint.Row >= len(lineOffsets) || startByte != lineOffsets[startPoint.Row]+startPoint.Column {
		return nil, fmt.Errorf("%s: start_byte %d does not match start_point %v", node.Kind(), startByte, startPoint)
	}
	if endPoint.Row >= len(lineOffsets) || endByte != lineOffsets[endPoint.Row]+endPoint.Column {
		return nil, fmt.Errorf("%s: end_byte %d does not match end_point %v", node.Kind(), endByte, endPoint)
	}

	return &urSizeCheckFrame{
		node:              node,
		endByte:           endByte,
		endPoint:          endPoint,
		childCount:        node.ChildCount(),
		lastChildEndByte:  startByte,
		lastChildEndPoint: startPoint,
	}, nil
}

// urCheckConsistentSizes is check_consistent_sizes. It returns an error
// where upstream fails an assertion.
func urCheckConsistentSizes(tree *transit.Tree, input []byte) error {
	lineOffsets := []int{0}
	for i, c := range input {
		if c == '\n' {
			lineOffsets = append(lineOffsets, i+1)
		}
	}

	frame, err := urNewSizeCheckFrame(tree.RootNode(), lineOffsets)
	if err != nil {
		return err
	}
	stack := []*urSizeCheckFrame{frame}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		if top.childIndex < top.childCount {
			i := top.childIndex
			child, ok := top.node.Child(i)
			if !ok {
				return fmt.Errorf("%s: no child %d", top.node.Kind(), i)
			}

			if child.StartByte() < top.lastChildEndByte {
				return fmt.Errorf("%s: child %d starts at %d, before the end %d of the child before it", top.node.Kind(), i, child.StartByte(), top.lastChildEndByte)
			}
			if urPointLess(child.StartPoint(), top.lastChildEndPoint) {
				return fmt.Errorf("%s: child %d starts at %v, before the end %v of the child before it", top.node.Kind(), i, child.StartPoint(), top.lastChildEndPoint)
			}
			if child.HasChanges() {
				top.someChildHasChanges = true
			}
			if child.IsNamed() {
				top.actualNamedChildCount++
			}
			top.lastChildEndByte = child.EndByte()
			top.lastChildEndPoint = child.EndPoint()
			top.childIndex++

			frame, err := urNewSizeCheckFrame(child, lineOffsets)
			if err != nil {
				return err
			}
			stack = append(stack, frame)
			continue
		}

		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.actualNamedChildCount != frame.node.NamedChildCount() {
			return fmt.Errorf("%s: %d named children, and named_child_count is %d", frame.node.Kind(), frame.actualNamedChildCount, frame.node.NamedChildCount())
		}
		if frame.childCount > 0 {
			if frame.endByte < frame.lastChildEndByte {
				return fmt.Errorf("%s: ends at %d, before its last child at %d", frame.node.Kind(), frame.endByte, frame.lastChildEndByte)
			}
			if urPointLess(frame.endPoint, frame.lastChildEndPoint) {
				return fmt.Errorf("%s: ends at %v, before its last child at %v", frame.node.Kind(), frame.endPoint, frame.lastChildEndPoint)
			}
		}
		if frame.someChildHasChanges && !frame.node.HasChanges() {
			return fmt.Errorf("%s: a child has changes, and the node has none", frame.node.Kind())
		}
	}
	return nil
}

// urScopeSequence is ScopeSequence of fuzz/scope_sequence.rs: the stack of
// the kinds of the nodes at each byte of a text.
type urScopeSequence [][]string

// urNewScopeSequence is ScopeSequence::new.
func urNewScopeSequence(tree *transit.Tree) urScopeSequence {
	var result urScopeSequence
	var scopeStack []string

	cursor := tree.Walk()
	visitedChildren := false
	for {
		node := cursor.Node()
		for len(result) < node.StartByte() {
			result = append(result, slices.Clone(scopeStack))
		}
		if visitedChildren {
			for len(result) < node.EndByte() {
				result = append(result, slices.Clone(scopeStack))
			}
			scopeStack = scopeStack[:len(scopeStack)-1]
			if cursor.GotoNextSibling() {
				visitedChildren = false
			} else if !cursor.GotoParent() {
				break
			}
		} else {
			scopeStack = append(scopeStack, cursor.Node().Kind())
			if !cursor.GotoFirstChild() {
				visitedChildren = true
			}
		}
	}

	return result
}

// checkChanges is check_changes.
func (s urScopeSequence) checkChanges(other urScopeSequence, text []byte, knownChangedRanges []transit.Range) error {
	var position transit.Point
	for i := range max(len(s), len(other)) {
		var stack, otherStack []string
		hasStack, hasOther := i < len(s), i < len(other)
		if hasStack {
			stack = s[i]
		}
		if hasOther {
			otherStack = other[i]
		}
		if i >= len(text) {
			return fmt.Errorf("byte offset %d is past the end of the text", i)
		}
		if (hasStack != hasOther || !slices.Equal(stack, otherStack)) && text[i] != '\r' && text[i] != '\n' {
			contained := false
			for _, r := range knownChangedRanges {
				if !urPointLess(position, r.StartPoint) && urPointLess(position, r.EndPoint) {
					contained = true
					break
				}
			}
			if !contained {
				line, _, _ := bytes.Cut(text[i-position.Column:], []byte("\n"))
				return fmt.Errorf("Position: %v\nByte offset: %d\nLine: %s\n%s^\nOld scopes: %q\nNew scopes: %q\nInvalidated ranges: %v",
					position, i, strings.ToValidUTF8(string(line), "�"),
					strings.Repeat(" ", position.Column+len("Line: ")),
					stack, otherStack, knownChangedRanges)
			}
		}

		if text[i] == '\n' {
			position.Row++
			position.Column = 0
		} else {
			position.Column++
		}
	}
	return nil
}

// urCheckChangedRanges is check_changed_ranges.
func urCheckChangedRanges(oldTree, newTree *transit.Tree, input []byte) error {
	changedRanges := oldTree.ChangedRanges(newTree)
	oldScopeSequence := urNewScopeSequence(oldTree)
	newScopeSequence := urNewScopeSequence(newTree)

	oldRange := oldTree.RootNode().Range()
	newRange := newTree.RootNode().Range()

	byteRangeEnd := max(oldRange.EndByte, newRange.EndByte)
	pointRangeEnd := oldRange.EndPoint
	if urPointLess(pointRangeEnd, newRange.EndPoint) {
		pointRangeEnd = newRange.EndPoint
	}

	for _, r := range changedRanges {
		if r.EndByte > byteRangeEnd || urPointLess(pointRangeEnd, r.EndPoint) {
			return fmt.Errorf("changed range extends outside of the old and new trees %v", r)
		}
	}

	return oldScopeSequence.checkChanges(newScopeSequence, input, changedRanges)
}

// urCheckInitialParse is check_initial_parse of FlattenedTest.
func urCheckInitialParse(test urFlattenedTest, language *transit.Language, displayName string) error {
	parser := transit.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		return err
	}
	if err := urSetIncludedRanges(parser, test.input, test.templateDelimiters); err != nil {
		return err
	}

	tree, err := parser.Parse(context.Background(), test.input, nil)
	if err != nil {
		return err
	}

	actualOutput := urRenderTestOutput(test.input, tree, test.cst, test.hasFields)

	if actualOutput != test.output {
		return fmt.Errorf("Incorrect initial parse for %s\n  actual:   %s\n  expected: %s", displayName, actualOutput, test.output)
	}
	return nil
}

// urStartSeed is START_SEED and new_seed of fuzz.rs.
func urStartSeed() uint64 {
	if s, err := strconv.ParseUint(os.Getenv("TREE_SITTER_SEED"), 10, 64); err == nil {
		return s
	}
	return rand.Uint64()
}

// urRetrySeeds is the attribute test_with_seed(retry=10, seed=*START_SEED,
// seed_fn=new_seed) of upstream: it runs a test again with a new seed while
// it fails, up to count more times, and reports the failures of the last
// run.
func urRetrySeeds(t *testing.T, count int, run func(seed uint64) []string) {
	t.Helper()
	seed := urStartSeed()
	for i := 0; ; i++ {
		failures := run(seed)
		if len(failures) == 0 {
			return
		}
		if i == count {
			for _, f := range failures {
				t.Error(f)
			}
			return
		}
		seed = rand.Uint64()
		t.Logf("Retry %d/%d with a new seed %d", i+1, count, seed)
	}
}

// urCorpusGrammar returns the language of a fixture grammar and the
// folder of its corpus.
func urCorpusGrammar(t *testing.T, name string) (*transit.Language, string) {
	t.Helper()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		if f.Name == name {
			_, corpus := grammarDirs(cache, f)
			return fixtureGrammar(t, name).Language, corpus
		}
	}
	t.Fatalf("no fixture grammar %s in grammars/grammars.json", name)
	return nil, ""
}

// urTestLanguageCorpus is test_language_corpus. fixtureName is the name of
// the fixture grammar of get_language, such as tsx for typescript/tsx.
func urTestLanguageCorpus(t *testing.T, languageName, fixtureName string, skipped []string, languageDir string) {
	t.Helper()
	t.Parallel()
	root, _ := setup(t)
	language, corpusDir := urCorpusGrammar(t, fixtureName)

	errorCorpusFile := filepath.Join(root, "tree-sitter", "test", "fixtures", "error_corpus", languageName+"_errors.txt")
	templateCorpusFile := filepath.Join(root, "tree-sitter", "test", "fixtures", "template_corpus", languageName+"_templates.txt")

	t.Logf("Testing %s corpus @ %s", languageName, corpusDir)

	mainTests, err := urParseTests(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	tests := urFlattenTests(mainTests)
	if errorTests, err := urParseTests(errorCorpusFile); err == nil {
		tests = append(tests, urFlattenTests(errorTests)...)
	}
	if templateTests, err := urParseTests(templateCorpusFile); err == nil {
		for _, test := range urFlattenTests(templateTests) {
			test.templateDelimiters = &[2]string{"<%", "%>"}
			tests = append(tests, test)
		}
	}

	tests = slices.DeleteFunc(tests, func(test urFlattenedTest) bool {
		return test.languages[0] != "" && !slices.Contains(test.languages, languageDir)
	})
	t.Logf("%d tests", len(tests))

	urRetrySeeds(t, 10, func(startSeed uint64) []string {
		var failures []string
		skipCounts := map[string]int{}
		for _, s := range skipped {
			skipCounts[s] = 0
		}

		for testIndex, test := range tests {
			testName := languageName + " - " + test.name
			if _, ok := skipCounts[testName]; ok {
				t.Logf("  %d. %s - SKIPPED", testIndex, testName)
				skipCounts[testName]++
				continue
			}

			if err := urCheckInitialParse(test, language, testName); err != nil {
				failures = append(failures, err.Error())
				continue
			}

			parser := transit.NewParser()
			if err := parser.SetLanguage(language); err != nil {
				t.Fatal(err)
			}
			tree, err := parser.Parse(context.Background(), test.input, nil)
			if err != nil {
				t.Fatal(err)
			}

			for trial := range uint64(urIterationCount) {
				seed := startSeed + trial
				if msg := urCorpusTrial(t, language, test, tree, seed); msg != "" {
					failures = append(failures, fmt.Sprintf("%s - seed %d with start seed %d\n%s", testName, seed, startSeed, msg))
					break
				}
			}
		}

		if len(failures) > 0 {
			failures = append(failures, fmt.Sprintf("%d %s corpus tests failed", len(failures), languageName))
		}

		var unmatched []string
		for k, v := range skipCounts {
			if v == 0 {
				unmatched = append(unmatched, k)
			}
		}
		if len(unmatched) > 0 {
			slices.Sort(unmatched)
			failures = append(failures, "Non matchable skip definitions needs to be removed: "+strings.Join(unmatched, ", "))
		}
		return failures
	})
}

// urCorpusTrial runs one trial of test_language_corpus: a random series of
// edits and a parse, then the undo of the edits and a parse. It returns
// the failure, or "".
func urCorpusTrial(t *testing.T, language *transit.Language, test urFlattenedTest, tree *transit.Tree, seed uint64) string {
	t.Helper()
	rand := urNewRand(seed)
	parser := transit.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		return err.Error()
	}
	tree = tree.Copy()
	input := slices.Clone(test.input)

	// Perform a random series of edits and reparse.
	editCount := rand.unsigned(urEditCount)
	undoStack := make([]urEdit, 0, editCount)
	for range editCount + 1 {
		edit := urGetRandomEdit(rand, input)
		undoStack = append(undoStack, urInvertEdit(input, edit))
		urPerformEdit(t, tree, &input, edit)
	}

	if err := urSetIncludedRanges(parser, input, test.templateDelimiters); err != nil {
		return err.Error()
	}
	tree2, err := parser.Parse(context.Background(), input, tree)
	if err != nil {
		return err.Error()
	}

	// Check that the new tree is consistent.
	if err := urCheckConsistentSizes(tree2, input); err != nil {
		return "Inconsistent sizes after the edits\n" + err.Error()
	}
	if err := urCheckChangedRanges(tree, tree2, input); err != nil {
		return "Unexpected scope change\n" + err.Error()
	}

	// Undo all of the edits and re-parse again.
	for len(undoStack) > 0 {
		edit := undoStack[len(undoStack)-1]
		undoStack = undoStack[:len(undoStack)-1]
		urPerformEdit(t, tree2, &input, edit)
	}

	if err := urSetIncludedRanges(parser, test.input, test.templateDelimiters); err != nil {
		return err.Error()
	}
	tree3, err := parser.Parse(context.Background(), input, tree2)
	if err != nil {
		return err.Error()
	}

	// Verify that the final tree matches the expectation from the corpus.
	actualOutput := urRenderTestOutput(input, tree3, test.cst, test.hasFields)

	if actualOutput != test.output {
		return fmt.Sprintf("Incorrect parse\n  actual:   %s\n  expected: %s", actualOutput, test.output)
	}

	// Check that the edited tree is consistent.
	if err := urCheckConsistentSizes(tree3, input); err != nil {
		return "Inconsistent sizes after the undo\n" + err.Error()
	}
	if err := urCheckChangedRanges(tree2, tree3, input); err != nil {
		return "Unexpected scope change after the undo\n" + err.Error()
	}

	return ""
}

func TestCorpusForBashLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "bash", "bash", []string{
		// Fragile tests where edit customization changes
		// lead to significant parse tree structure changes.
		"bash - corpus - commands - Nested Heredocs",
		"bash - corpus - commands - Quoted Heredocs",
		"bash - corpus - commands - Heredocs with weird characters",
	}, "")
}

func TestCorpusForCLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "c", "c", nil, "")
}

func TestCorpusForCppLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "cpp", "cpp", nil, "")
}

func TestCorpusForEmbeddedTemplateLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "embedded-template", "embedded_template", nil, "")
}

func TestCorpusForGoLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "go", "go", nil, "")
}

func TestCorpusForHTMLLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "html", "html", nil, "")
}

func TestCorpusForJavaLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "java", "java",
		[]string{"java - corpus - expressions - switch with unnamed pattern variable"}, "")
}

func TestCorpusForJavascriptLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "javascript", "javascript", nil, "")
}

func TestCorpusForJSONLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "json", "json", nil, "")
}

func TestCorpusForPHPLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "php", "php", nil, "php")
}

func TestCorpusForPythonLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "python", "python", nil, "")
}

func TestCorpusForRubyLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "ruby", "ruby", nil, "")
}

func TestCorpusForRustLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "rust", "rust", nil, "")
}

func TestCorpusForTypescriptLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "typescript", "typescript", nil, "typescript")
}

func TestCorpusForTsxLanguage(t *testing.T) {
	urTestLanguageCorpus(t, "typescript", "tsx", nil, "tsx")
}

// TestFeatureCorpusFiles reads the grammar.json that the golden harness
// writes for each test grammar in generate/testdata, in place of its
// grammar.js (D17).
func TestFeatureCorpusFiles(t *testing.T) {
	root, _ := setup(t)
	testGrammarsDir := filepath.Join(root, "tree-sitter", "test", "fixtures", "test_grammars")

	failureCount := 0
	entries, err := os.ReadDir(testGrammarsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		languageName := entry.Name()

		testPath := filepath.Join(testGrammarsDir, languageName)
		jsonDir, _ := urTestGrammarDirs(root, languageName)
		grammarJSON, err := os.ReadFile(filepath.Join(jsonDir, "grammar.json"))
		if err != nil {
			t.Errorf("Could not load grammar file for test language '%s' at %s: %v", languageName, jsonDir, err)
			failureCount++
			continue
		}
		var diagnostics []generate.Diagnostic
		_, _, generateErr := generate.ParserForGrammar(grammarJSON, &generate.SemanticVersion{}, generate.OptLevelMergeStates, c.Backend{}, &diagnostics)

		errorMessagePath := filepath.Join(testPath, "expected_error.txt")
		if b, err := os.ReadFile(errorMessagePath); err == nil {
			t.Logf("test language: %q", languageName)

			expectedMessage := strings.ReplaceAll(string(b), "\r\n", "\n")
			if generateErr != nil {
				actualMessage := strings.ReplaceAll(generateErr.Error(), "\r\n", "\n")
				if expectedMessage != actualMessage {
					t.Errorf("Unexpected error message for test grammar '%s'.\n\nExpected:\n\n`%s`\nActual:\n\n`%s`\n", languageName, expectedMessage, actualMessage)
					failureCount++
				}
			} else {
				t.Errorf("Expected error message but got none for test grammar '%s'", languageName)
				failureCount++
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}

		if generateErr != nil {
			t.Errorf("Unexpected error for test grammar '%s':\n%v", languageName, generateErr)
			failureCount++
			continue
		}

		corpusPath := filepath.Join(testPath, "corpus.txt")
		language := urTestLanguage(t, string(grammarJSON), testPath)
		test, err := urParseTests(corpusPath)
		if err != nil {
			t.Fatal(err)
		}
		tests := urFlattenTests(test)

		if len(tests) > 0 {
			t.Logf("test language: %q", languageName)
		}

		for _, test := range tests {
			t.Logf("  example: %q", test.name)

			if err := urCheckInitialParse(test, language, test.name); err != nil {
				t.Errorf("%s: %v", languageName, err)
				failureCount++
			}
		}
	}

	if failureCount != 0 {
		t.Errorf("%d corpus tests failed", failureCount)
	}
}
