package grammartest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/transit"
	golang "github.com/xo/transit/generate/backend/go"
	"github.com/xo/transit/inject"
)

// This file ports crates/cli/src/test_highlight.rs: test_highlights,
// test_highlight, get_highlight_positions, iterate_assertions and Failure
// (D80). It also ports the parts of crates/loader/src/loader.rs that they
// use: the entry of a grammar in tree-sitter.json, read_queries,
// highlight_config and language_configuration_for_injection_string. The
// loader of upstream knows each grammar that it finds on the machine, and
// the port knows the grammars of the module of the grammar package. It
// skips an injection whose name no grammar of the module matches.
//
// Upstream finds the grammar of a test file by its file type, and the port
// takes the grammar of the package. Upstream reads the names of the
// highlights before it highlights a file, so a name that a grammar injected
// in that file adds makes it panic. The port reads them after.

// Grammar is a grammar of the module of a grammar package, with what the
// highlight test needs of it.
type Grammar struct {
	Language *transit.Language
	// Queries holds the query files of the grammar that tree-sitter.json
	// names, such as queries/highlights.scm. It is the Queries of a grammar
	// package.
	Queries fs.FS
}

// Highlight runs the highlight test of the grammar with language and the
// query files fsys, such as the Queries of a grammar package, on each file
// of dir, such as testdata/highlight. It highlights each file as the
// highlighter of upstream does, and checks the assertions in its comments
// (D79, D80).
//
// The queries are the files that the nearest tree-sitter.json, in the
// working folder or in a folder above it, lists for the grammar under
// highlights, injections and locals, in that order, or
// queries/highlights.scm, queries/injections.scm and queries/locals.scm
// when a list is missing. Upstream reads each path from the folder of
// tree-sitter.json, and the test reads it from fsys. A file of another
// grammar comes from queries/<other grammar>/ of fsys (D84).
//
// others are the other grammars of the module. An injection gets the
// grammar of the module whose injection-regex in tree-sitter.json matches
// the most of its name, and an injection that no grammar matches is
// skipped. The grammar itself can match.
//
// Highlight is test_highlights.
func Highlight(t *testing.T, language *transit.Language, fsys fs.FS, dir string, others ...Grammar) {
	t.Helper()
	l, err := newLoader(Grammar{Language: language, Queries: fsys}, others)
	if err != nil {
		t.Fatal(err)
	}
	config, err := l.own.highlightConfig(&l.highlightNames)
	switch {
	case err != nil:
		t.Fatal(err)
	case config == nil:
		t.Fatalf("No highlighting config found for %s", language.Name())
	}
	testHighlights(t, l, config, dir)
}

// testHighlights runs the test of each file of dir as a subtest, and of
// each folder that is not empty as a group.
func testHighlights(t *testing.T, l *loader, config *highlightConfiguration, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if children, err := os.ReadDir(name); err == nil && len(children) > 0 {
				t.Run(e.Name(), func(t *testing.T) {
					testHighlights(t, l, config, name)
				})
				continue
			}
		}
		t.Run(e.Name(), func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := testHighlight(l, config, src); err != nil {
				t.Error(err)
			}
		})
	}
}

// highlightError is an assertion that fails: its position, the highlight
// that it expects, and the highlights at its position.
//
// highlightError is Failure.
type highlightError struct {
	row, column       int
	expectedHighlight string
	actualHighlights  []string
}

// Error returns the failure as upstream writes it.
//
// Error is the Display of Failure.
func (f *highlightError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Failure - row: %d, column: %d, expected highlight '%s', actual highlights: ", f.row, f.column, f.expectedHighlight)
	if len(f.actualHighlights) == 0 {
		b.WriteString("none.")
	}
	for i, h := range f.actualHighlights {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "'%s'", h)
	}
	return b.String()
}

// Unwrap makes the failure an errAssertion.
func (f *highlightError) Unwrap() error {
	return errAssertion
}

// highlightPosition is a highlight and the range that it covers.
type highlightPosition struct {
	start, end utf8Point
	highlight  int
}

