package cgrammar

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/transit"
)

// This file ports crates/cli/src/tests/query_test.rs of upstream (D35),
// with the helpers of upstream_query_helpers_test.go.

// This file ports the tests of crates/cli/src/tests/query_test.rs of
// upstream, from test_query_errors_on_invalid_syntax to
// test_query_matches_with_wildcard_within_wildcard.

func TestQueryErrorsOnInvalidSyntax(t *testing.T) {
	language := uqLanguage(t, "javascript")

	if _, err := transit.NewQuery(language, "(if_statement)"); err != nil {
		t.Errorf("NewQuery(%q): %v", "(if_statement)", err)
	}
	if _, err := transit.NewQuery(
		language,
		"(if_statement condition:(parenthesized_expression (identifier)))",
	); err != nil {
		t.Errorf("NewQuery: %v", err)
	}

	// Mismatched parens
	uqCheckQueryErrorMessage(t, language, "(if_statement", uqLines(
		"(if_statement", //
		"             ^",
	))
	uqCheckQueryErrorMessage(t, language, "; comment 1\n; comment 2\n  (if_statement))", uqLines(
		"  (if_statement))", //
		"                ^",
	))

	// Return an error at the *beginning* of a bare identifier not followed a colon.
	// If there's a colon but no pattern, return an error at the end of the colon.
	uqCheckQueryErrorMessage(t, language, "(if_statement identifier)", uqLines(
		"(if_statement identifier)", //
		"              ^",
	))
	uqCheckQueryErrorMessage(t, language, "(if_statement condition:)", uqLines(
		"(if_statement condition:)", //
		"                        ^",
	))

	// Return an error at the beginning of an unterminated string.
	uqCheckQueryErrorMessage(t, language, `(identifier) "h `, uqLines(
		`(identifier) "h `, //
		`             ^`,
	))

	// Empty tree pattern
	uqCheckQueryErrorMessage(t, language, `((identifier) ()`, uqLines(
		"((identifier) ()", //
		"               ^",
	))

	// Empty alternation
	uqCheckQueryErrorMessage(t, language, `((identifier) [])`, uqLines(
		"((identifier) [])", //
		"               ^",
	))

	// Unclosed sibling expression with predicate
	uqCheckQueryErrorMessage(t, language, `((identifier) (#a?)`, uqLines(
		"((identifier) (#a?)", //
		"                   ^",
	))

	// Predicate not ending in `?` or `!`
	uqCheckQueryErrorMessage(t, language, `((identifier) (#a))`, uqLines(
		"((identifier) (#a))", //
		"                 ^",
	))

	// Unclosed predicate
	uqCheckQueryErrorMessage(t, language, `((identifier) @x (#eq? @x a`, uqLines(
		`((identifier) @x (#eq? @x a`,
		`                           ^`,
	))

	// Need at least one child node for a child anchor
	uqCheckQueryErrorMessage(t, language, `(statement_block .)`, uqLines(
		//
		`(statement_block .)`,
		`                  ^`,
	))

	// Need a field name after a negated field operator
	uqCheckQueryErrorMessage(t, language, `(statement_block ! (if_statement))`, uqLines(
		`(statement_block ! (if_statement))`,
		`                   ^`,
	))

	// Unclosed alternation within a tree
	// tree-sitter/tree-sitter/issues/968
	uqCheckQueryErrorMessage(t, uqLanguage(t, "c"), `(parameter_list [ ")" @foo)`, uqLines(
		`(parameter_list [ ")" @foo)`,
		`                          ^`,
	))

	// Unclosed tree within an alternation
	// tree-sitter/tree-sitter/issues/1436
	uqCheckQueryErrorMessage(
		t,
		uqLanguage(t, "python"),
		`[(unary_operator (_) @operand) (not_operator (_) @operand]`,
		uqLines(
			`[(unary_operator (_) @operand) (not_operator (_) @operand]`,
			`                                                         ^`,
		),
	)

	// MISSING keyword with full pattern
	uqCheckQueryErrorMessage(
		t,
		uqLanguage(t, "c"),
		`(MISSING (function_declarator (identifier))) `,
		uqLines(
			`(MISSING (function_declarator (identifier))) `,
			`         ^`,
		),
	)

	// MISSING keyword with multiple identifiers
	uqCheckQueryErrorMessage(
		t,
		uqLanguage(t, "c"),
		`(MISSING function_declarator function_declarator) `,
		uqLines(
			`(MISSING function_declarator function_declarator) `,
			`                             ^`,
		),
	)
	uqCheckQueryError(t, language, "(statement / export_statement)", transit.QueryError{
		Row:    0,
		Offset: 11,
		Column: 11,
		Kind:   transit.QueryErrorSyntax,
		Message: uqLines(
			"(statement / export_statement)", //
			"           ^",
		),
	})
}

func TestQueryErrorsOnAnchorAtGroupEdge(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// Anchors between siblings, or at the first/last position of a *node*
	// pattern, are valid.
	for _, s := range []string{
		"((_) . (_))",
		"(program (_) (_) .)",
		"(program (_)* @x . (_))",
	} {
		if _, err := transit.NewQuery(language, s); err != nil {
			t.Errorf("NewQuery(%q): %v", s, err)
		}
	}

	// A `.` at the edge of a *group* is rejected. A group is not a node, so it
	// has no last child to anchor, and there is no sibling within the group to
	// anchor to.
	uqCheckQueryError(t, language, "((_) .)", transit.QueryError{
		Row:    0,
		Offset: 5,
		Column: 5,
		Kind:   transit.QueryErrorSyntax,
		Message: uqLines(
			"((_) .)", //
			"     ^",
		),
	})
	uqCheckQueryError(t, language, "(program ((_)+ .)? (_))", transit.QueryError{
		Row:    0,
		Offset: 15,
		Column: 15,
		Kind:   transit.QueryErrorSyntax,
		Message: uqLines(
			"(program ((_)+ .)? (_))", //
			"               ^",
		),
	})
}

func TestQueryErrorsOnInvalidSymbols(t *testing.T) {
	language := uqLanguage(t, "javascript")

	uqCheckQueryError(t, language, "\">>>>\"", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\">>>>\"",
	})
	uqCheckQueryError(t, language, "\"te\\\"st\"", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"te\\\"st\"",
	})
	uqCheckQueryError(t, language, "\"\\\\\" @cap", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"\\\\\"",
	})
	uqCheckQueryError(t, language, "(clas)", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"clas\"",
	})
	uqCheckQueryError(t, language, "(if_statement (arrayyyyy))", transit.QueryError{
		Row:     0,
		Offset:  15,
		Column:  15,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"arrayyyyy\"",
	})
	uqCheckQueryError(t, language, "(if_statement condition: (non_existent3))", transit.QueryError{
		Row:     0,
		Offset:  26,
		Column:  26,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"non_existent3\"",
	})
	uqCheckQueryError(t, language, "(if_statement condit: (identifier))", transit.QueryError{
		Row:     0,
		Offset:  14,
		Column:  14,
		Kind:    transit.QueryErrorField,
		Message: "\"condit\"",
	})
	uqCheckQueryError(t, language, "(if_statement conditioning: (identifier))", transit.QueryError{
		Row:     0,
		Offset:  14,
		Column:  14,
		Kind:    transit.QueryErrorField,
		Message: "\"conditioning\"",
	})
	uqCheckQueryError(t, language, "(if_statement !alternativ)", transit.QueryError{
		Row:     0,
		Offset:  15,
		Column:  15,
		Kind:    transit.QueryErrorField,
		Message: "\"alternativ\"",
	})
	uqCheckQueryError(t, language, "(if_statement !alternatives)", transit.QueryError{
		Row:     0,
		Offset:  15,
		Column:  15,
		Kind:    transit.QueryErrorField,
		Message: "\"alternatives\"",
	})
	uqCheckQueryError(t, language, "fakefield: (identifier)", transit.QueryError{
		Row:     0,
		Offset:  0,
		Column:  0,
		Kind:    transit.QueryErrorField,
		Message: "\"fakefield\"",
	})
	uqCheckQueryError(t, language, "(ERR)", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"ERR\"",
	})
	uqCheckQueryError(t, language, "(MISS)", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"MISS\"",
	})
}

func TestQueryErrorsOnInvalidPredicates(t *testing.T) {
	language := uqLanguage(t, "javascript")

	uqCheckQueryError(t, language, "((identifier) @id (@id))", transit.QueryError{
		Kind:   transit.QueryErrorSyntax,
		Row:    0,
		Column: 19,
		Offset: 19,
		Message: uqLines(
			"((identifier) @id (@id))", //
			"                   ^",
		),
	})
	uqCheckQueryError(t, language, "((identifier) @id (#eq? @id))", transit.QueryError{
		Kind:    transit.QueryErrorPredicate,
		Row:     0,
		Column:  0,
		Offset:  0,
		Message: "Wrong number of arguments to #eq? predicate. Expected 2, got 1.",
	})
	uqCheckQueryError(t, language, "((identifier) @id (#eq? @id @ok))", transit.QueryError{
		Kind:    transit.QueryErrorCapture,
		Row:     0,
		Column:  29,
		Offset:  29,
		Message: "\"ok\"",
	})
}

func TestQueryErrorsOnImpossiblePatterns(t *testing.T) {
	jsLang := uqLanguage(t, "javascript")
	rbLang := uqLanguage(t, "ruby")

	uqCheckQueryError(
		t,
		jsLang,
		"(binary_expression left: (expression (identifier)) left: (expression (identifier)))",
		transit.QueryError{
			Kind:   transit.QueryErrorStructure,
			Row:    0,
			Offset: 37,
			Column: 37,
			Message: uqLines(
				"(binary_expression left: (expression (identifier)) left: (expression (identifier)))",
				"                                     ^",
			),
		},
	)

	uqNewQuery(
		t,
		jsLang,
		"(function_declaration name: (identifier) (statement_block))",
	)
	uqCheckQueryError(t, jsLang, "(function_declaration name: (statement_block))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 22,
		Column: 22,
		Message: uqLines(
			"(function_declaration name: (statement_block))",
			"                      ^",
		),
	})

	uqNewQuery(t, rbLang, "(call receiver:(call))")
	uqCheckQueryError(t, rbLang, "(call receiver:(binary))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 6,
		Column: 6,
		Message: uqLines(
			"(call receiver:(binary))", //
			"      ^",
		),
	})

	uqNewQuery(
		t,
		jsLang,
		`[
                (function_expression (identifier))
                (function_declaration (identifier))
                (generator_function_declaration (identifier))
            ]`,
	)
	uqCheckQueryError(
		t,
		jsLang,
		`[
                    (function_expression (identifier))
                    (function_declaration (object))
                    (generator_function_declaration (identifier))
                ]`,
		transit.QueryError{
			Kind:   transit.QueryErrorStructure,
			Row:    2,
			Offset: 99,
			Column: 42,
			Message: uqLines(
				"                    (function_declaration (object))", //
				"                                          ^",
			),
		},
	)

	uqCheckQueryError(t, jsLang, "(identifier (identifier))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 12,
		Column: 12,
		Message: uqLines(
			"(identifier (identifier))", //
			"            ^",
		),
	})
	uqCheckQueryError(t, jsLang, "(true (true))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 6,
		Column: 6,
		Message: uqLines(
			"(true (true))", //
			"      ^",
		),
	})

	uqNewQuery(
		t,
		jsLang,
		`(if_statement
                condition: (parenthesized_expression (expression) @cond))`,
	)

	uqCheckQueryError(t, jsLang, "(if_statement condition: (expression))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 14,
		Column: 14,
		Message: uqLines(
			"(if_statement condition: (expression))", //
			"              ^",
		),
	})
	uqCheckQueryError(t, jsLang, "(identifier/identifier)", transit.QueryError{
		Row:    0,
		Offset: 0,
		Column: 0,
		Kind:   transit.QueryErrorStructure,
		Message: uqLines(
			"(identifier/identifier)", //
			"^",
		),
	})

	if jsLang.ABIVersion() >= 15 {
		uqCheckQueryError(t, jsLang, "(statement/identifier)", transit.QueryError{
			Row:    0,
			Offset: 0,
			Column: 0,
			Kind:   transit.QueryErrorStructure,
			Message: uqLines(
				"(statement/identifier)", //
				"^",
			),
		})
		uqCheckQueryError(t, jsLang, "(statement/pattern)", transit.QueryError{
			Row:    0,
			Offset: 0,
			Column: 0,
			Kind:   transit.QueryErrorStructure,
			Message: uqLines(
				"(statement/pattern)", //
				"^",
			),
		})
	}
}

func TestQueryVerifiesPossiblePatternsWithAliasedParentNodes(t *testing.T) {
	language := uqLanguage(t, "ruby")

	uqNewQuery(t, language, "(destructured_parameter (identifier))")

	uqCheckQueryError(t, language, "(destructured_parameter (string))", transit.QueryError{
		Kind:   transit.QueryErrorStructure,
		Row:    0,
		Offset: 24,
		Column: 24,
		Message: uqLines(
			"(destructured_parameter (string))", //
			"                        ^",
		),
	})
}

func TestQueryMatchesWithSimplePattern(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		"(function_declaration name: (identifier) @fn-name)",
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"function one() { two(); function three() {} }",
		[]uqMatch{
			{0, uqCaptures{{"fn-name", "one"}}},
			{0, uqCaptures{{"fn-name", "three"}}},
		},
	)
}

func TestQueryMatchesWithMultipleOnSameRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`(class_declaration
                name: (identifier) @the-class-name
                (class_body
                    (method_definition
                        name: (property_identifier) @the-method-name)))`,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            class Person {
                // the constructor
                constructor(name) { this.name = name; }

                // the getter
                getFullName() { return this.name; }
            }
            `,
		[]uqMatch{
			{
				0,
				uqCaptures{
					{"the-class-name", "Person"},
					{"the-method-name", "constructor"},
				},
			},
			{
				0,
				uqCaptures{
					{"the-class-name", "Person"},
					{"the-method-name", "getFullName"},
				},
			},
		},
	)
}

func TestQueryMatchesWithMultiplePatternsDifferentRoots(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
                (function_declaration name:(identifier) @fn-def)
                (call_expression function:(identifier) @fn-ref)
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            function f1() {
                f2(f3());
            }
            `,
		[]uqMatch{
			{0, uqCaptures{{"fn-def", "f1"}}},
			{1, uqCaptures{{"fn-ref", "f2"}}},
			{1, uqCaptures{{"fn-ref", "f3"}}},
		},
	)
}

func TestQueryMatchesWithMultiplePatternsSameRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
              (pair
                key: (property_identifier) @method-def
                value: (function_expression))

              (pair
                key: (property_identifier) @method-def
                value: (arrow_function))
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            a = {
                b: () => { return c; },
                d: function() { return d; }
            };
            `,
		[]uqMatch{
			{1, uqCaptures{{"method-def", "b"}}},
			{0, uqCaptures{{"method-def", "d"}}},
		},
	)
}

func TestQueryMatchesWithNestingAndNoFields(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
                (array
                    (array
                        (identifier) @x1
                        (identifier) @x2))
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            [[a]];
            [[c, d], [e, f, g, h]];
            [[h], [i]];
            `,
		[]uqMatch{
			{0, uqCaptures{{"x1", "c"}, {"x2", "d"}}},
			{0, uqCaptures{{"x1", "e"}, {"x2", "f"}}},
			{0, uqCaptures{{"x1", "e"}, {"x2", "g"}}},
			{0, uqCaptures{{"x1", "f"}, {"x2", "g"}}},
			{0, uqCaptures{{"x1", "e"}, {"x2", "h"}}},
			{0, uqCaptures{{"x1", "f"}, {"x2", "h"}}},
			{0, uqCaptures{{"x1", "g"}, {"x2", "h"}}},
		},
	)
}

func TestQueryMatchesWithManyResults(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "(array (identifier) @element)")

	want := make([]uqMatch, 50)
	for i := range want {
		want[i] = uqMatch{0, uqCaptures{{"element", "hello"}}}
	}
	uqAssertQueryMatches(
		t,
		language,
		query,
		strings.Repeat("[hello];\n", 50),
		want,
	)
}

func TestQueryMatchesWithManyOverlappingResults(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (call_expression
                function: (member_expression
                    property: (property_identifier) @method))
            (call_expression
                function: (identifier) @function)
            ((identifier) @constant
             (#match? @constant "[A-Z\\d_]+"))
            `,
	)

	count := 1024

	// Deeply nested chained function calls:
	// a
	//    .foo(bar(BAZ))
	//    .foo(bar(BAZ))
	//    .foo(bar(BAZ))
	//    ...
	source := "a" + strings.Repeat("\n  .foo(bar(BAZ))", count)

	cycle := []uqMatch{
		{0, uqCaptures{{"method", "foo"}}},
		{1, uqCaptures{{"function", "bar"}}},
		{2, uqCaptures{{"constant", "BAZ"}}},
	}
	want := make([]uqMatch, 0, 3*count)
	for range count {
		want = append(want, cycle...)
	}
	uqAssertQueryMatches(t, language, query, source, want)
}

func TestQueryMatchesCapturingErrorNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (ERROR (identifier) @the-error-identifier) @the-error
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"function a(b,, c, d :e:) {}",
		[]uqMatch{{0, uqCaptures{{"the-error", ":e:"}, {"the-error-identifier", "e"}}}},
	)
}

func TestQueryMatchesCapturingMissingNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (MISSING
              ; Comments should be valid
            ) @missing
            (MISSING
              ; Comments should be valid
              ";"
              ; Comments should be valid
              ) @missing-semicolon
            `,
	)

	// Missing anonymous nodes
	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            x = function(a) { b; } function(c) { d; }
            //                    ^ MISSING semicolon here
            `,
		[]uqMatch{
			{0, uqCaptures{{"missing", ""}}},
			{1, uqCaptures{{"missing-semicolon", ""}}},
		},
	)

	language = uqLanguage(t, "c")
	query = uqNewQuery(
		t,
		language,
		`(MISSING field_identifier) @missing-field-ident
            (MISSING identifier) @missing-ident
            (MISSING) @missing-anything`,
	)

	// Missing named nodes
	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            int main() {
              if (a.) {
              //    ^ MISSING field_identifier here
                b();
                c();

                if (*) d();
                //   ^ MISSING identifier here
              }
            }
            `,
		[]uqMatch{
			{0, uqCaptures{{"missing-field-ident", ""}}},
			{2, uqCaptures{{"missing-anything", ""}}},
			{1, uqCaptures{{"missing-ident", ""}}},
			{2, uqCaptures{{"missing-anything", ""}}},
		},
	)
}

func TestQueryMatchesWithExtraChildren(t *testing.T) {
	language := uqLanguage(t, "ruby")
	query := uqNewQuery(
		t,
		language,
		`
            (program(comment) @top_level_comment)
            (argument_list (heredoc_body) @heredoc_in_args)
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            # top-level
            puts(
                # not-top-level
                <<-IN_ARGS, bar.baz
                HELLO
                IN_ARGS
            )

            puts <<-NOT_IN_ARGS
            NO
            NOT_IN_ARGS
            `,
		[]uqMatch{
			{0, uqCaptures{{"top_level_comment", "# top-level"}}},
			{
				1,
				uqCaptures{{
					"heredoc_in_args",
					"\n                HELLO\n                IN_ARGS",
				}},
			},
		},
	)
}

