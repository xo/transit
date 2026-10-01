package inject

import (
	"fmt"

	"github.com/xo/transit"
)

// This file holds the options of Layers. It ports no upstream file: the
// options are an API that upstream does not have (D28). The first one
// replaces the text of nodes before a layer is parsed, so that a variable of
// the input of usql reaches the SQL grammar as a placeholder of the same
// length (D101).

// Option changes what Layers does.
type Option func(*options)

// options holds the options of one call of Layers.
type options struct {
	replacer Replacer
}

// Replacer gives the text that takes the place of the node n of a parent
// layer in the text of the injected layer of the language name, and true.
// It returns false for a node whose text stays. src is the whole text. The
// text must have as many bytes as the node, so that every offset of the
// layer stays the same.
type Replacer func(name string, n transit.Node, src []byte) ([]byte, bool)

// WithReplacer makes Layers ask r about the nodes of the parent layer that
// an injected layer holds, before it parses that layer. Layers walks the
// named and anonymous nodes of the parent tree inside the ranges of the
// layer, from the root down. When r gives a text for a node, the layer
// parses that text in place of the text of the node, and Layers does not
// ask r about the children of the node. The root layer has no parent, so
// its text stays. The text of the other layers, and the text that the
// injection queries match, stay the text of src.
func WithReplacer(r Replacer) Option {
	return func(o *options) { o.replacer = r }
}

// replaced returns the text that the layer of the language name over ranges
// parses: src, with the replacements of the replacer for the nodes of
// parent that the ranges hold. It returns src when nothing is replaced, and
// a copy of src when something is. A replacement whose length is not the
// length of its node is an error.
func (b *builder) replaced(name string, parent *transit.Tree, ranges []transit.Range) ([]byte, error) {
	if b.replacer == nil || parent == nil {
		return b.src, nil
	}
	text, copied := b.src, false
	c := parent.Walk()
	for {
		n := c.Node()
		inside, overlaps := within(n, ranges)
		replacement, ok := []byte(nil), false
		if inside {
			replacement, ok = b.replacer(name, n, b.src)
		}
		if ok {
			if len(replacement) != n.EndByte()-n.StartByte() {
				return nil, fmt.Errorf("replacing the node %s at byte %d in the layer of %s: the text has %d bytes, and the node has %d",
					n.Kind(), n.StartByte(), name, len(replacement), n.EndByte()-n.StartByte())
			}
			if !copied {
				text, copied = append([]byte(nil), b.src...), true
			}
			copy(text[n.StartByte():], replacement)
		}
		if !ok && overlaps && c.GotoFirstChild() {
			continue
		}
		for !c.GotoNextSibling() {
			if !c.GotoParent() {
				return text, nil
			}
		}
	}
}

// within reports whether one of the ranges holds the whole node n, and
// whether one of them holds a part of it.
func within(n transit.Node, ranges []transit.Range) (inside, overlaps bool) {
	start, end := n.StartByte(), n.EndByte()
	for _, r := range ranges {
		if start >= r.StartByte && end <= r.EndByte && start < end {
			return true, true
		}
		if start < r.EndByte && end > r.StartByte {
			overlaps = true
		}
	}
	return false, overlaps
}
