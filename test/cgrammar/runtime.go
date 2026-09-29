package cgrammar

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/xo/transit"
)

// Snapshot is a node of a tree and its children, as the C runtime or the Go
// runtime gives it. Two runtimes that give equal snapshots for a text give
// the same tree.
type Snapshot struct {
	Fields

	Children []Snapshot
}

// Fields is what a snapshot holds of one node.
type Fields struct {
	Symbol         uint16
	GrammarSymbol  uint16
	Field          string
	StartByte      uint32
	EndByte        uint32
	StartPoint     [2]uint32
	EndPoint       [2]uint32
	Named          bool
	Missing        bool
	Extra          bool
	HasError       bool
	HasChanges     bool
	ParseState     uint16
	NextParseState uint16
	Descendants    uint32
}

// CParse parses src with the C runtime and returns the snapshot of the tree
// and the text of ts_node_string.
func (g *Grammar) CParse(src []byte) (Snapshot, string, error) {
	tree, err := g.cParse(src, nil)
	if err != nil {
		return Snapshot{}, "", err
	}
	defer C.rt_tree_delete(tree.tree)
	defer C.rt_parser_delete(tree.parser)
	root := C.rt_tree_root_node(tree.tree)
	return cSnapshot(root, ""), cString(root), nil
}

// cTree is a tree of the C runtime and the parser that made it.
type cTree struct {
	parser unsafe.Pointer
	tree   unsafe.Pointer
}

// cParse parses src with the C runtime, with the old tree old or none.
func (g *Grammar) cParse(src []byte, old unsafe.Pointer) (cTree, error) {
	if !runtimeLoaded {
		return cTree{}, errNoRuntime
	}
	parser := C.rt_parser_new()
	if !C.rt_parser_set_language(parser, g.lang) {
		C.rt_parser_delete(parser)
		return cTree{}, fmt.Errorf("setting the language %s in the C runtime", g.Name)
	}
	var text *C.char
	if len(src) > 0 {
		text = (*C.char)(C.CBytes(src))
		defer C.free(unsafe.Pointer(text))
	}
	tree := C.rt_parser_parse_string(parser, old, text, C.uint32_t(len(src)))
	if tree == nil {
		C.rt_parser_delete(parser)
		return cTree{}, fmt.Errorf("parsing with the C runtime and %s: no tree", g.Name)
	}
	return cTree{parser: parser, tree: tree}, nil
}

// cString returns the text of ts_node_string of a node.
func cString(n C.CNode) string {
	s := C.rt_node_string(n)
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}

// cSnapshot returns the snapshot of a node of the C runtime.
func cSnapshot(n C.CNode, field string) Snapshot {
	start, end := C.rt_node_start_point(n), C.rt_node_end_point(n)
	s := Snapshot{
		Symbol:         uint16(C.rt_node_symbol(n)),
		GrammarSymbol:  uint16(C.rt_node_grammar_symbol(n)),
		Field:          field,
		StartByte:      uint32(C.rt_node_start_byte(n)),
		EndByte:        uint32(C.rt_node_end_byte(n)),
		StartPoint:     [2]uint32{uint32(start.row), uint32(start.column)},
		EndPoint:       [2]uint32{uint32(end.row), uint32(end.column)},
		Named:          bool(C.rt_node_is_named(n)),
		Missing:        bool(C.rt_node_is_missing(n)),
		Extra:          bool(C.rt_node_is_extra(n)),
		HasError:       bool(C.rt_node_has_error(n)),
		HasChanges:     bool(C.rt_node_has_changes(n)),
		ParseState:     uint16(C.rt_node_parse_state(n)),
		NextParseState: uint16(C.rt_node_next_parse_state(n)),
		Descendants:    uint32(C.rt_node_descendant_count(n)),
	}
	for i := range uint32(C.rt_node_child_count(n)) {
		var name string
		if f := C.rt_node_field_name_for_child(n, C.uint32_t(i)); f != nil {
			name = C.GoString(f)
		}
		s.Children = append(s.Children, cSnapshot(C.rt_node_child(n, C.uint32_t(i)), name))
	}
	return s
}

// GoSnapshot returns the snapshot of a node of the Go runtime.
func GoSnapshot(n transit.Node, field string) Snapshot {
	start, end := n.StartPoint(), n.EndPoint()
	s := Snapshot{
		Symbol:         uint16(n.KindID()),
		GrammarSymbol:  uint16(n.GrammarID()),
		Field:          field,
		StartByte:      uint32(n.StartByte()),
		EndByte:        uint32(n.EndByte()),
		StartPoint:     [2]uint32{uint32(start.Row), uint32(start.Column)},
		EndPoint:       [2]uint32{uint32(end.Row), uint32(end.Column)},
		Named:          n.IsNamed(),
		Missing:        n.IsMissing(),
		Extra:          n.IsExtra(),
		HasError:       n.HasError(),
		HasChanges:     n.HasChanges(),
		ParseState:     uint16(n.ParseState()),
		NextParseState: uint16(n.NextParseState()),
		Descendants:    uint32(n.DescendantCount()),
	}
	for i := range n.ChildCount() {
		child, ok := n.Child(i)
		if !ok {
			break
		}
		s.Children = append(s.Children, GoSnapshot(child, n.FieldNameForChild(i)))
	}
	return s
}

