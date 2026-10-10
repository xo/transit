package generate

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"
	"strconv"
	"strings"
)

// This file ports crates/generate/src/build_tables/item.rs: the parse items,
// the sets of parse items that become the states of the parse table, and the
// pool of lookahead sets.

// itemStartSteps is the one step of the start production: the start symbol,
// the non-terminal 0.
//
// itemStartSteps is START_STEPS.
var itemStartSteps = [1]ProductionStep{{
	SymIndex: 0,
	PrecVal:  0,
	Alias:    0,
	Field:    0,
	Reserved: NoReservedWords,
	Flags:    uint8(SymbolNonTerminal),
}}

// StartProductionID is the production id of the start production, which the
// generator adds and which is not in the grammar.
//
// StartProductionID is START_PRODUCTION_ID.
const StartProductionID uint32 = math.MaxUint32

// itemStartProduction returns the start production.
//
// itemStartProduction is start_production.
func itemStartProduction() ProdRef {
	return ProdRef{
		Steps:                itemStartSteps[:],
		DynamicPrecedence:    0,
		RequiresEOFLookahead: false,
	}
}

// DotKeys is the pair of identity keys of one production and one position of
// the dot.
//
// Cmp is the rank of the content that the Ord of ParseItem compares: the
// dynamic precedence, RequiresEOFLookahead, the length, the precedence and the
// associativity at the dot, then the aliases and the fields of the steps
// before the dot, and the steps after the dot in full. Two ranks are equal
// exactly when the content is equal, so Cmp is also the class of equality of
// an item that has no preceding inherited fields. EqWithSyms divides Cmp by
// the symbols of the steps before the dot, which take part in the equality
// only when HasPrecedingInheritedFields is true.
//
// DotKeys is DotKeys.
type DotKeys struct {
	Cmp        uint32
	EqWithSyms uint32
}

// ItemKeyMap holds the identity keys of every production and every position
// of the dot in a grammar: all the productions of the grammar, every inlined
// production, and the start production.
//
// ItemKeyMap is ItemKeyMap.
type ItemKeyMap struct {
	keys  [][]DotKeys
	start []DotKeys
}

// NewItemKeyMap returns the identity keys of the productions of a grammar.
//
// NewItemKeyMap is ItemKeyMap::new.
func NewItemKeyMap(grammar *SyntaxGrammar, strPool *StrPool) *ItemKeyMap {
	prod := func(slot int) ProdRef {
		if slot == 0 {
			return itemStartProduction()
		}
		return grammar.Production(uint32(slot - 1))
	}
	slotCount := len(grammar.Productions) + 1

	type slotDot struct{ pi, dot uint32 }
	contents := make([]slotDot, 0, slotCount)
	for pi := range slotCount {
		for dot := range len(prod(pi).Steps) + 1 {
			contents = append(contents, slotDot{pi: uint32(pi), dot: uint32(dot)})
		}
	}
	content := func(p slotDot) itemContent {
		return itemContent{production: prod(int(p.pi)), dot: int(p.dot), strPool: strPool}
	}
	// Upstream sorts with sort_unstable_by, so the order of two pairs with
	// equal content is not fixed. That order does not change the ranks in
	// Cmp. It changes only the numbers that EqWithSyms gives to the classes
	// of one rank, and so the order of two items that differ only in the
	// symbols before the dot. Two such items are never in one item set: every
	// item of a set with the dot at d has the same d symbols before the dot,
	// the last d symbols of the path to the state. So only the equality of
	// EqWithSyms matters, and it is the same for every order. The stable sort
	// makes the Go numbers fixed.
	slices.SortStableFunc(contents, func(a, b slotDot) int {
		return content(a).compare(content(b))
	})

	slotKeys := make([][]DotKeys, slotCount)
	for pi := range slotKeys {
		slotKeys[pi] = make([]DotKeys, len(prod(pi).Steps)+1)
	}
	// Dense ids in sorted order: equal content shares an id, distinct content gets the next up
	var cmpID uint32
	var prev slotDot
	hasPrev := false
	for _, p := range contents {
		if hasPrev && content(prev).compare(content(p)) != 0 {
			cmpID++
		}
		slotKeys[p.pi][p.dot].Cmp = cmpID
		prev, hasPrev = p, true
	}

	// Refine each cmp class by preceding symbols:  read only under
	// `has_preceding_inherited_fields`.
	//
	// Upstream keys an FxHashMap with the class and the symbols, and only
	// looks keys up.
	symClasses := make(map[string]uint32)
	var buf []byte
	for _, p := range contents {
		keys := &slotKeys[p.pi][p.dot]
		buf = binary.LittleEndian.AppendUint32(buf[:0], keys.Cmp)
		for _, s := range prod(int(p.pi)).Steps[:p.dot] {
			buf = binary.LittleEndian.AppendUint64(buf, s.Symbol().packedKey())
		}
		next := uint32(len(symClasses))
		class, ok := symClasses[string(buf)]
		if !ok {
			class = next
			symClasses[string(buf)] = class
		}
		keys.EqWithSyms = class
	}

	return &ItemKeyMap{keys: slotKeys[1:], start: slotKeys[0]}
}

