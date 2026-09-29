package generate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/transit/generate/internal/regexsyntax/hir"
)

// This file ports crates/generate/src/prepare_grammar/expand_tokens.rs: the
// pass that builds the NFA of every token.
//
// Upstream wraps some errors in a variant that only passes them through,
// such as ExpandRuleError::Parse and ExpandRuleError::ExpandRegex. The Go
// functions return the inner error as it is.

// nfaBuilder builds the NFA of the tokens, from the last state to the first.
//
// nfaBuilder is NfaBuilder.
type nfaBuilder struct {
	nfa             Nfa
	isSep           bool
	precedenceStack []int32
}

// ExpandTokensErrorKind is the kind of an error of the pass that expands the
// tokens.
type ExpandTokensErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ExpandTokensEmptyString ExpandTokensErrorKind = iota
	ExpandTokensProcessing
)

// ExpandTokensError is an error of the pass that expands the tokens. Its text
// is the text of upstream.
//
// ExpandTokensError is ExpandTokensError, with ExpandTokensProcessingError in
// it. The variant ExpandRule passes its error through, so the pass returns
// that error as it is.
type ExpandTokensError struct {
	Kind ExpandTokensErrorKind
	// Rule is the name of the token.
	Rule string
	// Err is the error of the rule of the token, for ExpandTokensProcessing.
	Err error
}

// Error returns the text of the error.
func (e *ExpandTokensError) Error() string {
	if e.Kind == ExpandTokensProcessing {
		return "Error processing rule " + e.Rule + ": " + e.Err.Error() + "\n"
	}
	return "The rule `" + e.Rule + "` matches the empty string.\n" +
		"Tree-sitter does not support syntactic rules that match the empty string\n" +
		"unless they are used only as the grammar's start rule.\n"
}

// Unwrap returns the error of the rule of the token.
func (e *ExpandTokensError) Unwrap() error {
	return e.Err
}

// getImplicitPrecedence returns the precedence that a token has from its
// form: 2 for a string, and 1 more for each token.immediate around it.
//
// getImplicitPrecedence is get_implicit_precedence.
func getImplicitPrecedence(pool *RulePool, root RuleID) int32 {
	id := root
	var boost int32
	for {
		n := pool.Node(id)
		switch n.Kind {
		case RuleString:
			return 2 + boost
		case RuleMetadata:
			if pool.Params(n.Params).IsMainToken {
				boost++
			}
			id = n.Child
		default:
			return boost
		}
	}
}

// getCompletionPrecedence returns the number precedence of a token, or 0.
//
// getCompletionPrecedence is get_completion_precedence.
func getCompletionPrecedence(pool *RulePool, id RuleID) int32 {
	if n := pool.Node(id); n.Kind == RuleMetadata {
		if prec := pool.Params(n.Params).Precedence; prec.Kind == PrecedenceInteger {
			return prec.Integer
		}
	}
	return 0
}

// expandTokens builds the lexical grammar: the NFA of each token, with the
// separators before each token that is not immediate.
//
// expandTokens is expand_tokens.
func expandTokens(pool *RulePool, lexicalVariables []LexicalToken, separatorRoots []RuleID) (*LexicalGrammar, error) {
	builder := &nfaBuilder{isSep: true, precedenceStack: []int32{0}}
	separatorRoot := buildSeparator(pool, separatorRoots)

	variables := make([]LexicalVariable, 0, len(lexicalVariables))
	for i, variable := range lexicalVariables {
		if pool.SubtreeMatchesEmptyString(variable.Root) {
			return nil, &ExpandTokensError{Kind: ExpandTokensEmptyString, Rule: pool.Resolve(variable.Name)}
		}
		isImmediateToken := false
		if n := pool.Node(variable.Root); n.Kind == RuleMetadata {
			isImmediateToken = pool.Params(n.Params).IsMainToken
		}

		builder.isSep = false
		builder.nfa.States = append(builder.nfa.States, NfaState{
			Kind:          NfaAccept,
			VariableIndex: i,
			Precedence:    getCompletionPrecedence(pool, variable.Root),
		})
		if _, err := builder.expandRule(pool, variable.Root, builder.nfa.LastStateID()); err != nil {
			return nil, &ExpandTokensError{Kind: ExpandTokensProcessing, Rule: pool.Resolve(variable.Name), Err: err}
		}

		if !isImmediateToken {
			builder.isSep = true
			if _, err := builder.expandRule(pool, separatorRoot, builder.nfa.LastStateID()); err != nil {
				return nil, err
			}
		}

		variables = append(variables, LexicalVariable{
			Name:               variable.Name,
			Kind:               variable.Kind,
			ImplicitPrecedence: getImplicitPrecedence(pool, variable.Root),
			StartState:         builder.nfa.LastStateID(),
		})
	}

	return &LexicalGrammar{Nfa: builder.nfa, Variables: variables}, nil
}

