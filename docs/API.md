# The target API

This document holds the target Go API of transit: what the runtime package
exports, what a generated grammar package exports, and what the module
`styles` exports. It comes from the
working C example of phase 1 (D10), in `_samples/example/`, which uses the
upstream runtime at the base commit and a real grammar.

The API follows the Rust binding, in idiomatic Go (D24, D25). A lookup that can
find no node returns the node and a `bool` (D56). Ken accepted this document
on 2026-10-01, which ended phase 1 (D103). A change to it is a decision first. The choices that the
example raised are decided: `#lua-match?` (D55, which waits for the tier 1
grammars, D69), the form of a lookup (D56)
and `StatesAt` (D57). The API of the package `inject` is decided too (D72),
and so is the form of `QueryMatch::remove` (D89).

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

This is the surface of the root package. The bodies are ports of the
upstream functions that the table below names. Each enum type, such as
`SymbolType`, `Encoding` and `LogType`, also has a `String` method, which the
list leaves out.

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

func (e InputEdit) EditPoint(p *Point, byteOffset *int)
func (e InputEdit) EditRange(r *Range)

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

// NodeType is one entry of node-types.json, which the NodeTypes function of
// a grammar package returns. The JSON keys are the keys of the file.
type NodeType struct {
	Kind     string               // "type"
	Named    bool                 // "named"
	Root     bool                 // "root"
	Extra    bool                 // "extra"
	Fields   map[string]FieldInfo // "fields"
	Children *FieldInfo           // "children", or nil
	Subtypes []NodeKind           // "subtypes", for a supertype
}

// FieldInfo says which nodes a field, or the children of a node, can hold.
type FieldInfo struct {
	Multiple bool
	Required bool
	Types    []NodeKind
}

