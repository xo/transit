package generate

import (
	"cmp"
	"iter"
	"maps"
	"slices"
	"strings"
)

// This file ports crates/generate/src/node_types.rs: the summary of the
// structure of each node type, and node-types.json. It also holds
// ProductionStep::child_type of grammars.rs, because that method returns the
// ChildType of this module.
//
// Upstream keeps the fields of a VariableInfo, and some maps of its own, in
// an FxHashMap. The Go port uses a Go map in each place, and a comment at
// each loop over such a map says why the order of the loop cannot reach the
// output.

// ChildTypeKind is the kind of a ChildType.
type ChildTypeKind uint8

// The kinds of child type, in the order of upstream. A child type sorts by
// its kind first.
const (
	ChildTypeNormal ChildTypeKind = iota
	ChildTypeAliased
)

// ChildType is the type of a child of a node: a symbol, or an alias.
//
// Only the field of the kind is set, and the other one is zero, so two child
// types compare with ==. NormalChildType and AliasedChildType make one.
//
// ChildType is ChildType, an enum with data upstream.
type ChildType struct {
	Kind ChildTypeKind
	// Symbol is the symbol of a ChildTypeNormal.
	Symbol Symbol
	// Alias is the alias of a ChildTypeAliased.
	Alias Alias
}

// NormalChildType returns the child type of a symbol.
//
// NormalChildType is ChildType::Normal.
func NormalChildType(symbol Symbol) ChildType {
	return ChildType{Kind: ChildTypeNormal, Symbol: symbol}
}

// AliasedChildType returns the child type of an alias.
//
// AliasedChildType is ChildType::Aliased.
func AliasedChildType(alias Alias) ChildType {
	return ChildType{Kind: ChildTypeAliased, Alias: alias}
}

// CompareChildType orders child types as the derived Ord of upstream does: a
// symbol before an alias, then symbols by CompareSymbol, and aliases by their
// value and then by whether they are named.
//
// CompareChildType is the Ord of ChildType.
func CompareChildType(a, b ChildType) int {
	if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	if a.Kind == ChildTypeNormal {
		return CompareSymbol(a.Symbol, b.Symbol)
	}
	return compareAlias(a.Alias, b.Alias)
}

// compareAlias orders aliases by their value and then by whether they are
// named.
//
// compareAlias is the Ord of Alias.
func compareAlias(a, b Alias) int {
	if c := cmp.Compare(a.Value, b.Value); c != 0 {
		return c
	}
	return compareBool(a.IsNamed, b.IsNamed)
}

// ChildType returns the child type of the step: its own alias, else the
// default alias of its symbol, else its symbol.
//
// ChildType is ProductionStep::child_type in grammars.rs.
func (s ProductionStep) ChildType(defaultAliases AliasMap) ChildType {
	if alias, ok := s.GetAlias(); ok {
		return AliasedChildType(alias)
	}
	if alias, ok := defaultAliases[s.Symbol()]; ok {
		return AliasedChildType(alias)
	}
	return NormalChildType(s.Symbol())
}

// FieldInfo is the types and the quantity of the children of a field, or of
// a set of children.
//
// The zero FieldInfo has a zero quantity. The default FieldInfo of upstream
// has the quantity one, and newFieldInfo gives it.
//
// FieldInfo is FieldInfo.
type FieldInfo struct {
	Quantity ChildQuantity
	// Types is sorted by CompareChildType, and it has no duplicates.
	Types []ChildType
}

// newFieldInfo returns the default FieldInfo of upstream, with the quantity
// one and no types.
//
// newFieldInfo is FieldInfo::default.
func newFieldInfo() FieldInfo {
	return FieldInfo{Quantity: oneChildQuantity()}
}

// clone returns a copy of the info that shares no memory with it.
//
// clone is the Clone of FieldInfo.
func (f FieldInfo) clone() FieldInfo {
	return FieldInfo{Quantity: f.Quantity, Types: slices.Clone(f.Types)}
}

// VariableInfo is the summary of the public structure of one variable of
// the syntax grammar: its fields, its visible children, and its named
// children without a field.
//
// Fields is a Go map, and a loop over it has a random order. Upstream keeps
// it in an FxHashMap, and no caller of upstream lets the order of that map
// reach the output. A caller that needs an order must sort the keys.
//
// VariableInfo is VariableInfo.
type VariableInfo struct {
	Fields                 map[StrID]*FieldInfo
	Children               FieldInfo
	ChildrenWithoutFields  FieldInfo
	HasMultiStepProduction bool
}

// newVariableInfo returns the default VariableInfo of upstream.
//
// newVariableInfo is VariableInfo::default.
func newVariableInfo() VariableInfo {
	return VariableInfo{
		Fields:                map[StrID]*FieldInfo{},
		Children:              newFieldInfo(),
		ChildrenWithoutFields: newFieldInfo(),
	}
}

// clone returns a copy of the info that shares no memory with it.
//
// clone is the Clone of VariableInfo.
func (v VariableInfo) clone() VariableInfo {
	c := VariableInfo{
		Fields:                 make(map[StrID]*FieldInfo, len(v.Fields)),
		Children:               v.Children.clone(),
		ChildrenWithoutFields:  v.ChildrenWithoutFields.clone(),
		HasMultiStepProduction: v.HasMultiStepProduction,
	}
	for name, info := range v.Fields {
		fc := info.clone()
		c.Fields[name] = &fc
	}
	return c
}

// field returns the info of a field, and adds the default info when the
// variable does not have the field yet.
//
// field is `fields.entry(name).or_default()` upstream.
func (v *VariableInfo) field(name StrID) *FieldInfo {
	info, ok := v.Fields[name]
	if !ok {
		fi := newFieldInfo()
		info = &fi
		v.Fields[name] = info
	}
	return info
}

// nodeInfoJSON is one entry of node-types.json.
//
// A nil fields is the None of upstream, and a non-nil empty fields is
// Some of an empty map. A nil subtypes is None too. A supertype with no
// visible subtypes has a non-nil empty subtypes.
//
// nodeInfoJSON is NodeInfoJSON.
type nodeInfoJSON struct {
	kind     StrID
	named    bool
	root     bool
	extra    bool
	fields   map[StrID]*fieldInfoJSON
	children *fieldInfoJSON
	subtypes []nodeTypeRef
}

// nodeTypeRef is the identity of a node type: its name, and whether it is
// named.
//
// nodeTypeRef is NodeTypeRef.
type nodeTypeRef struct {
	kind  StrID
	named bool
}

// fieldInfoJSON is a field, or the children, of an entry of node-types.json.
//
// fieldInfoJSON is FieldInfoJSON.
type fieldInfoJSON struct {
	multiple bool
	required bool
	types    []nodeTypeRef
}

// newFieldInfoJSON returns the default fieldInfoJSON: required, not multiple
// and with no types.
//
// newFieldInfoJSON is FieldInfoJSON::default.
func newFieldInfoJSON() *fieldInfoJSON {
	return &fieldInfoJSON{required: true}
}

// ChildQuantity says whether a child exists, whether it is required, and
// whether it can repeat.
//
// The zero ChildQuantity is ChildQuantity::zero of upstream. The default of
// upstream is ChildQuantity::one.
//
// ChildQuantity is ChildQuantity.
type ChildQuantity struct {
	exists   bool
	required bool
	multiple bool
}

