package usql

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/grammartest"
)

// optionSets are the options of the families of dialects of D101, which had
// a grammar each before D108, the options of MySQL, SQL Server, Oracle,
// Firebird, SAP HANA and Trino with BeginEndBlocks (D112, D130, D134), the
// options of Oracle with AnonymousBlocks (D128), the options of CQL with
// Batches, and the options of Neo4j, which has no dash comments (D139).
// Each set of a SQL dialect has DashComments, and sqlite and
// sqlserver_blocks have Brackets (D140). The test module compares the Go
// scanner with the C scanner on each of them.
var optionSets = map[string]Options{
	"postgres":         {DollarQuotes: true, BlockComments: true, DashComments: true},
	"mysql":            {BlockComments: true, HashComments: true, Backticks: true, DashComments: true},
	"sqlite":           {BlockComments: true, Backticks: true, DashComments: true, Brackets: true},
	"standard":         {BlockComments: true, DashComments: true},
	"cql":              {DollarQuotes: true, BlockComments: true, SlashComments: true, DashComments: true},
	"plain":            {},
	"mysql_blocks":     {BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true, DashComments: true},
	"sqlserver_blocks": {BlockComments: true, BeginEndBlocks: true, DashComments: true, Brackets: true},
	"oracle_blocks":    {BlockComments: true, BeginEndBlocks: true, DashComments: true},
	"oracle_anonymous": {BlockComments: true, BeginEndBlocks: true, AnonymousBlocks: true, DashComments: true},
	"firebird_blocks":  {BlockComments: true, BeginEndBlocks: true, DashComments: true},
	"hana_blocks":      {BlockComments: true, BeginEndBlocks: true, DashComments: true},
	"trino_blocks":     {BlockComments: true, BeginEndBlocks: true, DashComments: true},
	"cql_batches":      {DollarQuotes: true, BlockComments: true, SlashComments: true, Batches: true, DashComments: true},
	"neo4j":            {BlockComments: true, SlashComments: true, Backticks: true},
}

// TestOptions parses each case of testdata/options/<name>.txt with the
// options <name> of optionSets, and compares its tree with the expected
// tree, as the corpus test does. The cases need options that are not the
// default options, so the corpus of test/corpus does not hold them.
func TestOptions(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "options", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected the files of testdata/options, got none")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".txt")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			opts, ok := optionSets[name]
			if !ok {
				t.Fatalf("optionSets has no options %s", name)
			}
			grammartest.Corpus(t, LanguageFor(opts), file)
		})
	}
}

// TestOptionsHighlight highlights each file of testdata/highlight whose
// first line names a set of options, such as -- options: oracle_anonymous,
// with those options of optionSets. It checks the assertions in its
// comments, as the highlight test does (D137). The files need options that
// are not the default options, such as an anonymous block of
// AnonymousBlocks (D128). The test of a file fails when optionSets has no
// options of its name.
func TestOptionsHighlight(t *testing.T) {
	t.Parallel()
	grammartest.HighlightOptions(t, func(name string) (*transit.Language, bool) {
		opts, ok := optionSets[name]
		if !ok {
			return nil, false
		}
		return LanguageFor(opts), true
	}, Queries, "testdata/highlight")
}

// TestLanguageFor makes sure that LanguageFor gives Language for the default
// options, the same language each time for the same options, and the tables
// of Language for every set of options.
func TestLanguageFor(t *testing.T) {
	t.Parallel()
	if LanguageFor(optionSets["postgres"]) != Language() {
		t.Error("expected Language for the default options, got another language")
	}
	for name, opts := range optionSets {
		l := LanguageFor(opts)
		if l != LanguageFor(opts) {
			t.Errorf("%s: expected the same language for the same options, got two", name)
		}
		if l.Name() != "usql" || l.SymbolCount() != Language().SymbolCount() || l.StateCount() != Language().StateCount() {
			t.Errorf("%s: expected the tables of Language, got %s with %d symbols and %d states", name, l.Name(), l.SymbolCount(), l.StateCount())
		}
	}
}

