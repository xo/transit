package generate

import (
	"slices"
	"strings"
)

// This file ports crates/generate/src/prepare_grammar/intern_symbols.rs: the
// pass that turns each named symbol of the rules into a symbol with an index.

// InternSymbolsErrorKind is the kind of an error of the pass that interns the
// symbols.
type InternSymbolsErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	InternSymbolsHiddenStartRule InternSymbolsErrorKind = iota
	InternSymbolsUndefined
	InternSymbolsUndefinedSupertype
	InternSymbolsUndefinedConflict
	InternSymbolsUndefinedWordToken
)

// InternSymbolsError is an error of the pass that interns the symbols. Its
// text is the text of upstream.
//
// InternSymbolsError is InternSymbolsError.
type InternSymbolsError struct {
	Kind InternSymbolsErrorKind
	// Name is the symbol that is not defined, for every kind but
	// InternSymbolsHiddenStartRule.
	Name string
}

// Error returns the text of the error.
func (e *InternSymbolsError) Error() string {
	switch e.Kind {
	case InternSymbolsHiddenStartRule:
		return "A grammar's start rule must be visible."
	case InternSymbolsUndefined:
		return "Undefined symbol `" + e.Name + "`"
	case InternSymbolsUndefinedSupertype:
		return "Undefined symbol `" + e.Name + "` in grammar's supertypes array"
	case InternSymbolsUndefinedConflict:
		return "Undefined symbol `" + e.Name + "` in grammar's conflicts array"
	case InternSymbolsUndefinedWordToken:
		return "Undefined symbol `" + e.Name + "` as grammar's word token"
	}
	return ""
}

// internedGrammarMeta is what the pass gives besides the rules, which it
// rewrites in place.
//
// internedGrammarMeta is InternedGrammarMeta.
type internedGrammarMeta struct {
	kinds []VariableType
	// externalTokens holds the name and the kind of each external token
	externalTokens []internedExternalToken
	supertypes     []Symbol
	conflicts      [][]Symbol
	inline         []Symbol
	word           Symbol
	hasWord        bool
}

// internedExternalToken is the name and the kind of an external token. A zero
// name stands for the None of upstream, for a token that is not a named
// symbol.
type internedExternalToken struct {
	name StrID
	kind VariableType
}

