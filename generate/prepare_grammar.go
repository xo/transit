package generate

import (
	"cmp"
	"slices"
	"strings"
)

// This file ports crates/generate/src/prepare_grammar.rs. Upstream wraps the
// error of each pass in PrepareGrammarError and ValidatePrecedenceError,
// which only pass the error through, so the Go functions return the error of
// the pass.

// IndirectRecursionError is the error of a grammar with a rule that derives
// itself through a chain of rules of one symbol, such as A -> B -> A.
//
// IndirectRecursionError is IndirectRecursionError.
type IndirectRecursionError struct {
	// Symbols is the cycle, with its first symbol again at the end.
	Symbols []string
}

// Error returns the text of the error.
func (e *IndirectRecursionError) Error() string {
	return "Grammar contains an indirectly recursive rule: " + strings.Join(e.Symbols, " -> ")
}

// EmptyStringExtraError is the error of an extra that can match the empty
// string.
//
// EmptyStringExtraError is EmptyStringExtraError.
type EmptyStringExtraError struct {
	// Name is the name of the extra.
	Name string
}

// Error returns the text of the error.
func (e *EmptyStringExtraError) Error() string {
	return "The extra rule `" + e.Name + "` matches the empty string.\n\n" +
		"Tree-sitter does not support extras that match the empty string.\n"
}

// UndeclaredPrecedenceError is the error of a named precedence that no list
// in precedences declares.
//
// UndeclaredPrecedenceError is UndeclaredPrecedenceError.
type UndeclaredPrecedenceError struct {
	Precedence string
	Rule       string
}

// Error returns the text of the error.
func (e *UndeclaredPrecedenceError) Error() string {
	return "Undeclared precedence '" + e.Precedence + "' in rule '" + e.Rule + "'"
}

// ConflictingPrecedenceOrderingError is the error of two precedences that one
// list puts in one order and another list in the other order.
//
// ConflictingPrecedenceOrderingError is ConflictingPrecedenceOrderingError.
type ConflictingPrecedenceOrderingError struct {
	Precedence1 string
	Precedence2 string
}

// Error returns the text of the error.
func (e *ConflictingPrecedenceOrderingError) Error() string {
	return "Conflicting orderings for precedences " + e.Precedence1 + " and " + e.Precedence2
}

// PreparedGrammar is the grammar split into the parts that the builder of
// the tables reads.
//
// PreparedGrammar is PreparedGrammar.
type PreparedGrammar struct {
	SyntaxGrammar  SyntaxGrammar
	LexicalGrammar LexicalGrammar
	Inlines        InlinedProductionMap
	DefaultAliases AliasMap
	StrPool        *StrPool
}

// LexicalToken is a token that is taken out of the input grammar and is not
// yet expanded into the NFA of the lexer. Root is still the rule of the token
// in the pool.
//
// LexicalToken is LexicalToken.
type LexicalToken struct {
	// Name is a made name for an anonymous token, or the name of the rule of
	// a variable that became a token.
	Name StrID
	Kind VariableType
	// Root is the rule in the pool that defines the token.
	Root RuleID
}