func TestQueryMatchesWithNamedWildcard(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (return_statement (_) @the-return-value)
            (binary_expression operator: _ @the-operator)
            `,
	)

	source := "return a + b - c;"

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckMatches(t, uqCollectMatches(matches, query, source), []uqMatch{
		{0, uqCaptures{{"the-return-value", "a + b - c"}}},
		{1, uqCaptures{{"the-operator", "+"}}},
		{1, uqCaptures{{"the-operator", "-"}}},
	})
}

func TestQueryMatchesWithWildcardAtTheRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (_
                (comment) @doc
                .
                (function_declaration
                    name: (identifier) @name))
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"/* one */ var x; /* two */ function y() {} /* three */ class Z {}",
		[]uqMatch{{0, uqCaptures{{"doc", "/* two */"}, {"name", "y"}}}},
	)

	query = uqNewQuery(
		t,
		language,
		`
                (_ (string) @a)
                (_ (number) @b)
                (_ (true) @c)
                (_ (false) @d)
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"['hi', x(true), {y: false}]",
		[]uqMatch{
			{0, uqCaptures{{"a", "'hi'"}}},
			{2, uqCaptures{{"c", "true"}}},
			{3, uqCaptures{{"d", "false"}}},
		},
	)
}

func TestQueryMatchesWithWildcardWithinWildcard(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (_ (_) @child) @parent
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"/* a */ b; c;",
		[]uqMatch{
			{0, uqCaptures{{"parent", "/* a */ b; c;"}, {"child", "/* a */"}}},
			{0, uqCaptures{{"parent", "/* a */ b; c;"}, {"child", "b;"}}},
			{0, uqCaptures{{"parent", "b;"}, {"child", "b"}}},
			{0, uqCaptures{{"parent", "/* a */ b; c;"}, {"child", "c;"}}},
			{0, uqCaptures{{"parent", "c;"}, {"child", "c"}}},
		},
	)
}

// This file ports the tests of lines 1156 to 2288 of
// crates/cli/src/tests/query_test.rs of upstream.

func TestQueryMatchesWithImmediateSiblings(t *testing.T) {
	language := uqLanguage(t, "python")

	// The immediate child operator '.' can be used in three similar ways:
	// 1. Before the first child node in a pattern, it means that there cannot be any named
	//    siblings before that child node.
	// 2. After the last child node in a pattern, it means that there cannot be any named
	//    sibling after that child node.
	// 3. Between two child nodes in a pattern, it specifies that there cannot be any named
	//    siblings between those two child nodes.
	query := uqNewQuery(t, language, `
            (dotted_name
                (identifier) @parent
                .
                (identifier) @child)
            (dotted_name
                (identifier) @last-child
                .)
            (list
                .
                (_) @first-element)
            `)

	uqAssertQueryMatches(t, language, query,
		"import a.b.c.d; return [w, [1, y], z]",
		[]uqMatch{
			{0, uqCaptures{{"parent", "a"}, {"child", "b"}}},
			{0, uqCaptures{{"parent", "b"}, {"child", "c"}}},
			{0, uqCaptures{{"parent", "c"}, {"child", "d"}}},
			{1, uqCaptures{{"last-child", "d"}}},
			{2, uqCaptures{{"first-element", "w"}}},
			{2, uqCaptures{{"first-element", "1"}}},
		},
	)

	query = uqNewQuery(t, language, `
            (block . (_) @first-stmt)
            (block (_) @stmt)
            (block (_) @last-stmt .)
            `)

	uqAssertQueryMatches(t, language, query, `
            if a:
                b()
                c()
                if d(): e(); f()
                g()
            `,
		[]uqMatch{
			{0, uqCaptures{{"first-stmt", "b()"}}},
			{1, uqCaptures{{"stmt", "b()"}}},
			{1, uqCaptures{{"stmt", "c()"}}},
			{1, uqCaptures{{"stmt", "if d(): e(); f()"}}},
			{0, uqCaptures{{"first-stmt", "e()"}}},
			{1, uqCaptures{{"stmt", "e()"}}},
			{1, uqCaptures{{"stmt", "f()"}}},
			{2, uqCaptures{{"last-stmt", "f()"}}},
			{1, uqCaptures{{"stmt", "g()"}}},
			{2, uqCaptures{{"last-stmt", "g()"}}},
		},
	)
}

func TestQueryMatchesWithAnchorAfterZeroQuantifier(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language,
		"(program (comment)* @doc . (function_declaration name: (identifier) @name))",
	)

	// No comments and the function is not the first child. An anchor after a
	// zero-matched quantifier is vacuous, so the function still matches.
	uqAssertQueryMatches(t, language, query, `
class X {}
function foo() {}
`,
		[]uqMatch{{0, uqCaptures{{"name", "foo"}}}},
	)

	// With at least one comment the anchor applies, so the comments must
	// immediately precede the function.
	uqAssertQueryMatches(t, language, query, `
// c
function foo() {}
`,
		[]uqMatch{{0, uqCaptures{{"doc", "// c"}, {"name", "foo"}}}},
	)
}

func TestQueryMatchesWithAnchorAfterNestedZeroQuantifier(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (_
              (field_definition
                property: (_) @name
                value: (_)? @value
              ) @field
              .
              ";" @semicolon
            )
            `)

	uqAssertQueryMatches(t, language, query,
		"class Foo { bar; baz = 0; }",
		[]uqMatch{
			{
				0,
				uqCaptures{{"field", "bar"}, {"name", "bar"}, {"semicolon", ";"}},
			},
			{
				0,
				uqCaptures{
					{"field", "baz = 0"},
					{"name", "baz"},
					{"value", "0"},
					{"semicolon", ";"},
				},
			},
		},
	)
}

func TestQueryMatchesWithLastChildAnchorAfterOptional(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language,
		"(preproc_if (preproc_def)+ @def . (preproc_else)? @else .)",
	)

	// The optional `(preproc_else)?` is absent, so the trailing anchor's
	// last-child requirement transfers to the last `preproc_def`. A trailing
	// comment means the def is not the last child, so nothing matches.
	uqAssertQueryMatches(t, language, query, `
#if X
#define A
// c
#endif
`,
		nil,
	)

	// With the def as the last child, the (else-less) match is allowed.
	uqAssertQueryMatches(t, language, query, `
#if X
#define A
#endif
`,
		[]uqMatch{{0, uqCaptures{{"def", "#define A\n"}}}},
	)
}

func TestQueryMatchesWithAnchorsOnBothSidesOfZeroQuantifier(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language,
		"(program (lexical_declaration) @a . (comment)* . (function_declaration) @b)",
	)

	// Anchors on both sides of a zero-matched quantifier collapse into a single
	// adjacency constraint: with no comments, the declaration must be immediately
	// followed by the function.
	uqAssertQueryMatches(t, language, query, `
const a = 1;
const b = 2;
function foo() {}
`,
		[]uqMatch{{0, uqCaptures{{"a", "const b = 2;"}, {"b", "function foo() {}"}}}},
	)

	// With a comment present the quantifier is non-zero, so the anchors apply
	// normally: the comment must sit immediately between the declaration and the
	// function.
	uqAssertQueryMatches(t, language, query, `
const b = 2;
// c
function foo() {}
`,
		[]uqMatch{{0, uqCaptures{{"a", "const b = 2;"}, {"b", "function foo() {}"}}}},
	)
}

func TestQueryMatchesWithLeadingAnchorBeforeZeroQuantifier(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language,
		"(translation_unit . (comment)* (function_definition) @f)",
	)

	// The leading `.` anchors the comment run to the parent's first child. When the
	// run matches zero comments, that first-child requirement transfers to the
	// function, so it matches only when it is itself the first child.
	uqAssertQueryMatches(t, language, query, `
int main() {}
`,
		[]uqMatch{{0, uqCaptures{{"f", "int main() {}"}}}},
	)

	// The function is the second child, so with no leading comments it must not match.
	uqAssertQueryMatches(t, language, query, `
int a;
int main() {}
`,
		nil,
	)

	// With a leading comment the run starts at the first child and the function follows.
	uqAssertQueryMatches(t, language, query, `
// c
int main() {}
`,
		[]uqMatch{{0, uqCaptures{{"f", "int main() {}"}}}},
	)
}

func TestQueryMatchesWithLastNamedChild(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language, `(compound_statement
                (_)
                (_)
                (expression_statement
                    (identifier) @last_id) .)`)
	uqAssertQueryMatches(t, language, query, `
            void one() { a; b; c; }
            void two() { d; e; }
            void three() { f; g; h; i; }
            `,
		[]uqMatch{{0, uqCaptures{{"last_id", "c"}}}, {0, uqCaptures{{"last_id", "i"}}}},
	)
}

func TestQueryMatchesWithNegatedFields(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (import_specifier
                !alias
                name: (identifier) @import_name)

            (export_specifier
                !alias
                name: (identifier) @export_name)

            (export_statement
                !decorator
                !source
                (_) @exported)

            ; This negated field list is an extension of a previous
            ; negated field list. The order of the children and negated
            ; fields doesn't matter.
            (export_statement
                !decorator
                !source
                (_) @exported_expr
                !declaration)

            ; This negated field list is a prefix of a previous
            ; negated field list.
            (export_statement
                !decorator
                (_) @export_child .)
            `)
	uqAssertQueryMatches(t, language, query, `
            import {a as b, c} from 'p1';
            export {g, h as i} from 'p2';

            @foo
            export default 1;

            export var j = 1;

            export default k;
            `,
		[]uqMatch{
			{0, uqCaptures{{"import_name", "c"}}},
			{1, uqCaptures{{"export_name", "g"}}},
			{4, uqCaptures{{"export_child", "'p2'"}}},
			{2, uqCaptures{{"exported", "var j = 1;"}}},
			{4, uqCaptures{{"export_child", "var j = 1;"}}},
			{2, uqCaptures{{"exported", "k"}}},
			{3, uqCaptures{{"exported_expr", "k"}}},
			{4, uqCaptures{{"export_child", "k"}}},
		},
	)
}

func TestQueryMatchesWithFieldAtRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "name: (identifier) @name")
	uqAssertQueryMatches(t, language, query, `
            a();
            function b() {}
            class c extends d {}
            `,
		[]uqMatch{{0, uqCaptures{{"name", "b"}}}, {0, uqCaptures{{"name", "c"}}}},
	)
}

func TestQueryMatchesWithRepeatedLeafNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")

	query := uqNewQuery(t, language, `
            (
                (comment)+ @doc
                .
                (class_declaration
                    name: (identifier) @name)
            )

            (
                (comment)+ @doc
                .
                (function_declaration
                    name: (identifier) @name)
            )
            `)

	uqAssertQueryMatches(t, language, query, `
            // one
            // two
            a();

            // three
            {
                // four
                // five
                // six
                class B {}

                // seven
                c();

                // eight
                function d() {}
            }
            `,
		[]uqMatch{
			{
				0,
				uqCaptures{
					{"doc", "// four"},
					{"doc", "// five"},
					{"doc", "// six"},
					{"name", "B"},
				},
			},
			{1, uqCaptures{{"doc", "// eight"}, {"name", "d"}}},
		},
	)
}

func TestQueryMatchesOptionalCaptureBeforeUncapturedRequiredSibling(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(t, language, "(block (line_comment)? @doc (line_comment))")

	// The optional `(line_comment)? @doc` before an *uncaptured* required
	// `(line_comment)` yields two candidate completions at the block:
	//     - one where the optional captured `// a` (the required node is then `// b`),
	//     - one where the optional matched zero (the required node absorbs `// a`,
	//     leaving `@doc` unbound).
	// The zero-match completion's captures are a strict subset of the other's, so the
	// longest-match rule must drop it (there is exactly one match). This regressed when
	// the dedup pass gained an early-break. Without the accompanying capture-position
	// sort, the break skips the subset "loser" and it leaks as an extra empty match.
	uqAssertQueryMatches(t, language, query, `
            fn f() {
                // a
                // b
            }
            `,
		[]uqMatch{{0, uqCaptures{{"doc", "// a"}}}},
	)
}

func TestQueryMatchesWithOptionalNodesInsideOfRepetitions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `(array (","? (number) @num)+)`)

	uqAssertQueryMatches(t, language, query, `
            var a = [1, 2, 3, 4]
            `,
		[]uqMatch{{
			0,
			uqCaptures{{"num", "1"}, {"num", "2"}, {"num", "3"}, {"num", "4"}},
		}},
	)
}

func TestQueryMatchesWithTopLevelRepetitions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (comment)+ @doc
            `)

	uqAssertQueryMatches(t, language, query, `
            // a
            // b
            // c

            d()

            // e
            `,
		[]uqMatch{
			{0, uqCaptures{{"doc", "// a"}, {"doc", "// b"}, {"doc", "// c"}}},
			{0, uqCaptures{{"doc", "// e"}}},
		},
	)
}

func TestQueryMatchesWithNonTerminalRepetitionsWithinRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "(_ (expression_statement (identifier) @id)+)")

	uqAssertQueryMatches(t, language, query, `
            function f() {
                d;
                e;
                f;
                g;
            }
            a;
            b;
            c;
            `,
		[]uqMatch{
			{0, uqCaptures{{"id", "d"}, {"id", "e"}, {"id", "f"}, {"id", "g"}}},
			{0, uqCaptures{{"id", "a"}, {"id", "b"}, {"id", "c"}}},
		},
	)
}

func TestQueryMatchesWithNestedRepetitions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (variable_declaration
                (","? (variable_declarator name: (identifier) @x))+)+
            `)

	uqAssertQueryMatches(t, language, query, `
            var a = b, c, d
            var e, f

            // more
            var g
            `,
		[]uqMatch{
			{
				0,
				uqCaptures{{"x", "a"}, {"x", "c"}, {"x", "d"}, {"x", "e"}, {"x", "f"}},
			},
			{0, uqCaptures{{"x", "g"}}},
		},
	)
}

func TestQueryMatchesWithMultipleRepetitionPatternsThatIntersectOtherPattern(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// When this query sees a comment, it must keep track of several potential
	// matches: up to two for each pattern that begins with a comment.
	query := uqNewQuery(t, language, `
            (call_expression
                function: (member_expression
                    property: (property_identifier) @name)) @ref.method

            ((comment)* @doc (function_declaration))
            ((comment)* @doc (generator_function_declaration))
            ((comment)* @doc (class_declaration))
            ((comment)* @doc (lexical_declaration))
            ((comment)* @doc (variable_declaration))
            ((comment)* @doc (method_definition))

            (comment) @comment
            `)

	// Here, a series of comments occurs in the middle of a match of the first
	// pattern. To avoid exceeding the storage limits and discarding that outer
	// match, the comment-related matches need to be managed efficiently.
	source := "theObject\n" + strings.Repeat("  // the comment\n", 64) + "\n.theMethod()"

	want := slices.Repeat([]uqMatch{{7, uqCaptures{{"comment", "// the comment"}}}}, 64)
	want = append(want, uqMatch{
		0,
		uqCaptures{{"ref.method", source}, {"name", "theMethod"}},
	})
	uqAssertQueryMatches(t, language, query, source, want)
}

func TestQueryMatchesWithTrailingRepetitionsOfLastChild(t *testing.T) {
	language := uqLanguage(t, "javascript")

	query := uqNewQuery(t, language, `
            (unary_expression (primary_expression)+ @operand)
            `)

	uqAssertQueryMatches(t, language, query, `
            a = typeof (!b && ~c);
            `,
		[]uqMatch{
			{0, uqCaptures{{"operand", "b"}}},
			{0, uqCaptures{{"operand", "c"}}},
			{0, uqCaptures{{"operand", "(!b && ~c)"}}},
		},
	)
}

func TestQueryMatchesWithLeadingZeroOrMoreRepeatedLeafNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")

	query := uqNewQuery(t, language, `
            (
                (comment)* @doc
                .
                (function_declaration
                    name: (identifier) @name)
            )
            `)

	uqAssertQueryMatches(t, language, query, `
            function a() {
                // one
                var b;

                function c() {}

                // two
                // three
                var d;

                // four
                // five
                function e() {

                }
            }

            // six
            `,
		[]uqMatch{
			{0, uqCaptures{{"name", "a"}}},
			{0, uqCaptures{{"name", "c"}}},
			{
				0,
				uqCaptures{{"doc", "// four"}, {"doc", "// five"}, {"name", "e"}},
			},
		},
	)
}

func TestMatchesWithAnchorSiblingInsideParent(t *testing.T) {
	language := uqLanguage(t, "rust")

	query := uqNewQuery(t, language, `
            (source_file
                (line_comment)
                .
                (function_item
                    name: (identifier) @name)
            )`)

	uqAssertQueryMatches(t, language, query, `
            // A
            fn a() {}

            // B
            fn b() {}
            `,
		[]uqMatch{{0, uqCaptures{{"name", "a"}}}, {0, uqCaptures{{"name", "b"}}}},
	)
}

func TestMatchesWithAnchorSiblingWithQuantifierInsideParent(t *testing.T) {
	language := uqLanguage(t, "rust")

	query := uqNewQuery(t, language, `
            (source_file
                (line_comment)+
                .
                (function_item
                    name: (identifier) @name)
            )`)

	uqAssertQueryMatches(t, language, query, `
            // A
            fn a() {}

            // B
            fn b() {}
            `,
		[]uqMatch{{0, uqCaptures{{"name", "a"}}}, {0, uqCaptures{{"name", "b"}}}},
	)
}

func TestMatchesWithAnchorSiblingWithQuantifierCapturedInsideParent(t *testing.T) {
	language := uqLanguage(t, "rust")

	query := uqNewQuery(t, language, `
            (source_file
                (line_comment)+ @doc
                .
                (function_item
                    name: (identifier) @name)
            )`)

	uqAssertQueryMatches(t, language, query, `
            // A
            fn a() {}

            // B
            fn b() {}
            `,
		[]uqMatch{
			{0, uqCaptures{{"doc", "// A"}, {"name", "a"}}},
			{0, uqCaptures{{"doc", "// B"}, {"name", "b"}}},
		},
	)
}

func TestMatchesAnchoredQuantifiedSiblingInsideParent(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language,
		"(translation_unit (comment)* @comment . (declaration) @decl)",
	)
	uqAssertQueryMatches(t, language, query, `
void foo() {}

// this one has
// two comments
extern int baz;

// this one has a comment
extern int bar;
`,
		[]uqMatch{
			{
				0,
				uqCaptures{
					{"comment", "// this one has"},
					{"comment", "// two comments"},
					{"decl", "extern int baz;"},
				},
			},
			{
				0,
				uqCaptures{
					{"comment", "// this one has a comment"},
					{"decl", "extern int bar;"},
				},
			},
		},
	)
}

func TestQueryMatchesWithTrailingOptionalNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")

	query := uqNewQuery(t, language, `
            (class_declaration
                name: (identifier) @class
                (class_heritage
                  (identifier) @superclass)?)
            `)

	uqAssertQueryMatches(t, language, query,
		"class A {}",
		[]uqMatch{{0, uqCaptures{{"class", "A"}}}},
	)

	uqAssertQueryMatches(t, language, query, `
            class A {}
            class B extends C {}
            class D extends (E.F) {}
            `,
		[]uqMatch{
			{0, uqCaptures{{"class", "A"}}},
			{0, uqCaptures{{"class", "B"}, {"superclass", "C"}}},
			{0, uqCaptures{{"class", "D"}}},
		},
	)
}

func TestQueryMatchesWithNestedOptionalNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// A function call, optionally containing a function call, which optionally contains a
	// number
	query := uqNewQuery(t, language, `
            (call_expression
                function: (identifier) @outer-fn
                arguments: (arguments
                    (call_expression
                        function: (identifier) @inner-fn
                        arguments: (arguments
                            (number)? @num))?))
            `)

	uqAssertQueryMatches(t, language, query, `
            a(b, c(), d(null, 1, 2))
            e()
            f(g())
            `,
		[]uqMatch{
			{0, uqCaptures{{"outer-fn", "a"}, {"inner-fn", "c"}}},
			{0, uqCaptures{{"outer-fn", "c"}}},
			{0, uqCaptures{{"outer-fn", "a"}, {"inner-fn", "d"}, {"num", "1"}}},
			{0, uqCaptures{{"outer-fn", "a"}, {"inner-fn", "d"}, {"num", "2"}}},
			{0, uqCaptures{{"outer-fn", "d"}}},
			{0, uqCaptures{{"outer-fn", "e"}}},
			{0, uqCaptures{{"outer-fn", "f"}, {"inner-fn", "g"}}},
			{0, uqCaptures{{"outer-fn", "g"}}},
		},
	)
}

func TestQueryMatchesWithRepeatedInternalNodes(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (_
                (method_definition
                    (decorator (identifier) @deco)+
                    name: (property_identifier) @name))
            `)

	uqAssertQueryMatches(t, language, query, `
            class A {
                @c
                @d
                e() {}
            }
            `,
		[]uqMatch{{0, uqCaptures{{"deco", "c"}, {"deco", "d"}, {"name", "e"}}}},
	)
}

func TestQueryMatchesWithSimpleAlternatives(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (pair
                key: [(property_identifier) (string)] @key
                value: [(function_expression) @val1 (arrow_function) @val2])
            `)

	uqAssertQueryMatches(t, language, query, `
            a = {
                b: c,
                'd': e => f,
                g: {
                    h: function i() {},
                    'x': null,
                    j: _ => k
                },
                'l': function m() {},
            };
            `,
		[]uqMatch{
			{0, uqCaptures{{"key", "'d'"}, {"val2", "e => f"}}},
			{0, uqCaptures{{"key", "h"}, {"val1", "function i() {}"}}},
			{0, uqCaptures{{"key", "j"}, {"val2", "_ => k"}}},
			{0, uqCaptures{{"key", "'l'"}, {"val1", "function m() {}"}}},
		},
	)
}

func TestQueryMatchesWithAlternativesInRepetitions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (array
                [(identifier) (string)] @el
                .
                (
                    ","
                    .
                    [(identifier) (string)] @el
                )*)
            `)

	uqAssertQueryMatches(t, language, query, `
            a = [b, 'c', d, 1, e, 'f', 'g', h];
            `,
		[]uqMatch{
			{0, uqCaptures{{"el", "b"}, {"el", "'c'"}, {"el", "d"}}},
			{
				0,
				uqCaptures{{"el", "e"}, {"el", "'f'"}, {"el", "'g'"}, {"el", "h"}},
			},
		},
	)
}

func TestQueryMatchesWithAlternativesAtRoot(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            [
                "if"
                "else"
                "function"
                "throw"
                "return"
            ] @keyword
            `)

	uqAssertQueryMatches(t, language, query, `
            function a(b, c, d) {
                if (b) {
                    return c;
                } else {
                    throw d;
                }
            }
            `,
		[]uqMatch{
			{0, uqCaptures{{"keyword", "function"}}},
			{0, uqCaptures{{"keyword", "if"}}},
			{0, uqCaptures{{"keyword", "return"}}},
			{0, uqCaptures{{"keyword", "else"}}},
			{0, uqCaptures{{"keyword", "throw"}}},
		},
	)
}

func TestQueryMatchesWithAlternativesUnderFields(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (assignment_expression
                left: [
                    (identifier) @variable
                    (member_expression property: (property_identifier) @variable)
                ])
            `)

	uqAssertQueryMatches(t, language, query, `
            a = b;
            b = c.d;
            e.f = g;
            h.i = j.k;
            `,
		[]uqMatch{
			{0, uqCaptures{{"variable", "a"}}},
			{0, uqCaptures{{"variable", "b"}}},
			{0, uqCaptures{{"variable", "f"}}},
			{0, uqCaptures{{"variable", "i"}}},
		},
	)
}

// This file ports the tests of lines 2289 to 3419 of
// crates/cli/src/tests/query_test.rs of upstream.

func TestQueryMatchesInLanguageWithSimpleAliases(t *testing.T) {
	language := uqLanguage(t, "html")

	// HTML uses different tokens to track start tags names, end
	// tag names, script tag names, and style tag names. All of
	// these tokens are aliased to `tag_name`.
	query := uqNewQuery(t, language, "(tag_name) @tag")

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            <div>
                <script>hi</script>
                <style>hi</style>
            </div>
            `,
		[]uqMatch{
			{0, uqCaptures{{"tag", "div"}}},
			{0, uqCaptures{{"tag", "script"}}},
			{0, uqCaptures{{"tag", "script"}}},
			{0, uqCaptures{{"tag", "style"}}},
			{0, uqCaptures{{"tag", "style"}}},
			{0, uqCaptures{{"tag", "div"}}},
		},
	)
}

func TestQueryMatchesWithDifferentTokensWithTheSameStringValue(t *testing.T) {
	// In Rust, there are two '<' tokens: one for the binary operator,
	// and one with higher precedence for generics.
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
                "<" @less
                ">" @greater
                `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		"const A: B<C> = d < e || f > g;",
		[]uqMatch{
			{0, uqCaptures{{"less", "<"}}},
			{1, uqCaptures{{"greater", ">"}}},
			{0, uqCaptures{{"less", "<"}}},
			{1, uqCaptures{{"greater", ">"}}},
		},
	)
}

func TestQueryMatchesWithTooManyPermutationsToTrack(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (array (identifier) @pre (identifier) @post)
        `,
	)

	source := "[" + strings.Repeat("hello, ", 50) + "];"

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	cursor.SetMatchLimit(32)
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))

	// For this pathological query, some match permutations will be dropped.
	// Just check that a subset of the results are returned, and no crash or
	// leak occurs.
	got := uqCollectMatches(matches, query, source)
	if len(got) == 0 {
		t.Fatal("the query gave no match")
	}
	uqCheckMatches(t, got[:1], []uqMatch{{0, uqCaptures{{"pre", "hello"}, {"post", "hello"}}}})
	if !cursor.DidExceedMatchLimit() {
		t.Error("DidExceedMatchLimit() = false, want true")
	}
}

func TestQuerySiblingPatternsDontMatchChildrenOfAnError(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
            ("{" @open "}" @close)

            [
              (line_comment)
              (block_comment)
            ] @comment

            ("<" @first "<" @second)
            `,
	)

	// Most of the document will fail to parse, resulting in a
	// large number of tokens that are *direct* children of an
	// ERROR node.
	//
	// These children should still match, unless they are part
	// of a "non-rooted" pattern, in which there are multiple
	// top-level sibling nodes. Those patterns should not match
	// directly inside of an error node, because the contents of
	// an error node are not syntactically well-structured, so we
	// would get many spurious matches.
	source := `
            fn a() {}

            <<<<<<<<<< add pub b fn () {}
            // comment 1
            pub fn b() {
            /* comment 2 */
            ==========
            pub fn c() {
            // comment 3
            >>>>>>>>>> add pub c fn () {}
            }
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"open", "{"}, {"close", "}"}}},
			{1, uqCaptures{{"comment", "// comment 1"}}},
			{1, uqCaptures{{"comment", "/* comment 2 */"}}},
			{1, uqCaptures{{"comment", "// comment 3"}}},
		},
	)
}

func TestQueryMatchesWithAlternativesAndTooManyPermutationsToTrack(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (
                (comment) @doc
                ; not immediate
                (class_declaration) @class
            )

            (call_expression
                function: [
                    (identifier) @function
                    (member_expression property: (property_identifier) @method)
                ])
            `,
	)

	source := strings.Repeat("/* hi */ a.b(); ", 50)

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	cursor.SetMatchLimit(32)
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))

	want := make([]uqMatch, 50)
	for i := range want {
		want[i] = uqMatch{1, uqCaptures{{"method", "b"}}}
	}
	uqCheckMatches(t, uqCollectMatches(matches, query, source), want)
	if !cursor.DidExceedMatchLimit() {
		t.Error("DidExceedMatchLimit() = false, want true")
	}
}

func TestRepetitionsBeforeWithAlternatives(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
            (
                (line_comment)* @comment
                .
                [
                    (struct_item name: (_) @name)
                    (function_item name: (_) @name)
                    (enum_item name: (_) @name)
                    (impl_item type: (_) @name)
                ]
            )
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            // a
            // b
            fn c() {}

            // d
            // e
            impl F {}
            `,
		[]uqMatch{
			{
				0,
				uqCaptures{{"comment", "// a"}, {"comment", "// b"}, {"name", "c"}},
			},
			{
				0,
				uqCaptures{{"comment", "// d"}, {"comment", "// e"}, {"name", "F"}},
			},
		},
	)
}

func TestQueryMatchesWithAnonymousTokens(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            ";" @punctuation
            "&&" @operator
            "\"" @quote
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`foo(a && "b");`,
		[]uqMatch{
			{1, uqCaptures{{"operator", "&&"}}},
			{2, uqCaptures{{"quote", "\""}}},
			{2, uqCaptures{{"quote", "\""}}},
			{0, uqCaptures{{"punctuation", ";"}}},
		},
	)
}

func TestQueryMatchesWithSupertypes(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(
		t,
		language,
		`
            (argument_list (expression) @arg)

            (keyword_argument
                value: (expression) @kw_arg)

            (assignment
              left: (identifier) @var_def)

            (primary_expression/identifier) @var_ref
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
                a = b.c(
                    [d],
                    # a comment
                    e=f
                )
            `,
		[]uqMatch{
			{2, uqCaptures{{"var_def", "a"}}},
			{3, uqCaptures{{"var_ref", "b"}}},
			{0, uqCaptures{{"arg", "[d]"}}},
			{3, uqCaptures{{"var_ref", "d"}}},
			{1, uqCaptures{{"kw_arg", "f"}}},
			{3, uqCaptures{{"var_ref", "f"}}},
		},
	)
}

func TestQueryMatchesWithinByteRange(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "(identifier) @element")

	source := "[a, b, c, d, e, f, g]"

	tree := uqParse(t, language, source)

	cursor := transit.NewQueryCursor()

	cursor.SetByteRange(0, 8)
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "a"}}},
			{0, uqCaptures{{"element", "b"}}},
			{0, uqCaptures{{"element", "c"}}},
		},
	)

	cursor.SetByteRange(5, 15)
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "c"}}},
			{0, uqCaptures{{"element", "d"}}},
			{0, uqCaptures{{"element", "e"}}},
		},
	)

	// An end byte of zero indicates there is no end
	cursor.SetByteRange(12, 0)
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "e"}}},
			{0, uqCaptures{{"element", "f"}}},
			{0, uqCaptures{{"element", "g"}}},
		},
	)
}

func TestQueryMatchesWithinPointRange(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "(identifier) @element")

	source := uqUnindent(`
            [
              a, b,
              c, d,
              e, f,
              g, h,
              i, j,
              k, l,
            ]
        `)

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	cursor.SetPointRange(transit.Point{Row: 1, Column: 0}, transit.Point{Row: 2, Column: 3})
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "a"}}},
			{0, uqCaptures{{"element", "b"}}},
			{0, uqCaptures{{"element", "c"}}},
		},
	)

	cursor.SetPointRange(transit.Point{Row: 2, Column: 0}, transit.Point{Row: 3, Column: 3})
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "c"}}},
			{0, uqCaptures{{"element", "d"}}},
			{0, uqCaptures{{"element", "e"}}},
		},
	)

	// Zero end point is treated like no end point.
	cursor.SetPointRange(transit.Point{Row: 4, Column: 1}, transit.Point{Row: 0, Column: 0})
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"element", "g"}}},
			{0, uqCaptures{{"element", "h"}}},
			{0, uqCaptures{{"element", "i"}}},
			{0, uqCaptures{{"element", "j"}}},
			{0, uqCaptures{{"element", "k"}}},
			{0, uqCaptures{{"element", "l"}}},
		},
	)
}

func TestQueryCapturesWithinByteRange(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(
		t,
		language,
		`
            (call_expression
                function: (identifier) @function
                arguments: (argument_list (string_literal) @string.arg))

            (string_literal) @string
           `,
	)

	source := `DEFUN ("safe-length", Fsafe_length, Ssafe_length, 1, 1, 0)`

	tree := uqParse(t, language, source)

	cursor := transit.NewQueryCursor()
	cursor.SetByteRange(3, 27)
	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckCaptures(
		t,
		uqCollectCaptures(captures, query, source),
		uqCaptures{
			{"function", "DEFUN"},
			{"string.arg", "\"safe-length\""},
			{"string", "\"safe-length\""},
		},
	)
}

func TestQueryCursorNextCaptureWithByteRange(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(
		t,
		language,
		`(function_definition name: (identifier) @function)
             (attribute attribute: (identifier) @property)
             ((identifier) @variable)`,
	)

	source := "def func():\n  foo.bar.baz()\n"
	//            ^            ^    ^          ^
	// byte_pos   0           12    17        27
	// point_pos (0,0)      (1,0)  (1,5)    (1,15)

	tree := uqParse(t, language, source)

	cursor := transit.NewQueryCursor()
	cursor.SetByteRange(12, 17)
	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckCaptures(
		t,
		uqCollectCaptures(captures, query, source),
		uqCaptures{{"variable", "foo"}},
	)
}

func TestQueryCursorNextCaptureWithPointRange(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(
		t,
		language,
		`(function_definition name: (identifier) @function)
             (attribute attribute: (identifier) @property)
             ((identifier) @variable)`,
	)

	source := "def func():\n  foo.bar.baz()\n"
	//            ^            ^    ^          ^
	// byte_pos   0           12    17        27
	// point_pos (0,0)      (1,0)  (1,5)    (1,15)

	tree := uqParse(t, language, source)

	cursor := transit.NewQueryCursor()
	cursor.SetPointRange(transit.Point{Row: 1, Column: 0}, transit.Point{Row: 1, Column: 5})
	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckCaptures(
		t,
		uqCollectCaptures(captures, query, source),
		uqCaptures{{"variable", "foo"}},
	)
}

