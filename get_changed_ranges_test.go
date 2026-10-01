package transit

import (
	"math"
	"slices"
	"testing"
)

// rangesOfBytes returns ranges on one row from pairs of byte offsets.
func rangesOfBytes(offsets ...uint32) []textRange {
	var ranges []textRange
	for i := 0; i+1 < len(offsets); i += 2 {
		ranges = append(ranges, textRange{
			startPoint: point{0, offsets[i]},
			endPoint:   point{0, offsets[i+1]},
			startByte:  offsets[i],
			endByte:    offsets[i+1],
		})
	}
	return ranges
}

// rangesBytes returns the byte offsets of ranges, as pairs.
func rangesBytes(ranges []Range) []int {
	var offsets []int
	for _, r := range ranges {
		offsets = append(offsets, r.StartByte, r.EndByte)
	}
	return offsets
}

func TestRangesAdd(t *testing.T) {
	var ranges []textRange
	rangeArrayAdd(&ranges, ln(2), ln(4))
	// an empty range adds nothing
	rangeArrayAdd(&ranges, ln(6), ln(6))
	// a range that starts in the last range extends it
	rangeArrayAdd(&ranges, ln(4), ln(5))
	rangeArrayAdd(&ranges, ln(7), ln(9))
	if want := rangesOfBytes(2, 5, 7, 9); !slices.Equal(ranges, want) {
		t.Errorf("the ranges are %+v, want %+v", ranges, want)
	}
}

func TestRangesIntersects(t *testing.T) {
	ranges := rangesOfBytes(2, 4, 6, 8)
	for _, test := range []struct {
		startIndex uint32
		start, end uint32
		want       bool
	}{
		{0, 0, 2, false},
		{0, 0, 3, true},
		{0, 3, 5, true},
		{0, 4, 6, false},
		{0, 7, 20, true},
		{0, 8, 20, false},
		{1, 3, 5, false},
		{1, 5, 7, true},
		{2, 0, 20, false},
	} {
		if got := rangeArrayIntersects(ranges, test.startIndex, test.start, test.end); got != test.want {
			t.Errorf("rangeArrayIntersects(%d, %d, %d) = %t, want %t", test.startIndex, test.start, test.end, got, test.want)
		}
	}
}

func TestRangesGetChangedRanges(t *testing.T) {
	for _, test := range []struct {
		name     string
		old, new []textRange
		want     []textRange
	}{
		{"none", nil, nil, nil},
		{"the same", rangesOfBytes(0, 5), rangesOfBytes(0, 5), nil},
		{"a new range", nil, rangesOfBytes(2, 5), rangesOfBytes(2, 5)},
		{"an old range", rangesOfBytes(2, 5), nil, rangesOfBytes(2, 5)},
		{"a longer range", rangesOfBytes(0, 5), rangesOfBytes(0, 10), rangesOfBytes(5, 10)},
		{"a gap that closes", rangesOfBytes(0, 5, 10, 15), rangesOfBytes(0, 15), rangesOfBytes(5, 10)},
		{"a range that moves", rangesOfBytes(0, 4), rangesOfBytes(2, 6), rangesOfBytes(0, 2, 4, 6)},
		{"two changes", rangesOfBytes(0, 2, 4, 6), rangesOfBytes(1, 2, 4, 8), rangesOfBytes(0, 1, 6, 8)},
		{"the default range", []textRange{defaultRange}, rangesOfBytes(0, 5), []textRange{{point{0, 5}, pointMax, 5, math.MaxUint32}}},
	} {
		var got []textRange
		rangeArrayGetChangedRanges(test.old, test.new, &got)
		if !slices.Equal(got, test.want) {
			t.Errorf("%s: the differences are %+v, want %+v", test.name, got, test.want)
		}
	}
}

func TestRangesEdit(t *testing.T) {
	// an edit that replaces the bytes 4 to 6 with the bytes 4 to 9
	edit := InputEdit{
		StartByte: 4, OldEndByte: 6, NewEndByte: 9,
		StartPoint: Point{Column: 4}, OldEndPoint: Point{Column: 6}, NewEndPoint: Point{Column: 9},
	}
	for _, test := range []struct {
		name string
		r    textRange
		want textRange
	}{
		{"before the edit", rangesOfBytes(0, 3)[0], rangesOfBytes(0, 3)[0]},
		{"after the edit", rangesOfBytes(7, 10)[0], rangesOfBytes(10, 13)[0]},
		{"around the edit", rangesOfBytes(2, 8)[0], rangesOfBytes(2, 11)[0]},
		{"in the edit", rangesOfBytes(5, 5)[0], rangesOfBytes(4, 4)[0]},
		{"to the end", defaultRange, defaultRange},
		{"a range whose end overflows", textRange{point{0, 7}, point{0, math.MaxUint32 - 1}, 7, math.MaxUint32 - 1},
			textRange{point{0, 10}, pointMax, 10, math.MaxUint32}},
	} {
		got := test.r
		got.edit(edit)
		if got != test.want {
			t.Errorf("%s: the range is %+v, want %+v", test.name, got, test.want)
		}
	}
}

// rangesTree returns a tree of the test language for "a + b", with sizes
// of its three leaves.
func rangesTree(l *Language, sizes [3]uint32, included []textRange) *Tree {
	pool := newSubtreePool(0)
	root := newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, sizes[0]),
		leaf(&pool, l, testSymPlus, 1, sizes[1]),
		leaf(&pool, l, testSymIdentifier, 1, sizes[2]),
	}, 1, l)
	return newTree(root, l, included)
}

