package generate

import (
	"cmp"
	"hash/fnv"
	"iter"
	"maps"
	"slices"
)

// This file ports crates/generate/src/rules.rs: the flat rule representation
// of the generator. The nodes live in one pool that only grows, and a node
// refers to its children by index, so each pass walks the pool and rewrites
// nodes in place.

// SymbolType is the kind of a symbol. The order of the values is the order of
// upstream, and symbols sort by it.
//
// SymbolType is SymbolType.
type SymbolType uint8

// The kinds of symbol.
const (
	SymbolExternal SymbolType = iota
	SymbolEnd
	SymbolEndOfNonTerminalExtra
	SymbolTerminal
	SymbolNonTerminal
)

// Associativity is the associativity of a precedence.
//
// Associativity is Associativity. Upstream wraps it in an Option, and
// AssociativityNone stands for the None of that Option.
type Associativity uint8

// The associativities.
const (
	AssociativityNone Associativity = iota
	AssociativityLeft
	AssociativityRight
)

// Alias is an alias of a rule: the name that it shows, and whether the node is
// named.
//
// Alias is Alias.
type Alias struct {
	Value   StrID
	IsNamed bool
}

// Kind returns the variable type that the alias gives.
//
// Kind is Alias::kind.
func (a Alias) Kind() VariableType {
	if a.IsNamed {
		return VariableNamed
	}
	return VariableAnonymous
}

// compareBool orders false before true, as Rust does.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	}
	return 1
}

// PrecedenceKind is the kind of a precedence.
type PrecedenceKind uint8

// The kinds of precedence, in the order of upstream.
const (
	PrecedenceNone PrecedenceKind = iota
	PrecedenceInteger
	PrecedenceName
)

// Precedence is a precedence: none, a number, or a name.
//
// Precedence is Precedence, an enum with data upstream.
type Precedence struct {
	Kind    PrecedenceKind
	Integer int32
	Name    StrID
}

// MetadataParams is the metadata of a rule. A zero Alias.Value and a zero
// Field stand for the None of upstream.
//
// MetadataParams is MetadataParams.
type MetadataParams struct {
	Precedence        Precedence
	Associativity     Associativity
	DynamicPrecedence int32
	Alias             Alias
	HasAlias          bool
	Field             StrID
	IsToken           bool
	IsMainToken       bool
}

// ExternalTokenIndex is the index of an external token. It is a type of the
// macro symbol_index, which gives each kind of symbol its own index.
type ExternalTokenIndex uint32

// TerminalIndex is the index of a terminal. It is a type of the macro
// symbol_index.
type TerminalIndex uint32

// NonTerminalIndex is the index of a non-terminal. It is a type of the macro
// symbol_index.
type NonTerminalIndex uint32

// Symbol returns the symbol of the index.
func (i ExternalTokenIndex) Symbol() Symbol { return Symbol{kind: SymbolExternal, index: uint32(i)} }

// Symbol returns the symbol of the index.
func (i TerminalIndex) Symbol() Symbol { return Symbol{kind: SymbolTerminal, index: uint32(i)} }

// Symbol returns the symbol of the index.
func (i NonTerminalIndex) Symbol() Symbol { return Symbol{kind: SymbolNonTerminal, index: uint32(i)} }

// Symbol is a symbol of a grammar: a kind, and the index in the table of that
// kind.
//
// Symbol is Symbol. Two symbols compare with ==, and CompareSymbol orders them
// by kind and then by index.
type Symbol struct {
	kind  SymbolType
	index uint32
}

// The symbols with no index.
var (
	// SymbolEndValue is the symbol at the end of the input.
	SymbolEndValue = Symbol{kind: SymbolEnd}
	// SymbolEndOfNonTerminalExtraValue ends a non-terminal extra.
	SymbolEndOfNonTerminalExtraValue = Symbol{kind: SymbolEndOfNonTerminalExtra}
)

// NonTerminalSymbol returns the non-terminal with an index.
//
// NonTerminalSymbol is Symbol::non_terminal.
func NonTerminalSymbol(index int) Symbol {
	return Symbol{kind: SymbolNonTerminal, index: uint32(index)}
}

