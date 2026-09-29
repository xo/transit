package generate

import (
	"cmp"

	"github.com/xo/transit/generate/internal/fxhash"
)

// This file ports crates/generate/src/build_tables/token_conflicts.rs: the
// kinds of overlap between each pair of tokens. The Debug text of
// TokenConflictMapDisplay is a debugging aid that upstream does not use, and
// it is not ported. Upstream keeps the following tokens and the following
// characters in TokenConflictMap only for that text, so the port does not
// keep them.

// tokenConflictStatus is the conflict status of a pair of tokens (i, j),
// packed into one byte.
//
// tokenConflictStatus is TokenConflictStatus, a bitflags type.
type tokenConflictStatus uint8

// The flags of tokenConflictStatus.
const (
	conflictMatchesPrefix          tokenConflictStatus = 1 << 0
	conflictDoesMatchContinuation  tokenConflictStatus = 1 << 1
	conflictDoesMatchValidCont     tokenConflictStatus = 1 << 2
	conflictDoesMatchSeparators    tokenConflictStatus = 1 << 3
	conflictMatchesSameString      tokenConflictStatus = 1 << 4
	conflictMatchesDifferentString tokenConflictStatus = 1 << 5
)

// contains reports whether s holds every flag of other.
//
// contains is the contains of bitflags.
func (s tokenConflictStatus) contains(other tokenConflictStatus) bool {
	return s&other == other
}

// intersects reports whether s holds any flag of other.
//
// intersects is the intersects of bitflags.
func (s tokenConflictStatus) intersects(other tokenConflictStatus) bool {
	return s&other != 0
}

// TokenConflictMap holds the kinds of overlap between each pair of tokens of
// a lexical grammar.
//
// TokenConflictMap is TokenConflictMap.
type TokenConflictMap struct {
	n                    int
	statusMatrix         []tokenConflictStatus
	startingCharsByIndex []CharacterSet
	// conflictOrPrefixBits holds a bitset for each token, for fast batch
	// conflict checks. Row i spans conflictOrPrefixBits[i*rowWords :
	// (i+1)*rowWords]. Bit j is set if DoesConflict(i, j) or
	// DoesMatchPrefix(i, j).
	conflictOrPrefixBits []uint64
	// overlapEitherBits has bit j of row i set if DoesOverlap(i, j) or
	// DoesOverlap(j, i).
	overlapEitherBits []uint64
	rowWords          int
}

// NewTokenConflictMap creates a token conflict map from a lexical grammar,
// which describes the structure of each token, and followingTokens, which
// holds for each token the tokens that can come immediately after it.
//
// It analyzes the possible kinds of overlap between each pair of tokens and
// stores them in a matrix.
//
// NewTokenConflictMap is TokenConflictMap::new.
func NewTokenConflictMap(grammar *LexicalGrammar, followingTokens []TokenSet) *TokenConflictMap {
	cursor := NewNfaCursor(&grammar.Nfa, nil)
	startingChars := getStartingChars(cursor, grammar)
	followingChars := getFollowingChars(startingChars, followingTokens)

	// Pre-compute O(1) lookup: NFA state ID->owning variable index.
	// Replaces repeated O(log n) binary searches in computeConflictStatus.
	nfaStateToVar := make([]int, len(grammar.Nfa.States))
	for id := range nfaStateToVar {
		nfaStateToVar[id] = grammar.VariableIndexForNfaState(uint32(id))
	}

	n := len(grammar.Variables)
	statusMatrix := make([]tokenConflictStatus, n*n)
	for i := range grammar.Variables {
		for j := range i {
			status0, status1 := computeConflictStatus(cursor, grammar, followingChars, nfaStateToVar, i, j)
			statusMatrix[conflictMatrixIndex(n, i, j)] = status0
			statusMatrix[conflictMatrixIndex(n, j, i)] = status1
		}
	}

	// Precompute per-row bitsets for vectorized checkTokenConflicts.
	rowWords := (n + 63) / 64
	conflictMask := conflictDoesMatchValidCont |
		conflictDoesMatchSeparators |
		conflictMatchesSameString |
		conflictMatchesPrefix
	overlapMask := conflictDoesMatchSeparators |
		conflictMatchesPrefix |
		conflictMatchesSameString |
		conflictDoesMatchContinuation

	conflictOrPrefixBits := make([]uint64, n*rowWords)
	overlapEitherBits := make([]uint64, n*rowWords)
	for i := range n {
		rowBase := i * rowWords
		for j := range n {
			entryIJ := statusMatrix[conflictMatrixIndex(n, i, j)]
			if entryIJ.intersects(conflictMask) {
				conflictOrPrefixBits[rowBase+j/64] |= 1 << (j % 64)
			}
			entryJI := statusMatrix[conflictMatrixIndex(n, j, i)]
			if entryIJ.intersects(overlapMask) || entryJI.intersects(overlapMask) {
				overlapEitherBits[rowBase+j/64] |= 1 << (j % 64)
			}
		}
	}

	return &TokenConflictMap{
		n:                    n,
		statusMatrix:         statusMatrix,
		startingCharsByIndex: startingChars,
		conflictOrPrefixBits: conflictOrPrefixBits,
		overlapEitherBits:    overlapEitherBits,
		rowWords:             rowWords,
	}
}