// NodeKind names a node type: its kind, and whether it is named.
type NodeKind struct {
	Kind  string
	Named bool
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
func (p *Parser) ParseCustomEncoding(ctx context.Context, in Input, decode func(b []byte) (rune, int), old *Tree) (*Tree, error)
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
// changes it, so call Copy first to keep a version. Close is optional. After
// Close, the program must not use the tree or its nodes (D91).
type Tree struct{ /* unexported */ }

func (t *Tree) Close()
func (t *Tree) RootNode() Node
func (t *Tree) RootNodeWithOffset(offset int, at Point) Node
func (t *Tree) Language() *Language
func (t *Tree) Edit(e InputEdit)
func (t *Tree) Copy() *Tree
func (t *Tree) ChangedRanges(newTree *Tree) []Range
func (t *Tree) IncludedRanges() []Range
func (t *Tree) Walk() *TreeCursor
func (t *Tree) PrintDotGraph(w io.Writer)

// Node is a node of a tree. It is a small value, and two nodes compare with ==,
// which also compares their positions. Equal is the comparison of C (D68).
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
func (n Node) Equal(other Node) bool
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
func (q *Query) Copy() *Query

// QueryError says where a query fails to compile.
type QueryError struct {
	Offset  int
	Row     int
	Column  int
	Kind    QueryErrorKind
	Message string
}

func (e *QueryError) Error() string

// QueryErrorKind is the kind of a QueryError. QueryErrorPredicate comes from
// the Rust binding, and the others from TSQueryError.
type QueryErrorKind int

const (
	QueryErrorNone QueryErrorKind = iota
	QueryErrorSyntax
	QueryErrorNodeType
	QueryErrorField
	QueryErrorCapture
	QueryErrorStructure
	QueryErrorLanguage
	QueryErrorPredicate
)

// Quantifier says how many times a capture can occur in a pattern.
type Quantifier int

const (
	QuantifierZero Quantifier = iota
	QuantifierZeroOrOne
	QuantifierZeroOrMore
	QuantifierOne
	QuantifierOneOrMore
)

// QueryProperty is a key and a value of #set!, #is? or #is-not?. CaptureID is
// -1 when the predicate names no capture.
type QueryProperty struct {
	Key       string
	Value     string
	HasValue  bool
	CaptureID int
}

// QueryPropertyPredicate is a property of #is?, which is positive, or of
// #is-not?.
type QueryPropertyPredicate struct {
	Property QueryProperty
	Positive bool
}

// QueryPredicate is a predicate that the query does not evaluate, such as a
// predicate of Neovim, with its arguments.
type QueryPredicate struct {
	Operator string
	Args     []QueryPredicateArg
}

// QueryPredicateArg is a capture or a string.
type QueryPredicateArg struct {
	IsCapture bool
	Capture   int
	Value     string
}

// QueryCursor runs a query on a tree. It belongs to one goroutine at a time.
// Matches and Captures evaluate the text predicates of the query (D27).
// The predicates of Neovim, such as #lua-match?, wait for the tier 1 grammars
// (D69), and until then each one is a general predicate, as in the Rust
// binding. docs/NEOVIM.md lists them.
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

// QueryMatch is one match of a pattern. Captures is valid until the sequence
// moves on, because the cursor reuses it, as the Rust binding does.
type QueryMatch struct {
	PatternIndex int
	Captures     []QueryCapture
	// unexported: the cursor and the id of the match
}

// Remove removes the match from its cursor, so that the sequence gives no
// more captures of it. It is QueryMatch::remove of the Rust binding (D89).
func (m QueryMatch) Remove()

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
`%w`. `ParseCustomEncoding` reads a text in an encoding that the runtime does
not know. The parser calls `decode` for each character, with the text from
the character to the end of the chunk. `decode` returns the code point and
the number of bytes of the character, and it returns -1 as the code point
when the text is not valid. A nil `decode` gives `ErrInvalidInput`.
`(*Query).Copy` gives a query that `DisablePattern` and `DisableCapture`
change apart from the first one. `EditPoint` and `EditRange` change a point
and its byte offset, or a range, so that they stay at the same place in the
text after an edit, with no tree and no node. D119 adds these four.

`Captures` gives each match with the index of the capture in the match,
in the order of the text, which is the order that highlighting needs. The
highlighter of upstream removes the match of an injection with
`QueryMatch::remove` while it reads the captures. The package `inject` calls
`Remove` in the same place (D89). `Remove` is valid only while the sequence
that gave the match runs, because the next run of the cursor gives the same
ids to other matches.

When the context of `Matches` or `Captures` ends, the cursor halts, as a
query cursor halts in C when its progress callback returns true. The cursor
drops the matches in progress. `Captures` still gives the captures of the
matches that finished before, and then its sequence ends.

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
| `ts_tree_delete` | `(*Tree).Close`, which is optional. The garbage collector frees a tree that is not closed (D91) |
| `ts_node_string` | `Node.String` |
| `ts_node_type`, `ts_node_grammar_symbol` | `Node.Kind`, `Node.GrammarID` |
| `ts_node_start_byte`, `ts_node_end_byte`, `ts_node_start_point` | `Node.StartByte`, `Node.EndByte`, `Node.StartPoint`, as `int` |
| `ts_node_parent`, `ts_node_prev_sibling` | `Node.Parent`, `Node.PrevSibling`, each with a `bool` |
| `ts_node_child_count`, `ts_node_child` | `Node.ChildCount`, `Node.Child`, or `Node.Children` as an `iter.Seq` |
| `ts_node_descendant_for_byte_range` | `Node.DescendantForByteRange` |
| `ts_node_named_descendant_for_byte_range` | `Node.NamedDescendantForByteRange` |
| `ts_node_is_error`, `ts_node_is_missing`, `ts_node_has_error` | `Node.IsError`, `Node.IsMissing`, `Node.HasError` |
| `ts_node_parse_state`, `ts_node_next_parse_state` | `Node.ParseState`, `Node.NextParseState` |
| `ts_node_eq` | `Node.Equal`. `==` also compares the position (D68) |
| `ts_tree_cursor_new`, `ts_tree_cursor_delete` | `Node.Walk` |
| `ts_tree_cursor_goto_first_child`, `ts_tree_cursor_goto_next_sibling` | `(*TreeCursor).GotoFirstChild`, `GotoNextSibling` |
| `ts_tree_cursor_current_node`, `ts_tree_cursor_current_field_name` | `(*TreeCursor).Node`, `FieldName` |
| `ts_query_new` | `NewQuery`, which returns a `*QueryError` |
| `ts_query_pattern_count`, `ts_query_capture_count` | `(*Query).PatternCount`, `len(q.CaptureNames())` |
| `ts_query_capture_name_for_id` | `q.CaptureNames()[i]` |
| `ts_query_predicates_for_pattern`, `ts_query_string_value_for_id` | evaluated inside `Matches` and `Captures` (D27). `GeneralPredicates` gives the rest |
| `ts_query_cursor_new`, `ts_query_cursor_delete` | `NewQueryCursor` |
| `ts_query_cursor_exec`, `ts_query_cursor_next_capture` | `(*QueryCursor).Captures` |
| `ts_query_cursor_remove_match` | `QueryMatch.Remove`. `Captures` also calls it for a match whose predicates fail |
| `ts_query_cursor_set_byte_range` | `(*QueryCursor).SetByteRange` |
| `ts_language_next_state`, `ts_language_symbol_type` | `(*Language).NextState`, `SymbolType` |
| `ts_lookahead_iterator_new`, `_next`, `_current_symbol_name` | `(*Language).LookaheadIterator`, `Names` |
| `ts_language_name`, `ts_language_abi_version` | `(*Language).Name`, `ABIVersion` |

The example does not call these C functions. D119 gives them a Go form, so
that the tests of upstream that use them are ported:

| C | Go |
| --- | --- |
| `ts_parser_parse` with `TSInputEncodingCustom` and a `decode` function | `(*Parser).ParseCustomEncoding(ctx, in, decode, old)`, which is `parse_custom_encoding` of the Rust binding |
| `ts_query_copy` | `(*Query).Copy`, which is `deep_clone` of the Rust binding |
| `ts_point_edit` | `InputEdit.EditPoint(&p, &byteOffset)`, which is `edit_point` of the Rust binding |
| `ts_range_edit` | `InputEdit.EditRange(&r)`, which is `edit_range` of the Rust binding |

These upstream functions have no Go form:

1. The functions that free memory or count references: each `_delete` other
   than `ts_tree_delete`, `ts_language_copy` and `ts_set_allocator` (D24,
   D91).
2. The WebAssembly functions: `ts_language_is_wasm`, `ts_parser_set_wasm_store`,
   `ts_parser_take_wasm_store` and each `ts_wasm_store_` function (D1).
3. `ts_node_is_null`, because a lookup returns a `bool` with the node.
4. `ts_parser_parse_string_encoding` and `ts_parser_parse_with_options`, whose
   jobs `ParseInput`, `ParseCustomEncoding` and the context do.
5. `ts_language_is_parseable`, which a language that Go generated always is.

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

`states_at.go` holds it. Each stack version stops before the first token
that ends after the offset, or before the end of the text. The lexer reads
all of `src`, so the tokens before the offset are the tokens of a parse of
all of it. When the offset is inside a word, the states are the states before
the word. A word that ends at the offset is handled, so to complete a word, a
consumer gives the offset of its start. D70 records these rules.

The test module measures it. For `SELECT * FROM `, the SQL grammar of the C
example gives state 9144, which accepts 14 symbols, such as `identifier`,
`object_reference` and `subquery`. The state after `FROM` in
`SELECT id FROM u` is the same state. The tree has an error there, and state
0 accepts 408 symbols. On the corpora of the 17 fixture grammars, a state that
`StatesAt` gives accepts the next token at each of 13,664 offsets.

The second one is the predicates `#lua-match?` and `#not-lua-match?`, which
match the text of a capture with a Lua pattern, as Neovim does (D55). They
wait until the tier 1 grammars need them (D69), with the rest of the Neovim
dialect that [`NEOVIM.md`](NEOVIM.md) lists.

The third one is `NodeType`, `FieldInfo` and `NodeKind`, the form of
`node-types.json` in Go, which the `NodeTypes` function of a grammar package
returns. `node_type.go` holds them. Their names follow `NodeInfoJSON`,
`FieldInfoJSON` and `NodeTypeJSON` of `crates/generate/src/node_types.rs`,
and `Kind` is the key `type`, as `Node.Kind` is the type of a node. D76
records them.

The fourth one is `inject.WithReplacer`, which replaces the text of nodes
before a layer is parsed (D101). The fifth one is `inject.Layer.Text` and
`inject.Layer.StatesAt`, which give the text that a layer parses and the
parse states of the layer at an offset (D111). "The package inject" below
gives them.

## The package inject

The package `inject` finds the injections of a text and parses each layer
(D27). An injection is a range of the text that another grammar parses, such
as a `<script>` element of HTML, or a SQL statement in the input of usql
(D13). A layer is one tree of one language over the ranges of its
injections. It ports the part of `crates/highlight` that does this, without
the highlighting. Upstream has no public API for it, so D72 sets this one:

```go
package inject

// Config is the injection part of HighlightConfiguration: a language, its
// name and its injection query.
type Config struct{ /* unexported */ }

func NewConfig(language *transit.Language, name, injectionQuery string) (*Config, error)
func (c *Config) Name() string
func (c *Config) Language() *transit.Language

// Layer is one tree of one language over the ranges of its injections.
type Layer struct {
	Name   string // the language name that the query gave, such as "js"
	Config *Config
	Tree   *transit.Tree
	Ranges []transit.Range
	Depth  int    // 0 for the root layer
	Text   []byte // the text that the layer parses, after the replacements
}

// StatesAt returns the parse states of the layer at offset, as
// transit.Parser.StatesAt gives them. It is an API that upstream does not
// have.
func (l Layer) StatesAt(ctx context.Context, p *transit.Parser, offset int) ([]transit.StateID, error)

// Layers parses src with c, finds every injection, and parses the layer of
// each. lookup gives the configuration of an injected language name.
func (c *Config) Layers(ctx context.Context, p *transit.Parser, src []byte,
	lookup func(name string) (*Config, bool), opts ...Option) ([]Layer, error)

// Option changes what Layers does. It is an API that upstream does not have.
type Option func(*options)

// Replacer gives the text that takes the place of the node n of a parent
// layer in the text of the injected layer of the language name, and true,
// or false for a node whose text stays. The text has the length of the node.
type Replacer func(name string, n transit.Node, src []byte) ([]byte, bool)

// WithReplacer makes Layers ask r about the nodes of the parent layer inside
// the ranges of an injected layer, before it parses that layer.
func WithReplacer(r Replacer) Option
```

`inject` takes UTF-8 only, and a Rust oracle in the test module compares its
layers with the layers of `crates/highlight` (D71). The layers come sorted by
the start of their first range, then by depth, so the root layer comes first.
D72 holds the rest of the rules: the skips, the errors and the faults of
upstream that the port keeps.

usql finds the layer at the cursor, and asks it for the node there:

```go
layers, err := usqlConfig.Layers(ctx, parser, src, lookup)
for _, l := range slices.Backward(layers) {
	if holds(l.Ranges, cursor) {
		node, _ := l.Tree.RootNode().DescendantForByteRange(cursor, cursor)
		// ...
	}
}
```

`holds` is code of usql that reports whether one of the ranges holds the
offset.

`WithReplacer` is an API that upstream does not have (D28). `replace.go`
holds it, and the options of `Layers`. A variable of the input of usql, such
as `:id`, is an error in a SQL grammar. So usql replaces each variable with a
placeholder of the same length before the SQL layer is parsed, and every
offset of the layer stays the offset of the input (D101). Layers walks the
nodes of the parent tree inside the ranges of the layer, from the root down,
and asks the replacer about each one. The replacer does not see the children
of a node that it replaces. The root layer has no parent, so its text stays.
The queries match the text of `src`, and only the parse of the layer reads
the new text. A text whose length is not the length of its node is an error
of `Layers`.

The test module shows it with the grammar `usql` and the SQL grammar of
DerekStride. `select * from :tbl where id = :id` gives a SQL layer with an
error, and with this replacer the layer has no error:

```go
func placeholder(_ string, n transit.Node, src []byte) ([]byte, bool) {
	if n.Kind() != "variable" {
		return nil, false
	}
	name, _ := n.ChildByFieldName("name")
	text := src[n.StartByte():n.EndByte()]
	id := "_" + string(src[name.StartByte():name.EndByte()])
	switch {
	case bytes.HasPrefix(text, []byte(":{?")): // TRUE, then spaces
		return append([]byte("TRUE"), bytes.Repeat([]byte(" "), len(text)-4)...), true
	case bytes.HasPrefix(text, []byte(":'")): // a string
		return []byte("'" + id + "'"), true
	case bytes.HasPrefix(text, []byte(`:"`)): // a quoted identifier
		return []byte(`"` + id + `"`), true
	}
	return []byte(id), true // an identifier
}

