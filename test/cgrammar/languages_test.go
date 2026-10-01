package cgrammar

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// This file gives the tests of phase 3 the languages that they test: the
// tables of the C grammar of a fixture grammar in the Go runtime, and the
// grammar package that the Go backend writes for it. Phase 4 ends when
// every test of phase 3 passes on the grammar packages too (docs/PLAN.md).
//
// A test that loops over the fixture grammars, and that has a reference of
// its own, such as the C runtime or the Rust oracle, gets both languages
// from fixtureLanguages, and it runs the reference once for both. A test
// that gets one language from fixtureGrammar, such as a ported test of
// upstream, runs a second time under TestGoPackageTests or
// TestGoPackageParallelTests. There, fixtureGrammar gives the grammar
// package in place of the C tables.

// testLanguage is a language that a test runs in the Go runtime, and the
// name of where its tables come from: "C" or "Go".
type testLanguage struct {
	source   string
	language *transit.Language
}

// goPackageLanguage returns the language of the grammar package of a
// fixture grammar. It stops the test when goPackages has no package for the
// grammar, because phase 4 gives each fixture grammar a package.
func goPackageLanguage(t *testing.T, name string) *transit.Language {
	t.Helper()
	if l := findGoPackage(name); l != nil {
		return l
	}
	t.Fatalf("the fixture grammar %s has no grammar package in goPackages", name)
	return nil
}

// findGoPackage returns the language of the grammar package of a grammar,
// or nil when goPackages has no package for it.
func findGoPackage(name string) *transit.Language {
	for _, gp := range goPackages {
		if gp.name == name {
			return gp.language()
		}
	}
	return nil
}

// fixtureLanguages returns the two languages of a fixture grammar that Load
// opened: its C tables, and its grammar package.
func fixtureLanguages(t *testing.T, g *Grammar) []testLanguage {
	t.Helper()
	return []testLanguage{{"C", g.Language}, {"Go", goPackageLanguage(t, g.Name)}}
}

// goPackageRunners are the names of the tests that run other tests on the
// grammar packages.
var goPackageRunners = []string{"TestGoPackageTests", "TestGoPackageParallelTests"}

// onGoPackages reports whether a test runs under one of goPackageRunners,
// as a subtest or as a subtest of a subtest. fixtureGrammar then gives the
// grammar package.
func onGoPackages(t *testing.T) bool {
	t.Helper()
	runner, _, _ := strings.Cut(t.Name(), "/")
	return strings.Contains(t.Name(), "/") && slices.Contains(goPackageRunners, runner)
}

// namedTest is a test function and its name.
type namedTest struct {
	name string
	fn   func(*testing.T)
}

