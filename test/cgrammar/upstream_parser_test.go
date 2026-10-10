package cgrammar

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xo/transit"
	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
)

// This file ports crates/cli/src/tests/parser_test.rs of upstream (D35).
//
// A progress callback of ParseOptions is a context whose Err calls a
// function (urProgressContext), because the parser reads Err where C calls
// the callback. The Go API has no ParseState, so a test that reads
// current_byte_offset reads the offset of the last read of its input in
// place of it. The attribute retry is urRetry, and allocations::record is
// dropped, because Go has no allocator to count.
//
// The crate encoding_rs is not a package of Go, so test_decode_cp1252 and
// test_decode_macintosh encode the one character of their text that is not
// ASCII by hand.

// urRustSexp is the tree that three tests expect for "pub fn foo() {...}"
// with an integer literal in the body.
const urRustSexp = "(source_file (function_item (visibility_modifier) name: (identifier) parameters: (parameters) body: (block (integer_literal))))"

// urRetry is the attribute retry of upstream: it runs a test again while
// it fails, up to count more times, and reports the error of the last run.
func urRetry(t *testing.T, count int, run func() error) {
	t.Helper()
	var err error
	for range count + 1 {
		if err = run(); err == nil {
			return
		}
	}
	t.Error(err)
}

func TestParsingSimpleString(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParse(t, parser, `
        struct Stuff {}
        fn main() {}
    `, nil)

	rootNode := tree.RootNode()
	urEqual(t, "kind", rootNode.Kind(), "source_file")

	urEqual(t, "to_sexp", rootNode.String(),
		"(source_file "+
			"(struct_item name: (type_identifier) body: (field_declaration_list)) "+
			"(function_item name: (identifier) parameters: (parameters) body: (block)))")

	structNode := must(rootNode.Child(0))
	urEqual(t, "kind", structNode.Kind(), "struct_item")
}

func TestParsingWithLogging(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	type message struct {
		logType transit.LogType
		text    string
	}
	var messages []message
	parser.SetLogger(func(logType transit.LogType, text string) {
		messages = append(messages, message{logType, text})
	})

	urParse(t, parser, `
        struct Stuff {}
        fn main() {}
    `, nil)

	if !slices.Contains(messages, message{transit.LogParse, "reduce sym:struct_item, child_count:3"}) {
		t.Error("no parse message: reduce sym:struct_item, child_count:3")
	}
	if !slices.Contains(messages, message{transit.LogLex, "skip character:' '"}) {
		t.Error("no lex message: skip character:' '")
	}

	rowStartsFrom0 := false
	for _, m := range messages {
		if strings.Contains(m.text, "row:0") {
			rowStartsFrom0 = true
			break
		}
	}
	urEqual(t, "row_starts_from_0", rowStartsFrom0, true)
}

func TestParsingWithDebugGraphEnabled(t *testing.T) {
	hasZeroIndexedRow := func(s string) bool { return strings.Contains(s, "position: 0,") }

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	var debugGraphFile bytes.Buffer
	parser.PrintDotGraphs(&debugGraphFile)
	urParse(t, parser, "const zero = 0", nil)

	for line := range strings.Lines(debugGraphFile.String()) {
		if hasZeroIndexedRow(line) {
			t.Errorf("Graph log output includes zero-indexed row: %s", line)
		}
	}
}

func TestParsingWithCustomUTF8Input(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	lines := []string{"pub fn foo() {", "  1", "}"}

	tree := urParseInput(t, parser, urInputFunc(func(_ int, position transit.Point) []byte {
		row := position.Row
		column := position.Column
		if row < len(lines) {
			if column < len(lines[row]) {
				return []byte(lines[row][column:])
			}
			return []byte("\n")
		}
		return nil
	}), transit.EncodingUTF8, nil)

	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(),
		"(source_file "+
			"(function_item "+
			"(visibility_modifier) "+
			"name: (identifier) "+
			"parameters: (parameters) "+
			"body: (block (integer_literal))))")
	urEqual(t, "kind", root.Kind(), "source_file")
	urEqual(t, "has_error", root.HasError(), false)
	urEqual(t, "child(0).kind", must(root.Child(0)).Kind(), "function_item")
}

// urParseUTF16Lines parses lines of UTF-16 with a callback that gives the
// rest of a line, or a newline at its end, as the tests of custom UTF-16
// input do.
func urParseUTF16Lines(t *testing.T, parser *transit.Parser, order binary.AppendByteOrder, enc transit.Encoding) *transit.Tree {
	t.Helper()
	var lines [][]uint16
	for _, s := range []string{"pub fn foo() {", "  1", "}"} {
		lines = append(lines, utf16.Encode([]rune(s)))
	}

	newline := []uint16{'\n'}

	return urParseInput(t, parser, urUTF16Input{
		read: func(_ int, position transit.Point) []uint16 {
			row := position.Row
			column := position.Column
			if row < len(lines) {
				if column < len(lines[row]) {
					return lines[row][column:]
				}
				return newline
			}
			return nil
		},
		order: order,
	}, enc, nil)
}

func TestParsingWithCustomUTF16leInput(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParseUTF16Lines(t, parser, binary.LittleEndian, transit.EncodingUTF16LE)

	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(), urRustSexp)
	urEqual(t, "kind", root.Kind(), "source_file")
	urEqual(t, "has_error", root.HasError(), false)
	urEqual(t, "child(0).kind", must(root.Child(0)).Kind(), "function_item")
}

func TestParsingWithCustomUTF16BeInput(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParseUTF16Lines(t, parser, binary.BigEndian, transit.EncodingUTF16BE)
	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(), urRustSexp)
	urEqual(t, "kind", root.Kind(), "source_file")
	urEqual(t, "has_error", root.HasError(), false)
	urEqual(t, "child(0).kind", must(root.Child(0)).Kind(), "function_item")
}

func TestUTF16DecodesSurrogatePairs(t *testing.T) {
	language := urTestFixtureLanguage(t, "utf16_surrogate_oob")
	parser := urParser(t, language)

	le := binary.LittleEndian.AppendUint16(binary.LittleEndian.AppendUint16(nil, 0xD83D), 0xDE00)
	tree := urParseInput(t, parser, urBytesInput(le), transit.EncodingUTF16LE, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(program (supplementary))")

	be := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, 0xD83D), 0xDE00)
	tree = urParseInput(t, parser, urBytesInput(be), transit.EncodingUTF16BE, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(program (supplementary))")
}

func TestUTF16DecodeDoesNotReadOOB(t *testing.T) {
	// Test for a buffer over-read in ts_decode_utf16_le/be when a lead surrogate
	// is the last code unit in a chunk. The test grammar's external scanner
	// distinguishes surrogate code points from supplementary-plane characters,
	// making the over-read directly observable in the parse tree.
	//
	// Buffer layout:
	//   buf[0] = 0xD83E  (lead surrogate)
	//   buf[1] = 0xDD8B  (POISON: fake trail surrogate, adjacent in memory)
	//
	// The callback returns only buf[0..1] (one code unit = 2 bytes).
	//
	// When functioning correctly, this test passes a length of 2 bytes, which is
	// interpreted as 2/2 = 1 code unit, and thus doesn't over-read into the "poison"
	// fake trail surrogate. If an over-read does occur, the scanner sees a
	// supplementary token.
	language := urTestFixtureLanguage(t, "utf16_surrogate_oob")
	parser := urParser(t, language)

	buf := []uint16{
		0xD83E, // lead surrogate (the only "visible" code unit)
		0xDD8B, // POISON: adjacent in Vec memory, past the chunk
	}
	urEqual(t, "from_utf16", string([]rune{0x1F98B}), "🦋")

	// The bytes of buf in little endian, with the poison in the capacity of
	// the chunk that the callback returns.
	raw := binary.LittleEndian.AppendUint16(binary.LittleEndian.AppendUint16(nil, buf[0]), buf[1])
	callback := urInputFunc(func(offset int, _ transit.Point) []byte {
		// only expose buf[0], never buf[1]
		if offset >= 2 {
			return nil
		}
		return raw[0:2]
	})

	tree := urParseInput(t, parser, callback, transit.EncodingUTF16LE, nil)

	root := tree.RootNode()

	// Correct: scanner sees raw surrogate (0xD83E) -> `surrogate` node
	// Incorrect: scanner sees supplementary (U+1F98B, aka 🦋) -> `supplementary` node
	if got := root.String(); got != "(program (surrogate))" {
		t.Errorf("to_sexp = %q: buffer over-read: decoder read past chunk boundary and formed a "+
			"supplementary character from OOB adjacent memory", got)
	}
}

