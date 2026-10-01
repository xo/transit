package grammartest

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/xo/transit"
	"github.com/xo/transit/inject"
)

// This file ports the part of crates/highlight/src/highlight.rs that gives
// the highlight events of a text, for the highlight test (D80):
// HighlightConfiguration::new, configure, sort_key, and the Iterator of
// HighlightIter with emit_event, sort_layers and insert_layer. It leaves out
// the HtmlRenderer, the cancellation flag and the UTF-16 encodings.
//
// The package inject ports the part that parses the layers (D72). So the
// port takes every layer from inject.Layers before the first event, and it
// leaves out HighlightIterLayer::new and the part of the Iterator that
// parses a layer at an injection. A layer that upstream adds at an
// injection capture starts at or after that capture, so it gives the same
// events.

// highlightNone is the value of a highlight that is None upstream.
const highlightNone = -1

// highlightConfiguration holds the data needed to highlight code written in
// a particular language.
//
// highlightConfiguration is HighlightConfiguration. Its combined injections
// query is in inject, which parses the layers.
type highlightConfiguration struct {
	language     *transit.Language
	languageName string
	query        *transit.Query
	// inject is the configuration of the injections of the language.
	inject                    *inject.Config
	localsPatternIndex        int
	highlightsPatternIndex    int
	highlightIndices          []int
	nonLocalVariablePatterns  []bool
	localScopeCaptureIndex    int
	localDefCaptureIndex      int
	localDefValueCaptureIndex int
	localRefCaptureIndex      int
}

// newHighlightConfiguration creates a highlightConfiguration for a given
// language and set of highlighting queries.
//
// newHighlightConfiguration is HighlightConfiguration::new, with the
// configuration of inject in place of the combined injections query.
func newHighlightConfiguration(language *transit.Language, name, highlightsQuery, injectionQuery, localsQuery string) (*highlightConfiguration, error) {
	// Concatenate the query strings, keeping track of the start offset of each section.
	querySource := injectionQuery + localsQuery + highlightsQuery
	localsQueryOffset := len(injectionQuery)
	highlightsQueryOffset := len(injectionQuery) + len(localsQuery)

	// Construct a single query by concatenating the three query strings, but record the
	// range of pattern indices that belong to each individual string.
	query, err := transit.NewQuery(language, querySource)
	if err != nil {
		return nil, fmt.Errorf("compiling the queries of %s: %w", name, err)
	}
	localsPatternIndex, highlightsPatternIndex := 0, 0
	for i := range query.PatternCount() {
		patternOffset := query.StartByteForPattern(i)
		if patternOffset < highlightsQueryOffset {
			highlightsPatternIndex++
			if patternOffset < localsQueryOffset {
				localsPatternIndex++
			}
		}
	}

	// The package inject handles the 'combined injections'. Disable the
	// combined injection patterns in the main query.
	injectConfig, err := inject.NewConfig(language, name, injectionQuery)
	if err != nil {
		return nil, fmt.Errorf("compiling the injections of %s: %w", name, err)
	}
	for patternIndex := range localsPatternIndex {
		if slices.ContainsFunc(query.PropertySettings(patternIndex), func(s transit.QueryProperty) bool {
			return s.Key == "injection.combined"
		}) {
			query.DisablePattern(patternIndex)
		}
	}

	// Find all of the highlighting patterns that are disabled for nodes that
	// have been identified as local variables.
	nonLocalVariablePatterns := make([]bool, query.PatternCount())
	for i := range nonLocalVariablePatterns {
		nonLocalVariablePatterns[i] = slices.ContainsFunc(query.PropertyPredicates(i), func(p transit.QueryPropertyPredicate) bool {
			return !p.Positive && p.Property.Key == "local"
		})
	}

	// Store the numeric ids for all of the special captures.
	c := &highlightConfiguration{
		language:                  language,
		languageName:              name,
		query:                     query,
		inject:                    injectConfig,
		localsPatternIndex:        localsPatternIndex,
		highlightsPatternIndex:    highlightsPatternIndex,
		nonLocalVariablePatterns:  nonLocalVariablePatterns,
		localScopeCaptureIndex:    -1,
		localDefCaptureIndex:      -1,
		localDefValueCaptureIndex: -1,
		localRefCaptureIndex:      -1,
	}
	for i, name := range query.CaptureNames() {
		switch name {
		case "local.definition":
			c.localDefCaptureIndex = i
		case "local.definition-value":
			c.localDefValueCaptureIndex = i
		case "local.reference":
			c.localRefCaptureIndex = i
		case "local.scope":
			c.localScopeCaptureIndex = i
		}
	}
	c.highlightIndices = slices.Repeat([]int{highlightNone}, len(query.CaptureNames()))
	return c, nil
}

