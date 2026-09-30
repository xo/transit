package grammartest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// This file ports the tests of parse_test_content in crates/cli/src/test.rs
// that the port of the corpus reader keeps (D35): the tests of the fields
// that the port has. The other tests check the report and the update of a
// corpus file, which the port leaves out.

// example is what a test expects of a corpus test.
type example struct {
	name   string
	input  string
	output string
}

// checkContent parses content and compares its tests with want.
func checkContent(t *testing.T, content string, want []example) {
	t.Helper()
	entry := parseTestContent("the-filename", strings.TrimSpace(content), "")
	if entry.name != "the-filename" || !entry.isGroup {
		t.Fatalf("the group is %q, group %t", entry.name, entry.isGroup)
	}
	if len(entry.children) != len(want) {
		t.Fatalf("found %d tests, want %d: %+v", len(entry.children), len(want), entry.children)
	}
	for i, w := range want {
		got := entry.children[i]
		if got.name != w.name || string(got.input) != w.input || got.output != w.output {
			t.Errorf("test %d is %q %q %q, want %q %q %q", i, got.name, got.input, got.output, w.name, w.input, w.output)
		}
		if a := got.attributes; !a.platform || a.failFast || a.expectation != expectPass || a.cst || len(a.languages) != 1 || a.languages[0] != "" {
			t.Errorf("test %d has the attributes %+v, want the default", i, a)
		}
	}
}

func TestParseTestContentSimple(t *testing.T) {
	checkContent(t, `
===============
The first test
===============

a b c

---

(a
    (b c))

================
The second test
================
d
---
(d)
`, []example{
		{"The first test", "\na b c\n", "(a (b c))"},
		{"The second test", "d", "(d)"},
	})
}

func TestParseTestContentWithDashesInSourceCode(t *testing.T) {
	checkContent(t, `
==================
Code with dashes
==================
abc
---
defg
----
hijkl
-------

(a (b))

=========================
Code ending with dashes
=========================
abc
-----------
-------------------

(c (d))
`, []example{
		{"Code with dashes", "abc\n---\ndefg\n----\nhijkl", "(a (b))"},
		{"Code ending with dashes", "abc\n-----------", "(c (d))"},
	})
}

func TestParseTestContentWithEqualsInSourceCode(t *testing.T) {
	checkContent(t, `
==========
First
==========
a
===
b
---
(a)

==========
Second
==========
c
---
(c)
`, []example{
		{"First", "a\n===\nb", "(a)"},
		{"Second", "c", "(c)"},
	})
}

func TestParseTestContentWithTiedDividerLength(t *testing.T) {
	checkContent(t, `
==========
Tied dashes
==========
a
---
b
---
(c)
`, []example{
		{"Tied dashes", "a\n---\nb", "(c)"},
	})
}

func TestParseTestContentWithCommentsInSexp(t *testing.T) {
	checkContent(t, `
==================
sexp with comment
==================
code
---

; Line start comment
(a (b))

==================
sexp with comment between
==================
code
---

; Line start comment
(a
; ignore this
    (b)
    ; also ignore this
)

=========================
sexp with ';'
=========================
code
---

(MISSING ";")
`, []example{
		{"sexp with comment", "code", "(a (b))"},
		{"sexp with comment between", "code", "(a (b))"},
		{"sexp with ';'", "code", `(MISSING ";")`},
	})
}

func TestParseTestContentWithNewlinesInTestNames(t *testing.T) {
	checkContent(t, `
===============
name
with
newlines
===============
a
---
(b)

====================
name with === signs
====================
code with ----
---
(d)
`, []example{
		{"name\nwith\nnewlines", "a", "(b)"},
		{"name with === signs", "code with ----", "(d)"},
	})
}