layers, err := usqlConfig.Layers(ctx, parser, src, lookup, inject.WithReplacer(placeholder))
```

psql gives `TRUE` or `FALSE` for `:{?name}`, so its placeholder is `TRUE`
with spaces after it. The name of a variable needs at least one character,
so each placeholder fits.

`Layer.Text` and `Layer.StatesAt` are an API that upstream does not have
(D28, D111). `Text` is the text that the layer parses: the text of
`Layers`, with the replacements of `WithReplacer` for that layer. It has the
length and the offsets of the text of `Layers`. When nothing is replaced,
it is the text of `Layers` and not a copy.

To complete in a layer, usql runs `StatesAt` on `Text` with the language of
the layer. A layer whose ranges do not cover the whole text also needs its
ranges on the parser. Without them, the parser reads the text of the other
layers too. For example, the quote of `\echo 'it` before a statement starts
a string in SQL that holds the statement. `Layer.StatesAt` sets the language
and the included ranges of the layer on the parser, and it gives the tree of
the layer as the old tree. It clears the included ranges before it returns,
as `Layers` does:

```go
for _, l := range slices.Backward(layers) {
	if holds(l.Ranges, cursor) {
		states, err := l.StatesAt(ctx, parser, cursor)
		// ...
	}
}
```

The test module parses `select * from :tbl where id = :id;` after a meta
command, with `usql.LanguageFor` and the options of PostgreSQL, and with the
package `postgres` for the statement. At the start of `:tbl`, the states
accept `table_ref` and `relation_expr`. After `where `, they accept `a_expr`
and `columnref`. At both offsets, the states are the states of a parse of
`select * from _tbl where id = _id;` alone.

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

