package golang

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/xo/transit/generate"
	"github.com/xo/transit/internal/abi"
)

// This file ports crates/generate/src/render.rs, as the C backend in
// generate/backend/c does, function by function. Where render.rs writes a C
// table, the port fills the same table of an abi.Language, with the numbers
// that the C enums give. Where render.rs writes a C lex function, the port
// builds an abi.LexTable with the same behavior. write.go then writes the
// tables as Go.
//
// Upstream keeps four FxHashMaps in the Generator: symbol_order, symbol_ids,
// alias_ids and symbol_map. The Generator only looks them up, and never
// iterates them, so the port uses Go maps. The FxHashMaps that are local to a
// method are iterated, and the method sorts the entries after that by a key
// that is unique. Each such method says so. The FxHashSet of assign_symbol_id
// is only looked up.
//
// The port leaves out add_header, add_includes and add_pragmas, which write
// C only, and add_external_token_enum, because the Go port of a scanner
// writes the enum of its tokens itself (docs/GRAMMAR.md).

// smallStateThreshold is the number of entries above which a state of the
// parse table uses the large representation, when the grammar has at least
// twice as many symbols.
//
// smallStateThreshold is SMALL_STATE_THRESHOLD. ABI_VERSION_MIN and
// ABI_VERSION_MAX are generate.ABIVersionMin and generate.ABIVersionMax.
const smallStateThreshold = 64

// abiVersionWithReservedWords is the first ABI version that has the sets of
// reserved words, the supertype map and the metadata of the grammar.
//
// abiVersionWithReservedWords is ABI_VERSION_WITH_RESERVED_WORDS.
const abiVersionWithReservedWords = 15

// RenderErrorKind is the kind of a RenderError.
type RenderErrorKind uint8

// The kinds of error. The first two are the kinds of upstream, in its order.
const (
	// RenderErrorParseTable is a parse table with too many actions.
	RenderErrorParseTable RenderErrorKind = iota
	// RenderErrorABI is an ABI version that the backend does not write.
	RenderErrorABI
	// RenderErrorCharacterSet is a large character set that has a range of
	// surrogates only. The C code reads past the end of such a set, and the
	// Go backend cannot write a lexer that does the same.
	RenderErrorCharacterSet
)

// RenderError is an error of the Go backend. The text of the first two kinds
// is the text of upstream.
//
// RenderError is RenderError, an enum with data upstream.
type RenderError struct {
	Kind RenderErrorKind
	// Value is the number of actions of the parse table, for
	// RenderErrorParseTable, and the ABI version, for RenderErrorABI.
	Value int
	// Name is the name of the character set, for RenderErrorCharacterSet.
	Name string
}

// Error returns the text of the error.
//
// Error is the Display of RenderError.
func (e *RenderError) Error() string {
	switch e.Kind {
	case RenderErrorParseTable:
		return "Parse table action count " + strconv.Itoa(e.Value) + " exceeds maximum value of " + strconv.Itoa(math.MaxUint16)
	case RenderErrorABI:
		return "This version of Tree-sitter can only generate parsers with ABI version " +
			strconv.Itoa(generate.ABIVersionMin) + " - " + strconv.Itoa(generate.ABIVersionMax) + ", not " + strconv.Itoa(e.Value)
	case RenderErrorCharacterSet:
		return "the character set " + e.Name + " has a range of surrogates only, which the Go backend cannot write"
	}
	return ""
}

// output is what the generator builds for one grammar: the tables, the
// constants of the symbols and the fields, the lex tables and the keywords.
// write.go writes it as Go.
type output struct {
	// name is the name of the grammar.
	name string
	// tables holds every table. Its function fields are nil.
	tables abi.Language
	// symbols and fields are the constants of the symbol enum and of the
	// field enum, in the order of the enums.
	symbols []constant
	fields  []constant
	// mainLex and keywordLex are the lex tables. keywordLex is nil when the
	// grammar has no word token.
	mainLex    *abi.LexTable
	keywordLex *abi.LexTable
	// hasExternalScanner is true when the grammar has external tokens.
	hasExternalScanner bool
	// keywords holds the names of the keywords, sorted, with no repeats.
	keywords []string
}

// constant is a Go constant of a symbol or a field: its Go name, its value,
// and the name of the symbol or the field in the grammar.
type constant struct {
	name    string
	value   uint16
	display string
}

// generator builds the output of a grammar.
//
// generator is Generator. Upstream owns the tables and the grammars, and the
// port holds pointers to the ones in the RenderInput, which it does not
// change.
type generator struct {
	out                            *output
	languageName                   string
	parseTable                     *generate.ParseTable[generate.ActionListID]
	mainLexTable                   *generate.LexTable
	keywordLexTable                *generate.LexTable
	largeCharacterSets             []generate.LargeCharacterSet
	largeCharacterSetInfo          []largeCharacterSetInfo
	largeStateCount                int
	syntaxGrammar                  *generate.SyntaxGrammar
	lexicalGrammar                 *generate.LexicalGrammar
	defaultAliases                 generate.AliasMap
	symbolOrder                    map[generate.Symbol]int
	symbolIDs                      map[generate.Symbol]string
	aliasIDs                       map[generate.Alias]string
	uniqueAliases                  []generate.Alias
	symbolMap                      map[generate.Symbol]generate.Symbol
	reservedWordSets               []generate.TokenSet
	reservedWordSetIDsByParseState []int
	fieldNames                     []generate.StrID
	supertypeSymbolMap             generate.SupertypeSymbolMap
	// supertypeMap is a BTreeMap upstream. sortedSupertypes gives its
	// order.
	supertypeMap map[string][]generate.ChildType
	abiVersion   int
	metadata     *generate.SemanticVersion
	strPool      *generate.StrPool

	// values maps each C identifier of a symbol, an alias and a field to the
	// number that its C enum gives it. The C code names a symbol by its
	// identifier, and the tables of the port hold the number.
	values map[string]uint16
	// setRanges holds the ranges of each large character set, for the
	// evaluation of set_contains.
	setRanges [][][2]int32
	// err is the first error of a lex table.
	err error
}

// largeCharacterSetInfo is the name of the constant of a large character
// set, and whether a lex state uses it.
//
// largeCharacterSetInfo is LargeCharacterSetInfo.
type largeCharacterSetInfo struct {
	constantName string
	isUsed       bool
}

// generate builds the output.
//
// generate is Generator::generate.
func (g *generator) generate() (*output, error) {
	g.init()
	g.addStats()
	g.addSymbolEnum()
	g.addSymbolNamesList()
	g.addUniqueSymbolMap()
	g.addSymbolMetadataList()

	if len(g.fieldNames) != 0 {
		g.addFieldNameEnum()
		g.addFieldNameNamesList()
		g.addFieldSequences()
	}

	if len(g.parseTable.ProductionInfos) != 0 {
		g.addAliasSequences()
	}

	g.addNonTerminalAliasMap()
	g.addPrimaryStateIDList()

	if g.abiVersion >= abiVersionWithReservedWords && len(g.supertypeMap) != 0 {
		g.addSupertypeMap()
	}

	g.out.mainLex = g.addLexFunction(g.mainLexTable)

	if g.syntaxGrammar.HasWordToken {
		g.out.keywordLex = g.addLexFunction(g.keywordLexTable)
	}

	// upstream writes the constants of the large character sets that the lex
	// functions use, and the port checks that it can evaluate each of them
	for ix := range g.largeCharacterSets {
		g.addCharacterSet(ix)
	}
	if g.err != nil {
		return nil, g.err
	}

	g.addLexModes()

	if g.abiVersion >= abiVersionWithReservedWords && len(g.reservedWordSets) > 1 {
		g.addReservedWordSets()
	}

	if err := g.addParseTable(); err != nil {
		return nil, err
	}

	if len(g.syntaxGrammar.ExternalTokens) != 0 {
		g.addExternalScannerSymbolMap()
		g.addExternalScannerStatesList()
	}

	g.addParserExport()
	g.addKeywords()

	return g.out, nil
}

// value returns the number of a C identifier.
func (g *generator) value(id string) uint16 {
	v, ok := g.values[id]
	if !ok {
		panic("golang: no value for the identifier " + id)
	}
	return v
}

// symbolValue returns the number of a symbol.
func (g *generator) symbolValue(symbol generate.Symbol) uint16 {
	return g.value(g.symbolIDs[symbol])
}

