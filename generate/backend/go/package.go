package golang

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/transit/generate"
)

// This file writes the files of a grammar package in its folder, as
// docs/GRAMMAR.md lays it out. It ports no upstream file. It takes the place
// of generate.ParserInDirectory for the Go backend, because a grammar
// package holds grammar.json in its own folder, and not in src. It also
// reads the entry of a grammar in grammars/grammars.json, the record that the
// golden harness writes (D40).

// Options says what the folder of a grammar package holds, which the files
// of the package depend on.
type Options struct {
	// Queries is true when the folder holds a file queries/*.scm.
	Queries bool
	// Corpus is true when the folder holds testdata/corpus.
	Corpus bool
	// Highlight is true when the folder holds testdata/highlight.
	Highlight bool
	// Others holds the other grammar packages of the module that the
	// highlight test can inject: each grammar of tree-sitter.json with an
	// injection-regex (D80).
	Others []Other
}

// Other is another grammar package of the module of a grammar package.
type Other struct {
	// Package is the name of the package, and ImportPath its import path.
	Package    string
	ImportPath string
}

// ReadOptions reads the options of the grammar package in dir.
func ReadOptions(dir string) (Options, error) {
	var opts Options
	queries, err := filepath.Glob(filepath.Join(dir, "queries", "*.scm"))
	if err != nil {
		return opts, fmt.Errorf("finding the queries in %s: %w", dir, err)
	}
	opts.Queries = len(queries) != 0
	for _, f := range []struct {
		name string
		v    *bool
	}{
		{"corpus", &opts.Corpus},
		{"highlight", &opts.Highlight},
	} {
		info, err := os.Stat(filepath.Join(dir, "testdata", f.name))
		switch {
		case err == nil:
			*f.v = info.IsDir()
		case !errors.Is(err, fs.ErrNotExist):
			return opts, fmt.Errorf("finding testdata/%s in %s: %w", f.name, dir, err)
		}
	}
	return opts, nil
}

// ReadGrammarOptions reads the options of the grammar package in dir: the
// options that ReadOptions reads, and the other grammars of the module from
// the nearest tree-sitter.json.
func ReadGrammarOptions(dir string) (Options, error) {
	opts, err := ReadOptions(dir)
	if err != nil {
		return opts, err
	}
	if opts.Others, err = readOthers(dir); err != nil {
		return opts, err
	}
	return opts, nil
}

// GrammarFolder returns the folder in its module of the package of a
// grammar whose path in tree-sitter.json is p: "." for the folder of the
// module, and else the last element of p with each "_" and "-" removed, as
// docs/GRAMMAR.md lays it out. The path php_only gives the folder phponly.
func GrammarFolder(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || p == "" {
		return "."
	}
	return strings.NewReplacer("_", "", "-", "").Replace(path.Base(p))
}