// iterateAssertions checks each assertion against the highlights that span
// its position, and returns the number of assertions.
//
// iterateAssertions is iterate_assertions.
func iterateAssertions(assertions []assertion, highlights []highlightPosition, highlightNames []string) (int, error) {
	// Iterate through all of the highlighting assertions, checking each one against the
	// actual highlights.
	i := 0
	var actualHighlights []string
	for _, a := range assertions {
		position := a.position
		// Iterate through all of the highlights that start at or before this assertion's
		// position, looking for one that matches the assertion.
		actualHighlights = actualHighlights[:0]
		passed := false
		endColumn := position.column + a.length - 1
		for _, highlight := range highlights[i:] {
			// The assertions are ordered by position, so skip past all of the highlights that
			// end at or before this assertion's position.
			if highlight.end.compare(position) <= 0 {
				i++
				continue
			}
			if highlight.start.row > position.row || highlight.start.row == position.row && highlight.start.column > endColumn {
				break
			}

			// If the highlight matches the assertion, or if the highlight doesn't
			// match the assertion but it's negative, this test passes. Otherwise,
			// add this highlight to the list of actual highlights that span the
			// assertion's position, in order to generate an error message in the event
			// of a failure.
			highlightName := highlightNames[highlight.highlight]
			if (highlightName == a.expectedCaptureName) == a.negative {
				actualHighlights = append(actualHighlights, highlightName)
			} else {
				passed = true
				break
			}
		}

		if !passed {
			expected := a.expectedCaptureName
			if a.negative {
				expected = "!" + expected
			}
			return 0, &highlightError{
				row:               position.row,
				column:            endColumn,
				expectedHighlight: expected,
				actualHighlights:  actualHighlights,
			}
		}
	}

	return len(assertions), nil
}

// testHighlight highlights a file, and checks the assertions in its
// comments. It returns the number of assertions.
//
// testHighlight is test_highlight.
func testHighlight(l *loader, config *highlightConfiguration, source []byte) (int, error) {
	// Highlight the file, and parse out all of the highlighting assertions.
	highlights, err := getHighlightPositions(l, config, source)
	if err != nil {
		return 0, err
	}
	highlightNames := l.highlightNames
	assertions, err := parsePositionComments(l.parser, config.language, source)
	if err != nil {
		return 0, err
	}

	return iterateAssertions(assertions, highlights, highlightNames)
}

