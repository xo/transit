# The target API

This document holds the target Go API of transit: what the runtime package
exports, and what a generated grammar package exports. It comes from the
working C example of phase 1 (D10), in `_samples/example/`, which uses the
upstream runtime at the base commit and a real grammar.

The API follows the Rust binding, in idiomatic Go (D24, D25). A lookup that can
find no node returns the node and a `bool` (D56). Ken accepts this document
before phase 1 ends, and until then it is a proposal. The choices that the
example raised are decided: `#lua-match?` (D55), the form of a lookup (D56)
and `StatesAt` (D57).

## The working C example

`_samples/example/build.sh` builds the upstream tool at the base commit,
generates the SQL grammar of `DerekStride/tree-sitter-sql` at the tag
`v0.3.11` with it, compiles `example.c` with the upstream runtime, and runs it.
Downloads and build output go to `$XDG_CACHE_HOME/transit`, outside the
repository.

The example does eight steps, each for a need of rline or usql:

1. It parses a statement of three rows and prints the tree.
2. It highlights the statement with `queries/highlights.scm` of the grammar,
   and it evaluates the query predicates as the Rust binding does.
3. It types a statement one key at a time. After each key, it edits the old
   tree, parses again with it, reads the changed ranges, and highlights again.
4. It finds the node at a cursor offset, its parent and its field.
5. It parses three statements that are not finished, and prints the `ERROR`
   and `MISSING` nodes.
6. It lists the symbols that the parser can accept at the cursor.
7. It highlights only the second row, with a byte range.
8. It times each step.

### What it measured

These numbers are from one run on the development machine on 2026-09-29. They
are the first C baseline for the benchmarks of phase 3 (D37).

| Step | Time |
| --- | --- |
| first parse of 68 bytes | 36 µs |
| compile of `highlights.scm`, 445 rows and 24 patterns | 6,727 µs |
| highlight of the statement, 19 captures | 50 µs |
| one key: edit, parse again, changed ranges and highlight, mean of 49 keys | 12.2 µs |
| one key, the worst of 49 keys | 33.9 µs |

The SQL grammar has 729 symbols, 53 fields and 17,329 parse states.

### What it found

1. A query costs much more to compile than to run. A consumer compiles each
   query once, and shares it (D52).
2. The highlight query of the SQL grammar uses `#match?` with the patterns
   `^[-+]?%d+$` and `^[-+]?%d*\.%d*$`. These are patterns of Lua, which
   Neovim uses, and not regular expressions. In a regular expression, `%d` is
   the two characters `%` and `d`, so both patterns never match, and `42` and
   `1.5` fall through to the capture `string`. The Rust binding has the same
   result. transit adds `#lua-match?`, which Neovim queries use for such
   patterns (D55).
3. The generic SQL grammar parses `SELECT id, FROM users` with no error. It
   reads `FROM` as a column and `users` as its alias. A consumer cannot find
   every mistake from `ERROR` nodes.
4. `ts_node_next_parse_state` gives 0 for a token that the parser lexed before
   a reduce, such as `FROM` in `SELECT id FROM u`. The token keeps the state in
   which the parser lexed it, and the action of the token in that state is a
   reduce. The state before the token is the state of its parent, when it is
   the first child, or the next state of the sibling before it. From there the
   lookahead lists 11 symbols after `FROM`, such as `relation`, `identifier` and
   `subquery`. Each of the two methods fails in a case where the other works.
5. Inside an error, both methods give state 0, the state of the recovery, and
   the lookahead lists 375 symbols. This is why transit adds an API for the
   states before the recovery (D28). "The API that upstream does not have"
   below proposes it.

## The package transit

This is the proposed surface of the root package. The bodies are ports of the
upstream functions that the table below names.

