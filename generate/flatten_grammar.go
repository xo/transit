package generate

import (
	"math"
	"slices"
	"strconv"
)

// This file ports crates/generate/src/prepare_grammar/flatten_grammar.rs: the
// pass that turns each rule into flat productions, one for each path through
// its choices.

// FlattenGrammarErrorKind is the kind of an error of the pass that flattens
// the grammar.
type FlattenGrammarErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	FlattenGrammarNoReservedWordSet FlattenGrammarErrorKind = iota
	FlattenGrammarTooManyReservedWordSets
	FlattenGrammarEmptyString
	FlattenGrammarRecursiveInline
	FlattenGrammarNoReachableProductions
)

// FlattenGrammarError is an error of the pass that flattens the grammar. Its
// text is the text of upstream.
//
// FlattenGrammarError is FlattenGrammarError.
type FlattenGrammarError struct {
	Kind FlattenGrammarErrorKind
	// Name is the name of the rule or of the set of reserved words, for
	// every kind but FlattenGrammarTooManyReservedWordSets.
	Name string
	// Count is the number of sets of reserved words, for
	// FlattenGrammarTooManyReservedWordSets.
	Count int
}

// Error returns the text of the error.
func (e *FlattenGrammarError) Error() string {
	switch e.Kind {
	case FlattenGrammarNoReservedWordSet:
		return "No such reserved word set: " + e.Name
	case FlattenGrammarTooManyReservedWordSets:
		return "Reserved word set count " + strconv.Itoa(e.Count) + " exceeds the maximum of " + strconv.Itoa(math.MaxUint16)
	case FlattenGrammarEmptyString:
		return "The rule `" + e.Name + "` matches the empty string.\n\n" +
			"Tree-sitter does not support syntactic rules that match the empty string\n" +
			"unless they are used only as the grammar's start rule.\n"
	case FlattenGrammarRecursiveInline:
		return "Rule `" + e.Name + "` cannot be inlined because it contains a reference to itself"
	case FlattenGrammarNoReachableProductions:
		return "Rule `" + e.Name + "` has no reachable productions."
	}
	return ""
}

// flattenCtx is the metadata that a step takes from the rules around it. An
// alias with hasAlias false and a zero field stand for the None of upstream.
//
// flattenCtx is FlattenCtx.
type flattenCtx struct {
	prec     Precedence
	assoc    Associativity
	alias    Alias
	hasAlias bool
	field    StrID
	reserved uint16
}

// choiceCursor selects one branch at each choice of a walk from the root to
// the leaves. After a production is made, advance moves the path on, as a
// counter with a different base at each digit. The later decisions are
// dropped, because another branch at an earlier choice can lead to other
// choices.
//
// choiceCursor is ChoiceCursor.
type choiceCursor struct {
	decisions []uint32
	arities   []uint32
	depth     int
}

// reset starts a walk for a new variable.
//
// reset is ChoiceCursor::reset.
func (c *choiceCursor) reset() {
	c.decisions = c.decisions[:0]
	c.arities = c.arities[:0]
	c.depth = 0
}

// beginPath starts a new walk for the variable of the walk before.
//
// beginPath is ChoiceCursor::begin_path.
func (c *choiceCursor) beginPath() {
	c.arities = c.arities[:0]
	c.depth = 0
}

// selectBranch returns the branch to take at a choice of n branches, and
// records the choice.
//
// selectBranch is ChoiceCursor::select.
func (c *choiceCursor) selectBranch(n uint32) uint32 {
	var selected uint32
	if c.depth < len(c.decisions) {
		selected = c.decisions[c.depth]
	}
	c.arities = append(c.arities, n)
	c.depth++
	return selected
}

