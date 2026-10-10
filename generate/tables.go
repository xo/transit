package generate

import (
	"cmp"
	"encoding/binary"
	"iter"
	"maps"
	"math"
	"slices"
)

// This file ports crates/generate/src/tables.rs: the parse table and the lex
// tables that the generator builds and renders. It also holds IndexMap, the
// Go form of the map of the indexmap crate that the parse states use.

// ProductionInfoID is the index of a ProductionInfo in a parse table.
//
// ProductionInfoID is ProductionInfoId.
type ProductionInfoID = uint32

// ParseStateID is the index of a state in a parse table.
//
// ParseStateID is ParseStateId.
type ParseStateID = uint32

// LexStateID is the index of a state in a lex table.
//
// LexStateID is LexStateId.
type LexStateID = uint32

// ParseActionKind is the kind of a parse action.
type ParseActionKind uint8

// The kinds of parse action, in the order of the enum ParseAction.
const (
	ParseActionAccept ParseActionKind = iota
	ParseActionShift
	ParseActionShiftExtra
	ParseActionRecover
	ParseActionReduce
)

// ParseAction is an action of the parser for a lookahead token.
//
// ParseAction is ParseAction, an enum with data upstream. The fields that a
// kind uses are:
//
//   - ParseActionShift: State and IsRepetition.
//   - ParseActionReduce: Symbol, ChildCount, DynamicPrecedence and
//     ProductionID.
//
// A field that the kind does not use is zero, so two actions compare with ==.
type ParseAction struct {
	Kind              ParseActionKind
	State             ParseStateID
	IsRepetition      bool
	Symbol            Symbol
	ChildCount        uint16
	DynamicPrecedence int32
	ProductionID      uint16
}

// GotoActionKind is the kind of a goto action.
type GotoActionKind uint8

// The kinds of goto action, in the order of the enum GotoAction.
const (
	GotoActionGoto GotoActionKind = iota
	GotoActionShiftExtra
)

// GotoAction is the action of the parser for a non-terminal: go to a state,
// or shift an extra.
//
// GotoAction is GotoAction, an enum with data upstream. Only GotoActionGoto
// uses State, and State is zero for GotoActionShiftExtra, so two actions
// compare with ==.
type GotoAction struct {
	Kind  GotoActionKind
	State ParseStateID
}

// ActionList is the list of actions of one entry of the parse table.
//
// ActionList is ActionList. Upstream holds a list of one action inline, and
// its slice is the same as the Go slice. Two lists are equal when their
// actions are equal, so they compare with slices.Equal. A copy of an
// ActionList shares its memory, so a list that is copied and then changed
// must be copied with Clone.
type ActionList []ParseAction

// Push adds an action at the end of the list.
//
// Push is ActionList::push.
func (l *ActionList) Push(action ParseAction) {
	*l = append(*l, action)
}

// Pop removes the last action and returns it, and false when the list is
// empty.
//
// Pop is ActionList::pop.
func (l *ActionList) Pop() (ParseAction, bool) {
	n := len(*l)
	if n == 0 {
		return ParseAction{}, false
	}
	action := (*l)[n-1]
	*l = (*l)[:n-1]
	return action, true
}

// Clear removes every action.
//
// Clear is ActionList::clear.
func (l *ActionList) Clear() {
	*l = nil
}

// KeepLast removes every action but the last one.
//
// KeepLast is ActionList::keep_last.
func (l *ActionList) KeepLast() {
	if n := len(*l); n > 1 {
		*l = ActionList{(*l)[n-1]}
	}
}

// ActionListFromSlice returns a list that holds a copy of the actions.
//
// ActionListFromSlice is ActionList::from_slice.
func ActionListFromSlice(actions []ParseAction) ActionList {
	if len(actions) == 0 {
		return nil
	}
	return slices.Clone(ActionList(actions))
}

// Clone returns a copy of the list.
//
// Clone is the Clone of ActionList.
func (l ActionList) Clone() ActionList {
	return ActionListFromSlice(l)
}

