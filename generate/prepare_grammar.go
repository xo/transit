package generate

import (
	"cmp"
	"slices"
	"strings"
)

// This file ports crates/generate/src/prepare_grammar.rs. The function
// prepare_grammar, which runs the passes in order, comes when every pass that
// it calls is ported, and its tests come with it. Upstream wraps the error of
// each pass in PrepareGrammarError and ValidatePrecedenceError, which only
// pass the error through, so the Go functions return the error of the pass.

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

// validateIndirectRecursion returns an error when a rule derives itself
// through a chain of rules of one symbol, such as A -> B and B -> A. Such a
// cycle makes the parser loop.
//
// validateIndirectRecursion is validate_indirect_recursion.
func validateIndirectRecursion(g *InputGrammar) error {
	// Upstream keeps the transitions in an IndexMap of BTreeSets, so the
	// names keep the order of the variables, and the symbols of each name
	// are sorted by id.
	var names []StrID
	transitions := make(map[StrID][]StrID, len(g.Variables))
	var stack []RuleID
	for _, v := range g.Variables {
		var productions []StrID
		stack = append(stack[:0], v.Root)
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n := g.Pool.Node(id)
			switch n.Kind {
			case RuleNamedSymbol:
				// a rule that refers to itself directly makes no loop
				if n.Str != v.Name {
					if i, found := slices.BinarySearch(productions, n.Str); !found {
						productions = slices.Insert(productions, i, n.Str)
					}
				}
			case RuleChoice:
				stack = append(stack, g.Pool.ChildSlice(n.Children)...)
			case RuleMetadata:
				stack = append(stack, n.Child)
			}
		}
		if _, ok := transitions[v.Name]; !ok {
			names = append(names, v.Name)
		}
		transitions[v.Name] = productions
	}

	for _, start := range names {
		visited := map[StrID]bool{}
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
