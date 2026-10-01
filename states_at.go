package transit

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/xo/transit/internal/abi"
)

// This file ports no upstream file. It holds StatesAt, the first API that
// upstream does not have (D28, D57, D70). It runs the parse loop of ParseInput
// with the functions of parser.go, and it stops each stack version before
// the version handles a token at the offset.

// StatesAt parses src up to offset, and returns the parse state of each
// stack version at offset, before the parser recovers from an error. A
// consumer lists the symbols that can come next with the lookahead iterator
// of each state.
//
// StatesAt stops each stack version before the version reduces or detects
// an error on the first token that ends after offset, or on the end of the
// text. A token with no width at offset, such as one that an external
// scanner gives before a word, is handled. The lexer reads the text after
// offset as it does in a parse of all of src, so a token before offset is
// the token of that parse. When offset is inside a word, StatesAt gives the
// states before the word. A word that ends at offset is handled, so to
// complete a word, a consumer gives the offset of its start.
//
// The states come in the order of the stack versions, with no state twice.
// A version that recovers from an error before offset gives the state that
// it has at offset after the recovery. When old is the tree of src, after
// Tree.Edit, the parser reuses the parts of it that come before offset. old
// does not change.
//
// StatesAt uses the parser, so it resets the parser first, as Reset does, and
// a parse that a context stopped does not go on. When ctx ends, StatesAt
// returns its error, wrapped.
func (p *Parser) StatesAt(ctx context.Context, src []byte, offset int, old *Tree) ([]StateID, error) {
	switch {
	case p.language == nil:
		return nil, ErrNoLanguage
	case offset < 0 || offset > len(src):
		return nil, fmt.Errorf("offset %d outside text of %d bytes: %w", offset, len(src), ErrInvalidInput)
	case old != nil && old.language != p.language:
		return nil, ErrLanguageMismatch
	}
	p.Reset()
	defer p.Reset()

	p.lexer.setInput(input{read: &stringInput{text: src}, encoding: EncodingUTF8})
	p.includedRangeDifferences = p.includedRangeDifferences[:0]
	p.includedRangeDifferenceIndex = 0
	p.operationCount = 0
	p.externalScannerCreate()
	if old != nil {
		// A node that holds the last leaf before offset, or a token after
		// it, ends after the last token that a version must handle. An edit
		// of a copy of the old tree that changes nothing from the start of
		// that leaf to the end of the text marks each such node as changed,
		// so the parser does not reuse it.
		old = old.Copy()
		start, startPoint := 0, Point{}
		if leaf, ok := lastLeafBefore(old.RootNode(), offset); ok {
			start, startPoint = leaf.StartByte(), leaf.StartPoint()
		}
		end := pointAt(src, len(src))
		old.Edit(InputEdit{
			StartByte:   start,
			OldEndByte:  len(src),
			NewEndByte:  len(src),
			StartPoint:  startPoint,
			OldEndPoint: end,
			NewEndPoint: end,
		})
		old.root.retain()
		p.oldTree = old.root
		rangeArrayGetChangedRanges(
			old.includedRanges,
			p.lexer.includedRanges,
			&p.includedRangeDifferences,
		)
		p.reusableNode.reset(old.root)
	} else {
		p.reusableNode.clear()
	}

	var position, lastPosition, versionCount uint32
	for {
		advanced := false
		for version := stackVersion(0); ; version++ {
			versionCount = p.stack.versionCount()
			if uint32(version) >= versionCount {
				break
			}
			allowNodeReuse := versionCount == 1
			for p.stack.isActive(version) {
				at, ok := p.atOffset(version, uint32(offset))
				if !ok && p.reduceNonTerminalExtra(version) {
					// advance would reduce the extra, and then lex and handle
					// the next token with no check.
					advanced = true
					continue
				}
				if at {
					break
				}
				if !p.advance(ctx, version, allowNodeReuse) {
					return nil, fmt.Errorf("finding states at offset %d: %w", offset, ctx.Err())
				}
				advanced = true
				position = p.stack.position(version).bytes
				if position > lastPosition || (version > 0 && position == lastPosition) {
					lastPosition = position
					break
				}
			}
		}

		// When no version advanced, each version that is left is at offset.
		if !advanced {
			break
		}

		p.condenseStack()
		for int(p.includedRangeDifferenceIndex) < len(p.includedRangeDifferences) {
			r := &p.includedRangeDifferences[p.includedRangeDifferenceIndex]
			if r.endByte <= position {
				p.includedRangeDifferenceIndex++
			} else {
				break
			}
		}
		if versionCount == 0 {
			break
		}
	}

	var states []StateID
	for version := stackVersion(0); uint32(version) < p.stack.versionCount(); version++ {
		if state := p.stack.state(version); p.stack.isActive(version) && !slices.Contains(states, state) {
			states = append(states, state)
		}
	}
	return states, nil
}

