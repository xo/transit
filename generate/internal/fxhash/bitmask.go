package fxhash

import "math/bits"

// This file ports src/control/bitmask.rs of hashbrown 0.17.1, as the standard
// library of rustc 1.97.1 vendors it at library/vendor/hashbrown-0.17.1, for
// the SSE2 group of sse2.go. There, a BitMaskWord is a u16, BITMASK_STRIDE is
// 1 and BITMASK_ITER_MASK is all ones. The target is not arm, so the branches
// for arm are left out, and so are the methods that the port does not call.

// bitMask has one bit for each byte of a group. A bit is set when the byte
// matches a test.
//
// bitMask is BitMask.
type bitMask uint16

// removeLowestBit returns the mask without its lowest set bit.
//
// removeLowestBit is BitMask::remove_lowest_bit.
func (b bitMask) removeLowestBit() bitMask {
	return b & (b - 1)
}

// anyBitSet reports whether a bit of the mask is set.
//
// anyBitSet is BitMask::any_bit_set.
func (b bitMask) anyBitSet() bool {
	return b != 0
}

// lowestSetBit returns the index of the lowest set bit, and false when no bit
// is set.
//
// lowestSetBit is BitMask::lowest_set_bit.
func (b bitMask) lowestSetBit() (int, bool) {
	if b == 0 {
		return 0, false
	}
	return bits.TrailingZeros16(uint16(b)), true
}

// iter returns an iterator over the set bits of the mask, from the lowest
// bit.
//
// iter is IntoIterator::into_iter of BitMask.
func (b bitMask) iter() bitMaskIter {
	return bitMaskIter{b}
}

// bitMaskIter iterates over the set bits of a mask, from the lowest bit.
//
// bitMaskIter is BitMaskIter.
type bitMaskIter struct {
	mask bitMask
}

// next returns the index of the next set bit, and false at the end.
//
// next is Iterator::next of BitMaskIter.
func (it *bitMaskIter) next() (int, bool) {
	bit, ok := it.mask.lowestSetBit()
	if !ok {
		return 0, false
	}
	it.mask = it.mask.removeLowestBit()
	return bit, true
}
