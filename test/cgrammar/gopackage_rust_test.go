package cgrammar

import (
	"strings"
	"testing"

	"github.com/xo/transit/grammars/rust"
)

func init() {
	goPackages = append(goPackages, goPackage{"rust", rust.Language})
}

// rustScannerInputs are inputs that reach the branches of the scanner of
// rust that the corpus does not reach, or reaches only a few times.
var rustScannerInputs = []string{
	// block comments, with doc markers, nesting and no end
	"/*! inner */\nfn a() {}\n", "/** outer */\nfn a() {}\n", "/**/\n", "/***/\n", "/*!*/\n", "/****/\n",
	"/* a /* b */ c */\n", "/* a /* b /* c */ */ */ x\n", "/* a */ /* b */\n", "/* no end", "/** no end", "/*! no end",
	"/** a ** b */\n", "/*/ a */\n", "/* a **/\n", "/*/*/ a */*/\n", "/* a", "/*",
	// a character whose low byte is '*', '/' or '!', which the C scanner keeps in a char
	"/*Ī*/\n", "/*į a */\n", "/*ġ*/\n", "/* a Ī/ */\n",
	// strings
	"\"abc\"\n", "\"a\\nb\"\n", "\"\"\n", "\"abc", "\"a\\", "b\"abc\"\n", "c\"abc\"\n",
	// line doc comments
	"/// doc\nfn a() {}\n", "//! doc\n", "/// doc", "///", "//! a\r\n//! b\n",
	// raw strings
	"r\"x\";\n", "r#\"x\"#;\n", "br##\"a\"#b\"##;\n", "cr\"x\";\n", "b\"x\";\n", "r#x;\n", "r#\"abc", "r#\"a\"b\"#;\n",
	"br\"\";\n", "rb\"x\";\n", "r##\"a\"#\"##;\n",
	// 256 hashes, which the count of one byte of the scanner does not hold
	"r" + strings.Repeat("#", 256) + "\"a\"" + strings.Repeat("#", 256) + ";\n",
	"r" + strings.Repeat("#", 257) + "\"a\"#\"" + strings.Repeat("#", 257) + ";\n",
	// floats
	"1.0;\n", "1e10;\n", "1E+5;\n", "1e-x;\n", "1.max(2);\n", "1..2;\n", "1.0f32;\n", "1.0u8;\n", "1e5f64;\n",
	"1.0f;\n", "1_000.5_0;\n", "1.;\n", "1u32;\n", "1.0i;\n", "1.e5;\n", "1.0e;\n", "1.0e+;\n", "0.5.0;\n", "1._;\n",
	"x.0.1;\n", "1.0é;\n", "1.é;\n",
	// whitespace before a raw string and a float
	"let x =  \t r\"x\";\n", "let x =\n\n  1.5;\n", "let x =  1.5;\n",
	// errors, where the parser gives every token as valid
	"fn a( { \"x\" r#\"y\"# 1.5 /* c */ }\n", "@@ r\"x\" 1.0 /// d\n", "fn { /*! */ } 1e\n",
	// the end of the input
	"", "/", "r", "1", "1.", "1e",
	// bytes that are not UTF-8, and characters that are not ASCII
	"\"\xff\"\n", "/* \xff */\n", "r#\"\xff\"#;\n", "let σ = 1.5é;\n",
}

// rustScannerTexts are the texts that TestRustScannerScans scans from each
// position.
var rustScannerTexts = []string{
	"/*!*/ /** a */ /***/ /* /* */ */ /*/",
	"\"a\\\" r#\"b\"# br\"c\" cr#\"d\"",
	"1.0e+5f64 1.max 1..2 1_0.5u8 1e-x 1.0i",
	"/// a\n//! b\r\n",
	" \t\r\n \xffĪ",
}

// rustScannerStates are states of the scanner of rust: no state, and raw
// strings with 0, 1, 2 and 255 hashes.
var rustScannerStates = [][]byte{nil, {0}, {1}, {2}, {255}}

// TestRustScannerMatchesC parses each corpus input of rust and the inputs of
// rustScannerInputs with the Go runtime, once with the Go scanner and once
// with the C scanner, and compares each call of the two scanners. Upstream
// has no error corpus for rust.
func TestRustScannerMatchesC(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"rust", rust.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range rustScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, rust.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestRustScannerScans makes a Go scanner and a C scanner of rust scan each
// text of rustScannerTexts from each position, from each state of
// rustScannerStates and with each set of valid symbols of the grammar, and
// compares the calls, the results and the states after.
func TestRustScannerScans(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"rust", rust.Language})
	var texts [][]byte
	for _, s := range rustScannerTexts {
		texts = append(texts, []byte(s))
	}
	scans := compareScannerScans(t, g, rust.Language(), rustScannerStates, texts, validSymbolSets(rust.Language()))
	t.Logf("%d scans", scans)
}

// TestRustScannerDeserialize gives a Go scanner and a C scanner of rust
// random states, and compares the states that they serialize after them.
func TestRustScannerDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"rust", rust.Language})
	states := append(randomStates(3, 2000, 4), rustScannerStates...)
	compareScannerStates(t, g, rust.Language(), states)
}
