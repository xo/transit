package golang

import (
	"fmt"
	"go/format"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xo/transit/internal/abi"
)

// This file writes the output of the generator as parser.go. It ports no
// upstream file. The small part of the file, with the functions, the
// constants and the tables of the symbols, goes through go/format. The large
// tables come after it, in a form that gofmt keeps as it is, so the whole
// file is what gofmt writes. go/format would need too much memory for the
// tables of a large grammar.

// goNames gives each C identifier of an enum its Go name, with no repeats.
type goNames map[string]bool

// newGoNames returns an empty set of Go names.
func newGoNames() goNames {
	return goNames{}
}

// goPrefixes maps each prefix of a C identifier that render.rs writes to
// the prefix of the Go name. A longer prefix comes before a shorter one that
// it ends with.
var goPrefixes = []struct{ c, goName string }{
	{"ts_builtin_sym_", "BuiltinSym"},
	{"anon_alias_sym_", "AnonAliasSym"},
	{"alias_sym_", "AliasSym"},
	{"anon_sym_", "AnonSym"},
	{"aux_sym_", "AuxSym"},
	{"sym_", "Sym"},
	{"field_", "Field"},
}

// name returns the Go name of a C identifier. The prefix of the identifier
// becomes a Go prefix, and each part of the rest between two underscores
// starts with an upper case letter. A Go name that an earlier identifier
// has gets the number 2, 3 and so on at its end, as assign_symbol_id does
// for a C identifier.
func (n goNames) name(cID string) string {
	var b strings.Builder
	rest := cID
	for _, p := range goPrefixes {
		if after, ok := strings.CutPrefix(cID, p.c); ok {
			b.WriteString(p.goName)
			rest = after
			break
		}
	}
	for part := range strings.SplitSeq(rest, "_") {
		if part == "" {
			continue
		}
		if c := part[0]; c >= 'a' && c <= 'z' {
			b.WriteByte(c - 'a' + 'A')
			part = part[1:]
		}
		b.WriteString(part)
	}
	base := b.String()
	name := base
	for i := 2; n[name]; i++ {
		name = base + strconv.Itoa(i)
	}
	n[name] = true
	return name
}

// PackageName returns the name of the Go package of the grammar package in
// the folder dir: the name of the folder in lowercase, as docs/GRAMMAR.md
// says (D107). The folder sqlserver of the grammar TSQL gives the package
// sqlserver. The folder go gives the package golang, as D26 names the
// package of the Go backend (D77). In the Go module cache, the folder of a
// module ends with "@" and the version, such as json@v0.1.0, and the name
// ends before the "@". PackageName returns an error when the result is not
// a name that a Go package can have.
func PackageName(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("finding the folder %s: %w", dir, err)
	}
	folder, _, _ := strings.Cut(filepath.Base(abs), "@")
	name := strings.ToLower(folder)
	if name == "go" {
		return "golang", nil
	}
	if !token.IsIdentifier(name) {
		return "", fmt.Errorf("naming the Go package of the folder %s: %q %w", abs, name, errPackageName)
	}
	return name, nil
}

// errPackageName is the error of a folder whose name gives no Go package
// name, such as func, which is a keyword of Go.
var errPackageName = constError("is not a name of a Go package")

// constError is an error that is a constant.
type constError string

// Error returns the text of the error.
func (e constError) Error() string {
	return string(e)
}

// writer writes Go source.
type writer struct {
	strings.Builder
}

// comment writes text as a comment of lines of at most 80 columns. A word
// longer than a line stays on a line of its own.
func (w *writer) comment(text string) {
	line := "//"
	for word := range strings.FieldsSeq(text) {
		if len(line) > len("//") && len(line)+1+len(word) > 80 {
			w.printf("%s\n", line)
			line = "//"
		}
		line += " " + word
	}
	w.printf("%s\n", line)
}