// buildSeparator returns the rule of the separators: a repeat of a choice of
// blank and each separator, with the choices flattened and the duplicates
// dropped.
//
// buildSeparator is build_separator.
func buildSeparator(pool *RulePool, separatorRoots []RuleID) RuleID {
	blank := pool.PushNode(Rule{Kind: RuleBlank})
	if len(separatorRoots) == 0 {
		return blank
	}
	elements := make([]RuleID, 0, len(separatorRoots)+1)
	stack := make([]RuleID, 0, len(separatorRoots)+1)
	stack = append(stack, blank)
	for _, root := range slices.Backward(separatorRoots) {
		stack = append(stack, root)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n := pool.Node(id); n.Kind == RuleChoice {
			base := len(stack)
			stack = append(stack, pool.ChildSlice(n.Children)...)
			slices.Reverse(stack[base:])
		} else if !slices.ContainsFunc(elements, func(e RuleID) bool { return pool.SubtreeEqual(e, id) }) {
			elements = append(elements, id)
		}
	}
	choice := pool.PushNode(Rule{Kind: RuleChoice, Children: pool.PushChildren(elements)})
	return pool.PushNode(Rule{Kind: RuleRepeat, Child: choice})
}

// ExpandRuleErrorKind is the kind of an error of a rule of a token.
type ExpandRuleErrorKind uint8

// The kinds of error, in the order of upstream. The variants Parse and
// ExpandRegex pass their errors through, so a rule returns a *RegexError or
// an *ExpandRegexError as it is.
const (
	ExpandRuleUnexpectedSymbol ExpandRuleErrorKind = iota
	ExpandRuleUnexpectedReserved
	ExpandRuleUnexpectedEOF
)

// ExpandRuleError is an error of a rule of a token. Its text is the text of
// upstream.
//
// ExpandRuleError is ExpandRuleError.
type ExpandRuleError struct {
	Kind ExpandRuleErrorKind
	// Symbol is the symbol of ExpandRuleUnexpectedSymbol.
	Symbol Symbol
	// Context is the name of the set of reserved words of
	// ExpandRuleUnexpectedReserved.
	Context string
}

// Error returns the text of the error.
func (e *ExpandRuleError) Error() string {
	switch e.Kind {
	case ExpandRuleUnexpectedSymbol:
		return "unexpected symbol " + symbolDebug(e.Symbol)
	case ExpandRuleUnexpectedReserved:
		return "unexpected reserved-word context " + e.Context
	case ExpandRuleUnexpectedEOF:
		return "`eof()` cannot be used inside a token. A lexical rule cannot check for end of input, " +
			"so use `eof()` only at the end of a syntactic rule."
	}
	return ""
}

