package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/mysql"
)

func init() {
	goPackages = append(goPackages, goPackage{"mysql", mysql.Language})
}

// mysqlScannerInputs are inputs that reach each branch of the scanner of
// the MySQL grammar: each kind of comment, a -- before each kind of
// character and at the end of the input, a block comment that does not
// end, and version comments of MySQL and of MariaDB, closed and not closed.
var mysqlScannerInputs = []string{
	// line comments
	"SELECT 1 # a\n;",
	"SELECT 1 #",
	"SELECT 1 -- a\r\n;",
	"SELECT 1 --\ta\n;",
	"SELECT 1 --\n;",
	"SELECT 1 --",
	"SELECT 1 --\x01x",
	"SELECT 1 --\x7f",
	"SELECT 5--3;",
	"SELECT 5 - -3;",
	"SELECT 5 -",
	// block comments
	"SELECT /* a */ 1;",
	"SELECT /* a * / ** b */ 1;",
	"SELECT /**/ 1;",
	"SELECT /* a",
	"SELECT 4 / 2;",
	"SELECT 4 /",
	"SELECT /*+ BKA(t) */ * FROM t;",
	// version comments
	"/*!40101 SET NAMES utf8 */;",
	"/*! SELECT 1 */;",
	"/*M!100100 SELECT 1 */;",
	"/*M SELECT 1 */;",
	"/*Mx */ SELECT 1;",
	"SELECT /*!50000 2 * 3 */ * 4;",
	"SELECT /*!50000 2 */ */ 4;",
	"SELECT 2 */ 3;",
	"/*!40101 SET @a = 1",
	"/*!40101 SET @a = 1 *",
	"/*!40101 /* inner */ SELECT 1 */;",
	// white space and other characters
	" \t\r\n\f\vSELECT 1;",
	"SELECT 'a -- b', \"c # d\", `e /* f`;",
	"SELECT é FROM t;",
	"",
}

// TestGoPackageScannerMysql compares the Go scanner of the MySQL grammar
// with the C scanner, call by call, on every corpus input and the inputs
// that reach each branch of the scanner.
func TestGoPackageScannerMysql(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"mysql", mysql.Language})
	inputs := make([][]byte, 0, len(examples)+len(mysqlScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range mysqlScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, mysql.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerMysqlDeserialize gives Deserialize of the Go scanner
// and of the C scanner of the MySQL grammar the same random bytes, and
// compares the bytes that Serialize writes after it.
func TestGoPackageScannerMysqlDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"mysql", mysql.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(mysql.Language()).ExternalScanner.Create)
}
