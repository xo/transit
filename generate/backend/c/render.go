package c

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/xo/transit/generate"
)

// This file ports crates/generate/src/render.rs: the code that writes
// parser.c from the tables of a grammar.
//
// Upstream keeps four FxHashMaps in the Generator: symbol_order, symbol_ids,
// alias_ids and symbol_map. The Generator only looks them up, and never
// iterates them, so the port uses Go maps. The FxHashMaps that are local to a
// method are iterated, and the method sorts the entries after that by a key
// that is unique. Each such method says so. The FxHashSet of assign_symbol_id
// is only looked up.

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

// The kinds of error, in the order of upstream.
const (
	// RenderErrorParseTable is a parse table with too many actions.
	RenderErrorParseTable RenderErrorKind = iota
	// RenderErrorABI is an ABI version that the backend does not write.
	RenderErrorABI
)

// RenderError is an error of the C backend. Its text is the text of
// upstream.
//
// RenderError is RenderError, an enum with data upstream.
type RenderError struct {
	Kind RenderErrorKind
	// Value is the number of actions of the parse table, for
	// RenderErrorParseTable, and the ABI version, for RenderErrorABI.
	Value int
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
	}
	return ""
}

// addf writes formatted text to the buffer.
//
// addf is the macro add.
func (g *generator) addf(format string, args ...any) {
	fmt.Fprintf(&g.buffer, format, args...)
}

// addWhitespace writes two spaces for each level of indent.
//
// addWhitespace is the macro add_whitespace.
func (g *generator) addWhitespace() {
	for range g.indentLevel {
		g.buffer.WriteString("  ")
	}
}

// addLinef writes the indent, formatted text and a newline.
//
// addLinef is the macro add_line.
func (g *generator) addLinef(format string, args ...any) {
	g.addWhitespace()
	fmt.Fprintf(&g.buffer, format, args...)
	g.buffer.WriteString("\n")
}

// indent adds a level of indent.
//
// indent is the macro indent.
func (g *generator) indent() {
	g.indentLevel++
}

// dedent removes a level of indent. It panics when there is none.
//
// dedent is the macro dedent.
func (g *generator) dedent() {
	if g.indentLevel == 0 {
		panic("c: dedent with no indent")
	}
	g.indentLevel--
}

// generator writes parser.c.
//
// generator is Generator. Upstream owns the tables and the grammars, and the
// port holds pointers to the ones in the RenderInput, which it does not
// change.
type generator struct {
	buffer                         bytes.Buffer
	indentLevel                    int
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
}

// largeCharacterSetInfo is the name of the constant of a large character
// set, and whether a lex state uses it.
//
// largeCharacterSetInfo is LargeCharacterSetInfo.
type largeCharacterSetInfo struct {
	constantName string
	isUsed       bool
}