// TerminalSymbol returns the terminal with an index.
//
// TerminalSymbol is Symbol::terminal.
func TerminalSymbol(index int) Symbol {
	return Symbol{kind: SymbolTerminal, index: uint32(index)}
}

// ExternalSymbol returns the external token with an index.
//
// ExternalSymbol is Symbol::external.
func ExternalSymbol(index int) Symbol {
	return Symbol{kind: SymbolExternal, index: uint32(index)}
}

// Kind returns the kind of the symbol.
//
// Kind is Symbol::kind.
func (s Symbol) Kind() SymbolType {
	return s.kind
}

// ExternalIndex returns the index of an external token.
//
// ExternalIndex is Symbol::external_index.
func (s Symbol) ExternalIndex() (ExternalTokenIndex, bool) {
	return ExternalTokenIndex(s.index), s.kind == SymbolExternal
}

// TerminalIndex returns the index of a terminal.
//
// TerminalIndex is Symbol::terminal_index.
func (s Symbol) TerminalIndex() (TerminalIndex, bool) {
	return TerminalIndex(s.index), s.kind == SymbolTerminal
}

// NonTerminalIndex returns the index of a non-terminal.
//
// NonTerminalIndex is Symbol::non_terminal_index.
func (s Symbol) NonTerminalIndex() (NonTerminalIndex, bool) {
	return NonTerminalIndex(s.index), s.kind == SymbolNonTerminal
}

// packedKey returns the kind and the index in one number, which orders as
// CompareSymbol does.
//
// packedKey is Symbol::packed_key.
func (s Symbol) packedKey() uint64 {
	return uint64(s.kind)<<32 | uint64(s.index)
}

// CompareSymbol orders symbols by kind, then by index, as the Ord of Symbol
// does.
func CompareSymbol(a, b Symbol) int {
	return cmp.Compare(a.packedKey(), b.packedKey())
}

// RuleKind is the kind of a rule node.
type RuleKind uint8

// The kinds of rule node, in the order of the enum Rule.
const (
	RuleBlank RuleKind = iota
	RuleString
	RulePattern
	RuleNamedSymbol
	RuleSym
	RuleSeq
	RuleChoice
	RuleRepeat
	RuleEOF
	RuleMetadata
	RuleReserved
)

// Rule is one node of a rule pool.
//
// Rule is Rule, an enum with data upstream. The fields that a kind uses are:
//
//   - RuleString and RuleNamedSymbol: Str.
//   - RulePattern: Str for the pattern and Flags for its flags.
//   - RuleSym: Sym.
//   - RuleSeq and RuleChoice: Children.
//   - RuleRepeat: Child.
//   - RuleMetadata: Params and Child.
//   - RuleReserved: Child, and Str for the context.
type Rule struct {
	Kind     RuleKind
	Str      StrID
	Flags    StrID
	Sym      Symbol
	Children RuleIDRange
	Child    RuleID
	Params   ParamsID
}

// Symbol returns the symbol of a RuleSym node.
//
// Symbol is Rule::symbol.
func (r Rule) Symbol() (Symbol, bool) {
	return r.Sym, r.Kind == RuleSym
}

// RulePool is the pool of the rule nodes, of their children and their
// metadata, and of the strings that they intern. It only grows. A pass
// rewrites nodes in place, and it can leave a subtree that nothing reaches.
//
// RulePool is RulePool.
type RulePool struct {
	nodes    []Rule
	children []RuleID
	params   []MetadataParams
	strPool  *StrPool
	// scratch is the stack that Choice reuses to flatten choices
	scratch []RuleID
}

// NewRulePool returns an empty pool.
//
// NewRulePool is RulePool::default.
func NewRulePool() *RulePool {
	return &RulePool{strPool: NewStrPool()}
}

// Node returns a node.
//
// Node is RulePool::node.
func (p *RulePool) Node(id RuleID) Rule {
	return p.nodes[id.Index()]
}

// NodeCount returns the number of nodes.
//
// NodeCount is RulePool::node_count.
func (p *RulePool) NodeCount() int {
	return len(p.nodes)
}

