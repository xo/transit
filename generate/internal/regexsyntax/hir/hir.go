// Package hir is the high level intermediate form (the HIR) of a regular
// expression, and the translator that makes it from a syntax tree. It ports
// the module hir of the Rust crate regex-syntax 0.8.11, with the files
// unicode.rs and parser.rs of the crate, which the generator of tree-sitter
// uses to read the pattern of a token (D59).
//
// The package ast parses a pattern into a syntax tree. The translator of this
// package turns the tree into an Hir. The constructors of Hir, such as
// NewConcat and NewAlternation, make the Hir simpler as they build it, and
// the translator uses them too. So an Hir is rarely the same shape as its
// pattern. The Parser of this package does both steps.
package hir

import (
	"bytes"
	"encoding/binary"
	"iter"
	"math"
	"math/bits"
	"slices"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/xo/transit/generate/internal/regexsyntax/ast"
)

// This file ports src/hir/mod.rs: the types of the HIR, their constructors,
// the properties of an Hir, and the error of the translator.
//
// A Rust enum with data becomes one of two Go forms, as in the package ast:
//
//   - HirKind becomes the interface Kind, and Class becomes the interface
//     Class, which is a Kind. Each variant is a pointer: *Empty, *Literal,
//     *ClassUnicode, *ClassBytes, *Look, *Repetition, *Capture, *Concat and
//     *Alternation. Only the pointer implements Kind, so a type switch on a
//     value type, such as case Look, does not compile.
//   - A small enum, such as ErrorKind or Dot, becomes a typed constant. The
//     data of a variant is a field of the struct that holds the kind.
//
// An Option of upstream becomes a pointer, such as the field Max of
// Repetition, or a result with a bool, such as Properties.MinimumLen. The
// field Name of Capture is empty for None, because a capture name cannot be
// empty. A Rust usize becomes an int. A saturated or checked sum or product
// of lengths saturates or fails at math.MaxInt, not at the largest usize.
//
// A value of the HIR does not change after a constructor returns it. Two
// Hir values can share a sub-expression. A caller that takes a Kind with
// IntoKind can change it, as upstream can after into_kind, and must not use
// the Hir after that.
//
// The port leaves out these items of upstream:
//
//   - The Display of Error, which calls the Formatter of error.rs.
//     tree-sitter writes the text of an error itself. Error returns the
//     Display of the kind only, as the package ast does.
//   - The Display of Hir, which calls print.rs, and the Debug of Hir,
//     HirKind, Literal, Class, ClassUnicodeRange and ClassBytesRange.
//   - The Drop of Hir. The garbage collector frees an Hir.
//   - The modules literal and visitor, and the function visit and the trait
//     Visitor that this module exports from visitor.rs. The generator does
//     not call them.
//
// ClassUnicodeIter, ClassBytesIter and LookSetIter become an iter.Seq
// (D25).
//
// Upstream returns a CaseFoldError from a case fold when the feature
// unicode-case is off. The port follows the default features, which turn it
// on, so a case fold never fails. The methods of the port that upstream
// exports keep the error in the signature, such as TryCaseFoldSimple, and the
// others drop it.

// Error is an error of the translator. The text of the error is the Display
// of its kind.
//
// Error is Error. Error::kind, Error::pattern and Error::span are the
// fields.
type Error struct {
	// Kind is the kind of the error.
	Kind ErrorKind
	// Pattern is the pattern of the syntax tree. The span of the error is a
	// valid range of this string.
	Pattern string
	// Span is the span of the error.
	Span ast.Span
}

// Error returns the text of the kind of the error, as the Display of upstream
// ErrorKind writes it. The text does not hold the pattern or the span.
//
// Error is the Display of ErrorKind.
func (e *Error) Error() string {
	return e.Kind.String()
}

// ErrorKind is the kind of an error of the translator.
//
// ErrorKind is ErrorKind.
type ErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	// UnicodeNotAllowed is a Unicode class or a Unicode literal where
	// Unicode mode is off.
	UnicodeNotAllowed ErrorKind = iota
	// InvalidUTF8 is an expression that can match bytes that are not valid
	// UTF-8, when the translator must not allow it.
	InvalidUTF8
	// InvalidLineTerminator is a line terminator that is not ASCII, for a .
	// in Unicode mode.
	InvalidLineTerminator
	// UnicodePropertyNotFound is the name of a Unicode property that the
	// tables do not hold.
	UnicodePropertyNotFound
	// UnicodePropertyValueNotFound is a value of a Unicode property that the
	// tables do not hold.
	UnicodePropertyValueNotFound
	// UnicodePerlClassNotFound is a Unicode Perl class, such as \w, when its
	// table is not available. The port always has the table.
	UnicodePerlClassNotFound
	// UnicodeCaseUnavailable is a case insensitive match in Unicode mode,
	// when the table of case folding is not available. The port always has
	// the table.
	UnicodeCaseUnavailable
)

// String returns the text of the kind.
//
// String is the Display of ErrorKind.
func (k ErrorKind) String() string {
	switch k {
	case UnicodeNotAllowed:
		return "Unicode not allowed here"
	case InvalidUTF8:
		return "pattern can match invalid UTF-8"
	case InvalidLineTerminator:
		return "invalid line terminator, must be ASCII"
	case UnicodePropertyNotFound:
		return "Unicode property not found"
	case UnicodePropertyValueNotFound:
		return "Unicode property value not found"
	case UnicodePerlClassNotFound:
		return "Unicode-aware Perl class not found (make sure the unicode-perl feature is enabled)"
	case UnicodeCaseUnavailable:
		return "Unicode-aware case insensitivity matching is not available (make sure the unicode-case feature is enabled)"
	}
	return ""
}

// Hir is the high level intermediate form of a regular expression: a tree of
// kinds, and the properties of each node. A constructor, such as NewConcat,
// builds an Hir and computes its properties.
//
// Hir is Hir.
type Hir struct {
	// kind is the kind of the expression.
	kind Kind
	// props are the properties of the expression.
	props Properties
}

// Kind returns the kind of the expression.
//
// Kind is Hir::kind.
func (h *Hir) Kind() Kind {
	return h.kind
}

// IntoKind returns the kind of the expression, for a caller that takes it
// apart. The caller must not use h after the call.
//
// IntoKind is Hir::into_kind.
func (h *Hir) IntoKind() Kind {
	return h.kind
}

// Properties returns the properties of the expression. The caller must not
// change them.
//
// Properties is Hir::properties.
func (h *Hir) Properties() *Properties {
	return &h.props
}

// intoParts returns the kind and the properties of the expression.
//
// intoParts is Hir::into_parts.
func (h *Hir) intoParts() (Kind, Properties) {
	return h.kind, h.props
}

// Equal reports whether two expressions have the same kind and the same
// properties, with each sub-expression compared in the same way.
//
// Equal is the derived PartialEq of Hir.
func (h *Hir) Equal(other *Hir) bool {
	return kindEqual(h.kind, other.kind) && h.props == other.props
}

// NewEmpty returns an expression that matches only the empty string.
//
// NewEmpty is Hir::empty.
func NewEmpty() *Hir {
	props := emptyProperties()
	return &Hir{kind: &Empty{}, props: props}
}

// NewFail returns an expression that never matches. It is an empty class of
// bytes.
//
// NewFail is Hir::fail.
func NewFail() *Hir {
	class := EmptyClassBytes()
	props := classProperties(class)
	// The code cannot call NewClass here, because NewClass calls NewFail to
	// give one canonical form to the Hir that cannot match.
	return &Hir{kind: class, props: props}
}

// NewLiteral returns an expression that matches the bytes of lit. If lit is
// empty, it returns NewEmpty. The expression holds a copy of lit.
//
// NewLiteral is Hir::literal.
func NewLiteral(lit []byte) *Hir {
	if len(lit) == 0 {
		return NewEmpty()
	}

	l := Literal(slices.Clone(lit))
	props := literalProperties(l)
	return &Hir{kind: &l, props: props}
}

// NewClass returns an expression that matches one character of class. If the
// class is empty, it returns NewFail. If the class holds one character, it
// returns the literal of that character.
//
// NewClass is Hir::class.
func NewClass(class Class) *Hir {
	if class.IsEmpty() {
		return NewFail()
	} else if b, ok := class.Literal(); ok {
		return NewLiteral(b)
	}
	props := classProperties(class)
	return &Hir{kind: class, props: props}
}

// NewLook returns an expression of the assertion look.
//
// NewLook is Hir::look.
func NewLook(look Look) *Hir {
	props := lookProperties(look)
	return &Hir{kind: &look, props: props}
}

// NewRepetition returns an expression of the repetition rep. It returns the
// empty expression for a repetition of at most 0 times, and the
// sub-expression for a repetition of exactly 1 time.
//
// NewRepetition is Hir::repetition.
func NewRepetition(rep Repetition) *Hir {
	// If the sub-expression can only match the empty string, then its
	// maximum is at most 1.
	if n, ok := rep.Sub.Properties().MaximumLen(); ok && n == 0 {
		rep.Min = min(rep.Min, 1)
		m := uint32(1)
		if rep.Max != nil {
			m = min(*rep.Max, 1)
		}
		rep.Max = &m
	}
	// The pattern a{0} is always the same as the empty pattern. This is true
	// even when a is an expression that never matches, such as \P{any}.
	//
	// And the pattern a{1} is always the same as a.
	if rep.Min == 0 && rep.Max != nil && *rep.Max == 0 {
		return NewEmpty()
	} else if rep.Min == 1 && rep.Max != nil && *rep.Max == 1 {
		return rep.Sub
	}
	props := repetitionProperties(&rep)
	return &Hir{kind: &rep, props: props}
}

// NewCapture returns an expression of the capture group capture.
//
// NewCapture is Hir::capture.
func NewCapture(capture Capture) *Hir {
	props := captureProperties(&capture)
	return &Hir{kind: &capture, props: props}
}