// symbolDebug returns the Debug text of a Symbol in Rust, such as
// Symbol { kind: Terminal, index: 3 }.
func symbolDebug(s Symbol) string {
	kinds := [...]string{
		SymbolExternal:              "External",
		SymbolEnd:                   "End",
		SymbolEndOfNonTerminalExtra: "EndOfNonTerminalExtra",
		SymbolTerminal:              "Terminal",
		SymbolNonTerminal:           "NonTerminal",
	}
	return "Symbol { kind: " + kinds[s.kind] + ", index: " + strconv.FormatUint(uint64(s.index), 10) + " }"
}

// ExpandRegexErrorKind is the kind of an error of the HIR of a pattern.
type ExpandRegexErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ExpandRegexUTF8 ExpandRegexErrorKind = iota
	ExpandRegexAssertion
	ExpandRegexNonASCIIByteClass
)

// ExpandRegexError is an error of the HIR of a pattern. Its text is the text
// of upstream.
//
// ExpandRegexError is ExpandRegexError, with NonAsciiByteClassError in it.
type ExpandRegexError struct {
	Kind ExpandRegexErrorKind
	// Text is the text of the UTF-8 error, for ExpandRegexUTF8.
	Text string
	// Start and End are the bytes of the class, for
	// ExpandRegexNonASCIIByteClass.
	Start, End byte
}

// Error returns the text of the error.
func (e *ExpandRegexError) Error() string {
	switch e.Kind {
	case ExpandRegexUTF8:
		return e.Text
	case ExpandRegexAssertion:
		return "Regex error: Assertions are not supported"
	case ExpandRegexNonASCIIByteClass:
		class := fmt.Sprintf(`\x%02x`, e.Start)
		if e.Start != e.End {
			class = fmt.Sprintf(`\x%02x-\x%02x`, e.Start, e.End)
		}
		return "The non-ASCII byte class " + class + " via (?-u:...) is not supported. Remove the `-u` flag."
	}
	return ""
}

// perlClassReplacements are the replacements of \w, \s and \d, and of their
// negations, with the ASCII sets that a grammar means by them, in the order
// of upstream.
var perlClassReplacements = []struct{ old, new string }{
	{`\w`, `[0-9A-Za-z_]`},
	{`\s`, `[\t-\r ]`},
	{`\d`, `[0-9]`},
	{`\W`, `[^0-9A-Za-z_]`},
	{`\S`, `[^\t-\r ]`},
	{`\D`, `[^0-9]`},
}