func TestParsingWithCallbackReturningOwnedStrings(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	text := []byte("pub fn foo() { 1 }")

	tree := urParseInput(t, parser, urInputFunc(func(i int, _ transit.Point) []byte {
		return slices.Clone(text[i:])
	}), transit.EncodingUTF8, nil)

	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(), urRustSexp)
}

func TestParsingTextWithByteOrderMark(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	// Parse UTF16 text with a BOM
	tree := urParseInput(t, parser, urBytesInput(urUTF16("\uFEFFfn a() {}", binary.LittleEndian)), transit.EncodingUTF16LE, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(source_file (function_item name: (identifier) parameters: (parameters) body: (block)))")
	urEqual(t, "start_byte", tree.RootNode().StartByte(), 2)

	// Parse UTF8 text with a BOM
	tree = urParse(t, parser, "\uFEFFfn a() {}", nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(source_file (function_item name: (identifier) parameters: (parameters) body: (block)))")
	urEqual(t, "start_byte", tree.RootNode().StartByte(), 3)

	// Edit the text, inserting a character before the BOM. The BOM is now an error.
	tree.Edit(transit.InputEdit{
		StartByte:   0,
		OldEndByte:  0,
		NewEndByte:  1,
		StartPoint:  urPoint(0, 0),
		OldEndPoint: urPoint(0, 0),
		NewEndPoint: urPoint(0, 1),
	})
	tree = urParse(t, parser, " \uFEFFfn a() {}", tree)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(source_file (ERROR (UNEXPECTED 65279)) (function_item name: (identifier) parameters: (parameters) body: (block)))")
	urEqual(t, "start_byte", tree.RootNode().StartByte(), 1)

	// Edit the text again, putting the BOM back at the beginning.
	tree.Edit(transit.InputEdit{
		StartByte:   0,
		OldEndByte:  1,
		NewEndByte:  0,
		StartPoint:  urPoint(0, 0),
		OldEndPoint: urPoint(0, 1),
		NewEndPoint: urPoint(0, 0),
	})
	tree = urParse(t, parser, "\uFEFFfn a() {}", tree)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(source_file (function_item name: (identifier) parameters: (parameters) body: (block)))")
	urEqual(t, "start_byte", tree.RootNode().StartByte(), 3)
}

func TestParsingInvalidCharsAtEOF(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "json").Language)
	tree := urParse(t, parser, []byte("\xdf"), nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(document (ERROR (UNEXPECTED INVALID)))")
}

func TestParsingUnexpectedNullCharactersWithinSource(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	tree := urParse(t, parser, []byte("var \x00 something;"), nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		`(program (variable_declaration (ERROR (UNEXPECTED '\0')) (variable_declarator name: (identifier))))`)
}

func TestParsingEndsWhenInputCallbackReturnsEmpty(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	i := 0
	source := []byte("abcdefghijklmnoqrs")
	tree := urParseInput(t, parser, urInputFunc(func(offset int, _ transit.Point) []byte {
		i++
		if offset >= 6 {
			return nil
		}
		return source[offset:min(len(source), offset+3)]
	}), transit.EncodingUTF8, nil)
	urEqual(t, "end_byte", tree.RootNode().EndByte(), 6)
}

// Incremental parsing

func TestParsingAfterEditingBeginningOfCode(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	code := []byte("123 + 456 * (10 + x);")
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (binary_expression "+
			"left: (number) "+
			"right: (binary_expression left: (number) right: (parenthesized_expression "+
			"(binary_expression left: (number) right: (identifier)))))))")

	urPerformEdit(t, tree, &code, urEdit{
		position:      3,
		deletedLength: 0,
		insertedText:  []byte(" || 5"),
	})

	recorder := &urReadRecorder{content: code}
	tree = urParseInput(t, parser, recorder, transit.EncodingUTF8, tree)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (binary_expression "+
			"left: (number) "+
			"right: (binary_expression "+
			"left: (number) "+
			"right: (binary_expression "+
			"left: (number) "+
			"right: (parenthesized_expression (binary_expression left: (number) right: (identifier))))))))")

	urSlice(t, "strings_read", recorder.stringsRead(), []string{"123 || 5 "})
}

func TestParsingAfterEditingEndOfCode(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	code := []byte("x * (100 + abc);")
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (binary_expression "+
			"left: (identifier) "+
			"right: (parenthesized_expression (binary_expression left: (number) right: (identifier))))))")

	position := len(code) - 2
	urPerformEdit(t, tree, &code, urEdit{
		position:      position,
		deletedLength: 0,
		insertedText:  []byte(".d"),
	})

	recorder := &urReadRecorder{content: code}
	tree = urParseInput(t, parser, recorder, transit.EncodingUTF8, tree)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program (expression_statement (binary_expression "+
			"left: (identifier) "+
			"right: (parenthesized_expression (binary_expression "+
			"left: (number) "+
			"right: (member_expression "+
			"object: (identifier) "+
			"property: (property_identifier)))))))")

	urSlice(t, "strings_read", recorder.stringsRead(), []string{" * ", "abc.d)"})
}

func TestParsingEmptyFileWithReusedTree(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParse(t, parser, "", nil)
	urParse(t, parser, "", tree)

	tree = urParse(t, parser, "\n  ", nil)
	urParse(t, parser, "\n  ", tree)
}

func TestParsingAfterEditingTreeThatDependsOnColumnValues(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "uses_current_column"))

	code := []byte(`
a = b
c = do d
       e + f
       g
h + i
    `)
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(block "+
			"(binary_expression (identifier) (identifier)) "+
			"(binary_expression (identifier) (do_expression (block (identifier) (binary_expression (identifier) (identifier)) (identifier)))) "+
			"(binary_expression (identifier) (identifier)))")

	urPerformEdit(t, tree, &code, urEdit{
		position:      8,
		deletedLength: 0,
		insertedText:  []byte("1234"),
	})

	urEqual(t, "code", string(code), `
a = b
c1234 = do d
       e + f
       g
h + i
    `)

	recorder := &urReadRecorder{content: code}
	tree = urParseInput(t, parser, recorder, transit.EncodingUTF8, tree)

	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(block "+
			"(binary_expression (identifier) (identifier)) "+
			"(binary_expression (identifier) (do_expression (block (identifier)))) "+
			"(binary_expression (identifier) (identifier)) "+
			"(identifier) "+
			"(binary_expression (identifier) (identifier)))")

	urSlice(t, "strings_read", recorder.stringsRead(), []string{"\nc1234 = do d\n       e + f\n       g\n"})
}