// configure sets the list of recognized highlight names. A capture name
// gets the recognized name with the most parts, all of which are parts of
// the capture name, such as function.builtin for
// function.builtin.constructor.
//
// configure is HighlightConfiguration::configure.
func (c *highlightConfiguration) configure(recognizedNames []string) {
	c.highlightIndices = c.highlightIndices[:0]
	for _, captureName := range c.query.CaptureNames() {
		captureParts := strings.Split(captureName, ".")

		bestIndex := highlightNone
		bestMatchLen := 0
		for i, recognizedName := range recognizedNames {
			length := 0
			matches := true
			for part := range strings.SplitSeq(recognizedName, ".") {
				length++
				if !slices.Contains(captureParts, part) {
					matches = false
					break
				}
			}
			if matches && length > bestMatchLen {
				bestIndex = i
				bestMatchLen = length
			}
		}
		c.highlightIndices = append(c.highlightIndices, bestIndex)
	}
}

// highlightEventKind is the kind of a highlightEvent.
type highlightEventKind uint8

// The kinds of highlightEvent.
const (
	eventSource highlightEventKind = iota
	eventHighlightStart
	eventHighlightEnd
)

// highlightEvent is a single step in rendering a syntax-highlighted
// document: the source from start to end, the start of a highlight, or the
// end of the last highlight that started.
//
// highlightEvent is HighlightEvent.
type highlightEvent struct {
	kind       highlightEventKind
	start, end int
	highlight  int
}

// localDef is a definition of a local variable.
//
// localDef is LocalDef.
type localDef struct {
	name                 string
	valueStart, valueEnd int
	highlight            int
}

// localScope is a scope of local variables.
//
// localScope is LocalScope.
type localScope struct {
	inherits   bool
	start, end int
	localDefs  []*localDef
}

// layerCapture is a capture of a layer and a copy of its match.
type layerCapture struct {
	match        transit.QueryMatch
	captureIndex int
}

// node returns the captured node.
func (c layerCapture) node() transit.Node {
	return c.match.Captures[c.captureIndex].Node
}

// highlightIterLayer is one layer of highlighting for a document.
//
// highlightIterLayer is HighlightIterLayer.
type highlightIterLayer struct {
	config            *highlightConfiguration
	highlightEndStack []int
	scopeStack        []localScope
	ranges            []transit.Range
	depth             int

	// next and stop pull the captures of the layer, and peeked holds the
	// capture that peek gave.
	next      func() (transit.QueryMatch, int, bool)
	stop      func()
	peeked    layerCapture
	hasPeeked bool
}

// newHighlightIterLayer makes the layer of highlighting of a layer of
// inject.
func newHighlightIterLayer(ctx context.Context, config *highlightConfiguration, l inject.Layer, src []byte) *highlightIterLayer {
	next, stop := iter.Pull2(transit.NewQueryCursor().Captures(ctx, config.query, l.Tree.RootNode(), src))
	return &highlightIterLayer{
		config:     config,
		scopeStack: []localScope{{inherits: false, start: 0, end: maxOffset}},
		ranges:     l.Ranges,
		depth:      l.Depth,
		next:       next,
		stop:       stop,
	}
}

// maxOffset is usize::MAX as an end of a range.
const maxOffset = int(^uint(0) >> 1)

