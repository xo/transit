package generate

import (
	"iter"
	"math/bits"
)

// This file ports crates/generate/src/bitvec.rs. Upstream allocates the words
// of each BitVec from an arena that a thread owns, and frees them to a free
// list. The garbage collector replaces the arena, WordArena and its free list
// (D24).

// BitVec is a vector of bits in words of 64 bits. Token sets are joined many
// times, and a join of whole words is faster than a join of bits.
//
// The bits at and after Len are always zero.
//
// BitVec is BitVec.
type BitVec struct {
	words   []uint64
	numBits uint32
}

// NewBitVecWithCapacity returns an empty BitVec with room for n bits.
//
// NewBitVecWithCapacity is BitVec::with_capacity.
func NewBitVecWithCapacity(n int) BitVec {
	return BitVec{words: make([]uint64, 0, (n+63)/64)}
}

// wordsInUse returns the number of words that hold the bits before Len.
//
// wordsInUse is BitVec::words_in_use.
func (b *BitVec) wordsInUse() int {
	return (int(b.numBits) + 63) / 64
}

// Words returns the words in use.
//
// Words is BitVec::as_slice.
func (b *BitVec) Words() []uint64 {
	return b.words[:b.wordsInUse()]
}

// Len returns the number of bits.
//
// Len is BitVec::len.
func (b *BitVec) Len() int {
	return int(b.numBits)
}

// Get returns the bit at an index, and false when the index is not before
// Len.
//
// Get is BitVec::get.
func (b *BitVec) Get(index int) (bool, bool) {
	if index >= int(b.numBits) {
		return false, false
	}
	return b.words[index/64]>>(index%64)&1 != 0, true
}

// Set sets the bit at an index before Len.
//
// Set is BitVec::set.
func (b *BitVec) Set(index int, val bool) {
	if val {
		b.words[index/64] |= 1 << (index % 64)
	} else {
		b.words[index/64] &^= 1 << (index % 64)
	}
}

// ensureWords makes sure that the vector holds at least n words.
//
// ensureWords is BitVec::ensure_words.
func (b *BitVec) ensureWords(n int) {
	for len(b.words) < n {
		b.words = append(b.words, 0)
	}
}

// Resize changes Len to n. A new bit takes the value val.
//
// Resize is BitVec::resize.
func (b *BitVec) Resize(n int, val bool) {
	newWords := (n + 63) / 64
	oldWords := b.wordsInUse()
	b.ensureWords(newWords)
	var fill uint64
	if val {
		fill = ^uint64(0)
	}
	switch {
	case newWords > oldWords:
		for i := oldWords; i < newWords; i++ {
			b.words[i] = fill
		}
		// When the new bits are true, clear the bits of the last word at and
		// after n, so that the bits after Len stay zero.
		if val && n%64 != 0 {
			b.words[newWords-1] &= 1<<(n%64) - 1
		}
	case newWords < oldWords:
		// zero the words that are cut, so that nothing stale is visible
		for i := newWords; i < oldWords; i++ {
			b.words[i] = 0
		}
	}
	b.numBits = uint32(n)
}

// Last returns the last bit, and false when the vector is empty.
//
// Last is BitVec::last.
func (b *BitVec) Last() (bool, bool) {
	if b.numBits == 0 {
		return false, false
	}
	return b.Get(int(b.numBits) - 1)
}

// Pop removes the last bit and returns it, and false when the vector is empty.
//
// Pop is BitVec::pop.
func (b *BitVec) Pop() (bool, bool) {
	if b.numBits == 0 {
		return false, false
	}
	b.numBits--
	word, bit := int(b.numBits)/64, int(b.numBits)%64
	val := b.words[word]>>bit&1 != 0
	if val {
		b.words[word] &^= 1 << bit
	}
	// zero the word when it is no longer in use
	if word >= b.wordsInUse() {
		b.words[word] = 0
	}
	return val, true
}

// UnsetAll unsets every bit, keeping the length.
//
// UnsetAll is BitVec::unset_all.
func (b *BitVec) UnsetAll() {
	clear(b.words[:b.wordsInUse()])
}

// InsertAll sets each bit that is set in other, word by word, and reports
// whether any bit was new.
//
// InsertAll is BitVec::insert_all.
func (b *BitVec) InsertAll(other *BitVec) bool {
	otherWords := other.wordsInUse()
	if otherWords == 0 {
		return false
	}
	selfWords := b.wordsInUse()
	b.ensureWords(otherWords)
	// clear stale data in the words that the join is about to reach
	for i := selfWords; i < otherWords; i++ {
		b.words[i] = 0
	}
	b.numBits = max(b.numBits, other.numBits)
	var anyNew uint64
	for i, ow := range other.words[:otherWords] {
		anyNew |= ow &^ b.words[i]
		b.words[i] |= ow
	}
	return anyNew != 0
}

// Clone returns a copy of the vector.
//
// Clone is the Clone of BitVec.
func (b *BitVec) Clone() BitVec {
	return BitVec{words: append([]uint64(nil), b.Words()...), numBits: b.numBits}
}

// Equal reports whether two vectors hold the same bits. A missing word counts
// as zero, so two vectors that differ only in zero words at the end are equal.
//
// Equal is the PartialEq of BitVec.
func (b *BitVec) Equal(other *BitVec) bool {
	x, y := b.Words(), other.Words()
	for i := range max(len(x), len(y)) {
		if wordAt(x, i) != wordAt(y, i) {
			return false
		}
	}
	return true
}

// Compare orders two vectors. At the lowest bit where they differ, the vector
// with the bit set is the greater.
//
// Compare is the Ord of BitVec.
func (b *BitVec) Compare(other *BitVec) int {
	x, y := b.Words(), other.Words()
	for i := range max(len(x), len(y)) {
		xw, yw := wordAt(x, i), wordAt(y, i)
		if xw != yw {
			if xw>>bits.TrailingZeros64(xw^yw)&1 != 0 {
				return 1
			}
			return -1
		}
	}
	return 0
}

// key returns a string that is the same for two equal vectors, for a map key.
// It stands in for the Hash of BitVec, which hashes the words up to the last
// one that is not zero.
func (b *BitVec) key() string {
	w := b.Words()
	for len(w) > 0 && w[len(w)-1] == 0 {
		w = w[:len(w)-1]
	}
	buf := make([]byte, 0, 8*len(w))
	for _, x := range w {
		for i := range 8 {
			buf = append(buf, byte(x>>(8*i)))
		}
	}
	return string(buf)
}

// wordAt returns a word, or zero after the end.
func wordAt(w []uint64, i int) uint64 {
	if i < len(w) {
		return w[i]
	}
	return 0
}

// setBits returns the index of each set bit in words, from the lowest. It
// skips a zero word whole.
//
// setBits is SetBitsIter.
func setBits(words []uint64) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i, w := range words {
			for w != 0 {
				if !yield(i*64 + bits.TrailingZeros64(w)) {
					return
				}
				// clear the lowest set bit
				w &= w - 1
			}
		}
	}
}

// popCount returns the number of set bits in a word.
func popCount(w uint64) int {
	return bits.OnesCount64(w)
}
