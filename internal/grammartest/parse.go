package grammartest

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/transit"
)

// This file ports the output of a concrete syntax tree of
// crates/cli/src/parse.rs, which a corpus test with the attribute :cst
// compares (D79): render_cst and the functions that it calls, and
// render_test_cst of crates/cli/src/test.rs. The output has no colors, and
// it has the ranges of the nodes, as the options of render_test_cst give.
// The line feed is "\n", as on each platform but Windows.

// RenderCST returns the concrete syntax tree of tree, whose text is
// sourceCode, as a corpus test with the attribute :cst expects it. Each
// node is one line with its range, its field name, its kind and, for a
// node with no children, its text.
//
// RenderCST is render_test_cst.
func RenderCST(sourceCode []byte, tree *transit.Tree) string {
	var out strings.Builder
	renderCST(&out, sourceCode, tree.Walk())
	return strings.TrimSpace(out.String())
}

// ilog10 is checked_ilog10 with unwrap_or(0): the number of decimal digits
// of n less one, and 0 for 0.
func ilog10(n int) int {
	if n <= 0 {
		return 0
	}
	return len(strconv.Itoa(n)) - 1
}

// fromUTF8Lossy returns b as a text, with U+FFFD in place of each maximal
// part of an invalid UTF-8 sequence, as String::from_utf8_lossy does. It
// differs from strings.ToValidUTF8, which puts one U+FFFD in place of a run
// of invalid bytes.
func fromUTF8Lossy(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var out strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r != utf8.RuneError || size > 1 {
			out.Write(b[:size])
			b = b[size:]
			continue
		}
		out.WriteRune(utf8.RuneError)
		b = b[invalidLen(b):]
	}
	return out.String()
}

// invalidLen returns the length of the maximal part at the start of b that
// does not begin a valid character: a lead byte and the continuation bytes
// that can follow it, or 1 for a byte that cannot lead.
func invalidLen(b []byte) int {
	var n int
	lo, hi := byte(0x80), byte(0xBF)
	switch c := b[0]; {
	case c >= 0xC2 && c <= 0xDF:
		n = 2
	case c == 0xE0:
		n, lo = 3, 0xA0
	case c == 0xED:
		n, hi = 3, 0x9F
	case c >= 0xE1 && c <= 0xEF:
		n = 3
	case c == 0xF0:
		n, lo = 4, 0x90
	case c >= 0xF1 && c <= 0xF3:
		n = 4
	case c == 0xF4:
		n, hi = 4, 0x8F
	default:
		return 1
	}
	i := 1
	for ; i < n && i < len(b); i++ {
		if b[i] < lo || b[i] > hi {
			break
		}
		// only the second byte has a range of its own
		lo, hi = 0x80, 0xBF
	}
	return i
}

// rustLines yields the lines of a text as str::lines does: each line without
// its "\n", or without its "\r\n", and no empty line after a last "\n".
func rustLines(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for line := range strings.Lines(s) {
			if l, ok := strings.CutSuffix(line, "\n"); ok {
				line = strings.TrimSuffix(l, "\r")
			}
			if !yield(line) {
				return
			}
		}
	}
}

// renderCST writes the concrete syntax tree of the node of cursor and of
// each node under it.
//
// renderCST is render_cst, with no_ranges false.
func renderCST(out *strings.Builder, sourceCode []byte, cursor *transit.TreeCursor) {
	lossySourceCode := fromUTF8Lossy(sourceCode)
	totalWidth, row := 0, 0
	for line := range rustLines(lossySourceCode) {
		totalWidth = max(totalWidth, ilog10(row)+ilog10(len(line))+1)
		row++
	}
	if row == 0 {
		totalWidth = 1
	}
	indentLevel := 1
	didVisitChildren := false
	inError := false
	for {
		if didVisitChildren {
			switch {
			case cursor.GotoNextSibling():
				didVisitChildren = false
			case cursor.GotoParent():
				didVisitChildren = true
				indentLevel--
				if !cursor.Node().HasError() {
					inError = false
				}
			default:
				return
			}
		} else {
			cstRenderNode(out, cursor, sourceCode, totalWidth, indentLevel, inError)
			if cursor.GotoFirstChild() {
				didVisitChildren = false
				indentLevel++
				if cursor.Node().HasError() {
					inError = true
				}
			} else {
				didVisitChildren = true
			}
		}
	}
}