// zeroChildQuantity returns the quantity of no child.
//
// zeroChildQuantity is ChildQuantity::zero.
func zeroChildQuantity() ChildQuantity {
	return ChildQuantity{}
}

// oneChildQuantity returns the quantity of exactly one child.
//
// oneChildQuantity is ChildQuantity::one, and the Default of ChildQuantity.
func oneChildQuantity() ChildQuantity {
	return ChildQuantity{exists: true, required: true}
}

// append adds the quantity of children that follow in a sequence.
//
// append is ChildQuantity::append.
func (q *ChildQuantity) append(other ChildQuantity) {
	if other.exists {
		if q.exists || other.multiple {
			q.multiple = true
		}
		if other.required {
			q.required = true
		}
		q.exists = true
	}
}

// union adds the quantity of children of another production. It reports
// whether q changed.
//
// union is ChildQuantity::union.
func (q *ChildQuantity) union(other ChildQuantity) bool {
	result := false
	if !q.exists && other.exists {
		result = true
		q.exists = true
	}
	if q.required && !other.required {
		result = true
		q.required = false
	}
	if !q.multiple && other.multiple {
		result = true
		q.multiple = true
	}
	return result
}

// InvalidSupertypeError is the error of a supertype that can have more than
// one visible child.
//
// Upstream wraps it in VariableInfoError::InvalidSupertype, which is
// transparent, so GetVariableInfo returns it as it is.
//
// InvalidSupertypeError is InvalidSupertypeError.
type InvalidSupertypeError struct {
	Supertype string
	// Child is the name of a hidden child that can expand into several
	// nodes, or empty when there is none. A variable never has an empty
	// name, so empty stands for the None of upstream.
	Child string
}

// Error returns the text of the error.
//
// Error is the Display of InvalidSupertypeError.
func (e *InvalidSupertypeError) Error() string {
	s := "Supertypes must have a single visible child, but `" + e.Supertype + "` can have multiple."
	if e.Child != "" {
		s += " The hidden child `" + e.Child + "` can expand into multiple nodes. Consider making `" + e.Child + "` visible."
	}
	return s
}

// SupertypeAliasCollisionError is the error of a named alias that has the
// name of a supertype.
//
// SupertypeAliasCollisionError is VariableInfoError::SupertypeAliasCollision.
type SupertypeAliasCollisionError struct {
	Name string
}

// Error returns the text of the error.
func (e *SupertypeAliasCollisionError) Error() string {
	return "Named alias `" + e.Name + "` conflicts with a supertype of the same name."
}

// GetVariableInfo computes a summary of the public structure of each
// variable in the grammar. Each variable in the grammar is a distinct public
// node type. The result has one VariableInfo for each variable, by index.
//
// The information about each node type N is:
//
//  1. Children: the types of visible children that can appear in N.
//  2. Fields: the fields that N can have. For each field, the types of
//     visible children that the field can hold, whether N always has the
//     field, and whether N can have several children in the field.
//  3. ChildrenWithoutFields: the other named children of N, which have no
//     field. Their types, whether N always has at least one of them, and
//     whether N can have several of them.
//
// Each summary accounts for two indirect factors:
//
//  1. Hidden nodes. When a parent node N has a hidden child C, the visible
//     children of C seem to be direct children of N.
//  2. Aliases. When a parent node type M has the alias N, a node that seems
//     to have the type N can have the inner structure of M.
//
// The error is an *InvalidSupertypeError or a *SupertypeAliasCollisionError.
//
// GetVariableInfo is get_variable_info.
func GetVariableInfo(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap, strPool *StrPool) ([]VariableInfo, error) {
	if err := validateSupertypeAliases(syntaxGrammar, defaultAliases, strPool); err != nil {
		return nil, err
	}
	result := computeVariableInfoFixedPoint(syntaxGrammar, lexicalGrammar, defaultAliases)
	if err := validateSupertypeStructure(result, syntaxGrammar, lexicalGrammar, defaultAliases, strPool); err != nil {
		return nil, err
	}
	stripHiddenChildTypes(result, syntaxGrammar, lexicalGrammar)
	return result, nil
}

// validateSupertypeAliases rejects an alias that has the public identity of
// a supertype. The schema of node-types.json cannot hold one identity as
// both an abstract supertype and a concrete aliased node.
//
// validateSupertypeAliases is validate_supertype_aliases.
func validateSupertypeAliases(syntaxGrammar *SyntaxGrammar, defaultAliases AliasMap, strPool *StrPool) error {
	aliasesBySymbol := getAliasesBySymbol(syntaxGrammar, defaultAliases)
	for _, supertypeSymbol := range syntaxGrammar.SupertypeSymbols {
		index, ok := supertypeSymbol.NonTerminalIndex()
		if !ok {
			panic("unreachable")
		}
		supertype := syntaxGrammar.Variables[index]
		collision := optionalAlias{alias: Alias{Value: supertype.Name, IsNamed: true}, ok: true}
		// The order of this loop over a Go map cannot reach the output,
		// because the loop only asks whether any set holds the alias.
		for _, aliases := range aliasesBySymbol {
			if slices.Contains(aliases, collision) {
				return &SupertypeAliasCollisionError{Name: strPool.Resolve(supertype.Name)}
			}
		}
	}
	return nil
}

