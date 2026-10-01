package cgrammar

import (
	"testing"

	"github.com/xo/transit/grammars/surrealdb"
)

func init() {
	goPackages = append(goPackages, goPackage{"surrealql", surrealdb.Language})
}

// surrealdbScannerInputs are inputs that reach each branch of the scanner of
// surrealql: objects and blocks after spaces and each kind of comment, keys
// of each kind, the call fn::, and JavaScript functions with strings,
// template literals, comments and braces, closed and not closed.
var surrealdbScannerInputs = []string{
	// objects and blocks
	"RETURN {};",
	"RETURN { };",
	"RETURN {a: 1, b: {c: 2}};",
	"RETURN { # c\n a: 1 };",
	"RETURN { // c\n a: 1 };",
	"RETURN { -- c\n a: 1 };",
	"RETURN { /x: 1 };",
	"RETURN { -1 };",
	"RETURN { 1: 2 };",
	"RETURN { 1a: 2 };",
	"RETURN { 'a': 1, \"b\": 2 };",
	"RETURN { 'a\\'b': 1, \"c\\\"d\": 2 };",
	"RETURN { 'a",
	"RETURN { 'a\\",
	"RETURN { {a: 1} };",
	"RETURN { a };",
	"RETURN { a; b };",
	"RETURN { fn::a(1); };",
	"RETURN { a :: b };",
	"RETURN { a : 1 };",
	"RETURN { a",
	"RETURN {",
	"RETURN # c\n{ a: 1 };",
	"IF true { a: 1 } ELSE { b; };",
	"LET $a = { é: 1 };",
	"RETURN { a\x00: 1 };",
	// JavaScript functions
	"RETURN function() { return 1; };",
	"RETURN function($a, $b) { return $a + $b; };",
	"RETURN function() {\n\treturn { a: 1 };\n};",
	"RETURN function() { return 'a}' + \"b}\" + 'c\\'d'; };",
	"RETURN function() { return 'a\n}; };",
	"RETURN function() { return `a${ {b: 1}.b }c`; };",
	"RETURN function() { return `a${ 'b}' }${ \"c\" }${ `d${e}` }`; };",
	"RETURN function() { return `a\\`b$c$`; };",
	"RETURN function() { // }\n return 1; };",
	"RETURN function() { /* } */ return 1; };",
	"RETURN function() { /* } * / */ return 2 / 1; };",
	"RETURN function() { /* }",
	"RETURN function() { return `a${",
	"RETURN function() { return `a",
	"RETURN function() { return 'a\\",
	"RETURN function() {",
	"RETURN function() x",
	"RETURN function()\n\r\t { return 1; };",
	"",
}

// TestGoPackageScannerSurrealdb compares the Go scanner of surrealql with
// the C scanner, call by call, on every corpus input and the inputs that
// reach each branch of the scanner.
func TestGoPackageScannerSurrealdb(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"surrealql", surrealdb.Language})
	inputs := make([][]byte, 0, len(examples)+len(surrealdbScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range surrealdbScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, surrealdb.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerSurrealdbDeserialize gives Deserialize of the Go
// scanner and of the C scanner of surrealql the same random bytes, and
// compares the bytes that Serialize writes after it.
func TestGoPackageScannerSurrealdbDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"surrealql", surrealdb.Language})
	jsCompareDeserialize(t, tablesOf(g.Language).ExternalScanner.Create, tablesOf(surrealdb.Language()).ExternalScanner.Create)
}