// readOthers returns the other grammar packages of the module of the
// package in dir that the highlight test can inject: each folder of an
// entry of the nearest tree-sitter.json, in dir or in a folder above it,
// but for the folder of dir, where an entry has an injection-regex (D80).
// The folder of tree-sitter.json is the folder of the module, and its go.mod
// gives the module path, and the folder of each other package gives the
// name of the package. A folder that holds no grammar.json yet is left out.
func readOthers(dir string) ([]Other, error) {
	pkgDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("finding the folder %s: %w", dir, err)
	}
	moduleDir := pkgDir
	var b []byte
	for {
		b, err = os.ReadFile(filepath.Join(moduleDir, "tree-sitter.json"))
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading tree-sitter.json: %w", err)
		}
		parent := filepath.Dir(moduleDir)
		if parent == moduleDir {
			return nil, nil
		}
		moduleDir = parent
	}
	var cfg struct {
		Grammars []struct {
			Path           string `json:"path"`
			InjectionRegex string `json:"injection-regex"`
		} `json:"grammars"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Join(moduleDir, "tree-sitter.json"), err)
	}
	rel, err := filepath.Rel(moduleDir, pkgDir)
	if err != nil {
		return nil, fmt.Errorf("finding the folder of the package: %w", err)
	}
	own := GrammarFolder(rel)
	var others []Other
	var seen []string
	modulePath := ""
	for _, g := range cfg.Grammars {
		folder := GrammarFolder(g.Path)
		if folder == own || g.InjectionRegex == "" || slices.Contains(seen, folder) {
			continue
		}
		seen = append(seen, folder)
		if !isFile(filepath.Join(moduleDir, folder, "grammar.json")) {
			continue
		}
		pkg, err := PackageName(filepath.Join(moduleDir, folder))
		if err != nil {
			return nil, err
		}
		if modulePath == "" {
			if modulePath, err = readModulePath(moduleDir); err != nil {
				return nil, err
			}
		}
		others = append(others, Other{Package: pkg, ImportPath: path.Join(modulePath, folder)})
	}
	return others, nil
}

// readModulePath returns the module path of the go.mod in dir.
func readModulePath(dir string) (string, error) {
	p := filepath.Join(dir, "go.mod")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("reading the go.mod of the module of the grammars: %w", err)
	}
	for line := range strings.Lines(string(b)) {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", fmt.Errorf("%s names no module", p)
}

// Files holds the files of a grammar package that the Go backend writes.
type Files struct {
	// Parser is parser.go.
	Parser string
	// NodeTypes is node-types.json, which parser.go embeds.
	NodeTypes string
	// Tests is grammar_test.go.
	Tests string
}

// Package generates the files of the grammar package in dir, from
// dir/grammar.json and the version in the nearest tree-sitter.json, in dir
// or in a folder above it.
func Package(dir string, abiVersion int, optimizations generate.OptLevel, diagnostics *[]generate.Diagnostic) (*Files, error) {
	grammarPath := filepath.Join(dir, "grammar.json")
	grammarJSON, err := os.ReadFile(grammarPath)
	if err != nil {
		return nil, &generate.Error{Kind: generate.ErrorLoadGrammarFile, Path: grammarPath, Err: err}
	}
	semanticVersion, err := generate.ReadGrammarVersion(dir)
	if err != nil {
		return nil, err
	}
	inputGrammar, err := generate.ParseGrammar(grammarJSON, diagnostics)
	if err != nil {
		return nil, err
	}
	pkg, err := PackageName(dir)
	if err != nil {
		return nil, err
	}
	opts, err := ReadGrammarOptions(dir)
	if err != nil {
		return nil, err
	}
	parser, err := generate.ParserForGrammarWithOpts(inputGrammar, abiVersion, semanticVersion, optimizations, Backend{Package: pkg, Queries: opts.Queries}, diagnostics)
	if err != nil {
		return nil, err
	}
	return &Files{Parser: parser.Code, NodeTypes: parser.NodeTypesJSON, Tests: Tests(pkg, opts)}, nil
}

// PackageInDirectory generates the files of the grammar package in dir, as
// Package does, and writes them to outDir, or to dir when outDir is empty.
// With generateParser false, it writes node-types.json only.
func PackageInDirectory(dir, outDir string, abiVersion int, generateParser bool, optimizations generate.OptLevel, diagnostics *[]generate.Diagnostic) error {
	if outDir == "" {
		outDir = dir
	}
	if !generateParser {
		// ParserInDirectory needs no version for node-types.json, so the
		// folder of grammar.json does not matter to it
		return generate.ParserInDirectory(dir, outDir, filepath.Join(dir, "grammar.json"), abiVersion, false, optimizations, Backend{}, diagnostics)
	}
	files, err := Package(dir, abiVersion, optimizations, diagnostics)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return &generate.Error{Kind: generate.ErrorIO, Path: outDir, Err: err}
	}
	for _, f := range []struct{ name, body string }{
		{"parser.go", files.Parser},
		{"node-types.json", files.NodeTypes},
		{"grammar_test.go", files.Tests},
	} {
		p := filepath.Join(outDir, f.name)
		if err := os.WriteFile(p, []byte(f.body), 0o644); err != nil {
			return &generate.Error{Kind: generate.ErrorIO, Path: p, Err: err}
		}
	}
	return nil
}

// Tests returns grammar_test.go of the grammar package pkg: the tests of
// docs/GRAMMAR.md, which call the package internal/grammartest. The corpus
// test and the highlight test are there only when the package has a
// corpus and a folder of highlight tests. The highlight test gives each
// grammar of opts.Others. The file holds no other data of the grammar, so
// it is the same in and outside a checkout of transit (D88).
func Tests(pkg string, opts Options) string {
	var b strings.Builder
	b.WriteString("// Code generated by transit. DO NOT EDIT.\n\n")
	b.WriteString("package " + pkg + "\n\n")
	imports := []string{"github.com/xo/transit/internal/grammartest"}
	if opts.Highlight {
		for _, o := range opts.Others {
			imports = append(imports, o.ImportPath)
		}
	}
	slices.Sort(imports)
	b.WriteString("import (\n\t\"testing\"\n\n")
	for _, p := range imports {
		b.WriteString("\t" + strconv.Quote(p) + "\n")
	}
	b.WriteString(")\n\n")
	if opts.Corpus {
		b.WriteString(`// TestCorpus parses each case of testdata/corpus, and compares its tree with
// the expected tree, as tree-sitter test does. It expects the cases of
// testdata/failing.txt to fail, because they fail upstream.
func TestCorpus(t *testing.T) {
	t.Parallel()
	grammartest.Corpus(t, Language(), "testdata/corpus")
}

`)
	}
	b.WriteString(`// TestQueries makes sure that each file of queries compiles.
func TestQueries(t *testing.T) {
	t.Parallel()
	grammartest.Queries(t, Language(), Queries)
}

`)
	if opts.Highlight {
		b.WriteString(`// TestHighlight highlights each file of testdata/highlight as the
// highlighter of upstream does, and checks the assertions in its comments.
// The part of the test that looks for each capture name in the package
// styles waits for phase 5, because that package does not exist yet.
func TestHighlight(t *testing.T) {
	t.Parallel()
`)
		if len(opts.Others) == 0 {
			b.WriteString("\tgrammartest.Highlight(t, Language(), Queries, \"testdata/highlight\")\n")
		} else {
			b.WriteString("\tgrammartest.Highlight(t, Language(), Queries, \"testdata/highlight\",\n")
			for _, o := range opts.Others {
				b.WriteString("\t\tgrammartest.Grammar{Language: " + o.Package + ".Language(), Queries: " + o.Package + ".Queries},\n")
			}
			b.WriteString("\t)\n")
		}
		b.WriteString("}\n\n")
	}
	b.WriteString(`// TestNodeTypes makes sure that each node type is a symbol of the language.
func TestNodeTypes(t *testing.T) {
	t.Parallel()
	grammartest.NodeTypes(t, Language(), NodeTypes())
}

// TestKeywords makes sure that each keyword is a symbol of the language.
func TestKeywords(t *testing.T) {
	t.Parallel()
	grammartest.Keywords(t, Language(), Keywords())
}

// TestGenerator generates the package again, and compares the files with
// the files of the package, and with the hashes and the corpus cases that
// fail upstream of grammars/grammars.json.
func TestGenerator(t *testing.T) {
	t.Parallel()
	grammartest.Generator(t, Language())
}
`)
	return b.String()
}

// Record is the entry of a grammar in grammars/grammars.json, with the
// fields that the files and the tests of a grammar package read.
type Record struct {
	Name       string                `json:"name"`
	Repository string                `json:"repository"`
	Golden     map[string]GoldenFile `json:"golden"`
	// Corpus is nil when the grammar has no corpus result.
	Corpus *CorpusResult `json:"corpus"`
}

// GoldenFile is a golden file of a Record: the SHA-256 of the parser.c and
// of the node-types.json that the upstream tool writes, or the error of the
// tool.
type GoldenFile struct {
	ParserC   string `json:"parser_c"`
	NodeTypes string `json:"node_types"`
	Error     string `json:"error"`
}

// CorpusResult is the result of the corpus of a grammar with the upstream
// tool.
type CorpusResult struct {
	// Failing holds the path of each corpus case that fails upstream, such
	// as "expressions/Binary operators". testdata/failing.txt of the
	// package holds the same names (D79, D88).
	Failing []string `json:"failing"`
}

// FindRecord returns the entry of the grammar name in the
// grammars/grammars.json of dir or of a folder above it. It returns nil and
// no error when no such folder holds grammars/grammars.json, which is the
// case for a package outside a checkout of transit. When several entries
// have the name, the entry is the one whose repository gives the name of the
// folder of the module, the nearest folder that holds a go.mod, as
// docs/GRAMMAR.md says. When several entries give that name too, as the two
// repositories tree-sitter-sql do, the entry is the one whose repository
// the tree-sitter.json of the module names (D106).
func FindRecord(dir, name string) (*Record, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("finding the folder %s: %w", dir, err)
	}
	var moduleDir, recordFile string
	for {
		if moduleDir == "" && isFile(filepath.Join(dir, "go.mod")) {
			moduleDir = dir
		}
		if p := filepath.Join(dir, "grammars", "grammars.json"); isFile(p) {
			recordFile = p
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
	b, err := os.ReadFile(recordFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", recordFile, err)
	}
	var file struct {
		Grammars []Record `json:"grammars"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", recordFile, err)
	}
	var found []Record
	for _, r := range file.Grammars {
		if r.Name == name {
			found = append(found, r)
		}
	}
	if len(found) > 1 {
		found = slices.DeleteFunc(found, func(r Record) bool {
			return ModuleFolderName(r.Repository) != filepath.Base(moduleDir)
		})
	}
	if len(found) > 1 {
		repository, err := moduleRepository(moduleDir)
		if err != nil {
			return nil, err
		}
		found = slices.DeleteFunc(found, func(r Record) bool {
			return !sameRepository(r.Repository, repository)
		})
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("%s has %d entries for the grammar %s in the module folder %s", recordFile, len(found), name, filepath.Base(moduleDir))
	}
	return &found[0], nil
}

