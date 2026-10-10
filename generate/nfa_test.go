package generate

import (
	"fmt"
	"slices"
	"testing"
	"unicode"
)

// characterSetFromRange returns the set of the characters from first to last,
// with last in the set. It swaps them when first is after last.
//
// characterSetFromRange is CharacterSet::from_range, which only the tests of
// upstream use.
func characterSetFromRange(first, last rune) CharacterSet {
	if first > last {
		first, last = last, first
	}
	return CharacterSet{ranges: []charRange{{start: uint32(first), end: uint32(last) + 1}}}
}

// symmetricDifference returns the characters that are in exactly one of s and
// other.
//
// symmetricDifference is CharacterSet::symmetric_difference, which only the
// tests of upstream use.
func symmetricDifference(s, other CharacterSet) CharacterSet {
	s.RemoveIntersection(&other)
	return s.Add(other)
}

// String returns the ranges of a set, for the messages of a failed test.
func (s CharacterSet) String() string {
	return fmt.Sprint(s.ranges)
}

// TestAddingRanges is test_adding_ranges in nfa.rs.
func TestAddingRanges(t *testing.T) {
	t.Parallel()
	set := CharacterSet{}.AddRange('c', 'm').AddRange('q', 's')

	// within existing range
	set = set.AddChar('d')
	expectSet(t, set, CharacterSet{}.AddRange('c', 'm').AddRange('q', 's'))

	// at end of existing range
	set = set.AddChar('m')
	expectSet(t, set, CharacterSet{}.AddRange('c', 'm').AddRange('q', 's'))

	// adjacent to end of existing range
	set = set.AddChar('n')
	expectSet(t, set, CharacterSet{}.AddRange('c', 'n').AddRange('q', 's'))

	// filling gap between existing ranges
	set = set.AddRange('o', 'p')
	expectSet(t, set, CharacterSet{}.AddRange('c', 's'))

	set = CharacterSet{}.AddRange('c', 'f').AddRange('i', 'l').AddRange('n', 'r')
	set = set.AddRange('d', 'o')
	expectSet(t, set, CharacterSet{}.AddRange('c', 'r'))
}

// TestAddingSortedRanges is test_adding_sorted_ranges in nfa.rs.
func TestAddingSortedRanges(t *testing.T) {
	t.Parallel()
	expected := make([]charRange, 4096)
	for i := range expected {
		expected[i] = charRange{start: uint32(4 * i), end: uint32(4*i + 2)}
	}
	set := CharacterSet{}
	for _, r := range expected {
		set = set.AddRange(rune(r.start), rune(r.end-1))
	}
	if !slices.Equal(set.ranges, expected) {
		t.Fatalf("got %d ranges %v, want %d ranges", len(set.ranges), set.ranges, len(expected))
	}

	// A touching range must still merge with the tail, not be appended.
	end := expected[len(expected)-1].end
	set = set.AddRange(rune(end), unicode.MaxRune)
	if got := set.RangeCount(); got != len(expected) {
		t.Errorf("RangeCount: got %d, want %d", got, len(expected))
	}
	if got := set.ranges[len(set.ranges)-1].end; got != charEnd {
		t.Errorf("end of the last range: got %d, want %d", got, charEnd)
	}
}

// TestAddingSets is test_adding_sets in nfa.rs.
func TestAddingSets(t *testing.T) {
	t.Parallel()
	set1 := CharacterSet{}.AddRange('c', 'f').AddRange('i', 'l')
	set2 := CharacterSet{}.AddRange('b', 'g').AddChar('h')
	expectSet(t, set1.Add(set2), CharacterSet{}.AddRange('b', 'g').AddRange('h', 'l'))
}

