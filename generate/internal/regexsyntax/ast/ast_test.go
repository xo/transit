package ast

import (
	"testing"
	"unsafe"
)

// This file ports the tests of src/ast/mod.rs.

// TestNoStackOverflowOnDrop is no_stack_overflow_on_drop in mod.rs.
//
// Upstream builds a deep tree on a thread with a small stack, to test that
// the Drop of Ast uses a stack of the same depth for any tree. Go frees a tree
// with the garbage collector and grows the stack of a goroutine, so the test
// builds the same tree and checks it.
func TestNoStackOverflowOnDrop(t *testing.T) {
	t.Parallel()
	span := func() Span { return SplatSpan(NewPosition(0, 0, 0)) }
	var ast Ast = &Empty{Span: span()}
	for i := range uint32(200) {
		ast = &Group{
			Span:  span(),
			Kind:  GroupCaptureIndex,
			Index: i,
			Ast:   ast,
		}
	}
	if IsEmpty(ast) {
		t.Error("the tree is empty")
	}
}

// TestAstSize is ast_size in mod.rs. An Ast is not larger than two words.
func TestAstSize(t *testing.T) {
	t.Parallel()
	maxSize := 2 * unsafe.Sizeof(uintptr(0))
	var ast Ast
	if size := unsafe.Sizeof(ast); size > maxSize {
		t.Errorf("Ast size of %d bytes is bigger than suggested max %d", size, maxSize)
	}
}
