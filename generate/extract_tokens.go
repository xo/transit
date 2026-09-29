package generate

import (
	"cmp"
	"slices"
	"strconv"
)

// This file ports crates/generate/src/prepare_grammar/extract_tokens.rs: the
// pass that moves the tokens of the grammar into the lexical grammar.
// Upstream wraps NonTerminalWordTokenError in the variant WordToken, which
// only passes it through, so the Go pass returns it as it is.

// ExtractTokensErrorKind is the kind of an error of the pass that extracts
// the tokens.
type ExtractTokensErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ExtractTokensEmptyString ExtractTokensErrorKind = iota
	ExtractTokensSupertypeTerminal
	ExtractTokensExternalTokenNonTerminal
	ExtractTokensNonSymbolExternalToken
	ExtractTokensNonTokenReservedWord
)

// ExtractTokensError is an error of the pass that extracts the tokens. Its
// text is the text of upstream.
//
// ExtractTokensError is ExtractTokensError.
type ExtractTokensError struct {
	Kind ExtractTokensErrorKind
	// Name is the name of the rule or the token, for every kind but
	// ExtractTokensNonSymbolExternalToken.
	Name string
}

// Error returns the text of the error.
func (e *ExtractTokensError) Error() string {
	switch e.Kind {
	case ExtractTokensEmptyString:
		return "The rule `" + e.Name + "` contains an empty string.\n\n" +
			"Tree-sitter does not support syntactic rules that contain an empty string\n" +
			"unless they are used only as the grammar's start rule.\n"
	case ExtractTokensSupertypeTerminal:
		return "Terminal rule '" + e.Name + "' cannot be used as a supertype"
	case ExtractTokensExternalTokenNonTerminal:
		return "Rule '" + e.Name + "' cannot be used as both an external token and a non-terminal rule"
	case ExtractTokensNonSymbolExternalToken:
		return "Non-symbol rules cannot be used as external tokens"
	case ExtractTokensNonTokenReservedWord:
		return "Reserved word '" + e.Name + "' must be a token"
	}
	return ""
}

// NonTerminalWordTokenError is the error of a word token that is a
// non-terminal.
//
// NonTerminalWordTokenError is NonTerminalWordTokenError.
type NonTerminalWordTokenError struct {
	SymbolName string
	// ConflictingSymbolName is the name of a rule with the same body as the
	// word, or empty when no rule has it.
	ConflictingSymbolName string
}

// Error returns the text of the error. Upstream ends it with a newline.
func (e *NonTerminalWordTokenError) Error() string {
	s := "Non-terminal symbol '" + e.SymbolName + "' cannot be used as the word token"
	if e.ConflictingSymbolName != "" {
		s += ", because its rule is duplicated in '" + e.ConflictingSymbolName + "'"
	}
	return s + "\n"
}

// extractedGrammarMeta is what the pass gives for the syntax grammar besides
// the rules. Its symbols have their final indices before the rewrites of the
// pool are committed.
//
// extractedGrammarMeta is ExtractedGrammarMeta.
type extractedGrammarMeta struct {
	kinds          []VariableType
	extraSymbols   []Symbol
	externalTokens []ExternalToken
	reservedSets   []reservedSymbolSet
	supertypes     []Symbol
	conflicts      [][]Symbol
	inline         []Symbol
	word           Symbol
	hasWord        bool
}

// reservedSymbolSet is a set of reserved words and its name.
type reservedSymbolSet struct {
	name    StrID
	symbols []Symbol
}

// tokenRewrite is a node that becomes a terminal, and the index of the
// terminal.
type tokenRewrite struct {
	id    RuleID
	token uint32
}

// tokenExtractor finds the tokens in the rules, and makes one token for each
// set of equal subtrees.
//
// tokenExtractor is TokenExtractor.
type tokenExtractor struct {
	lexical     []LexicalToken
	usageCounts []uint32
	// memo holds the tokens with each hash of a subtree. The hash only
	// narrows the search, and SubtreeEqual decides.
	memo map[uint64][]uint32
	// rewrites holds the nodes that become terminals. They wait until the
	// lexical expansion reads the original nodes.
	rewrites []tokenRewrite
}