// advance finds the last choice that can take its next branch, takes it, and
// drops every decision after it. It reports whether it found one.
//
// advance is ChoiceCursor::advance.
func (c *choiceCursor) advance() bool {
	for depth := len(c.arities) - 1; depth >= 0; depth-- {
		var selected uint32
		if depth < len(c.decisions) {
			selected = c.decisions[depth]
		}
		if selected+1 < c.arities[depth] {
			for len(c.decisions) < depth+1 {
				c.decisions = append(c.decisions, 0)
			}
			c.decisions = c.decisions[:depth+1]
			c.decisions[depth] = selected + 1
			return true
		}
	}
	return false
}

// flattenState is the space that the pass reuses to walk and flatten one
// path at a time.
//
// flattenState is FlattenState.
type flattenState struct {
	steps   []ProductionStep
	choices choiceCursor
	dynPrec int32
	dead    bool
	// emitted holds the positions in ProductionStore.Productions of the
	// current variable's productions, by the hash of their ProdRef.
	emitted map[uint64][]uint32
}

// resetVariable starts the paths of a new variable.
//
// resetVariable is FlattenState::reset_variable.
func (s *flattenState) resetVariable() {
	s.choices.reset()
	clear(s.emitted)
	s.resetPath()
}

// resetPath starts a new path.
//
// resetPath is FlattenState::reset_path.
func (s *flattenState) resetPath() {
	s.steps = s.steps[:0]
	s.dynPrec = 0
	s.dead = false
	s.choices.beginPath()
}

// pushStep adds a step of a symbol with the metadata of ctx.
//
// pushStep is FlattenState::push_step.
func (s *flattenState) pushStep(symbol Symbol, ctx flattenCtx) {
	s.steps = append(s.steps, PackProductionStep(symbol, ctx.prec, ctx.assoc, ctx.alias, ctx.hasAlias, ctx.field, ctx.reserved))
}

// restoreOuterPrec gives the last step the precedence of the outer rule.
//
// restoreOuterPrec is FlattenState::restore_outer_prec.
func (s *flattenState) restoreOuterPrec(outer Precedence) {
	s.steps[len(s.steps)-1].SetPrecedence(outer)
}

// restoreOuterAssoc gives the last step the associativity of the outer rule.
//
// restoreOuterAssoc is FlattenState::restore_outer_assoc.
func (s *flattenState) restoreOuterAssoc(outer Associativity) {
	s.steps[len(s.steps)-1].SetAssociativity(outer)
}

// flattenApply flattens the path that the choice cursor selects in the rule
// at node, and reports whether it added a step. A choice only selects a
// child, and the caller walks the next paths from the root.
//
// flattenApply is apply in flatten_grammar.rs.
func flattenApply(pool *RulePool, reservedIDs map[StrID]uint16, node RuleID, ctx flattenCtx, atEnd bool, st *flattenState) (bool, error) {
	n := pool.Node(node)
	switch n.Kind {
	case RuleSym:
		st.pushStep(n.Sym, ctx)
		return true, nil
	case RuleSeq:
		children := pool.ChildSlice(n.Children)
		didPush := false
		for i, child := range children {
			pushed, err := flattenApply(pool, reservedIDs, child, ctx, atEnd && i+1 == len(children), st)
			if err != nil {
				return false, err
			}
			didPush = didPush || pushed
			if st.dead {
				break
			}
		}
		return didPush, nil
	case RuleChoice:
		// an empty choice matches nothing, so no production can hold it
		if n.Children.Len == 0 {
			st.dead = true
			return false, nil
		}
		selected := st.choices.selectBranch(n.Children.Len)
		return flattenApply(pool, reservedIDs, pool.ChildSlice(n.Children)[selected], ctx, atEnd, st)
	case RuleEOF:
		st.pushStep(SymbolEndValue, ctx)
		return true, nil
	case RuleMetadata:
		params := pool.Params(n.Params)
		inner := ctx
		if params.Precedence.Kind != PrecedenceNone {
			inner.prec = params.Precedence
		}
		if params.Associativity != AssociativityNone {
			inner.assoc = params.Associativity
		}
		if params.HasAlias {
			inner.alias, inner.hasAlias = params.Alias, true
		}
		if params.Field != 0 {
			inner.field = params.Field
		}
		if abs32(params.DynamicPrecedence) > abs32(st.dynPrec) {
			st.dynPrec = params.DynamicPrecedence
		}

		didPush, err := flattenApply(pool, reservedIDs, n.Child, inner, atEnd, st)
		if err != nil {
			return false, err
		}
		// The precedence and the associativity of a step hold at the
		// position just after it, so the last step of the region owns the
		// gap after it. When more steps follow, that gap is outside the
		// region, and it takes the outer values again. At the end of the
		// production the gap is the reduction, and it keeps the values of
		// the region.
		if didPush && !atEnd {
			if params.Precedence.Kind != PrecedenceNone {
				st.restoreOuterPrec(ctx.prec)
			}
			if params.Associativity != AssociativityNone {
				st.restoreOuterAssoc(ctx.assoc)
			}
		}
		return didPush, nil
	case RuleReserved:
		reserved, ok := reservedIDs[n.Str]
		if !ok {
			return false, &FlattenGrammarError{Kind: FlattenGrammarNoReservedWordSet, Name: pool.Resolve(n.Str)}
		}
		inner := ctx
		inner.reserved = reserved
		return flattenApply(pool, reservedIDs, n.Child, inner, atEnd, st)
	}
	return false, nil
}