// HasSameConflictStatus reports whether tokens a and b have the same
// conflict status with token other.
//
// HasSameConflictStatus is TokenConflictMap::has_same_conflict_status.
func (m *TokenConflictMap) HasSameConflictStatus(a, b, other int) bool {
	left := m.statusMatrix[conflictMatrixIndex(m.n, a, other)]
	right := m.statusMatrix[conflictMatrixIndex(m.n, b, other)]
	return left == right
}

// DoesMatchDifferentString reports whether token i matches any strings that
// token j does not match.
//
// DoesMatchDifferentString is TokenConflictMap::does_match_different_string.
func (m *TokenConflictMap) DoesMatchDifferentString(i, j int) bool {
	return m.statusMatrix[conflictMatrixIndex(m.n, i, j)].contains(conflictMatchesDifferentString)
}

// DoesMatchSameString reports whether token i matches any strings that token
// j also matches, where token i is preferred over token j.
//
// DoesMatchSameString is TokenConflictMap::does_match_same_string.
func (m *TokenConflictMap) DoesMatchSameString(i, j int) bool {
	return m.statusMatrix[conflictMatrixIndex(m.n, i, j)].contains(conflictMatchesSameString)
}

// DoesConflict reports whether the status of the pair (i, j) holds
// DOES_MATCH_VALID_CONT, DOES_MATCH_SEPARATORS or MATCHES_SAME_STRING.
//
// DoesConflict is TokenConflictMap::does_conflict. Upstream reads the matrix
// without a bounds check. Go checks the bounds, and an index out of bounds
// panics.
func (m *TokenConflictMap) DoesConflict(i, j int) bool {
	entry := m.statusMatrix[conflictMatrixIndex(m.n, i, j)]
	return entry.intersects(conflictDoesMatchValidCont | conflictDoesMatchSeparators | conflictMatchesSameString)
}

// DoesMatchPrefix reports whether token i matches any strings that are
// prefixes of strings that token j matches.
//
// DoesMatchPrefix is TokenConflictMap::does_match_prefix.
func (m *TokenConflictMap) DoesMatchPrefix(i, j int) bool {
	return m.statusMatrix[conflictMatrixIndex(m.n, i, j)].contains(conflictMatchesPrefix)
}

// DoesMatchShorterOrLonger reports whether the status of the pair (i, j)
// holds DOES_MATCH_VALID_CONT or DOES_MATCH_SEPARATORS, and the status of the
// pair (j, i) does not hold DOES_MATCH_SEPARATORS.
//
// DoesMatchShorterOrLonger is TokenConflictMap::does_match_shorter_or_longer.
func (m *TokenConflictMap) DoesMatchShorterOrLonger(i, j int) bool {
	entry := m.statusMatrix[conflictMatrixIndex(m.n, i, j)]
	reverseEntry := m.statusMatrix[conflictMatrixIndex(m.n, j, i)]
	return entry.intersects(conflictDoesMatchValidCont|conflictDoesMatchSeparators) &&
		!reverseEntry.contains(conflictDoesMatchSeparators)
}