// TestParseTestAttributes checks the markers after the name of a test.
func TestParseTestAttributes(t *testing.T) {
	entry := parseTestContent("f", `==========
Skipped
:skip
:error
==========
a
---
(a)
==========
Languages
:language(x)
:language(y)
:fail-fast
:cst
:platform(nowhere)
==========
b
---
(b)
`, "")
	if len(entry.children) != 2 {
		t.Fatalf("found %d tests, want 2", len(entry.children))
	}
	if a := entry.children[0].attributes; a.expectation != expectSkip {
		t.Errorf("the first test expects %d, want :skip, which wins over :error", a.expectation)
	}
	a := entry.children[1].attributes
	if strings.Join(a.languages, ",") != "x,y" || !a.failFast || !a.cst || a.platform {
		t.Errorf("the second test has the attributes %+v", a)
	}
}

func TestStripSexpFields(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"(a key: (b) value: (c))", "(a (b) (c))"},
		{"(a (b))", "(a (b))"},
		{`(a ":": (b))`, `(a ":": (b))`},
		{"(a k-x: (b))", "(a k-x: (b))"},
	} {
		if got := stripSexpFields(c.in); got != c.want {
			t.Errorf("stripSexpFields(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeSexpOutput(t *testing.T) {
	got, hasFields := normalizeSexpOutput("\n(a\n  key: (b )\r\n  ; comment\n  (c))\n\n")
	if got != "(a key: (b) (c))" || !hasFields {
		t.Errorf("normalizeSexpOutput = %q, %t", got, hasFields)
	}
}

func TestParseTestsReadsAFolder(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"b.txt":       "===\nB\n===\nb\n---\n(b)\n",
		"a.txt":       "===\nA\n===\na\n---\n(a)\n",
		".hidden.txt": "===\nH\n===\nh\n---\n(h)\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entry, err := parseTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range entry.children {
		names = append(names, file.name+"/"+file.children[0].name)
	}
	if got := strings.Join(names, " "); got != "a/A b/B" {
		t.Errorf("the tests are %s, want a/A b/B", got)
	}
}

func TestToUTF8Point(t *testing.T) {
	src := []byte("ab\né x\n")
	for _, c := range []struct {
		point transit.Point
		want  utf8Point
	}{
		{transit.Point{Row: 0, Column: 0}, utf8Point{0, 0}},
		{transit.Point{Row: 0, Column: 1}, utf8Point{0, 1}},
		{transit.Point{Row: 1, Column: 2}, utf8Point{1, 1}},
		{transit.Point{Row: 1, Column: 3}, utf8Point{1, 2}},
	} {
		if got := toUTF8Point(c.point, src); got != c.want {
			t.Errorf("toUTF8Point(%v) = %v, want %v", c.point, got, c.want)
		}
	}
}

// TestIterateAssertions checks assertions against highlights in the order
// of the text, as get_highlight_positions gives them: the text of an escape
// inside a string, and a number.
func TestIterateAssertions(t *testing.T) {
	names := []string{"string", "escape", "number"}
	highlights := []highlightPosition{
		{start: utf8Point{0, 0}, end: utf8Point{0, 2}, highlight: 0},
		{start: utf8Point{0, 2}, end: utf8Point{0, 4}, highlight: 1},
		{start: utf8Point{0, 4}, end: utf8Point{0, 10}, highlight: 0},
		{start: utf8Point{1, 0}, end: utf8Point{1, 3}, highlight: 2},
	}
	for _, c := range []struct {
		name string
		a    assertion
		want string
	}{
		{"the innermost highlight", assertion{position: utf8Point{0, 2}, length: 1, expectedCaptureName: "escape"}, ""},
		{"the outer highlight", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "string"}, ""},
		{"a wrong name", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "number"},
			"Failure - row: 0, column: 6, expected highlight 'number', actual highlights: 'string'"},
		{"a negative assertion", assertion{position: utf8Point{1, 0}, length: 2, negative: true, expectedCaptureName: "string"}, ""},
		{"a negative assertion that fails", assertion{position: utf8Point{1, 0}, length: 1, negative: true, expectedCaptureName: "number"},
			"Failure - row: 1, column: 0, expected highlight '!number', actual highlights: 'number'"},
		{"no highlight", assertion{position: utf8Point{2, 0}, length: 1, negative: true, expectedCaptureName: "x"},
			"Failure - row: 2, column: 0, expected highlight '!x', actual highlights: none."},
		{"arrows over two highlights", assertion{position: utf8Point{0, 1}, length: 3, expectedCaptureName: "escape"}, ""},
	} {
		n, err := iterateAssertions([]assertion{c.a}, highlights, names)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: got the error %v", c.name, err)
		case c.want == "" && n != 1:
			t.Errorf("%s: counted %d assertions", c.name, n)
		case c.want != "" && (err == nil || err.Error() != c.want):
			t.Errorf("%s: got the error %v, want %s", c.name, err, c.want)
		case c.want != "" && !errors.Is(err, errAssertion):
			t.Errorf("%s: the error %v is not errAssertion", c.name, err)
		}
	}
}