// expandRule adds the states of a rule of a token to the NFA, before the
// state nextStateID, and reports whether it added a state.
//
// expandRule is NfaBuilder::expand_rule.
func (b *nfaBuilder) expandRule(pool *RulePool, id RuleID, nextStateID uint32) (bool, error) {
	n := pool.Node(id)
	switch n.Kind {
	case RulePattern:
		// With Unicode on, \w, \s and \d expand to sets of characters that
		// are much larger than a grammar means, so they become the sets that
		// they stand for. A grammar that needs the whole Unicode range uses
		// \p{L}, \p{Z} and \p{N}. Upstream replaces them one after the
		// other, and so does the port.
		s := pool.Resolve(n.Str)
		for _, r := range perlClassReplacements {
			s = strings.ReplaceAll(s, r.old, r.new)
		}
		// Parse with no case folding, and fold in the port (see
		// foldASCIISafe). The fold of regex-syntax would take the long s and
		// the Kelvin sign into s and k.
		h, err := parsePattern(s, strings.Contains(pool.Resolve(n.Flags), "i"))
		if err != nil {
			return false, err
		}
		return b.expandRegex(h, nextStateID)
	case RuleString:
		s := []rune(pool.Resolve(n.Str))
		for _, c := range slices.Backward(s) {
			b.pushAdvance(CharacterSetFromChar(c), nextStateID)
			nextStateID = b.nfa.LastStateID()
		}
		return len(s) > 0, nil
	case RuleChoice:
		elements := pool.ChildSlice(n.Children)
		alternativeStateIDs := make([]uint32, 0, len(elements))
		didExpand := false
		for _, element := range elements {
			expanded, err := b.expandRule(pool, element, nextStateID)
			if err != nil {
				return false, err
			}
			if expanded {
				didExpand = true
				alternativeStateIDs = append(alternativeStateIDs, b.nfa.LastStateID())
			} else {
				alternativeStateIDs = append(alternativeStateIDs, nextStateID)
			}
		}
		slices.Sort(alternativeStateIDs)
		alternativeStateIDs = slices.Compact(alternativeStateIDs)
		last := b.nfa.LastStateID()
		alternativeStateIDs = slices.DeleteFunc(alternativeStateIDs, func(i uint32) bool { return i == last })
		for _, alternativeStateID := range alternativeStateIDs {
			b.pushSplit(alternativeStateID)
		}
		return didExpand, nil
	case RuleSeq:
		result := false
		for _, element := range slices.Backward(pool.ChildSlice(n.Children)) {
			expanded, err := b.expandRule(pool, element, nextStateID)
			if err != nil {
				return false, err
			}
			if expanded {
				result = true
			}
			nextStateID = b.nfa.LastStateID()
		}
		return result, nil
	case RuleRepeat:
		// a placeholder for the split
		b.nfa.States = append(b.nfa.States, NfaState{Kind: NfaAccept})
		splitStateID := b.nfa.LastStateID()
		expanded, err := b.expandRule(pool, n.Child, splitStateID)
		if err != nil {
			return false, err
		}
		if expanded {
			b.nfa.States[splitStateID] = NfaState{Kind: NfaSplit, Left: b.nfa.LastStateID(), Right: nextStateID}
			return true, nil
		}
		b.nfa.States = b.nfa.States[:len(b.nfa.States)-1]
		return false, nil
	case RuleMetadata:
		hasPrecedence := false
		if prec := pool.Params(n.Params).Precedence; prec.Kind == PrecedenceInteger {
			b.precedenceStack = append(b.precedenceStack, prec.Integer)
			hasPrecedence = true
		}
		result, err := b.expandRule(pool, n.Child, nextStateID)
		if hasPrecedence {
			b.precedenceStack = b.precedenceStack[:len(b.precedenceStack)-1]
		}
		return result, err
	case RuleBlank:
		return false, nil
	case RuleEOF:
		return false, &ExpandRuleError{Kind: ExpandRuleUnexpectedEOF}
	case RuleSym:
		return false, &ExpandRuleError{Kind: ExpandRuleUnexpectedSymbol, Symbol: n.Sym}
	case RuleReserved:
		return false, &ExpandRuleError{Kind: ExpandRuleUnexpectedReserved, Context: pool.Resolve(n.Str)}
	}
	// intern_symbols turns every named symbol into a symbol
	panic("internal error: entered unreachable code")
}