func TestQueryMatchesWithUnrootedPatternsIntersectingByteRange(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
            ("{" @left "}" @right)
            ("<" @left ">" @right)
            `,
	)

	source := "mod a { fn a<B: C, D: E>(f: B) { g(f) } }"

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	// within the type parameter list
	offset := uqcIndex(t, source, "D: E>")
	cursor.SetByteRange(offset, offset)
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{1, uqCaptures{{"left", "<"}, {"right", ">"}}},
			{0, uqCaptures{{"left", "{"}, {"right", "}"}}},
		},
	)

	// from within the type parameter list to within the function body
	startOffset := uqcIndex(t, source, "D: E>")
	endOffset := uqcIndex(t, source, "g(f)")
	cursor.SetByteRange(startOffset, endOffset)
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{1, uqCaptures{{"left", "<"}, {"right", ">"}}},
			{0, uqCaptures{{"left", "{"}, {"right", "}"}}},
			{0, uqCaptures{{"left", "{"}, {"right", "}"}}},
		},
	)
}

func TestQueryMatchesWithWildcardAtRootIntersectingByteRange(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(
		t,
		language,
		`
            [
                (_ body: (block))
                (_ consequence: (block))
            ] @indent
            `,
	)

	source := strings.TrimSpace(`
            class A:
                def b():
                    if c:
                        d
                    else:
                        e
        `)

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	// uqcFirstKinds returns the kind of the first capture of each match
	// that meets the offset.
	uqcFirstKinds := func(offset int) []string {
		var matches []string
		cursor.SetByteRange(offset, offset)
		for m := range cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)) {
			if len(m.Captures) > 0 {
				matches = append(matches, m.Captures[0].Node.Kind())
			}
		}
		return matches
	}

	// After the first line of the class definition
	offset := uqcIndex(t, source, "A:") + 2
	uqcCheckStrings(t, uqcFirstKinds(offset), []string{"class_definition"})

	// After the first line of the function definition
	offset = uqcIndex(t, source, "b():") + 4
	uqcCheckStrings(t, uqcFirstKinds(offset), []string{"class_definition", "function_definition"})

	// After the first line of the if statement
	offset = uqcIndex(t, source, "c:") + 2
	uqcCheckStrings(
		t,
		uqcFirstKinds(offset),
		[]string{"class_definition", "function_definition", "if_statement"},
	)
}

func TestQueryCapturesWithinByteRangeAssignedAfterIterating(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
            (function_item
                name: (identifier) @fn_name)

            (mod_item
                name: (identifier) @mod_name
                body: (declaration_list
                    "{" @lbrace
                    "}" @rbrace))

            ; functions that return Result<()>
            ((function_item
                return_type: (generic_type
                    type: (type_identifier) @result
                    type_arguments: (type_arguments
                        (unit_type)))
                body: _ @fallible_fn_body)
             (#eq? @result "Result"))
            `,
	)
	source := `
        mod m1 {
            mod m2 {
                fn f1() -> Option<()> { Some(()) }
            }
            fn f2() -> Result<()> { Ok(()) }
            fn f3() {}
        }
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	next, stop := iter.Pull2(cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)))
	defer stop()

	// Retrieve some captures
	var results uqCaptures
	for range 5 {
		mat, captureIndex, ok := next()
		if !ok {
			break
		}
		capture := mat.Captures[captureIndex]
		results = append(results, [2]string{
			query.CaptureNames()[capture.Index],
			source[capture.Node.StartByte():capture.Node.EndByte()],
		})
	}
	uqCheckCaptures(
		t,
		results,
		uqCaptures{
			{"mod_name", "m1"},
			{"lbrace", "{"},
			{"mod_name", "m2"},
			{"lbrace", "{"},
			{"fn_name", "f1"},
		},
	)

	// Advance to a range that only partially intersects some matches.
	// Captures from these matches are reported, but only those that
	// intersect the range.
	results = results[:0]
	cursor.SetByteRange(uqcIndex(t, source, "Ok"), len(source))
	for {
		mat, captureIndex, ok := next()
		if !ok {
			break
		}
		capture := mat.Captures[captureIndex]
		results = append(results, [2]string{
			query.CaptureNames()[capture.Index],
			source[capture.Node.StartByte():capture.Node.EndByte()],
		})
	}
	uqCheckCaptures(
		t,
		results,
		uqCaptures{
			{"fallible_fn_body", "{ Ok(()) }"},
			{"fn_name", "f3"},
			{"rbrace", "}"},
		},
	)
}

func TestQueryMatchesWithinRangeOfLongRepetition(t *testing.T) {
	language := uqLanguage(t, "rust")
	query := uqNewQuery(
		t,
		language,
		`
            (function_item name: (identifier) @fn-name)
            `,
	)

	source := uqUnindent(`
            fn zero() {}
            fn one() {}
            fn two() {}
            fn three() {}
            fn four() {}
            fn five() {}
            fn six() {}
            fn seven() {}
            fn eight() {}
            fn nine() {}
            fn ten() {}
            fn eleven() {}
            fn twelve() {}
        `)

	parser := uqParser(t, language)
	cursor := transit.NewQueryCursor()

	tree, err := parser.Parse(t.Context(), []byte(source), nil)
	if err != nil {
		t.Fatal(err)
	}

	cursor.SetPointRange(transit.Point{Row: 8, Column: 0}, transit.Point{Row: 20, Column: 0})
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{
			{0, uqCaptures{{"fn-name", "eight"}}},
			{0, uqCaptures{{"fn-name", "nine"}}},
			{0, uqCaptures{{"fn-name", "ten"}}},
			{0, uqCaptures{{"fn-name", "eleven"}}},
			{0, uqCaptures{{"fn-name", "twelve"}}},
		},
	)
}

func TestQueryMatchesContainedWithinRange(t *testing.T) {
	language := uqLanguage(t, "json")
	query := uqNewQuery(
		t,
		language,
		`
            ("[" @l_bracket "]" @r_bracket)
            ("{" @l_brace "}" @r_brace)
            `,
	)

	source := uqUnindent(`
            [
                {"key1": "value1"},
                {"key2": "value2"},
                {"key3": "value3"},
                {"key4": "value4"},
                {"key5": "value5"},
                {"key6": "value6"},
                {"key7": "value7"},
                {"key8": "value8"},
                {"key9": "value9"},
                {"key10": "value10"},
                {"key11": "value11"},
                {"key12": "value12"},
            ]
        `)

	tree := uqParse(t, language, source)

	expectedMatches := []uqMatch{
		{1, uqCaptures{{"l_brace", "{"}, {"r_brace", "}"}}},
		{1, uqCaptures{{"l_brace", "{"}, {"r_brace", "}"}}},
	}
	{
		cursor := transit.NewQueryCursor()
		cursor.SetContainingPointRange(transit.Point{Row: 5, Column: 0}, transit.Point{Row: 7, Column: 0})
		matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
		uqCheckMatches(t, uqCollectMatches(matches, query, source), expectedMatches)
	}
	{
		cursor := transit.NewQueryCursor()
		cursor.SetContainingByteRange(78, 120)
		matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
		uqCheckMatches(t, uqCollectMatches(matches, query, source), expectedMatches)
	}
}

func TestQueryMatchesDifferentQueriesSameCursor(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query1 := uqNewQuery(
		t,
		language,
		`
            (array (identifier) @id1)
        `,
	)
	query2 := uqNewQuery(
		t,
		language,
		`
            (array (identifier) @id1)
            (pair (identifier) @id2)
        `,
	)
	query3 := uqNewQuery(
		t,
		language,
		`
            (array (identifier) @id1)
            (pair (identifier) @id2)
            (parenthesized_expression (identifier) @id3)
        `,
	)

	source := "[a, {b: b}, (c)];"

	parser := uqParser(t, language)
	cursor := transit.NewQueryCursor()

	tree, err := parser.Parse(t.Context(), []byte(source), nil)
	if err != nil {
		t.Fatal(err)
	}

	matches := cursor.Matches(t.Context(), query1, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query1, source),
		[]uqMatch{{0, uqCaptures{{"id1", "a"}}}},
	)

	matches = cursor.Matches(t.Context(), query3, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query3, source),
		[]uqMatch{
			{0, uqCaptures{{"id1", "a"}}},
			{1, uqCaptures{{"id2", "b"}}},
			{2, uqCaptures{{"id3", "c"}}},
		},
	)

	matches = cursor.Matches(t.Context(), query2, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query2, source),
		[]uqMatch{{0, uqCaptures{{"id1", "a"}}}, {1, uqCaptures{{"id2", "b"}}}},
	)
}

func TestQueryMatchesWithMultipleCapturesOnANode(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`(function_declaration
                (identifier) @name1 @name2 @name3
                (statement_block) @body1 @body2)`,
	)

	source := "function foo() { return 1; }"
	parser := uqParser(t, language)
	cursor := transit.NewQueryCursor()

	tree, err := parser.Parse(t.Context(), []byte(source), nil)
	if err != nil {
		t.Fatal(err)
	}

	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{{
			0,
			uqCaptures{
				{"name1", "foo"},
				{"name2", "foo"},
				{"name3", "foo"},
				{"body1", "{ return 1; }"},
				{"body2", "{ return 1; }"},
			},
		}},
	)

	// disabling captures still works when there are multiple captures on a
	// single node.
	query.DisableCapture("name2")
	matches = cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(
		t,
		uqCollectMatches(matches, query, source),
		[]uqMatch{{
			0,
			uqCaptures{
				{"name1", "foo"},
				{"name3", "foo"},
				{"body1", "{ return 1; }"},
				{"body2", "{ return 1; }"},
			},
		}},
	)
}

func TestQueryMatchesWithCapturedWildcardAtRoot(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(
		t,
		language,
		`
            ; captured wildcard at the root
            (_ [
                (except_clause (block) @block)
                (finally_clause (block) @block)
            ]) @stmt

            [
                (while_statement (block) @block)
                (if_statement (block) @block)

                ; captured wildcard at the root within an alternation
                (_ [
                    (else_clause (block) @block)
                    (elif_clause (block) @block)
                ])

                (try_statement (block) @block)
                (for_statement (block) @block)
            ] @stmt
            `,
	)

	source := strings.TrimSpace(`
        for i in j:
            while True:
                if a:
                    print b
                elif c:
                    print d
                else:
                    try:
                        print f
                    except:
                        print g
                    finally:
                        print h
            else:
                print i
        `)

	parser := uqParser(t, language)
	cursor := transit.NewQueryCursor()
	tree, err := parser.Parse(t.Context(), []byte(source), nil)
	if err != nil {
		t.Fatal(err)
	}

	var matchCaptureNamesAndRows [][]uqcCaptureRow
	for m := range cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)) {
		captures := make([]uqcCaptureRow, 0, len(m.Captures))
		for _, c := range m.Captures {
			captures = append(captures, uqcCaptureRow{
				query.CaptureNames()[c.Index],
				c.Node.Kind(),
				c.Node.StartPoint().Row,
			})
		}
		matchCaptureNamesAndRows = append(matchCaptureNamesAndRows, captures)
	}

	want := [][]uqcCaptureRow{
		{{"stmt", "for_statement", 0}, {"block", "block", 1}},
		{{"stmt", "while_statement", 1}, {"block", "block", 2}},
		{{"stmt", "if_statement", 2}, {"block", "block", 3}},
		{{"stmt", "if_statement", 2}, {"block", "block", 5}},
		{{"stmt", "if_statement", 2}, {"block", "block", 7}},
		{{"stmt", "try_statement", 7}, {"block", "block", 8}},
		{{"stmt", "try_statement", 7}, {"block", "block", 10}},
		{{"stmt", "try_statement", 7}, {"block", "block", 12}},
		{{"stmt", "while_statement", 1}, {"block", "block", 14}},
	}
	if !slices.EqualFunc(matchCaptureNamesAndRows, want, slices.Equal) {
		t.Errorf("the captures are\n%v\nwant\n%v", matchCaptureNamesAndRows, want)
	}
}

func TestQueryMatchesWithNoCaptures(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(
		t,
		language,
		`
            (identifier)
            (string) @s
            `,
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            a = 'hi';
            b = 'bye';
            `,
		[]uqMatch{
			{0, nil},
			{1, uqCaptures{{"s", "'hi'"}}},
			{0, nil},
			{1, uqCaptures{{"s", "'bye'"}}},
		},
	)
}

func TestQueryMatchesWithRepeatedFields(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(
		t,
		language,
		"(field_declaration declarator: (field_identifier) @field)",
	)

	uqAssertQueryMatches(
		t,
		language,
		query,
		`
            struct S {
                int a, b, c;
            };
            `,
		[]uqMatch{
			{0, uqCaptures{{"field", "a"}}},
			{0, uqCaptures{{"field", "b"}}},
			{0, uqCaptures{{"field", "c"}}},
		},
	)
}

// uqcCaptureRow is the name of a capture, the kind of its node and the row
// where the node starts.
type uqcCaptureRow struct {
	name string
	kind string
	row  int
}

// uqcIndex returns the offset of the first s in source. It is
// source.find(s).unwrap(). It stops the test when source holds no s.
func uqcIndex(t *testing.T, source, s string) int {
	t.Helper()
	i := strings.Index(source, s)
	if i < 0 {
		t.Fatalf("the source holds no %q", s)
	}
	return i
}

// uqcCheckStrings compares a list of strings with want. An empty list and
// nil are the same.
func uqcCheckStrings(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// This file ports the tests of lines 3420 to 4779 of
// crates/cli/src/tests/query_test.rs of upstream.

func TestQueryMatchesWithDeeplyNestedPatternsWithFields(t *testing.T) {
	language := uqLanguage(t, "python")
	query := uqNewQuery(t, language, `
            (call
                function: (_) @func
                arguments: (_) @args)
            (call
                function: (attribute
                    object: (_) @receiver
                    attribute: (identifier) @method)
                arguments: (argument_list))

            ; These don't match anything, but they require additional
            ; states to keep track of their captures.
            (call
                function: (_) @fn
                arguments: (argument_list
                    (keyword_argument
                        name: (identifier) @name
                        value: (_) @val) @arg) @args) @call
            (call
                function: (identifier) @fn
                (#eq? @fn "super")) @super_call
            `)

	uqAssertQueryMatches(t, language, query, `
            a(1).b(2).c(3).d(4).e(5).f(6).g(7).h(8)
            `, []uqMatch{
		{0, uqCaptures{{"func", "a"}, {"args", "(1)"}}},
		{0, uqCaptures{{"func", "a(1).b"}, {"args", "(2)"}}},
		{1, uqCaptures{{"receiver", "a(1)"}, {"method", "b"}}},
		{0, uqCaptures{{"func", "a(1).b(2).c"}, {"args", "(3)"}}},
		{1, uqCaptures{{"receiver", "a(1).b(2)"}, {"method", "c"}}},
		{0, uqCaptures{{"func", "a(1).b(2).c(3).d"}, {"args", "(4)"}}},
		{1, uqCaptures{{"receiver", "a(1).b(2).c(3)"}, {"method", "d"}}},
		{0, uqCaptures{{"func", "a(1).b(2).c(3).d(4).e"}, {"args", "(5)"}}},
		{1, uqCaptures{{"receiver", "a(1).b(2).c(3).d(4)"}, {"method", "e"}}},
		{0, uqCaptures{{"func", "a(1).b(2).c(3).d(4).e(5).f"}, {"args", "(6)"}}},
		{1, uqCaptures{{"receiver", "a(1).b(2).c(3).d(4).e(5)"}, {"method", "f"}}},
		{0, uqCaptures{{"func", "a(1).b(2).c(3).d(4).e(5).f(6).g"}, {"args", "(7)"}}},
		{1, uqCaptures{
			{"receiver", "a(1).b(2).c(3).d(4).e(5).f(6)"},
			{"method", "g"},
		}},
		{0, uqCaptures{
			{"func", "a(1).b(2).c(3).d(4).e(5).f(6).g(7).h"},
			{"args", "(8)"},
		}},
		{1, uqCaptures{
			{"receiver", "a(1).b(2).c(3).d(4).e(5).f(6).g(7)"},
			{"method", "h"},
		}},
	})
}

func TestQueryAlternationWithInnerQuantifier(t *testing.T) {
	language := uqLanguage(t, "c")
	sourceCode := `#include <foo>
#include <bar>
#include <baz>

// comment`
	matches := []uqMatch{
		{0, uqCaptures{
			{"capture", "#include <foo>\n"},
			{"capture", "#include <bar>\n"},
			{"capture", "#include <baz>\n"},
		}},
		{0, uqCaptures{{"capture", "// comment"}}},
	}

	query := uqNewQuery(t, language, `[
       (preproc_include)+
       (comment)
    ] @capture`)
	uqAssertQueryMatches(t, language, query, sourceCode, matches)

	query = uqNewQuery(t, language, `[
       (comment)
       (preproc_include)+
    ] @capture`)
	uqAssertQueryMatches(t, language, query, sourceCode, matches)
}

func TestQueryAlternationWithOuterQuantifier(t *testing.T) {
	language := uqLanguage(t, "c")
	sourceCode := `#include <foo>
#include <bar>
#include <baz>

// comment`
	matches := []uqMatch{{
		0,
		uqCaptures{
			{"capture", "#include <foo>\n"},
			{"capture", "#include <bar>\n"},
			{"capture", "#include <baz>\n"},
			{"capture", "// comment"},
		},
	}}

	query := uqNewQuery(t, language, `[
        (preproc_include)
        (comment)
    ]+ @capture`)
	uqAssertQueryMatches(t, language, query, sourceCode, matches)

	query = uqNewQuery(t, language, `([
        (preproc_include)
        (comment)
    ] (_)?)+ @capture`)
	uqAssertQueryMatches(t, language, query, sourceCode, matches)
}

func TestQueryMatchesWithAlternationsAndPredicates(t *testing.T) {
	language := uqLanguage(t, "java")
	query := uqNewQuery(t, language, `
            (block
                [
                    (local_variable_declaration
                        (variable_declarator
                            (identifier) @def.a
                            (string_literal) @lit.a
                        )
                    )
                    (local_variable_declaration
                        (variable_declarator
                            (identifier) @def.b
                            (null_literal) @lit.b
                        )
                    )
                ]
                (expression_statement
                    (method_invocation [
                        (argument_list
                            (identifier) @ref.a
                            (string_literal)
                        )
                        (argument_list
                            (null_literal)
                            (identifier) @ref.b
                        )
                    ])
                )
                (#eq? @def.a @ref.a )
                (#eq? @def.b @ref.b )
            )
            `)

	uqAssertQueryMatches(t, language, query, `
            void test() {
                int a = "foo";
                f(null, b);
            }
            `, nil)
}

func TestQueryMatchesWithIndefiniteStepContainingNoCaptures(t *testing.T) {
	// This pattern depends on the field declarations within the
	// struct's body, but doesn't capture anything within the body.
	// It demonstrates that internally, state-splitting needs to occur
	// for each field declaration within the body, in order to avoid
	// prematurely failing if the first field does not match.
	//
	// https://github.com/tree-sitter/tree-sitter/issues/937
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language, `(struct_specifier
                name: (type_identifier) @name
                body: (field_declaration_list
                    (field_declaration
                        type: (union_specifier))))`)

	uqAssertQueryMatches(t, language, query, `
            struct LacksUnionField {
                int a;
                struct {
                    B c;
                } d;
                G *h;
            };

            struct HasUnionField {
                int a;
                struct {
                    B c;
                } d;
                union {
                    bool e;
                    float f;
                } g;
                G *h;
            };
            `, []uqMatch{{0, uqCaptures{{"name", "HasUnionField"}}}})
}

func TestQueryCapturesBasic(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (pair
              key: _ @method.def
              (function_expression
                name: (identifier) @method.alias))

            (variable_declarator
              name: _ @function.def
              value: (function_expression
                name: (identifier) @function.alias))

            ":" @delimiter
            "=" @operator
            `)

	source := `
          a({
            bc: function de() {
              const fg = function hi() {}
            },
            jk: function lm() {
              const no = function pq() {}
            },
          });
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckMatches(t, uqCollectMatches(matches, query, source), []uqMatch{
		{2, uqCaptures{{"delimiter", ":"}}},
		{0, uqCaptures{{"method.def", "bc"}, {"method.alias", "de"}}},
		{3, uqCaptures{{"operator", "="}}},
		{1, uqCaptures{{"function.def", "fg"}, {"function.alias", "hi"}}},
		{2, uqCaptures{{"delimiter", ":"}}},
		{0, uqCaptures{{"method.def", "jk"}, {"method.alias", "lm"}}},
		{3, uqCaptures{{"operator", "="}}},
		{1, uqCaptures{{"function.def", "no"}, {"function.alias", "pq"}}},
	})

	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), uqCaptures{
		{"method.def", "bc"},
		{"delimiter", ":"},
		{"method.alias", "de"},
		{"function.def", "fg"},
		{"operator", "="},
		{"function.alias", "hi"},
		{"method.def", "jk"},
		{"delimiter", ":"},
		{"method.alias", "lm"},
		{"function.def", "no"},
		{"operator", "="},
		{"function.alias", "pq"},
	})
}

