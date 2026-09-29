package hir

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/xo/transit/generate/internal/regexsyntax/ast"
)

// This file ports src/hir/translate.rs: the translator of a syntax tree to
// an Hir.
//
// Upstream keeps the state of Translator in a Cell or a RefCell, so that a
// method that takes &self can change it. The Go methods change it through a
// pointer. An Either of upstream becomes two results and a bool that tells
// which one holds the value. A panic of upstream, such as a failed assert, is
// a panic here.
//
// The Default of TranslatorBuilder is NewTranslatorBuilder, because the zero
// TranslatorBuilder has utf8 off and a line terminator of 0.

// TranslatorBuilder holds the configuration of a translator.
//
// TranslatorBuilder is TranslatorBuilder.
type TranslatorBuilder struct {
	utf8           bool
	lineTerminator byte
	flags          flags
}

// NewTranslatorBuilder returns a builder with the default configuration.
//
// NewTranslatorBuilder is TranslatorBuilder::new.
func NewTranslatorBuilder() *TranslatorBuilder {
	return &TranslatorBuilder{
		utf8:           true,
		lineTerminator: '\n',
		flags:          flags{},
	}
}

// Build returns a translator with the configuration of b.
//
// Build is TranslatorBuilder::build.
func (b *TranslatorBuilder) Build() *Translator {
	return &Translator{
		stack:          nil,
		flags:          b.flags,
		utf8:           b.utf8,
		lineTerminator: b.lineTerminator,
	}
}

// UTF8 sets whether the translator must only build an expression that
// matches valid UTF-8, and returns b.
//
// When it is off, the translator can build an expression that matches bytes
// that are not valid UTF-8.
//
// When it is on, which is the default, each match of the expression that is
// not empty is valid UTF-8. Otherwise the translator returns an error. An
// empty pattern, or a negated ASCII word boundary, (?-u:\B), is still
// allowed, even though its match can split the UTF-8 encoding of a
// character. This applies only to matches of zero width. The engine must
// handle them, for example by dropping each empty match that splits a
// character.
//
// UTF8 is TranslatorBuilder::utf8.
func (b *TranslatorBuilder) UTF8(yes bool) *TranslatorBuilder {
	b.utf8 = yes
	return b
}

// LineTerminator sets the line terminator for (?u-s:.) and (?-us:.), and
// returns b. The . then matches every character but that byte, in place of
// every character but \n.
//
// If a . occurs in Unicode mode and the byte is not ASCII, the translator
// returns an error. When Unicode mode is off, any byte is allowed, but the
// translator returns an error if UTF-8 mode is on and the byte is not
// ASCII.
//
// If the mode R is on, it wins, and \r and \n are both line terminators.
//
// The line terminator does not change the assertions (?m:^) and (?m:$). The
// engine usually controls them with a configuration of its own.
//
// LineTerminator is TranslatorBuilder::line_terminator.
func (b *TranslatorBuilder) LineTerminator(term byte) *TranslatorBuilder {
	b.lineTerminator = term
	return b
}

// CaseInsensitive sets whether the flag i is on by default, and returns b.
//
// CaseInsensitive is TranslatorBuilder::case_insensitive.
func (b *TranslatorBuilder) CaseInsensitive(yes bool) *TranslatorBuilder {
	b.flags.caseInsensitive = optTrue(yes)
	return b
}

// MultiLine sets whether the flag m is on by default, and returns b.
//
// MultiLine is TranslatorBuilder::multi_line.
func (b *TranslatorBuilder) MultiLine(yes bool) *TranslatorBuilder {
	b.flags.multiLine = optTrue(yes)
	return b
}

// DotMatchesNewLine sets whether the flag s is on by default, and returns b.
//
// DotMatchesNewLine is TranslatorBuilder::dot_matches_new_line.
func (b *TranslatorBuilder) DotMatchesNewLine(yes bool) *TranslatorBuilder {
	b.flags.dotMatchesNewLine = optTrue(yes)
	return b
}

// CRLF sets whether the flag R is on by default, and returns b.
//
// CRLF is TranslatorBuilder::crlf.
func (b *TranslatorBuilder) CRLF(yes bool) *TranslatorBuilder {
	b.flags.crlf = optTrue(yes)
	return b
}

// SwapGreed sets whether the flag U is on by default, and returns b.
//
// SwapGreed is TranslatorBuilder::swap_greed.
func (b *TranslatorBuilder) SwapGreed(yes bool) *TranslatorBuilder {
	b.flags.swapGreed = optTrue(yes)
	return b
}

// Unicode sets whether the flag u is on by default, and returns b.
//
// Unicode is TranslatorBuilder::unicode.
func (b *TranslatorBuilder) Unicode(yes bool) *TranslatorBuilder {
	if yes {
		b.flags.unicode = nil
	} else {
		b.flags.unicode = new(false)
	}
	return b
}

// optTrue returns a pointer to true if yes is true, and nil otherwise. It is
// if yes { Some(true) } else { None } upstream.
func optTrue(yes bool) *bool {
	if yes {
		return new(true)
	}
	return nil
}

// Translator turns a syntax tree into an Hir. A translator can translate
// many syntax trees, one after another. A TranslatorBuilder sets the
// configuration of a translator.
//
// Translator is Translator.
type Translator struct {
	// stack is the call stack, on the heap.
	stack []hirFrame
	// flags are the flags in effect.
	flags flags
	// utf8 is true when the translator must not build an expression that
	// can match bytes that are not valid UTF-8.
	utf8 bool
	// lineTerminator is the line terminator for a . pattern.
	lineTerminator byte
}

// NewTranslator returns a translator with the default configuration.
//
// NewTranslator is Translator::new.
func NewTranslator() *Translator {
	return NewTranslatorBuilder().Build()
}

// Translate translates the syntax tree ast into an Hir. The error is an
// *Error.
//
// pattern must be the pattern that the parser read to make ast. A
// translation that works does not use it. The error uses it.
//
// Translate is Translator::translate.
func (t *Translator) Translate(pattern string, a ast.Ast) (*Hir, error) {
	return ast.Visit[*Hir](a, newTranslatorI(t, pattern))
}

// hirFrameKind is the kind of an hirFrame.
type hirFrameKind uint8

// The kinds of hirFrame, in the order of the enum HirFrame.
const (
	frameExpr hirFrameKind = iota
	frameLiteral
	frameClassUnicode
	frameClassBytes
	frameRepetition
	frameGroup
	frameConcat
	frameAlternation
	frameAlternationBranch
)