// actionListKey returns a string that is the same for two equal lists, for a
// map key. It stands in for the Hash of ActionList. Every field of each action
// goes in the key, and the fields that a kind does not use are zero.
func actionListKey(list []ParseAction) string {
	buf := make([]byte, 0, 24*len(list))
	for _, a := range list {
		buf = append(buf, byte(a.Kind), byte(boolWord(a.IsRepetition)))
		buf = binary.LittleEndian.AppendUint32(buf, a.State)
		buf = binary.LittleEndian.AppendUint64(buf, a.Symbol.packedKey())
		buf = binary.LittleEndian.AppendUint16(buf, a.ChildCount)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(a.DynamicPrecedence))
		buf = binary.LittleEndian.AppendUint16(buf, a.ProductionID)
	}
	return string(buf)
}

// ActionListID is the index of an action list in an ActionListPool. Its high
// bit tells whether the list is reusable.
//
// ActionListID is ActionListId.
type ActionListID uint32

// actionListReusableBit is the bit of an ActionListID that tells whether the
// list is reusable. ActionListPool holds one entry for each list that is not
// the same as another. In practice, there are only a few thousand of these,
// and render limits the number of tables to math.MaxUint16. These ids never
// come close to 2^31, so the top bit is free to tell whether a list is
// reusable.
//
// actionListReusableBit is ActionListId::REUSABLE_BIT.
const actionListReusableBit uint32 = 1 << 31

// NewActionListID returns the id of the list at an index.
//
// NewActionListID is ActionListId::new.
func NewActionListID(index uint32, reusable bool) ActionListID {
	if index >= actionListReusableBit {
		panic("generate: an ActionListID index uses the reusable bit")
	}
	if reusable {
		return ActionListID(index | actionListReusableBit)
	}
	return ActionListID(index)
}

// Index returns the index of the list in its pool.
//
// Index is ActionListId::index.
func (id ActionListID) Index() int {
	return int(uint32(id) &^ actionListReusableBit)
}

// Reusable reports whether the list is reusable.
//
// Reusable is ActionListId::reusable.
func (id ActionListID) Reusable() bool {
	return uint32(id)&actionListReusableBit != 0
}

// SetReusable sets whether the list is reusable.
//
// SetReusable is ActionListId::set_reusable.
func (id *ActionListID) SetReusable(reusable bool) {
	v := uint32(*id) &^ actionListReusableBit
	if reusable {
		v |= actionListReusableBit
	}
	*id = ActionListID(v)
}

// actionListRange is the range of the actions of one list in an
// ActionListPool.
//
// actionListRange is ActionListRange.
type actionListRange struct {
	offset uint32
	len    uint32
}

// ActionListPool holds each action list of the terminal entries of the parse
// table once.
//
// Every entry of a state and a terminal has an action list, but in a grammar
// 98 to 99 percent of these lists are the same as another list. Each entry
// holds an ActionListID. The id is the index of a range, and the range is the
// slice of the actions of one list.
//
// The builder of the parse table interns the lists of each state into this
// pool as it completes the state, and every list becomes an id. Then minimize
// changes the targets of the shift actions in the pool. Canonicalize builds
// the pool again before render, with each list stored once. The list at
// index 0 of the pool is always the empty list.
//
// ActionListPool is ActionListPool. The zero ActionListPool is empty and
// ready to use.
type ActionListPool struct {
	actions      []ParseAction
	ranges       []actionListRange
	remapScratch []bool
}

// Push adds a list to the pool and returns its index.
//
// Push is ActionListPool::push.
func (p *ActionListPool) Push(list []ParseAction) uint32 {
	index := uint32(len(p.ranges))
	offset := uint32(len(p.actions))
	p.actions = append(p.actions, list...)
	p.ranges = append(p.ranges, actionListRange{offset: offset, len: uint32(len(list))})
	return index
}

