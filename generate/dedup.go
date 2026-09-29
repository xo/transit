package generate

import "slices"

// This file ports crates/generate/src/dedup.rs: the step that splits groups
// of states, which the minimization of the parse table and of the lex table
// share.

// SplitStateIDGroups splits each group of states, from startGroupID on, into
// the states that are compatible with the first states of the group and the
// states that are not. It moves the states that are not compatible to a new
// group at the end, which the loop then reaches too. shouldSplit gets two
// states of a group and the group of each state, and reports whether the two
// states are not compatible. SplitStateIDGroups reports whether it split a
// group.
//
// SplitStateIDGroups is split_state_id_groups.
func SplitStateIDGroups[S any](
	states []S,
	stateIDsByGroupID *[][]uint32,
	groupIDsByStateID []uint32,
	startGroupID uint32,
	shouldSplit func(left, right *S, groupIDsByStateID []uint32) bool,
) bool {
	result := false

	// Use a flat bitset instead of Vec::contains for O(1) membership tests.
	isSplit := make([]bool, len(states))

	groupID := startGroupID
	for int(groupID) < len(*stateIDsByGroupID) {
		stateIDs := (*stateIDsByGroupID)[groupID]
		var splitStateIDs []uint32

		for i, leftStateID := range stateIDs {
			if isSplit[leftStateID] {
				continue
			}

			leftState := &states[leftStateID]

			// Identify all of the other states in the group that are incompatible with
			// this state.
			for j := i + 1; j < len(stateIDs); j++ {
				rightStateID := stateIDs[j]
				if isSplit[rightStateID] {
					continue
				}
				rightState := &states[rightStateID]

				if shouldSplit(leftState, rightState, groupIDsByStateID) {
					splitStateIDs = append(splitStateIDs, rightStateID)
					isSplit[rightStateID] = true
				}
			}
		}

		// If any states were removed from the group, add them all as a new group.
		if len(splitStateIDs) > 0 {
			result = true
			(*stateIDsByGroupID)[groupID] = slices.DeleteFunc(stateIDs, func(id uint32) bool { return isSplit[id] })

			newGroupID := uint32(len(*stateIDsByGroupID))
			for _, id := range splitStateIDs {
				groupIDsByStateID[id] = newGroupID
				isSplit[id] = false
			}

			*stateIDsByGroupID = append(*stateIDsByGroupID, splitStateIDs)
		}

		groupID++
	}

	return result
}
