// Package generate is the parser generator of transit, a port of the crate
// crates/generate of upstream tree-sitter (D7). It reads a grammar from
// grammar.json (D17), builds the parse table and the lexer tables, and gives
// them to a backend, which writes a parser (D8).
//
// ParserForGrammar runs the whole generator on a grammar.json, and
// ParserForGrammarWithOpts gives the ABI version and the other
// options. The package writes no parser itself. A backend, such as the C
// backend in generate/backend/c, takes a RenderInput and writes one.
package generate

import (
	"fmt"
	"strings"
)

// This file ports crates/generate/src/generate.rs. The port holds the
// functions that run the generator from a grammar.json in memory. The
// functions that read and write the files of a grammar folder, and that run
// grammar.js, are the job of the command (D17, D41). Upstream calls
// render_c_code, and the port calls a Backend in its place (D8).

// LanguageVersion is the ABI version that the generator writes when no
// other one is asked for.
//
// LanguageVersion is LANGUAGE_VERSION.
const LanguageVersion = 15

// The lowest and the highest ABI version that a backend writes.
//
// ABIVersionMin and ABIVersionMax are ABI_VERSION_MIN and ABI_VERSION_MAX of
// render.rs, which generate.rs exports.
const (
	ABIVersionMin = 14
	ABIVersionMax = LanguageVersion
)

// SemanticVersion is the version of a grammar, from tree-sitter.json.
//
// SemanticVersion is the (u8, u8, u8) of upstream.
type SemanticVersion struct {
	Major, Minor, Patch uint8
}

// RenderInput is what a backend reads to write a parser: the grammar and
// its tables, as the generator builds them (D8).
//
// RenderInput holds the arguments of render_c_code.
type RenderInput struct {
	// Name is the name of the grammar, in StrPool.
	Name           StrID
	Tables         *Tables
	SyntaxGrammar  *SyntaxGrammar
	LexicalGrammar *LexicalGrammar
	DefaultAliases AliasMap
	StrPool        *StrPool
	ABIVersion     int
	// SemanticVersion is the version of the grammar, or nil when it has
	// none.
	SemanticVersion    *SemanticVersion
	SupertypeSymbolMap SupertypeSymbolMap
}

// Backend writes a parser in one target language (D8).
type Backend interface {
	// Render writes the parser of a grammar.
	Render(in *RenderInput) (string, error)
}

// jsonOutput is what the generator makes before it builds the tables: the
// prepared grammar, the variable info and node-types.json.
//
// jsonOutput is JSONOutput.
type jsonOutput struct {
	nodeTypesJSON  string
	syntaxGrammar  *SyntaxGrammar
	lexicalGrammar *LexicalGrammar
	inlines        *InlinedProductionMap
	simpleAliases  AliasMap
	variableInfo   []VariableInfo
	strPool        *StrPool
}

// GeneratedParser is the output of the generator: the code that the backend
// writes, and node-types.json.
//
// GeneratedParser is GeneratedParser. Its field c_code is Code, because a
// backend other than the C backend writes it too.
type GeneratedParser struct {
	Code          string
	NodeTypesJSON string
}

// ParserForGrammar runs the generator on a grammar.json at
// LanguageVersion, and returns the name of the grammar and the code that the
// backend writes.
//
// ParserForGrammar is generate_parser_for_grammar, without the prefix
// Generate, which would repeat the name of the package.
func ParserForGrammar(grammarJSON []byte, semanticVersion *SemanticVersion, optimizations OptLevel, backend Backend, diagnostics *[]Diagnostic) (string, string, error) {
	inputGrammar, err := ParseGrammar(grammarJSON, diagnostics)
	if err != nil {
		return "", "", err
	}
	name := inputGrammar.Pool.Resolve(inputGrammar.Name)
	parser, err := ParserForGrammarWithOpts(inputGrammar, LanguageVersion, semanticVersion, optimizations, backend, diagnostics)
	if err != nil {
		return "", "", err
	}
	return name, parser.Code, nil
}

// generateNodeTypesFromGrammar prepares a grammar and makes its variable
// info and node-types.json.
//
// generateNodeTypesFromGrammar is generate_node_types_from_grammar.
func generateNodeTypesFromGrammar(inputGrammar *InputGrammar, diagnostics *[]Diagnostic) (*jsonOutput, error) {
	prepared, err := PrepareGrammar(inputGrammar, diagnostics)
	if err != nil {
		return nil, err
	}
	variableInfo, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)
	if err != nil {
		return nil, err
	}
	nodeTypesJSON, err := NodeTypesJSON(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, variableInfo, prepared.StrPool)
	if err != nil {
		return nil, err
	}
	return &jsonOutput{
		nodeTypesJSON:  nodeTypesJSON,
		syntaxGrammar:  &prepared.SyntaxGrammar,
		lexicalGrammar: &prepared.LexicalGrammar,
		inlines:        &prepared.Inlines,
		simpleAliases:  prepared.DefaultAliases,
		variableInfo:   variableInfo,
		strPool:        prepared.StrPool,
	}, nil
}