// Get returns the actions of a list.
//
// Get is ActionListPool::get.
func (p *ActionListPool) Get(id ActionListID) []ParseAction {
	r := p.ranges[id.Index()]
	return p.actions[r.offset : r.offset+r.len]
}

// Len returns the number of lists in the pool.
//
// Len is ActionListPool::len.
func (p *ActionListPool) Len() int {
	return len(p.ranges)
}

// Intern returns the index of a list. When dedup does not hold the list yet,
// Intern adds the list to the pool and to dedup. Make dedup with
// make(map[string]uint32).
//
// Intern is ActionListPool::intern. Upstream keys dedup with the list, and
// the Go key is the string of actionListKey. Upstream only looks up dedup.
func (p *ActionListPool) Intern(dedup map[string]uint32, list ActionList) uint32 {
	key := actionListKey(list)
	if index, ok := dedup[key]; ok {
		return index
	}
	index := p.Push(list)
	dedup[key] = index
	return index
}

// Canonicalize builds the pool again, with only the lists that the states
// use, each stored once, and changes the ids of the states to the new
// indices. The empty list gets index 0.
//
// Canonicalize is ActionListPool::canonicalize.
func (p *ActionListPool) Canonicalize(states []ParseState) {
	oldActions, oldRanges := p.actions, p.ranges
	p.actions, p.ranges = nil, nil
	ids := make(map[string]uint32)
	// the "no action" default of render
	p.Intern(ids, nil)
	oldToNew := make([]uint32, len(oldRanges))
	for i := range oldToNew {
		oldToNew[i] = math.MaxUint32
	}
	for i := range states {
		for id := range states[i].TerminalEntries.ValuesMut() {
			oldIndex := id.Index()
			newIndex := oldToNew[oldIndex]
			// INVARIANT: [`ParseTableEntry::index`] can never return `u32::MAX`
			if newIndex == math.MaxUint32 {
				r := oldRanges[oldIndex]
				list := ActionListFromSlice(oldActions[r.offset : r.offset+r.len])
				newIndex = p.Intern(ids, list)
				oldToNew[oldIndex] = newIndex
			}
			*id = NewActionListID(newIndex, id.Reusable())
		}
	}
}

// ParseTableEntry is the entry of the parse table for a state and a terminal,
// while the table is built.
//
// ParseTableEntry is ParseTableEntry.
type ParseTableEntry struct {
	Actions  ActionList
	Reusable bool
}

// TerminalEntries holds the terminal entries of a parse state: the action
// list for each lookahead, in the order in which the entries were added. The
// zero TerminalEntries is empty and ready to use.
//
// TerminalEntries is TerminalEntries, a Vec<(Symbol, ActionListId)>.
type TerminalEntries struct {
	entries []terminalEntry
}

// terminalEntry is a lookahead and its action list.
//
// terminalEntry is (Symbol, ActionListId).
type terminalEntry struct {
	symbol Symbol
	id     ActionListID
}

// Len returns the number of entries.
//
// Len is TerminalEntries::len.
func (t *TerminalEntries) Len() int {
	return len(t.entries)
}

// All returns each symbol and its action list, in order.
//
// All is TerminalEntries::iter, and the IntoIterator of TerminalEntries.
func (t *TerminalEntries) All() iter.Seq2[Symbol, ActionListID] {
	return func(yield func(Symbol, ActionListID) bool) {
		for _, e := range t.entries {
			if !yield(e.symbol, e.id) {
				return
			}
		}
	}
}

// AllMut returns each symbol and a pointer to its action list, in order.
//
// AllMut is TerminalEntries::iter_mut.
func (t *TerminalEntries) AllMut() iter.Seq2[Symbol, *ActionListID] {
	return func(yield func(Symbol, *ActionListID) bool) {
		for i := range t.entries {
			if !yield(t.entries[i].symbol, &t.entries[i].id) {
				return
			}
		}
	}
}

