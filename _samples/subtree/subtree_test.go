// Package subtree is the benchmark of D29: it compares two Go forms of a
// subtree of the runtime before subtree.c is ported. It is a prototype, and
// the runtime does not import it. Run it with:
//
//	go test ./_samples/subtree -bench . -benchmem
//	go test ./_samples/subtree -run Session -v
//
// Form A is a pointer to a struct that holds a slice of its children, with
// two variants: A-inline holds a small leaf inline in a value of 16 bytes, as
// C does, and A-slab takes the nodes of a parse from chunks. Form B is an
// index into slices that the trees share: a tree that an edit makes
// reuses the nodes of the old tree, as the parser reuses old subtrees, so
// the nodes of every version live in one arena, which a compaction copies
// when it grows too large.
//
// Both forms hold the fields of SubtreeHeapData of upstream, and both build
// the same trees from the same seeded token stream.
package subtree

import (
	"math/rand/v2"
	"runtime"
	"runtime/metrics"
	"slices"
	"testing"
	"time"
)

// length is Length of upstream: bytes, and a point of rows and columns.
type length struct {
	bytes, row, column uint32
}

// add returns the sum of two lengths, as length_add does.
func (a length) add(b length) length {
	if b.row > 0 {
		return length{a.bytes + b.bytes, a.row + b.row, b.column}
	}
	return length{a.bytes + b.bytes, a.row, a.column + b.column}
}

// fields are the fields of SubtreeHeapData, without the children.
type fields struct {
	padding, size                                              length
	lookaheadBytes, errorCost                                  uint32
	symbol, parseState                                         uint16
	visible, named, extra, fragile                             bool
	visibleChildCount, namedChildCount, visibleDescendantCount uint32
	dynamicPrecedence                                          int32
	repeatDepth, productionID                                  uint16
}

// token is one leaf of the input.
type token struct {
	padding, size length
	symbol        uint16
}

// tokens returns a seeded stream of n tokens, with the sizes of the tokens of
// a program: most one to eight bytes, some padding, and a new line in about
// one token of ten.
func tokens(n int) []token {
	r := rand.New(rand.NewPCG(1, 2))
	out := make([]token, n)
	for i := range out {
		p := length{bytes: uint32(r.IntN(2))}
		if r.IntN(10) == 0 {
			p = length{bytes: 1 + uint32(r.IntN(4)), row: 1, column: uint32(r.IntN(4))}
		}
		s := uint32(1 + r.IntN(8))
		out[i] = token{padding: p, size: length{bytes: s, column: s}, symbol: uint16(1 + r.IntN(100))}
	}
	return out
}

// shape is the order of the reductions of an LR parser over the tokens: for
// each step, a shift (0) or a reduction of k items (k > 0). It is seeded, so
// both forms build the same trees. The tokens come in statements of 5 to 15
// tokens, which reduce inside themselves, and the statements join in a
// balanced binary tree, as a repetition does after ts_subtree_compress. So
// the depth is the depth of a statement and the log of their number, as in
// the tree of a real grammar.
func shape(n int) []int {
	r := rand.New(rand.NewPCG(3, 4))
	var steps []int
	var ranks []int
	for i := 0; i < n; {
		m := min(n-i, 5+r.IntN(11))
		depth := 0
		for range m {
			steps = append(steps, 0)
			i++
			depth++
			for depth >= 2 && r.IntN(3) > 0 {
				k := min(depth, 2+r.IntN(3))
				steps = append(steps, k)
				depth -= k - 1
			}
		}
		for depth > 1 {
			k := min(depth, 4)
			steps = append(steps, k)
			depth -= k - 1
		}
		// join the statements of equal rank, as a binary counter does
		ranks = append(ranks, 0)
		for len(ranks) >= 2 && ranks[len(ranks)-1] == ranks[len(ranks)-2] {
			steps = append(steps, 2)
			rank := ranks[len(ranks)-1] + 1
			ranks = append(ranks[:len(ranks)-2], rank)
		}
	}
	for len(ranks) > 1 {
		steps = append(steps, 2)
		ranks = ranks[:len(ranks)-1]
	}
	return steps
}

// depth returns the height of the tree, for the log of the benchmark.
func depth(t *subtreeA) int {
	d := 0
	for _, c := range t.children {
		d = max(d, depth(c))
	}
	return d + 1
}