// NewConcat returns the concatenation of subs, in order. It makes the
// concatenation simpler: it joins literals that are next to each other,
// takes the parts of a concatenation into this one, and drops each empty
// expression. With no expression left, it returns NewEmpty, and with one, it
// returns that expression.
//
// NewConcat is Hir::concat.
func NewConcat(subs []*Hir) *Hir {
	// The code builds a new concatenation that is simpler. Upstream finds it
	// tricky to do in place.
	var nw []*Hir
	// This joins literals that are next to each other. When the code finds
	// a literal, it adds its bytes to priorLit. When it finds anything else,
	// it first adds the bytes of priorLit to the new concatenation.
	var priorLit []byte
	hasPriorLit := false
	for _, sub := range subs {
		kind, props := sub.intoParts()
		switch k := kind.(type) {
		case *Literal:
			if hasPriorLit {
				priorLit = append(priorLit, *k...)
			} else {
				priorLit = slices.Clone(*k)
				hasPriorLit = true
			}
		// The code also takes in the parts of a concatenation that is a
		// direct child of this one. One level is enough, because NewConcat
		// is the only way to build a concatenation, so each level did this
		// when it was built.
		case *Concat:
			for _, sub2 := range *k {
				kind2, props2 := sub2.intoParts()
				switch k2 := kind2.(type) {
				case *Literal:
					if hasPriorLit {
						priorLit = append(priorLit, *k2...)
					} else {
						priorLit = slices.Clone(*k2)
						hasPriorLit = true
					}
				default:
					if hasPriorLit {
						nw = append(nw, NewLiteral(priorLit))
						priorLit, hasPriorLit = nil, false
					}
					nw = append(nw, &Hir{kind: kind2, props: props2})
				}
			}
		// The code skips an empty expression.
		case *Empty:
		default:
			if hasPriorLit {
				nw = append(nw, NewLiteral(priorLit))
				priorLit, hasPriorLit = nil, false
			}
			nw = append(nw, &Hir{kind: kind, props: props})
		}
	}
	if hasPriorLit {
		nw = append(nw, NewLiteral(priorLit))
	}
	if len(nw) == 0 {
		return NewEmpty()
	} else if len(nw) == 1 {
		return nw[0]
	}
	props := concatProperties(nw)
	c := Concat(nw)
	return &Hir{kind: &c, props: props}
}

// NewAlternation returns the alternation of subs, in order of preference. It
// makes the alternation simpler: it takes the branches of an alternation into
// this one, joins branches of single characters or classes into one class,
// and lifts a common prefix of the branches out. With no branch, it returns
// NewFail, and with one, it returns that branch.
//
// NewAlternation is Hir::alternation.
func NewAlternation(subs []*Hir) *Hir {
	// The code builds a new alternation that is simpler, as NewConcat does.
	// Here it does not join literals. It only takes in the branches of an
	// alternation.
	nw := make([]*Hir, 0, len(subs))
	for _, sub := range subs {
		kind, props := sub.intoParts()
		switch k := kind.(type) {
		case *Alternation:
			nw = append(nw, *k...)
		default:
			nw = append(nw, &Hir{kind: kind, props: props})
		}
	}
	if len(nw) == 0 {
		return NewFail()
	} else if len(nw) == 1 {
		return nw[0]
	}
	// The alternation is flat now. Look for the special case of
	// char1|char2|...|charN, and make it one class. The code looks for
	// characters first and then for bytes. If the branches hold characters
	// that are not ASCII and single bytes that are not ASCII, then no one
	// class can hold them, because a class holds only characters or only
	// bytes. So the code looks for all characters and then for all bytes,
	// and does nothing else.
	if singletons, ok := singletonChars(nw); ok {
		ranges := make([]ClassUnicodeRange, 0, len(singletons))
		for _, ch := range singletons {
			ranges = append(ranges, ClassUnicodeRange{start: ch, end: ch})
		}
		return NewClass(NewClassUnicode(ranges))
	}
	if singletons, ok := singletonBytes(nw); ok {
		ranges := make([]ClassBytesRange, 0, len(singletons))
		for _, b := range singletons {
			ranges = append(ranges, ClassBytesRange{start: b, end: b})
		}
		return NewClass(NewClassBytes(ranges))
	}
	// In the same way, an alternation of classes can become one class.
	if cls, ok := classChars(nw); ok {
		return NewClass(cls)
	}
	if cls, ok := classBytes(nw); ok {
		return NewClass(cls)
	}
	// Lift a common prefix out, if there is one. This can make the
	// expression simpler and open the way to more simplifications. It can
	// also make an NFA or a DFA faster, because the branches are shorter.
	lifted, unchanged := liftCommonPrefix(nw)
	if lifted != nil {
		return lifted
	}
	nw = unchanged
	props := alternationProperties(nw)
	a := Alternation(nw)
	return &Hir{kind: &a, props: props}
}

// NewDot returns an expression of the class that dot names.
//
// NewDot is Hir::dot.
func NewDot(dot Dot) *Hir {
	switch dot.Kind {
	case DotAnyChar:
		return NewClass(NewClassUnicode([]ClassUnicodeRange{
			NewClassUnicodeRange(0, maxRune),
		}))
	case DotAnyByte:
		return NewClass(NewClassBytes([]ClassBytesRange{
			NewClassBytesRange(0, 0xFF),
		}))
	case DotAnyCharExcept:
		cls := NewClassUnicode([]ClassUnicodeRange{NewClassUnicodeRange(dot.Char, dot.Char)})
		cls.Negate()
		return NewClass(cls)
	case DotAnyCharExceptLF:
		return NewClass(NewClassUnicode([]ClassUnicodeRange{
			NewClassUnicodeRange(0, '\x09'),
			NewClassUnicodeRange('\x0B', maxRune),
		}))
	case DotAnyCharExceptCRLF:
		return NewClass(NewClassUnicode([]ClassUnicodeRange{
			NewClassUnicodeRange(0, '\x09'),
			NewClassUnicodeRange('\x0B', '\x0C'),
			NewClassUnicodeRange('\x0E', maxRune),
		}))
	case DotAnyByteExcept:
		cls := NewClassBytes([]ClassBytesRange{NewClassBytesRange(dot.Byte, dot.Byte)})
		cls.Negate()
		return NewClass(cls)
	case DotAnyByteExceptLF:
		return NewClass(NewClassBytes([]ClassBytesRange{
			NewClassBytesRange(0, '\x09'),
			NewClassBytesRange('\x0B', 0xFF),
		}))
	case DotAnyByteExceptCRLF:
		return NewClass(NewClassBytes([]ClassBytesRange{
			NewClassBytesRange(0, '\x09'),
			NewClassBytesRange('\x0B', '\x0C'),
			NewClassBytesRange('\x0E', 0xFF),
		}))
	}
	panic("unknown kind of dot")
}

// Kind is the kind of an expression of the HIR, with its data. It is one of
// *Empty, *Literal, *ClassUnicode, *ClassBytes, *Look, *Repetition,
// *Capture, *Concat and *Alternation.
//
// Kind is HirKind.
type Kind interface {
	// Subs returns the sub-expressions of the kind, which can be none.
	//
	// Subs is HirKind::subs.
	Subs() []*Hir
	isKind()
}

// Empty is the empty expression, which matches everywhere, including between
// two characters of the UTF-8 encoding of one character.
//
// Empty is HirKind::Empty.
type Empty struct{}

// Concat is a concatenation of expressions. It holds two expressions or
// more, and none of them is a concatenation or an empty expression.
//
// Concat is HirKind::Concat.
type Concat []*Hir

// Alternation is an alternation of expressions, in order of preference. It
// holds two expressions or more, and none of them is an alternation.
//
// Alternation is HirKind::Alternation.
type Alternation []*Hir

// Subs returns nil.
func (*Empty) Subs() []*Hir { return nil }

// Subs returns nil.
func (*Literal) Subs() []*Hir { return nil }

// Subs returns nil.
func (*ClassUnicode) Subs() []*Hir { return nil }

// Subs returns nil.
func (*ClassBytes) Subs() []*Hir { return nil }

// Subs returns nil.
func (*Look) Subs() []*Hir { return nil }

// Subs returns the sub-expression.
func (r *Repetition) Subs() []*Hir { return []*Hir{r.Sub} }

// Subs returns the sub-expression.
func (c *Capture) Subs() []*Hir { return []*Hir{c.Sub} }

// Subs returns the expressions of the concatenation.
func (c *Concat) Subs() []*Hir { return *c }

// Subs returns the branches of the alternation.
func (a *Alternation) Subs() []*Hir { return *a }

// isKind marks *Empty as a Kind.
func (*Empty) isKind() {}

// isKind marks *Literal as a Kind.
func (*Literal) isKind() {}

// isKind marks *ClassUnicode as a Kind.
func (*ClassUnicode) isKind() {}

// isKind marks *ClassBytes as a Kind.
func (*ClassBytes) isKind() {}

// isKind marks *Look as a Kind.
func (*Look) isKind() {}

// isKind marks *Repetition as a Kind.
func (*Repetition) isKind() {}

// isKind marks *Capture as a Kind.
func (*Capture) isKind() {}

// isKind marks *Concat as a Kind.
func (*Concat) isKind() {}

// isKind marks *Alternation as a Kind.
func (*Alternation) isKind() {}