// goPackageTests are the tests that get a language from fixtureGrammar, and
// that do not call t.Parallel. TestGoPackageTestsAreComplete makes sure
// that the two lists hold each such test.
var goPackageTests = []namedTest{
	// language_test.go
	{"TestLookaheadIterator", TestLookaheadIterator},
	{"TestLookaheadIteratorExhaustion", TestLookaheadIteratorExhaustion},
	{"TestSymbolMetadataChecks", TestSymbolMetadataChecks},
	{"TestSupertypes", TestSupertypes},
	// upstream_node_test.go
	{"TestNodeChild", TestNodeChild},
	{"TestNodeChildren", TestNodeChildren},
	{"TestNodeChildrenByFieldName", TestNodeChildrenByFieldName},
	{"TestNodeParentOfChildByFieldName", TestNodeParentOfChildByFieldName},
	{"TestParentOfZeroWidthNode", TestParentOfZeroWidthNode},
	{"TestFirstChildForOffset", TestFirstChildForOffset},
	{"TestFirstNamedChildForOffset", TestFirstNamedChildForOffset},
	{"TestNodeFieldNameForChild", TestNodeFieldNameForChild},
	{"TestNodeFieldNameForNamedChild", TestNodeFieldNameForNamedChild},
	{"TestNodeChildByFieldNameWithExtraHiddenChildren", TestNodeChildByFieldNameWithExtraHiddenChildren},
	{"TestNodeNamedChild", TestNodeNamedChild},
	{"TestNodeDescendantCount", TestNodeDescendantCount},
	{"TestDescendantCountSingleNodeTree", TestDescendantCountSingleNodeTree},
	{"TestNodeDescendantForRange", TestNodeDescendantForRange},
	{"TestNodeEdit", TestNodeEdit},
	{"TestRootNodeWithOffset", TestRootNodeWithOffset},
	{"TestNodeIsExtra", TestNodeIsExtra},
	{"TestNodeIsError", TestNodeIsError},
	{"TestNodeSexp", TestNodeSexp},
	{"TestNodeNumericSymbolsRespectSimpleAliases", TestNodeNumericSymbolsRespectSimpleAliases},
	{"TestHiddenZeroWidthNodeWithVisibleChild", TestHiddenZeroWidthNodeWithVisibleChild},
	// upstream_parser_test.go
	{"TestParsingSimpleString", TestParsingSimpleString},
	{"TestParsingWithLogging", TestParsingWithLogging},
	{"TestParsingWithDebugGraphEnabled", TestParsingWithDebugGraphEnabled},
	{"TestParsingWithCustomUTF8Input", TestParsingWithCustomUTF8Input},
	{"TestParsingWithCustomUTF16leInput", TestParsingWithCustomUTF16leInput},
	{"TestParsingWithCustomUTF16BeInput", TestParsingWithCustomUTF16BeInput},
	{"TestParsingWithCallbackReturningOwnedStrings", TestParsingWithCallbackReturningOwnedStrings},
	{"TestParsingTextWithByteOrderMark", TestParsingTextWithByteOrderMark},
	{"TestParsingInvalidCharsAtEOF", TestParsingInvalidCharsAtEOF},
	{"TestParsingUnexpectedNullCharactersWithinSource", TestParsingUnexpectedNullCharactersWithinSource},
	{"TestParsingEndsWhenInputCallbackReturnsEmpty", TestParsingEndsWhenInputCallbackReturnsEmpty},
	{"TestParsingAfterEditingBeginningOfCode", TestParsingAfterEditingBeginningOfCode},
	{"TestParsingAfterEditingEndOfCode", TestParsingAfterEditingEndOfCode},
	{"TestParsingEmptyFileWithReusedTree", TestParsingEmptyFileWithReusedTree},
	{"TestParsingAfterDetectingErrorInTheMiddleOfAStringToken", TestParsingAfterDetectingErrorInTheMiddleOfAStringToken},
	{"TestParsingOnMultipleThreads", TestParsingOnMultipleThreads},
	{"TestParsingCancelledByAnotherThread", TestParsingCancelledByAnotherThread},
	{"TestParsingWithATimeout", TestParsingWithATimeout},
	{"TestParsingWithATimeoutAndAReset", TestParsingWithATimeoutAndAReset},
	{"TestParsingWithATimeoutAndImplicitReset", TestParsingWithATimeoutAndImplicitReset},
	{"TestParsingWithTimeoutAndNoCompletion", TestParsingWithTimeoutAndNoCompletion},
	{"TestParsingWithTimeoutDuringBalancing", TestParsingWithTimeoutDuringBalancing},
	{"TestParsingWithTimeoutWhenErrorDetected", TestParsingWithTimeoutWhenErrorDetected},
	{"TestParsingWithOneIncludedRange", TestParsingWithOneIncludedRange},
	{"TestParsingWithMultipleIncludedRanges", TestParsingWithMultipleIncludedRanges},
	{"TestParsingWithIncludedRangeContainingMismatchedPositions", TestParsingWithIncludedRangeContainingMismatchedPositions},
	{"TestParsingUTF16CodeWithErrorsAtTheEndOfAnIncludedRange", TestParsingUTF16CodeWithErrorsAtTheEndOfAnIncludedRange},
	{"TestParsingWithExternalScannerThatUsesIncludedRangeBoundaries", TestParsingWithExternalScannerThatUsesIncludedRangeBoundaries},
	{"TestParsingWithANewlyExcludedRange", TestParsingWithANewlyExcludedRange},
	{"TestParsingWithANewlyIncludedRange", TestParsingWithANewlyIncludedRange},
	{"TestParseStackRecursiveMergeErrorCostCalculationBug", TestParseStackRecursiveMergeErrorCostCalculationBug},
	{"TestParsingByHaltingAtOffset", TestParsingByHaltingAtOffset},
	{"TestParseOptionsReborrow", TestParseOptionsReborrow},
	// upstream_pathological_test.go
	{"TestPathologicalExample1", TestPathologicalExample1},
	// upstream_query_test.go
	{"TestQueryErrorsOnInvalidSyntax", TestQueryErrorsOnInvalidSyntax},
	{"TestQueryErrorsOnAnchorAtGroupEdge", TestQueryErrorsOnAnchorAtGroupEdge},
	{"TestQueryErrorsOnInvalidSymbols", TestQueryErrorsOnInvalidSymbols},
	{"TestQueryErrorsOnInvalidPredicates", TestQueryErrorsOnInvalidPredicates},
	{"TestQueryErrorsOnImpossiblePatterns", TestQueryErrorsOnImpossiblePatterns},
	{"TestQueryVerifiesPossiblePatternsWithAliasedParentNodes", TestQueryVerifiesPossiblePatternsWithAliasedParentNodes},
	{"TestQueryMatchesWithSimplePattern", TestQueryMatchesWithSimplePattern},
	{"TestQueryMatchesWithMultipleOnSameRoot", TestQueryMatchesWithMultipleOnSameRoot},
	{"TestQueryMatchesWithMultiplePatternsDifferentRoots", TestQueryMatchesWithMultiplePatternsDifferentRoots},
	{"TestQueryMatchesWithMultiplePatternsSameRoot", TestQueryMatchesWithMultiplePatternsSameRoot},
	{"TestQueryMatchesWithNestingAndNoFields", TestQueryMatchesWithNestingAndNoFields},
	{"TestQueryMatchesWithManyResults", TestQueryMatchesWithManyResults},
	{"TestQueryMatchesWithManyOverlappingResults", TestQueryMatchesWithManyOverlappingResults},
	{"TestQueryMatchesCapturingErrorNodes", TestQueryMatchesCapturingErrorNodes},
	{"TestQueryMatchesCapturingMissingNodes", TestQueryMatchesCapturingMissingNodes},
	{"TestQueryMatchesWithExtraChildren", TestQueryMatchesWithExtraChildren},
	{"TestQueryMatchesWithNamedWildcard", TestQueryMatchesWithNamedWildcard},
	{"TestQueryMatchesWithWildcardAtTheRoot", TestQueryMatchesWithWildcardAtTheRoot},
	{"TestQueryMatchesWithWildcardWithinWildcard", TestQueryMatchesWithWildcardWithinWildcard},
	{"TestQueryMatchesWithImmediateSiblings", TestQueryMatchesWithImmediateSiblings},
	{"TestQueryMatchesWithAnchorAfterZeroQuantifier", TestQueryMatchesWithAnchorAfterZeroQuantifier},
	{"TestQueryMatchesWithAnchorAfterNestedZeroQuantifier", TestQueryMatchesWithAnchorAfterNestedZeroQuantifier},
	{"TestQueryMatchesWithLastChildAnchorAfterOptional", TestQueryMatchesWithLastChildAnchorAfterOptional},
	{"TestQueryMatchesWithAnchorsOnBothSidesOfZeroQuantifier", TestQueryMatchesWithAnchorsOnBothSidesOfZeroQuantifier},
	{"TestQueryMatchesWithLeadingAnchorBeforeZeroQuantifier", TestQueryMatchesWithLeadingAnchorBeforeZeroQuantifier},
	{"TestQueryMatchesWithLastNamedChild", TestQueryMatchesWithLastNamedChild},
	{"TestQueryMatchesWithNegatedFields", TestQueryMatchesWithNegatedFields},
	{"TestQueryMatchesWithFieldAtRoot", TestQueryMatchesWithFieldAtRoot},
	{"TestQueryMatchesWithRepeatedLeafNodes", TestQueryMatchesWithRepeatedLeafNodes},
	{"TestQueryMatchesOptionalCaptureBeforeUncapturedRequiredSibling", TestQueryMatchesOptionalCaptureBeforeUncapturedRequiredSibling},
	{"TestQueryMatchesWithOptionalNodesInsideOfRepetitions", TestQueryMatchesWithOptionalNodesInsideOfRepetitions},
	{"TestQueryMatchesWithTopLevelRepetitions", TestQueryMatchesWithTopLevelRepetitions},
	{"TestQueryMatchesWithNonTerminalRepetitionsWithinRoot", TestQueryMatchesWithNonTerminalRepetitionsWithinRoot},
	{"TestQueryMatchesWithNestedRepetitions", TestQueryMatchesWithNestedRepetitions},
	{"TestQueryMatchesWithMultipleRepetitionPatternsThatIntersectOtherPattern", TestQueryMatchesWithMultipleRepetitionPatternsThatIntersectOtherPattern},
	{"TestQueryMatchesWithTrailingRepetitionsOfLastChild", TestQueryMatchesWithTrailingRepetitionsOfLastChild},
	{"TestQueryMatchesWithLeadingZeroOrMoreRepeatedLeafNodes", TestQueryMatchesWithLeadingZeroOrMoreRepeatedLeafNodes},
	{"TestMatchesWithAnchorSiblingInsideParent", TestMatchesWithAnchorSiblingInsideParent},
	{"TestMatchesWithAnchorSiblingWithQuantifierInsideParent", TestMatchesWithAnchorSiblingWithQuantifierInsideParent},
	{"TestMatchesWithAnchorSiblingWithQuantifierCapturedInsideParent", TestMatchesWithAnchorSiblingWithQuantifierCapturedInsideParent},
	{"TestMatchesAnchoredQuantifiedSiblingInsideParent", TestMatchesAnchoredQuantifiedSiblingInsideParent},
	{"TestQueryMatchesWithTrailingOptionalNodes", TestQueryMatchesWithTrailingOptionalNodes},
	{"TestQueryMatchesWithNestedOptionalNodes", TestQueryMatchesWithNestedOptionalNodes},
	{"TestQueryMatchesWithRepeatedInternalNodes", TestQueryMatchesWithRepeatedInternalNodes},
	{"TestQueryMatchesWithSimpleAlternatives", TestQueryMatchesWithSimpleAlternatives},
	{"TestQueryMatchesWithAlternativesInRepetitions", TestQueryMatchesWithAlternativesInRepetitions},
	{"TestQueryMatchesWithAlternativesAtRoot", TestQueryMatchesWithAlternativesAtRoot},
	{"TestQueryMatchesWithAlternativesUnderFields", TestQueryMatchesWithAlternativesUnderFields},
	{"TestQueryMatchesInLanguageWithSimpleAliases", TestQueryMatchesInLanguageWithSimpleAliases},
	{"TestQueryMatchesWithDifferentTokensWithTheSameStringValue", TestQueryMatchesWithDifferentTokensWithTheSameStringValue},
	{"TestQueryMatchesWithTooManyPermutationsToTrack", TestQueryMatchesWithTooManyPermutationsToTrack},
	{"TestQuerySiblingPatternsDontMatchChildrenOfAnError", TestQuerySiblingPatternsDontMatchChildrenOfAnError},
	{"TestQueryMatchesWithAlternativesAndTooManyPermutationsToTrack", TestQueryMatchesWithAlternativesAndTooManyPermutationsToTrack},
	{"TestRepetitionsBeforeWithAlternatives", TestRepetitionsBeforeWithAlternatives},
	{"TestQueryMatchesWithAnonymousTokens", TestQueryMatchesWithAnonymousTokens},
	{"TestQueryMatchesWithSupertypes", TestQueryMatchesWithSupertypes},
	{"TestQueryMatchesWithinByteRange", TestQueryMatchesWithinByteRange},
	{"TestQueryMatchesWithinPointRange", TestQueryMatchesWithinPointRange},
	{"TestQueryCapturesWithinByteRange", TestQueryCapturesWithinByteRange},
	{"TestQueryCursorNextCaptureWithByteRange", TestQueryCursorNextCaptureWithByteRange},
	{"TestQueryCursorNextCaptureWithPointRange", TestQueryCursorNextCaptureWithPointRange},
	{"TestQueryMatchesWithUnrootedPatternsIntersectingByteRange", TestQueryMatchesWithUnrootedPatternsIntersectingByteRange},
	{"TestQueryMatchesWithWildcardAtRootIntersectingByteRange", TestQueryMatchesWithWildcardAtRootIntersectingByteRange},
	{"TestQueryCapturesWithinByteRangeAssignedAfterIterating", TestQueryCapturesWithinByteRangeAssignedAfterIterating},
	{"TestQueryMatchesWithinRangeOfLongRepetition", TestQueryMatchesWithinRangeOfLongRepetition},
	{"TestQueryMatchesContainedWithinRange", TestQueryMatchesContainedWithinRange},
	{"TestQueryMatchesDifferentQueriesSameCursor", TestQueryMatchesDifferentQueriesSameCursor},
	{"TestQueryMatchesWithMultipleCapturesOnANode", TestQueryMatchesWithMultipleCapturesOnANode},
	{"TestQueryMatchesWithCapturedWildcardAtRoot", TestQueryMatchesWithCapturedWildcardAtRoot},
	{"TestQueryMatchesWithNoCaptures", TestQueryMatchesWithNoCaptures},
	{"TestQueryMatchesWithRepeatedFields", TestQueryMatchesWithRepeatedFields},
	{"TestQueryMatchesWithDeeplyNestedPatternsWithFields", TestQueryMatchesWithDeeplyNestedPatternsWithFields},
	{"TestQueryAlternationWithInnerQuantifier", TestQueryAlternationWithInnerQuantifier},
	{"TestQueryAlternationWithOuterQuantifier", TestQueryAlternationWithOuterQuantifier},
	{"TestQueryMatchesWithAlternationsAndPredicates", TestQueryMatchesWithAlternationsAndPredicates},
	{"TestQueryMatchesWithIndefiniteStepContainingNoCaptures", TestQueryMatchesWithIndefiniteStepContainingNoCaptures},
	{"TestQueryCapturesBasic", TestQueryCapturesBasic},
	{"TestQueryCapturesWithTextConditions", TestQueryCapturesWithTextConditions},
	{"TestQueryCapturesWithPredicates", TestQueryCapturesWithPredicates},
	{"TestQueryCapturesWithQuotedPredicateArgs", TestQueryCapturesWithQuotedPredicateArgs},
	{"TestQueryCapturesWithDuplicates", TestQueryCapturesWithDuplicates},
	{"TestQueryCapturesWithManyNestedResultsWithoutFields", TestQueryCapturesWithManyNestedResultsWithoutFields},
	{"TestQueryCapturesWithManyNestedResultsWithFields", TestQueryCapturesWithManyNestedResultsWithFields},
	{"TestQueryCapturesWithTooManyNestedResults", TestQueryCapturesWithTooManyNestedResults},
	{"TestQueryCapturesWithDefinitePatternContainingManyNestedMatches", TestQueryCapturesWithDefinitePatternContainingManyNestedMatches},
	{"TestQueryCapturesOrderedByBothStartAndEndPositions", TestQueryCapturesOrderedByBothStartAndEndPositions},
	{"TestQueryCapturesWithMatchesRemoved", TestQueryCapturesWithMatchesRemoved},
	{"TestQueryCapturesWithMatchesRemovedBeforeTheyFinish", TestQueryCapturesWithMatchesRemovedBeforeTheyFinish},
	{"TestQueryCapturesAndMatchesIteratorsAreFused", TestQueryCapturesAndMatchesIteratorsAreFused},
	{"TestQueryTextCallbackReturnsChunks", TestQueryTextCallbackReturnsChunks},
	{"TestQueryStartEndByteForPattern", TestQueryStartEndByteForPattern},
	{"TestQueryCaptureNames", TestQueryCaptureNames},
	{"TestQueryLifetimeIsSeparateFromNodesLifetime", TestQueryLifetimeIsSeparateFromNodesLifetime},
	{"TestQueryWithNoPatterns", TestQueryWithNoPatterns},
	{"TestQueryComments", TestQueryComments},
	{"TestQueryDisablePattern", TestQueryDisablePattern},
	{"TestQueryDeepClone", TestQueryDeepClone},
	{"TestQueryAlternativePredicatePrefix", TestQueryAlternativePredicatePrefix},
	{"TestQueryRandom", TestQueryRandom},
	{"TestQueryIsPatternGuaranteedAtStep", TestQueryIsPatternGuaranteedAtStep},
	{"TestQueryIsPatternRooted", TestQueryIsPatternRooted},
	{"TestQueryIsPatternNonLocal", TestQueryIsPatternNonLocal},
	{"TestCaptureQuantifiers", TestCaptureQuantifiers},
	{"TestQueryQuantifiedCaptures", TestQueryQuantifiedCaptures},
	{"TestQueryMaxStartDepth", TestQueryMaxStartDepth},
	{"TestQueryErrorDoesNotOOB", TestQueryErrorDoesNotOOB},
	{"TestConsecutiveZeroOrModifiers", TestConsecutiveZeroOrModifiers},
	{"TestQueryMaxStartDepthMore", TestQueryMaxStartDepthMore},
	{"TestQueryWithFirstChildInGroupIsAnchor", TestQueryWithFirstChildInGroupIsAnchor},
	{"TestQueryCompilerOOBAccess", TestQueryCompilerOOBAccess},
	{"TestQueryWildcardWithImmediateFirstChild", TestQueryWildcardWithImmediateFirstChild},
	{"TestQueryOnEmptySourceCode", TestQueryOnEmptySourceCode},
	{"TestQueryExecutionWithTimeout", TestQueryExecutionWithTimeout},
	{"TestQueryProgressCallbackLivesAsLongAsMatches", TestQueryProgressCallbackLivesAsLongAsMatches},
	{"TestQueryExecutionWithPointsCausingUnderflow", TestQueryExecutionWithPointsCausingUnderflow},
	{"TestWildcardBehaviorBeforeAnchor", TestWildcardBehaviorBeforeAnchor},
	{"TestPatternAlternativesFollowLastChildConstraint", TestPatternAlternativesFollowLastChildConstraint},
	{"TestWildcardParentAllowsFallibleChildPatterns", TestWildcardParentAllowsFallibleChildPatterns},
	{"TestUnfinishedCapturesAreNotDefiniteWithPendingAnchors", TestUnfinishedCapturesAreNotDefiniteWithPendingAnchors},
	{"TestQueryWithPredicateCausingOOBAccess", TestQueryWithPredicateCausingOOBAccess},
	{"TestQueryAllowsErrorNodesWithChildren", TestQueryAllowsErrorNodesWithChildren},
	{"TestLastChildAnchorLooksPastHiddenNode", TestLastChildAnchorLooksPastHiddenNode},
	// upstream_tree_test.go
	{"TestTreeEdit", TestTreeEdit},
	{"TestTreeEditWithIncludedRanges", TestTreeEditWithIncludedRanges},
	{"TestTreeCursor", TestTreeCursor},
	{"TestTreeCursorPreviousSibling", TestTreeCursorPreviousSibling},
	{"TestTreeCursorFields", TestTreeCursorFields},
	{"TestTreeCursorChildForPoint", TestTreeCursorChildForPoint},
	{"TestTreeNodeEquality", TestTreeNodeEquality},
	{"TestGetChangedRanges", TestGetChangedRanges},
	{"TestConsistencyWithMidCodepointEdit", TestConsistencyWithMidCodepointEdit},
	{"TestTreeCursorOnAliasedRootWithExtraChild", TestTreeCursorOnAliasedRootWithExtraChild},
}