// SetNode replaces a node.
//
// SetNode is RulePool::set_node.
func (p *RulePool) SetNode(id RuleID, node Rule) {
	p.nodes[id.Index()] = node
}

// PushNode adds a node and returns its id.
//
// PushNode is RulePool::push_node.
func (p *RulePool) PushNode(node Rule) RuleID {
	id := RuleID(len(p.nodes))
	p.nodes = append(p.nodes, node)
	return id
}

// ChildSlice returns the children of a range.
//
// ChildSlice is RulePool::child_slice.
func (p *RulePool) ChildSlice(r RuleIDRange) []RuleID {
	return p.children[r.Start : r.Start+r.Len]
}

// PushChildren adds children and returns their range.
//
// PushChildren is RulePool::push_children.
func (p *RulePool) PushChildren(ids []RuleID) RuleIDRange {
	start := uint32(len(p.children))
	p.children = append(p.children, ids...)
	return RuleIDRange{Start: start, Len: uint32(len(ids))}
}

// Params returns metadata.
//
// Params is RulePool::params.
func (p *RulePool) Params(id ParamsID) MetadataParams {
	return p.params[id]
}

// PushParams adds metadata and returns its id.
//
// PushParams is RulePool::push_params.
func (p *RulePool) PushParams(params MetadataParams) ParamsID {
	id := ParamsID(len(p.params))
	p.params = append(p.params, params)
	return id
}

// SetParams replaces metadata.
//
// SetParams is RulePool::set_params.
func (p *RulePool) SetParams(id ParamsID, params MetadataParams) {
	p.params[id] = params
}

// metadataWith gives content the metadata that f sets. When content is
// already a metadata node that is not a token, f changes that metadata in
// place. Otherwise a new metadata node wraps content.
//
// metadataWith is RulePool::metadata_with.
func (p *RulePool) metadataWith(content RuleID, f func(*MetadataParams)) RuleID {
	if n := p.Node(content); n.Kind == RuleMetadata {
		params := p.Params(n.Params)
		if !params.IsToken {
			f(&params)
			p.SetParams(n.Params, params)
			return content
		}
	}
	var params MetadataParams
	f(&params)
	id := p.PushParams(params)
	return p.PushNode(Rule{Kind: RuleMetadata, Params: id, Child: content})
}

// Blank adds a blank node.
//
// Blank is RulePool::blank.
func (p *RulePool) Blank() RuleID {
	return p.PushNode(Rule{Kind: RuleBlank})
}

// String adds a string node.
//
// String is RulePool::string.
func (p *RulePool) String(value StrID) RuleID {
	return p.PushNode(Rule{Kind: RuleString, Str: value})
}

// Pattern adds a pattern node.
//
// Pattern is RulePool::pattern.
func (p *RulePool) Pattern(value, flags StrID) RuleID {
	return p.PushNode(Rule{Kind: RulePattern, Str: value, Flags: flags})
}

// NamedSymbol adds a node that names a rule.
//
// NamedSymbol is RulePool::named_symbol.
func (p *RulePool) NamedSymbol(name StrID) RuleID {
	return p.PushNode(Rule{Kind: RuleNamedSymbol, Str: name})
}

// Field gives content a field name.
//
// Field is RulePool::field.
func (p *RulePool) Field(name StrID, content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) { m.Field = name })
}

// Alias gives content an alias.
//
// Alias is RulePool::alias.
func (p *RulePool) Alias(content RuleID, value StrID, isNamed bool) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) {
		m.Alias, m.HasAlias = Alias{Value: value, IsNamed: isNamed}, true
	})
}

// Token makes content a token.
//
// Token is RulePool::token.
func (p *RulePool) Token(content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) { m.IsToken = true })
}

// ImmediateToken makes content a token that follows the last token with no
// extra between them.
//
// ImmediateToken is RulePool::immediate_token.
func (p *RulePool) ImmediateToken(content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) {
		m.IsToken = true
		m.IsMainToken = true
	})
}

// Prec gives content a precedence.
//
// Prec is RulePool::prec.
func (p *RulePool) Prec(value Precedence, content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) { m.Precedence = value })
}