// atOffset reports whether the next token of a version ends after offset, or
// is the end of the text. It lexes the token as advance does, and it puts the
// token in the token cache, so that advance does not lex it again. At the end
// of a non-terminal extra, lex gives no token, and ok is false.
func (p *Parser) atOffset(version stackVersion, offset uint32) (at, ok bool) {
	state := p.stack.state(version)
	position := p.stack.position(version).bytes
	lastExternalToken := p.stack.lastExternalToken(version)
	var entry tableEntry
	lookahead := p.getCachedToken(state, position, lastExternalToken, &entry)
	if lookahead.ptr == nil {
		if lookahead = p.lex(version, state); lookahead.ptr == nil {
			return false, false
		}
		p.setCachedToken(position, lastExternalToken, lookahead)
	}
	at = lookahead.symbol() == builtinSymEnd || position+lookahead.totalSize().bytes > offset
	lookahead.release(&p.treePool)
	return at, true
}

// reduceNonTerminalExtra does what advance does at the end of a non-terminal
// extra, before it lexes the next token. It reduces the extra with the
// actions of the end of the text, and it keeps the version of the last
// reduction as the version. It returns false, and does nothing, when the
// table has no reduction there.
func (p *Parser) reduceNonTerminalExtra(version stackVersion) bool {
	entry := p.language.tableEntry(p.stack.state(version), builtinSymEnd)
	if !slices.ContainsFunc(entry.actions, func(a abi.ParseActionEntry) bool {
		return a.Action.Type == abi.ParseActionTypeReduce
	}) {
		return false
	}
	lastReductionVersion := stackVersionNone
	for _, a := range entry.actions {
		if a.Action.Type != abi.ParseActionTypeReduce {
			continue
		}
		if p.logging() {
			p.logf("reduce sym:%s, child_count:%d", p.symName(Symbol(a.Action.Reduce.Symbol)), a.Action.Reduce.ChildCount)
		}
		reductionVersion := p.reduce(
			version, Symbol(a.Action.Reduce.Symbol), uint32(a.Action.Reduce.ChildCount),
			int32(a.Action.Reduce.DynamicPrecedence), a.Action.Reduce.ProductionID,
			len(entry.actions) > 1, true,
		)
		if reductionVersion != stackVersionNone {
			lastReductionVersion = reductionVersion
		}
	}
	if lastReductionVersion != stackVersionNone {
		p.stack.renumberVersion(lastReductionVersion, version)
	} else {
		// The reduction merged into another version.
		p.stack.halt(version)
	}
	return true
}

// lastLeafBefore returns the last leaf of a tree that starts before offset.
func lastLeafBefore(n Node, offset int) (Node, bool) {
	found := false
	for {
		var next Node
		ok := false
		for c := range n.Children() {
			if c.StartByte() >= offset {
				break
			}
			next, ok = c, true
		}
		if !ok {
			return n, found
		}
		n, found = next, true
	}
}

// pointAt returns the point of a byte offset of src.
func pointAt(src []byte, offset int) Point {
	row := bytes.Count(src[:offset], []byte{'\n'})
	return Point{Row: row, Column: offset - (bytes.LastIndexByte(src[:offset], '\n') + 1)}
}