// computeVariableInfoFixedPoint computes the info of every syntax variable
// again and again until nothing changes. The summary of a variable can
// depend on the summaries of other hidden variables, and variables can refer
// to each other, so the loop runs until no more changes occur.
//
// computeVariableInfoFixedPoint is compute_variable_info_fixed_point.
func computeVariableInfoFixedPoint(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap) []VariableInfo {
	childTypeIsVisible := func(t ChildType) bool {
		return variableTypeForChildType(t, syntaxGrammar, lexicalGrammar) >= VariableAnonymous
	}
	childTypeIsNamed := func(t ChildType) bool {
		return variableTypeForChildType(t, syntaxGrammar, lexicalGrammar) == VariableNamed
	}

	didChange := true
	allInitialized := false
	result := make([]VariableInfo, len(syntaxGrammar.Variables))
	for i := range result {
		result[i] = newVariableInfo()
	}
	for didChange {
		didChange = false

		for i := range syntaxGrammar.Variables {
			variableInfo := result[i].clone()

			// Look at each production of the variable. The child types of
			// the variable combine across all productions at once, but the
			// child quantities are recorded for each production.
			start, end := syntaxGrammar.VariableProdIDs(i)
			for prodID := start; prodID < end; prodID++ {
				production := syntaxGrammar.Production(prodID)
				productionFieldQuantities := map[StrID]ChildQuantity{}
				productionChildrenQuantity := zeroChildQuantity()
				productionChildrenWithoutFieldsQuantity := zeroChildQuantity()
				productionHasUninitializedInvisibleChildren := false

				if len(production.Steps) > 1 {
					variableInfo.HasMultiStepProduction = true
				}

				for _, step := range production.Steps {
					childSymbol := step.Symbol()
					childType := step.ChildType(defaultAliases)

					childIsHidden := !childTypeIsVisible(childType) &&
						!slices.Contains(syntaxGrammar.SupertypeSymbols, childSymbol)

					// Keep the set of all child types of this variable, and
					// the quantity of visible children in this production.
					didChange = extendSorted(&variableInfo.Children.Types, childType) || didChange
					if !childIsHidden {
						productionChildrenQuantity.append(oneChildQuantity())
					}

					// Keep the set of child types of each field, and the
					// quantity of children of each field in this production.
					if fieldName, ok := step.GetField(); ok {
						fieldInfo := variableInfo.field(fieldName)
						didChange = extendSorted(&fieldInfo.Types, childType) || didChange

						productionFieldQuantity, ok := productionFieldQuantities[fieldName]
						if !ok {
							productionFieldQuantity = zeroChildQuantity()
						}

						// Inherit the types and quantities of hidden children
						// of fields.
						if index, ok := childSymbol.NonTerminalIndex(); childIsHidden && ok {
							childVariableInfo := &result[index]
							didChange = extendSorted(&fieldInfo.Types, childVariableInfo.Children.Types...) || didChange
							productionFieldQuantity.append(childVariableInfo.Children.Quantity)
						} else {
							productionFieldQuantity.append(oneChildQuantity())
						}
						productionFieldQuantities[fieldName] = productionFieldQuantity
					} else if childTypeIsNamed(childType) {
						// Keep the set of named children without fields in
						// this variable.
						productionChildrenWithoutFieldsQuantity.append(oneChildQuantity())
						didChange = extendSorted(&variableInfo.ChildrenWithoutFields.Types, childType) || didChange
					}

					// Inherit all child information from hidden children.
					if index, ok := childSymbol.NonTerminalIndex(); childIsHidden && ok {
						_, hasField := step.GetField()
						didChange = inheritHiddenChildInfo(
							&result[index],
							!hasField,
							&variableInfo,
							productionFieldQuantities,
							&productionChildrenQuantity,
							&productionChildrenWithoutFieldsQuantity,
						) || didChange
					}

					// Note whether this production has children whose
					// summaries are not computed yet.
					var childIndex uint32
					switch childSymbol.Kind() {
					case SymbolExternal, SymbolTerminal, SymbolNonTerminal:
						childIndex = childSymbol.index
					default:
						panic("unreachable")
					}
					if int(childIndex) >= i && !allInitialized {
						productionHasUninitializedInvisibleChildren = true
					}
				}

				// When the summaries of all the children of this production
				// are initialized, add the quantities that this production
				// allows.
				if !productionHasUninitializedInvisibleChildren {
					didChange = variableInfo.Children.Quantity.union(productionChildrenQuantity) || didChange

					didChange = variableInfo.ChildrenWithoutFields.Quantity.union(productionChildrenWithoutFieldsQuantity) || didChange

					// The order of this loop over a Go map cannot reach the
					// output, because each field changes only its own info.
					for fieldName, info := range variableInfo.Fields {
						q, ok := productionFieldQuantities[fieldName]
						if !ok {
							q = zeroChildQuantity()
						}
						didChange = info.Quantity.union(q) || didChange
					}
				}
			}

			result[i] = variableInfo
		}

		allInitialized = true
	}

	return result
}

// inheritHiddenChildInfo gives the parent the fields, the children and the
// children without fields of a hidden child variable. It reports whether
// anything changed.
//
// inheritHiddenChildInfo is inherit_hidden_child_info.
func inheritHiddenChildInfo(
	childVariableInfo *VariableInfo,
	stepHasNoField bool,
	variableInfo *VariableInfo,
	productionFieldQuantities map[StrID]ChildQuantity,
	productionChildrenQuantity *ChildQuantity,
	productionChildrenWithoutFieldsQuantity *ChildQuantity,
) bool {
	didChange := false

	// When a hidden child can have several children, its parent node can
	// seem to have several children.
	if childVariableInfo.HasMultiStepProduction {
		variableInfo.HasMultiStepProduction = true
	}

	// When a hidden child has fields, the parent node can seem to have the
	// same fields. The order of this loop over a Go map cannot reach the
	// output, because each field changes only its own entries.
	for fieldName, childFieldInfo := range childVariableInfo.Fields {
		q, ok := productionFieldQuantities[fieldName]
		if !ok {
			q = zeroChildQuantity()
		}
		q.append(childFieldInfo.Quantity)
		productionFieldQuantities[fieldName] = q
		didChange = extendSorted(&variableInfo.field(fieldName).Types, childFieldInfo.Types...) || didChange
	}

	// When a hidden child has children, the parent node can seem to have
	// the same children.
	productionChildrenQuantity.append(childVariableInfo.Children.Quantity)
	didChange = extendSorted(&variableInfo.Children.Types, childVariableInfo.Children.Types...) || didChange

	// When a hidden child can have named children without fields, the
	// parent node can seem to have the same children.
	if stepHasNoField {
		grandchildrenInfo := &childVariableInfo.ChildrenWithoutFields
		if len(grandchildrenInfo.Types) != 0 {
			productionChildrenWithoutFieldsQuantity.append(childVariableInfo.ChildrenWithoutFields.Quantity)
			didChange = extendSorted(&variableInfo.ChildrenWithoutFields.Types, childVariableInfo.ChildrenWithoutFields.Types...) || didChange
		}
	}

	return didChange
}

// validateSupertypeStructure makes sure that no supertype has a production
// of several steps. Such a production lets it have more than one visible
// child.
//
// validateSupertypeStructure is validate_supertype_structure.
func validateSupertypeStructure(result []VariableInfo, syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap, strPool *StrPool) error {
	childTypeIsVisible := func(t ChildType) bool {
		return variableTypeForChildType(t, syntaxGrammar, lexicalGrammar) >= VariableAnonymous
	}

	for _, supertypeSymbol := range syntaxGrammar.SupertypeSymbols {
		supertypeIndex, ok := supertypeSymbol.NonTerminalIndex()
		if !ok {
			panic("unreachable")
		}
		if !result[supertypeIndex].HasMultiStepProduction {
			continue
		}
		variable := syntaxGrammar.Variables[supertypeIndex]
		// A symbol can have a production of several steps directly, or
		// through an inlined anonymous child. In the second case, the error
		// can be more specific.
		hiddenChildName := ""
		start, end := syntaxGrammar.VariableProdIDs(int(supertypeIndex))
		for prodID := start; prodID < end; prodID++ {
			steps := syntaxGrammar.Production(prodID).Steps
			if len(steps) != 1 {
				continue
			}
			step := steps[0]
			childSymbol := step.Symbol()
			childType := step.ChildType(defaultAliases)
			childIsHidden := !childTypeIsVisible(childType) &&
				!slices.Contains(syntaxGrammar.SupertypeSymbols, childSymbol)
			index, ok := childSymbol.NonTerminalIndex()
			if ok && childIsHidden && result[index].HasMultiStepProduction {
				hiddenChildName = strPool.Resolve(syntaxGrammar.Variables[index].Name)
				break
			}
		}

		return &InvalidSupertypeError{
			Supertype: strPool.Resolve(variable.Name),
			Child:     hiddenChildName,
		}
	}
	return nil
}

