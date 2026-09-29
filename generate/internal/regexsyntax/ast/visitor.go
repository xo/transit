package ast

// This file ports src/ast/visitor.rs: a walk of a syntax tree, in depth first
// order, with a stack on the heap in place of recursion.
//
// A Rust trait with default methods becomes an interface and a struct with
// the default methods, BaseVisitor, which a visitor can embed. The associated
// types of the trait become the type parameter T for the output, and the
// error type of Go for Err.
//
// The port leaves out the Debug of ClassFrame and of ClassInduct, which only
// debugging uses.

// Visitor visits a syntax tree in depth first order. Visit runs it with a
// stack on the heap, so the depth of the Go stack stays the same for any tree.
// The size of a tree follows the size of its pattern, and a pattern can come
// from a user.
//
// A visitor can embed BaseVisitor, which has every method but Finish, and
// does nothing in each.
//
// Visitor is Visitor. The output of the visitor is T.
type Visitor[T any] interface {
	// Finish returns the result of the visit, or an error.
	Finish() (T, error)
	// Start runs before the walk starts.
	Start()
	// VisitPre runs on a tree before the walk goes into its children.
	VisitPre(ast Ast) error
	// VisitPost runs on a tree after the walk visits all its children.
	VisitPost(ast Ast) error
	// VisitAlternationIn runs between two branches of an alternation.
	VisitAlternationIn() error
	// VisitConcatIn runs between two expressions of a concatenation.
	VisitConcatIn() error
	// VisitClassSetItemPre runs on each item of a class before the walk
	// goes into its children.
	VisitClassSetItemPre(ast ClassSetItem) error
	// VisitClassSetItemPost runs on each item of a class after the walk
	// visits all its children.
	VisitClassSetItemPost(ast ClassSetItem) error
	// VisitClassSetBinaryOpPre runs on each set operation before the walk
	// goes into its children.
	VisitClassSetBinaryOpPre(ast *ClassSetBinaryOp) error
	// VisitClassSetBinaryOpPost runs on each set operation after the walk
	// visits all its children.
	VisitClassSetBinaryOpPost(ast *ClassSetBinaryOp) error
	// VisitClassSetBinaryOpIn runs between the left side and the right side
	// of a set operation.
	VisitClassSetBinaryOpIn(ast *ClassSetBinaryOp) error
}

// BaseVisitor has the default methods of Visitor. Each one does nothing and
// returns nil. A visitor embeds it and adds the methods that it needs.
//
// BaseVisitor holds the default methods of the trait Visitor.
type BaseVisitor struct{}

// Start does nothing.
func (BaseVisitor) Start() {}

// VisitPre does nothing.
func (BaseVisitor) VisitPre(Ast) error { return nil }

// VisitPost does nothing.
func (BaseVisitor) VisitPost(Ast) error { return nil }

// VisitAlternationIn does nothing.
func (BaseVisitor) VisitAlternationIn() error { return nil }

// VisitConcatIn does nothing.
func (BaseVisitor) VisitConcatIn() error { return nil }

// VisitClassSetItemPre does nothing.
func (BaseVisitor) VisitClassSetItemPre(ClassSetItem) error { return nil }

// VisitClassSetItemPost does nothing.
func (BaseVisitor) VisitClassSetItemPost(ClassSetItem) error { return nil }

// VisitClassSetBinaryOpPre does nothing.
func (BaseVisitor) VisitClassSetBinaryOpPre(*ClassSetBinaryOp) error { return nil }

// VisitClassSetBinaryOpPost does nothing.
func (BaseVisitor) VisitClassSetBinaryOpPost(*ClassSetBinaryOp) error { return nil }

// VisitClassSetBinaryOpIn does nothing.
func (BaseVisitor) VisitClassSetBinaryOpIn(*ClassSetBinaryOp) error { return nil }

