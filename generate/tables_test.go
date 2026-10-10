package generate

import (
	"slices"
	"testing"
)

// TestTerminalRemapSkipsUnreferencedPoolEntries is
// terminal_remap_skips_unreferenced_pool_entries in tables.rs.
//
// `remap_terminal_references` must remap only the pool slots some state references.
// Dead slots (e.g. left by remove_unit_reductions' COW) can hold Shift targets
// outside the remap's domain, so touching them would panic or corrupt. Here, slot 0
// is live, slot 1 is dead, and the closure is defined only for state 0. A regressed
// skip would call `f` on slot 1 and index out of bounds here.
func TestTerminalRemapSkipsUnreferencedPoolEntries(t *testing.T) {
	t.Parallel()
	var table ParseTable
	table.ActionLists.Push(ActionList{{Kind: ParseActionShift, State: 0}})
	table.ActionLists.Push(ActionList{{Kind: ParseActionShift, State: 1}})
	var state ParseState
	state.TerminalEntries.Insert(SymbolEndValue, NewActionListID(0, true))
	table.States = append(table.States, state)

	// Slot 1 is unreferenced, so `f` must never see state 1 (else this indexes out of bounds).
	replacement := []ParseStateID{7}
	table.RemapTerminalReferences(func(state ParseStateID) ParseStateID { return replacement[state] })

	if a := table.ActionLists.Get(NewActionListID(0, false))[0]; a.Kind != ParseActionShift || a.State != 7 {
		t.Errorf("slot 0: got %+v, want a shift to state 7", a)
	}
	if a := table.ActionLists.Get(NewActionListID(1, false))[0]; a.Kind != ParseActionShift || a.State != 1 {
		t.Errorf("slot 1: got %+v, want a shift to state 1", a)
	}
}

// TestIndexMapOrder checks that IndexMap keeps the order of the first insert
// of each key, as the map of the indexmap crate does. It is not an upstream
// test.
func TestIndexMapOrder(t *testing.T) {
	t.Parallel()
	var m IndexMap[Symbol, int]
	m.Insert(TerminalSymbol(3), 1)
	m.Insert(SymbolEndValue, 2)
	*m.GetOrInsert(TerminalSymbol(1), 0) += 3
	if old, ok := m.Insert(TerminalSymbol(3), 4); !ok || old != 1 {
		t.Errorf("Insert of a held key: got %d, %t, want 1, true", old, ok)
	}
	*m.GetOrInsertFunc(SymbolEndValue, func() int { return 100 }) += 10

	var other IndexMap[Symbol, int]
	other.Insert(ExternalSymbol(0), 5)
	other.Insert(TerminalSymbol(1), 6)
	m.Extend(other.All())

	wantKeys := []Symbol{TerminalSymbol(3), SymbolEndValue, TerminalSymbol(1), ExternalSymbol(0)}
	if keys := slices.Collect(m.Keys()); !slices.Equal(keys, wantKeys) {
		t.Errorf("keys: got %v, want %v", keys, wantKeys)
	}
	wantValues := []int{4, 12, 6, 5}
	if values := slices.Collect(m.Values()); !slices.Equal(values, wantValues) {
		t.Errorf("values: got %v, want %v", values, wantValues)
	}
	if i, ok := m.GetIndexOf(TerminalSymbol(1)); !ok || i != 2 {
		t.Errorf("GetIndexOf: got %d, %t, want 2, true", i, ok)
	}
	if k, v, ok := m.GetIndex(3); !ok || k != ExternalSymbol(0) || v != 5 {
		t.Errorf("GetIndex(3): got %v, %d, %t", k, v, ok)
	}
	if _, _, ok := m.GetIndex(4); ok {
		t.Error("GetIndex(4): got true, want false")
	}

	s := m.Clone()
	s.ShrinkToFit()
	if keys := slices.Collect(s.Keys()); !slices.Equal(keys, wantKeys) {
		t.Errorf("keys after ShrinkToFit: got %v, want %v", keys, wantKeys)
	}
	if v, ok := s.Get(SymbolEndValue); !ok || v != 12 {
		t.Errorf("Get after ShrinkToFit: got %d, %t, want 12, true", v, ok)
	}
	var empty IndexMap[Symbol, int]
	empty.ShrinkToFit()
	empty.Insert(TerminalSymbol(0), 1)
	if empty.Len() != 1 {
		t.Errorf("Len after ShrinkToFit of an empty map and an insert: got %d, want 1", empty.Len())
	}

	c := m.Clone()
	c.Insert(TerminalSymbol(9), 9)
	if m.Len() != 4 || c.Len() != 5 || m.ContainsKey(TerminalSymbol(9)) {
		t.Error("Clone shares memory with the original")
	}

	var r IndexMap[Symbol, int]
	r.ReserveExact(m.Len())
	d := m.Clone()
	for k, v := range d.Drain() {
		r.Insert(k, v)
	}
	if d.Len() != 0 || d.ContainsKey(SymbolEndValue) {
		t.Errorf("Len after Drain: got %d, want 0", d.Len())
	}
	if keys := slices.Collect(r.Keys()); !slices.Equal(keys, wantKeys) {
		t.Errorf("keys after Drain: got %v, want %v", keys, wantKeys)
	}
	d.Insert(TerminalSymbol(9), 9)
	if v, ok := d.Get(TerminalSymbol(9)); !ok || v != 9 || d.Len() != 1 {
		t.Errorf("Get after Drain and an insert: got %d, %t, want 9, true", v, ok)
	}
}

// TestCanonicalizeKeepsEmptyListFirst checks that Canonicalize gives the
// empty list index 0 and stores each list once, in the order of the states
// and their entries. It is not an upstream test.
func TestCanonicalizeKeepsEmptyListFirst(t *testing.T) {
	t.Parallel()
	shift := ParseAction{Kind: ParseActionShift, State: 2}
	reduce := ParseAction{Kind: ParseActionReduce, Symbol: NonTerminalSymbol(1), ChildCount: 1}
	var interned ParseTable
	interned.States = make([]ParseState, 2)
	ids := make(map[string]uint32)
	intern := func(state int, symbol Symbol, list ActionList, reusable bool) {
		index := interned.ActionLists.Intern(ids, list)
		interned.States[state].TerminalEntries.Insert(symbol, NewActionListID(index, reusable))
	}
	intern(0, TerminalSymbol(0), ActionList{shift}, true)
	intern(0, TerminalSymbol(1), ActionList{reduce}, false)
	intern(1, TerminalSymbol(2), ActionList{shift}, true)
	if n := interned.ActionLists.Len(); n != 2 {
		t.Fatalf("Intern: got %d lists, want 2", n)
	}
	interned.ActionLists.Canonicalize(interned.States)
	if n := interned.ActionLists.Len(); n != 3 {
		t.Fatalf("Canonicalize: got %d lists, want 3", n)
	}
	if l := interned.ActionLists.Get(NewActionListID(0, false)); len(l) != 0 {
		t.Errorf("list 0: got %v, want the empty list", l)
	}
	want := []ActionListID{NewActionListID(1, true), NewActionListID(2, false), NewActionListID(1, true)}
	var got []ActionListID
	for i := range interned.States {
		got = slices.AppendSeq(got, interned.States[i].TerminalEntries.Values())
	}
	if !slices.Equal(got, want) {
		t.Errorf("ids: got %v, want %v", got, want)
	}
}
