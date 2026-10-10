package generate

import (
	"encoding/binary"
	"slices"
)

// This file ports crates/generate/src/build_tables/coincident_tokens.rs: the
// pairs of tokens that are valid in the same parse state. The Debug text of
// CoincidentTokenIndexDisplay is a debugging aid that upstream does not use,
// and it is not ported.

// CoincidentTokenIndex records which pairs of tokens are valid lookaheads in
// the same parse state.
//
// CoincidentTokenIndex is CoincidentTokenIndex.
type CoincidentTokenIndex struct {
	// containsBits is a flat bitset for fast Contains checks. Bit a*n+b is
	// the pair (a, b). Both (a, b) and (b, a) are set, so no min/max
	// normalization is needed.
	containsBits []uint64
	// withoutWordBits is a flat bitset for fast
	// AllCoincidentStatesHaveWord checks. Bit (a, b) is set if tokens a and
	// b are coincident in some parse state where the word token of the
	// grammar is not a valid lookahead. It answers the question "do all
	// states that contain this pair also contain the word token?".
	withoutWordBits []uint64
	// rowBits holds a bitset for each token, aligned to words, for the
	// intersection checks. Row a spans rowBits[a*rowWords : (a+1)*rowWords].
	// Bit b is set if tokens a and b are coincident in some parse state.
	rowBits []uint64
	n       int
}

// NewCoincidentTokenIndex returns the index of the tokens that are coincident
// in the states of a parse table. hasWordToken is false when the grammar has
// no word token.
//
// NewCoincidentTokenIndex is CoincidentTokenIndex::new. Upstream keeps the
// recorded sets of terminals in two FxHashSets, and only looks them up. The
// Go form maps the bytes of each sorted set to true. Upstream sorts with
// sort_unstable, and two indices that compare equal are the same value, so
// the order is the same in Go.
func NewCoincidentTokenIndex(table *ParseTable, lexicalGrammar *LexicalGrammar, wordToken Symbol, hasWordToken bool) *CoincidentTokenIndex {
	n := len(lexicalGrammar.Variables)
	rowWords := (n + 63) / 64
	result := &CoincidentTokenIndex{
		n:               n,
		containsBits:    make([]uint64, (n*n+63)/64),
		withoutWordBits: make([]uint64, (n*n+63)/64),
		rowBits:         make([]uint64, n*rowWords),
	}
	// Pre-collect terminal indices up front rather than continuously
	// recomputing within the loop below.
	var terminalIndices []uint32
	// The index only records which terminals share some state, so a state adds nothing if a
	// state with the same terminals was already recorded, with/without the word token.
	recordedWithWord := make(map[string]bool)
	recordedWithoutWord := make(map[string]bool)
	var key []byte
	for s := range table.States {
		state := &table.States[s]
		terminalIndices = terminalIndices[:0]
		for sym := range state.TerminalEntries.Keys() {
			if index, ok := sym.TerminalIndex(); ok {
				terminalIndices = append(terminalIndices, uint32(index))
			}
		}
		slices.Sort(terminalIndices)
		hasWord := hasWordToken && state.TerminalEntries.ContainsKey(wordToken)
		recorded := recordedWithoutWord
		if hasWord {
			recorded = recordedWithWord
		}
		key = key[:0]
		for _, index := range terminalIndices {
			key = binary.LittleEndian.AppendUint32(key, index)
		}
		if recorded[string(key)] {
			continue
		}
		recorded[string(key)] = true
		for i, a := range terminalIndices {
			for _, b := range terminalIndices[i:] {
				a, b := int(a), int(b)
				// Set both (a,b) and (b,a) bits so Contains needs no
				// min/max normalization.
				ab := a*n + b
				ba := b*n + a
				result.containsBits[ab/64] |= 1 << (ab % 64)
				result.containsBits[ba/64] |= 1 << (ba % 64)
				if !hasWord {
					result.withoutWordBits[ab/64] |= 1 << (ab % 64)
					result.withoutWordBits[ba/64] |= 1 << (ba % 64)
				}
				// Also populate the word-aligned row bitsets.
				result.rowBits[a*rowWords+b/64] |= 1 << (b % 64)
				result.rowBits[b*rowWords+a/64] |= 1 << (a % 64)
			}
		}
	}
	return result
}

// AllCoincidentStatesHaveWord reports whether the word token is a valid
// lookahead in every parse state where tokens a and b are both valid.
//
// AllCoincidentStatesHaveWord is
// CoincidentTokenIndex::all_coincident_states_have_word.
func (idx *CoincidentTokenIndex) AllCoincidentStatesHaveWord(a, b TerminalIndex) bool {
	bitIndex := int(a)*idx.n + int(b)
	return idx.withoutWordBits[bitIndex/64]&(1<<(bitIndex%64)) == 0
}

// Contains reports whether tokens a and b are both valid in some parse
// state.
//
// Contains is CoincidentTokenIndex::contains.
func (idx *CoincidentTokenIndex) Contains(a, b TerminalIndex) bool {
	bitIndex := int(a)*idx.n + int(b)
	return idx.containsBits[bitIndex/64]&(1<<(bitIndex%64)) != 0
}