// expandRegex adds the states of an HIR to the NFA, before the state
// nextStateID, and reports whether it added a state.
//
// expandRegex is NfaBuilder::expand_regex.
func (b *nfaBuilder) expandRegex(h *hir.Hir, nextStateID uint32) (bool, error) {
	switch k := h.Kind().(type) {
	case *hir.Empty:
		return false, nil
	case *hir.Literal:
		s, err := rustUTF8String(*k)
		if err != nil {
			return false, err
		}
		for _, c := range slices.Backward([]rune(s)) {
			b.pushAdvance(CharacterSetFromChar(c), nextStateID)
			nextStateID = b.nfa.LastStateID()
		}
		return true, nil
	case *hir.ClassUnicode:
		var chars CharacterSet
		for _, c := range k.Ranges() {
			chars = chars.AddRange(c.Start(), c.End())
		}
		b.pushAdvance(chars, nextStateID)
		return true, nil
	case *hir.ClassBytes:
		// A class of bytes comes only from a (?-u:...) group, which the
		// regex syntax of JavaScript cannot write. A byte below 0x80 is its
		// own code point, so it converts exactly. A byte above that is one
		// part of a UTF-8 sequence, and no character stands for it.
		var chars CharacterSet
		for _, c := range k.Ranges() {
			if c.End() >= 0x80 {
				return false, &ExpandRegexError{Kind: ExpandRegexNonASCIIByteClass, Start: c.Start(), End: c.End()}
			}
			chars = chars.AddRange(rune(c.Start()), rune(c.End()))
		}
		b.pushAdvance(chars, nextStateID)
		return true, nil
	case *hir.Look:
		return false, &ExpandRegexError{Kind: ExpandRegexAssertion}
	case *hir.Repetition:
		switch {
		case k.Min == 0 && k.Max != nil && *k.Max == 1:
			return b.expandZeroOrOne(k.Sub, nextStateID)
		case k.Min == 1 && k.Max == nil:
			return b.expandOneOrMore(k.Sub, nextStateID)
		case k.Min == 0 && k.Max == nil:
			return b.expandZeroOrMore(k.Sub, nextStateID)
		case k.Max != nil && k.Min == *k.Max:
			return b.expandCount(k.Sub, k.Min, nextStateID)
		case k.Max == nil:
			expanded, err := b.expandZeroOrMore(k.Sub, nextStateID)
			if err != nil || !expanded {
				return false, err
			}
			return b.expandCount(k.Sub, k.Min, nextStateID)
		}
		result, err := b.expandCount(k.Sub, k.Min, nextStateID)
		if err != nil {
			return false, err
		}
		for range *k.Max - k.Min {
			if result {
				nextStateID = b.nfa.LastStateID()
			}
			expanded, err := b.expandZeroOrOne(k.Sub, nextStateID)
			if err != nil {
				return false, err
			}
			if expanded {
				result = true
			}
		}
		return result, nil
	case *hir.Capture:
		return b.expandRegex(k.Sub, nextStateID)
	case *hir.Concat:
		result := false
		for _, sub := range slices.Backward(*k) {
			expanded, err := b.expandRegex(sub, nextStateID)
			if err != nil {
				return false, err
			}
			if expanded {
				result = true
				nextStateID = b.nfa.LastStateID()
			}
		}
		return result, nil
	case *hir.Alternation:
		alternativeStateIDs := make([]uint32, 0, len(*k))
		for _, sub := range *k {
			expanded, err := b.expandRegex(sub, nextStateID)
			if err != nil {
				return false, err
			}
			if expanded {
				alternativeStateIDs = append(alternativeStateIDs, b.nfa.LastStateID())
			} else {
				alternativeStateIDs = append(alternativeStateIDs, nextStateID)
			}
		}
		slices.Sort(alternativeStateIDs)
		alternativeStateIDs = slices.Compact(alternativeStateIDs)
		last := b.nfa.LastStateID()
		alternativeStateIDs = slices.DeleteFunc(alternativeStateIDs, func(i uint32) bool { return i == last })
		for _, alternativeStateID := range alternativeStateIDs {
			b.pushSplit(alternativeStateID)
		}
		return true, nil
	}
	panic("generate: an HIR kind that expand_regex does not know")
}

// rustUTF8String returns the bytes as a string, or the error that
// str::from_utf8 of Rust gives, with the text of that error.
func rustUTF8String(b []byte) (string, error) {
	i := 0
	for i < len(b) {
		start := i
		first := b[i]
		if first < 0x80 {
			i++
			continue
		}
		// next returns the next byte of the character, or false at the end
		// of the bytes
		next := func() (byte, bool) {
			i++
			if i >= len(b) {
				return 0, false
			}
			return b[i], true
		}
		isCont := func(c byte) bool { return c&0xC0 == 0x80 }
		fail := func(errorLen int) (string, error) {
			text := "incomplete utf-8 byte sequence from index " + strconv.Itoa(start)
			if errorLen > 0 {
				text = "invalid utf-8 sequence of " + strconv.Itoa(errorLen) + " bytes from index " + strconv.Itoa(start)
			}
			return "", &ExpandRegexError{Kind: ExpandRegexUTF8, Text: text}
		}
		var lo, hi byte = 0x80, 0xBF
		var width int
		switch {
		case first >= 0xC2 && first <= 0xDF:
			width = 2
		case first >= 0xE0 && first <= 0xEF:
			width = 3
			switch first {
			case 0xE0:
				lo = 0xA0
			case 0xED:
				hi = 0x9F
			}
		case first >= 0xF0 && first <= 0xF4:
			width = 4
			switch first {
			case 0xF0:
				lo = 0x90
			case 0xF4:
				hi = 0x8F
			}
		default:
			return fail(1)
		}
		c, ok := next()
		if !ok {
			return fail(0)
		}
		if c < lo || c > hi {
			return fail(1)
		}
		for n := 2; n < width; n++ {
			c, ok := next()
			if !ok {
				return fail(0)
			}
			if !isCont(c) {
				return fail(n)
			}
		}
		i++
	}
	return string(b), nil
}