func TestParsingAfterEditingTreeThatDependsOnColumnPosition(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "depends_on_column"))

	code := []byte("\n x")
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(x_is_at (odd_column))")

	urPerformEdit(t, tree, &code, urEdit{
		position:      1,
		deletedLength: 0,
		insertedText:  []byte(" "),
	})

	urEqual(t, "code", string(code), "\n  x")

	recorder := &urReadRecorder{content: code}
	tree = urParseInput(t, parser, recorder, transit.EncodingUTF8, tree)

	urEqual(t, "to_sexp", tree.RootNode().String(), "(x_is_at (even_column))")
	urSlice(t, "strings_read", recorder.stringsRead(), []string{"\n  x"})

	urPerformEdit(t, tree, &code, urEdit{
		position:      1,
		deletedLength: 0,
		insertedText:  []byte("\n"),
	})

	urEqual(t, "code", string(code), "\n\n  x")

	recorder = &urReadRecorder{content: code}
	tree = urParseInput(t, parser, recorder, transit.EncodingUTF8, tree)

	urEqual(t, "to_sexp", tree.RootNode().String(), "(x_is_at (even_column))")
	urSlice(t, "strings_read", recorder.stringsRead(), []string{"\n\n  x"})
}

// TestParsingAfterFailedExternalScanThatDependsOnColumn is
// test_parsing_after_failed_external_scan_that_depends_on_column of
// parser_test.rs.
func TestParsingAfterFailedExternalScanThatDependsOnColumn(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "depends_on_column_failed_scan"))

	code := []byte("ax")
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(document (letter) (tail))")

	urPerformEdit(t, tree, &code, urEdit{
		position:      0,
		deletedLength: 1,
		insertedText:  []byte("\n"),
	})

	incremental := urParse(t, parser, code, tree)
	fresh := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", fresh.RootNode().String(), "(document (newline) (head))")
	urEqual(t, "to_sexp", incremental.RootNode().String(), fresh.RootNode().String())
}

func TestParsingAfterDetectingErrorInTheMiddleOfAStringToken(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "python").Language)

	source := []byte("a = b, 'c, d'")
	tree := urParse(t, parser, source, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(module (expression_statement (assignment left: (identifier) right: (expression_list (identifier) (string (string_start) (string_content) (string_end))))))")

	// Delete a suffix of the source code, starting in the middle of the string
	// literal, after some whitespace. With this deletion, the remaining string
	// content: "c, " looks like two valid python tokens: an identifier and a comma.
	// When this edit is undone, in order correctly recover the original tree, the
	// parser needs to remember that before matching the `c` as an identifier, it
	// lookahead ahead several bytes, trying to find the closing quotation mark in
	// order to match the "string content" node.
	editIx := bytes.Index(source, []byte("d'"))
	edit := urEdit{
		position:      editIx,
		deletedLength: len(source) - editIx,
	}
	undo := urInvertEdit(source, edit)

	tree2 := tree.Copy()
	urPerformEdit(t, tree2, &source, edit)
	tree2 = urParse(t, parser, source, tree2)
	urEqual(t, "has_error", tree2.RootNode().HasError(), true)

	tree3 := tree2.Copy()
	urPerformEdit(t, tree3, &source, undo)
	tree3 = urParse(t, parser, source, tree3)
	urEqual(t, "to_sexp", tree3.RootNode().String(), tree.RootNode().String())
}

// Thread safety

// TestParsingOnMultipleThreads reads parser_test.rs from the checkout of
// upstream, as include_str! does.
func TestParsingOnMultipleThreads(t *testing.T) {
	root, _ := setup(t)
	// Parse this source file so that each thread has a non-trivial amount of
	// work to do.
	b, err := os.ReadFile(filepath.Join(root, "tree-sitter", "crates", "cli", "src", "tests", "parser_test.rs"))
	if err != nil {
		t.Fatal(err)
	}
	thisFileSource := string(b)

	language := fixtureGrammar(t, "rust").Language
	parser := urParser(t, language)
	tree := urParse(t, parser, thisFileSource, nil)

	trees := make([]*transit.Tree, 4)
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for threadID := 1; threadID < 5; threadID++ {
		treeClone := tree.Copy()
		wg.Go(func() {
			// For each thread, prepend a different number of declarations to the
			// source code.
			prependLineCount := 2 * threadID
			prependedSource := strings.Repeat("struct X {}\n\n", threadID)

			treeClone.Edit(transit.InputEdit{
				StartByte:   0,
				OldEndByte:  0,
				NewEndByte:  len(prependedSource),
				StartPoint:  urPoint(0, 0),
				OldEndPoint: urPoint(0, 0),
				NewEndPoint: urPoint(prependLineCount, 0),
			})
			prependedSource += thisFileSource

			// Reparse using the old tree as a starting point.
			parser := transit.NewParser()
			if err := parser.SetLanguage(language); err != nil {
				errs[threadID-1] = err
				return
			}
			trees[threadID-1], errs[threadID-1] = parser.Parse(context.Background(), []byte(prependedSource), treeClone)
		})
	}
	wg.Wait()

	// Check that the trees have the expected relationship to one another.
	var childCountDifferences []int
	for i, tr := range trees {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		childCountDifferences = append(childCountDifferences, tr.RootNode().ChildCount()-tree.RootNode().ChildCount())
	}

	urSlice(t, "child_count_differences", childCountDifferences, []int{1, 2, 3, 4})
}

// TestParsingCancelledByAnotherThread cancels a context from another
// goroutine, in place of the cancellation flag that the progress callback
// reads.
func TestParsingCancelledByAnotherThread(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	// Long input - parsing succeeds
	tree, err := parser.ParseInput(ctx, urInputFunc(func(offset int, _ transit.Point) []byte {
		switch {
		case offset == 0:
			return []byte(" [")
		case offset >= 20000:
			return []byte("")
		default:
			return []byte("0,")
		}
	}), transit.EncodingUTF8, nil)
	if err != nil || tree == nil {
		t.Fatalf("the long input gave no tree: %v", err)
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
		close(done)
	}()

	// Infinite input
	tree, err = parser.ParseInput(ctx, urInputFunc(func(offset int, _ transit.Point) []byte {
		time.Sleep(10 * time.Millisecond)
		if offset == 0 {
			return []byte(" [")
		}
		return []byte("0,")
	}), transit.EncodingUTF8, nil)

	// Parsing returns None because it was cancelled.
	<-done
	if tree != nil {
		t.Error("the canceled parse gave a tree")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the error is %v, want context.Canceled", err)
	}
}

// Timeouts

// urInfiniteArray is the input of an infinitely-long array of JSON.
var urInfiniteArray = urInputFunc(func(offset int, _ transit.Point) []byte {
	if offset == 0 {
		return []byte(" [")
	}
	return []byte(",0")
})

// urAfter returns a context that stops a parse when the time since start
// passes d, as a progress callback that reads start_time.elapsed().
func urAfter(start time.Time, d time.Duration) context.Context {
	return urProgressContext(func() bool { return time.Since(start) > d })
}

func TestParsingWithATimeout(t *testing.T) {
	must := urMust(t)
	language := fixtureGrammar(t, "json").Language
	urRetry(t, 10, func() error {
		parser := urParser(t, language)

		// Parse an infinitely-long array, but pause after 1ms of processing.
		startTime := time.Now()
		tree, _ := parser.ParseInput(urAfter(startTime, 1000*time.Microsecond), urInfiniteArray, transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the first parse gave a tree")
		}
		if e := time.Since(startTime); e >= 2000*time.Microsecond {
			return fmt.Errorf("the first parse stopped after %v, want less than 2ms", e)
		}

		// Continue parsing, but pause after 1 ms of processing.
		startTime = time.Now()
		tree, _ = parser.ParseInput(urAfter(startTime, 5000*time.Microsecond), urInfiniteArray, transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the second parse gave a tree")
		}
		if e := time.Since(startTime); e <= 100*time.Microsecond || e >= 10000*time.Microsecond {
			return fmt.Errorf("the second parse stopped after %v, want from 100µs to 10ms", e)
		}

		// Finish parsing
		tree = urParseInput(t, parser, urInputFunc(func(offset int, _ transit.Point) []byte {
			switch {
			case offset >= 5001:
				return []byte("")
			case offset == 5000:
				return []byte("]")
			default:
				return []byte(",0")
			}
		}), transit.EncodingUTF8, nil)
		if kind := must(tree.RootNode().Child(0)).Kind(); kind != "array" {
			return fmt.Errorf("child(0).kind = %q, want array", kind)
		}
		return nil
	})
}

