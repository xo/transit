package generate

import (
	"cmp"
	"iter"
	"slices"
	"unicode"
	"unicode/utf8"
)

// This file ports crates/generate/src/nfa.rs: the sets of characters and the
// automaton that the tokens of a grammar compile to. The Debug text of
// CharacterSet and Nfa, and the Hash of CharacterSet, wait for the modules
// that use them.

// charRange is a half-open range of code points.
//
// charRange is Range<u32>.
type charRange struct {
	start uint32
	end   uint32
}

// len returns the number of code points in the range.
func (r charRange) len() int {
	return int(r.end - r.start)
}

// CharacterSet is a set of characters, as sorted ranges that do not touch.
// The zero CharacterSet is the empty set.
//
// CharacterSet is CharacterSet, and the zero CharacterSet is
// CharacterSet::empty. A method that takes self in upstream, and returns
// Self, works like append: it can change the set that it is called on, so
// use only the set that it returns.
type CharacterSet struct {
	ranges []charRange
}

// NfaStateKind is the kind of a state of an NFA.
type NfaStateKind uint8

// The kinds of NFA state, in the order of the enum NfaState.
const (
	NfaAdvance NfaStateKind = iota
	NfaSplit
	NfaAccept
)

// NfaState is a state of an NFA.
//
// NfaState is NfaState, an enum with data upstream. The fields that a kind
// uses are:
//
//   - NfaAdvance: Chars, StateID, IsSep and Precedence.
//   - NfaSplit: Left and Right.
//   - NfaAccept: VariableIndex and Precedence.
type NfaState struct {
	Kind          NfaStateKind
	Chars         CharacterSet
	StateID       uint32
	IsSep         bool
	Precedence    int32
	Left          uint32
	Right         uint32
	VariableIndex int
}

// Nfa is a nondeterministic finite automaton, the tokens of a lexical
// grammar. The zero Nfa has no states.
//
// Nfa is Nfa, and the zero Nfa is Nfa::new.
type Nfa struct {
	States []NfaState
}

// NfaCursor is a set of states of an NFA, which moves on the characters of
// the input.
//
// NfaCursor is NfaCursor.
type NfaCursor struct {
	stateIDs []uint32
	nfa      *Nfa
}

// NfaTransition is a move of an NFA cursor on a set of characters.
//
// NfaTransition is NfaTransition.
type NfaTransition struct {
	Characters  CharacterSet
	IsSeparator bool
	Precedence  int32
	States      []uint32
}

// charEnd is the code point after the last one, the end of a set that holds
// the last code point.
//
// charEnd is END.
const charEnd = unicode.MaxRune + 1

// CharacterSetFromChar returns the set of one character.
//
// CharacterSetFromChar is CharacterSet::from_char.
func CharacterSetFromChar(c rune) CharacterSet {
	return CharacterSet{ranges: []charRange{{start: uint32(c), end: uint32(c) + 1}}}
}

// Negate returns the set of the characters that are not in s.
//
// Negate is CharacterSet::negate.
func (s CharacterSet) Negate() CharacterSet {
	i := 0
	var previousEnd uint32
	for i < len(s.ranges) {
		r := s.ranges[i]
		start := previousEnd
		previousEnd = r.end
		if start < r.start {
			s.ranges[i] = charRange{start: start, end: r.start}
			i++
		} else {
			s.ranges = slices.Delete(s.ranges, i, i+1)
		}
	}
	if previousEnd < charEnd {
		s.ranges = append(s.ranges, charRange{start: previousEnd, end: charEnd})
	}
	return s
}

// AddChar returns s with a character added.
//
// AddChar is CharacterSet::add_char.
func (s CharacterSet) AddChar(c rune) CharacterSet {
	s.addIntRange(0, uint32(c), uint32(c)+1)
	return s
}

// AddRange returns s with the characters from start to end added. The range
// holds end.
//
// AddRange is CharacterSet::add_range.
func (s CharacterSet) AddRange(start, end rune) CharacterSet {
	s.addIntRange(0, uint32(start), uint32(end)+1)
	return s
}

// Add returns s with the characters of other added.
//
// Add is CharacterSet::add.
func (s CharacterSet) Add(other CharacterSet) CharacterSet {
	index := 0
	for _, r := range other.ranges {
		index = s.addIntRange(index, r.start, r.end)
	}
	return s
}

