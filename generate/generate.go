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
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// This file ports crates/generate/src/generate.rs. Upstream calls
// render_c_code, and the port calls a Backend in its place (D8). The port
// reads only grammar.json and runs no JavaScript (D17), so it leaves out
// load_js_grammar_file and the option js_runtime, and a grammar.js path is an
// error. It also leaves out report_symbol_name, which only the log of
// build_tables reads. An error of the operating system has the text of Go,
// and not the text of Rust.

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

// The headers that a generated parser includes, copied from upstream into
// generate/templates. ParserInDirectory writes them to src/tree_sitter.
//
// AllocHeader, ArrayHeader and ParserHeader are ALLOC_HEADER, ARRAY_HEADER
// and PARSER_HEADER.
var (
	//go:embed templates/alloc.h
	AllocHeader string
	//go:embed templates/array.h
	ArrayHeader string
	//go:embed templates/parser.h
	ParserHeader string
)

// ErrorKind is the kind of an Error.
type ErrorKind uint8

// The kinds of error, in the order of upstream. The other variants of
// GenerateError pass the error of a pass through, so ParserInDirectory
// returns that error as it is.
const (
	ErrorGrammarPath ErrorKind = iota
	ErrorIO
	ErrorLoadGrammarFile
	ErrorGrammarFileNotFound
	ErrorParseVersion
)

// Error is an error of ParserInDirectory. Its text is the text of upstream,
// but the text of an error of the operating system is the text of Go.
//
// Error is GenerateError, with LoadGrammarError, ParseVersionError and
// IoError in it, without the prefix Generate, which would repeat the name of
// the package.
type Error struct {
	Kind ErrorKind
	// Path is the file or the folder of the error.
	Path string
	// Text is the text of a ErrorLoadGrammarFile or ErrorParseVersion
	// error that has no Err.
	Text string
	// Err is the error of the operating system, when there is one.
	Err error
}

// Error returns the text of the error.
func (e *Error) Error() string {
	io := ""
	if e.Err != nil {
		io = e.Err.Error()
		if e.Path != "" && !strings.Contains(io, e.Path) {
			io += " (" + e.Path + ")"
		}
	}
	switch e.Kind {
	case ErrorGrammarPath:
		return "Error with specified path -- " + io
	case ErrorIO:
		return io
	case ErrorLoadGrammarFile:
		if e.Err != nil {
			return "Failed to load grammar.json -- " + io
		}
		return e.Text
	case ErrorGrammarFileNotFound:
		return "Grammar file `" + e.Path + "` not found"
	case ErrorParseVersion:
		if e.Err != nil {
			return io
		}
		return e.Text
	}
	return ""
}

// Unwrap returns the error of the operating system.
func (e *Error) Unwrap() error {
	return e.Err
}

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

// ParserInDirectory generates the parser of a grammar folder: it reads
// src/grammar.json of repoPath, or the grammar file of grammarPath, and writes
// parser.c, node-types.json and the headers to outPath, or to the folder
// src of the grammar. With generateParser false, it writes node-types.json
// only. repoPath is the folder of the grammar when grammarPath is empty.
//
// ParserInDirectory is generate_parser_in_directory, without the prefix
// Generate, which would repeat the name of the package. It reads no
// grammar.js (D17).
func ParserInDirectory(repoPath, outPath, grammarPath string, abiVersion int, generateParser bool, optimizations OptLevel, backend Backend, diagnostics *[]Diagnostic) error {
	// Fill a new empty grammar folder, or find the root of the grammar from
	// the explicit path of its file: grammar.js is in the root, and
	// grammar.json is in <root>/src.
	var grammarFile string
	if grammarPath != "" {
		if _, err := os.Stat(grammarPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return &Error{Kind: ErrorGrammarPath, Path: grammarPath, Err: err}
			}
			// a missing path with an extension, such as
			// tree-sitter-foo/grammar.json, is a missing input file, and
			// not a folder to make
			if filepath.Ext(grammarPath) != "" {
				return &Error{Kind: ErrorGrammarFileNotFound, Path: grammarPath}
			}
			if err := os.MkdirAll(grammarPath, 0o755); err != nil {
				return &Error{Kind: ErrorIO, Path: grammarPath, Err: err}
			}
			repoPath = grammarPath
			grammarFile = filepath.Join(repoPath, "grammar.js")
		} else {
			switch filepath.Ext(grammarPath) {
			case ".js":
				repoPath = filepath.Dir(grammarPath)
			case ".json":
				repoPath = filepath.Dir(filepath.Dir(grammarPath))
			}
			grammarFile = grammarPath
		}
	} else {
		// upstream reads grammar.js here, and the port reads the grammar.json
		// that the upstream tool makes from it (D17)
		grammarFile = filepath.Join(repoPath, "src", "grammar.json")
	}

	grammarJSON, err := loadGrammarFile(grammarFile)
	if err != nil {
		return err
	}

	srcPath := outPath
	if srcPath == "" {
		srcPath = filepath.Join(repoPath, "src")
	}
	headerPath := filepath.Join(srcPath, "tree_sitter")

	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		return &Error{Kind: ErrorIO, Path: srcPath, Err: err}
	}

	inputGrammar, err := ParseGrammar(grammarJSON, diagnostics)
	if err != nil {
		return err
	}

	if !generateParser {
		out, err := generateNodeTypesFromGrammar(inputGrammar, diagnostics)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(srcPath, "node-types.json"), out.nodeTypesJSON)
	}

	semanticVersion, err := readGrammarVersion(repoPath)
	if err != nil {
		return err
	}

	parser, err := ParserForGrammarWithOpts(inputGrammar, abiVersion, semanticVersion, optimizations, backend, diagnostics)
	if err != nil {
		return err
	}

	if err := writeFile(filepath.Join(srcPath, "parser.c"), parser.Code); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(srcPath, "node-types.json"), parser.NodeTypesJSON); err != nil {
		return err
	}
	if err := os.MkdirAll(headerPath, 0o755); err != nil {
		return &Error{Kind: ErrorIO, Path: headerPath, Err: err}
	}
	for _, h := range []struct{ name, body string }{
		{"alloc.h", AllocHeader},
		{"array.h", ArrayHeader},
		{"parser.h", ParserHeader},
	} {
		if err := writeFile(filepath.Join(headerPath, h.name), h.body); err != nil {
			return err
		}
	}
	return nil
}

