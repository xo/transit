package fxhash

import (
	"math"
	"testing"
)

// This file ports the tests of src/lib.rs of rustc-hash 2.1.3, for a target
// with 64-bit pointers. A test hashes a value with FxBuildHasher::hash_one,
// which calls the method of Hasher for the type of the value, and then
// Finish. A signed integer goes to the method for the unsigned type of the
// same width, as the defaults of the trait Hasher do.

// hashOf returns the hash of the value that write adds to a new Hasher.
//
// hashOf is FxBuildHasher.hash_one, in the macro test_hash.
func hashOf(write func(h *Hasher)) uint64 {
	var h Hasher
	write(&h)
	return h.Finish()
}

func u8Of(v uint8) func(*Hasher)         { return func(h *Hasher) { h.WriteU8(v) } }
func u16Of(v uint16) func(*Hasher)       { return func(h *Hasher) { h.WriteU16(v) } }
func u32Of(v uint32) func(*Hasher)       { return func(h *Hasher) { h.WriteU32(v) } }
func u64Of(v uint64) func(*Hasher)       { return func(h *Hasher) { h.WriteU64(v) } }
func u128Of(lo, hi uint64) func(*Hasher) { return func(h *Hasher) { h.writeU128(lo, hi) } }
func usizeOf(v uint64) func(*Hasher)     { return func(h *Hasher) { h.WriteUsize(v) } }
func bytesOf(v []byte) func(*Hasher)     { return func(h *Hasher) { h.Write(v) } }

// TestUnsigned is unsigned in lib.rs.
func TestUnsigned(t *testing.T) {
	tests := []struct {
		name  string
		write func(*Hasher)
		want  uint64
	}{
		{"0_u8", u8Of(0), 0},
		{"1_u8", u8Of(1), 12157901119326311915},
		{"100_u8", u8Of(100), 16751747135202103309},
		{"u8::MAX", u8Of(math.MaxUint8), 1211781028898739645},

		{"0_u16", u16Of(0), 0},
		{"1_u16", u16Of(1), 12157901119326311915},
		{"100_u16", u16Of(100), 16751747135202103309},
		{"u16::MAX", u16Of(math.MaxUint16), 16279819243059860173},

		{"0_u32", u32Of(0), 0},
		{"1_u32", u32Of(1), 12157901119326311915},
		{"100_u32", u32Of(100), 16751747135202103309},
		{"u32::MAX", u32Of(math.MaxUint32), 7729994835221066939},

		{"0_u64", u64Of(0), 0},
		{"1_u64", u64Of(1), 12157901119326311915},
		{"100_u64", u64Of(100), 16751747135202103309},
		{"u64::MAX", u64Of(math.MaxUint64), 6288842954450348564},

		{"0_u128", u128Of(0, 0), 0},
		{"1_u128", u128Of(1, 0), 13032756267696824044},
		{"100_u128", u128Of(100, 0), 12003541609544029302},
		{"u128::MAX", u128Of(math.MaxUint64, math.MaxUint64), 11702830760530184999},

		{"0_usize", usizeOf(0), 0},
		{"1_usize", usizeOf(1), 12157901119326311915},
		{"100_usize", usizeOf(100), 16751747135202103309},
		{"usize::MAX", usizeOf(math.MaxUint64), 6288842954450348564},
	}
	for _, test := range tests {
		if got := hashOf(test.write); got != test.want {
			t.Errorf("hash(%s) = %d, want %d", test.name, got, test.want)
		}
	}
}

