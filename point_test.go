package transit

import (
	"math"
	"testing"
)

func TestPointArithmetic(t *testing.T) {
	tests := []struct {
		a, b     point
		add, sub point
	}{
		{point{1, 5}, point{0, 3}, point{1, 8}, point{1, 5}},
		{point{1, 5}, point{2, 3}, point{3, 3}, point{0, 2}},
		{point{3, 5}, point{1, 9}, point{4, 9}, point{2, 5}},
		// the column of a sub on one row does not go below zero
		{point{0, 2}, point{0, 5}, point{0, 7}, point{0, 0}},
		// uint32 wraps, as in C
		{point{0, math.MaxUint32}, point{0, 1}, point{0, 0}, point{0, math.MaxUint32 - 1}},
	}
	for _, test := range tests {
		if got := test.a.add(test.b); got != test.add {
			t.Errorf("%v.add(%v) = %v, want %v", test.a, test.b, got, test.add)
		}
		if got := test.a.sub(test.b); got != test.sub {
			t.Errorf("%v.sub(%v) = %v, want %v", test.a, test.b, got, test.sub)
		}
	}
}

func TestPointComparisons(t *testing.T) {
	points := []point{pointZero, {0, 1}, {1, 0}, {1, 7}, {2, 0}, pointMax}
	for i, a := range points {
		for j, b := range points {
			if got := a.lt(b); got != (i < j) {
				t.Errorf("%v.lt(%v) = %t", a, b, got)
			}
			if got := a.lte(b); got != (i <= j) {
				t.Errorf("%v.lte(%v) = %t", a, b, got)
			}
			if got := a.gt(b); got != (i > j) {
				t.Errorf("%v.gt(%v) = %t", a, b, got)
			}
			if got := a.gte(b); got != (i >= j) {
				t.Errorf("%v.gte(%v) = %t", a, b, got)
			}
			if got := a.eq(b); got != (i == j) {
				t.Errorf("%v.eq(%v) = %t", a, b, got)
			}
		}
	}
}

func TestPointConversion(t *testing.T) {
	p := newPoint(3, math.MaxUint32)
	if got := p.public(); got != (Point{Row: 3, Column: math.MaxUint32}) {
		t.Errorf("public() = %v", got)
	}
	if got := p.public().internal(); got != p {
		t.Errorf("public().internal() = %v, want %v", got, p)
	}
	if got := (Point{Row: 1 << 32, Column: 1<<32 + 5}).internal(); got != (point{0, 5}) {
		t.Errorf("internal() of a Point past 32 bits = %v, want it cut", got)
	}
}

func TestInputEditEditPoint(t *testing.T) {
	// an edit that replaces the bytes 4 to 6, on row 0, with the bytes 4 to 9,
	// which end on row 1
	e := InputEdit{
		StartByte:   4,
		OldEndByte:  6,
		NewEndByte:  9,
		StartPoint:  Point{0, 4},
		OldEndPoint: Point{0, 6},
		NewEndPoint: Point{1, 2},
	}
	tests := []struct {
		name      string
		p         Point
		b         int
		wantPoint Point
		wantByte  int
	}{
		{"before the edit", Point{0, 3}, 3, Point{0, 3}, 3},
		{"at the start of the edit", Point{0, 4}, 4, Point{0, 4}, 4},
		{"in the edit", Point{0, 5}, 5, Point{1, 2}, 9},
		{"after the edit", Point{0, 8}, 8, Point{1, 4}, 11},
	}
	for _, test := range tests {
		p, b := test.p, test.b
		e.EditPoint(&p, &b)
		if p != test.wantPoint || b != test.wantByte {
			t.Errorf("%s: EditPoint gives %v and %d, want %v and %d", test.name, p, b, test.wantPoint, test.wantByte)
		}
	}
}