// Visit runs a visitor on every node of a tree, and returns the result of
// Finish. The Go stack stays the same depth, and the heap grows with the size
// of the tree. If a method of the visitor returns an error, the walk stops
// and Visit returns that error.
//
// Visit is visit.
func Visit[T any](ast Ast, visitor Visitor[T]) (T, error) {
	return newHeapVisitor[T]().visit(ast, visitor)
}

// heapVisitor visits every node of a tree with a stack on the heap, in place
// of the Go stack.
//
// heapVisitor is HeapVisitor. The methods of upstream have a type parameter
// for the visitor, and a Go method cannot, so the type has it.
type heapVisitor[T any] struct {
	// stack is a stack of trees, like the call stack of a recursive walk.
	stack []astFrame
	// stackClass is a stack of the same kind for the classes, which have a
	// recursive syntax of their own.
	stackClass []classAstFrame
}

// astFrame is a tree on the stack, with its frame.
type astFrame struct {
	ast   Ast
	frame frame
}

// classAstFrame is a step of a class on the stack, with its frame.
type classAstFrame struct {
	ast   classInduct
	frame classFrame
}

// frameKind is the kind of a frame.
type frameKind uint8

// The kinds of frame, in the order of the enum Frame.
const (
	frameRepetition frameKind = iota
	frameGroup
	frameConcat
	frameAlternation
)

// frame is a frame of the stack in a walk of a tree.
//
// frame is Frame, an enum with data upstream. The fields that a kind uses
// are:
//
//   - frameRepetition: repetition, before the walk goes into its child.
//   - frameGroup: group, before the walk goes into its child.
//   - frameConcat and frameAlternation: head, the child that the walk visits
//     now, and tail, the children that it visits after head. tail can be
//     empty.
type frame struct {
	kind       frameKind
	repetition *Repetition
	group      *Group
	head       Ast
	tail       []Ast
}

// classFrameKind is the kind of a frame of a class.
type classFrameKind uint8

// The kinds of frame of a class, in the order of the enum ClassFrame.
const (
	classFrameUnion classFrameKind = iota
	classFrameBinary
	classFrameBinaryLHS
	classFrameBinaryRHS
)

// classFrame is a frame of the stack in a walk of a class.
//
// classFrame is ClassFrame, an enum with data upstream. The fields that a
// kind uses are:
//
//   - classFrameUnion: head, the item that the walk visits now, and tail,
//     the items that it visits after head. tail can be empty.
//   - classFrameBinary: op, while the walk visits a set operation.
//   - classFrameBinaryLHS: op, lhs and rhs, before the walk goes into the
//     left side of op.
//   - classFrameBinaryRHS: op and rhs, before the walk goes into the right
//     side of op.
type classFrame struct {
	kind classFrameKind
	head ClassSetItem
	tail []ClassSetItem
	op   *ClassSetBinaryOp
	lhs  ClassSet
	rhs  ClassSet
}

// classInduct is one step of the walk of a class: an item or a set
// operation. A ClassSet holds the same two cases, so classInduct is a
// ClassSet. An item cannot be a set operation, because the syntax has no
// union of set operations.
//
// classInduct is ClassInduct. ClassInduct::from_bracketed of a class is its
// field Kind, and ClassInduct::from_set of a set is the set itself.
type classInduct = ClassSet

// newHeapVisitor returns a heapVisitor with empty stacks.
//
// newHeapVisitor is HeapVisitor::new.
func newHeapVisitor[T any]() *heapVisitor[T] {
	return &heapVisitor[T]{}
}