// KeysFor returns the keys of a production of the grammar, by the position of
// the dot.
//
// KeysFor is ItemKeyMap::keys_for.
func (m *ItemKeyMap) KeysFor(id uint32) []DotKeys {
	if id == StartProductionID {
		return m.start
	}
	return m.keys[id]
}

// StartKeys returns the keys of the start production.
//
// StartKeys is ItemKeyMap::start_keys.
func (m *ItemKeyMap) StartKeys() []DotKeys {
	return m.start
}

// itemContent is the content that the Ord of ParseItem, and its Eq when the
// item has no preceding inherited fields, look at, for one production and one
// position of the dot. A rank of all the keys in this order keeps the order,
// so Cmp is dense. Items compare only at the same dot, and the comparison of
// the dot keeps the order total across dots, so the ranks are well defined.
//
// itemContent is ItemContent.
type itemContent struct {
	production ProdRef
	dot        int
	strPool    *StrPool
}

// prec returns the precedence of the step before the dot.
//
// prec is ItemContent::prec.
func (c itemContent) prec() Precedence {
	if c.dot > 0 {
		return c.production.Steps[c.dot-1].Precedence()
	}
	return Precedence{}
}

// assoc returns the associativity of the step before the dot.
//
// assoc is ItemContent::assoc.
func (c itemContent) assoc() Associativity {
	if c.dot > 0 {
		return c.production.Steps[c.dot-1].Associativity()
	}
	return AssociativityNone
}

// precCmp orders two precedences: none, then the numbers, then the names.
// Two names compare by their text.
//
// precCmp is ItemContent::prec_cmp.
func (c itemContent) precCmp(a, b Precedence) int {
	if a.Kind != b.Kind {
		return cmp.Compare(a.Kind, b.Kind)
	}
	switch a.Kind {
	case PrecedenceInteger:
		return cmp.Compare(a.Integer, b.Integer)
	case PrecedenceName:
		return strings.Compare(c.strPool.Resolve(a.Name), c.strPool.Resolve(b.Name))
	}
	return 0
}

// aliasCmp orders two steps by their alias: none first, then by the text,
// then by whether the alias is named.
//
// aliasCmp is ItemContent::alias_cmp.
func (c itemContent) aliasCmp(a, b ProductionStep) int {
	x, xok := a.GetAlias()
	y, yok := b.GetAlias()
	if r := compareBool(xok, yok); r != 0 || !xok {
		return r
	}
	return cmp.Or(
		strings.Compare(c.strPool.Resolve(x.Value), c.strPool.Resolve(y.Value)),
		compareBool(x.IsNamed, y.IsNamed),
	)
}

// fieldCmp orders two steps by their field: none first, then by the text.
//
// fieldCmp is ItemContent::field_cmp.
func (c itemContent) fieldCmp(a, b ProductionStep) int {
	x, xok := a.GetField()
	y, yok := b.GetField()
	if r := compareBool(xok, yok); r != 0 || !xok {
		return r
	}
	return strings.Compare(c.strPool.Resolve(x), c.strPool.Resolve(y))
}

