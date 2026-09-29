package transit

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// This file ports the query part of lib/binding_rust/lib.rs: the types
// Query, QueryCursor, QueryMatch, QueryCapture, QueryProperty,
// QueryPredicate and QueryError, the parsing of the predicates in
// Query::from_raw_parts, and their evaluation in
// QueryMatch::satisfies_text_predicates (D27). The Rust binding wraps
// TSQuery and TSQueryCursor, and the Go types wrap query and queryCursor of
// query.go in the same way.
//
// These parts have no Go form. The TextProvider of Rust is the text of the
// tree, a []byte. Query::deep_clone copies a query with ts_query_copy, which
// Go does not port. The StreamingIterator of QueryMatches and QueryCaptures
// is an iter.Seq and an iter.Seq2, and the options with a progress callback
// are the context of Matches and Captures (D25).
//
// #match? compiles its pattern with the package regexp, and not with the
// Rust crate regex (D27). The predicates of Neovim, such as #lua-match?, are
// general predicates, as they are in the Rust binding (D69).

// Query is a compiled query. It is safe to share between goroutines, after
// any call to DisablePattern or DisableCapture.
//
// Query is Query of the Rust binding.
type Query struct {
	ptr                *query
	captureNames       []string
	captureQuantifiers [][]Quantifier
	textPredicates     [][]textPredicateCapture
	propertySettings   [][]QueryProperty
	propertyPredicates [][]QueryPropertyPredicate
	generalPredicates  [][]QueryPredicate
}

// QueryCursor runs a query on a tree. It belongs to one goroutine at a time.
// Matches and Captures evaluate the text predicates of the query (D27).
//
// QueryCursor is QueryCursor of the Rust binding.
type QueryCursor struct {
	ptr *queryCursor

	// captures holds the captures of the last match, which the sequences of
	// the cursor give out.
	captures []QueryCapture
}

// QueryProperty is a key and a value of #set!, #is? or #is-not? in a
// pattern.
//
// QueryProperty is QueryProperty of the Rust binding.
type QueryProperty struct {
	Key string
	// Value is the value of the property. HasValue is false when the
	// predicate names no value.
	Value    string
	HasValue bool
	// CaptureID is the index of the capture that the predicate names, or -1
	// when it names none.
	CaptureID int
}

// QueryPropertyPredicate is a property of #is?, which is positive, or of
// #is-not?, which is not.
//
// QueryPropertyPredicate is the tuple (QueryProperty, bool) of
// property_predicates of the Rust binding.
type QueryPropertyPredicate struct {
	Property QueryProperty
	Positive bool
}

// QueryPredicateArg is an argument of a general predicate: a capture or a
// string.
//
// QueryPredicateArg is QueryPredicateArg of the Rust binding, an enum of
// Capture and String.
type QueryPredicateArg struct {
	// IsCapture is true for a capture, and then Capture is its index. For a
	// string, Value is the string.
	IsCapture bool
	Capture   int
	Value     string
}

// QueryPredicate is a predicate that the query does not evaluate: its
// operator, such as "lua-match?", and its arguments.
//
// QueryPredicate is QueryPredicate of the Rust binding.
type QueryPredicate struct {
	Operator string
	Args     []QueryPredicateArg
}

// QueryMatch is one match of a pattern. Captures is valid until the
// sequence that gave the match moves on, because the cursor reuses it, as
// the Rust binding reuses the match of its streaming iterator. Copy it to
// keep it.
//
// QueryMatch is QueryMatch of the Rust binding.
type QueryMatch struct {
	PatternIndex int
	Captures     []QueryCapture
}

// QueryCapture is one captured node. Index is its place in CaptureNames.
//
// QueryCapture is QueryCapture of the Rust binding.
type QueryCapture struct {
	Node  Node
	Index int
}

// QueryError says where a query fails to compile.
//
// QueryError is QueryError of the Rust binding.
type QueryError struct {
	Offset  int
	Row     int
	Column  int
	Kind    QueryErrorKind
	Message string
}

// The names of the two predicates that the parser of the predicates tests
// most.
const (
	predicateEq    = "eq?"
	predicateMatch = "match?"
)

