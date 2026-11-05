package usql

import (
	"sync"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// Options are the options of the syntax of a SQL dialect that the external
// scanner reads (D108). DollarQuotes, BlockComments, SlashComments,
// HashComments, Backticks, DashComments, Brackets and BackslashEscapes are
// flags of the type Syntax of dbmeta, with the same names. dbmeta holds the
// value of each flag for each dialect. With the zero value, none of these
// texts is a string or a comment, a backslash in a string or in a name in
// double quotes is text, and a ; outside parentheses always ends a
// statement.
//
// Each field is the flag of the same name in src/scanner.c, such as
// USQL_DOLLAR_QUOTES for DollarQuotes, and field i is the flag 1<<i.
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
	// the body does not end the statement. EXECUTE BLOCK of Firebird and
	// DO BEGIN of SAP HANA at the start of a statement open a block too
	// (D130). A ; in the declarations of EXECUTE BLOCK before its BEGIN
	// does not end the statement. The statement ends at the ; after the
	// END of the block. WITH FUNCTION of Trino at the start of a statement
	// opens a block for the body of each routine that it defines (D134).
	// A ; in such a body does not end the statement, and the statement
	// ends at the ; after the query that follows the routines. It is not a
	// flag of dbmeta.
	BeginEndBlocks bool
	// AnonymousBlocks keeps an anonymous block of PL/SQL in one statement
	// (D128). A statement that starts with BEGIN or DECLARE is a block. The
	// scanner counts BEGIN and END, and the statement ends at the ; after
	// the END of the outer BEGIN. In the block, a BEGIN that a ;, WORK or
	// TRANSACTION follows starts a transaction and not a block (D131). A
	// statement that starts with another word does not change.
	//
	// A line with only / is the text of a statement, with AnonymousBlocks
	// on or off, as in psql (D138). Another PL/SQL unit, such as a function
	// with declarations before its BEGIN, holds a ; that ends the
	// statement. The script escapes each such ; as \;. It is not a flag of
	// dbmeta.
	AnonymousBlocks bool
	// Batches keeps a batch of CQL in one statement. After BEGIN BATCH,
	// BEGIN UNLOGGED BATCH or BEGIN COUNTER BATCH at the start of a
	// statement, a ; does not end the statement until APPLY BATCH. It is not
	// a flag of dbmeta.
	Batches bool

	// DashComments makes -- start a comment to the end of the line (D139).
	// With it off, -- is the text of the statement, as the relationship
	// (a)--(b) of Cypher is.
	DashComments bool
	// Brackets makes [...] a quoted identifier, as in SQL Server and SQLite
	// (D140). ]] in the brackets is a ], and every other character is the
	// text of the name: a quote, a ;, a comment, a backslash, a colon and a
	// new line. With it off, [ is text, as in a[1] and ARRAY[1,2] of
	// PostgreSQL.
	Brackets bool
	// BackslashEscapes makes a backslash escape the next character in a
	// string in single quotes and in a name in double quotes, as in MySQL
	// (D142). \' is a quote, \" is a double quote and \\ is a backslash.
	// With it off, a backslash in such a string or name is text, as in
	// 'C:\data\', and only a doubled quote escapes a quote. A string of the
	// form E'...' of PostgreSQL keeps the backslash escapes with it off. The
	// E must start a word, so name'x' is not such a string. A backslash in
	// backticks or in brackets is text, with it on or off. In an argument
	// of a meta command, a backslash escapes in a string and it is text in a
	// name in double quotes, with it on or off, as in psql. It is the flag
	// Syntax.BackslashEscapes of dbmeta (dbmeta D266). It is the last field,
	// so that its flag in src/scanner.c is the next free flag.
	BackslashEscapes bool
}

// defaultOptions are the options of Language, and of src/scanner.c when the
// build does not set USQL_OPTIONS: dollar quotes, block comments, dash
// comments and backslash escapes (D142).
var defaultOptions = Options{DollarQuotes: true, BlockComments: true, DashComments: true, BackslashEscapes: true}

// languages holds the language of each set of options that LanguageFor
// built, and mu guards it.
var (
	mu        sync.Mutex
	languages = map[Options]*transit.Language{}
)

// LanguageFor returns the language of the grammar usql with an external
// scanner that reads opts. Every set of options uses the tables of
// Language. LanguageFor returns the same language each time for the same
// options, and it returns Language for dollar quotes, block comments, dash
// comments and backslash escapes, the default options.
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