// init gives each symbol and alias its C identifier, maps each symbol to its
// public symbol, and finds the field names, the unique aliases, the sets of
// reserved words and the number of large states.
//
// init is Generator::init.
func (g *generator) init() {
	symbolIdentifiers := map[string]bool{}
	for i := range g.parseTable.Symbols {
		g.assignSymbolID(g.parseTable.Symbols[i], symbolIdentifiers)
	}
	g.symbolIDs[generate.SymbolEndOfNonTerminalExtraValue] = g.symbolIDs[generate.SymbolEndValue]

	g.symbolMap = map[generate.Symbol]generate.Symbol{}

	for _, symbol := range g.parseTable.Symbols {
		mapping := symbol

		// There can be multiple symbols in the grammar that have the same name and kind,
		// due to simple aliases. When that happens, ensure that they map to the same
		// public-facing symbol. If one of the symbols is not aliased, choose that one
		// to be the public-facing symbol. Otherwise, pick the symbol with the lowest
		// numeric value.
		if alias, ok := g.defaultAliases[symbol]; ok {
			kind := alias.Kind()
			for _, otherSymbol := range g.parseTable.Symbols {
				if otherAlias, ok := g.defaultAliases[otherSymbol]; ok {
					if generate.CompareSymbol(otherSymbol, mapping) < 0 && otherAlias == alias {
						mapping = otherSymbol
					}
				} else if name, otherKind := g.metadataForSymbol(otherSymbol); name == alias.Value && otherKind == kind {
					mapping = otherSymbol
					break
				}
			}
		} else if symbol.Kind() == generate.SymbolTerminal {
			// Two anonymous tokens with different flags but the same string value
			// should be represented with the same symbol in the public API. Examples:
			// * "<" and token(prec(1, "<"))
			// * "(" and token.immediate("(")
			name, kind := g.metadataForSymbol(symbol)
			for _, otherSymbol := range g.parseTable.Symbols {
				otherName, otherKind := g.metadataForSymbol(otherSymbol)
				if otherName == name && otherKind == kind {
					if mapped, ok := g.symbolMap[otherSymbol]; ok && mapped == symbol {
						break
					}
					mapping = otherSymbol
					break
				}
			}
		}

		g.symbolMap[symbol] = mapping
	}

	for i := range g.parseTable.ProductionInfos {
		productionInfo := &g.parseTable.ProductionInfos[i]
		// Build a list of all field names
		for fieldName := range productionInfo.FieldMap.Sorted() {
			target := g.strPool.Resolve(fieldName)
			ix, found := slices.BinarySearchFunc(g.fieldNames, target, func(sid generate.StrID, target string) int {
				return cmp.Compare(g.strPool.Resolve(sid), target)
			})
			if !found {
				g.fieldNames = slices.Insert(g.fieldNames, ix, fieldName)
			}
		}

		// Generate a mapping from aliases to C identifiers.
		for _, alias := range productionInfo.AliasSequence {
			// a zero alias is the None of upstream, which flatten skips
			if alias == (generate.Alias{}) {
				continue
			}
			var aliasID string
			// Some aliases match an existing symbol in the grammar.
			if existing := g.symbolsForAlias(alias); len(existing) != 0 {
				aliasID = g.symbolIDs[g.symbolMap[existing[0]]]
			} else {
				// Other aliases don't match any existing symbol, and need their own
				// identifiers.
				value := g.strPool.Resolve(alias.Value)
				ix, found := slices.BinarySearchFunc(g.uniqueAliases, alias, func(candidate, alias generate.Alias) int {
					return cmp.Or(
						cmp.Compare(g.strPool.Resolve(candidate.Value), value),
						compareBool(candidate.IsNamed, alias.IsNamed),
					)
				})
				if !found {
					g.uniqueAliases = slices.Insert(g.uniqueAliases, ix, alias)
				}

				if alias.IsNamed {
					aliasID = "alias_sym_" + g.sanitizeIdentifier(alias.Value)
				} else {
					aliasID = "anon_alias_sym_" + g.sanitizeIdentifier(alias.Value)
				}
			}

			if _, ok := g.aliasIDs[alias]; !ok {
				g.aliasIDs[alias] = aliasID
			}
		}
	}

	for ix, set := range g.largeCharacterSets {
		count := 1
		for _, other := range g.largeCharacterSets[:ix] {
			if other.HasSymbol == set.HasSymbol && (!set.HasSymbol || other.Symbol == set.Symbol) {
				count++
			}
		}
		var constantName string
		if set.HasSymbol {
			constantName = g.symbolIDs[set.Symbol] + "_character_set_" + strconv.Itoa(count)
		} else {
			constantName = "extras_character_set_" + strconv.Itoa(count)
		}
		g.largeCharacterSetInfo = append(g.largeCharacterSetInfo, largeCharacterSetInfo{
			constantName: constantName,
			isUsed:       false,
		})
	}

	// Assign an id to each unique reserved word set
	g.reservedWordSets = append(g.reservedWordSets, generate.TokenSet{})
	for i := range g.parseTable.States {
		state := &g.parseTable.States[i]
		id := slices.IndexFunc(g.reservedWordSets, func(set generate.TokenSet) bool {
			return set.Equal(&state.ReservedWords)
		})
		if id < 0 {
			g.reservedWordSets = append(g.reservedWordSets, state.ReservedWords.Clone())
			id = len(g.reservedWordSets) - 1
		}
		g.reservedWordSetIDsByParseState = append(g.reservedWordSetIDsByParseState, id)
	}

	if g.abiVersion >= abiVersionWithReservedWords {
		for supertype, subtypes := range g.supertypeSymbolMap.Sorted() {
			if supertype, ok := g.symbolIDs[supertype]; ok {
				if _, ok := g.supertypeMap[supertype]; !ok {
					g.supertypeMap[supertype] = slices.Clone(subtypes)
				}
			}
		}

		g.supertypeSymbolMap = nil
	}

	// Determine which states should use the "small state" representation, and which should
	// use the normal array representation.
	threshold := min(smallStateThreshold, len(g.parseTable.Symbols)/2)
	g.largeStateCount = 0
	for i := range g.parseTable.States {
		s := &g.parseTable.States[i]
		if i > 1 && s.TerminalEntries.Len()+s.NonterminalEntries.Len() <= threshold {
			break
		}
		g.largeStateCount++
	}
}

// addStats sets the counts of the tables.
//
// addStats is Generator::add_stats.
func (g *generator) addStats() {
	tokenCount := 0
	for _, symbol := range g.parseTable.Symbols {
		switch symbol.Kind() {
		case generate.SymbolTerminal, generate.SymbolEnd:
			tokenCount++
		case generate.SymbolExternal:
			index, _ := symbol.ExternalIndex()
			if !g.syntaxGrammar.ExternalTokens[index].HasCorrespondingInternalToken {
				tokenCount++
			}
		case generate.SymbolEndOfNonTerminalExtra, generate.SymbolNonTerminal:
		}
	}

	t := &g.out.tables
	t.ABIVersion = uint32(g.abiVersion)
	t.StateCount = uint32(len(g.parseTable.States))
	t.LargeStateCount = uint32(g.largeStateCount)

	t.SymbolCount = uint32(len(g.parseTable.Symbols))
	t.AliasCount = uint32(len(g.uniqueAliases))
	t.TokenCount = uint32(tokenCount)
	t.ExternalTokenCount = uint32(len(g.syntaxGrammar.ExternalTokens))
	t.FieldCount = uint32(len(g.fieldNames))
	t.MaxAliasSequenceLength = uint16(g.parseTable.MaxAliasedProductionLength)

	t.ProductionIDCount = uint32(len(g.parseTable.ProductionInfos))
}

// maxReservedWordSetSize returns the size of the largest set of reserved
// words. The list always holds the empty set.
//
// maxReservedWordSetSize is the expression
// `self.reserved_word_sets.iter().map(TokenSet::len).max().unwrap()`, which
// upstream writes twice.
func (g *generator) maxReservedWordSetSize() int {
	size := g.reservedWordSets[0].Len()
	for i := range g.reservedWordSets {
		size = max(size, g.reservedWordSets[i].Len())
	}
	return size
}

// addSymbolEnum gives each symbol its number and its order, and makes the
// Go constant of each symbol and alias.
//
// addSymbolEnum is Generator::add_symbol_enum.
func (g *generator) addSymbolEnum() {
	names := newGoNames()
	g.symbolOrder[generate.SymbolEndValue] = 0
	g.values[g.symbolIDs[generate.SymbolEndValue]] = 0
	g.out.symbols = append(g.out.symbols, constant{
		name:    names.name(g.symbolIDs[generate.SymbolEndValue]),
		display: g.symbolName(generate.SymbolEndValue),
	})
	i := 1
	for _, symbol := range g.parseTable.Symbols {
		if symbol != generate.SymbolEndValue {
			g.symbolOrder[symbol] = i
			id := g.symbolIDs[symbol]
			g.values[id] = uint16(i)
			g.out.symbols = append(g.out.symbols, constant{name: names.name(id), value: uint16(i), display: g.symbolName(symbol)})
			i++
		}
	}
	for _, alias := range g.uniqueAliases {
		id := g.aliasIDs[alias]
		g.values[id] = uint16(i)
		g.out.symbols = append(g.out.symbols, constant{name: names.name(id), value: uint16(i), display: cString(g.strPool.Resolve(alias.Value))})
		i++
	}
}

// allSymbolCount returns the number of the symbols and the unique aliases,
// which is the length of the tables of the symbols.
func (g *generator) allSymbolCount() int {
	return len(g.parseTable.Symbols) + len(g.uniqueAliases)
}