// Queries holds queries/ of the grammar: its own queries, and in
// queries/<grammar>/ the queries of another grammar that its
// tree-sitter.json lists (D84).
var Queries embed.FS

// Keywords returns the names of the keywords of the grammar: the symbols
// that the keyword lex table accepts, and the reserved words (D77).
func Keywords() []string

// NodeTypes returns the node types of node-types.json: the fields and the
// children of each node, and the supertypes.
func NodeTypes() []transit.NodeType
```

The numbers in this sketch are examples. The generator writes the real ones.
`transit.NodeType` is a type of the runtime that `NodeTypes` needs, and it is
in the list above.

The name of each constant comes from the C name that `render.rs` gives the
symbol or the field. The prefix `sym_` becomes `Sym`, `anon_sym_` becomes
`AnonSym`, `aux_sym_` becomes `AuxSym`, `alias_sym_` becomes `AliasSym`,
`anon_alias_sym_` becomes `AnonAliasSym`, `field_` becomes `Field`, and
`ts_builtin_sym_end` becomes `BuiltinSymEnd`. Each part of the rest between
two underscores starts with an upper case letter, and the rest of the part
stays as it is. So `sym_keyword_select` becomes `SymKeywordSelect`, and
`anon_sym_LBRACE` becomes `AnonSymLBRACE`. When two C names give the same
Go name, such as `sym_value` and `sym__value`, the later one gets the number
2 at its end, as `render.rs` does for two C names. The comment of each
constant is the name of the symbol in the grammar. D77 holds these rules, and
the rule that the grammar `go` gets the package `golang`.

A grammar that has an external scanner gets its scanner from the function
`newScanner` of `scanner.go`, which a person ports (`GRAMMAR.md`). It
returns a value that implements the scanner of the tables, so `Language`
wires `Scan`, `Serialize` and `Deserialize`. The runtime gives `Scan` the
valid external tokens of each state from the tables.

### The package usql

The package `github.com/xo/transit/grammars/usql` holds the grammar of the
input of usql, which xo writes (D13, D42). It is one language for every SQL
dialect, and its scanner takes the options of the dialect (D108). A person
writes this API in `options.go`, beside the generated files:

```go
package usql