// PrecLeft gives content a precedence that associates to the left.
//
// PrecLeft is RulePool::prec_left.
func (p *RulePool) PrecLeft(value Precedence, content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) {
		m.Associativity = AssociativityLeft
		m.Precedence = value
	})
}

// PrecRight gives content a precedence that associates to the right.
//
// PrecRight is RulePool::prec_right.
func (p *RulePool) PrecRight(value Precedence, content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) {
		m.Associativity = AssociativityRight
		m.Precedence = value
	})
}

// PrecDynamic gives content a dynamic precedence.
//
// PrecDynamic is RulePool::prec_dynamic.
func (p *RulePool) PrecDynamic(value int32, content RuleID) RuleID {
	return p.metadataWith(content, func(m *MetadataParams) { m.DynamicPrecedence = value })
}

// Repeat adds a node that repeats content one or more times.
//
// Repeat is RulePool::repeat.
func (p *RulePool) Repeat(content RuleID) RuleID {
	return p.PushNode(Rule{Kind: RuleRepeat, Child: content})
}

// EOF adds a node for the end of the input.
//
// EOF is RulePool::eof.
func (p *RulePool) EOF() RuleID {
	return p.PushNode(Rule{Kind: RuleEOF})
}

// Seq adds a sequence of nodes.
//
// Seq is RulePool::seq.
func (p *RulePool) Seq(ids []RuleID) RuleID {
	r := p.PushChildren(ids)
	return p.PushNode(Rule{Kind: RuleSeq, Children: r})
}

// TrySeq adds a sequence of the nodes that f makes from items. The children
// are reserved first, so that the nodes that f adds do not come between them.
// It stops at the first error of f.
//
// TrySeq is RulePool::try_seq.
func TrySeq[T any](p *RulePool, items []T, f func(*RulePool, T) (RuleID, error)) (RuleID, error) {
	start := len(p.children)
	p.children = append(p.children, make([]RuleID, len(items))...)
	for i, item := range items {
		id, err := f(p, item)
		if err != nil {
			return 0, err
		}
		p.children[start+i] = id
	}
	r := RuleIDRange{Start: uint32(start), Len: uint32(len(items))}
	return p.PushNode(Rule{Kind: RuleSeq, Children: r}), nil
}

// Choice adds a choice of nodes. It flattens nested choices and drops a node
// that is the same as one before it, and it keeps a Choice node even for one
// member.
//
// Choice is RulePool::choice.
func (p *RulePool) Choice(ids []RuleID) RuleID {
	// The members are built at the end of the children. The walk only reads
	// nodes, so nothing else grows the children while it runs.
	start := len(p.children)
	stack := p.scratch[:0]
	for _, id := range slices.Backward(ids) {
		stack = append(stack, id)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n := p.Node(id); n.Kind == RuleChoice {
			base := len(stack)
			stack = append(stack, p.ChildSlice(n.Children)...)
			slices.Reverse(stack[base:])
		} else if !slices.ContainsFunc(p.children[start:], func(e RuleID) bool { return p.SubtreeEqual(e, id) }) {
			p.children = append(p.children, id)
		}
	}
	p.scratch = stack
	r := RuleIDRange{Start: uint32(start), Len: uint32(len(p.children) - start)}
	return p.PushNode(Rule{Kind: RuleChoice, Children: r})
}

// Reserved adds a node that parses content with the reserved words of a
// context.
//
// Reserved is RulePool::reserved.
func (p *RulePool) Reserved(content RuleID, ctx StrID) RuleID {
	return p.PushNode(Rule{Kind: RuleReserved, Child: content, Str: ctx})
}

// Intern interns a string in the pool.
//
// Intern is RulePool::intern.
func (p *RulePool) Intern(s string) StrID {
	return p.strPool.Intern(s)
}

// Resolve returns an interned string.
//
// Resolve is RulePool::resolve.
func (p *RulePool) Resolve(id StrID) string {
	return p.strPool.Resolve(id)
}

// Interner returns the string pool.
//
// Interner is RulePool::into_interner.
func (p *RulePool) Interner() *StrPool {
	return p.strPool
}

