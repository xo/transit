package generate

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/transit/generate/internal/regexsyntax/ast"
	"github.com/xo/transit/generate/internal/regexsyntax/hir"
)

// This file ports crates/generate/src/prepare_grammar/pattern.rs: the reader
// of the pattern of a token.
//
// Unicode simple case folding maps two code points that are not ASCII onto
// ASCII letters: the long s ſ onto s, and the Kelvin sign K onto k. Without a
// change, every case-insensitive ASCII token takes them in too, which a
// grammar almost never wants, and the token can then not be a keyword. So
// the port folds the i flag itself.
//
// The parser of regex-syntax reads a pattern into a syntax tree, and its
// translator turns the tree into the HIR. The translator applies the i flag.
// It folds each leaf of a class before the negation of the leaf and the set
// operations above it. That order makes (?i)[^x] leave out X, and not take it
// back. So the port keeps that order and changes only the fold:
//
//   - It walks the syntax tree.
//   - It folds each leaf as if ſ and K each were a group of case variants of
//     its own.
//   - It gives the result to the translator, with the folding of the
//     translator turned off.
//
// To fold at the leaves is enough. Case folding puts each character in one
// group of case variants, such as {a, A} and {s, S, ſ}. To fold a set adds,
// for each character in it, the rest of its group, so the result is whole
// groups. A negation and a set operation keep that, because they treat every
// character of a group alike. To fold a set of whole groups adds nothing. So
// once the leaves are folded, every fold that the translator makes after
// that adds nothing, and the translator gives what it would give with this
// fold in place of its own.
//
// Upstream mirrors the error kinds of regex-syntax in RegexErrorKind with a
// macro, so that it can serialize them. The Go port keeps the mirror as an
// enum with a text for each kind.

// foldMode is the flags that decide whether the port folds the leaves of a
// scope.
//
// foldMode is FoldMode.
type foldMode struct {
	caseInsensitive bool
	unicode         bool
}

// foldsHere reports whether the port folds the leaves of this scope. In a
// (?-u:...) scope, the translator works on bytes and folds ASCII only, which
// can never take in ſ or K, so such a scope keeps the folding of the
// translator.
//
// foldsHere is FoldMode::folds_here.
func (m foldMode) foldsHere() bool {
	return m.caseInsensitive && m.unicode
}

// patternExpander is the state of the walk: the flags in effect, and one
// translator that resolves \p{...} and the other classes, so that it is not
// built again for each leaf.
//
// patternExpander is Expander.
type patternExpander struct {
	translator *hir.Translator
	pattern    string
	mode       foldMode
}

// parsePattern reads the pattern of a token into the HIR, and folds the i
// flag itself when caseInsensitive is true.
//
// parsePattern is parse in pattern.rs.
func parsePattern(pattern string, caseInsensitive bool) (*hir.Hir, error) {
	a, err := ast.NewParserBuilder().Build().Parse(pattern)
	if err != nil {
		return nil, newRegexError(err)
	}
	e := &patternExpander{
		translator: hir.NewTranslatorBuilder().CaseInsensitive(false).Unicode(true).UTF8(false).Build(),
		pattern:    pattern,
		mode:       foldMode{caseInsensitive: caseInsensitive, unicode: true},
	}
	if err := e.expand(&a); err != nil {
		return nil, newRegexError(err)
	}
	h, err := e.translator.Translate(pattern, a)
	if err != nil {
		return nil, newRegexError(err)
	}
	return h, nil
}