// textPredicateKind is the variant of TextPredicateCapture.
type textPredicateKind uint8

// The variants of TextPredicateCapture.
const (
	// textPredicateEqString is EqString.
	textPredicateEqString textPredicateKind = iota
	// textPredicateEqCapture is EqCapture.
	textPredicateEqCapture
	// textPredicateMatchString is MatchString.
	textPredicateMatchString
	// textPredicateAnyString is AnyString.
	textPredicateAnyString
)

// String returns the name of the variant.
func (k textPredicateKind) String() string {
	switch k {
	case textPredicateEqString:
		return "eq string"
	case textPredicateEqCapture:
		return "eq capture"
	case textPredicateMatchString:
		return "match string"
	case textPredicateAnyString:
		return "any string"
	}
	return unknownName
}

// textPredicateCapture is TextPredicateCapture, an enum of Rust. The first
// capture is the capture that the predicate tests. isPositive is false for a
// not- predicate, and matchAll is false for an any- predicate.
type textPredicateCapture struct {
	kind         textPredicateKind
	capture      uint32
	otherCapture uint32
	value        string
	re           *regexp.Regexp
	values       []string
	isPositive   bool
	matchAll     bool
}

// NewQuery compiles a query from a text of one or more patterns, for a
// language. The error is a *QueryError.
//
// NewQuery is Query::new, with Query::new_raw and Query::from_raw_parts.
func NewQuery(language *Language, source string) (*Query, error) {
	ptr, err := newQueryRaw(language, source)
	if err != nil {
		return nil, err
	}
	q, qerr := queryFromRawParts(ptr, source)
	if qerr != nil {
		return nil, qerr
	}
	return q, nil
}

// newQueryRaw is Query::new_raw. It compiles the query, and it turns the
// offset and the kind of an error into a QueryError.
func newQueryRaw(language *Language, source string) (*query, *QueryError) {
	// Compile the query.
	ptr, errorOffset, errorType := newQuery(language, source)
	if ptr != nil {
		return ptr, nil
	}

	// On failure, build an error based on the error code and offset.
	if errorType == QueryErrorLanguage {
		return nil, &QueryError{
			Row:    0,
			Column: 0,
			Offset: 0,
			Message: fmt.Sprintf("Incompatible language version %d. Expected minimum %d, maximum %d",
				language.ABIVersion(), minCompatibleLanguageVersion, languageVersion),
			Kind: QueryErrorLanguage,
		}
	}

	offset := int(errorOffset)
	lineStart := 0
	row := 0
	lineContainingError := ""
	foundLine := false
	for _, line := range rustLines(source) {
		lineEnd := lineStart + len(line) + 1
		if lineEnd > offset {
			lineContainingError = line
			foundLine = true
			break
		}
		lineStart = lineEnd
		row++
	}
	column := offset - lineStart

	var message string
	var kind QueryErrorKind
	switch errorType {
	// Error types that report names
	case QueryErrorNodeType, QueryErrorField, QueryErrorCapture:
		suffix := source[offset:]
		inQuotes := offset > 0 && source[offset-1] == '"'
		backslashes := 0
		endOffset := len(suffix)
		for i, c := range suffix {
			var end bool
			if inQuotes {
				switch {
				case c == '"' && backslashes%2 == 0:
					end = true
				case c == '\\':
					backslashes++
				default:
					backslashes = 0
				}
			} else {
				end = !isAlphanumeric(c) && c != '_' && c != '-'
			}
			if end {
				endOffset = i
				break
			}
		}
		message = `"` + suffix[:endOffset] + `"`
		kind = errorType

	// Error types that report positions
	default:
		if foundLine {
			message = lineContainingError + "\n" + strings.Repeat(" ", offset-lineStart) + "^"
		} else {
			message = "Unexpected EOF"
		}
		if errorType == QueryErrorStructure {
			kind = QueryErrorStructure
		} else {
			kind = QueryErrorSyntax
		}
	}

	return nil, &QueryError{
		Row:     row,
		Column:  column,
		Offset:  offset,
		Message: message,
		Kind:    kind,
	}
}