// Assign makes s hold the characters of other. It reuses the memory of s.
//
// Assign is CharacterSet::assign.
func (s *CharacterSet) Assign(other CharacterSet) {
	s.ranges = append(s.ranges[:0], other.ranges...)
}

// addIntRange adds the code points from start to end, before end, and
// returns the index of the range that holds them. The search starts at the
// range with index i.
//
// addIntRange is CharacterSet::add_int_range.
func (s *CharacterSet) addIntRange(i int, start, end uint32) int {
	for i < len(s.ranges) {
		r := &s.ranges[i]
		if r.start > end {
			s.ranges = slices.Insert(s.ranges, i, charRange{start: start, end: end})
			return i
		}
		if r.end >= start {
			r.end = max(r.end, end)
			r.start = min(r.start, start)
			// join this range with the next range if needed
			for i+1 < len(s.ranges) && s.ranges[i+1].start <= s.ranges[i].end {
				s.ranges[i].end = max(s.ranges[i].end, s.ranges[i+1].end)
				s.ranges = slices.Delete(s.ranges, i+1, i+2)
			}
			return i
		}
		i++
	}
	s.ranges = append(s.ranges, charRange{start: start, end: end})
	return i
}

// DoesIntersect reports whether s and other have a character in common.
//
// DoesIntersect is CharacterSet::does_intersect.
func (s CharacterSet) DoesIntersect(other CharacterSet) bool {
	i, j := 0, 0
	for i < len(s.ranges) && j < len(other.ranges) {
		left, right := s.ranges[i], other.ranges[j]
		switch {
		case left.end <= right.start:
			i++
		case left.start >= right.end:
			j++
		default:
			return true
		}
	}
	return false
}

// RemoveIntersection returns the characters that are in both s and other,
// and removes them from both.
//
// RemoveIntersection is CharacterSet::remove_intersection.
func (s *CharacterSet) RemoveIntersection(other *CharacterSet) CharacterSet {
	var intersection []charRange
	leftI, rightI := 0, 0
	for leftI < len(s.ranges) && rightI < len(other.ranges) {
		left := &s.ranges[leftI]
		right := &other.ranges[rightI]
		switch {
		case left.start < right.start:
			// [ L ]
			//     [ R ]
			if left.end <= right.start {
				leftI++
				continue
			}
			switch {
			case left.end < right.end:
				// [ L ]
				//   [ R ]
				intersection = append(intersection, charRange{start: right.start, end: left.end})
				left.end, right.start = right.start, left.end
				leftI++
			case left.end == right.end:
				// [  L  ]
				//   [ R ]
				intersection = append(intersection, *right)
				left.end = right.start
				other.ranges = slices.Delete(other.ranges, rightI, rightI+1)
			default:
				// [   L   ]
				//   [ R ]
				intersection = append(intersection, *right)
				newRange := charRange{start: left.start, end: right.start}
				left.start = right.end
				s.ranges = slices.Insert(s.ranges, leftI, newRange)
				other.ranges = slices.Delete(other.ranges, rightI, rightI+1)
				leftI++
			}
		case left.start == right.start:
			switch {
			case left.end < right.end:
				// [ L ]
				// [  R  ]
				intersection = append(intersection, *left)
				right.start = left.end
				s.ranges = slices.Delete(s.ranges, leftI, leftI+1)
			case left.end == right.end:
				// [ L ]
				// [ R ]
				intersection = append(intersection, *left)
				s.ranges = slices.Delete(s.ranges, leftI, leftI+1)
				other.ranges = slices.Delete(other.ranges, rightI, rightI+1)
			default:
				// [  L  ]
				// [ R ]
				intersection = append(intersection, *right)
				left.start = right.end
				other.ranges = slices.Delete(other.ranges, rightI, rightI+1)
			}
		default:
			//     [ L ]
			// [ R ]
			if left.start >= right.end {
				rightI++
				continue
			}
			switch {
			case left.end < right.end:
				//   [ L ]
				// [   R   ]
				intersection = append(intersection, *left)
				newRange := charRange{start: right.start, end: left.start}
				right.start = left.end
				other.ranges = slices.Insert(other.ranges, rightI, newRange)
				s.ranges = slices.Delete(s.ranges, leftI, leftI+1)
				rightI++
			case left.end == right.end:
				//   [ L ]
				// [  R  ]
				intersection = append(intersection, *left)
				right.end = left.start
				s.ranges = slices.Delete(s.ranges, leftI, leftI+1)
			default:
				//   [   L   ]
				// [   R   ]
				intersection = append(intersection, charRange{start: left.start, end: right.end})
				left.start, right.end = right.end, left.start
				rightI++
			}
		}
	}
	return CharacterSet{ranges: intersection}
}