// moduleRepository returns the repository that the tree-sitter.json in the
// folder of a module names in metadata.links.repository, or "" when the
// folder has no tree-sitter.json or the file names no repository.
func moduleRepository(moduleDir string) (string, error) {
	p := filepath.Join(moduleDir, "tree-sitter.json")
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", p, err)
	}
	var cfg struct {
		Metadata struct {
			Links struct {
				Repository string `json:"repository"`
			} `json:"links"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", fmt.Errorf("decoding %s: %w", p, err)
	}
	return cfg.Metadata.Links.Repository, nil
}

// sameRepository reports whether two URLs name the same repository. It
// ignores the case, a prefix git+ and a suffix .git or /, so that
// git+https://github.com/derekstride/tree-sitter-sql.git names
// https://github.com/DerekStride/tree-sitter-sql. An empty URL names no
// repository.
func sameRepository(a, b string) bool {
	clean := func(u string) string {
		u = strings.ToLower(strings.TrimPrefix(u, "git+"))
		return strings.TrimSuffix(strings.TrimSuffix(u, "/"), ".git")
	}
	return a != "" && b != "" && clean(a) == clean(b)
}

// ModuleFolderName returns the name of the folder of the module of a
// repository: its name without the prefix tree-sitter- and with each "-"
// removed, as docs/GRAMMAR.md says.
func ModuleFolderName(repository string) string {
	name := strings.TrimPrefix(path.Base(repository), "tree-sitter-")
	return strings.ReplaceAll(name, "-", "")
}

// isFile reports whether a file or a folder exists at a path.
func isFile(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