// Form A: a pointer to a struct with a slice of children.

type subtreeA struct {
	fields
	children []*subtreeA
}

// summarizeA does the work of ts_subtree_summarize_children for form A.
func summarizeA(t *subtreeA) {
	t.visibleChildCount, t.namedChildCount, t.visibleDescendantCount = 0, 0, 0
	t.errorCost, t.dynamicPrecedence = 0, 0
	var total length
	for i, c := range t.children {
		if i == 0 {
			t.padding = c.padding
			total = c.size
		} else {
			total = total.add(c.padding).add(c.size)
		}
		t.errorCost += c.errorCost
		t.dynamicPrecedence += c.dynamicPrecedence
		t.visibleDescendantCount += c.visibleDescendantCount
		if c.visible {
			t.visibleDescendantCount++
			t.visibleChildCount++
			if c.named {
				t.namedChildCount++
			}
		} else if len(c.children) > 0 {
			t.visibleChildCount += c.visibleChildCount
			t.namedChildCount += c.namedChildCount
		}
		t.lookaheadBytes = max(t.lookaheadBytes, c.lookaheadBytes)
	}
	t.size = total
}

// buildA builds the tree of the tokens and the shape in form A.
func buildA(toks []token, steps []int) *subtreeA {
	var stack []*subtreeA
	next := 0
	for _, k := range steps {
		if k == 0 {
			tk := toks[next]
			next++
			stack = append(stack, &subtreeA{fields: fields{padding: tk.padding, size: tk.size, symbol: tk.symbol, visible: tk.symbol%3 != 0, named: tk.symbol%2 == 0, lookaheadBytes: 1}})
			continue
		}
		children := slices.Clone(stack[len(stack)-k:])
		stack = stack[:len(stack)-k]
		n := &subtreeA{fields: fields{symbol: uint16(200 + k), visible: k%2 == 0, named: true}, children: children}
		summarizeA(n)
		stack = append(stack, n)
	}
	return stack[0]
}

// walkA walks the tree in document order, as a tree cursor does, and returns
// the number of visible nodes and the end byte, so the work is not dropped.
func walkA(root *subtreeA) (int, uint32) {
	type frame struct {
		t     *subtreeA
		start uint32
	}
	stack := []frame{{root, 0}}
	visible, end := 0, uint32(0)
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.t.visible {
			visible++
		}
		end = max(end, f.start+f.t.padding.bytes+f.t.size.bytes)
		off := f.start
		for _, c := range f.t.children {
			stack = append(stack, frame{c, off})
			off += c.padding.bytes + c.size.bytes
		}
	}
	return visible, end
}

// editA changes the size of the leaf at a byte offset, and returns the new
// tree. It clones each node on the path from the root, as ts_subtree_make_mut
// does for a shared subtree, summarizes the path again, and shares every
// other subtree with the old tree.
func editA(root *subtreeA, at uint32, delta uint32) *subtreeA {
	clone := *root
	clone.children = slices.Clone(root.children)
	cur := &clone
	var path []*subtreeA
	off := uint32(0)
	for len(cur.children) > 0 {
		path = append(path, cur)
		i := 0
		for ; i < len(cur.children)-1; i++ {
			c := cur.children[i]
			if off+c.padding.bytes+c.size.bytes > at {
				break
			}
			off += c.padding.bytes + c.size.bytes
		}
		child := *cur.children[i]
		child.children = slices.Clone(cur.children[i].children)
		cur.children[i] = &child
		cur = &child
	}
	cur.size.bytes += delta
	cur.size.column += delta
	for _, n := range slices.Backward(path) {
		summarizeA(n)
	}
	return &clone
}

// Form A-inline: form A, but a subtree is a value of 16 bytes that holds
// either a pointer or a small leaf inline, as C packs a small leaf into the
// pointer (SubtreeInlineData). A leaf that fits needs no allocation, and a
// parent holds its leaves in its slice of children.

// inlineLeaf is the data of a small leaf, as in SubtreeInlineData.
type inlineLeaf struct {
	paddingBytes, sizeBytes, paddingColumns, paddingRows uint8
	symbol                                               uint8
	visible, named                                       bool
	parseState                                           uint16
}

// subtreeI is a subtree of form A-inline: a pointer, or an inline leaf when
// the pointer is nil.
type subtreeI struct {
	ptr  *heapI
	leaf inlineLeaf
}

