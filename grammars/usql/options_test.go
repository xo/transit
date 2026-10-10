package usql

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/transit/internal/grammartest"
)

// optionSets are the options of the families of dialects of D101, which
// had a grammar each before D108, the options of MySQL, SQL Server and
// Oracle with BeginEndBlocks (D112), and the options of CQL with Batches.
// The test module compares the Go scanner with the C scanner on each of
// them.
var optionSets = map[string]Options{
	"postgres":         {DollarQuotes: true, BlockComments: true},
	"mysql":            {BlockComments: true, HashComments: true, Backticks: true},
	"sqlite":           {BlockComments: true, Backticks: true},
	"standard":         {BlockComments: true},
	"cql":              {DollarQuotes: true, BlockComments: true, SlashComments: true},
	"plain":            {},
	"mysql_blocks":     {BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true},
	"sqlserver_blocks": {BlockComments: true, BeginEndBlocks: true},
	"oracle_blocks":    {BlockComments: true, BeginEndBlocks: true},
	"cql_batches":      {DollarQuotes: true, BlockComments: true, SlashComments: true, Batches: true},
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
