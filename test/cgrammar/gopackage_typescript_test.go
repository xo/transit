package cgrammar

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/xo/transit/grammars/typescript/tsx"
	"github.com/xo/transit/grammars/typescript/typescript"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages,
		goPackage{"typescript", typescript.Language},
		goPackage{"tsx", tsx.Language},
	)
}

// typescriptPackages are the two packages of the module
// grammars/typescript, which share the scanner of common/scanner.h.
var typescriptPackages = []goPackage{
	{"typescript", typescript.Language},
	{"tsx", tsx.Language},
}

// typescriptScannerInputs are inputs that reach each branch of the scanner
// of tree-sitter-typescript: each function of common/scanner.h, each case of
// scan_automatic_semicolon, and the end of the input in each loop. The last
// group holds characters that are not ASCII, for which iswspace, iswalpha
// and iswdigit of the C locale and the functions of the package unicode give
// different answers (D39). The tests of this package set internal/wctype to
// the C locale (D46), so the two scanners must match on them too.
var typescriptScannerInputs = []string{
	// scan_template_chars
	"`a${b}c\\n`;",
	"`$x $ {y}`;",
	"`${a}${b}`;",
	"`\\``;",
	"``;",
	"`abc",
	"`a$",
	"`\u00e9 \U0001F600`;",
	"f`x${`y${z}`}`;",
	// scan_automatic_semicolon and scan_whitespace_and_comments
	"a\nb",
	"a\n\n\t b",
	"a  \t\n b",
	"a\r\nb",
	"a",
	"a\n",
	"a \n",
	"a\n(b)",
	"a\n[b]",
	"type T = A\n[]",
	"let x: A\n| B",
	"a\n.b",
	"a\n,b",
	"a\n;b",
	"a\n*b",
	"a\n%b",
	"a\n>b",
	"a\n<b",
	"a\n=b",
	"a\n?b:c",
	"a\n^b",
	"a\n|b",
	"a\n&b",
	"a\n/b/",
	"a\n:b",
	"a\n`t`",
	"a\n++b",
	"a\n--b",
	"a\n+b",
	"a\n-b",
	"a\n+",
	"a\n!b",
	"a\n!=b",
	"a\nin b",
	"a\nin",
	"a\ninx",
	"a\ninstanceof b",
	"a\ninstanceofx",
	"a\ninstanceo",
	"a\ninside",
	"a\nib",
	"a\ni",
	"a\nx",
	"a\n{b}",
	"a\n// c\nb",
	"a\n// c",
	"a // c\nb",
	"a\n/* c */ b",
	"a\n/* c\n*/\nb",
	"a\n/* c ** d */ b",
	"a\n/* c",
	"a\n/* c *",
	"a\n/",
	"a\n\u00e9",
	"function f(): void\n{}",
	"function f(): void\nfunction f(a) {}",
	"interface I { f(): void\n g(): void }",
	"declare function f(): void\n",
	"class C { f(): void\n f() {} }",
	"abstract class C { abstract f(): void\n}",
	"x = { a }\n",
	"type F = ({a}: {a: number}) => number;",
	"function f({ a }: T) {}",
	"x ? { a } : b",
	"x ? {a}\n: b",
	"if (a) { b }\nc",
	"{ a } \n : b",
	"{ a }",
	"a }",
	// scan_ternary_qmark
	"a ? b : c",
	"a?b:c",
	"a ?\n b : c",
	"a?.b",
	"a ?? b",
	"a?.[0]",
	"a ? .5 : 1",
	"a ? .b : c",
	"a ? . : c",
	"f(a?: b)",
	"f(a ?  : b)",
	"f(a?, b)",
	"f(a?)",
	"(a?) => 1",
	"a\n? b : c",
	"a // c\n? b : c",
	"a /* c */ ? b : c",
	"a ?",
	"a ? ",
	"let x = a\n?? b",
	"class C { a? = 1; b?: T }",
	"type T = { a?: number, b?(): void }",
	"type T = A extends B ? C : D",
	// scan_closing_comment
	"<!-- x\na",
	"<!-- x",
	"--> x\na",
	"a\n--> x\nb",
	"<!- x\na",
	"<! x",
	"-- x",
	"->x",
	"a;\n<!-- b\nc",
	"a; <!-- b\u2028c",
	"a; <!-- b\u2029c",
	"<!--x",
	// scan_jsx_text, for tsx
	"<a>hi</a>;",
	"<a>\n  hi\n  </a>;",
	"<a>  </a>;",
	"<a>\n  \n</a>;",
	"<a>\n</a>;",
	"<a> hi {b} there &amp; </a>;",
	"<a>x > y</a>;",
	"<a>{b}}</a>;",
	"<a>\u00e9\u00e8</a>;",
	"<a>\n \t x</a>;",
	"<a>text",
	"<a>\n",
	"<Element<T>>hi</Element>;",
	"<>fragment</>;",
	"<a b=\"c\">d<e/>f</a>;",
	// the error recovery and mixed inputs
	"a ?? ? b",
	"let x = ;\n}",
	"`${`",
	"a\n?",
	"function\n",
	"x\n)",
	// iswspace, iswalpha and iswdigit outside ASCII
	"a\u00a0\nb",
	"a\n\u00a0(b)",
	"a\nin\u00e9",
	"a\ninstanceof\u00e9",
	"a ? .\u0661 : 1",
	"a\u2028b",
	"a\u3000? b : c",
	"<!-- x\u2028a",
	"\u00a0<!-- x",
	"<a>\u00a0\n\u2003x</a>;",
	"`\u00a0`;",
}

// TestTypescriptScannersMatchC compares the Go scanner of each package of
// grammars/typescript with its C scanner on each corpus input and on each
// input of typescriptScannerInputs.
func TestTypescriptScannersMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range typescriptPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			inputs := make([][]byte, 0, len(examples)+len(typescriptScannerInputs))
			for _, e := range examples {
				inputs = append(inputs, e.Input)
			}
			for _, s := range typescriptScannerInputs {
				inputs = append(inputs, []byte(s))
			}
			n, calls := compareScanners(t, g, gp.language(), inputs)
			if calls == 0 {
				t.Errorf("the scanner was not called on %d inputs", n)
			}
			t.Logf("%d inputs, %d scanner calls", n, calls)
		})
	}
}

// TestTypescriptScannerDeserializeMatchesC gives random bytes to Deserialize
// of the Go scanner and of the C scanner of each package, and compares what
// Serialize writes after it.
func TestTypescriptScannerDeserializeMatchesC(t *testing.T) {
	t.Parallel()
	for _, gp := range typescriptPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			goCreate := tablesOf(gp.language()).ExternalScanner.Create
			cCreate := tablesOf(g.Language).ExternalScanner.Create
			r := rand.New(rand.NewPCG(1, 2))
			for i := range 1000 {
				in := make([]byte, r.IntN(abi.SerializationBufferSize+1))
				for k := range in {
					in[k] = byte(r.Uint32())
				}
				goScanner, cScanner := goCreate(), cCreate()
				goScanner.Deserialize(in)
				cScanner.Deserialize(in)
				goBuf := make([]byte, abi.SerializationBufferSize)
				cBuf := make([]byte, abi.SerializationBufferSize)
				goN, cN := goScanner.Serialize(goBuf), cScanner.Serialize(cBuf)
				if goN != cN || !bytes.Equal(goBuf[:goN], cBuf[:cN]) {
					t.Fatalf("run %d with %d bytes: Serialize writes %x in Go and %x in C", i, len(in), goBuf[:goN], cBuf[:cN])
				}
			}
		})
	}
}