// TestGroupTransitions is test_group_transitions in nfa.rs.
func TestGroupTransitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in       []rawTransition
		expected []NfaTransition
	}{
		{
			"overlapping character classes",
			[]rawTransition{
				{CharacterSet{}.AddRange('a', 'f'), false, 0, 1},
				{CharacterSet{}.AddRange('d', 'i'), false, 1, 2},
			},
			[]NfaTransition{
				{Characters: CharacterSet{}.AddRange('a', 'c'), Precedence: 0, States: []uint32{1}},
				{Characters: CharacterSet{}.AddRange('d', 'f'), Precedence: 1, States: []uint32{1, 2}},
				{Characters: CharacterSet{}.AddRange('g', 'i'), Precedence: 1, States: []uint32{2}},
			},
		},
		{
			"large character class followed by many individual characters",
			[]rawTransition{
				{CharacterSet{}.AddRange('a', 'z'), false, 0, 1},
				{CharacterSet{}.AddChar('d'), false, 0, 2},
				{CharacterSet{}.AddChar('i'), false, 0, 3},
				{CharacterSet{}.AddChar('f'), false, 0, 4},
			},
			[]NfaTransition{
				{Characters: CharacterSet{}.AddChar('d'), States: []uint32{1, 2}},
				{Characters: CharacterSet{}.AddChar('f'), States: []uint32{1, 4}},
				{Characters: CharacterSet{}.AddChar('i'), States: []uint32{1, 3}},
				{
					Characters: CharacterSet{}.AddRange('a', 'c').AddChar('e').AddRange('g', 'h').AddRange('j', 'z'),
					States:     []uint32{1},
				},
			},
		},
		{
			"negated character class followed by an individual character",
			[]rawTransition{
				{CharacterSet{}.AddChar('0'), false, 0, 1},
				{CharacterSet{}.AddChar('b'), false, 0, 2},
				{CharacterSet{}.AddRange('a', 'f').Negate(), false, 0, 3},
				{CharacterSet{}.AddChar('c'), false, 0, 4},
			},
			[]NfaTransition{
				{Characters: CharacterSet{}.AddChar('0'), States: []uint32{1, 3}},
				{Characters: CharacterSet{}.AddChar('b'), States: []uint32{2}},
				{Characters: CharacterSet{}.AddChar('c'), States: []uint32{4}},
				{Characters: CharacterSet{}.AddRange('a', 'f').AddChar('0').Negate(), States: []uint32{3}},
			},
		},
		{
			"multiple negated character classes",
			[]rawTransition{
				{CharacterSetFromChar('a'), false, 0, 1},
				{characterSetFromRange('a', 'c').Negate(), false, 0, 2},
				{CharacterSetFromChar('g'), false, 0, 6},
				{characterSetFromRange('d', 'f').Negate(), false, 0, 3},
				{characterSetFromRange('g', 'i').Negate(), false, 0, 4},
				{CharacterSetFromChar('g'), false, 0, 5},
			},
			[]NfaTransition{
				{Characters: CharacterSetFromChar('a'), States: []uint32{1, 3, 4}},
				{Characters: CharacterSetFromChar('g'), States: []uint32{2, 3, 5, 6}},
				{Characters: characterSetFromRange('b', 'c'), States: []uint32{3, 4}},
				{Characters: characterSetFromRange('h', 'i'), States: []uint32{2, 3}},
				{Characters: characterSetFromRange('d', 'f'), States: []uint32{2, 4}},
				{Characters: characterSetFromRange('a', 'i').Negate(), States: []uint32{2, 3, 4}},
			},
		},
		{
			"disjoint characters with same state",
			[]rawTransition{
				{CharacterSetFromChar('a'), false, 0, 1},
				{CharacterSetFromChar('b'), false, 0, 2},
				{CharacterSetFromChar('c'), false, 0, 1},
				{CharacterSetFromChar('d'), false, 0, 1},
				{CharacterSetFromChar('e'), false, 0, 2},
			},
			[]NfaTransition{
				{Characters: CharacterSet{}.AddChar('b').AddChar('e'), States: []uint32{2}},
				{Characters: CharacterSet{}.AddChar('a').AddRange('c', 'd'), States: []uint32{1}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := groupTransitions(slices.Values(test.in))
			if !slices.EqualFunc(actual, test.expected, equalTransitions) {
				t.Errorf("expected %v, got: %v", test.expected, actual)
			}
		})
	}
}

