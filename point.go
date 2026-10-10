package transit

import "math"

// This file ports lib/src/point.h and lib/src/point.c, and the type TSPoint
// of lib/include/tree_sitter/api.h. The runtime computes with point, which
// keeps the widths of TSPoint, and the exported API uses Point, which counts
// with int (D25). InputEdit.EditPoint is InputEdit::edit_point of the Rust
// binding, which calls ts_point_edit (D119).

// Point is a position as a row and a column. Both count from zero, and the
// column counts bytes.
//
// Point is TSPoint.
type Point struct {
	Row    int
	Column int
}

// point is TSPoint inside the runtime, with the widths of C.
type point struct {
	row    uint32
	column uint32
}

// pointZero is POINT_ZERO.
var pointZero = point{0, 0}

// pointMax is POINT_MAX.
var pointMax = point{math.MaxUint32, math.MaxUint32}

// public returns the point as a Point of the exported API.
func (a point) public() Point {
	return Point{Row: int(a.row), Column: int(a.column)}
}

// internal returns the Point as a point of the runtime. A row or a column
// that does not fit in 32 bits is cut, as a conversion in C cuts it.
func (p Point) internal() point {
	return point{row: uint32(p.Row), column: uint32(p.Column)}
}

// newPoint is point__new.
func newPoint(row, column uint32) point {
	return point{row, column}
}

// add is point_add.
func (a point) add(b point) point {
	if b.row > 0 {
		return newPoint(a.row+b.row, b.column)
	}
	return newPoint(a.row, a.column+b.column)
}

// sub is point_sub.
func (a point) sub(b point) point {
	if a.row > b.row {
		return newPoint(a.row-b.row, a.column)
	}
	if a.column >= b.column {
		return newPoint(0, a.column-b.column)
	}
	return newPoint(0, 0)
}

// lte is point_lte.
func (a point) lte(b point) bool {
	return (a.row < b.row) || (a.row == b.row && a.column <= b.column)
}

// lt is point_lt.
func (a point) lt(b point) bool {
	return (a.row < b.row) || (a.row == b.row && a.column < b.column)
}

// gt is point_gt.
func (a point) gt(b point) bool {
	return (a.row > b.row) || (a.row == b.row && a.column > b.column)
}

// gte is point_gte.
func (a point) gte(b point) bool {
	return (a.row > b.row) || (a.row == b.row && a.column >= b.column)
}

// eq is point_eq.
func (a point) eq(b point) bool {
	return a.row == b.row && a.column == b.column
}

// pointEdit is ts_point_edit of lib/src/point.c. The C function changes a
// point and its byte offset in place, and the Go function returns them.
func pointEdit(pt point, byteOffset uint32, edit InputEdit) (point, uint32) {
	startByte := byteOffset
	startPoint := pt

	if startByte >= uint32(edit.OldEndByte) {
		startByte = uint32(edit.NewEndByte) + (startByte - uint32(edit.OldEndByte))
		startPoint = edit.NewEndPoint.internal().add(startPoint.sub(edit.OldEndPoint.internal()))
	} else if startByte > uint32(edit.StartByte) {
		startByte = uint32(edit.NewEndByte)
		startPoint = edit.NewEndPoint.internal()
	}

	return startPoint, startByte
}

// EditPoint changes a point and its byte offset so that they stay at the
// same place in the text after the edit e. It needs no tree and no node.
//
// EditPoint is InputEdit::edit_point of the Rust binding, with
// ts_point_edit.
func (e InputEdit) EditPoint(p *Point, byteOffset *int) {
	pt, b := pointEdit(p.internal(), uint32(*byteOffset), e)
	*p = pt.public()
	*byteOffset = int(b)
}
