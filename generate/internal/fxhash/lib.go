package fxhash

import (
	"encoding/binary"
	"math/bits"
)

// This file ports src/lib.rs of the Rust crate rustc-hash 2.1.3, for a target
// with 64-bit pointers, such as x86_64. A usize of upstream is a uint64 here.
//
// The port leaves out the type aliases FxHashMap and FxHashSet, FxBuildHasher
// and the seeded and random states, because the generator uses only the
// hasher. The crate is built without its feature nightly, so the Hasher does
// not override write_length_prefix and write_str, and the defaults of the
// trait Hasher apply.

// Hasher is a fast hash that is not cryptographic. rustc uses it, because the
// hash map of the standard library uses SipHash, which is slower. The
// compiler does not have to resist denial of service attacks.
//
// The hash is a polynomial hash with one bit rotation at the end, which
// Orson Peters designed. The zero Hasher is ready to use.
//
// Hasher is FxHasher, and the zero Hasher is FxHasher::default.
type Hasher struct {
	hash uint64
}

// A polynomial hash
//
//	m[0] * k    + m[1] * k^2  + m[2] * k^3  + ...
//
// is also a multilinear hash with the keystream k[..]
//
//	m[0] * k[0] + m[1] * k[1] + m[2] * k[2] + ...
//
// where a multiplicative congruential pseudorandom number generator (MCG)
// makes the keystream k. For that reason, upstream chose a constant that is
// good for an MCG, from "Computationally Easy, Spectrally Good Multipliers for
// Congruential Pseudorandom Number Generators" by Guy Steele and Sebastiano
// Vigna.
//
// k is K for target_pointer_width = "64".
const k uint64 = 0xf1357aea2e62a9c5

// withSeed returns a hasher with the given seed.
//
// withSeed is FxHasher::with_seed.
func withSeed(seed uint64) Hasher {
	return Hasher{hash: seed}
}

// addToHash adds i to the hash.
//
// addToHash is FxHasher::add_to_hash.
func (h *Hasher) addToHash(i uint64) {
	h.hash = (h.hash + i) * k
}

// Write adds bytes to the hash. It compresses the bytes to one u64 and adds
// that to the hash.
//
// Write is Hasher::write of FxHasher.
func (h *Hasher) Write(bytes []byte) {
	h.WriteU64(hashBytes(bytes))
}

// WriteU8 adds i to the hash.
//
// WriteU8 is Hasher::write_u8 of FxHasher.
func (h *Hasher) WriteU8(i uint8) {
	h.addToHash(uint64(i))
}

// WriteU16 adds i to the hash.
//
// WriteU16 is Hasher::write_u16 of FxHasher.
func (h *Hasher) WriteU16(i uint16) {
	h.addToHash(uint64(i))
}

// WriteU32 adds i to the hash.
//
// WriteU32 is Hasher::write_u32 of FxHasher.
func (h *Hasher) WriteU32(i uint32) {
	h.addToHash(uint64(i))
}

// WriteU64 adds i to the hash.
//
// WriteU64 is Hasher::write_u64 of FxHasher.
func (h *Hasher) WriteU64(i uint64) {
	h.addToHash(i)
}

// writeU128 adds the u128 value with the low half lo and the high half hi to
// the hash. Go has no 128-bit integer, so the value comes in two halves.
//
// writeU128 is Hasher::write_u128 of FxHasher.
func (h *Hasher) writeU128(lo, hi uint64) {
	h.addToHash(lo)
	h.addToHash(hi)
}

// WriteUsize adds i to the hash. i is a usize of a target with 64-bit
// pointers.
//
// WriteUsize is Hasher::write_usize of FxHasher.
func (h *Hasher) WriteUsize(i uint64) {
	h.addToHash(i)
}

// Finish returns the hash.
//
// The hash is multiplicative, so the top bits have the most entropy. The top
// bit has the most, and it decreases bit by bit. Most hash tables, hashbrown
// too, compute the index of a bucket from the bottom bits, so Finish moves
// bits from the top to the bottom. The best rotation is by the size of the
// table, but the size is not known, so Finish rotates by 26 bits. That gives
// good entropy up to tables of 2^26 buckets.
//
// A bit reversal is better, but hashbrown also expects good entropy in the
// top 7 bits, and a bit reversal fills those bits with low entropy. Also, a
// bit reversal is very slow on x86-64. A byte reversal is faster, but it has
// a latency of 2 cycles on x86-64, and a rotation has a latency of 1 cycle.
// It has the problem with the top 7 bits too.
//
// Finish is Hasher::finish of FxHasher.
func (h *Hasher) Finish() uint64 {
	const rotate = 26
	return bits.RotateLeft64(h.hash, rotate)
}