// DoesOverlap reports whether token i overlaps token j in any way.
//
// DoesOverlap is TokenConflictMap::does_overlap.
func (m *TokenConflictMap) DoesOverlap(i, j int) bool {
	return m.statusMatrix[conflictMatrixIndex(m.n, i, j)].intersects(
		conflictDoesMatchSeparators |
			conflictMatchesPrefix |
			conflictMatchesSameString |
			conflictDoesMatchContinuation,
	)
}

// PreferToken reports whether the lexer prefers the left token over the
// right token, when both match the same string. A token is its precedence
// and its variable index.
//
// PreferToken is TokenConflictMap::prefer_token. Upstream takes each token as
// a tuple (i32, usize).
func PreferToken(grammar *LexicalGrammar, leftPrecedence int32, leftID int, rightPrecedence int32, rightID int) bool {
	switch cmp.Compare(leftPrecedence, rightPrecedence) {
	case -1:
		return false
	case 1:
		return true
	}
	switch cmp.Compare(grammar.Variables[leftID].ImplicitPrecedence, grammar.Variables[rightID].ImplicitPrecedence) {
	case -1:
		return false
	case 1:
		return true
	}
	return leftID < rightID
}

// PreferTransition reports whether the lexer prefers to take transition t
// over accepting the completed token.
//
// PreferTransition is TokenConflictMap::prefer_transition.
func PreferTransition(grammar *LexicalGrammar, t *NfaTransition, completedID int, completedPrecedence int32, hasSeparatorTransitions bool) bool {
	if t.Precedence < completedPrecedence {
		return false
	}
	if t.Precedence == completedPrecedence {
		if t.IsSeparator {
			return false
		}
		if hasSeparatorTransitions {
			found := false
			for i := range grammar.VariableIndicesForNfaStates(t.States) {
				if i == completedID {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// conflictMatrixIndex returns the index of the pair (i, j) in the status
// matrix.
//
// conflictMatrixIndex is matrix_index.
func conflictMatrixIndex(variableCount, i, j int) int {
	return variableCount*i + j
}

// getStartingChars returns, for each token, the characters that a string of
// the token can start with.
//
// getStartingChars is get_starting_chars.
func getStartingChars(cursor *NfaCursor, grammar *LexicalGrammar) []CharacterSet {
	result := make([]CharacterSet, 0, len(grammar.Variables))
	for _, variable := range grammar.Variables {
		cursor.Reset([]uint32{variable.StartState})
		var allChars CharacterSet
		for chars := range cursor.TransitionChars() {
			allChars = allChars.Add(chars)
		}
		result = append(result, allChars)
	}
	return result
}

// getFollowingChars returns, for each token, the characters that the tokens
// that can follow it can start with.
//
// getFollowingChars is get_following_chars.
func getFollowingChars(startingChars []CharacterSet, followingTokens []TokenSet) []CharacterSet {
	result := make([]CharacterSet, 0, len(followingTokens))
	for i := range followingTokens {
		var chars CharacterSet
		for token := range followingTokens[i].All() {
			if index, ok := token.TerminalIndex(); ok {
				chars = chars.Add(startingChars[index])
			}
		}
		result = append(result, chars)
	}
	return result
}

// hashStateSet hashes a sorted slice of NFA state IDs to a single uint64,
// for use as a key of the visited set.
//
// NOTE: This trades exact equality for speed. Two distinct state sets that
// hash to the same uint64 will be treated as "already visited", and the
// search can skip a state set. In practice the probability of a collision is
// negligible (1 in 2^64 per comparison), but this is a probabilistic
// optimization rather than an exact one. The port uses the same hash as
// upstream, so that a collision has the same effect.
//
// hashStateSet is hash_state_set.
func hashStateSet(states []uint32) uint64 {
	return fxhash.HashU32Slice(states)
}

// computeConflictStatus returns the conflict status of the pair (i, j) and
// of the pair (j, i).
//
// computeConflictStatus is compute_conflict_status. The visited set of
// upstream is an FxHashSet<u64>, which it only looks up, so a Go map holds
// it.
func computeConflictStatus(
	cursor *NfaCursor,
	grammar *LexicalGrammar,
	followingChars []CharacterSet,
	nfaStateToVar []int,
	i, j int,
) (tokenConflictStatus, tokenConflictStatus) {
	visitedStateSets := make(map[uint64]bool)
	stateSetQueue := make([][]uint32, 0, 4)
	stateSetQueue = append(stateSetQueue, []uint32{
		grammar.Variables[i].StartState,
		grammar.Variables[j].StartState,
	})
	var result0, result1 tokenConflictStatus

	for len(stateSetQueue) > 0 {
		stateSet := stateSetQueue[len(stateSetQueue)-1]
		stateSetQueue = stateSetQueue[:len(stateSetQueue)-1]

		// If only one of the two tokens could possibly match from this state,
		// then there is no reason to analyze any of its successors. Just
		// record the fact that the token matches a string that the other
		// token does not match.
		firstLiveVariableIndex := nfaStateToVar[stateSet[0]]
		allSame := true
		for _, s := range stateSet {
			if nfaStateToVar[s] != firstLiveVariableIndex {
				allSame = false
				break
			}
		}
		if allSame {
			if firstLiveVariableIndex == i {
				result0 |= conflictMatchesDifferentString
			} else {
				result1 |= conflictMatchesDifferentString
			}
			continue
		}

		// Don't pursue states where there's no potential for conflict.
		cursor.Reset(stateSet)

		// Compute lazily: most states of the search have no completions, so
		// withinSeparator is never needed in those iterations.
		withinSeparator, hasWithinSeparator := false, false

		// Examine each possible completed token in this state.
		completedID, completedPrecedence, hasCompletion := 0, int32(0), false
		for id, precedence := range cursor.Completions() {
			if !hasWithinSeparator {
				for _, sep := range cursor.TransitionChars() {
					if sep {
						withinSeparator = true
						break
					}
				}
				hasWithinSeparator = true
			}
			if withinSeparator {
				if id == i {
					result0 |= conflictDoesMatchSeparators
				} else {
					result1 |= conflictDoesMatchSeparators
				}
			}

			// If the other token has already completed, then this is a
			// same-string conflict.
			if hasCompletion {
				prevID, prevPrecedence := completedID, completedPrecedence
				if id == prevID {
					continue
				}

				// Determine which of the two tokens is preferred.
				var preferredID int
				if PreferToken(grammar, prevPrecedence, prevID, precedence, id) {
					preferredID = prevID
				} else {
					preferredID = id
					completedID, completedPrecedence = id, precedence
				}

				if preferredID == i {
					result0 |= conflictMatchesSameString
				} else {
					result1 |= conflictMatchesSameString
				}
			} else {
				completedID, completedPrecedence, hasCompletion = id, precedence, true
			}
		}

		// Examine each possible transition from this state to detect
		// substring conflicts.
		for _, transition := range cursor.Transitions() {
			canAdvance := true

			// If there is already a completed token in this state, then
			// determine if the next state can also match the completed token.
			// If so, then this is *not* a conflict.
			if hasCompletion {
				advancedID, hasAdvancedID := 0, false
				successorContainsCompletedID := false
				prevVar, hasPrevVar := 0, false
				for _, stateID := range transition.States {
					varID := nfaStateToVar[stateID]
					if hasPrevVar && prevVar == varID {
						continue
					}
					prevVar, hasPrevVar = varID, true
					if varID == completedID {
						successorContainsCompletedID = true
						break
					}
					advancedID, hasAdvancedID = varID, true
				}

				// Determine which action is preferred: matching the already
				// complete token, or continuing on to try and match the other
				// longer token.
				if hasAdvancedID && !successorContainsCompletedID {
					switch {
					case PreferTransition(grammar, &transition, completedID, completedPrecedence, withinSeparator):
						canAdvance = true
						if advancedID == i {
							result0 |= conflictDoesMatchContinuation
							if transition.Characters.DoesIntersect(followingChars[j]) {
								result0 |= conflictDoesMatchValidCont
							}
						} else {
							result1 |= conflictDoesMatchContinuation
							if transition.Characters.DoesIntersect(followingChars[i]) {
								result1 |= conflictDoesMatchValidCont
							}
						}
					case completedID == i:
						result0 |= conflictMatchesPrefix
					default:
						result1 |= conflictMatchesPrefix
					}
				}
			}

			if canAdvance {
				h := hashStateSet(transition.States)
				if !visitedStateSets[h] {
					visitedStateSets[h] = true
					stateSetQueue = append(stateSetQueue, transition.States)
				}
			}
		}
	}
	return result0, result1
}