// peek returns the next capture of the layer, and false when there is
// none.
func (l *highlightIterLayer) peek() (layerCapture, bool) {
	if !l.hasPeeked {
		m, i, ok := l.next()
		if !ok {
			return layerCapture{}, false
		}
		// the cursor reuses the captures, so the layer keeps a copy
		m.Captures = slices.Clone(m.Captures)
		l.peeked, l.hasPeeked = layerCapture{match: m, captureIndex: i}, true
	}
	return l.peeked, true
}

// nextCapture returns the next capture of the layer, as peek does, and
// moves past it.
func (l *highlightIterLayer) nextCapture() (layerCapture, bool) {
	c, ok := l.peek()
	l.hasPeeked = false
	return c, ok
}

// sortKey is the key of the next event of a layer.
type sortKey struct {
	offset  int
	isStart bool
	depth   int
}

// compare orders two keys as the tuple (usize, bool, isize) of upstream.
func (k sortKey) compare(other sortKey) int {
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	return cmp.Or(cmp.Compare(k.offset, other.offset), cmp.Compare(b(k.isStart), b(other.isStart)), cmp.Compare(k.depth, other.depth))
}

// sortKey sorts scope boundaries by their byte offset in the document. At a
// given position, it emits scope endings before scope beginnings. Finally,
// it emits scope boundaries from deeper layers first.
//
// sortKey is sort_key.
func (l *highlightIterLayer) sortKey() (sortKey, bool) {
	depth := -l.depth
	next, hasStart := l.peek()
	hasEnd := len(l.highlightEndStack) > 0
	switch {
	case hasStart && hasEnd:
		start, end := next.node().StartByte(), l.highlightEndStack[len(l.highlightEndStack)-1]
		if start < end {
			return sortKey{start, true, depth}, true
		}
		return sortKey{end, false, depth}, true
	case hasStart:
		return sortKey{next.node().StartByte(), true, depth}, true
	case hasEnd:
		return sortKey{l.highlightEndStack[len(l.highlightEndStack)-1], false, depth}, true
	}
	return sortKey{}, false
}

// highlightIter gives the highlight events of a text.
//
// highlightIter is HighlightIter.
type highlightIter struct {
	source             []byte
	byteOffset         int
	layers             []*highlightIterLayer
	nextEvent          *highlightEvent
	lastHighlightRange *[3]int
}

// newHighlightIter makes the layers of highlighting of the layers of
// inject. configs gives the highlight configuration of the configuration
// of each layer.
//
// newHighlightIter is Highlighter::highlight, with every layer in place of
// the root layer.
func newHighlightIter(ctx context.Context, source []byte, layers []inject.Layer, configs map[*inject.Config]*highlightConfiguration) *highlightIter {
	it := &highlightIter{source: source}
	for _, l := range layers {
		it.insertLayer(newHighlightIterLayer(ctx, configs[l.Config], l, source))
	}
	it.sortLayers()
	return it
}

// close stops the captures of each layer.
func (it *highlightIter) close() {
	for _, l := range it.layers {
		l.stop()
	}
}

// emitEvent is emit_event.
func (it *highlightIter) emitEvent(offset int, event *highlightEvent) (highlightEvent, bool) {
	var result highlightEvent
	ok := false
	if it.byteOffset < offset {
		result, ok = highlightEvent{kind: eventSource, start: it.byteOffset, end: offset}, true
		it.byteOffset = offset
		it.nextEvent = event
	} else if event != nil {
		result, ok = *event, true
	}
	it.sortLayers()
	return result, ok
}

// sortLayers is sort_layers.
func (it *highlightIter) sortLayers() {
	for len(it.layers) > 0 {
		if key, ok := it.layers[0].sortKey(); ok {
			i := 0
			for i+1 < len(it.layers) {
				if nextOffset, ok := it.layers[i+1].sortKey(); ok && nextOffset.compare(key) < 0 {
					i++
					continue
				}
				break
			}
			if i > 0 {
				first := it.layers[0]
				copy(it.layers[:i], it.layers[1:i+1])
				it.layers[i] = first
			}
			break
		}
		it.layers[0].stop()
		it.layers = it.layers[1:]
	}
}