func TestQueryCapturesWithTextConditions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            ((identifier) @constant
             (#match? @constant "^[A-Z]{2,}$"))

             ((identifier) @constructor
              (#match? @constructor "^[A-Z]"))

            ((identifier) @function.builtin
             (#eq? @function.builtin "require"))

            ((identifier) @variable.builtin
              (#any-of? @variable.builtin
                        "arguments"
                        "module"
                        "console"
                        "window"
                        "document"))

            ((identifier) @variable
             (#not-match? @variable "^(lambda|load)$"))
            `)

	source := `
          toad
          load
          panda
          lambda
          const ab = require('./ab');
          new Cd(EF);
          document;
          module;
          console;
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), uqCaptures{
		{"variable", "toad"},
		{"variable", "panda"},
		{"variable", "ab"},
		{"function.builtin", "require"},
		{"variable", "require"},
		{"constructor", "Cd"},
		{"variable", "Cd"},
		{"constant", "EF"},
		{"constructor", "EF"},
		{"variable", "EF"},
		{"variable.builtin", "document"},
		{"variable", "document"},
		{"variable.builtin", "module"},
		{"variable", "module"},
		{"variable.builtin", "console"},
		{"variable", "console"},
	})
}

func TestQueryCapturesWithPredicates(t *testing.T) {
	language := uqLanguage(t, "javascript")

	query := uqNewQuery(t, language, `
            ((call_expression (identifier) @foo)
             (#set! name something)
             (#set! cool)
             (#something! @foo omg))

            ((property_identifier) @bar
             (#is? cool)
             (#is-not? name something))`)

	if got, want := query.PropertySettings(0), []transit.QueryProperty{
		{Key: "name", Value: "something", HasValue: true, CaptureID: -1},
		{Key: "cool", CaptureID: -1},
	}; !slices.Equal(got, want) {
		t.Errorf("PropertySettings(0) = %+v, want %+v", got, want)
	}
	if got, want := query.GeneralPredicates(0), []transit.QueryPredicate{{
		Operator: "something!",
		Args: []transit.QueryPredicateArg{
			{IsCapture: true, Capture: 0},
			{Value: "omg"},
		},
	}}; !reflect.DeepEqual(got, want) {
		t.Errorf("GeneralPredicates(0) = %+v, want %+v", got, want)
	}
	if got := query.PropertySettings(1); len(got) != 0 {
		t.Errorf("PropertySettings(1) = %+v, want none", got)
	}
	if got := query.PropertyPredicates(0); len(got) != 0 {
		t.Errorf("PropertyPredicates(0) = %+v, want none", got)
	}
	if got, want := query.PropertyPredicates(1), []transit.QueryPropertyPredicate{
		{Property: transit.QueryProperty{Key: "cool", CaptureID: -1}, Positive: true},
		{Property: transit.QueryProperty{Key: "name", Value: "something", HasValue: true, CaptureID: -1}, Positive: false},
	}; !slices.Equal(got, want) {
		t.Errorf("PropertyPredicates(1) = %+v, want %+v", got, want)
	}

	source := "const a = window.b"
	tree := uqParse(t, language, source)

	query = uqNewQuery(t, language, `((identifier) @variable.builtin
                (#match? @variable.builtin "^(arguments|module|console|window|document)$")
                (#is-not? local))
            `)

	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	got := uqCollectMatches(matches, query, source)

	uqCheckMatches(t, got, []uqMatch{{0, uqCaptures{{"variable.builtin", "window"}}}})
}

func TestQueryCapturesWithQuotedPredicateArgs(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// Double-quoted strings can contain:
	// * special escape sequences like \n and \r
	// * escaped double quotes with \*
	// * literal backslashes with \\
	query := uqNewQuery(t, language, `
            ((call_expression (identifier) @foo)
             (#set! one "\"something\ngreat\""))

            ((identifier)
             (#set! two "\\s(\r?\n)*$"))

            ((function_declaration)
             (#set! three "\"something\ngreat\""))
            `)

	if got, want := query.PropertySettings(0), []transit.QueryProperty{
		{Key: "one", Value: "\"something\ngreat\"", HasValue: true, CaptureID: -1},
	}; !slices.Equal(got, want) {
		t.Errorf("PropertySettings(0) = %+v, want %+v", got, want)
	}
	if got, want := query.PropertySettings(1), []transit.QueryProperty{
		{Key: "two", Value: "\\s(\r?\n)*$", HasValue: true, CaptureID: -1},
	}; !slices.Equal(got, want) {
		t.Errorf("PropertySettings(1) = %+v, want %+v", got, want)
	}
	if got, want := query.PropertySettings(2), []transit.QueryProperty{
		{Key: "three", Value: "\"something\ngreat\"", HasValue: true, CaptureID: -1},
	}; !slices.Equal(got, want) {
		t.Errorf("PropertySettings(2) = %+v, want %+v", got, want)
	}
}

func TestQueryCapturesWithDuplicates(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (variable_declarator
                name: (identifier) @function
                value: (function_expression))

            (identifier) @variable
            `)

	source := `
          var x = function() {};
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), uqCaptures{{"function", "x"}, {"variable", "x"}})
}

func TestQueryCapturesWithManyNestedResultsWithoutFields(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// Search for key-value pairs whose values are anonymous functions.
	query := uqNewQuery(t, language, `
            (pair
              key: _ @method-def
              (arrow_function))

            ":" @colon
            "," @comma
            `)

	// The `pair` node for key `y` does not match any pattern, but inside of
	// its value, it contains many other `pair` nodes that do match the pattern.
	// The match for the *outer* pair should be terminated *before* descending into
	// the object value, so that we can avoid needing to buffer all of the inner
	// matches.
	methodCount := 50
	var b strings.Builder
	b.WriteString("x = { y: {\n")
	for i := range methodCount {
		fmt.Fprintf(&b, "    method%d: $ => null,\n", i)
	}
	b.WriteString("}};\n")
	source := b.String()

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := uqCollectCaptures(cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)), query, source)

	if len(captures) < 13 {
		t.Fatalf("len(captures) = %d, want at least 13", len(captures))
	}
	uqCheckCaptures(t, captures[0:13], uqCaptures{
		{"colon", ":"},
		{"method-def", "method0"},
		{"colon", ":"},
		{"comma", ","},
		{"method-def", "method1"},
		{"colon", ":"},
		{"comma", ","},
		{"method-def", "method2"},
		{"colon", ":"},
		{"comma", ","},
		{"method-def", "method3"},
		{"colon", ":"},
		{"comma", ","},
	})

	// Ensure that we don't drop matches because of needing to buffer too many.
	if got, want := len(captures), 1+3*methodCount; got != want {
		t.Errorf("len(captures) = %d, want %d", got, want)
	}
}

func TestQueryCapturesWithManyNestedResultsWithFields(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// Search expressions like `a ? a.b : null`
	query := uqNewQuery(t, language, `
            ((ternary_expression
                condition: (identifier) @left
                consequence: (member_expression
                    object: (identifier) @right)
                alternative: (null))
             (#eq? @left @right))
            `)

	// The outer expression does not match the pattern, but the consequence of the ternary
	// is an object that *does* contain many occurrences of the pattern.
	count := 50
	var b strings.Builder
	b.WriteString("a ? {")
	for i := range count {
		fmt.Fprintf(&b, "  x: y%d ? y%d.z : null,\n", i, i)
	}
	b.WriteString("} : null;\n")
	source := b.String()

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := uqCollectCaptures(cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)), query, source)

	if len(captures) < 20 {
		t.Fatalf("len(captures) = %d, want at least 20", len(captures))
	}
	uqCheckCaptures(t, captures[0:20], uqCaptures{
		{"left", "y0"},
		{"right", "y0"},
		{"left", "y1"},
		{"right", "y1"},
		{"left", "y2"},
		{"right", "y2"},
		{"left", "y3"},
		{"right", "y3"},
		{"left", "y4"},
		{"right", "y4"},
		{"left", "y5"},
		{"right", "y5"},
		{"left", "y6"},
		{"right", "y6"},
		{"left", "y7"},
		{"right", "y7"},
		{"left", "y8"},
		{"right", "y8"},
		{"left", "y9"},
		{"right", "y9"},
	})

	// Ensure that we don't drop matches because of needing to buffer too many.
	if got, want := len(captures), 2*count; got != want {
		t.Errorf("len(captures) = %d, want %d", got, want)
	}
}

func TestQueryCapturesWithTooManyNestedResults(t *testing.T) {
	language := uqLanguage(t, "javascript")

	// Search for method calls in general, and also method calls with a template string
	// in place of an argument list (aka "tagged template strings") in particular.
	//
	// This second pattern, which looks for the tagged template strings, is expensive to
	// use with the `captures()` method, because:
	// 1. When calling `captures`, all of the captures must be returned in order of their
	//    appearance.
	// 2. This pattern captures the root `call_expression`.
	// 3. This pattern's result also depends on the final child (the template string).
	// 4. In between the `call_expression` and the possible `template_string`, there can be an
	//    arbitrarily deep subtree.
	//
	// This means that, if any patterns match *after* the initial `call_expression` is
	// captured, but before the final `template_string` is found, those matches must
	// be buffered, in order to prevent captures from being returned out-of-order.
	query := uqNewQuery(t, language, `
            ;; easy 👇
            (call_expression
              function: (member_expression
                property: (property_identifier) @method-name))

            ;; hard 👇
            (call_expression
              function: (member_expression
                property: (property_identifier) @template-tag)
              arguments: (template_string)) @template-call
            `)

	// There are a *lot* of matches in between the beginning of the outer `call_expression`
	// (the call to `a(...).f`), which starts at the beginning of the file, and the final
	// template string, which occurs at the end of the file. The query algorithm imposes a
	// limit on the total number of matches which can be buffered at a time. But we don't
	// want to neglect the inner matches just because of the expensive outer match, so we
	// abandon the outer match (which would have captured `f` as a `template-tag`).
	source := strings.TrimSpace(`
        a(b => {
            b.c0().d0 ` + "`😄`" + `;
            b.c1().d1 ` + "`😄`" + `;
            b.c2().d2 ` + "`😄`" + `;
            b.c3().d3 ` + "`😄`" + `;
            b.c4().d4 ` + "`😄`" + `;
            b.c5().d5 ` + "`😄`" + `;
            b.c6().d6 ` + "`😄`" + `;
            b.c7().d7 ` + "`😄`" + `;
            b.c8().d8 ` + "`😄`" + `;
            b.c9().d9 ` + "`😄`" + `;
        }).e().f ` + "``" + `;
        `)

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	cursor.SetMatchLimit(32)
	captures := uqCollectCaptures(cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)), query, source)

	if len(captures) < 40 {
		t.Fatalf("len(captures) = %d, want at least 40\n%q", len(captures), captures)
	}
	uqCheckCaptures(t, captures[0:4], uqCaptures{
		{"template-call", "b.c0().d0 `😄`"},
		{"method-name", "c0"},
		{"method-name", "d0"},
		{"template-tag", "d0"},
	})
	uqCheckCaptures(t, captures[36:40], uqCaptures{
		{"template-call", "b.c9().d9 `😄`"},
		{"method-name", "c9"},
		{"method-name", "d9"},
		{"template-tag", "d9"},
	})
	uqCheckCaptures(t, captures[40:], uqCaptures{{"method-name", "e"}, {"method-name", "f"}})
}

func TestQueryCapturesWithDefinitePatternContainingManyNestedMatches(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (array
              "[" @l-bracket
              "]" @r-bracket)

            "." @dot
            `)

	// The '[' node must be returned before all of the '.' nodes,
	// even though its pattern does not finish until the ']' node
	// at the end of the document. But because the '[' is definite,
	// it can be returned before the pattern finishes matching.
	source := `
        [
            a.b.c.d.e.f.g.h.i,
            a.b.c.d.e.f.g.h.i,
            a.b.c.d.e.f.g.h.i,
            a.b.c.d.e.f.g.h.i,
            a.b.c.d.e.f.g.h.i,
        ]
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))
	want := uqCaptures{{"l-bracket", "["}}
	for range 40 {
		want = append(want, [2]string{"dot", "."})
	}
	want = append(want, [2]string{"r-bracket", "]"})
	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), want)
}

func TestQueryCapturesOrderedByBothStartAndEndPositions(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (call_expression) @call
            (member_expression) @member
            (identifier) @variable
            `)

	source := `
          a.b(c.d().e).f;
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), uqCaptures{
		{"member", "a.b(c.d().e).f"},
		{"call", "a.b(c.d().e)"},
		{"member", "a.b"},
		{"variable", "a"},
		{"member", "c.d().e"},
		{"call", "c.d()"},
		{"member", "c.d"},
		{"variable", "c"},
	})
}

func TestQueryCapturesWithMatchesRemoved(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (binary_expression
                left: (identifier) @left
                operator: _ @op
                right: (identifier) @right)
            `)

	source := `
          a === b && c > d && e < f;
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	var capturedStrings []string

	for m, i := range cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)) {
		capture := m.Captures[i]
		text := capture.Node.Text([]byte(source))
		if text == "a" {
			m.Remove()
			continue
		}
		capturedStrings = append(capturedStrings, text)
	}

	if want := []string{"c", ">", "d", "e", "<", "f"}; !slices.Equal(capturedStrings, want) {
		t.Errorf("the captured strings are %q, want %q", capturedStrings, want)
	}
}

func TestQueryCapturesWithMatchesRemovedBeforeTheyFinish(t *testing.T) {
	language := uqLanguage(t, "javascript")
	// When Tree-sitter detects that a pattern is guaranteed to match,
	// it will start to eagerly return the captures that it has found,
	// even though it hasn't matched the entire pattern yet. A
	// namespace_import node always has "*", "as" and then an identifier
	// for children, so captures will be emitted eagerly for this pattern.
	query := uqNewQuery(t, language, `
            (namespace_import
              "*" @star
              "as" @as
              (identifier) @identifier)
            `)

	source := `
          import * as name from 'module-name';
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	var capturedStrings []string
	for m, i := range cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)) {
		capture := m.Captures[i]
		text := capture.Node.Text([]byte(source))
		if text == "as" {
			m.Remove()
			continue
		}
		capturedStrings = append(capturedStrings, text)
	}

	// .remove() removes the match before it is finished. The identifier
	// "name" is part of this match, so we expect that removing the "as"
	// capture from the match should prevent "name" from matching:
	if want := []string{"*"}; !slices.Equal(capturedStrings, want) {
		t.Errorf("the captured strings are %q, want %q", capturedStrings, want)
	}
}

func TestQueryCapturesAndMatchesIteratorsAreFused(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (comment) @comment
            `)

	source := `
          // one
          // two
          // three
          /* unfinished
        `

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	nextCapture, stopCaptures := iter.Pull2(cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source)))
	for n := range 3 {
		m, _, ok := nextCapture()
		if !ok {
			stopCaptures()
			t.Fatalf("capture %d: the sequence ended", n)
		}
		if got := m.Captures[0].Index; got != 0 {
			t.Errorf("capture %d: Captures[0].Index = %d, want 0", n, got)
		}
	}
	for range 3 {
		if _, _, ok := nextCapture(); ok {
			t.Error("the sequence of captures gave a capture after its end")
		}
	}
	stopCaptures()

	nextMatch, stopMatches := iter.Pull(cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)))
	defer stopMatches()
	for n := range 3 {
		m, ok := nextMatch()
		if !ok {
			t.Fatalf("match %d: the sequence ended", n)
		}
		if got := m.Captures[0].Index; got != 0 {
			t.Errorf("match %d: Captures[0].Index = %d, want 0", n, got)
		}
	}
	for range 3 {
		if _, ok := nextMatch(); ok {
			t.Error("the sequence of matches gave a match after its end")
		}
	}
}