// generate writes parser.c.
//
// generate is Generator::generate.
func (g *generator) generate() (string, error) {
	g.init()
	g.addHeader()
	g.addIncludes()
	g.addPragmas()
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

	bufferOffsetBeforeLexFunctions := g.buffer.Len()

	// upstream swaps each lex table out of the Generator, because
	// add_lex_function takes it by value, and the port passes a pointer
	g.addLexFunction("ts_lex", g.mainLexTable)

	if g.syntaxGrammar.HasWordToken {
		g.addLexFunction("ts_lex_keywords", g.keywordLexTable)
	}

	// Once the lex functions are generated, and we've determined which large
	// character sets are actually used, we can generate the large character set
	// constants. Insert them into the output buffer before the lex functions.
	lexFunctions := bytes.Clone(g.buffer.Bytes()[bufferOffsetBeforeLexFunctions:])
	g.buffer.Truncate(bufferOffsetBeforeLexFunctions)
	for ix := range g.largeCharacterSets {
		g.addCharacterSet(ix)
	}
	g.buffer.Write(lexFunctions)

	g.addLexModes()

	if g.abiVersion >= abiVersionWithReservedWords && len(g.reservedWordSets) > 1 {
		g.addReservedWordSets()
	}

	if err := g.addParseTable(); err != nil {
		return "", err
	}

	if len(g.syntaxGrammar.ExternalTokens) != 0 {
		g.addExternalTokenEnum()
		g.addExternalScannerSymbolMap()
		g.addExternalScannerStatesList()
	}

	g.addParserExport()

	return g.buffer.String(), nil
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

// addHeader writes the comment at the top of the file.
//
// addHeader is Generator::add_header.
func (g *generator) addHeader() {
	g.addLinef("/* Automatically @generated by tree-sitter */")
	g.addLinef("")
}

// addIncludes writes the include of parser.h.
//
// addIncludes is Generator::add_includes.
func (g *generator) addIncludes() {
	g.addLinef("#include \"tree_sitter/parser.h\"")
	g.addLinef("")
}

// addPragmas writes the pragmas of the compilers.
//
// addPragmas is Generator::add_pragmas.
func (g *generator) addPragmas() {
	g.addLinef("#if defined(__GNUC__) || defined(__clang__)")
	g.addLinef("#pragma GCC diagnostic ignored \"-Wmissing-field-initializers\"")
	g.addLinef("#endif")
	g.addLinef("")

	// Compiling large lexer functions with optimization can be very slow, so
	// disable most optimizations for large lexers. GCC 15 and later also
	// disable jump tables at O0, which makes the lexer much slower at runtime.
	if len(g.mainLexTable.States) > 300 {
		g.addLinef("#ifdef _MSC_VER")
		g.addLinef("#pragma optimize(\"\", off)")
		g.addLinef("#elif defined(__clang__)")
		g.addLinef("#pragma clang optimize off")
		g.addLinef("#elif defined(__GNUC__)")
		g.addLinef("#pragma GCC optimize (\"O0\", \"jump-tables\")")
		g.addLinef("#endif")
		g.addLinef("")
	}
}

// addStats writes the defines of the counts.
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

	g.addLinef("#define LANGUAGE_VERSION %d", g.abiVersion)
	g.addLinef("#define STATE_COUNT %d", len(g.parseTable.States))
	g.addLinef("#define LARGE_STATE_COUNT %d", g.largeStateCount)

	g.addLinef("#define SYMBOL_COUNT %d", len(g.parseTable.Symbols))
	g.addLinef("#define ALIAS_COUNT %d", len(g.uniqueAliases))
	g.addLinef("#define TOKEN_COUNT %d", tokenCount)
	g.addLinef("#define EXTERNAL_TOKEN_COUNT %d", len(g.syntaxGrammar.ExternalTokens))
	g.addLinef("#define FIELD_COUNT %d", len(g.fieldNames))
	g.addLinef("#define MAX_ALIAS_SEQUENCE_LENGTH %d", g.parseTable.MaxAliasedProductionLength)
	g.addLinef("#define MAX_RESERVED_WORD_SET_SIZE %d", g.maxReservedWordSetSize())

	g.addLinef("#define PRODUCTION_ID_COUNT %d", len(g.parseTable.ProductionInfos))
	g.addLinef("#define SUPERTYPE_COUNT %d", len(g.supertypeMap))
	g.addLinef("")
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

// addSymbolEnum writes the enum of the symbol identifiers, and gives each
// symbol its order.
//
// addSymbolEnum is Generator::add_symbol_enum.
func (g *generator) addSymbolEnum() {
	g.addLinef("enum ts_symbol_identifiers {")
	g.indent()
	g.symbolOrder[generate.SymbolEndValue] = 0
	i := 1
	for _, symbol := range g.parseTable.Symbols {
		if symbol != generate.SymbolEndValue {
			g.symbolOrder[symbol] = i
			g.addLinef("%s = %d,", g.symbolIDs[symbol], i)
			i++
		}
	}
	for _, alias := range g.uniqueAliases {
		g.addLinef("%s = %d,", g.aliasIDs[alias], i)
		i++
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addSymbolNamesList writes the name of each symbol.
//
// addSymbolNamesList is Generator::add_symbol_names_list.
func (g *generator) addSymbolNamesList() {
	g.addLinef("static const char * const ts_symbol_names[] = {")
	g.indent()
	for _, symbol := range g.parseTable.Symbols {
		var nameID generate.StrID
		if alias, ok := g.defaultAliases[symbol]; ok {
			nameID = alias.Value
		} else {
			nameID, _ = g.metadataForSymbol(symbol)
		}
		name := g.sanitizeString(nameID)
		g.addLinef("[%s] = \"%s\",", g.symbolIDs[symbol], name)
	}
	for _, alias := range g.uniqueAliases {
		g.addLinef("[%s] = \"%s\",", g.aliasIDs[alias], g.sanitizeString(alias.Value))
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addUniqueSymbolMap writes the public symbol of each symbol.
//
// addUniqueSymbolMap is Generator::add_unique_symbol_map.
func (g *generator) addUniqueSymbolMap() {
	g.addLinef("static const TSSymbol ts_symbol_map[] = {")
	g.indent()
	for _, symbol := range g.parseTable.Symbols {
		g.addLinef("[%s] = %s,", g.symbolIDs[symbol], g.symbolIDs[g.symbolMap[symbol]])
	}

	for _, alias := range g.uniqueAliases {
		g.addLinef("[%s] = %s,", g.aliasIDs[alias], g.aliasIDs[alias])
	}

	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addFieldNameEnum writes the enum of the field identifiers.
//
// addFieldNameEnum is Generator::add_field_name_enum.
func (g *generator) addFieldNameEnum() {
	g.addLinef("enum ts_field_identifiers {")
	g.indent()
	for i, fieldName := range g.fieldNames {
		g.addLinef("%s = %d,", fieldID(g.strPool.Resolve(fieldName)), i+1)
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addFieldNameNamesList writes the name of each field.
//
// addFieldNameNamesList is Generator::add_field_name_names_list.
func (g *generator) addFieldNameNamesList() {
	g.addLinef("static const char * const ts_field_names[] = {")
	g.indent()
	g.addLinef("[0] = NULL,")
	for _, fieldName := range g.fieldNames {
		name := g.strPool.Resolve(fieldName)
		g.addLinef("[%s] = \"%s\",", fieldID(name), name)
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addSymbolMetadataList writes whether each symbol is visible and named.
//
// addSymbolMetadataList is Generator::add_symbol_metadata_list.
func (g *generator) addSymbolMetadataList() {
	g.addLinef("static const TSSymbolMetadata ts_symbol_metadata[] = {")
	g.indent()
	for _, symbol := range g.parseTable.Symbols {
		g.addLinef("[%s] = {", g.symbolIDs[symbol])
		g.indent()
		if alias, ok := g.defaultAliases[symbol]; ok {
			g.addLinef(".visible = true,")
			g.addLinef(".named = %t,", alias.IsNamed)
		} else {
			_, kind := g.metadataForSymbol(symbol)
			switch kind {
			case generate.VariableNamed:
				g.addLinef(".visible = true,")
				g.addLinef(".named = true,")
			case generate.VariableAnonymous:
				g.addLinef(".visible = true,")
				g.addLinef(".named = false,")
			case generate.VariableHidden:
				g.addLinef(".visible = false,")
				g.addLinef(".named = true,")
				if slices.Contains(g.syntaxGrammar.SupertypeSymbols, symbol) {
					g.addLinef(".supertype = true,")
				}
			case generate.VariableAuxiliary:
				g.addLinef(".visible = false,")
				g.addLinef(".named = false,")
			}
		}
		g.dedent()
		g.addLinef("},")
	}
	for _, alias := range g.uniqueAliases {
		g.addLinef("[%s] = {", g.aliasIDs[alias])
		g.indent()
		g.addLinef(".visible = true,")
		g.addLinef(".named = %t,", alias.IsNamed)
		g.dedent()
		g.addLinef("},")
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addAliasSequences writes the alias of each child of each production.
//
// addAliasSequences is Generator::add_alias_sequences.
func (g *generator) addAliasSequences() {
	g.addLinef("static const TSSymbol ts_alias_sequences[PRODUCTION_ID_COUNT][MAX_ALIAS_SEQUENCE_LENGTH] = {")
	g.indent()
	for i := range g.parseTable.ProductionInfos {
		productionInfo := &g.parseTable.ProductionInfos[i]
		if len(productionInfo.AliasSequence) == 0 {
			// Work around MSVC's intolerance of empty array initializers by
			// explicitly zero-initializing the first element.
			if i == 0 {
				g.addLinef("[0] = {0},")
			}
			continue
		}

		g.addLinef("[%d] = {", i)
		g.indent()
		for j, alias := range productionInfo.AliasSequence {
			if alias != (generate.Alias{}) {
				g.addLinef("[%d] = %s,", j, g.aliasIDs[alias])
			}
		}
		g.dedent()
		g.addLinef("},")
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addNonTerminalAliasMap writes, for each non-terminal that a production
// aliases, its public symbol and the ids of its aliases.
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

	g.addLinef("static const uint16_t ts_non_terminal_alias_map[] = {")
	g.indent()
	for _, symbol := range symbols {
		aliasIDs := aliasIDsBySymbol[symbol]
		symbolID := g.symbolIDs[symbol]
		publicSymbolID := g.symbolIDs[g.symbolMap[symbol]]
		g.addLinef("%s, %d,", symbolID, 1+len(aliasIDs))
		g.indent()
		g.addLinef("%s,", publicSymbolID)
		for _, aliasID := range aliasIDs {
			g.addLinef("%s,", aliasID)
		}
		g.dedent()
	}
	g.addLinef("0,")
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
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
	g.addLinef("static const TSStateId ts_primary_state_ids[STATE_COUNT] = {")
	g.indent()
	firstStateForEachCoreID := map[uint32]int{}
	for idx := range g.parseTable.States {
		coreID := g.parseTable.States[idx].CoreID
		primaryState, ok := firstStateForEachCoreID[coreID]
		if !ok {
			primaryState = idx
			firstStateForEachCoreID[coreID] = idx
		}
		g.addLinef("[%d] = %d,", idx, primaryState)
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
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

// addFieldSequences writes the fields of each production.
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

	g.addLinef("static const TSMapSlice ts_field_map_slices[PRODUCTION_ID_COUNT] = {")
	g.indent()
	for productionID, id := range fieldMapIDs {
		if id.length > 0 {
			g.addLinef("[%d] = {.index = %d, .length = %d},", productionID, id.rowID, id.length)
		}
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")

	g.addLinef("static const TSFieldMapEntry ts_field_map_entries[] = {")
	g.indent()
	for _, row := range flatFieldMaps[1:] {
		g.addLinef("[%d] =", row.index)
		g.indent()
		for _, entry := range row.entries {
			g.addWhitespace()
			g.addf("{%s, %d", fieldID(g.strPool.Resolve(entry.name)), entry.location.Index)
			if entry.location.Inherited {
				g.addf(", .inherited = true")
			}
			g.addf("},\n")
		}
		g.dedent()
	}

	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addSupertypeMap writes the supertypes and the subtypes of each one.
//
// addSupertypeMap is Generator::add_supertype_map.
func (g *generator) addSupertypeMap() {
	supertypes := g.sortedSupertypes()
	g.addLinef("static const TSSymbol ts_supertype_symbols[SUPERTYPE_COUNT] = {")
	g.indent()
	for _, supertype := range supertypes {
		g.addLinef("%s,", supertype)
	}
	g.dedent()
	g.addLinef("};\n")

	g.addLinef("static const TSMapSlice ts_supertype_map_slices[] = {")
	g.indent()
	rowID := 0
	supertypeIDs := []int{0}
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
	for i, supertype := range supertypes {
		length := len(supertypeStringMap[i])
		g.addLinef("[%s] = {.index = %d, .length = %d},", supertype, rowID, length)
		rowID += length
		supertypeIDs = append(supertypeIDs, rowID)
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")

	g.addLinef("static const TSSymbol ts_supertype_map_entries[] = {")
	g.indent()
	for i, subtypes := range supertypeStringMap {
		rowIndex := supertypeIDs[i]
		g.addLinef("[%d] =", rowIndex)
		g.indent()
		for _, subtype := range subtypes {
			g.addWhitespace()
			g.addf("%s,\n", subtype)
		}
		g.dedent()
	}

	g.dedent()
	g.addLinef("};")
	g.addLinef("")
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

// addLexFunction writes a lex function.
//
// addLexFunction is Generator::add_lex_function.
func (g *generator) addLexFunction(name string, lexTable *generate.LexTable) {
	g.addLinef("static bool %s(TSLexer *lexer, TSStateId state) {", name)
	g.indent()

	g.addLinef("START_LEXER();")
	g.addLinef("eof = lexer->eof(lexer);")
	g.addLinef("switch (state) {")

	g.indent()
	for i := range lexTable.States {
		g.addLinef("case %d:", i)
		g.indent()
		g.addLexState(i, &lexTable.States[i])
		g.dedent()
	}

	g.addLinef("default:")
	g.indent()
	g.addLinef("return false;")
	g.dedent()

	g.dedent()
	g.addLinef("}")

	g.dedent()
	g.addLinef("}")
	g.addLinef("")
}

// bestLargeCharSet is a large character set that a transition uses, and the
// characters that the transition adds to it and removes from it.
//
// bestLargeCharSet is (usize, CharacterSet, CharacterSet).
type bestLargeCharSet struct {
	ix                  int
	additions, removals generate.CharacterSet
}

// addLexState writes one state of a lex function.
//
// addLexState is Generator::add_lex_state.
func (g *generator) addLexState(_ int, state *generate.LexState) {
	if state.HasAcceptAction {
		g.addLinef("ACCEPT_TOKEN(%s);", g.symbolIDs[state.AcceptAction])
	}

	if state.HasEOFAction {
		g.addLinef("if (eof) ADVANCE(%d);", state.EOFAction.State)
	}

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
		g.addLinef("ADVANCE_MAP(")
		g.indent()
		for _, advance := range state.AdvanceActions[:leadingSimpleTransitionCount] {
			for start, end := range advance.Chars.Ranges() {
				g.addWhitespace()
				g.addCharacter(start)
				g.addf(", %d,\n", advance.Action.State)
				if end > start {
					g.addWhitespace()
					g.addCharacter(end)
					g.addf(", %d,\n", advance.Action.State)
				}
			}
			ruledOutChars = ruledOutChars.Add(advance.Chars)
		}
		g.dedent()
		g.addLinef(");")
	} else {
		leadingSimpleTransitionCount = 0
	}

	for _, advance := range state.AdvanceActions[leadingSimpleTransitionCount:] {
		chars := advance.Chars
		g.addWhitespace()

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
			// Prefer the last candidate on ties. Searching backwards lets
			// us stop as soon as a match needs no additional checks.
			for ix, set := range slices.Backward(g.largeCharacterSets) {
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
						// Only a set that needs fewer checks replaces the best one, so on
						// a tie the set found first wins.
						if bestRangeCount <= totalRangeCount {
							continue
						}
					}
					best = &bestLargeCharSet{ix: ix, additions: additions, removals: removals}
					if totalRangeCount == 0 {
						break
					}
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

		lineBreak := "\n" + strings.Repeat("  ", g.indentLevel+2)

		hasPositiveCondition := hasLargeCharSet || !assertedChars.IsEmpty()
		hasNegativeCondition := !negatedChars.IsEmpty()
		hasCondition := hasPositiveCondition || hasNegativeCondition
		if hasCondition {
			g.addf("if (")
			if hasPositiveCondition && hasNegativeCondition {
				g.addf("(")
			}
		}

		if hasLargeCharSet {
			largeSet := g.largeCharacterSets[largeCharSetIx].Chars

			// If the character set contains the null character, check that we
			// are not at the end of the file.
			checkEOF := largeSet.Contains(0)
			if checkEOF {
				g.addf("(!eof && ")
			}

			charSetInfo := &g.largeCharacterSetInfo[largeCharSetIx]
			charSetInfo.isUsed = true
			g.addf("set_contains(%s, %d, lookahead)", charSetInfo.constantName, largeSet.RangeCount())
			if checkEOF {
				g.addf(")")
			}
		}

		if !assertedChars.IsEmpty() {
			if hasLargeCharSet {
				g.addf(" ||%s", lineBreak)
			}

			// If the character set contains the max character, then it probably
			// corresponds to a negated character class in a regex, so it will be more
			// concise and readable to express it in terms of negated ranges.
			isIncluded := !assertedChars.Contains(unicode.MaxRune)
			if !isIncluded {
				assertedChars = assertedChars.Negate().AddChar(0)
			}

			g.addCharacterRangeConditions(assertedChars, isIncluded, lineBreak)
		}

		if hasNegativeCondition {
			if hasPositiveCondition {
				g.addf(") &&%s", lineBreak)
			}
			g.addCharacterRangeConditions(negatedChars, false, lineBreak)
		}

		if hasCondition {
			g.addf(") ")
		}

		g.addAdvanceAction(advance.Action)
		g.addf("\n")
	}

	g.addLinef("END_STATE();")
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

// addCharacterRangeConditions writes the conditions on lookahead for a set of
// characters: that it is in the set when isIncluded is true, and that it is
// not when isIncluded is false.
//
// addCharacterRangeConditions is Generator::add_character_range_conditions.
func (g *generator) addCharacterRangeConditions(characters generate.CharacterSet, isIncluded bool, lineBreak string) {
	i := 0
	for start, end := range characters.Ranges() {
		if isIncluded {
			if i > 0 {
				g.addf(" ||%s", lineBreak)
			}

			switch {
			case start == 0:
				g.addf("(!eof && ")
				if end == 0 {
					g.addf("lookahead == 0")
				} else {
					g.addf("lookahead <= ")
				}
				g.addCharacter(end)
				g.addf(")")
			case end == start:
				g.addf("lookahead == ")
				g.addCharacter(start)
			case uint32(end) == uint32(start)+1:
				g.addf("lookahead == ")
				g.addCharacter(start)
				g.addf(" ||%slookahead == ", lineBreak)
				g.addCharacter(end)
			default:
				g.addf("(")
				g.addCharacter(start)
				g.addf(" <= lookahead && lookahead <= ")
				g.addCharacter(end)
				g.addf(")")
			}
		} else {
			if i > 0 {
				g.addf(" &&%s", lineBreak)
			}
			switch {
			case end == start:
				g.addf("lookahead != ")
				g.addCharacter(start)
			case uint32(end) == uint32(start)+1:
				g.addf("lookahead != ")
				g.addCharacter(start)
				g.addf(" &&%slookahead != ", lineBreak)
				g.addCharacter(end)
			case start != 0:
				g.addf("(lookahead < ")
				g.addCharacter(start)
				g.addf(" || ")
				g.addCharacter(end)
				g.addf(" < lookahead)")
			default:
				g.addf("lookahead > ")
				g.addCharacter(end)
			}
		}
		i++
	}
}

// addCharacterSet writes the constant of a large character set, when a lex
// state uses it.
//
// addCharacterSet is Generator::add_character_set.
func (g *generator) addCharacterSet(ix int) {
	characters := g.largeCharacterSets[ix].Chars
	info := &g.largeCharacterSetInfo[ix]
	if !info.isUsed {
		return
	}

	g.addLinef("static const TSCharacterRange %s[] = {", info.constantName)

	g.indent()
	ix = 0
	for start, end := range characters.Ranges() {
		column := ix % 8
		if column == 0 {
			if ix > 0 {
				g.addf("\n")
			}
			g.addWhitespace()
		} else {
			g.addf(" ")
		}
		g.addf("{")
		g.addCharacter(start)
		g.addf(", ")
		g.addCharacter(end)
		g.addf("},")
		ix++
	}
	g.addf("\n")
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addAdvanceAction writes the action of a transition.
//
// addAdvanceAction is Generator::add_advance_action.
func (g *generator) addAdvanceAction(action generate.AdvanceAction) {
	if action.InMainToken {
		g.addf("ADVANCE(%d);", action.State)
	} else {
		g.addf("SKIP(%d);", action.State)
	}
}

// addLexModes writes the lex state of each parse state.
//
// addLexModes is Generator::add_lex_modes.
func (g *generator) addLexModes() {
	lexMode := "TSLexMode"
	if g.abiVersion >= abiVersionWithReservedWords {
		lexMode = "TSLexerMode"
	}
	g.addLinef("static const %s ts_lex_modes[STATE_COUNT] = {", lexMode)
	g.indent()
	for i := range g.parseTable.States {
		state := &g.parseTable.States[i]
		g.addWhitespace()
		g.addf("[%d] = {", i)
		if state.IsEndOfNonTerminalExtra() {
			g.addf("(TSStateId)(-1),")
		} else {
			g.addf(".lex_state = %d", state.LexStateID)

			if state.ExternalLexStateID > 0 {
				g.addf(", .external_lex_state = %d", state.ExternalLexStateID)
			}

			if g.abiVersion >= abiVersionWithReservedWords {
				reservedWordSetID := g.reservedWordSetIDsByParseState[i]
				if reservedWordSetID != 0 {
					g.addf(", .reserved_word_set_id = %d", reservedWordSetID)
				}
			}
		}

		g.addf("},\n")
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addReservedWordSets writes the sets of reserved words.
//
// addReservedWordSets is Generator::add_reserved_word_sets.
func (g *generator) addReservedWordSets() {
	g.addLinef("static const TSSymbol ts_reserved_words[%d][MAX_RESERVED_WORD_SET_SIZE] = {", len(g.reservedWordSets))
	g.indent()
	for id := range g.reservedWordSets {
		if id == 0 {
			continue
		}
		g.addLinef("[%d] = {", id)
		g.indent()
		for token := range g.reservedWordSets[id].All() {
			g.addLinef("%s,", g.symbolIDs[token])
		}
		g.dedent()
		g.addLinef("},")
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addExternalTokenEnum writes the enum of the external tokens.
//
// addExternalTokenEnum is Generator::add_external_token_enum.
func (g *generator) addExternalTokenEnum() {
	g.addLinef("enum ts_external_scanner_symbol_identifiers {")
	g.indent()
	for i := range g.syntaxGrammar.ExternalTokens {
		g.addLinef("%s = %d,", g.externalTokenID(i), i)
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addExternalScannerSymbolMap writes the symbol of each external token.
//
// addExternalScannerSymbolMap is Generator::add_external_scanner_symbol_map.
func (g *generator) addExternalScannerSymbolMap() {
	g.addLinef("static const TSSymbol ts_external_scanner_symbol_map[EXTERNAL_TOKEN_COUNT] = {")
	g.indent()
	for i := range g.syntaxGrammar.ExternalTokens {
		token := &g.syntaxGrammar.ExternalTokens[i]
		idToken := generate.ExternalSymbol(i)
		if token.HasCorrespondingInternalToken {
			idToken = token.CorrespondingInternalToken
		}
		g.addLinef("[%s] = %s,", g.externalTokenID(i), g.symbolIDs[idToken])
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addExternalScannerStatesList writes the external tokens that are valid in
// each external lex state.
//
// addExternalScannerStatesList is
// Generator::add_external_scanner_states_list.
func (g *generator) addExternalScannerStatesList() {
	g.addLinef("static const bool ts_external_scanner_states[%d][EXTERNAL_TOKEN_COUNT] = {", len(g.parseTable.ExternalLexStates))
	g.indent()
	for i := range g.parseTable.ExternalLexStates {
		if !g.parseTable.ExternalLexStates[i].IsEmpty() {
			g.addLinef("[%d] = {", i)
			g.indent()
			for index := range g.parseTable.ExternalLexStates[i].Externals() {
				g.addLinef("[%s] = true,", g.externalTokenID(int(index)))
			}
			g.dedent()
			g.addLinef("},")
		}
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
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

// addParseTable writes the parse table: the large states, the small states
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

	g.addLinef("static const uint16_t ts_parse_table[LARGE_STATE_COUNT][SYMBOL_COUNT] = {")
	g.indent()

	var terminalEntries []terminalEntry
	var nonterminalEntries []nonterminalEntry

	for i := range g.parseTable.States[:g.largeStateCount] {
		state := &g.parseTable.States[i]
		g.addLinef("[STATE(%d)] = {", i)
		g.indent()

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
			g.addLinef("[%s] = STATE(%d),", g.symbolIDs[entry.symbol], target)
		}

		for _, entry := range terminalEntries {
			entryID := getParseActionListID(
				entry.id,
				&g.parseTable.ActionLists,
				parseTableEntries,
				&nextParseActionListIndex,
			)
			g.addLinef("[%s] = ACTIONS(%d),", g.symbolIDs[entry.symbol], entryID)
		}

		g.dedent()
		g.addLinef("},")
	}

	g.dedent()
	g.addLinef("};")
	g.addLinef("")

	if g.largeStateCount < len(g.parseTable.States) {
		g.addLinef("static const uint16_t ts_small_parse_table[] = {")
		g.indent()

		nextTableIndex := 0
		smallStateIndices := make([]int, 0, max(len(g.parseTable.States)-g.largeStateCount, 0))
		symbolsByValue := map[valueKey][]generate.Symbol{}
		for i := g.largeStateCount; i < len(g.parseTable.States); i++ {
			state := &g.parseTable.States[i]
			smallStateIndices = append(smallStateIndices, nextTableIndex)
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

			g.addLinef("[%d] = %d,", nextTableIndex, len(valuesWithSymbols))
			g.indent()
			nextTableIndex++

			for _, v := range valuesWithSymbols {
				nextTableIndex += 2 + len(v.symbols)
				if v.key.kind == generate.SymbolNonTerminal {
					g.addLinef("STATE(%d), %d,", v.key.value, len(v.symbols))
				} else {
					g.addLinef("ACTIONS(%d), %d,", v.key.value, len(v.symbols))
				}

				// the symbols of a group are unique
				slices.SortFunc(v.symbols, generate.CompareSymbol)
				g.indent()
				for _, symbol := range v.symbols {
					g.addLinef("%s,", g.symbolIDs[symbol])
				}
				g.dedent()
			}

			g.dedent()
		}

		g.dedent()
		g.addLinef("};")
		g.addLinef("")

		g.addLinef("static const uint32_t ts_small_parse_table_map[] = {")
		g.indent()
		for i := g.largeStateCount; i < len(g.parseTable.States); i++ {
			g.addLinef("[SMALL_STATE(%d)] = %d,", i, smallStateIndices[i-g.largeStateCount])
		}
		g.dedent()
		g.addLinef("};")
		g.addLinef("")
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
	g.addParseActionList(entries)

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

// addParseActionList writes the action lists.
//
// addParseActionList is Generator::add_parse_action_list.
func (g *generator) addParseActionList(parseTableEntries []parseTableEntry) {
	g.addLinef("static const TSParseActionEntry ts_parse_actions[] = {")
	g.indent()
	for _, entry := range parseTableEntries {
		actions := g.parseTable.ActionLists.Get(entry.id)
		g.addf("  [%d] = {.entry = {.count = %d, .reusable = %t}},", entry.index, len(actions), entry.id.Reusable())
		for _, action := range actions {
			g.addf(" ")
			switch action.Kind {
			case generate.ParseActionAccept:
				g.addf(" ACCEPT_INPUT()")
			case generate.ParseActionRecover:
				g.addf("RECOVER()")
			case generate.ParseActionShiftExtra:
				g.addf("SHIFT_EXTRA()")
			case generate.ParseActionShift:
				if action.IsRepetition {
					g.addf("SHIFT_REPEAT(%d)", action.State)
				} else {
					g.addf("SHIFT(%d)", action.State)
				}
			case generate.ParseActionReduce:
				g.addf("REDUCE(%s, %d, %d, %d)", g.symbolIDs[action.Symbol], action.ChildCount, action.DynamicPrecedence, action.ProductionID)
			}
			g.addf(",")
		}
		g.addf("\n")
	}
	g.dedent()
	g.addLinef("};")
	g.addLinef("")
}

// addParserExport writes the function that returns the language.
//
// addParserExport is Generator::add_parser_export.
func (g *generator) addParserExport() {
	languageFunctionName := "tree_sitter_" + g.languageName
	externalScannerName := languageFunctionName + "_external_scanner"

	g.addLinef("#ifdef __cplusplus")
	g.addLinef(`extern "C" {`)
	g.addLinef("#endif")

	if len(g.syntaxGrammar.ExternalTokens) != 0 {
		g.addLinef("void *%s_create(void);", externalScannerName)
		g.addLinef("void %s_destroy(void *);", externalScannerName)
		g.addLinef("bool %s_scan(void *, TSLexer *, const bool *);", externalScannerName)
		g.addLinef("unsigned %s_serialize(void *, char *);", externalScannerName)
		g.addLinef("void %s_deserialize(void *, const char *, unsigned);", externalScannerName)
		g.addLinef("")
	}

	g.addLinef("#ifdef TREE_SITTER_HIDE_SYMBOLS")
	g.addLinef("#define TS_PUBLIC")
	g.addLinef("#elif defined(_WIN32)")
	g.addLinef("#define TS_PUBLIC __declspec(dllexport)")
	g.addLinef("#else")
	g.addLinef("#define TS_PUBLIC __attribute__((visibility(\"default\")))")
	g.addLinef("#endif")
	g.addLinef("")

	g.addLinef("TS_PUBLIC const TSLanguage *%s(void) {", languageFunctionName)
	g.indent()
	g.addLinef("static const TSLanguage language = {")
	g.indent()
	g.addLinef(".abi_version = LANGUAGE_VERSION,")

	// Quantities
	g.addLinef(".symbol_count = SYMBOL_COUNT,")
	g.addLinef(".alias_count = ALIAS_COUNT,")
	g.addLinef(".token_count = TOKEN_COUNT,")
	g.addLinef(".external_token_count = EXTERNAL_TOKEN_COUNT,")
	g.addLinef(".state_count = STATE_COUNT,")
	g.addLinef(".large_state_count = LARGE_STATE_COUNT,")
	g.addLinef(".production_id_count = PRODUCTION_ID_COUNT,")
	if g.abiVersion >= abiVersionWithReservedWords {
		g.addLinef(".supertype_count = SUPERTYPE_COUNT,")
	}
	g.addLinef(".field_count = FIELD_COUNT,")
	g.addLinef(".max_alias_sequence_length = MAX_ALIAS_SEQUENCE_LENGTH,")

	// Parse table
	g.addLinef(".parse_table = &ts_parse_table[0][0],")
	if g.largeStateCount < len(g.parseTable.States) {
		g.addLinef(".small_parse_table = ts_small_parse_table,")
		g.addLinef(".small_parse_table_map = ts_small_parse_table_map,")
	}
	g.addLinef(".parse_actions = ts_parse_actions,")

	// Metadata
	g.addLinef(".symbol_names = ts_symbol_names,")
	if len(g.fieldNames) != 0 {
		g.addLinef(".field_names = ts_field_names,")
		g.addLinef(".field_map_slices = ts_field_map_slices,")
		g.addLinef(".field_map_entries = ts_field_map_entries,")
	}
	if len(g.supertypeMap) != 0 && g.abiVersion >= abiVersionWithReservedWords {
		g.addLinef(".supertype_map_slices = ts_supertype_map_slices,")
		g.addLinef(".supertype_map_entries = ts_supertype_map_entries,")
		g.addLinef(".supertype_symbols = ts_supertype_symbols,")
	}
	g.addLinef(".symbol_metadata = ts_symbol_metadata,")
	g.addLinef(".public_symbol_map = ts_symbol_map,")
	g.addLinef(".alias_map = ts_non_terminal_alias_map,")
	if len(g.parseTable.ProductionInfos) != 0 {
		g.addLinef(".alias_sequences = &ts_alias_sequences[0][0],")
	}

	// Lexing
	g.addLinef(".lex_modes = (const void*)ts_lex_modes,")
	g.addLinef(".lex_fn = ts_lex,")
	if g.syntaxGrammar.HasWordToken {
		g.addLinef(".keyword_lex_fn = ts_lex_keywords,")
		g.addLinef(".keyword_capture_token = %s,", g.symbolIDs[g.syntaxGrammar.WordToken])
	}

	if len(g.syntaxGrammar.ExternalTokens) != 0 {
		g.addLinef(".external_scanner = {")
		g.indent()
		g.addLinef("&ts_external_scanner_states[0][0],")
		g.addLinef("ts_external_scanner_symbol_map,")
		g.addLinef("%s_create,", externalScannerName)
		g.addLinef("%s_destroy,", externalScannerName)
		g.addLinef("%s_scan,", externalScannerName)
		g.addLinef("%s_serialize,", externalScannerName)
		g.addLinef("%s_deserialize,", externalScannerName)
		g.dedent()
		g.addLinef("},")
	}

	g.addLinef(".primary_state_ids = ts_primary_state_ids,")

	if g.abiVersion >= abiVersionWithReservedWords {
		g.addLinef(".name = \"%s\",", g.languageName)

		if len(g.reservedWordSets) > 1 {
			g.addLinef(".reserved_words = &ts_reserved_words[0][0],")
		}

		g.addLinef(".max_reserved_word_set_size = %d,", g.maxReservedWordSetSize())

		var metadata generate.SemanticVersion
		if g.metadata != nil {
			metadata = *g.metadata
		}

		g.addLinef(".metadata = {")
		g.indent()
		g.addLinef(".major_version = %d,", metadata.Major)
		g.addLinef(".minor_version = %d,", metadata.Minor)
		g.addLinef(".patch_version = %d,", metadata.Patch)
		g.dedent()
		g.addLinef("},")
	}

	g.dedent()
	g.addLinef("};")
	g.addLinef("return &language;")
	g.dedent()
	g.addLinef("}")
	g.addLinef("#ifdef __cplusplus")
	g.addLinef("}")
	g.addLinef("#endif")
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

// externalTokenID returns the C identifier of an external token.
//
// externalTokenID is Generator::external_token_id.
func (g *generator) externalTokenID(tokenIdx int) string {
	token := &g.syntaxGrammar.ExternalTokens[tokenIdx]
	return "ts_external_token_" + g.sanitizeIdentifier(token.Name)
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
	panic("c: a symbol of an unknown kind")
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

// sanitizeString returns a name as the text of a C string literal.
//
// sanitizeString is Generator::sanitize_string.
func (g *generator) sanitizeString(nameID generate.StrID) string {
	name := g.strPool.Resolve(nameID)
	var result strings.Builder
	result.Grow(len(name))
	for _, c := range name {
		switch {
		case c == '"':
			result.WriteString("\\\"")
		case c == '?':
			result.WriteString("\\?")
		case c == '\\':
			result.WriteString("\\\\")
		case c == '\a':
			result.WriteString("\\a")
		case c == '\b':
			result.WriteString("\\b")
		case c == '\v':
			result.WriteString("\\v")
		case c == '\f':
			result.WriteString("\\f")
		case c == '\n':
			result.WriteString("\\n")
		case c == '\r':
			result.WriteString("\\r")
		case c == '\t':
			result.WriteString("\\t")
		case c == 0:
			result.WriteString("\\0")
		case c >= 0x01 && c <= 0x1f:
			fmt.Fprintf(&result, "\\x%02x", c)
		case c >= 0x7f && c <= 0xFFFF:
			fmt.Fprintf(&result, "\\u%04x", c)
		case c >= 0x10000 && c <= unicode.MaxRune:
			fmt.Fprintf(&result, "\\U%08x", c)
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

// addCharacter writes a character as a C expression.
//
// addCharacter is Generator::add_character.
func (g *generator) addCharacter(c rune) {
	switch c {
	case '\'':
		g.addf(`'\''`)
	case '\\':
		g.addf(`'\\'`)
	case '\f':
		g.addf(`'\f'`)
	case '\n':
		g.addf(`'\n'`)
	case '\t':
		g.addf(`'\t'`)
	case '\r':
		g.addf(`'\r'`)
	default:
		switch {
		case c == 0:
			g.addf("0")
		case c == ' ' || isASCIIGraphic(c):
			g.addf("'%c'", c)
		default:
			g.addf("0x%02x", c)
		}
	}
}

// isASCIIAlphanumeric reports whether c is an ASCII letter or digit.
//
// isASCIIAlphanumeric is char::is_ascii_alphanumeric.
func isASCIIAlphanumeric(c rune) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isASCIIGraphic reports whether c is an ASCII character that prints and is
// not a space.
//
// isASCIIGraphic is char::is_ascii_graphic.
func isASCIIGraphic(c rune) bool {
	return c >= '!' && c <= '~'
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

// Backend is the C backend. It writes parser.c.
type Backend struct{}

// Render returns the C code of the parser of a grammar. It does not change
// the RenderInput.
//
// The arguments of upstream are the fields of in:
//
//   - Name is the name of the language.
//   - Tables holds the parse table, the main lex table, the keyword lex
//     table and the large character sets of the language.
//   - SyntaxGrammar and LexicalGrammar are the grammars that the generator
//     extracts from the grammar of the language.
//   - DefaultAliases maps each symbol that is always aliased in the same way
//     to its alias.
//   - StrPool holds the strings of the ids in the grammars and the aliases.
//   - ABIVersion is the ABI version of the language that the code is for.
//     This is usually the current version of tree-sitter, but right after an
//     ABI change, the previous one can be useful.
//
// Render is render_c_code.
func (Backend) Render(in *generate.RenderInput) (string, error) {
	if in.ABIVersion < generate.ABIVersionMin || in.ABIVersion > generate.ABIVersionMax {
		return "", &RenderError{Kind: RenderErrorABI, Value: in.ABIVersion}
	}

	g := &generator{
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
	}
	return g.generate()
}