// rustLines splits a text into lines as str::lines of Rust does: at each
// "\n", with one "\r" before it removed, and with no empty line after a
// final "\n".
func rustLines(s string) []string {
	var lines []string
	for s != "" {
		line, rest, found := strings.Cut(s, "\n")
		if found {
			line = strings.TrimSuffix(line, "\r")
		}
		lines = append(lines, line)
		s = rest
	}
	return lines
}

// isAlphanumeric is char::is_alphanumeric of Rust: a letter or a number of
// Unicode.
func isAlphanumeric(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsNumber(c)
}

// queryFromRawParts is Query::from_raw_parts. It reads the capture names,
// the quantifiers and the predicates of each pattern, and it sorts each
// predicate into a text predicate, a property setting, a property predicate
// or a general predicate.
func queryFromRawParts(ptr *query, source string) (*Query, *QueryError) {
	stringCount := ptr.stringCount()
	captureCount := ptr.captureCount()
	patternCount := ptr.patternCount()

	captureNames := make([]string, 0, captureCount)
	captureQuantifiersVec := make([][]Quantifier, 0, patternCount)
	textPredicatesVec := make([][]textPredicateCapture, 0, patternCount)
	propertyPredicatesVec := make([][]QueryPropertyPredicate, 0, patternCount)
	propertySettingsVec := make([][]QueryProperty, 0, patternCount)
	generalPredicatesVec := make([][]QueryPredicate, 0, patternCount)

	// Build a vector of strings to store the capture names.
	for i := range captureCount {
		captureNames = append(captureNames, ptr.captureNameForID(i))
	}

	// Build a vector to store capture quantifiers.
	for i := range patternCount {
		captureQuantifiers := make([]Quantifier, 0, captureCount)
		for j := range captureCount {
			captureQuantifiers = append(captureQuantifiers, ptr.captureQuantifierForID(i, j))
		}
		captureQuantifiersVec = append(captureQuantifiersVec, captureQuantifiers)
	}

	// Build a vector of strings to represent literal values used in predicates.
	stringValues := make([]string, 0, stringCount)
	for i := range stringCount {
		stringValues = append(stringValues, ptr.stringValueForID(i))
	}

	// Build a vector of predicates for each pattern.
	for i := range patternCount {
		predicateSteps := ptr.predicatesForPattern(i)

		byteOffset := int(ptr.startByteForPattern(i))
		row := strings.Count(source[:min(byteOffset, len(source))], "\n")

		var textPredicates []textPredicateCapture
		var propertyPredicates []QueryPropertyPredicate
		var propertySettings []QueryProperty
		var generalPredicates []QueryPredicate
		for p := range splitPredicateSteps(predicateSteps) {
			if len(p) == 0 {
				continue
			}

			if p[0].typ != queryPredicateStepTypeString {
				return nil, predicateError(row, fmt.Sprintf(
					"Expected predicate to start with a function name. Got @%s.",
					captureNames[p[0].valueID],
				))
			}

			// Build a predicate for each of the known predicate function names.
			operatorName := stringValues[p[0].valueID]
			switch operatorName {
			case predicateEq, "not-eq?", "any-eq?", "any-not-eq?":
				if len(p) != 3 {
					return nil, predicateError(row, fmt.Sprintf(
						"Wrong number of arguments to #eq? predicate. Expected 2, got %d.",
						len(p)-1,
					))
				}
				if p[1].typ != queryPredicateStepTypeCapture {
					return nil, predicateError(row, fmt.Sprintf(
						"First argument to #eq? predicate must be a capture name. Got literal \"%s\".",
						stringValues[p[1].valueID],
					))
				}

				isPositive := operatorName == predicateEq || operatorName == "any-eq?"
				matchAll := operatorName == predicateEq || operatorName == "not-eq?"
				if p[2].typ == queryPredicateStepTypeCapture {
					textPredicates = append(textPredicates, textPredicateCapture{
						kind:         textPredicateEqCapture,
						capture:      p[1].valueID,
						otherCapture: p[2].valueID,
						isPositive:   isPositive,
						matchAll:     matchAll,
					})
				} else {
					textPredicates = append(textPredicates, textPredicateCapture{
						kind:       textPredicateEqString,
						capture:    p[1].valueID,
						value:      stringValues[p[2].valueID],
						isPositive: isPositive,
						matchAll:   matchAll,
					})
				}

			case predicateMatch, "not-match?", "any-match?", "any-not-match?":
				if len(p) != 3 {
					return nil, predicateError(row, fmt.Sprintf(
						"Wrong number of arguments to #match? predicate. Expected 2, got %d.",
						len(p)-1,
					))
				}
				if p[1].typ != queryPredicateStepTypeCapture {
					return nil, predicateError(row, fmt.Sprintf(
						"First argument to #match? predicate must be a capture name. Got literal \"%s\".",
						stringValues[p[1].valueID],
					))
				}
				if p[2].typ == queryPredicateStepTypeCapture {
					return nil, predicateError(row, fmt.Sprintf(
						"Second argument to #match? predicate must be a literal. Got capture @%s.",
						captureNames[p[2].valueID],
					))
				}

				isPositive := operatorName == predicateMatch || operatorName == "any-match?"
				matchAll := operatorName == predicateMatch || operatorName == "not-match?"
				pattern := stringValues[p[2].valueID]
				re, err := regexp.Compile(pattern)
				if err != nil {
					return nil, predicateError(row, fmt.Sprintf("Invalid regex '%s'", pattern))
				}
				textPredicates = append(textPredicates, textPredicateCapture{
					kind:       textPredicateMatchString,
					capture:    p[1].valueID,
					re:         re,
					isPositive: isPositive,
					matchAll:   matchAll,
				})

			case "set!":
				property, qerr := parseProperty(row, operatorName, captureNames, stringValues, p[1:])
				if qerr != nil {
					return nil, qerr
				}
				propertySettings = append(propertySettings, property)

			case "is?", "is-not?":
				property, qerr := parseProperty(row, operatorName, captureNames, stringValues, p[1:])
				if qerr != nil {
					return nil, qerr
				}
				propertyPredicates = append(propertyPredicates, QueryPropertyPredicate{
					Property: property,
					Positive: operatorName == "is?",
				})

			case "any-of?", "not-any-of?":
				if len(p) < 2 {
					return nil, predicateError(row, fmt.Sprintf(
						"Wrong number of arguments to #any-of? predicate. Expected at least 1, got %d.",
						len(p)-1,
					))
				}
				if p[1].typ != queryPredicateStepTypeCapture {
					return nil, predicateError(row, fmt.Sprintf(
						"First argument to #any-of? predicate must be a capture name. Got literal \"%s\".",
						stringValues[p[1].valueID],
					))
				}

				isPositive := operatorName == "any-of?"
				values := make([]string, 0, len(p)-2)
				for _, arg := range p[2:] {
					if arg.typ == queryPredicateStepTypeCapture {
						return nil, predicateError(row, fmt.Sprintf(
							"Arguments to #any-of? predicate must be literals. Got capture @%s.",
							captureNames[arg.valueID],
						))
					}
					values = append(values, stringValues[arg.valueID])
				}
				textPredicates = append(textPredicates, textPredicateCapture{
					kind:       textPredicateAnyString,
					capture:    p[1].valueID,
					values:     values,
					isPositive: isPositive,
				})

			default:
				args := make([]QueryPredicateArg, 0, len(p)-1)
				for _, a := range p[1:] {
					if a.typ == queryPredicateStepTypeCapture {
						args = append(args, QueryPredicateArg{IsCapture: true, Capture: int(a.valueID)})
					} else {
						args = append(args, QueryPredicateArg{Value: stringValues[a.valueID]})
					}
				}
				generalPredicates = append(generalPredicates, QueryPredicate{
					Operator: operatorName,
					Args:     args,
				})
			}
		}

		textPredicatesVec = append(textPredicatesVec, textPredicates)
		propertyPredicatesVec = append(propertyPredicatesVec, propertyPredicates)
		propertySettingsVec = append(propertySettingsVec, propertySettings)
		generalPredicatesVec = append(generalPredicatesVec, generalPredicates)
	}

	return &Query{
		ptr:                ptr,
		captureNames:       captureNames,
		captureQuantifiers: captureQuantifiersVec,
		textPredicates:     textPredicatesVec,
		propertyPredicates: propertyPredicatesVec,
		propertySettings:   propertySettingsVec,
		generalPredicates:  generalPredicatesVec,
	}, nil
}