// symbolName returns the name of a symbol, as ts_symbol_names holds it.
func (g *generator) symbolName(symbol generate.Symbol) string {
	var nameID generate.StrID
	if alias, ok := g.defaultAliases[symbol]; ok {
		nameID = alias.Value
	} else {
		nameID, _ = g.metadataForSymbol(symbol)
	}
	return cString(g.strPool.Resolve(nameID))
}

// addSymbolNamesList sets the name of each symbol.
//
// addSymbolNamesList is Generator::add_symbol_names_list.
func (g *generator) addSymbolNamesList() {
	names := make([]string, g.allSymbolCount())
	for _, symbol := range g.parseTable.Symbols {
		names[g.symbolValue(symbol)] = g.symbolName(symbol)
	}
	for _, alias := range g.uniqueAliases {
		names[g.value(g.aliasIDs[alias])] = cString(g.strPool.Resolve(alias.Value))
	}
	g.out.tables.SymbolNames = names
}

// addUniqueSymbolMap sets the public symbol of each symbol.
//
// addUniqueSymbolMap is Generator::add_unique_symbol_map.
func (g *generator) addUniqueSymbolMap() {
	m := make([]uint16, g.allSymbolCount())
	for _, symbol := range g.parseTable.Symbols {
		m[g.symbolValue(symbol)] = g.symbolValue(g.symbolMap[symbol])
	}

	for _, alias := range g.uniqueAliases {
		v := g.value(g.aliasIDs[alias])
		m[v] = v
	}

	g.out.tables.PublicSymbolMap = m
}

// addFieldNameEnum gives each field its number, and makes the Go constant of
// each field.
//
// addFieldNameEnum is Generator::add_field_name_enum.
func (g *generator) addFieldNameEnum() {
	names := newGoNames()
	for i, fieldName := range g.fieldNames {
		name := g.strPool.Resolve(fieldName)
		id := fieldID(name)
		g.values[id] = uint16(i + 1)
		g.out.fields = append(g.out.fields, constant{name: names.name(id), value: uint16(i + 1), display: name})
	}
}

// addFieldNameNamesList sets the name of each field.
//
// addFieldNameNamesList is Generator::add_field_name_names_list.
func (g *generator) addFieldNameNamesList() {
	// the name at index 0 is NULL in C
	names := []string{""}
	for _, fieldName := range g.fieldNames {
		names = append(names, g.strPool.Resolve(fieldName))
	}
	g.out.tables.FieldNames = names
}

// addSymbolMetadataList sets whether each symbol is visible and named.
//
// addSymbolMetadataList is Generator::add_symbol_metadata_list.
func (g *generator) addSymbolMetadataList() {
	md := make([]abi.SymbolMetadata, g.allSymbolCount())
	for _, symbol := range g.parseTable.Symbols {
		var m abi.SymbolMetadata
		if alias, ok := g.defaultAliases[symbol]; ok {
			m = abi.SymbolMetadata{Visible: true, Named: alias.IsNamed}
		} else {
			_, kind := g.metadataForSymbol(symbol)
			switch kind {
			case generate.VariableNamed:
				m = abi.SymbolMetadata{Visible: true, Named: true}
			case generate.VariableAnonymous:
				m = abi.SymbolMetadata{Visible: true, Named: false}
			case generate.VariableHidden:
				m = abi.SymbolMetadata{
					Visible:   false,
					Named:     true,
					Supertype: slices.Contains(g.syntaxGrammar.SupertypeSymbols, symbol),
				}
			case generate.VariableAuxiliary:
				m = abi.SymbolMetadata{Visible: false, Named: false}
			}
		}
		md[g.symbolValue(symbol)] = m
	}
	for _, alias := range g.uniqueAliases {
		md[g.value(g.aliasIDs[alias])] = abi.SymbolMetadata{Visible: true, Named: alias.IsNamed}
	}
	g.out.tables.SymbolMetadata = md
}

// addAliasSequences sets the alias of each child of each production.
//
// addAliasSequences is Generator::add_alias_sequences.
func (g *generator) addAliasSequences() {
	width := g.parseTable.MaxAliasedProductionLength
	seq := make([]uint16, len(g.parseTable.ProductionInfos)*width)
	for i := range g.parseTable.ProductionInfos {
		productionInfo := &g.parseTable.ProductionInfos[i]
		for j, alias := range productionInfo.AliasSequence {
			if alias != (generate.Alias{}) {
				seq[i*width+j] = g.value(g.aliasIDs[alias])
			}
		}
	}
	g.out.tables.AliasSequences = seq
}

// addNonTerminalAliasMap sets, for each non-terminal that a production
// aliases, its public symbol and the numbers of its aliases.
//
// addNonTerminalAliasMap is Generator::add_non_terminal_alias_map. Upstream
// collects the entries in an FxHashMap, and then sorts them by the symbol,
// which is unique, so the order of the map does not reach the output.
func (g *generator) addNonTerminalAliasMap() {
	aliasIDsBySymbol := map[generate.Symbol][]string{}
	for i := range g.syntaxGrammar.Variables {
		start, end := g.syntaxGrammar.VariableProdIDs(i)
		for prodID := start; prodID < end; prodID++ {
			for _, step := range g.syntaxGrammar.Production(prodID).Steps {
				alias, ok := step.GetAlias()
				if !ok || step.Symbol().Kind() != generate.SymbolNonTerminal {
					continue
				}
				if defaultAlias, ok := g.defaultAliases[step.Symbol()]; ok && defaultAlias == alias {
					continue
				}
				if _, ok := g.symbolIDs[step.Symbol()]; !ok {
					continue
				}
				aliasID, ok := g.aliasIDs[alias]
				if !ok {
					continue
				}
				aliasIDs := aliasIDsBySymbol[step.Symbol()]
				if ix, found := slices.BinarySearch(aliasIDs, aliasID); !found {
					aliasIDs = slices.Insert(aliasIDs, ix, aliasID)
				}
				aliasIDsBySymbol[step.Symbol()] = aliasIDs
			}
		}
	}

	symbols := make([]generate.Symbol, 0, len(aliasIDsBySymbol))
	for symbol := range aliasIDsBySymbol {
		symbols = append(symbols, symbol)
	}
	slices.SortFunc(symbols, generate.CompareSymbol)

	var out []uint16
	for _, symbol := range symbols {
		aliasIDs := aliasIDsBySymbol[symbol]
		out = append(out, g.symbolValue(symbol), uint16(1+len(aliasIDs)), g.symbolValue(g.symbolMap[symbol]))
		for _, aliasID := range aliasIDs {
			out = append(out, g.value(aliasID))
		}
	}
	out = append(out, 0)
	g.out.tables.AliasMap = out
}

// addPrimaryStateIDList produces a list of the "primary state" for every
// state in the grammar.
//
// The "primary state" for a given state is the first encountered state that
// behaves identically with respect to query analysis. We derive this by
// keeping track of the core_id for each state and treating the first state
// with a given core_id as primary.
//
// addPrimaryStateIDList is Generator::add_primary_state_id_list. Upstream
// only looks up its FxHashMap.
func (g *generator) addPrimaryStateIDList() {
	ids := make([]uint16, len(g.parseTable.States))
	firstStateForEachCoreID := map[uint32]int{}
	for idx := range g.parseTable.States {
		coreID := g.parseTable.States[idx].CoreID
		primaryState, ok := firstStateForEachCoreID[coreID]
		if !ok {
			primaryState = idx
			firstStateForEachCoreID[coreID] = idx
		}
		ids[idx] = uint16(primaryState)
	}
	g.out.tables.PrimaryStateIDs = ids
}

// fieldEntry is a field and one of its locations.
//
// fieldEntry is (StrId, FieldLocation).
type fieldEntry struct {
	name     generate.StrID
	location generate.FieldLocation
}

// flatFieldMap is the index of a row of ts_field_map_entries and its fields.
//
// flatFieldMap is (usize, Vec<(StrId, FieldLocation)>).
type flatFieldMap struct {
	index   int
	entries []fieldEntry
}