```go
package transit

// Symbol, FieldID and StateID are the numbers of the tables of a language.
type (
	Symbol  uint16
	FieldID uint16
	StateID uint16
)

// SymbolType is the kind of a symbol.
type SymbolType int

const (
	SymbolRegular SymbolType = iota
	SymbolAnonymous
	SymbolSupertype
	SymbolAuxiliary
)

// Point is a position as a row and a column. Both count from zero, and the
// column counts bytes.
type Point struct {
	Row    int
	Column int
}

// Range is a range of the text, in bytes and in points.
type Range struct {
	StartByte  int
	EndByte    int
	StartPoint Point
	EndPoint   Point
}

// InputEdit describes one edit of the text.
type InputEdit struct {
	StartByte   int
	OldEndByte  int
	NewEndByte  int
	StartPoint  Point
	OldEndPoint Point
	NewEndPoint Point
}

// Language is the tables of one grammar. A grammar package returns it. It is
// safe to share between goroutines.
type Language struct{ /* unexported */ }

// NewLanguage builds a language from its tables. Only code under
// github.com/xo/transit/, such as a grammar package, can build an
// *abi.Language, because the package abi is internal (D63).
func NewLanguage(tables *abi.Language) *Language

func (l *Language) Name() string
func (l *Language) ABIVersion() int
func (l *Language) Metadata() (LanguageMetadata, bool)
func (l *Language) SymbolCount() int
func (l *Language) SymbolName(s Symbol) string
func (l *Language) SymbolForName(name string, named bool) (Symbol, bool)
func (l *Language) SymbolType(s Symbol) SymbolType
func (l *Language) Supertypes() []Symbol
func (l *Language) Subtypes(supertype Symbol) []Symbol
func (l *Language) FieldCount() int
func (l *Language) FieldName(f FieldID) string
func (l *Language) FieldForName(name string) (FieldID, bool)
func (l *Language) StateCount() int
func (l *Language) NextState(s StateID, sym Symbol) StateID
func (l *Language) LookaheadIterator(s StateID) (*LookaheadIterator, bool)

// LanguageMetadata is the version of a grammar.
type LanguageMetadata struct {
	Major, Minor, Patch int
}

// Parser builds trees. It belongs to one goroutine at a time.
type Parser struct{ /* unexported */ }

func NewParser() *Parser
func (p *Parser) SetLanguage(l *Language) error
func (p *Parser) Language() *Language
func (p *Parser) SetIncludedRanges(ranges []Range) error
func (p *Parser) IncludedRanges() []Range
func (p *Parser) SetLogger(fn func(LogType, string))
func (p *Parser) PrintDotGraphs(w io.Writer)
func (p *Parser) Parse(ctx context.Context, src []byte, old *Tree) (*Tree, error)
func (p *Parser) ParseInput(ctx context.Context, in Input, enc Encoding, old *Tree) (*Tree, error)
func (p *Parser) Reset()

// Input gives the text in chunks, as TSInput does.
type Input interface {
	ReadAt(offset int, at Point) []byte
}

// Encoding is the encoding of the text.
type Encoding int

const (
	EncodingUTF8 Encoding = iota
	EncodingUTF16LE
	EncodingUTF16BE
)

// LogType says whether a log message comes from the parser or the lexer.
type LogType int

const (
	LogParse LogType = iota
	LogLex
)

// Tree is a syntax tree. It is safe for reads from many goroutines. Edit
// changes it, so call Copy first to keep a version.
type Tree struct{ /* unexported */ }

func (t *Tree) RootNode() Node
func (t *Tree) RootNodeWithOffset(offset int, at Point) Node
func (t *Tree) Language() *Language
func (t *Tree) Edit(e InputEdit)
func (t *Tree) Copy() *Tree
func (t *Tree) ChangedRanges(newTree *Tree) []Range
func (t *Tree) IncludedRanges() []Range
func (t *Tree) Walk() *TreeCursor
func (t *Tree) PrintDotGraph(w io.Writer)

// Node is a node of a tree. It is a small value, and two nodes compare with ==.
type Node struct{ /* unexported */ }

func (n Node) Kind() string
func (n Node) KindID() Symbol
func (n Node) GrammarKind() string
func (n Node) GrammarID() Symbol
func (n Node) Language() *Language
func (n Node) IsNamed() bool
func (n Node) IsExtra() bool
func (n Node) IsMissing() bool
func (n Node) IsError() bool
func (n Node) HasError() bool
func (n Node) HasChanges() bool
func (n Node) StartByte() int
func (n Node) EndByte() int
func (n Node) StartPoint() Point
func (n Node) EndPoint() Point
func (n Node) Range() Range
func (n Node) Text(src []byte) string
func (n Node) String() string
func (n Node) ParseState() StateID
func (n Node) NextParseState() StateID
func (n Node) Parent() (Node, bool)
func (n Node) ChildWithDescendant(descendant Node) (Node, bool)
func (n Node) ChildCount() int
func (n Node) Child(i int) (Node, bool)
func (n Node) NamedChildCount() int
func (n Node) NamedChild(i int) (Node, bool)
func (n Node) Children() iter.Seq[Node]
func (n Node) NamedChildren() iter.Seq[Node]
func (n Node) ChildByFieldName(name string) (Node, bool)
func (n Node) ChildByFieldID(f FieldID) (Node, bool)
func (n Node) ChildrenByFieldName(name string) iter.Seq[Node]
func (n Node) FieldNameForChild(i int) string
func (n Node) FieldNameForNamedChild(i int) string
func (n Node) NextSibling() (Node, bool)
func (n Node) PrevSibling() (Node, bool)
func (n Node) NextNamedSibling() (Node, bool)
func (n Node) PrevNamedSibling() (Node, bool)
func (n Node) FirstChildForByte(offset int) (Node, bool)
func (n Node) FirstNamedChildForByte(offset int) (Node, bool)
func (n Node) DescendantCount() int
func (n Node) DescendantForByteRange(start, end int) (Node, bool)
func (n Node) NamedDescendantForByteRange(start, end int) (Node, bool)
func (n Node) DescendantForPointRange(start, end Point) (Node, bool)
func (n Node) NamedDescendantForPointRange(start, end Point) (Node, bool)
func (n *Node) Edit(e InputEdit)
func (n Node) Walk() *TreeCursor

// TreeCursor walks a tree. It belongs to one goroutine at a time.
type TreeCursor struct{ /* unexported */ }

func (c *TreeCursor) Node() Node
func (c *TreeCursor) FieldName() string
func (c *TreeCursor) FieldID() FieldID
func (c *TreeCursor) Depth() int
func (c *TreeCursor) DescendantIndex() int
func (c *TreeCursor) GotoFirstChild() bool
func (c *TreeCursor) GotoLastChild() bool
func (c *TreeCursor) GotoParent() bool
func (c *TreeCursor) GotoNextSibling() bool
func (c *TreeCursor) GotoPreviousSibling() bool
func (c *TreeCursor) GotoDescendant(index int)
func (c *TreeCursor) GotoFirstChildForByte(offset int) (int, bool)
func (c *TreeCursor) GotoFirstChildForPoint(at Point) (int, bool)
func (c *TreeCursor) Reset(n Node)
func (c *TreeCursor) ResetTo(other *TreeCursor)
func (c *TreeCursor) Copy() *TreeCursor

// Query is a compiled query. It is safe to share between goroutines, after
// any call to DisablePattern or DisableCapture.
type Query struct{ /* unexported */ }

func NewQuery(l *Language, source string) (*Query, error) // the error is a *QueryError
func (q *Query) PatternCount() int
func (q *Query) CaptureNames() []string
func (q *Query) CaptureIndexForName(name string) (int, bool)
func (q *Query) CaptureQuantifiers(pattern int) []Quantifier
func (q *Query) StartByteForPattern(pattern int) int
func (q *Query) EndByteForPattern(pattern int) int
func (q *Query) PropertySettings(pattern int) []QueryProperty
func (q *Query) PropertyPredicates(pattern int) []QueryPropertyPredicate
func (q *Query) GeneralPredicates(pattern int) []QueryPredicate
func (q *Query) IsPatternRooted(pattern int) bool
func (q *Query) IsPatternNonLocal(pattern int) bool
func (q *Query) IsPatternGuaranteedAtStep(offset int) bool
func (q *Query) DisablePattern(pattern int)
func (q *Query) DisableCapture(name string)

// QueryError says where a query fails to compile.
type QueryError struct {
	Offset  int
	Row     int
	Column  int
	Kind    QueryErrorKind
	Message string
}

func (e *QueryError) Error() string

// QueryCursor runs a query on a tree. It belongs to one goroutine at a time.
// Matches and Captures evaluate the text predicates of the query (D27), and
// #lua-match? and #not-lua-match? (D55).
type QueryCursor struct{ /* unexported */ }

func NewQueryCursor() *QueryCursor
func (c *QueryCursor) SetMatchLimit(limit int)
func (c *QueryCursor) MatchLimit() int
func (c *QueryCursor) DidExceedMatchLimit() bool
func (c *QueryCursor) SetMaxStartDepth(depth int)
func (c *QueryCursor) SetByteRange(start, end int)
func (c *QueryCursor) SetPointRange(start, end Point)
func (c *QueryCursor) SetContainingByteRange(start, end int)
func (c *QueryCursor) SetContainingPointRange(start, end Point)
func (c *QueryCursor) Matches(ctx context.Context, q *Query, n Node, src []byte) iter.Seq[QueryMatch]
func (c *QueryCursor) Captures(ctx context.Context, q *Query, n Node, src []byte) iter.Seq2[QueryMatch, int]

// QueryMatch is one match of a pattern.
type QueryMatch struct {
	PatternIndex int
	Captures     []QueryCapture
}

// QueryCapture is one captured node. Index is its place in CaptureNames.
type QueryCapture struct {
	Node  Node
	Index int
}

// LookaheadIterator lists the symbols that the parser can accept in a state.
// It belongs to one goroutine at a time.
type LookaheadIterator struct{ /* unexported */ }

func (it *LookaheadIterator) Language() *Language
func (it *LookaheadIterator) Symbols() iter.Seq[Symbol]
func (it *LookaheadIterator) Names() iter.Seq[string]
func (it *LookaheadIterator) Reset(l *Language, s StateID) bool
func (it *LookaheadIterator) ResetState(s StateID) bool

// Error is an error of the runtime.
type Error string

func (e Error) Error() string

const (
	ErrIncompatibleLanguage Error = "incompatible language version"
	ErrInvalidRanges        Error = "invalid included ranges"
	ErrNoLanguage           Error = "parser has no language"
	ErrLanguageMismatch     Error = "old tree has another language"
	ErrInvalidInput         Error = "invalid input"
)
```

