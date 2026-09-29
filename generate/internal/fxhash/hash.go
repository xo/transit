package fxhash

import "encoding/binary"

// This file ports the parts of library/core/src/hash/mod.rs of rustc 1.97.1
// that hash a u32 value and a slice of u32 values with a Hasher:
// BuildHasher::hash_one, the impl of Hash for [T], and the impl of Hash for
// u32 that the macro impl_write makes. The target is x86_64, which is little
// endian and has 64-bit pointers.

// hashOneU32 returns the hash of x under a new Hasher.
//
// hashOneU32 is BuildHasher::hash_one of FxBuildHasher for a u32. The Hash of
// a u32 calls Hasher::write_u32.
func hashOneU32(x uint32) uint64 {
	var h Hasher
	h.WriteU32(x)
	return h.Finish()
}

// HashU32Slice returns the hash of states under a new Hasher. The hash is
// the length of states as a usize, then the bytes of all the values, in one
// call to Write.
//
// Upstream tree-sitter calls it hash_state_set, in
// crates/generate/src/build_tables/token_conflicts.rs. A collision there
// changes the output, so the hash must be the same as upstream.
//
// HashU32Slice is `states.hash(&mut h); h.finish()` for `states: &[u32]` and
// `h = FxHasher::default()`. The Hash of [T] calls
// Hasher::write_length_prefix, which is Hasher::write_usize by default, and
// then Hash::hash_slice. The hash_slice of the macro impl_write gives the
// memory of the slice, in the byte order of the target, to Hasher::write.
func HashU32Slice(states []uint32) uint64 {
	var h Hasher
	h.WriteUsize(uint64(len(states)))
	buf := make([]byte, 0, 4*len(states))
	for _, s := range states {
		buf = binary.LittleEndian.AppendUint32(buf, s)
	}
	h.Write(buf)
	return h.Finish()
}