// Keys returns each symbol, in order.
//
// Keys is TerminalEntries::keys.
func (t *TerminalEntries) Keys() iter.Seq[Symbol] {
	return func(yield func(Symbol) bool) {
		for _, e := range t.entries {
			if !yield(e.symbol) {
				return
			}
		}
	}
}

// Values returns each action list, in order.
//
// Values is TerminalEntries::values.
func (t *TerminalEntries) Values() iter.Seq[ActionListID] {
	return func(yield func(ActionListID) bool) {
		for _, e := range t.entries {
			if !yield(e.id) {
				return
			}
		}
	}
}

// ValuesMut returns a pointer to each action list, in order.
//
// ValuesMut is TerminalEntries::values_mut.
func (t *TerminalEntries) ValuesMut() iter.Seq[*ActionListID] {
	return func(yield func(*ActionListID) bool) {
		for i := range t.entries {
			if !yield(&t.entries[i].id) {
				return
			}
		}
	}
}

// GetIndex returns the symbol and the action list at an index, and false
// when the index is not before Len.
//
// GetIndex is TerminalEntries::get_index.
func (t *TerminalEntries) GetIndex(i int) (Symbol, ActionListID, bool) {
	if i < 0 || i >= len(t.entries) {
		return Symbol{}, 0, false
	}
	return t.entries[i].symbol, t.entries[i].id, true
}

// GetIndexMut returns the symbol and a pointer to the action list at an
// index, and false when the index is not before Len.
//
// GetIndexMut is TerminalEntries::get_index_mut.
func (t *TerminalEntries) GetIndexMut(i int) (Symbol, *ActionListID, bool) {
	if i < 0 || i >= len(t.entries) {
		return Symbol{}, nil, false
	}
	return t.entries[i].symbol, &t.entries[i].id, true
}

// ContainsKey reports whether there is an entry for symbol.
//
// ContainsKey is TerminalEntries::contains_key.
func (t *TerminalEntries) ContainsKey(symbol Symbol) bool {
	for _, e := range t.entries {
		if e.symbol == symbol {
			return true
		}
	}
	return false
}

// Insert sets the action list for symbol. An entry that exists keeps its
// place.
//
// Insert is TerminalEntries::insert.
func (t *TerminalEntries) Insert(symbol Symbol, id ActionListID) {
	for i := range t.entries {
		if t.entries[i].symbol == symbol {
			t.entries[i].id = id
			return
		}
	}
	t.entries = append(t.entries, terminalEntry{symbol: symbol, id: id})
}

// InsertIfMissing adds an entry for symbol, unless there is one.
//
// InsertIfMissing is TerminalEntries::insert_if_missing.
func (t *TerminalEntries) InsertIfMissing(symbol Symbol, id ActionListID) {
	if !t.ContainsKey(symbol) {
		t.entries = append(t.entries, terminalEntry{symbol: symbol, id: id})
	}
}

// Push adds an entry for symbol, which must not have one yet.
//
// Push is TerminalEntries::push. Upstream checks the symbol only with
// debug_assert.
func (t *TerminalEntries) Push(symbol Symbol, id ActionListID) {
	t.entries = append(t.entries, terminalEntry{symbol: symbol, id: id})
}

// ReserveExact makes room for additional more entries, with no spare
// capacity.
//
// ReserveExact is TerminalEntries::reserve_exact.
func (t *TerminalEntries) ReserveExact(additional int) {
	if n := len(t.entries) + additional; cap(t.entries) < n {
		entries := make([]terminalEntry, len(t.entries), n)
		copy(entries, t.entries)
		t.entries = entries
	}
}

// ParseState is a state of the parse table.
//
// ParseState is ParseState. The zero ParseState is ParseState::default. A
// ParseState holds IndexMaps, so it must not be copied while it is in use
// (see IndexMap).
type ParseState struct {
	ID                 ParseStateID
	TerminalEntries    TerminalEntries
	NonterminalEntries IndexMap[Symbol, GotoAction]
	ReservedWords      TokenSet
	LexStateID         LexStateID
	ExternalLexStateID LexStateID
	CoreID             uint32
	HasEOFGatedReduce  bool
}

