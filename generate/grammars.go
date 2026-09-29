package generate

// This file ports crates/generate/src/grammars.rs. Two parts wait for the
// modules that they use: LexicalGrammar, which holds the NFA of nfa.rs, with
// its methods variable_indices_for_nfa_states and
// variable_index_for_nfa_state, and ProductionStep::child_type, which returns
// the ChildType of node_types.rs. Each one comes with its module.

// VariableType is the kind of a variable. The order of the values is the
// order of upstream.
//
// VariableType is VariableType.
type VariableType uint8

// The kinds of variable.
const (
	VariableHidden VariableType = iota
	VariableAuxiliary
	VariableAnonymous
	VariableNamed
)

// IsVisible reports whether a node of the kind shows in the tree.
//
// IsVisible is VariableType::is_visible.
func (t VariableType) IsVisible() bool {
	return t == VariableNamed || t == VariableAnonymous
}

// Variable is a rule of a grammar: its name, and the root of its rule in the
// pool.
//
// Variable is Variable.
type Variable struct {
	Name StrID
	Root RuleID
}

// PrecedenceEntryKind is the kind of an entry of a precedence list.
type PrecedenceEntryKind uint8

// The kinds of entry, in the order of upstream.
const (
	PrecedenceEntryName PrecedenceEntryKind = iota
	PrecedenceEntrySymbol
)

// PrecedenceEntry is an entry of a list in precedences: the name of a
// precedence, or the name of a symbol.
//
// PrecedenceEntry is PrecedenceEntry, an enum with data upstream.
type PrecedenceEntry struct {
	Kind  PrecedenceEntryKind
	Value StrID
}

// InputGrammar is the grammar as grammar.json gives it, after the rules that
// nothing uses are dropped.
//
// InputGrammar is InputGrammar.
type InputGrammar struct {
	Pool                *RulePool
	Name                StrID
	Variables           []Variable
	ExternalRoots       []RuleID
	ExtraRoots          []RuleID
	ReservedSets        []ReservedWordContext
	SupertypeNames      []StrID
	ConflictNames       [][]StrID
	InlineNames         []StrID
	WordName            StrID // zero when the grammar has no word
	PrecedenceOrderings [][]PrecedenceEntry
}

// ReservedWordContext is a named set of reserved words.
//
// ReservedWordContext is ReservedWordContext.
type ReservedWordContext struct {
	Name  StrID
	Roots []RuleID
}

// LexicalVariable is a token of the lexical grammar.
//
// LexicalVariable is LexicalVariable.
type LexicalVariable struct {
	Name               StrID
	Kind               VariableType
	ImplicitPrecedence int32
	StartState         uint32
}

// ProductionStep is one step of a production, packed as upstream packs it.
//
// The bits of Flags are:
//
//   - 0 to 2: the kind of the symbol, a SymbolType.
//   - 3 and 4: the kind of the precedence: none 00, a number 01, a name 10.
//   - 5 and 6: the associativity: none 00, left 01, right 10.
//   - 7: whether the alias is named.
//
// ProductionStep is ProductionStep.
type ProductionStep struct {
	SymIndex uint32
	PrecVal  int32
	Alias    uint32
	Field    uint32
	Reserved uint16
	Flags    uint8
}

// The bits of ProductionStep.Flags.
const (
	stepKindMask    uint8 = 0b0000_0111
	stepPrecInteger uint8 = 0b0000_1000
	stepPrecName    uint8 = 0b0001_0000
	stepPrecMask    uint8 = 0b0001_1000
	stepAssocLeft   uint8 = 0b0010_0000
	stepAssocRight  uint8 = 0b0100_0000
	stepAssocMask   uint8 = 0b0110_0000
	stepAliasNamed  uint8 = 0b1000_0000
)

// NoReservedWords is the value of ProductionStep.Reserved that means no set of
// reserved words at all. Only the start production that the generator adds
// has it, and it never indexes the table of reserved word sets.
//
// NoReservedWords is ProductionStep::NO_RESERVED_WORDS.
const NoReservedWords uint16 = 1<<16 - 1

// PackProductionStep packs the parts of a production step. A zero field and a
// zero alias mean none.
//
// PackProductionStep is ProductionStep::pack.
func PackProductionStep(sym Symbol, prec Precedence, assoc Associativity, alias Alias, hasAlias bool, field StrID, reserved uint16) ProductionStep {
	s := ProductionStep{SymIndex: sym.index, Reserved: reserved, Flags: uint8(sym.kind)}
	if sym.kind == SymbolEnd || sym.kind == SymbolEndOfNonTerminalExtra {
		s.SymIndex = 0
	}
	s.SetPrecedence(prec)
	s.SetAssociativity(assoc)
	s.SetAlias(alias, hasAlias)
	s.SetField(field)
	return s
}