// internSymbols rewrites each named symbol of the rules to the symbol of the
// variable or the external token with that name, and returns the kinds of the
// variables and the symbols of the other lists of the grammar.
//
// internSymbols is intern_symbols.
func internSymbols(g *InputGrammar, diagnostics *[]Diagnostic) (*internedGrammarMeta, error) {
	pool := g.Pool

	// The symbol of each name of a variable or an external token. An
	// external token with the name of a variable resolves to the variable,
	// because the variables go in first.
	nameOfSymbol := make(map[StrID]Symbol, len(g.Variables)+len(g.ExternalRoots))
	for i, v := range g.Variables {
		nameOfSymbol[v.Name] = NonTerminalSymbol(i)
	}
	for i, root := range g.ExternalRoots {
		if n := pool.Node(root); n.Kind == RuleNamedSymbol {
			if _, ok := nameOfSymbol[n.Str]; !ok {
				nameOfSymbol[n.Str] = ExternalSymbol(i)
			}
		}
	}
	if variableTypeForName(pool.Resolve(g.Variables[0].Name)) == VariableHidden {
		return nil, &InternSymbolsError{Kind: InternSymbolsHiddenStartRule}
	}

	// read the names and kinds of the external tokens before the rewrite
	// replaces them
	externalTokens := make([]internedExternalToken, len(g.ExternalRoots))
	for i, root := range g.ExternalRoots {
		if n := pool.Node(root); n.Kind == RuleNamedSymbol {
			externalTokens[i] = internedExternalToken{name: n.Str, kind: variableTypeForName(pool.Resolve(n.Str))}
		} else {
			externalTokens[i] = internedExternalToken{kind: VariableAnonymous}
		}
	}

	kinds := make([]VariableType, len(g.Variables))
	for i, v := range g.Variables {
		kinds[i] = variableTypeForName(pool.Resolve(v.Name))
	}

	var stack []RuleID
	for _, v := range g.Variables {
		if err := internRoot(pool, v.Root, v.Name, nameOfSymbol, diagnostics, &stack); err != nil {
			return nil, err
		}
	}
	for _, root := range slices.Concat(g.ExternalRoots, g.ExtraRoots) {
		if err := internRoot(pool, root, 0, nameOfSymbol, diagnostics, &stack); err != nil {
			return nil, err
		}
	}
	for _, set := range g.ReservedSets {
		for _, root := range set.Roots {
			if err := internRoot(pool, root, 0, nameOfSymbol, diagnostics, &stack); err != nil {
				return nil, err
			}
		}
	}

	supertypes := make([]Symbol, 0, len(g.SupertypeNames))
	for _, s := range g.SupertypeNames {
		sym, ok := nameOfSymbol[s]
		if !ok {
			return nil, &InternSymbolsError{Kind: InternSymbolsUndefinedSupertype, Name: pool.Resolve(s)}
		}
		supertypes = append(supertypes, sym)
	}
	conflicts := make([][]Symbol, 0, len(g.ConflictNames))
	for _, c := range g.ConflictNames {
		conflict := make([]Symbol, 0, len(c))
		for _, s := range c {
			sym, ok := nameOfSymbol[s]
			if !ok {
				return nil, &InternSymbolsError{Kind: InternSymbolsUndefinedConflict, Name: pool.Resolve(s)}
			}
			conflict = append(conflict, sym)
		}
		conflicts = append(conflicts, conflict)
	}

	// an inline name that is not defined is skipped, and no error says so
	var inline []Symbol
	for _, s := range g.InlineNames {
		if sym, ok := nameOfSymbol[s]; ok {
			inline = append(inline, sym)
		}
	}

	// An inlined rule makes no nodes, so a supertype entry for it has no
	// meaning, and the pass drops it.
	kept := supertypes[:0]
	for i, sym := range supertypes {
		if slices.Contains(inline, sym) {
			*diagnostics = append(*diagnostics, Diagnostic{Kind: DiagnosticSupertypeInlined, Name: pool.Resolve(g.SupertypeNames[i])})
			continue
		}
		kept = append(kept, sym)
	}
	supertypes = kept

	var word Symbol
	hasWord := false
	if g.WordName != 0 {
		sym, ok := nameOfSymbol[g.WordName]
		if !ok {
			return nil, &InternSymbolsError{Kind: InternSymbolsUndefinedWordToken, Name: pool.Resolve(g.WordName)}
		}
		word, hasWord = sym, true
	}

	for _, s := range supertypes {
		if index, ok := s.NonTerminalIndex(); ok {
			kinds[index] = VariableHidden
		}
	}

	return &internedGrammarMeta{
		kinds:          kinds,
		externalTokens: externalTokens,
		supertypes:     supertypes,
		conflicts:      conflicts,
		inline:         inline,
		word:           word,
		hasWord:        hasWord,
	}, nil
}

// internRoot walks a rule in pre-order, and rewrites each named symbol to its
// symbol in place. varName is the name of the variable of the rule, or zero
// for a rule that is not the root of a variable. The walk reuses stack.
//
// internRoot is intern_root.
func internRoot(pool *RulePool, root RuleID, varName StrID, nameOfSymbol map[StrID]Symbol, diagnostics *[]Diagnostic, stack *[]RuleID) error {
	*stack = append((*stack)[:0], root)
	for len(*stack) > 0 {
		id := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		n := pool.Node(id)
		switch n.Kind {
		case RuleNamedSymbol:
			sym, ok := nameOfSymbol[n.Str]
			if !ok {
				return &InternSymbolsError{Kind: InternSymbolsUndefined, Name: pool.Resolve(n.Str)}
			}
			pool.SetNode(id, Rule{Kind: RuleSym, Sym: sym})
		case RuleSeq, RuleChoice:
			children := pool.ChildSlice(n.Children)
			// A seq or a choice of one element in a hidden rule makes queries
			// behave in odd ways, so the pass warns about it.
			if len(children) == 1 {
				if k := pool.Node(children[0]).Kind; k == RuleString || k == RulePattern {
					var name string
					if varName != 0 {
						name = pool.Resolve(varName)
					}
					kind := DiagnosticUnarySeq
					if n.Kind == RuleChoice {
						kind = DiagnosticUnaryChoice
					}
					*diagnostics = append(*diagnostics, Diagnostic{Kind: kind, Name: name})
				}
			}
			base := len(*stack)
			*stack = append(*stack, children...)
			slices.Reverse((*stack)[base:])
		case RuleRepeat, RuleMetadata, RuleReserved:
			*stack = append(*stack, n.Child)
		}
	}
	return nil
}

// variableTypeForName returns the kind of a variable with a name: hidden when
// the name starts with an underscore, and named otherwise.
//
// variableTypeForName is variable_type_for_name.
func variableTypeForName(name string) VariableType {
	if strings.HasPrefix(name, "_") {
		return VariableHidden
	}
	return VariableNamed
}