// printf writes formatted text. A strings.Builder never fails to write.
func (w *writer) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// writeParser returns the text of parser.go of the package pkg for the
// output of a grammar. queries is true when the package holds
// queries/*.scm.
func writeParser(out *output, pkg string, queries bool) string {
	w := &writer{}
	writeHeader(w, out, pkg, queries)
	writeConstants(w, out)
	writeSymbolTables(w, out)
	head, err := format.Source([]byte(w.String()))
	if err != nil {
		// the generator writes the text, so it is valid Go
		panic("golang: formatting parser.go: " + err.Error())
	}

	w = &writer{}
	w.printf("%s\n", head)
	writeLargeTables(w, &out.tables)
	writeLexTable(w, "mainLexTable", "lex", "the main lex function", out.mainLex)
	if out.keywordLex != nil {
		writeLexTable(w, "keywordLexTable", "lexKeywords", "the lex function of the keywords", out.keywordLex)
	}
	return strings.TrimRight(w.String(), "\n") + "\n"
}

// writeHeader writes the comment of the file, the package clause, the
// imports, and the functions that the package exports.
func writeHeader(w *writer, out *output, pkg string, queries bool) {
	t := &out.tables
	w.printf("// Code generated by transit. DO NOT EDIT.\n\n")
	w.comment(fmt.Sprintf("Package %s is the grammar %s for the Go runtime of transit. Language returns its language, and the constants name its symbols and its fields.", pkg, out.name))
	w.printf("package %s\n\n", pkg)
	w.printf("import (\n\t\"embed\"\n\t\"encoding/json\"\n\t\"sync\"\n\n\t\"github.com/xo/transit\"\n\t\"github.com/xo/transit/internal/abi\"\n)\n\n")

	doc := fmt.Sprintf("Language returns the language of the grammar %s.", out.name)
	if out.hasExternalScanner {
		doc += " Its external scanner is the scanner that newScanner returns."
	}
	w.comment(doc)
	w.printf("func Language() *transit.Language {\n\treturn language()\n}\n\n")
	w.printf("// language builds the language once. The tables are static data, so this\n// copies no table.\n")
	w.printf("var language = sync.OnceValue(func() *transit.Language {\n\treturn transit.NewLanguage(&abi.Language{\n")
	field := func(name, value string) {
		w.printf("\t\t%s: %s,\n", name, value)
	}
	number := func(name string, v uint32) {
		field(name, strconv.FormatUint(uint64(v), 10))
	}
	table := func(name string, n int) {
		if n > 0 {
			field(name, lowerFirst(name))
		}
	}
	number("ABIVersion", t.ABIVersion)
	number("SymbolCount", t.SymbolCount)
	number("AliasCount", t.AliasCount)
	number("TokenCount", t.TokenCount)
	number("ExternalTokenCount", t.ExternalTokenCount)
	number("StateCount", t.StateCount)
	number("LargeStateCount", t.LargeStateCount)
	number("ProductionIDCount", t.ProductionIDCount)
	number("FieldCount", t.FieldCount)
	number("MaxAliasSequenceLength", uint32(t.MaxAliasSequenceLength))
	table("ParseTable", len(t.ParseTable))
	table("SmallParseTable", len(t.SmallParseTable))
	table("SmallParseTableMap", len(t.SmallParseTableMap))
	table("ParseActions", len(t.ParseActions))
	table("SymbolNames", len(t.SymbolNames))
	table("FieldNames", len(t.FieldNames))
	table("FieldMapSlices", len(t.FieldMapSlices))
	table("FieldMapEntries", len(t.FieldMapEntries))
	table("SymbolMetadata", len(t.SymbolMetadata))
	table("PublicSymbolMap", len(t.PublicSymbolMap))
	table("AliasMap", len(t.AliasMap))
	table("AliasSequences", len(t.AliasSequences))
	table("LexModes", len(t.LexModes))
	field("LexFn", "lex")
	if out.keywordLex != nil {
		field("KeywordLexFn", "lexKeywords")
	}
	number("KeywordCaptureToken", uint32(t.KeywordCaptureToken))
	if out.hasExternalScanner {
		w.printf("\t\tExternalScanner: abi.ExternalScanner{\n")
		w.printf("\t\t\tStates: externalScannerStates,\n")
		w.printf("\t\t\tSymbolMap: externalScannerSymbolMap,\n")
		w.printf("\t\t\tCreate: func() abi.Scanner { return newScanner() },\n")
		w.printf("\t\t},\n")
	}
	table("PrimaryStateIDs", len(t.PrimaryStateIDs))
	if t.Name != "" {
		field("Name", strconv.Quote(t.Name))
	}
	table("ReservedWords", len(t.ReservedWords))
	number("MaxReservedWordSetSize", uint32(t.MaxReservedWordSetSize))
	number("SupertypeCount", t.SupertypeCount)
	table("SupertypeSymbols", len(t.SupertypeSymbols))
	table("SupertypeMapSlices", len(t.SupertypeMapSlices))
	table("SupertypeMapEntries", len(t.SupertypeMapEntries))
	if t.Metadata != (abi.LanguageMetadata{}) {
		field("Metadata", fmt.Sprintf("abi.LanguageMetadata{MajorVersion: %d, MinorVersion: %d, PatchVersion: %d}",
			t.Metadata.MajorVersion, t.Metadata.MinorVersion, t.Metadata.PatchVersion))
	}
	w.printf("\t})\n})\n\n")

	if queries {
		w.comment(fmt.Sprintf("Queries holds queries/ of the grammar %s: its own queries, and in queries/<grammar>/ the queries of another grammar that its tree-sitter.json lists (D84).", out.name))
		w.printf("//\n//go:embed queries\n")
	} else {
		w.printf("// Queries is empty, because the grammar %s has no queries/*.scm.\n", out.name)
	}
	w.printf("var Queries embed.FS\n\n")

	w.printf("// nodeTypes is node-types.json of the grammar.\n//\n//go:embed node-types.json\nvar nodeTypes []byte\n\n")
	w.printf("// NodeTypes returns the node types of node-types.json of the grammar %s: the\n", out.name)
	w.printf("// fields and the children of each node, and the subtypes of each supertype.\n")
	w.printf("// Each call returns a new slice.\n")
	w.printf("func NodeTypes() []transit.NodeType {\n\tvar types []transit.NodeType\n")
	w.printf("\tif err := json.Unmarshal(nodeTypes, &types); err != nil {\n")
	w.printf("\t\t// the generator writes node-types.json, so it is valid\n")
	w.printf("\t\tpanic(%s + err.Error())\n\t}\n\treturn types\n}\n\n", strconv.Quote(pkg+": reading node-types.json: "))

	w.printf("// Keywords returns the keywords of the grammar %s: the tokens that the word\n", out.name)
	w.printf("// token captures, and the reserved words. Each call returns a new slice.\n")
	w.printf("func Keywords() []string {\n")
	if len(out.keywords) == 0 {
		w.printf("\treturn nil\n}\n\n")
	} else {
		w.printf("\treturn []string{\n")
		for _, k := range out.keywords {
			w.printf("\t\t%s,\n", strconv.Quote(k))
		}
		w.printf("\t}\n}\n\n")
	}
}