// stepCmp orders two steps by their symbol, precedence, associativity, alias,
// field and reserved word set.
//
// stepCmp is ItemContent::step_cmp.
func (c itemContent) stepCmp(a, b ProductionStep) int {
	if r := CompareSymbol(a.Symbol(), b.Symbol()); r != 0 {
		return r
	}
	if r := c.precCmp(a.Precedence(), b.Precedence()); r != 0 {
		return r
	}
	if r := cmp.Compare(a.Associativity(), b.Associativity()); r != 0 {
		return r
	}
	if r := c.aliasCmp(a, b); r != 0 {
		return r
	}
	if r := c.fieldCmp(a, b); r != 0 {
		return r
	}
	return cmp.Compare(a.Reserved, b.Reserved)
}

// compare orders two contents.
//
// compare is the Ord of ItemContent, and its PartialEq is compare() == 0.
func (c itemContent) compare(other itemContent) int {
	if r := cmp.Compare(c.production.DynamicPrecedence, other.production.DynamicPrecedence); r != 0 {
		return r
	}
	if r := compareBool(c.production.RequiresEOFLookahead, other.production.RequiresEOFLookahead); r != 0 {
		return r
	}
	if r := cmp.Compare(len(c.production.Steps), len(other.production.Steps)); r != 0 {
		return r
	}
	if r := c.precCmp(c.prec(), other.prec()); r != 0 {
		return r
	}
	if r := cmp.Compare(c.assoc(), other.assoc()); r != 0 {
		return r
	}
	if r := cmp.Compare(c.dot, other.dot); r != 0 {
		return r
	}
	for i := range min(len(c.production.Steps), len(other.production.Steps)) {
		sa, sb := c.production.Steps[i], other.production.Steps[i]
		var o int
		if i < c.dot {
			o = cmp.Or(c.aliasCmp(sa, sb), c.fieldCmp(sa, sb))
		} else {
			o = c.stepCmp(sa, sb)
		}
		if o != 0 {
			return o
		}
	}
	return 0
}

// ParseItem is a match of one production of a grammar that is in progress.
//
// ParseItem is ParseItem.
type ParseItem struct {
	// VariableIndex is the index of the parent rule within the grammar.
	VariableIndex uint32
	// StepIndex is the number of symbols that have already been matched.
	StepIndex uint32
	// ProdID is the id of the production being matched.
	ProdID uint32
	// Keys is the identity keys of the production, indexed by StepIndex.
	Keys []DotKeys
	// HasPrecedingInheritedFields is true when any of the already-matched
	// children were hidden nodes and had fields. Ordinarily, a parse item's
	// behavior is not affected by the symbols of its preceding children; it
	// only needs to keep track of their fields and aliases.
	//
	// Take for example these two items:
	//   X -> a b • c
	//   X -> a g • c
	//
	// They can be considered equivalent, for the purposes of parse table
	// generation, because they entail the same actions. But if this flag is
	// true, then the item's set of inherited fields may depend on the specific
	// symbols of its preceding children.
	HasPrecedingInheritedFields bool
}

// LookaheadSetID is the id of a lookahead set in a LookaheadSetPool. The ids
// are canonical, so two ids are equal exactly when their sets are equal. The
// zero LookaheadSetID is the empty set.
//
// LookaheadSetID is LookaheadSetId, and its Default is
// LookaheadSetPool::EMPTY.
type LookaheadSetID uint32

// lookaheadInsertKey is the key of the memo of LookaheadSetPool.Insert.
type lookaheadInsertKey struct {
	id     LookaheadSetID
	symbol Symbol
}

// LookaheadSetPool interns lookahead sets. The entries of an item set hold
// ids.
//
// LookaheadSetPool is LookaheadSetPool. Upstream holds the sets in an
// IndexSet. Go holds them in a slice, with a map from the Key of each set to
// its index.
type LookaheadSetPool struct {
	sets       []TokenSet
	indices    map[string]uint32
	unionMemo  map[[2]LookaheadSetID]LookaheadSetID
	insertMemo map[lookaheadInsertKey]LookaheadSetID
}

// LookaheadSetPoolEmpty is the id of the empty set.
//
// LookaheadSetPoolEmpty is LookaheadSetPool::EMPTY.
const LookaheadSetPoolEmpty LookaheadSetID = 0