// heapI is a node, or a leaf too large to be inline.
type heapI struct {
	fields
	children []subtreeI
}

// padding, size, visible and named read either form, as the SUBTREE_GET
// macro of upstream does.
func (t subtreeI) padding() length {
	if t.ptr != nil {
		return t.ptr.padding
	}
	return length{bytes: uint32(t.leaf.paddingBytes), row: uint32(t.leaf.paddingRows), column: uint32(t.leaf.paddingColumns)}
}

func (t subtreeI) size() length {
	if t.ptr != nil {
		return t.ptr.size
	}
	return length{bytes: uint32(t.leaf.sizeBytes), column: uint32(t.leaf.sizeBytes)}
}

func (t subtreeI) visible() bool {
	if t.ptr != nil {
		return t.ptr.visible
	}
	return t.leaf.visible
}

func (t subtreeI) named() bool {
	if t.ptr != nil {
		return t.ptr.named
	}
	return t.leaf.named
}

// summarizeI does the work of ts_subtree_summarize_children for form
// A-inline.
func summarizeI(t *heapI) {
	t.visibleChildCount, t.namedChildCount, t.visibleDescendantCount = 0, 0, 0
	t.errorCost, t.dynamicPrecedence = 0, 0
	var total length
	for i, c := range t.children {
		if i == 0 {
			t.padding = c.padding()
			total = c.size()
		} else {
			total = total.add(c.padding()).add(c.size())
		}
		if c.ptr != nil {
			t.errorCost += c.ptr.errorCost
			t.dynamicPrecedence += c.ptr.dynamicPrecedence
			t.visibleDescendantCount += c.ptr.visibleDescendantCount
			t.lookaheadBytes = max(t.lookaheadBytes, c.ptr.lookaheadBytes)
		} else {
			t.lookaheadBytes = max(t.lookaheadBytes, 1)
		}
		if c.visible() {
			t.visibleDescendantCount++
			t.visibleChildCount++
			if c.named() {
				t.namedChildCount++
			}
		} else if c.ptr != nil && len(c.ptr.children) > 0 {
			t.visibleChildCount += c.ptr.visibleChildCount
			t.namedChildCount += c.ptr.namedChildCount
		}
	}
	t.size = total
}

// newLeafI returns a leaf of form A-inline, inline when it fits, as
// ts_subtree_new_leaf does.
func newLeafI(tk token) subtreeI {
	if tk.symbol < 256 && tk.padding.bytes < 256 && tk.size.bytes < 256 && tk.padding.row < 16 && tk.padding.column < 256 && tk.size.row == 0 {
		return subtreeI{leaf: inlineLeaf{
			paddingBytes: uint8(tk.padding.bytes), sizeBytes: uint8(tk.size.bytes),
			paddingColumns: uint8(tk.padding.column), paddingRows: uint8(tk.padding.row),
			symbol: uint8(tk.symbol), visible: tk.symbol%3 != 0, named: tk.symbol%2 == 0,
		}}
	}
	return subtreeI{ptr: &heapI{fields: fields{padding: tk.padding, size: tk.size, symbol: tk.symbol, visible: tk.symbol%3 != 0, named: tk.symbol%2 == 0, lookaheadBytes: 1}}}
}

// buildI builds the tree of the tokens and the shape in form A-inline.
func buildI(toks []token, steps []int) subtreeI {
	var stack []subtreeI
	next := 0
	for _, k := range steps {
		if k == 0 {
			stack = append(stack, newLeafI(toks[next]))
			next++
			continue
		}
		children := slices.Clone(stack[len(stack)-k:])
		stack = stack[:len(stack)-k]
		n := &heapI{fields: fields{symbol: uint16(200 + k), visible: k%2 == 0, named: true}, children: children}
		summarizeI(n)
		stack = append(stack, subtreeI{ptr: n})
	}
	return stack[0]
}

// walkI walks the tree in document order, as walkA does.
func walkI(root subtreeI) (int, uint32) {
	type frame struct {
		t     subtreeI
		start uint32
	}
	stack := []frame{{root, 0}}
	visible, end := 0, uint32(0)
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.t.visible() {
			visible++
		}
		end = max(end, f.start+f.t.padding().bytes+f.t.size().bytes)
		if f.t.ptr == nil {
			continue
		}
		off := f.start
		for _, c := range f.t.ptr.children {
			stack = append(stack, frame{c, off})
			off += c.padding().bytes + c.size().bytes
		}
	}
	return visible, end
}