// cstNodeText escapes the invisible characters of a text and the quotes that
// delimit it.
//
// cstNodeText is the Display of CstNodeText, with escape_invisible and
// escape_delimiter.
func cstNodeText(s string) string {
	var out strings.Builder
	for _, c := range s {
		switch c {
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case 0:
			out.WriteString(`\0`)
		case '\\':
			out.WriteString(`\\`)
		case '\x0b':
			out.WriteString(`\v`)
		case '\x0c':
			out.WriteString(`\f`)
		case '`':
			out.WriteString("\\`")
		case '"':
			out.WriteString(`\"`)
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}

// writeNodeText writes the text of a node: a literal in double quotes, or
// the text of a named node in backquotes, one line of the output for each
// line of a text of more than one line.
//
// writeNodeText is write_node_text.
func writeNodeText(out *strings.Builder, cursor *transit.TreeCursor, isNamed bool, source string, totalWidth, indentLevel int) {
	if !isNamed {
		out.WriteString(`"` + cstNodeText(source) + `"`)
		return
	}
	multiline := strings.Contains(source, "\n")
	i := 0
	for line := range strings.Lines(source) {
		nodeRange := cursor.Node().Range()
		// For each line of text, adjust the row by shifting it down `i` rows,
		// and adjust the column by setting it to the length of *this* line.
		nodeRange.StartPoint.Row += i
		nodeRange.EndPoint.Row = nodeRange.StartPoint.Row
		nodeRange.EndPoint.Column = len(line)
		if i == 0 {
			nodeRange.EndPoint.Column += nodeRange.StartPoint.Column
		}
		if multiline {
			out.WriteString("\n")
			out.WriteString(cstNodeRange(totalWidth, nodeRange))
			for range indentLevel + 1 {
				out.WriteString("  ")
			}
		} else {
			out.WriteString(" ")
		}
		out.WriteString("`" + cstLineFeed(line) + "`")
		i++
	}
}

// cstLineFeed escapes a line of text, with its line feed.
//
// cstLineFeed is the Display of CstLineFeed.
func cstLineFeed(source string) string {
	parts := strings.Split(source, "\n")
	var out strings.Builder
	out.WriteString(cstNodeText(parts[0]))
	for _, part := range parts[1:] {
		out.WriteString(cstNodeText("\n") + cstNodeText(part))
	}
	return out.String()
}

// cstNodeRange returns the range of a node, with the spaces that align the
// columns of the output.
//
// cstNodeRange is the Display of CstNodeRange.
func cstNodeRange(totalWidth int, r transit.Range) string {
	remainingWidth := func(row, col int) int {
		return max(totalWidth-ilog10(row)-ilog10(col), 1)
	}
	start, end := r.StartPoint, r.EndPoint
	return fmt.Sprintf("%d:%d%s- %d:%d%s",
		start.Row, start.Column, strings.Repeat(" ", remainingWidth(start.Row, start.Column)),
		end.Row, end.Column, strings.Repeat(" ", remainingWidth(end.Row, end.Column)))
}

// cstRenderNode writes the line of the node of cursor.
//
// cstRenderNode is cst_render_node.
func cstRenderNode(out *strings.Builder, cursor *transit.TreeCursor, sourceCode []byte, totalWidth, indentLevel int, inError bool) {
	node := cursor.Node()
	isNamed := node.IsNamed()
	out.WriteString(cstNodeRange(totalWidth, node.Range()))
	out.WriteString(strings.Repeat("  ", indentLevel))
	if inError && !node.HasError() {
		out.WriteString(" ")
	}
	switch {
	case isNamed:
		if fieldName := cursor.FieldName(); fieldName != "" {
			out.WriteString(fieldName + ": ")
		}

		if node.HasError() || node.IsError() {
			out.WriteString("•")
		}

		out.WriteString(node.Kind())

		if node.ChildCount() == 0 {
			// Node text from a pattern or external scanner
			text := fromUTF8Lossy(sourceCode[node.StartByte():node.EndByte()])
			writeNodeText(out, cursor, isNamed, text, totalWidth, indentLevel)
		}
	case node.IsMissing():
		out.WriteString(`MISSING: "` + node.Kind() + `"`)
	default:
		// Terminal literals, like "fn"
		writeNodeText(out, cursor, isNamed, node.Kind(), totalWidth, indentLevel)
	}
	out.WriteString("\n")
}