// NewLookaheadSetPool returns a pool that holds the empty set, with the id
// LookaheadSetPoolEmpty.
//
// NewLookaheadSetPool is LookaheadSetPool::new.
func NewLookaheadSetPool() *LookaheadSetPool {
	pool := &LookaheadSetPool{
		indices:    make(map[string]uint32),
		unionMemo:  make(map[[2]LookaheadSetID]LookaheadSetID),
		insertMemo: make(map[lookaheadInsertKey]LookaheadSetID),
	}
	if empty := pool.Intern(TokenSet{}); empty != LookaheadSetPoolEmpty {
		panic("generate: the empty lookahead set must have the first id")
	}
	return pool
}

// Get returns the set of an id. Do not change the set.
//
// Get is LookaheadSetPool::get.
func (p *LookaheadSetPool) Get(id LookaheadSetID) *TokenSet {
	return &p.sets[id]
}

// Intern returns the id of a set, and adds the set to the pool when it is new.
// The pool takes the set, so do not change it after the call.
//
// Intern is LookaheadSetPool::intern.
func (p *LookaheadSetPool) Intern(set TokenSet) LookaheadSetID {
	key := set.Key()
	if index, ok := p.indices[key]; ok {
		return LookaheadSetID(index)
	}
	index := uint32(len(p.sets))
	p.sets = append(p.sets, set)
	p.indices[key] = index
	return LookaheadSetID(index)
}

// InternRef returns the id of a set, and adds a copy of the set to the pool
// when it is new.
//
// InternRef is LookaheadSetPool::intern_ref.
func (p *LookaheadSetPool) InternRef(set *TokenSet) LookaheadSetID {
	if index, ok := p.indices[set.Key()]; ok {
		return LookaheadSetID(index)
	}
	return p.Intern(set.Clone())
}

// Singleton returns the id of the set that holds only symbol.
//
// Singleton is LookaheadSetPool::singleton.
func (p *LookaheadSetPool) Singleton(symbol Symbol) LookaheadSetID {
	return p.Insert(LookaheadSetPoolEmpty, symbol)
}

// Insert returns the id of the set of id with symbol added.
//
// Insert is LookaheadSetPool::insert.
func (p *LookaheadSetPool) Insert(id LookaheadSetID, symbol Symbol) LookaheadSetID {
	if p.Get(id).Contains(symbol) {
		return id
	}
	key := lookaheadInsertKey{id: id, symbol: symbol}
	if result, ok := p.insertMemo[key]; ok {
		return result
	}
	set := p.Get(id).Clone()
	set.Insert(symbol)
	result := p.Intern(set)
	p.insertMemo[key] = result
	return result
}

// Union returns the id of the union of two sets.
//
// Union is LookaheadSetPool::union.
func (p *LookaheadSetPool) Union(left, right LookaheadSetID) LookaheadSetID {
	if left == right || right == LookaheadSetPoolEmpty {
		return left
	}
	if left == LookaheadSetPoolEmpty {
		return right
	}
	key := [2]LookaheadSetID{min(left, right), max(left, right)}
	if result, ok := p.unionMemo[key]; ok {
		return result
	}
	set := p.Get(key[0]).Clone()
	set.InsertAll(p.Get(key[1]))
	result := p.Intern(set)
	p.unionMemo[key] = result
	return result
}

// ParseItemSet is a set of matches of productions of a grammar that are in
// progress, sorted by item.
//
// For each in-progress match, a set of "lookaheads" (tokens that are allowed to
// *follow* the in-progress rule) are included. This object corresponds directly
// to a state in the final parse table.
//
// ParseItemSet is ParseItemSet. The zero ParseItemSet is
// ParseItemSet::default.
type ParseItemSet struct {
	Entries []ParseItemSetEntry
}

// ParseItemSetEntry is an item of an item set, with its lookahead set and the
// reserved word set that can follow it.
//
// ParseItemSetEntry is ParseItemSetEntry.
type ParseItemSetEntry struct {
	Item                     ParseItem
	Lookaheads               LookaheadSetID
	FollowingReservedWordSet ReservedWordSetID
}

// ParseItemSetCore is like a ParseItemSet, but without the lookahead
// information. Parse states with the same core are candidates for merging.
//
// ParseItemSetCore is ParseItemSetCore.
type ParseItemSetCore struct {
	Entries []ParseItem
}

// ParseItemDisplay shows an item as text, for the messages of the generator.
//
// ParseItemDisplay is ParseItemDisplay.
type ParseItemDisplay struct {
	Item    *ParseItem
	Syntax  *SyntaxGrammar
	Lexical *LexicalGrammar
	Pool    *StrPool
}