// semver matches a version in the strict form of the Rust crate semver:
// three numbers with no leading zero, and a pre-release and build metadata
// after them.
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// readGrammarVersion reads the version of a grammar from the nearest
// tree-sitter.json, in the folder of the grammar or in a folder above it.
// It returns nil when no folder has one. Each number is cut to 8 bits, as
// the cast of upstream does.
//
// readGrammarVersion is read_grammar_version. The text of an error of JSON
// is the text of encoding/json, and the text of a version error is that of
// the port, not of the Rust crate semver.
func readGrammarVersion(repoPath string) (*SemanticVersion, error) {
	const filename = "tree-sitter.json"
	dir, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, &Error{Kind: ErrorParseVersion, Path: repoPath, Err: err}
	}
	for {
		path := filepath.Join(dir, filename)
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			var cfg struct {
				Metadata *struct {
					Version *string `json:"version"`
				} `json:"metadata"`
			}
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, &Error{Kind: ErrorParseVersion, Text: "Failed to parse `" + path + "` -- " + err.Error()}
			}
			if cfg.Metadata == nil || cfg.Metadata.Version == nil {
				field := "metadata"
				if cfg.Metadata != nil {
					field = "version"
				}
				return nil, &Error{Kind: ErrorParseVersion, Text: "Failed to parse `" + path + "` -- missing field `" + field + "`"}
			}
			m := semver.FindStringSubmatch(*cfg.Metadata.Version)
			if m == nil {
				return nil, &Error{Kind: ErrorParseVersion, Text: "Failed to parse `" + path + "` version as semver -- " + strconv.Quote(*cfg.Metadata.Version) + " is not a version of the form MAJOR.MINOR.PATCH"}
			}
			var parts [3]uint8
			for i := range parts {
				n, err := strconv.ParseUint(m[i+1], 10, 64)
				if err != nil {
					return nil, &Error{Kind: ErrorParseVersion, Text: "Failed to parse `" + path + "` version as semver -- " + err.Error()}
				}
				parts[i] = uint8(n)
			}
			return &SemanticVersion{Major: parts[0], Minor: parts[1], Patch: parts[2]}, nil
		case !errors.Is(err, os.ErrNotExist):
			return nil, &Error{Kind: ErrorParseVersion, Path: path, Err: err}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

// loadGrammarFile reads a grammar.json. A grammar.js is an error, because
// the port runs no JavaScript (D17).
//
// loadGrammarFile is load_grammar_file.
func loadGrammarFile(grammarPath string) ([]byte, error) {
	if fi, err := os.Stat(grammarPath); err == nil && fi.IsDir() {
		return nil, &Error{Kind: ErrorLoadGrammarFile, Text: "Path to a grammar file with `.js` or `.json` extension is required"}
	}
	switch filepath.Ext(grammarPath) {
	case ".js":
		return nil, &Error{Kind: ErrorLoadGrammarFile, Text: "Failed to load grammar.js -- transit reads only grammar.json and runs no JavaScript (D17). Make src/grammar.json with the upstream tool first: " + grammarPath}
	case ".json":
		b, err := os.ReadFile(grammarPath)
		if err != nil {
			return nil, &Error{Kind: ErrorLoadGrammarFile, Path: grammarPath, Err: err}
		}
		return b, nil
	}
	return nil, &Error{Kind: ErrorLoadGrammarFile, Text: "Unknown grammar file extension: " + strconv.Quote(grammarPath)}
}

// writeFile writes a file of the output.
//
// writeFile is write_file.
func writeFile(path, body string) error {
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return &Error{Kind: ErrorIO, Path: path, Err: err}
	}
	return nil
}