// These seeds are digits of pi.
//
// seed1, seed2 and preventTrivialZeroCollapse are SEED1, SEED2 and
// PREVENT_TRIVIAL_ZERO_COLLAPSE.
const (
	seed1                      uint64 = 0x243f6a8885a308d3
	seed2                      uint64 = 0x13198a2e03707344
	preventTrivialZeroCollapse uint64 = 0xa4093822299f31d0
)

// multiplyMix returns the XOR of the low half and the high half of the full
// 128-bit product of x and y.
//
// The middle bits of the full product change the most when the input changes
// a little. These are the top bits of the low half and the bottom bits of the
// high half. The XOR of the two halves makes all of the output change when
// the input changes a little.
//
// Both 2^64 + 1 and 2^64 - 1 have small prime factors. If they had none, a
// sum or a difference of the halves gives a very strong hash, because
//
//	x * y = 2^64 * hi + lo = (-1) * hi + lo = lo - hi,   (mod 2^64 + 1)
//	x * y = 2^64 * hi + lo =    1 * hi + lo = lo + hi,   (mod 2^64 - 1)
//
// and multiplicative hashing is universal in a field (like mod p).
//
// multiplyMix is multiply_mix, on a target with a 64-bit to 128-bit
// multiplication, such as x86_64. The port leaves out the branch for 32-bit
// targets.
func multiplyMix(x, y uint64) uint64 {
	hi, lo := bits.Mul64(x, y)
	return lo ^ hi
}

// hashBytes returns a hash of bytes. Orson Peters designed it after wyhash.
// It does not resist collisions, and it is small and fast for short strings.
//
// The 64-bit version of the hash passes the test suite SMHasher3 on the full
// 64-bit output: f(hash_bytes(b) ^ f(seed)) passed all tests with no failures,
// for a good permutation f that avalanches.
//
// hashBytes does not avalanche, because the caller feeds the hash into a
// multiplication, and then takes the high bits, which avalanches.
//
// hashBytes is hash_bytes.
func hashBytes(bytes []byte) uint64 {
	n := len(bytes)
	s0 := seed1
	s1 := seed2

	switch {
	case n <= 16:
		// XOR the input into s0 and s1.
		switch {
		case n >= 8:
			s0 ^= binary.LittleEndian.Uint64(bytes[0:8])
			s1 ^= binary.LittleEndian.Uint64(bytes[n-8:])
		case n >= 4:
			s0 ^= uint64(binary.LittleEndian.Uint32(bytes[0:4]))
			s1 ^= uint64(binary.LittleEndian.Uint32(bytes[n-4:]))
		case n > 0:
			lo := bytes[0]
			mid := bytes[n/2]
			hi := bytes[n-1]
			s0 ^= uint64(lo)
			s1 ^= uint64(hi)<<8 | uint64(mid)
		}
	default:
		// Handle the bulk, which can overlap the suffix.
		bulk := bytes[:n-1]
		for len(bulk) >= 16 {
			x := binary.LittleEndian.Uint64(bulk[:8])
			y := binary.LittleEndian.Uint64(bulk[8:16])

			// Replace s1 with a mix of s0, x and y, and s0 with s1. The
			// compiler can then unroll the loop into two streams that do not
			// depend on each other, one on s0 and one on s1.
			//
			// Zeros are a common input, so the XOR of a constant with y keeps
			// the hash from a trivial collapse.
			t := multiplyMix(s0^x, preventTrivialZeroCollapse^y)
			s0 = s1
			s1 = t
			bulk = bulk[16:]
		}

		suffix := bytes[n-16:]
		s0 ^= binary.LittleEndian.Uint64(suffix[0:8])
		s1 ^= binary.LittleEndian.Uint64(suffix[8:16])
	}

	return multiplyMix(s0, s1) ^ uint64(n)
}