// urJSONCode is the code of the tests of a timeout and a reset.
const urJSONCode = `["ok", 1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32]`

// urJSONNullCode is urJSONCode with null in place of "ok".
const urJSONNullCode = `[null, 1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32]`

// urFirstElementKind returns the kind of the first element of the array of
// a tree of JSON.
func urFirstElementKind(t *testing.T, tree *transit.Tree) string {
	t.Helper()
	must := urMust(t)
	return must(must(tree.RootNode().NamedChild(0)).NamedChild(0)).Kind()
}

func TestParsingWithATimeoutAndAReset(t *testing.T) {
	language := fixtureGrammar(t, "json").Language
	urRetry(t, 10, func() error {
		parser := urParser(t, language)

		startTime := time.Now()
		code := urJSONCode
		tree, _ := parser.ParseInput(urAfter(startTime, 5*time.Microsecond), urBytesInput(code), transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the first parse gave a tree")
		}

		// Without calling reset, the parser continues from where it left off, so
		// it does not see the changes to the beginning of the source code.
		tree = urParse(t, parser, urJSONNullCode, nil)
		if kind := urFirstElementKind(t, tree); kind != "string" {
			return fmt.Errorf("the first element is %s, want string", kind)
		}

		startTime = time.Now()
		tree, _ = parser.ParseInput(urAfter(startTime, 5*time.Microsecond), urBytesInput(code), transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the second parse gave a tree")
		}

		// By calling reset, we force the parser to start over from scratch so
		// that it sees the changes to the beginning of the source code.
		parser.Reset()
		tree = urParse(t, parser, urJSONNullCode, nil)
		if kind := urFirstElementKind(t, tree); kind != "null" {
			return fmt.Errorf("the first element after reset is %s, want null", kind)
		}
		return nil
	})
}

func TestParsingWithATimeoutAndImplicitReset(t *testing.T) {
	javascript := fixtureGrammar(t, "javascript").Language
	json := fixtureGrammar(t, "json").Language
	urRetry(t, 10, func() error {
		parser := urParser(t, javascript)

		code := urJSONCode
		startTime := time.Now()
		tree, _ := parser.ParseInput(urAfter(startTime, 5*time.Microsecond), urBytesInput(code), transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the first parse gave a tree")
		}

		// Changing the parser's language implicitly resets, discarding
		// the previous partial parse.
		if err := parser.SetLanguage(json); err != nil {
			return err
		}
		tree = urParse(t, parser, urJSONNullCode, nil)
		if kind := urFirstElementKind(t, tree); kind != "null" {
			return fmt.Errorf("the first element is %s, want null", kind)
		}
		return nil
	})
}

func TestParsingWithTimeoutAndNoCompletion(t *testing.T) {
	language := fixtureGrammar(t, "javascript").Language
	urRetry(t, 10, func() error {
		parser := urParser(t, language)

		code := urJSONCode
		startTime := time.Now()
		tree, _ := parser.ParseInput(urAfter(startTime, 5*time.Microsecond), urBytesInput(code), transit.EncodingUTF8, nil)
		if tree != nil {
			return errors.New("the parse gave a tree")
		}

		// drop the parser when it has an unfinished parse
		return nil
	})
}

// urOffsetInput gives a text in chunks of one byte, and records the offset
// of the last read. A test reads it in place of current_byte_offset of
// ParseState, which the Go API does not have.
type urOffsetInput struct {
	code []byte
	last atomic.Int64
}

// ReadAt returns one byte from the offset.
func (in *urOffsetInput) ReadAt(offset int, _ transit.Point) []byte {
	in.last.Store(int64(offset))
	if offset >= len(in.code) {
		return nil
	}
	return in.code[offset : offset+1]
}

func TestParsingWithTimeoutDuringBalancing(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	functionCount := 100

	code := &urOffsetInput{code: []byte(strings.Repeat("function() {}\n", functionCount))}
	currentByteOffset := int64(0)
	inBalancing := false
	tree, _ := parser.ParseInput(urProgressContext(func() bool {
		// The parser will call the progress_callback during parsing, and at the very end
		// during tree-balancing. For very large trees, this balancing act can take quite
		// some time, so we want to verify that timing out during this operation is
		// possible.
		//
		// We verify this by checking the current byte offset, as this number will *not* be
		// updated during tree balancing. If we see the same offset twice, we know that we
		// are in the balancing phase.
		if offset := code.last.Load(); offset != currentByteOffset {
			currentByteOffset = offset
			return false
		}
		inBalancing = true
		return true
	}), code, transit.EncodingUTF8, nil)

	if tree != nil {
		t.Error("the first parse gave a tree")
	}
	urEqual(t, "in_balancing", inBalancing, true)

	// This should not cause an assertion failure.
	parser.Reset()
	tree, _ = parser.ParseInput(urProgressContext(func() bool {
		if offset := code.last.Load(); offset != currentByteOffset {
			currentByteOffset = offset
			return false
		}
		inBalancing = true
		return true
	}), code, transit.EncodingUTF8, nil)

	if tree != nil {
		t.Error("the second parse gave a tree")
	}
	urEqual(t, "in_balancing", inBalancing, true)

	// If we resume parsing (implying we didn't call `parser.reset()`), we should be able to
	// finish parsing the tree, continuing from where we left off.
	tree, err := parser.ParseInput(urProgressContext(func() bool {
		// Because we've already finished parsing, we should only be resuming the
		// balancing phase.
		urEqual(t, "current_byte_offset", code.last.Load(), currentByteOffset)
		return false
	}), code, transit.EncodingUTF8, nil)
	if err != nil {
		t.Fatal(err)
	}
	urEqual(t, "has_error", tree.RootNode().HasError(), false)
	urEqual(t, "child_count", tree.RootNode().ChildCount(), functionCount)
}

// TestParsingWithTimeoutWhenErrorDetected is ported in part. The Go API has
// no ParseState, so the progress function cannot read has_error and stop
// the parse when the parser finds the error. The test stops the parse once
// the input gives the erroneous code, and checks that the parse gives no
// tree. It reads the offset of the last read in place of
// current_byte_offset.
func TestParsingWithTimeoutWhenErrorDetected(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "json").Language)

	// Parse an infinitely-long array, but insert an error after 1000 characters.
	offset := 0
	last := 0
	erroneousCode := "!,"
	tree, _ := parser.ParseInput(urProgressContext(func() bool {
		offset = last
		return offset > 1000
	}), urInputFunc(func(i int, _ transit.Point) []byte {
		last = i
		switch {
		case i == 0:
			return []byte("[")
		case i <= 1000:
			return []byte("0,")
		default:
			return []byte(erroneousCode)
		}
	}), transit.EncodingUTF8, nil)

	// The callback is called at the end of parsing, however, what we're asserting here is that
	// parsing ends immediately as the error is detected. This is verified by checking the offset
	// of the last byte processed is the length of the erroneous code we inserted, aka, 1002, or
	// 1000 + the length of the erroneous code.
	if offset <= 1000 {
		t.Errorf("offset = %d, want an offset past 1000", offset)
	}
	if tree != nil {
		t.Error("the parse gave a tree")
	}
}