// TokenSetDisplay shows a token set as text, for the messages of the
// generator.
//
// TokenSetDisplay is TokenSetDisplay.
type TokenSetDisplay struct {
	Set     *TokenSet
	Syntax  *SyntaxGrammar
	Lexical *LexicalGrammar
	Pool    *StrPool
}

// ParseItemSetDisplay shows an item set as text, for the messages of the
// generator.
//
// ParseItemSetDisplay is ParseItemSetDisplay.
type ParseItemSetDisplay struct {
	Set        *ParseItemSet
	Syntax     *SyntaxGrammar
	Lexical    *LexicalGrammar
	Pool       *StrPool
	Lookaheads *LookaheadSetPool
}

// StartParseItem returns the item of the start production, with the dot at
// the start.
//
// StartParseItem is ParseItem::start.
func StartParseItem(keyMap *ItemKeyMap) ParseItem {
	return ParseItem{
		VariableIndex:               math.MaxUint32,
		ProdID:                      StartProductionID,
		Keys:                        keyMap.StartKeys(),
		StepIndex:                   0,
		HasPrecedingInheritedFields: false,
	}
}

// Production returns the production being matched.
//
// Production is ParseItem::production.
func (i *ParseItem) Production(grammar *SyntaxGrammar) ProdRef {
	if i.ProdID == StartProductionID {
		return itemStartProduction()
	}
	return grammar.Production(i.ProdID)
}

// Step returns the step after the dot, and false when the dot is at the end.
//
// Step is ParseItem::step.
func (i *ParseItem) Step(grammar *SyntaxGrammar) (ProductionStep, bool) {
	steps := i.Production(grammar).Steps
	if int(i.StepIndex) < len(steps) {
		return steps[i.StepIndex], true
	}
	return ProductionStep{}, false
}

// Symbol returns the symbol after the dot, and false when the dot is at the
// end.
//
// Symbol is ParseItem::symbol.
func (i *ParseItem) Symbol(grammar *SyntaxGrammar) (Symbol, bool) {
	step, ok := i.Step(grammar)
	if !ok {
		return Symbol{}, false
	}
	return step.Symbol(), true
}

// Associativity returns the associativity of the step before the dot, and
// AssociativityNone when the dot is at the start.
//
// Associativity is ParseItem::associativity.
func (i *ParseItem) Associativity(grammar *SyntaxGrammar) Associativity {
	step, ok := i.PrevStep(grammar)
	if !ok {
		return AssociativityNone
	}
	return step.Associativity()
}

// Precedence returns the precedence of the step before the dot, and no
// precedence when the dot is at the start.
//
// Precedence is ParseItem::precedence.
func (i *ParseItem) Precedence(grammar *SyntaxGrammar) Precedence {
	step, ok := i.PrevStep(grammar)
	if !ok {
		return Precedence{}
	}
	return step.Precedence()
}

// PrevStep returns the step before the dot, and false when the dot is at the
// start.
//
// PrevStep is ParseItem::prev_step.
func (i *ParseItem) PrevStep(grammar *SyntaxGrammar) (ProductionStep, bool) {
	if i.StepIndex > 0 {
		return i.Production(grammar).Steps[i.StepIndex-1], true
	}
	return ProductionStep{}, false
}

// IsDone reports whether the dot is at the end of the production.
//
// IsDone is ParseItem::is_done.
func (i *ParseItem) IsDone() bool {
	return int(i.StepIndex)+1 == len(i.Keys)
}

// IsAugmented reports whether the item matches the start production.
//
// IsAugmented is ParseItem::is_augmented.
func (i *ParseItem) IsAugmented() bool {
	return i.VariableIndex == math.MaxUint32
}

// Successor returns an item like this one, but advanced by one step.
//
// Successor is ParseItem::successor.
func (i *ParseItem) Successor() ParseItem {
	return ParseItem{
		VariableIndex:               i.VariableIndex,
		ProdID:                      i.ProdID,
		Keys:                        i.Keys,
		StepIndex:                   i.StepIndex + 1,
		HasPrecedingInheritedFields: i.HasPrecedingInheritedFields,
	}
}