`ErrNoLanguage`, `ErrLanguageMismatch` and `ErrInvalidInput` came with the
port of the parser. The C parser returns NULL for a parse with no language,
with an old tree of another language, or with no input, and Go names each
case. A parse whose context ends keeps its state, as a parse whose progress
callback stops does in C, and the next parse goes on from it.

`Parse` returns the error of its context when the context ends, wrapped with
`%w`. `Captures` gives each match with the index of the capture in the match,
in the order of the text, which is the order that highlighting needs.

## From C to Go

The example calls these C functions. Each row names the Go form.

| C | Go |
| --- | --- |
| `ts_parser_new`, `ts_parser_delete` | `NewParser`. The garbage collector frees it |
| `ts_parser_set_language` | `(*Parser).SetLanguage`, which returns `ErrIncompatibleLanguage` |
| `ts_parser_parse_string` | `(*Parser).Parse(ctx, src, old)` |
| `ts_tree_root_node` | `(*Tree).RootNode` |
| `ts_tree_edit` | `(*Tree).Edit` |
| `ts_tree_get_changed_ranges` | `(*Tree).ChangedRanges`, which returns a slice |
| `ts_tree_delete` | nothing |
| `ts_node_string` | `Node.String` |
| `ts_node_type`, `ts_node_grammar_symbol` | `Node.Kind`, `Node.GrammarID` |
| `ts_node_start_byte`, `ts_node_end_byte`, `ts_node_start_point` | `Node.StartByte`, `Node.EndByte`, `Node.StartPoint`, as `int` |
| `ts_node_parent`, `ts_node_prev_sibling` | `Node.Parent`, `Node.PrevSibling`, each with a `bool` |
| `ts_node_child_count`, `ts_node_child` | `Node.ChildCount`, `Node.Child`, or `Node.Children` as an `iter.Seq` |
| `ts_node_descendant_for_byte_range` | `Node.DescendantForByteRange` |
| `ts_node_named_descendant_for_byte_range` | `Node.NamedDescendantForByteRange` |
| `ts_node_is_error`, `ts_node_is_missing`, `ts_node_has_error` | `Node.IsError`, `Node.IsMissing`, `Node.HasError` |
| `ts_node_parse_state`, `ts_node_next_parse_state` | `Node.ParseState`, `Node.NextParseState` |
| `ts_node_eq` | `==` |
| `ts_tree_cursor_new`, `ts_tree_cursor_delete` | `Node.Walk` |
| `ts_tree_cursor_goto_first_child`, `ts_tree_cursor_goto_next_sibling` | `(*TreeCursor).GotoFirstChild`, `GotoNextSibling` |
| `ts_tree_cursor_current_node`, `ts_tree_cursor_current_field_name` | `(*TreeCursor).Node`, `FieldName` |
| `ts_query_new` | `NewQuery`, which returns a `*QueryError` |
| `ts_query_pattern_count`, `ts_query_capture_count` | `(*Query).PatternCount`, `len(q.CaptureNames())` |
| `ts_query_capture_name_for_id` | `q.CaptureNames()[i]` |
| `ts_query_predicates_for_pattern`, `ts_query_string_value_for_id` | evaluated inside `Matches` and `Captures` (D27). `GeneralPredicates` gives the rest |
| `ts_query_cursor_new`, `ts_query_cursor_delete` | `NewQueryCursor` |
| `ts_query_cursor_exec`, `ts_query_cursor_next_capture`, `ts_query_cursor_remove_match` | `(*QueryCursor).Captures`, which removes a match whose predicates fail |
| `ts_query_cursor_set_byte_range` | `(*QueryCursor).SetByteRange` |
| `ts_language_next_state`, `ts_language_symbol_type` | `(*Language).NextState`, `SymbolType` |
| `ts_lookahead_iterator_new`, `_next`, `_current_symbol_name` | `(*Language).LookaheadIterator`, `Names` |
| `ts_language_name`, `ts_language_abi_version` | `(*Language).Name`, `ABIVersion` |