// Diff returns the first difference of two snapshots, as a path of child
// indices and the two nodes, or "" when they are equal.
func Diff(a, b Snapshot) string {
	return diff(a, b, "root")
}

// diff is Diff at the path path.
func diff(a, b Snapshot, path string) string {
	if a.Fields != b.Fields {
		return fmt.Sprintf("%s:\n  C:  %+v\n  Go: %+v", path, a.Fields, b.Fields)
	}
	if len(a.Children) != len(b.Children) {
		return fmt.Sprintf("%s: %d children in C and %d in Go", path, len(a.Children), len(b.Children))
	}
	for i := range a.Children {
		if d := diff(a.Children[i], b.Children[i], fmt.Sprintf("%s/%d", path, i)); d != "" {
			return d
		}
	}
	return ""
}

// String returns the snapshot as an indented list of nodes, for a message.
func (s Snapshot) String() string {
	var b strings.Builder
	s.write(&b, 0)
	return b.String()
}

// write writes the snapshot to b at a depth.
func (s Snapshot) write(b *strings.Builder, depth int) {
	fmt.Fprintf(b, "%s%d %s [%d-%d]\n", strings.Repeat("  ", depth), s.Symbol, s.Field, s.StartByte, s.EndByte)
	for _, c := range s.Children {
		c.write(b, depth+1)
	}
}

// CSession is a parser and a tree of the C runtime, for the tests of an
// edit and a parse with the old tree.
type CSession struct {
	g    *Grammar
	tree cTree
}

// NewCSession parses src with the C runtime.
func (g *Grammar) NewCSession(src []byte) (*CSession, error) {
	tree, err := g.cParse(src, nil)
	if err != nil {
		return nil, err
	}
	return &CSession{g: g, tree: tree}, nil
}

// Edit edits the tree of the session, as ts_tree_edit does.
func (s *CSession) Edit(e transit.InputEdit) {
	edit := C.CInputEdit{
		start_byte:    C.uint32_t(e.StartByte),
		old_end_byte:  C.uint32_t(e.OldEndByte),
		new_end_byte:  C.uint32_t(e.NewEndByte),
		start_point:   cPoint(e.StartPoint),
		old_end_point: cPoint(e.OldEndPoint),
		new_end_point: cPoint(e.NewEndPoint),
	}
	C.rt_tree_edit(s.tree.tree, &edit)
}

// cPoint returns the C form of a point.
func cPoint(p transit.Point) C.CPoint {
	return C.CPoint{row: C.uint32_t(p.Row), column: C.uint32_t(p.Column)}
}

// Reparse parses src with the edited tree of the session as the old tree,
// keeps the new tree, and returns the ranges that changed, as
// ts_tree_get_changed_ranges gives them.
func (s *CSession) Reparse(src []byte) ([]transit.Range, error) {
	var text *C.char
	if len(src) > 0 {
		text = (*C.char)(C.CBytes(src))
		defer C.free(unsafe.Pointer(text))
	}
	tree := C.rt_parser_parse_string(s.tree.parser, s.tree.tree, text, C.uint32_t(len(src)))
	if tree == nil {
		return nil, fmt.Errorf("parsing again with the C runtime and %s: no tree", s.g.Name)
	}
	var n C.uint32_t
	ranges := C.rt_tree_get_changed_ranges(s.tree.tree, tree, &n)
	var out []transit.Range
	for _, r := range array(ranges, int(n)) {
		out = append(out, transit.Range{
			StartByte:  int(r.start_byte),
			EndByte:    int(r.end_byte),
			StartPoint: transit.Point{Row: int(r.start_point.row), Column: int(r.start_point.column)},
			EndPoint:   transit.Point{Row: int(r.end_point.row), Column: int(r.end_point.column)},
		})
	}
	C.free(unsafe.Pointer(ranges))
	C.rt_tree_delete(s.tree.tree)
	s.tree.tree = tree
	return out, nil
}

// Snapshot returns the snapshot of the tree of the session, and the text of
// ts_node_string.
func (s *CSession) Snapshot() (Snapshot, string) {
	root := C.rt_tree_root_node(s.tree.tree)
	return cSnapshot(root, ""), cString(root)
}

// Close frees the parser and the tree of the session.
func (s *CSession) Close() {
	C.rt_tree_delete(s.tree.tree)
	C.rt_parser_delete(s.tree.parser)
}