// extractToken returns the index of the token for the subtree at tokenRoot,
// and makes the token when no equal one exists. stringName is the string of
// a string token, or zero for any other token.
//
// extractToken is TokenExtractor::extract_token.
func (e *tokenExtractor) extractToken(pool *RulePool, tokenRoot RuleID, stringName, varName StrID, auxTokenCount *uint32, isFirst bool) (uint32, error) {
	hash := pool.SubtreeHash(tokenRoot)
	for _, i := range e.memo[hash] {
		if pool.SubtreeEqual(e.lexical[i].Root, tokenRoot) {
			e.usageCounts[i]++
			return i, nil
		}
	}
	var name StrID
	var kind VariableType
	if stringName != 0 {
		if pool.Resolve(stringName) == "" && !isFirst {
			var rule string
			if varName != 0 {
				rule = pool.Resolve(varName)
			}
			return 0, &ExtractTokensError{Kind: ExtractTokensEmptyString, Name: rule}
		}
		name, kind = stringName, VariableAnonymous
	} else {
		*auxTokenCount++
		var prefix string
		if varName != 0 {
			prefix = pool.Resolve(varName)
		}
		name, kind = pool.Intern(prefix+"_token"+strconv.FormatUint(uint64(*auxTokenCount), 10)), VariableAuxiliary
	}
	// A shallow copy: the new node points to the same subtree, and becomes
	// the root of the token in the lexical grammar. The caller later
	// overwrites the node at tokenRoot with the terminal, but the copy still
	// points to the subtree.
	index := uint32(len(e.lexical))
	root := pool.PushNode(pool.Node(tokenRoot))
	e.lexical = append(e.lexical, LexicalToken{Name: name, Kind: kind, Root: root})
	e.usageCounts = append(e.usageCounts, 1)
	if e.memo == nil {
		e.memo = map[uint64][]uint32{}
	}
	e.memo[hash] = append(e.memo[hash], index)
	return index, nil
}

// extractInRoot finds the tokens of one rule. It records each node that
// becomes a terminal, so that every rule is read before a shared node
// changes. A string or a pattern is always a token. A token() is a token
// too: without other metadata, its content is the token, and with other
// metadata, the whole metadata node is.
//
// extractInRoot is TokenExtractor::extract_in_root.
func (e *tokenExtractor) extractInRoot(pool *RulePool, root RuleID, varName StrID, isFirst bool, stack *[]RuleID) error {
	var auxTokenCount uint32
	*stack = append((*stack)[:0], root)
	for len(*stack) > 0 {
		id := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		n := pool.Node(id)
		switch n.Kind {
		case RuleString:
			i, err := e.extractToken(pool, id, n.Str, varName, &auxTokenCount, isFirst)
			if err != nil {
				return err
			}
			e.rewrites = append(e.rewrites, tokenRewrite{id: id, token: i})
		case RulePattern:
			i, err := e.extractToken(pool, id, 0, varName, &auxTokenCount, isFirst)
			if err != nil {
				return err
			}
			e.rewrites = append(e.rewrites, tokenRewrite{id: id, token: i})
		case RuleMetadata:
			p := pool.Params(n.Params)
			if !p.IsToken {
				*stack = append(*stack, n.Child)
				continue
			}
			cleaned := p
			cleaned.IsToken = false
			var stringName StrID
			if inner := pool.Node(n.Child); inner.Kind == RuleString {
				stringName = inner.Str
			}
			tokenRoot := id
			if cleaned == (MetadataParams{}) {
				// only a token, so drop the wrapper
				tokenRoot = n.Child
			}
			i, err := e.extractToken(pool, tokenRoot, stringName, varName, &auxTokenCount, isFirst)
			if err != nil {
				return err
			}
			e.rewrites = append(e.rewrites, tokenRewrite{id: id, token: i})
		case RuleSeq, RuleChoice:
			base := len(*stack)
			*stack = append(*stack, pool.ChildSlice(n.Children)...)
			slices.Reverse((*stack)[base:])
		case RuleRepeat, RuleReserved:
			*stack = append(*stack, n.Child)
		}
	}
	return nil
}

