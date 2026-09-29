package cgrammar

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Example is one test of a corpus file: its name and the text that it
// parses.
type Example struct {
	File  string
	Name  string
	Input []byte
}

// ReadCorpus reads the tests of each file in the folder dir, such as
// test/corpus of a grammar, and its subfolders, in the order of their paths.
//
// It splits a file as parse_test_content of crates/cli/src/test.rs of
// upstream does: a header of `===` lines with a name, and a body with the
// input, a `---` line and the expected tree. The tests compare two runtimes
// on the same input, so ReadCorpus keeps only the input.
func ReadCorpus(dir string) ([]Example, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the corpus in %s: %w", dir, err)
	}
	slices.Sort(files)
	var out []Example
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading the corpus file %s: %w", file, err)
		}
		for _, e := range parseCorpus(string(b)) {
			e.File = file
			out = append(out, e)
		}
	}
	return out, nil
}

// delimiter is parse_delimiter_line: a line that starts with at least three
// of c, and the rest of the line after them.
func delimiter(line string, c byte) (int, string, bool) {
	n := 0
	for n < len(line) && line[n] == c {
		n++
	}
	if n < 3 {
		return 0, "", false
	}
	return n, strings.TrimRight(line[n:], "\r\n"), true
}

// suffixMatches is suffix_matches.
func suffixMatches(firstSuffix *string, suffix string) bool {
	if firstSuffix == nil {
		return suffix == ""
	}
	return suffix != "" && *firstSuffix == suffix
}

// parseCorpus is parse_test_content, for the inputs only.
func parseCorpus(content string) []Example {
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Determine the suffix from the first `===` line in the file.
	var firstSuffix *string
	for _, line := range lines {
		if _, suffix, ok := delimiter(line, '='); ok && suffix != "" {
			firstSuffix = &suffix
			break
		}
	}

	var out []Example
	type pending struct {
		name      string
		bodyStart int
	}
	var prev *pending
	for i := 0; i < len(lines); {
		name, bodyStart, ok := parseHeader(lines, firstSuffix, i)
		if !ok {
			i++
			continue
		}
		if prev != nil {
			if input, ok := corpusInput(lines[prev.bodyStart:i], firstSuffix); ok {
				out = append(out, Example{Name: prev.name, Input: input})
			}
		}
		prev = &pending{name: name, bodyStart: bodyStart}
		i = bodyStart
	}
	if prev != nil {
		if input, ok := corpusInput(lines[prev.bodyStart:], firstSuffix); ok {
			out = append(out, Example{Name: prev.name, Input: input})
		}
	}
	return out
}

// parseHeader is parse_header: the name of the test whose header starts at
// the line start, and the first line of its body.
func parseHeader(lines []string, firstSuffix *string, start int) (string, int, bool) {
	if _, suffix, ok := delimiter(lines[start], '='); !ok || !suffixMatches(firstSuffix, suffix) {
		return "", 0, false
	}
	var name []string
	seenMarker := false
	i := start + 1
	for ; i < len(lines); i++ {
		if _, suffix, ok := delimiter(lines[i], '='); ok && suffixMatches(firstSuffix, suffix) {
			break
		}
		trimmed := strings.TrimSpace(lines[i])
		// Reject a blank line in the name region so a literal `===` inside a
		// test body can't be mistaken for an opening delimiter.
		if trimmed == "" && !seenMarker {
			return "", 0, false
		}
		if strings.HasPrefix(trimmed, ":") {
			seenMarker = true
			continue
		}
		if trimmed != "" {
			name = append(name, trimmed)
		}
	}
	if i >= len(lines) {
		return "", 0, false
	}
	return strings.Join(name, " "), i + 1, true
}

// corpusInput is the input part of build_test_entry: the lines before the
// longest `---` line, with the last newline removed.
func corpusInput(body []string, firstSuffix *string) ([]byte, bool) {
	best, bestLen := -1, 0
	for j, line := range body {
		if n, suffix, ok := delimiter(line, '-'); ok && suffixMatches(firstSuffix, suffix) {
			// For ties prefer the later candidate, as an earlier same-length
			// `---` is a literal in the input.
			if total := n + len(suffix); total >= bestLen {
				best, bestLen = j, total
			}
		}
	}
	if best < 0 {
		return nil, false
	}
	input := []byte(strings.Join(body[:best], ""))
	input = bytes.TrimSuffix(input, []byte("\n"))
	input = bytes.TrimSuffix(input, []byte("\r"))
	return input, true
}
