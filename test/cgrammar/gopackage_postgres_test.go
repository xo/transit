package cgrammar

import (
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/postgres/plpgsql"
	"github.com/xo/transit/grammars/postgres/postgres"
)

func init() {
	goPackages = append(goPackages, postgresPackages...)
}

// postgresPackages are the two packages of the module grammars/postgres,
// postgres and plpgsql, which xo writes (D122). Each has its own scanner.
var postgresPackages = []goPackage{
	{"postgres", postgres.Language},
	{"plpgsql", plpgsql.Language},
}

// postgresScannerTexts are texts that reach each branch of the scanner of
// postgres, which reads the comments and the dollar-quoted strings: a line
// comment with each end, block comments that nest and one with no end, a
// - and a / that start no comment, dollar quotes with the empty tag and
// with tags of each kind of character, a closing tag that matches only in
// part, a tag of 255 characters and one of 256, a parameter, and a string
// with no end.
var postgresScannerTexts = []string{
	"select 1 -- a\n, 2 -- b\r\n, 3 --",
	"select /* a /* b */ c */ 1, /**/ 2, /* * / */ 3",
	"select 1 /* a /* b */",
	"select 4 - 2, 4 / 2, -1",
	"select $$a$$, $$$$, $a$b$a$, $_1$c$_1$, $$x$y$$, $ab$x$a$ $ab$",
	"select $\u00e9$x$\u00e9$, $\u0101$y$\u0101$, \t$t$\n$t$",
	"select $" + strings.Repeat("t", 255) + "$x$" + strings.Repeat("t", 255) + "$",
	"select $" + strings.Repeat("t", 256) + "$x$" + strings.Repeat("t", 256) + "$",
	"select $1, $a, $ $",
	"select $a$x$a",
	"select $$",
	"select $$\xff$$",
}

// plpgsqlScannerTexts are texts that reach each branch of the scanner of
// plpgsql: each end of each token of SQL, the parentheses, the brackets and
// the CASE expressions that hide an end, the first words that a token
// refuses, the names of commands and of parameters before := and =>, the
// strings with and without escapes, the quoted identifiers, the parameters,
// the dollar quotes with each kind of tag, a tag of 64 characters and one of
// 65, the comments inside and before a token, a - and a / that start no
// comment, words outside ASCII and words longer than 16 bytes, and texts
// with no end.
var plpgsqlScannerTexts = []string{
	"select x into y from z; insert into t values (1); x := 1; comment := 2; set = 3; table.c := 4; a[1] := 5;",
	"a > 1 then b; c when d; e loop f; 1..10 loop; g by h loop; i, j into k using l; m, n using o loop",
	"f(a, b) then c; (a then b) then; [a, b] , c; d) e] f; g(h)) i; case when a then b end then c",
	"next x; query y; execute z; reverse 1; key := 1; key => 2; a = b; a : b; a :: int",
	"'a''b;' \"c\"\"d;\" e'x\\'y;' E'z' $1; $a$b;$a$ $$c;$$ $x $t$d;$tt$ $t$; $é$e$é$",
	"a -- b; c\nd; a - b; -- x\n; a /* b /* c */ ; */ d; a / b; /* x */ y; /* z",
	"-1; /2; --c\n-x; /*d*/ /x; - ; /",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ then; _x$y é ā then; xéy then; THEN; Loop; WHEN",
	"$" + strings.Repeat("t", 64) + "$x$" + strings.Repeat("t", 64) + "$; $" + strings.Repeat("t", 65) + "$x",
	"'unterminated",
	"\"unterminated",
	"e'unterminated\\",
	"$tag$unterminated",
	"$$",
	"",
	"   ",
}

// postgresScannerSets returns the sets of valid symbols that a test of the
// scanner of a grammar gives: the sets of validSymbolSets, and for each
// external token a set with only that token valid.
func postgresScannerSets(l *transit.Language) [][]bool {
	sets := validSymbolSets(l)
	n := int(tablesOf(l).ExternalTokenCount)
	for i := range n {
		set := make([]bool, n)
		set[i] = true
		sets = append(sets, set)
	}
	return sets
}

// TestPostgresScannersMatchC parses each corpus input of postgres and of
// plpgsql and the texts of the scanner of each, with the Go runtime, once
// with the Go scanner and once with the C scanner, and compares each call of
// the two scanners.
func TestPostgresScannersMatchC(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gp    goPackage
		texts []string
	}{
		{postgresPackages[0], postgresScannerTexts},
		{postgresPackages[1], plpgsqlScannerTexts},
	} {
		t.Run(c.gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, c.gp)
			inputs := make([][]byte, 0, len(examples)+len(c.texts))
			for _, e := range examples {
				inputs = append(inputs, e.Input)
			}
			for _, s := range c.texts {
				inputs = append(inputs, []byte(s))
			}
			n, calls := compareScanners(t, g, c.gp.language(), inputs)
			if calls == 0 {
				t.Errorf("the scanner was not called on %d inputs", n)
			}
			t.Logf("%d inputs, %d scanner calls", n, calls)
		})
	}
}

// TestPostgresScannerScans makes a Go scanner and a C scanner of postgres
// and of plpgsql scan each of their texts from each position, with each set
// of postgresScannerSets, and compares the calls, the results and the
// states after. Each scan starts from the empty state, from a state of
// random bytes, and from the states of postgres in a dollar-quoted string
// with the empty tag and with the tag t.
func TestPostgresScannerScans(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gp    goPackage
		texts []string
	}{
		{postgresPackages[0], postgresScannerTexts},
		{postgresPackages[1], plpgsqlScannerTexts},
	} {
		t.Run(c.gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, c.gp)
			texts := make([][]byte, 0, len(c.texts))
			for _, s := range c.texts {
				texts = append(texts, []byte(s))
			}
			states := [][]byte{nil, {0xff, 1, 2}, {1, 0}, {1, 1, 't', 0, 0, 0}}
			scans := compareScannerScans(t, g, c.gp.language(), states, texts, postgresScannerSets(c.gp.language()))
			t.Logf("%d scans", scans)
		})
	}
}

// TestPostgresScannerDeserialize gives a Go scanner and a C scanner of
// postgres and of plpgsql random states, and compares the states that they
// serialize after them. The scanner of plpgsql writes no bytes.
func TestPostgresScannerDeserialize(t *testing.T) {
	t.Parallel()
	for _, gp := range postgresPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			compareScannerStates(t, g, gp.language(), randomStates(4, 200, 64))
		})
	}
}