// FieldLocation is where a field is in a production: the index of the child,
// and whether the child only passes on the field of a hidden child.
//
// FieldLocation is FieldLocation.
type FieldLocation struct {
	Index     uint32
	Inherited bool
}

// FieldMap maps the name of a field to its locations.
//
// FieldMap is BTreeMap<StrId, Vec<FieldLocation>>, which iterates in the
// order of the names. Sorted gives that order.
type FieldMap map[StrID][]FieldLocation

// Sorted returns each name and its locations, in the order of the names.
func (m FieldMap) Sorted() iter.Seq2[StrID, []FieldLocation] {
	return func(yield func(StrID, []FieldLocation) bool) {
		for _, name := range slices.Sorted(maps.Keys(m)) {
			if !yield(name, m[name]) {
				return
			}
		}
	}
}

// ProductionInfo is the aliases and the fields of a production. A zero Alias
// in AliasSequence stands for the None of upstream, because no alias has the
// zero StrID.
//
// ProductionInfo is ProductionInfo. The zero ProductionInfo is
// ProductionInfo::default.
type ProductionInfo struct {
	AliasSequence []Alias
	FieldMap      FieldMap
}

// Equal reports whether two infos hold the same aliases and fields.
//
// Equal is the PartialEq of ProductionInfo.
func (p *ProductionInfo) Equal(other *ProductionInfo) bool {
	return slices.Equal(p.AliasSequence, other.AliasSequence) &&
		maps.EqualFunc(p.FieldMap, other.FieldMap, slices.Equal)
}

// ParseTable is the parse table.
//
// ParseTable is ParseTable. The zero ParseTable is ParseTable::default.
type ParseTable struct {
	States                     []ParseState
	ActionLists                ActionListPool
	Symbols                    []Symbol
	ProductionInfos            []ProductionInfo
	MaxAliasedProductionLength int
	ExternalLexStates          []TokenSet
}

// AdvanceAction is the action of the lexer that moves to a state.
//
// AdvanceAction is AdvanceAction.
type AdvanceAction struct {
	State       LexStateID
	InMainToken bool
}

// CompareAdvanceAction orders two actions by state, then by InMainToken.
//
// CompareAdvanceAction is the Ord of AdvanceAction.
func CompareAdvanceAction(a, b AdvanceAction) int {
	return cmp.Or(cmp.Compare(a.State, b.State), compareBool(a.InMainToken, b.InMainToken))
}

// LexAdvance is a set of characters and the action of the lexer on them.
//
// LexAdvance is (CharacterSet, AdvanceAction).
type LexAdvance struct {
	Chars  CharacterSet
	Action AdvanceAction
}

// LexState is a state of a lex table. HasAcceptAction and HasEOFAction are
// false for the None of upstream.
//
// LexState is LexState. The zero LexState is LexState::default.
type LexState struct {
	AcceptAction    Symbol
	HasAcceptAction bool
	EOFAction       AdvanceAction
	HasEOFAction    bool
	AdvanceActions  []LexAdvance
}

// Equal reports whether two states are the same.
//
// Equal is the PartialEq of LexState.
func (s *LexState) Equal(other *LexState) bool {
	return CompareLexState(s, other) == 0
}

// CompareLexState orders two states by the accept action, then by the action
// at the end of the input, then by the advance actions. None comes before a
// value.
//
// CompareLexState is the Ord of LexState.
func CompareLexState(a, b *LexState) int {
	if c := compareBool(a.HasAcceptAction, b.HasAcceptAction); c != 0 {
		return c
	}
	if a.HasAcceptAction {
		if c := CompareSymbol(a.AcceptAction, b.AcceptAction); c != 0 {
			return c
		}
	}
	if c := compareBool(a.HasEOFAction, b.HasEOFAction); c != 0 {
		return c
	}
	if a.HasEOFAction {
		if c := CompareAdvanceAction(a.EOFAction, b.EOFAction); c != 0 {
			return c
		}
	}
	return slices.CompareFunc(a.AdvanceActions, b.AdvanceActions, func(x, y LexAdvance) int {
		return cmp.Or(x.Chars.Compare(y.Chars), CompareAdvanceAction(x.Action, y.Action))
	})
}

