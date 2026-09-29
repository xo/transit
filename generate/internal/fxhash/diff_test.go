package fxhash

import (
	"slices"
	"testing"
)

// This file compares the port with the values that Rust gives, which are in
// rust_test.go. The generators of the inputs are the same as the ones of the
// Rust program that made rust_test.go.

// splitMix64 is the pseudorandom number generator SplitMix64.
type splitMix64 uint64

// next returns the next number.
func (s *splitMix64) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// genValues returns n values of the given kind. The seed of the generator
// depends on kind and n.
func genValues(kind, n int) []uint32 {
	rng := splitMix64(uint64(kind)<<32 | uint64(n))
	values := make([]uint32, n)
	switch kind {
	case 0:
		for i := range values {
			values[i] = uint32(i)
		}
	case 1:
		for i := range values {
			values[i] = 1000 + uint32(i)
		}
	case 2:
		for i := range values {
			values[i] = uint32(rng.next() % 64)
		}
	case 3:
		for i := range values {
			values[i] = uint32(rng.next() % 4096)
		}
	case 4:
		for i := range values {
			values[i] = uint32(rng.next())
		}
	case 5:
		for i := range values {
			values[i] = uint32(n - 1 - i)
		}
	case 6:
		half := (n + 1) / 2
		base := make([]uint32, half)
		for i := range base {
			base[i] = uint32(rng.next() % 1_000_000)
		}
		for i := range values {
			values[i] = base[i%half]
		}
	case 7:
		for i := range values {
			values[i] = uint32(i) * 0x9e3779b9
		}
	default:
		panic("unknown kind")
	}
	return values
}

// genBytes returns n pseudorandom bytes.
func genBytes(n int) []byte {
	rng := splitMix64(99<<32 | uint64(n))
	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = byte(rng.next())
	}
	return bytes
}

// fnvDigest returns the FNV-1a hash of the little endian bytes of order.
func fnvDigest(order []uint32) uint64 {
	h := uint64(0xcbf29ce484222325)
	for _, v := range order {
		for range 4 {
			h ^= uint64(v & 0xff)
			h *= 0x100000001b3
			v >>= 8
		}
	}
	return h
}

// setOrder returns the order of values in a set, with the size hint of a
// filter_map, or with an exact size hint when exact is true.
func setOrder(values []uint32, exact bool) []uint32 {
	if !exact {
		return SetOrderU32(values)
	}
	return slices.Collect(setFromIter(values, len(values)).iter())
}

func TestHashU32Slice(t *testing.T) {
	for _, test := range hashSliceExplicit {
		if got := HashU32Slice(test.states); got != test.want {
			t.Errorf("HashU32Slice(%v) = %#x, want %#x", test.states, got, test.want)
		}
	}
	for _, test := range hashSliceGenerated {
		if got := HashU32Slice(genValues(test.kind, test.n)); got != test.want {
			t.Errorf("HashU32Slice(genValues(%d, %d)) = %#x, want %#x", test.kind, test.n, got, test.want)
		}
	}
}

func TestHasherWrite(t *testing.T) {
	for _, test := range hashBytesGenerated {
		var h Hasher
		h.Write(genBytes(test.n))
		if got := h.Finish(); got != test.want {
			t.Errorf("hash of genBytes(%d) = %#x, want %#x", test.n, got, test.want)
		}
	}
}

func TestSetOrderU32(t *testing.T) {
	for _, test := range setOrderExplicit {
		got := setOrder(genValues(test.kind, test.n), test.exact)
		if !slices.Equal(got, test.want) {
			t.Errorf("order of genValues(%d, %d), exact %t:\n got %v\nwant %v", test.kind, test.n, test.exact, got, test.want)
		}
	}
	for _, test := range setOrderDigests {
		for n, want := range test.digests {
			values := genValues(test.kind, n)
			order := setOrder(values, test.exact)
			if got := fnvDigest(order); got != want {
				t.Errorf("order of genValues(%d, %d), exact %t: digest %#x, want %#x", test.kind, n, test.exact, got, want)
			}
			// The order holds each value once.
			unique := slices.Clone(values)
			slices.Sort(unique)
			unique = slices.Compact(unique)
			sorted := slices.Sorted(slices.Values(order))
			if !slices.Equal(sorted, unique) {
				t.Errorf("order of genValues(%d, %d), exact %t: values %v, want %v", test.kind, n, test.exact, sorted, unique)
			}
		}
	}
}

func TestSetOrderU32Empty(t *testing.T) {
	if got := SetOrderU32(nil); len(got) != 0 {
		t.Errorf("SetOrderU32(nil) = %v, want no values", got)
	}
}
