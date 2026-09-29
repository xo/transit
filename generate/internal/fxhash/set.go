package fxhash

import (
	"iter"
	"slices"
)

// This file ports two files named set.rs, for a set of u32 values with
// FxBuildHasher:
//
//   - The parts of library/std/src/collections/hash/set.rs of rustc 1.97.1
//     that collect an iterator into a HashSet and iterate over it. The
//     HashSet of std wraps the HashSet of hashbrown.
//   - The parts of src/set.rs of hashbrown 0.17.1, as the standard library
//     vendors it at library/vendor/hashbrown-0.17.1, that extend a HashSet
//     and iterate over it. The HashSet of hashbrown wraps a HashMap<u32, ()>.
//
// The two wrappers add nothing that changes the order, so one type hashSet
// stands for both.

// SetOrderU32 returns the values of values, with the duplicates dropped, in
// the order in which a Rust program iterates over them after it collects
// them into a hash set.
//
// Upstream tree-sitter iterates over such a set in
// crates/generate/src/build_tables/build_parse_table.rs, where it collects the
// variable_index of the items of a filter_map into an FxHashSet, and then
// names the parent symbols of an AmbiguousExtraError in the order of the set.
//
// SetOrderU32 is the order of iteration of
// `values.into_iter().collect::<FxHashSet<u32>>()`, where FxHashSet is
// std::collections::HashSet<u32, rustc_hash::FxBuildHasher>, in the standard
// library of rustc 1.97.1 on x86_64. The order depends on the lower bound of
// the size hint of the iterator. SetOrderU32 takes it to be 0, as it is for
// the filter_map of upstream.
func SetOrderU32(values []uint32) []uint32 {
	return slices.Collect(setFromIter(values, 0).iter())
}

// hashSet is a hash set of u32 values, with the hasher of lib.go.
//
// hashSet is std::collections::HashSet<u32, FxBuildHasher>, and the
// hashbrown::HashSet<u32, FxBuildHasher> in it.
type hashSet struct {
	mp hashMap
}

// setFromIter returns a set of values. sizeHint is the lower bound of the
// size hint of the Rust iterator that gives the values: 0 for a filter_map,
// and the length of the vector for vec::IntoIter.
//
// setFromIter is FromIterator::from_iter of the HashSet of std, which makes
// the set with HashSet::with_hasher and calls extend.
func setFromIter(values []uint32, sizeHint int) *hashSet {
	s := &hashSet{mp: newHashMap()}
	s.extend(values, sizeHint)
	return s
}

// extend inserts each value of values, in order. sizeHint is the lower bound
// of the size hint of the Rust iterator that gives the values.
//
// extend is Extend::extend of the HashSet of std, which calls Extend::extend
// of the HashSet of hashbrown. That calls Extend::extend of its map, with
// each value mapped to (value, ()), which keeps the size hint.
func (s *hashSet) extend(values []uint32, sizeHint int) {
	s.mp.extend(values, sizeHint)
}

// iter returns the values of the set, in the order of the buckets of its
// table.
//
// iter is HashSet::iter of std, which calls HashSet::iter of hashbrown. That
// iterates over the keys of its map, with RawTable::iter.
func (s *hashSet) iter() iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		it := s.mp.table.iter()
		for {
			v, ok := it.next()
			if !ok || !yield(v) {
				return
			}
		}
	}
}