// stripHiddenChildTypes removes the hidden child types from the children of
// the supertypes, from the types of the fields and from the children
// without fields. It drops a field that has no types left.
//
// stripHiddenChildTypes is strip_hidden_child_types.
func stripHiddenChildTypes(result []VariableInfo, syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar) {
	childTypeIsHidden := func(t ChildType) bool {
		return variableTypeForChildType(t, syntaxGrammar, lexicalGrammar) < VariableAnonymous
	}

	for _, supertypeSymbol := range syntaxGrammar.SupertypeSymbols {
		index, ok := supertypeSymbol.NonTerminalIndex()
		if !ok {
			panic("unreachable")
		}
		result[index].Children.Types = slices.DeleteFunc(result[index].Children.Types, childTypeIsHidden)
	}
	for i := range result {
		variableInfo := &result[i]
		// The order of this loop over a Go map cannot reach the output,
		// because each field changes only its own info.
		for name, fieldInfo := range variableInfo.Fields {
			fieldInfo.Types = slices.DeleteFunc(fieldInfo.Types, childTypeIsHidden)
			if len(fieldInfo.Types) == 0 {
				delete(variableInfo.Fields, name)
			}
		}
		variableInfo.ChildrenWithoutFields.Types = slices.DeleteFunc(variableInfo.ChildrenWithoutFields.Types, childTypeIsHidden)
	}
}

// optionalAlias is an alias, or none when ok is false. A none alias has a
// zero alias, so two of them compare with ==.
//
// optionalAlias is Option<Alias>.
type optionalAlias struct {
	alias Alias
	ok    bool
}

// compareOptionalAlias orders none before an alias, and aliases by
// compareAlias.
//
// compareOptionalAlias is the Ord of Option<Alias>.
func compareOptionalAlias(a, b optionalAlias) int {
	if c := compareBool(a.ok, b.ok); c != 0 {
		return c
	}
	return compareAlias(a.alias, b.alias)
}

// insertOptionalAlias adds an alias to a sorted set of aliases, when the set
// does not hold it yet.
//
// insertOptionalAlias is BTreeSet::insert.
func insertOptionalAlias(set []optionalAlias, alias optionalAlias) []optionalAlias {
	i, found := slices.BinarySearchFunc(set, alias, compareOptionalAlias)
	if found {
		return set
	}
	return slices.Insert(set, i, alias)
}

// getAliasesBySymbol returns every alias that each symbol appears with. A
// none alias in a set means that the symbol appears without an alias. Each
// set is sorted by compareOptionalAlias.
//
// getAliasesBySymbol is get_aliases_by_symbol. Upstream returns an
// FxHashMap of BTreeSets.
func getAliasesBySymbol(syntaxGrammar *SyntaxGrammar, defaultAliases AliasMap) map[Symbol][]optionalAlias {
	aliasesBySymbol := map[Symbol][]optionalAlias{}
	// The order of this loop over a Go map cannot reach the output, because
	// each symbol is a different key.
	for symbol, alias := range defaultAliases {
		aliasesBySymbol[symbol] = []optionalAlias{{alias: alias, ok: true}}
	}
	for _, extraSymbol := range syntaxGrammar.ExtraSymbols {
		if _, ok := defaultAliases[extraSymbol]; !ok {
			aliasesBySymbol[extraSymbol] = insertOptionalAlias(aliasesBySymbol[extraSymbol], optionalAlias{})
		}
	}
	for i := range syntaxGrammar.Variables {
		start, end := syntaxGrammar.VariableProdIDs(i)
		for prodID := start; prodID < end; prodID++ {
			for _, step := range syntaxGrammar.Production(prodID).Steps {
				var alias optionalAlias
				if a, ok := step.GetAlias(); ok {
					alias = optionalAlias{alias: a, ok: true}
				} else if a, ok := defaultAliases[step.Symbol()]; ok {
					alias = optionalAlias{alias: a, ok: true}
				}
				aliasesBySymbol[step.Symbol()] = insertOptionalAlias(aliasesBySymbol[step.Symbol()], alias)
			}
		}
	}
	aliasesBySymbol[NonTerminalSymbol(0)] = []optionalAlias{{}}
	return aliasesBySymbol
}

// SupertypeSymbolMap maps each supertype to the types of its visible
// children.
//
// SupertypeSymbolMap is the BTreeMap<Symbol, Vec<ChildType>> that
// get_supertype_symbol_map returns. It iterates in the order of the symbols,
// and Sorted gives that order.
type SupertypeSymbolMap map[Symbol][]ChildType

// Sorted returns each supertype and its child types, in the order of the
// symbols.
func (m SupertypeSymbolMap) Sorted() iter.Seq2[Symbol, []ChildType] {
	return func(yield func(Symbol, []ChildType) bool) {
		for _, sym := range slices.SortedFunc(maps.Keys(m), CompareSymbol) {
			if !yield(sym, m[sym]) {
				return
			}
		}
	}
}

// GetSupertypeSymbolMap returns the child types of each supertype.
//
// Upstream also builds a map from each alias to its symbols here, and never
// reads it. Go rejects a variable that is not read, and the map changes
// nothing, so the port leaves it out.
//
// GetSupertypeSymbolMap is get_supertype_symbol_map.
func GetSupertypeSymbolMap(syntaxGrammar *SyntaxGrammar, defaultAliases AliasMap, variableInfo []VariableInfo) SupertypeSymbolMap {
	supertypeSymbolMap := SupertypeSymbolMap{}
	for i, info := range variableInfo {
		symbol := NonTerminalSymbol(i)
		if slices.Contains(syntaxGrammar.SupertypeSymbols, symbol) {
			supertypeSymbolMap[symbol] = slices.Clone(info.Children.Types)
		}
	}
	return supertypeSymbolMap
}

// SuperTypeCycleError is the error of supertypes that contain each other in
// a cycle.
//
// SuperTypeCycleError is SuperTypeCycleError.
type SuperTypeCycleError struct {
	Items []string
}

// Error returns the text of the error.
//
// Error is the Display of SuperTypeCycleError.
func (e *SuperTypeCycleError) Error() string {
	var b strings.Builder
	b.WriteString("Dependency cycle detected in node types:")
	for i, item := range e.Items {
		b.WriteString(" " + item)
		if i < len(e.Items)-1 {
			b.WriteString(",")
		}
	}
	return b.String()
}

// NodeTypesJSON returns the text of node-types.json, as upstream
// writes it with serde_json::to_string_pretty. The text has no newline at
// its end, because the text of upstream has none.
//
// The error is a *SuperTypeCycleError.
//
// NodeTypesJSON is generate_node_types_json.
func NodeTypesJSON(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap, variableInfo []VariableInfo, strPool *StrPool) (string, error) {
	nodes, err := generateNodeTypes(syntaxGrammar, lexicalGrammar, defaultAliases, variableInfo, strPool)
	if err != nil {
		return "", err
	}
	var w prettyJSON
	w.writeNodes(nodes, strPool)
	return w.b.String(), nil
}

