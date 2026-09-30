package grammartest

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file ports the parts of crates/cli/src/test.rs that read a corpus:
// parse_tests and the functions that it calls, normalize_sexp_output and
// strip_sexp_fields. It leaves out what `tree-sitter test` needs to update a
// corpus file, to print a report and to measure the speed of a parse. The
// warnings of parse_header go nowhere, because the log of the command is not
// ported (D41).

// parseDelimiterLine reports whether a line is 3 or more repetitions of c
// followed by an optional suffix. It returns the number of repetitions and
// the suffix, which is empty when there is none.
//
// parseDelimiterLine is parse_delimiter_line.
func parseDelimiterLine(line string, c byte) (int, string, bool) {
	delimLen := len(line) - len(strings.TrimLeft(line, string(c)))
	if delimLen < 3 {
		return 0, "", false
	}
	suffix := strings.TrimRight(line[delimLen:], "\r\n")
	return delimLen, suffix, true
}

// normalizeSexpOutput normalizes the expected sexp output: it removes the
// comment lines (lines starting with `;`), collapses the whitespace, and
// removes the spaces before the closing parens. It also reports whether the
// output names a field.
//
// normalizeSexpOutput is normalize_sexp_output.
func normalizeSexpOutput(raw string) (string, bool) {
	result := make([]byte, 0, len(raw))
	prevWasSpace := false

	for line := range lines(raw) {
		// Skip comment lines: lines whose first non-whitespace character is `;`
		if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), ";") {
			continue
		}
		for _, ch := range line {
			if unicode.IsSpace(ch) {
				if !prevWasSpace && len(result) != 0 {
					result = append(result, ' ')
					prevWasSpace = true
				}
			} else {
				if ch == ')' && prevWasSpace {
					// remove trailing space before `)`
					result = result[:len(result)-1]
				}
				result = utf8.AppendRune(result, ch)
				prevWasSpace = false
			}
		}
		// Line boundary counts as whitespace
		if len(result) != 0 && !prevWasSpace {
			result = append(result, ' ')
			prevWasSpace = true
		}
	}

	// No leading whitespace
	out := strings.TrimRightFunc(string(result), unicode.IsSpace)
	hasFields := strings.Contains(out, ": (")

	return out, hasFields
}

// lines yields the lines of a text as str::lines does: each line without
// its "\n" or "\r\n", and no empty line after a last "\n".
func lines(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for s != "" {
			line, rest, found := strings.Cut(s, "\n")
			if !found {
				rest = ""
			}
			if !yield(strings.TrimSuffix(line, "\r")) {
				return
			}
			s = rest
		}
	}
}

// testExpectation is what a corpus test expects.
//
// testExpectation is TestExpectation.
type testExpectation uint8

// The expectations of a corpus test.
const (
	expectPass testExpectation = iota
	expectError
	expectSkip
)

// testAttributes are the attributes of a corpus test, from the lines after
// its name.
//
// testAttributes is TestAttributes.
type testAttributes struct {
	platform    bool
	failFast    bool
	expectation testExpectation
	cst         bool
	languages   []string
}

// testEntry is a group of corpus tests or one corpus test, an example.
// A group has children, and an example has an input and an output.
//
// testEntry is TestEntry, an enum with data upstream. The port keeps the
// fields that the tests of a grammar read.
type testEntry struct {
	name     string
	children []testEntry
	isGroup  bool
	filePath string

	input      []byte
	output     string
	hasFields  bool
	attributes testAttributes
}

// parseTests reads the corpus tests of a file, or of each file in a folder
// and its subfolders. The name of each group is the name of its file or
// folder without the extension.
//
// parseTests is parse_tests.
func parseTests(path string) (testEntry, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	info, err := os.Stat(path)
	if err != nil {
		return testEntry{}, err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return testEntry{}, err
		}
		var childPaths []string
		for _, entry := range entries {
			hidden := strings.HasPrefix(entry.Name(), ".")
			if !hidden {
				childPaths = append(childPaths, filepath.Join(path, entry.Name()))
			}
		}
		slices.SortFunc(childPaths, func(a, b string) int {
			return strings.Compare(filepath.Base(a), filepath.Base(b))
		})
		group := testEntry{name: name, isGroup: true}
		for _, p := range childPaths {
			child, err := parseTests(p)
			if err != nil {
				return testEntry{}, err
			}
			group.children = append(group.children, child)
		}
		return group, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return testEntry{}, err
	}
	return parseTestContent(name, string(content), path), nil
}