// SubtreeHash returns a hash of a subtree. Two subtrees that SubtreeEqual
// finds equal have the same hash.
//
// SubtreeHash is RulePool::subtree_hash. Upstream hashes with FxHasher. The
// hash only finds candidates in a memo, and SubtreeEqual decides between
// them, so another hash gives the same result, and this one uses FNV.
func (p *RulePool) SubtreeHash(root RuleID) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	put := func(x uint64) {
		for i := range 8 {
			buf[i] = byte(x >> (8 * i))
		}
		_, _ = h.Write(buf[:])
	}
	stack := []RuleID{root}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := p.Node(id)
		put(uint64(n.Kind))
		switch n.Kind {
		case RuleString, RuleNamedSymbol:
			put(uint64(n.Str))
		case RulePattern:
			put(uint64(n.Str))
			put(uint64(n.Flags))
		case RuleSym:
			put(n.Sym.packedKey())
		case RuleSeq, RuleChoice:
			put(uint64(n.Children.Len))
			base := len(stack)
			stack = append(stack, p.ChildSlice(n.Children)...)
			slices.Reverse(stack[base:])
		case RuleRepeat:
			stack = append(stack, n.Child)
		case RuleMetadata:
			m := p.Params(n.Params)
			put(uint64(m.Precedence.Kind))
			put(uint64(uint32(m.Precedence.Integer)))
			put(uint64(m.Precedence.Name))
			put(uint64(m.Associativity))
			put(uint64(uint32(m.DynamicPrecedence)))
			put(uint64(m.Alias.Value))
			put(boolWord(m.HasAlias, m.Alias.IsNamed, m.IsToken, m.IsMainToken))
			put(uint64(m.Field))
			stack = append(stack, n.Child)
		case RuleReserved:
			put(uint64(n.Str))
			stack = append(stack, n.Child)
		}
	}
	return h.Sum64()
}

// boolWord packs booleans into one word for a hash.
func boolWord(bs ...bool) uint64 {
	var w uint64
	for i, b := range bs {
		if b {
			w |= 1 << i
		}
	}
	return w
}

// SubtreeEqual reports whether two subtrees are the same.
//
// SubtreeEqual is RulePool::subtree_eq.
func (p *RulePool) SubtreeEqual(a, b RuleID) bool {
	type pair struct{ a, b RuleID }
	stack := []pair{{a, b}}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := p.Node(top.a), p.Node(top.b)
		if x.Kind != y.Kind {
			return false
		}
		switch x.Kind {
		case RuleBlank, RuleEOF:
		case RuleString, RuleNamedSymbol:
			if x.Str != y.Str {
				return false
			}
		case RulePattern:
			if x.Str != y.Str || x.Flags != y.Flags {
				return false
			}
		case RuleSym:
			if x.Sym != y.Sym {
				return false
			}
		case RuleSeq, RuleChoice:
			if x.Children.Len != y.Children.Len {
				return false
			}
			xs, ys := p.ChildSlice(x.Children), p.ChildSlice(y.Children)
			for i := range xs {
				stack = append(stack, pair{xs[i], ys[i]})
			}
		case RuleRepeat:
			stack = append(stack, pair{x.Child, y.Child})
		case RuleMetadata:
			if p.Params(x.Params) != p.Params(y.Params) {
				return false
			}
			stack = append(stack, pair{x.Child, y.Child})
		case RuleReserved:
			if x.Str != y.Str {
				return false
			}
			stack = append(stack, pair{x.Child, y.Child})
		}
	}
	return true
}

// SubtreeMatchesEmptyString reports whether the subtree at id can only match
// the empty string.
//
// SubtreeMatchesEmptyString is RulePool::subtree_matches_empty_str.
func (p *RulePool) SubtreeMatchesEmptyString(id RuleID) bool {
	n := p.Node(id)
	switch n.Kind {
	case RuleString:
		return p.Resolve(n.Str) == ""
	case RuleMetadata, RuleRepeat, RuleReserved:
		return p.SubtreeMatchesEmptyString(n.Child)
	case RuleChoice:
		return slices.ContainsFunc(p.ChildSlice(n.Children), p.SubtreeMatchesEmptyString)
	case RuleSeq:
		for _, c := range p.ChildSlice(n.Children) {
			if !p.SubtreeMatchesEmptyString(c) {
				return false
			}
		}
		return true
	}
	return false
}

