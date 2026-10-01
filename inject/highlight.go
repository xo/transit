package inject

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/xo/transit"
)

// This file ports the part of crates/highlight/src/highlight.rs of upstream
// that finds injections and parses their layers (D27). HighlightIter finds
// the layers as it gives highlight events. Layers finds all of them at once,
// with the same functions, and sorts them (D72).

// Config holds the language, the name and the injection query of a
// language. It does not change after NewConfig, so goroutines can share it.
//
// Config is the injection part of HighlightConfiguration.
type Config struct {
	language                      *transit.Language
	languageName                  string
	query                         *transit.Query
	combinedInjectionsQuery       *transit.Query
	localsPatternIndex            int
	injectionContentCaptureIndex  int
	injectionLanguageCaptureIndex int
}

// Layer is one tree of one language over the ranges of its injections.
type Layer struct {
	// Name is the language name that the injection query gave, such as
	// "js". For the root layer it is the name of its Config.
	Name string
	// Config is the configuration that the lookup of Layers gave for Name.
	Config *Config
	// Tree is the tree of the layer.
	Tree *transit.Tree
	// Ranges are the ranges of the text that the layer parses. The root
	// layer has one range, from the start of the text to the largest byte
	// and point that the parser takes.
	Ranges []transit.Range
	// Depth is 0 for the root layer, and one more than the depth of its
	// parent for another layer.
	Depth int
}

// NewConfig compiles the injection query of a language. name is the name of
// the language, which injection.self gives. The error wraps a
// *transit.QueryError.
//
// NewConfig is HighlightConfiguration::new, with no highlight query and no
// locals query (D72).
func NewConfig(language *transit.Language, name, injectionQuery string) (*Config, error) {
	// Upstream joins the injection, locals and highlight queries into one
	// query, and every pattern before localsPatternIndex is an injection
	// pattern. With the injection query only, every pattern is one.
	query, err := transit.NewQuery(language, injectionQuery)
	if err != nil {
		return nil, fmt.Errorf("compiling the injection query of %s: %w", name, err)
	}
	localsPatternIndex := query.PatternCount()

	// Construct a separate query just for dealing with the 'combined
	// injections'. Disable the combined injection patterns in the main
	// query.
	combinedInjectionsQuery, err := transit.NewQuery(language, injectionQuery)
	if err != nil {
		return nil, fmt.Errorf("compiling the injection query of %s: %w", name, err)
	}
	hasCombinedQueries := false
	for patternIndex := range localsPatternIndex {
		settings := query.PropertySettings(patternIndex)
		if slices.ContainsFunc(settings, func(s transit.QueryProperty) bool {
			return s.Key == "injection.combined"
		}) {
			hasCombinedQueries = true
			query.DisablePattern(patternIndex)
		} else {
			combinedInjectionsQuery.DisablePattern(patternIndex)
		}
	}
	if !hasCombinedQueries {
		combinedInjectionsQuery = nil
	}

	// Store the numeric ids for all of the special captures.
	c := &Config{
		language:                      language,
		languageName:                  name,
		query:                         query,
		combinedInjectionsQuery:       combinedInjectionsQuery,
		localsPatternIndex:            localsPatternIndex,
		injectionContentCaptureIndex:  -1,
		injectionLanguageCaptureIndex: -1,
	}
	for i, name := range query.CaptureNames() {
		switch name {
		case "injection.content":
			c.injectionContentCaptureIndex = i
		case "injection.language":
			c.injectionLanguageCaptureIndex = i
		}
	}
	return c, nil
}

// Name returns the name of the language.
func (c *Config) Name() string {
	return c.languageName
}

// Language returns the language.
func (c *Config) Language() *transit.Language {
	return c.language
}