// LexTable is a lex table.
//
// LexTable is LexTable. The zero LexTable is LexTable::default.
type LexTable struct {
	States []LexState
}

// NewParseTableEntry returns an entry with no actions that is reusable.
//
// NewParseTableEntry is ParseTableEntry::new.
func NewParseTableEntry() ParseTableEntry {
	return ParseTableEntry{Reusable: true}
}

// Clone returns a copy of the entry.
//
// Clone is the Clone of ParseTableEntry.
func (e ParseTableEntry) Clone() ParseTableEntry {
	return ParseTableEntry{Actions: e.Actions.Clone(), Reusable: e.Reusable}
}

// IsEndOfNonTerminalExtra reports whether the state ends a non-terminal
// extra.
//
// IsEndOfNonTerminalExtra is ParseState::is_end_of_non_terminal_extra.
func (s *ParseState) IsEndOfNonTerminalExtra() bool {
	return s.TerminalEntries.ContainsKey(SymbolEndOfNonTerminalExtraValue)
}

// ReferencedStates returns each state that s refers to: the target of each
// shift action of the terminal entries, in the order of the entries, then the
// target of each goto action of the non-terminal entries.
//
// ReferencedStates is ParseState::referenced_states.
func (s *ParseState) ReferencedStates(pool *ActionListPool) iter.Seq[ParseStateID] {
	return func(yield func(ParseStateID) bool) {
		for id := range s.TerminalEntries.Values() {
			for _, action := range pool.Get(id) {
				if action.Kind == ParseActionShift && !yield(action.State) {
					return
				}
			}
		}
		for action := range s.NonterminalEntries.Values() {
			if action.Kind == GotoActionGoto && !yield(action.State) {
				return
			}
		}
	}
}

// UpdateNonterminalReferences changes the target of each goto action of the
// non-terminal entries to what f returns for it. f gets the target and the
// state.
//
// UpdateNonterminalReferences is ParseState::update_nonterminal_references.
func (s *ParseState) UpdateNonterminalReferences(f func(ParseStateID, *ParseState) ParseStateID) {
	type update struct {
		symbol   Symbol
		newState ParseStateID
	}
	var updates []update
	for symbol, action := range s.NonterminalEntries.All() {
		if action.Kind == GotoActionGoto {
			result := f(action.State, s)
			if result != action.State {
				updates = append(updates, update{symbol: symbol, newState: result})
			}
		}
	}
	for _, u := range updates {
		s.NonterminalEntries.Insert(u.symbol, GotoAction{Kind: GotoActionGoto, State: u.newState})
	}
}

// RemapTerminalReferences changes the target of each shift action to what f
// returns for it. It changes only the lists of the pool that a state uses.
//
// RemapTerminalReferences is ParseTable::remap_terminal_references.
func (t *ParseTable) RemapTerminalReferences(f func(ParseStateID) ParseStateID) {
	p := &t.ActionLists
	if n := p.Len(); len(p.remapScratch) < n {
		p.remapScratch = append(p.remapScratch, make([]bool, n-len(p.remapScratch))...)
	} else {
		p.remapScratch = p.remapScratch[:n]
	}
	clear(p.remapScratch)
	for i := range t.States {
		for id := range t.States[i].TerminalEntries.Values() {
			p.remapScratch[id.Index()] = true
		}
	}
	for index, r := range p.ranges {
		if !p.remapScratch[index] {
			continue
		}
		for i := r.offset; i < r.offset+r.len; i++ {
			if action := &p.actions[i]; action.Kind == ParseActionShift {
				action.State = f(action.State)
			}
		}
	}
}