// splitPredicateSteps is the split of the steps of a pattern at each step of
// the type Done, as slice::split of Rust does. It gives an empty slice
// between two Done steps and after a final one, as Rust does.
func splitPredicateSteps(steps []queryPredicateStep) iter.Seq[[]queryPredicateStep] {
	return func(yield func([]queryPredicateStep) bool) {
		start := 0
		for i, s := range steps {
			if s.typ == queryPredicateStepTypeDone {
				if !yield(steps[start:i]) {
					return
				}
				start = i + 1
			}
		}
		yield(steps[start:])
	}
}

// StartByteForPattern returns the byte offset where a pattern starts in the
// text of the query. It panics for a pattern that the query does not have,
// as the Rust binding does.
//
// StartByteForPattern is Query::start_byte_for_pattern.
func (q *Query) StartByteForPattern(patternIndex int) int {
	q.checkPatternIndex(patternIndex)
	return int(q.ptr.startByteForPattern(uint32(patternIndex)))
}

// EndByteForPattern returns the byte offset where a pattern ends in the text
// of the query. It panics for a pattern that the query does not have, as the
// Rust binding does.
//
// EndByteForPattern is Query::end_byte_for_pattern.
func (q *Query) EndByteForPattern(patternIndex int) int {
	q.checkPatternIndex(patternIndex)
	return int(q.ptr.endByteForPattern(uint32(patternIndex)))
}