// getHighlightPositions highlights a file, and returns the range of each
// run of text and the innermost highlight over it.
//
// getHighlightPositions is get_highlight_positions.
func getHighlightPositions(l *loader, config *highlightConfiguration, sourceBytes []byte) ([]highlightPosition, error) {
	row, column, byteOffset := 0, 0, 0
	wasNewline := false
	var result []highlightPosition
	var highlightStack []int
	source := fromUTF8Lossy(sourceBytes)
	charOffset := 0
	nextChar := func() (int, rune, bool) {
		if charOffset >= len(source) {
			return 0, 0, false
		}
		c, size := utf8.DecodeRuneInString(source[charOffset:])
		i := charOffset
		charOffset += size
		return i, c, true
	}

	ctx := context.Background()
	layers, err := config.inject.Layers(ctx, l.parser, []byte(source), l.injectionConfig)
	if err != nil {
		return nil, fmt.Errorf("highlighting the source: %w", err)
	}
	it := newHighlightIter(ctx, []byte(source), layers, l.configs)
	defer it.close()
	for {
		event, ok := it.next()
		if !ok {
			break
		}
		switch event.kind {
		case eventHighlightStart:
			highlightStack = append(highlightStack, event.highlight)
		case eventHighlightEnd:
			highlightStack = highlightStack[:len(highlightStack)-1]
		case eventSource:
			startPosition := transit.Point{Row: row, Column: column}
			for byteOffset < event.end {
				if byteOffset <= event.start {
					startPosition = transit.Point{Row: row, Column: column}
				}
				i, c, ok := nextChar()
				if !ok {
					break
				}
				if wasNewline {
					row++
					column = 0
				} else {
					column += i - byteOffset
				}
				wasNewline = c == '\n'
				byteOffset = i
			}
			if n := len(highlightStack); n > 0 {
				utf8StartPosition := toUTF8Point(startPosition, []byte(source))
				utf8EndPosition := toUTF8Point(transit.Point{Row: row, Column: column}, []byte(source))
				result = append(result, highlightPosition{start: utf8StartPosition, end: utf8EndPosition, highlight: highlightStack[n-1]})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("highlighting the source: %w", err)
	}
	return result, nil
}

// loader holds the grammars of a module, as the Loader of upstream holds
// its language configurations.
type loader struct {
	parser   *transit.Parser
	grammars []*loaderGrammar
	// own is the first configuration of the grammar of the package.
	own *loaderGrammar
	// highlightNames holds each capture name of each configuration that the
	// loader made, as Loader holds them with use_all_highlight_names.
	highlightNames []string
	// configs gives the highlight configuration of the configuration of
	// inject of each one.
	configs map[*inject.Config]*highlightConfiguration
}

// loaderGrammar is a grammar of the loader.
//
// loaderGrammar is LanguageConfiguration.
type loaderGrammar struct {
	language       *transit.Language
	queries        fs.FS
	entry          treeSitterGrammar
	injectionRegex *regexp.Regexp
	configs        map[*inject.Config]*highlightConfiguration

	loaded bool
	config *highlightConfiguration
	err    error
}

// pathsJSON is a list of paths in tree-sitter.json: a path, a list of paths,
// or nothing.
//
// pathsJSON is PathsJSON.
type pathsJSON struct {
	paths []string
	set   bool
}

// UnmarshalJSON reads a path or a list of paths.
func (p *pathsJSON) UnmarshalJSON(b []byte) error {
	if string(bytes.TrimSpace(b)) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*p = pathsJSON{paths: []string{single}, set: true}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(b, &multiple); err != nil {
		return fmt.Errorf("reading a list of paths: %w", err)
	}
	*p = pathsJSON{paths: multiple, set: true}
	return nil
}

// newLoader makes the loader of the grammars of the module, with their
// entries in the nearest tree-sitter.json. own is the grammar of the
// package, and others are the other grammars of the module. As in the
// Loader of upstream, each entry is a configuration of its own, and an
// entry for the folder of a grammar, such as flow in the folder of tsx,
// gets the language of that grammar. An entry of a folder that no grammar
// holds is left out. With no entry for own, own gets an entry with no
// lists, which reads the default query files.
func newLoader(own Grammar, others []Grammar) (*loader, error) {
	m, err := readModule(own.Language.Name())
	if err != nil {
		return nil, err
	}
	return newLoaderFor(m, own, others)
}

// newLoaderFor is newLoader with the module m.
func newLoaderFor(m *module, own Grammar, others []Grammar) (*loader, error) {
	l := &loader{parser: transit.NewParser(), configs: map[*inject.Config]*highlightConfiguration{}}
	byFolder := map[string]Grammar{m.own: own}
	for _, g := range others {
		if f := golang.GrammarFolder(g.Language.Name()); f != m.own {
			byFolder[f] = g
		}
	}
	for _, e := range m.entries {
		g, ok := byFolder[e.folder()]
		if !ok {
			continue
		}
		lg := &loaderGrammar{language: g.Language, queries: g.Queries, entry: e, configs: l.configs}
		if e.InjectionRegex != "" {
			var err error
			if lg.injectionRegex, err = regexp.Compile(e.InjectionRegex); err != nil {
				return nil, fmt.Errorf("compiling the injection-regex of %s: %w", e.Name, err)
			}
		}
		l.grammars = append(l.grammars, lg)
		if l.own == nil && e.folder() == m.own {
			l.own = lg
		}
	}
	if l.own == nil {
		l.own = &loaderGrammar{language: own.Language, queries: own.Queries, configs: l.configs}
		l.grammars = append(l.grammars, l.own)
	}
	return l, nil
}

// injectionConfig returns the configuration of inject of the grammar whose
// injection-regex matches the most of an injected name, and false when no
// grammar matches or when the grammar has no highlight configuration.
//
// injectionConfig is highlight_config_for_injection_string, with
// language_configuration_for_injection_string.
func (l *loader) injectionConfig(name string) (*inject.Config, bool) {
	bestMatchLength := 0
	var best *loaderGrammar
	for _, g := range l.grammars {
		if g.injectionRegex == nil {
			continue
		}
		if m := g.injectionRegex.FindStringIndex(name); m != nil && m[1]-m[0] > bestMatchLength {
			best = g
			bestMatchLength = m[1] - m[0]
		}
	}
	if best == nil {
		return nil, false
	}
	// upstream logs an error and gives no configuration
	config, err := best.highlightConfig(&l.highlightNames)
	if err != nil || config == nil {
		return nil, false
	}
	return config.inject, true
}

// highlightConfig returns the highlight configuration of the grammar, which
// it makes once, and nil when the grammar has no highlight query. It adds
// the capture names of the configuration to names, and configures it with
// names.
//
// highlightConfig is LanguageConfiguration::highlight_config.
func (g *loaderGrammar) highlightConfig(names *[]string) (*highlightConfiguration, error) {
	if g.loaded {
		return g.config, g.err
	}
	g.loaded = true
	g.config, g.err = g.makeHighlightConfig(names)
	return g.config, g.err
}

// makeHighlightConfig reads the queries of the grammar and makes its
// highlight configuration.
func (g *loaderGrammar) makeHighlightConfig(names *[]string) (*highlightConfiguration, error) {
	highlightsQuery, err := readQueries(g.queries, g.entry.Highlights, "highlights.scm")
	if err != nil {
		return nil, err
	}
	injectionsQuery, err := readQueries(g.queries, g.entry.Injections, "injections.scm")
	if err != nil {
		return nil, err
	}
	localsQuery, err := readQueries(g.queries, g.entry.Locals, "locals.scm")
	if err != nil {
		return nil, err
	}
	if highlightsQuery == "" {
		return nil, nil
	}
	// upstream names the configuration by its entry
	name := g.entry.Name
	if name == "" {
		name = g.language.Name()
	}
	result, err := newHighlightConfiguration(g.language, name, highlightsQuery, injectionsQuery, localsQuery)
	if err != nil {
		return nil, err
	}
	for _, captureName := range result.query.CaptureNames() {
		if !slices.Contains(*names, captureName) {
			*names = append(*names, captureName)
		}
	}
	result.configure(*names)
	g.configs[result.inject] = result
	return result, nil
}

// readQueries joins the query files of a list of tree-sitter.json, in
// order, or reads queries/defaultPath when the list is missing. A default
// file that does not exist gives an empty query. A file of another grammar
// comes from queries/<other grammar>/ of the package (D84), as
// packageQueryPath says.
//
// readQueries is read_queries.
func readQueries(fsys fs.FS, paths pathsJSON, defaultPath string) (string, error) {
	if paths.set {
		var query strings.Builder
		for _, p := range paths.paths {
			b, err := fs.ReadFile(fsys, packageQueryPath(p))
			if err != nil {
				return "", fmt.Errorf("reading the query file %s: %w", p, err)
			}
			query.Write(b)
		}
		return query.String(), nil
	}
	b, err := fs.ReadFile(fsys, path.Join("queries", defaultPath))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("reading the query file queries/%s: %w", defaultPath, err)
	}
	return string(b), nil
}

// packageQueryPath returns the path in a grammar package of a query file
// that tree-sitter.json lists. A file of another grammar, such as
// node_modules/tree-sitter-javascript/queries/highlights.scm, is in the
// folder queries/<other grammar>/ of the package, such as
// queries/javascript/highlights.scm (D84). The name of the other grammar is
// the name of its repository without the prefix tree-sitter-, with each "-"
// replaced by "_", as in c_sharp. Any other path is the same in the
// package.
func packageQueryPath(p string) string {
	p = path.Clean(p)
	rest, ok := strings.CutPrefix(p, "node_modules/tree-sitter-")
	if !ok {
		return p
	}
	repo, file, ok := strings.Cut(rest, "/queries/")
	if !ok || strings.Contains(repo, "/") {
		return p
	}
	return path.Join("queries", strings.ReplaceAll(repo, "-", "_"), file)
}
