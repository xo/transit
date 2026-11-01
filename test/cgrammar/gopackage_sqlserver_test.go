package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/sqlserver"
)

func init() {
	goPackages = append(goPackages, goPackage{"sqlserver", sqlserver.Language})
}

// sqlServerScannerInputs are inputs that reach each branch of the scanner
// of the SQL Server grammar: each kind of comment, a comment that nests, a
// comment that does not end, a minus and a slash that start no comment, and
// GO alone on its line, with a count and a comment, and in places where it
// is not alone. The inputs also reach each branch of the end of a statement
// with no semicolon: a word that starts a statement, a word that does not, a
// label, GO, FETCH of a query and of a cursor, and WITH before a common
// table expression and before table hints.
var sqlServerScannerInputs = []string{
	// comments
	"SELECT 1 -- a\r\n;",
	"SELECT 1 --",
	"SELECT 5--3\nFROM t",
	"SELECT 5 - -3 FROM t",
	"SELECT 5 -",
	"SELECT /* a */ 1 FROM t",
	"SELECT /* a * / ** b */ 1 FROM t",
	"SELECT /* a /* b */ c */ 1 FROM t",
	"SELECT /* a /* b */ 1",
	"SELECT /* a",
	"SELECT 4 / 2 FROM t",
	"SELECT 4 /",
	// GO alone on its line
	"SELECT 1\nGO\nSELECT 2",
	"SELECT 1;\n  go  \r\nSELECT 2;\nGo 12",
	"GO\nSELECT 1",
	"SELECT 1\nGO 3 -- three times\nSELECT 2",
	"SELECT 1\nGO -x",
	"SELECT 1\nGO x",
	"SELECT 1\nGOTO",
	"SELECT 1\nG",
	"SELECT go FROM t",
	"SELECT 1 GO",
	"SELECT 1\nGO",
	// the end of a statement with no semicolon
	"SELECT 1 PRINT 2\nelse END",
	"SELECT 1 SELECT_x",
	"SELECT 1 SELECT2",
	"SELECT 1 abcdefghijklmnopqrstuvwxyz",
	"SELECT 1 é",
	"SELECT 1 + 2",
	"SELECT 1 retry: SELECT 2",
	"SELECT 1 x::y",
	"SELECT 1\nGO 2 -- c",
	"SELECT 1\n  go x",
	"SELECT a FROM t OFFSET 1 ROWS FETCH NEXT 1 ROWS ONLY",
	"SELECT a FROM t OFFSET 1 ROWS FETCH FIRST FROM c",
	"SELECT a FROM t OFFSET 1 ROWS FETCH NEXT\nFROM c",
	"SELECT 1 FETCH c",
	"SELECT 1 FETCH",
	"SELECT * FROM t WITH (NOLOCK)",
	"SELECT 1 WITH c AS (SELECT 1) SELECT 2",
	"SELECT 1 WITH c (a, b) AS (SELECT 1, 2) SELECT 2",
	"SELECT 1 WITH [c d] AS (SELECT 1) SELECT 2",
	"SELECT 1 WITH \"c\" AS (SELECT 1) SELECT 2",
	"SELECT 1 WITH XMLNAMESPACES ('u' AS p) SELECT 2",
	"SELECT 1 WITH c",
	"SELECT 1 WITH [c",
	"SELECT 1 WITH c (a",
	"SELECT 1 WITH 5",
	"IF @a = 1 SELECT 1 ELSE SELECT 2",
	"SELECT 1 ) SELECT 2",
	// white space and other characters
	" \t\r\n\f\vSELECT 1 FROM t",
	"SELECT 'a -- b', \"c /* d\", [e -- f] FROM t",
	"SELECT é FROM t",
	"",
}

// TestGoPackageScannerSQLServer compares the Go scanner of the SQL Server
// grammar with the C scanner, call by call, on every corpus input and the
// inputs that reach each branch of the scanner.
func TestGoPackageScannerSQLServer(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"sqlserver", sqlserver.Language})
	inputs := make([][]byte, 0, len(examples)+len(sqlServerScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range sqlServerScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, sqlserver.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerSQLServerDeserialize gives Deserialize of the Go
// scanner and of the C scanner of the SQL Server grammar the same random
// bytes, and compares the bytes that Serialize writes after it.
func TestGoPackageScannerSQLServerDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"sqlserver", sqlserver.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(sqlserver.Language()).ExternalScanner.Create)
}