// SubstituteProduction returns an item identical to this one, but with a
// different production. This is used when dynamically "inlining" certain
// symbols in a production.
//
// SubstituteProduction is ParseItem::substitute_production.
func (i *ParseItem) SubstituteProduction(prodID uint32, keys []DotKeys) ParseItem {
	result := *i
	result.ProdID = prodID
	result.Keys = keys
	return result
}

// dotKeys returns the identity keys of the item at the current dot.
//
// dotKeys is ParseItem::dot_keys.
func (i *ParseItem) dotKeys() DotKeys {
	return i.Keys[i.StepIndex]
}

// Insert adds an item to the set in its sorted place, with an empty lookahead
// set, when the set does not hold it yet. It returns the entry of the item.
// The pointer is valid until the next Insert.
//
// Insert is ParseItemSet::insert.
func (s *ParseItemSet) Insert(item ParseItem) *ParseItemSetEntry {
	// Entries is sorted with no duplicates, so an item that sorts after the last entry
	// belongs at the end, which is where the binary search would put it.
	//
	// Checking the end first pays off because items mostly arrive in order: addActions
	// builds each successor set by advancing a sorted closure's items past the same symbol.
	// Items are ordered by step index, then rule, then their content at the dot. Advancing
	// adds one to every step index and keeps every rule, so only items of the same rule at
	// the same step can change places.
	var index int
	if n := len(s.Entries); n == 0 || s.Entries[n-1].Item.Compare(item) < 0 {
		index = n
	} else {
		i, found := slices.BinarySearchFunc(s.Entries, item, func(e ParseItemSetEntry, item ParseItem) int {
			return e.Item.Compare(item)
		})
		if found {
			return &s.Entries[i]
		}
		index = i
	}
	s.Entries = slices.Insert(s.Entries, index, ParseItemSetEntry{
		Item:                     item,
		Lookaheads:               LookaheadSetPoolEmpty,
		FollowingReservedWordSet: 0,
	})
	return &s.Entries[index]
}

// Core returns the items of the set, without their lookaheads.
//
// Core is ParseItemSet::core.
func (s *ParseItemSet) Core() ParseItemSetCore {
	entries := make([]ParseItem, len(s.Entries))
	for i, e := range s.Entries {
		entries[i] = e.Item
	}
	return ParseItemSetCore{Entries: entries}
}

// PrecDisplay returns the text of a precedence: none, a number, or a name in
// quotes.
//
// PrecDisplay is prec_display.
func PrecDisplay(prec Precedence, strPool *StrPool) string {
	switch prec.Kind {
	case PrecedenceInteger:
		return strconv.FormatInt(int64(prec.Integer), 10)
	case PrecedenceName:
		return "'" + strPool.Resolve(prec.Name) + "'"
	}
	return "none"
}

// associativityDebug returns the Debug text of an associativity that is not
// none.
//
// associativityDebug is the Debug of Associativity.
func associativityDebug(a Associativity) string {
	if a == AssociativityLeft {
		return "Left"
	}
	return "Right"
}