// TestCharacterSetIntersectionDifferenceOps is
// test_character_set_intersection_difference_ops in nfa.rs.
func TestCharacterSetIntersectionDifferenceOps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		left, right, leftOnly, rightOnly, intersection CharacterSet
	}{
		// [ L ]
		//     [ R ]
		{
			left:         characterSetFromRange('a', 'f'),
			right:        characterSetFromRange('g', 'm'),
			leftOnly:     characterSetFromRange('a', 'f'),
			rightOnly:    characterSetFromRange('g', 'm'),
			intersection: CharacterSet{},
		},
		// [ L ]
		//   [ R ]
		{
			left:         characterSetFromRange('a', 'f'),
			right:        characterSetFromRange('c', 'i'),
			leftOnly:     characterSetFromRange('a', 'b'),
			rightOnly:    characterSetFromRange('g', 'i'),
			intersection: characterSetFromRange('c', 'f'),
		},
		// [  L  ]
		//   [ R ]
		{
			left:         characterSetFromRange('a', 'f'),
			right:        characterSetFromRange('d', 'f'),
			leftOnly:     characterSetFromRange('a', 'c'),
			rightOnly:    CharacterSet{},
			intersection: characterSetFromRange('d', 'f'),
		},
		// [   L   ]
		//   [ R ]
		{
			left:         characterSetFromRange('a', 'm'),
			right:        characterSetFromRange('d', 'f'),
			leftOnly:     CharacterSet{}.AddRange('a', 'c').AddRange('g', 'm'),
			rightOnly:    CharacterSet{},
			intersection: characterSetFromRange('d', 'f'),
		},
		// [    L    ]
		//         [R]
		{
			left:         characterSetFromRange(',', '/'),
			right:        CharacterSetFromChar('/'),
			leftOnly:     characterSetFromRange(',', '.'),
			rightOnly:    CharacterSet{},
			intersection: CharacterSetFromChar('/'),
		},
		// [    L    ]
		//         [R]
		{
			left:         characterSetFromRange(',', '/'),
			right:        CharacterSetFromChar('/'),
			leftOnly:     characterSetFromRange(',', '.'),
			rightOnly:    CharacterSet{},
			intersection: CharacterSetFromChar('/'),
		},
		// [ L1 ] [ L2 ]
		//    [  R  ]
		{
			left:         CharacterSet{}.AddRange('a', 'e').AddRange('h', 'l'),
			right:        characterSetFromRange('c', 'i'),
			leftOnly:     CharacterSet{}.AddRange('a', 'b').AddRange('j', 'l'),
			rightOnly:    characterSetFromRange('f', 'g'),
			intersection: CharacterSet{}.AddRange('c', 'e').AddRange('h', 'i'),
		},
		// [       L       ]
		//   [R1]    [R2]
		{
			left:         characterSetFromRange('a', 'm'),
			right:        CharacterSet{}.AddRange('c', 'd').AddRange('h', 'i'),
			leftOnly:     CharacterSet{}.AddRange('a', 'b').AddRange('e', 'g').AddRange('j', 'm'),
			rightOnly:    CharacterSet{},
			intersection: CharacterSet{}.AddRange('c', 'd').AddRange('h', 'i'),
		},
		// [L1] [L2] [L3] [L4] [L5]
		// [R1]      [R2]
		{
			left: CharacterSet{}.
				AddRange('a', 'b').
				AddRange('d', 'e').
				AddRange('g', 'h').
				AddRange('j', 'k').
				AddRange('m', 'n'),
			right:        CharacterSet{}.AddRange('a', 'b').AddRange('g', 'h'),
			leftOnly:     CharacterSet{}.AddRange('d', 'e').AddRange('j', 'k').AddRange('m', 'n'),
			rightOnly:    CharacterSet{},
			intersection: CharacterSet{}.AddRange('a', 'b').AddRange('g', 'h'),
		},
		// [L1] [ L2 ]
		//        [R]
		{
			left:         CharacterSet{}.AddRange('a', 'b').AddRange('d', 'f'),
			right:        characterSetFromRange('e', 'f'),
			leftOnly:     CharacterSet{}.AddRange('a', 'b').AddRange('d', 'd'),
			rightOnly:    CharacterSet{},
			intersection: characterSetFromRange('e', 'f'),
		},
	}
	for i, test := range tests {
		left, right := test.left.Clone(), test.right.Clone()
		if actual := left.RemoveIntersection(&right); !actual.Equal(test.intersection) {
			t.Errorf("row %da: %v && %v: expected %v, got: %v", i, test.left, test.right, test.intersection, actual)
		}
		if !left.Equal(test.leftOnly) {
			t.Errorf("row %da: %v - %v: expected %v, got: %v", i, test.left, test.right, test.leftOnly, left)
		}
		if !right.Equal(test.rightOnly) {
			t.Errorf("row %da: %v - %v: expected %v, got: %v", i, test.right, test.left, test.rightOnly, right)
		}

		left, right = test.left.Clone(), test.right.Clone()
		if actual := right.RemoveIntersection(&left); !actual.Equal(test.intersection) {
			t.Errorf("row %db: %v && %v: expected %v, got: %v", i, test.left, test.right, test.intersection, actual)
		}
		if !left.Equal(test.leftOnly) {
			t.Errorf("row %db: %v - %v: expected %v, got: %v", i, test.left, test.right, test.leftOnly, left)
		}
		if !right.Equal(test.rightOnly) {
			t.Errorf("row %db: %v - %v: expected %v, got: %v", i, test.right, test.left, test.rightOnly, right)
		}

		if actual := test.left.Clone().Difference(test.right.Clone()); !actual.Equal(test.leftOnly) {
			t.Errorf("row %db: %v -- %v: expected %v, got: %v", i, test.left, test.right, test.leftOnly, actual)
		}

		expected := test.leftOnly.Clone().Add(test.rightOnly)
		if actual := symmetricDifference(test.left.Clone(), test.right.Clone()); !actual.Equal(expected) {
			t.Errorf("row %db: %v ~~ %v: expected %v, got: %v", i, test.left, test.right, expected, actual)
		}
	}
}