// PrepareGrammar splits a grammar into the parts that the builder of the
// tables reads. It runs each pass in order, and it changes g.
//
// PrepareGrammar is prepare_grammar.
func PrepareGrammar(g *InputGrammar, diagnostics *[]Diagnostic) (*PreparedGrammar, error) {
	if err := validatePrecedences(g); err != nil {
		return nil, err
	}

	internedMeta, err := internSymbols(g, diagnostics)
	if err != nil {
		return nil, err
	}
	pendingTokens, err := extractTokens(g, internedMeta)
	if err != nil {
		return nil, err
	}
	extMeta, lexicalGrammar, err := pendingTokens.expandAndCommit()
	if err != nil {
		return nil, err
	}
	if err := expandRepeats(g, extMeta); err != nil {
		return nil, err
	}

	var state flattenState
	var out ProductionStore
	if err := flattenGrammar(g, extMeta, &state, &out); err != nil {
		return nil, err
	}
	if err := validateIndirectRecursion(g, &out); err != nil {
		return nil, err
	}
	if err := validateExtras(g, extMeta.extraSymbols, lexicalGrammar, &out); err != nil {
		return nil, err
	}

	defaultAliases := extractDefaultAliases(g, extMeta, lexicalGrammar.Variables, &out)
	inlines, err := processInlines(g, extMeta, lexicalGrammar.Variables, &out)
	if err != nil {
		return nil, err
	}

	syntaxGrammar, strPool := assembleSyntaxGrammar(g, extMeta, &out)
	return &PreparedGrammar{
		SyntaxGrammar:  *syntaxGrammar,
		LexicalGrammar: *lexicalGrammar,
		Inlines:        inlines,
		DefaultAliases: defaultAliases,
		StrPool:        strPool,
	}, nil
}

// validateIndirectRecursion returns an error when a rule derives itself
// through a chain of rules of one symbol, such as A -> B and B -> A. Such a
// cycle makes the parser loop.
//
// validateIndirectRecursion is validate_indirect_recursion. It runs on the
// flattened productions of g.
func validateIndirectRecursion(g *InputGrammar, productions *ProductionStore) error {
	// Upstream keeps the transitions in an IndexMap of BTreeSets, so the
	// names keep the order of the variables, and the symbols of each name
	// are sorted by id. Upstream zips the variables with VarProds, so it
	// stops at the shorter of the two.
	var names []StrID
	transitions := make(map[StrID][]StrID, len(g.Variables))
	for i, v := range g.Variables {
		if i >= len(productions.VarProds) {
			break
		}
		start, end := productions.VarProds[i][0], productions.VarProds[i][1]
		var symbols []StrID
		for _, p := range productions.Productions[start:end] {
			// Only a production containing exactly one nonterminal adds an edge
			stepStart, stepEnd := p.StepRange()
			if stepEnd-stepStart != 1 {
				continue
			}
			index, ok := productions.Steps[stepStart].NonTerminalIndex()
			if !ok {
				continue
			}
			name := g.Variables[index].Name
			// Rules that *directly* reference themselves don't cause a parsing loop.
			if name == v.Name {
				continue
			}
			if j, found := slices.BinarySearch(symbols, name); !found {
				symbols = slices.Insert(symbols, j, name)
			}
		}
		if _, ok := transitions[v.Name]; !ok {
			names = append(names, v.Name)
		}
		transitions[v.Name] = symbols
	}

	visited := map[StrID]bool{}
	for _, start := range names {
		var path []StrID
		if first, last, ok := getCycle(start, transitions, visited, &path); ok {
			symbols := make([]string, 0, last-first+1)
			for _, s := range path[first : last+1] {
				symbols = append(symbols, g.Pool.Resolve(s))
			}
			return &IndirectRecursionError{Symbols: symbols}
		}
	}
	return nil
}

// getCycle searches depth first for a cycle of transitions from current. It
// returns the first and the last index of the cycle in path.
//
// getCycle is get_cycle.
func getCycle(current StrID, transitions map[StrID][]StrID, visited map[StrID]bool, path *[]StrID) (int, int, bool) {
	if first := slices.Index(*path, current); first >= 0 {
		*path = append(*path, current)
		return first, len(*path) - 1, true
	}
	if visited[current] {
		return 0, 0, false
	}
	*path = append(*path, current)
	visited[current] = true
	for _, next := range transitions[current] {
		if first, last, ok := getCycle(next, transitions, visited, path); ok {
			return first, last, true
		}
	}
	*path = (*path)[:len(*path)-1]
	return 0, 0, false
}