// Included Ranges

func TestParsingWithOneIncludedRange(t *testing.T) {
	must := urMust(t)
	sourceCode := "<span>hi</span><script>console.log('sup');</script>"

	parser := urParser(t, fixtureGrammar(t, "html").Language)
	htmlTree := urParse(t, parser, sourceCode, nil)
	scriptContentNode := must(must(htmlTree.RootNode().Child(1)).Child(1))
	urEqual(t, "kind", scriptContentNode.Kind(), "raw_text")

	urSlice(t, "included_ranges", parser.IncludedRanges(), []transit.Range{{
		StartByte:  0,
		EndByte:    math.MaxUint32,
		StartPoint: urPoint(0, 0),
		EndPoint:   urPoint(math.MaxUint32, math.MaxUint32),
	}})
	if err := parser.SetIncludedRanges([]transit.Range{scriptContentNode.Range()}); err != nil {
		t.Fatal(err)
	}
	urSlice(t, "included_ranges", parser.IncludedRanges(), []transit.Range{scriptContentNode.Range()})
	if err := parser.SetLanguage(fixtureGrammar(t, "javascript").Language); err != nil {
		t.Fatal(err)
	}
	jsTree := urParse(t, parser, sourceCode, nil)

	urEqual(t, "to_sexp", jsTree.RootNode().String(),
		"(program (expression_statement (call_expression "+
			"function: (member_expression object: (identifier) property: (property_identifier)) "+
			"arguments: (arguments (string (string_fragment))))))")
	urEqual(t, "start_position", jsTree.RootNode().StartPoint(), urPoint(0, strings.Index(sourceCode, "console")))
	urSlice(t, "included_ranges", jsTree.IncludedRanges(), []transit.Range{scriptContentNode.Range()})
}

func TestParsingWithMultipleIncludedRanges(t *testing.T) {
	must := urMust(t)
	sourceCode := "html `<div>Hello, ${name.toUpperCase()}, it's <b>${now()}</b>.</div>`"
	find := func(s string) int { return strings.Index(sourceCode, s) }

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	jsTree := urParse(t, parser, sourceCode, nil)
	templateStringNode := must(jsTree.RootNode().DescendantForByteRange(find("`<"), find(">`")))
	urEqual(t, "kind", templateStringNode.Kind(), "template_string")

	openQuoteNode := must(templateStringNode.Child(0))
	interpolationNode1 := must(templateStringNode.Child(2))
	interpolationNode2 := must(templateStringNode.Child(4))
	closeQuoteNode := must(templateStringNode.Child(6))

	if err := parser.SetLanguage(fixtureGrammar(t, "html").Language); err != nil {
		t.Fatal(err)
	}
	htmlRanges := []transit.Range{
		{
			StartByte:  openQuoteNode.EndByte(),
			StartPoint: openQuoteNode.EndPoint(),
			EndByte:    interpolationNode1.StartByte(),
			EndPoint:   interpolationNode1.StartPoint(),
		},
		{
			StartByte:  interpolationNode1.EndByte(),
			StartPoint: interpolationNode1.EndPoint(),
			EndByte:    interpolationNode2.StartByte(),
			EndPoint:   interpolationNode2.StartPoint(),
		},
		{
			StartByte:  interpolationNode2.EndByte(),
			StartPoint: interpolationNode2.EndPoint(),
			EndByte:    closeQuoteNode.StartByte(),
			EndPoint:   closeQuoteNode.StartPoint(),
		},
	}
	if err := parser.SetIncludedRanges(htmlRanges); err != nil {
		t.Fatal(err)
	}
	htmlTree := urParse(t, parser, sourceCode, nil)

	urEqual(t, "to_sexp", htmlTree.RootNode().String(),
		"(document (element"+
			" (start_tag (tag_name))"+
			" (text)"+
			" (element (start_tag (tag_name)) (end_tag (tag_name)))"+
			" (text)"+
			" (end_tag (tag_name))))")
	urSlice(t, "included_ranges", htmlTree.IncludedRanges(), htmlRanges)

	divElementNode := must(htmlTree.RootNode().Child(0))
	helloTextNode := must(divElementNode.Child(1))
	bElementNode := must(divElementNode.Child(2))
	bStartTagNode := must(bElementNode.Child(0))
	bEndTagNode := must(bElementNode.Child(1))

	urEqual(t, "hello_text_node.kind", helloTextNode.Kind(), "text")
	urEqual(t, "hello_text_node.start_byte", helloTextNode.StartByte(), find("Hello"))
	urEqual(t, "hello_text_node.end_byte", helloTextNode.EndByte(), find(" <b>"))

	urEqual(t, "b_start_tag_node.kind", bStartTagNode.Kind(), "start_tag")
	urEqual(t, "b_start_tag_node.start_byte", bStartTagNode.StartByte(), find("<b>"))
	urEqual(t, "b_start_tag_node.end_byte", bStartTagNode.EndByte(), find("${now()}"))

	urEqual(t, "b_end_tag_node.kind", bEndTagNode.Kind(), "end_tag")
	urEqual(t, "b_end_tag_node.start_byte", bEndTagNode.StartByte(), find("</b>"))
	urEqual(t, "b_end_tag_node.end_byte", bEndTagNode.EndByte(), find(".</div>"))
}

func TestParsingWithIncludedRangeContainingMismatchedPositions(t *testing.T) {
	sourceCode := "<div>test</div>{_ignore_this_part_}"

	parser := urParser(t, fixtureGrammar(t, "html").Language)

	endByte := strings.Index(sourceCode, "{_ignore_this_part_")

	rangeToParse := transit.Range{
		StartByte: 0,
		StartPoint: transit.Point{
			Row:    10,
			Column: 12,
		},
		EndByte: endByte,
		EndPoint: transit.Point{
			Row:    10,
			Column: 12 + endByte,
		},
	}

	if err := parser.SetIncludedRanges([]transit.Range{rangeToParse}); err != nil {
		t.Fatal(err)
	}

	htmlTree := urParseInput(t, parser, urChunkedInput(sourceCode, 3), transit.EncodingUTF8, nil)

	urEqual(t, "range", htmlTree.RootNode().Range(), rangeToParse)

	urEqual(t, "to_sexp", htmlTree.RootNode().String(),
		"(document (element (start_tag (tag_name)) (text) (end_tag (tag_name))))")
}

// TestParsingErrorInInvalidIncludedRanges checks ErrInvalidRanges. The
// IncludedRangesError of the Rust binding holds the index of the first
// invalid range, and the Go error does not.
func TestParsingErrorInInvalidIncludedRanges(t *testing.T) {
	parser := transit.NewParser()

	// Ranges are not ordered
	err := parser.SetIncludedRanges([]transit.Range{
		{
			StartByte:  23,
			EndByte:    29,
			StartPoint: urPoint(0, 23),
			EndPoint:   urPoint(0, 29),
		},
		{
			StartByte:  0,
			EndByte:    5,
			StartPoint: urPoint(0, 0),
			EndPoint:   urPoint(0, 5),
		},
		{
			StartByte:  50,
			EndByte:    60,
			StartPoint: urPoint(0, 50),
			EndPoint:   urPoint(0, 60),
		},
	})
	if !errors.Is(err, transit.ErrInvalidRanges) {
		t.Errorf("the error of ranges that are not ordered is %v, want ErrInvalidRanges", err)
	}

	// Range ends before it starts
	err = parser.SetIncludedRanges([]transit.Range{{
		StartByte:  10,
		EndByte:    5,
		StartPoint: urPoint(0, 10),
		EndPoint:   urPoint(0, 5),
	}})
	if !errors.Is(err, transit.ErrInvalidRanges) {
		t.Errorf("the error of a range that ends before it starts is %v, want ErrInvalidRanges", err)
	}
}

