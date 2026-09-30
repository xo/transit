package grammartest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/transit"
)

// This file ports crates/cli/src/query_testing.rs: the assertions in the
// comments of a file of test/highlight. It leaves out
// assert_expected_captures, because the highlight test of a grammar checks
// an assertion against the innermost capture, as the highlighter of upstream
// does, and not against the first one (grammartest.go).
//
// to_utf8_point counts grapheme clusters upstream, and the standard library
// of Go has no grapheme clusters, so the port counts code points. The two
// counts differ only on a line with a character of more than one code point,
// such as an emoji with a modifier, before the column.

// captureNameRegex matches a capture name in an assertion.
//
// captureNameRegex is CAPTURE_NAME_REGEX.
var captureNameRegex = regexp.MustCompile(`[\w_\-.]+`)

// utf8Point is a position as a row and a column that counts characters.
//
// utf8Point is Utf8Point.
type utf8Point struct {
	row    int
	column int
}

// String returns the point as upstream writes it.
//
// String is the Display of Utf8Point.
func (p utf8Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.row, p.column)
}

// compare orders two points by the row, and then by the column.
func (p utf8Point) compare(other utf8Point) int {
	if p.row != other.row {
		return p.row - other.row
	}
	return p.column - other.column
}

// toUTF8Point returns a point of the tree, whose column counts bytes, as a
// point whose column counts characters.
//
// toUTF8Point is to_utf8_point.
func toUTF8Point(point transit.Point, source []byte) utf8Point {
	if point.Column == 0 {
		return utf8Point{row: point.Row, column: 0}
	}

	line := source
	for range point.Row {
		i := bytes.IndexByte(line, '\n')
		if i < 0 {
			line = nil
			break
		}
		line = line[i+1:]
	}

	utf8Column := 0
	for i := 0; i < len(line); {
		_, size := utf8.DecodeRune(line[i:])
		i += size
		utf8Column++
		if i >= point.Column {
			break
		}
	}

	return utf8Point{row: point.Row, column: utf8Column}
}

// captureInfo is a capture of a query: its name and its range.
//
// captureInfo is CaptureInfo.
type captureInfo struct {
	name  string
	start utf8Point
	end   utf8Point
}

// assertion is an assertion in a comment: the position that it points at,
// the number of its arrows, whether it is negative, and the capture name
// that it expects.
//
// assertion is Assertion.
type assertion struct {
	position            utf8Point
	length              int
	negative            bool
	expectedCaptureName string
}

// parsePositionComments parses the given source code, finding all of the
// comments that contain highlighting assertions. It returns the assertions,
// each with its position and the expected highlight name.
//
// parsePositionComments is parse_position_comments.
func parsePositionComments(parser *transit.Parser, language *transit.Language, source []byte) ([]assertion, error) {
	var result []assertion
	type pointRange struct{ start, end transit.Point }
	var assertionRanges []pointRange

	// Parse the code.
	if err := parser.SetIncludedRanges(nil); err != nil {
		return nil, fmt.Errorf("resetting the included ranges: %w", err)
	}
	if err := parser.SetLanguage(language); err != nil {
		return nil, fmt.Errorf("setting the language: %w", err)
	}
	tree, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing the source: %w", err)
	}

	// Walk the tree, finding comment nodes that contain assertions.
	ascending := false
	cursor := tree.RootNode().Walk()
	for {
		if ascending {
			node := cursor.Node()

			// Find every comment node.
			if strings.Contains(strings.ToLower(node.Kind()), "comment") && utf8.Valid(source[node.StartByte():node.EndByte()]) {
				text := node.Text(source)
				position := node.StartPoint()
				if position.Row > 0 {
					// Find the arrow character ("^" or "<-") in the comment. A left arrow
					// refers to the column where the comment node starts. An up arrow refers
					// to its own column.
					hasLeftCaret := false
					hasArrow := false
					negative := false
					arrowEnd := 0
					arrowCount := 1
					for i, c := range text {
						arrowEnd = i + 1
						if c == '-' && hasLeftCaret {
							hasArrow = true
							break
						}
						if c == '^' {
							hasArrow = true
							position.Column += i
							// Continue counting remaining arrows and update their end column
							for _, c := range text[arrowEnd:] {
								if c != '^' {
									arrowEnd += arrowCount - 1
									break
								}
								arrowCount++
							}
							break
						}
						hasLeftCaret = c == '<'
					}

					// find any ! after arrows but before capture name
					if hasArrow {
						for i, c := range text[arrowEnd:] {
							if c == '!' {
								negative = true
								arrowEnd += i + 1
								break
							} else if !unicode.IsSpace(c) {
								break
							}
						}
					}

					// If the comment node contains an arrow and a highlight name, record the
					// highlight name and the position.
					if hasArrow {
						if mat := captureNameRegex.FindString(text[arrowEnd:]); mat != "" {
							assertionRanges = append(assertionRanges, pointRange{node.StartPoint(), node.EndPoint()})
							result = append(result, assertion{
								position:            toUTF8Point(position, source),
								length:              arrowCount,
								negative:            negative,
								expectedCaptureName: mat,
							})
						}
					}
				}
			}

			// Continue walking the tree.
			if cursor.GotoNextSibling() {
				ascending = false
			} else if !cursor.GotoParent() {
				break
			}
		} else if !cursor.GotoFirstChild() {
			ascending = true
		}
	}

	// Adjust the row number in each assertion's position to refer to the line of
	// code *above* the assertion. There can be multiple lines of assertion comments and empty
	// lines, so the positions may have to be decremented by more than one row.
	i := 0
	lines := linesWithTerminator(source)
	for k := range result {
		a := &result[k]
		originalPosition := a.position
		for {
			onAssertionLine := slices.ContainsFunc(assertionRanges[i:], func(r pointRange) bool {
				return r.start.Row == a.position.row
			})
			onEmptyLine := len(lines[a.position.row]) <= a.position.column
			if onAssertionLine || onEmptyLine {
				if a.position.row > 0 {
					a.position.row--
				} else {
					return nil, fmt.Errorf("%w: could not find a line that corresponds to the assertion `%s` located at %s",
						errAssertion, a.expectedCaptureName, originalPosition)
				}
			} else {
				for i < len(assertionRanges) && assertionRanges[i].start.Row < a.position.row {
					i++
				}
				break
			}
		}
	}

	// The assertions can end up out of order due to the line adjustments.
	slices.SortStableFunc(result, func(a, b assertion) int {
		return a.position.compare(b.position)
	})

	return result, nil
}

// errAssertion is the error of an assertion that fails.
var errAssertion = errors.New("assertion failed")

// linesWithTerminator returns the lines of a text, each with its "\n".
func linesWithTerminator(source []byte) [][]byte {
	var out [][]byte
	for len(source) > 0 {
		i := bytes.IndexByte(source, '\n')
		if i < 0 {
			out = append(out, source)
			break
		}
		out = append(out, source[:i+1])
		source = source[i+1:]
	}
	return out
}
