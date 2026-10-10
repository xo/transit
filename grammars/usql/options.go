package usql

import (
	"sync"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// Options are the options of the syntax of a SQL dialect that the external
// scanner reads (D108). Each field but BeginEndBlocks and Batches is a flag
// of the type Syntax of dbmeta, with the same name. With the zero value, none of
// these texts is a string or a comment, and a ; outside parentheses always
// ends a statement. A comment that starts with -- is a comment with every
// set of options.
//
// Each field is the flag of the same name in src/scanner.c, such as
// USQL_DOLLAR_QUOTES for DollarQuotes.
type Options struct {
	// DollarQuotes makes $tag$ ... $tag$ and $$ ... $$ a string.
	DollarQuotes bool
	// BlockComments makes /* ... */ a comment.
	BlockComments bool
	// SlashComments makes // start a comment to the end of the line.
	SlashComments bool
	// HashComments makes # start a comment to the end of the line.
	HashComments bool
	// Backticks makes `...` a quoted identifier.
	Backticks bool

	// BeginEndBlocks keeps a stored program of MySQL, SQL Server or Oracle
	// in one statement (D112). After CREATE ... PROCEDURE, FUNCTION,
	// TRIGGER or EVENT, the scanner counts BEGIN and END, and a ; inside
	// the body does not end the statement. It is not a flag of dbmeta.
	BeginEndBlocks bool
	// Batches keeps a batch of CQL in one statement. After BEGIN BATCH,
	// BEGIN UNLOGGED BATCH or BEGIN COUNTER BATCH at the start of a
	// statement, a ; does not end the statement until APPLY BATCH. It is not
	// a flag of dbmeta.
	Batches bool
}

// defaultOptions are the options of Language, and of src/scanner.c when the
// build does not set USQL_OPTIONS: dollar quotes and block comments, the
// options of PostgreSQL.
var defaultOptions = Options{DollarQuotes: true, BlockComments: true}

// languages holds the language of each set of options that LanguageFor
// built, and mu guards it.
var (
	mu        sync.Mutex
	languages = map[Options]*transit.Language{}
)

// LanguageFor returns the language of the grammar usql with an external
// scanner that reads opts. Every set of options uses the tables of
// Language. LanguageFor returns the same language each time for the same
// options, and it returns Language for dollar quotes and block comments,
// the default options.
func LanguageFor(opts Options) *transit.Language {
	if opts == defaultOptions {
		return Language()
	}
	mu.Lock()
	defer mu.Unlock()
	if l, ok := languages[opts]; ok {
		return l
	}
	tables := abi.TablesOf(Language())
	tables.ExternalScanner.Create = func() abi.Scanner { return &scanner{options: opts} }
	l := transit.NewLanguage(&tables)
	languages[opts] = l
	return l
}