func TestParsingUTF16CodeWithErrorsAtTheEndOfAnIncludedRange(t *testing.T) {
	sourceCode := "<script>a.</script>"
	utf16SourceCode := urUTF16(sourceCode, binary.LittleEndian)

	startByte := 2 * strings.Index(sourceCode, "a.")
	endByte := 2 * strings.Index(sourceCode, "</script>")

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	if err := parser.SetIncludedRanges([]transit.Range{{
		StartByte:  startByte,
		EndByte:    endByte,
		StartPoint: urPoint(0, startByte),
		EndPoint:   urPoint(0, endByte),
	}}); err != nil {
		t.Fatal(err)
	}
	tree := urParseInput(t, parser, urBytesInput(utf16SourceCode), transit.EncodingUTF16LE, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(), "(program (ERROR (identifier)))")
}

func TestParsingWithExternalScannerThatUsesIncludedRangeBoundaries(t *testing.T) {
	must := urMust(t)
	sourceCode := "a <%= b() %> c <% d() %>"
	range1StartByte := strings.Index(sourceCode, " b() ")
	range1EndByte := range1StartByte + len(" b() ")
	range2StartByte := strings.Index(sourceCode, " d() ")
	range2EndByte := range2StartByte + len(" d() ")

	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	if err := parser.SetIncludedRanges([]transit.Range{
		{
			StartByte:  range1StartByte,
			EndByte:    range1EndByte,
			StartPoint: urPoint(0, range1StartByte),
			EndPoint:   urPoint(0, range1EndByte),
		},
		{
			StartByte:  range2StartByte,
			EndByte:    range2EndByte,
			StartPoint: urPoint(0, range2StartByte),
			EndPoint:   urPoint(0, range2EndByte),
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := urParse(t, parser, sourceCode, nil)
	root := tree.RootNode()
	statement1 := must(root.Child(0))
	statement2 := must(root.Child(1))

	urEqual(t, "to_sexp", root.String(),
		"(program"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments)))"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments))))")

	urEqual(t, "statement1.start_byte", statement1.StartByte(), strings.Index(sourceCode, "b()"))
	urEqual(t, "statement1.end_byte", statement1.EndByte(), strings.Index(sourceCode, " %> c"))
	urEqual(t, "statement2.start_byte", statement2.StartByte(), strings.Index(sourceCode, "d()"))
	urEqual(t, "statement2.end_byte", statement2.EndByte(), len(sourceCode)-len(" %>"))
}

func TestParsingWithANewlyExcludedRange(t *testing.T) {
	sourceCode := "<div><span><%= something %></span></div>"

	// Parse HTML including the template directive, which will cause an error
	parser := urParser(t, fixtureGrammar(t, "html").Language)
	firstTree := urParseInput(t, parser, urChunkedInput(sourceCode, 3), transit.EncodingUTF8, nil)

	// Insert code at the beginning of the document.
	prefix := "a very very long line of plain text. "
	firstTree.Edit(transit.InputEdit{
		StartByte:   0,
		OldEndByte:  0,
		NewEndByte:  len(prefix),
		StartPoint:  urPoint(0, 0),
		OldEndPoint: urPoint(0, 0),
		NewEndPoint: urPoint(0, len(prefix)),
	})
	sourceCode = prefix + sourceCode

	// Parse the HTML again, this time *excluding* the template directive
	// (which has moved since the previous parse).
	directiveStart := strings.Index(sourceCode, "<%=")
	directiveEnd := strings.Index(sourceCode, "</span>")
	sourceCodeEnd := len(sourceCode)
	if err := parser.SetIncludedRanges([]transit.Range{
		{
			StartByte:  0,
			EndByte:    directiveStart,
			StartPoint: urPoint(0, 0),
			EndPoint:   urPoint(0, directiveStart),
		},
		{
			StartByte:  directiveEnd,
			EndByte:    sourceCodeEnd,
			StartPoint: urPoint(0, directiveEnd),
			EndPoint:   urPoint(0, sourceCodeEnd),
		},
	}); err != nil {
		t.Fatal(err)
	}
	tree := urParseInput(t, parser, urChunkedInput(sourceCode, 3), transit.EncodingUTF8, firstTree)

	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(document (text) (element"+
			" (start_tag (tag_name))"+
			" (element (start_tag (tag_name)) (end_tag (tag_name)))"+
			" (end_tag (tag_name))))")

	urSlice(t, "changed_ranges", tree.ChangedRanges(firstTree), []transit.Range{
		// The first range that has changed syntax is the range of the newly-inserted text.
		{
			StartByte:  0,
			EndByte:    len(prefix),
			StartPoint: urPoint(0, 0),
			EndPoint:   urPoint(0, len(prefix)),
		},
		// Even though no edits were applied to the outer `div` element,
		// its contents have changed syntax because a range of text that
		// was previously included is now excluded.
		{
			StartByte:  directiveStart,
			EndByte:    directiveEnd,
			StartPoint: urPoint(0, directiveStart),
			EndPoint:   urPoint(0, directiveEnd),
		},
	})
}

func TestParsingWithANewlyIncludedRange(t *testing.T) {
	sourceCode := "<div><%= foo() %></div><span><%= bar() %></span><%= baz() %>"
	range1Start := strings.Index(sourceCode, " foo")
	range2Start := strings.Index(sourceCode, " bar")
	range3Start := strings.Index(sourceCode, " baz")
	range1End := range1Start + 7
	range2End := range2Start + 7
	range3End := range3Start + 7

	// Parse only the first code directive as JavaScript
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)
	if err := parser.SetIncludedRanges([]transit.Range{urSimpleRange(range1Start, range1End)}); err != nil {
		t.Fatal(err)
	}
	tree := urParseInput(t, parser, urChunkedInput(sourceCode, 3), transit.EncodingUTF8, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(program"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments))))")

	// Parse both the first and third code directives as JavaScript, using the old tree as a
	// reference.
	if err := parser.SetIncludedRanges([]transit.Range{
		urSimpleRange(range1Start, range1End),
		urSimpleRange(range3Start, range3End),
	}); err != nil {
		t.Fatal(err)
	}
	tree2 := urParseInput(t, parser, urChunkedInput(sourceCode, 3), transit.EncodingUTF8, tree)
	urEqual(t, "to_sexp", tree2.RootNode().String(),
		"(program"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments)))"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments))))")
	urSlice(t, "changed_ranges", tree2.ChangedRanges(tree), []transit.Range{urSimpleRange(range1End, range3End)})

	// Parse all three code directives as JavaScript, using the old tree as a
	// reference.
	if err := parser.SetIncludedRanges([]transit.Range{
		urSimpleRange(range1Start, range1End),
		urSimpleRange(range2Start, range2End),
		urSimpleRange(range3Start, range3End),
	}); err != nil {
		t.Fatal(err)
	}
	tree3 := urParse(t, parser, sourceCode, tree)
	urEqual(t, "to_sexp", tree3.RootNode().String(),
		"(program"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments)))"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments)))"+
			" (expression_statement (call_expression function: (identifier) arguments: (arguments))))")
	urSlice(t, "changed_ranges", tree3.ChangedRanges(tree2), []transit.Range{urSimpleRange(range2Start+1, range2End-1)})
}