// editI changes the size of the leaf at a byte offset, as editA does. An
// inline leaf is a value, so the change writes the clone of its parent.
func editI(root subtreeI, at, delta uint32) subtreeI {
	clone := *root.ptr
	clone.children = slices.Clone(root.ptr.children)
	cur := &clone
	var path []*heapI
	off := uint32(0)
	for {
		path = append(path, cur)
		i := 0
		for ; i < len(cur.children)-1; i++ {
			c := cur.children[i]
			if off+c.padding().bytes+c.size().bytes > at {
				break
			}
			off += c.padding().bytes + c.size().bytes
		}
		c := cur.children[i]
		if c.ptr == nil {
			c.leaf.sizeBytes += uint8(delta)
			cur.children[i] = c
			break
		}
		child := *c.ptr
		child.children = slices.Clone(c.ptr.children)
		cur.children[i] = subtreeI{ptr: &child}
		if len(child.children) == 0 {
			child.size.bytes += delta
			child.size.column += delta
			break
		}
		cur = &child
	}
	for _, n := range slices.Backward(path) {
		summarizeI(n)
	}
	return subtreeI{ptr: &clone}
}

// Form A-slab: form A, but a parse takes its nodes and its slices of
// children from chunks of a fixed size, so it allocates once for many nodes,
// and nodes that a parse makes together sit together. A chunk never grows,
// so a parse that writes new nodes into it never moves the nodes that an old
// tree reads, and the garbage collector frees a chunk when no tree holds a
// node of it.

// slab gives out nodes and slices of children from chunks.
type slab struct {
	nodes    []subtreeA
	children []*subtreeA
}

// slabChunk is the number of nodes, and of child pointers, of a chunk.
const slabChunk = 512

// node returns a new node from the chunk, and starts a new chunk when the
// chunk is full.
func (s *slab) node(f fields) *subtreeA {
	if len(s.nodes) == cap(s.nodes) {
		s.nodes = make([]subtreeA, 0, slabChunk)
	}
	s.nodes = append(s.nodes, subtreeA{fields: f})
	return &s.nodes[len(s.nodes)-1]
}

// kids returns a slice of k children, copied from src, from the chunk. The
// slice has no room to grow, so an append to it copies it.
func (s *slab) kids(src []*subtreeA) []*subtreeA {
	k := len(src)
	if cap(s.children)-len(s.children) < k {
		s.children = make([]*subtreeA, 0, max(slabChunk, k))
	}
	start := len(s.children)
	s.children = append(s.children, src...)
	return s.children[start : start+k : start+k]
}

// buildS builds the tree of the tokens and the shape in form A-slab.
func buildS(s *slab, toks []token, steps []int) *subtreeA {
	var stack []*subtreeA
	next := 0
	for _, k := range steps {
		if k == 0 {
			tk := toks[next]
			next++
			stack = append(stack, s.node(fields{padding: tk.padding, size: tk.size, symbol: tk.symbol, visible: tk.symbol%3 != 0, named: tk.symbol%2 == 0, lookaheadBytes: 1}))
			continue
		}
		n := s.node(fields{symbol: uint16(200 + k), visible: k%2 == 0, named: true})
		n.children = s.kids(stack[len(stack)-k:])
		stack = stack[:len(stack)-k]
		summarizeA(n)
		stack = append(stack, n)
	}
	return stack[0]
}

// editS changes the size of the leaf at a byte offset, as editA does, with
// the clones from the slab.
func editS(s *slab, root *subtreeA, at, delta uint32) *subtreeA {
	clone := s.node(root.fields)
	clone.children = s.kids(root.children)
	cur := clone
	var path []*subtreeA
	off := uint32(0)
	for len(cur.children) > 0 {
		path = append(path, cur)
		i := 0
		for ; i < len(cur.children)-1; i++ {
			c := cur.children[i]
			if off+c.padding.bytes+c.size.bytes > at {
				break
			}
			off += c.padding.bytes + c.size.bytes
		}
		old := cur.children[i]
		child := s.node(old.fields)
		if len(old.children) > 0 {
			child.children = s.kids(old.children)
		}
		cur.children[i] = child
		cur = child
	}
	cur.size.bytes += delta
	cur.size.column += delta
	for _, n := range slices.Backward(path) {
		summarizeA(n)
	}
	return clone
}

