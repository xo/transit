package fxhash

// This file ports src/control/tag.rs of hashbrown 0.17.1, as the standard
// library of rustc 1.97.1 vendors it at library/vendor/hashbrown-0.17.1. The
// port leaves out the Debug text and the trait TagSliceExt. fillEmpty in
// raw.go does what fill_empty does. A table that only inserts has no deleted
// buckets, so the port also leaves out Tag::DELETED and Tag::is_special. The
// debug assertion of Tag::special_is_empty, which calls is_special, is left
// out too.

// tag is a control byte. It tells whether a bucket is empty, deleted or
// full. A full bucket has 7 bits of the hash of its value.
//
// tag is Tag.
type tag uint8

// tagEmpty marks a bucket that is empty.
//
// tagEmpty is Tag::EMPTY.
const tagEmpty tag = 0b1111_1111

// isFull reports whether the tag marks a full bucket.
//
// isFull is Tag::is_full.
func (t tag) isFull() bool {
	return t&0x80 == 0
}

// specialIsEmpty reports whether a special tag is tagEmpty and not the tag
// of a deleted bucket.
//
// specialIsEmpty is Tag::special_is_empty.
func (t tag) specialIsEmpty() bool {
	return t&0x01 != 0
}

// tagFull returns the tag of a full bucket for hash: the top 7 bits of the
// hash.
//
// tagFull is Tag::full, on a target with 64-bit pointers.
func tagFull(hash uint64) tag {
	const minHashLen = 8
	top7 := hash >> (minHashLen*8 - 7)
	return tag(top7 & 0x7f)
}