// find returns the index of the token whose body equals the subtree at root.
//
// find is TokenExtractor::find.
func (e *tokenExtractor) find(pool *RulePool, root RuleID) (uint32, bool) {
	for _, i := range e.memo[pool.SubtreeHash(root)] {
		if pool.SubtreeEqual(e.lexical[i].Root, root) {
			return i, true
		}
	}
	return 0, false
}

// finalizeRewrites sorts the rewrites by node, and joins the rewrites of a
// node that the pass reached twice. A node maps to one terminal only, because
// the pass reads only subtrees that did not change.
//
// finalizeRewrites is TokenExtractor::finalize_rewrites.
func (e *tokenExtractor) finalizeRewrites() {
	slices.SortFunc(e.rewrites, func(a, b tokenRewrite) int {
		return cmp.Or(cmp.Compare(a.id, b.id), cmp.Compare(a.token, b.token))
	})
	e.rewrites = slices.Compact(e.rewrites)
}

// symbolAfterRewrites returns the symbol that the node at id holds after the
// rewrites are committed, and does not change the pool.
//
// symbolAfterRewrites is TokenExtractor::symbol_after_rewrites.
func (e *tokenExtractor) symbolAfterRewrites(pool *RulePool, id RuleID) (Symbol, bool) {
	if i, found := slices.BinarySearchFunc(e.rewrites, id, func(r tokenRewrite, id RuleID) int {
		return cmp.Compare(r.id, id)
	}); found {
		return TerminalSymbol(int(e.rewrites[i].token)), true
	}
	return pool.Node(id).Symbol()
}

// pendingTokenExtraction is the result of the pass before it is committed.
// The roots of the tokens and of the separators still point to their
// original bodies, and no other pass can change the pool until
// expand_and_commit runs.
//
// pendingTokenExtraction is PendingTokenExtraction.
type pendingTokenExtraction struct {
	grammar                    *InputGrammar
	meta                       *extractedGrammarMeta
	lexicalVariables           []LexicalToken
	separatorRoots             []RuleID
	rewrites                   []tokenRewrite
	syntaxVariableReplacements map[uint32]uint32
	syntaxVariableShift        []uint32
	stack                      []RuleID
}

// expandAndCommit expands the roots of the tokens and of the separators,
// then commits the rewrites of the terminals and renumbers the symbols of
// the syntax grammar that are left.
//
// expandAndCommit is PendingTokenExtraction::expand_and_commit.
func (p *pendingTokenExtraction) expandAndCommit() (*extractedGrammarMeta, *LexicalGrammar, error) {
	g := p.grammar
	lexicalGrammar, err := expandTokens(g.Pool, p.lexicalVariables, p.separatorRoots)
	if err != nil {
		return nil, nil, err
	}

	for _, r := range p.rewrites {
		g.Pool.SetNode(r.id, Rule{Kind: RuleSym, Sym: TerminalSymbol(int(r.token))})
	}

	replaceSymbol := func(s Symbol) Symbol {
		index, ok := s.NonTerminalIndex()
		if !ok {
			return s
		}
		if tokenIndex, ok := p.syntaxVariableReplacements[uint32(index)]; ok {
			return TerminalSymbol(int(tokenIndex))
		}
		return NonTerminalSymbol(int(uint32(index) - p.syntaxVariableShift[index]))
	}

	if len(p.syntaxVariableReplacements) > 0 {
		renumberedNodes := make([]bool, g.Pool.NodeCount())
		for _, v := range g.Variables {
			renumberRoot(g.Pool, v.Root, replaceSymbol, &p.stack, renumberedNodes)
		}
		for _, root := range g.ExternalRoots {
			renumberRoot(g.Pool, root, replaceSymbol, &p.stack, renumberedNodes)
		}
	}

	return p.meta, lexicalGrammar, nil
}