// Form B: an index into slices that the trees share.

type nodeB struct {
	fields
	childStart, childCount uint32
}

// arenaB holds the nodes of every tree that shares it, and the children of
// each node.
type arenaB struct {
	nodes    []nodeB
	children []uint32
}

// summarizeB does the work of ts_subtree_summarize_children for form B.
func (a *arenaB) summarizeB(id uint32) {
	t := &a.nodes[id]
	t.visibleChildCount, t.namedChildCount, t.visibleDescendantCount = 0, 0, 0
	t.errorCost, t.dynamicPrecedence = 0, 0
	var total length
	for i, cid := range a.children[t.childStart : t.childStart+t.childCount] {
		c := &a.nodes[cid]
		if i == 0 {
			t.padding = c.padding
			total = c.size
		} else {
			total = total.add(c.padding).add(c.size)
		}
		t.errorCost += c.errorCost
		t.dynamicPrecedence += c.dynamicPrecedence
		t.visibleDescendantCount += c.visibleDescendantCount
		if c.visible {
			t.visibleDescendantCount++
			t.visibleChildCount++
			if c.named {
				t.namedChildCount++
			}
		} else if c.childCount > 0 {
			t.visibleChildCount += c.visibleChildCount
			t.namedChildCount += c.namedChildCount
		}
		t.lookaheadBytes = max(t.lookaheadBytes, c.lookaheadBytes)
	}
	t.size = total
}

// buildB builds the tree of the tokens and the shape in form B, into a new
// arena, and returns the arena and the root.
func buildB(toks []token, steps []int) (*arenaB, uint32) {
	a := &arenaB{}
	var stack []uint32
	next := 0
	for _, k := range steps {
		if k == 0 {
			tk := toks[next]
			next++
			a.nodes = append(a.nodes, nodeB{fields: fields{padding: tk.padding, size: tk.size, symbol: tk.symbol, visible: tk.symbol%3 != 0, named: tk.symbol%2 == 0, lookaheadBytes: 1}})
			stack = append(stack, uint32(len(a.nodes)-1))
			continue
		}
		start := uint32(len(a.children))
		a.children = append(a.children, stack[len(stack)-k:]...)
		stack = stack[:len(stack)-k]
		a.nodes = append(a.nodes, nodeB{fields: fields{symbol: uint16(200 + k), visible: k%2 == 0, named: true}, childStart: start, childCount: uint32(k)})
		id := uint32(len(a.nodes) - 1)
		a.summarizeB(id)
		stack = append(stack, id)
	}
	return a, stack[0]
}

// walkB walks the tree in document order, as walkA does.
func (a *arenaB) walkB(root uint32) (int, uint32) {
	type frame struct {
		id, start uint32
	}
	stack := []frame{{root, 0}}
	visible, end := 0, uint32(0)
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t := &a.nodes[f.id]
		if t.visible {
			visible++
		}
		end = max(end, f.start+t.padding.bytes+t.size.bytes)
		off := f.start
		for _, cid := range a.children[t.childStart : t.childStart+t.childCount] {
			c := &a.nodes[cid]
			stack = append(stack, frame{cid, off})
			off += c.padding.bytes + c.size.bytes
		}
	}
	return visible, end
}

// editB changes the size of the leaf at a byte offset, as editA does: it
// appends a clone of each node on the path to the arena, and shares every
// other node with the old tree. The old nodes stay in the arena.
func (a *arenaB) editB(root, at, delta uint32) uint32 {
	cloneNode := func(id uint32) uint32 {
		n := a.nodes[id]
		if n.childCount > 0 {
			start := uint32(len(a.children))
			a.children = append(a.children, a.children[n.childStart:n.childStart+n.childCount]...)
			n.childStart = start
		}
		a.nodes = append(a.nodes, n)
		return uint32(len(a.nodes) - 1)
	}
	newRoot := cloneNode(root)
	cur := newRoot
	var path []uint32
	off := uint32(0)
	for a.nodes[cur].childCount > 0 {
		path = append(path, cur)
		n := a.nodes[cur]
		kids := a.children[n.childStart : n.childStart+n.childCount]
		i := 0
		for ; i < len(kids)-1; i++ {
			c := &a.nodes[kids[i]]
			if off+c.padding.bytes+c.size.bytes > at {
				break
			}
			off += c.padding.bytes + c.size.bytes
		}
		child := cloneNode(kids[i])
		a.children[n.childStart+uint32(i)] = child
		cur = child
	}
	a.nodes[cur].size.bytes += delta
	a.nodes[cur].size.column += delta
	for _, id := range slices.Backward(path) {
		a.summarizeB(id)
	}
	return newRoot
}

