package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file ports the tests of parse_test_content in crates/cli/src/test.rs
// that the port of the corpus reader keeps (D35): the tests of the fields
// that the port has. The other tests check the report and the update of a
// corpus file, which the port leaves out.

// example is what a test expects of a corpus test.
type example struct {
	Name   string
	Input  string
	Output string
}

// checkContent parses content and compares its tests with want.
func checkContent(t *testing.T, content string, want []example) {
	t.Helper()
	entry := parseTestContent("the-filename", strings.TrimSpace(content), "")
	if entry.Name != "the-filename" || !entry.IsGroup {
		t.Fatalf("the group is %q, group %t", entry.Name, entry.IsGroup)
	}
	if len(entry.Children) != len(want) {
		t.Fatalf("found %d tests, want %d: %+v", len(entry.Children), len(want), entry.Children)
	}
	for i, w := range want {
		got := entry.Children[i]
		if got.Name != w.Name || string(got.Input) != w.Input || got.Output != w.Output {
			t.Errorf("test %d is %q %q %q, want %q %q %q", i, got.Name, got.Input, got.Output, w.Name, w.Input, w.Output)
		}
		if a := got.Attributes; !a.Platform || a.FailFast || a.Expectation != ExpectPass || a.CST || len(a.Languages) != 1 || a.Languages[0] != "" {
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
	if len(entry.Children) != 2 {
		t.Fatalf("found %d tests, want 2", len(entry.Children))
	}
	if a := entry.Children[0].Attributes; a.Expectation != ExpectSkip {
		t.Errorf("the first test expects %d, want :skip, which wins over :error", a.Expectation)
	}
	a := entry.Children[1].Attributes
	if strings.Join(a.Languages, ",") != "x,y" || !a.FailFast || !a.CST || a.Platform {
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
		if got := StripSexpFields(c.in); got != c.want {
			t.Errorf("StripSexpFields(%q) = %q, want %q", c.in, got, c.want)
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
	entry, err := Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range entry.Children {
		names = append(names, file.Name+"/"+file.Children[0].Name)
	}
	if got := strings.Join(names, " "); got != "a/A b/B" {
		t.Errorf("the tests are %s, want a/A b/B", got)
	}
}