// generateNodeTypes returns the entries of node-types.json, in their order.
//
// generateNodeTypes is generate_node_types.
func generateNodeTypes(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap, variableInfo []VariableInfo, strPool *StrPool) ([]nodeInfoJSON, error) {
	aliasesBySymbol := getAliasesBySymbol(syntaxGrammar, defaultAliases)
	extraNodeTypes := collectExtraNodeTypes(syntaxGrammar, lexicalGrammar, aliasesBySymbol)

	// Upstream keeps the entries in a BTreeMap. The Go map is enough,
	// because the sort at the end gives the order, and no two entries
	// compare equal there.
	nodeTypesJSON := map[nodeTypeRef]*nodeInfoJSON{}
	subtypeMap := buildSupertypeEntries(
		nodeTypesJSON,
		syntaxGrammar,
		lexicalGrammar,
		defaultAliases,
		variableInfo,
		strPool,
		extraNodeTypes,
	)
	buildRegularEntries(
		nodeTypesJSON,
		syntaxGrammar,
		lexicalGrammar,
		defaultAliases,
		variableInfo,
		strPool,
		aliasesBySymbol,
		extraNodeTypes,
	)

	if err := sortSubtypeMapTopologically(subtypeMap, strPool); err != nil {
		return nil, err
	}
	applySupertypeCollapsing(nodeTypesJSON, subtypeMap)

	buildTokenEntries(
		nodeTypesJSON,
		syntaxGrammar,
		lexicalGrammar,
		aliasesBySymbol,
		extraNodeTypes,
	)

	result := make([]nodeInfoJSON, 0, len(nodeTypesJSON))
	for _, node := range nodeTypesJSON {
		result = append(result, *node)
	}
	// Upstream sorts with sort_unstable_by. No two entries compare equal,
	// because the name and the namedness of an entry are its key, and two
	// ids of the pool never have the same string. So the order is the same
	// in Go.
	slices.SortFunc(result, func(a, b nodeInfoJSON) int {
		if c := compareBool(b.subtypes != nil, a.subtypes != nil); c != 0 {
			return c
		}
		aIsLeaf := a.children == nil && a.fields == nil
		bIsLeaf := b.children == nil && b.fields == nil
		if c := compareBool(aIsLeaf, bIsLeaf); c != 0 {
			return c
		}
		if c := cmpStrIDs(a.kind, b.kind, strPool); c != 0 {
			return c
		}
		return compareBool(a.named, b.named)
	})
	return result, nil
}

// childTypeToNodeType returns the identity of a child type in
// node-types.json, with its alias resolved.
//
// childTypeToNodeType is child_type_to_node_type.
func childTypeToNodeType(childType ChildType, syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap) nodeTypeRef {
	if childType.Kind == ChildTypeAliased {
		return nodeTypeRef{kind: childType.Alias.Value, named: childType.Alias.IsNamed}
	}
	symbol := childType.Symbol
	if alias, ok := defaultAliases[symbol]; ok {
		return nodeTypeRef{kind: alias.Value, named: alias.IsNamed}
	}
	switch symbol.Kind() {
	case SymbolNonTerminal:
		variable := syntaxGrammar.Variables[symbol.index]
		return nodeTypeRef{kind: variable.Name, named: variable.Kind != VariableAnonymous}
	case SymbolTerminal:
		variable := lexicalGrammar.Variables[symbol.index]
		return nodeTypeRef{kind: variable.Name, named: variable.Kind != VariableAnonymous}
	case SymbolExternal:
		variable := syntaxGrammar.ExternalTokens[symbol.index]
		return nodeTypeRef{kind: variable.Name, named: variable.Kind != VariableAnonymous}
	}
	panic("Unexpected symbol type")
}

// populateFieldInfoJSON merges the info of a field into its entry. A field
// with no types is absent from this rule, so it cannot be required.
//
// populateFieldInfoJSON is populate_field_info_json.
func populateFieldInfoJSON(json *fieldInfoJSON, info *FieldInfo, syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, defaultAliases AliasMap, strPool *StrPool) {
	if len(info.Types) == 0 {
		json.required = false
		return
	}
	json.multiple = json.multiple || info.Quantity.multiple
	json.required = json.required && info.Quantity.required
	for _, t := range info.Types {
		json.types = append(json.types, childTypeToNodeType(t, syntaxGrammar, lexicalGrammar, defaultAliases))
	}
	sortNodeTypeRefs(json.types, strPool)
	json.types = slices.Compact(json.types)
}

// collectExtraNodeTypes returns every node identity that an extra symbol
// can appear as, with its aliases.
//
// collectExtraNodeTypes is collect_extra_node_types. Upstream returns an
// FxHashSet, and only asks whether it holds an identity.
func collectExtraNodeTypes(syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar, aliasesBySymbol map[Symbol][]optionalAlias) map[nodeTypeRef]bool {
	result := map[nodeTypeRef]bool{}
	for _, symbol := range syntaxGrammar.ExtraSymbols {
		for _, alias := range aliasesBySymbol[symbol] {
			var kind StrID
			var variableType VariableType
			if alias.ok {
				kind, variableType = alias.alias.Value, alias.alias.Kind()
			} else {
				switch symbol.Kind() {
				case SymbolNonTerminal:
					variable := syntaxGrammar.Variables[symbol.index]
					kind, variableType = variable.Name, variable.Kind
				case SymbolTerminal:
					variable := lexicalGrammar.Variables[symbol.index]
					kind, variableType = variable.Name, variable.Kind
				case SymbolExternal:
					variable := syntaxGrammar.ExternalTokens[symbol.index]
					kind, variableType = variable.Name, variable.Kind
				default:
					// The expansion of the lexical separators rejects eof()
					// in an extra, so SymbolEnd cannot be an extra symbol.
					// build_parse_table adds SymbolEndOfNonTerminalExtra
					// after this pass runs.
					panic("unreachable")
				}
			}
			result[nodeTypeRef{kind: kind, named: variableType != VariableAnonymous}] = true
		}
	}
	return result
}

// subtypeEntry is a supertype and its subtypes.
//
// subtypeEntry is the (NodeTypeRef, Vec<NodeTypeRef>) of the subtype map.
type subtypeEntry struct {
	supertype nodeTypeRef
	subtypes  []nodeTypeRef
}

// buildSupertypeEntries adds one entry for each supertype, and returns the
// map from each supertype to its subtypes.
//
// buildSupertypeEntries is build_supertype_entries.
func buildSupertypeEntries(
	nodeTypesJSON map[nodeTypeRef]*nodeInfoJSON,
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	defaultAliases AliasMap,
	variableInfo []VariableInfo,
	strPool *StrPool,
	extraNodeTypes map[nodeTypeRef]bool,
) []subtypeEntry {
	var subtypeMap []subtypeEntry
	for i, info := range variableInfo {
		symbol := NonTerminalSymbol(i)
		if !slices.Contains(syntaxGrammar.SupertypeSymbols, symbol) {
			continue
		}
		variable := syntaxGrammar.Variables[i]
		nodeType := nodeTypeRef{kind: variable.Name, named: true}
		nodeTypeJSON, ok := nodeTypesJSON[nodeType]
		if !ok {
			nodeTypeJSON = &nodeInfoJSON{
				kind:  variable.Name,
				named: true,
				extra: extraNodeTypes[nodeType],
			}
			nodeTypesJSON[nodeType] = nodeTypeJSON
		}
		subtypes := make([]nodeTypeRef, 0, len(info.Children.Types))
		for _, t := range info.Children.Types {
			subtypes = append(subtypes, childTypeToNodeType(t, syntaxGrammar, lexicalGrammar, defaultAliases))
		}
		sortNodeTypeRefs(subtypes, strPool)
		subtypes = slices.Compact(subtypes)
		supertype := nodeTypeRef{kind: nodeTypeJSON.kind, named: true}

		// Add to the subtype map only when there are visible subtypes. A
		// supertype can have no subtypes when all its children are hidden,
		// for example when it wraps a hidden external token.
		if len(subtypes) != 0 {
			subtypeMap = append(subtypeMap, subtypeEntry{supertype: supertype, subtypes: slices.Clone(subtypes)})
		}
		nodeTypeJSON.subtypes = subtypes
	}
	return subtypeMap
}