func TestQueryTextCallbackReturnsChunks(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            ((identifier) @leading_upper
             (#match? @leading_upper "^[A-Z][A-Z_]*[a-z]"))
            ((identifier) @all_upper
             (#match? @all_upper "^[A-Z][A-Z_]*$"))
            ((identifier) @all_lower
             (#match? @all_lower "^[a-z][a-z_]*$"))
            `)

	source := "SOMETHING[a] = transform(AnotherThing[b].property[c], PARAMETER);"

	// Store the source code in chunks of 3 bytes, and expose it via
	// an iterator API.
	sourceChunks := slices.Collect(slices.Chunk([]byte(source), 3))
	chunksInRange := func(start, end int) string {
		var b strings.Builder
		offset := 0
		for _, chunk := range sourceChunks {
			endOffset := offset + len(chunk)
			if offset < end && start < endOffset {
				endInChunk := min(end-offset, len(chunk))
				startInChunk := max(start, offset) - offset
				b.Write(chunk[startInChunk:endInChunk])
			}
			offset = endOffset
		}
		return b.String()
	}
	if got := chunksInRange(0, 9); got != "SOMETHING" {
		t.Errorf("chunksInRange(0, 9) = %q, want %q", got, "SOMETHING")
	}
	if got := chunksInRange(15, 24); got != "transform" {
		t.Errorf("chunksInRange(15, 24) = %q, want %q", got, "transform")
	}

	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	// The Rust test gives the cursor a closure that returns the chunks of
	// the text of a node. Captures of the Go API takes only the whole text,
	// so this test gives it the whole text.
	captures := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(source))

	uqCheckCaptures(t, uqCollectCaptures(captures, query, source), uqCaptures{
		{"all_upper", "SOMETHING"},
		{"all_lower", "a"},
		{"all_lower", "transform"},
		{"leading_upper", "AnotherThing"},
		{"all_lower", "b"},
		{"all_lower", "c"},
		{"all_upper", "PARAMETER"},
	})
}

func TestQueryStartEndByteForPattern(t *testing.T) {
	language := uqLanguage(t, "javascript")

	patterns1 := uqUnindent(`
        "+" @operator
        "-" @operator
        "*" @operator
        "=" @operator
        "=>" @operator
    `)

	patterns2 := uqUnindent(`
        (identifier) @a
        (string) @b
    `)

	patterns3 := uqUnindent(`
        ((identifier) @b (#match? @b i))
        (function_declaration name: (identifier) @c)
        (method_definition name: (property_identifier) @d)
    `)

	source := patterns1 + patterns2 + patterns3

	query := uqNewQuery(t, language, source)

	checks := []struct {
		name      string
		got, want int
	}{
		{"StartByteForPattern(0)", query.StartByteForPattern(0), 0},
		{"EndByteForPattern(0)", query.EndByteForPattern(0), len("\"+\" @operator\n")},
		{"StartByteForPattern(5)", query.StartByteForPattern(5), len(patterns1)},
		{"EndByteForPattern(5)", query.EndByteForPattern(5), len(patterns1) + len("(identifier) @a\n")},
		{"StartByteForPattern(7)", query.StartByteForPattern(7), len(patterns1) + len(patterns2)},
		{"EndByteForPattern(7)", query.EndByteForPattern(7), len(patterns1) + len(patterns2) + len("((identifier) @b (#match? @b i))\n")},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestQueryCaptureNames(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
            (if_statement
              condition: (parenthesized_expression (binary_expression
                left: _ @left-operand
                operator: "||"
                right: _ @right-operand))
              consequence: (statement_block) @body)

            (while_statement
              condition: _ @loop-condition)
            `)

	if got, want := query.CaptureNames(), []string{"left-operand", "right-operand", "body", "loop-condition"}; !slices.Equal(got, want) {
		t.Errorf("CaptureNames() = %q, want %q", got, want)
	}
}

func TestQueryLifetimeIsSeparateFromNodesLifetime(t *testing.T) {
	query := `(call_expression) @call`
	source := "a(1); b(2);"

	language := uqLanguage(t, "javascript")
	tree := uqParse(t, language, source)

	// The Rust test checks that the lifetime of a node does not depend on
	// the lifetime of the query. Go has no lifetimes, so this test only
	// takes a node out of a query that goes out of scope.
	takeFirstNodeFromCaptures := func(source, query string, node transit.Node) transit.Node {
		// Following 2 lines are redundant but needed to demonstrate
		// more understandable compiler error message
		language := uqLanguage(t, "javascript")
		query2 := uqNewQuery(t, language, query)
		cursor := transit.NewQueryCursor()

		for m := range cursor.Matches(t.Context(), query2, node, []byte(source)) {
			return m.Captures[0].Node
		}
		t.Fatal("the query gave no match")
		return transit.Node{}
	}

	node := takeFirstNodeFromCaptures(source, query, tree.RootNode())
	if got := node.Kind(); got != "call_expression" {
		t.Errorf("Kind() = %q, want %q", got, "call_expression")
	}

	takeFirstNodeFromMatches := func(source, query string, node transit.Node) transit.Node {
		language := uqLanguage(t, "javascript")
		query2 := uqNewQuery(t, language, query)
		cursor := transit.NewQueryCursor()

		for m := range cursor.Captures(t.Context(), query2, node, []byte(source)) {
			return m.Captures[0].Node
		}
		t.Fatal("the query gave no capture")
		return transit.Node{}
	}

	node = takeFirstNodeFromMatches(source, query, tree.RootNode())
	if got := node.Kind(); got != "call_expression" {
		t.Errorf("Kind() = %q, want %q", got, "call_expression")
	}
}

func TestQueryWithNoPatterns(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "")
	if got := query.CaptureNames(); len(got) != 0 {
		t.Errorf("CaptureNames() = %q, want none", got)
	}
	if got := query.PatternCount(); got != 0 {
		t.Errorf("PatternCount() = %d, want 0", got)
	}
}

func TestQueryComments(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
                ; this is my first comment
                ; i have two comments here
                (function_declaration
                    ; there is also a comment here
                    ; and here
                    name: (identifier) @fn-name)`)

	source := "function one() { }"
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(t, uqCollectMatches(matches, query, source), []uqMatch{{0, uqCaptures{{"fn-name", "one"}}}})
}

func TestQueryDisablePattern(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, `
                (function_declaration
                    name: (identifier) @name)
                (function_declaration
                    body: (statement_block) @body)
                (class_declaration
                    name: (identifier) @name)
                (class_declaration
                    body: (class_body) @body)
            `)

	// disable the patterns that match names
	query.DisablePattern(0)
	query.DisablePattern(2)

	source := "class A { constructor() {} } function b() { return 1; }"
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
	uqCheckMatches(t, uqCollectMatches(matches, query, source), []uqMatch{
		{3, uqCaptures{{"body", "{ constructor() {} }"}}},
		{1, uqCaptures{{"body", "{ return 1; }"}}},
	})
}

func TestQueryDeepClone(t *testing.T) {
	t.Skip("the Go API has no form of Query::deep_clone")

	language := uqLanguage(t, "javascript")
	querySource := `
                (function_declaration
                    name: (identifier) @name)
                (function_declaration
                    body: (statement_block) @body)
            `
	query := uqNewQuery(t, language, querySource)

	// The Rust test calls query.deep_clone() here. A second compile of the
	// same source stands in for the clone, so that the rest compiles.
	clone := uqNewQuery(t, language, querySource)
	clone.DisablePattern(1)

	source := "function foo() { return 1; }"
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	// The clone with pattern 1 disabled only produces the @name match.
	cloneMatches := uqCollectMatches(
		cursor.Matches(t.Context(), clone, tree.RootNode(), []byte(source)),
		clone,
		source,
	)
	uqCheckMatches(t, cloneMatches, []uqMatch{{0, uqCaptures{{"name", "foo"}}}})

	// The original is unaffected and still produces both @name and @body.
	originalMatches := uqCollectMatches(
		cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)),
		query,
		source,
	)
	uqCheckMatches(t, originalMatches, []uqMatch{
		{0, uqCaptures{{"name", "foo"}}},
		{1, uqCaptures{{"body", "{ return 1; }"}}},
	})
}

func TestQueryAlternativePredicatePrefix(t *testing.T) {
	language := uqLanguage(t, "c")
	query := uqNewQuery(t, language, `
            ((call_expression
              function: (identifier) @keyword
              arguments: (argument_list
                          (string_literal) @function))
             (.eq? @keyword "DEFUN"))
        `)
	source := `
            DEFUN ("identity", Fidentity, Sidentity, 1, 1, 0,
                   doc: /* Return the argument unchanged.  */
                   attributes: const)
              (Lisp_Object arg)
            {
              return arg;
            }
        `
	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"keyword", "DEFUN"}, {"function", "\"identity\""}}},
	})
}

// uqIterationCount is the number of iterations of a random test. It is
// ITERATION_COUNT of upstream, which the variable TREE_SITTER_ITERATIONS of
// the environment sets, and which is 10 by default.
func uqIterationCount() int {
	if n, err := strconv.Atoi(os.Getenv("TREE_SITTER_ITERATIONS")); err == nil {
		return n
	}
	return 10
}

// The random patterns differ from those of upstream, because upstream uses
// the generator StdRng of the Rust crate rand, and the port uses the
// generator PCG of math/rand/v2 with the same seeds. The test compares the
// matches of the query with the matches that a walk of the tree finds, so
// any pattern works.
func TestQueryRandom(t *testing.T) {
	root, _ := setup(t)
	language := uqLanguage(t, "rust")
	parser := uqParser(t, language)
	cursor := transit.NewQueryCursor()
	cursor.SetMatchLimit(64)

	helpers, err := os.ReadFile(filepath.Join(root, "tree-sitter", "crates", "cli", "src", "tests", "helpers", "query_helpers.rs"))
	if err != nil {
		t.Fatal(err)
	}
	parserTest, err := os.ReadFile(filepath.Join(root, "tree-sitter", "crates", "cli", "src", "tests", "parser_test.rs"))
	if err != nil {
		t.Fatal(err)
	}
	patternTree, err := parser.Parse(t.Context(), helpers, nil)
	if err != nil {
		t.Fatal(err)
	}
	testTree, err := parser.Parse(t.Context(), helpers, nil)
	if err != nil {
		t.Fatal(err)
	}

	startSeed := 0
	endSeed := startSeed + uqIterationCount()

	for seed := startSeed; seed < startSeed+endSeed; seed++ {
		rng := rand.New(rand.NewPCG(uint64(seed), 0))
		patternAST, _, _ := uqRandomPatternInTree(patternTree, rng)
		pattern := patternAST.String()
		expectedMatches := patternAST.matchesInTree(testTree)

		query, err := transit.NewQuery(language, pattern)
		if err != nil {
			t.Fatalf("failed to build query for pattern %s. seed: %d\n%v", pattern, seed, err)
		}
		var actualMatches []uqNodeMatch
		for mat := range cursor.Matches(t.Context(), query, testTree.RootNode(), parserTest) {
			var transformedMatch uqNodeMatch
			for _, c := range mat.Captures {
				transformedMatch.captures = append(transformedMatch.captures, uqNodeCapture{query.CaptureNames()[c.Index], c.Node})
			}
			actualMatches = append(actualMatches, transformedMatch)
		}

		// actual_matches.sort_unstable();
		actualMatches = slices.CompactFunc(actualMatches, uqEqualNodeMatches)

		if !cursor.DidExceedMatchLimit() && !slices.EqualFunc(actualMatches, expectedMatches, uqEqualNodeMatches) {
			t.Errorf("seed: %d, pattern:\n%s\nthe matches are\n%swant\n%s", seed, pattern, uqFormatNodeMatches(actualMatches), uqFormatNodeMatches(expectedMatches))
		}
	}
}

// uqeSkipExample reports whether the environment variable
// TREE_SITTER_TEST_EXAMPLE_FILTER is set and the description of an example
// does not hold its value. It is the EXAMPLE_FILTER of query_test.rs.
func uqeSkipExample(description string) bool {
	f := os.Getenv("TREE_SITTER_TEST_EXAMPLE_FILTER")
	return f != "" && !strings.Contains(description, f)
}

// uqeCollapse joins the words of s with one space. It is
// split_ascii_whitespace().collect::<Vec<_>>().join(" ") of Rust.
func uqeCollapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestQueryIsPatternGuaranteedAtStep(t *testing.T) {
	type result struct {
		substring  string
		isDefinite bool
	}
	type row struct {
		language           string
		description        string
		pattern            string
		resultsBySubstring []result
	}

	rows := []row{
		{
			description:        "no guaranteed steps",
			language:           "python",
			pattern:            `(expression_statement (string))`,
			resultsBySubstring: []result{{"expression_statement", false}, {"string", false}},
		},
		{
			description:        "all guaranteed steps",
			language:           "javascript",
			pattern:            `(object "{" "}")`,
			resultsBySubstring: []result{{"object", false}, {"{", true}, {"}", true}},
		},
		{
			description: "a fallible step that is optional",
			language:    "javascript",
			pattern:     `(object "{" (identifier)? @foo "}")`,
			resultsBySubstring: []result{
				{"object", false},
				{"{", true},
				{"(identifier)?", false},
				{"}", true},
			},
		},
		{
			description: "multiple fallible steps that are optional",
			language:    "javascript",
			pattern:     `(object "{" (identifier)? @id1 ("," (identifier) @id2)? "}")`,
			resultsBySubstring: []result{
				{"object", false},
				{"{", true},
				{"(identifier)? @id1", false},
				{"\",\"", false},
				{"}", true},
			},
		},
		{
			description:        "guaranteed step after fallibe step",
			language:           "javascript",
			pattern:            `(pair (property_identifier) ":")`,
			resultsBySubstring: []result{{"pair", false}, {"property_identifier", false}, {":", true}},
		},
		{
			description: "fallible step in between two guaranteed steps",
			language:    "javascript",
			pattern: `(ternary_expression
                condition: (_)
                "?"
                consequence: (call_expression)
                ":"
                alternative: (_))`,
			resultsBySubstring: []result{
				{"condition:", false},
				{"\"?\"", false},
				{"consequence:", false},
				{"\":\"", true},
				{"alternative:", true},
			},
		},
		{
			description:        "one guaranteed step after a repetition",
			language:           "javascript",
			pattern:            `(object "{" (_) "}")`,
			resultsBySubstring: []result{{"object", false}, {"{", false}, {"(_)", false}, {"}", true}},
		},
		{
			description: "guaranteed steps after multiple repetitions",
			language:    "json",
			pattern:     `(object "{" (pair) "," (pair) "," (_) "}")`,
			resultsBySubstring: []result{
				{"object", false},
				{"{", false},
				{"(pair) \",\" (pair)", false},
				{"(pair) \",\" (_)", false},
				{"\",\" (_)", false},
				{"(_)", true},
				{"}", true},
			},
		},
		{
			description: "a guaranteed step with a field",
			language:    "javascript",
			pattern:     `(binary_expression left: (expression) right: (_))`,
			resultsBySubstring: []result{
				{"binary_expression", false},
				{"(expression)", false},
				{"(_)", true},
			},
		},
		{
			description: "multiple guaranteed steps with fields",
			language:    "javascript",
			pattern:     `(function_declaration name: (identifier) body: (statement_block))`,
			resultsBySubstring: []result{
				{"function_declaration", false},
				{"identifier", true},
				{"statement_block", true},
			},
		},
		{
			description: "nesting, one guaranteed step",
			language:    "javascript",
			pattern: `
                (function_declaration
                    name: (identifier)
                    body: (statement_block "{" (expression_statement) "}"))`,
			resultsBySubstring: []result{
				{"function_declaration", false},
				{"identifier", false},
				{"statement_block", false},
				{"{", false},
				{"expression_statement", false},
				{"}", true},
			},
		},
		{
			description: "a guaranteed step after some deeply nested hidden nodes",
			language:    "ruby",
			pattern: `
            (singleton_class
                value: (constant)
                "end")
            `,
			resultsBySubstring: []result{
				{"singleton_class", false},
				{"constant", false},
				{"end", true},
			},
		},
		{
			description: "nesting, no guaranteed steps",
			language:    "javascript",
			pattern: `
            (call_expression
                function: (member_expression
                  property: (property_identifier) @template-tag)
                arguments: (template_string)) @template-call
            `,
			resultsBySubstring: []result{{"property_identifier", false}, {"template_string", false}},
		},
		{
			description: "a guaranteed step after a nested node",
			language:    "javascript",
			pattern: `
            (subscript_expression
                object: (member_expression
                    object: (identifier) @obj
                    property: (property_identifier) @prop)
                "[")
            `,
			resultsBySubstring: []result{
				{"identifier", false},
				{"property_identifier", false},
				{"[", true},
			},
		},
		{
			description: "a step that is fallible due to a predicate",
			language:    "javascript",
			pattern: `
            (subscript_expression
                object: (member_expression
                    object: (identifier) @obj
                    property: (property_identifier) @prop)
                "["
                (#match? @prop "foo"))
            `,
			resultsBySubstring: []result{
				{"identifier", false},
				{"property_identifier", false},
				{"[", true},
			},
		},
		{
			description: "alternation where one branch has guaranteed steps",
			language:    "javascript",
			pattern: `
            [
                (unary_expression (identifier))
                (call_expression
                  function: (_)
                  arguments: (_))
                (binary_expression right: (call_expression))
            ]
            `,
			resultsBySubstring: []result{
				{"identifier", false},
				{"right:", false},
				{"function:", true},
				{"arguments:", true},
			},
		},
		{
			description: "guaranteed step at the end of an aliased parent node",
			language:    "ruby",
			pattern: `
            (method_parameters "(" (identifier) @id")")
            `,
			resultsBySubstring: []result{{"\"(\"", false}, {"(identifier)", false}, {"\")\"", true}},
		},
		{
			description: "long, but not too long to analyze",
			language:    "javascript",
			pattern: `
            (object "{" (pair) (pair) (pair) (pair) "}")
            `,
			resultsBySubstring: []result{
				{"\"{\"", false},
				{"(pair)", false},
				{"(pair) \"}\"", false},
				{"\"}\"", true},
			},
		},
		{
			description: "too long to analyze",
			language:    "javascript",
			pattern: `
            (object "{" (pair) (pair) (pair) (pair) (pair) (pair) (pair) (pair) (pair) (pair) (pair) (pair) "}")
            `,
			resultsBySubstring: []result{
				{"\"{\"", false},
				{"(pair)", false},
				{"(pair) \"}\"", false},
				{"\"}\"", false},
			},
		},
		{
			description: "hidden nodes that have several fields",
			language:    "java",
			pattern: `
            (method_declaration name: (identifier))
            `,
			resultsBySubstring: []result{{"name:", true}},
		},
		{
			description: "top-level non-terminal extra nodes",
			language:    "ruby",
			pattern: `
            (heredoc_body
                (interpolation)
                (heredoc_end) @end)
            `,
			resultsBySubstring: []result{
				{"(heredoc_body", false},
				{"(interpolation)", false},
				{"(heredoc_end)", true},
			},
		},
		// TODO: figure out why line comments, an extra, are no longer allowed *anywhere*
		// likely culprits are the fact that it's no longer a token itself or that it uses an
		// external token
		// Row {
		//     description: "multiple extra nodes",
		//     language: get_language("rust"),
		//     pattern: r"
		//     (call_expression
		//         (line_comment) @a
		//         (line_comment) @b
		//         (arguments))
		//     ",
		//     results_by_substring: &[
		//         ("(line_comment) @a", false),
		//         ("(line_comment) @b", false),
		//         ("(arguments)", true),
		//     ],
		// },
	}

	for _, row := range rows {
		if uqeSkipExample(row.description) {
			continue
		}
		t.Logf("  query example: %q", row.description)
		query := uqNewQuery(t, uqLanguage(t, row.language), row.pattern)
		for _, r := range row.resultsBySubstring {
			offset := strings.Index(row.pattern, r.substring)
			if offset < 0 {
				t.Fatalf("the pattern %q does not hold %q", row.pattern, r.substring)
			}
			if got := query.IsPatternGuaranteedAtStep(offset); got != r.isDefinite {
				t.Errorf("Description: %s, Pattern: %q, substring: %q, expected is_definite to be %t",
					row.description, uqeCollapse(row.pattern), r.substring, r.isDefinite)
			}
		}
	}
}

func TestQueryIsPatternRooted(t *testing.T) {
	type row struct {
		description string
		pattern     string
		isRooted    bool
	}

	rows := []row{
		{
			description: "simple token",
			pattern:     `(identifier)`,
			isRooted:    true,
		},
		{
			description: "simple non-terminal",
			pattern:     `(function_definition name: (identifier))`,
			isRooted:    true,
		},
		{
			description: "alternative of many tokens",
			pattern:     `["if" "def" (identifier) (comment)]`,
			isRooted:    true,
		},
		{
			description: "alternative of many non-terminals",
			pattern: `[
                (function_definition name: (identifier))
                (class_definition name: (identifier))
                (block)
            ]`,
			isRooted: true,
		},
		{
			description: "two siblings",
			pattern:     `("{" "}")`,
			isRooted:    false,
		},
		{
			description: "top-level repetition",
			pattern:     `(comment)*`,
			isRooted:    false,
		},
		{
			description: "alternative where one option has two siblings",
			pattern: `[
                (block)
                (class_definition)
                ("(" ")")
                (function_definition)
            ]`,
			isRooted: false,
		},
		{
			description: "alternative where one option has a top-level repetition",
			pattern: `[
                (block)
                (class_definition)
                (comment)*
                (function_definition)
            ]`,
			isRooted: false,
		},
	}

	language := uqLanguage(t, "python")
	for _, row := range rows {
		if uqeSkipExample(row.description) {
			continue
		}
		t.Logf("  query example: %q", row.description)
		query := uqNewQuery(t, language, row.pattern)
		if got := query.IsPatternRooted(0); got != row.isRooted {
			t.Errorf("Description: %s, Pattern: %q: IsPatternRooted(0) = %t, want %t",
				row.description, uqeCollapse(row.pattern), got, row.isRooted)
		}
	}
}

func TestQueryIsPatternNonLocal(t *testing.T) {
	type row struct {
		description string
		pattern     string
		language    string
		isNonLocal  bool
	}

	rows := []row{
		{
			description: "simple token",
			pattern:     `(identifier)`,
			language:    "python",
			isNonLocal:  false,
		},
		{
			description: "siblings that can occur in an argument list",
			pattern:     `((identifier) (identifier))`,
			language:    "python",
			isNonLocal:  true,
		},
		{
			description: "siblings that can occur in a statement block",
			pattern:     `((return_statement) (return_statement))`,
			language:    "python",
			isNonLocal:  true,
		},
		{
			description: "siblings that can occur in a source file",
			pattern:     `((function_definition) (class_definition))`,
			language:    "python",
			isNonLocal:  true,
		},
		{
			description: "siblings that can't occur in any repetition",
			pattern:     `("{" "}")`,
			language:    "python",
			isNonLocal:  false,
		},
		{
			description: "siblings that can't occur in any repetition, wildcard root",
			pattern:     `(_ "{" "}") @foo`,
			language:    "javascript",
			isNonLocal:  false,
		},
		{
			description: "siblings that can occur in a class body, wildcard root",
			pattern:     `(_ (method_definition) (method_definition)) @foo`,
			language:    "javascript",
			isNonLocal:  true,
		},
		{
			description: "top-level repetitions that can occur in a class body",
			pattern:     `(method_definition)+ @foo`,
			language:    "javascript",
			isNonLocal:  true,
		},
		{
			description: "top-level repetitions that can occur in a statement block",
			pattern:     `(return_statement)+ @foo`,
			language:    "javascript",
			isNonLocal:  true,
		},
		{
			description: "rooted pattern that can occur in a statement block",
			pattern:     `(return_statement) @foo`,
			language:    "javascript",
			isNonLocal:  false,
		},
	}

	for _, row := range rows {
		if uqeSkipExample(row.description) {
			continue
		}
		t.Logf("  query example: %q", row.description)
		query := uqNewQuery(t, uqLanguage(t, row.language), row.pattern)
		if got := query.IsPatternNonLocal(0); got != row.isNonLocal {
			t.Errorf("Description: %s, Pattern: %q: IsPatternNonLocal(0) = %t, want %t",
				row.description, uqeCollapse(row.pattern), got, row.isNonLocal)
		}
	}
}

func TestCaptureQuantifiers(t *testing.T) {
	type quantifier struct {
		pattern    int
		capture    string
		quantifier transit.Quantifier
	}
	type row struct {
		description        string
		language           string
		pattern            string
		captureQuantifiers []quantifier
	}

	rows := []row{
		// Simple quantifiers
		{
			description: "Top level capture",
			language:    "python",
			pattern: `
                (module) @mod
            `,
			captureQuantifiers: []quantifier{{0, "mod", transit.QuantifierOne}},
		},
		{
			description: "Nested list capture capture",
			language:    "javascript",
			pattern: `
                (array (_)* @elems) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "elems", transit.QuantifierZeroOrMore},
			},
		},
		{
			description: "Nested non-empty list capture capture",
			language:    "javascript",
			pattern: `
                (array (_)+ @elems) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "elems", transit.QuantifierOneOrMore},
			},
		},
		// Nested quantifiers
		{
			description: "capture nested in optional pattern",
			language:    "javascript",
			pattern: `
                (array (call_expression (arguments (_) @arg))? @call) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "call", transit.QuantifierZeroOrOne},
				{0, "arg", transit.QuantifierZeroOrOne},
			},
		},
		{
			description: "optional capture nested in non-empty list pattern",
			language:    "javascript",
			pattern: `
                (array (call_expression (arguments (_)? @arg))+ @call) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "call", transit.QuantifierOneOrMore},
				{0, "arg", transit.QuantifierZeroOrMore},
			},
		},
		{
			description: "non-empty list capture nested in optional pattern",
			language:    "javascript",
			pattern: `
                (array (call_expression (arguments (_)+ @args))? @call) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "call", transit.QuantifierZeroOrOne},
				{0, "args", transit.QuantifierZeroOrMore},
			},
		},
		// Quantifiers in alternations
		{
			description: "capture is the same in all alternatives",
			language:    "javascript",
			pattern: `[
                (function_declaration name:(identifier) @name)
                (call_expression function:(identifier) @name)
            ]`,
			captureQuantifiers: []quantifier{{0, "name", transit.QuantifierOne}},
		},
		{
			description: "capture appears in some alternatives",
			language:    "javascript",
			pattern: `[
                (function_declaration name:(identifier) @name)
                (function_expression)
            ] @fun`,
			captureQuantifiers: []quantifier{
				{0, "fun", transit.QuantifierOne},
				{0, "name", transit.QuantifierZeroOrOne},
			},
		},
		{
			description: "capture has different quantifiers in alternatives",
			language:    "javascript",
			pattern: `[
                (call_expression arguments: (arguments (_)+ @args))
                (new_expression  arguments: (arguments (_)? @args))
            ] @call`,
			captureQuantifiers: []quantifier{
				{0, "call", transit.QuantifierOne},
				{0, "args", transit.QuantifierZeroOrMore},
			},
		},
		// Quantifiers in siblings
		{
			description: "siblings have different captures with different quantifiers",
			language:    "javascript",
			pattern: `
                (call_expression (arguments (identifier)? @self (_)* @args)) @call
            `,
			captureQuantifiers: []quantifier{
				{0, "call", transit.QuantifierOne},
				{0, "self", transit.QuantifierZeroOrOne},
				{0, "args", transit.QuantifierZeroOrMore},
			},
		},
		{
			description: "siblings have same capture with different quantifiers",
			language:    "javascript",
			pattern: `
                (call_expression (arguments (identifier) @args (_)* @args)) @call
            `,
			captureQuantifiers: []quantifier{
				{0, "call", transit.QuantifierOne},
				{0, "args", transit.QuantifierOneOrMore},
			},
		},
		// Combined scenarios
		{
			description: "combined nesting, alternatives, and siblings",
			language:    "javascript",
			pattern: `
                (array
                    (call_expression
                        (arguments [
                            (identifier) @self
                            (_)+ @args
                        ])
                    )+ @call
                ) @array
            `,
			captureQuantifiers: []quantifier{
				{0, "array", transit.QuantifierOne},
				{0, "call", transit.QuantifierOneOrMore},
				{0, "self", transit.QuantifierZeroOrMore},
				{0, "args", transit.QuantifierZeroOrMore},
			},
		},
		// Multiple patterns
		{
			description: "multiple patterns",
			language:    "javascript",
			pattern: `
                (function_declaration name: (identifier) @x)
                (statement_identifier) @y
                (property_identifier)+ @z
                (array (identifier)* @x)
            `,
			captureQuantifiers: []quantifier{
				// x
				{0, "x", transit.QuantifierOne},
				{1, "x", transit.QuantifierZero},
				{2, "x", transit.QuantifierZero},
				{3, "x", transit.QuantifierZeroOrMore},
				// y
				{0, "y", transit.QuantifierZero},
				{1, "y", transit.QuantifierOne},
				{2, "y", transit.QuantifierZero},
				{3, "y", transit.QuantifierZero},
				// z
				{0, "z", transit.QuantifierZero},
				{1, "z", transit.QuantifierZero},
				{2, "z", transit.QuantifierOneOrMore},
				{3, "z", transit.QuantifierZero},
			},
		},
		{
			description: "multiple alternatives",
			language:    "javascript",
			pattern: `
            [
                (array (identifier) @x)
                (function_declaration name: (identifier)+ @x)
            ]
            [
                (array (identifier) @x)
                (function_declaration name: (identifier)+ @x)
            ]
            `,
			captureQuantifiers: []quantifier{
				{0, "x", transit.QuantifierOneOrMore},
				{1, "x", transit.QuantifierOneOrMore},
			},
		},
	}

	for _, row := range rows {
		if uqeSkipExample(row.description) {
			continue
		}
		t.Logf("  query example: %q", row.description)
		query := uqNewQuery(t, uqLanguage(t, row.language), row.pattern)
		for _, q := range row.captureQuantifiers {
			index, ok := query.CaptureIndexForName(q.capture)
			if !ok {
				t.Fatalf("CaptureIndexForName(%q) found no capture", q.capture)
			}
			actual := query.CaptureQuantifiers(q.pattern)[index]
			if actual != q.quantifier {
				t.Errorf("Description: %s, Pattern: %q, expected quantifier of @%s to be %v instead of %v",
					row.description, uqeCollapse(row.pattern), q.capture, q.quantifier, actual)
			}
		}
	}
}