// checkPatternIndex is the assert! of start_byte_for_pattern and
// end_byte_for_pattern.
func (q *Query) checkPatternIndex(patternIndex int) {
	if patternIndex < 0 || patternIndex >= len(q.textPredicates) {
		panic(fmt.Sprintf("Pattern index is %d but the pattern count is %d", patternIndex, len(q.textPredicates)))
	}
}

// PatternCount returns the number of patterns of the query.
//
// PatternCount is Query::pattern_count.
func (q *Query) PatternCount() int {
	return int(q.ptr.patternCount())
}

// CaptureNames returns the names of the captures of the query. The index of
// a name is the Index of a QueryCapture. The slice is the query's own, so do
// not change it.
//
// CaptureNames is Query::capture_names.
func (q *Query) CaptureNames() []string {
	return q.captureNames
}

// CaptureQuantifiers returns the quantifier of each capture in a pattern.
//
// CaptureQuantifiers is Query::capture_quantifiers.
func (q *Query) CaptureQuantifiers(index int) []Quantifier {
	return q.captureQuantifiers[index]
}

// CaptureIndexForName returns the index of a capture name, and reports
// whether the query has it.
//
// CaptureIndexForName is Query::capture_index_for_name.
func (q *Query) CaptureIndexForName(name string) (int, bool) {
	i := slices.Index(q.captureNames, name)
	return i, i >= 0
}

// PropertyPredicates returns the properties that a pattern tests, with #is?
// and #is-not?.
//
// PropertyPredicates is Query::property_predicates.
func (q *Query) PropertyPredicates(index int) []QueryPropertyPredicate {
	return q.propertyPredicates[index]
}

// PropertySettings returns the properties that a pattern sets, with #set!.
//
// PropertySettings is Query::property_settings.
func (q *Query) PropertySettings(index int) []QueryProperty {
	return q.propertySettings[index]
}

// GeneralPredicates returns the predicates of a pattern that the query does
// not evaluate: each operator but #eq?, #match?, #any-of?, #is?, #is-not?,
// #set! and their not- and any- forms.
//
// GeneralPredicates is Query::general_predicates.
func (q *Query) GeneralPredicates(index int) []QueryPredicate {
	return q.generalPredicates[index]
}

// DisableCapture makes the query give no capture with a name. The query
// then does less work.
//
// DisableCapture is Query::disable_capture.
func (q *Query) DisableCapture(name string) {
	q.ptr.disableCapture(name)
}

