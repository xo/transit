package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/jsdoc"
)

func init() {
	goPackages = append(goPackages, goPackage{"jsdoc", jsdoc.Language})
}

// jsdocScannerInputs are inputs that reach each branch of the scanner of
// jsdoc: a type with braces that balance and that do not, a newline and a
// NUL character inside a type, and the end of the input.
var jsdocScannerInputs = []string{
	"/** @param {string} a */",
	"/** @param {} a */",
	"/** @param {{a: string}} a */",
	"/** @param {{a: {b: c}}} a */",
	"/** @param {Array<{a}>} a */",
	"/** @param {string",
	"/** @param {{a}",
	"/** @param {",
	"/** @param {string\n * } a */",
	"/** @param {a\x00b} c */",
	"/** @param {a\r\nb} c */",
	"/** {@link a} */",
	"/** {@link {a}} */",
	"/** @returns {a}} */",
	"/** @type {a}{b} */",
	"/** @param {a} {b} c */",
	"/**\n * @param {a\n * @param {b} c\n */",
	"/** @param {é} a */",
	"/** @param {\xff} a */",
	"",
}

// TestGoPackageScannerJsdoc compares the Go scanner of jsdoc with the C
// scanner, call by call, on every corpus input, the error corpus of
// upstream and the inputs that reach each branch of the scanner.
func TestGoPackageScannerJsdoc(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"jsdoc", jsdoc.Language})
	inputs := make([][]byte, 0, len(examples)+len(jsdocScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range jsdocScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, jsdoc.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerJsdocDeserialize gives Deserialize of the Go scanner
// and of the C scanner of jsdoc the same random bytes, and compares the
// bytes that Serialize writes after it.
func TestGoPackageScannerJsdocDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"jsdoc", jsdoc.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(jsdoc.Language()).ExternalScanner.Create)
}
