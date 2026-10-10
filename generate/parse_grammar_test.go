package generate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestParseGrammar is test_parse_grammar in parse_grammar.rs.
func TestParseGrammar(t *testing.T) {
	t.Parallel()
	g, err := ParseGrammar([]byte(`{
		"name": "my_lang",
		"rules": {
			"file": {
				"type": "REPEAT1",
				"content": {
					"type": "SYMBOL",
					"name": "statement"
				}
			},
			"statement": {
				"type": "STRING",
				"value": "foo"
			}
		}
	}`), new([]Diagnostic))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if name := g.Pool.Resolve(g.Name); name != "my_lang" {
		t.Errorf("expected %q, got: %q", "my_lang", name)
	}
	var names []string
	for _, v := range g.Variables {
		names = append(names, g.Pool.Resolve(v.Name))
	}
	if len(names) != 2 || names[0] != "file" || names[1] != "statement" {
		t.Fatalf("expected [file statement], got: %v", names)
	}
	// file = repeat(named_symbol "statement")
	file := g.Pool.Node(g.Variables[0].Root)
	if file.Kind != RuleRepeat {
		t.Fatalf("expected a repeat, got: %+v", file)
	}
	if inner := g.Pool.Node(file.Child); inner.Kind != RuleNamedSymbol || g.Pool.Resolve(inner.Str) != "statement" {
		t.Errorf("expected the named symbol statement, got: %+v", inner)
	}
	// statement = string "foo"
	if s := g.Pool.Node(g.Variables[1].Root); s.Kind != RuleString || g.Pool.Resolve(s.Str) != "foo" {
		t.Errorf("expected the string foo, got: %+v", s)
	}
}

// TestParseGrammarKeepsTheOrderOfTheRules makes sure that the rules keep the
// order of the document, as the preserve_order of serde_json keeps it, and not
// the order of their names.
func TestParseGrammarKeepsTheOrderOfTheRules(t *testing.T) {
	t.Parallel()
	g, err := ParseGrammar([]byte(`{"name": "x", "rules": {
		"zebra": {"type": "SYMBOL", "name": "apple"},
		"apple": {"type": "SYMBOL", "name": "mango"},
		"mango": {"type": "STRING", "value": "m"}}}`), new([]Diagnostic))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range g.Variables {
		names = append(names, g.Pool.Resolve(v.Name))
	}
	if len(names) != 3 || names[0] != "zebra" || names[1] != "apple" || names[2] != "mango" {
		t.Errorf("expected [zebra apple mango], got: %v", names)
	}
}

// TestParseGrammarDropsUnusedRules makes sure that normalize drops a rule that
// nothing uses, and the references to it.
func TestParseGrammarDropsUnusedRules(t *testing.T) {
	t.Parallel()
	g, err := ParseGrammar([]byte(`{"name": "x",
		"rules": {
			"start": {"type": "STRING", "value": "a"},
			"unused": {"type": "STRING", "value": "b"}
		},
		"inline": ["unused"],
		"supertypes": ["unused"],
		"conflicts": [["start", "unused"]]}`), new([]Diagnostic))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Variables) != 1 || len(g.InlineNames) != 0 || len(g.SupertypeNames) != 0 || len(g.ConflictNames) != 0 {
		t.Errorf("expected only start and no references to unused, got %d variables, %v, %v, %v",
			len(g.Variables), g.InlineNames, g.SupertypeNames, g.ConflictNames)
	}
}

