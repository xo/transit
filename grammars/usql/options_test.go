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
// Each set of a SQL dialect has DashComments. The test module compares the
// Go scanner with the C scanner on each of them.
var optionSets = map[string]Options{
	"postgres":         {DollarQuotes: true, BlockComments: true, DashComments: true},
	"mysql":            {BlockComments: true, HashComments: true, Backticks: true, DashComments: true},
	"sqlite":           {BlockComments: true, Backticks: true, DashComments: true},
	"standard":         {BlockComments: true, DashComments: true},
	"cql":              {DollarQuotes: true, BlockComments: true, SlashComments: true, DashComments: true},
	"plain":            {},
	"mysql_blocks":     {BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true, DashComments: true},
	"sqlserver_blocks": {BlockComments: true, BeginEndBlocks: true, DashComments: true},
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
