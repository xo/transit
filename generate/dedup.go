package generate

import (
	"cmp"
	"slices"
)

// This file ports crates/generate/src/dedup.rs: the step that splits groups
// of states, which the minimization of the parse table and of the lex table
// share.

// SplitCriterion decides which states of a group SplitStateIDGroups
// separates.
//
// Besides ShouldSplit, a criterion can sort a group's states into classes of
// equivalent states. These are states that never need to be split from each
// other, and that need to be split from exactly the same states. Only the
// first state of each class is then compared, and the rest of the class
// follows it.
//
// SplitCriterion is the trait SplitCriterion. SplitFunc gives the default
// methods of the trait.
type SplitCriterion[S any] interface {
	// ShouldSplit reports whether left and right, from the same group, must
	// be in different groups. This must be symmetric.
	//
	// ShouldSplit is SplitCriterion::should_split.
	ShouldSplit(left, right *S, groupIDsByStateID []uint32) bool
	// Signature returns a hash of what Equivalent compares, and false if
	// this criterion does not sort states into classes.
	//
	// Signature is SplitCriterion::signature.
	Signature(state *S, groupIDsByStateID []uint32) (uint64, bool)
	// Equivalent reports whether two states with the same signature are
	// equivalent.
	//
	// Equivalent is SplitCriterion::equivalent.
	Equivalent(left, right *S, groupIDsByStateID []uint32) bool
	// StartGroup is called before a group's states are compared, e.g. to
	// reset what CompatibleWithAll keeps.
	//
	// StartGroup is SplitCriterion::start_group.
	StartGroup()
	// CompatibleWithAll reports whether state can stay in its group with all
	// of kept.
	//
	// kept holds the first state of each class that stayed before state, in
	// order. false is always safe: the states are then compared one at a
	// time.
	//
	// CompatibleWithAll is SplitCriterion::compatible_with_all.
	CompatibleWithAll(state *S, kept []uint32, groupIDsByStateID []uint32) bool
}

// SplitFunc lets a function that decides SplitCriterion.ShouldSplit, like
// the lexer's, be a criterion. It has no classes, so every state is
// compared.
//
// SplitFunc is the impl of SplitCriterion for FnMut(&S, &S, &[u32]) -> bool,
// with the default methods of the trait.
type SplitFunc[S any] func(left, right *S, groupIDsByStateID []uint32) bool

// ShouldSplit calls f.
func (f SplitFunc[S]) ShouldSplit(left, right *S, groupIDsByStateID []uint32) bool {
	return f(left, right, groupIDsByStateID)
}

// Signature returns false, because f has no classes.
func (f SplitFunc[S]) Signature(*S, []uint32) (uint64, bool) {
	return 0, false
}

// Equivalent returns false.
func (f SplitFunc[S]) Equivalent(_, _ *S, _ []uint32) bool {
	return false
}

// StartGroup does nothing.
func (f SplitFunc[S]) StartGroup() {}

// CompatibleWithAll returns false.
func (f SplitFunc[S]) CompatibleWithAll(*S, []uint32, []uint32) bool {
	return false
}

// splitMovedState is a state that does not stay in its group, with the
// position in kept of the class it was split from.
//
// splitMovedState is (u32, u32).
type splitMovedState struct {
	splitter int
	stateID  uint32
}