// compact copies the nodes that root reaches into a new arena, and returns
// the new root. It is the cost that form B pays to drop the nodes of old
// trees.
func (a *arenaB) compact(root uint32) (*arenaB, uint32) {
	out := &arenaB{nodes: make([]nodeB, 0, len(a.nodes)/2), children: make([]uint32, 0, len(a.children)/2)}
	var copyNode func(id uint32) uint32
	copyNode = func(id uint32) uint32 {
		n := a.nodes[id]
		if n.childCount > 0 {
			kids := make([]uint32, n.childCount)
			for i, cid := range a.children[n.childStart : n.childStart+n.childCount] {
				kids[i] = copyNode(cid)
			}
			n.childStart = uint32(len(out.children))
			out.children = append(out.children, kids...)
		}
		out.nodes = append(out.nodes, n)
		return uint32(len(out.nodes) - 1)
	}
	r := copyNode(root)
	return out, r
}

// sizes are the sizes of the benchmark: a line that rline parses on each
// key, and a large source file.
var sizes = []struct {
	name string
	n    int
}{
	{"1k", 1_000},
	{"200k", 200_000},
}

func BenchmarkBuild(b *testing.B) {
	for _, s := range sizes {
		toks, steps := tokens(s.n), shape(s.n)
		b.Run("A/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildA(toks, steps)
			}
		})
		b.Run("A-inline/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildI(toks, steps)
			}
		})
		b.Run("A-slab/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildS(&slab{}, toks, steps)
			}
		})
		b.Run("B/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildB(toks, steps)
			}
		})
	}
}

func BenchmarkWalk(b *testing.B) {
	for _, s := range sizes {
		toks, steps := tokens(s.n), shape(s.n)
		ra := buildA(toks, steps)
		ri := buildI(toks, steps)
		rs := buildS(&slab{}, toks, steps)
		arena, rb := buildB(toks, steps)
		b.Run("A/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				walkA(ra)
			}
		})
		b.Run("A-inline/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				walkI(ri)
			}
		})
		b.Run("A-slab/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				walkA(rs)
			}
		})
		b.Run("B/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				arena.walkB(rb)
			}
		})
	}
}

func BenchmarkEdit(b *testing.B) {
	for _, s := range sizes {
		toks, steps := tokens(s.n), shape(s.n)
		b.Run("A/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			root := buildA(toks, steps)
			total := root.padding.bytes + root.size.bytes
			i := uint32(0)
			for b.Loop() {
				root = editA(root, (i*7919)%total, 1)
				i++
			}
		})
		b.Run("A-inline/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			root := buildI(toks, steps)
			total := root.padding().bytes + root.size().bytes
			i := uint32(0)
			for b.Loop() {
				root = editI(root, (i*7919)%total, 1)
				i++
			}
		})
		b.Run("A-slab/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			sl := &slab{}
			root := buildS(sl, toks, steps)
			total := root.padding.bytes + root.size.bytes
			i := uint32(0)
			for b.Loop() {
				root = editS(sl, root, (i*7919)%total, 1)
				i++
			}
		})
		b.Run("B/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			arena, root := buildB(toks, steps)
			total := arena.nodes[root].padding.bytes + arena.nodes[root].size.bytes
			base := len(arena.nodes)
			i := uint32(0)
			for b.Loop() {
				root = arena.editB(root, (i*7919)%total, 1)
				i++
				// drop the old trees when the arena has doubled
				if len(arena.nodes) > 2*base {
					arena, root = arena.compact(root)
				}
			}
		})
	}
}