// buildRegularEntries adds the entries of the visible rules that are not
// supertypes, and of the aliases of supertypes, which are regular concrete
// nodes. The info of a rule merges into every regular name that it can
// appear as.
//
// buildRegularEntries is build_regular_entries.
func buildRegularEntries(
	nodeTypesJSON map[nodeTypeRef]*nodeInfoJSON,
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	defaultAliases AliasMap,
	variableInfo []VariableInfo,
	strPool *StrPool,
	aliasesBySymbol map[Symbol][]optionalAlias,
	extraNodeTypes map[nodeTypeRef]bool,
) {
	for i := range variableInfo {
		info := &variableInfo[i]
		symbol := NonTerminalSymbol(i)
		// An inlined symbol has no entry of its own.
		if slices.Contains(syntaxGrammar.VariablesToInline, symbol) {
			continue
		}
		isSupertype := slices.Contains(syntaxGrammar.SupertypeSymbols, symbol)
		variable := syntaxGrammar.Variables[i]

		// When a rule has several alias names, its info goes into several
		// entries.
		for _, alias := range aliasesBySymbol[symbol] {
			// The canonical supertype has its own entry with its subtypes.
			// An alias of that supertype is a regular visible node, and it
			// is handled here.
			if isSupertype && !alias.ok {
				continue
			}
			var kind StrID
			var isNamed bool
			switch {
			case alias.ok:
				kind, isNamed = alias.alias.Value, alias.alias.IsNamed
			case variable.Kind.IsVisible():
				kind, isNamed = variable.Name, variable.Kind == VariableNamed
			default:
				continue
			}

			// An entry with this identity can exist already, because
			// several rules can have aliases with the same name and
			// namedness.
			nodeTypeExisted := true
			nodeType := nodeTypeRef{kind: kind, named: isNamed}
			nodeTypeJSON, ok := nodeTypesJSON[nodeType]
			if !ok {
				nodeTypeExisted = false
				nodeTypeJSON = &nodeInfoJSON{
					kind:   kind,
					named:  isNamed,
					root:   i == 0,
					extra:  extraNodeTypes[nodeType],
					fields: map[StrID]*fieldInfoJSON{},
				}
				nodeTypesJSON[nodeType] = nodeTypeJSON
			}

			fieldsJSON := nodeTypeJSON.fields
			if fieldsJSON == nil {
				panic("called `Option::unwrap()` on a `None` value")
			}
			// The order of this loop over a Go map cannot reach the output,
			// because each field changes only its own entry.
			for newField, fieldInfo := range info.Fields {
				fieldJSON, ok := fieldsJSON[newField]
				if !ok {
					// When another rule has an alias with the same identity,
					// and does not have this field, the field cannot be
					// required.
					fieldJSON = newFieldInfoJSON()
					if nodeTypeExisted {
						fieldJSON.required = false
					}
					fieldsJSON[newField] = fieldJSON
				}
				populateFieldInfoJSON(fieldJSON, fieldInfo, syntaxGrammar, lexicalGrammar, defaultAliases, strPool)
			}

			// When another rule has an alias with the same identity, a field
			// that this rule does not have cannot be required.
			for existingField, fieldJSON := range fieldsJSON {
				if _, ok := info.Fields[existingField]; !ok {
					fieldJSON.required = false
				}
			}

			if nodeTypeJSON.children == nil {
				nodeTypeJSON.children = newFieldInfoJSON()
			}
			populateFieldInfoJSON(nodeTypeJSON.children, &info.ChildrenWithoutFields, syntaxGrammar, lexicalGrammar, defaultAliases, strPool)
		}
	}
}

// sortSubtypeMapTopologically sorts the subtype map so that a subtype comes
// before its supertypes.
//
// sortSubtypeMapTopologically is sort_subtype_map_topologically.
func sortSubtypeMapTopologically(subtypeMap []subtypeEntry, strPool *StrPool) error {
	sortedNodeTypes := make([]nodeTypeRef, 0, len(subtypeMap))
	topSort := newTopologicalSort()
	for _, entry := range subtypeMap {
		for _, subtype := range entry.subtypes {
			topSort.addDependency(subtype, entry.supertype)
		}
	}
	for {
		nextNodeTypes := topSort.popAll()
		if len(nextNodeTypes) != 0 {
			sortNodeTypeRefs(nextNodeTypes, strPool)
			sortedNodeTypes = append(sortedNodeTypes, nextNodeTypes...)
			continue
		}
		if !topSort.isEmpty() {
			// Upstream takes the items out with the Iterator of
			// TopologicalSort, which pops an item that nothing depends on.
			// popAll just found no such item, so the list is always empty.
			// The port keeps that fault.
			var items []string
			for {
				nodeType, ok := topSort.pop()
				if !ok {
					break
				}
				items = append(items, strPool.Resolve(nodeType.kind))
			}
			slices.Sort(items)
			return &SuperTypeCycleError{Items: items}
		}
		break
	}
	slices.SortStableFunc(subtypeMap, func(a, b subtypeEntry) int {
		return cmp.Compare(slices.Index(sortedNodeTypes, a.supertype), slices.Index(sortedNodeTypes, b.supertype))
	})
	return nil
}

// applySupertypeCollapsing replaces the subtypes of a supertype with the
// supertype itself, in the types of the children and of the fields.
//
// applySupertypeCollapsing is apply_supertype_collapsing.
func applySupertypeCollapsing(nodeTypesJSON map[nodeTypeRef]*nodeInfoJSON, subtypeMap []subtypeEntry) {
	// The order of this loop over a Go map cannot reach the output, because
	// each entry changes only itself.
	for _, nodeTypeJSON := range nodeTypesJSON {
		if nodeTypeJSON.children != nil && len(nodeTypeJSON.children.types) == 0 {
			nodeTypeJSON.children = nil
		}

		if nodeTypeJSON.children != nil {
			processSupertypes(nodeTypeJSON.children, subtypeMap)
		}
		for _, fieldInfo := range nodeTypeJSON.fields {
			processSupertypes(fieldInfo, subtypeMap)
		}
	}
}