// DisablePattern makes a pattern of the query match nothing. The query then
// does less work.
//
// DisablePattern is Query::disable_pattern.
func (q *Query) DisablePattern(index int) {
	q.ptr.disablePattern(uint32(index))
}

// IsPatternRooted reports whether a pattern has a single root node.
//
// IsPatternRooted is Query::is_pattern_rooted.
func (q *Query) IsPatternRooted(index int) bool {
	return q.ptr.isPatternRooted(uint32(index))
}

// IsPatternNonLocal reports whether a pattern is non-local: whether a match
// of it can start in a node that is outside the range of the cursor.
//
// IsPatternNonLocal is Query::is_pattern_non_local.
func (q *Query) IsPatternNonLocal(index int) bool {
	return q.ptr.isPatternNonLocal(uint32(index))
}

// IsPatternGuaranteedAtStep reports whether the step at a byte offset of the
// text of the query is definite: whether its pattern is sure to match once
// the cursor reaches it.
//
// IsPatternGuaranteedAtStep is Query::is_pattern_guaranteed_at_step.
func (q *Query) IsPatternGuaranteedAtStep(byteOffset int) bool {
	return q.ptr.isPatternGuaranteedAtStep(uint32(byteOffset))
}

// parseProperty is Query::parse_property.
func parseProperty(
	row int,
	functionName string,
	captureNames []string,
	stringValues []string,
	args []queryPredicateStep,
) (QueryProperty, *QueryError) {
	if len(args) == 0 || len(args) > 3 {
		return QueryProperty{}, predicateError(row, fmt.Sprintf(
			"Wrong number of arguments to %s predicate. Expected 1 to 3, got %d.",
			functionName, len(args),
		))
	}

	captureID := -1
	var key, value string
	hasKey, hasValue := false, false

	for _, arg := range args {
		switch {
		case arg.typ == queryPredicateStepTypeCapture:
			if captureID >= 0 {
				return QueryProperty{}, predicateError(row, fmt.Sprintf(
					"Invalid arguments to %s predicate. Unexpected second capture name @%s",
					functionName, captureNames[arg.valueID],
				))
			}
			captureID = int(arg.valueID)
		case !hasKey:
			key, hasKey = stringValues[arg.valueID], true
		case !hasValue:
			value, hasValue = stringValues[arg.valueID], true
		default:
			return QueryProperty{}, predicateError(row, fmt.Sprintf(
				"Invalid arguments to %s predicate. Unexpected third argument @%s",
				functionName, stringValues[arg.valueID],
			))
		}
	}

	if !hasKey {
		return QueryProperty{}, predicateError(row, fmt.Sprintf(
			"Invalid arguments to %s predicate. Missing key argument", functionName,
		))
	}
	return QueryProperty{Key: key, Value: value, HasValue: hasValue, CaptureID: captureID}, nil
}

// NewQueryCursor returns a cursor that runs queries.
//
// NewQueryCursor is QueryCursor::new.
func NewQueryCursor() *QueryCursor {
	return &QueryCursor{ptr: newQueryCursor()}
}

// MatchLimit returns the largest number of matches that the cursor keeps in
// progress at one time.
//
// MatchLimit is QueryCursor::match_limit.
func (c *QueryCursor) MatchLimit() int {
	return int(c.ptr.matchLimit())
}

// SetMatchLimit sets the largest number of matches that the cursor keeps in
// progress at one time. The limit must be above 0 and at most 65536.
//
// SetMatchLimit is QueryCursor::set_match_limit.
func (c *QueryCursor) SetMatchLimit(limit int) {
	c.ptr.setMatchLimit(uint32(limit))
}

// DidExceedMatchLimit reports whether the last run of the cursor had more
// matches in progress than its limit.
//
// DidExceedMatchLimit is QueryCursor::did_exceed_match_limit.
func (c *QueryCursor) DidExceedMatchLimit() bool {
	return c.ptr.didExceedMatchLimit()
}