// writeConstants writes the constants of the symbols and of the fields.
func writeConstants(w *writer, out *output) {
	w.printf("// The symbols of the grammar %s, as Node.KindID and Node.GrammarID\n", out.name)
	w.printf("// return them. The comment of each one is its name.\n")
	w.printf("const (\n")
	for _, c := range out.symbols {
		w.printf("\t%s transit.Symbol = %d // %s\n", c.name, c.value, strconv.Quote(c.display))
	}
	w.printf(")\n\n")
	if len(out.fields) == 0 {
		return
	}
	w.printf("// The fields of the grammar %s, as Node.ChildByFieldID takes them. The\n", out.name)
	w.printf("// comment of each one is its name.\n")
	w.printf("const (\n")
	for _, c := range out.fields {
		w.printf("\t%s transit.FieldID = %d // %s\n", c.name, c.value, strconv.Quote(c.display))
	}
	w.printf(")\n\n")
}

// writeSymbolTables writes the names and the metadata of the symbols, and
// the names of the fields, by their constants.
func writeSymbolTables(w *writer, out *output) {
	t := &out.tables
	w.printf("// The short names of the types of the tables.\n")
	w.printf("type (\n\taction = abi.ParseAction\n\theader = abi.EntryHeader\n\tshift = abi.ShiftAction\n\treduce = abi.ReduceAction\n)\n\n")
	w.printf("// The kinds of a parse action.\n")
	w.printf("const (\n\ttypeShift = abi.ParseActionTypeShift\n\ttypeReduce = abi.ParseActionTypeReduce\n\ttypeAccept = abi.ParseActionTypeAccept\n\ttypeRecover = abi.ParseActionTypeRecover\n)\n\n")

	w.printf("// symbolNames is ts_symbol_names.\nvar symbolNames = []string{\n")
	for _, c := range out.symbols {
		w.printf("\t%s: %s,\n", c.name, strconv.Quote(t.SymbolNames[c.value]))
	}
	w.printf("}\n\n")

	w.printf("// symbolMetadata is ts_symbol_metadata.\nvar symbolMetadata = []abi.SymbolMetadata{\n")
	for _, c := range out.symbols {
		m := t.SymbolMetadata[c.value]
		var parts []string
		if m.Visible {
			parts = append(parts, "Visible: true")
		}
		if m.Named {
			parts = append(parts, "Named: true")
		}
		if m.Supertype {
			parts = append(parts, "Supertype: true")
		}
		w.printf("\t%s: {%s},\n", c.name, strings.Join(parts, ", "))
	}
	w.printf("}\n\n")

	if len(t.FieldNames) > 0 {
		w.printf("// fieldNames is ts_field_names. The name at index 0 is empty, as it is\n// NULL in C.\nvar fieldNames = []string{\n")
		for _, c := range out.fields {
			w.printf("\t%s: %s,\n", c.name, strconv.Quote(t.FieldNames[c.value]))
		}
		w.printf("}\n\n")
	}
}