// TestReadModule makes sure that the folder of a package in its module comes
// from the working folder, or from the name of its language when no entry
// of tree-sitter.json has the working folder, and that a case runs in the
// package that D83 names.
func TestReadModule(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"grammars": [{"name": "typescript", "path": "typescript"}, {"name": "tsx", "path": "tsx"}, {"name": "flow", "path": "tsx"}]}`
	if err := os.WriteFile(filepath.Join(dir, "tree-sitter.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"typescript", "tsx", "other"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ wd, own string }{
		{"typescript", "typescript"},
		{"tsx", "tsx"},
		{"other", "tsx"},
		{".", "tsx"},
	} {
		t.Chdir(filepath.Join(dir, c.wd))
		m, err := readModule("tsx")
		if err != nil {
			t.Fatal(err)
		}
		if m.own != c.own || len(m.entries) != 3 {
			t.Errorf("in %s: own %q with %d entries, want %q with 3", c.wd, m.own, len(m.entries), c.own)
		}
	}
	t.Chdir(filepath.Join(dir, "tsx"))
	m, err := readModule("tsx")
	if err != nil {
		t.Fatal(err)
	}
	r := &corpusRun{module: m}
	for _, c := range []struct {
		languages []string
		owner     string
	}{
		{[]string{""}, "typescript"},
		{[]string{"tsx"}, "tsx"},
		{[]string{"flow"}, "tsx"},
		{[]string{"typescript"}, "typescript"},
		{[]string{"css"}, "tsx"},
	} {
		if got := r.owner(testAttributes{languages: c.languages}); got != c.owner {
			t.Errorf("owner of %q = %q, want %q", c.languages, got, c.owner)
		}
	}
	t.Chdir(t.TempDir())
	if m, err := readModule("tsx"); err != nil || m.own != "." || len(m.entries) != 0 {
		t.Errorf("with no tree-sitter.json, readModule = %+v, %v", m, err)
	}
}

// TestPackageQueryPath makes sure that a query file of another grammar that
// tree-sitter.json lists is in the folder of that grammar under queries/
// (D84), and that any other path stays the same.
func TestPackageQueryPath(t *testing.T) {
	for p, want := range map[string]string{
		"queries/highlights.scm": "queries/highlights.scm",
		"./queries/locals.scm":   "queries/locals.scm",
		"node_modules/tree-sitter-javascript/queries/highlights.scm": "queries/javascript/highlights.scm",
		"node_modules/tree-sitter-c/queries/highlights.scm":          "queries/c/highlights.scm",
		"node_modules/tree-sitter-c-sharp/queries/tags.scm":          "queries/c_sharp/tags.scm",
		"node_modules/other/queries/highlights.scm":                  "node_modules/other/queries/highlights.scm",
	} {
		if got := packageQueryPath(p); got != want {
			t.Errorf("packageQueryPath(%q) = %q, want %q", p, got, want)
		}
	}
}
