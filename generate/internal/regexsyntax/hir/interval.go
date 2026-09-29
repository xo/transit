package hir

import (
	"cmp"
	"math"
	"slices"
)

// This file ports src/hir/interval.rs: a set of intervals, which is the
// storage of ClassUnicode and ClassBytes.
//
// The main invariant of an interval set is its canonical order. The set holds
// its intervals in order, and no two intervals overlap or touch. The code in
// this file breaks the invariant for a short time in some places, but a caller
// never sees that.
//
// Case folding breaks the invariant, so this file holds it, even though it
// does not fit well in a generic interval set. That is why this file uses the
// Unicode tables.
//
// Upstream wants a sequential form that uses no more memory than it must. In
// many places the code uses linear extra memory, but at most two times the
// size of the set, and amortized. Classes, and Unicode classes most of all,
// can be large, and upstream wants a regular expression to compile fast even
// in a debug build.
//
// The tests of interval sets are in hir_test.go, on the exported API of the
// HIR.
//
// The Rust traits Interval and Bound become the type constraints interval
// and bound. A method of the trait Interval with a default body becomes a
// generic function. A Rust interval is a value that a method changes in
// place. A Go method on a value cannot do that, so setLower and setUpper
// return the changed interval.

// intervalSet is a set of intervals, in canonical order.
//
// intervalSet is IntervalSet. The type parameter B is the type of a bound,
// which is the associated type Interval::Bound upstream.
type intervalSet[I interval[I, B], B bound] struct {
	// ranges is a sorted set of ranges that do not overlap.
	ranges []I
	// folded is true when the set is case folded. It is not needed for a
	// right result. It lets the set skip work that it did before, for
	// example to case fold a set that is case folded already. The set
	// operations keep it: if both sets are case folded, then the difference,
	// the union, the intersection and the symmetric difference are case
	// folded too.
	//
	// When folded is true, the set must be case folded. When it is false,
	// the set can be case folded or not. The code sets it to true only when
	// it knows that the set is case folded. It leaves it false when to know
	// the answer costs too much. So code cannot read false as "not case
	// folded".
	//
	// In short, folded makes the code faster, and does nothing else.
	folded bool
}

// equal reports whether the two sets hold the same intervals. It does not
// compare folded, because folded is only there to make the code faster.
//
// equal is the PartialEq of IntervalSet.
func (s *intervalSet[I, B]) equal(other *intervalSet[I, B]) bool {
	return slices.Equal(s.ranges, other.ranges)
}

// newIntervalSet returns a set of the intervals. Each interval holds both of
// its bounds. The intervals can be in any order, and they can overlap. The
// set holds a copy of the slice.
//
// newIntervalSet is IntervalSet::new.
func newIntervalSet[I interval[I, B], B bound](intervals []I) intervalSet[I, B] {
	ranges := slices.Clone(intervals)
	// An empty set is case folded.
	folded := len(ranges) == 0
	set := intervalSet[I, B]{ranges: ranges, folded: folded}
	set.canonicalize()
	return set
}

// clone returns a copy of the set that shares no memory with it.
//
// clone is the Clone of IntervalSet.
func (s *intervalSet[I, B]) clone() intervalSet[I, B] {
	return intervalSet[I, B]{ranges: slices.Clone(s.ranges), folded: s.folded}
}

// push adds an interval to the set.
//
// push is IntervalSet::push.
func (s *intervalSet[I, B]) push(interval I) {
	i, found := slices.BinarySearchFunc(s.ranges, interval, compareIntervals[I, B])
	if found {
		// This is an exact match, so interval is in the set already.
		return
	}

	// The search finds the first index where the start of the interval
	// before it is less than or equal to the start of the new interval. The
	// intervals of the set do not overlap, so the new interval can only join
	// the one interval before it.
	start := i
	if i > 0 {
		before := s.ranges[i-1]
		if union, ok := intervalUnion(before, interval); ok {
			interval = union
			start = i - 1
		}
	}
	// The new interval can overlap any number of the intervals after the
	// place where it goes. So join each of them until the first one that it
	// does not overlap.
	end := i
	for afterI := i; afterI < len(s.ranges); afterI++ {
		after := s.ranges[afterI]
		union, ok := intervalUnion(interval, after)
		if !ok {
			break
		}
		interval = union
		end = afterI + 1
	}
	s.ranges = slices.Replace(s.ranges, start, end, interval)

	// The code does not know if the set with the new interval is case
	// folded. So if it was case folded, it takes the safe choice: the whole
	// set is no longer case folded.
	s.folded = false
}