// lowerFirst returns a name with its first letter in lower case, the name of
// the package variable of a table.
func lowerFirst(name string) string {
	return strings.ToLower(name[:1]) + name[1:]
}

// numbers writes a table of numbers, 16 on a line. rowLen is the length of
// a row of the table that gets a comment with the row number, such as a
// state of the parse table, or 0 for none.
func numbers[T ~uint16 | ~uint32](w *writer, name, doc, typ string, v []T, rowLen int) {
	if len(v) == 0 {
		return
	}
	w.printf("// %s is %s.\nvar %s = []%s{", name, doc, name, typ)
	for i, x := range v {
		col := i % 16
		if rowLen > 0 {
			col = i % rowLen % 16
			if i%rowLen == 0 {
				w.printf("\n\t// state %d", i/rowLen)
			}
		}
		if col == 0 {
			w.printf("\n\t")
		} else {
			w.printf(" ")
		}
		w.printf("%d,", x)
	}
	w.printf("\n}\n\n")
}

// writeLargeTables writes the tables that go/format does not read, each in a
// form that gofmt keeps as it is.
func writeLargeTables(w *writer, t *abi.Language) {
	numbers(w, "parseTable", "ts_parse_table, one row of SymbolCount entries for each large state", "uint16", t.ParseTable, int(t.SymbolCount))
	writeSmallParseTable(w, t)
	numbers(w, "smallParseTableMap", "ts_small_parse_table_map", "uint32", t.SmallParseTableMap, 0)
	writeParseActions(w, t)

	if len(t.FieldMapSlices) > 0 {
		w.printf("// fieldMapSlices is ts_field_map_slices.\nvar fieldMapSlices = []abi.MapSlice{\n")
		for _, s := range t.FieldMapSlices {
			w.printf("\t%s,\n", mapSlice(s))
		}
		w.printf("}\n\n")
	}
	if len(t.FieldMapEntries) > 0 {
		w.printf("// fieldMapEntries is ts_field_map_entries.\nvar fieldMapEntries = []abi.FieldMapEntry{\n")
		for _, e := range t.FieldMapEntries {
			w.printf("\t{FieldID: %d, ChildIndex: %d", e.FieldID, e.ChildIndex)
			if e.Inherited {
				w.printf(", Inherited: true")
			}
			w.printf("},\n")
		}
		w.printf("}\n\n")
	}

	numbers(w, "publicSymbolMap", "ts_symbol_map", "uint16", t.PublicSymbolMap, 0)
	numbers(w, "aliasMap", "ts_non_terminal_alias_map", "uint16", t.AliasMap, 0)
	numbers(w, "aliasSequences", "ts_alias_sequences, MaxAliasSequenceLength entries for each production", "uint16", t.AliasSequences, 0)

	w.printf("// lexModes is ts_lex_modes.\nvar lexModes = []abi.LexerMode{\n")
	for _, m := range t.LexModes {
		var parts []string
		if m.LexState != 0 {
			parts = append(parts, fmt.Sprintf("LexState: %d", m.LexState))
		}
		if m.ExternalLexState != 0 {
			parts = append(parts, fmt.Sprintf("ExternalLexState: %d", m.ExternalLexState))
		}
		if m.ReservedWordSetID != 0 {
			parts = append(parts, fmt.Sprintf("ReservedWordSetID: %d", m.ReservedWordSetID))
		}
		w.printf("\t{%s},\n", strings.Join(parts, ", "))
	}
	w.printf("}\n\n")

	numbers(w, "primaryStateIDs", "ts_primary_state_ids", "uint16", t.PrimaryStateIDs, 0)
	numbers(w, "reservedWords", "ts_reserved_words, MaxReservedWordSetSize entries for each set", "uint16", t.ReservedWords, 0)
	numbers(w, "supertypeSymbols", "ts_supertype_symbols", "uint16", t.SupertypeSymbols, 0)
	if len(t.SupertypeMapSlices) > 0 {
		w.printf("// supertypeMapSlices is ts_supertype_map_slices.\nvar supertypeMapSlices = []abi.MapSlice{\n")
		for _, s := range t.SupertypeMapSlices {
			w.printf("\t%s,\n", mapSlice(s))
		}
		w.printf("}\n\n")
	}
	numbers(w, "supertypeMapEntries", "ts_supertype_map_entries", "uint16", t.SupertypeMapEntries, 0)

	if len(t.ExternalScanner.States) > 0 {
		w.printf("// externalScannerStates is ts_external_scanner_states, one row of\n// ExternalTokenCount entries for each external lex state.\nvar externalScannerStates = []bool{")
		width := int(t.ExternalTokenCount)
		for i, b := range t.ExternalScanner.States {
			if i%width == 0 {
				w.printf("\n\t")
			} else {
				w.printf(" ")
			}
			w.printf("%t,", b)
		}
		w.printf("\n}\n\n")
		numbers(w, "externalScannerSymbolMap", "ts_external_scanner_symbol_map", "uint16", t.ExternalScanner.SymbolMap, 0)
	}
}