func TestTreeChangedRanges(t *testing.T) {
	l := testLanguage(15)

	// two trees of the same shape have no changes
	oldTree := treeSample(l)
	if got := oldTree.ChangedRanges(treeSample(l)); len(got) != 0 {
		t.Errorf("the changed ranges of the same trees are %+v", got)
	}

	// the edits change b and z, which are in nodes with fields, aliases,
	// extras and hidden nodes, and the new tree has a "+" at each
	for _, test := range []struct {
		name  string
		b, z  Symbol
		start int
		want  []int
	}{
		{"b", testSymPlus, testSymIdentifier, 10, []int{10, 11}},
		{"z", testSymIdentifier, testSymPlus, 14, []int{14, 15}},
	} {
		oldTree = treeSample(l)
		oldTree.Edit(InputEdit{
			StartByte: test.start, OldEndByte: test.start + 1, NewEndByte: test.start + 1,
			StartPoint:  Point{Column: test.start},
			OldEndPoint: Point{Column: test.start + 1},
			NewEndPoint: Point{Column: test.start + 1},
		})
		if got := rangesBytes(oldTree.ChangedRanges(treeSampleWith(l, test.b, test.z))); !slices.Equal(got, test.want) {
			t.Errorf("the changed ranges of an edit of %s are %v, want %v", test.name, got, test.want)
		}
	}

	// the edit makes b longer, and the new tree has a longer b. The same
	// symbols at the same places are no change of the structure.
	oldTree = rangesTree(l, [3]uint32{1, 1, 1}, nil)
	oldTree.Edit(InputEdit{
		StartByte: 5, OldEndByte: 5, NewEndByte: 6,
		StartPoint: Point{Column: 5}, OldEndPoint: Point{Column: 5}, NewEndPoint: Point{Column: 6},
	})
	updated := rangesTree(l, [3]uint32{1, 1, 2}, nil)
	if got := oldTree.ChangedRanges(updated); len(got) != 0 {
		t.Errorf("the changed ranges of a longer leaf are %+v", got)
	}

	// the edit changes b, and the new tree has a "+" there
	pool := newSubtreePool(0)
	updated = newTree(newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 0, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
	}, 1, l), l, nil)
	edit := InputEdit{
		StartByte: 4, OldEndByte: 5, NewEndByte: 5,
		StartPoint: Point{Column: 4}, OldEndPoint: Point{Column: 5}, NewEndPoint: Point{Column: 5},
	}
	oldTree = rangesTree(l, [3]uint32{1, 1, 1}, nil)
	if got := oldTree.ChangedRanges(updated); len(got) != 0 {
		t.Errorf("the changed ranges of a tree with no edit are %+v", got)
	}
	oldTree.Edit(edit)
	if got := rangesBytes(oldTree.ChangedRanges(updated)); !slices.Equal(got, []int{4, 5}) {
		t.Errorf("the changed ranges of another symbol are %v", got)
	}
	// the compare finds a change only below a node that the edit changed
	if got := updated.ChangedRanges(oldTree); len(got) != 0 {
		t.Errorf("the changed ranges from the tree with no edit are %+v", got)
	}

	// a new tree that is longer adds the new end
	oldTree = rangesTree(l, [3]uint32{1, 1, 1}, nil)
	updated = rangesTree(l, [3]uint32{1, 1, 3}, nil)
	if got := rangesBytes(oldTree.ChangedRanges(updated)); !slices.Equal(got, []int{5, 7}) {
		t.Errorf("the changed ranges of a longer tree are %v", got)
	}
	if got := rangesBytes(updated.ChangedRanges(oldTree)); !slices.Equal(got, []int{5, 7}) {
		t.Errorf("the changed ranges of a shorter tree are %v", got)
	}

	// a new tree that starts later adds its padding and its new end. The
	// root has the same size, so the rest matches.
	oldTree = rangesTree(l, [3]uint32{1, 1, 1}, nil)
	pool = newSubtreePool(0)
	updated = newTree(newNode(&pool, testSymExpression, subtreeArray{
		leaf(&pool, l, testSymIdentifier, 2, 1),
		leaf(&pool, l, testSymPlus, 1, 1),
		leaf(&pool, l, testSymIdentifier, 1, 1),
	}, 1, l), l, nil)
	if got := rangesBytes(oldTree.ChangedRanges(updated)); !slices.Equal(got, []int{0, 2, 5, 7}) {
		t.Errorf("the changed ranges of a tree that starts later are %v", got)
	}

	// the same trees with other included ranges
	oldTree = rangesTree(l, [3]uint32{1, 1, 1}, rangesOfBytes(0, 3))
	updated = rangesTree(l, [3]uint32{1, 1, 1}, rangesOfBytes(0, 5))
	if got := oldTree.ChangedRanges(updated); len(got) != 0 {
		t.Errorf("the changed ranges of the same leaves with other included ranges are %+v", got)
	}
}

func TestIteratorComparisonString(t *testing.T) {
	for c, want := range map[iteratorComparison]string{
		iteratorDiffers:       "differs",
		iteratorMayDiffer:     "may differ",
		iteratorMatches:       "matches",
		iteratorComparison(9): unknownName,
	} {
		if got := c.String(); got != want {
			t.Errorf("String() = %s, want %s", got, want)
		}
	}
}
