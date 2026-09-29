// Package c is the C backend of the transit generator (D8). It writes the
// parser.c of a grammar from the tables that package generate builds, and
// the file is the same, byte for byte, as the parser.c that upstream
// tree-sitter writes.
//
// Backend implements generate.Backend. Give it to
// generate.ParserForGrammarWithOpts to write the parser of a grammar.
package c