// Options are the options of the syntax of a SQL dialect that the external
// scanner reads. Each field but BeginEndBlocks and Batches is a flag of the
// type Syntax of dbmeta, with the same name.
type Options struct {
	DollarQuotes  bool // $tag$ ... $tag$ and $$ ... $$ are a string
	BlockComments bool // /* ... */ is a comment
	SlashComments bool // // starts a comment
	HashComments  bool // # starts a comment
	Backticks     bool // `...` is a quoted identifier

	BeginEndBlocks bool // a stored program with BEGIN ... END is one statement
	Batches        bool // a batch of CQL, BEGIN BATCH ... APPLY BATCH, is one statement
}

// LanguageFor returns the language with an external scanner that reads
// opts. Every set of options uses the tables of Language.
func LanguageFor(opts Options) *transit.Language
```

`Language` gives the language with dollar quotes and block comments, the
options of PostgreSQL, because the client lexer of PostgreSQL is the
reference of the grammar (D101). `LanguageFor` gives `Language` for these
options, and for other options it builds a language once and keeps it. A
new option is a new field and a new flag of the scanner, and no new grammar
(D108). The package reads the tables of `Language` with `abi.TablesOf`, a
function of the internal package `abi` that the root package sets. So the
root package exports nothing new for it.

`BeginEndBlocks` is the option of D112, and it is off by default. With it
on, a statement that starts with `CREATE ... PROCEDURE`, `FUNCTION`,
`TRIGGER` or `EVENT` keeps its `BEGIN ... END` body, and a `;` inside the
body does not end the statement. `docs/USQL.md` says which words the
scanner reads.

`Batches` keeps a batch of CQL in one statement, and it is off by default.
With it on, a statement that starts with `BEGIN BATCH`, `BEGIN UNLOGGED
BATCH` or `BEGIN COUNTER BATCH` runs to the first `;` after `APPLY BATCH`.

## The module styles

The package `github.com/xo/transit/styles` holds the styles of transit (D65).
A style gives a color and a font to each capture name of a highlight query.
The package is a module of its own, in `styles/` (D99). It imports only the
standard library, and it does not require the root module. It exports:

```go
package styles