// RuleIsReferenced reports whether a rule names target. It is how the
// generator finds whether a rule uses a variable. When isExternal is true, a
// named symbol at the top of the rule does not count: for an external rule and
// a normal rule that are both called foo, the external rule named foo does not
// use foo unless another rule inside it does.
//
// RuleIsReferenced is RulePool::rule_is_referenced.
func (p *RulePool) RuleIsReferenced(id RuleID, target StrID, isExternal bool) bool {
	n := p.Node(id)
	switch n.Kind {
	case RuleNamedSymbol:
		return n.Str == target && !isExternal
	case RuleChoice, RuleSeq:
		for _, c := range p.ChildSlice(n.Children) {
			if p.RuleIsReferenced(c, target, false) {
				return true
			}
		}
		return false
	case RuleMetadata, RuleReserved:
		return p.RuleIsReferenced(n.Child, target, isExternal)
	case RuleRepeat:
		return p.RuleIsReferenced(n.Child, target, false)
	}
	return false
}

// CollectReferencedIDs appends to out the name of each named symbol that id
// reaches. When skipTopLevel is true, a named symbol at the root of id does
// not count, which an entry of externals needs, because it names itself.
//
// CollectReferencedIDs is RulePool::collect_referenced_ids.
func (p *RulePool) CollectReferencedIDs(id RuleID, skipTopLevel bool, out *[]StrID) {
	n := p.Node(id)
	switch n.Kind {
	case RuleNamedSymbol:
		if !skipTopLevel {
			*out = append(*out, n.Str)
		}
	case RuleChoice, RuleSeq:
		for _, c := range p.ChildSlice(n.Children) {
			p.CollectReferencedIDs(c, false, out)
		}
	case RuleMetadata, RuleReserved:
		p.CollectReferencedIDs(n.Child, skipTopLevel, out)
	case RuleRepeat:
		p.CollectReferencedIDs(n.Child, false, out)
	}
}

// RuleID is the index of a node in a rule pool.
//
// RuleID is RuleId.
type RuleID uint32

// Index returns the index of the node.
//
// Index is RuleId::index.
func (id RuleID) Index() int {
	return int(id)
}

// ParamsID is the index of metadata in a rule pool.
//
// ParamsID is ParamsId.
type ParamsID uint32

// RuleIDRange is the range [Start, Start+Len) of the children of a pool.
//
// RuleIDRange is RuleIdRange.
type RuleIDRange struct {
	Start uint32
	Len   uint32
}

// TokenSet is a set of tokens, as bit vectors. A token is a small number, of
// about 400 at most, so a bit for each token holds a set well.
//
// TokenSet is TokenSet.
type TokenSet struct {
	terminalBits          BitVec
	externalBits          BitVec
	eof                   bool
	endOfNonTerminalExtra bool
}

// NewTokenSetWithCapacity returns an empty set with room for the tokens.
//
// NewTokenSetWithCapacity is TokenSet::with_capacity.
func NewTokenSetWithCapacity(terminals, externals int) TokenSet {
	return TokenSet{
		terminalBits: NewBitVecWithCapacity(terminals),
		externalBits: NewBitVecWithCapacity(externals),
	}
}

// All returns each symbol in the set: the terminals, then the external
// tokens, then the end of the input, then the end of a non-terminal extra.
//
// All is TokenSet::iter.
func (s *TokenSet) All() iter.Seq[Symbol] {
	return func(yield func(Symbol) bool) {
		for i := range setBits(s.terminalBits.Words()) {
			if !yield(TerminalSymbol(i)) {
				return
			}
		}
		for i := range setBits(s.externalBits.Words()) {
			if !yield(ExternalSymbol(i)) {
				return
			}
		}
		if s.eof && !yield(SymbolEndValue) {
			return
		}
		if s.endOfNonTerminalExtra {
			yield(SymbolEndOfNonTerminalExtraValue)
		}
	}
}