// validateExtras returns an error when an extra can match the empty string
// through internal tokens and rules.
//
// validateExtras is validate_extras.
//
// Reject extras that can match the empty string through internal tokens and rules.
// Parsing an extra doesn't change the parse state, so the parser could keep parsing
// an empty one at the same position.
func validateExtras(g *InputGrammar, extraSymbols []Symbol, lexicalGrammar *LexicalGrammar, productions *ProductionStore) error {
	empty := newEmptyMatches(lexicalGrammar, productions)
	for _, symbol := range extraSymbols {
		var matchesEmpty bool
		var name StrID
		switch symbol.Kind() {
		case SymbolTerminal:
			index, _ := symbol.TerminalIndex()
			matchesEmpty, name = empty.token(int(index)), lexicalGrammar.Variables[index].Name
		case SymbolNonTerminal:
			index, _ := symbol.NonTerminalIndex()
			matchesEmpty, name = empty.rule(int(index)), g.Variables[index].Name
		case SymbolExternal:
			// External scanners decide whether a token consumes input, so generation
			// cannot check them.
			continue
		default:
			// INVARIANT: Token extraction only resolves extras to terminals, non-terminals,
			// and external tokens.
			panic("internal error: entered unreachable code")
		}
		if matchesEmpty {
			return &EmptyStringExtraError{Name: g.Pool.Resolve(name)}
		}
	}
	return nil
}

// emptyMatches finds which tokens and rules can match the empty string, as
// the extras reach them.
//
// emptyMatches is EmptyMatches.
type emptyMatches struct {
	lexicalGrammar *LexicalGrammar
	productions    *ProductionStore
	// The NFA states reached so far while checking a token
	reached []uint32
	// Rules that have been checked, or are being checked
	checkedRules BitVec
	// The checked rules that can match the empty string
	emptyRules BitVec
	// Rules found not to match while another rule was still being checked
	provisional []uint32
	// How many rules are being checked
	depth uint32
}

// newEmptyMatches is EmptyMatches::new.
func newEmptyMatches(lexicalGrammar *LexicalGrammar, productions *ProductionStore) *emptyMatches {
	return &emptyMatches{
		lexicalGrammar: lexicalGrammar,
		productions:    productions,
	}
}

// symbol is EmptyMatches::symbol.
func (e *emptyMatches) symbol(symbol Symbol) bool {
	switch symbol.Kind() {
	case SymbolTerminal:
		index, _ := symbol.TerminalIndex()
		return e.token(int(index))
	case SymbolNonTerminal:
		index, _ := symbol.NonTerminalIndex()
		return e.rule(int(index))
	case SymbolExternal:
		// External token nullability is unknown. Only reject empty matches
		// established by the internal grammar.
		return false
	}
	// INVARIANT: Flattening never leaves an `End` step in a production, and
	// `EndOfNonTerminalExtra` only appears in parse table lookaheads.
	panic("internal error: entered unreachable code")
}

// token is EmptyMatches::token.
//
// Whether the token's NFA can reach an accept state without consuming a character.
func (e *emptyMatches) token(index int) bool {
	e.reached = e.reached[:0]
	e.reached = append(e.reached, e.lexicalGrammar.Variables[index].StartState)
	for i := 0; i < len(e.reached); i++ {
		switch state := &e.lexicalGrammar.Nfa.States[e.reached[i]]; state.Kind {
		case NfaAccept:
			return true
		case NfaSplit:
			for _, next := range [2]uint32{state.Left, state.Right} {
				if !slices.Contains(e.reached, next) {
					e.reached = append(e.reached, next)
				}
			}
		case NfaAdvance:
		}
	}
	return false
}