// String returns the name of the kind, as upstream names the variant.
func (k hirFrameKind) String() string {
	switch k {
	case frameExpr:
		return "Expr"
	case frameLiteral:
		return "Literal"
	case frameClassUnicode:
		return "ClassUnicode"
	case frameClassBytes:
		return "ClassBytes"
	case frameRepetition:
		return "Repetition"
	case frameGroup:
		return "Group"
	case frameConcat:
		return "Concat"
	case frameAlternation:
		return "Alternation"
	case frameAlternationBranch:
		return "AlternationBranch"
	}
	return ""
}

// hirFrame is one frame of the stack of the translator, for a node of the
// syntax tree that the walk visits. The frame is not the whole state of the
// walk. The visitor of the package ast holds the state of the walk of the
// tree itself.
//
// hirFrame is HirFrame, an enum with data upstream. The kinds, and the fields
// that each kind uses, are:
//
//   - frameExpr: expr, any expression. The translator pushes it at a base
//     case of the tree, and pops it when a step of induction is done.
//   - frameLiteral: literal, a literal that the translator builds one
//     character at a time. The tree has a node for each character. When the
//     translator sees a character, it looks at the frame on top. If it is a
//     literal, it adds the character to it. Otherwise it pushes a new
//     literal. When it pops the frame, NewLiteral turns it into an Hir.
//   - frameClassUnicode: classUnicode, a Unicode class that changes while
//     the translator walks the tree of a class, which is a small recursive
//     structure of its own.
//   - frameClassBytes: classBytes, a class of bytes that changes while the
//     translator walks the tree of a class. The translator makes a class of
//     bytes when the flag u is off. If utf8 is on, which is the default, a
//     class of bytes can only match ASCII.
//   - frameRepetition: a marker that the translator pushes for a repetition.
//     After it visits the sub-expression, the marker must be on top of the
//     stack. It stops other steps, such as the joining of literals, from
//     going across a repetition.
//   - frameGroup: oldFlags, the flags in effect when the group opened. The
//     translator pushes it for any group, and pops it when it leaves the
//     group. If the group sets flags, then the new flags are oldFlags merged
//     with the flags of the group. When the translator pops the group, the
//     flags go back to oldFlags.
//   - frameConcat: a marker that the translator pushes for a
//     concatenation. After it visits each sub-expression, it pops the stack
//     until it sees this marker.
//   - frameAlternation: a marker that the translator pushes for an
//     alternation. After it visits each sub-expression, it pops the stack
//     until it sees this marker.
//   - frameAlternationBranch: a marker that the translator pushes before
//     each branch of an alternation. It separates the branches on the stack,
//     and stops the joining of literals from going across branches. The
//     translator pops it after each expression of a branch, until it sees
//     frameAlternation in the post visit of the alternation.
type hirFrame struct {
	kind         hirFrameKind
	expr         *Hir
	literal      []byte
	classUnicode *ClassUnicode
	classBytes   *ClassBytes
	oldFlags     flags
}

// unwrapExpr returns the expression of the frame. It panics if the frame is
// not an expression or a literal.
//
// unwrapExpr is HirFrame::unwrap_expr.
func (f hirFrame) unwrapExpr() *Hir {
	switch f.kind {
	case frameExpr:
		return f.expr
	case frameLiteral:
		return NewLiteral(f.literal)
	}
	panic(fmt.Sprintf("tried to unwrap expr from HirFrame, got: %s", f.kind))
}

// unwrapClassUnicode returns the Unicode class of the frame. It panics if
// the frame is not a Unicode class.
//
// unwrapClassUnicode is HirFrame::unwrap_class_unicode.
func (f hirFrame) unwrapClassUnicode() *ClassUnicode {
	if f.kind == frameClassUnicode {
		return f.classUnicode
	}
	panic(fmt.Sprintf("tried to unwrap Unicode class from HirFrame, got: %s", f.kind))
}

// unwrapClassBytes returns the class of bytes of the frame. It panics if the
// frame is not a class of bytes.
//
// unwrapClassBytes is HirFrame::unwrap_class_bytes.
func (f hirFrame) unwrapClassBytes() *ClassBytes {
	if f.kind == frameClassBytes {
		return f.classBytes
	}
	panic(fmt.Sprintf("tried to unwrap byte class from HirFrame, got: %s", f.kind))
}

// unwrapRepetition panics if the frame is not the marker of a repetition.
//
// unwrapRepetition is HirFrame::unwrap_repetition.
func (f hirFrame) unwrapRepetition() {
	if f.kind != frameRepetition {
		panic(fmt.Sprintf("tried to unwrap repetition from HirFrame, got: %s", f.kind))
	}
}

// unwrapGroup returns the flags that were in effect when the group opened.
// It panics if the frame is not a group.
//
// unwrapGroup is HirFrame::unwrap_group.
func (f hirFrame) unwrapGroup() flags {
	if f.kind == frameGroup {
		return f.oldFlags
	}
	panic(fmt.Sprintf("tried to unwrap group from HirFrame, got: %s", f.kind))
}

// unwrapAlternationPipe panics if the frame is not the marker of a branch of
// an alternation.
//
// unwrapAlternationPipe is HirFrame::unwrap_alternation_pipe.
func (f hirFrame) unwrapAlternationPipe() {
	if f.kind != frameAlternationBranch {
		panic(fmt.Sprintf("tried to unwrap alt pipe from HirFrame, got: %s", f.kind))
	}
}

// Finish returns the one expression on the stack.
//
// Finish is the Visitor::finish of TranslatorI.
func (t *translatorI) Finish() (*Hir, error) {
	// The stack must hold exactly one expression.
	if n := len(t.trans.stack); n != 1 {
		panic(fmt.Sprintf("assertion `left == right` failed\n  left: %d\n right: 1", n))
	}
	frame, _ := t.pop()
	return frame.unwrapExpr(), nil
}