func TestParsingWithIncludedRangesAndMissingTokens(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, urTestLanguage(t, `{
            "name": "test_leading_missing_token",
            "rules": {
                "program": {
                    "type": "SEQ",
                    "members": [
                        {"type": "SYMBOL", "name": "A"},
                        {"type": "SYMBOL", "name": "b"},
                        {"type": "SYMBOL", "name": "c"},
                        {"type": "SYMBOL", "name": "A"},
                        {"type": "SYMBOL", "name": "b"},
                        {"type": "SYMBOL", "name": "c"}
                    ]
                },
                "A": {"type": "SYMBOL", "name": "a"},
                "a": {"type": "STRING", "value": "a"},
                "b": {"type": "STRING", "value": "b"},
                "c": {"type": "STRING", "value": "c"}
            }
        }`, ""))

	// There's a missing `a` token at the beginning of the code. It must be inserted
	// at the beginning of the first included range, not at {0, 0}.
	sourceCode := "__bc__bc__"
	if err := parser.SetIncludedRanges([]transit.Range{
		{
			StartByte:  2,
			EndByte:    4,
			StartPoint: urPoint(0, 2),
			EndPoint:   urPoint(0, 4),
		},
		{
			StartByte:  6,
			EndByte:    8,
			StartPoint: urPoint(0, 6),
			EndPoint:   urPoint(0, 8),
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := urParse(t, parser, sourceCode, nil)
	root := tree.RootNode()
	urEqual(t, "to_sexp", root.String(), "(program (A (MISSING a)) (b) (c) (A (MISSING a)) (b) (c))")
	urEqual(t, "start_byte", root.StartByte(), 2)
	urEqual(t, "child(3).start_byte", must(root.Child(3)).StartByte(), 4)
}

func TestGrammarsThatCanHangOnEOF(t *testing.T) {
	parser := urParser(t, urTestLanguage(t, `
        {
            "name": "test_single_null_char_regex",
            "rules": {
                "source_file": {
                    "type": "SEQ",
                    "members": [
                        { "type": "STRING", "value": "\"" },
                        { "type": "PATTERN", "value": "[\\x00]*" },
                        { "type": "STRING", "value": "\"" }
                    ]
                }
            },
            "extras": [ { "type": "PATTERN", "value": "\\s" } ]
        }
        `, ""))
	urParse(t, parser, `"`, nil)

	if err := parser.SetLanguage(urTestLanguage(t, `
        {
            "name": "test_null_char_with_next_char_regex",
            "rules": {
                "source_file": {
                    "type": "SEQ",
                    "members": [
                        { "type": "STRING", "value": "\"" },
                        { "type": "PATTERN", "value": "[\\x00-\\x01]*" },
                        { "type": "STRING", "value": "\"" }
                    ]
                }
            },
            "extras": [ { "type": "PATTERN", "value": "\\s" } ]
        }
        `, "")); err != nil {
		t.Fatal(err)
	}
	urParse(t, parser, `"`, nil)

	if err := parser.SetLanguage(urTestLanguage(t, `
        {
            "name": "test_null_char_with_range_regex",
            "rules": {
                "source_file": {
                    "type": "SEQ",
                    "members": [
                        { "type": "STRING", "value": "\"" },
                        { "type": "PATTERN", "value": "[\\x00-\\x7F]*" },
                        { "type": "STRING", "value": "\"" }
                    ]
                }
            },
            "extras": [ { "type": "PATTERN", "value": "\\s" } ]
        }
        `, "")); err != nil {
		t.Fatal(err)
	}
	urParse(t, parser, `"`, nil)
}

func TestParseStackRecursiveMergeErrorCostCalculationBug(t *testing.T) {
	sourceCode := `
fn main() {
  if n == 1 {
  } else if n == 2 {
  } else {
  }
}

let y = if x == 5 { 10 } else { 15 };

if foo && bar {}

if foo && bar || baz {}
`

	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	tree := urParse(t, parser, sourceCode, nil)

	edit := urEdit{
		position:      60,
		deletedLength: 63,
	}
	input := []byte(sourceCode)
	urPerformEdit(t, tree, &input, edit)

	urParse(t, parser, input, tree)
}

func TestParsingWithScannerLogging(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "external_tokens"))

	found := false
	parser.SetLogger(func(logType transit.LogType, message string) {
		if logType == transit.LogLex && message == "Found a percent string" {
			found = true
		}
	})

	sourceCode := "x + %(sup (external) scanner?)"

	urParse(t, parser, sourceCode, nil)
	urEqual(t, "found", found, true)
}

func TestParsingGetColumnAtEOF(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "get_col_eof"))

	urParse(t, parser, "a", nil)
}

func TestParsingByHaltingAtOffset(t *testing.T) {
	parser := urParser(t, fixtureGrammar(t, "javascript").Language)

	sourceCode := strings.Repeat("function foo() { return 1; }", 1000)

	seenByteOffsets := 0

	urParseInputContext(urProgressContext(func() bool {
		seenByteOffsets++
		return false
	}), t, parser, urBytesInput(sourceCode))

	if seenByteOffsets <= 100 {
		t.Errorf("the progress function ran %d times, want more than 100", seenByteOffsets)
	}
}