// TestCharacterSetDoesIntersect is test_character_set_does_intersect in
// nfa.rs.
func TestCharacterSetDoesIntersect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b     CharacterSet
		expected bool
	}{
		{CharacterSet{}, CharacterSet{}, false},
		{CharacterSet{}.AddChar('a'), CharacterSet{}.AddChar('a'), true},
		{CharacterSet{}.AddChar('b'), CharacterSet{}.AddChar('a').AddChar('c'), false},
		{CharacterSetFromChar('b'), characterSetFromRange('a', 'c'), true},
		{CharacterSetFromChar('b'), characterSetFromRange('a', 'c').Negate(), false},
		{CharacterSetFromChar('a').Negate(), CharacterSetFromChar('a').Negate(), true},
		{CharacterSetFromChar('c'), CharacterSetFromChar('a').Negate(), true},
		{characterSetFromRange('c', 'f'), CharacterSetFromChar('f'), true},
	}
	for i, test := range tests {
		if actual := test.a.DoesIntersect(test.b); actual != test.expected {
			t.Errorf("row %d: %v and %v: expected %t, got: %t", i, test.a, test.b, test.expected, actual)
		}
		if actual := test.b.DoesIntersect(test.a); actual != test.expected {
			t.Errorf("row %d: %v and %v: expected %t, got: %t", i, test.b, test.a, test.expected, actual)
		}
	}
}

// TestCharacterSetSimplifyIgnoring is test_character_set_simplify_ignoring
// in nfa.rs. Each expected range holds both its ends.
func TestCharacterSetSimplifyIgnoring(t *testing.T) {
	t.Parallel()
	tests := []struct {
		chars          []rune
		ruledOutChars  []rune
		expectedRanges [][2]rune
	}{
		{[]rune{'a'}, nil, [][2]rune{{'a', 'a'}}},
		{[]rune{'a', 'b', 'c', 'e', 'z'}, nil, [][2]rune{{'a', 'c'}, {'e', 'e'}, {'z', 'z'}}},
		{[]rune{'a', 'b', 'c', 'e', 'h', 'z'}, []rune{'d', 'f', 'g'}, [][2]rune{{'a', 'h'}, {'z', 'z'}}},
		{[]rune{'a', 'b', 'c', 'g', 'h', 'i'}, []rune{'d', 'j'}, [][2]rune{{'a', 'c'}, {'g', 'i'}}},
		{[]rune{'c', 'd', 'e', 'g', 'h'}, []rune{'a', 'b', 'c', 'd', 'e', 'f'}, [][2]rune{{'g', 'h'}}},
		{[]rune{'I', 'N'}, []rune{'A', 'I', 'N', 'Z'}, nil},
	}
	for _, test := range tests {
		var ruledOut CharacterSet
		for _, c := range test.ruledOutChars {
			ruledOut = ruledOut.AddChar(c)
		}
		var set CharacterSet
		for _, c := range test.chars {
			set = set.AddChar(c)
		}
		var expected CharacterSet
		for _, r := range test.expectedRanges {
			expected = expected.AddRange(r[0], r[1])
		}
		if actual := set.SimplifyIgnoring(ruledOut); !actual.Equal(expected) {
			t.Errorf("chars: %q, ruled out chars: %q: expected %v, got: %v", test.chars, test.ruledOutChars, expected, actual)
		}
	}
}