// IndexMap is a map that keeps its keys in the order in which they were first
// inserted. The zero IndexMap is empty and ready to use.
//
// An IndexMap must not be copied while it is in use, because a copy shares
// part of its memory with the original. Copy it with Clone. A pointer that a
// method returns into the map is valid only until the next insert of a new
// key.
//
// IndexMap is IndexMap of the indexmap crate. Upstream hashes its keys with
// FxHasher, which decides only how a key is found, not the order.
type IndexMap[K comparable, V any] struct {
	keys    []K
	values  []V
	indices map[K]int
}

// Len returns the number of entries.
//
// Len is IndexMap::len.
func (m *IndexMap[K, V]) Len() int {
	return len(m.keys)
}

// IsEmpty reports whether the map has no entries.
//
// IsEmpty is IndexMap::is_empty.
func (m *IndexMap[K, V]) IsEmpty() bool {
	return len(m.keys) == 0
}

// Get returns the value of a key, and false when the map does not hold the
// key.
//
// Get is IndexMap::get.
func (m *IndexMap[K, V]) Get(key K) (V, bool) {
	if i, ok := m.indices[key]; ok {
		return m.values[i], true
	}
	var zero V
	return zero, false
}

// GetMut returns a pointer to the value of a key, and nil when the map does
// not hold the key.
//
// GetMut is IndexMap::get_mut.
func (m *IndexMap[K, V]) GetMut(key K) *V {
	if i, ok := m.indices[key]; ok {
		return &m.values[i]
	}
	return nil
}

// ContainsKey reports whether the map holds a key.
//
// ContainsKey is IndexMap::contains_key.
func (m *IndexMap[K, V]) ContainsKey(key K) bool {
	_, ok := m.indices[key]
	return ok
}

// GetIndexOf returns the index of a key, and false when the map does not hold
// the key.
//
// GetIndexOf is IndexMap::get_index_of.
func (m *IndexMap[K, V]) GetIndexOf(key K) (int, bool) {
	i, ok := m.indices[key]
	return i, ok
}

// GetIndex returns the key and the value at an index, and false when the
// index is not before Len.
//
// GetIndex is IndexMap::get_index.
func (m *IndexMap[K, V]) GetIndex(i int) (K, V, bool) {
	if i < 0 || i >= len(m.keys) {
		var (
			k K
			v V
		)
		return k, v, false
	}
	return m.keys[i], m.values[i], true
}

// GetIndexMut returns the key and a pointer to the value at an index, and
// false when the index is not before Len.
//
// GetIndexMut is IndexMap::get_index_mut.
func (m *IndexMap[K, V]) GetIndexMut(i int) (K, *V, bool) {
	if i < 0 || i >= len(m.keys) {
		var k K
		return k, nil, false
	}
	return m.keys[i], &m.values[i], true
}

// Insert sets the value of a key. A new key goes at the end. A key that the
// map holds keeps its place, and Insert returns its old value and true.
//
// Insert is IndexMap::insert.
func (m *IndexMap[K, V]) Insert(key K, value V) (V, bool) {
	if i, ok := m.indices[key]; ok {
		old := m.values[i]
		m.values[i] = value
		return old, true
	}
	m.push(key, value)
	var zero V
	return zero, false
}

// push adds a new key at the end.
func (m *IndexMap[K, V]) push(key K, value V) int {
	if m.indices == nil {
		m.indices = make(map[K]int)
	}
	i := len(m.keys)
	m.indices[key] = i
	m.keys = append(m.keys, key)
	m.values = append(m.values, value)
	return i
}

// GetOrInsert returns a pointer to the value of a key. When the map does not
// hold the key, GetOrInsert adds it at the end with value.
//
// GetOrInsert is IndexMap::entry(key).or_insert(value), and
// or_default with the zero value.
func (m *IndexMap[K, V]) GetOrInsert(key K, value V) *V {
	i, ok := m.indices[key]
	if !ok {
		i = m.push(key, value)
	}
	return &m.values[i]
}