// TestParseGrammarErrors makes sure of the text of each error of upstream.
func TestParseGrammarErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		json string
		kind ParseGrammarErrorKind
		text string
	}{
		{`{"name": "x", "rules": {"a": {"type": "STRING", "value": "a"}}, "extras": [{"type": "STRING", "value": ""}]}`,
			ParseGrammarInvalidExtra, "Rules in the `extras` array must not contain empty strings"},
		{`{"name": "x", "rules": {"a": {"type": "STRING", "value": "a"}}, "precedences": [[{"type": "BLANK"}]]}`,
			ParseGrammarUnexpected, "Invalid rule in precedences array. Only strings and symbols are allowed"},
		{`{"name": "x", "rules": {"a": {"type": "STRING", "value": "a"}}, "reserved": {"global": {"type": "BLANK"}}}`,
			ParseGrammarInvalidReservedWordSet, "Reserved word sets must be arrays"},
		{`{"name": "x", "rules": {"a": {"type": "TOKEN", "content": {"type": "SYMBOL", "name": "b"}}}}`,
			ParseGrammarUnexpectedRule, "Grammar Error: Unexpected rule `b` in `token()` call"},
	} {
		_, err := ParseGrammar([]byte(c.json), new([]Diagnostic))
		var pe *ParseGrammarError
		if !errors.As(err, &pe) || pe.Kind != c.kind || pe.Error() != c.text {
			t.Errorf("expected %q, got: %v", c.text, err)
		}
	}
}

// TestParseGrammarWarnsOnFlags makes sure that a flag other than i, u and v
// gives a warning, and that only i stays in the pattern.
func TestParseGrammarWarnsOnFlags(t *testing.T) {
	t.Parallel()
	var diags []Diagnostic
	g, err := ParseGrammar([]byte(`{"name": "x", "rules": {"a": {"type": "PATTERN", "value": "a+", "flags": "giu"}}}`), &diags)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].String() != "unsupported regex flag `g` in pattern `a+`" {
		t.Errorf("expected one warning for the flag g, got: %v", diags)
	}
	if n := g.Pool.Node(g.Variables[0].Root); g.Pool.Resolve(n.Flags) != "i" {
		t.Errorf("expected the flags %q, got: %q", "i", g.Pool.Resolve(n.Flags))
	}
}

// TestParseGrammarReadsEveryTestGrammar reads the grammar.json of each test
// grammar of upstream, which the golden harness keeps in testdata (D58).
func TestParseGrammarReadsEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 70 {
		t.Fatalf("expected the grammar.json of the 70 test grammars in testdata, found %d", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := ParseGrammar(b, new([]Diagnostic))
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(g.Variables) == 0 {
			t.Errorf("%s: expected at least one rule", f)
		}
	}
}

// TestRegexMatchesEmpty makes sure that regexMatchesEmpty gives what
// Regex::new(pattern).is_ok_and(|r| r.is_match("")) of the Rust crate regex
// 1.13.1 gives. The expected values come from that crate. The same check
// agreed with the crate on every pattern of the 86 grammars on disk on
// 2026-09-29.
func TestRegexMatchesEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern  string
		expected bool
	}{
		{`a*`, true},
		{`a`, false},
		{`^`, true},
		{`$`, true},
		{`\b`, false},
		{`\B`, true},
		{`(a|)`, true},
		{`a?`, true},
		{`[`, false},
		{`(?-u:\xFF)*`, false},
		{`\b{start-half}`, true},
		{`\b{end-half}`, true},
		{`\b{start}`, false},
		{`\b{end}`, false},
		{`(?m)^$`, true},
		{`\A\z`, true},
		{`(?-u)\b`, false},
		{`(?-u)\B`, true},
		{`[^a]*`, true},
		{`[a&&b]`, false},
		{`(?i)`, true},
		{`()`, true},
		{`(?:)|a`, true},
		{`a{0}`, true},
		{`a{0,3}`, true},
		{`a{1,}`, false},
		{`x|`, true},
		{`\p{L}*`, true},
		{`[]a]`, false},
		{`(?x) a # c`, false},
		{`(?P<n>)`, true},
		{`\d{0}`, true},
		{`.`, false},
		{`.*`, true},
		{`(?s).?`, true},
		{`[\x00-\x{10FFFF}]*`, true},
		{`(?-u:[\x80-\xFF])*`, false},
		{`\u{D800}`, false},
		{`a**`, true},
		{`(`, false},
		{`)`, false},
		{`{`, false},
		{`\Q`, false},
		{`a{2,1}`, false},
		{`(?<=a)`, false},
		{`\1`, false},
	}
	for _, test := range tests {
		if actual := regexMatchesEmpty(test.pattern); actual != test.expected {
			t.Errorf("%q: expected %t, got: %t", test.pattern, test.expected, actual)
		}
	}
}
