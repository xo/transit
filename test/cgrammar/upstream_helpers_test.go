package cgrammar

import (
	"context"
	"encoding/binary"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xo/transit"
)

// This file holds the helpers of the ported tests of parser_test.rs,
// tree_test.rs, node_test.rs, corpus_test.rs and pathological_test.rs of
// crates/cli/src/tests of upstream (D35). It ports the parts of
// tests/helpers/fixtures.rs, tests/helpers/edits.rs, fuzz/edits.rs,
// fuzz/random.rs and perform_edit of parse.rs that those tests use.

// urTestGrammarDirs returns the folder in generate/testdata that holds the
// grammar.json of a test grammar of upstream, and the folder of the test
// grammar in the checkout of upstream, which holds its scanner and its
// corpus. transit reads grammar.json only (D17), and the golden harness
// writes the grammar.json of each test grammar from its grammar.js.
func urTestGrammarDirs(root, name string) (string, string) {
	return filepath.Join(root, "generate", "testdata", name),
		filepath.Join(root, "tree-sitter", "test", "fixtures", "test_grammars", name)
}

// urTestFixtureLanguage is get_test_fixture_language: it builds and loads
// a test grammar of upstream, with its scanner.
func urTestFixtureLanguage(t *testing.T, name string) *transit.Language {
	t.Helper()
	root, _ := setup(t)
	jsonDir, grammarDir := urTestGrammarDirs(root, name)
	b, err := os.ReadFile(filepath.Join(jsonDir, "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	return urTestLanguage(t, string(b), grammarDir)
}

// urTestLanguage is generate_parser and get_test_language: it generates
// the parser of a grammar.json with the generator of transit, and builds and
// loads it. scannerDir is the folder of the scanner.c of the grammar, or "".
func urTestLanguage(t *testing.T, grammarJSON, scannerDir string) *transit.Language {
	t.Helper()
	_, cache := setup(t)
	so, name, err := BuildGrammarJSON(context.Background(), []byte(grammarJSON), scannerDir, cache)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(so, name)
	if err != nil {
		t.Fatal(err)
	}
	return g.Language
}

// urParser returns a parser with a language.
func urParser(t *testing.T, language *transit.Language) *transit.Parser {
	t.Helper()
	p := transit.NewParser()
	if err := p.SetLanguage(language); err != nil {
		t.Fatal(err)
	}
	return p
}

// urParse parses a text, and fails the test when the parse fails, as unwrap
// does in Rust.
func urParse[S string | []byte](t *testing.T, p *transit.Parser, src S, old *transit.Tree) *transit.Tree {
	t.Helper()
	tree, err := p.Parse(context.Background(), []byte(src), old)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// urParseInput parses the text of an input, and fails the test when the
// parse fails.
func urParseInput(t *testing.T, p *transit.Parser, in transit.Input, enc transit.Encoding, old *transit.Tree) *transit.Tree {
	t.Helper()
	tree, err := p.ParseInput(context.Background(), in, enc, old)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// urMust returns a function that returns the node of a lookup, and fails
// the test when the lookup finds no node, as unwrap does in Rust.
func urMust(t *testing.T) func(transit.Node, bool) transit.Node {
	t.Helper()
	return func(n transit.Node, ok bool) transit.Node {
		t.Helper()
		if !ok {
			t.Fatal("the lookup found no node")
		}
		return n
	}
}

// urEqual reports a value that differs from the value of the upstream test.
func urEqual[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// urSlice reports a slice that differs from the slice of the upstream test.
func urSlice[T comparable](t *testing.T, what string, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %#v, want %#v", what, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s = %#v, want %#v", what, got, want)
			return
		}
	}
}

// urSameNode reports a node that is not the node of the upstream test. It
// compares with Node.Equal, which is ts_node_eq (D68), as the PartialEq of
// Node in the Rust binding compares the ids of the nodes.
func urSameNode(t *testing.T, what string, got, want transit.Node) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s is %s at %d, want %s at %d", what, got.Kind(), got.StartByte(), want.Kind(), want.StartByte())
	}
}

// urPoint is Point::new.
func urPoint(row, column int) transit.Point {
	return transit.Point{Row: row, Column: column}
}

// urSimpleRange is simple_range of parser_test.rs.
func urSimpleRange(start, end int) transit.Range {
	return transit.Range{
		StartByte:  start,
		EndByte:    end,
		StartPoint: urPoint(0, start),
		EndPoint:   urPoint(0, end),
	}
}

// urInputFunc is an Input from a function, as the callback of
// parse_with_options.
type urInputFunc func(offset int, at transit.Point) []byte

// ReadAt calls the function.
func (f urInputFunc) ReadAt(offset int, at transit.Point) []byte {
	return f(offset, at)
}

// urBytesInput is an Input that gives the rest of a text from each offset,
// as the callback of parse_utf16_le gives the rest of a slice.
type urBytesInput []byte

// ReadAt returns the text from the offset.
func (b urBytesInput) ReadAt(offset int, _ transit.Point) []byte {
	if offset >= len(b) {
		return nil
	}
	return b[offset:]
}

// urUTF16Input is an Input from a function that gives code units of
// UTF-16, as the callback of parse_utf16_le_with_options and
// parse_utf16_be_with_options. As the Rust binding does, it gives the
// function the offset and the column in code units, and it gives the parser
// the code units as bytes in the byte order order.
type urUTF16Input struct {
	read  func(offset int, at transit.Point) []uint16
	order binary.AppendByteOrder
}

// ReadAt calls the function, and returns its code units as bytes.
func (in urUTF16Input) ReadAt(offset int, at transit.Point) []byte {
	units := in.read(offset/2, urPoint(at.Row, at.Column/2))
	out := make([]byte, 0, 2*len(units))
	for _, u := range units {
		out = in.order.AppendUint16(out, u)
	}
	return out
}

// urUTF16 returns a text in UTF-16 as bytes in the byte order order, as
// encode_utf16 with to_le or to_be gives it.
func urUTF16(s string, order binary.AppendByteOrder) []byte {
	var out []byte
	for _, u := range utf16.Encode([]rune(s)) {
		out = order.AppendUint16(out, u)
	}
	return out
}

// urReadRecorder is ReadRecorder of tests/helpers/edits.rs: an Input that
// gives one byte at a time, and records each offset that the parser reads.
type urReadRecorder struct {
	content     []byte
	indicesRead []int
}

// ReadAt is read.
func (r *urReadRecorder) ReadAt(offset int, _ transit.Point) []byte {
	if offset < len(r.content) {
		if i, found := slices.BinarySearch(r.indicesRead, offset); !found {
			r.indicesRead = slices.Insert(r.indicesRead, i, offset)
		}
		return r.content[offset : offset+1]
	}
	return nil
}

// stringsRead is strings_read. As upstream does, it drops the first offset
// after a gap.
func (r *urReadRecorder) stringsRead() []string {
	var result []string
	start, end, has := 0, 0, false
	for _, index := range r.indicesRead {
		if has {
			if end == index {
				end++
			} else {
				result = append(result, string(r.content[start:end]))
				has = false
			}
		} else {
			start, end, has = index, index+1, true
		}
	}
	if has {
		result = append(result, string(r.content[start:end]))
	}
	return result
}

// urEdit is Edit of fuzz/edits.rs.
type urEdit struct {
	position      int
	deletedLength int
	insertedText  []byte
}

// urInvertEdit is invert_edit.
func urInvertEdit(input []byte, edit urEdit) urEdit {
	position := edit.position
	removedContent := input[position : position+edit.deletedLength]
	return urEdit{
		position:      position,
		deletedLength: len(edit.insertedText),
		insertedText:  append([]byte(nil), removedContent...),
	}
}

// urGetRandomEdit is get_random_edit.
func urGetRandomEdit(rand *urRand, input []byte) urEdit {
	choice := rand.unsigned(10)
	switch {
	case choice < 2:
		// Insert text at end
		insertedText := rand.words(3)
		return urEdit{position: len(input), insertedText: insertedText}
	case choice < 5:
		// Delete text from the end
		deletedLength := min(rand.unsigned(30), len(input))
		return urEdit{position: len(input) - deletedLength, deletedLength: deletedLength}
	case choice < 8:
		// Insert at a random position
		position := rand.unsigned(len(input))
		wordCount := 1 + rand.unsigned(3)
		insertedText := rand.words(wordCount)
		return urEdit{position: position, insertedText: insertedText}
	default:
		// Replace at random position
		position := rand.unsigned(len(input))
		deletedLength := rand.unsigned(len(input) - position)
		wordCount := 1 + rand.unsigned(3)
		insertedText := rand.words(wordCount)
		return urEdit{position: position, deletedLength: deletedLength, insertedText: insertedText}
	}
}

// urPerformEdit is perform_edit of parse.rs: it applies an edit to the
// text and to the tree, and returns the InputEdit.
func urPerformEdit(t *testing.T, tree *transit.Tree, input *[]byte, edit urEdit) transit.InputEdit {
	t.Helper()
	startByte := edit.position
	oldEndByte := edit.position + edit.deletedLength
	newEndByte := edit.position + len(edit.insertedText)
	if oldEndByte > len(*input) {
		t.Fatalf("Failed to address an offset: %d", oldEndByte)
	}
	startPosition := urPositionForOffset(*input, startByte)
	oldEndPosition := urPositionForOffset(*input, oldEndByte)
	next := make([]byte, 0, len(*input)-edit.deletedLength+len(edit.insertedText))
	next = append(next, (*input)[:startByte]...)
	next = append(next, edit.insertedText...)
	next = append(next, (*input)[oldEndByte:]...)
	*input = next
	newEndPosition := urPositionForOffset(*input, newEndByte)
	e := transit.InputEdit{
		StartByte:   startByte,
		OldEndByte:  oldEndByte,
		NewEndByte:  newEndByte,
		StartPoint:  startPosition,
		OldEndPoint: oldEndPosition,
		NewEndPoint: newEndPosition,
	}
	tree.Edit(e)
	return e
}

// urPositionForOffset is position_for_offset of parse.rs.
func urPositionForOffset(input []byte, offset int) transit.Point {
	var result transit.Point
	last := 0
	for pos, c := range input[:offset] {
		if c == '\n' {
			result.Row++
			last = pos
		}
	}
	if result.Row > 0 {
		result.Column = offset - last - 1
	} else {
		result.Column = offset
	}
	return result
}

// urOperators is OPERATORS of fuzz/random.rs.
const urOperators = "+-<>()*/&|!,.%"

// urAlphanumeric is the set of the distribution Alphanumeric of the Rust
// crate rand.
const urAlphanumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// urRand is Rand of fuzz/random.rs. It uses the PCG of math/rand/v2 in
// place of StdRng of the crate rand, so a seed gives other edits than in
// upstream.
type urRand struct {
	r *rand.Rand
}

// urNewRand is Rand::new.
func urNewRand(seed uint64) *urRand {
	return &urRand{r: rand.New(rand.NewPCG(seed, 0))}
}

// unsigned returns a number from 0 to maxValue, both included.
func (r *urRand) unsigned(maxValue int) int {
	return r.r.IntN(maxValue + 1)
}

// words returns up to maxCount words of letters and digits, or operators.
func (r *urRand) words(maxCount int) []byte {
	wordCount := r.unsigned(maxCount)
	result := make([]byte, 0, 2*wordCount)
	for i := range wordCount {
		if i > 0 {
			if r.unsigned(5) == 0 {
				result = append(result, '\n')
			} else {
				result = append(result, ' ')
			}
		}
		if r.unsigned(3) == 0 {
			index := r.unsigned(len(urOperators) - 1)
			result = append(result, urOperators[index])
		} else {
			for range r.unsigned(8) {
				result = append(result, urAlphanumeric[r.r.IntN(len(urAlphanumeric))])
			}
		}
	}
	return result
}

// urProgress is a context whose Err calls a function, in place of the
// progress callback of ParseOptions. The parser reads Err where C calls the
// callback (D25), so a function that returns true stops the parse, as a
// callback that returns ControlFlow::Break does. The Go API has no
// ParseState, so the function gets no state.
type urProgress struct {
	fn   func() bool
	once sync.Once
	done chan struct{}
}

// urProgressContext returns a context whose Err calls fn.
func urProgressContext(fn func() bool) context.Context {
	return &urProgress{fn: fn, done: make(chan struct{})}
}

// Deadline returns no deadline.
func (c *urProgress) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Done returns a channel that closes when the function returns true.
func (c *urProgress) Done() <-chan struct{} {
	return c.done
}

// Err returns context.Canceled once the function returns true. The
// parser reads Err again for the error that it returns, and that read does
// not call the function.
func (c *urProgress) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
	}
	if c.fn() {
		c.once.Do(func() { close(c.done) })
		return context.Canceled
	}
	return nil
}

// Value returns nil.
func (c *urProgress) Value(any) any {
	return nil
}