// addFieldSequences sets the fields of each production.
//
// addFieldSequences is Generator::add_field_sequences.
func (g *generator) addFieldSequences() {
	var flatFieldMaps []flatFieldMap
	nextFlatFieldMapIndex := 0
	getFieldMapID(nil, &flatFieldMaps, &nextFlatFieldMapIndex)

	type fieldMapID struct {
		rowID, length int
	}
	fieldMapIDs := make([]fieldMapID, 0, len(g.parseTable.ProductionInfos))
	for i := range g.parseTable.ProductionInfos {
		productionInfo := &g.parseTable.ProductionInfos[i]
		if len(productionInfo.FieldMap) == 0 {
			fieldMapIDs = append(fieldMapIDs, fieldMapID{0, 0})
		} else {
			flat := make([]fieldEntry, 0, len(productionInfo.FieldMap))
			for fieldName, locations := range productionInfo.FieldMap.Sorted() {
				for _, location := range locations {
					flat = append(flat, fieldEntry{name: fieldName, location: location})
				}
			}
			slices.SortStableFunc(flat, func(a, b fieldEntry) int {
				return cmp.Compare(g.strPool.Resolve(a.name), g.strPool.Resolve(b.name))
			})
			fieldMapLen := len(flat)
			fieldMapIDs = append(fieldMapIDs, fieldMapID{
				getFieldMapID(flat, &flatFieldMaps, &nextFlatFieldMapIndex),
				fieldMapLen,
			})
		}
	}

	mapSlices := make([]abi.MapSlice, len(fieldMapIDs))
	for productionID, id := range fieldMapIDs {
		if id.length > 0 {
			mapSlices[productionID] = abi.MapSlice{Index: uint16(id.rowID), Length: uint16(id.length)}
		}
	}
	g.out.tables.FieldMapSlices = mapSlices

	entries := make([]abi.FieldMapEntry, nextFlatFieldMapIndex)
	for _, row := range flatFieldMaps[1:] {
		for k, entry := range row.entries {
			entries[row.index+k] = abi.FieldMapEntry{
				FieldID:    g.value(fieldID(g.strPool.Resolve(entry.name))),
				ChildIndex: uint8(entry.location.Index),
				Inherited:  entry.location.Inherited,
			}
		}
	}
	g.out.tables.FieldMapEntries = entries
}

// addSupertypeMap sets the supertypes and the subtypes of each one.
//
// addSupertypeMap is Generator::add_supertype_map.
func (g *generator) addSupertypeMap() {
	supertypes := g.sortedSupertypes()
	t := &g.out.tables
	slicesLen := 0
	for _, supertype := range supertypes {
		v := g.value(supertype)
		t.SupertypeSymbols = append(t.SupertypeSymbols, v)
		slicesLen = max(slicesLen, int(v)+1)
	}

	rowID := 0
	// supertypeStringMap is a BTreeMap upstream, with the keys of
	// supertypeMap, and each value is a BTreeSet
	supertypeStringMap := make([][]string, 0, len(supertypes))
	for _, supertype := range supertypes {
		set := map[string]bool{}
		for _, s := range g.supertypeMap[supertype] {
			switch s.Kind {
			case generate.ChildTypeNormal:
				if id, ok := g.symbolIDs[s.Symbol]; ok {
					set[id] = true
				}
			case generate.ChildTypeAliased:
				if id, ok := g.aliasIDs[s.Alias]; ok {
					set[id] = true
				} else {
					for _, symbol := range g.symbolsForAlias(s.Alias) {
						if id, ok := g.symbolIDs[symbol]; ok {
							set[id] = true
						}
					}
				}
			}
		}
		subtypes := make([]string, 0, len(set))
		for id := range set {
			subtypes = append(subtypes, id)
		}
		slices.Sort(subtypes)
		supertypeStringMap = append(supertypeStringMap, subtypes)
	}
	// the C array is as long as its largest designated index
	t.SupertypeMapSlices = make([]abi.MapSlice, slicesLen)
	for i, supertype := range supertypes {
		length := len(supertypeStringMap[i])
		t.SupertypeMapSlices[g.value(supertype)] = abi.MapSlice{Index: uint16(rowID), Length: uint16(length)}
		rowID += length
	}

	for _, subtypes := range supertypeStringMap {
		for _, subtype := range subtypes {
			t.SupertypeMapEntries = append(t.SupertypeMapEntries, g.value(subtype))
		}
	}
}