// abs32 returns the absolute value of x. The absolute value of the lowest
// int32 is itself, as i32::abs gives in a release build of upstream.
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// flattenEmit adds the path as a production, unless the variable already has the
// same one. It drops a path with an eof() anywhere but at its last step,
// because such a production can never complete, and reports false.
//
// flattenEmit is emit in flatten_grammar.rs.
func flattenEmit(st *flattenState, out *ProductionStore) bool {
	eofIndex := slices.IndexFunc(st.steps, func(s ProductionStep) bool { return s.Symbol() == SymbolEndValue })
	if eofIndex < 0 {
		return flattenEmitReady(st, out, false)
	}
	if eofIndex != max(len(st.steps)-1, 0) {
		return false
	}
	st.steps = st.steps[:len(st.steps)-1]
	return flattenEmitReady(st, out, true)
}

// flattenEmitReady adds the path as a production, unless the variable already has
// the same one.
//
// flattenEmitReady is emit_ready in flatten_grammar.rs.
func flattenEmitReady(st *flattenState, out *ProductionStore, requiresEOFLookahead bool) bool {
	production := ProdRef{
		Steps:                st.steps,
		DynamicPrecedence:    st.dynPrec,
		RequiresEOFLookahead: requiresEOFLookahead,
	}
	hash := production.hash()
	for _, index := range st.emitted[hash] {
		if out.Production(index).equal(production) {
			// This variable already has an identical production.
			return true
		}
	}
	if st.emitted == nil {
		st.emitted = make(map[uint64][]uint32)
	}
	st.emitted[hash] = append(st.emitted[hash], uint32(len(out.Productions)))
	stepsStart := uint32(len(out.Steps))
	out.Steps = append(out.Steps, st.steps...)
	out.Productions = append(out.Productions, Production{
		StepsStart:           stepsStart,
		StepsLen:             uint32(len(st.steps)),
		DynamicPrecedence:    st.dynPrec,
		RequiresEOFLookahead: requiresEOFLookahead,
	})
	return true
}