These upstream functions have no Go form:

1. The functions that free memory or count references: each `_delete`,
   `ts_language_copy`, `ts_query_copy` and `ts_set_allocator` (D24).
2. The WebAssembly functions: `ts_language_is_wasm`, `ts_parser_set_wasm_store`,
   `ts_parser_take_wasm_store` and each `ts_wasm_store_` function (D1).
3. `ts_node_is_null`, because a lookup returns a `bool` with the node.
4. `ts_parser_parse_string_encoding` and `ts_parser_parse_with_options`, whose
   jobs `ParseInput` and the context do.
5. `ts_point_edit` and `ts_range_edit`, which become methods of `Point` and
   `Range` if a consumer needs them.
6. `ts_language_is_parseable`, which a language that Go generated always is.

## The API that upstream does not have

D28 allows an API that only adds, in files that port no upstream file, with a
decision for each. The first one is for completion (D57):

```go
// StatesAt parses src up to offset, and returns the parse state of each
// stack version at offset, before the parser recovers from an error. A
// consumer lists the symbols that can come next with the lookahead iterator
// of each state.
func (p *Parser) StatesAt(ctx context.Context, src []byte, offset int, old *Tree) ([]StateID, error)
```

For `SELECT * FROM `, the parser has shifted `FROM` when it reaches the end of
the text, so the state before its recovery is expected to list the symbols of
a table reference, as the state after `FROM` does in `SELECT id FROM u`. This
is not measured yet, because the C runtime has no such API. Phase 3 measures
it in the test module.

