package transit_test

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/xo/transit"
)

// This example fills an InputEdit for text that a user types at the end of
// a text of three rows. Tree.Edit takes it before the next parse. A point
// counts its column in bytes from the start of its row.
//
// The examples that parse a text are in the package
// github.com/xo/transit/grammars/json, because they need a grammar.
func ExampleInputEdit() {
	src := []byte("SELECT name\nFROM t\nWHERE")
	insert := []byte(" id = 1;\n")
	start := len(src) // the end of the text

	// point gives the point of a byte offset of a text.
	point := func(text []byte, offset int) transit.Point {
		row := bytes.Count(text[:offset], []byte("\n"))
		column := offset - (bytes.LastIndexByte(text[:offset], '\n') + 1)
		return transit.Point{Row: row, Column: column}
	}
	newSrc := slices.Concat(src[:start], insert, src[start:])
	edit := transit.InputEdit{
		StartByte:   start,
		OldEndByte:  start,
		NewEndByte:  start + len(insert),
		StartPoint:  point(src, start),
		OldEndPoint: point(src, start),
		NewEndPoint: point(newSrc, start+len(insert)),
	}
	fmt.Printf("%+v\n", edit)
	// Output:
	// {StartByte:24 OldEndByte:24 NewEndByte:33 StartPoint:{Row:2 Column:5} OldEndPoint:{Row:2 Column:5} NewEndPoint:{Row:3 Column:0}}
}
