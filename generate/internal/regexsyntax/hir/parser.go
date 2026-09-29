package hir

import (
	"github.com/xo/transit/generate/internal/regexsyntax/ast"
)

// This file ports src/parser.rs: the parser of the crate, which parses a
// pattern into a syntax tree and translates the tree into an Hir. It is in
// package hir, because it joins the packages ast and hir.
//
// Upstream returns the Error of error.rs, an enum of an ast::Error and an
// hir::Error. The port leaves out error.rs. A Go error holds the same two
// cases: the error of Parse is an *ast.Error or an *Error.
//
// The Default of ParserBuilder is NewParserBuilder, because the zero
// ParserBuilder has no builders.

// Parse parses pattern into an Hir with the default configuration. The error
// is an *ast.Error or an *Error.
//
// To set the configuration, use a ParserBuilder.
//
// Parse is parse.
func Parse(pattern string) (*Hir, error) {
	return NewParser().Parse(pattern)
}

// ParserBuilder holds the configuration of a parser. It sets the
// configuration of the parser of the syntax tree and of the translator.
//
// ParserBuilder is ParserBuilder.
type ParserBuilder struct {
	ast *ast.ParserBuilder
	hir *TranslatorBuilder
}

// NewParserBuilder returns a builder with the default configuration.
//
// NewParserBuilder is ParserBuilder::new, which is ParserBuilder::default.
func NewParserBuilder() *ParserBuilder {
	return &ParserBuilder{
		ast: ast.NewParserBuilder(),
		hir: NewTranslatorBuilder(),
	}
}

// Build returns a parser with the configuration of b.
//
// Build is ParserBuilder::build.
func (b *ParserBuilder) Build() *Parser {
	return &Parser{ast: b.ast.Build(), hir: b.hir.Build()}
}

// NestLimit sets the nest limit of the parser, and returns b. The nest limit
// is the depth that the syntax tree can have. ast.ParserBuilder.NestLimit
// says more.
//
// NestLimit is ParserBuilder::nest_limit.
func (b *ParserBuilder) NestLimit(limit uint32) *ParserBuilder {
	b.ast.NestLimit(limit)
	return b
}

// Octal sets whether the parser accepts the octal syntax, such as \141 for
// a, and returns b. The octal syntax is off by default.
//
// Octal is ParserBuilder::octal.
func (b *ParserBuilder) Octal(yes bool) *ParserBuilder {
	b.ast.Octal(yes)
	return b
}

// UTF8 sets whether the parser must only build an expression that matches
// valid UTF-8, and returns b. It is on by default. TranslatorBuilder.UTF8
// says more.
//
// UTF8 is ParserBuilder::utf8.
func (b *ParserBuilder) UTF8(yes bool) *ParserBuilder {
	b.hir.UTF8(yes)
	return b
}

// IgnoreWhitespace sets the verbose mode, and returns b. In verbose mode,
// the parser ignores white space in many places, and a # starts a comment.
// The flag x of a pattern turns it on for part of the pattern.
//
// IgnoreWhitespace is ParserBuilder::ignore_whitespace.
func (b *ParserBuilder) IgnoreWhitespace(yes bool) *ParserBuilder {
	b.ast.IgnoreWhitespace(yes)
	return b
}

// CaseInsensitive sets whether the flag i is on by default, and returns b.
//
// CaseInsensitive is ParserBuilder::case_insensitive.
func (b *ParserBuilder) CaseInsensitive(yes bool) *ParserBuilder {
	b.hir.CaseInsensitive(yes)
	return b
}

// MultiLine sets whether the flag m is on by default, and returns b.
//
// MultiLine is ParserBuilder::multi_line.
func (b *ParserBuilder) MultiLine(yes bool) *ParserBuilder {
	b.hir.MultiLine(yes)
	return b
}

// DotMatchesNewLine sets whether the flag s is on by default, and returns b.
//
// DotMatchesNewLine is ParserBuilder::dot_matches_new_line.
func (b *ParserBuilder) DotMatchesNewLine(yes bool) *ParserBuilder {
	b.hir.DotMatchesNewLine(yes)
	return b
}

// CRLF sets whether the flag R is on by default, and returns b.
//
// CRLF is ParserBuilder::crlf.
func (b *ParserBuilder) CRLF(yes bool) *ParserBuilder {
	b.hir.CRLF(yes)
	return b
}

// LineTerminator sets the line terminator for a . pattern, and returns b.
// TranslatorBuilder.LineTerminator says more.
//
// LineTerminator is ParserBuilder::line_terminator.
func (b *ParserBuilder) LineTerminator(term byte) *ParserBuilder {
	b.hir.LineTerminator(term)
	return b
}

// SwapGreed sets whether the flag U is on by default, and returns b.
//
// SwapGreed is ParserBuilder::swap_greed.
func (b *ParserBuilder) SwapGreed(yes bool) *ParserBuilder {
	b.hir.SwapGreed(yes)
	return b
}

// Unicode sets whether the flag u is on by default, and returns b. It is on
// by default.
//
// Unicode is ParserBuilder::unicode.
func (b *ParserBuilder) Unicode(yes bool) *ParserBuilder {
	b.hir.Unicode(yes)
	return b
}

// Parser parses a pattern into a syntax tree, and translates the tree into
// an Hir. A ParserBuilder sets the configuration of a parser.
//
// Parser is Parser.
type Parser struct {
	ast *ast.Parser
	hir *Translator
}

// NewParser returns a parser with the default configuration.
//
// NewParser is Parser::new.
func NewParser() *Parser {
	return NewParserBuilder().Build()
}

// Parse parses pattern into an Hir. The error is an *ast.Error or an *Error.
//
// Parse is Parser::parse.
func (p *Parser) Parse(pattern string) (*Hir, error) {
	a, err := p.ast.Parse(pattern)
	if err != nil {
		return nil, err
	}
	hir, err := p.hir.Translate(pattern, a)
	if err != nil {
		return nil, err
	}
	return hir, nil
}
