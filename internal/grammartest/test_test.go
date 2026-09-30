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

func TestCheckAssertions(t *testing.T) {
	infos := []captureInfo{
		{name: "string", start: utf8Point{0, 0}, end: utf8Point{0, 10}},
		{name: "escape", start: utf8Point{0, 2}, end: utf8Point{0, 4}},
		{name: "number", start: utf8Point{1, 0}, end: utf8Point{1, 3}},
	}
	for _, c := range []struct {
		name string
		a    assertion
		ok   bool
	}{
		{"the innermost capture", assertion{position: utf8Point{0, 2}, length: 1, expectedCaptureName: "escape"}, true},
		{"the outer capture", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "string"}, true},
		{"a wrong name", assertion{position: utf8Point{0, 6}, length: 1, expectedCaptureName: "number"}, false},
		{"a negative assertion", assertion{position: utf8Point{1, 0}, length: 2, negative: true, expectedCaptureName: "string"}, true},
		{"a negative assertion that fails", assertion{position: utf8Point{1, 0}, length: 1, negative: true, expectedCaptureName: "number"}, false},
		{"no capture", assertion{position: utf8Point{2, 0}, length: 1, negative: true, expectedCaptureName: "x"}, false},
		{"arrows past the end", assertion{position: utf8Point{1, 1}, length: 3, expectedCaptureName: "number"}, false},
	} {
		n, err := checkAssertions(infos, []assertion{c.a})
		if ok := err == nil; ok != c.ok {
			t.Errorf("%s: got the error %v", c.name, err)
		}
		if err != nil && !errors.Is(err, errAssertion) {
			t.Errorf("%s: the error %v is not errAssertion", c.name, err)
		}
		if err == nil && n != 1 {
			t.Errorf("%s: counted %d assertions", c.name, n)
		}
	}
}

func TestModuleFolderName(t *testing.T) {
	for repo, want := range map[string]string{
		"https://github.com/tree-sitter/tree-sitter-json":              "json",
		"https://github.com/tree-sitter/tree-sitter-embedded-template": "embeddedtemplate",
		"https://github.com/DerekStride/tree-sitter-sql":               "sql",
	} {
		if got := moduleFolderName(repo); got != want {
			t.Errorf("moduleFolderName(%q) = %q, want %q", repo, got, want)
		}
	}
}