The second one is the predicates `#lua-match?` and `#not-lua-match?`, which
match the text of a capture with a Lua pattern, as Neovim does (D55).

## A generated grammar package

The Go backend writes one package for each grammar (D26, D31). For the SQL
grammar, the package `sql` exports:

```go
package sql

// Language returns the tables of the SQL grammar.
func Language() *transit.Language

// A constant for each symbol and each field.
const (
	SymKeywordSelect transit.Symbol = 2
	SymKeywordFrom   transit.Symbol = 3
	// ...
	SymRelation transit.Symbol = 431
)

const (
	FieldAlias transit.FieldID = 1
	FieldName  transit.FieldID = 25
	// ...
)

// Queries holds queries/*.scm of the grammar.
var Queries embed.FS

// Keywords returns the keywords of the grammar: the string tokens that the
// word token captures, and the reserved words.
func Keywords() []string

// NodeTypes returns the node types of node-types.json: the fields and the
// children of each node, and the supertypes.
func NodeTypes() []transit.NodeType
```

The numbers in this sketch are examples. The generator writes the real ones.
`transit.NodeType` is a type of the runtime that `NodeTypes` needs, and it
joins the list above when the Go backend exists.

## What rline and usql call

rline highlights with `Parse`, `Edit`, `NewQuery` once, and `Captures` with a
byte range for the rows that it shows. usql completes with `Parse`,
`DescendantForByteRange`, `Parent`, `FieldNameForChild`, `StatesAt` and
`LookaheadIterator`, and it decides what kind of name goes at the cursor from
the symbols that come back and from its own queries (D6). `RLINE.md` and
`USQL.md` hold the details.