// buildTokenEntries adds the entries of the visible tokens.
//
// buildTokenEntries is build_token_entries.
func buildTokenEntries(
	nodeTypesJSON map[nodeTypeRef]*nodeInfoJSON,
	syntaxGrammar *SyntaxGrammar,
	lexicalGrammar *LexicalGrammar,
	aliasesBySymbol map[Symbol][]optionalAlias,
	extraNodeTypes map[nodeTypeRef]bool,
) {
	type token struct {
		name StrID
		kind VariableType
	}
	var tokens []token
	for i, variable := range lexicalGrammar.Variables {
		for _, alias := range aliasesBySymbol[TerminalSymbol(i)] {
			if alias.ok {
				tokens = append(tokens, token{alias.alias.Value, alias.alias.Kind()})
			} else {
				tokens = append(tokens, token{variable.Name, variable.Kind})
			}
		}
	}
	for i, t := range syntaxGrammar.ExternalTokens {
		for _, alias := range aliasesBySymbol[ExternalSymbol(i)] {
			if alias.ok {
				tokens = append(tokens, token{alias.alias.Value, alias.alias.Kind()})
			} else {
				tokens = append(tokens, token{t.Name, t.Kind})
			}
		}
	}

	for _, t := range tokens {
		switch t.kind {
		case VariableNamed, VariableAnonymous:
			named := t.kind == VariableNamed
			nodeType := nodeTypeRef{kind: t.name, named: named}
			if nodeTypeJSON, ok := nodeTypesJSON[nodeType]; ok {
				// This token is a leaf appearance of a node identity that
				// exists, so the children and the fields of the other
				// appearances are optional.
				if nodeTypeJSON.children != nil {
					nodeTypeJSON.children.required = false
				}
				for _, field := range nodeTypeJSON.fields {
					field.required = false
				}
			} else {
				nodeTypesJSON[nodeType] = &nodeInfoJSON{
					kind:  t.name,
					named: named,
					extra: extraNodeTypes[nodeType],
				}
			}
		}
	}
}

// processSupertypes removes the subtypes of each supertype that the types
// hold.
//
// processSupertypes is process_supertypes.
func processSupertypes(info *fieldInfoJSON, subtypeMap []subtypeEntry) {
	for _, entry := range subtypeMap {
		if slices.Contains(info.types, entry.supertype) {
			info.types = slices.DeleteFunc(info.types, func(t nodeTypeRef) bool {
				return slices.Contains(entry.subtypes, t)
			})
		}
	}
}

// variableTypeForChildType returns the visibility of a child type in
// node-types.json.
//
// An alias wins over everything. Then a supertype is always VariableNamed,
// an inlined rule is always VariableHidden, and everything else has the
// declared kind of its variable. A symbol is never both a supertype and
// inlined, because intern_symbols drops the supertype of such a rule.
//
// variableTypeForChildType is variable_type_for_child_type. Upstream checks
// the rule of the last paragraph with a debug_assert, which does not run in
// a release build, so the port does not check it.
func variableTypeForChildType(childType ChildType, syntaxGrammar *SyntaxGrammar, lexicalGrammar *LexicalGrammar) VariableType {
	if childType.Kind == ChildTypeAliased {
		return childType.Alias.Kind()
	}
	symbol := childType.Symbol
	isSupertype := slices.Contains(syntaxGrammar.SupertypeSymbols, symbol)
	isInline := slices.Contains(syntaxGrammar.VariablesToInline, symbol)
	switch {
	case isSupertype:
		return VariableNamed
	case isInline:
		return VariableHidden
	}
	switch symbol.Kind() {
	case SymbolNonTerminal:
		return syntaxGrammar.Variables[symbol.index].Kind
	case SymbolTerminal:
		return lexicalGrammar.Variables[symbol.index].Kind
	case SymbolExternal:
		return syntaxGrammar.ExternalTokens[symbol.index].Kind
	}
	return VariableHidden
}

// extendSorted adds each value to a sorted slice when the slice does not
// hold it yet. It reports whether it added any value.
//
// extendSorted is extend_sorted.
func extendSorted(vec *[]ChildType, values ...ChildType) bool {
	result := false
	for _, value := range values {
		if i, found := slices.BinarySearchFunc(*vec, value, CompareChildType); !found {
			*vec = slices.Insert(*vec, i, value)
			result = true
		}
	}
	return result
}

// cmpStrIDs orders two ids of the pool by their strings.
//
// cmpStrIDs is cmp_str_ids.
func cmpStrIDs(a, b StrID, strPool *StrPool) int {
	return strings.Compare(strPool.Resolve(a), strPool.Resolve(b))
}

// sortNodeTypeRefs sorts node identities by their names, and then by their
// namedness.
//
// sortNodeTypeRefs is sort_node_type_refs. Upstream sorts with
// sort_unstable_by. Two identities that compare equal have the same string
// and the same namedness, and so the same id, because the pool gives one id
// to each string. They are the same value, so the order is the same in Go.
func sortNodeTypeRefs(types []nodeTypeRef, strPool *StrPool) {
	slices.SortFunc(types, func(a, b nodeTypeRef) int {
		if c := cmpStrIDs(a.kind, b.kind, strPool); c != 0 {
			return c
		}
		return compareBool(a.named, b.named)
	})
}

// prettyJSON writes JSON as the PrettyFormatter of serde_json 1.0.151 does,
// with an indent of two spaces. It writes only the values that
// node-types.json holds.
//
// prettyJSON is serde_json::ser::PrettyFormatter.
type prettyJSON struct {
	b             strings.Builder
	currentIndent int
	hasValue      bool
}

// indent writes the indent of the current level.
//
// indent is indent in serde_json/src/ser.rs.
func (w *prettyJSON) indent() {
	for range w.currentIndent {
		w.b.WriteString("  ")
	}
}

// begin starts an array or an object.
//
// begin is PrettyFormatter::begin_array and PrettyFormatter::begin_object.
func (w *prettyJSON) begin(open byte) {
	w.currentIndent++
	w.hasValue = false
	w.b.WriteByte(open)
}

// end ends an array or an object.
//
// end is PrettyFormatter::end_array and PrettyFormatter::end_object.
func (w *prettyJSON) end(closer byte) {
	w.currentIndent--
	if w.hasValue {
		w.b.WriteByte('\n')
		w.indent()
	}
	w.b.WriteByte(closer)
}

// beginValue starts a value of an array, or a key of an object.
//
// beginValue is PrettyFormatter::begin_array_value and
// PrettyFormatter::begin_object_key.
func (w *prettyJSON) beginValue(first bool) {
	if first {
		w.b.WriteByte('\n')
	} else {
		w.b.WriteString(",\n")
	}
	w.indent()
}

// key writes the key of an entry of an object, and the colon after it.
//
// key is begin_object_key, the key and begin_object_value of
// PrettyFormatter.
func (w *prettyJSON) key(first bool, k string) {
	w.beginValue(first)
	w.writeString(k)
	w.b.WriteString(": ")
}

// endValue ends a value of an array or an object.
//
// endValue is PrettyFormatter::end_array_value and
// PrettyFormatter::end_object_value.
func (w *prettyJSON) endValue() {
	w.hasValue = true
}

// writeBool writes true or false.
//
// writeBool is Formatter::write_bool.
func (w *prettyJSON) writeBool(v bool) {
	if v {
		w.b.WriteString("true")
	} else {
		w.b.WriteString("false")
	}
}

// writeString writes a string with the escapes of serde_json. It escapes the
// quote, the backslash and the control characters below 0x20, and nothing
// else. The control characters with a short escape get it, and the others
// get \u00 and two lowercase hex digits.
//
// writeString is format_escaped_str in serde_json/src/ser.rs.
func (w *prettyJSON) writeString(s string) {
	const hex = "0123456789abcdef"
	w.b.WriteByte('"')
	start := 0
	for i := range len(s) {
		c := s[i]
		var esc string
		switch {
		case c == '"':
			esc = `\"`
		case c == '\\':
			esc = `\\`
		case c == '\b':
			esc = `\b`
		case c == '\t':
			esc = `\t`
		case c == '\n':
			esc = `\n`
		case c == '\f':
			esc = `\f`
		case c == '\r':
			esc = `\r`
		case c < 0x20:
			esc = `\u00` + string(hex[c>>4]) + string(hex[c&0xf])
		default:
			continue
		}
		w.b.WriteString(s[start:i])
		w.b.WriteString(esc)
		start = i + 1
	}
	w.b.WriteString(s[start:])
	w.b.WriteByte('"')
}

