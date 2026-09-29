package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCommittedFiles runs the converter on the crate and makes sure that it
// writes the files of generate/internal/regexsyntax/unicodetables, byte for
// byte. It makes sure too that the package holds no other file that the
// converter wrote, and that the LICENSE of the port is the LICENSE-MIT of the
// crate.
func TestCommittedFiles(t *testing.T) {
	src, err := findSource(crateVersion)
	if err != nil {
		t.Skipf("the source of regex-syntax is not on disk: %v", err)
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "generate", "internal", "regexsyntax", "unicodetables")
	out := t.TempDir()
	if err := checkCrateVersion(src, crateVersion); err != nil {
		t.Fatal(err)
	}
	if err := convert(io.Discard, src, out, crateVersion); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range written {
		names = append(names, e.Name())
		want, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(pkg, e.Name()))
		if err != nil {
			t.Errorf("expected the committed file %s, got: %v", e.Name(), err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: expected the output of the converter, got other bytes. Run: cd test && go run ./cmd/regextables", e.Name())
		}
	}
	committed, err := os.ReadDir(pkg)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range committed {
		n := e.Name()
		if n == "doc.go" || strings.HasSuffix(n, "_test.go") || slices.Contains(names, n) {
			continue
		}
		t.Errorf("expected only the files of the converter, doc.go and the tests, got: %s", n)
	}
	mit, err := os.ReadFile(filepath.Join(src, "..", "..", "LICENSE-MIT"))
	if err != nil {
		t.Fatal(err)
	}
	lic, err := os.ReadFile(filepath.Join(pkg, "..", "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lic, mit) {
		t.Errorf("expected generate/internal/regexsyntax/LICENSE to be the LICENSE-MIT of the crate")
	}
}

// TestLex makes sure that the lexer reads each form of literal that the
// table files use.
func TestLex(t *testing.T) {
	for _, c := range []struct {
		src  string
		kind tokenKind
		text string
		r    rune
	}{
		{`'a'`, tokChar, "", 'a'},
		{`'\t'`, tokChar, "", '\t'},
		{`'\n'`, tokChar, "", '\n'},
		{`'\r'`, tokChar, "", '\r'},
		{`'\0'`, tokChar, "", 0},
		{`'\''`, tokChar, "", '\''},
		{`'\\'`, tokChar, "", '\\'},
		{`'"'`, tokChar, "", '"'},
		{`'\"'`, tokChar, "", '"'},
		{`'\x7f'`, tokChar, "", 0x7f},
		{`'\u{7fd}'`, tokChar, "", 0x7fd},
		{`'\u{10ffff}'`, tokChar, "", 0x10ffff},
		{`'\u{1_0000}'`, tokChar, "", 0x10000},
		{`'𖿡'`, tokChar, "", 0x16FE1},
		{`'static`, tokLifetime, "static", 0},
		{`"Cased_Letter"`, tokString, "Cased_Letter", 0},
		{`"a\"b\\c\u{41}"`, tokString, "a\"b\\cA", 0},
		{`""`, tokString, "", 0},
		{`BY_NAME`, tokIdent, "BY_NAME", 0},
		{`V10_0`, tokIdent, "V10_0", 0},
		{`&`, tokPunct, "&", 0},
		{`// a comment`, tokComment, " a comment", 0},
	} {
		toks, err := lex(c.src)
		if err != nil {
			t.Errorf("%s: expected no error, got: %v", c.src, err)
			continue
		}
		if len(toks) != 2 || toks[1].kind != tokEOF {
			t.Errorf("%s: expected one token, got: %v", c.src, toks)
			continue
		}
		if got := toks[0]; got.kind != c.kind || got.text != c.text || got.r != c.r {
			t.Errorf("%s: expected %s %q %U, got: %s %q %U", c.src, c.kind, c.text, c.r, got.kind, got.text, got.r)
		}
	}
}

// TestLexErrors makes sure that the lexer fails on the forms that it does not
// know.
func TestLexErrors(t *testing.T) {
	for _, src := range []string{
		`'ab'`,
		`''`,
		`'é`,
		`'\n`,
		`'\q'`,
		`'\u41'`,
		`'\u{}'`,
		`'\u{1234567}'`,
		`'\u{d800}'`,
		`'\u{110000}'`,
		`'\x80'`,
		`'\x4'`,
		"'\t'",
		`'static'`,
		`"abc`,
		"\"a\\\nb\"",
		`/* a */`,
		`1`,
		`+`,
		"\xff",
	} {
		if toks, err := lex(src); err == nil {
			t.Errorf("%q: expected an error, got: %v", src, toks)
		}
	}
}

// TestNames makes sure that the Rust names become the Go names that doc.go of
// the package describes.
func TestNames(t *testing.T) {
	for _, c := range []struct {
		base, name, want string
	}{
		{"general_category", "BY_NAME", "GeneralCategoryByName"},
		{"general_category", "CASED_LETTER", "GeneralCategoryCasedLetter"},
		{"perl_word", "PERL_WORD", "PerlWord"},
		{"case_folding_simple", "CASE_FOLDING_SIMPLE", "CaseFoldingSimple"},
		{"property_names", "PROPERTY_NAMES", "PropertyNames"},
		{"property_values", "PROPERTY_VALUES", "PropertyValues"},
		{"perl_space", "WHITE_SPACE", "PerlSpaceWhiteSpace"},
		{"age", "V1_1", "AgeV1_1"},
		{"age", "V11_0", "AgeV11_0"},
		{"grapheme_cluster_break", "LVT", "GraphemeClusterBreakLvt"},
	} {
		got, err := goName(c.base, c.name)
		if err != nil {
			t.Errorf("%s %s: expected no error, got: %v", c.base, c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s %s: expected %s, got: %s", c.base, c.name, c.want, got)
		}
	}
	for _, name := range []string{"A__B", "_A", "A-B"} {
		if got, err := goName("age", name); err == nil {
			t.Errorf("%s: expected an error, got: %s", name, got)
		}
	}
}

// header is the header that ucd-generate writes, for the tests.
const header = `// DO NOT EDIT THIS FILE. IT WAS AUTOMATICALLY GENERATED BY:
//
//   ucd-generate test ucd-16.0.0 --chars
//
// Unicode version: 16.0.0.
//
// ucd-generate 0.3.1 is available on crates.io.
`

// TestEmit makes sure that each shape of table becomes the Go code that the
// package unicodetables declares.
func TestEmit(t *testing.T) {
	src := header + `
pub const BY_NAME: &'static [(&'static str, &'static [(char, char)])] =
    &[("Letter", LETTER), ("Other", OTHER)];

pub const LETTER: &'static [(char, char)] = &[('A', 'Z'), ('\u{10ffff}', '\u{10ffff}')];

pub const OTHER: &'static [(char, char)] = &[
    ('\'', '\''),
];

pub const FOLDS: &'static [(char, &'static [char])] = &[('K', &['k', '\u{212a}'])];

pub const NAMES: &'static [(&'static str, &'static str)] = &[("gc", "General_Category"),];

pub const VALUES: &'static [(
    &'static str,
    &'static [(&'static str, &'static str)],
)] = &[("Age", &[("1.1", "V1_1")])];
`
	f, err := parseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := emit("test", "0.8.11", f, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := `// Code generated by test/cmd/regextables from regex-syntax 0.8.11. DO NOT EDIT.
//
// ucd-generate 0.3.1 made the Rust file test.rs with this command, at Unicode 16.0.0:
//
//	ucd-generate test ucd-16.0.0 --chars

package unicodetables

// TestByName is the table BY_NAME of test.rs.
var TestByName = []Named{
	{"Letter", TestLetter},
	{"Other", TestOther},
}

// TestLetter is the table LETTER of test.rs.
var TestLetter = []Range{
	{0x0041, 0x005A},
	{0x10FFFF, 0x10FFFF},
}

// TestOther is the table OTHER of test.rs.
var TestOther = []Range{
	{0x0027, 0x0027},
}

// TestFolds is the table FOLDS of test.rs.
var TestFolds = []Fold{
	{0x004B, []rune{0x006B, 0x212A}},
}

// TestNames is the table NAMES of test.rs.
var TestNames = []Alias{
	{"gc", "General_Category"},
}

// TestValues is the table VALUES of test.rs.
var TestValues = []Property{
	{"Age", []Alias{
		{"1.1", "V1_1"},
	}},
}
`
	if string(got) != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

// TestParseErrors makes sure that the converter fails on the forms that it
// does not know.
func TestParseErrors(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
	}{
		{"no header", `pub const A: &'static [(char, char)] = &[('a', 'a')];`},
		{"another header", strings.Replace(header, "16.0.0.", "sixteen.", 1) + `pub const A: &'static [(char, char)] = &[('a', 'a')];`},
		{"no items", header},
		{"a comment after the header", header + "// note\npub const A: &'static [(char, char)] = &[('a', 'a')];"},
		{"a static", header + `pub static A: &'static [(char, char)] = &[('a', 'a')];`},
		{"no semicolon", header + `pub const A: &'static [(char, char)] = &[('a', 'a')]`},
		{"another lifetime", header + `pub const A: &'a [(char, char)] = &[('a', 'a')];`},
		{"an unknown type", header + `pub const A: &'static [u8] = &[1];`},
		{"a tuple of three", header + `pub const A: &'static [(char, char, char)] = &[('a', 'a', 'a')];`},
		{"an array without a reference", header + `pub const A: &'static [(char, char)] = [('a', 'a')];`},
		{"a raw string", header + `pub const A: &'static [(&'static str, &'static str)] = &[(r"a", "b")];`},
		{"a number", header + `pub const A: &'static [(char, char)] = &[(97, 97)];`},
	} {
		if f, err := parseFile(c.src); err == nil {
			if _, err := emit("test", "0.8.11", f, map[string]string{}); err == nil {
				t.Errorf("%s: expected an error, got none", c.name)
			}
		}
	}
}

// TestEmitErrors makes sure that the converter fails on a value that does not
// fit its type, and on a name that it cannot resolve.
func TestEmitErrors(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
	}{
		{"a string for a char", `pub const A: &'static [(char, char)] = &[("a", 'a')];`},
		{"a char for a string", `pub const A: &'static [(&'static str, &'static str)] = &[('a', "a")];`},
		{"a short tuple", `pub const A: &'static [(char, char)] = &[('a',)];`},
		{"a long tuple", `pub const A: &'static [(char, char)] = &[('a', 'b', 'c')];`},
		{"a missing name", `pub const A: &'static [(&'static str, &'static [(char, char)])] = &[("b", B)];`},
		{"a name of another type", "pub const A: &'static [(&'static str, &'static [(char, char)])] = &[(\"a\", A)];"},
		{"an item twice", `pub const A: &'static [(char, char)] = &[('a', 'a')];
pub const A: &'static [(char, char)] = &[('a', 'a')];`},
		{"a type that is not a slice", `pub const A: &'static str = "a";`},
	} {
		f, err := parseFile(header + c.src)
		if err != nil {
			continue
		}
		if _, err := emit("test", "0.8.11", f, map[string]string{}); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
	f, err := parseFile(header + `pub const BY_NAME: &'static [(char, char)] = &[('a', 'a')];`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := emit("test", "0.8.11", f, map[string]string{"TestByName": "other.rs"}); err == nil {
		t.Errorf("expected an error for a Go name that two tables make, got none")
	}
}

// TestParseModules makes sure that the converter reads the modules of mod.rs,
// and skips the attributes.
func TestParseModules(t *testing.T) {
	src := `#[cfg(feature = "unicode-age")]
pub mod age;

#[cfg(all(feature = "unicode-perl", not(feature = "unicode-gencat")))]
#[allow(dead_code)]
pub mod perl_decimal;

#[cfg(any(
    feature = "unicode-age",
    feature = "unicode-bool",
))]
pub mod property_names;
`
	got, err := parseModules(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"age", "perl_decimal", "property_names"}; !slices.Equal(got, want) {
		t.Errorf("expected %v, got: %v", want, got)
	}
	for _, bad := range []string{"", "#[cfg(feature = \"a\"]\npub mod a;", "pub mod a", "mod a;", "#[cfg(a)"} {
		if got, err := parseModules(bad); err == nil {
			t.Errorf("%q: expected an error, got: %v", bad, got)
		}
	}
}

// TestCheckCrateVersion makes sure that the converter reads the name and the
// version of the crate from its Cargo.toml.
func TestCheckCrateVersion(t *testing.T) {
	for _, c := range []struct {
		toml string
		ok   bool
	}{
		{"[package]\nname = \"regex-syntax\"\nversion = \"0.8.11\"\n", true},
		{"[package]\nname = \"regex-syntax\"\nversion = \"0.8.10\"\n", false},
		{"[package]\nname = \"regex\"\nversion = \"0.8.11\"\n", false},
		{"[package]\nname = \"regex-syntax\"\n\n[dependencies]\nversion = \"0.8.11\"\n", false},
		{"name = \"regex-syntax\"\nversion = \"0.8.11\"\n", false},
	} {
		dir := t.TempDir()
		src := filepath.Join(dir, "src", "unicode_tables")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(c.toml), 0o644); err != nil {
			t.Fatal(err)
		}
		err := checkCrateVersion(src, "0.8.11")
		if c.ok && err != nil {
			t.Errorf("%q: expected no error, got: %v", c.toml, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%q: expected an error, got none", c.toml)
		}
	}
}

// TestRemoveStale makes sure that the converter removes a Go file that it
// wrote before and that no table makes now, and keeps every other file.
func TestRemoveStale(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"old.go":   generatedPrefix + "from regex-syntax 0.8.10. DO NOT EDIT.\n\npackage unicodetables\n",
		"keep.go":  generatedPrefix + "from regex-syntax 0.8.11. DO NOT EDIT.\n\npackage unicodetables\n",
		"doc.go":   "// Package unicodetables holds tables.\npackage unicodetables\n",
		"short.go": "package x\n",
	}
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeStale(io.Discard, dir, map[string][]byte{"keep.go": []byte("x")}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if want := []string{"doc.go", "keep.go", "short.go"}; !slices.Equal(got, want) {
		t.Errorf("expected %v, got: %v", want, got)
	}
}