func TestQueryQuantifiedCaptures(t *testing.T) {
	type row struct {
		description string
		language    string
		code        string
		pattern     string
		captures    uqCaptures
	}

	// #[rustfmt::skip]
	rows := []row{
		{
			description: "doc comments where all must match the prefix",
			language:    "c",
			code: uqUnindent(`
            /// foo
            /// bar
            /// baz

            void main() {}

            /// qux
            /// quux
            // quuz
        `),
			pattern: `
                ((comment)+ @comment.documentation
                  (#match? @comment.documentation "^///"))
            `,
			captures: uqCaptures{
				{"comment.documentation", "/// foo"},
				{"comment.documentation", "/// bar"},
				{"comment.documentation", "/// baz"},
			},
		},
		{
			description: "doc comments where one must match the prefix",
			language:    "c",
			code: uqUnindent(`
            /// foo
            /// bar
            /// baz

            void main() {}

            /// qux
            /// quux
            // quuz
        `),
			pattern: `
                ((comment)+ @comment.documentation
                  (#any-match? @comment.documentation "^///"))
            `,
			captures: uqCaptures{
				{"comment.documentation", "/// foo"},
				{"comment.documentation", "/// bar"},
				{"comment.documentation", "/// baz"},
				{"comment.documentation", "/// qux"},
				{"comment.documentation", "/// quux"},
				{"comment.documentation", "// quuz"},
			},
		},
		{
			description: "multiple quantifiers should not hang query parsing",
			language:    "c",
			code: uqUnindent(`
            // foo
            // bar
            // baz
        `),
			pattern: `
                ((comment) ?+ @comment)
            `,
			// This should be identical to the `*` quantifier.
			captures: uqCaptures{
				{"comment", "// foo"},
				{"comment", "// foo"},
				{"comment", "// foo"},
				{"comment", "// bar"},
				{"comment", "// baz"},
			},
		},
	}

	for _, row := range rows {
		t.Logf("  quantified query example: %q", row.description)

		language := uqLanguage(t, row.language)
		tree := uqParse(t, language, row.code)

		query := uqNewQuery(t, language, row.pattern)

		cursor := transit.NewQueryCursor()
		matches := cursor.Captures(t.Context(), query, tree.RootNode(), []byte(row.code))

		uqCheckCaptures(t, uqCollectCaptures(matches, query, row.code), row.captures)
	}
}

func TestQueryMaxStartDepth(t *testing.T) {
	type row struct {
		description string
		pattern     string
		depth       int
		matches     []uqMatch
	}

	source := uqUnindent(`
        if (a1 && a2) {
            if (b1 && b2) { }
            if (c) { }
        }
        if (d) {
            if (e1 && e2) { }
            if (f) { }
        }
    `)

	rows := []row{
		{
			description: "depth 0: match translation unit",
			depth:       0,
			pattern: `
                (translation_unit) @capture
            `,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "if (a1 && a2) {\n    if (b1 && b2) { }\n    if (c) { }\n}\nif (d) {\n    if (e1 && e2) { }\n    if (f) { }\n}\n"}}},
			},
		},
		{
			description: "depth 0: match none",
			depth:       0,
			pattern: `
                (if_statement) @capture
            `,
			matches: nil,
		},
		{
			description: "depth 1: match 2 if statements at the top level",
			depth:       1,
			pattern: `
                (if_statement) @capture
            `,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "if (a1 && a2) {\n    if (b1 && b2) { }\n    if (c) { }\n}"}}},
				{0, uqCaptures{{"capture", "if (d) {\n    if (e1 && e2) { }\n    if (f) { }\n}"}}},
			},
		},
		{
			description: "depth 1 with deep pattern: match the only the first if statement",
			depth:       1,
			pattern: `
                (if_statement
                    condition: (parenthesized_expression
                        (binary_expression)
                    )
                ) @capture
            `,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "if (a1 && a2) {\n    if (b1 && b2) { }\n    if (c) { }\n}"}}},
			},
		},
		{
			description: "depth 3 with deep pattern: match all if statements with a binexpr condition",
			depth:       3,
			pattern: `
                (if_statement
                    condition: (parenthesized_expression
                        (binary_expression)
                    )
                ) @capture
            `,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "if (a1 && a2) {\n    if (b1 && b2) { }\n    if (c) { }\n}"}}},
				{0, uqCaptures{{"capture", "if (b1 && b2) { }"}}},
				{0, uqCaptures{{"capture", "if (e1 && e2) { }"}}},
			},
		},
	}

	language := uqLanguage(t, "c")
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()

	for _, row := range rows {
		t.Logf("  query example: %q", row.description)

		query := uqNewQuery(t, language, row.pattern)
		cursor.SetMaxStartDepth(row.depth)

		matches := cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source))
		uqCheckMatches(t, uqCollectMatches(matches, query, source), row.matches)
	}
}

func TestQueryErrorDoesNotOOB(t *testing.T) {
	language := uqLanguage(t, "javascript")

	uqCheckQueryError(t, language, "(clas", transit.QueryError{
		Row:     0,
		Offset:  1,
		Column:  1,
		Kind:    transit.QueryErrorNodeType,
		Message: "\"clas\"",
	})
}

func TestConsecutiveZeroOrModifiers(t *testing.T) {
	language := uqLanguage(t, "javascript")

	zeroSource := ""
	threeSource := "/**/ /**/ /**/"

	zeroTree := uqParse(t, language, zeroSource)
	threeTree := uqParse(t, language, threeSource)

	tests := []string{
		"(comment)*** @capture",
		"(comment)??? @capture",
		"(comment)*?* @capture",
		"(comment)?*? @capture",
	}

	for _, test := range tests {
		query := uqNewQuery(t, language, test)

		cursor := transit.NewQueryCursor()
		found := false
		for range cursor.Matches(t.Context(), query, zeroTree.RootNode(), []byte(zeroSource)) {
			found = true
			break
		}
		if !found {
			t.Errorf("%s: no match in the empty source", test)
		}

		cursor = transit.NewQueryCursor()

		len3 := false
		len1 := false

		for m := range cursor.Matches(t.Context(), query, threeTree.RootNode(), []byte(threeSource)) {
			if len(m.Captures) == 3 {
				len3 = true
			}
			if len(m.Captures) == 1 {
				len1 = true
			}
		}

		if want := strings.Contains(test, "*"); len3 != want {
			t.Errorf("%s: a match of 3 captures is %v, want %v", test, len3, want)
		}
		if want := strings.Contains(test, "???"); len1 != want {
			t.Errorf("%s: a match of 1 capture is %v, want %v", test, len1, want)
		}
	}
}

func TestQueryMaxStartDepthMore(t *testing.T) {
	type row struct {
		depth   int
		matches []uqMatch
	}

	source := uqUnindent(`
        {
            { }
            {
                { }
            }
        }
    `)

	rows := []row{
		{
			depth: 0,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "{\n    { }\n    {\n        { }\n    }\n}"}}},
			},
		},
		{
			depth: 1,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "{\n    { }\n    {\n        { }\n    }\n}"}}},
				{0, uqCaptures{{"capture", "{ }"}}},
				{0, uqCaptures{{"capture", "{\n        { }\n    }"}}},
			},
		},
		{
			depth: 2,
			matches: []uqMatch{
				{0, uqCaptures{{"capture", "{\n    { }\n    {\n        { }\n    }\n}"}}},
				{0, uqCaptures{{"capture", "{ }"}}},
				{0, uqCaptures{{"capture", "{\n        { }\n    }"}}},
				{0, uqCaptures{{"capture", "{ }"}}},
			},
		},
	}

	language := uqLanguage(t, "c")
	tree := uqParse(t, language, source)
	cursor := transit.NewQueryCursor()
	query := uqNewQuery(t, language, "(compound_statement) @capture")

	var node transit.Node
	found := false
	for m := range cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)) {
		node = m.Captures[0].Node
		found = true
		break
	}
	if !found {
		t.Fatal("no match of (compound_statement)")
	}
	if node.Kind() != "compound_statement" {
		t.Fatalf("Kind() = %q, want compound_statement", node.Kind())
	}

	for _, row := range rows {
		t.Logf("  depth: %d", row.depth)

		cursor.SetMaxStartDepth(row.depth)

		matches := cursor.Matches(t.Context(), query, node, []byte(source))
		uqCheckMatches(t, uqCollectMatches(matches, query, source), row.matches)
	}
}

func TestGrammarWithAliasedLiteralQuery(t *testing.T) {
	// module.exports = grammar({
	//   name: 'test',
	//
	//   rules: {
	//     source: $ => repeat(choice($.compound_statement, $.expansion)),
	//
	//     compound_statement: $ => seq(alias(token(prec(-1, '}')), '}')),
	//
	//     expansion: $ => seq('}'),
	//   },
	// });
	language := uqTestLanguage(t, `
        {
            "name": "test",
            "rules": {
                "source": {
                    "type": "REPEAT",
                    "content": {
                        "type": "CHOICE",
                        "members": [
                            {
                                "type": "SYMBOL",
                                "name": "compound_statement"
                            },
                            {
                                "type": "SYMBOL",
                                "name": "expansion"
                            }
                        ]
                    }
                },
                "compound_statement": {
                    "type": "SEQ",
                    "members": [
                        {
                            "type": "ALIAS",
                            "content": {
                                "type": "TOKEN",
                                "content": {
                                    "type": "PREC",
                                    "value": -1,
                                    "content": {
                                        "type": "STRING",
                                        "value": "}"
                                    }
                                }
                            },
                            "named": false,
                            "value": "}"
                        }
                    ]
                },
                "expansion": {
                    "type": "SEQ",
                    "members": [
                        {
                            "type": "STRING",
                            "value": "}"
                        }
                    ]
                }
            }
        }
        `)

	_, err := transit.NewQuery(language, `
        (compound_statement "}" @bracket1)
        (expansion "}" @bracket2)
        `)

	if err != nil {
		t.Errorf("NewQuery: %v", err)
	}
}

func TestQueryWithFirstChildInGroupIsAnchor(t *testing.T) {
	language := uqLanguage(t, "c")
	sourceCode := `void fun(int a, char b, int c) { };`
	query := `
            (parameter_list
              .
              ((parameter_declaration) @constant
                (#match? @constant "^int")))`
	q := uqNewQuery(t, language, query)
	uqAssertQueryMatches(t, language, q, sourceCode, []uqMatch{
		{0, uqCaptures{{"constant", "int a"}}},
	})
}

// This test needs be executed with UBSAN enabled to check for regressions:
// ```
// UBSAN_OPTIONS="halt_on_error=1" \
// CFLAGS="-fsanitize=undefined"   \
// RUSTFLAGS="-lubsan"             \
// cargo test --target $(rustc -vV | sed -nr 's/^host: //p') -- --test-threads 1
// ```
//
// The Go runtime has no undefined behavior to find, and a read out of the
// bounds of a slice panics, so the Go test only compiles the query.
func TestQueryCompilerOOBAccess(t *testing.T) {
	language := uqLanguage(t, "java")
	// UBSAN should not report any OOB access
	if _, err := transit.NewQuery(language, "(package_declaration _ (_) @name _)"); err != nil {
		t.Errorf("NewQuery: %v", err)
	}
}

func TestQueryWildcardWithImmediateFirstChild(t *testing.T) {
	language := uqLanguage(t, "javascript")
	query := uqNewQuery(t, language, "(_ . (identifier) @firstChild)")
	source := "function name(one, two, three) { }"

	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"firstChild", "name"}}},
		{0, uqCaptures{{"firstChild", "one"}}},
	})
}

func TestQueryOnEmptySourceCode(t *testing.T) {
	language := uqLanguage(t, "javascript")
	sourceCode := ""
	query := "(program) @program"
	q := uqNewQuery(t, language, query)
	uqAssertQueryMatches(t, language, q, sourceCode, []uqMatch{
		{0, uqCaptures{{"program", ""}}},
	})
}

// The Rust test stops the run with a progress callback after 1000
// microseconds. The Go API stops a run when its context ends, so the Go
// test gives the run a context with a deadline of 1000 microseconds.
func TestQueryExecutionWithTimeout(t *testing.T) {
	language := uqLanguage(t, "javascript")
	parser := uqParser(t, language)

	count := 10_000
	sourceCode := strings.Repeat("function foo() { while (true) { } }\n", count)
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}

	query := uqNewQuery(t, language, "(function_declaration) @function")
	ctx, cancel := context.WithTimeout(t.Context(), 1000*time.Microsecond)
	defer cancel()
	cursor := transit.NewQueryCursor()
	matches := 0
	for range cursor.Matches(ctx, query, tree.RootNode(), []byte(sourceCode)) {
		matches++
	}
	if matches >= count {
		t.Errorf("%d matches before the deadline, want fewer than %d", matches, count)
	}

	matches = 0
	for range cursor.Matches(t.Context(), query, tree.RootNode(), []byte(sourceCode)) {
		matches++
	}
	if matches != count {
		t.Errorf("%d matches, want %d", matches, count)
	}
}