// visit walks ast with visitor.
//
// visit is HeapVisitor::visit.
func (h *heapVisitor[T]) visit(ast Ast, visitor Visitor[T]) (T, error) {
	var zero T
	h.stack = h.stack[:0]
	h.stackClass = h.stackClass[:0]

	visitor.Start()
	for {
		if err := visitor.VisitPre(ast); err != nil {
			return zero, err
		}
		x, ok, err := h.induct(ast, visitor)
		if err != nil {
			return zero, err
		}
		if ok {
			child := x.child()
			h.stack = append(h.stack, astFrame{ast: ast, frame: x})
			ast = child
			continue
		}
		// No induction means that ast is a base case, so the walk can post
		// visit it now.
		if err := visitor.VisitPost(ast); err != nil {
			return zero, err
		}

		// Pop the stack until it is empty or until a frame has another
		// inductive step.
		for {
			if len(h.stack) == 0 {
				return visitor.Finish()
			}
			top := h.stack[len(h.stack)-1]
			h.stack = h.stack[:len(h.stack)-1]
			// A concatenation or an alternation can have more inductive
			// steps.
			if x, ok := h.pop(top.frame); ok {
				switch x.kind {
				case frameAlternation:
					if err := visitor.VisitAlternationIn(); err != nil {
						return zero, err
					}
				case frameConcat:
					if err := visitor.VisitConcatIn(); err != nil {
						return zero, err
					}
				}
				ast = x.child()
				h.stack = append(h.stack, astFrame{ast: top.ast, frame: x})
				break
			}
			// Otherwise the walk visited all the children of this tree, so
			// it can post visit the tree now.
			if err := visitor.VisitPost(top.ast); err != nil {
				return zero, err
			}
		}
	}
}

// induct returns the frame for ast, and true, if ast has children. Otherwise
// it returns false. For a class, it visits the class, and it returns the
// error of the visitor, if there is one.
//
// induct is HeapVisitor::induct.
func (h *heapVisitor[T]) induct(ast Ast, visitor Visitor[T]) (frame, bool, error) {
	switch x := ast.(type) {
	case *ClassBracketed:
		if err := h.visitClass(x, visitor); err != nil {
			return frame{}, false, err
		}
		return frame{}, false, nil
	case *Repetition:
		return frame{kind: frameRepetition, repetition: x}, true, nil
	case *Group:
		return frame{kind: frameGroup, group: x}, true, nil
	case *Concat:
		if len(x.Asts) == 0 {
			return frame{}, false, nil
		}
		return frame{kind: frameConcat, head: x.Asts[0], tail: x.Asts[1:]}, true, nil
	case *Alternation:
		if len(x.Asts) == 0 {
			return frame{}, false, nil
		}
		return frame{kind: frameAlternation, head: x.Asts[0], tail: x.Asts[1:]}, true, nil
	}
	return frame{}, false, nil
}

// pop returns the next inductive step of a frame that the walk pops, and
// true. If the frame has no more steps, pop returns false.
//
// pop is HeapVisitor::pop.
func (h *heapVisitor[T]) pop(induct frame) (frame, bool) {
	switch induct.kind {
	case frameRepetition, frameGroup:
		return frame{}, false
	case frameConcat, frameAlternation:
		if len(induct.tail) == 0 {
			return frame{}, false
		}
		return frame{kind: induct.kind, head: induct.tail[0], tail: induct.tail[1:]}, true
	}
	return frame{}, false
}

// visitClass walks a class in brackets with visitor.
//
// visitClass is HeapVisitor::visit_class.
func (h *heapVisitor[T]) visitClass(ast *ClassBracketed, visitor Visitor[T]) error {
	step := ast.Kind
	for {
		if err := h.visitClassPre(step, visitor); err != nil {
			return err
		}
		if x, ok := h.inductClass(step); ok {
			child := x.child()
			h.stackClass = append(h.stackClass, classAstFrame{ast: step, frame: x})
			step = child
			continue
		}
		if err := h.visitClassPost(step, visitor); err != nil {
			return err
		}

		// Pop the stack until it is empty or until a frame has another
		// inductive step.
		for {
			if len(h.stackClass) == 0 {
				return nil
			}
			top := h.stackClass[len(h.stackClass)-1]
			h.stackClass = h.stackClass[:len(h.stackClass)-1]
			// A union or a set operation can have more inductive steps.
			if x, ok := h.popClass(top.frame); ok {
				if x.kind == classFrameBinaryRHS {
					if err := visitor.VisitClassSetBinaryOpIn(x.op); err != nil {
						return err
					}
				}
				step = x.child()
				h.stackClass = append(h.stackClass, classAstFrame{ast: top.ast, frame: x})
				break
			}
			// Otherwise the walk visited all the children of this node, so
			// it can post visit the node now.
			if err := h.visitClassPost(top.ast, visitor); err != nil {
				return err
			}
		}
	}
}

