package cgrammar

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/xo/transit/grammars/javascript"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"javascript", javascript.Language})
}

// javascriptScannerInputs are inputs that reach each branch of the scanner
// of javascript, in the order of the functions of src/scanner.c.
var javascriptScannerInputs = []string{
	// scan_template_chars
	"`abc`",
	"``",
	"`a${b}c`",
	"`${a}`",
	"`a$b`",
	"`$`",
	"`a\\nb\\`c`",
	"`abc",
	"`a\x00b`",
	"x = `a\nb\n${c}\nd`",
	// scan_whitespace_and_comments
	"a\n// line\nb",
	"a // line\nb",
	"a /* block */\nb",
	"a /* block\n */ b",
	"a /* block */ b",
	"a /* block */ /* again */ b",
	"a /* block */// line\nb",
	"a /* x */, b",
	"a /* x\n */, b",
	"a /* x\n */ = b",
	"a /* x */\n/ b",
	"a /* x",
	"a /* * ** */ b",
	"a /* x */ /\nb",
	"a\n/b/g",
	"a\n/ b",
	"a //\n\n/* c */ b",
	"a\t\v\f\r\nb",
	// scan_automatic_semicolon
	"a",
	"a\n",
	"{ a }",
	"{ a\n}",
	"a b",
	"a\nb",
	"a\n`b`",
	"a\n, b",
	"a\n: b",
	"a\n; b",
	"a\n* b",
	"a\n% b",
	"a\n> b",
	"a\n< b",
	"a\n= b",
	"a\n[b]",
	"a\n(b)",
	"a\n? b : c",
	"a\n^ b",
	"a\n| b",
	"a\n& b",
	"a\n/ b",
	"a\n.5",
	"a\n.b",
	"a\n++b",
	"a\n+ b",
	"a\n--b",
	"a\n- b",
	"a\n!b",
	"a\n!= b",
	"a\ni",
	"a\nif (b) c",
	"a\nin b",
	"a\nins",
	"a\ninst",
	"a\ninstanceof b",
	"a\ninstanceofx",
	"a\ninstanceo",
	"a\ninx",
	"a\nin",
	"let a = 1\nlet b = 2",
	"return\na",
	"a || b\n|| c",
	"do x\nwhile (y)",
	// scan_ternary_qmark
	"a ? b : c",
	"a ?? b",
	"a ?.5 : 1",
	"a ?.b",
	"a?.b",
	"a ?\n.5 : 1",
	"a\n? b : c",
	"a /* x */ ? b : c",
	"a\t?\tb\t:\tc",
	"a ?",
	// scan_html_comment
	"<!-- comment\na",
	"a\n<!-- comment\nb",
	"a\n--> comment\nb",
	"--> comment",
	"<!- x",
	"<! x",
	"<x",
	"-x",
	"- x",
	"a\n<!--",
	"a\n-->",
	"a\n-- > b",
	"a\n<!-- c\u2028b",
	// scan_jsx_text
	"<a>text</a>",
	"<a>  text  </a>",
	"<a>\n  text\n</a>",
	"<a>\n  \n</a>",
	"<a>\n</a>",
	"<a> </a>",
	"<a>text{b}text</a>",
	"<a>a &amp; b</a>",
	"<a>a > b</a>",
	"<a>}</a>",
	"<a>text",
	"<a>\n\t \n  x\n</a>",
	// the tokens of the tables in the external scanner
	"a || b",
	"'a\\nb'",
	"/a[/]b/g",
	"x = /a/ || b",
	"",
	"\n",
}

// javascriptScannerInputsOutsideASCII are inputs whose characters outside
// ASCII reach iswspace, iswdigit and iswalpha. In the C locale, the C
// library answers as for ASCII, and in this package the functions of
// internal/wctype do the same (D46).
var javascriptScannerInputsOutsideASCII = []string{
	"a\u00a0\nb",
	"a\n\u00a0b",
	"a\u2028b",
	"a\u2029b",
	"a\u3000b",
	"a /* x\u2028 */ b",
	"a\n.\u0663",
	"a\nin\u00e9",
	"a\ninstanceof\u00e9",
	"a ?.\u0663 : 1",
	"a\u00a0? b : c",
	"\u00a0<!-- c",
	"\u2028<!-- c",
	"<a>\n\u00a0\n</a>",
	"<a>\u00a0</a>",
	"let x = { a: // \u2028 3\n}",
}

// TestGoPackageScannerJavascript compares the Go scanner of javascript with
// the C scanner, call by call, on every corpus input, the error corpus of
// upstream and the inputs that reach each branch of the scanner.
func TestGoPackageScannerJavascript(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"javascript", javascript.Language})
	inputs := make([][]byte, 0, len(examples)+len(javascriptScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range javascriptScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, javascript.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerJavascriptOutsideASCII compares the two scanners of
// javascript on input with characters outside ASCII, where iswspace,
// iswdigit and iswalpha of the C library and the package unicode answer
// differently (D39). With the C locale of internal/wctype, the two scanners
// must match (D46).
func TestGoPackageScannerJavascriptOutsideASCII(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"javascript", javascript.Language})
	var inputs [][]byte
	for _, s := range javascriptScannerInputsOutsideASCII {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, javascript.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerJavascriptDeserialize gives Deserialize of the Go
// scanner and of the C scanner of javascript the same random bytes, and
// compares the bytes that Serialize writes after it.
func TestGoPackageScannerJavascriptDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"javascript", javascript.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(javascript.Language()).ExternalScanner.Create)
}

// jsCompareDeserialize gives Deserialize of a new C scanner and of a
// new Go scanner the same random bytes, of each length up to the size of
// the buffer, and fails the test when Serialize after it writes different
// bytes. The tests of javascript and of jsdoc call it.
func jsCompareDeserialize(t *testing.T, cCreate, goCreate func() abi.Scanner) {
	t.Helper()
	r := rand.New(rand.NewChaCha8([32]byte{'j', 's'}))
	for n := range abi.SerializationBufferSize + 1 {
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = byte(r.Uint32())
		}
		c, gs := cCreate(), goCreate()
		c.Deserialize(slices.Clone(buf))
		gs.Deserialize(slices.Clone(buf))
		cOut := make([]byte, abi.SerializationBufferSize)
		goOut := make([]byte, abi.SerializationBufferSize)
		cn, gn := c.Serialize(cOut), gs.Serialize(goOut)
		if !slices.Equal(cOut[:cn], goOut[:gn]) {
			t.Fatalf("after %d random bytes: C serializes %x, Go serializes %x", n, cOut[:cn], goOut[:gn])
		}
	}
}