// intervals returns the intervals of the set, in canonical order. The caller
// must not change the slice.
//
// intervals is IntervalSet::intervals. IntervalSet::iter, which walks the
// same slice, is a range over it.
func (s *intervalSet[I, B]) intervals() []I {
	return s.ranges
}

// caseFoldSimple adds to the set every character that is a simple case fold
// of a character in the set. For example, if the set holds a-z, then the set
// holds a-z and A-Z after the fold.
//
// caseFoldSimple is IntervalSet::case_fold_simple. Upstream returns an error
// if the tables for case folding are not available. The port always has
// them, so it drops the error and the code that handles it.
func (s *intervalSet[I, B]) caseFoldSimple() {
	if s.folded {
		return
	}
	n := len(s.ranges)
	for i := range n {
		r := s.ranges[i]
		s.ranges = r.caseFoldSimple(s.ranges)
	}
	s.canonicalize()
	s.folded = true
}

// union adds the intervals of other to the set, in place.
//
// union is IntervalSet::union.
func (s *intervalSet[I, B]) union(other *intervalSet[I, B]) {
	if len(other.ranges) == 0 || slices.Equal(s.ranges, other.ranges) {
		return
	}
	// Upstream expects that a faster way to do this exists.
	s.ranges = append(s.ranges, other.ranges...)
	s.canonicalize()
	s.folded = s.folded && other.folded
}

// intersect keeps only the parts of the set that are in other too, in place.
//
// intersect is IntervalSet::intersect.
func (s *intervalSet[I, B]) intersect(other *intervalSet[I, B]) {
	if len(s.ranges) == 0 {
		return
	}
	if len(other.ranges) == 0 {
		s.ranges = s.ranges[:0]
		// An empty set is case folded.
		s.folded = true
		return
	}

	// Upstream found no simple way to do this in place with constant
	// memory. So the code appends the intersection to the end of the
	// ranges, and removes the old ranges before it returns.
	drainEnd := len(s.ranges)

	a, b := 0, 0
	for {
		if ab, ok := intervalIntersect(s.ranges[a], other.ranges[b]); ok {
			s.ranges = append(s.ranges, ab)
		}
		if s.ranges[a].upper() < other.ranges[b].upper() {
			a++
			if a >= drainEnd {
				break
			}
		} else {
			b++
			if b >= len(other.ranges) {
				break
			}
		}
	}
	s.ranges = slices.Delete(s.ranges, 0, drainEnd)
	s.folded = s.folded && other.folded
}

// difference removes the intervals of other from the set, in place.
//
// difference is IntervalSet::difference.
func (s *intervalSet[I, B]) difference(other *intervalSet[I, B]) {
	if len(s.ranges) == 0 || len(other.ranges) == 0 {
		return
	}

	// Upstream finds this algorithm surprisingly complex. Interval trees
	// or segment trees can do it, but upstream wants to avoid their cost at
	// run time and in the code. Read each line with care to follow it.
	//
	// The code can assume the canonical order here: in each set, all the
	// ranges are sorted, and no two of them overlap or touch.
	drainEnd := len(s.ranges)
	a, b := 0, 0
loop:
	for a < drainEnd && b < len(other.ranges) {
		// The easy cases are the ones where the two ranges do not overlap.
		// If the range b is before the range a, then skip b.
		if other.ranges[b].upper() < s.ranges[a].lower() {
			b++
			continue
		}
		// In the same way, if the range a is before the range b, then add
		// a as it is.
		if s.ranges[a].upper() < other.ranges[b].lower() {
			r := s.ranges[a]
			s.ranges = append(s.ranges, r)
			a++
			continue
		}
		// Otherwise the two ranges overlap.
		if intervalIsIntersectionEmpty(s.ranges[a], other.ranges[b]) {
			panic("assertion failed: !self.ranges[a].is_intersection_empty(&other.ranges[b])")
		}

		// This part is not easy, and the tests hold examples of it. There
		// are two reasons. First, the difference of two ranges can be two
		// ranges. Second, after the code takes a range away, a range after
		// it can still change the result. The loop below moves b forward
		// until the ranges b cannot change the range a.
		//
		// For example, if the range a is a-t and the next ranges b are a-c,
		// g-i, r-t and x-z, then the code takes three differences before it
		// goes on to the next range a.
		r := s.ranges[a]
		for b < len(other.ranges) && !intervalIsIntersectionEmpty(r, other.ranges[b]) {
			oldRange := r
			range1, ok1, range2, ok2 := intervalDifference(r, other.ranges[b])
			switch {
			case !ok1 && !ok2:
				// The whole range is gone, so go on to the next one without
				// adding this one.
				a++
				continue loop
			case ok1 && !ok2:
				r = range1
			case !ok1 && ok2:
				r = range2
			default:
				s.ranges = append(s.ranges, range1)
				r = range2
			}
			// The range b can change more. If it ends after the old range,
			// then it can change the next range a, and it changed the range
			// a as much as it can. So stop, and do not move b forward, so
			// that the next range a can use it.
			if other.ranges[b].upper() > oldRange.upper() {
				break
			}
			// Otherwise the next range b can change the range a.
			b++
		}
		s.ranges = append(s.ranges, r)
		a++
	}
	for a < drainEnd {
		r := s.ranges[a]
		s.ranges = append(s.ranges, r)
		a++
	}
	s.ranges = slices.Delete(s.ranges, 0, drainEnd)
	s.folded = s.folded && other.folded
}