// String returns the text of the item.
//
// String is the Display of ParseItemDisplay.
func (d ParseItemDisplay) String() string {
	var b strings.Builder
	if d.Item.IsAugmented() {
		b.WriteString("START →")
	} else {
		b.WriteString(d.Pool.Resolve(d.Syntax.Variables[d.Item.VariableIndex].Name) + " →")
	}

	production := d.Item.Production(d.Syntax)
	for i, step := range production.Steps {
		symbol := step.Symbol()
		if i == int(d.Item.StepIndex) {
			b.WriteString(" •")
			if step.Precedence().Kind != PrecedenceNone ||
				step.Associativity() != AssociativityNone ||
				step.Reserved != 0 {
				b.WriteString(" (")
				if step.Precedence().Kind != PrecedenceNone {
					b.WriteString(" " + PrecDisplay(step.Precedence(), d.Pool))
				}
				if a := step.Associativity(); a != AssociativityNone {
					b.WriteString(" " + associativityDebug(a))
				}
				if step.Reserved != 0 {
					b.WriteString("reserved: " + strconv.Itoa(int(step.Reserved)))
				}
				b.WriteString(" )")
			}
		}

		b.WriteString(" ")
		switch symbol.Kind() {
		case SymbolTerminal:
			index := int(symbol.index)
			if index < len(d.Lexical.Variables) {
				b.WriteString(d.Pool.Resolve(d.Lexical.Variables[index].Name))
			} else {
				b.WriteString("terminal-" + strconv.Itoa(index))
			}
		case SymbolExternal:
			b.WriteString(d.Pool.Resolve(d.Syntax.ExternalTokens[symbol.index].Name))
		case SymbolNonTerminal:
			b.WriteString(d.Pool.Resolve(d.Syntax.Variables[symbol.index].Name))
		case SymbolEnd:
			b.WriteString("<EOF>")
		case SymbolEndOfNonTerminalExtra:
			b.WriteString("<END_OF_NONTERMINAL_EXTRA>")
		}

		if alias, ok := step.GetAlias(); ok {
			b.WriteString("@" + d.Pool.Resolve(alias.Value))
		}
	}

	if d.Item.IsDone() {
		b.WriteString(" •")
		if n := len(production.Steps); n > 0 {
			step := production.Steps[n-1]
			if a := step.Associativity(); a != AssociativityNone {
				if step.Precedence().Kind == PrecedenceNone {
					b.WriteString(" (" + associativityDebug(a) + ")")
				} else {
					b.WriteString(" (" + PrecDisplay(step.Precedence(), d.Pool) + " " + associativityDebug(a) + ")")
				}
			} else if step.Precedence().Kind != PrecedenceNone {
				b.WriteString(" (" + PrecDisplay(step.Precedence(), d.Pool) + ")")
			}
		}
	}

	return b.String()
}

// escapeInvisible returns the escape of a character that does not show, and
// false for any other character.
//
// escapeInvisible is escape_invisible.
func escapeInvisible(c byte) (string, bool) {
	switch c {
	case '\n':
		return `\n`, true
	case '\r':
		return `\r`, true
	case '\t':
		return `\t`, true
	case 0:
		return `\0`, true
	case '\\':
		return `\\`, true
	case '\v':
		return `\v`, true
	case '\f':
		return `\f`, true
	}
	return "", false
}

// displayVariableName returns a name with each character that does not show
// escaped.
//
// displayVariableName is display_variable_name. Upstream walks the
// characters. Every character that it escapes is ASCII, so Go walks the
// bytes, and every other byte stays as it is.
func displayVariableName(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	for i := range len(source) {
		if esc, ok := escapeInvisible(source[i]); ok {
			b.WriteString(esc)
		} else {
			b.WriteByte(source[i])
		}
	}
	return b.String()
}

// String returns the text of the token set.
//
// String is the Display of TokenSetDisplay.
func (d TokenSetDisplay) String() string {
	var b strings.Builder
	b.WriteString("[")
	i := 0
	for symbol := range d.Set.All() {
		if i > 0 {
			b.WriteString(", ")
		}
		i++

		switch symbol.Kind() {
		case SymbolTerminal:
			index := int(symbol.index)
			if index < len(d.Lexical.Variables) {
				b.WriteString(displayVariableName(d.Pool.Resolve(d.Lexical.Variables[index].Name)))
			} else {
				b.WriteString("terminal-" + strconv.Itoa(index))
			}
		case SymbolExternal:
			b.WriteString(d.Pool.Resolve(d.Syntax.ExternalTokens[symbol.index].Name))
		case SymbolNonTerminal:
			b.WriteString(d.Pool.Resolve(d.Syntax.Variables[symbol.index].Name))
		case SymbolEnd:
			b.WriteString("<EOF>")
		case SymbolEndOfNonTerminalExtra:
			b.WriteString("<END_OF_NONTERMINAL_EXTRA>")
		}
	}
	b.WriteString("]")
	return b.String()
}

