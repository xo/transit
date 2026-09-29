// Package cgrammar loads a grammar that is compiled from C, for the tests
// that compare the Go runtime with the C runtime (D12).
//
// A grammar is a shared library, built from its parser.c and its scanner.
// Load opens it with dlopen, copies the tables of its TSLanguage into an
// abi.Language, and gives the Go runtime a lex function and an external
// scanner that call the C code through cgo. The C runtime is a shared
// library too, which LoadRuntime opens, so the package builds with no
// checkout of upstream. The tests build both libraries, and they skip when
// the checkout of upstream or the cache of grammars is missing.
package cgrammar

/*
#cgo CFLAGS: -I${SRCDIR}/../../generate/templates
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// Grammar is a grammar that Load opened.
type Grammar struct {
	// Name is the name of the grammar, as in tree_sitter_<name>.
	Name string
	// Language is the language of the Go runtime, with the tables of the C
	// grammar.
	Language *transit.Language

	lang *C.TSLanguage
}

// TokenCount returns the number of the terminal symbols of the grammar. The
// symbols below it are tokens.
func (g *Grammar) TokenCount() int {
	return int(g.lang.token_count)
}

// Load opens the shared library of a grammar and returns the grammar that
// the function tree_sitter_<name> of the library gives.
func Load(path, name string) (*Grammar, error) {
	lib, err := dlopen(path)
	if err != nil {
		return nil, err
	}
	sym := "tree_sitter_" + name
	csym := C.CString(sym)
	defer C.free(unsafe.Pointer(csym))
	fn := C.bridge_dlsym(lib, csym)
	if fn == nil {
		return nil, fmt.Errorf("finding %s in %s: %s", sym, path, C.GoString(C.bridge_dlerror()))
	}
	lang := C.bridge_call_language(fn)
	if lang == nil {
		return nil, fmt.Errorf("calling %s of %s: no language", sym, path)
	}
	g := &Grammar{Name: name, lang: lang}
	g.Language = transit.NewLanguage(tables(lang))
	return g, nil
}

// dlopen opens a shared library.
func dlopen(path string) (unsafe.Pointer, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	lib := C.bridge_dlopen(cpath)
	if lib == nil {
		return nil, fmt.Errorf("opening %s: %s", path, C.GoString(C.bridge_dlerror()))
	}
	return lib, nil
}

// errNoRuntime is the error of a function of the C runtime before
// LoadRuntime.
var errNoRuntime = errors.New("the C runtime is not loaded")

// runtimeLoaded is true after LoadRuntime.
var runtimeLoaded bool

// LoadRuntime opens the shared library of the upstream C runtime, which is
// built from lib/src/lib.c.
func LoadRuntime(path string) error {
	lib, err := dlopen(path)
	if err != nil {
		return err
	}
	if missing := C.rt_load(lib); missing != nil {
		return fmt.Errorf("finding %s in %s", C.GoString(missing), path)
	}
	if err := loadQuery(lib); err != nil {
		return err
	}
	runtimeLoaded = true
	return nil
}

// array returns n elements of the C array at p, or nil when p is NULL.
func array[T any](p *T, n int) []T {
	if p == nil || n <= 0 {
		return nil
	}
	return unsafe.Slice(p, n)
}

// u16s copies n uint16 values from the C array at p.
func u16s(p *C.uint16_t, n int) []uint16 {
	out := make([]uint16, 0, max(n, 0))
	for _, v := range array(p, n) {
		out = append(out, uint16(v))
	}
	return out
}

// tables copies the tables of a TSLanguage into an abi.Language. A C table
// has no length, so tables finds each length from the counts, as the C
// runtime reads the table.
func tables(l *C.TSLanguage) *abi.Language {
	abiVersion := uint32(l.abi_version)
	symbolCount := int(l.symbol_count)
	aliasCount := int(l.alias_count)
	tokenCount := int(l.token_count)
	stateCount := int(l.state_count)
	largeStateCount := int(l.large_state_count)
	productionIDCount := int(l.production_id_count)
	fieldCount := int(l.field_count)
	maxAliasSequenceLength := int(l.max_alias_sequence_length)
	externalTokenCount := int(l.external_token_count)
	allSymbols := symbolCount + aliasCount

	t := &abi.Language{
		ABIVersion:             abiVersion,
		SymbolCount:            uint32(symbolCount),
		AliasCount:             uint32(aliasCount),
		TokenCount:             uint32(tokenCount),
		ExternalTokenCount:     uint32(externalTokenCount),
		StateCount:             uint32(stateCount),
		LargeStateCount:        uint32(largeStateCount),
		ProductionIDCount:      uint32(productionIDCount),
		FieldCount:             uint32(fieldCount),
		MaxAliasSequenceLength: uint16(maxAliasSequenceLength),
		KeywordCaptureToken:    uint16(l.keyword_capture_token),
	}

	t.ParseTable = u16s(l.parse_table, largeStateCount*symbolCount)

	// The small parse table ends at the end of the groups of its last state.
	for _, index := range array(l.small_parse_table_map, stateCount-largeStateCount) {
		t.SmallParseTableMap = append(t.SmallParseTableMap, uint32(index))
	}
	smallEnd := 0
	var small []C.uint16_t
	if l.small_parse_table != nil {
		small = unsafe.Slice(l.small_parse_table, 1<<30)
	}
	for _, index := range t.SmallParseTableMap {
		pos := int(index)
		groupCount := int(small[pos])
		pos++
		for range groupCount {
			symbols := int(small[pos+1])
			pos += 2 + symbols
		}
		smallEnd = max(smallEnd, pos)
	}
	t.SmallParseTable = u16s(l.small_parse_table, smallEnd)

	t.ParseActions = parseActions(l, t, tokenCount)

	for _, name := range array(l.symbol_names, allSymbols) {
		t.SymbolNames = append(t.SymbolNames, C.GoString(name))
	}
	if fieldCount > 0 {
		for i, name := range array(l.field_names, fieldCount+1) {
			if i == 0 || name == nil {
				t.FieldNames = append(t.FieldNames, "")
				continue
			}
			t.FieldNames = append(t.FieldNames, C.GoString(name))
		}
	}

	entries := 0
	for _, s := range array(l.field_map_slices, productionIDCount) {
		t.FieldMapSlices = append(t.FieldMapSlices, abi.MapSlice{Index: uint16(s.index), Length: uint16(s.length)})
		entries = max(entries, int(s.index)+int(s.length))
	}
	for _, e := range array(l.field_map_entries, entries) {
		t.FieldMapEntries = append(t.FieldMapEntries, abi.FieldMapEntry{
			FieldID:    uint16(e.field_id),
			ChildIndex: uint8(e.child_index),
			Inherited:  bool(e.inherited),
		})
	}

	for _, m := range array(l.symbol_metadata, allSymbols) {
		t.SymbolMetadata = append(t.SymbolMetadata, abi.SymbolMetadata{
			Visible:   bool(m.visible),
			Named:     bool(m.named),
			Supertype: bool(m.supertype),
		})
	}
	t.PublicSymbolMap = u16s(l.public_symbol_map, allSymbols)

	// The alias map is groups of a symbol, a count and the aliases, and a 0
	// ends it.
	if l.alias_map != nil {
		aliasMap := unsafe.Slice(l.alias_map, 1<<30)
		aliasEnd := 0
		for aliasMap[aliasEnd] != 0 {
			aliasEnd += 2 + int(aliasMap[aliasEnd+1])
		}
		t.AliasMap = u16s(l.alias_map, aliasEnd+1)
	}
	t.AliasSequences = u16s(l.alias_sequences, productionIDCount*maxAliasSequenceLength)

	// ABI 14 has TSLexMode, with no reserved words.
	reservedSets := 1
	if abiVersion < 15 {
		for _, m := range array((*C.TSLexMode)(unsafe.Pointer(l.lex_modes)), stateCount) {
			t.LexModes = append(t.LexModes, abi.LexerMode{
				LexState:         uint16(m.lex_state),
				ExternalLexState: uint16(m.external_lex_state),
			})
		}
	} else {
		for _, m := range array(l.lex_modes, stateCount) {
			t.LexModes = append(t.LexModes, abi.LexerMode{
				LexState:          uint16(m.lex_state),
				ExternalLexState:  uint16(m.external_lex_state),
				ReservedWordSetID: uint16(m.reserved_word_set_id),
			})
			reservedSets = max(reservedSets, int(m.reserved_word_set_id)+1)
		}
	}
	externalStates := 0
	for _, m := range t.LexModes {
		externalStates = max(externalStates, int(m.ExternalLexState)+1)
	}

	t.LexFn = lexFunc(l.lex_fn)
	t.KeywordLexFn = lexFunc(l.keyword_lex_fn)

	scanner := l.external_scanner
	if scanner.states != nil {
		for _, v := range array(scanner.states, externalStates*externalTokenCount) {
			t.ExternalScanner.States = append(t.ExternalScanner.States, bool(v))
		}
		t.ExternalScanner.SymbolMap = u16s(scanner.symbol_map, externalTokenCount)
		if scanner.create != nil {
			t.ExternalScanner.Create = scannerCreate(l)
		}
	}

	if abiVersion >= 14 {
		t.PrimaryStateIDs = u16s(l.primary_state_ids, stateCount)
	}
	if abiVersion >= 15 {
		t.Name = C.GoString(l.name)
		t.MaxReservedWordSetSize = uint16(l.max_reserved_word_set_size)
		t.ReservedWords = u16s(l.reserved_words, reservedSets*int(l.max_reserved_word_set_size))
		t.SupertypeCount = uint32(l.supertype_count)
		t.SupertypeSymbols = u16s(l.supertype_symbols, int(l.supertype_count))
		slicesLen := 0
		for _, s := range t.SupertypeSymbols {
			slicesLen = max(slicesLen, int(s)+1)
		}
		supertypeEntries := 0
		for _, s := range array(l.supertype_map_slices, slicesLen) {
			t.SupertypeMapSlices = append(t.SupertypeMapSlices, abi.MapSlice{Index: uint16(s.index), Length: uint16(s.length)})
			supertypeEntries = max(supertypeEntries, int(s.index)+int(s.length))
		}
		t.SupertypeMapEntries = u16s(l.supertype_map_entries, supertypeEntries)
		t.Metadata = abi.LanguageMetadata{
			MajorVersion: uint8(l.metadata.major_version),
			MinorVersion: uint8(l.metadata.minor_version),
			PatchVersion: uint8(l.metadata.patch_version),
		}
	}
	return t
}

// parseActions copies the parse actions of a language. Each value of the
// parse tables for a terminal symbol is the index of a group of actions, so
// the table ends after the group with the largest index.
func parseActions(l *C.TSLanguage, t *abi.Language, tokenCount int) []abi.ParseActionEntry {
	last := 0
	symbolCount := int(t.SymbolCount)
	for state := range int(t.LargeStateCount) {
		for symbol := range tokenCount {
			last = max(last, int(t.ParseTable[state*symbolCount+symbol]))
		}
	}
	for _, index := range t.SmallParseTableMap {
		pos := int(index)
		groupCount := int(t.SmallParseTable[pos])
		pos++
		for range groupCount {
			value := int(t.SmallParseTable[pos])
			symbols := int(t.SmallParseTable[pos+1])
			for _, s := range t.SmallParseTable[pos+2 : pos+2+symbols] {
				if int(s) < tokenCount {
					last = max(last, value)
				}
			}
			pos += 2 + symbols
		}
	}

	// TSParseActionEntry is a union of 8 bytes. The first entry of a group
	// is the count and the flag reusable, and each action after it has both
	// views of the union, shift and reduce, as C reads them.
	raw := unsafe.Slice((*[8]byte)(unsafe.Pointer(l.parse_actions)), last+1+256)
	n := last + 1 + int(raw[last][0])
	out := make([]abi.ParseActionEntry, n)
	for i := 0; i < n; {
		count := int(raw[i][0])
		out[i].Entry = abi.EntryHeader{Count: raw[i][0], Reusable: raw[i][1] != 0}
		for j := i + 1; j <= i+count && j < n; j++ {
			out[j].Action = decodeAction(raw[j])
		}
		i += 1 + count
	}
	return out
}

// decodeAction decodes a TSParseAction of 8 bytes, in the byte order of the
// machine, which is little endian on every platform that CI tests.
func decodeAction(b [8]byte) abi.ParseAction {
	u16 := func(i int) uint16 { return uint16(b[i]) | uint16(b[i+1])<<8 }
	return abi.ParseAction{
		Type: abi.ParseActionType(b[0]),
		Shift: abi.ShiftAction{
			State:      u16(2),
			Extra:      b[4] != 0,
			Repetition: b[5] != 0,
		},
		Reduce: abi.ReduceAction{
			ChildCount:        b[1],
			Symbol:            u16(2),
			DynamicPrecedence: int16(u16(4)),
			ProductionID:      u16(6),
		},
	}
}