// Terminals returns the index of each terminal in the set.
//
// Terminals is TokenSet::terminals.
func (s *TokenSet) Terminals() iter.Seq[TerminalIndex] {
	return func(yield func(TerminalIndex) bool) {
		for i := range setBits(s.terminalBits.Words()) {
			if !yield(TerminalIndex(i)) {
				return
			}
		}
	}
}

// Externals returns the index of each external token in the set.
//
// Externals is TokenSet::externals.
func (s *TokenSet) Externals() iter.Seq[ExternalTokenIndex] {
	return func(yield func(ExternalTokenIndex) bool) {
		for i := range setBits(s.externalBits.Words()) {
			if !yield(ExternalTokenIndex(i)) {
				return
			}
		}
	}
}

// Contains reports whether the set holds a symbol. It panics for a
// non-terminal, which a token set never holds.
//
// Contains is TokenSet::contains.
func (s *TokenSet) Contains(sym Symbol) bool {
	switch sym.kind {
	case SymbolNonTerminal:
		panic("generate: a TokenSet cannot hold a non-terminal")
	case SymbolTerminal:
		v, _ := s.terminalBits.Get(int(sym.index))
		return v
	case SymbolExternal:
		v, _ := s.externalBits.Get(int(sym.index))
		return v
	case SymbolEnd:
		return s.eof
	}
	return s.endOfNonTerminalExtra
}

// ContainsTerminal reports whether the set holds the terminal with an index.
//
// ContainsTerminal is TokenSet::contains_terminal.
func (s *TokenSet) ContainsTerminal(index int) bool {
	v, _ := s.terminalBits.Get(index)
	return v
}

// TerminalWords returns the words of the terminal bits.
//
// TerminalWords is TokenSet::terminal_bits_words.
func (s *TokenSet) TerminalWords() []uint64 {
	return s.terminalBits.Words()
}

// Insert adds a symbol to the set. It panics for a non-terminal.
//
// Insert is TokenSet::insert.
func (s *TokenSet) Insert(sym Symbol) {
	var v *BitVec
	switch sym.kind {
	case SymbolNonTerminal:
		panic("generate: a TokenSet cannot hold a non-terminal")
	case SymbolTerminal:
		v = &s.terminalBits
	case SymbolExternal:
		v = &s.externalBits
	case SymbolEnd:
		s.eof = true
		return
	default:
		s.endOfNonTerminalExtra = true
		return
	}
	i := int(sym.index)
	if i >= v.Len() {
		v.Resize(i+1, false)
	}
	v.Set(i, true)
}

// Remove removes a symbol from the set, and reports whether the set held it.
// It panics for a non-terminal.
//
// Remove is TokenSet::remove.
func (s *TokenSet) Remove(sym Symbol) bool {
	var v *BitVec
	switch sym.kind {
	case SymbolNonTerminal:
		panic("generate: a TokenSet cannot hold a non-terminal")
	case SymbolTerminal:
		v = &s.terminalBits
	case SymbolExternal:
		v = &s.externalBits
	case SymbolEnd:
		held := s.eof
		s.eof = false
		return held
	default:
		held := s.endOfNonTerminalExtra
		s.endOfNonTerminalExtra = false
		return held
	}
	i := int(sym.index)
	if held, ok := v.Get(i); !ok || !held {
		return false
	}
	v.Set(i, false)
	for last, ok := v.Last(); ok && !last; last, ok = v.Last() {
		v.Pop()
	}
	return true
}

// IsEmpty reports whether the set holds no symbol.
//
// IsEmpty is TokenSet::is_empty.
func (s *TokenSet) IsEmpty() bool {
	return !s.eof && !s.endOfNonTerminalExtra && allZero(s.terminalBits.Words()) && allZero(s.externalBits.Words())
}

// allZero reports whether every word is zero.
func allZero(words []uint64) bool {
	for _, w := range words {
		if w != 0 {
			return false
		}
	}
	return true
}