// Layers parses src with c, finds every injection, and parses the layer of
// each. lookup gives the configuration of an injected language name, and an
// injection whose name it does not find is skipped. The parser p parses
// every layer. Layers clears the included ranges of p before it returns, and
// the language of the last layer stays set.
//
// The layers come sorted by the start of their first range, then by depth,
// so the root layer comes first. Two layers that are equal on both keep the
// order in which Layers finds them (D72). A layer with no range is not
// built, and neither is a layer whose ranges the parser rejects. When the
// parser cannot take the language of a layer, or when ctx ends, Layers
// returns the error, wrapped, and no layers.
//
// As upstream does, injection.parent gives the name of the root language at
// every depth, and there is no limit on the depth (hard rule 6).
//
// Layers runs HighlightIterLayer::new for the root layer, and the injection
// part of the Iterator of HighlightIter for each layer, as Highlighter::highlight
// does when its events are read to the end.
func (c *Config) Layers(ctx context.Context, p *transit.Parser, src []byte, lookup func(name string) (*Config, bool)) ([]Layer, error) {
	defer func() {
		// Clearing the ranges gives the parser its default range, which it
		// always takes.
		_ = p.SetIncludedRanges(nil)
	}()

	b := &builder{
		p:              p,
		src:            src,
		lookup:         lookup,
		capturesCursor: transit.NewQueryCursor(),
		matchesCursor:  transit.NewQueryCursor(),
	}
	layers, err := b.newLayer(ctx, "", false, c, c.languageName, 0, []transit.Range{{
		StartByte:  0,
		StartPoint: transit.Point{Row: 0, Column: 0},
		EndByte:    math.MaxUint32,
		EndPoint:   transit.Point{Row: math.MaxUint32, Column: math.MaxUint32},
	}})
	if err != nil {
		return nil, err
	}

	// Each layer of the list, and each layer that it adds, gives its
	// injections in turn. The name of the root language is the parent name
	// of every injection, as in the Iterator of HighlightIter.
	for i := 0; i < len(layers); i++ {
		added, err := b.injections(ctx, c.languageName, layers[i])
		if err != nil {
			return nil, err
		}
		layers = append(layers, added...)
	}

	slices.SortStableFunc(layers, func(a, b Layer) int {
		return cmp.Or(
			cmp.Compare(a.Ranges[0].StartByte, b.Ranges[0].StartByte),
			cmp.Compare(a.Depth, b.Depth),
		)
	})
	return layers, nil
}

// builder holds what the functions of Layers share: the parser, the text,
// the lookup and the query cursors of the Highlighter of upstream.
type builder struct {
	p      *transit.Parser
	src    []byte
	lookup func(name string) (*Config, bool)

	// capturesCursor runs the injection query of each layer in turn.
	// matchesCursor runs the query of the combined injections inside
	// newLayer, which injections calls while capturesCursor runs.
	capturesCursor *transit.QueryCursor
	matchesCursor  *transit.QueryCursor
}

// queuedLayer is a layer of a combined injection that newLayer builds after
// the layer that holds it.
type queuedLayer struct {
	config *Config
	name   string
	depth  int
	ranges []transit.Range
}

// combinedInjection is the language name, the content nodes and the
// include-children setting of the matches of one pattern of the combined
// injections.
type combinedInjection struct {
	languageName    string
	hasLanguageName bool
	contentNodes    []transit.Node
	includeChildren bool
}