// TestSession runs 10,000 edits of each form on each size, keeping only the
// newest tree, as rline does on each key, and reports the time, the pauses
// of the garbage collector and the heap at the end.
func TestSession(t *testing.T) {
	if testing.Short() {
		t.Skip("the session is slow")
	}
	const edits = 10_000
	for _, s := range sizes {
		toks, steps := tokens(s.n), shape(s.n)
		for _, form := range []string{"A", "A-inline", "A-slab", "B"} {
			runtime.GC()
			before := gcPauses()
			start := time.Now()
			switch form {
			case "A":
				root := buildA(toks, steps)
				total := root.padding.bytes + root.size.bytes
				for i := range uint32(edits) {
					root = editA(root, (i*7919)%total, 1)
				}
				runtime.KeepAlive(root)
			case "A-inline":
				root := buildI(toks, steps)
				total := root.padding().bytes + root.size().bytes
				for i := range uint32(edits) {
					root = editI(root, (i*7919)%total, 1)
				}
				runtime.KeepAlive(root)
			case "A-slab":
				sl := &slab{}
				root := buildS(sl, toks, steps)
				total := root.padding.bytes + root.size.bytes
				for i := range uint32(edits) {
					root = editS(sl, root, (i*7919)%total, 1)
				}
				runtime.KeepAlive(root)
			case "B":
				arena, root := buildB(toks, steps)
				total := arena.nodes[root].padding.bytes + arena.nodes[root].size.bytes
				base := len(arena.nodes)
				for i := range uint32(edits) {
					root = arena.editB(root, (i*7919)%total, 1)
					if len(arena.nodes) > 2*base {
						arena, root = arena.compact(root)
					}
				}
				runtime.KeepAlive(arena)
			}
			elapsed := time.Since(start)
			after := gcPauses()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			t.Logf("%-8s %-4s %6d edits in %8.2fms, %6.2fµs each; GC: %4d pauses, max %6.1fµs; heap in use %6.1f MB",
				form, s.name, edits, float64(elapsed.Microseconds())/1000, float64(elapsed.Nanoseconds())/1000/edits,
				after.count-before.count, after.max*1e6, float64(ms.HeapInuse)/1e6)
		}
	}
}

// pauses is the number of pauses of the garbage collector and the longest.
type pauses struct {
	count uint64
	max   float64
}

// gcPauses reads the histogram of the pauses of the garbage collector.
func gcPauses() pauses {
	sample := []metrics.Sample{{Name: "/sched/pauses/total/gc:seconds"}}
	metrics.Read(sample)
	h := sample[0].Value.Float64Histogram()
	var p pauses
	for i, c := range h.Counts {
		p.count += c
		if c > 0 {
			p.max = h.Buckets[i+1]
		}
	}
	return p
}

// TestTheFormsBuildTheSameTree makes sure that the two forms compute the
// same tree, so that the benchmark compares the same work.
func TestTheFormsBuildTheSameTree(t *testing.T) {
	for _, s := range sizes {
		t.Logf("%s: the tree has a height of %d", s.name, depth(buildA(tokens(s.n), shape(s.n))))
	}
	toks, steps := tokens(5_000), shape(5_000)
	ra := buildA(toks, steps)
	arena, rb := buildB(toks, steps)
	va, ea := walkA(ra)
	vb, eb := arena.walkB(rb)
	ri := buildI(toks, steps)
	vi, ei := walkI(ri)
	rs := buildS(&slab{}, toks, steps)
	vs, es := walkA(rs)
	if vi != va || ei != ea || vs != va || es != ea || ri.ptr.visibleDescendantCount != ra.visibleDescendantCount {
		t.Fatalf("the variants of A differ: %d %d, %d %d, %d %d", va, ea, vi, ei, vs, es)
	}
	if editI(ri, 1234, 3).size() != editA(ra, 1234, 3).size || editS(&slab{}, rs, 1234, 3).size != editA(ra, 1234, 3).size {
		t.Fatal("the edits of the variants of A differ")
	}
	if va != vb || ea != eb || ra.visibleDescendantCount != arena.nodes[rb].visibleDescendantCount {
		t.Fatalf("the forms differ: %d %d %d, and %d %d %d", va, ea, ra.visibleDescendantCount, vb, eb, arena.nodes[rb].visibleDescendantCount)
	}
	ra = editA(ra, 1234, 3)
	rb = arena.editB(rb, 1234, 3)
	if ra.size != arena.nodes[rb].size {
		t.Fatalf("the edits differ: %v and %v", ra.size, arena.nodes[rb].size)
	}
	arena2, rb2 := arena.compact(rb)
	if arena2.nodes[rb2].size != arena.nodes[rb].size {
		t.Fatal("the compaction changes the tree")
	}
}