// symmetricDifference changes the set, in place, to the characters that are
// in one of the two sets and not in both. It removes each character of the
// set that other holds too, and adds each character of other that the set
// does not hold.
//
// symmetricDifference is IntervalSet::symmetric_difference.
func (s *intervalSet[I, B]) symmetricDifference(other *intervalSet[I, B]) {
	// Upstream plans to amortize the allocation here.
	intersection := s.clone()
	intersection.intersect(other)
	s.union(other)
	s.difference(&intersection)
}

// negate changes the set to its complement. After it, the set holds each
// value that it did not hold before, and no value that it held.
//
// negate is IntervalSet::negate.
func (s *intervalSet[I, B]) negate() {
	if len(s.ranges) == 0 {
		lo, hi := minBound[B](), maxBound[B]()
		s.ranges = append(s.ranges, intervalCreate[I](lo, hi))
		// The set that holds everything must be case folded.
		s.folded = true
		return
	}

	// Upstream found no simple way to do this in place with constant
	// memory. So the code appends the negation to the end of the ranges,
	// and removes the old ranges before it returns.
	drainEnd := len(s.ranges)

	// The arithmetic below is checked, because of the canonical order.
	if s.ranges[0].lower() > minBound[B]() {
		upper := decrement(s.ranges[0].lower())
		s.ranges = append(s.ranges, intervalCreate[I](minBound[B](), upper))
	}
	for i := 1; i < drainEnd; i++ {
		lower := increment(s.ranges[i-1].upper())
		upper := decrement(s.ranges[i].lower())
		s.ranges = append(s.ranges, intervalCreate[I](lower, upper))
	}
	if s.ranges[drainEnd-1].upper() < maxBound[B]() {
		lower := increment(s.ranges[drainEnd-1].upper())
		s.ranges = append(s.ranges, intervalCreate[I](lower, maxBound[B]()))
	}
	s.ranges = slices.Delete(s.ranges, 0, drainEnd)
	// The negation does not change folded, because the negation keeps it
	// safe. If a set is not case folded, then its negation can be case
	// folded, for example [^☃]. But folded can be false for a case folded
	// set.
	//
	// If a set is case folded, then its negation is case folded too. Each
	// character of a case folded set comes with all the characters that it
	// case folds with. The negation takes each of these groups out whole,
	// and adds whole each group that was not in the set.
}

// canonicalize puts the set in canonical order.
//
// canonicalize is IntervalSet::canonicalize.
func (s *intervalSet[I, B]) canonicalize() {
	if s.isCanonical() {
		return
	}
	slices.SortFunc(s.ranges, compareIntervals[I, B])
	if len(s.ranges) == 0 {
		panic("assertion failed: !self.ranges.is_empty()")
	}

	// Upstream found no way to do this in place with constant memory. So
	// the code appends the canonical ranges to the end of the ranges, and
	// removes the old ranges before it returns.
	drainEnd := len(s.ranges)
	for oldi := range drainEnd {
		// If the code added one new range or more, and this range can join
		// the last range that it added, join them.
		if len(s.ranges) > drainEnd {
			last := &s.ranges[len(s.ranges)-1]
			if union, ok := intervalUnion(*last, s.ranges[oldi]); ok {
				*last = union
				continue
			}
		}
		r := s.ranges[oldi]
		s.ranges = append(s.ranges, r)
	}
	s.ranges = slices.Delete(s.ranges, 0, drainEnd)
}