// writeNodes writes the entries of node-types.json as an array.
//
// writeNodes is the Serialize of SerializeWithPool<[NodeInfoJSON]>.
func (w *prettyJSON) writeNodes(nodes []nodeInfoJSON, strPool *StrPool) {
	w.begin('[')
	for i := range nodes {
		w.beginValue(i == 0)
		w.writeNode(&nodes[i], strPool)
		w.endValue()
	}
	w.end(']')
}

// writeNodeTypeRef writes a node identity as an object.
//
// writeNodeTypeRef is the Serialize of SerializeWithPool<NodeTypeRef>.
func (w *prettyJSON) writeNodeTypeRef(t nodeTypeRef, strPool *StrPool) {
	w.begin('{')
	w.key(true, "type")
	w.writeString(strPool.Resolve(t.kind))
	w.endValue()
	w.key(false, "named")
	w.writeBool(t.named)
	w.endValue()
	w.end('}')
}

// writeNodeTypeRefs writes node identities as an array.
//
// writeNodeTypeRefs is the Serialize of SerializeWithPool<[NodeTypeRef]>.
func (w *prettyJSON) writeNodeTypeRefs(types []nodeTypeRef, strPool *StrPool) {
	w.begin('[')
	for i, t := range types {
		w.beginValue(i == 0)
		w.writeNodeTypeRef(t, strPool)
		w.endValue()
	}
	w.end(']')
}

// writeFieldInfo writes a field, or the children, of an entry as an object.
//
// writeFieldInfo is the Serialize of SerializeWithPool<FieldInfoJSON>.
func (w *prettyJSON) writeFieldInfo(info *fieldInfoJSON, strPool *StrPool) {
	w.begin('{')
	w.key(true, "multiple")
	w.writeBool(info.multiple)
	w.endValue()
	w.key(false, "required")
	w.writeBool(info.required)
	w.endValue()
	w.key(false, "types")
	w.writeNodeTypeRefs(info.types, strPool)
	w.endValue()
	w.end('}')
}

// writeFields writes the fields of an entry as an object, sorted by their
// names. The names are unique, so the unstable sort of upstream gives the
// same order.
//
// writeFields is the Serialize of SerializeWithPool<NodeFields>.
func (w *prettyJSON) writeFields(fields map[StrID]*fieldInfoJSON, strPool *StrPool) {
	names := slices.SortedFunc(maps.Keys(fields), func(a, b StrID) int {
		return cmpStrIDs(a, b, strPool)
	})
	w.begin('{')
	for i, name := range names {
		w.key(i == 0, strPool.Resolve(name))
		w.writeFieldInfo(fields[name], strPool)
		w.endValue()
	}
	w.end('}')
}

// writeNode writes an entry of node-types.json as an object. It leaves out
// root and extra when they are false, and fields, children and subtypes
// when they are none.
//
// writeNode is the Serialize of SerializeWithPool<NodeInfoJSON>.
func (w *prettyJSON) writeNode(node *nodeInfoJSON, strPool *StrPool) {
	w.begin('{')
	w.key(true, "type")
	w.writeString(strPool.Resolve(node.kind))
	w.endValue()
	w.key(false, "named")
	w.writeBool(node.named)
	w.endValue()
	if node.root {
		w.key(false, "root")
		w.writeBool(node.root)
		w.endValue()
	}
	if node.extra {
		w.key(false, "extra")
		w.writeBool(node.extra)
		w.endValue()
	}
	if node.fields != nil {
		w.key(false, "fields")
		w.writeFields(node.fields, strPool)
		w.endValue()
	}
	if node.children != nil {
		w.key(false, "children")
		w.writeFieldInfo(node.children, strPool)
		w.endValue()
	}
	if node.subtypes != nil {
		w.key(false, "subtypes")
		w.writeNodeTypeRefs(node.subtypes, strPool)
		w.endValue()
	}
	w.end('}')
}

// topologicalSort sorts node identities so that each comes after the ones
// that it depends on.
//
// topologicalSort is TopologicalSort of the crate topological-sort 0.2.2.
// Upstream keeps the items in a std HashMap, whose order is random in each
// run. popAll returns the items in that order, and the only caller sorts
// them, so a Go map gives the same result.
type topologicalSort struct {
	top map[nodeTypeRef]*tsDependency
}

// tsDependency is the number of items that an item depends on, and the set
// of items that depend on it.
//
// tsDependency is Dependency of the crate topological-sort.
type tsDependency struct {
	numPrec int
	succ    map[nodeTypeRef]bool
}

// newTopologicalSort returns an empty sort.
//
// newTopologicalSort is TopologicalSort::new.
func newTopologicalSort() *topologicalSort {
	return &topologicalSort{top: map[nodeTypeRef]*tsDependency{}}
}

// isEmpty reports whether the sort holds no items.
//
// isEmpty is TopologicalSort::is_empty.
func (t *topologicalSort) isEmpty() bool {
	return len(t.top) == 0
}

// addDependency records that succ depends on prec. A dependency that is
// recorded already changes nothing.
//
// addDependency is TopologicalSort::add_dependency.
func (t *topologicalSort) addDependency(prec, succ nodeTypeRef) {
	if dep, ok := t.top[prec]; ok {
		if dep.succ[succ] {
			return
		}
		dep.succ[succ] = true
	} else {
		t.top[prec] = &tsDependency{succ: map[nodeTypeRef]bool{succ: true}}
	}

	if dep, ok := t.top[succ]; ok {
		dep.numPrec++
	} else {
		t.top[succ] = &tsDependency{numPrec: 1, succ: map[nodeTypeRef]bool{}}
	}
}

// pop removes an item that depends on nothing, and returns it. It returns
// false when there is no such item.
//
// pop is TopologicalSort::pop. Upstream picks the first such item in the
// order of its HashMap. The only caller calls pop when no such item exists.
func (t *topologicalSort) pop() (nodeTypeRef, bool) {
	for key, dep := range t.top {
		if dep.numPrec == 0 {
			t.remove(key)
			return key, true
		}
	}
	return nodeTypeRef{}, false
}

// popAll removes all the items that depend on nothing, and returns them in
// a random order.
//
// popAll is TopologicalSort::pop_all.
func (t *topologicalSort) popAll() []nodeTypeRef {
	var keys []nodeTypeRef
	for key, dep := range t.top {
		if dep.numPrec == 0 {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		t.remove(key)
	}
	return keys
}

// remove removes an item, and lowers the count of each item that depends on
// it.
//
// remove is TopologicalSort::remove.
func (t *topologicalSort) remove(prec nodeTypeRef) {
	dep, ok := t.top[prec]
	if !ok {
		return
	}
	delete(t.top, prec)
	for s := range dep.succ {
		if y, ok := t.top[s]; ok {
			y.numPrec--
		}
	}
}