// bomInputs are texts with U+FEFF, the byte order mark, for
// TestByteOrderMark. The test module gives them to the C scanner too.
var bomInputs = []string{
	"\ufeffselect 1;",
	"select 1;\ufeffselect 2;",
	" \ufeffselect 1;",
	"00000000;\ufeff",
	"select\ufeff1;",
	"select 1\ufeff;\ufeff",
	"select 'a\ufeffb",
	"select 1\ufeff\\g\ufeff\n",
	"\ufeff",
	" \ufeff \ufeff",
	"\\x\ufeff\\d\ufeffa\ufeff\n\ufeff\\p",
	"\\g\ufeff(\ufeffformat\ufeff=\ufeffcsv\ufeff)\ufeff",
	"\\echo\ufeff'\ufeff'\ufeff\"\ufeff\"\ufeff`\ufeff`",
	"\\!\ufeffls\ufeff",
	"select :\ufefffoo, :foo\ufeff, :'\ufeff', :'foo\ufeff', :{?\ufeff};",
	"select '\ufeff', \"\ufeff\", `\ufeff`, [\ufeff];",
	"select $$\ufeff$$, $\ufeff$, $a\ufeff$, x\ufeff$$a$$;",
	"select 1 /*\ufeff*/ -- \ufeff\n# \ufeff\n// \ufeff\n;",
	"begin\ufeff;",
	"begin\ufeffselect 1; end;",
	"create procedure p() begin\ufeffselect 1; end;\ufeffselect 2;",
	"create function f return number as\ufeffx number; begin return x; end;",
	"execute\ufeffblock as declare x int; begin x = 1; end;",
	"do\ufeffbegin select 1; end;",
	"with\ufefffunction f() returns int begin return 1; end select f();",
	"begin\ufeffbatch insert into t (a) values (1); apply\ufeffbatch;",
	"create procedure p() begin\ufeffcase\ufeffwhen 1 then select 1; end\ufeffcase; end\ufeff;",
}

// TestByteOrderMark parses each text of bomInputs with each set of options
// of optionSets (D141). Its tree must be the tree of the same text with
// three spaces in place of each U+FEFF, because U+FEFF is white space
// outside a string, a quoted name, a dollar quote and a comment, and is
// text inside them. No statement starts or ends with U+FEFF, and no meta
// command starts with it.
func TestByteOrderMark(t *testing.T) {
	t.Parallel()
	for name, opts := range optionSets {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parser := transit.NewParser()
			if err := parser.SetLanguage(LanguageFor(opts)); err != nil {
				t.Fatal(err)
			}
			for _, input := range bomInputs {
				tree := parseText(t, parser, input)
				spaced := parseText(t, parser, strings.ReplaceAll(input, "\ufeff", "   "))
				if got, want := tree.RootNode().String(), spaced.RootNode().String(); got != want {
					t.Errorf("%q: expected the tree %s of the text with spaces, got %s", input, want, got)
				}
				checkBOMEnds(t, input, tree.RootNode())
			}
		})
	}
}

// parseText parses the text src with the parser p.
func parseText(t *testing.T, p *transit.Parser, src string) *transit.Tree {
	t.Helper()
	tree, err := p.Parse(t.Context(), []byte(src), nil)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return tree
}

// checkBOMEnds makes sure that no statement in the node n of the text src
// starts or ends with U+FEFF, and that no meta command starts with it. A
// meta command ends after the white space at the end of its line, as it
// does with spaces.
func checkBOMEnds(t *testing.T, src string, n transit.Node) {
	t.Helper()
	text := src[n.StartByte():n.EndByte()]
	switch n.Kind() {
	case "statement":
		if strings.HasPrefix(text, "\ufeff") || strings.HasSuffix(text, "\ufeff") {
			t.Errorf("%q: expected a statement with no U+FEFF at its ends, got %q", src, text)
		}
	case "meta_command":
		if strings.HasPrefix(text, "\ufeff") {
			t.Errorf("%q: expected a meta command with no U+FEFF at its start, got %q", src, text)
		}
	}
	for i := range n.ChildCount() {
		child, _ := n.Child(i)
		checkBOMEnds(t, src, child)
	}
}