// Matches runs a query on the node n and its descendants, and gives each
// match in the order that the cursor finds it. src is the text of the tree.
// A match whose text predicates fail is left out.
//
// Each range of the sequence runs the query from the start. When ctx ends,
// the sequence ends, so a caller that needs to know reads ctx.Err().
//
// The Rust binding has a fault, which the port keeps: #any-eq?, #any-not-eq?,
// #any-match? and #any-not-match? hold when no node of their capture holds
// them, so they keep every match.
//
// Matches is QueryCursor::matches and QueryCursor::matches_with_options,
// with the iterator QueryMatches.
func (c *QueryCursor) Matches(ctx context.Context, q *Query, n Node, src []byte) iter.Seq[QueryMatch] {
	return func(yield func(QueryMatch) bool) {
		c.ptr.exec(q.ptr, n)
		for {
			m, ok := c.ptr.nextMatch(ctx)
			if !ok {
				return
			}
			result := c.newMatch(m)
			if result.satisfiesTextPredicates(q, src) {
				if !yield(result) {
					return
				}
			}
		}
	}
}

// Captures runs a query on the node n and its descendants, and gives each
// capture in the order of the text, with the match that holds it. The int
// is the index of the capture in the Captures of the match. src is the text
// of the tree. A match whose text predicates fail is left out, and removed
// from the cursor.
//
// Each range of the sequence runs the query from the start. When ctx ends,
// the sequence ends, so a caller that needs to know reads ctx.Err().
//
// Captures is QueryCursor::captures and QueryCursor::captures_with_options,
// with the iterator QueryCaptures.
func (c *QueryCursor) Captures(ctx context.Context, q *Query, n Node, src []byte) iter.Seq2[QueryMatch, int] {
	return func(yield func(QueryMatch, int) bool) {
		c.ptr.exec(q.ptr, n)
		for {
			m, captureIndex, ok := c.ptr.nextCapture(ctx)
			if !ok {
				return
			}
			result := c.newMatch(m)
			if result.satisfiesTextPredicates(q, src) {
				if !yield(result, int(captureIndex)) {
					return
				}
				continue
			}
			c.ptr.removeMatch(m.id)
		}
	}
}

// SetByteRange makes the cursor look only for matches that meet the range
// of bytes from start to end.
//
// SetByteRange is QueryCursor::set_byte_range.
func (c *QueryCursor) SetByteRange(start, end int) {
	c.ptr.setByteRange(uint32(start), uint32(end))
}

// SetPointRange makes the cursor look only for matches that meet the range
// of points from start to end.
//
// SetPointRange is QueryCursor::set_point_range.
func (c *QueryCursor) SetPointRange(start, end Point) {
	c.ptr.setPointRange(start.internal(), end.internal())
}

// SetContainingByteRange makes the cursor give only the matches whose nodes
// are all inside the range of bytes from start to end. It works with
// SetByteRange, for example to find the matches that meet row 5000 and lie
// inside rows 4500 to 5500.
//
// SetContainingByteRange is QueryCursor::set_containing_byte_range.
func (c *QueryCursor) SetContainingByteRange(start, end int) {
	c.ptr.setContainingByteRange(uint32(start), uint32(end))
}

// SetContainingPointRange makes the cursor give only the matches whose
// nodes are all inside the range of points from start to end.
//
// SetContainingPointRange is QueryCursor::set_containing_point_range.
func (c *QueryCursor) SetContainingPointRange(start, end Point) {
	c.ptr.setContainingPointRange(start.internal(), end.internal())
}

// SetMaxStartDepth makes the cursor start a match only at a node whose depth
// below the node of the run is at most depth. A depth of 0 starts a match
// only at that node. The other nodes of a pattern can still be at any depth.
// A negative depth removes the limit.
//
// SetMaxStartDepth is QueryCursor::set_max_start_depth, where None is a
// negative depth.
func (c *QueryCursor) SetMaxStartDepth(depth int) {
	if depth < 0 {
		c.ptr.setMaxStartDepth(math.MaxUint32)
		return
	}
	c.ptr.setMaxStartDepth(uint32(depth))
}

// newMatch is QueryMatch::new. It copies the captures of a match into the
// buffer of the cursor, which the match then shares.
func (c *QueryCursor) newMatch(m queryMatch) QueryMatch {
	c.captures = c.captures[:0]
	for _, capture := range m.captures {
		c.captures = append(c.captures, QueryCapture{Node: capture.node, Index: int(capture.index)})
	}
	return QueryMatch{PatternIndex: int(m.patternIndex), Captures: c.captures}
}

