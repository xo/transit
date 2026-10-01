package cgrammar

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/xo/transit"
)

// SpeedSession is a parser of the C runtime and some texts in C memory, for
// the benchmarks of the speed targets of D37. It makes the parser and copies
// the texts once, so that a benchmark measures only the work of the C
// runtime. CSession makes a parser for each first parse, copies the text for
// each parse and gets the changed ranges after each parse.
//
// A session of a grammar that LoadO2 opened uses the runtime that
// LoadRuntimeO2 opened, which is built with -O2 (D92). A session of a grammar
// that Load opened uses the runtime of the tests.
type SpeedSession struct {
	g      *Grammar
	o2     C.bool
	parser unsafe.Pointer
	tree   unsafe.Pointer
	texts  []*C.char
	sizes  []int
}

// NewSpeedSession makes a parser of the C runtime for the grammar, and copies
// the texts to C memory. The session has no tree until the first call of
// Parse.
func (g *Grammar) NewSpeedSession(texts ...[]byte) (*SpeedSession, error) {
	if (g.o2 && !runtimeO2Loaded) || (!g.o2 && !runtimeLoaded) {
		return nil, errNoRuntime
	}
	o2 := C.bool(g.o2)
	parser := C.sp_parser_new(o2)
	if !C.sp_parser_set_language(o2, parser, g.lang) {
		C.sp_parser_delete(o2, parser)
		return nil, fmt.Errorf("setting the language %s in the C runtime", g.Name)
	}
	s := &SpeedSession{g: g, o2: o2, parser: parser}
	for _, text := range texts {
		s.texts = append(s.texts, (*C.char)(C.CBytes(text)))
		s.sizes = append(s.sizes, len(text))
	}
	return s, nil
}

// Parse parses the text with the index i. When old is true, it parses with
// the tree of the session as the old tree. It keeps the new tree and deletes
// the tree before it.
func (s *SpeedSession) Parse(i int, old bool) error {
	var oldTree unsafe.Pointer
	if old {
		oldTree = s.tree
	}
	tree := C.sp_parser_parse_string(s.o2, s.parser, oldTree, s.texts[i], C.uint32_t(s.sizes[i]))
	if tree == nil {
		return fmt.Errorf("parsing with the C runtime and %s: no tree", s.g.Name)
	}
	if s.tree != nil {
		C.sp_tree_delete(s.o2, s.tree)
	}
	s.tree = tree
	return nil
}

// Edit edits the tree of the session, as ts_tree_edit does.
func (s *SpeedSession) Edit(e transit.InputEdit) {
	edit := C.CInputEdit{
		start_byte:    C.uint32_t(e.StartByte),
		old_end_byte:  C.uint32_t(e.OldEndByte),
		new_end_byte:  C.uint32_t(e.NewEndByte),
		start_point:   cPoint(e.StartPoint),
		old_end_point: cPoint(e.OldEndPoint),
		new_end_point: cPoint(e.NewEndPoint),
	}
	C.sp_tree_edit(s.o2, s.tree, &edit)
}

// Snapshot returns the snapshot of the tree of the session. It reads the
// tree with the runtime of the tests, so the grammar of the session must be
// one that Load opened.
func (s *SpeedSession) Snapshot() Snapshot {
	return cSnapshot(C.rt_tree_root_node(s.tree), "")
}

// Close frees the parser, the tree and the texts of the session.
func (s *SpeedSession) Close() {
	if s.tree != nil {
		C.sp_tree_delete(s.o2, s.tree)
	}
	C.sp_parser_delete(s.o2, s.parser)
	for _, text := range s.texts {
		C.free(unsafe.Pointer(text))
	}
}
