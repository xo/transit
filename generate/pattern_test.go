package generate

import (
	"errors"
	"testing"

	"github.com/xo/transit/generate/internal/regexsyntax/ast"
	"github.com/xo/transit/generate/internal/regexsyntax/hir"
)

// TestRegexErrorKindsMirrorRegexSyntax makes sure that the text of each kind
// of RegexErrorKind is the text of the kind of regex-syntax that it mirrors.
// Upstream copied the texts by hand, so a new version of regex-syntax can
// change one of them.
func TestRegexErrorKindsMirrorRegexSyntax(t *testing.T) {
	t.Parallel()
	for kind, mirror := range astErrorKinds {
		if mirror == RegexNestLimitExceeded {
			continue
		}
		if expected, actual := (&ast.Error{Kind: kind}).Error(), regexErrorKindText[mirror]; actual != expected {
			t.Errorf("ast kind %d: expected %q, got: %q", kind, expected, actual)
		}
	}
	for kind, mirror := range hirErrorKinds {
		if expected, actual := (&hir.Error{Kind: kind}).Error(), regexErrorKindText[mirror]; actual != expected {
			t.Errorf("hir kind %d: expected %q, got: %q", kind, expected, actual)
		}
	}
	re := &RegexError{Kind: RegexNestLimitExceeded, NestLimit: 250}
	if expected, actual := (&ast.Error{Kind: ast.NestLimitExceeded, Limit: 250}).Error(), re.kindText(); actual != expected {
		t.Errorf("nest limit: expected %q, got: %q", expected, actual)
	}
}

// TestRegexErrorText makes sure of the text of a RegexError: the pattern, a
// line of carets under each span, and the kind.
func TestRegexErrorText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern  string
		expected string
	}{
		{"a(", "regex parse error:\n    a(\n     ^\nerror: unclosed group"},
		// the auxiliary span of a duplicate flag is the first use of it
		{"(?ii)", "regex parse error:\n    (?ii)\n      ^^\nerror: duplicate flag"},
		// a column counts characters, not bytes
		{"é\\p{Nope}", "regex parse error:\n    é\\p{Nope}\n     ^^^^^^^^\nerror: Unicode property not found"},
	}
	for _, test := range tests {
		_, err := parsePattern(test.pattern, false)
		re, ok := errors.AsType[*RegexError](err)
		if !ok {
			t.Errorf("%q: expected a RegexError, got: %v", test.pattern, err)
			continue
		}
		if actual := re.Error(); actual != test.expected {
			t.Errorf("%q: expected\n%s\ngot:\n%s", test.pattern, test.expected, actual)
		}
	}
}