// flattenGrammar adds the productions of each variable to out, one for each
// path through its choices, and checks them.
//
// flattenGrammar is flatten_grammar.
func flattenGrammar(g *InputGrammar, meta *extractedGrammarMeta, st *flattenState, out *ProductionStore) error {
	if len(meta.reservedSets) > math.MaxUint16 {
		return &FlattenGrammarError{Kind: FlattenGrammarTooManyReservedWordSets, Count: len(meta.reservedSets)}
	}
	// the last set of a name wins
	reservedIDs := make(map[StrID]uint16, len(meta.reservedSets))
	for i, set := range meta.reservedSets {
		reservedIDs[set.name] = uint16(i)
	}
	for _, v := range g.Variables {
		prodStart := uint32(len(out.Productions))
		droppedForEOF := false
		st.resetVariable()
		for {
			if _, err := flattenApply(g.Pool, reservedIDs, v.Root, flattenCtx{}, true, st); err != nil {
				return err
			}
			if !st.dead && !flattenEmit(st, out) {
				droppedForEOF = true
			}
			if !st.choices.advance() {
				break
			}
			st.resetPath()
		}
		if droppedForEOF && prodStart == uint32(len(out.Productions)) {
			return &FlattenGrammarError{Kind: FlattenGrammarNoReachableProductions, Name: g.Pool.Resolve(v.Name)}
		}
		out.VarProds = append(out.VarProds, [2]uint32{prodStart, uint32(len(out.Productions))})
	}
	return flattenCheck(g, meta, out)
}

// flattenCheck returns an error when a variable that a step uses has an empty
// production, or when an inlined variable refers to itself.
//
// flattenCheck is check in flatten_grammar.rs.
func flattenCheck(g *InputGrammar, meta *extractedGrammarMeta, out *ProductionStore) error {
	// Which variables appear in some production, by index. A step naming a non-terminal
	// that has no variable (as in some hand-built test grammars) uses none of them.
	used := make([]bool, len(out.VarProds))
	for _, step := range out.Steps {
		if index, ok := step.Symbol().NonTerminalIndex(); ok && int(index) < len(used) {
			used[index] = true
		}
	}
	for i, prods := range out.VarProds {
		symbol := NonTerminalSymbol(i)
		inlined := slices.Contains(meta.inline, symbol)
		for _, p := range out.Productions[prods[0]:prods[1]] {
			if used[i] && p.StepsLen == 0 && !p.RequiresEOFLookahead {
				return &FlattenGrammarError{Kind: FlattenGrammarEmptyString, Name: g.Pool.Resolve(g.Variables[i].Name)}
			}
			start, end := p.StepRange()
			if inlined && slices.ContainsFunc(out.Steps[start:end], func(s ProductionStep) bool { return s.Symbol() == symbol }) {
				return &FlattenGrammarError{Kind: FlattenGrammarRecursiveInline, Name: g.Pool.Resolve(g.Variables[i].Name)}
			}
		}
	}
	return nil
}

// assembleSyntaxGrammar builds the syntax grammar from the grammar, its
// metadata and its productions, and returns it with the string pool.
//
// assembleSyntaxGrammar is assemble_syntax_grammar.
func assembleSyntaxGrammar(g *InputGrammar, meta *extractedGrammarMeta, out *ProductionStore) (*SyntaxGrammar, *StrPool) {
	reservedWordSets := make([]TokenSet, 0, max(len(meta.reservedSets), 1))
	for _, set := range meta.reservedSets {
		reservedWordSets = append(reservedWordSets, TokenSetFrom(slices.Values(set.symbols)))
	}
	if len(reservedWordSets) == 0 {
		reservedWordSets = append(reservedWordSets, TokenSet{})
	}
	n := min(len(g.Variables), len(meta.kinds))
	variables := make([]SyntaxVariable, 0, n)
	for i, v := range g.Variables[:n] {
		variables = append(variables, SyntaxVariable{Name: v.Name, Kind: meta.kinds[i]})
	}
	return &SyntaxGrammar{
		Variables:           variables,
		ExtraSymbols:        meta.extraSymbols,
		ExpectedConflicts:   meta.conflicts,
		ExternalTokens:      meta.externalTokens,
		SupertypeSymbols:    meta.supertypes,
		VariablesToInline:   meta.inline,
		WordToken:           meta.word,
		HasWordToken:        meta.hasWord,
		PrecedenceOrderings: g.PrecedenceOrderings,
		ReservedWordSets:    reservedWordSets,

		Steps:       out.Steps,
		Productions: out.Productions,
		VarProds:    out.VarProds,
	}, g.Pool.Interner()
}