// visitClassPre calls the pre method of visitor for a step.
//
// visitClassPre is HeapVisitor::visit_class_pre.
func (h *heapVisitor[T]) visitClassPre(ast classInduct, visitor Visitor[T]) error {
	switch x := ast.(type) {
	case ClassSetItem:
		return visitor.VisitClassSetItemPre(x)
	case *ClassSetBinaryOp:
		return visitor.VisitClassSetBinaryOpPre(x)
	}
	return nil
}

// visitClassPost calls the post method of visitor for a step.
//
// visitClassPost is HeapVisitor::visit_class_post.
func (h *heapVisitor[T]) visitClassPost(ast classInduct, visitor Visitor[T]) error {
	switch x := ast.(type) {
	case ClassSetItem:
		return visitor.VisitClassSetItemPost(x)
	case *ClassSetBinaryOp:
		return visitor.VisitClassSetBinaryOpPost(x)
	}
	return nil
}

// inductClass returns the frame for a step of a class, and true, if the step
// has children. Otherwise it returns false.
//
// inductClass is HeapVisitor::induct_class.
func (h *heapVisitor[T]) inductClass(ast classInduct) (classFrame, bool) {
	switch x := ast.(type) {
	case *ClassBracketed:
		switch kind := x.Kind.(type) {
		case ClassSetItem:
			return classFrame{kind: classFrameUnion, head: kind}, true
		case *ClassSetBinaryOp:
			return classFrame{kind: classFrameBinary, op: kind}, true
		}
	case *ClassSetUnion:
		if len(x.Items) == 0 {
			return classFrame{}, false
		}
		return classFrame{kind: classFrameUnion, head: x.Items[0], tail: x.Items[1:]}, true
	case *ClassSetBinaryOp:
		return classFrame{kind: classFrameBinaryLHS, op: x, lhs: x.LHS, rhs: x.RHS}, true
	}
	return classFrame{}, false
}

// popClass returns the next inductive step of a frame of a class that the
// walk pops, and true. If the frame has no more steps, popClass returns
// false.
//
// popClass is HeapVisitor::pop_class.
func (h *heapVisitor[T]) popClass(induct classFrame) (classFrame, bool) {
	switch induct.kind {
	case classFrameUnion:
		if len(induct.tail) == 0 {
			return classFrame{}, false
		}
		return classFrame{kind: classFrameUnion, head: induct.tail[0], tail: induct.tail[1:]}, true
	case classFrameBinary:
		return classFrame{}, false
	case classFrameBinaryLHS:
		return classFrame{kind: classFrameBinaryRHS, op: induct.op, rhs: induct.rhs}, true
	case classFrameBinaryRHS:
		return classFrame{}, false
	}
	return classFrame{}, false
}

// child returns the next child tree that the walk visits for the frame.
//
// child is Frame::child.
func (f frame) child() Ast {
	switch f.kind {
	case frameRepetition:
		return f.repetition.Ast
	case frameGroup:
		return f.group.Ast
	}
	return f.head
}

// child returns the next step of the class that the walk visits for the
// frame.
//
// child is ClassFrame::child.
func (f classFrame) child() classInduct {
	switch f.kind {
	case classFrameUnion:
		return f.head
	case classFrameBinary:
		return f.op
	case classFrameBinaryLHS:
		return f.lhs
	}
	return f.rhs
}
