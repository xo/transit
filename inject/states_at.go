package inject

import (
	"context"
	"fmt"

	"github.com/xo/transit"
)

// This file ports no upstream file. Upstream has no StatesAt, so the way to
// run it on a layer is a rule of transit (D28, D111).

// StatesAt returns the parse states of the layer at offset, as
// transit.Parser.StatesAt gives them. offset is a byte offset of the text of
// Layers, inside one of the ranges of the layer.
//
// StatesAt parses l.Text with the language of the layer, over the ranges of
// the layer, so the states are those of the parse that gave l.Tree. A layer
// whose ranges do not cover the whole text needs those ranges: without them,
// the parser reads the text of the other layers too. StatesAt sets the
// language and the included ranges on p, and it gives l.Tree to the parser
// as the old tree. It clears the included ranges of p before it returns, as
// Layers does, and the language of the layer stays set.
func (l Layer) StatesAt(ctx context.Context, p *transit.Parser, offset int) ([]transit.StateID, error) {
	defer func() {
		// Clearing the ranges gives the parser its default range, which it
		// always takes.
		_ = p.SetIncludedRanges(nil)
	}()
	if err := p.SetLanguage(l.Config.language); err != nil {
		return nil, fmt.Errorf("finding the states of the layer of %s: %w", l.Name, err)
	}
	if err := p.SetIncludedRanges(l.Ranges); err != nil {
		return nil, fmt.Errorf("finding the states of the layer of %s: %w", l.Name, err)
	}
	states, err := p.StatesAt(ctx, l.Text, offset, l.Tree)
	if err != nil {
		return nil, fmt.Errorf("finding the states of the layer of %s: %w", l.Name, err)
	}
	return states, nil
}