// Len returns the number of symbols in the set.
//
// Len is TokenSet::len.
func (s *TokenSet) Len() int {
	n := 0
	if s.eof {
		n++
	}
	if s.endOfNonTerminalExtra {
		n++
	}
	for _, w := range s.terminalBits.Words() {
		n += popCount(w)
	}
	for _, w := range s.externalBits.Words() {
		n += popCount(w)
	}
	return n
}

// InsertAllTerminals adds the terminals of other, and reports whether any was
// new.
//
// InsertAllTerminals is TokenSet::insert_all_terminals.
func (s *TokenSet) InsertAllTerminals(other *TokenSet) bool {
	return s.terminalBits.InsertAll(&other.terminalBits)
}

// insertAllExternals adds the external tokens of other, and reports whether
// any was new.
//
// insertAllExternals is TokenSet::insert_all_externals.
func (s *TokenSet) insertAllExternals(other *TokenSet) bool {
	return s.externalBits.InsertAll(&other.externalBits)
}

// InsertAll adds every symbol of other, and reports whether any was new.
//
// InsertAll is TokenSet::insert_all.
func (s *TokenSet) InsertAll(other *TokenSet) bool {
	result := false
	if other.eof {
		result = result || !s.eof
		s.eof = true
	}
	if other.endOfNonTerminalExtra {
		result = result || !s.endOfNonTerminalExtra
		s.endOfNonTerminalExtra = true
	}
	result = s.InsertAllTerminals(other) || result
	result = s.insertAllExternals(other) || result
	return result
}

// Clone returns a copy of the set.
//
// Clone is the Clone of TokenSet.
func (s *TokenSet) Clone() TokenSet {
	return TokenSet{
		terminalBits:          s.terminalBits.Clone(),
		externalBits:          s.externalBits.Clone(),
		eof:                   s.eof,
		endOfNonTerminalExtra: s.endOfNonTerminalExtra,
	}
}

// Equal reports whether two sets hold the same symbols.
//
// Equal is the PartialEq of TokenSet.
func (s *TokenSet) Equal(other *TokenSet) bool {
	return s.terminalBits.Equal(&other.terminalBits) && s.externalBits.Equal(&other.externalBits) &&
		s.eof == other.eof && s.endOfNonTerminalExtra == other.endOfNonTerminalExtra
}

// Compare orders two sets by their terminals, then their external tokens,
// then the end of the input, then the end of a non-terminal extra.
//
// Compare is the Ord of TokenSet.
func (s *TokenSet) Compare(other *TokenSet) int {
	return cmp.Or(
		s.terminalBits.Compare(&other.terminalBits),
		s.externalBits.Compare(&other.externalBits),
		compareBool(s.eof, other.eof),
		compareBool(s.endOfNonTerminalExtra, other.endOfNonTerminalExtra),
	)
}

// Key returns a string that is the same for two equal sets and differs for
// two sets that are not equal, for a map key. It stands in for the Hash and
// the Eq of TokenSet. The key starts with the length of the terminal part,
// because the bytes of a part can hold any value, so no separator byte can
// mark where the part ends.
func (s *TokenSet) Key() string {
	terminals := s.terminalBits.key()
	n := len(terminals)
	return string([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}) +
		terminals + s.externalBits.key() + string([]byte{byte(boolWord(s.eof, s.endOfNonTerminalExtra))})
}

// TokenSetFrom returns the set of the symbols.
//
// TokenSetFrom is the FromIterator of TokenSet.
func TokenSetFrom(symbols iter.Seq[Symbol]) TokenSet {
	var s TokenSet
	for sym := range symbols {
		s.Insert(sym)
	}
	return s
}

// AliasMap maps a symbol to its alias.
//
// AliasMap is AliasMap, a BTreeMap upstream, which iterates in the order of
// the symbols. Sorted gives that order.
type AliasMap map[Symbol]Alias

// Sorted returns each symbol and its alias, in the order of the symbols.
func (m AliasMap) Sorted() iter.Seq2[Symbol, Alias] {
	return func(yield func(Symbol, Alias) bool) {
		for _, sym := range slices.SortedFunc(maps.Keys(m), CompareSymbol) {
			if !yield(sym, m[sym]) {
				return
			}
		}
	}
}
