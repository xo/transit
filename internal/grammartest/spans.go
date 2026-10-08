package grammartest

import (
	"context"
	"fmt"

	"github.com/xo/transit/internal/corpus"
)

// This file gives the highlights of the highlight test to a program, such as
// test/cmd/stylesvg, which draws them. It ports no upstream file.

// Span is a range of bytes of a text and the capture name that the
// highlighter gives it.
type Span struct {
	Start, End int
	Name       string
}

// Spans highlights src with the grammar own, as the highlight test does, and
// returns each range of src that has a highlight, in order. dir is the
// folder of the package of own. The nearest tree-sitter.json in dir or in a
// folder above it gives the queries and the injections, as for Highlight,
// and others are the other grammars of the module.
func Spans(ctx context.Context, dir string, own Grammar, others []Grammar, src []byte) ([]Span, error) {
	m, err := corpus.ReadModuleIn(dir, own.Language.Name())
	if err != nil {
		return nil, err
	}
	l, err := newLoaderFor(m, own, others)
	if err != nil {
		return nil, err
	}
	config, err := l.own.highlightConfig(&l.highlightNames)
	switch {
	case err != nil:
		return nil, err
	case config == nil:
		return nil, fmt.Errorf("finding the highlight query of %s: the grammar has none", own.Language.Name())
	}
	layers, err := config.inject.Layers(ctx, l.parser, src, l.injectionConfig)
	if err != nil {
		return nil, fmt.Errorf("highlighting the source: %w", err)
	}
	it := newHighlightIter(ctx, src, layers, l.configs)
	defer it.close()
	var spans []Span
	var stack []int
	for {
		event, ok := it.next()
		if !ok {
			break
		}
		switch event.kind {
		case eventHighlightStart:
			stack = append(stack, event.highlight)
		case eventHighlightEnd:
			stack = stack[:len(stack)-1]
		case eventSource:
			if n := len(stack); n > 0 && event.start < event.end {
				spans = append(spans, Span{Start: event.start, End: event.end, Name: l.highlightNames[stack[n-1]]})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("highlighting the source: %w", err)
	}
	return spans, nil
}