// Difference returns the characters of s that are not in other. Like Negate,
// it can change s, and it can change other too.
//
// Difference is CharacterSet::difference.
func (s CharacterSet) Difference(other CharacterSet) CharacterSet {
	s.RemoveIntersection(&other)
	return s
}

// CharCodes returns each code point in the set, surrogates included.
//
// CharCodes is CharacterSet::char_codes.
func (s CharacterSet) CharCodes() iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		for _, r := range s.ranges {
			for c := r.start; c < r.end; c++ {
				if !yield(c) {
					return
				}
			}
		}
	}
}

// Chars returns each character in the set. It skips the code points that are
// not characters, which are the surrogates.
//
// Chars is CharacterSet::chars.
func (s CharacterSet) Chars() iter.Seq[rune] {
	return func(yield func(rune) bool) {
		for c := range s.CharCodes() {
			if utf8.ValidRune(rune(c)) && !yield(rune(c)) {
				return
			}
		}
	}
}

// RangeCount returns the number of ranges in the set.
//
// RangeCount is CharacterSet::range_count.
func (s CharacterSet) RangeCount() int {
	return len(s.ranges)
}

// Ranges returns each range of the set as its first and its last character.
// A range starts and ends at a character, not at a surrogate, and a range of
// surrogates only is skipped.
//
// Ranges is CharacterSet::ranges.
func (s CharacterSet) Ranges() iter.Seq2[rune, rune] {
	return func(yield func(rune, rune) bool) {
		for _, r := range s.ranges {
			first, ok := firstChar(r)
			if !ok {
				continue
			}
			last, ok := lastChar(r)
			if !ok {
				continue
			}
			if !yield(first, last) {
				return
			}
		}
	}
}

// firstChar returns the first code point of a range that is a character.
func firstChar(r charRange) (rune, bool) {
	c := r.start
	if utf16Surrogate(c) {
		c = 0xE000
	}
	if c < r.end && c <= unicode.MaxRune {
		return rune(c), true
	}
	return 0, false
}

// lastChar returns the last code point of a range that is a character.
func lastChar(r charRange) (rune, bool) {
	c := min(r.end-1, unicode.MaxRune)
	if utf16Surrogate(c) {
		c = 0xD7FF
	}
	if c >= r.start && c < r.end {
		return rune(c), true
	}
	return 0, false
}

// utf16Surrogate reports whether a code point is a surrogate, which is not a
// character.
func utf16Surrogate(c uint32) bool {
	return 0xD800 <= c && c <= 0xDFFF
}

// IsEmpty reports whether the set is empty.
//
// IsEmpty is CharacterSet::is_empty.
func (s CharacterSet) IsEmpty() bool {
	return len(s.ranges) == 0
}

// SimplifyIgnoring returns a set with fewer ranges, which holds the same
// characters as s where the characters in ruledOut do not matter.
//
// SimplifyIgnoring is CharacterSet::simplify_ignoring.
func (s CharacterSet) SimplifyIgnoring(ruledOut CharacterSet) CharacterSet {
	var ranges []charRange
	var prev charRange
	hasPrev := false
	for i := 0; i <= len(s.ranges); i++ {
		// the loop runs once more after the last range, to add the range
		// that is left in prev
		last := i == len(s.ranges)
		if !last {
			r := s.ranges[i]
			if ruledOut.containsCodepointRange(r) {
				continue
			}
			if hasPrev && ruledOut.containsCodepointRange(charRange{start: prev.end, end: r.start}) {
				prev.end = r.end
				continue
			}
		}
		if hasPrev {
			ranges = append(ranges, prev)
		}
		if !last {
			prev, hasPrev = s.ranges[i], true
		}
	}
	return CharacterSet{ranges: ranges}
}

// containsCodepointRange reports whether one range of the set holds every
// code point of seek.
//
// containsCodepointRange is CharacterSet::contains_codepoint_range.
func (s CharacterSet) containsCodepointRange(seek charRange) bool {
	ix, _ := slices.BinarySearchFunc(s.ranges, seek, func(probe, seek charRange) int {
		switch {
		case probe.end <= seek.start:
			return -1
		case probe.start > seek.start:
			return 1
		}
		return 0
	})
	return ix < len(s.ranges) && s.ranges[ix].start <= seek.start && s.ranges[ix].end >= seek.end
}