// nodesForCaptureIndex is QueryMatch::nodes_for_capture_index.
func (m QueryMatch) nodesForCaptureIndex(captureIndex uint32) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, capture := range m.Captures {
			if capture.Index == int(captureIndex) && !yield(capture.Node) {
				return
			}
		}
	}
}

// nodeText returns the text of a node, as the TextProvider of a byte slice
// of the Rust binding gives it.
func nodeText(src []byte, n Node) []byte {
	return src[n.StartByte():n.EndByte()]
}

// satisfiesTextPredicates is QueryMatch::satisfies_text_predicates, with the
// text of the tree as the text provider.
func (m QueryMatch) satisfiesTextPredicates(q *Query, src []byte) bool {
	for _, predicate := range q.textPredicates[m.PatternIndex] {
		if !m.satisfiesTextPredicate(predicate, src) {
			return false
		}
	}
	return true
}

// satisfiesTextPredicate is the closure of all in
// satisfies_text_predicates: the result of one text predicate.
func (m QueryMatch) satisfiesTextPredicate(predicate textPredicateCapture, src []byte) bool {
	switch predicate.kind {
	case textPredicateEqCapture:
		nextNode1, stop1 := iter.Pull(m.nodesForCaptureIndex(predicate.capture))
		defer stop1()
		nextNode2, stop2 := iter.Pull(m.nodesForCaptureIndex(predicate.otherCapture))
		defer stop2()
		for {
			node1, ok1 := nextNode1()
			node2, ok2 := nextNode2()
			if !ok1 || !ok2 {
				// the result of Rust: both sequences end at the same node
				return !ok1 && !ok2
			}
			isPositiveMatch := bytes.Equal(nodeText(src, node1), nodeText(src, node2))
			if isPositiveMatch != predicate.isPositive && predicate.matchAll {
				return false
			}
			if isPositiveMatch == predicate.isPositive && !predicate.matchAll {
				return true
			}
		}

	case textPredicateEqString:
		for node := range m.nodesForCaptureIndex(predicate.capture) {
			isPositiveMatch := string(nodeText(src, node)) == predicate.value
			if isPositiveMatch != predicate.isPositive && predicate.matchAll {
				return false
			}
			if isPositiveMatch == predicate.isPositive && !predicate.matchAll {
				return true
			}
		}
		return true

	case textPredicateMatchString:
		for node := range m.nodesForCaptureIndex(predicate.capture) {
			isPositiveMatch := predicate.re.Match(nodeText(src, node))
			if isPositiveMatch != predicate.isPositive && predicate.matchAll {
				return false
			}
			if isPositiveMatch == predicate.isPositive && !predicate.matchAll {
				return true
			}
		}
		return true

	case textPredicateAnyString:
		for node := range m.nodesForCaptureIndex(predicate.capture) {
			text := string(nodeText(src, node))
			if slices.Contains(predicate.values, text) != predicate.isPositive {
				return false
			}
		}
		return true
	}
	return true
}

// predicateError is predicate_error of the Rust binding.
func predicateError(row int, message string) *QueryError {
	return &QueryError{
		Kind:    QueryErrorPredicate,
		Row:     row,
		Column:  0,
		Offset:  0,
		Message: message,
	}
}

// Error returns the text of the error, as the Display of QueryError of the
// Rust binding writes it.
//
// Error is the Display of QueryError.
func (e *QueryError) Error() string {
	var msg string
	switch e.Kind {
	case QueryErrorField:
		msg = "Invalid field name "
	case QueryErrorNodeType:
		msg = "Invalid node type "
	case QueryErrorCapture:
		msg = "Invalid capture name "
	case QueryErrorPredicate:
		msg = "Invalid predicate: "
	case QueryErrorStructure:
		msg = "Impossible pattern:\n"
	case QueryErrorSyntax:
		msg = "Invalid syntax:\n"
	}
	if msg == "" {
		return e.Message
	}
	return fmt.Sprintf("Query error at %d:%d. %s%s", e.Row+1, e.Column+1, msg, e.Message)
}