// The Go API has no progress callback. The context of a run takes its
// place, and the Go test cannot see that the run looks at it. So the Go
// test drops the check of callback_was_called, and keeps the count of the
// matches of a run with a context.
func TestQueryProgressCallbackLivesAsLongAsMatches(t *testing.T) {
	language := uqLanguage(t, "javascript")
	parser := uqParser(t, language)

	sourceCode := strings.Repeat("function foo() {}\n", 1000)
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}
	query := uqNewQuery(t, language, "(function_declaration) @function")

	cursor := transit.NewQueryCursor()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	matches := cursor.Matches(ctx, query, tree.RootNode(), []byte(sourceCode))

	count := 0
	for range matches {
		count++
	}
	if count != 1000 {
		t.Errorf("%d matches, want 1000", count)
	}
}

// The context of uqProgressContext calls the function each time that the
// cursor reads its Err, as C calls the progress callback.
func TestQueryCapturesProgressCallbackStopsBehindAnOpenMatch(t *testing.T) {
	language := uqLanguage(t, "javascript")
	parser := uqParser(t, language)

	numbers := make([]string, 1000)
	for i := range numbers {
		numbers[i] = strconv.Itoa(i)
	}
	sourceCode := "[" + strings.Join(numbers, ",") + "];"
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}

	// The first pattern stays in progress until the array ends, so every number
	// capture finishes behind it.
	query := uqNewQuery(t, language, "(array (number) @first (string)) (number) @number")

	calls := 0
	ctx := uqProgressContext(func() bool {
		calls++
		return true
	})
	cursor := transit.NewQueryCursor()
	captures := 0
	for range cursor.Captures(ctx, query, tree.RootNode(), []byte(sourceCode)) {
		captures++
	}

	if captures >= 1000 {
		t.Errorf("%d captures, want fewer than 1000", captures)
	}
	if calls != 1 {
		t.Errorf("%d calls of the progress function, want 1", calls)
	}
}

func TestQueryCapturesProgressCallbackDiscardsInProgressMatches(t *testing.T) {
	language := uqLanguage(t, "json")
	parser := uqParser(t, language)

	// Within each element, the first pattern holds back the second pattern's captures
	// until the inner array ends. The second pattern then stays in progress, but
	// definite, until the element itself ends.
	trues := make([]string, 12)
	for i := range trues {
		trues[i] = "true"
	}
	element := `[[{"a":1},1,2],` + strings.Join(trues, ",") + "]"
	elements := make([]string, 50)
	for i := range elements {
		elements[i] = element
	}
	sourceCode := "[" + strings.Join(elements, ",") + "]"
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}
	query := uqNewQuery(t, language, `
        (array (object) @object (string))
        (array (array (number) @number) "]" @close)
        (number) @n
        `)

	for cancelAt := 1; cancelAt <= 20; cancelAt++ {
		calls := 0
		ctx := uqProgressContext(func() bool {
			calls++
			return calls >= cancelAt
		})
		cursor := transit.NewQueryCursor()
		for m := range cursor.Captures(ctx, query, tree.RootNode(), []byte(sourceCode)) {
			// Once the callback has asked to stop, only finished matches may be returned.
			if calls >= cancelAt && m.PatternIndex == 1 && len(m.Captures) != 2 {
				t.Errorf("cancelled at callback %d: a match of pattern 1 has %d captures, want 2", cancelAt, len(m.Captures))
			}
		}
	}
}

// Matches restarts the query at each range of its sequence, and the
// sequence ends when the cursor returns no match. So the Go test pulls once
// more from the sequence that the cancellation ended, and that pull gives
// nothing.
func TestQueryProgressCallbackHaltsForGood(t *testing.T) {
	language := uqLanguage(t, "javascript")
	parser := uqParser(t, language)

	sourceCode := strings.Repeat("function foo() {}\n", 1000)
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}
	query := uqNewQuery(t, language, "(function_declaration) @function")

	stop := true
	ctx := uqProgressContext(func() bool {
		return stop
	})
	cursor := transit.NewQueryCursor()
	next, done := iter.Pull(cursor.Matches(ctx, query, tree.RootNode(), []byte(sourceCode)))
	defer done()
	count := 0
	for {
		if _, ok := next(); !ok {
			break
		}
		count++
	}
	if count >= 1000 {
		t.Errorf("%d matches, want fewer than 1000", count)
	}

	// Asking the cursor for more after a cancellation does not resume the query.
	stop = false
	if _, ok := next(); ok {
		t.Error("the cursor gave a match after the cancellation")
	}
}

func TestQueryProgressCallbackKeepsUpWithInProgressStates(t *testing.T) {
	language := uqLanguage(t, "javascript")
	parser := uqParser(t, language)

	depth := 1000
	sourceCode := strings.Repeat("[", depth) + strings.Repeat("]", depth) + ";"
	tree, err := parser.Parse(t.Context(), []byte(sourceCode), nil)
	if err != nil {
		t.Fatal(err)
	}

	// No array has a string, so every enclosing array keeps a state in
	// progress, and the cursor visits all of them at every node it steps over.
	query := uqNewQuery(t, language, "(array (string) @string)")

	calls := 0
	ctx := uqProgressContext(func() bool {
		calls++
		return false
	})
	cursor := transit.NewQueryCursor()
	matches := 0
	for range cursor.Matches(ctx, query, tree.RootNode(), []byte(sourceCode)) {
		matches++
	}

	if matches != 0 {
		t.Errorf("%d matches, want 0", matches)
	}
	if calls <= depth {
		t.Errorf("%d calls for %d nested arrays", calls, depth)
	}
}

func TestQueryExecutionWithPointsCausingUnderflow(t *testing.T) {
	language := uqLanguage(t, "rust")
	parser := uqParser(t, language)

	code := `fn main() {
    println!("{:?}", foo());
}`
	if err := parser.SetIncludedRanges([]transit.Range{{
		StartByte:  24,
		EndByte:    39,
		StartPoint: transit.Point{Row: 0, Column: 0}, // 5, 12
		EndPoint:   transit.Point{Row: 0, Column: 0}, // 5, 27
	}}); err != nil {
		t.Fatal(err)
	}

	query := uqNewQuery(t, language, "(call_expression) @cap")
	cursor := transit.NewQueryCursor()

	tree, err := parser.Parse(t.Context(), []byte(code), nil)
	if err != nil {
		t.Fatal(err)
	}

	rootNode := tree.RootNode()
	matches := uqCollectMatches(cursor.Matches(t.Context(), query, rootNode, []byte(code)), query, code)

	tree.Edit(transit.InputEdit{
		StartByte:   40,
		OldEndByte:  40,
		NewEndByte:  41,
		StartPoint:  transit.Point{Row: 1, Column: 28},
		OldEndPoint: transit.Point{Row: 1, Column: 28},
		NewEndPoint: transit.Point{Row: 2, Column: 0},
	})

	tree2, err := parser.Parse(t.Context(), []byte(code), tree)
	if err != nil {
		t.Fatal(err)
	}

	rootNode = tree2.RootNode()
	matches2 := uqCollectMatches(cursor.Matches(t.Context(), query, rootNode, []byte(code)), query, code)

	uqCheckMatches(t, matches, matches2)
}

func TestWildcardBehaviorBeforeAnchor(t *testing.T) {
	language := uqLanguage(t, "python")

	source := `
        (a, b)
        (c, d,)
    `

	//  In this query, we're targeting any *named* node immediately before a closing parenthesis.
	query := uqNewQuery(t, language, `(tuple (_) @last . ")" .) @match`)
	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"match", "(a, b)"}, {"last", "b"}}},
		{0, uqCaptures{{"match", "(c, d,)"}, {"last", "d"}}},
	})

	// In this query, we're targeting *any* node immediately before a closing
	// parenthesis.
	query = uqNewQuery(t, language, `(tuple _ @last . ")" .) @match`)
	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"match", "(a, b)"}, {"last", "b"}}},
		{0, uqCaptures{{"match", "(c, d,)"}, {"last", ","}}},
	})
}

func TestPatternAlternativesFollowLastChildConstraint(t *testing.T) {
	language := uqLanguage(t, "rust")

	code := `
fn f() {
    if a {} // <- should NOT match
    if b {}
}`

	tree := uqParse(t, language, code)
	cursor := transit.NewQueryCursor()

	query := uqNewQuery(t, language, `(block
        [
            (type_cast_expression)
            (expression_statement)
        ] @last
        .
        )`)

	rootNode := tree.RootNode()
	matches := uqCollectMatches(cursor.Matches(t.Context(), query, rootNode, []byte(code)), query, code)

	flippedQuery := uqNewQuery(t, language, `(block
        [
            (expression_statement)
            (type_cast_expression)
        ] @last
        .
        )`)

	flippedMatches := uqCollectMatches(cursor.Matches(t.Context(), flippedQuery, rootNode, []byte(code)), flippedQuery, code)

	uqCheckMatches(t, matches, []uqMatch{{0, uqCaptures{{"last", "if b {}"}}}})
	uqCheckMatches(t, matches, flippedMatches)
}

func TestWildcardParentAllowsFallibleChildPatterns(t *testing.T) {
	language := uqLanguage(t, "javascript")

	sourceCode := `
function foo() {
    "bar"
}
    `

	query := uqNewQuery(t, language, `(function_declaration
          (_
            (expression_statement)
          )
        ) @part`)

	uqAssertQueryMatches(t, language, query, sourceCode, []uqMatch{
		{0, uqCaptures{{"part", "function foo() {\n    \"bar\"\n}"}}},
	})
}

func TestUnfinishedCapturesAreNotDefiniteWithPendingAnchors(t *testing.T) {
	language := uqLanguage(t, "javascript")

	sourceCode := `
const foo = [
  1, 2, 3
]
`

	tree := uqParse(t, language, sourceCode)
	query := uqNewQuery(t, language, `(array (_) @foo . "]")`)
	matchesCursor := transit.NewQueryCursor()
	capturesCursor := transit.NewQueryCursor()

	captures := uqCollectCaptures(capturesCursor.Captures(t.Context(), query, tree.RootNode(), []byte(sourceCode)), query, sourceCode)

	matches := uqCollectMatches(matchesCursor.Matches(t.Context(), query, tree.RootNode(), []byte(sourceCode)), query, sourceCode)

	uqCheckCaptures(t, captures, uqCaptures{{"foo", "3"}})
	if len(matches) != 1 {
		t.Fatalf("%d matches, want 1", len(matches))
	}
	uqCheckCaptures(t, matches[0].Captures, captures)
}

func TestQueryWithPredicateCausingOOBAccess(t *testing.T) {
	language := uqLanguage(t, "rust")

	query := "(call_expression\n" +
		"     function: (scoped_identifier\n" +
		"       path: (scoped_identifier (identifier) @_regex (#any-of? @_regex \"Regex\" \"RegexBuilder\") .))\n" +
		"     (#set! injection.language \"regex\"))"
	uqNewQuery(t, language, query)
}

func TestQueryWithAnonymousErrorNode(t *testing.T) {
	language := uqTestFixtureLanguage(t, "anonymous_error")

	source := "ERROR"

	tree := uqParse(t, language, source)
	query := uqNewQuery(t, language, `
          "ERROR" @error
          (document "ERROR" @error)
        `)
	cursor := transit.NewQueryCursor()
	matches := uqCollectMatches(cursor.Matches(t.Context(), query, tree.RootNode(), []byte(source)), query, source)

	uqCheckMatches(t, matches, []uqMatch{
		{1, uqCaptures{{"error", "ERROR"}}},
		{0, uqCaptures{{"error", "ERROR"}}},
	})
}

func TestQueryAllowsErrorNodesWithChildren(t *testing.T) {
	language := uqLanguage(t, "cpp")

	code := "SomeStruct foo{.bar{}};"

	tree := uqParse(t, language, code)
	root := tree.RootNode()

	query := uqNewQuery(t, language, "(initializer_list (ERROR) @error)")
	cursor := transit.NewQueryCursor()

	matches := uqCollectMatches(cursor.Matches(t.Context(), query, root, []byte(code)), query, code)
	uqCheckMatches(t, matches, []uqMatch{{0, uqCaptures{{"error", ".bar"}}}})
}

// The Rust test loads the grammar from the text of its grammar.js with
// load_grammar_file, which runs node. The Go test holds the grammar.json
// that load_grammar_file of upstream writes for that grammar.js:
//
//	export default grammar({
//	  name: "query_assertion_crash",
//
//	  rules: {
//	    source_file: $ => repeat($.expression),
//
//	    expression: $ => choice(
//	      $.await_binding,
//	      $.await_expr,
//	      $.equal_expr,
//	      prec(3, $.identifier),
//	    ),
//
//	    await_binding: $ => prec(1, seq('await', $.identifier, '=', $.expression)),
//
//	    await_expr: $ => prec(1, seq('await', $.expression)),
//
//	    equal_expr: $ => prec.right(2, seq($.expression, '=', $.expression)),
//
//	    identifier: _ => /[a-z]+/,
//	  }
//	});
func TestQueryAssertionOnUnreachableNodeWithChild(t *testing.T) {
	// The `await_binding` rule is unreachable because it has a lower precedence than
	// `identifier`, so we'll always reduce to an expression of type `identifier`
	// instead whenever we see the token `await` followed by an identifier.
	//
	// A query that tries to capture the `await` token in the `await_binding` rule
	// should not cause an assertion failure during query analysis.
	grammarJSON := `{
  "$schema": "https://tree-sitter.github.io/tree-sitter/assets/schemas/grammar.schema.json",
  "name": "query_assertion_crash",
  "rules": {
    "source_file": {"type": "REPEAT", "content": {"type": "SYMBOL", "name": "expression"}},
    "expression": {
      "type": "CHOICE",
      "members": [
        {"type": "SYMBOL", "name": "await_binding"},
        {"type": "SYMBOL", "name": "await_expr"},
        {"type": "SYMBOL", "name": "equal_expr"},
        {"type": "PREC", "value": 3, "content": {"type": "SYMBOL", "name": "identifier"}}
      ]
    },
    "await_binding": {
      "type": "PREC",
      "value": 1,
      "content": {
        "type": "SEQ",
        "members": [
          {"type": "STRING", "value": "await"},
          {"type": "SYMBOL", "name": "identifier"},
          {"type": "STRING", "value": "="},
          {"type": "SYMBOL", "name": "expression"}
        ]
      }
    },
    "await_expr": {
      "type": "PREC",
      "value": 1,
      "content": {
        "type": "SEQ",
        "members": [
          {"type": "STRING", "value": "await"},
          {"type": "SYMBOL", "name": "expression"}
        ]
      }
    },
    "equal_expr": {
      "type": "PREC_RIGHT",
      "value": 2,
      "content": {
        "type": "SEQ",
        "members": [
          {"type": "SYMBOL", "name": "expression"},
          {"type": "STRING", "value": "="},
          {"type": "SYMBOL", "name": "expression"}
        ]
      }
    },
    "identifier": {"type": "PATTERN", "value": "[a-z]+"}
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}],
  "conflicts": [],
  "precedences": [],
  "externals": [],
  "inline": [],
  "supertypes": [],
  "reserved": {}
}`

	language := uqTestLanguage(t, grammarJSON)

	uqCheckQueryError(t, language, `(await_binding "await")`, transit.QueryError{
		Kind:    transit.QueryErrorStructure,
		Row:     0,
		Offset:  0,
		Column:  0,
		Message: uqLines("(await_binding \"await\")", "^"),
	})
}

// The Rust test loads the grammar from the text of its grammar.js with
// load_grammar_file, which runs node. The Go test holds the grammar.json
// that load_grammar_file of upstream writes for that grammar.js:
//
//	export default grammar({
//	  name: "supertype_anonymous_test",
//
//	  extras: $ => [/\s/, $.comment],
//
//	  supertypes: $ => [$.expression],
//
//	  word: $ => $.identifier,
//
//	  rules: {
//	    source_file: $ => repeat($.expression),
//
//	    expression: $ => choice(
//	      $.function_call,
//	      '()' // an empty tuple, which should be queryable with the supertype syntax
//	    ),
//
//	    function_call: $ => seq($.identifier, '()'),
//
//	    identifier: _ => /[a-zA-Z_][a-zA-Z0-9_]*/,
//
//	    comment: _ => token(seq('//', /.*/)),
//	  }
//	});
func TestQuerySupertypeWithAnonymousNode(t *testing.T) {
	grammarJSON := `{
  "$schema": "https://tree-sitter.github.io/tree-sitter/assets/schemas/grammar.schema.json",
  "name": "supertype_anonymous_test",
  "word": "identifier",
  "rules": {
    "source_file": {"type": "REPEAT", "content": {"type": "SYMBOL", "name": "expression"}},
    "expression": {
      "type": "CHOICE",
      "members": [
        {"type": "SYMBOL", "name": "function_call"},
        {"type": "STRING", "value": "()"}
      ]
    },
    "function_call": {
      "type": "SEQ",
      "members": [
        {"type": "SYMBOL", "name": "identifier"},
        {"type": "STRING", "value": "()"}
      ]
    },
    "identifier": {"type": "PATTERN", "value": "[a-zA-Z_][a-zA-Z0-9_]*"},
    "comment": {
      "type": "TOKEN",
      "content": {
        "type": "SEQ",
        "members": [
          {"type": "STRING", "value": "//"},
          {"type": "PATTERN", "value": ".*"}
        ]
      }
    }
  },
  "extras": [{"type": "PATTERN", "value": "\\s"}, {"type": "SYMBOL", "name": "comment"}],
  "conflicts": [],
  "precedences": [],
  "externals": [],
  "inline": [],
  "supertypes": ["expression"],
  "reserved": {}
}`

	language := uqTestLanguage(t, grammarJSON)

	query, err := transit.NewQuery(language, `(expression/"()") @tuple`)
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}

	source := "foo()\n()"

	uqAssertQueryMatches(t, language, query, source, []uqMatch{{0, uqCaptures{{"tuple", "()"}}}})
}

func TestLastChildAnchorLooksPastHiddenRepeat(t *testing.T) {
	language := uqTestFixtureLanguage(t, "last_child_anchor_past_hidden_repeat")

	source := "T a.b.c\nL a.b.c\nN a!.b!.c\n"

	query := uqNewQuery(t, language, `
        (trailing_sep (name) @last .)
        (leading_sep (name) @last .)
        (trailing_named (name) @last .)
        `)

	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"last", "c"}}},
		{1, uqCaptures{{"last", "c"}}},
		{2, uqCaptures{{"last", "c"}}},
	})

	query = uqNewQuery(t, language, `
        (trailing_sep . (name) @first)
        (trailing_sep (name) @a . (name) @b)
        `)

	uqAssertQueryMatches(t, language, query, source, []uqMatch{
		{0, uqCaptures{{"first", "a"}}},
		{1, uqCaptures{{"a", "a"}, {"b", "b"}}},
		{1, uqCaptures{{"a", "b"}, {"b", "c"}}},
	})
}

func TestLastChildAnchorLooksPastHiddenNode(t *testing.T) {
	language := uqLanguage(t, "c")

	query := uqNewQuery(t, language, "(translation_unit (_) @last .)")

	source := "enum E { A };\nint x;\nint y;\n"

	uqAssertQueryMatches(t, language, query, source, []uqMatch{{0, uqCaptures{{"last", "int y;"}}}})
}