// insertLayer puts a layer after each layer whose key is not greater. A
// layer with no event is dropped.
//
// insertLayer is insert_layer, which starts at the second layer, because
// upstream inserts a layer while the first layer runs. The first layer that
// newHighlightIter inserts goes first.
func (it *highlightIter) insertLayer(layer *highlightIterLayer) {
	key, ok := layer.sortKey()
	if !ok {
		layer.stop()
		return
	}
	i := min(1, len(it.layers))
	for i < len(it.layers) {
		if keyI, ok := it.layers[i].sortKey(); ok {
			if keyI.compare(key) > 0 {
				it.layers = slices.Insert(it.layers, i, layer)
				return
			}
			i++
		} else {
			it.layers[i].stop()
			it.layers = slices.Delete(it.layers, i, i+1)
		}
	}
	it.layers = append(it.layers, layer)
}

// next returns the next event, and false at the end of the events.
//
// next is next of the Iterator of HighlightIter.
func (it *highlightIter) next() (highlightEvent, bool) {
	for {
		// If we've already determined the next highlight boundary, just return it.
		if e := it.nextEvent; e != nil {
			it.nextEvent = nil
			return *e, true
		}

		// If none of the layers have any more highlight boundaries, terminate.
		if len(it.layers) == 0 {
			if it.byteOffset < len(it.source) {
				result := highlightEvent{kind: eventSource, start: it.byteOffset, end: len(it.source)}
				it.byteOffset = len(it.source)
				return result, true
			}
			return highlightEvent{}, false
		}

		// Get the next capture from whichever layer has the earliest highlight boundary.
		layer := it.layers[0]
		next, ok := layer.peek()
		if !ok {
			// If there are no more captures, then emit any remaining highlight end events.
			// And if there are none of those, then just advance to the end of the document.
			if n := len(layer.highlightEndStack); n > 0 {
				endByte := layer.highlightEndStack[n-1]
				layer.highlightEndStack = layer.highlightEndStack[:n-1]
				return it.emitEvent(endByte, &highlightEvent{kind: eventHighlightEnd})
			}
			return it.emitEvent(len(it.source), nil)
		}
		rangeStart, rangeEnd := next.node().StartByte(), next.node().EndByte()

		// If any previous highlight ends before this node starts, then before
		// processing this capture, emit the source code up until the end of the
		// previous highlight, and an end event for that highlight.
		if n := len(layer.highlightEndStack); n > 0 && layer.highlightEndStack[n-1] <= rangeStart {
			endByte := layer.highlightEndStack[n-1]
			layer.highlightEndStack = layer.highlightEndStack[:n-1]
			return it.emitEvent(endByte, &highlightEvent{kind: eventHighlightEnd})
		}

		current, _ := layer.nextCapture()
		match := current.match
		capture := match.Captures[current.captureIndex]

		// If this capture represents an injection, inject.Layers already
		// parsed its layer. Explicitly remove this match so that none of its
		// other captures will remain in the stream of captures.
		if match.PatternIndex < layer.config.localsPatternIndex {
			match.Remove()
			it.sortLayers()
			continue
		}

		// Remove from the local scope stack any local scopes that have already ended.
		for rangeStart > layer.scopeStack[len(layer.scopeStack)-1].end {
			layer.scopeStack = layer.scopeStack[:len(layer.scopeStack)-1]
		}

		// If this capture is for tracking local variables, then process the
		// local variable info.
		referenceHighlight := highlightNone
		var definitionHighlight *localDef
		localsDone := false
		for match.PatternIndex < layer.config.highlightsPatternIndex {
			// If the node represents a local scope, push a new local scope onto
			// the scope stack.
			switch {
			case capture.Index == layer.config.localScopeCaptureIndex:
				definitionHighlight = nil
				scope := localScope{inherits: true, start: rangeStart, end: rangeEnd}
				for _, prop := range layer.config.query.PropertySettings(match.PatternIndex) {
					if prop.Key == "local.scope-inherits" {
						scope.inherits = !prop.HasValue || prop.Value == "true"
					}
				}
				layer.scopeStack = append(layer.scopeStack, scope)
			// If the node represents a definition, add a new definition to the
			// local scope at the top of the scope stack.
			case capture.Index == layer.config.localDefCaptureIndex:
				referenceHighlight = highlightNone
				definitionHighlight = nil
				scope := &layer.scopeStack[len(layer.scopeStack)-1]

				valueStart, valueEnd := 0, 0
				for _, c := range match.Captures {
					if c.Index == layer.config.localDefValueCaptureIndex {
						valueStart, valueEnd = c.Node.StartByte(), c.Node.EndByte()
					}
				}

				if name := it.source[rangeStart:rangeEnd]; utf8.Valid(name) {
					def := &localDef{name: string(name), valueStart: valueStart, valueEnd: valueEnd, highlight: highlightNone}
					scope.localDefs = append(scope.localDefs, def)
					definitionHighlight = def
				}
			// If the node represents a reference, then try to find the corresponding
			// definition in the scope stack.
			case capture.Index == layer.config.localRefCaptureIndex && definitionHighlight == nil:
				if name := it.source[rangeStart:rangeEnd]; utf8.Valid(name) {
				scopes:
					for _, scope := range slices.Backward(layer.scopeStack) {
						for _, def := range slices.Backward(scope.localDefs) {
							if def.name == string(name) && rangeStart >= def.valueEnd {
								referenceHighlight = def.highlight
								break scopes
							}
						}
						if !scope.inherits {
							break
						}
					}
				}
			}

			// Continue processing any additional matches for the same node.
			if following, ok := layer.peek(); ok && following.node().Equal(capture.Node) {
				layer.nextCapture()
				capture = following.match.Captures[following.captureIndex]
				match = following.match
				continue
			}

			it.sortLayers()
			localsDone = true
			break
		}
		if localsDone {
			continue
		}

		// Otherwise, this capture must represent a highlight.
		// If this exact range has already been highlighted by an earlier pattern, or by
		// a different layer, then skip over this one.
		if last := it.lastHighlightRange; last != nil && rangeStart == last[0] && rangeEnd == last[1] && layer.depth < last[2] {
			it.sortLayers()
			continue
		}

		// Once a highlighting pattern is found for the current node, keep iterating over
		// any later highlighting patterns that also match this node and set the match to it.
		// Captures for a given node are ordered by pattern index, so these subsequent
		// captures are guaranteed to be for highlighting, not injections or
		// local variables.
		for {
			following, ok := layer.peek()
			if !ok || !following.node().Equal(capture.Node) {
				break
			}
			layer.nextCapture()
			// If the current node was found to be a local variable, then ignore
			// the following match if it's a highlighting pattern that is disabled
			// for local variables.
			if (definitionHighlight != nil || referenceHighlight != highlightNone) &&
				layer.config.nonLocalVariablePatterns[following.match.PatternIndex] {
				continue
			}
			match.Remove()
			capture = following.match.Captures[following.captureIndex]
			match = following.match
		}

		currentHighlight := layer.config.highlightIndices[capture.Index]

		// If this node represents a local definition, then store the current
		// highlight value on the local scope entry representing this node.
		if definitionHighlight != nil {
			definitionHighlight.highlight = currentHighlight
		}

		// Emit a scope start event and push the node's end position to the stack.
		highlight := referenceHighlight
		if highlight == highlightNone {
			highlight = currentHighlight
		}
		if highlight != highlightNone {
			it.lastHighlightRange = &[3]int{rangeStart, rangeEnd, layer.depth}
			layer.highlightEndStack = append(layer.highlightEndStack, rangeEnd)
			return it.emitEvent(rangeStart, &highlightEvent{kind: eventHighlightStart, highlight: highlight})
		}

		it.sortLayers()
	}
}