// String returns the text of the item set, one entry on each line.
//
// String is the Display of ParseItemSetDisplay.
func (d ParseItemSetDisplay) String() string {
	var b strings.Builder
	for i := range d.Set.Entries {
		entry := &d.Set.Entries[i]
		b.WriteString(ParseItemDisplay{Item: &entry.Item, Syntax: d.Syntax, Lexical: d.Lexical, Pool: d.Pool}.String())
		b.WriteString("\t")
		b.WriteString(TokenSetDisplay{Set: d.Lookaheads.Get(entry.Lookaheads), Syntax: d.Syntax, Lexical: d.Lexical, Pool: d.Pool}.String())
		if entry.FollowingReservedWordSet != 0 {
			b.WriteString("\treserved word set: " + strconv.FormatUint(uint64(entry.FollowingReservedWordSet), 10))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ParseItemKey is a value that is the same for two items exactly when they
// are equal, so it can be the key of a Go map.
//
// ParseItemKey is what the Hash of ParseItem hashes.
type ParseItemKey struct {
	VariableIndex               uint32
	StepIndex                   uint32
	HasPrecedingInheritedFields bool
	Class                       uint32
}

// Key returns the key of the item.
//
// The already-matched children don't play any role in the parse state for
// this item, unless any of the following are true:
//   - the children have fields
//   - the children have aliases
//   - the children are hidden and represent rules that have fields.
//
// See the docs for HasPrecedingInheritedFields. Preceding symbols participate
// only in EqWithSyms.
//
// Key stands in for the Hash of ParseItem.
func (i *ParseItem) Key() ParseItemKey {
	keys := i.dotKeys()
	class := keys.Cmp
	if i.HasPrecedingInheritedFields {
		class = keys.EqWithSyms
	}
	return ParseItemKey{
		VariableIndex:               i.VariableIndex,
		StepIndex:                   i.StepIndex,
		HasPrecedingInheritedFields: i.HasPrecedingInheritedFields,
		Class:                       class,
	}
}

// Equal reports whether two items are equal.
//
// Equal is the PartialEq of ParseItem.
func (i *ParseItem) Equal(other *ParseItem) bool {
	if i.VariableIndex != other.VariableIndex ||
		i.StepIndex != other.StepIndex ||
		i.HasPrecedingInheritedFields != other.HasPrecedingInheritedFields {
		return false
	}

	if i.HasPrecedingInheritedFields {
		return i.dotKeys().EqWithSyms == other.dotKeys().EqWithSyms
	}
	return i.dotKeys().Cmp == other.dotKeys().Cmp
}

// Compare orders two items by step index, variable index, the rank of their
// content, HasPrecedingInheritedFields, and then, when that is true, the
// class of their preceding symbols.
//
// Compare is the Ord of ParseItem.
func (i ParseItem) Compare(other ParseItem) int {
	if c := cmp.Compare(i.StepIndex, other.StepIndex); c != 0 {
		return c
	}
	if c := cmp.Compare(i.VariableIndex, other.VariableIndex); c != 0 {
		return c
	}
	if c := cmp.Compare(i.dotKeys().Cmp, other.dotKeys().Cmp); c != 0 {
		return c
	}
	if c := compareBool(i.HasPrecedingInheritedFields, other.HasPrecedingInheritedFields); c != 0 {
		return c
	}
	if i.HasPrecedingInheritedFields {
		return cmp.Compare(i.dotKeys().EqWithSyms, other.dotKeys().EqWithSyms)
	}
	return 0
}

// appendItemKey appends the key of an item to buf.
func appendItemKey(buf []byte, item *ParseItem) []byte {
	k := item.Key()
	buf = binary.LittleEndian.AppendUint32(buf, k.VariableIndex)
	buf = binary.LittleEndian.AppendUint32(buf, k.StepIndex)
	buf = append(buf, byte(boolWord(k.HasPrecedingInheritedFields)))
	return binary.LittleEndian.AppendUint32(buf, k.Class)
}

// Key returns a string that is the same for two item sets exactly when they
// are equal, for a map key. Two sets are equal when their entries are equal
// in order: the items, the lookahead ids and the reserved word sets.
//
// Key stands in for the Hash and the Eq of ParseItemSet.
func (s *ParseItemSet) Key() string {
	buf := make([]byte, 0, 21*len(s.Entries))
	for i := range s.Entries {
		entry := &s.Entries[i]
		buf = appendItemKey(buf, &entry.Item)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(entry.Lookaheads))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(entry.FollowingReservedWordSet))
	}
	return string(buf)
}

// Key returns a string that is the same for two cores exactly when they are
// equal, for a map key.
//
// Key stands in for the Hash and the Eq of ParseItemSetCore.
func (c *ParseItemSetCore) Key() string {
	buf := make([]byte, 0, 13*len(c.Entries))
	for i := range c.Entries {
		buf = appendItemKey(buf, &c.Entries[i])
	}
	return string(buf)
}
