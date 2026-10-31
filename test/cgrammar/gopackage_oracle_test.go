package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/oracle"
)

func init() {
	goPackages = append(goPackages, goPackage{"oracle", oracle.Language})
}

// oracleScannerInputs are inputs that reach each branch of the scanner of
// the Oracle grammar: each kind of comment, a comment that does not end,
// each delimiter of a string with the alternative quoting, such a string
// that does not end, a / alone on its line and a / that is a division.
var oracleScannerInputs = []string{
	// comments
	"SELECT 1 -- a\r\n;",
	"SELECT 1 --",
	"SELECT 5--3\nFROM dual;",
	"SELECT 5 - -3 FROM dual;",
	"SELECT 5 -",
	"SELECT /* a */ 1 FROM dual;",
	"SELECT /* a * / ** b */ 1 FROM dual;",
	"SELECT /*+ FULL(t) */ * FROM t;",
	"SELECT /* a",
	// strings with the alternative quoting
	"SELECT q'[a'b]' FROM dual;",
	"SELECT Q'{a}b}' FROM dual;",
	"SELECT q'(a)', q'<b>', q'!c!', q'''' FROM dual;",
	"SELECT nq'[a]', Nq'[b]', nQ'[c]' FROM dual;",
	"SELECT q'[a",
	"SELECT q'",
	"SELECT q' a' FROM dual;",
	"SELECT q, n, nx, qx FROM t;",
	"SELECT n'a', N'b' FROM dual;",
	// a / alone on its line
	"SELECT 1 FROM dual\n/\nSELECT 2 FROM dual",
	"SELECT 1 FROM dual;\n  /  \r\nSELECT 2 FROM dual;\n/",
	"/\nSELECT 1 FROM dual",
	"SELECT 4\n/ 2 FROM dual;",
	"SELECT 4 / 2 FROM dual;",
	"SELECT 4 /\n2 FROM dual;",
	"SELECT 1 FROM dual\n/ x",
	// white space and other characters
	" \t\r\n\f\vSELECT 1 FROM dual;",
	"SELECT 'a -- b', \"c /* d\" FROM dual;",
	"SELECT é FROM t;",
	"",
}

// TestGoPackageScannerOracle compares the Go scanner of the Oracle grammar
// with the C scanner, call by call, on every corpus input and the inputs
// that reach each branch of the scanner.
func TestGoPackageScannerOracle(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"oracle", oracle.Language})
	inputs := make([][]byte, 0, len(examples)+len(oracleScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range oracleScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, oracle.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerOracleDeserialize gives Deserialize of the Go scanner
// and of the C scanner of the Oracle grammar the same random bytes, and
// compares the bytes that Serialize writes after it.
func TestGoPackageScannerOracleDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"oracle", oracle.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(oracle.Language()).ExternalScanner.Create)
}