// rule is EmptyMatches::rule.
//
// Whether every step of one of the rule's productions can match the empty string.
//
// A rule that's still being checked counts as not matching, which cuts cycles. If it
// turns out to match, answers found while it was open are dropped and rechecked.
func (e *emptyMatches) rule(index int) bool {
	if e.checkedRules.Len() == 0 {
		rules := len(e.productions.VarProds)
		e.checkedRules.Resize(rules, false)
		e.emptyRules.Resize(rules, false)
	}
	if checked, _ := e.checkedRules.Get(index); !checked {
		e.checkedRules.Set(index, true)
		since := len(e.provisional)
		e.depth++
		start, end := e.productions.VarProds[index][0], e.productions.VarProds[index][1]
		empty := slices.ContainsFunc(e.productions.Productions[start:end], func(p Production) bool {
			stepStart, stepEnd := p.StepRange()
			for _, step := range e.productions.Steps[stepStart:stepEnd] {
				if !e.symbol(step.Symbol()) {
					return false
				}
			}
			return true
		})
		e.depth--
		if empty {
			e.emptyRules.Set(index, true)
			// These may have only failed because this rule was still open
			for _, rule := range e.provisional[since:] {
				e.checkedRules.Set(int(rule), false)
			}
			e.provisional = e.provisional[:since]
		} else if e.depth > 0 {
			e.provisional = append(e.provisional, uint32(index))
		}
	}
	empty, _ := e.emptyRules.Get(index)
	return empty
}

// validatePrecedences returns an error when a rule uses a named precedence
// that no list in precedences declares, or when two lists put two
// precedences in opposite orders.
//
// validatePrecedences is validate_precedences.
func validatePrecedences(g *InputGrammar) error {
	display := func(e PrecedenceEntry) string {
		if e.Kind == PrecedenceEntryName {
			return "'" + g.Pool.Resolve(e.Value) + "'"
		}
		return "$." + g.Pool.Resolve(e.Value)
	}
	compare := func(a, b PrecedenceEntry) int {
		if a.Kind == b.Kind {
			return strings.Compare(g.Pool.Resolve(a.Value), g.Pool.Resolve(b.Value))
		}
		// a name orders before a symbol
		return cmp.Compare(a.Kind, b.Kind)
	}

	// When a comes before b in one list, it cannot come after b in another.
	type pair struct{ a, b PrecedenceEntry }
	pairs := map[pair]int{}
	for _, list := range g.PrecedenceOrderings {
		for i := range list {
			// Upstream swaps entry1 in place, and entry1 lives for the whole
			// inner loop. So after a swap, the entries that follow are paired
			// with the swapped entry. The port keeps that.
			entry1 := list[i]
			for _, entry2 := range list[i+1:] {
				if entry1 == entry2 {
					continue
				}
				ordering := 1
				if compare(entry1, entry2) > 0 {
					ordering = -1
					entry1, entry2 = entry2, entry1
				}
				if o, ok := pairs[pair{entry1, entry2}]; !ok {
					pairs[pair{entry1, entry2}] = ordering
				} else if o != ordering {
					return &ConflictingPrecedenceOrderingError{Precedence1: display(entry1), Precedence2: display(entry2)}
				}
			}
		}
	}

	precedenceNames := map[StrID]bool{}
	for _, list := range g.PrecedenceOrderings {
		for _, p := range list {
			if p.Kind == PrecedenceEntryName {
				precedenceNames[p.Value] = true
			}
		}
	}

	var stack []RuleID
	for _, v := range g.Variables {
		stack = append(stack[:0], v.Root)
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n := g.Pool.Node(id)
			switch n.Kind {
			case RuleRepeat:
				stack = append(stack, n.Child)
			case RuleSeq, RuleChoice:
				stack = append(stack, g.Pool.ChildSlice(n.Children)...)
			case RuleMetadata:
				if prec := g.Pool.Params(n.Params).Precedence; prec.Kind == PrecedenceName && !precedenceNames[prec.Name] {
					return &UndeclaredPrecedenceError{Precedence: g.Pool.Resolve(prec.Name), Rule: g.Pool.Resolve(v.Name)}
				}
				stack = append(stack, n.Child)
			}
		}
	}
	return nil
}