// TestSigned is signed in lib.rs.
func TestSigned(t *testing.T) {
	// A signed value is cast to the unsigned type of the same width, as the
	// default methods write_i8 and so on of the trait Hasher do. A negative
	// i128 has all bits of its high half set.
	i8Of := func(v int8) func(*Hasher) { return u8Of(uint8(v)) }
	i16Of := func(v int16) func(*Hasher) { return u16Of(uint16(v)) }
	i32Of := func(v int32) func(*Hasher) { return u32Of(uint32(v)) }
	i64Of := func(v int64) func(*Hasher) { return u64Of(uint64(v)) }
	isizeOf := func(v int64) func(*Hasher) { return usizeOf(uint64(v)) }
	tests := []struct {
		name  string
		write func(*Hasher)
		want  uint64
	}{
		{"i8::MIN", i8Of(math.MinInt8), 6684841074112525780},
		{"0_i8", i8Of(0), 0},
		{"1_i8", i8Of(1), 12157901119326311915},
		{"100_i8", i8Of(100), 16751747135202103309},
		{"i8::MAX", i8Of(math.MaxInt8), 12973684028562874344},

		{"i16::MIN", i16Of(math.MinInt16), 14218860181193086044},
		{"0_i16", i16Of(0), 0},
		{"1_i16", i16Of(1), 12157901119326311915},
		{"100_i16", i16Of(100), 16751747135202103309},
		{"i16::MAX", i16Of(math.MaxInt16), 2060959061933882993},

		{"i32::MIN", i32Of(math.MinInt32), 9943947977240134995},
		{"0_i32", i32Of(0), 0},
		{"1_i32", i32Of(1), 12157901119326311915},
		{"100_i32", i32Of(100), 16751747135202103309},
		{"i32::MAX", i32Of(math.MaxInt32), 16232790931690483559},

		{"i64::MIN", i64Of(math.MinInt64), 33554432},
		{"0_i64", i64Of(0), 0},
		{"1_i64", i64Of(1), 12157901119326311915},
		{"100_i64", i64Of(100), 16751747135202103309},
		{"i64::MAX", i64Of(math.MaxInt64), 6288842954483902996},

		{"i128::MIN", u128Of(0, 1<<63), 33554432},
		{"0_i128", u128Of(0, 0), 0},
		{"1_i128", u128Of(1, 0), 13032756267696824044},
		{"100_i128", u128Of(100, 0), 12003541609544029302},
		{"i128::MAX", u128Of(math.MaxUint64, math.MaxInt64), 11702830760496630567},

		{"isize::MIN", isizeOf(math.MinInt64), 33554432},
		{"0_isize", isizeOf(0), 0},
		{"1_isize", isizeOf(1), 12157901119326311915},
		{"100_isize", isizeOf(100), 16751747135202103309},
		{"isize::MAX", isizeOf(math.MaxInt64), 6288842954483902996},
	}
	for _, test := range tests {
		if got := hashOf(test.write); got != test.want {
			t.Errorf("hash(%s) = %d, want %d", test.name, got, test.want)
		}
	}
}

// TestBytes is bytes in lib.rs. The test calls Write directly, so it does
// not depend on the Hash of a type.
func TestBytes(t *testing.T) {
	tests := []struct {
		bytes []byte
		want  uint64
	}{
		{[]byte{}, 17606491139363777937},
		{[]byte{0}, 5448590020104574886},
		{[]byte{0, 0, 0, 0, 0, 0}, 16766921560080789783},
		{[]byte{1}, 5922447956811044110},
		{[]byte{2}, 5229781508510959783},
		{[]byte("uwu"), 7168164714682931527},
		{[]byte("These are some bytes for testing rustc_hash."), 2349210501944688211},
	}
	for _, test := range tests {
		if got := hashOf(bytesOf(test.bytes)); got != test.want {
			t.Errorf("hash(%q) = %d, want %d", test.bytes, got, test.want)
		}
	}
}

// TestWithSeedActuallyDifferent is with_seed_actually_different in lib.rs.
func TestWithSeedActuallyDifferent(t *testing.T) {
	seeds := [][2]uint64{
		{1, 2},
		{42, 17},
		{124436707, 99237},
		{0, math.MaxUint64},
	}

	for _, seed := range seeds {
		for x := range math.MaxUint8 + 1 {
			a := withSeed(seed[0])
			b := withSeed(seed[1])

			a.WriteU8(uint8(x))
			b.WriteU8(uint8(x))

			if a.Finish() == b.Finish() {
				t.Errorf("seeds %d and %d give the same hash for %d", seed[0], seed[1], x)
			}
		}
	}
}