// Symbol returns the symbol of the step.
//
// Symbol is ProductionStep::symbol.
func (s ProductionStep) Symbol() Symbol {
	switch s.Flags & stepKindMask {
	case 0:
		return ExternalTokenIndex(s.SymIndex).Symbol()
	case 1:
		return SymbolEndValue
	case 2:
		return SymbolEndOfNonTerminalExtraValue
	case 3:
		return TerminalIndex(s.SymIndex).Symbol()
	}
	return NonTerminalIndex(s.SymIndex).Symbol()
}

// NonTerminalIndex returns the index of the symbol, when it is a non-terminal.
//
// NonTerminalIndex is ProductionStep::non_terminal_index.
func (s ProductionStep) NonTerminalIndex() (NonTerminalIndex, bool) {
	return NonTerminalIndex(s.SymIndex), s.Flags&stepKindMask == uint8(SymbolNonTerminal)
}

// Precedence returns the precedence of the step.
//
// Precedence is ProductionStep::precedence.
func (s ProductionStep) Precedence() Precedence {
	switch s.Flags & stepPrecMask {
	case stepPrecInteger:
		return Precedence{Kind: PrecedenceInteger, Integer: s.PrecVal}
	case stepPrecName:
		return Precedence{Kind: PrecedenceName, Name: StrIDFromRaw(uint32(s.PrecVal))}
	}
	return Precedence{}
}

// SetPrecedence sets the precedence of the step.
//
// SetPrecedence is ProductionStep::set_precedence.
func (s *ProductionStep) SetPrecedence(prec Precedence) {
	var bits uint8
	var val int32
	switch prec.Kind {
	case PrecedenceInteger:
		bits, val = stepPrecInteger, prec.Integer
	case PrecedenceName:
		bits, val = stepPrecName, int32(prec.Name.Raw())
	}
	s.PrecVal = val
	s.Flags = s.Flags&^stepPrecMask | bits
}

// Associativity returns the associativity of the step.
//
// Associativity is ProductionStep::associativity.
func (s ProductionStep) Associativity() Associativity {
	switch s.Flags & stepAssocMask {
	case stepAssocLeft:
		return AssociativityLeft
	case stepAssocRight:
		return AssociativityRight
	}
	return AssociativityNone
}

// SetAssociativity sets the associativity of the step.
//
// SetAssociativity is ProductionStep::set_associativity.
func (s *ProductionStep) SetAssociativity(assoc Associativity) {
	var bits uint8
	switch assoc {
	case AssociativityLeft:
		bits = stepAssocLeft
	case AssociativityRight:
		bits = stepAssocRight
	}
	s.Flags = s.Flags&^stepAssocMask | bits
}

// GetAlias returns the alias of the step, and false when it has none.
//
// GetAlias is ProductionStep::alias. The name differs because Alias is the
// field that holds the packed id.
func (s ProductionStep) GetAlias() (Alias, bool) {
	if s.Alias == 0 {
		return Alias{}, false
	}
	return Alias{Value: StrIDFromRaw(s.Alias), IsNamed: s.Flags&stepAliasNamed != 0}, true
}

// SetAlias sets the alias of the step, or removes it when ok is false.
//
// SetAlias is ProductionStep::set_alias.
func (s *ProductionStep) SetAlias(alias Alias, ok bool) {
	s.Alias = 0
	if ok {
		s.Alias = alias.Value.Raw()
	}
	s.SetAliasNamed(ok && alias.IsNamed)
}

// SetAliasNamed sets whether the alias of the step is named.
//
// SetAliasNamed is ProductionStep::set_alias_named.
func (s *ProductionStep) SetAliasNamed(named bool) {
	s.Flags &^= stepAliasNamed
	if named {
		s.Flags |= stepAliasNamed
	}
}

// GetField returns the field name of the step, and false when it has none.
//
// GetField is ProductionStep::field. The name differs because Field is the
// field that holds the packed id.
func (s ProductionStep) GetField() (StrID, bool) {
	return StrID(s.Field), s.Field != 0
}

// SetField sets the field name of the step. A zero id removes it.
//
// SetField is ProductionStep::set_field.
func (s *ProductionStep) SetField(field StrID) {
	s.Field = field.Raw()
}

// ReservedWordSetID is the id of a set of reserved words.
//
// ReservedWordSetID is ReservedWordSetId.
type ReservedWordSetID uint32