// TestCharacterSetRangesSkipSurrogates makes sure that Ranges starts and ends
// each range at a character, as the conversion to char does upstream.
func TestCharacterSetRangesSkipSurrogates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		set      CharacterSet
		expected [][2]rune
	}{
		{"everything", CharacterSet{}.Negate(), [][2]rune{{0, 0x10FFFF}}},
		{"ends in the surrogates", CharacterSet{ranges: []charRange{{start: 'a', end: 0xDC00}}}, [][2]rune{{'a', 0xD7FF}}},
		{"starts in the surrogates", CharacterSet{ranges: []charRange{{start: 0xD900, end: 0xE005}}}, [][2]rune{{0xE000, 0xE004}}},
		{"only surrogates", CharacterSet{ranges: []charRange{{start: 'a', end: 'b'}, {start: 0xD800, end: 0xE000}}}, [][2]rune{{'a', 'a'}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var actual [][2]rune
			for first, last := range test.set.Ranges() {
				actual = append(actual, [2]rune{first, last})
			}
			if !slices.Equal(actual, test.expected) {
				t.Errorf("expected %q, got: %q", test.expected, actual)
			}
		})
	}
	n := 0
	for range characterSetFromRange(0xD7FE, 0xE001).Chars() {
		n++
	}
	if n != 4 {
		t.Errorf("expected 4 chars, got: %d", n)
	}
}

// TestNfaCursor makes sure that a cursor follows the split states, and gives
// the moves and the accepting states of the states that it is at.
func TestNfaCursor(t *testing.T) {
	t.Parallel()
	// The NFA of two tokens, "a" and "ab", with state 4 as the entry state.
	nfa := &Nfa{States: []NfaState{
		{Kind: NfaAccept, VariableIndex: 1, Precedence: 0},
		{Kind: NfaAdvance, Chars: CharacterSetFromChar('b'), StateID: 0},
		{Kind: NfaAdvance, Chars: CharacterSetFromChar('a'), StateID: 1},
		{Kind: NfaAccept, VariableIndex: 0, Precedence: 2},
		{Kind: NfaSplit, Left: 2, Right: 5},
		{Kind: NfaAdvance, Chars: CharacterSetFromChar('a'), StateID: 3, IsSep: true},
	}}
	if id := nfa.LastStateID(); id != 5 {
		t.Errorf("expected last state 5, got: %d", id)
	}
	cursor := NewNfaCursor(nfa, []uint32{4})
	if !slices.Equal(cursor.stateIDs, []uint32{2, 5}) {
		t.Fatalf("expected states [2 5], got: %v", cursor.stateIDs)
	}
	transitions, anySep := cursor.TransitionsAndAnySep()
	expected := []NfaTransition{{Characters: CharacterSetFromChar('a'), States: []uint32{1, 3}}}
	if !slices.EqualFunc(transitions, expected, equalTransitions) || !anySep {
		t.Errorf("expected %v and a separator, got: %v, %t", expected, transitions, anySep)
	}
	cursor.Reset(transitions[0].States)
	var completions [][2]int
	for index, prec := range cursor.Completions() {
		completions = append(completions, [2]int{index, int(prec)})
	}
	if !slices.Equal(completions, [][2]int{{0, 2}}) {
		t.Errorf("expected [[0 2]], got: %v", completions)
	}
}

// TestLexicalGrammarVariableIndices makes sure that the states of the NFA map
// to the variable whose start state is the first one at or after them.
func TestLexicalGrammarVariableIndices(t *testing.T) {
	t.Parallel()
	g := &LexicalGrammar{Variables: []LexicalVariable{{StartState: 2}, {StartState: 5}, {StartState: 6}}}
	actual := slices.Collect(g.VariableIndicesForNfaStates([]uint32{0, 1, 2, 3, 5, 6, 6}))
	if !slices.Equal(actual, []int{0, 1, 2}) {
		t.Errorf("expected [0 1 2], got: %v", actual)
	}
}

// expectSet fails the test when two sets differ.
func expectSet(t *testing.T, actual, expected CharacterSet) {
	t.Helper()
	if !actual.Equal(expected) {
		t.Errorf("expected %v, got: %v", expected, actual)
	}
}

// equalTransitions reports whether two moves are equal, as the PartialEq of
// NfaTransition does.
func equalTransitions(a, b NfaTransition) bool {
	return a.Characters.Equal(b.Characters) &&
		a.IsSeparator == b.IsSeparator &&
		a.Precedence == b.Precedence &&
		slices.Equal(a.States, b.States)
}