// Contains reports whether the set holds a character.
//
// Contains is CharacterSet::contains.
func (s CharacterSet) Contains(c rune) bool {
	return s.containsCodepointRange(charRange{start: uint32(c), end: uint32(c) + 1})
}

// Clone returns a copy of the set. A method that works like append changes
// the memory of the set, so a caller that uses a set again copies it first.
//
// Clone is the Clone of CharacterSet.
func (s CharacterSet) Clone() CharacterSet {
	return CharacterSet{ranges: slices.Clone(s.ranges)}
}

// Equal reports whether two sets hold the same ranges.
//
// Equal is the PartialEq of CharacterSet.
func (s CharacterSet) Equal(other CharacterSet) bool {
	return slices.Equal(s.ranges, other.ranges)
}

// Compare orders two sets: by the number of code points, then range by
// range, by the length of the range and then by its first code point.
//
// Compare is the Ord of CharacterSet. Upstream compares two ranges of the
// same length code point by code point, and the first code points decide
// that.
func (s CharacterSet) Compare(other CharacterSet) int {
	count := func(ranges []charRange) int {
		n := 0
		for _, r := range ranges {
			n += r.len()
		}
		return n
	}
	if c := cmp.Compare(count(s.ranges), count(other.ranges)); c != 0 {
		return c
	}
	for i := range min(len(s.ranges), len(other.ranges)) {
		left, right := s.ranges[i], other.ranges[i]
		if c := cmp.Compare(left.len(), right.len()); c != 0 {
			return c
		}
		if c := cmp.Compare(left.start, right.start); c != 0 {
			return c
		}
	}
	return 0
}

// LastStateID returns the id of the last state. It panics when the NFA has
// no states.
//
// LastStateID is Nfa::last_state_id.
func (n *Nfa) LastStateID() uint32 {
	if len(n.States) == 0 {
		panic("generate: the NFA has no states")
	}
	return uint32(len(n.States) - 1)
}

// NewNfaCursor returns a cursor on an NFA at a set of states. It takes
// states, and the caller must not use it after.
//
// NewNfaCursor is NfaCursor::new.
func NewNfaCursor(nfa *Nfa, states []uint32) *NfaCursor {
	c := &NfaCursor{nfa: nfa}
	c.AddStates(states)
	return c
}

// Reset moves the cursor to a set of states, and to the states that they
// split to. It takes states, and the caller must not use it after.
//
// Reset is NfaCursor::reset.
func (c *NfaCursor) Reset(states []uint32) {
	c.stateIDs = c.stateIDs[:0]
	c.AddStates(states)
}

// ForceReset moves the cursor to exactly a set of states. It takes states,
// and the caller must not use it after.
//
// ForceReset is NfaCursor::force_reset.
func (c *NfaCursor) ForceReset(states []uint32) {
	c.stateIDs = states
}

// TransitionChars returns the characters of each move from the states of the
// cursor, and whether the move is a separator. The caller must not change a
// set.
//
// TransitionChars is NfaCursor::transition_chars.
func (c *NfaCursor) TransitionChars() iter.Seq2[CharacterSet, bool] {
	return func(yield func(CharacterSet, bool) bool) {
		for t := range c.rawTransitions() {
			if !yield(t.chars, t.isSep) {
				return
			}
		}
	}
}

// Transitions returns the moves from the states of the cursor, grouped so
// that no two moves share a character.
//
// Transitions is NfaCursor::transitions.
func (c *NfaCursor) Transitions() []NfaTransition {
	return groupTransitions(c.rawTransitions())
}

// TransitionsAndAnySep returns what Transitions returns, and whether any move
// is a separator.
//
// TransitionsAndAnySep is NfaCursor::transitions_and_any_sep.
func (c *NfaCursor) TransitionsAndAnySep() ([]NfaTransition, bool) {
	anySep := false
	result := groupTransitions(func(yield func(rawTransition) bool) {
		for t := range c.rawTransitions() {
			anySep = anySep || t.isSep
			if !yield(t) {
				return
			}
		}
	})
	return result, anySep
}

// rawTransition is one move of one state, before moves are grouped.
type rawTransition struct {
	chars CharacterSet
	isSep bool
	prec  int32
	state uint32
}