// ParserForGrammarWithOpts runs the generator on a parsed grammar at
// an ABI version, and gives the result to a backend. It changes inputGrammar.
//
// ParserForGrammarWithOpts is generate_parser_for_grammar_with_opts, without
// the prefix Generate, which would repeat the name of the package.
// The port leaves out its argument report_symbol_name, which only the log of
// build_tables reads.
func ParserForGrammarWithOpts(inputGrammar *InputGrammar, abiVersion int, semanticVersion *SemanticVersion, optimizations OptLevel, backend Backend, diagnostics *[]Diagnostic) (*GeneratedParser, error) {
	grammarName := inputGrammar.Name
	out, err := generateNodeTypesFromGrammar(inputGrammar, diagnostics)
	if err != nil {
		return nil, err
	}
	supertypeSymbolMap := GetSupertypeSymbolMap(out.syntaxGrammar, out.simpleAliases, out.variableInfo)
	tables, err := BuildTables(out.syntaxGrammar, out.lexicalGrammar, out.simpleAliases, out.variableInfo, out.inlines, out.strPool, optimizations, diagnostics)
	if err != nil {
		return nil, err
	}
	code, err := backend.Render(&RenderInput{
		Name:               grammarName,
		Tables:             tables,
		SyntaxGrammar:      out.syntaxGrammar,
		LexicalGrammar:     out.lexicalGrammar,
		DefaultAliases:     out.simpleAliases,
		StrPool:            out.strPool,
		ABIVersion:         abiVersion,
		SemanticVersion:    semanticVersion,
		SupertypeSymbolMap: supertypeSymbolMap,
	})
	if err != nil {
		return nil, err
	}
	return &GeneratedParser{Code: code, NodeTypesJSON: out.nodeTypesJSON}, nil
}

// OptLevel is a set of flags for the optimizations of the generator.
//
// OptLevel is OptLevel, a bitflags type upstream. Its Default is
// OptLevelMergeStates.
type OptLevel uint32

// The optimizations.
const (
	// OptLevelMergeStates merges the parse states that are compatible.
	OptLevelMergeStates OptLevel = 1 << 0
)

// Contains reports whether o holds every flag of other.
//
// Contains is OptLevel::contains.
func (o OptLevel) Contains(other OptLevel) bool {
	return o&other == other
}

// DiagnosticKind is the kind of a diagnostic.
type DiagnosticKind uint8

// The kinds of diagnostic, in the order of upstream.
const (
	DiagnosticUnnecessaryConflicts DiagnosticKind = iota
	DiagnosticUnaryChoice
	DiagnosticUnarySeq
	DiagnosticEmptyStringMatch
	DiagnosticUnsupportedRegexFlag
	DiagnosticSupertypeInlined
)

// Diagnostic is a warning that the generator reports and that does not stop
// it.
//
// Diagnostic is Diagnostic, an enum with data upstream. The fields that a kind
// uses are:
//
//   - DiagnosticUnnecessaryConflicts: Conflicts.
//   - DiagnosticUnaryChoice and DiagnosticUnarySeq: Name, which is empty for
//     a rule with no name.
//   - DiagnosticEmptyStringMatch and DiagnosticSupertypeInlined: Name.
//   - DiagnosticUnsupportedRegexFlag: Flag and Pattern.
type Diagnostic struct {
	Kind      DiagnosticKind
	Conflicts [][]string
	Name      string
	Flag      rune
	Pattern   string
}

// String returns the text of the diagnostic, as upstream writes it.
//
// String is the Display of Diagnostic.
func (d Diagnostic) String() string {
	var b strings.Builder
	switch d.Kind {
	case DiagnosticUnnecessaryConflicts:
		b.WriteString("unnecessary conflicts:\n")
		for i, conflict := range d.Conflicts {
			b.WriteString("  ")
			for j, symbol := range conflict {
				fmt.Fprintf(&b, "`%s`", symbol)
				if j < len(conflict)-1 {
					b.WriteString(", ")
				}
			}
			if i < len(d.Conflicts)-1 {
				b.WriteString("\n")
			}
		}
	case DiagnosticUnaryChoice:
		fmt.Fprintf(&b, "rule %s contains a `choice` rule with a single element. this is unnecessary.", nameOrAnonymous(d.Name))
	case DiagnosticUnarySeq:
		fmt.Fprintf(&b, "rule %s contains a `seq` rule with a single element. this is unnecessary.", nameOrAnonymous(d.Name))
	case DiagnosticEmptyStringMatch:
		fmt.Fprintf(&b, "named extra rule `%s` matches the empty string. inline this to avoid infinite loops while parsing.", d.Name)
	case DiagnosticUnsupportedRegexFlag:
		fmt.Fprintf(&b, "unsupported regex flag `%c` in pattern `%s`", d.Flag, d.Pattern)
	case DiagnosticSupertypeInlined:
		fmt.Fprintf(&b, "rule `%s` is both a supertype and inlined. the supertype is ignored.", d.Name)
	}
	return b.String()
}

// nameOrAnonymous returns the name, or <ANONYMOUS> when it is empty.
func nameOrAnonymous(name string) string {
	if name == "" {
		return "<ANONYMOUS>"
	}
	return name
}