// extractTokens finds the tokens of the grammar, and decides which variables
// become tokens and how the other symbols are numbered. It changes no rule,
// and the result holds the changes until they are committed.
//
// extractTokens is extract_tokens.
func extractTokens(g *InputGrammar, interned *internedGrammarMeta) (*pendingTokenExtraction, error) {
	var extractor tokenExtractor
	var stack []RuleID

	for i, v := range g.Variables {
		if err := extractor.extractInRoot(g.Pool, v.Root, v.Name, i == 0, &stack); err != nil {
			return nil, err
		}
	}
	for i, root := range g.ExternalRoots {
		if err := extractor.extractInRoot(g.Pool, root, interned.externalTokens[i].name, false, &stack); err != nil {
			return nil, err
		}
	}
	extractor.finalizeRewrites()

	// When the whole rule of a variable became a token, and no other rule
	// uses that token, the variable leaves the syntax grammar and gives its
	// name to the token. A symbol of that variable then points to the token,
	// and the index of each later variable goes down.
	oldLen := len(g.Variables)
	replacements := map[uint32]uint32{}
	retained := make([]Variable, 0, oldLen)
	kinds := make([]VariableType, 0, oldLen)

	// the start variable never becomes a token
	retained = append(retained, g.Variables[0])
	kinds = append(kinds, interned.kinds[0])
	for i := 1; i < len(g.Variables); i++ {
		v := g.Variables[i]
		if sym, ok := extractor.symbolAfterRewrites(g.Pool, v.Root); ok {
			if index, ok := sym.TerminalIndex(); ok && extractor.usageCounts[index] == 1 {
				lexical := &extractor.lexical[index]
				if lexical.Kind == VariableAuxiliary || interned.kinds[i] != VariableHidden {
					lexical.Kind = interned.kinds[i]
					lexical.Name = v.Name
					replacements[uint32(i)] = uint32(index)
					continue
				}
			}
		}
		retained = append(retained, v)
		kinds = append(kinds, interned.kinds[i])
	}
	g.Variables = retained

	// the new index of each variable is its old index less the number of
	// variables before it that left
	shift := make([]uint32, oldLen)
	var removed uint32
	for i := range shift {
		shift[i] = removed
		if _, ok := replacements[uint32(i)]; ok {
			removed++
		}
	}
	replaceSymbol := func(s Symbol) Symbol {
		index, ok := s.NonTerminalIndex()
		if !ok {
			return s
		}
		if r, ok := replacements[uint32(index)]; ok {
			return TerminalSymbol(int(r))
		}
		return NonTerminalSymbol(int(uint32(index) - shift[index]))
	}

	// renumber each conflict, then sort it and drop the duplicates
	conflicts := make([][]Symbol, 0, len(interned.conflicts))
	for _, c := range interned.conflicts {
		result := make([]Symbol, 0, len(c))
		for _, s := range c {
			result = append(result, replaceSymbol(s))
		}
		slices.SortFunc(result, CompareSymbol)
		conflicts = append(conflicts, slices.Compact(result))
	}

	supertypes := make([]Symbol, 0, len(interned.supertypes))
	for _, s := range interned.supertypes {
		sym := replaceSymbol(s)
		// a supertype cannot become a token
		if index, ok := sym.TerminalIndex(); ok {
			return nil, &ExtractTokensError{Kind: ExtractTokensSupertypeTerminal, Name: g.Pool.Resolve(extractor.lexical[index].Name)}
		}
		supertypes = append(supertypes, sym)
	}

	inline := make([]Symbol, 0, len(interned.inline))
	for _, s := range interned.inline {
		inline = append(inline, replaceSymbol(s))
	}

	// Resolve each extra to the symbol that it becomes, with the index of a
	// non-terminal adjusted for the variables that left. The rules are
	// renumbered only after every extra is resolved. An extra that is no
	// symbol and no token is a separator.
	var separatorRoots []RuleID
	extraSymbols := make([]Symbol, 0, len(g.ExtraRoots))
	for _, root := range g.ExtraRoots {
		if s, ok := extractor.symbolAfterRewrites(g.Pool, root); ok {
			extraSymbols = append(extraSymbols, replaceSymbol(s))
		} else if i, ok := extractor.find(g.Pool, root); ok {
			extraSymbols = append(extraSymbols, TerminalSymbol(int(i)))
		} else {
			separatorRoots = append(separatorRoots, root)
		}
	}

	externalTokens := make([]ExternalToken, 0, len(g.ExternalRoots))
	for i, root := range g.ExternalRoots {
		name, kind := interned.externalTokens[i].name, interned.externalTokens[i].kind
		s, ok := extractor.symbolAfterRewrites(g.Pool, root)
		if !ok {
			return nil, &ExtractTokensError{Kind: ExtractTokensNonSymbolExternalToken}
		}
		s = replaceSymbol(s)
		if index, ok := s.NonTerminalIndex(); ok {
			return nil, &ExtractTokensError{Kind: ExtractTokensExternalTokenNonTerminal, Name: g.Pool.Resolve(g.Variables[index].Name)}
		}
		switch s.Kind() {
		case SymbolExternal:
			if name == 0 {
				return nil, &ExtractTokensError{Kind: ExtractTokensNonSymbolExternalToken}
			}
			externalTokens = append(externalTokens, ExternalToken{Name: name, Kind: kind})
		case SymbolTerminal:
			index, _ := s.TerminalIndex()
			externalTokens = append(externalTokens, ExternalToken{
				Name:                          extractor.lexical[index].Name,
				Kind:                          kind,
				CorrespondingInternalToken:    s,
				HasCorrespondingInternalToken: true,
			})
		default:
			panic("generate: an external token is not an external or a terminal symbol")
		}
	}

	var word Symbol
	hasWord := interned.hasWord
	if hasWord {
		word = replaceSymbol(interned.word)
		if tokenIndex, ok := word.NonTerminalIndex(); ok {
			wordRoot := g.Variables[tokenIndex].Root
			var conflicting string
			for i, v := range g.Variables {
				if i != int(tokenIndex) && g.Pool.SubtreeEqual(v.Root, wordRoot) {
					conflicting = g.Pool.Resolve(v.Name)
					break
				}
			}
			return nil, &NonTerminalWordTokenError{SymbolName: g.Pool.Resolve(g.Variables[tokenIndex].Name), ConflictingSymbolName: conflicting}
		}
	}

	reservedSets := make([]reservedSymbolSet, 0, len(g.ReservedSets))
	for _, set := range g.ReservedSets {
		symbols := make([]Symbol, 0, len(set.Roots))
		for _, root := range set.Roots {
			if s, ok := extractor.symbolAfterRewrites(g.Pool, root); ok {
				symbols = append(symbols, replaceSymbol(s))
			} else if i, ok := extractor.find(g.Pool, root); ok {
				symbols = append(symbols, TerminalSymbol(int(i)))
			} else {
				inner := g.Pool.Node(root)
				if inner.Kind == RuleMetadata {
					inner = g.Pool.Node(inner.Child)
				}
				tokenName := "unknown"
				if inner.Kind == RuleString || inner.Kind == RulePattern {
					tokenName = g.Pool.Resolve(inner.Str)
				}
				return nil, &ExtractTokensError{Kind: ExtractTokensNonTokenReservedWord, Name: tokenName}
			}
		}
		reservedSets = append(reservedSets, reservedSymbolSet{name: set.Name, symbols: symbols})
	}

	return &pendingTokenExtraction{
		grammar: g,
		meta: &extractedGrammarMeta{
			kinds:          kinds,
			extraSymbols:   extraSymbols,
			externalTokens: externalTokens,
			reservedSets:   reservedSets,
			supertypes:     supertypes,
			conflicts:      conflicts,
			inline:         inline,
			word:           word,
			hasWord:        hasWord,
		},
		lexicalVariables:           extractor.lexical,
		separatorRoots:             separatorRoots,
		rewrites:                   extractor.rewrites,
		syntaxVariableReplacements: replacements,
		syntaxVariableShift:        shift,
		stack:                      stack,
	}, nil
}

// renumberRoot renumbers the symbols of the nodes that root reaches. It skips
// each node that another root reached before in this pass.
//
// renumberRoot is renumber_root.
func renumberRoot(pool *RulePool, root RuleID, replace func(Symbol) Symbol, stack *[]RuleID, visited []bool) {
	*stack = append((*stack)[:0], root)
	for len(*stack) > 0 {
		id := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		if visited[id.Index()] {
			continue
		}
		visited[id.Index()] = true
		n := pool.Node(id)
		switch n.Kind {
		case RuleSym:
			pool.SetNode(id, Rule{Kind: RuleSym, Sym: replace(n.Sym)})
		case RuleSeq, RuleChoice:
			*stack = append(*stack, pool.ChildSlice(n.Children)...)
		case RuleRepeat, RuleMetadata, RuleReserved:
			*stack = append(*stack, n.Child)
		}
	}
}