// rawTransitions returns the move of each state of the cursor that advances.
//
// rawTransitions is NfaCursor::raw_transitions.
func (c *NfaCursor) rawTransitions() iter.Seq[rawTransition] {
	return func(yield func(rawTransition) bool) {
		for _, id := range c.stateIDs {
			state := &c.nfa.States[id]
			if state.Kind != NfaAdvance {
				continue
			}
			if !yield(rawTransition{chars: state.Chars, isSep: state.IsSep, prec: state.Precedence, state: state.StateID}) {
				return
			}
		}
	}
}

// groupTransitions splits moves so that no two share a character, joins the
// moves that go to the same states, and sorts them by their characters.
//
// groupTransitions is NfaCursor::group_transitions.
func groupTransitions(transitions iter.Seq[rawTransition]) []NfaTransition {
	var result []NfaTransition
	// Reuse one set for each move. When the set goes into result, the loop
	// starts a new one, as mem::take does upstream.
	var chars CharacterSet
	for t := range transitions {
		chars.Assign(t.chars)
		for i := 0; i < len(result) && !chars.IsEmpty(); i++ {
			intersection := result[i].Characters.RemoveIntersection(&chars)
			if intersection.IsEmpty() {
				continue
			}
			charsIsEmpty := result[i].Characters.IsEmpty()
			var intersectionStates []uint32
			if charsIsEmpty {
				intersectionStates = result[i].States
			} else {
				intersectionStates = slices.Clone(result[i].States)
			}
			if j, found := slices.BinarySearch(intersectionStates, t.state); !found {
				intersectionStates = slices.Insert(intersectionStates, j, t.state)
			}
			intersectionTransition := NfaTransition{
				Characters:  intersection,
				IsSeparator: result[i].IsSeparator && t.isSep,
				Precedence:  max(result[i].Precedence, t.prec),
				States:      intersectionStates,
			}
			if charsIsEmpty {
				result[i] = intersectionTransition
			} else {
				// Upstream appends here instead of inserting at i. The
				// intersection and the rest of chars share no character, so
				// the loop passes over the intersection when it reaches it,
				// and the sort at the end sets the order.
				result = append(result, intersectionTransition)
			}
		}
		if !chars.IsEmpty() {
			result = append(result, NfaTransition{
				Characters:  chars,
				Precedence:  t.prec,
				States:      []uint32{t.state},
				IsSeparator: t.isSep,
			})
			chars = CharacterSet{}
		}
	}

	for i := 0; i < len(result); i++ {
		for j := range i {
			if slices.Equal(result[j].States, result[i].States) &&
				result[j].IsSeparator == result[i].IsSeparator &&
				result[j].Precedence == result[i].Precedence {
				result[j].Characters = result[j].Characters.Add(result[i].Characters)
				// swap_remove
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				i--
				break
			}
		}
	}

	// Upstream sorts with an unstable sort. No two moves share a character,
	// so no two sets compare equal, and any sort gives the same order.
	slices.SortFunc(result, func(a, b NfaTransition) int {
		return a.Characters.Compare(b.Characters)
	})
	return result
}

// Completions returns the variable index and the precedence of each state of
// the cursor that accepts.
//
// Completions is NfaCursor::completions.
func (c *NfaCursor) Completions() iter.Seq2[int, int32] {
	return func(yield func(int, int32) bool) {
		for _, id := range c.stateIDs {
			state := &c.nfa.States[id]
			if state.Kind != NfaAccept {
				continue
			}
			if !yield(state.VariableIndex, state.Precedence) {
				return
			}
		}
	}
}

// AddStates adds states to the cursor. A split state is not added, but the
// two states that it splits to are. It takes newStateIDs, and the caller must
// not use it after.
//
// AddStates is NfaCursor::add_states.
func (c *NfaCursor) AddStates(newStateIDs []uint32) {
	for i := 0; i < len(newStateIDs); i++ {
		stateID := newStateIDs[i]
		state := &c.nfa.States[stateID]
		if state.Kind == NfaSplit {
			left, right := state.Left, state.Right
			hasLeft, hasRight := false, false
			for _, id := range newStateIDs {
				if id == left {
					hasLeft = true
				}
				if id == right {
					hasRight = true
				}
			}
			if !hasLeft {
				newStateIDs = append(newStateIDs, left)
			}
			if !hasRight {
				newStateIDs = append(newStateIDs, right)
			}
		} else if j, found := slices.BinarySearch(c.stateIDs, stateID); !found {
			c.stateIDs = slices.Insert(c.stateIDs, j, stateID)
		}
	}
}