// newLayer parses a layer of config over ranges, and the layers of its
// combined injections, and returns them in that order. parentName is the
// name that injection.parent gives, when hasParentName is true.
//
// In the event that the new layer contains "combined injections"
// (injections where multiple disjoint ranges are parsed as one syntax tree),
// these will be eagerly processed and added to the returned slice.
//
// newLayer is HighlightIterLayer::new.
func (b *builder) newLayer(
	ctx context.Context,
	parentName string,
	hasParentName bool,
	config *Config,
	name string,
	depth int,
	ranges []transit.Range,
) ([]Layer, error) {
	result := make([]Layer, 0, 1)
	var queue []queuedLayer
	for {
		if b.p.SetIncludedRanges(ranges) == nil {
			if err := b.p.SetLanguage(config.language); err != nil {
				return nil, fmt.Errorf("parsing the layer of %s: %w", config.languageName, err)
			}
			tree, err := b.p.Parse(ctx, b.src, nil)
			if err != nil {
				return nil, fmt.Errorf("parsing the layer of %s: %w", config.languageName, err)
			}

			// Process combined injections.
			if combinedInjectionsQuery := config.combinedInjectionsQuery; combinedInjectionsQuery != nil {
				injectionsByPatternIndex := make([]combinedInjection, combinedInjectionsQuery.PatternCount())
				for m := range b.matchesCursor.Matches(ctx, combinedInjectionsQuery, tree.RootNode(), b.src) {
					entry := &injectionsByPatternIndex[m.PatternIndex]
					inj := injectionForMatch(config, parentName, hasParentName, combinedInjectionsQuery, m, b.src)
					if inj.hasLanguageName {
						entry.languageName, entry.hasLanguageName = inj.languageName, true
					}
					if inj.hasContentNode {
						entry.contentNodes = append(entry.contentNodes, inj.contentNode)
					}
					entry.includeChildren = inj.includeChildren
				}
				if err := ctx.Err(); err != nil {
					return nil, fmt.Errorf("parsing the layer of %s: %w", config.languageName, err)
				}
				for _, entry := range injectionsByPatternIndex {
					if !entry.hasLanguageName || len(entry.contentNodes) == 0 {
						continue
					}
					nextConfig, ok := b.lookup(entry.languageName)
					if !ok {
						continue
					}
					injectionRanges := intersectRanges(ranges, entry.contentNodes, entry.includeChildren)
					if len(injectionRanges) != 0 {
						queue = append(queue, queuedLayer{nextConfig, entry.languageName, depth + 1, injectionRanges})
					}
				}
			}

			result = append(result, Layer{
				Name:   name,
				Config: config,
				Tree:   tree,
				Ranges: ranges,
				Depth:  depth,
			})
		}

		if len(queue) == 0 {
			break
		}

		next := queue[0]
		queue = queue[1:]
		config, name, depth, ranges = next.config, next.name, next.depth, next.ranges
	}
	return result, nil
}

// injections runs the injection query of a layer, and parses the layers of
// the injections that it finds. parentName is the name that
// injection.parent gives.
//
// injections is the part of next of the Iterator of HighlightIter that
// processes a capture of an injection pattern.
func (b *builder) injections(ctx context.Context, parentName string, layer Layer) ([]Layer, error) {
	var added []Layer
	config := layer.Config
	for m := range b.capturesCursor.Captures(ctx, config.query, layer.Tree.RootNode(), b.src) {
		// If this capture represents an injection, then process the
		// injection.
		if m.PatternIndex >= config.localsPatternIndex {
			continue
		}
		inj := injectionForMatch(config, parentName, true, config.query, m, b.src)

		// Explicitly remove this match so that none of its other captures will
		// remain in the stream of captures.
		m.Remove()

		// If a language is found with the given name, then add a new
		// language layer to the document.
		if !inj.hasLanguageName || !inj.hasContentNode {
			continue
		}
		nextConfig, ok := b.lookup(inj.languageName)
		if !ok {
			continue
		}
		ranges := intersectRanges(layer.Ranges, []transit.Node{inj.contentNode}, inj.includeChildren)
		if len(ranges) == 0 {
			continue
		}
		layers, err := b.newLayer(ctx, parentName, true, nextConfig, inj.languageName, layer.Depth+1, ranges)
		if err != nil {
			return nil, err
		}
		added = append(added, layers...)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("finding the injections of %s: %w", config.languageName, err)
	}
	return added, nil
}