// kindEqual reports whether two kinds are equal, with each sub-expression
// compared by Hir.Equal.
//
// kindEqual is the derived PartialEq of HirKind.
func kindEqual(a, b Kind) bool {
	switch x := a.(type) {
	case *Empty:
		_, ok := b.(*Empty)
		return ok
	case *Literal:
		y, ok := b.(*Literal)
		return ok && bytes.Equal(*x, *y)
	case *ClassUnicode:
		y, ok := b.(*ClassUnicode)
		return ok && x.Equal(y)
	case *ClassBytes:
		y, ok := b.(*ClassBytes)
		return ok && x.Equal(y)
	case *Look:
		y, ok := b.(*Look)
		return ok && *x == *y
	case *Repetition:
		y, ok := b.(*Repetition)
		return ok && x.Min == y.Min && optU32Equal(x.Max, y.Max) &&
			x.Greedy == y.Greedy && x.Sub.Equal(y.Sub)
	case *Capture:
		y, ok := b.(*Capture)
		return ok && x.Index == y.Index && x.Name == y.Name && x.Sub.Equal(y.Sub)
	case *Concat:
		y, ok := b.(*Concat)
		return ok && slices.EqualFunc(*x, *y, (*Hir).Equal)
	case *Alternation:
		y, ok := b.(*Alternation)
		return ok && slices.EqualFunc(*x, *y, (*Hir).Equal)
	}
	return false
}