// goPackageParallelTests are the tests that get a language from
// fixtureGrammar, and that call t.Parallel.
var goPackageParallelTests = []namedTest{
	// upstream_corpus_test.go
	{"TestCorpusForBashLanguage", TestCorpusForBashLanguage},
	{"TestCorpusForCLanguage", TestCorpusForCLanguage},
	{"TestCorpusForCppLanguage", TestCorpusForCppLanguage},
	{"TestCorpusForEmbeddedTemplateLanguage", TestCorpusForEmbeddedTemplateLanguage},
	{"TestCorpusForGoLanguage", TestCorpusForGoLanguage},
	{"TestCorpusForHTMLLanguage", TestCorpusForHTMLLanguage},
	{"TestCorpusForJavaLanguage", TestCorpusForJavaLanguage},
	{"TestCorpusForJavascriptLanguage", TestCorpusForJavascriptLanguage},
	{"TestCorpusForJSONLanguage", TestCorpusForJSONLanguage},
	{"TestCorpusForPHPLanguage", TestCorpusForPHPLanguage},
	{"TestCorpusForPythonLanguage", TestCorpusForPythonLanguage},
	{"TestCorpusForRubyLanguage", TestCorpusForRubyLanguage},
	{"TestCorpusForRustLanguage", TestCorpusForRustLanguage},
	{"TestCorpusForTypescriptLanguage", TestCorpusForTypescriptLanguage},
	{"TestCorpusForTsxLanguage", TestCorpusForTsxLanguage},
}