// mapSlice returns the Go literal of a MapSlice.
func mapSlice(s abi.MapSlice) string {
	if s == (abi.MapSlice{}) {
		return "{}"
	}
	return fmt.Sprintf("{Index: %d, Length: %d}", s.Index, s.Length)
}

// writeSmallParseTable writes ts_small_parse_table, with a comment before
// the groups of each small state.
func writeSmallParseTable(w *writer, t *abi.Language) {
	if len(t.SmallParseTable) == 0 {
		return
	}
	w.printf("// smallParseTable is ts_small_parse_table. The entries of a small state\n")
	w.printf("// are its number of groups, and for each group its value, its number of\n")
	w.printf("// symbols and the symbols.\n")
	w.printf("var smallParseTable = []uint16{")
	for k, start := range t.SmallParseTableMap {
		end := uint32(len(t.SmallParseTable))
		if k+1 < len(t.SmallParseTableMap) {
			end = t.SmallParseTableMap[k+1]
		}
		w.printf("\n\t// state %d", int(t.LargeStateCount)+k)
		for i, x := range t.SmallParseTable[start:end] {
			if i%16 == 0 {
				w.printf("\n\t")
			} else {
				w.printf(" ")
			}
			w.printf("%d,", x)
		}
	}
	w.printf("\n}\n\n")
}

// writeParseActions writes ts_parse_actions, one group of actions on each
// line.
func writeParseActions(w *writer, t *abi.Language) {
	w.printf("// parseActions is ts_parse_actions. Each group of actions is one entry\n")
	w.printf("// with the header of the group, and then its actions.\n")
	w.printf("var parseActions = []abi.ParseActionEntry{\n")
	for i := 0; i < len(t.ParseActions); {
		h := t.ParseActions[i].Entry
		w.printf("\t{Entry: header{Count: %d", h.Count)
		if h.Reusable {
			w.printf(", Reusable: true")
		}
		w.printf("}},")
		for k := 1; k <= int(h.Count); k++ {
			w.printf(" {Action: %s},", actionLiteral(t.ParseActions[i+k].Action))
		}
		w.printf("\n")
		i += 1 + int(h.Count)
	}
	w.printf("}\n\n")
}

