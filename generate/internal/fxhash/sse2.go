package fxhash

// This file ports src/control/group/sse2.rs of hashbrown 0.17.1, as the
// standard library of rustc 1.97.1 vendors it at
// library/vendor/hashbrown-0.17.1. hashbrown uses the SSE2 group on x86_64,
// and upstream tree-sitter runs there. The port computes with plain Go what
// the SSE2 instructions compute. It leaves out store_aligned and
// convert_special_to_empty_and_full_to_deleted, which only a rehash in place
// calls.

// groupWidth is the number of control bytes in a group.
//
// groupWidth is Group::WIDTH.
const groupWidth = 16

// group is a group of control bytes, which the table tests together.
//
// group is Group.
type group [groupWidth]tag

// staticEmpty returns the control bytes of a table with no buckets: one group
// of empty tags.
//
// staticEmpty is Group::static_empty.
func staticEmpty() []tag {
	ctrl := make([]tag, groupWidth)
	fillEmpty(ctrl)
	return ctrl
}

// groupLoad loads the group that starts at the index pos of ctrl.
//
// groupLoad is Group::load, and Group::load_aligned too, because Go does not
// care about the alignment.
func groupLoad(ctrl []tag, pos int) group {
	var g group
	copy(g[:], ctrl[pos:pos+groupWidth])
	return g
}

// matchTag returns a mask of the bytes of the group that are equal to t.
//
// matchTag is Group::match_tag.
func (g group) matchTag(t tag) bitMask {
	var m bitMask
	for i, c := range g {
		if c == t {
			m |= 1 << i
		}
	}
	return m
}

// matchEmpty returns a mask of the bytes of the group that are empty.
//
// matchEmpty is Group::match_empty.
func (g group) matchEmpty() bitMask {
	return g.matchTag(tagEmpty)
}

// matchEmptyOrDeleted returns a mask of the bytes of the group that are
// empty or deleted. These are the bytes with the top bit set.
//
// matchEmptyOrDeleted is Group::match_empty_or_deleted.
func (g group) matchEmptyOrDeleted() bitMask {
	var m bitMask
	for i, c := range g {
		if c&0x80 != 0 {
			m |= 1 << i
		}
	}
	return m
}

// matchFull returns a mask of the bytes of the group that are full.
//
// matchFull is Group::match_full.
func (g group) matchFull() bitMask {
	return ^g.matchEmptyOrDeleted()
}