// VisitPre pushes the frame of a node with children.
//
// VisitPre is the Visitor::visit_pre of TranslatorI.
func (t *translatorI) VisitPre(a ast.Ast) error {
	switch x := a.(type) {
	case *ast.ClassBracketed:
		if t.flags().getUnicode() {
			cls := EmptyClassUnicode()
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			cls := EmptyClassBytes()
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	case *ast.Repetition:
		t.push(hirFrame{kind: frameRepetition})
	case *ast.Group:
		oldFlags := t.flags()
		if f := x.GetFlags(); f != nil {
			oldFlags = t.setFlags(f)
		}
		t.push(hirFrame{kind: frameGroup, oldFlags: oldFlags})
	case *ast.Concat:
		t.push(hirFrame{kind: frameConcat})
	case *ast.Alternation:
		t.push(hirFrame{kind: frameAlternation})
		if len(x.Asts) != 0 {
			t.push(hirFrame{kind: frameAlternationBranch})
		}
	}
	return nil
}

// VisitPost builds the expression of a node, and pushes it.
//
// VisitPost is the Visitor::visit_post of TranslatorI.
func (t *translatorI) VisitPost(a ast.Ast) error {
	switch x := a.(type) {
	case *ast.Empty:
		t.push(hirFrame{kind: frameExpr, expr: NewEmpty()})
	case *ast.SetFlags:
		t.setFlags(&x.Flags)
		// The flags of the tree are directives, not expressions. But a
		// pattern such as ((?i)) can use them, and it needs an expression
		// there. The empty expression is the right choice.
		//
		// A pattern such as (?i)+ is possible too, but the parser rejects
		// it. The parser can allow it in the future, for consistency.
		t.push(hirFrame{kind: frameExpr, expr: NewEmpty()})
	case *ast.Literal:
		ch, b, isByte, err := t.astLiteralToScalar(x)
		if err != nil {
			return err
		}
		if isByte {
			t.pushByte(b)
			break
		}
		expr, err := t.caseFoldChar(x.Span, ch)
		if err != nil {
			return err
		}
		if expr == nil {
			t.pushChar(ch)
		} else {
			t.push(hirFrame{kind: frameExpr, expr: expr})
		}
	case *ast.Dot:
		expr, err := t.hirDot(x.Span)
		if err != nil {
			return err
		}
		t.push(hirFrame{kind: frameExpr, expr: expr})
	case *ast.Assertion:
		t.push(hirFrame{kind: frameExpr, expr: t.hirAssertion(x)})
	case *ast.ClassPerl:
		if t.flags().getUnicode() {
			cls := t.hirPerlUnicodeClass(x)
			t.push(hirFrame{kind: frameExpr, expr: NewClass(cls)})
		} else {
			cls, err := t.hirPerlByteClass(x)
			if err != nil {
				return err
			}
			t.push(hirFrame{kind: frameExpr, expr: NewClass(cls)})
		}
	case *ast.ClassUnicode:
		cls, err := t.hirUnicodeClass(x)
		if err != nil {
			return err
		}
		t.push(hirFrame{kind: frameExpr, expr: NewClass(cls)})
	case *ast.ClassBracketed:
		if t.flags().getUnicode() {
			cls := t.mustPop().unwrapClassUnicode()
			if err := t.unicodeFoldAndNegate(x.Span, x.Negated, cls); err != nil {
				return err
			}
			expr := NewClass(cls)
			t.push(hirFrame{kind: frameExpr, expr: expr})
		} else {
			cls := t.mustPop().unwrapClassBytes()
			if err := t.bytesFoldAndNegate(x.Span, x.Negated, cls); err != nil {
				return err
			}
			expr := NewClass(cls)
			t.push(hirFrame{kind: frameExpr, expr: expr})
		}
	case *ast.Repetition:
		expr := t.mustPop().unwrapExpr()
		t.mustPop().unwrapRepetition()
		t.push(hirFrame{kind: frameExpr, expr: t.hirRepetition(x, expr)})
	case *ast.Group:
		expr := t.mustPop().unwrapExpr()
		oldFlags := t.mustPop().unwrapGroup()
		t.trans.flags = oldFlags
		t.push(hirFrame{kind: frameExpr, expr: t.hirCapture(x, expr)})
	case *ast.Concat:
		var exprs []*Hir
		for {
			expr, ok := t.popConcatExpr()
			if !ok {
				break
			}
			if _, empty := expr.Kind().(*Empty); !empty {
				exprs = append(exprs, expr)
			}
		}
		reverse(exprs)
		t.push(hirFrame{kind: frameExpr, expr: NewConcat(exprs)})
	case *ast.Alternation:
		var exprs []*Hir
		for {
			expr, ok := t.popAltExpr()
			if !ok {
				break
			}
			t.mustPop().unwrapAlternationPipe()
			exprs = append(exprs, expr)
		}
		reverse(exprs)
		t.push(hirFrame{kind: frameExpr, expr: NewAlternation(exprs)})
	}
	return nil
}

// reverse reverses the order of the expressions, in place.
func reverse(exprs []*Hir) {
	for i, j := 0, len(exprs)-1; i < j; i, j = i+1, j-1 {
		exprs[i], exprs[j] = exprs[j], exprs[i]
	}
}

// VisitAlternationIn pushes the marker of a branch of an alternation.
//
// VisitAlternationIn is the Visitor::visit_alternation_in of TranslatorI.
func (t *translatorI) VisitAlternationIn() error {
	t.push(hirFrame{kind: frameAlternationBranch})
	return nil
}

// VisitClassSetItemPre pushes an empty class for a nested class.
//
// VisitClassSetItemPre is the Visitor::visit_class_set_item_pre of
// TranslatorI.
func (t *translatorI) VisitClassSetItemPre(item ast.ClassSetItem) error {
	if _, ok := item.(*ast.ClassBracketed); ok {
		if t.flags().getUnicode() {
			cls := EmptyClassUnicode()
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			cls := EmptyClassBytes()
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	}
	// The code does not need to handle a union here, because the visitor
	// does it.
	return nil
}

// VisitClassSetItemPost adds an item to the class on top of the stack.
//
// VisitClassSetItemPost is the Visitor::visit_class_set_item_post of
// TranslatorI.
func (t *translatorI) VisitClassSetItemPost(item ast.ClassSetItem) error {
	switch x := item.(type) {
	case *ast.Empty:
	case *ast.Literal:
		if t.flags().getUnicode() {
			cls := t.mustPop().unwrapClassUnicode()
			cls.Push(NewClassUnicodeRange(x.C, x.C))
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			cls := t.mustPop().unwrapClassBytes()
			b, err := t.classLiteralByte(x)
			if err != nil {
				return err
			}
			cls.Push(NewClassBytesRange(b, b))
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	case *ast.ClassSetRange:
		if t.flags().getUnicode() {
			cls := t.mustPop().unwrapClassUnicode()
			cls.Push(NewClassUnicodeRange(x.Start.C, x.End.C))
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			cls := t.mustPop().unwrapClassBytes()
			start, err := t.classLiteralByte(&x.Start)
			if err != nil {
				return err
			}
			end, err := t.classLiteralByte(&x.End)
			if err != nil {
				return err
			}
			cls.Push(NewClassBytesRange(start, end))
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	case *ast.ClassASCII:
		if t.flags().getUnicode() {
			xcls, err := t.hirASCIIUnicodeClass(x)
			if err != nil {
				return err
			}
			cls := t.mustPop().unwrapClassUnicode()
			cls.Union(xcls)
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			xcls, err := t.hirASCIIByteClass(x)
			if err != nil {
				return err
			}
			cls := t.mustPop().unwrapClassBytes()
			cls.Union(xcls)
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	case *ast.ClassUnicode:
		xcls, err := t.hirUnicodeClass(x)
		if err != nil {
			return err
		}
		cls := t.mustPop().unwrapClassUnicode()
		cls.Union(xcls)
		t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
	case *ast.ClassPerl:
		if t.flags().getUnicode() {
			xcls := t.hirPerlUnicodeClass(x)
			cls := t.mustPop().unwrapClassUnicode()
			cls.Union(xcls)
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
		} else {
			xcls, err := t.hirPerlByteClass(x)
			if err != nil {
				return err
			}
			cls := t.mustPop().unwrapClassBytes()
			cls.Union(xcls)
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
		}
	case *ast.ClassBracketed:
		if t.flags().getUnicode() {
			cls1 := t.mustPop().unwrapClassUnicode()
			if err := t.unicodeFoldAndNegate(x.Span, x.Negated, cls1); err != nil {
				return err
			}

			cls2 := t.mustPop().unwrapClassUnicode()
			cls2.Union(cls1)
			t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls2})
		} else {
			cls1 := t.mustPop().unwrapClassBytes()
			if err := t.bytesFoldAndNegate(x.Span, x.Negated, cls1); err != nil {
				return err
			}

			cls2 := t.mustPop().unwrapClassBytes()
			cls2.Union(cls1)
			t.push(hirFrame{kind: frameClassBytes, classBytes: cls2})
		}
	// The visitor handles a union.
	case *ast.ClassSetUnion:
	}
	return nil
}

// VisitClassSetBinaryOpPre pushes an empty class for a set operation.
//
// VisitClassSetBinaryOpPre is the Visitor::visit_class_set_binary_op_pre of
// TranslatorI.
func (t *translatorI) VisitClassSetBinaryOpPre(*ast.ClassSetBinaryOp) error {
	if t.flags().getUnicode() {
		cls := EmptyClassUnicode()
		t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
	} else {
		cls := EmptyClassBytes()
		t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
	}
	return nil
}

// VisitClassSetBinaryOpIn pushes an empty class for the right side of a set
// operation.
//
// VisitClassSetBinaryOpIn is the Visitor::visit_class_set_binary_op_in of
// TranslatorI.
func (t *translatorI) VisitClassSetBinaryOpIn(*ast.ClassSetBinaryOp) error {
	if t.flags().getUnicode() {
		cls := EmptyClassUnicode()
		t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
	} else {
		cls := EmptyClassBytes()
		t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
	}
	return nil
}

// VisitClassSetBinaryOpPost applies a set operation to its two sides, and
// adds the result to the class below them.
//
// VisitClassSetBinaryOpPost is the Visitor::visit_class_set_binary_op_post
// of TranslatorI.
func (t *translatorI) VisitClassSetBinaryOpPost(op *ast.ClassSetBinaryOp) error {
	if t.flags().getUnicode() {
		rhs := t.mustPop().unwrapClassUnicode()
		lhs := t.mustPop().unwrapClassUnicode()
		cls := t.mustPop().unwrapClassUnicode()
		if t.flags().getCaseInsensitive() {
			if err := rhs.TryCaseFoldSimple(); err != nil {
				return t.error(op.RHS.GetSpan(), UnicodeCaseUnavailable)
			}
			if err := lhs.TryCaseFoldSimple(); err != nil {
				return t.error(op.LHS.GetSpan(), UnicodeCaseUnavailable)
			}
		}
		switch op.Kind {
		case ast.ClassSetBinaryOpIntersection:
			lhs.Intersect(rhs)
		case ast.ClassSetBinaryOpDifference:
			lhs.Difference(rhs)
		case ast.ClassSetBinaryOpSymmetricDifference:
			lhs.SymmetricDifference(rhs)
		}
		cls.Union(lhs)
		t.push(hirFrame{kind: frameClassUnicode, classUnicode: cls})
	} else {
		rhs := t.mustPop().unwrapClassBytes()
		lhs := t.mustPop().unwrapClassBytes()
		cls := t.mustPop().unwrapClassBytes()
		if t.flags().getCaseInsensitive() {
			rhs.CaseFoldSimple()
			lhs.CaseFoldSimple()
		}
		switch op.Kind {
		case ast.ClassSetBinaryOpIntersection:
			lhs.Intersect(rhs)
		case ast.ClassSetBinaryOpDifference:
			lhs.Difference(rhs)
		case ast.ClassSetBinaryOpSymmetricDifference:
			lhs.SymmetricDifference(rhs)
		}
		cls.Union(lhs)
		t.push(hirFrame{kind: frameClassBytes, classBytes: cls})
	}
	return nil
}

// translatorI is the translator at work on one syntax tree. It holds the
// pattern, which is not tied to the state of a translator. A translatorI
// lives for the translation of one tree.
//
// translatorI is TranslatorI. It is the visitor that Translate runs, and
// it embeds ast.BaseVisitor for the methods of the trait that it does not
// implement.
type translatorI struct {
	ast.BaseVisitor

	// trans is the translator.
	trans *Translator
	// pattern is the pattern of the tree.
	pattern string
}

// newTranslatorI returns the translator at work on the tree of pattern.
//
// newTranslatorI is TranslatorI::new.
func newTranslatorI(trans *Translator, pattern string) *translatorI {
	return &translatorI{trans: trans, pattern: pattern}
}

// push pushes a frame on the call stack.
//
// push is TranslatorI::push.
func (t *translatorI) push(frame hirFrame) {
	t.trans.stack = append(t.trans.stack, frame)
}

// pushChar pushes a literal character on the call stack. If the frame on
// top is a literal, the character goes at its end. Otherwise the code
// pushes a new literal that holds the character.
//
// pushChar is TranslatorI::push_char.
func (t *translatorI) pushChar(ch rune) {
	b := utf8.AppendRune(nil, ch)
	stack := t.trans.stack
	if n := len(stack); n > 0 && stack[n-1].kind == frameLiteral {
		stack[n-1].literal = append(stack[n-1].literal, b...)
	} else {
		t.trans.stack = append(t.trans.stack, hirFrame{kind: frameLiteral, literal: b})
	}
}

// pushByte pushes a literal byte on the call stack. If the frame on top is
// a literal, the byte goes at its end. Otherwise the code pushes a new
// literal that holds the byte.
//
// pushByte is TranslatorI::push_byte.
func (t *translatorI) pushByte(b byte) {
	stack := t.trans.stack
	if n := len(stack); n > 0 && stack[n-1].kind == frameLiteral {
		stack[n-1].literal = append(stack[n-1].literal, b)
	} else {
		t.trans.stack = append(t.trans.stack, hirFrame{kind: frameLiteral, literal: []byte{b}})
	}
}

// pop pops the frame on top of the call stack, and returns true. If the
// stack is empty, it returns false.
//
// pop is TranslatorI::pop.
func (t *translatorI) pop() (hirFrame, bool) {
	n := len(t.trans.stack)
	if n == 0 {
		return hirFrame{}, false
	}
	frame := t.trans.stack[n-1]
	t.trans.stack[n-1] = hirFrame{}
	t.trans.stack = t.trans.stack[:n-1]
	return frame, true
}

// mustPop pops the frame on top of the call stack. It panics if the stack is
// empty.
//
// mustPop is self.pop().unwrap() upstream.
func (t *translatorI) mustPop() hirFrame {
	frame, ok := t.pop()
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return frame
}

// popConcatExpr pops an expression from the top of the stack, for a
// concatenation, and returns true. If the stack is empty, or if the frame is
// the marker of a concatenation, it returns false. Otherwise, if the frame
// is not an expression, it panics.
//
// popConcatExpr is TranslatorI::pop_concat_expr.
func (t *translatorI) popConcatExpr() (*Hir, bool) {
	frame, ok := t.pop()
	if !ok {
		return nil, false
	}
	switch frame.kind {
	case frameConcat:
		return nil, false
	case frameExpr:
		return frame.expr, true
	case frameLiteral:
		return NewLiteral(frame.literal), true
	case frameClassUnicode:
		panic("internal error: entered unreachable code: expected expr or concat, got Unicode class")
	case frameClassBytes:
		panic("internal error: entered unreachable code: expected expr or concat, got byte class")
	case frameRepetition:
		panic("internal error: entered unreachable code: expected expr or concat, got repetition")
	case frameGroup:
		panic("internal error: entered unreachable code: expected expr or concat, got group")
	case frameAlternation:
		panic("internal error: entered unreachable code: expected expr or concat, got alt marker")
	case frameAlternationBranch:
		panic("internal error: entered unreachable code: expected expr or concat, got alt branch marker")
	}
	panic("unknown kind of frame")
}

// popAltExpr pops an expression from the top of the stack, for an
// alternation, and returns true. If the stack is empty, or if the frame is
// the marker of an alternation, it returns false. Otherwise, if the frame
// is not an expression, it panics.
//
// popAltExpr is TranslatorI::pop_alt_expr.
func (t *translatorI) popAltExpr() (*Hir, bool) {
	frame, ok := t.pop()
	if !ok {
		return nil, false
	}
	switch frame.kind {
	case frameAlternation:
		return nil, false
	case frameExpr:
		return frame.expr, true
	case frameLiteral:
		return NewLiteral(frame.literal), true
	case frameClassUnicode:
		panic("internal error: entered unreachable code: expected expr or alt, got Unicode class")
	case frameClassBytes:
		panic("internal error: entered unreachable code: expected expr or alt, got byte class")
	case frameRepetition:
		panic("internal error: entered unreachable code: expected expr or alt, got repetition")
	case frameGroup:
		panic("internal error: entered unreachable code: expected expr or alt, got group")
	case frameConcat:
		panic("internal error: entered unreachable code: expected expr or alt, got concat marker")
	case frameAlternationBranch:
		panic("internal error: entered unreachable code: expected expr or alt, got alt branch marker")
	}
	panic("unknown kind of frame")
}

// error returns a new error with a span and a kind.
//
// error is TranslatorI::error.
func (t *translatorI) error(span ast.Span, kind ErrorKind) *Error {
	return &Error{Kind: kind, Pattern: t.pattern, Span: span}
}

// flags returns a copy of the flags in effect.
//
// flags is TranslatorI::flags.
func (t *translatorI) flags() flags {
	return t.trans.flags
}

// setFlags sets the flags of the translator from the flags of the tree, and
// returns the old flags.
//
// setFlags is TranslatorI::set_flags.
func (t *translatorI) setFlags(astFlags *ast.Flags) flags {
	oldFlags := t.flags()
	newFlags := flagsFromAST(astFlags)
	newFlags.merge(&oldFlags)
	t.trans.flags = newFlags
	return oldFlags
}

// astLiteralToScalar returns the scalar value of a literal of the tree. The
// value is a character, or a byte when the bool is true.
//
// In Unicode mode, it always returns a character, a Unicode scalar value.
//
// When Unicode mode is off, it still returns a character where it can. It
// returns a byte only when invalid UTF-8 is allowed and the byte is not
// ASCII. When invalid UTF-8 is not allowed, a byte that is not ASCII is an
// error.
//
// astLiteralToScalar is TranslatorI::ast_literal_to_scalar, which returns
// Either<char, u8>.
func (t *translatorI) astLiteralToScalar(lit *ast.Literal) (rune, byte, bool, error) {
	if t.flags().getUnicode() {
		return lit.C, 0, false, nil
	}
	b, ok := lit.Byte()
	if !ok {
		return lit.C, 0, false, nil
	}
	if b <= 0x7F {
		return rune(b), 0, false, nil
	}
	if t.trans.utf8 {
		return 0, 0, false, t.error(lit.Span, InvalidUTF8)
	}
	return 0, b, true, nil
}

// caseFoldChar returns a class of c and the characters that it case folds
// with, when the flag i is on and the fold adds a character. Otherwise it
// returns nil.
//
// caseFoldChar is TranslatorI::case_fold_char.
func (t *translatorI) caseFoldChar(span ast.Span, c rune) (*Hir, error) {
	if !t.flags().getCaseInsensitive() {
		return nil, nil
	}
	if t.flags().getUnicode() {
		// If the case fold does nothing, do not try it. Upstream returns
		// UnicodeCaseUnavailable if newSimpleCaseFolder fails, which it
		// never does in the port.
		folder := newSimpleCaseFolder()
		if !folder.overlaps(c, c) {
			return nil, nil
		}
		cls := NewClassUnicode([]ClassUnicodeRange{NewClassUnicodeRange(c, c)})
		if err := cls.TryCaseFoldSimple(); err != nil {
			return nil, t.error(span, UnicodeCaseUnavailable)
		}
		return NewClass(cls), nil
	}
	if c > 0x7F {
		return nil, nil
	}
	// If the case fold does nothing, do not try it.
	switch {
	case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z':
	default:
		return nil, nil
	}
	// The conversion is right, because c is ASCII.
	cls := NewClassBytes([]ClassBytesRange{NewClassBytesRange(byte(c), byte(c))})
	cls.CaseFoldSimple()
	return NewClass(cls), nil
}

// hirDot returns the expression of a . pattern.
//
// hirDot is TranslatorI::hir_dot.
func (t *translatorI) hirDot(span ast.Span) (*Hir, error) {
	utf8Mode, lineterm, f := t.trans.utf8, t.trans.lineTerminator, t.flags()
	if utf8Mode && (!f.getUnicode() || lineterm > 0x7F) {
		return nil, t.error(span, InvalidUTF8)
	}
	var dot Dot
	switch {
	case f.getDotMatchesNewLine():
		if f.getUnicode() {
			dot = Dot{Kind: DotAnyChar}
		} else {
			dot = Dot{Kind: DotAnyByte}
		}
	case f.getUnicode():
		if f.getCRLF() {
			dot = Dot{Kind: DotAnyCharExceptCRLF}
		} else {
			if lineterm > 0x7F {
				return nil, t.error(span, InvalidLineTerminator)
			}
			dot = Dot{Kind: DotAnyCharExcept, Char: rune(lineterm)}
		}
	default:
		if f.getCRLF() {
			dot = Dot{Kind: DotAnyByteExceptCRLF}
		} else {
			dot = Dot{Kind: DotAnyByteExcept, Byte: lineterm}
		}
	}
	return NewDot(dot), nil
}

// hirAssertion returns the expression of an assertion.
//
// hirAssertion is TranslatorI::hir_assertion. Upstream returns a Result that
// is always Ok, and the port drops the error.
func (t *translatorI) hirAssertion(asst *ast.Assertion) *Hir {
	unicode := t.flags().getUnicode()
	multiLine := t.flags().getMultiLine()
	crlf := t.flags().getCRLF()
	pick := func(yes bool, a, b Look) Look {
		if yes {
			return a
		}
		return b
	}
	switch asst.Kind {
	case ast.AssertionStartLine:
		if multiLine {
			return NewLook(pick(crlf, LookStartCRLF, LookStartLF))
		}
		return NewLook(LookStart)
	case ast.AssertionEndLine:
		if multiLine {
			return NewLook(pick(crlf, LookEndCRLF, LookEndLF))
		}
		return NewLook(LookEnd)
	case ast.AssertionStartText:
		return NewLook(LookStart)
	case ast.AssertionEndText:
		return NewLook(LookEnd)
	case ast.AssertionWordBoundary:
		return NewLook(pick(unicode, LookWordUnicode, LookWordASCII))
	case ast.AssertionNotWordBoundary:
		return NewLook(pick(unicode, LookWordUnicodeNegate, LookWordASCIINegate))
	case ast.AssertionWordBoundaryStart, ast.AssertionWordBoundaryStartAngle:
		return NewLook(pick(unicode, LookWordStartUnicode, LookWordStartASCII))
	case ast.AssertionWordBoundaryEnd, ast.AssertionWordBoundaryEndAngle:
		return NewLook(pick(unicode, LookWordEndUnicode, LookWordEndASCII))
	case ast.AssertionWordBoundaryStartHalf:
		return NewLook(pick(unicode, LookWordStartHalfUnicode, LookWordStartHalfASCII))
	case ast.AssertionWordBoundaryEndHalf:
		return NewLook(pick(unicode, LookWordEndHalfUnicode, LookWordEndHalfASCII))
	}
	panic("unknown kind of assertion")
}

// hirCapture returns the expression of a group. A group that does not
// capture is its expression.
//
// hirCapture is TranslatorI::hir_capture.
func (t *translatorI) hirCapture(group *ast.Group, expr *Hir) *Hir {
	var index uint32
	var name string
	switch group.Kind {
	case ast.GroupCaptureIndex:
		index = group.Index
	case ast.GroupCaptureName:
		index, name = group.Name.Index, group.Name.Name
	case ast.GroupNonCapturing:
		// The HIR has no group that does not capture, because the form of
		// its types handles such a group already.
		return expr
	}
	return NewCapture(Capture{Index: index, Name: name, Sub: expr})
}

// hirRepetition returns the expression of a repetition of expr.
//
// hirRepetition is TranslatorI::hir_repetition.
func (t *translatorI) hirRepetition(rep *ast.Repetition, expr *Hir) *Hir {
	var minimum uint32
	var maximum *uint32
	switch rep.Op.Kind {
	case ast.RepetitionZeroOrOne:
		minimum, maximum = 0, new(uint32(1))
	case ast.RepetitionZeroOrMore:
		minimum, maximum = 0, nil
	case ast.RepetitionOneOrMore:
		minimum, maximum = 1, nil
	case ast.RepetitionKindRange:
		switch rep.Op.Range.Kind {
		case ast.RepetitionRangeExactly:
			minimum, maximum = rep.Op.Range.M, new(rep.Op.Range.M)
		case ast.RepetitionRangeAtLeast:
			minimum, maximum = rep.Op.Range.M, nil
		case ast.RepetitionRangeBounded:
			minimum, maximum = rep.Op.Range.M, new(rep.Op.Range.N)
		}
	}
	greedy := rep.Greedy
	if t.flags().getSwapGreed() {
		greedy = !rep.Greedy
	}
	return NewRepetition(Repetition{
		Min:    minimum,
		Max:    maximum,
		Greedy: greedy,
		Sub:    expr,
	})
}

// hirUnicodeClass returns the class of a Unicode class of the tree.
//
// hirUnicodeClass is TranslatorI::hir_unicode_class.
func (t *translatorI) hirUnicodeClass(astClass *ast.ClassUnicode) (*ClassUnicode, error) {
	if !t.flags().getUnicode() {
		return nil, t.error(astClass.Span, UnicodeNotAllowed)
	}
	var query classQuery
	switch astClass.Kind {
	case ast.ClassUnicodeOneLetter:
		query = classQuery{kind: queryOneLetter, letter: astClass.Letter}
	case ast.ClassUnicodeNamed:
		query = classQuery{kind: queryBinary, name: astClass.Name}
	case ast.ClassUnicodeNamedValue:
		query = classQuery{kind: queryByValue, propertyName: astClass.Name, propertyValue: astClass.Value}
	}
	class, err := unicodeClass(query)
	class, err = t.convertUnicodeClassError(astClass.Span, class, err)
	if err == nil {
		if err := t.unicodeFoldAndNegate(astClass.Span, astClass.IsNegated(), class); err != nil {
			return nil, err
		}
	}
	return class, err
}

// hirASCIIUnicodeClass returns the Unicode class of an ASCII class of the
// tree.
//
// hirASCIIUnicodeClass is TranslatorI::hir_ascii_unicode_class.
func (t *translatorI) hirASCIIUnicodeClass(a *ast.ClassASCII) (*ClassUnicode, error) {
	var ranges []ClassUnicodeRange
	for _, r := range asciiClassAsChars(a.Kind) {
		ranges = append(ranges, NewClassUnicodeRange(r[0], r[1]))
	}
	cls := NewClassUnicode(ranges)
	if err := t.unicodeFoldAndNegate(a.Span, a.Negated, cls); err != nil {
		return nil, err
	}
	return cls, nil
}

// hirASCIIByteClass returns the class of bytes of an ASCII class of the
// tree.
//
// hirASCIIByteClass is TranslatorI::hir_ascii_byte_class.
func (t *translatorI) hirASCIIByteClass(a *ast.ClassASCII) (*ClassBytes, error) {
	var ranges []ClassBytesRange
	for _, r := range asciiClass(a.Kind) {
		ranges = append(ranges, NewClassBytesRange(r[0], r[1]))
	}
	cls := NewClassBytes(ranges)
	if err := t.bytesFoldAndNegate(a.Span, a.Negated, cls); err != nil {
		return nil, err
	}
	return cls, nil
}

// hirPerlUnicodeClass returns the Unicode class of a Perl class of the tree.
//
// hirPerlUnicodeClass is TranslatorI::hir_perl_unicode_class. Upstream
// returns a Result, which is always Ok with the default features, and the
// port drops the error.
func (t *translatorI) hirPerlUnicodeClass(astClass *ast.ClassPerl) *ClassUnicode {
	if !t.flags().getUnicode() {
		panic("assertion failed: self.flags().unicode()")
	}
	// Upstream turns the error of a lookup into an error of the translator
	// here. A Perl class never fails in the port, as unicode.go says.
	var class *ClassUnicode
	switch astClass.Kind {
	case ast.ClassPerlDigit:
		class = perlDigit()
	case ast.ClassPerlSpace:
		class = perlSpace()
	case ast.ClassPerlWord:
		class = perlWord()
	}
	// The code does not need a case fold here, because the Unicode Perl
	// classes are closed under the Unicode simple case folding already.
	if astClass.Negated {
		class.Negate()
	}
	return class
}

// hirPerlByteClass returns the class of bytes of a Perl class of the tree.
//
// hirPerlByteClass is TranslatorI::hir_perl_byte_class.
func (t *translatorI) hirPerlByteClass(astClass *ast.ClassPerl) (*ClassBytes, error) {
	if t.flags().getUnicode() {
		panic("assertion failed: !self.flags().unicode()")
	}
	var class *ClassBytes
	switch astClass.Kind {
	case ast.ClassPerlDigit:
		class = hirASCIIClassBytes(ast.ClassASCIIDigit)
	case ast.ClassPerlSpace:
		class = hirASCIIClassBytes(ast.ClassASCIISpace)
	case ast.ClassPerlWord:
		class = hirASCIIClassBytes(ast.ClassASCIIWord)
	}
	// The code does not need a case fold here, because the ASCII Perl
	// classes are closed under the ASCII case folding already.
	if astClass.Negated {
		class.Negate()
	}
	// A negated Perl class of bytes can match invalid UTF-8. That is only
	// allowed if the configuration of the translator allows it.
	if t.trans.utf8 && !class.IsASCII() {
		return nil, t.error(astClass.Span, InvalidUTF8)
	}
	return class, nil
}

// convertUnicodeClassError turns an error of a lookup in the Unicode tables
// into an error of the translator. The span is about where the error occurs.
//
// convertUnicodeClassError is TranslatorI::convert_unicode_class_error.
func (t *translatorI) convertUnicodeClassError(span ast.Span, class *ClassUnicode, err error) (*ClassUnicode, error) {
	if err == nil {
		return class, nil
	}
	var uerr unicodeError
	if !errors.As(err, &uerr) {
		return nil, err
	}
	switch uerr {
	case errPropertyNotFound:
		return nil, t.error(span, UnicodePropertyNotFound)
	case errPropertyValueNotFound:
		return nil, t.error(span, UnicodePropertyValueNotFound)
	case errPerlClassNotFound:
		return nil, t.error(span, UnicodePerlClassNotFound)
	}
	return nil, err
}

// unicodeFoldAndNegate case folds the class if the flag i is on, and then
// negates it if negated is true.
//
// unicodeFoldAndNegate is TranslatorI::unicode_fold_and_negate.
func (t *translatorI) unicodeFoldAndNegate(span ast.Span, negated bool, class *ClassUnicode) error {
	// The case fold must come before the negation. Take (?i)[^x]. If the
	// negation comes first, the result is the class of every Unicode scalar
	// value.
	if t.flags().getCaseInsensitive() {
		if err := class.TryCaseFoldSimple(); err != nil {
			return t.error(span, UnicodeCaseUnavailable)
		}
	}
	if negated {
		class.Negate()
	}
	return nil
}

// bytesFoldAndNegate case folds the class if the flag i is on, and then
// negates it if negated is true. If utf8 is on and the result holds a byte
// that is not ASCII, it returns an error.
//
// bytesFoldAndNegate is TranslatorI::bytes_fold_and_negate.
func (t *translatorI) bytesFoldAndNegate(span ast.Span, negated bool, class *ClassBytes) error {
	// The case fold must come before the negation. Take (?i)[^x]. If the
	// negation comes first, the result is the class of every Unicode scalar
	// value.
	if t.flags().getCaseInsensitive() {
		class.CaseFoldSimple()
	}
	if negated {
		class.Negate()
	}
	if t.trans.utf8 && !class.IsASCII() {
		return t.error(span, InvalidUTF8)
	}
	return nil
}

// classLiteralByte returns the byte of a literal, for a class of bytes.
//
// classLiteralByte is TranslatorI::class_literal_byte.
func (t *translatorI) classLiteralByte(a *ast.Literal) (byte, error) {
	ch, b, isByte, err := t.astLiteralToScalar(a)
	if err != nil {
		return 0, err
	}
	if isByte {
		return b, nil
	}
	if ch <= 0x7F {
		return byte(ch), nil
	}
	// A class of bytes cannot support Unicode in a practical way. It does
	// not do a Unicode case fold.
	return 0, t.error(a.Span, UnicodeNotAllowed)
}

// flags are the flags of a regular expression at one moment of the
// translation. Each flag is absent, present and off, or present and on. A
// nil pointer is absent, which is None upstream.
//
// flags is Flags. The flag x is not here, because the parser handles it.
type flags struct {
	caseInsensitive   *bool
	multiLine         *bool
	dotMatchesNewLine *bool
	swapGreed         *bool
	unicode           *bool
	crlf              *bool
}

// flagsFromAST returns the flags of a group of flags of the tree.
//
// flagsFromAST is Flags::from_ast.
func flagsFromAST(a *ast.Flags) flags {
	var f flags
	enable := true
	for _, item := range a.Items {
		if item.Kind == ast.FlagsItemNegation {
			enable = false
			continue
		}
		switch item.Flag {
		case ast.FlagCaseInsensitive:
			f.caseInsensitive = new(enable)
		case ast.FlagMultiLine:
			f.multiLine = new(enable)
		case ast.FlagDotMatchesNewLine:
			f.dotMatchesNewLine = new(enable)
		case ast.FlagSwapGreed:
			f.swapGreed = new(enable)
		case ast.FlagUnicode:
			f.unicode = new(enable)
		case ast.FlagCRLF:
			f.crlf = new(enable)
		case ast.FlagIgnoreWhitespace:
		}
	}
	return f
}

// merge sets each absent flag of f to the flag of previous.
//
// merge is Flags::merge.
func (f *flags) merge(previous *flags) {
	if f.caseInsensitive == nil {
		f.caseInsensitive = previous.caseInsensitive
	}
	if f.multiLine == nil {
		f.multiLine = previous.multiLine
	}
	if f.dotMatchesNewLine == nil {
		f.dotMatchesNewLine = previous.dotMatchesNewLine
	}
	if f.swapGreed == nil {
		f.swapGreed = previous.swapGreed
	}
	if f.unicode == nil {
		f.unicode = previous.unicode
	}
	if f.crlf == nil {
		f.crlf = previous.crlf
	}
}

// valueOr returns the value of b, or def if b is nil. It is
// Option::unwrap_or.
func valueOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// getCaseInsensitive reports whether the flag i is on. It is off by default.
// The name has the prefix get, because the field has the name without it,
// as in the package ast.
//
// getCaseInsensitive is Flags::case_insensitive.
func (f flags) getCaseInsensitive() bool {
	return valueOr(f.caseInsensitive, false)
}

// getMultiLine reports whether the flag m is on. It is off by default.
//
// getMultiLine is Flags::multi_line.
func (f flags) getMultiLine() bool {
	return valueOr(f.multiLine, false)
}

// getDotMatchesNewLine reports whether the flag s is on. It is off by default.
//
// getDotMatchesNewLine is Flags::dot_matches_new_line.
func (f flags) getDotMatchesNewLine() bool {
	return valueOr(f.dotMatchesNewLine, false)
}

// getSwapGreed reports whether the flag U is on. It is off by default.
//
// getSwapGreed is Flags::swap_greed.
func (f flags) getSwapGreed() bool {
	return valueOr(f.swapGreed, false)
}

// getUnicode reports whether the flag u is on. It is on by default.
//
// getUnicode is Flags::unicode.
func (f flags) getUnicode() bool {
	return valueOr(f.unicode, true)
}

// getCRLF reports whether the flag R is on. It is off by default.
//
// getCRLF is Flags::crlf.
func (f flags) getCRLF() bool {
	return valueOr(f.crlf, false)
}

// hirASCIIClassBytes returns the class of bytes of an ASCII class.
//
// hirASCIIClassBytes is hir_ascii_class_bytes.
func hirASCIIClassBytes(kind ast.ClassASCIIKind) *ClassBytes {
	var ranges []ClassBytesRange
	for _, r := range asciiClass(kind) {
		ranges = append(ranges, NewClassBytesRange(r[0], r[1]))
	}
	return NewClassBytes(ranges)
}

// asciiClass returns the ranges of bytes of an ASCII class. Each range is a
// start and an end, both in the range.
//
// asciiClass is ascii_class.
func asciiClass(kind ast.ClassASCIIKind) [][2]byte {
	switch kind {
	case ast.ClassASCIIAlnum:
		return [][2]byte{{'0', '9'}, {'A', 'Z'}, {'a', 'z'}}
	case ast.ClassASCIIAlpha:
		return [][2]byte{{'A', 'Z'}, {'a', 'z'}}
	case ast.ClassASCIIASCII:
		return [][2]byte{{'\x00', '\x7F'}}
	case ast.ClassASCIIBlank:
		return [][2]byte{{'\t', '\t'}, {' ', ' '}}
	case ast.ClassASCIICntrl:
		return [][2]byte{{'\x00', '\x1F'}, {'\x7F', '\x7F'}}
	case ast.ClassASCIIDigit:
		return [][2]byte{{'0', '9'}}
	case ast.ClassASCIIGraph:
		return [][2]byte{{'!', '~'}}
	case ast.ClassASCIILower:
		return [][2]byte{{'a', 'z'}}
	case ast.ClassASCIIPrint:
		return [][2]byte{{' ', '~'}}
	case ast.ClassASCIIPunct:
		return [][2]byte{{'!', '/'}, {':', '@'}, {'[', '`'}, {'{', '~'}}
	case ast.ClassASCIISpace:
		return [][2]byte{
			{'\t', '\t'},
			{'\n', '\n'},
			{'\x0B', '\x0B'},
			{'\x0C', '\x0C'},
			{'\r', '\r'},
			{' ', ' '},
		}
	case ast.ClassASCIIUpper:
		return [][2]byte{{'A', 'Z'}}
	case ast.ClassASCIIWord:
		return [][2]byte{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
	case ast.ClassASCIIXdigit:
		return [][2]byte{{'0', '9'}, {'A', 'F'}, {'a', 'f'}}
	}
	panic("unknown kind of ASCII class")
}

// asciiClassAsChars returns the ranges of an ASCII class as characters.
//
// asciiClassAsChars is ascii_class_as_chars.
func asciiClassAsChars(kind ast.ClassASCIIKind) [][2]rune {
	bs := asciiClass(kind)
	rs := make([][2]rune, 0, len(bs))
	for _, r := range bs {
		rs = append(rs, [2]rune{rune(r[0]), rune(r[1])})
	}
	return rs
}