// SplitStateIDGroups splits every group from startGroupID on, including the
// groups split off from them, so that no group has two states that criterion
// separates. It reports whether any group split.
//
// A group's states are scanned in order: each state stays unless it must be
// split from a state that stayed before it, and the states that don't stay
// form a new group, ordered by the state they were split from, then by
// position.
//
// SplitStateIDGroups is split_state_id_groups. Upstream holds each element
// of split_from as an Option<u32>, and the Go form holds the position, or -1
// for None.
func SplitStateIDGroups[S any](
	states []S,
	stateIDsByGroupID *[][]uint32,
	groupIDsByStateID []uint32,
	startGroupID uint32,
	criterion SplitCriterion[S],
) bool {
	result := false
	var classes groupClasses
	// The first state of each class that stays, in order.
	var kept []uint32
	// For each class, the position in `kept` of the class it was split from, if any.
	var splitFrom []int
	// The states that don't stay, with the position in `kept` of the class they were split
	// from.
	var moved []splitMovedState

	groupID := int(startGroupID)
	for groupID < len(*stateIDsByGroupID) {
		stateIDs := (*stateIDsByGroupID)[groupID]
		// A single state has nothing to be split from.
		if len(stateIDs) < 2 {
			groupID++
			continue
		}
		groupClassesClassify(&classes, states, stateIDs, groupIDsByStateID, criterion)

		// A class follows its first state, so compare only those.
		criterion.StartGroup()
		kept = kept[:0]
		splitFrom = splitFrom[:0]
		for _, stateID := range classes.firstStates {
			state := &states[stateID]
			// Most states that are split off are split from the first kept state. Past it, one
			// check against all kept states can save comparing the state with each.
			splitter := -1
			if len(kept) > 0 {
				first, rest := kept[0], kept[1:]
				switch {
				case criterion.ShouldSplit(&states[first], state, groupIDsByStateID):
					splitter = 0
				case len(rest) == 0 || criterion.CompatibleWithAll(state, kept, groupIDsByStateID):
				default:
					if position := slices.IndexFunc(rest, func(keptID uint32) bool {
						return criterion.ShouldSplit(&states[keptID], state, groupIDsByStateID)
					}); position >= 0 {
						splitter = position + 1
					}
				}
			}
			if splitter < 0 {
				kept = append(kept, stateID)
			}
			splitFrom = append(splitFrom, splitter)
		}

		moved = moved[:0]
		for i, stateID := range stateIDs {
			if splitter := splitFrom[classes.classIDs[i]]; splitter >= 0 {
				moved = append(moved, splitMovedState{splitter: splitter, stateID: stateID})
			}
		}
		if len(moved) > 0 {
			result = true
			// The sort is stable, so states split from the same state stay in position order.
			slices.SortStableFunc(moved, func(a, b splitMovedState) int {
				return cmp.Compare(a.splitter, b.splitter)
			})
			// The loop visits the states in order, as `retain` does.
			retained := stateIDs[:0]
			for i, stateID := range stateIDs {
				if splitFrom[classes.classIDs[i]] < 0 {
					retained = append(retained, stateID)
				}
			}
			(*stateIDsByGroupID)[groupID] = retained
			newGroupID := uint32(len(*stateIDsByGroupID))
			newGroup := make([]uint32, len(moved))
			for i, m := range moved {
				groupIDsByStateID[m.stateID] = newGroupID
				newGroup[i] = m.stateID
			}
			*stateIDsByGroupID = append(*stateIDsByGroupID, newGroup)
		}

		groupID++
	}

	return result
}

// groupClasses holds the states of a group, sorted into classes of
// equivalent states by a SplitCriterion.
//
// groupClasses is GroupClasses. The zero value is GroupClasses::default.
// Upstream keeps classIDsBySignature in an FxHashMap, and only looks it up.
type groupClasses struct {
	// classIDs holds the class of each of the group's states, by position.
	classIDs []uint32
	// firstStates holds the first state of each class, in order.
	firstStates []uint32
	// classIDsBySignature holds the first class found with each signature. A
	// state with the same signature that isn't equivalent to that class's
	// first state gets a class of its own.
	classIDsBySignature map[uint64]uint32
}

// groupClassesClassify sorts the states of a group, stateIDs, into classes.
//
// groupClassesClassify is GroupClasses::classify. A Go method cannot have a
// type parameter, so it is a function.
func groupClassesClassify[S any](
	c *groupClasses,
	states []S,
	stateIDs []uint32,
	groupIDsByStateID []uint32,
	criterion SplitCriterion[S],
) {
	c.classIDs = c.classIDs[:0]
	c.firstStates = c.firstStates[:0]
	if c.classIDsBySignature == nil {
		c.classIDsBySignature = make(map[uint64]uint32)
	}
	clear(c.classIDsBySignature)
	// With fewer than three states, classes can't save a comparison. Without a signature,
	// the criterion has no classes.
	var firstSignature uint64
	hasFirstSignature := false
	if len(stateIDs) >= 3 {
		firstSignature, hasFirstSignature = criterion.Signature(&states[stateIDs[0]], groupIDsByStateID)
	}
	if !hasFirstSignature {
		c.firstStates = append(c.firstStates, stateIDs...)
		for i := range stateIDs {
			c.classIDs = append(c.classIDs, uint32(i))
		}
		return
	}
	c.classIDsBySignature[firstSignature] = 0
	c.firstStates = append(c.firstStates, stateIDs[0])
	c.classIDs = append(c.classIDs, 0)
	for _, stateID := range stateIDs[1:] {
		state := &states[stateID]
		newClassID := uint32(len(c.firstStates))
		classID := newClassID
		if signature, ok := criterion.Signature(state, groupIDsByStateID); ok {
			if id, occupied := c.classIDsBySignature[signature]; occupied {
				if criterion.Equivalent(&states[c.firstStates[id]], state, groupIDsByStateID) {
					classID = id
				}
			} else {
				c.classIDsBySignature[signature] = newClassID
			}
		}
		if classID == newClassID {
			c.firstStates = append(c.firstStates, stateID)
		}
		c.classIDs = append(c.classIDs, classID)
	}
}