// sortedSupertypes returns the keys of supertypeMap, in the order of the
// BTreeMap of upstream.
func (g *generator) sortedSupertypes() []string {
	keys := make([]string, 0, len(g.supertypeMap))
	for k := range g.supertypeMap {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// addLexFunction builds the lex table of a lex function. Each state holds
// the ranges of characters that the C code of add_lex_state tests, in the
// order of the C code, so that the first transition that the C code takes
// for a character is the transition of its range.
//
// addLexFunction is Generator::add_lex_function.
func (g *generator) addLexFunction(lexTable *generate.LexTable) *abi.LexTable {
	table := &abi.LexTable{}
	for i := range lexTable.States {
		state := &lexTable.States[i]
		transitions := g.addLexState(state)
		start := uint32(len(table.Ranges))
		table.Ranges = append(table.Ranges, g.lexRanges(transitions)...)
		table.States = append(table.States, g.lexState(state, transitions, start, uint32(len(table.Ranges))-start))
	}
	return table
}

// bestLargeCharSet is a large character set that a transition uses, and the
// characters that the transition adds to it and removes from it.
//
// bestLargeCharSet is (usize, CharacterSet, CharacterSet).
type bestLargeCharSet struct {
	ix                  int
	additions, removals generate.CharacterSet
}

// termKind is the kind of a test that the C code of a transition makes.
type termKind uint8

const (
	// termIn is lo <= lookahead && lookahead <= hi.
	termIn termKind = iota
	// termNotIn is the negation of termIn.
	termNotIn
	// termSet is set_contains of a large character set.
	termSet
)

// term is one test of the condition of a transition. eofGuard is true when
// the C code tests !eof before it, as it does for a range that holds 0.
type term struct {
	kind     termKind
	lo, hi   int64
	eofGuard bool
	set      int
}

// condition is the condition of a transition, as add_lex_state writes it:
// an OR of the positive clauses, each one an AND of its tests, and then an
// AND of the negative tests. always is true when the C code tests nothing.
type condition struct {
	always bool
	hasPos bool
	pos    [][]term
	neg    []term
}

// transition is a transition of a lex state in the order of the C code: an
// entry of ADVANCE_MAP, which compares one character, or an if statement.
type transition struct {
	isMap   bool
	mapChar int32
	cond    condition
	state   uint16
	skip    bool
}

// addLexState returns the transitions of one state of a lex function, in
// the order of the C code that upstream writes.
//
// addLexState is Generator::add_lex_state.
func (g *generator) addLexState(state *generate.LexState) []transition {
	var transitions []transition

	var charsCopy, largeSet, ruledOutChars generate.CharacterSet

	// The transitions in a lex state are sorted with the single-character
	// transitions first. If there are many single-character transitions,
	// then implement them using an array of (lookahead character, state)
	// pairs, instead of individual if statements, in order to reduce compile
	// time.
	leadingSimpleTransitionCount := 0
	leadingSimpleTransitionRangeCount := 0
	for _, advance := range state.AdvanceActions {
		if advance.Action.InMainToken && isSimpleTransition(advance.Chars) {
			leadingSimpleTransitionCount++
			leadingSimpleTransitionRangeCount += advance.Chars.RangeCount()
		} else {
			break
		}
	}

	if leadingSimpleTransitionRangeCount >= 8 {
		// ADVANCE_MAP holds the first and the last character of each range
		for _, advance := range state.AdvanceActions[:leadingSimpleTransitionCount] {
			for start, end := range advance.Chars.Ranges() {
				transitions = append(transitions, transition{isMap: true, mapChar: start, state: uint16(advance.Action.State)})
				if end > start {
					transitions = append(transitions, transition{isMap: true, mapChar: end, state: uint16(advance.Action.State)})
				}
			}
			ruledOutChars = ruledOutChars.Add(advance.Chars)
		}
	} else {
		leadingSimpleTransitionCount = 0
	}

	for _, advance := range state.AdvanceActions[leadingSimpleTransitionCount:] {
		chars := advance.Chars

		// The lex state's advance actions are represented with disjoint
		// sets of characters. When translating these disjoint sets into a
		// sequence of checks, we don't need to re-check conditions that
		// have already been checked due to previous transitions.
		//
		// Note that this simplification may result in an empty character set.
		// That means that the transition is guaranteed (nothing further needs to
		// be checked), not that this transition is impossible.
		simplifiedChars := chars.SimplifyIgnoring(ruledOutChars)

		// For large character sets, find the best matching character set from
		// a pre-selected list of large character sets, which are based on the
		// state transitions for individual tokens. This transition may not exactly
		// match one of the pre-selected character sets. In that case, determine
		// the additional checks that need to be performed to match this transition.
		var best *bestLargeCharSet
		if simplifiedChars.RangeCount() >= generate.LargeCharacterRangeCount {
			for ix, set := range g.largeCharacterSets {
				charsCopy.Assign(simplifiedChars)
				largeSet.Assign(set.Chars)
				intersection := charsCopy.RemoveIntersection(&largeSet)
				if !intersection.IsEmpty() {
					additions := charsCopy.SimplifyIgnoring(ruledOutChars)
					removals := largeSet.SimplifyIgnoring(ruledOutChars)
					totalRangeCount := additions.RangeCount() + removals.RangeCount()
					if totalRangeCount >= simplifiedChars.RangeCount() {
						continue
					}
					if best != nil {
						bestRangeCount := best.additions.RangeCount() + best.removals.RangeCount()
						if bestRangeCount < totalRangeCount {
							continue
						}
					}
					best = &bestLargeCharSet{ix: ix, additions: additions, removals: removals}
				}
			}
		}

		// Add this transition's character set to the set of ruled out characters,
		// which don't need to be checked for subsequent transitions in this state.
		ruledOutChars = ruledOutChars.Add(chars)

		largeCharSetIx, hasLargeCharSet := 0, false
		assertedChars := simplifiedChars
		var negatedChars generate.CharacterSet
		if best != nil {
			assertedChars = best.additions
			negatedChars = best.removals
			largeCharSetIx, hasLargeCharSet = best.ix, true
		}

		hasPositiveCondition := hasLargeCharSet || !assertedChars.IsEmpty()
		hasNegativeCondition := !negatedChars.IsEmpty()
		hasCondition := hasPositiveCondition || hasNegativeCondition
		c := condition{always: !hasCondition, hasPos: hasPositiveCondition}

		if hasLargeCharSet {
			largeSet := g.largeCharacterSets[largeCharSetIx].Chars

			// If the character set contains the null character, check that we
			// are not at the end of the file.
			checkEOF := largeSet.Contains(0)

			charSetInfo := &g.largeCharacterSetInfo[largeCharSetIx]
			charSetInfo.isUsed = true
			c.pos = append(c.pos, []term{{kind: termSet, set: largeCharSetIx, eofGuard: checkEOF}})
		}

		if !assertedChars.IsEmpty() {
			// If the character set contains the max character, then it probably
			// corresponds to a negated character class in a regex, so it will be more
			// concise and readable to express it in terms of negated ranges.
			isIncluded := !assertedChars.Contains(unicode.MaxRune)
			if !isIncluded {
				assertedChars = assertedChars.Negate().AddChar(0)
			}

			terms := addCharacterRangeConditions(assertedChars, isIncluded)
			if isIncluded {
				// each test is a clause of the OR
				for _, t := range terms {
					c.pos = append(c.pos, []term{t})
				}
			} else {
				// the negated tests are one clause of the OR, which ANDs them
				c.pos = append(c.pos, terms)
			}
		}

		if hasNegativeCondition {
			c.neg = append(c.neg, addCharacterRangeConditions(negatedChars, false)...)
		}

		transitions = append(transitions, addAdvanceAction(c, advance.Action))
	}

	return transitions
}

// isSimpleTransition reports whether each range of a set holds one or two
// characters, and whether each one fits in 16 bits.
//
// isSimpleTransition is the closure that add_lex_state gives to
// `chars.ranges().all`.
func isSimpleTransition(chars generate.CharacterSet) bool {
	for start, end := range chars.Ranges() {
		if uint32(end) > uint32(start)+1 || end > math.MaxUint16 {
			return false
		}
	}
	return true
}

// addCharacterRangeConditions returns the tests of lookahead for a set of
// characters that the C code makes: that it is in the set when isIncluded is
// true, and that it is not when isIncluded is false. The tests of an
// included set are ORed, and the tests of an excluded set are ANDed.
//
// addCharacterRangeConditions is Generator::add_character_range_conditions.
func addCharacterRangeConditions(characters generate.CharacterSet, isIncluded bool) []term {
	var terms []term
	for start, end := range characters.Ranges() {
		s, e := int64(start), int64(end)
		if isIncluded {
			switch {
			case start == 0:
				// (!eof && lookahead == 0) or (!eof && lookahead <= end)
				if end == 0 {
					terms = append(terms, term{kind: termIn, lo: 0, hi: 0, eofGuard: true})
				} else {
					terms = append(terms, term{kind: termIn, lo: math.MinInt32, hi: e, eofGuard: true})
				}
			case end == start:
				terms = append(terms, term{kind: termIn, lo: s, hi: s})
			case uint32(end) == uint32(start)+1:
				terms = append(terms, term{kind: termIn, lo: s, hi: s}, term{kind: termIn, lo: e, hi: e})
			default:
				terms = append(terms, term{kind: termIn, lo: s, hi: e})
			}
		} else {
			switch {
			case end == start:
				terms = append(terms, term{kind: termNotIn, lo: s, hi: s})
			case uint32(end) == uint32(start)+1:
				terms = append(terms, term{kind: termNotIn, lo: s, hi: s}, term{kind: termNotIn, lo: e, hi: e})
			case start != 0:
				terms = append(terms, term{kind: termNotIn, lo: s, hi: e})
			default:
				// lookahead > end
				terms = append(terms, term{kind: termNotIn, lo: math.MinInt32, hi: e})
			}
		}
	}
	return terms
}

// addCharacterSet keeps the ranges of a large character set that a lex
// state uses, for the evaluation of set_contains. The C code gives
// set_contains the number of ranges of the set, and writes the ranges that
// Ranges gives, which leaves out a range of surrogates only. So the two
// counts must be the same.
//
// addCharacterSet is Generator::add_character_set.
func (g *generator) addCharacterSet(ix int) {
	characters := g.largeCharacterSets[ix].Chars
	info := &g.largeCharacterSetInfo[ix]
	if !info.isUsed {
		return
	}
	n := 0
	for range characters.Ranges() {
		n++
	}
	if n != characters.RangeCount() && g.err == nil {
		g.err = &RenderError{Kind: RenderErrorCharacterSet, Name: info.constantName}
	}
}

// addAdvanceAction returns the transition of an advance action with its
// condition: ADVANCE for a character of the token, and SKIP for one that is
// not.
//
// addAdvanceAction is Generator::add_advance_action.
func addAdvanceAction(c condition, action generate.AdvanceAction) transition {
	return transition{cond: c, state: uint16(action.State), skip: !action.InMainToken}
}

// setContains reports whether a large character set holds the lookahead,
// with the binary search of the C code.
//
// setContains is set_contains of parser.h.
func setContains(ranges [][2]int32, n uint32, lookahead int64) bool {
	index := uint32(0)
	size := n - index
	for size > 1 {
		halfSize := size / 2
		midIndex := index + halfSize
		r := ranges[midIndex]
		if lookahead >= int64(r[0]) && lookahead <= int64(r[1]) {
			return true
		} else if lookahead > int64(r[1]) {
			index = midIndex
		}
		size -= halfSize
	}
	r := ranges[index]
	return lookahead >= int64(r[0]) && lookahead <= int64(r[1])
}

// characterSetRanges returns the ranges of a large character set.
func (g *generator) characterSetRanges(ix int) [][2]int32 {
	if g.setRanges == nil {
		g.setRanges = make([][][2]int32, len(g.largeCharacterSets))
	}
	if g.setRanges[ix] == nil {
		for start, end := range g.largeCharacterSets[ix].Chars.Ranges() {
			g.setRanges[ix] = append(g.setRanges[ix], [2]int32{start, end})
		}
	}
	return g.setRanges[ix]
}

// evalTerm evaluates one test at a lookahead, at the end of the input when
// eof is true.
func (g *generator) evalTerm(t term, lookahead int64, eof bool) bool {
	if t.eofGuard && eof {
		return false
	}
	switch t.kind {
	case termIn:
		return t.lo <= lookahead && lookahead <= t.hi
	case termNotIn:
		return lookahead < t.lo || t.hi < lookahead
	case termSet:
		ranges := g.characterSetRanges(t.set)
		return setContains(ranges, uint32(len(ranges)), lookahead)
	}
	return false
}

// eval reports whether the C code takes a transition at a lookahead, at the
// end of the input when eof is true.
func (g *generator) eval(tr *transition, lookahead int64, eof bool) bool {
	if tr.isMap {
		// ADVANCE_MAP compares the character with the lookahead, and does
		// not test eof
		return lookahead == int64(tr.mapChar)
	}
	c := &tr.cond
	if c.always {
		return true
	}
	pos := !c.hasPos
	for _, clause := range c.pos {
		all := true
		for _, t := range clause {
			if !g.evalTerm(t, lookahead, eof) {
				all = false
				break
			}
		}
		if all {
			pos = true
			break
		}
	}
	if !pos {
		return false
	}
	for _, t := range c.neg {
		if !g.evalTerm(t, lookahead, eof) {
			return false
		}
	}
	return true
}

// breakpoints appends the lookaheads where a test of a transition can
// change its result.
func (g *generator) breakpoints(tr *transition, points []int64) []int64 {
	if tr.isMap {
		return append(points, int64(tr.mapChar), int64(tr.mapChar)+1)
	}
	add := func(t term) {
		if t.kind == termSet {
			for _, r := range g.characterSetRanges(t.set) {
				points = append(points, int64(r[0]), int64(r[1])+1)
			}
			return
		}
		points = append(points, t.lo, t.hi+1)
	}
	for _, clause := range tr.cond.pos {
		for _, t := range clause {
			add(t)
		}
	}
	for _, t := range tr.cond.neg {
		add(t)
	}
	return points
}

// lexState returns the state of a lex table: its accept action, and what it
// does at the end of the input. At the end of the input, the lookahead is 0
// and eof is true. The C code tests the EOF action first, and then each
// transition in its order.
func (g *generator) lexState(state *generate.LexState, transitions []transition, start, count uint32) abi.LexState {
	s := abi.LexState{Start: start, Count: count}
	if state.HasAcceptAction {
		s.HasAccept, s.Accept = true, g.symbolValue(state.AcceptAction)
	}
	if state.HasEOFAction {
		s.HasEOF, s.EOFState = true, uint16(state.EOFAction.State)
		return s
	}
	for i := range transitions {
		if g.eval(&transitions[i], 0, true) {
			s.HasEOF, s.EOFState, s.EOFSkip = true, transitions[i].state, transitions[i].skip
			break
		}
	}
	return s
}

// lexRanges returns the sorted ranges of a state from its transitions,
// before the end of the input: for each range of characters where no test
// of a transition changes its result, the first transition that the C code
// takes. It merges the ranges next to each other that go to the same state.
func (g *generator) lexRanges(transitions []transition) []abi.LexRange {
	if len(transitions) == 0 {
		return nil
	}
	points := []int64{math.MinInt32}
	for i := range transitions {
		points = g.breakpoints(&transitions[i], points)
	}
	slices.Sort(points)
	points = slices.Compact(points)
	for len(points) > 0 && points[len(points)-1] > math.MaxInt32 {
		points = points[:len(points)-1]
	}
	var out []abi.LexRange
	for i, lo := range points {
		hi := int64(math.MaxInt32)
		if i+1 < len(points) {
			hi = points[i+1] - 1
		}
		for k := range transitions {
			tr := &transitions[k]
			if !g.eval(tr, lo, false) {
				continue
			}
			if n := len(out); n > 0 && int64(out[n-1].Hi)+1 == lo && out[n-1].State == tr.state && out[n-1].Skip == tr.skip {
				out[n-1].Hi = int32(hi)
			} else {
				out = append(out, abi.LexRange{Lo: int32(lo), Hi: int32(hi), State: tr.state, Skip: tr.skip})
			}
			break
		}
	}
	return out
}

// addLexModes sets the lex state of each parse state.
//
// addLexModes is Generator::add_lex_modes.
func (g *generator) addLexModes() {
	modes := make([]abi.LexerMode, len(g.parseTable.States))
	for i := range g.parseTable.States {
		state := &g.parseTable.States[i]
		if state.IsEndOfNonTerminalExtra() {
			// (TSStateId)(-1)
			modes[i] = abi.LexerMode{LexState: math.MaxUint16}
			continue
		}
		modes[i] = abi.LexerMode{
			LexState:         uint16(state.LexStateID),
			ExternalLexState: uint16(state.ExternalLexStateID),
		}
		if g.abiVersion >= abiVersionWithReservedWords {
			modes[i].ReservedWordSetID = uint16(g.reservedWordSetIDsByParseState[i])
		}
	}
	g.out.tables.LexModes = modes
}

// addReservedWordSets sets the sets of reserved words.
//
// addReservedWordSets is Generator::add_reserved_word_sets.
func (g *generator) addReservedWordSets() {
	width := g.maxReservedWordSetSize()
	words := make([]uint16, len(g.reservedWordSets)*width)
	for id := range g.reservedWordSets {
		if id == 0 {
			continue
		}
		k := 0
		for token := range g.reservedWordSets[id].All() {
			words[id*width+k] = g.symbolValue(token)
			k++
		}
	}
	g.out.tables.ReservedWords = words
}

// addExternalScannerSymbolMap sets the symbol of each external token.
//
// addExternalScannerSymbolMap is Generator::add_external_scanner_symbol_map.
func (g *generator) addExternalScannerSymbolMap() {
	m := make([]uint16, len(g.syntaxGrammar.ExternalTokens))
	for i := range g.syntaxGrammar.ExternalTokens {
		token := &g.syntaxGrammar.ExternalTokens[i]
		idToken := generate.ExternalSymbol(i)
		if token.HasCorrespondingInternalToken {
			idToken = token.CorrespondingInternalToken
		}
		m[i] = g.symbolValue(idToken)
	}
	g.out.tables.ExternalScanner.SymbolMap = m
}

// addExternalScannerStatesList sets the external tokens that are valid in
// each external lex state.
//
// addExternalScannerStatesList is
// Generator::add_external_scanner_states_list.
func (g *generator) addExternalScannerStatesList() {
	width := len(g.syntaxGrammar.ExternalTokens)
	states := make([]bool, len(g.parseTable.ExternalLexStates)*width)
	for i := range g.parseTable.ExternalLexStates {
		for index := range g.parseTable.ExternalLexStates[i].Externals() {
			states[i*width+int(index)] = true
		}
	}
	g.out.tables.ExternalScanner.States = states
}

// terminalEntry is a terminal entry of a parse state.
type terminalEntry struct {
	symbol generate.Symbol
	id     generate.ActionListID
}

// nonterminalEntry is a non-terminal entry of a parse state.
type nonterminalEntry struct {
	symbol generate.Symbol
	action generate.GotoAction
}

// valueKey is the key of a group of symbols in a small state: an action list
// id or a state, and whether the symbols are terminals or non-terminals.
//
// valueKey is (u32, SymbolType).
type valueKey struct {
	value uint32
	kind  generate.SymbolType
}

// addParseTable sets the parse table: the large states, the small states
// and the action lists.
//
// addParseTable is Generator::add_parse_table. Upstream keeps two
// FxHashMaps. It iterates parse_table_entries once, and sorts the entries by
// their index, which is unique. It iterates symbols_by_value once for each
// small state, and sorts the groups by a key that holds the key of the map,
// which is unique. So the order of the maps does not reach the output.
func (g *generator) addParseTable() error {
	parseTableEntries := map[generate.ActionListID]uint32{}
	var nextParseActionListIndex uint32

	// Parse action lists zero is for the default value, when a symbol is not valid.
	// `canonicalize` guarantees pool index 0 is the empty list.
	getParseActionListID(
		generate.NewActionListID(0, false),
		&g.parseTable.ActionLists,
		parseTableEntries,
		&nextParseActionListIndex,
	)

	t := &g.out.tables
	symbolCount := len(g.parseTable.Symbols)
	t.ParseTable = make([]uint16, g.largeStateCount*symbolCount)

	var terminalEntries []terminalEntry
	var nonterminalEntries []nonterminalEntry

	for i := range g.parseTable.States[:g.largeStateCount] {
		state := &g.parseTable.States[i]
		row := t.ParseTable[i*symbolCount : (i+1)*symbolCount]

		// Ensure the entries are in a deterministic order, since they are
		// internally represented as a hash map.
		terminalEntries = g.sortedTerminalEntries(terminalEntries[:0], state)
		nonterminalEntries = nonterminalEntries[:0]
		for symbol, action := range state.NonterminalEntries.All() {
			nonterminalEntries = append(nonterminalEntries, nonterminalEntry{symbol: symbol, action: action})
		}
		// the symbols of the entries of a state are unique
		slices.SortFunc(nonterminalEntries, func(a, b nonterminalEntry) int {
			return generate.CompareSymbol(a.symbol, b.symbol)
		})

		for _, entry := range nonterminalEntries {
			target := i
			if entry.action.Kind == generate.GotoActionGoto {
				target = int(entry.action.State)
			}
			row[g.symbolValue(entry.symbol)] = uint16(target)
		}

		for _, entry := range terminalEntries {
			entryID := getParseActionListID(
				entry.id,
				&g.parseTable.ActionLists,
				parseTableEntries,
				&nextParseActionListIndex,
			)
			row[g.symbolValue(entry.symbol)] = uint16(entryID)
		}
	}

	if g.largeStateCount < len(g.parseTable.States) {
		var small []uint16
		smallStateIndices := make([]uint32, 0, max(len(g.parseTable.States)-g.largeStateCount, 0))
		symbolsByValue := map[valueKey][]generate.Symbol{}
		for i := g.largeStateCount; i < len(g.parseTable.States); i++ {
			state := &g.parseTable.States[i]
			smallStateIndices = append(smallStateIndices, uint32(len(small)))
			clear(symbolsByValue)

			terminalEntries = g.sortedTerminalEntries(terminalEntries[:0], state)

			// In a given parse state, many lookahead symbols have the same actions.
			// So in the "small state" representation, group symbols by their action
			// in order to avoid repeating the action.
			for _, entry := range terminalEntries {
				entryID := getParseActionListID(
					entry.id,
					&g.parseTable.ActionLists,
					parseTableEntries,
					&nextParseActionListIndex,
				)
				key := valueKey{value: entryID, kind: generate.SymbolTerminal}
				symbolsByValue[key] = append(symbolsByValue[key], entry.symbol)
			}
			for symbol, action := range state.NonterminalEntries.All() {
				var stateID uint32
				switch action.Kind {
				case generate.GotoActionGoto:
					stateID = action.State
				case generate.GotoActionShiftExtra:
					stateID = uint32(g.largeStateCount + len(smallStateIndices) - 1)
				}
				key := valueKey{value: stateID, kind: generate.SymbolNonTerminal}
				symbolsByValue[key] = append(symbolsByValue[key], symbol)
			}

			type valueWithSymbols struct {
				key     valueKey
				symbols []generate.Symbol
			}
			valuesWithSymbols := make([]valueWithSymbols, 0, len(symbolsByValue))
			for key, symbols := range symbolsByValue {
				valuesWithSymbols = append(valuesWithSymbols, valueWithSymbols{key: key, symbols: symbols})
			}
			// the key holds the key of the map, which is unique, so the
			// unstable sort of upstream has no equal elements
			slices.SortFunc(valuesWithSymbols, func(a, b valueWithSymbols) int {
				return cmp.Or(
					cmp.Compare(len(a.symbols), len(b.symbols)),
					cmp.Compare(a.key.kind, b.key.kind),
					cmp.Compare(a.key.value, b.key.value),
					generate.CompareSymbol(a.symbols[0], b.symbols[0]),
				)
			})

			small = append(small, uint16(len(valuesWithSymbols)))
			for _, v := range valuesWithSymbols {
				small = append(small, uint16(v.key.value), uint16(len(v.symbols)))

				// the symbols of a group are unique
				slices.SortFunc(v.symbols, generate.CompareSymbol)
				for _, symbol := range v.symbols {
					small = append(small, g.symbolValue(symbol))
				}
			}
		}

		t.SmallParseTable = small
		t.SmallParseTableMap = smallStateIndices
	}
	if nextParseActionListIndex >= math.MaxUint16 {
		return &RenderError{Kind: RenderErrorParseTable, Value: int(nextParseActionListIndex)}
	}

	entries := make([]parseTableEntry, 0, len(parseTableEntries))
	for id, i := range parseTableEntries {
		entries = append(entries, parseTableEntry{index: i, id: id})
	}
	slices.SortFunc(entries, func(a, b parseTableEntry) int {
		return cmp.Compare(a.index, b.index)
	})
	g.addParseActionList(entries, int(nextParseActionListIndex))

	return nil
}

// sortedTerminalEntries appends the terminal entries of a state to entries,
// in the order of the symbols in the symbol enum, and returns the result.
//
// sortedTerminalEntries is the code that add_parse_table writes twice, which
// sorts the entries with sort_unstable_by_key on
// `self.symbol_order.get(e.0)`. Each symbol of the parse table has its own
// order. Only the end of a non-terminal extra has none, and it sorts first,
// as the None of upstream. A state holds it once at most, so the sort has no
// equal elements.
func (g *generator) sortedTerminalEntries(entries []terminalEntry, state *generate.ParseState[generate.ActionListID]) []terminalEntry {
	for symbol, id := range state.TerminalEntries.All() {
		entries = append(entries, terminalEntry{symbol: symbol, id: id})
	}
	order := func(symbol generate.Symbol) int {
		if o, ok := g.symbolOrder[symbol]; ok {
			return o
		}
		return -1
	}
	slices.SortFunc(entries, func(a, b terminalEntry) int {
		return cmp.Compare(order(a.symbol), order(b.symbol))
	})
	return entries
}

// parseTableEntry is the index of an action list in ts_parse_actions, and
// its id.
//
// parseTableEntry is (u32, ActionListId).
type parseTableEntry struct {
	index uint32
	id    generate.ActionListID
}

// addParseActionList sets the action lists. n is the length of the table.
// Each action holds the member of the C union that its macro sets: Shift for
// SHIFT, SHIFT_REPEAT and SHIFT_EXTRA, Reduce for REDUCE, and the type only
// for RECOVER and ACCEPT_INPUT.
//
// addParseActionList is Generator::add_parse_action_list.
func (g *generator) addParseActionList(parseTableEntries []parseTableEntry, n int) {
	out := make([]abi.ParseActionEntry, n)
	for _, entry := range parseTableEntries {
		actions := g.parseTable.ActionLists.Get(entry.id)
		i := int(entry.index)
		out[i].Entry = abi.EntryHeader{Count: uint8(len(actions)), Reusable: entry.id.Reusable()}
		for k, action := range actions {
			var a abi.ParseAction
			switch action.Kind {
			case generate.ParseActionAccept:
				a.Type = abi.ParseActionTypeAccept
			case generate.ParseActionRecover:
				a.Type = abi.ParseActionTypeRecover
			case generate.ParseActionShiftExtra:
				a.Type = abi.ParseActionTypeShift
				a.Shift.Extra = true
			case generate.ParseActionShift:
				a.Type = abi.ParseActionTypeShift
				a.Shift.State = uint16(action.State)
				a.Shift.Repetition = action.IsRepetition
			case generate.ParseActionReduce:
				a.Type = abi.ParseActionTypeReduce
				a.Reduce = abi.ReduceAction{
					ChildCount:        uint8(action.ChildCount),
					Symbol:            g.symbolValue(action.Symbol),
					DynamicPrecedence: int16(action.DynamicPrecedence),
					ProductionID:      action.ProductionID,
				}
			}
			out[i+1+k].Action = a
		}
	}
	g.out.tables.ParseActions = out
}

// addParserExport sets what the C literal of TSLanguage sets and no table
// above holds: the keyword capture token, whether the grammar has an
// external scanner, the name, the size of the sets of reserved words and the
// version of the grammar.
//
// addParserExport is Generator::add_parser_export.
func (g *generator) addParserExport() {
	t := &g.out.tables
	if g.abiVersion >= abiVersionWithReservedWords {
		t.SupertypeCount = uint32(len(g.supertypeMap))
	}
	if g.syntaxGrammar.HasWordToken {
		t.KeywordCaptureToken = g.symbolValue(g.syntaxGrammar.WordToken)
	}
	g.out.hasExternalScanner = len(g.syntaxGrammar.ExternalTokens) != 0

	if g.abiVersion >= abiVersionWithReservedWords {
		t.Name = g.languageName
		t.MaxReservedWordSetSize = uint16(g.maxReservedWordSetSize())

		var metadata generate.SemanticVersion
		if g.metadata != nil {
			metadata = *g.metadata
		}
		t.Metadata = abi.LanguageMetadata{
			MajorVersion: metadata.Major,
			MinorVersion: metadata.Minor,
			PatchVersion: metadata.Patch,
		}
	}
}

// addKeywords finds the keywords of the grammar: the tokens that the keyword
// lex function accepts, and the tokens of each set of reserved words. It
// leaves out a token that is not visible, such as a hidden token that the
// generator makes from a pattern inside a rule, because SymbolForName does
// not find it (D82). It ports nothing of upstream.
func (g *generator) addKeywords() {
	set := map[string]bool{}
	add := func(token generate.Symbol) {
		if g.out.tables.SymbolMetadata[g.symbolValue(token)].Visible {
			set[g.symbolName(token)] = true
		}
	}
	if g.syntaxGrammar.HasWordToken {
		for i := range g.keywordLexTable.States {
			if state := &g.keywordLexTable.States[i]; state.HasAcceptAction {
				add(state.AcceptAction)
			}
		}
	}
	for i := range g.reservedWordSets {
		for token := range g.reservedWordSets[i].All() {
			add(token)
		}
	}
	for name := range set {
		g.out.keywords = append(g.out.keywords, name)
	}
	slices.Sort(g.out.keywords)
}

// getParseActionListID returns the index of an action list in
// ts_parse_actions. A list that has no index yet gets the next one.
//
// getParseActionListID is Generator::get_parse_action_list_id. Upstream
// only looks up the FxHashMap here.
func getParseActionListID(id generate.ActionListID, pool *generate.ActionListPool, parseActionListOffsets map[generate.ActionListID]uint32, nextParseActionListIndex *uint32) uint32 {
	if index, ok := parseActionListOffsets[id]; ok {
		return index
	}
	result := *nextParseActionListIndex
	parseActionListOffsets[id] = result
	*nextParseActionListIndex += 1 + uint32(len(pool.Get(id)))
	return result
}

// getFieldMapID returns the index of the row of the fields of a production
// in ts_field_map_entries. A list of fields that has no row yet gets a new
// one.
//
// getFieldMapID is Generator::get_field_map_id.
func getFieldMapID(flat []fieldEntry, flatFieldMaps *[]flatFieldMap, nextFlatFieldMapIndex *int) int {
	for _, m := range *flatFieldMaps {
		if slices.Equal(m.entries, flat) {
			return m.index
		}
	}

	result := *nextFlatFieldMapIndex
	*nextFlatFieldMapIndex += len(flat)
	*flatFieldMaps = append(*flatFieldMaps, flatFieldMap{index: result, entries: flat})
	return result
}

// assignSymbolID gives a symbol its C identifier. An identifier that another
// symbol has gets a number at its end.
//
// assignSymbolID is Generator::assign_symbol_id. Upstream only looks up the
// FxHashSet usedIdentifiers.
func (g *generator) assignSymbolID(symbol generate.Symbol, usedIdentifiers map[string]bool) {
	var id string
	if symbol == generate.SymbolEndValue {
		id = "ts_builtin_sym_end"
	} else {
		name, kind := g.metadataForSymbol(symbol)
		switch kind {
		case generate.VariableAuxiliary:
			id = "aux_sym_" + g.sanitizeIdentifier(name)
		case generate.VariableAnonymous:
			id = "anon_sym_" + g.sanitizeIdentifier(name)
		case generate.VariableHidden, generate.VariableNamed:
			id = "sym_" + g.sanitizeIdentifier(name)
		}

		suffixNumber := 1
		suffix := ""
		for usedIdentifiers[id] {
			id = id[:len(id)-len(suffix)]
			suffixNumber++
			suffix = strconv.Itoa(suffixNumber)
			id += suffix
		}
	}

	usedIdentifiers[id] = true
	g.symbolIDs[symbol] = id
}

// fieldID returns the C identifier of a field.
//
// fieldID is Generator::field_id.
func fieldID(fieldName string) string {
	return "field_" + fieldName
}

// metadataForSymbol returns the name and the kind of a symbol.
//
// metadataForSymbol is Generator::metadata_for_symbol.
func (g *generator) metadataForSymbol(symbol generate.Symbol) (generate.StrID, generate.VariableType) {
	switch symbol.Kind() {
	case generate.SymbolEnd, generate.SymbolEndOfNonTerminalExtra:
		return generate.EndNameID, generate.VariableHidden
	case generate.SymbolNonTerminal:
		index, _ := symbol.NonTerminalIndex()
		variable := &g.syntaxGrammar.Variables[index]
		return variable.Name, variable.Kind
	case generate.SymbolTerminal:
		index, _ := symbol.TerminalIndex()
		variable := &g.lexicalGrammar.Variables[index]
		return variable.Name, variable.Kind
	case generate.SymbolExternal:
		index, _ := symbol.ExternalIndex()
		token := &g.syntaxGrammar.ExternalTokens[index]
		return token.Name, token.Kind
	}
	panic("golang: a symbol of an unknown kind")
}

// symbolsForAlias returns the symbols of the parse table that show as an
// alias: the symbols with the alias as their default alias, and the symbols
// with no default alias whose name and kind are the ones of the alias.
//
// symbolsForAlias is Generator::symbols_for_alias.
func (g *generator) symbolsForAlias(alias generate.Alias) []generate.Symbol {
	var symbols []generate.Symbol
	for _, symbol := range g.parseTable.Symbols {
		var matches bool
		if defaultAlias, ok := g.defaultAliases[symbol]; ok {
			matches = defaultAlias == alias
		} else {
			name, kind := g.metadataForSymbol(symbol)
			matches = name == alias.Value && kind == alias.Kind()
		}
		if matches {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// identifierReplacements holds the word that sanitizeIdentifier writes for
// each special character. A space is here for a name that is one space
// only, and sanitizeIdentifier drops a space in a longer name.
var identifierReplacements = map[rune]string{
	' ':      "SPACE",
	'~':      "TILDE",
	'`':      "BQUOTE",
	'!':      "BANG",
	'@':      "AT",
	'#':      "POUND",
	'$':      "DOLLAR",
	'%':      "PERCENT",
	'^':      "CARET",
	'&':      "AMP",
	'*':      "STAR",
	'(':      "LPAREN",
	')':      "RPAREN",
	'-':      "DASH",
	'+':      "PLUS",
	'=':      "EQ",
	'{':      "LBRACE",
	'}':      "RBRACE",
	'[':      "LBRACK",
	']':      "RBRACK",
	'\\':     "BSLASH",
	'|':      "PIPE",
	':':      "COLON",
	';':      "SEMI",
	'"':      "DQUOTE",
	'\'':     "SQUOTE",
	'<':      "LT",
	'>':      "GT",
	',':      "COMMA",
	'.':      "DOT",
	'?':      "QMARK",
	'/':      "SLASH",
	'\n':     "LF",
	'\r':     "CR",
	'\t':     "TAB",
	'\x00':   "NULL",
	'\x01':   "SOH",
	'\x02':   "STX",
	'\x03':   "ETX",
	'\x04':   "EOT",
	'\x05':   "ENQ",
	'\x06':   "ACK",
	'\x07':   "BEL",
	'\x08':   "BS",
	'\x0b':   "VTAB",
	'\x0c':   "FF",
	'\x0e':   "SO",
	'\x0f':   "SI",
	'\x10':   "DLE",
	'\x11':   "DC1",
	'\x12':   "DC2",
	'\x13':   "DC3",
	'\x14':   "DC4",
	'\x15':   "NAK",
	'\x16':   "SYN",
	'\x17':   "ETB",
	'\x18':   "CAN",
	'\x19':   "EM",
	'\x1a':   "SUB",
	'\x1b':   "ESC",
	'\x1c':   "FS",
	'\x1d':   "GS",
	'\x1e':   "RS",
	'\x1f':   "US",
	'\x7f':   "DEL",
	'\ufeff': "BOM",
}

// sanitizeIdentifier returns a name as a part of a C identifier. It keeps
// the ASCII letters, the digits and the underscores, writes a word for each
// special character, and writes the code point of any other character.
//
// sanitizeIdentifier is Generator::sanitize_identifier. Upstream holds the
// words in a match, and the port holds them in identifierReplacements.
func (g *generator) sanitizeIdentifier(nameID generate.StrID) string {
	name := g.strPool.Resolve(nameID)
	var result strings.Builder
	result.Grow(len(name))
	for _, c := range name {
		if isASCIIAlphanumeric(c) || c == '_' {
			result.WriteRune(c)
			continue
		}
		replacement, ok := identifierReplacements[c]
		switch {
		case c == ' ' && len(name) != 1:
			continue
		case ok:
		case c >= 0x80 && c <= 0xFFFF:
			fmt.Fprintf(&result, "u%04x", c)
			continue
		case c >= 0x10000:
			fmt.Fprintf(&result, "U%08x", c)
			continue
		}
		if s := result.String(); s != "" && !strings.HasSuffix(s, "_") {
			result.WriteByte('_')
		}
		result.WriteString(replacement)
	}
	return result.String()
}

// cString returns a name as the C code reads the string literal that
// sanitize_string writes for it: up to its first NUL.
//
// cString takes the place of Generator::sanitize_string, whose escapes the C
// compiler reads back as the same characters.
func cString(s string) string {
	before, _, _ := strings.Cut(s, "\x00")
	return before
}

// isASCIIAlphanumeric reports whether c is an ASCII letter or digit.
//
// isASCIIAlphanumeric is char::is_ascii_alphanumeric.
func isASCIIAlphanumeric(c rune) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// compareBool orders false before true, as Rust does.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	}
	return 1
}

// Backend is the Go backend. It writes parser.go.
type Backend struct {
	// Package is the name of the package, which PackageName gives for the
	// folder of the package.
	Package string
	// Queries is true when the folder of the package holds a file
	// queries/*.scm, which parser.go embeds.
	Queries bool
}

// Render returns the Go code of parser.go of a grammar. It does not change
// the RenderInput.
//
// Render is render_c_code, for Go.
func (b Backend) Render(in *generate.RenderInput) (string, error) {
	out, err := render(in)
	if err != nil {
		return "", err
	}
	return writeParser(out, b.Package, b.Queries), nil
}

// Tables returns the tables that Render writes for a grammar, with the lex
// functions of its lex tables and no external scanner. The test module
// compares them with the tables of the C backend (D12).
func Tables(in *generate.RenderInput) (*abi.Language, error) {
	out, err := render(in)
	if err != nil {
		return nil, err
	}
	t := out.tables
	t.LexFn = out.mainLex.Lex
	if out.keywordLex != nil {
		t.KeywordLexFn = out.keywordLex.Lex
	}
	return &t, nil
}

// render builds the output of a grammar.
func render(in *generate.RenderInput) (*output, error) {
	if in.ABIVersion < generate.ABIVersionMin || in.ABIVersion > generate.ABIVersionMax {
		return nil, &RenderError{Kind: RenderErrorABI, Value: in.ABIVersion}
	}

	g := &generator{
		out:                &output{},
		languageName:       in.StrPool.Resolve(in.Name),
		parseTable:         &in.Tables.ParseTable,
		mainLexTable:       &in.Tables.MainLexTable,
		keywordLexTable:    &in.Tables.KeywordLexTable,
		largeCharacterSets: in.Tables.LargeCharacterSets,
		syntaxGrammar:      in.SyntaxGrammar,
		lexicalGrammar:     in.LexicalGrammar,
		defaultAliases:     in.DefaultAliases,
		abiVersion:         in.ABIVersion,
		metadata:           in.SemanticVersion,
		supertypeSymbolMap: in.SupertypeSymbolMap,
		strPool:            in.StrPool,
		symbolOrder:        map[generate.Symbol]int{},
		symbolIDs:          map[generate.Symbol]string{},
		aliasIDs:           map[generate.Alias]string{},
		supertypeMap:       map[string][]generate.ChildType{},
		values:             map[string]uint16{},
	}
	out, err := g.generate()
	if err != nil {
		return nil, err
	}
	out.name = g.languageName
	return out, nil
}