// urParseInputContext parses an input of UTF-8 with a context, and fails
// the test when the parse fails.
func urParseInputContext(ctx context.Context, t *testing.T, parser *transit.Parser, in transit.Input) *transit.Tree {
	t.Helper()
	tree, err := parser.ParseInput(ctx, in, transit.EncodingUTF8, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// urDecodeText is the text of the four tests of a custom decode function.
const urDecodeText = "pub fn foo() { println!(\"€50\"); }"

// urDecodeSexp is the tree that the four tests of a custom decode function
// expect.
const urDecodeSexp = "(source_file (function_item (visibility_modifier) name: (identifier) parameters: (parameters) body: (block (expression_statement (macro_invocation macro: (identifier) (token_tree (string_literal (string_content))))))))"

// urParseCustomEncoding parses the bytes text of urDecodeText with the
// rust grammar and the decode function decode, and compares the tree with
// urDecodeSexp.
func urParseCustomEncoding(t *testing.T, text []byte, decode func([]byte) (rune, int)) {
	t.Helper()
	parser := urParser(t, fixtureGrammar(t, "rust").Language)
	tree, err := parser.ParseCustomEncoding(context.Background(), urBytesInput(text), decode, nil)
	if err != nil {
		t.Fatal(err)
	}
	urEqual(t, "to_sexp", tree.RootNode().String(), urDecodeSexp)
}

// urSingleByteDecode is the decoder of test_decode_cp1252 and
// test_decode_macintosh. It returns each byte as its code point.
func urSingleByteDecode(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}
	return rune(b[0]), 1
}

func TestDecodeUTF32(t *testing.T) {
	// The Rust test makes the text with u32cstr!, in the byte order of the
	// machine.
	var text []byte
	for _, c := range urDecodeText {
		text = binary.NativeEndian.AppendUint32(text, uint32(c))
	}

	urParseCustomEncoding(t, text, func(b []byte) (rune, int) {
		if len(b) >= 4 {
			return rune(binary.NativeEndian.Uint32(b)), 4
		}
		return 0, 0
	})
}

func TestDecodeCP1252(t *testing.T) {
	// The Rust test encodes the text with WINDOWS_1252 of the crate
	// encoding_rs. The euro sign is the only character that is not ASCII,
	// and Windows-1252 encodes it as 0x80.
	text := bytes.ReplaceAll([]byte(urDecodeText), []byte("€"), []byte{0x80})

	urParseCustomEncoding(t, text, urSingleByteDecode)
}

func TestDecodeMacintosh(t *testing.T) {
	// The Rust test encodes the text with MACINTOSH of the crate
	// encoding_rs. The euro sign is the only character that is not ASCII,
	// and Mac OS Roman encodes it as 0xDB.
	text := bytes.ReplaceAll([]byte(urDecodeText), []byte("€"), []byte{0xDB})

	urParseCustomEncoding(t, text, urSingleByteDecode)
}

func TestDecodeUTF24LE(t *testing.T) {
	var text []byte
	for _, c := range urDecodeText {
		text = append(text, byte(c&0xFF), byte((c>>8)&0xFF), byte((c>>16)&0xFF))
	}

	urParseCustomEncoding(t, text, func(b []byte) (rune, int) {
		if len(b) >= 3 {
			return rune(binary.LittleEndian.Uint32([]byte{b[0], b[1], b[2], 0})), 3
		}
		return 0, 0
	})
}

// urGenerate is generate_parser with the generator of transit.
func urGenerate(grammarJSON string) error {
	var diagnostics []generate.Diagnostic
	_, _, err := generate.ParserForGrammar([]byte(grammarJSON), &generate.SemanticVersion{}, generate.OptLevelMergeStates, c.Backend{}, &diagnostics)
	return err
}

func TestGrammarsThatShouldNotCompile(t *testing.T) {
	for _, grammar := range []string{
		`
        {
            "name": "issue_1111",
            "rules": {
                "source_file": { "type": "STRING", "value": "" }
            },
        }
        `,
		`
        {
            "name": "issue_1271",
            "rules": {
                "source_file": { "type": "SYMBOL", "name": "identifier" },
                "identifier": {
                    "type": "TOKEN",
                    "content": {
                        "type": "REPEAT",
                        "content": { "type": "PATTERN", "value": "a" }
                    }
                }
            },
        }
        `,
		`
        {
            "name": "issue_1156_expl_1",
            "rules": {
                "source_file": {
                    "type": "TOKEN",
                    "content": {
                        "type": "REPEAT",
                        "content": { "type": "STRING", "value": "c" }
                    }
                }
            },
        }
        `,
		`
        {
            "name": "issue_1156_expl_2",
            "rules": {
                "source_file": {
                    "type": "TOKEN",
                    "content": {
                        "type": "CHOICE",
                        "members": [
                            { "type": "STRING", "value": "e" },
                            { "type": "BLANK" }
                        ]
                    }
                }
            },
        }
        `,
		`
        {
            "name": "issue_1156_expl_3",
            "rules": {
                "source_file": {
                    "type": "IMMEDIATE_TOKEN",
                    "content": {
                        "type": "REPEAT",
                        "content": { "type": "STRING", "value": "p" }
                    }
                }
            },
        }
        `,
		`
        {
            "name": "issue_1156_expl_4",
            "rules": {
                "source_file": {
                    "type": "IMMEDIATE_TOKEN",
                    "content": {
                        "type": "CHOICE",
                        "members": [
                            { "type": "STRING", "value": "r" },
                            { "type": "BLANK" }
                        ]
                    }
                }
            },
        }
        `,
	} {
		if urGenerate(grammar) == nil {
			t.Errorf("the grammar compiled: %s", grammar)
		}
	}
}

// urChunkedInput is chunked_input of parser_test.rs.
func urChunkedInput(text string, size int) transit.Input {
	return urInputFunc(func(offset int, _ transit.Point) []byte {
		return []byte(text[offset:min(len(text), offset+size)])
	})
}

// TestParseOptionsReborrow uses one progress context for two parses, as
// upstream uses one ParseOptions.
func TestParseOptionsReborrow(t *testing.T) {
	must := urMust(t)
	parser := urParser(t, fixtureGrammar(t, "rust").Language)

	var parseCount atomic.Int64

	options := urProgressContext(func() bool {
		parseCount.Add(1)
		return false
	})

	text1 := strings.Repeat("fn first() {}", 20)
	text2 := strings.Repeat("fn second() {}", 20)

	tree1 := urParseInputContext(options, t, parser, urBytesInput(text1))

	urEqual(t, "child(0).kind", must(tree1.RootNode().Child(0)).Kind(), "function_item")

	tree2 := urParseInputContext(options, t, parser, urBytesInput(text2))

	urEqual(t, "child(0).kind", must(tree2.RootNode().Child(0)).Kind(), "function_item")

	if parseCount.Load() <= 0 {
		t.Error("the progress function did not run")
	}
}

// TestErrorRecoveryChecksVersionsAfterFailedMissingTokenReduction is
// test_error_recovery_checks_versions_after_failed_missing_token_reduction
// of parser_test.rs. The progress function stops the parse after 100 calls,
// so a parser that loops in error recovery fails the test and does not hang.
func TestErrorRecoveryChecksVersionsAfterFailedMissingTokenReduction(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "error_recovery_loop"))

	input := "<<a/a>{[:aaaaa"
	progressChecks := 0
	tree := urParseInputContext(urProgressContext(func() bool {
		progressChecks++
		// error recovery should finish without repeating the same stack versions
		return progressChecks > 100
	}), t, parser, urBytesInput(input))

	urEqual(t, "root_node().end_byte()", tree.RootNode().EndByte(), len(input))
}

// urHangEnv is the variable of the environment that makes the test
// binary run the body of TestGrammarThatShouldHangAndNotSegfault.
const urHangEnv = "TRANSIT_UR_HANG_TEST"

// TestGrammarThatShouldHangAndNotSegfault runs hang_test in a child
// process of the test binary, in place of a thread. A goroutine that runs
// the scanner, which loops forever in C, cannot be stopped, and the child
// process can be.
func TestGrammarThatShouldHangAndNotSegfault(t *testing.T) {
	hangTest := func() {
		language := urTestFixtureLanguage(t, "get_col_should_hang_not_crash")

		parser := urParser(t, language)

		codeThatShouldHang := "\nHello"

		if _, err := parser.Parse(context.Background(), []byte(codeThatShouldHang), nil); err != nil {
			t.Fatalf("Parse operation completed unexpectedly: %v", err)
		}
	}

	if os.Getenv(urHangEnv) != "" {
		hangTest()
		return
	}

	// Build the grammar before the timer starts, as the Rust test builds it
	// in its thread.
	urTestFixtureLanguage(t, "get_col_should_hang_not_crash")

	timeout := 500 * time.Millisecond
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestGrammarThatShouldHangAndNotSegfault$", "-test.count=1")
	cmd.Env = append(os.Environ(), urHangEnv+"=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("The test completed rather than hanging:\n%s", out.String())
		}
		t.Fatalf("The test panicked unexpectedly: %v\n%s", err, out.String())
	case <-time.After(timeout):
		// Expected
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		<-done
	}
}

// TestParsingAfterBalancingTreeThatDependsOnColumn is
// test_parsing_after_balancing_tree_that_depends_on_column of
// parser_test.rs.
func TestParsingAfterBalancingTreeThatDependsOnColumn(t *testing.T) {
	parser := urParser(t, urTestFixtureLanguage(t, "depends_on_column_repetition"))

	code := []byte("\nax\na")
	tree := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", tree.RootNode().String(),
		"(document (newline) (word) (tail) (newline) (word))")

	urPerformEdit(t, tree, &code, urEdit{
		position:      2,
		deletedLength: 0,
		insertedText:  []byte("\n"),
	})

	incremental := urParse(t, parser, code, tree)
	fresh := urParse(t, parser, code, nil)
	urEqual(t, "to_sexp", fresh.RootNode().String(),
		"(document (newline) (word) (newline) (head) (newline) (word))")
	urEqual(t, "to_sexp", incremental.RootNode().String(), fresh.RootNode().String())
}