// GetOrInsertFunc returns a pointer to the value of a key. When the map does
// not hold the key, GetOrInsertFunc adds it at the end with the value that f
// returns.
//
// GetOrInsertFunc is IndexMap::entry(key).or_insert_with(f).
func (m *IndexMap[K, V]) GetOrInsertFunc(key K, f func() V) *V {
	i, ok := m.indices[key]
	if !ok {
		i = m.push(key, f())
	}
	return &m.values[i]
}

// Extend inserts each key and value of seq, in order, as Insert does.
//
// Extend is the Extend of IndexMap.
func (m *IndexMap[K, V]) Extend(seq iter.Seq2[K, V]) {
	for k, v := range seq {
		m.Insert(k, v)
	}
}

// All returns each key and value, in order.
//
// All is IndexMap::iter.
func (m *IndexMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i, k := range m.keys {
			if !yield(k, m.values[i]) {
				return
			}
		}
	}
}

// AllMut returns each key and a pointer to its value, in order.
//
// AllMut is IndexMap::iter_mut.
func (m *IndexMap[K, V]) AllMut() iter.Seq2[K, *V] {
	return func(yield func(K, *V) bool) {
		for i, k := range m.keys {
			if !yield(k, &m.values[i]) {
				return
			}
		}
	}
}

// Keys returns each key, in order.
//
// Keys is IndexMap::keys.
func (m *IndexMap[K, V]) Keys() iter.Seq[K] {
	return slices.Values(m.keys)
}

// Values returns each value, in order.
//
// Values is IndexMap::values.
func (m *IndexMap[K, V]) Values() iter.Seq[V] {
	return slices.Values(m.values)
}

// ValuesMut returns a pointer to each value, in order.
//
// ValuesMut is IndexMap::values_mut.
func (m *IndexMap[K, V]) ValuesMut() iter.Seq[*V] {
	return func(yield func(*V) bool) {
		for i := range m.values {
			if !yield(&m.values[i]) {
				return
			}
		}
	}
}

// Clone returns a copy of the map. The values are copied as Go copies them.
//
// Clone is the Clone of IndexMap.
func (m *IndexMap[K, V]) Clone() IndexMap[K, V] {
	return IndexMap[K, V]{
		keys:    slices.Clone(m.keys),
		values:  slices.Clone(m.values),
		indices: maps.Clone(m.indices),
	}
}

// Drain returns each key and value, in order, and removes them all from the
// map. The map keeps its capacity. The entries are removed when the loop
// ends, even when it ends early.
//
// Drain is IndexMap::drain(..).
func (m *IndexMap[K, V]) Drain() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		defer func() {
			clear(m.keys)
			clear(m.values)
			m.keys = m.keys[:0]
			m.values = m.values[:0]
			clear(m.indices)
		}()
		for i, k := range m.keys {
			if !yield(k, m.values[i]) {
				return
			}
		}
	}
}

// ShrinkToFit gives back the capacity that the map grew into, and keeps its
// entries and their order. A pointer that a method returned into the map is
// not valid after ShrinkToFit.
//
// ShrinkToFit is IndexMap::shrink_to_fit. A Go map does not shrink, so
// ShrinkToFit makes the map of indices again, with room for the entries
// only.
func (m *IndexMap[K, V]) ShrinkToFit() {
	if len(m.keys) == 0 {
		*m = IndexMap[K, V]{}
		return
	}
	keys := make([]K, len(m.keys))
	copy(keys, m.keys)
	values := make([]V, len(m.values))
	copy(values, m.values)
	indices := make(map[K]int, len(keys))
	for i, k := range keys {
		indices[k] = i
	}
	*m = IndexMap[K, V]{keys: keys, values: values, indices: indices}
}