// Production is a flat production: a range of steps, its dynamic precedence,
// and whether its reduction waits for the end of the input.
//
// Production is Production.
type Production struct {
	StepsStart        uint32
	StepsLen          uint32
	DynamicPrecedence int32
	// RequiresEOFLookahead is true when the production ends in eof(). Its
	// reduction happens only on the end of the input, and never as a shift.
	RequiresEOFLookahead bool
}

// StepRange returns the range of the steps of the production.
//
// StepRange is Production::step_range.
func (p Production) StepRange() (start, end int) {
	return int(p.StepsStart), int(p.StepsStart + p.StepsLen)
}

// ProductionStore is the flat output of flattening: one store of steps, each
// production as a range in it, and the productions of each variable.
//
// ProductionStore is ProductionStore.
type ProductionStore struct {
	Steps       []ProductionStep
	Productions []Production
	VarProds    [][2]uint32
}

// InlinedProductionMap maps a production and a step to the productions that
// inline the symbol of the step.
//
// InlinedProductionMap is InlinedProductionMap. Upstream keys an FxHashMap
// with the pair, and only looks keys up.
type InlinedProductionMap struct {
	Map map[[2]uint32][]uint32
}

// InlinedProdIDs returns the productions that inline the symbol at a step.
//
// InlinedProdIDs is InlinedProductionMap::inlined_prod_ids.
func (m *InlinedProductionMap) InlinedProdIDs(prodID, stepIndex uint32) ([]uint32, bool) {
	ids, ok := m.Map[[2]uint32{prodID, stepIndex}]
	return ids, ok
}

// SyntaxVariable is a non-terminal of the syntax grammar.
//
// SyntaxVariable is SyntaxVariable.
type SyntaxVariable struct {
	Name StrID
	Kind VariableType
}

// IsAuxiliary reports whether the generator made the variable.
//
// IsAuxiliary is SyntaxVariable::is_auxiliary.
func (v SyntaxVariable) IsAuxiliary() bool {
	return v.Kind == VariableAuxiliary
}

// IsHidden reports whether the variable is hidden.
//
// IsHidden is SyntaxVariable::is_hidden.
func (v SyntaxVariable) IsHidden() bool {
	return v.Kind == VariableHidden || v.Kind == VariableAuxiliary
}

// ExternalToken is a token that the external scanner recognizes.
//
// ExternalToken is ExternalToken.
type ExternalToken struct {
	Name StrID
	Kind VariableType
	// CorrespondingInternalToken is the internal token of the same name,
	// when HasCorrespondingInternalToken is true.
	CorrespondingInternalToken    Symbol
	HasCorrespondingInternalToken bool
}

// SyntaxGrammar is the grammar of the parser, after the tokens are extracted
// and the rules are flattened.
//
// SyntaxGrammar is SyntaxGrammar.
type SyntaxGrammar struct {
	Variables           []SyntaxVariable
	ExtraSymbols        []Symbol
	ExpectedConflicts   [][]Symbol
	ExternalTokens      []ExternalToken
	SupertypeSymbols    []Symbol
	VariablesToInline   []Symbol
	WordToken           Symbol
	HasWordToken        bool
	PrecedenceOrderings [][]PrecedenceEntry
	ReservedWordSets    []TokenSet

	Steps       []ProductionStep
	Productions []Production
	VarProds    [][2]uint32
}

// Production returns a production by its id.
//
// Production is SyntaxGrammar::production.
func (g *SyntaxGrammar) Production(id uint32) ProdRef {
	p := g.Productions[id]
	start, end := p.StepRange()
	return ProdRef{
		Steps:                g.Steps[start:end],
		DynamicPrecedence:    p.DynamicPrecedence,
		RequiresEOFLookahead: p.RequiresEOFLookahead,
	}
}

// VariableProdIDs returns the range of the ids of the productions of a
// variable.
//
// VariableProdIDs is SyntaxGrammar::variable_prod_ids.
func (g *SyntaxGrammar) VariableProdIDs(variableIndex int) (start, end uint32) {
	r := g.VarProds[variableIndex]
	return r[0], r[1]
}

// ProdRef is a production in the store of a SyntaxGrammar.
//
// ProdRef is ProdRef.
type ProdRef struct {
	Steps                []ProductionStep
	DynamicPrecedence    int32
	RequiresEOFLookahead bool
}

// FirstSymbol returns the first symbol of the production, and false when the
// production is empty.
//
// FirstSymbol is ProdRef::first_symbol.
func (p ProdRef) FirstSymbol() (Symbol, bool) {
	if len(p.Steps) == 0 {
		return Symbol{}, false
	}
	return p.Steps[0].Symbol(), true
}