// expandOneOrMore adds the states of a repetition of one or more.
//
// expandOneOrMore is NfaBuilder::expand_one_or_more.
func (b *nfaBuilder) expandOneOrMore(h *hir.Hir, nextStateID uint32) (bool, error) {
	// a placeholder for the split
	b.nfa.States = append(b.nfa.States, NfaState{Kind: NfaAccept})
	splitStateID := b.nfa.LastStateID()
	expanded, err := b.expandRegex(h, splitStateID)
	if err != nil {
		return false, err
	}
	if expanded {
		b.nfa.States[splitStateID] = NfaState{Kind: NfaSplit, Left: b.nfa.LastStateID(), Right: nextStateID}
		return true, nil
	}
	b.nfa.States = b.nfa.States[:len(b.nfa.States)-1]
	return false, nil
}

// expandZeroOrOne adds the states of an optional expression.
//
// expandZeroOrOne is NfaBuilder::expand_zero_or_one.
func (b *nfaBuilder) expandZeroOrOne(h *hir.Hir, nextStateID uint32) (bool, error) {
	expanded, err := b.expandRegex(h, nextStateID)
	if err != nil || !expanded {
		return false, err
	}
	b.pushSplit(nextStateID)
	return true, nil
}

// expandZeroOrMore adds the states of a repetition of zero or more.
//
// expandZeroOrMore is NfaBuilder::expand_zero_or_more.
func (b *nfaBuilder) expandZeroOrMore(h *hir.Hir, nextStateID uint32) (bool, error) {
	expanded, err := b.expandOneOrMore(h, nextStateID)
	if err != nil || !expanded {
		return false, err
	}
	b.pushSplit(nextStateID)
	return true, nil
}

// expandCount adds the states of an expression count times.
//
// expandCount is NfaBuilder::expand_count.
func (b *nfaBuilder) expandCount(h *hir.Hir, count, nextStateID uint32) (bool, error) {
	result := false
	for range count {
		expanded, err := b.expandRegex(h, nextStateID)
		if err != nil {
			return false, err
		}
		if expanded {
			result = true
			nextStateID = b.nfa.LastStateID()
		}
	}
	return result, nil
}

// pushAdvance adds a state that moves on chars to the state stateID.
//
// pushAdvance is NfaBuilder::push_advance.
func (b *nfaBuilder) pushAdvance(chars CharacterSet, stateID uint32) {
	b.nfa.States = append(b.nfa.States, NfaState{
		Kind:       NfaAdvance,
		Chars:      chars,
		StateID:    stateID,
		Precedence: b.precedenceStack[len(b.precedenceStack)-1],
		IsSep:      b.isSep,
	})
}

// pushSplit adds a state that splits to the state stateID and to the last
// state.
//
// pushSplit is NfaBuilder::push_split.
func (b *nfaBuilder) pushSplit(stateID uint32) {
	last := b.nfa.LastStateID()
	b.nfa.States = append(b.nfa.States, NfaState{Kind: NfaSplit, Left: stateID, Right: last})
}