// TestGoPackageTests runs each test of goPackageTests with the grammar
// packages. It does not call t.Parallel, so that the tests run one after
// the other, as they do on the C tables. Some of them measure time.
func TestGoPackageTests(t *testing.T) {
	for _, test := range goPackageTests {
		t.Run(test.name, test.fn)
	}
}

// TestGoPackageParallelTests runs each test of goPackageParallelTests with
// the grammar packages, next to the other parallel tests.
func TestGoPackageParallelTests(t *testing.T) {
	t.Parallel()
	for _, test := range goPackageParallelTests {
		t.Run(test.name, test.fn)
	}
}

// TestGoPackageTestsAreComplete reads the test files of the package, and
// finds each test that calls fixtureGrammar, directly or through other
// functions of the package. Each one must be in goPackageTests or in
// goPackageParallelTests, by whether it calls t.Parallel, and the lists
// must hold no other test.
func TestGoPackageTestsAreComplete(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string][]string{}
	parallel := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					calls[name] = append(calls[name], fun.Name)
				case *ast.IndexExpr:
					if id, ok := fun.X.(*ast.Ident); ok {
						calls[name] = append(calls[name], id.Name)
					}
				}
				return true
			})
			// A call of t.Parallel in a function literal, such as a
			// subtest, is not a call of the test.
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" {
						parallel[name] = true
					}
				}
				return true
			})
		}
	}

	// reaches holds the functions that call fixtureGrammar, and parallel
	// the functions that call t.Parallel, directly or through a function
	// that they call.
	reaches := map[string]bool{"fixtureGrammar": true}
	for changed := true; changed; {
		changed = false
		for name, callees := range calls {
			for _, callee := range callees {
				if reaches[callee] && !reaches[name] {
					reaches[name], changed = true, true
				}
				if parallel[callee] && !parallel[name] {
					parallel[name], changed = true, true
				}
			}
		}
	}

	// listName is the name of the list that a test belongs in.
	listName := func(name string) string {
		if parallel[name] {
			return "goPackageParallelTests"
		}
		return "goPackageTests"
	}
	listed := map[string]bool{}
	for _, list := range []struct {
		name  string
		tests []namedTest
	}{{"goPackageTests", goPackageTests}, {"goPackageParallelTests", goPackageParallelTests}} {
		for _, test := range list.tests {
			listed[test.name] = true
			switch {
			case !reaches[test.name]:
				t.Errorf("%s does not call fixtureGrammar, so it must not be in %s", test.name, list.name)
			case listName(test.name) != list.name:
				t.Errorf("%s is in %s, and it must be in %s", test.name, list.name, listName(test.name))
			}
		}
	}
	var missing []string
	for name := range reaches {
		if strings.HasPrefix(name, "Test") && !listed[name] {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	for _, name := range missing {
		t.Errorf("%s calls fixtureGrammar, so it must be in %s", name, listName(name))
	}
}