// optU32Equal reports whether two optional values are equal: both nil, or
// both set to the same value.
func optU32Equal(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// Literal is a literal string of bytes. It is never empty. In Unicode mode,
// it is valid UTF-8. Otherwise it can hold any bytes.
//
// Literal is Literal.
type Literal []byte

// Class is a class of characters or of bytes. It is one of *ClassUnicode and
// *ClassBytes. A class can be empty, but NewClass never makes an expression of
// an empty class: it returns NewFail.
//
// Class is Class.
type Class interface {
	Kind
	// CaseFoldSimple adds to the class the simple case folds of each of its
	// characters. It panics if the tables for case folding are not
	// available.
	CaseFoldSimple()
	// TryCaseFoldSimple adds to the class the simple case folds of each of
	// its characters. It returns an error if the tables for case folding are
	// not available.
	TryCaseFoldSimple() error
	// Negate changes the class to its complement.
	Negate()
	// IsUTF8 reports whether the class can only match valid UTF-8.
	IsUTF8() bool
	// MinimumLen returns the length in bytes of the shortest string that the
	// class matches, and true. For an empty class, it returns false.
	MinimumLen() (int, bool)
	// MaximumLen returns the length in bytes of the longest string that the
	// class matches, and true. For an empty class, it returns false.
	MaximumLen() (int, bool)
	// IsEmpty reports whether the class is empty.
	IsEmpty() bool
	// Literal returns the bytes of the one character of the class, and true.
	// If the class does not hold exactly one character, it returns false.
	Literal() ([]byte, bool)
}

// ClassUnicode is a set of Unicode scalar values, as ranges in canonical
// order.
//
// ClassUnicode is ClassUnicode, and the variant Class::Unicode.
type ClassUnicode struct {
	set intervalSet[ClassUnicodeRange, rune]
}

// NewClassUnicode returns a class of the ranges. The ranges can be in any
// order, and they can overlap. The class holds a copy of the slice.
//
// NewClassUnicode is ClassUnicode::new.
func NewClassUnicode(ranges []ClassUnicodeRange) *ClassUnicode {
	return &ClassUnicode{set: newIntervalSet(ranges)}
}

// EmptyClassUnicode returns an empty class, which matches no character.
//
// EmptyClassUnicode is ClassUnicode::empty.
func EmptyClassUnicode() *ClassUnicode {
	return NewClassUnicode(nil)
}

// Clone returns a copy of the class that shares no memory with it.
//
// Clone is the Clone of ClassUnicode.
func (c *ClassUnicode) Clone() *ClassUnicode {
	return &ClassUnicode{set: c.set.clone()}
}

// Equal reports whether the two classes hold the same ranges.
//
// Equal is the derived PartialEq of ClassUnicode.
func (c *ClassUnicode) Equal(other *ClassUnicode) bool {
	return c.set.equal(&other.set)
}

// Push adds a range to the class.
//
// Push is ClassUnicode::push.
func (c *ClassUnicode) Push(r ClassUnicodeRange) {
	c.set.push(r)
}

// Iter returns the ranges of the class, in canonical order.
//
// Iter is ClassUnicode::iter.
func (c *ClassUnicode) Iter() iter.Seq[ClassUnicodeRange] {
	return slices.Values(c.set.intervals())
}

// Ranges returns the ranges of the class, in canonical order. The caller must
// not change the slice.
//
// Ranges is ClassUnicode::ranges.
func (c *ClassUnicode) Ranges() []ClassUnicodeRange {
	return c.set.intervals()
}

// CaseFoldSimple adds to the class the simple case folds of each of its
// characters. For example, the class [a-z] becomes [A-Za-z\u017F\u212A].
//
// CaseFoldSimple is ClassUnicode::case_fold_simple. Upstream panics if the
// tables for case folding are not available. The port always has them.
func (c *ClassUnicode) CaseFoldSimple() {
	c.set.caseFoldSimple()
}

// TryCaseFoldSimple adds to the class the simple case folds of each of its
// characters, and returns nil.
//
// TryCaseFoldSimple is ClassUnicode::try_case_fold_simple. Upstream returns
// an error if the tables for case folding are not available. The port always
// has them, but it keeps the error of upstream in the signature.
func (c *ClassUnicode) TryCaseFoldSimple() error {
	c.set.caseFoldSimple()
	return nil
}

// Negate changes the class to its complement.
//
// Negate is ClassUnicode::negate.
func (c *ClassUnicode) Negate() {
	c.set.negate()
}

// Union adds the ranges of other to the class.
//
// Union is ClassUnicode::union.
func (c *ClassUnicode) Union(other *ClassUnicode) {
	c.set.union(&other.set)
}

// Intersect keeps only the characters of the class that other holds too.
//
// Intersect is ClassUnicode::intersect.
func (c *ClassUnicode) Intersect(other *ClassUnicode) {
	c.set.intersect(&other.set)
}

// Difference removes the characters of other from the class.
//
// Difference is ClassUnicode::difference.
func (c *ClassUnicode) Difference(other *ClassUnicode) {
	c.set.difference(&other.set)
}

// SymmetricDifference changes the class to the characters that are in one
// of the two classes and not in both.
//
// SymmetricDifference is ClassUnicode::symmetric_difference.
func (c *ClassUnicode) SymmetricDifference(other *ClassUnicode) {
	c.set.symmetricDifference(&other.set)
}

// IsASCII reports whether the class holds only ASCII characters. An empty
// class holds only ASCII characters.
//
// IsASCII is ClassUnicode::is_ascii.
func (c *ClassUnicode) IsASCII() bool {
	rs := c.set.intervals()
	if len(rs) == 0 {
		return true
	}
	return rs[len(rs)-1].end <= 0x7F
}

// IsUTF8 returns true, because a class of characters only matches valid
// UTF-8.
//
// IsUTF8 is Class::is_utf8 for Class::Unicode.
func (c *ClassUnicode) IsUTF8() bool {
	return true
}

// MinimumLen returns the length in bytes of the UTF-8 encoding of the
// shortest character of the class, and true. For an empty class, it returns
// false.
//
// MinimumLen is ClassUnicode::minimum_len.
func (c *ClassUnicode) MinimumLen() (int, bool) {
	rs := c.Ranges()
	if len(rs) == 0 {
		return 0, false
	}
	// This is right, because c1 < c2 means that c1 is not longer than c2 in
	// UTF-8.
	return utf8.RuneLen(rs[0].start), true
}

// MaximumLen returns the length in bytes of the UTF-8 encoding of the
// longest character of the class, and true. For an empty class, it returns
// false.
//
// MaximumLen is ClassUnicode::maximum_len.
func (c *ClassUnicode) MaximumLen() (int, bool) {
	rs := c.Ranges()
	if len(rs) == 0 {
		return 0, false
	}
	// This is right, because c1 < c2 means that c1 is not longer than c2 in
	// UTF-8.
	return utf8.RuneLen(rs[len(rs)-1].end), true
}

// IsEmpty reports whether the class holds no character.
//
// IsEmpty is Class::is_empty for Class::Unicode.
func (c *ClassUnicode) IsEmpty() bool {
	return len(c.Ranges()) == 0
}

// Literal returns the UTF-8 encoding of the one character of the class, and
// true. If the class does not hold exactly one character, it returns false.
//
// Literal is ClassUnicode::literal.
func (c *ClassUnicode) Literal() ([]byte, bool) {
	rs := c.Ranges()
	if len(rs) == 1 && rs[0].start == rs[0].end {
		return utf8.AppendRune(nil, rs[0].start), true
	}
	return nil, false
}

// ToByteClass returns the class as a class of bytes, if the class holds only
// ASCII characters. Otherwise it returns nil.
//
// ToByteClass is ClassUnicode::to_byte_class.
func (c *ClassUnicode) ToByteClass() *ClassBytes {
	if !c.IsASCII() {
		return nil
	}
	rs := c.Ranges()
	ranges := make([]ClassBytesRange, 0, len(rs))
	for _, r := range rs {
		// The range is ASCII, so each bound fits in a byte.
		ranges = append(ranges, ClassBytesRange{start: byte(r.start), end: byte(r.end)})
	}
	return NewClassBytes(ranges)
}

// ClassUnicodeRange is a range of Unicode scalar values, with both ends in
// the range. The start is never after the end.
//
// ClassUnicodeRange is ClassUnicodeRange.
type ClassUnicodeRange struct {
	start rune
	end   rune
}

// lower returns the start.
//
// lower is Interval::lower.
func (r ClassUnicodeRange) lower() rune { return r.start }

// upper returns the end.
//
// upper is Interval::upper.
func (r ClassUnicodeRange) upper() rune { return r.end }

// setLower returns the range with the start b.
//
// setLower is Interval::set_lower.
func (r ClassUnicodeRange) setLower(b rune) ClassUnicodeRange {
	r.start = b
	return r
}

// setUpper returns the range with the end b.
//
// setUpper is Interval::set_upper.
func (r ClassUnicodeRange) setUpper(b rune) ClassUnicodeRange {
	r.end = b
	return r
}

// caseFoldSimple appends to ranges a range for each simple case fold of
// each character of the range, and returns the slice.
//
// caseFoldSimple is Interval::case_fold_simple.
func (r ClassUnicodeRange) caseFoldSimple(ranges []ClassUnicodeRange) []ClassUnicodeRange {
	folder := newSimpleCaseFolder()
	if !folder.overlaps(r.start, r.end) {
		return ranges
	}
	for cp := r.start; cp <= r.end; cp++ {
		if !isScalar(cp) {
			continue
		}
		for _, cpFolded := range folder.mapping(cp) {
			ranges = append(ranges, NewClassUnicodeRange(cpFolded, cpFolded))
		}
	}
	return ranges
}

// NewClassUnicodeRange returns the range from start to end. If start is
// after end, it swaps them.
//
// NewClassUnicodeRange is ClassUnicodeRange::new.
func NewClassUnicodeRange(start, end rune) ClassUnicodeRange {
	return intervalCreate[ClassUnicodeRange](start, end)
}

// Start returns the start of the range.
//
// Start is ClassUnicodeRange::start.
func (r ClassUnicodeRange) Start() rune {
	return r.start
}

// End returns the end of the range.
//
// End is ClassUnicodeRange::end.
func (r ClassUnicodeRange) End() rune {
	return r.end
}

// Len returns the number of code points in the range, surrogates included.
//
// Len is ClassUnicodeRange::len.
func (r ClassUnicodeRange) Len() int {
	diff := 1 + uint32(r.end) - uint32(r.start)
	return int(diff)
}

// ClassBytes is a set of bytes, as ranges in canonical order.
//
// ClassBytes is ClassBytes, and the variant Class::Bytes.
type ClassBytes struct {
	set intervalSet[ClassBytesRange, byte]
}

// NewClassBytes returns a class of the ranges. The ranges can be in any
// order, and they can overlap. The class holds a copy of the slice.
//
// NewClassBytes is ClassBytes::new.
func NewClassBytes(ranges []ClassBytesRange) *ClassBytes {
	return &ClassBytes{set: newIntervalSet(ranges)}
}

// EmptyClassBytes returns an empty class, which matches no byte.
//
// EmptyClassBytes is ClassBytes::empty.
func EmptyClassBytes() *ClassBytes {
	return NewClassBytes(nil)
}

// Clone returns a copy of the class that shares no memory with it.
//
// Clone is the Clone of ClassBytes.
func (c *ClassBytes) Clone() *ClassBytes {
	return &ClassBytes{set: c.set.clone()}
}

// Equal reports whether the two classes hold the same ranges.
//
// Equal is the derived PartialEq of ClassBytes.
func (c *ClassBytes) Equal(other *ClassBytes) bool {
	return c.set.equal(&other.set)
}

// Push adds a range to the class.
//
// Push is ClassBytes::push.
func (c *ClassBytes) Push(r ClassBytesRange) {
	c.set.push(r)
}

// Iter returns the ranges of the class, in canonical order.
//
// Iter is ClassBytes::iter.
func (c *ClassBytes) Iter() iter.Seq[ClassBytesRange] {
	return slices.Values(c.set.intervals())
}

// Ranges returns the ranges of the class, in canonical order. The caller must
// not change the slice.
//
// Ranges is ClassBytes::ranges.
func (c *ClassBytes) Ranges() []ClassBytesRange {
	return c.set.intervals()
}

// CaseFoldSimple adds to the class the ASCII case folds of each of its
// bytes. For example, the class [a-z] becomes [A-Za-z]. Only ASCII letters
// change.
//
// CaseFoldSimple is ClassBytes::case_fold_simple.
func (c *ClassBytes) CaseFoldSimple() {
	c.set.caseFoldSimple()
}

// TryCaseFoldSimple adds to the class the ASCII case folds of each of its
// bytes, and returns nil.
//
// TryCaseFoldSimple is Class::try_case_fold_simple for Class::Bytes, which
// calls ClassBytes::case_fold_simple. ClassBytes has no method of this name
// upstream.
func (c *ClassBytes) TryCaseFoldSimple() error {
	c.CaseFoldSimple()
	return nil
}

// Negate changes the class to its complement.
//
// Negate is ClassBytes::negate.
func (c *ClassBytes) Negate() {
	c.set.negate()
}

// Union adds the ranges of other to the class.
//
// Union is ClassBytes::union.
func (c *ClassBytes) Union(other *ClassBytes) {
	c.set.union(&other.set)
}

// Intersect keeps only the bytes of the class that other holds too.
//
// Intersect is ClassBytes::intersect.
func (c *ClassBytes) Intersect(other *ClassBytes) {
	c.set.intersect(&other.set)
}

// Difference removes the bytes of other from the class.
//
// Difference is ClassBytes::difference.
func (c *ClassBytes) Difference(other *ClassBytes) {
	c.set.difference(&other.set)
}

// SymmetricDifference changes the class to the bytes that are in one of the
// two classes and not in both.
//
// SymmetricDifference is ClassBytes::symmetric_difference.
func (c *ClassBytes) SymmetricDifference(other *ClassBytes) {
	c.set.symmetricDifference(&other.set)
}

// IsASCII reports whether the class holds only ASCII bytes. An empty class
// holds only ASCII bytes.
//
// IsASCII is ClassBytes::is_ascii.
func (c *ClassBytes) IsASCII() bool {
	rs := c.set.intervals()
	if len(rs) == 0 {
		return true
	}
	return rs[len(rs)-1].end <= 0x7F
}

// IsUTF8 reports whether the class holds only ASCII bytes, because a byte
// that is not ASCII is not valid UTF-8 on its own.
//
// IsUTF8 is Class::is_utf8 for Class::Bytes.
func (c *ClassBytes) IsUTF8() bool {
	return c.IsASCII()
}

// MinimumLen returns 1 and true. For an empty class, it returns false.
//
// MinimumLen is ClassBytes::minimum_len.
func (c *ClassBytes) MinimumLen() (int, bool) {
	if len(c.Ranges()) == 0 {
		return 0, false
	}
	return 1, true
}

// MaximumLen returns 1 and true. For an empty class, it returns false.
//
// MaximumLen is ClassBytes::maximum_len.
func (c *ClassBytes) MaximumLen() (int, bool) {
	if len(c.Ranges()) == 0 {
		return 0, false
	}
	return 1, true
}

// IsEmpty reports whether the class holds no byte.
//
// IsEmpty is Class::is_empty for Class::Bytes.
func (c *ClassBytes) IsEmpty() bool {
	return len(c.Ranges()) == 0
}

// Literal returns the one byte of the class, and true. If the class does not
// hold exactly one byte, it returns false.
//
// Literal is ClassBytes::literal.
func (c *ClassBytes) Literal() ([]byte, bool) {
	rs := c.Ranges()
	if len(rs) == 1 && rs[0].start == rs[0].end {
		return []byte{rs[0].start}, true
	}
	return nil, false
}

// ToUnicodeClass returns the class as a class of characters, if the class
// holds only ASCII bytes. Otherwise it returns nil.
//
// ToUnicodeClass is ClassBytes::to_unicode_class.
func (c *ClassBytes) ToUnicodeClass() *ClassUnicode {
	if !c.IsASCII() {
		return nil
	}
	rs := c.Ranges()
	ranges := make([]ClassUnicodeRange, 0, len(rs))
	for _, r := range rs {
		// The range is ASCII, so each byte is the same value as a
		// character.
		ranges = append(ranges, ClassUnicodeRange{start: rune(r.start), end: rune(r.end)})
	}
	return NewClassUnicode(ranges)
}

// ClassBytesRange is a range of bytes, with both ends in the range. The start
// is never after the end.
//
// ClassBytesRange is ClassBytesRange.
type ClassBytesRange struct {
	start byte
	end   byte
}

// lower returns the start.
//
// lower is Interval::lower.
func (r ClassBytesRange) lower() byte { return r.start }

// upper returns the end.
//
// upper is Interval::upper.
func (r ClassBytesRange) upper() byte { return r.end }

// setLower returns the range with the start b.
//
// setLower is Interval::set_lower.
func (r ClassBytesRange) setLower(b byte) ClassBytesRange {
	r.start = b
	return r
}

// setUpper returns the range with the end b.
//
// setUpper is Interval::set_upper.
func (r ClassBytesRange) setUpper(b byte) ClassBytesRange {
	r.end = b
	return r
}

// caseFoldSimple appends to ranges the ASCII case folds of the range, and
// returns the slice.
//
// caseFoldSimple is Interval::case_fold_simple.
func (r ClassBytesRange) caseFoldSimple(ranges []ClassBytesRange) []ClassBytesRange {
	if !intervalIsIntersectionEmpty(NewClassBytesRange('a', 'z'), r) {
		lower := max(r.start, 'a')
		upper := min(r.end, 'z')
		ranges = append(ranges, NewClassBytesRange(lower-32, upper-32))
	}
	if !intervalIsIntersectionEmpty(NewClassBytesRange('A', 'Z'), r) {
		lower := max(r.start, 'A')
		upper := min(r.end, 'Z')
		ranges = append(ranges, NewClassBytesRange(lower+32, upper+32))
	}
	return ranges
}

// NewClassBytesRange returns the range from start to end. If start is after
// end, it swaps them.
//
// NewClassBytesRange is ClassBytesRange::new.
func NewClassBytesRange(start, end byte) ClassBytesRange {
	return intervalCreate[ClassBytesRange](start, end)
}

// Start returns the start of the range.
//
// Start is ClassBytesRange::start.
func (r ClassBytesRange) Start() byte {
	return r.start
}

// End returns the end of the range.
//
// End is ClassBytesRange::end.
func (r ClassBytesRange) End() byte {
	return r.end
}

// Len returns the number of bytes in the range.
//
// Len is ClassBytesRange::len.
func (r ClassBytesRange) Len() int {
	return int(r.end-r.start) + 1
}

// Look is an assertion of zero width, such as ^ or \b. Each value is one bit,
// so that a LookSet can hold a set of them.
//
// Look is Look.
type Look uint32

// The assertions, in the order of upstream.
const (
	// LookStart is the start of the text, \A or ^ without the flag m.
	LookStart Look = 1 << 0
	// LookEnd is the end of the text, \z or $ without the flag m.
	LookEnd Look = 1 << 1
	// LookStartLF is the start of a line, (?m:^), where \n ends a line.
	LookStartLF Look = 1 << 2
	// LookEndLF is the end of a line, (?m:$), where \n ends a line.
	LookEndLF Look = 1 << 3
	// LookStartCRLF is the start of a line, (?mR:^), where \r, \n or \r\n
	// ends a line. It does not match between a \r and a \n.
	LookStartCRLF Look = 1 << 4
	// LookEndCRLF is the end of a line, (?mR:$), where \r, \n or \r\n ends a
	// line. It does not match between a \r and a \n.
	LookEndCRLF Look = 1 << 5
	// LookWordASCII is an ASCII word boundary, (?-u:\b).
	LookWordASCII Look = 1 << 6
	// LookWordASCIINegate is not an ASCII word boundary, (?-u:\B).
	LookWordASCIINegate Look = 1 << 7
	// LookWordUnicode is a Unicode word boundary, \b.
	LookWordUnicode Look = 1 << 8
	// LookWordUnicodeNegate is not a Unicode word boundary, \B.
	LookWordUnicodeNegate Look = 1 << 9
	// LookWordStartASCII is the start of an ASCII word, (?-u:\b{start}).
	LookWordStartASCII Look = 1 << 10
	// LookWordEndASCII is the end of an ASCII word, (?-u:\b{end}).
	LookWordEndASCII Look = 1 << 11
	// LookWordStartUnicode is the start of a Unicode word, \b{start}.
	LookWordStartUnicode Look = 1 << 12
	// LookWordEndUnicode is the end of a Unicode word, \b{end}.
	LookWordEndUnicode Look = 1 << 13
	// LookWordStartHalfASCII is the start half of an ASCII word boundary,
	// (?-u:\b{start-half}).
	LookWordStartHalfASCII Look = 1 << 14
	// LookWordEndHalfASCII is the end half of an ASCII word boundary,
	// (?-u:\b{end-half}).
	LookWordEndHalfASCII Look = 1 << 15
	// LookWordStartHalfUnicode is the start half of a Unicode word boundary,
	// \b{start-half}.
	LookWordStartHalfUnicode Look = 1 << 16
	// LookWordEndHalfUnicode is the end half of a Unicode word boundary,
	// \b{end-half}.
	LookWordEndHalfUnicode Look = 1 << 17
)

// Reversed returns the assertion with its direction reversed. For example,
// LookStart becomes LookEnd. An assertion with no direction, such as
// LookWordASCII, stays the same.
//
// Reversed is Look::reversed.
func (l Look) Reversed() Look {
	switch l {
	case LookStart:
		return LookEnd
	case LookEnd:
		return LookStart
	case LookStartLF:
		return LookEndLF
	case LookEndLF:
		return LookStartLF
	case LookStartCRLF:
		return LookEndCRLF
	case LookEndCRLF:
		return LookStartCRLF
	case LookWordASCII:
		return LookWordASCII
	case LookWordASCIINegate:
		return LookWordASCIINegate
	case LookWordUnicode:
		return LookWordUnicode
	case LookWordUnicodeNegate:
		return LookWordUnicodeNegate
	case LookWordStartASCII:
		return LookWordEndASCII
	case LookWordEndASCII:
		return LookWordStartASCII
	case LookWordStartUnicode:
		return LookWordEndUnicode
	case LookWordEndUnicode:
		return LookWordStartUnicode
	case LookWordStartHalfASCII:
		return LookWordEndHalfASCII
	case LookWordEndHalfASCII:
		return LookWordStartHalfASCII
	case LookWordStartHalfUnicode:
		return LookWordEndHalfUnicode
	case LookWordEndHalfUnicode:
		return LookWordStartHalfUnicode
	}
	return l
}

// AsRepr returns the bit of the assertion.
//
// AsRepr is Look::as_repr.
func (l Look) AsRepr() uint32 {
	return uint32(l)
}

// LookFromRepr returns the assertion of the bit repr, and true. If repr is
// not the bit of an assertion, it returns false.
//
// LookFromRepr is Look::from_repr.
func LookFromRepr(repr uint32) (Look, bool) {
	switch Look(repr) {
	case LookStart, LookEnd, LookStartLF, LookEndLF, LookStartCRLF,
		LookEndCRLF, LookWordASCII, LookWordASCIINegate, LookWordUnicode,
		LookWordUnicodeNegate, LookWordStartASCII, LookWordEndASCII,
		LookWordStartUnicode, LookWordEndUnicode, LookWordStartHalfASCII,
		LookWordEndHalfASCII, LookWordStartHalfUnicode, LookWordEndHalfUnicode:
		return Look(repr), true
	}
	return 0, false
}

// AsChar returns a character that stands for the assertion, for the text of
// a LookSet.
//
// AsChar is Look::as_char.
func (l Look) AsChar() rune {
	switch l {
	case LookStart:
		return 'A'
	case LookEnd:
		return 'z'
	case LookStartLF:
		return '^'
	case LookEndLF:
		return '$'
	case LookStartCRLF:
		return 'r'
	case LookEndCRLF:
		return 'R'
	case LookWordASCII:
		return 'b'
	case LookWordASCIINegate:
		return 'B'
	case LookWordUnicode:
		return '\U0001D6C3'
	case LookWordUnicodeNegate:
		return '\U0001D6A9'
	case LookWordStartASCII:
		return '<'
	case LookWordEndASCII:
		return '>'
	case LookWordStartUnicode:
		return '\u3008'
	case LookWordEndUnicode:
		return '\u3009'
	case LookWordStartHalfASCII:
		return '\u25C1'
	case LookWordEndHalfASCII:
		return '\u25B7'
	case LookWordStartHalfUnicode:
		return '\u25C0'
	case LookWordEndHalfUnicode:
		return '\u25B6'
	}
	return 0
}

// Capture is a capture group: a sub-expression, with its index and its
// name.
//
// Capture is Capture.
type Capture struct {
	// Index is the capture index. The group of the whole match, which has no
	// Capture, has the index 0. An explicit group has an index of 1 or more.
	Index uint32
	// Name is the name of the group. It is empty for a group with no name,
	// which is None upstream.
	Name string
	// Sub is the expression in the group.
	Sub *Hir
}

// Repetition is a repetition of a sub-expression, from Min times to Max
// times.
//
// Repetition is Repetition.
type Repetition struct {
	// Min is the least number of times the expression matches.
	Min uint32
	// Max is the most number of times the expression matches. It is nil for
	// no limit, which is None upstream. The constructors never change the
	// value that it points to.
	Max *uint32
	// Greedy is true when the repetition matches as much as it can. A lazy
	// repetition matches as little as it can.
	Greedy bool
	// Sub is the expression that repeats.
	Sub *Hir
}

// With returns a copy of the repetition with the sub-expression sub.
//
// With is Repetition::with.
func (r *Repetition) With(sub *Hir) Repetition {
	return Repetition{
		Min:    r.Min,
		Max:    r.Max,
		Greedy: r.Greedy,
		Sub:    sub,
	}
}

// Dot is a kind of the class of any character, for NewDot. The class can hold
// every character, or every character but one, and the same for bytes.
//
// Dot is Dot. The data of the variants AnyCharExcept and AnyByteExcept of
// upstream is in the fields Char and Byte.
type Dot struct {
	// Kind is the kind of the class.
	Kind DotKind
	// Char is the character that the class does not hold, for
	// DotAnyCharExcept.
	Char rune
	// Byte is the byte that the class does not hold, for DotAnyByteExcept.
	Byte byte
}

// DotKind is the kind of a Dot.
//
// DotKind is the kind of the enum Dot.
type DotKind uint8

// The kinds of Dot, in the order of upstream.
const (
	// DotAnyChar is any Unicode scalar value, (?su:.).
	DotAnyChar DotKind = iota
	// DotAnyByte is any byte, (?s-u:.).
	DotAnyByte
	// DotAnyCharExcept is any Unicode scalar value but the field Char.
	DotAnyCharExcept
	// DotAnyCharExceptLF is any Unicode scalar value but \n, (?u-s:.).
	DotAnyCharExceptLF
	// DotAnyCharExceptCRLF is any Unicode scalar value but \r and \n,
	// (?uR-s:.).
	DotAnyCharExceptCRLF
	// DotAnyByteExcept is any byte but the field Byte.
	DotAnyByteExcept
	// DotAnyByteExceptLF is any byte but \n, (?-su:.).
	DotAnyByteExceptLF
	// DotAnyByteExceptCRLF is any byte but \r and \n, (?R-su:.).
	DotAnyByteExceptCRLF
)

// optLen is a length, or no length.
//
// optLen is Option<usize>. The zero optLen is None.
type optLen struct {
	n  int
	ok bool
}

// someLen returns the length n.
func someLen(n int) optLen {
	return optLen{n: n, ok: true}
}

// Properties are the properties of an expression, which its constructor
// computes once. Each Hir has them, and Hir.Properties returns them.
//
// Properties is Properties, and its fields are the fields of PropertiesI.
type Properties struct {
	minimumLen                optLen
	maximumLen                optLen
	lookSet                   LookSet
	lookSetPrefix             LookSet
	lookSetSuffix             LookSet
	lookSetPrefixAny          LookSet
	lookSetSuffixAny          LookSet
	utf8                      bool
	explicitCapturesLen       int
	staticExplicitCapturesLen optLen
	literal                   bool
	alternationLiteral        bool
}

// MinimumLen returns the length in bytes of the shortest string that the
// expression can match, and true. If the expression never matches, it
// returns false. The length is a lower bound, and it saturates at
// math.MaxInt.
//
// MinimumLen is Properties::minimum_len.
func (p *Properties) MinimumLen() (int, bool) {
	return p.minimumLen.n, p.minimumLen.ok
}

// MaximumLen returns the length in bytes of the longest string that the
// expression can match, and true. If the expression never matches, or if the
// length has no limit or does not fit in an int, it returns false.
//
// MaximumLen is Properties::maximum_len.
func (p *Properties) MaximumLen() (int, bool) {
	return p.maximumLen.n, p.maximumLen.ok
}

// LookSet returns the set of every assertion in the expression.
//
// LookSet is Properties::look_set.
func (p *Properties) LookSet() LookSet {
	return p.lookSet
}

// LookSetPrefix returns the set of the assertions that every match must
// satisfy at its start.
//
// LookSetPrefix is Properties::look_set_prefix.
func (p *Properties) LookSetPrefix() LookSet {
	return p.lookSetPrefix
}

// LookSetPrefixAny returns the set of the assertions that a match can
// satisfy at its start.
//
// LookSetPrefixAny is Properties::look_set_prefix_any.
func (p *Properties) LookSetPrefixAny() LookSet {
	return p.lookSetPrefixAny
}

// LookSetSuffix returns the set of the assertions that every match must
// satisfy at its end.
//
// LookSetSuffix is Properties::look_set_suffix.
func (p *Properties) LookSetSuffix() LookSet {
	return p.lookSetSuffix
}

// LookSetSuffixAny returns the set of the assertions that a match can
// satisfy at its end.
//
// LookSetSuffixAny is Properties::look_set_suffix_any.
func (p *Properties) LookSetSuffixAny() LookSet {
	return p.lookSetSuffixAny
}

// IsUTF8 reports whether each match of the expression that is not empty is
// valid UTF-8. An empty match can still split the encoding of a character.
//
// IsUTF8 is Properties::is_utf8.
func (p *Properties) IsUTF8() bool {
	return p.utf8
}

// ExplicitCapturesLen returns the number of capture groups in the
// expression. The group of the whole match is not in the number.
//
// ExplicitCapturesLen is Properties::explicit_captures_len.
func (p *Properties) ExplicitCapturesLen() int {
	return p.explicitCapturesLen
}

// StaticExplicitCapturesLen returns the number of capture groups that take
// part in every match, and true, if that number is the same for every match.
// Otherwise it returns false. The group of the whole match is not in the
// number.
//
// StaticExplicitCapturesLen is Properties::static_explicit_captures_len.
func (p *Properties) StaticExplicitCapturesLen() (int, bool) {
	return p.staticExplicitCapturesLen.n, p.staticExplicitCapturesLen.ok
}

// IsLiteral reports whether the expression is a literal, or a concatenation
// of literals.
//
// IsLiteral is Properties::is_literal.
func (p *Properties) IsLiteral() bool {
	return p.literal
}

// IsAlternationLiteral reports whether the expression is an alternation of
// literals, or a literal.
//
// IsAlternationLiteral is Properties::is_alternation_literal.
func (p *Properties) IsAlternationLiteral() bool {
	return p.alternationLiteral
}

// MemoryUsage returns the size in bytes of the properties.
//
// MemoryUsage is Properties::memory_usage.
func (p *Properties) MemoryUsage() int {
	return int(unsafe.Sizeof(*p))
}

// UnionProperties returns the properties of an alternation of expressions
// with the properties props.
//
// UnionProperties is Properties::union.
func UnionProperties(props []*Properties) *Properties {
	// An empty alternation cannot occur, but the code works as though it
	// can. An empty alternation has an empty prefix and an empty suffix of
	// assertions. Otherwise the prefix and the suffix are the intersections
	// of the prefixes and of the suffixes of the branches.
	fix := FullLookSet()
	if len(props) == 0 {
		fix = EmptyLookSet()
	}
	// And an empty alternation has 0 static capture groups. Otherwise the
	// number starts as the number of the first branch. If a branch after it
	// has another number, then the number is not static.
	var staticExplicitCapturesLen optLen
	if len(props) > 0 {
		staticExplicitCapturesLen = props[0].staticExplicitCapturesLen
	}
	// The base case is an empty alternation, which matches nothing. It
	// cannot occur, because NewAlternation makes it an empty class.
	p := Properties{
		minimumLen:                optLen{},
		maximumLen:                optLen{},
		lookSet:                   EmptyLookSet(),
		lookSetPrefix:             fix,
		lookSetSuffix:             fix,
		lookSetPrefixAny:          EmptyLookSet(),
		lookSetSuffixAny:          EmptyLookSet(),
		utf8:                      true,
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: staticExplicitCapturesLen,
		literal:                   false,
		alternationLiteral:        true,
	}
	minPoisoned, maxPoisoned := false, false
	// Compute the properties that need each child.
	for _, x := range props {
		p.lookSet.SetUnion(x.LookSet())
		p.lookSetPrefix.SetIntersect(x.LookSetPrefix())
		p.lookSetSuffix.SetIntersect(x.LookSetSuffix())
		p.lookSetPrefixAny.SetUnion(x.LookSetPrefixAny())
		p.lookSetSuffixAny.SetUnion(x.LookSetSuffixAny())
		p.utf8 = p.utf8 && x.IsUTF8()
		p.explicitCapturesLen = saturatingAdd(p.explicitCapturesLen, x.ExplicitCapturesLen())
		if p.staticExplicitCapturesLen != x.staticExplicitCapturesLen {
			p.staticExplicitCapturesLen = optLen{}
		}
		p.alternationLiteral = p.alternationLiteral && x.IsLiteral()
		if !minPoisoned {
			if x.minimumLen.ok {
				if !p.minimumLen.ok || x.minimumLen.n < p.minimumLen.n {
					p.minimumLen = x.minimumLen
				}
			} else {
				p.minimumLen = optLen{}
				minPoisoned = true
			}
		}
		if !maxPoisoned {
			if x.maximumLen.ok {
				if !p.maximumLen.ok || x.maximumLen.n > p.maximumLen.n {
					p.maximumLen = x.maximumLen
				}
			} else {
				p.maximumLen = optLen{}
				maxPoisoned = true
			}
		}
	}
	return &p
}

// emptyProperties returns the properties of the empty expression.
//
// emptyProperties is Properties::empty.
func emptyProperties() Properties {
	return Properties{
		minimumLen:       someLen(0),
		maximumLen:       someLen(0),
		lookSet:          EmptyLookSet(),
		lookSetPrefix:    EmptyLookSet(),
		lookSetSuffix:    EmptyLookSet(),
		lookSetPrefixAny: EmptyLookSet(),
		lookSetSuffixAny: EmptyLookSet(),
		// Whether an empty expression always matches at valid UTF-8
		// boundaries is open to debate. From the view of bytes, it does
		// not. For example, there are many empty strings between the bytes
		// of the encoding of a ☃.
		//
		// But in Unicode mode, the unit of a match is a code point. There
		// an empty expression matches only at valid UTF-8 boundaries, and
		// never splits a code point. It is tricky for a regular expression
		// engine to enforce this for an expression that matches the empty
		// string. It usually needs a layer above the engine to filter out
		// such matches.
		//
		// In any case, true is the only coherent choice. If it is false,
		// then a* must be false too, because it can match the empty
		// string.
		utf8:                      true,
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: someLen(0),
		literal:                   false,
		alternationLiteral:        false,
	}
}

// literalProperties returns the properties of a literal.
//
// literalProperties is Properties::literal.
func literalProperties(lit Literal) Properties {
	return Properties{
		minimumLen:                someLen(len(lit)),
		maximumLen:                someLen(len(lit)),
		lookSet:                   EmptyLookSet(),
		lookSetPrefix:             EmptyLookSet(),
		lookSetSuffix:             EmptyLookSet(),
		lookSetPrefixAny:          EmptyLookSet(),
		lookSetSuffixAny:          EmptyLookSet(),
		utf8:                      utf8.Valid(lit),
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: someLen(0),
		literal:                   true,
		alternationLiteral:        true,
	}
}

// classProperties returns the properties of a class.
//
// classProperties is Properties::class.
func classProperties(class Class) Properties {
	var minimumLen, maximumLen optLen
	if n, ok := class.MinimumLen(); ok {
		minimumLen = someLen(n)
	}
	if n, ok := class.MaximumLen(); ok {
		maximumLen = someLen(n)
	}
	return Properties{
		minimumLen:                minimumLen,
		maximumLen:                maximumLen,
		lookSet:                   EmptyLookSet(),
		lookSetPrefix:             EmptyLookSet(),
		lookSetSuffix:             EmptyLookSet(),
		lookSetPrefixAny:          EmptyLookSet(),
		lookSetSuffixAny:          EmptyLookSet(),
		utf8:                      class.IsUTF8(),
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: someLen(0),
		literal:                   false,
		alternationLiteral:        false,
	}
}

// lookProperties returns the properties of an assertion.
//
// lookProperties is Properties::look.
func lookProperties(look Look) Properties {
	return Properties{
		minimumLen:       someLen(0),
		maximumLen:       someLen(0),
		lookSet:          SingletonLookSet(look),
		lookSetPrefix:    SingletonLookSet(look),
		lookSetSuffix:    SingletonLookSet(look),
		lookSetPrefixAny: SingletonLookSet(look),
		lookSetSuffixAny: SingletonLookSet(look),
		// Upstream does not count a match of the empty string as a match of
		// invalid UTF-8, even though each match of the empty string can
		// split the UTF-8 encoding of a code point, when the text is a
		// sequence of bytes. The reason is that a code point is the unit of
		// a match, so a match can only be between code points, not between
		// bytes.
		//
		// In practice, this must be true because it is true for NewEmpty.
		// Otherwise an expression such as a* matches invalid UTF-8 by this
		// property, and the property is of almost no use.
		utf8:                      true,
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: someLen(0),
		literal:                   false,
		alternationLiteral:        false,
	}
}

// repetitionProperties returns the properties of a repetition.
//
// repetitionProperties is Properties::repetition.
func repetitionProperties(rep *Repetition) Properties {
	p := rep.Sub.Properties()
	var minimumLen optLen
	if childMin, ok := p.MinimumLen(); ok {
		repMin := math.MaxInt
		if uint64(rep.Min) <= math.MaxInt {
			repMin = int(rep.Min)
		}
		minimumLen = someLen(saturatingMul(childMin, repMin))
	}
	var maximumLen optLen
	if rep.Max != nil && uint64(*rep.Max) <= math.MaxInt {
		repMax := int(*rep.Max)
		if childMax, ok := p.MaximumLen(); ok {
			if n, ok := checkedMul(childMax, repMax); ok {
				maximumLen = someLen(n)
			}
		}
	}

	inner := Properties{
		minimumLen:                minimumLen,
		maximumLen:                maximumLen,
		lookSet:                   p.LookSet(),
		lookSetPrefix:             EmptyLookSet(),
		lookSetSuffix:             EmptyLookSet(),
		lookSetPrefixAny:          p.LookSetPrefixAny(),
		lookSetSuffixAny:          p.LookSetSuffixAny(),
		utf8:                      p.IsUTF8(),
		explicitCapturesLen:       p.ExplicitCapturesLen(),
		staticExplicitCapturesLen: p.staticExplicitCapturesLen,
		literal:                   false,
		alternationLiteral:        false,
	}
	// If the repetition can match the empty string, then its prefix and
	// suffix of assertions stay empty, because a match does not need them.
	if rep.Min > 0 {
		inner.lookSetPrefix = p.LookSetPrefix()
		inner.lookSetSuffix = p.LookSetSuffix()
	}
	// If the static number of capture groups of the sub-expression is not
	// known, or is more than zero, then the repetition has it too, whatever
	// the repetition is. Otherwise it can change, but only when the
	// repetition can match 0 times.
	if rep.Min == 0 && inner.staticExplicitCapturesLen.ok && inner.staticExplicitCapturesLen.n > 0 {
		// If the repetition must match 0 times, then the number is surely
		// zero. Otherwise, if it can match the empty string, then no one
		// knows how many groups a match holds.
		if rep.Max != nil && *rep.Max == 0 {
			inner.staticExplicitCapturesLen = someLen(0)
		} else {
			inner.staticExplicitCapturesLen = optLen{}
		}
	}
	return inner
}

// captureProperties returns the properties of a capture group.
//
// captureProperties is Properties::capture.
func captureProperties(capture *Capture) Properties {
	p := *capture.Sub.Properties()
	p.explicitCapturesLen = saturatingAdd(p.explicitCapturesLen, 1)
	if p.staticExplicitCapturesLen.ok {
		p.staticExplicitCapturesLen = someLen(saturatingAdd(p.staticExplicitCapturesLen.n, 1))
	}
	p.literal = false
	p.alternationLiteral = false
	return p
}

// concatProperties returns the properties of a concatenation.
//
// concatProperties is Properties::concat.
func concatProperties(concat []*Hir) Properties {
	// The base case is an empty concatenation, which matches the empty
	// string. It cannot occur, because NewConcat makes it NewEmpty.
	props := Properties{
		minimumLen:                someLen(0),
		maximumLen:                someLen(0),
		lookSet:                   EmptyLookSet(),
		lookSetPrefix:             EmptyLookSet(),
		lookSetSuffix:             EmptyLookSet(),
		lookSetPrefixAny:          EmptyLookSet(),
		lookSetSuffixAny:          EmptyLookSet(),
		utf8:                      true,
		explicitCapturesLen:       0,
		staticExplicitCapturesLen: someLen(0),
		literal:                   true,
		alternationLiteral:        true,
	}
	// Compute the properties that need each child.
	for _, x := range concat {
		p := x.Properties()
		props.lookSet.SetUnion(p.LookSet())
		props.utf8 = props.utf8 && p.IsUTF8()
		props.explicitCapturesLen = saturatingAdd(props.explicitCapturesLen, p.ExplicitCapturesLen())
		if p.staticExplicitCapturesLen.ok && props.staticExplicitCapturesLen.ok {
			props.staticExplicitCapturesLen = someLen(saturatingAdd(p.staticExplicitCapturesLen.n, props.staticExplicitCapturesLen.n))
		} else {
			props.staticExplicitCapturesLen = optLen{}
		}
		props.literal = props.literal && p.IsLiteral()
		props.alternationLiteral = props.alternationLiteral && p.IsAlternationLiteral()
		if props.minimumLen.ok {
			if n, ok := p.MinimumLen(); ok {
				// The sum saturates, because the minimum is only a lower
				// bound. It cannot go higher than the number type allows.
				props.minimumLen = someLen(saturatingAdd(props.minimumLen.n, n))
			} else {
				props.minimumLen = optLen{}
			}
		}
		if props.maximumLen.ok {
			if n, ok := p.MaximumLen(); ok {
				if sum, ok := checkedAdd(props.maximumLen.n, n); ok {
					props.maximumLen = someLen(sum)
				} else {
					props.maximumLen = optLen{}
				}
			} else {
				props.maximumLen = optLen{}
			}
		}
	}
	// Compute the prefix properties. They need only the children up to the
	// first one that can match more than the empty string.
	for _, x := range concat {
		props.lookSetPrefix.SetUnion(x.Properties().LookSetPrefix())
		props.lookSetPrefixAny.SetUnion(x.Properties().LookSetPrefixAny())
		if n, ok := x.Properties().MaximumLen(); !ok || n > 0 {
			break
		}
	}
	// Do the same for the suffix properties, but in reverse.
	for _, x := range slices.Backward(concat) {
		props.lookSetSuffix.SetUnion(x.Properties().LookSetSuffix())
		props.lookSetSuffixAny.SetUnion(x.Properties().LookSetSuffixAny())
		if n, ok := x.Properties().MaximumLen(); !ok || n > 0 {
			break
		}
	}
	return props
}

// alternationProperties returns the properties of an alternation.
//
// alternationProperties is Properties::alternation.
func alternationProperties(alts []*Hir) Properties {
	props := make([]*Properties, 0, len(alts))
	for _, hir := range alts {
		props = append(props, hir.Properties())
	}
	return *UnionProperties(props)
}

// saturatingAdd returns a + b, or math.MaxInt if the sum does not fit. Both
// values are not negative.
//
// saturatingAdd is usize::saturating_add.
func saturatingAdd(a, b int) int {
	if sum, ok := checkedAdd(a, b); ok {
		return sum
	}
	return math.MaxInt
}

// checkedAdd returns a + b and true, or false if the sum does not fit in an
// int. Both values are not negative.
//
// checkedAdd is usize::checked_add.
func checkedAdd(a, b int) (int, bool) {
	if a > math.MaxInt-b {
		return 0, false
	}
	return a + b, true
}

// saturatingMul returns a * b, or math.MaxInt if the product does not fit.
// Both values are not negative.
//
// saturatingMul is usize::saturating_mul.
func saturatingMul(a, b int) int {
	if p, ok := checkedMul(a, b); ok {
		return p
	}
	return math.MaxInt
}

// checkedMul returns a * b and true, or false if the product does not fit in
// an int. Both values are not negative.
//
// checkedMul is usize::checked_mul.
func checkedMul(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > math.MaxInt/b {
		return 0, false
	}
	return a * b, true
}

// LookSet is a set of assertions. Each bit of Bits is one Look.
//
// LookSet is LookSet.
type LookSet struct {
	// Bits holds a bit for each Look in the set. The bits that are not the
	// bit of a Look have no meaning. FullLookSet sets them all.
	Bits uint32
}

// EmptyLookSet returns the empty set.
//
// EmptyLookSet is LookSet::empty.
func EmptyLookSet() LookSet {
	return LookSet{Bits: 0}
}

// FullLookSet returns the set of every assertion.
//
// FullLookSet is LookSet::full.
func FullLookSet() LookSet {
	return LookSet{Bits: math.MaxUint32}
}

// SingletonLookSet returns the set of look alone.
//
// SingletonLookSet is LookSet::singleton.
func SingletonLookSet(look Look) LookSet {
	return EmptyLookSet().Insert(look)
}

// Len returns the number of assertions in the set.
//
// Len is LookSet::len.
func (s LookSet) Len() int {
	return bits.OnesCount32(s.Bits)
}

// IsEmpty reports whether the set is empty.
//
// IsEmpty is LookSet::is_empty.
func (s LookSet) IsEmpty() bool {
	return s.Len() == 0
}

// Contains reports whether the set holds look.
//
// Contains is LookSet::contains.
func (s LookSet) Contains(look Look) bool {
	return s.Bits&look.AsRepr() != 0
}

// ContainsAnchor reports whether the set holds an anchor of the text or of a
// line.
//
// ContainsAnchor is LookSet::contains_anchor.
func (s LookSet) ContainsAnchor() bool {
	return s.ContainsAnchorHaystack() || s.ContainsAnchorLine()
}

// ContainsAnchorHaystack reports whether the set holds LookStart or LookEnd.
//
// ContainsAnchorHaystack is LookSet::contains_anchor_haystack.
func (s LookSet) ContainsAnchorHaystack() bool {
	return s.Contains(LookStart) || s.Contains(LookEnd)
}

// ContainsAnchorLine reports whether the set holds an anchor of a line.
//
// ContainsAnchorLine is LookSet::contains_anchor_line.
func (s LookSet) ContainsAnchorLine() bool {
	return s.Contains(LookStartLF) ||
		s.Contains(LookEndLF) ||
		s.Contains(LookStartCRLF) ||
		s.Contains(LookEndCRLF)
}

// ContainsAnchorLF reports whether the set holds LookStartLF or LookEndLF.
//
// ContainsAnchorLF is LookSet::contains_anchor_lf.
func (s LookSet) ContainsAnchorLF() bool {
	return s.Contains(LookStartLF) || s.Contains(LookEndLF)
}

// ContainsAnchorCRLF reports whether the set holds LookStartCRLF or
// LookEndCRLF.
//
// ContainsAnchorCRLF is LookSet::contains_anchor_crlf.
func (s LookSet) ContainsAnchorCRLF() bool {
	return s.Contains(LookStartCRLF) || s.Contains(LookEndCRLF)
}

// ContainsWord reports whether the set holds an assertion of a word
// boundary, ASCII or Unicode.
//
// ContainsWord is LookSet::contains_word.
func (s LookSet) ContainsWord() bool {
	return s.ContainsWordUnicode() || s.ContainsWordASCII()
}

// ContainsWordUnicode reports whether the set holds an assertion of a
// Unicode word boundary.
//
// ContainsWordUnicode is LookSet::contains_word_unicode.
func (s LookSet) ContainsWordUnicode() bool {
	return s.Contains(LookWordUnicode) ||
		s.Contains(LookWordUnicodeNegate) ||
		s.Contains(LookWordStartUnicode) ||
		s.Contains(LookWordEndUnicode) ||
		s.Contains(LookWordStartHalfUnicode) ||
		s.Contains(LookWordEndHalfUnicode)
}

// ContainsWordASCII reports whether the set holds an assertion of an ASCII
// word boundary.
//
// ContainsWordASCII is LookSet::contains_word_ascii.
func (s LookSet) ContainsWordASCII() bool {
	return s.Contains(LookWordASCII) ||
		s.Contains(LookWordASCIINegate) ||
		s.Contains(LookWordStartASCII) ||
		s.Contains(LookWordEndASCII) ||
		s.Contains(LookWordStartHalfASCII) ||
		s.Contains(LookWordEndHalfASCII)
}

// Iter returns the assertions of the set, from the lowest bit to the highest.
//
// Iter is LookSet::iter, and the Iterator of LookSetIter.
func (s LookSet) Iter() iter.Seq[Look] {
	return func(yield func(Look) bool) {
		set := s
		for !set.IsEmpty() {
			// There are never more than 255 distinct assertions, so bit
			// always fits in a uint16.
			bit := uint16(bits.TrailingZeros32(set.Bits))
			look, ok := LookFromRepr(1 << bit)
			if !ok {
				return
			}
			set = set.Remove(look)
			if !yield(look) {
				return
			}
		}
	}
}

// Insert returns the set with look added.
//
// Insert is LookSet::insert.
func (s LookSet) Insert(look Look) LookSet {
	return LookSet{Bits: s.Bits | look.AsRepr()}
}

// SetInsert adds look to the set, in place.
//
// SetInsert is LookSet::set_insert.
func (s *LookSet) SetInsert(look Look) {
	*s = s.Insert(look)
}

// Remove returns the set with look removed.
//
// Remove is LookSet::remove.
func (s LookSet) Remove(look Look) LookSet {
	return LookSet{Bits: s.Bits &^ look.AsRepr()}
}

// SetRemove removes look from the set, in place.
//
// SetRemove is LookSet::set_remove.
func (s *LookSet) SetRemove(look Look) {
	*s = s.Remove(look)
}

// Subtract returns the set with each assertion of other removed.
//
// Subtract is LookSet::subtract.
func (s LookSet) Subtract(other LookSet) LookSet {
	return LookSet{Bits: s.Bits &^ other.Bits}
}

// SetSubtract removes each assertion of other from the set, in place.
//
// SetSubtract is LookSet::set_subtract.
func (s *LookSet) SetSubtract(other LookSet) {
	*s = s.Subtract(other)
}

// Union returns the union of the set and other.
//
// Union is LookSet::union.
func (s LookSet) Union(other LookSet) LookSet {
	return LookSet{Bits: s.Bits | other.Bits}
}

// SetUnion adds each assertion of other to the set, in place.
//
// SetUnion is LookSet::set_union.
func (s *LookSet) SetUnion(other LookSet) {
	*s = s.Union(other)
}

// Intersect returns the intersection of the set and other.
//
// Intersect is LookSet::intersect.
func (s LookSet) Intersect(other LookSet) LookSet {
	return LookSet{Bits: s.Bits & other.Bits}
}

// SetIntersect keeps only the assertions of the set that other holds too,
// in place.
//
// SetIntersect is LookSet::set_intersect.
func (s *LookSet) SetIntersect(other LookSet) {
	*s = s.Intersect(other)
}

// ReadLookSetRepr returns the set in the first 4 bytes of slice, in the
// native byte order. It panics if slice is shorter than 4 bytes.
//
// ReadLookSetRepr is LookSet::read_repr.
func ReadLookSetRepr(slice []byte) LookSet {
	b := binary.NativeEndian.Uint32(slice[:4])
	return LookSet{Bits: b}
}

// WriteRepr writes the set to the first 4 bytes of slice, in the native byte
// order. It panics if slice is shorter than 4 bytes.
//
// WriteRepr is LookSet::write_repr.
func (s LookSet) WriteRepr(slice []byte) {
	var raw [4]byte
	binary.NativeEndian.PutUint32(raw[:], s.Bits)
	slice[0] = raw[0]
	slice[1] = raw[1]
	slice[2] = raw[2]
	slice[3] = raw[3]
}

// String returns the characters of the assertions of the set, or ∅ for the
// empty set.
//
// String is the Debug of LookSet.
func (s LookSet) String() string {
	if s.IsEmpty() {
		return "∅"
	}
	var b strings.Builder
	for look := range s.Iter() {
		b.WriteRune(look.AsChar())
	}
	return b.String()
}

// classChars returns the union of the classes, and true, if each expression
// is a class of characters, or a class of ASCII bytes. Otherwise it returns
// false.
//
// classChars is class_chars.
func classChars(hirs []*Hir) (Class, bool) {
	cls := NewClassUnicode(nil)
	for _, hir := range hirs {
		switch cls2 := hir.Kind().(type) {
		case *ClassUnicode:
			cls.Union(cls2)
		case *ClassBytes:
			u := cls2.ToUnicodeClass()
			if u == nil {
				return nil, false
			}
			cls.Union(u)
		default:
			return nil, false
		}
	}
	return cls, true
}

// classBytes returns the union of the classes, and true, if each expression
// is a class of bytes, or a class of ASCII characters. Otherwise it returns
// false.
//
// classBytes is class_bytes.
func classBytes(hirs []*Hir) (Class, bool) {
	cls := NewClassBytes(nil)
	for _, hir := range hirs {
		switch cls2 := hir.Kind().(type) {
		case *ClassUnicode:
			b := cls2.ToByteClass()
			if b == nil {
				return nil, false
			}
			cls.Union(b)
		case *ClassBytes:
			cls.Union(cls2)
		default:
			return nil, false
		}
	}
	return cls, true
}

// singletonChars returns the characters, and true, if each expression is a
// literal of exactly one character. Otherwise it returns false.
//
// singletonChars is singleton_chars. It decodes the literal with
// utf8.DecodeRune in place of utf8_decode of debug.rs. The two agree on the
// case that matters: the literal is one valid UTF-8 character, and nothing
// else.
func singletonChars(hirs []*Hir) ([]rune, bool) {
	var singletons []rune
	for _, hir := range hirs {
		lit, ok := hir.Kind().(*Literal)
		if !ok {
			return nil, false
		}
		ch, size := utf8.DecodeRune(*lit)
		if size == 0 {
			return nil, false
		}
		if ch == utf8.RuneError && size == 1 {
			return nil, false
		}
		if len(*lit) != utf8.RuneLen(ch) {
			return nil, false
		}
		singletons = append(singletons, ch)
	}
	return singletons, true
}

// singletonBytes returns the bytes, and true, if each expression is a
// literal of exactly one byte. Otherwise it returns false.
//
// singletonBytes is singleton_bytes.
func singletonBytes(hirs []*Hir) ([]byte, bool) {
	var singletons []byte
	for _, hir := range hirs {
		lit, ok := hir.Kind().(*Literal)
		if !ok {
			return nil, false
		}
		if len(*lit) != 1 {
			return nil, false
		}
		singletons = append(singletons, (*lit)[0])
	}
	return singletons, true
}

// liftCommonPrefix lifts a common prefix out of the branches of an
// alternation. If every branch is a concatenation, and the concatenations
// start with the same expressions, it returns a concatenation of that prefix
// and the alternation of the rest, and nil. Otherwise it returns nil and
// hirs as they are.
//
// liftCommonPrefix is lift_common_prefix, which returns Result<Hir,
// Vec<Hir>>.
func liftCommonPrefix(hirs []*Hir) (*Hir, []*Hir) {
	if len(hirs) <= 1 {
		return nil, hirs
	}
	first, ok := hirs[0].Kind().(*Concat)
	if !ok {
		return nil, hirs
	}
	prefix := []*Hir(*first)
	if len(prefix) == 0 {
		return nil, hirs
	}
	for _, h := range hirs[1:] {
		concat, ok := h.Kind().(*Concat)
		if !ok {
			return nil, hirs
		}
		commonLen := 0
		for commonLen < len(prefix) && commonLen < len(*concat) &&
			prefix[commonLen].Equal((*concat)[commonLen]) {
			commonLen++
		}
		prefix = prefix[:commonLen]
		if len(prefix) == 0 {
			return nil, hirs
		}
	}
	n := len(prefix)
	if n == 0 {
		panic("assertion `left != right` failed")
	}
	var prefixConcat []*Hir
	var suffixAlts []*Hir
	for _, h := range hirs {
		concat, ok := h.IntoKind().(*Concat)
		if !ok {
			// Every branch is a concatenation, as the code checked above.
			panic("internal error: entered unreachable code")
		}
		suffixAlts = append(suffixAlts, NewConcat((*concat)[n:]))
		if len(prefixConcat) == 0 {
			// The full slice expression makes the append below copy, so
			// that it does not write into the concatenation.
			prefixConcat = (*concat)[:n:n]
		}
	}
	prefixConcat = append(prefixConcat, NewAlternation(suffixAlts))
	return NewConcat(prefixConcat), nil
}