// expand folds the leaves of the syntax tree at a, in place.
//
// expand is Expander::expand.
func (e *patternExpander) expand(a *ast.Ast) error {
	switch n := (*a).(type) {
	case *ast.SetFlags:
		e.setFlags(&n.Flags)
		return nil
	case *ast.Repetition:
		return e.expand(&n.Ast)
	case *ast.Group:
		outer := e.mode
		if n.Kind == ast.GroupNonCapturing {
			e.setFlags(&n.Flags)
		}
		if err := e.expand(&n.Ast); err != nil {
			return err
		}
		e.mode = outer
		return nil
	case *ast.Alternation:
		// The flags that one branch sets carry into the next, as they do in
		// the translator, which scopes flags to groups but not to these.
		for i := range n.Asts {
			if err := e.expand(&n.Asts[i]); err != nil {
				return err
			}
		}
		return nil
	case *ast.Concat:
		for i := range n.Asts {
			if err := e.expand(&n.Asts[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if !e.mode.foldsHere() {
		// Left to the translator: no i is in effect, or this is a (?-u:...)
		// scope, whose folding is ASCII only and cannot take in ſ or K.
		return nil
	}
	var item ast.ClassSetItem
	switch n := (*a).(type) {
	case *ast.ClassBracketed:
		return e.expandClassSet(&n.Kind)
	// The other nodes that hold a class are the same leaves as a bracket
	// holds, so they fold in the same way.
	case *ast.Literal:
		l := *n
		item = &l
	case *ast.ClassUnicode:
		u := *n
		item = &u
	case *ast.ClassPerl:
		p := *n
		item = &p
	default:
		// a dot is closed under folding, and nothing else matches a
		// character
		return nil
	}
	if err := e.expandClassItem(&item); err != nil {
		return err
	}
	if class, ok := item.(*ast.ClassBracketed); ok {
		*a = class
	}
	return nil
}

// setFlags applies the flags of a directive to the mode, and then rewrites
// the directive to say what the translator must still do about i in the
// scope that it opens.
//
// The i flag comes out everywhere, because to fold a leaf that the port
// folded already would put ſ and K back. It goes back in wherever foldsHere
// says that the port left the folding alone, because that scope would
// otherwise lose its case-insensitivity.
//
// setFlags is Expander::set_flags.
func (e *patternExpander) setFlags(flags *ast.Flags) {
	negated := false
	for _, item := range flags.Items {
		switch {
		case item.Kind == ast.FlagsItemNegation:
			negated = true
		case item.Kind == ast.FlagsItemFlag && item.Flag == ast.FlagCaseInsensitive:
			e.mode.caseInsensitive = !negated
		case item.Kind == ast.FlagsItemFlag && item.Flag == ast.FlagUnicode:
			e.mode.unicode = !negated
		}
	}
	flags.Items = slices.DeleteFunc(flags.Items, func(it ast.FlagsItem) bool {
		return it.Kind == ast.FlagsItemFlag && it.Flag == ast.FlagCaseInsensitive
	})
	switch {
	case e.mode.foldsHere():
		flags.Items = append(flags.Items,
			ast.FlagsItem{Span: flags.Span, Kind: ast.FlagsItemNegation},
			ast.FlagsItem{Span: flags.Span, Kind: ast.FlagsItemFlag, Flag: ast.FlagCaseInsensitive})
	case e.mode.caseInsensitive:
		flags.Items = slices.Insert(flags.Items, 0, ast.FlagsItem{Span: flags.Span, Kind: ast.FlagsItemFlag, Flag: ast.FlagCaseInsensitive})
	}
}

// expandClassSet folds the leaves of a class expression, and leaves its
// structure as it is. The translator still does the negations and the
// operations &&, -- and ~~, on operands that are folded already.
//
// expandClassSet is Expander::expand_class_set.
func (e *patternExpander) expandClassSet(set *ast.ClassSet) error {
	if op, ok := (*set).(*ast.ClassSetBinaryOp); ok {
		if err := e.expandClassSet(&op.LHS); err != nil {
			return err
		}
		return e.expandClassSet(&op.RHS)
	}
	item, ok := (*set).(ast.ClassSetItem)
	if !ok {
		return nil
	}
	if err := e.expandClassItem(&item); err != nil {
		return err
	}
	*set = item
	return nil
}

// expandClassItem folds one item of a class, in place.
//
// expandClassItem is Expander::expand_class_item.
func (e *patternExpander) expandClassItem(item *ast.ClassSetItem) error {
	// whether the leaf holds a negation of its own, as \P{L}, [:^alpha:] and
	// \D do
	var span ast.Span
	var negated bool
	switch n := (*item).(type) {
	case *ast.ClassBracketed:
		return e.expandClassSet(&n.Kind)
	case *ast.ClassSetUnion:
		for i := range n.Items {
			if err := e.expandClassItem(&n.Items[i]); err != nil {
				return err
			}
		}
		return nil
	case *ast.Empty:
		return nil
	case *ast.Literal:
		span = n.Span
	case *ast.ClassSetRange:
		span = n.Span
	case *ast.ClassASCII:
		span, negated = n.Span, n.Negated
	case *ast.ClassPerl:
		span, negated = n.Span, n.Negated
	case *ast.ClassUnicode:
		span, negated = n.Span, n.IsNegated()
	}

	// leafSet applies the negation of the leaf, so undo it to find the
	// operand that the translator would fold, and give the negation back to
	// the replacement, so that it applies after the fold
	operand, err := e.leafSet(*item)
	if err != nil {
		return err
	}
	if negated {
		operand.Negate()
	}

	folded := operand.Clone()
	foldASCIISafe(folded)
	// A leaf that is closed under folding already stays as it is written,
	// which keeps most Unicode properties out of the expansion.
	if !slices.Equal(folded.Ranges(), operand.Ranges()) {
		*item = &ast.ClassBracketed{
			Span:    span,
			Negated: negated,
			Kind:    classToItem(span, folded),
		}
	}
	return nil
}

// leafSet returns the set of a leaf, with the negation of the leaf applied.
// A literal and a range are read from the syntax tree. Every other leaf goes
// through the translator, which holds the tables of the properties and of
// POSIX.
//
// leafSet is Expander::leaf_set.
func (e *patternExpander) leafSet(item ast.ClassSetItem) (*hir.ClassUnicode, error) {
	var r hir.ClassUnicodeRange
	switch n := item.(type) {
	case *ast.Literal:
		r = hir.NewClassUnicodeRange(n.C, n.C)
	case *ast.ClassSetRange:
		r = hir.NewClassUnicodeRange(n.Start.C, n.End.C)
	default:
		a := &ast.ClassBracketed{Span: item.GetSpan(), Kind: item}
		h, err := e.translator.Translate(e.pattern, a)
		if err != nil {
			return nil, err
		}
		return classOf(h), nil
	}
	return hir.NewClassUnicode([]hir.ClassUnicodeRange{r}), nil
}

// classOf returns the class of an HIR of one class. The port translates only
// one ClassBracketed, so Hir::class built the result: a class, or one of the
// two forms that a class becomes. The other kinds need nodes that leafSet
// never builds.
//
// classOf is Expander::class_of.
func classOf(h *hir.Hir) *hir.ClassUnicode {
	switch k := h.IntoKind().(type) {
	case *hir.ClassUnicode:
		return k
	case *hir.Literal:
		// a class of one character, translated in Unicode mode, so this is
		// the UTF-8 of that character
		s := string(*k)
		if !utf8.ValidString(s) {
			panic("called `Result::unwrap()` on an `Err` value")
		}
		ranges := make([]hir.ClassUnicodeRange, 0, utf8.RuneCountInString(s))
		for _, c := range s {
			ranges = append(ranges, hir.NewClassUnicodeRange(c, c))
		}
		return hir.NewClassUnicode(ranges)
	}
	// an empty class becomes Hir::fail, an empty class of bytes
	return hir.EmptyClassUnicode()
}

// classToItem writes a class as the syntax nodes that the translator reads
// back, the inverse of leafSet. Every node takes the span of the leaf, to
// keep the spans of the errors. The kind of the literal does not matter,
// because in Unicode mode the translator reads only the character.
//
// classToItem is Expander::class_to_item.
func classToItem(span ast.Span, class *hir.ClassUnicode) ast.ClassSetItem {
	literal := func(c rune) ast.Literal {
		return ast.Literal{Span: span, Kind: ast.LiteralVerbatim, C: c}
	}
	ranges := class.Ranges()
	items := make([]ast.ClassSetItem, 0, len(ranges))
	for _, r := range ranges {
		items = append(items, &ast.ClassSetRange{Span: span, Start: literal(r.Start()), End: literal(r.End())})
	}
	return (&ast.ClassSetUnion{Span: span, Items: items}).IntoItem()
}

// foldASCIISafe folds class in place, with ſ and K each in a group of its
// own, and not in the groups of s and S, and of k and K.
//
// To drop them before the fold keeps a ſ in the class from taking in s and
// S. To drop them after the fold removes the ones that the fold of s and S
// added. The union puts back the ones that the class held.
//
// foldASCIISafe is Expander::fold_ascii_safe.
func foldASCIISafe(class *hir.ClassUnicode) {
	// The code points that Unicode simple case folding maps onto ASCII
	// letters: the long s ſ folds with s and S, and the Kelvin sign K with k
	// and K.
	nonASCIIFolds := [2]rune{'ſ', 'K'}
	exotic := hir.NewClassUnicode([]hir.ClassUnicodeRange{
		hir.NewClassUnicodeRange(nonASCIIFolds[0], nonASCIIFolds[0]),
		hir.NewClassUnicodeRange(nonASCIIFolds[1], nonASCIIFolds[1]),
	})

	// the ones that the pattern asked for, which the fold must not drop
	askedFor := exotic.Clone()
	askedFor.Intersect(class)

	class.Difference(exotic)
	class.CaseFoldSimple()
	class.Difference(exotic)
	class.Union(askedFor)
}

// RegexErrorKind is the kind of an error of a pattern.
//
// RegexErrorKind is RegexErrorKind, the mirror of the error kinds of
// regex-syntax.
type RegexErrorKind uint8

// The kinds of error of a pattern, in the order of upstream: the kinds of
// the parser, the kinds of the translator, and then the kinds that hold
// data.
const (
	RegexCaptureLimitExceeded RegexErrorKind = iota
	RegexClassEscapeInvalid
	RegexClassRangeInvalid
	RegexClassRangeLiteral
	RegexClassUnclosed
	RegexDecimalEmpty
	RegexDecimalInvalid
	RegexEscapeHexEmpty
	RegexEscapeHexInvalid
	RegexEscapeHexInvalidDigit
	RegexEscapeUnexpectedEOF
	RegexEscapeUnrecognized
	RegexFlagDanglingNegation
	RegexFlagUnexpectedEOF
	RegexFlagUnrecognized
	RegexGroupNameEmpty
	RegexGroupNameInvalid
	RegexGroupNameUnexpectedEOF
	RegexGroupUnclosed
	RegexGroupUnopened
	RegexRepetitionCountInvalid
	RegexRepetitionCountDecimalEmpty
	RegexRepetitionCountUnclosed
	RegexRepetitionMissing
	RegexSpecialWordBoundaryUnclosed
	RegexSpecialWordBoundaryUnrecognized
	RegexSpecialWordOrRepetitionUnexpectedEOF
	RegexUnicodeClassInvalid
	RegexUnsupportedBackreference
	RegexUnsupportedLookAround
	RegexUnicodeNotAllowed
	RegexInvalidUTF8
	RegexInvalidLineTerminator
	RegexUnicodePropertyNotFound
	RegexUnicodePropertyValueNotFound
	RegexUnicodePerlClassNotFound
	RegexUnicodeCaseUnavailable
	RegexFlagDuplicate
	RegexFlagRepeatedNegation
	RegexGroupNameDuplicate
	RegexNestLimitExceeded
	// RegexOther is a kind that regex-syntax added after upstream wrote the
	// mirror.
	RegexOther
)

// regexErrorKindText is the text of each kind of RegexErrorKind that holds
// no data, as upstream writes it.
var regexErrorKindText = [...]string{
	RegexCaptureLimitExceeded:                 "exceeded the maximum number of capturing groups (4294967295)",
	RegexClassEscapeInvalid:                   "invalid escape sequence found in character class",
	RegexClassRangeInvalid:                    "invalid character class range, the start must be <= the end",
	RegexClassRangeLiteral:                    "invalid range boundary, must be a literal",
	RegexClassUnclosed:                        "unclosed character class",
	RegexDecimalEmpty:                         "decimal literal empty",
	RegexDecimalInvalid:                       "decimal literal invalid",
	RegexEscapeHexEmpty:                       "hexadecimal literal empty",
	RegexEscapeHexInvalid:                     "hexadecimal literal is not a Unicode scalar value",
	RegexEscapeHexInvalidDigit:                "invalid hexadecimal digit",
	RegexEscapeUnexpectedEOF:                  "incomplete escape sequence, reached end of pattern prematurely",
	RegexEscapeUnrecognized:                   "unrecognized escape sequence",
	RegexFlagDanglingNegation:                 "dangling flag negation operator",
	RegexFlagUnexpectedEOF:                    "expected flag but got end of regex",
	RegexFlagUnrecognized:                     "unrecognized flag",
	RegexGroupNameEmpty:                       "empty capture group name",
	RegexGroupNameInvalid:                     "invalid capture group character",
	RegexGroupNameUnexpectedEOF:               "unclosed capture group name",
	RegexGroupUnclosed:                        "unclosed group",
	RegexGroupUnopened:                        "unopened group",
	RegexRepetitionCountInvalid:               "invalid repetition count range, the start must be <= the end",
	RegexRepetitionCountDecimalEmpty:          "repetition quantifier expects a valid decimal",
	RegexRepetitionCountUnclosed:              "unclosed counted repetition",
	RegexRepetitionMissing:                    "repetition operator missing expression",
	RegexSpecialWordBoundaryUnclosed:          "special word boundary assertion is either unclosed or contains an invalid character",
	RegexSpecialWordBoundaryUnrecognized:      "unrecognized special word boundary assertion, valid choices are: start, end, start-half or end-half",
	RegexSpecialWordOrRepetitionUnexpectedEOF: "found either the beginning of a special word boundary or a bounded repetition on a \\b with an opening brace, but no closing brace",
	RegexUnicodeClassInvalid:                  "invalid Unicode character class",
	RegexUnsupportedBackreference:             "backreferences are not supported",
	RegexUnsupportedLookAround:                "look-around, including look-ahead and look-behind, is not supported",
	RegexUnicodeNotAllowed:                    "Unicode not allowed here",
	RegexInvalidUTF8:                          "pattern can match invalid UTF-8",
	RegexInvalidLineTerminator:                "invalid line terminator, must be ASCII",
	RegexUnicodePropertyNotFound:              "Unicode property not found",
	RegexUnicodePropertyValueNotFound:         "Unicode property value not found",
	RegexUnicodePerlClassNotFound:             "Unicode-aware Perl class not found (make sure the unicode-perl feature is enabled)",
	RegexUnicodeCaseUnavailable:               "Unicode-aware case insensitivity matching is not available (make sure the unicode-case feature is enabled)",
	RegexFlagDuplicate:                        "duplicate flag",
	RegexFlagRepeatedNegation:                 "flag negation operator repeated",
	RegexGroupNameDuplicate:                   "duplicate capture group name",
}

// astErrorKinds maps each kind of error of the parser to its mirror. A kind
// that is not in the map is RegexOther.
//
// astErrorKinds is the From<&ast::ErrorKind> of RegexErrorKind.
var astErrorKinds = map[ast.ErrorKind]RegexErrorKind{
	ast.CaptureLimitExceeded:                 RegexCaptureLimitExceeded,
	ast.ClassEscapeInvalid:                   RegexClassEscapeInvalid,
	ast.ClassRangeInvalid:                    RegexClassRangeInvalid,
	ast.ClassRangeLiteral:                    RegexClassRangeLiteral,
	ast.ClassUnclosed:                        RegexClassUnclosed,
	ast.DecimalEmpty:                         RegexDecimalEmpty,
	ast.DecimalInvalid:                       RegexDecimalInvalid,
	ast.EscapeHexEmpty:                       RegexEscapeHexEmpty,
	ast.EscapeHexInvalid:                     RegexEscapeHexInvalid,
	ast.EscapeHexInvalidDigit:                RegexEscapeHexInvalidDigit,
	ast.EscapeUnexpectedEOF:                  RegexEscapeUnexpectedEOF,
	ast.EscapeUnrecognized:                   RegexEscapeUnrecognized,
	ast.FlagDanglingNegation:                 RegexFlagDanglingNegation,
	ast.FlagUnexpectedEOF:                    RegexFlagUnexpectedEOF,
	ast.FlagUnrecognized:                     RegexFlagUnrecognized,
	ast.GroupNameEmpty:                       RegexGroupNameEmpty,
	ast.GroupNameInvalid:                     RegexGroupNameInvalid,
	ast.GroupNameUnexpectedEOF:               RegexGroupNameUnexpectedEOF,
	ast.GroupUnclosed:                        RegexGroupUnclosed,
	ast.GroupUnopened:                        RegexGroupUnopened,
	ast.RepetitionCountInvalid:               RegexRepetitionCountInvalid,
	ast.RepetitionCountDecimalEmpty:          RegexRepetitionCountDecimalEmpty,
	ast.RepetitionCountUnclosed:              RegexRepetitionCountUnclosed,
	ast.RepetitionMissing:                    RegexRepetitionMissing,
	ast.SpecialWordBoundaryUnclosed:          RegexSpecialWordBoundaryUnclosed,
	ast.SpecialWordBoundaryUnrecognized:      RegexSpecialWordBoundaryUnrecognized,
	ast.SpecialWordOrRepetitionUnexpectedEOF: RegexSpecialWordOrRepetitionUnexpectedEOF,
	ast.UnicodeClassInvalid:                  RegexUnicodeClassInvalid,
	ast.UnsupportedBackreference:             RegexUnsupportedBackreference,
	ast.UnsupportedLookAround:                RegexUnsupportedLookAround,
	ast.FlagDuplicate:                        RegexFlagDuplicate,
	ast.FlagRepeatedNegation:                 RegexFlagRepeatedNegation,
	ast.GroupNameDuplicate:                   RegexGroupNameDuplicate,
	ast.NestLimitExceeded:                    RegexNestLimitExceeded,
}

// hirErrorKinds maps each kind of error of the translator to its mirror. A
// kind that is not in the map is RegexOther.
//
// hirErrorKinds is the From<&hir::ErrorKind> of RegexErrorKind.
var hirErrorKinds = map[hir.ErrorKind]RegexErrorKind{
	hir.UnicodeNotAllowed:            RegexUnicodeNotAllowed,
	hir.InvalidUTF8:                  RegexInvalidUTF8,
	hir.InvalidLineTerminator:        RegexInvalidLineTerminator,
	hir.UnicodePropertyNotFound:      RegexUnicodePropertyNotFound,
	hir.UnicodePropertyValueNotFound: RegexUnicodePropertyValueNotFound,
	hir.UnicodePerlClassNotFound:     RegexUnicodePerlClassNotFound,
	hir.UnicodeCaseUnavailable:       RegexUnicodeCaseUnavailable,
}

// PatternSpan is a range of bytes in a pattern.
//
// PatternSpan is PatternSpan.
type PatternSpan struct {
	Start uint32
	End   uint32
}

// newPatternSpan returns the byte range of a span.
//
// newPatternSpan is PatternSpan::new.
func newPatternSpan(span ast.Span) *PatternSpan {
	return &PatternSpan{Start: uint32(span.Start.Offset), End: uint32(span.End.Offset)}
}

// RegexError is a pattern that parsePattern rejects, with the pattern and
// the range in it that is wrong.
//
// RegexError is RegexError.
type RegexError struct {
	Pattern string
	Kind    RegexErrorKind
	// NestLimit is the limit of RegexNestLimitExceeded.
	NestLimit uint32
	// Other is the text of RegexOther.
	Other string
	// Span is the range that is wrong, or nil.
	Span *PatternSpan
	// AuxSpan is a second range, such as the first use of a duplicate
	// flag, or nil.
	AuxSpan *PatternSpan
}

// newRegexError returns the RegexError of an error of the parser or of the
// translator.
//
// newRegexError is RegexError::new.
func newRegexError(err error) *RegexError {
	if pe, ok := errors.AsType[*ast.Error](err); ok {
		re := &RegexError{Pattern: pe.Pattern, Span: newPatternSpan(pe.Span)}
		kind, ok := astErrorKinds[pe.Kind]
		switch {
		case !ok:
			re.Kind, re.Other = RegexOther, pe.Error()
		case kind == RegexNestLimitExceeded:
			re.Kind, re.NestLimit = kind, pe.Limit
		default:
			re.Kind = kind
		}
		if aux, ok := pe.AuxiliarySpan(); ok {
			re.AuxSpan = newPatternSpan(aux)
		}
		return re
	}
	if te, ok := errors.AsType[*hir.Error](err); ok {
		re := &RegexError{Pattern: te.Pattern, Span: newPatternSpan(te.Span)}
		if kind, ok := hirErrorKinds[te.Kind]; ok {
			re.Kind = kind
		} else {
			re.Kind, re.Other = RegexOther, te.Kind.String()
		}
		return re
	}
	return &RegexError{Kind: RegexOther, Other: err.Error()}
}

// column returns the number of characters before a byte offset of the
// pattern.
//
// column is RegexError::column.
func (e *RegexError) column(offset uint32) int {
	return utf8.RuneCountInString(e.Pattern[:offset])
}

// kindText returns the text of the kind of the error.
func (e *RegexError) kindText() string {
	switch e.Kind {
	case RegexNestLimitExceeded:
		return "exceed the maximum number of nested parentheses/brackets (" + strconv.FormatUint(uint64(e.NestLimit), 10) + ")"
	case RegexOther:
		return e.Other
	}
	return regexErrorKindText[e.Kind]
}

// Error returns the text of the error: the pattern, a line of carets under
// the ranges that are wrong, and the kind.
//
// Error is the Display of RegexError.
func (e *RegexError) Error() string {
	var b strings.Builder
	b.WriteString("regex parse error:\n")
	b.WriteString("    " + e.Pattern + "\n")
	if e.Span != nil {
		spans := []*PatternSpan{e.Span, e.AuxSpan}
		// upstream sorts the two spans, with no span first
		slices.SortFunc(spans, func(a, c *PatternSpan) int {
			switch {
			case a == nil && c == nil:
				return 0
			case a == nil:
				return -1
			case c == nil:
				return 1
			}
			return cmp.Or(cmp.Compare(a.Start, c.Start), cmp.Compare(a.End, c.End))
		})
		b.WriteString("    ")
		column := 0
		for _, span := range spans {
			if span == nil {
				continue
			}
			start := e.column(span.Start)
			width := max(e.column(span.End)-start, 1)
			pad := max(start-column, 0)
			b.WriteString(strings.Repeat(" ", pad) + strings.Repeat("^", width))
			column = max(column, start) + width
		}
		b.WriteString("\n")
	}
	b.WriteString("error: " + e.kindText())
	return b.String()
}
