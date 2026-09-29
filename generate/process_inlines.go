package generate

import (
	"hash/fnv"
	"slices"
)

// This file ports crates/generate/src/prepare_grammar/process_inlines.rs:
// the pass that makes the productions of each inlined variable at each step
// where it appears.

// inlineBuilder makes the productions that replace the inlined variables.
//
// inlineBuilder is InlineBuilder.
type inlineBuilder struct {
	out          *ProductionStore
	firstInlined uint32
	inline       []Symbol
	m            map[[2]uint32][]uint32
	// memo holds the productions that inlining made, by a hash of their
	// content. The hash only narrows the search, and a comparison decides.
	memo map[uint64][]uint32
}

// scratchProd is a production that inlining builds, before it goes into the
// store.
//
// scratchProd is ScratchProd.
type scratchProd struct {
	steps                []ProductionStep
	dynamicPrecedence    int32
	requiresEOFLookahead bool
}

// inlineWork is a production and the step that the worklist reads next.
type inlineWork struct {
	prodID    uint32
	stepIndex uint32
}

// build returns the map of the inlined productions, and the ids of the
// productions whose every expansion was dropped.
//
// build is InlineBuilder::build.
func (b *inlineBuilder) build() (InlinedProductionMap, map[uint32]bool) {
	dead := map[uint32]bool{}
	var worklist []inlineWork
	for prodID := range b.firstInlined {
		survived := false
		worklist = append(worklist, inlineWork{prodID: prodID})
		for len(worklist) > 0 {
			i := 0
			for i < len(worklist) {
				w := worklist[i]
				step, ok := b.productionStepForID(w.prodID, w.stepIndex)
				if !ok {
					worklist = slices.Delete(worklist, i, i+1)
					survived = true
					continue
				}
				if !slices.Contains(b.inline, step.Symbol()) {
					worklist[i].stepIndex++
					i++
					continue
				}
				ids := b.inlineProductionAtStep(w.prodID, w.stepIndex)
				items := make([]inlineWork, len(ids))
				for k, id := range ids {
					items[k] = inlineWork{prodID: id, stepIndex: w.stepIndex}
				}
				worklist = slices.Replace(worklist, i, i+1, items...)
			}
		}
		if !survived {
			dead[prodID] = true
		}
	}
	return InlinedProductionMap{Map: b.m}, dead
}

// inlineProductionAtStep expands the inlined variable at one step of one
// production, drops the duplicates, stores the results and returns their
// ids.
//
// inlineProductionAtStep is InlineBuilder::inline_production_at_step.
func (b *inlineBuilder) inlineProductionAtStep(prodID, stepIndex uint32) []uint32 {
	if ids, ok := b.m[[2]uint32{prodID, stepIndex}]; ok {
		return slices.Clone(ids)
	}

	si := int(stepIndex)
	src := b.out.Productions[prodID]
	start, end := src.StepRange()
	scratch := []scratchProd{{
		steps:                slices.Clone(b.out.Steps[start:end]),
		dynamicPrecedence:    src.DynamicPrecedence,
		requiresEOFLookahead: src.RequiresEOFLookahead,
	}}
	for i := 0; i < len(scratch); {
		if si >= len(scratch[i].steps) || !slices.Contains(b.inline, scratch[i].steps[si].Symbol()) {
			i++
			continue
		}
		symbol := scratch[i].steps[si].Symbol()

		removedProd := scratch[i]
		scratch[i] = scratchProd{}
		removedStep := removedProd.steps[si]
		index, ok := symbol.NonTerminalIndex()
		if !ok {
			panic("generate: an inlined symbol is not a non-terminal")
		}
		vars := b.out.VarProds[index]
		var replacements []scratchProd
		for pIdx := vars[0]; pIdx < vars[1]; pIdx++ {
			p := b.out.Productions[pIdx]
			if p.RequiresEOFLookahead && si+1 < len(removedProd.steps) {
				continue
			}
			pStart, pEnd := p.StepRange()
			production := scratchProd{
				steps:                slices.Concat(removedProd.steps[:si], b.out.Steps[pStart:pEnd], removedProd.steps[si+1:]),
				dynamicPrecedence:    removedProd.dynamicPrecedence,
				requiresEOFLookahead: removedProd.requiresEOFLookahead,
			}
			inserted := production.steps[si : si+int(p.StepsLen)]
			if removedAlias, ok := removedStep.GetAlias(); ok {
				for k := range inserted {
					inserted[k].SetAlias(removedAlias, true)
				}
			}
			if removedField, ok := removedStep.GetField(); ok {
				for k := range inserted {
					inserted[k].SetField(removedField)
				}
			}
			if len(inserted) > 0 {
				last := &inserted[len(inserted)-1]
				if last.Precedence().Kind == PrecedenceNone {
					last.SetPrecedence(removedStep.Precedence())
				}
				if last.Associativity() == AssociativityNone {
					last.SetAssociativity(removedStep.Associativity())
				}
			}
			if abs32(p.DynamicPrecedence) > abs32(production.dynamicPrecedence) {
				production.dynamicPrecedence = p.DynamicPrecedence
			}
			production.requiresEOFLookahead = production.requiresEOFLookahead || p.RequiresEOFLookahead
			replacements = append(replacements, production)
		}
		scratch = slices.Replace(scratch, i, i+1, replacements...)
	}

	result := make([]uint32, 0, len(scratch))
	for _, sp := range scratch {
		hash := hashScratchProd(sp)
		existing := -1
		for _, id := range b.memo[hash] {
			p := b.out.Productions[id]
			pStart, pEnd := p.StepRange()
			if p.DynamicPrecedence == sp.dynamicPrecedence &&
				p.RequiresEOFLookahead == sp.requiresEOFLookahead &&
				slices.Equal(b.out.Steps[pStart:pEnd], sp.steps) {
				existing = int(id)
				break
			}
		}
		if existing >= 0 {
			result = append(result, uint32(existing))
			continue
		}
		stepsStart := uint32(len(b.out.Steps))
		b.out.Steps = append(b.out.Steps, sp.steps...)
		b.out.Productions = append(b.out.Productions, Production{
			StepsStart:           stepsStart,
			StepsLen:             uint32(len(sp.steps)),
			DynamicPrecedence:    sp.dynamicPrecedence,
			RequiresEOFLookahead: sp.requiresEOFLookahead,
		})
		id := uint32(len(b.out.Productions) - 1)
		b.memo[hash] = append(b.memo[hash], id)
		result = append(result, id)
	}

	b.m[[2]uint32{prodID, stepIndex}] = slices.Clone(result)
	return result
}