// Style is a style: an entry for each capture name, and the default entry.
type Style struct {
	Name     string           // such as "monokai"
	Source   string           // where the colors come from, and the license
	Default  Entry            // the entry of text that no capture names
	Captures map[string]Entry // the entry of each capture name
}

// Get returns the bundled style of a name. Names returns the names of the
// bundled styles, sorted.
func Get(name string) (*Style, bool)
func Names() []string

// Parse reads a style file of the consumer, in the form of the bundled ones.
func Parse(data []byte) (*Style, error)

// Lookup tries the whole capture name, then each shorter prefix by dots,
// then the default entry without its background. Background returns the
// background of the default entry, which is the background of the style.
func (s *Style) Lookup(capture string) Entry
func (s *Style) Background() Color

// Entry is a whole entry: a field that it leaves out does not come from
// another entry.
type Entry struct {
	Fg, Bg                  Color
	Bold, Italic, Underline bool
}

func ParseEntry(s string) (Entry, error)
func (e Entry) String() string

// Color keeps the kind that the file writes.
type Color struct {
	Kind  ColorKind // ColorNone, ColorANSI, Color256 or ColorRGB
	Value uint32    // 0 to 15, 0 to 255, or 0xrrggbb
}

func (c Color) IsSet() bool
func (c Color) String() string
func (c Color) RGB() (r, g, b uint8)
func (c Color) To256() Color
func (c Color) To16() Color

type Error string
const ErrSyntax Error = "invalid style"
```

A style is one JSON file with the keys `name`, `source`, `default` and
`captures`, and `//go:embed` holds the files. `styles/chroma/` holds the 74
styles of chroma `v2.27.0`, which the command `test/cmd/chromastyles`
converts. `styles/themes/` holds the styles that a person makes by hand. A
value is a style string. Its words are `bold`, `italic`, `underline`, their
`no` forms, a color, and `bg:` before the background color. A color is
`#rgb` or `#rrggbb`, a name of chroma for one of the 16 ANSI colors, such as
`#ansired`, or `#ansi` and the number of one of the 256 colors, such as
`#ansi208`.

The package does not look at the terminal. `RGB` gives an ANSI color and one
of the 256 colors in the default palette of xterm. `To256` reduces an RGB
color to the nearest of the colors 16 to 255, and `To16` reduces a color to
the nearest ANSI color. A line editor keeps the background of the terminal,
so `Lookup` gives no background for text that no capture names. An entry of
a capture has a background only when the style sets one for it, such as for
an error.

rline draws a capture of the highlight query like this:

```go
style, _ := styles.Get("monokai")
e := style.Lookup(capture.Name)
fg := e.Fg.To256() // for a terminal of 256 colors
```

`styles/captures.txt` lists the capture names of the highlight queries of
the grammar set. The test `TestCaptures` makes sure that each name reaches an
entry of each bundled style through its prefixes, without the default entry.

## What rline and usql call

rline highlights with `Parse`, `Edit`, `NewQuery` once, and `Captures` with a
byte range for the rows that it shows. For a language with injections, it
takes the layers from `inject` and runs its highlight query on each tree.
It takes the entry of each capture from `styles`. usql takes the layers of
its input from `inject`. On the layer at the cursor, it completes with
`DescendantForByteRange`, `Parent`, `FieldNameForChild`, `StatesAt` and
`LookaheadIterator`. It decides what kind of name goes at the cursor from
the symbols that come back and from its own queries (D6).
`RLINE.md` and `USQL.md` hold the details.

The sample programs of D53 make these calls. `_example/highlight` highlights
the input of usql as rline will, and `_example/complete` completes at a
cursor as usql will. They live in the module `github.com/xo/transit/_example`,
so the root module still requires no other module.