// isCanonical reports whether the set is in canonical order.
//
// isCanonical is IntervalSet::is_canonical.
func (s *intervalSet[I, B]) isCanonical() bool {
	for i := 0; i+1 < len(s.ranges); i++ {
		if compareIntervals(s.ranges[i], s.ranges[i+1]) >= 0 {
			return false
		}
		if intervalIsContiguous(s.ranges[i], s.ranges[i+1]) {
			return false
		}
	}
	return true
}

// interval is an interval with a lower bound and an upper bound. Both bounds
// are in the interval.
//
// interval is the trait Interval. Its type parameter I is the type that
// implements it, Self upstream. The order of intervals is the derived Ord of
// the types that implement it: by the lower bound, and then by the upper
// bound. compareIntervals holds that order.
type interval[I any, B bound] interface {
	comparable
	// lower returns the lower bound.
	lower() B
	// upper returns the upper bound.
	upper() B
	// setLower returns the interval with the lower bound bound.
	setLower(bound B) I
	// setUpper returns the interval with the upper bound bound.
	setUpper(bound B) I
	// caseFoldSimple appends to intervals the simple case folds of the
	// interval, and returns the slice. Upstream returns an error too, which
	// the port drops, as IntervalSet.caseFoldSimple says.
	caseFoldSimple(intervals []I) []I
}

// compareIntervals compares two intervals by their lower bounds, and then by
// their upper bounds. It returns -1, 0 or +1, as cmp.Compare does.
//
// compareIntervals is the derived Ord of ClassUnicodeRange and
// ClassBytesRange.
func compareIntervals[I interval[I, B], B bound](a, b I) int {
	if c := cmp.Compare(a.lower(), b.lower()); c != 0 {
		return c
	}
	return cmp.Compare(a.upper(), b.upper())
}

// intervalCreate returns a new interval. If lower is more than upper, it
// swaps them.
//
// intervalCreate is Interval::create.
func intervalCreate[I interval[I, B], B bound](lower, upper B) I {
	var i I
	if lower <= upper {
		i = i.setLower(lower)
		i = i.setUpper(upper)
	} else {
		i = i.setLower(upper)
		i = i.setUpper(lower)
	}
	return i
}

// intervalUnion returns the union of two intervals, and true. If the two
// intervals do not overlap or touch, it returns false.
//
// intervalUnion is Interval::union.
func intervalUnion[I interval[I, B], B bound](a, b I) (I, bool) {
	if !intervalIsContiguous(a, b) {
		var zero I
		return zero, false
	}
	lower := min(a.lower(), b.lower())
	upper := max(a.upper(), b.upper())
	return intervalCreate[I](lower, upper), true
}

// intervalIntersect returns the intersection of two intervals, and true. If
// the intersection is empty, it returns false.
//
// intervalIntersect is Interval::intersect.
func intervalIntersect[I interval[I, B], B bound](a, b I) (I, bool) {
	lower := max(a.lower(), b.lower())
	upper := min(a.upper(), b.upper())
	if lower <= upper {
		return intervalCreate[I](lower, upper), true
	}
	var zero I
	return zero, false
}

// intervalDifference returns the ranges that are left when b is taken from
// a. The result is zero, one or two ranges, each with a bool that is true
// when the range is there. If there is one range, it is the first one.
//
// intervalDifference is Interval::difference, which returns (Option<Self>,
// Option<Self>).
func intervalDifference[I interval[I, B], B bound](a, b I) (I, bool, I, bool) {
	var zero I
	if intervalIsSubset(a, b) {
		return zero, false, zero, false
	}
	if intervalIsIntersectionEmpty(a, b) {
		return a, true, zero, false
	}
	addLower := b.lower() > a.lower()
	addUpper := b.upper() < a.upper()
	// This holds because a is not a subset of b, and the intersection of the
	// two ranges is not empty.
	if !addLower && !addUpper {
		panic("assertion failed: add_lower || add_upper")
	}
	ret0, ok0, ret1, ok1 := zero, false, zero, false
	if addLower {
		upper := decrement(b.lower())
		ret0, ok0 = intervalCreate[I](a.lower(), upper), true
	}
	if addUpper {
		lower := increment(b.upper())
		r := intervalCreate[I](lower, a.upper())
		if !ok0 {
			ret0, ok0 = r, true
		} else {
			ret1, ok1 = r, true
		}
	}
	return ret0, ok0, ret1, ok1
}