// hashScratchProd returns a hash of the content of a production, for the
// memo. Upstream hashes with FxHasher, and any hash works, because a
// comparison decides.
func hashScratchProd(sp scratchProd) uint64 {
	h := fnv.New64a()
	var buf [20]byte
	put32 := func(b []byte, x uint32) {
		b[0], b[1], b[2], b[3] = byte(x), byte(x>>8), byte(x>>16), byte(x>>24)
	}
	put32(buf[:], uint32(sp.dynamicPrecedence))
	if sp.requiresEOFLookahead {
		buf[4] = 1
	}
	_, _ = h.Write(buf[:5])
	for _, s := range sp.steps {
		put32(buf[0:], s.SymIndex)
		put32(buf[4:], uint32(s.PrecVal))
		put32(buf[8:], s.Alias)
		put32(buf[12:], s.Field)
		buf[16], buf[17] = byte(s.Reserved), byte(s.Reserved>>8)
		buf[18] = s.Flags
		_, _ = h.Write(buf[:19])
	}
	return h.Sum64()
}

// productionStepForID returns a step of a production, and false when the
// production has no such step.
//
// productionStepForID is InlineBuilder::production_step_for_id.
func (b *inlineBuilder) productionStepForID(prodID, step uint32) (ProductionStep, bool) {
	p := b.out.Productions[prodID]
	if step >= p.StepsLen {
		return ProductionStep{}, false
	}
	return b.out.Steps[p.StepsStart+step], true
}

// ProcessInlinesErrorKind is the kind of an error of the pass that processes
// the inlined variables.
type ProcessInlinesErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ProcessInlinesExternalToken ProcessInlinesErrorKind = iota
	ProcessInlinesToken
	ProcessInlinesFirstRule
	ProcessInlinesNoReachableProductions
)

// ProcessInlinesError is an error of the pass that processes the inlined
// variables. Its text is the text of upstream.
//
// ProcessInlinesError is ProcessInlinesError.
type ProcessInlinesError struct {
	Kind ProcessInlinesErrorKind
	Name string
}

// Error returns the text of the error.
func (e *ProcessInlinesError) Error() string {
	switch e.Kind {
	case ProcessInlinesExternalToken:
		return "External token `" + e.Name + "` cannot be inlined"
	case ProcessInlinesToken:
		return "Token `" + e.Name + "` cannot be inlined"
	case ProcessInlinesFirstRule:
		return "Rule `" + e.Name + "` cannot be inlined because it is the first rule"
	case ProcessInlinesNoReachableProductions:
		return "Rule `" + e.Name + "` has no reachable productions after inlining"
	}
	return ""
}

// processInlines checks the inlined symbols, and returns the productions
// that replace an inlined variable at each step where it appears.
//
// processInlines is process_inlines.
func processInlines(g *InputGrammar, meta *extractedGrammarMeta, lexicalVariables []LexicalVariable, out *ProductionStore) (InlinedProductionMap, error) {
	if len(meta.inline) == 0 {
		return InlinedProductionMap{}, nil
	}
	for _, symbol := range meta.inline {
		switch symbol.Kind() {
		case SymbolExternal:
			return InlinedProductionMap{}, &ProcessInlinesError{Kind: ProcessInlinesExternalToken, Name: g.Pool.Resolve(meta.externalTokens[symbol.index].Name)}
		case SymbolTerminal:
			return InlinedProductionMap{}, &ProcessInlinesError{Kind: ProcessInlinesToken, Name: g.Pool.Resolve(lexicalVariables[symbol.index].Name)}
		case SymbolNonTerminal:
			if symbol.index == 0 {
				return InlinedProductionMap{}, &ProcessInlinesError{Kind: ProcessInlinesFirstRule, Name: g.Pool.Resolve(g.Variables[0].Name)}
			}
		default:
			panic("generate: an end symbol in the inlined symbols")
		}
	}

	b := &inlineBuilder{
		out:          out,
		firstInlined: uint32(len(out.Productions)),
		inline:       meta.inline,
		m:            map[[2]uint32][]uint32{},
		memo:         map[uint64][]uint32{},
	}
	m, dead := b.build()

	if len(dead) > 0 {
		for i, prods := range out.VarProds {
			if prods[0] == prods[1] {
				continue
			}
			allDead := true
			for p := prods[0]; p < prods[1]; p++ {
				if !dead[p] {
					allDead = false
					break
				}
			}
			if !allDead {
				continue
			}
			symbol := NonTerminalSymbol(i)
			if i == 0 || slices.ContainsFunc(out.Steps, func(s ProductionStep) bool { return s.Symbol() == symbol }) {
				return InlinedProductionMap{}, &ProcessInlinesError{Kind: ProcessInlinesNoReachableProductions, Name: g.Pool.Resolve(g.Variables[i].Name)}
			}
		}
	}

	return m, nil
}
