// Package golang is the Go backend of the transit generator (D8). It writes
// the Go package of a grammar from the tables that package generate builds:
// parser.go, which holds the tables of the grammar, and grammar_test.go,
// which holds the tests of the package. docs/GRAMMAR.md describes the
// package, and docs/API.md lists what it exports.
//
// The tables are Go literals, and the lexer is data: a table of character
// ranges for each lex state, which abi.LexTable runs (D74). The tables are the
// tables that the C backend writes to parser.c, with the same numbers.
//
// Backend implements generate.Backend. Package and PackageInDirectory write
// the whole package. The folder of the package is generate/backend/go, and
// the name of the package is golang, because go is a keyword (D26).
package golang