// intervalIsContiguous reports whether two intervals overlap or touch.
//
// intervalIsContiguous is Interval::is_contiguous.
func intervalIsContiguous[I interval[I, B], B bound](a, b I) bool {
	lower1 := asU32(a.lower())
	upper1 := asU32(a.upper())
	lower2 := asU32(b.lower())
	upper2 := asU32(b.upper())
	return max(lower1, lower2) <= saturatingAddU32(min(upper1, upper2), 1)
}

// intervalIsIntersectionEmpty reports whether the intersection of two
// intervals is empty.
//
// intervalIsIntersectionEmpty is Interval::is_intersection_empty.
func intervalIsIntersectionEmpty[I interval[I, B], B bound](a, b I) bool {
	lower1, upper1 := a.lower(), a.upper()
	lower2, upper2 := b.lower(), b.upper()
	return max(lower1, lower2) > min(upper1, upper2)
}

// intervalIsSubset reports whether a is a subset of b.
//
// intervalIsSubset is Interval::is_subset.
func intervalIsSubset[I interval[I, B], B bound](a, b I) bool {
	lower1, upper1 := a.lower(), a.upper()
	lower2, upper2 := b.lower(), b.upper()
	return (lower2 <= lower1 && lower1 <= upper2) &&
		(lower2 <= upper1 && upper1 <= upper2)
}

// saturatingAddU32 returns a + b, or the largest uint32 if the sum does not
// fit.
//
// saturatingAddU32 is u32::saturating_add.
func saturatingAddU32(a, b uint32) uint32 {
	if a > math.MaxUint32-b {
		return math.MaxUint32
	}
	return a + b
}

// bound is the type of the bounds of an interval: a byte or a rune.
//
// bound is the trait Bound. Its methods are the functions minBound,
// maxBound, asU32, increment and decrement, because a Go type parameter
// cannot add methods to byte and rune.
type bound interface {
	byte | rune
}

// minBound returns the smallest bound: 0 for a byte, and U+0000 for a rune.
//
// minBound is Bound::min_value.
func minBound[B bound]() B {
	return 0
}

// maxBound returns the largest bound: 0xFF for a byte, and U+10FFFF for a
// rune.
//
// maxBound is Bound::max_value.
func maxBound[B bound]() B {
	var b B
	switch any(b).(type) {
	case byte:
		var m byte = math.MaxUint8
		return B(m)
	default:
		var m rune = maxRune
		return B(m)
	}
}

// asU32 returns the bound as a uint32.
//
// asU32 is Bound::as_u32.
func asU32[B bound](b B) uint32 {
	return uint32(b)
}

// increment returns the bound after b. For a rune, the bound after U+D7FF
// is U+E000, because a surrogate is not a Unicode scalar value. It panics if
// b is the largest bound.
//
// increment is Bound::increment.
func increment[B bound](b B) B {
	switch x := any(b).(type) {
	case byte:
		if x == math.MaxUint8 {
			panic("called `Option::unwrap()` on a `None` value")
		}
	case rune:
		if x == 0xD7FF {
			var r rune = 0xE000
			return B(r)
		}
		if !isScalar(x + 1) {
			panic("called `Option::unwrap()` on a `None` value")
		}
	}
	return b + 1
}

// decrement returns the bound before b. For a rune, the bound before U+E000
// is U+D7FF, because a surrogate is not a Unicode scalar value. It panics if
// b is 0.
//
// decrement is Bound::decrement.
func decrement[B bound](b B) B {
	switch x := any(b).(type) {
	case byte:
		if x == 0 {
			panic("called `Option::unwrap()` on a `None` value")
		}
	case rune:
		if x == 0xE000 {
			var r rune = 0xD7FF
			return B(r)
		}
		if x == 0 || !isScalar(x-1) {
			panic("called `Option::unwrap()` on a `None` value")
		}
	}
	return b - 1
}

// maxRune is the largest Unicode scalar value, U+10FFFF.
const maxRune = 0x10FFFF

// isScalar reports whether r is a Unicode scalar value: a code point that is
// not a surrogate. A Rust char is always one, and char::from_u32 checks it.
func isScalar(r rune) bool {
	return 0 <= r && r <= maxRune && (r < 0xD800 || r > 0xDFFF)
}