// stripSexpFields replaces ` word: (` with ` (` throughout the string. It is
// for the output of Node.String, where the elements are separated by single
// spaces.
//
// stripSexpFields is strip_sexp_fields.
func stripSexpFields(sexp string) string {
	var result strings.Builder
	result.Grow(len(sexp))
	remaining := sexp
	for {
		pos := strings.Index(remaining, ": (")
		if pos < 0 {
			break
		}
		// Walk backwards from the `:` to find the field name and preceding space.
		if spacePos := strings.LastIndexByte(remaining[:pos], ' '); spacePos >= 0 {
			word := remaining[spacePos+1 : pos]
			if word != "" && strings.IndexFunc(word, func(r rune) bool { return !isWordByte(r) }) < 0 {
				// Emit everything up to and including the space, then `(`
				result.WriteString(remaining[:spacePos+1])
				result.WriteByte('(')
				remaining = remaining[pos+3:]
				continue
			}
		}
		// Not a field pattern: emit through `: (` and keep going
		result.WriteString(remaining[:pos+3])
		remaining = remaining[pos+3:]
	}
	result.WriteString(remaining)
	return result.String()
}

// isWordByte reports whether r is an ASCII letter, an ASCII digit or an
// underscore.
func isWordByte(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// suffixMatches reports whether the suffix of a delimiter line matches the
// first suffix of the file.
//
// suffixMatches is suffix_matches.
func suffixMatches(firstSuffix *string, suffix string) bool {
	if firstSuffix == nil {
		return suffix == ""
	}
	return suffix != "" && *firstSuffix == suffix
}

// pendingTest is the header of a test, kept while the scan looks for the end
// of its body.
//
// pendingTest is PendingTest.
type pendingTest struct {
	name          string
	attributes    testAttributes
	bodyStartLine int
}

// currentOS returns the name of the operating system as
// std::env::consts::OS gives it.
func currentOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

// parseHeader tries to parse a header block (the opening `===`, the name and
// the markers, and the closing `===`) that starts at lines[startLine]. It
// returns the header and the index of the line after the closing `===`, and
// false when lines[startLine] is not a matching `===` delimiter.
//
// parseHeader is parse_header.
func parseHeader(lines []string, firstSuffix *string, startLine int) (pendingTest, int, bool) {
	_, suffix, ok := parseDelimiterLine(lines[startLine], '=')
	if !ok || !suffixMatches(firstSuffix, suffix) {
		return pendingTest{}, 0, false
	}

	// Collect name and attribute lines until the closing `===` line.
	var testName strings.Builder
	seenMarker, seenSkip, seenError := false, false, false
	var platform *bool
	failFast, cst := false, false
	var languages []string

	lineNum := startLine + 1
	for lineNum < len(lines) {
		if _, closingSuffix, ok := parseDelimiterLine(lines[lineNum], '='); ok && suffixMatches(firstSuffix, closingSuffix) {
			break
		}
		trimmed := strings.TrimSpace(lines[lineNum])
		// Reject a blank line in the name region so a literal `===` inside a
		// test body can't be mistaken for an opening delimiter. Blank lines
		// between markers are allowed as visual separators.
		if trimmed == "" && !seenMarker {
			return pendingTest{}, 0, false
		}
		head, _, _ := strings.Cut(trimmed, "(")
		switch {
		case head == ":skip":
			seenMarker, seenSkip = true, true
		case head == ":platform":
			if platforms, ok := markerArgument(trimmed, "platform"); ok {
				seenMarker = true
				p := (platform != nil && *platform) || strings.TrimSpace(platforms) == currentOS()
				platform = &p
			}
		case head == ":fail-fast":
			seenMarker, failFast = true, true
		case head == ":error":
			seenMarker, seenError = true, true
		case head == ":language":
			if lang, ok := markerArgument(trimmed, "language"); ok {
				seenMarker = true
				languages = append(languages, lang)
			}
		case head == ":cst":
			seenMarker, cst = true, true
		case !seenMarker:
			// This line is part of the test name. Upstream warns when it holds
			// a token that looks like an attribute marker.
			testName.WriteString(lines[lineNum])
		}
		lineNum++
	}

	if lineNum >= len(lines) {
		// No closing `===` line found.
		return pendingTest{}, 0, false
	}

	var expectation testExpectation
	switch {
	case seenSkip:
		// with :error too, upstream warns and drops :error
		expectation = expectSkip
	case seenError:
		expectation = expectError
	default:
		expectation = expectPass
	}

	if len(languages) == 0 {
		languages = append(languages, "")
	}

	pending := pendingTest{
		name: strings.TrimRightFunc(testName.String(), unicode.IsSpace),
		attributes: testAttributes{
			platform:    platform == nil || *platform,
			failFast:    failFast,
			expectation: expectation,
			cst:         cst,
			languages:   languages,
		},
		bodyStartLine: lineNum + 1,
	}

	// +1 to consume the closing `===` line
	return pending, lineNum + 1, true
}

// markerArgument returns the argument of a marker line such as
// ":language(json)", for the marker name "language".
func markerArgument(trimmed, marker string) (string, bool) {
	s, ok := strings.CutPrefix(trimmed, ":"+marker+"(")
	if !ok {
		return "", false
	}
	return strings.CutSuffix(s, ")")
}

// parseTestContent splits the content of a corpus file into its tests.
//
// parseTestContent is parse_test_content.
func parseTestContent(name, content, filePath string) testEntry {
	var children []testEntry
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Determine the suffix from the first `===` line in the file.
	var firstSuffix *string
	for _, line := range lines {
		if _, suffix, ok := parseDelimiterLine(line, '='); ok && suffix != "" {
			firstSuffix = &suffix
			break
		}
	}

	// Scan for header blocks and build test entries from the bodies between them.
	lineNum := 0
	var prevTest *pendingTest

	for lineNum < len(lines) {
		pending, bodyStartLine, ok := parseHeader(lines, firstSuffix, lineNum)
		if !ok {
			lineNum++
			continue
		}

		openingLine := lineNum
		lineNum = bodyStartLine

		// Process the PREVIOUS test's body now that we know where it ends.
		if prevTest != nil {
			if entry, ok := buildTestEntry(lines[prevTest.bodyStartLine:openingLine], firstSuffix, *prevTest); ok {
				children = append(children, entry)
			}
		}

		prevTest = &pending
	}

	// Process the last test's body (terminated by end of content).
	if prevTest != nil {
		if entry, ok := buildTestEntry(lines[prevTest.bodyStartLine:], firstSuffix, *prevTest); ok {
			children = append(children, entry)
		}
	}

	return testEntry{name: name, children: children, isGroup: true, filePath: filePath}
}

// buildTestEntry builds a single test entry from the body lines between a
// header and the next header. It finds the longest matching `---` divider to
// separate the input from the expected output.
//
// buildTestEntry is build_test_entry.
func buildTestEntry(bodyLines []string, firstSuffix *string, pending pendingTest) (testEntry, bool) {
	// Find the longest `---` divider line in the body whose suffix matches.
	dividerLine := -1
	bestTotalLen := 0
	for j, line := range bodyLines {
		if delimLen, suffix, ok := parseDelimiterLine(line, '-'); ok && suffixMatches(firstSuffix, suffix) {
			totalLen := delimLen + len(suffix)
			// For ties prefer the later candidate, as an earlier same-length
			// `---` is a literal in the input.
			if totalLen >= bestTotalLen {
				dividerLine = j
				bestTotalLen = totalLen
			}
		}
	}
	if dividerLine < 0 {
		return testEntry{}, false
	}

	// Input: lines before the divider, with the trailing newline stripped.
	input := []byte(strings.Join(bodyLines[:dividerLine], ""))
	if n := len(input); n > 0 && input[n-1] == '\n' {
		input = input[:n-1]
	}
	if n := len(input); n > 0 && input[n-1] == '\r' {
		input = input[:n-1]
	}

	// Output: lines after the divider.
	outputStr := strings.Join(bodyLines[dividerLine+1:], "")

	var output string
	var hasFields bool
	if pending.attributes.cst {
		output = strings.TrimSpace(outputStr)
	} else {
		output, hasFields = normalizeSexpOutput(outputStr)
	}

	return testEntry{
		name:       pending.name,
		input:      input,
		output:     output,
		hasFields:  hasFields,
		attributes: pending.attributes,
	}, true
}
