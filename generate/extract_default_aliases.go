package generate

import "slices"

// This file ports
// crates/generate/src/prepare_grammar/extract_default_aliases.rs.

// symbolStatus is each alias of a symbol with its count, and whether the
// symbol appears without an alias.
//
// symbolStatus is SymbolStatus.
type symbolStatus struct {
	aliases          []aliasCount
	appearsUnaliased bool
}

// aliasCount is an alias and the number of steps that use it.
type aliasCount struct {
	alias Alias
	count int
}

// extractDefaultAliases finds the symbols that always have an alias, and
// makes one alias of each such symbol its default alias. A default alias
// applies everywhere, not in one production.
//
// That does two things. The parse table stores less alias data for each
// production. And in an ERROR node, where no alias of a production applies,
// the children have the symbols that they have in a valid tree.
//
// extractDefaultAliases is extract_default_aliases.
func extractDefaultAliases(g *InputGrammar, meta *extractedGrammarMeta, lexicalVariables []LexicalVariable, out *ProductionStore) AliasMap {
	terminalStatusList := make([]symbolStatus, len(lexicalVariables))
	nonTerminalStatusList := make([]symbolStatus, len(g.Variables))
	externalStatusList := make([]symbolStatus, len(meta.externalTokens))
	statusOf := func(symbol Symbol) *symbolStatus {
		switch symbol.Kind() {
		case SymbolExternal:
			return &externalStatusList[symbol.index]
		case SymbolNonTerminal:
			return &nonTerminalStatusList[symbol.index]
		case SymbolTerminal:
			return &terminalStatusList[symbol.index]
		}
		panic("Unexpected end token")
	}

	// For each symbol, find every alias that it appears with, and whether
	// it ever appears without an alias.
	for _, prod := range out.Productions {
		start, end := prod.StepRange()
		for _, step := range out.Steps[start:end] {
			symbol := step.Symbol()
			status := statusOf(symbol)

			// a default alias does not work for an inlined variable
			if slices.Contains(meta.inline, symbol) {
				continue
			}

			alias, ok := step.GetAlias()
			if !ok {
				status.appearsUnaliased = true
				continue
			}
			if i := slices.IndexFunc(status.aliases, func(a aliasCount) bool { return a.alias == alias }); i >= 0 {
				status.aliases[i].count++
			} else {
				status.aliases = append(status.aliases, aliasCount{alias: alias, count: 1})
			}
		}
	}

	for _, symbol := range meta.extraSymbols {
		statusOf(symbol).appearsUnaliased = true
	}

	// For each symbol that always appears with an alias, the alias that it
	// appears with most often is its default alias. On a tie, the alias
	// that came first wins.
	result := AliasMap{}
	decide := func(symbol Symbol, status *symbolStatus) {
		if status.appearsUnaliased {
			status.aliases = status.aliases[:0]
			return
		}
		if len(status.aliases) == 0 {
			return
		}
		best := status.aliases[0]
		for _, a := range status.aliases[1:] {
			if a.count > best.count {
				best = a
			}
		}
		status.aliases = append(status.aliases[:0], best)
		result[symbol] = best.alias
	}
	for i := range terminalStatusList {
		decide(TerminalSymbol(i), &terminalStatusList[i])
	}
	for i := range nonTerminalStatusList {
		decide(NonTerminalSymbol(i), &nonTerminalStatusList[i])
	}
	for i := range externalStatusList {
		decide(ExternalSymbol(i), &externalStatusList[i])
	}

	// Where a symbol has its default alias, drop the alias from the step,
	// because it is now redundant.
	var aliasPositionsToClear []int
	for _, prods := range out.VarProds {
		aliasPositionsToClear = aliasPositionsToClear[:0]

		productions := out.Productions[prods[0]:prods[1]]
		for i, prod := range productions {
			start, end := prod.StepRange()
			for j, step := range out.Steps[start:end] {
				status := statusOf(step.Symbol())

				// when the step has the default alias of its symbol, drop it
				alias, ok := step.GetAlias()
				if !ok || len(status.aliases) == 0 || alias != status.aliases[0].alias {
					continue
				}
				otherProductionsMustUseThisAliasAtThisIndex := false
				for otherI, otherProd := range productions {
					otherStart, otherEnd := otherProd.StepRange()
					otherSteps := out.Steps[otherStart:otherEnd]
					if otherI == i || len(otherSteps) <= j {
						continue
					}
					otherAlias, otherOK := otherSteps[j].GetAlias()
					if !otherOK || otherAlias != alias {
						continue
					}
					if def, ok := result[otherSteps[j].Symbol()]; !ok || def != alias {
						otherProductionsMustUseThisAliasAtThisIndex = true
						break
					}
				}
				if !otherProductionsMustUseThisAliasAtThisIndex {
					aliasPositionsToClear = append(aliasPositionsToClear, int(prod.StepsStart)+j)
				}
			}
		}

		for _, stepIndex := range aliasPositionsToClear {
			out.Steps[stepIndex].SetAlias(Alias{}, false)
		}
	}

	return result
}