// intersectRanges computes the ranges that should be included when parsing
// an injection. This takes into account three things:
//
//   - parentRanges: The ranges must all fall within the current layer's
//     ranges.
//   - nodes: Every injection takes place within a set of nodes. The
//     injection ranges are the ranges of those nodes.
//   - includesChildren: For some injections, the content nodes' children
//     should be excluded from the nested document, so that only the content
//     nodes' own content is reparsed. For other injections, the content
//     nodes' entire ranges should be reparsed, including the ranges of their
//     children.
//
// intersectRanges is HighlightIterLayer::intersect_ranges.
func intersectRanges(parentRanges []transit.Range, nodes []transit.Node, includesChildren bool) []transit.Range {
	var result []transit.Range
	if len(parentRanges) == 0 {
		panic("Layers should only be constructed with non-empty ranges vectors")
	}
	parentRangeIndex := 0
	parentRange := parentRanges[0]
	for _, node := range nodes {
		precedingRange := transit.Range{
			StartByte:  0,
			StartPoint: transit.Point{Row: 0, Column: 0},
			EndByte:    node.StartByte(),
			EndPoint:   node.StartPoint(),
		}
		followingRange := transit.Range{
			StartByte:  node.EndByte(),
			StartPoint: node.EndPoint(),
			EndByte:    math.MaxInt,
			EndPoint:   transit.Point{Row: math.MaxInt, Column: math.MaxInt},
		}

		var excludedRanges []transit.Range
		if !includesChildren {
			for child := range node.Children() {
				excludedRanges = append(excludedRanges, child.Range())
			}
		}
		excludedRanges = append(excludedRanges, followingRange)

		for _, excludedRange := range excludedRanges {
			r := transit.Range{
				StartByte:  precedingRange.EndByte,
				StartPoint: precedingRange.EndPoint,
				EndByte:    excludedRange.StartByte,
				EndPoint:   excludedRange.StartPoint,
			}
			precedingRange = excludedRange

			if r.EndByte < parentRange.StartByte {
				continue
			}

			for parentRange.StartByte <= r.EndByte {
				if parentRange.EndByte > r.StartByte {
					if r.StartByte < parentRange.StartByte {
						r.StartByte = parentRange.StartByte
						r.StartPoint = parentRange.StartPoint
					}

					if parentRange.EndByte < r.EndByte {
						if r.StartByte < parentRange.EndByte {
							result = append(result, transit.Range{
								StartByte:  r.StartByte,
								StartPoint: r.StartPoint,
								EndByte:    parentRange.EndByte,
								EndPoint:   parentRange.EndPoint,
							})
						}
						r.StartByte = parentRange.EndByte
						r.StartPoint = parentRange.EndPoint
					} else {
						if r.StartByte < r.EndByte {
							result = append(result, r)
						}
						break
					}
				}

				parentRangeIndex++
				if parentRangeIndex == len(parentRanges) {
					return result
				}
				parentRange = parentRanges[parentRangeIndex]
			}
		}
	}
	return result
}

// injection is what injectionForMatch finds in a match.
type injection struct {
	languageName    string
	hasLanguageName bool
	contentNode     transit.Node
	hasContentNode  bool
	includeChildren bool
}

// injectionForMatch finds the language name, the content node and the
// include-children setting of a match of an injection pattern.
//
// injectionForMatch is injection_for_match.
func injectionForMatch(
	config *Config,
	parentName string,
	hasParentName bool,
	query *transit.Query,
	m transit.QueryMatch,
	src []byte,
) injection {
	var inj injection
	for _, capture := range m.Captures {
		switch capture.Index {
		case config.injectionLanguageCaptureIndex:
			// utf8_text of upstream fails on text that is not UTF-8.
			if text := capture.Node.Text(src); utf8.ValidString(text) {
				inj.languageName, inj.hasLanguageName = text, true
			} else {
				inj.languageName, inj.hasLanguageName = "", false
			}
		case config.injectionContentCaptureIndex:
			inj.contentNode, inj.hasContentNode = capture.Node, true
		}
	}

	for _, prop := range query.PropertySettings(m.PatternIndex) {
		switch {
		// In addition to specifying the language name via the text of a
		// captured node, it can also be hard-coded via a #set! predicate
		// that sets the injection.language key.
		case prop.Key == "injection.language" && !inj.hasLanguageName:
			inj.languageName, inj.hasLanguageName = prop.Value, prop.HasValue

		// Setting the injection.self key can be used to specify that the
		// language name should be the same as the language of the current
		// layer.
		case prop.Key == "injection.self" && !inj.hasLanguageName:
			inj.languageName, inj.hasLanguageName = config.languageName, true

		// Setting the injection.parent key can be used to specify that the
		// language name should be the same as the language of the parent
		// layer.
		case prop.Key == "injection.parent" && !inj.hasLanguageName:
			inj.languageName, inj.hasLanguageName = parentName, hasParentName

		// By default, injections do not include the children of an
		// injection.content node, only the ranges that belong to the node
		// itself. This can be changed using a #set! predicate that sets the
		// injection.include-children key.
		case prop.Key == "injection.include-children":
			inj.includeChildren = true
		}
	}
	return inj
}