// actionLiteral returns the Go literal of a parse action.
func actionLiteral(a abi.ParseAction) string {
	switch a.Type {
	case abi.ParseActionTypeShift:
		var parts []string
		if a.Shift.State != 0 {
			parts = append(parts, "State: "+strconv.Itoa(int(a.Shift.State)))
		}
		if a.Shift.Extra {
			parts = append(parts, "Extra: true")
		}
		if a.Shift.Repetition {
			parts = append(parts, "Repetition: true")
		}
		return "action{Type: typeShift, Shift: shift{" + strings.Join(parts, ", ") + "}}"
	case abi.ParseActionTypeReduce:
		r := a.Reduce
		s := fmt.Sprintf("action{Type: typeReduce, Reduce: reduce{ChildCount: %d, Symbol: %d", r.ChildCount, r.Symbol)
		if r.DynamicPrecedence != 0 {
			s += fmt.Sprintf(", DynamicPrecedence: %d", r.DynamicPrecedence)
		}
		if r.ProductionID != 0 {
			s += fmt.Sprintf(", ProductionID: %d", r.ProductionID)
		}
		return s + "}}"
	case abi.ParseActionTypeAccept:
		return "action{Type: typeAccept}"
	case abi.ParseActionTypeRecover:
		return "action{Type: typeRecover}"
	}
	panic("golang: a parse action of an unknown type")
}

// writeLexTable writes a lex table and its lex function.
func writeLexTable(w *writer, name, fn, doc string, t *abi.LexTable) {
	w.printf("// %s is %s, which %s runs.\n", fn, doc, name)
	w.printf("func %s(lexer *abi.Lexer, state uint16) bool {\n\treturn %s.Lex(lexer, state)\n}\n\n", fn, name)
	w.printf("// %s is the lex table of %s. The ranges of each state are sorted.\n", name, fn)
	w.printf("var %s = abi.LexTable{\n\tStates: []abi.LexState{\n", name)
	for _, s := range t.States {
		var parts []string
		if s.Start != 0 {
			parts = append(parts, fmt.Sprintf("Start: %d", s.Start))
		}
		if s.Count != 0 {
			parts = append(parts, fmt.Sprintf("Count: %d", s.Count))
		}
		if s.HasAccept {
			parts = append(parts, fmt.Sprintf("Accept: %d, HasAccept: true", s.Accept))
		}
		if s.HasEOF {
			parts = append(parts, fmt.Sprintf("EOFState: %d, HasEOF: true", s.EOFState))
		}
		if s.EOFSkip {
			parts = append(parts, "EOFSkip: true")
		}
		w.printf("\t\t{%s},\n", strings.Join(parts, ", "))
	}
	if len(t.Ranges) == 0 {
		w.printf("\t},\n}\n\n")
		return
	}
	w.printf("\t},\n\tRanges: []abi.LexRange{")
	for i, r := range t.Ranges {
		if i%4 == 0 {
			w.printf("\n\t\t")
		} else {
			w.printf(" ")
		}
		w.printf("{Lo: %s, Hi: %s, State: %d", character(r.Lo), character(r.Hi), r.State)
		if r.Skip {
			w.printf(", Skip: true")
		}
		w.printf("},")
	}
	w.printf("\n\t},\n}\n\n")
}

// character returns a character of a lex range as a Go expression: a rune
// literal for an ASCII character that prints, as add_character writes it in
// C, and a number for any other value.
//
// character is Generator::add_character, for Go.
func character(c int32) string {
	switch c {
	case '\'':
		return `'\''`
	case '\\':
		return `'\\'`
	case '\f':
		return `'\f'`
	case '\n':
		return `'\n'`
	case '\t':
		return `'\t'`
	case '\r':
		return `'\r'`
	}
	switch {
	case c == ' ' || c >= '!' && c <= '~':
		return "'" + string(c) + "'"
	case c < 0:
		return strconv.Itoa(int(c))
	}
	return fmt.Sprintf("0x%02x", c)
}
